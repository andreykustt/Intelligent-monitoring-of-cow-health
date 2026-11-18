package api

import (
	"cowhealth/internal/platform"
	"net/http"
	"strings"
)

func (s *Server) alerts(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if state != "" && state != "open" && state != "acknowledged" && state != "resolved" {
		platform.Error(w, 422, "VALIDATION_ERROR", "Неизвестное состояние")
		return
	}
	s.paged(w, r, `SELECT to_jsonb(a) item,row_number() OVER(ORDER BY a.last_seen DESC) ordinal FROM alerts a JOIN cows c ON c.id=a.cow_id WHERE c.farm_id=ANY($1) AND ($2='' OR state=$2) AND ($3='' OR c.farm_id=$3)`, who(r).Farms, state, r.URL.Query().Get("farm_id"))
}
func (s *Server) alert(w http.ResponseWriter, r *http.Request) {
	s.one(w, r, `SELECT to_jsonb(a)||jsonb_build_object('prediction',to_jsonb(p),'actions',(SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.created_at),'[]') FROM alert_actions x WHERE x.alert_id=a.id)) FROM alerts a JOIN cows c ON c.id=a.cow_id JOIN ml_predictions p ON p.id=a.prediction_id WHERE a.id::text=$1 AND c.farm_id=ANY($2)`, r.PathValue("id"), who(r).Farms)
}
func (s *Server) alertAction(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	target := "acknowledged"
	if action == "resolve" {
		target = "resolved"
	} else if action != "acknowledge" {
		platform.Error(w, 404, "NOT_FOUND", "Действие не найдено")
		return
	}
	var b struct {
		Note string `json:"note"`
	}
	if !platform.Decode(w, r, &b) {
		return
	}
	if action == "resolve" && strings.TrimSpace(b.Note) == "" {
		platform.Error(w, 422, "VALIDATION_ERROR", "Укажите комментарий закрытия")
		return
	}
	ctx := r.Context()
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		s.fail(w, e)
		return
	}
	defer tx.Rollback(ctx)
	var state, cow, farm string
	e = tx.QueryRow(ctx, `SELECT a.state,a.cow_id,c.farm_id FROM alerts a JOIN cows c ON c.id=a.cow_id WHERE a.id::text=$1 AND c.farm_id=ANY($2) FOR UPDATE OF a`, r.PathValue("id"), who(r).Farms).Scan(&state, &cow, &farm)
	if e != nil {
		platform.Error(w, 404, "NOT_FOUND", "Уведомление не найдено")
		return
	}
	if state == target {
		platform.JSON(w, 200, map[string]string{"state": state})
		return
	}
	if state == "resolved" {
		platform.Error(w, 409, "CONFLICT", "Уведомление уже закрыто")
		return
	}
	_, e = tx.Exec(ctx, "UPDATE alerts SET state=$2 WHERE id::text=$1", r.PathValue("id"), target)
	if e == nil {
		_, e = tx.Exec(ctx, "INSERT INTO alert_actions VALUES($1,$2,$3,$4,$5,now())", platform.ID(), r.PathValue("id"), who(r).ID, action, b.Note)
	}
	if e == nil {
		_, e = tx.Exec(ctx, `INSERT INTO outbox_events(id,event_type,payload,owner) VALUES($1,'alert.updated.v1',jsonb_build_object('id',$2::text,'cow_id',$3::text,'farm_id',$4::text,'state',$5::text),'api-service')`, platform.ID(), r.PathValue("id"), cow, farm, target)
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		s.fail(w, e)
		return
	}
	platform.JSON(w, 200, map[string]string{"state": target})
}
