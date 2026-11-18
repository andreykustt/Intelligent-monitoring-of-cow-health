package api

import (
	"cowhealth/internal/platform"
	"encoding/json"
	"net/http"
)

func (s *Server) simulations(w http.ResponseWriter, r *http.Request) {
	if !admin(w, r) {
		return
	}
	s.paged(w, r, `SELECT to_jsonb(s) item,row_number() OVER(ORDER BY created_at DESC) ordinal FROM simulations s WHERE farm_id=ANY($1) AND ($2='' OR farm_id=$2)`, who(r).Farms, r.URL.Query().Get("farm_id"))
}
func (s *Server) simulation(w http.ResponseWriter, r *http.Request) {
	if !admin(w, r) {
		return
	}
	s.one(w, r, `SELECT to_jsonb(s) FROM simulations s WHERE id::text=$1 AND farm_id=ANY($2)`, r.PathValue("id"), who(r).Farms)
}
func (s *Server) simulationCreate(w http.ResponseWriter, r *http.Request) {
	if !admin(w, r) {
		return
	}
	var b struct {
		Farm   string             `json:"farm_id"`
		Config platform.SimConfig `json:"config"`
	}
	if !platform.Decode(w, r, &b) {
		return
	}
	if !farmAllowed(r, b.Farm) {
		platform.Error(w, 403, "FORBIDDEN", "Ферма недоступна")
		return
	}
	if e := b.Config.Validate(); e != nil {
		platform.Error(w, 422, "VALIDATION_ERROR", e.Error())
		return
	}
	var n int
	if e := s.DB.QueryRow(r.Context(), "SELECT count(*) FROM cows WHERE farm_id=$1", b.Farm).Scan(&n); e != nil {
		s.fail(w, e)
		return
	}
	if n < b.Config.CowCount {
		platform.Error(w, 422, "VALIDATION_ERROR", "Недостаточно животных в ферме")
		return
	}
	id := platform.ID()
	ctx := r.Context()
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		s.fail(w, e)
		return
	}
	defer tx.Rollback(ctx)
	raw, _ := json.Marshal(b.Config)
	_, e = tx.Exec(ctx, "INSERT INTO simulations(id,farm_id,state,config) VALUES($1,$2,'starting',$3)", id, b.Farm, raw)
	if e == nil {
		_, e = tx.Exec(ctx, "INSERT INTO simulation_commands VALUES($1,$2,1,'start',$3,now())", platform.ID(), id, raw)
	}
	if e == nil {
		_, e = tx.Exec(ctx, `INSERT INTO outbox_events(id,event_type,payload,owner) VALUES($1,'simulation.command.v1',jsonb_build_object('simulation_id',$2::text,'version',1,'action','start'),'api-service')`, platform.ID(), id)
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		s.fail(w, e)
		return
	}
	platform.JSON(w, 202, map[string]string{"id": id, "state": "starting"})
}
func (s *Server) simulationAction(w http.ResponseWriter, r *http.Request) {
	if !admin(w, r) {
		return
	}
	action := r.PathValue("action")
	if r.Method == "PATCH" {
		action = "config"
	}
	if action != "pause" && action != "resume" && action != "stop" && action != "config" {
		platform.Error(w, 404, "NOT_FOUND", "Неизвестная команда")
		return
	}
	ctx := r.Context()
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		s.fail(w, e)
		return
	}
	defer tx.Rollback(ctx)
	var state string
	var config json.RawMessage
	var version int
	e = tx.QueryRow(ctx, "SELECT state,config,version FROM simulations WHERE id::text=$1 AND farm_id=ANY($2) FOR UPDATE", r.PathValue("id"), who(r).Farms).Scan(&state, &config, &version)
	if e != nil {
		platform.Error(w, 404, "NOT_FOUND", "Симуляция не найдена")
		return
	}
	if state == "completed" || state == "stopped" || state == "failed" {
		platform.Error(w, 409, "CONFLICT", "Симуляция завершена")
		return
	}
	if action == "config" {
		var c platform.SimConfig
		if !platform.Decode(w, r, &c) {
			return
		}
		if e = c.Validate(); e != nil {
			platform.Error(w, 422, "VALIDATION_ERROR", e.Error())
			return
		}
		var old platform.SimConfig
		_ = json.Unmarshal(config, &old)
		if c.CowCount != old.CowCount || c.Mode != old.Mode {
			platform.Error(w, 422, "VALIDATION_ERROR", "Число животных и режим меняются только при новом запуске")
			return
		}
		config, _ = json.Marshal(c)
	}
	var last string
	if e = tx.QueryRow(ctx, "SELECT action FROM simulation_commands WHERE simulation_id::text=$1 ORDER BY version DESC LIMIT 1", r.PathValue("id")).Scan(&last); e != nil {
		s.fail(w, e)
		return
	}
	if last == action && action != "config" {
		platform.JSON(w, 202, map[string]string{"state": state})
		return
	}
	if action == "pause" && state != "running" || action == "resume" && state != "paused" {
		platform.Error(w, 409, "CONFLICT", "Команда недоступна в текущем состоянии")
		return
	}
	_, e = tx.Exec(ctx, "UPDATE simulations SET version=version+1,config=$2,updated_at=now() WHERE id::text=$1", r.PathValue("id"), config)
	if e == nil {
		_, e = tx.Exec(ctx, "INSERT INTO simulation_commands VALUES($1,$2,$3,$4,$5,now())", platform.ID(), r.PathValue("id"), version+1, action, config)
	}
	if e == nil {
		_, e = tx.Exec(ctx, `INSERT INTO outbox_events(id,event_type,payload,owner) VALUES($1,'simulation.command.v1',jsonb_build_object('simulation_id',$2::text,'version',$3::integer,'action',$4::text),'api-service')`, platform.ID(), r.PathValue("id"), version+1, action)
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		s.fail(w, e)
		return
	}
	platform.JSON(w, 202, map[string]any{"version": version + 1, "state": state})
}
