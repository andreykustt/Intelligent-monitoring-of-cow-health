package api

import (
	"cowhealth/internal/platform"
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"time"
)

type manualInput struct {
	Cow      string         `json:"cow_id"`
	Observed time.Time      `json:"observed_at"`
	Payload  map[string]any `json:"payload"`
	Version  int            `json:"version"`
	Reason   string         `json:"reason"`
}

func manualKind(r *http.Request) string {
	return strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/"), "/")[0]
}
func validateManual(kind string, b manualInput) string {
	if b.Cow == "" || b.Observed.IsZero() || b.Observed.After(time.Now().Add(time.Minute)) {
		return "Укажите животное и время наблюдения не в будущем"
	}
	required := map[string][]string{"milkings": {"yield_l", "flow_l_min", "temperature_c", "conductivity_fl", "conductivity_fr", "conductivity_rl", "conductivity_rr"}, "inspections": {"appetite", "gait", "condition"}, "laboratory-results": {"test", "value", "unit"}, "body-condition-records": {"bcs", "hoof"}, "veterinary-examinations": {"reproductive_status", "notes"}}
	optional := map[string][]string{"milkings": {"weight_kg"}, "inspections": {"notes"}, "veterinary-examinations": {"rectal_temperature_c"}}
	allowed := map[string]bool{}
	for _, k := range append(required[kind], optional[kind]...) {
		allowed[k] = true
	}
	if len(required[kind]) == 0 {
		return "Неизвестный тип записи"
	}
	for _, k := range required[kind] {
		v, ok := b.Payload[k]
		if !ok || v == nil || v == "" {
			return "Заполните поле: " + k
		}
	}
	numeric := map[string]bool{}
	for _, k := range []string{"yield_l", "flow_l_min", "temperature_c", "conductivity_fl", "conductivity_fr", "conductivity_rl", "conductivity_rr", "weight_kg", "value", "bcs", "rectal_temperature_c"} {
		numeric[k] = true
	}
	enums := map[string][]string{"appetite": {"обычный", "снижен", "отсутствует"}, "gait": {"без замечаний", "изменена", "затруднена"}, "condition": {"без замечаний", "требует внимания", "требует осмотра"}, "hoof": {"без замечаний", "требует внимания", "требует осмотра"}, "reproductive_status": {"не указан", "стельная", "не стельная", "после отёла"}}
	for k, v := range b.Payload {
		if !allowed[k] {
			return "Неизвестное поле: " + k
		}
		if numeric[k] {
			n, ok := v.(float64)
			if !ok || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 1e9 {
				return "Некорректное число: " + k
			}
			if k == "bcs" && (n < 1 || n > 5 || math.Mod(n*4, 1) != 0) {
				return "BCS: от 1 до 5 с шагом 0,25"
			}
		} else {
			text, ok := v.(string)
			if !ok || len(text) > 10000 {
				return "Некорректный текст: " + k
			}
			if choices := enums[k]; choices != nil {
				valid := false
				for _, choice := range choices {
					if text == choice {
						valid = true
					}
				}
				if !valid {
					return "Неизвестная категория: " + k
				}
			}
		}
	}
	if kind == "laboratory-results" {
		units := map[string]string{"bhb": "mmol/L", "calcium": "mmol/L", "scc": "cells/mL", "fat": "%", "protein": "%", "lactose": "%"}
		test, _ := b.Payload["test"].(string)
		if units[test] == "" || b.Payload["unit"] != units[test] {
			return "Проверьте анализ и единицу измерения"
		}
	}
	return ""
}

func (s *Server) manualCreate(w http.ResponseWriter, r *http.Request) {
	var b manualInput
	if !platform.Decode(w, r, &b) {
		return
	}
	if msg := validateManual(manualKind(r), b); msg != "" {
		platform.Error(w, 422, "VALIDATION_ERROR", msg)
		return
	}
	if !s.cowAllowed(w, r, b.Cow) {
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if len(key) < 8 || len(key) > 128 {
		platform.Error(w, 422, "VALIDATION_ERROR", "Нужен Idempotency-Key длиной 8–128 символов")
		return
	}
	raw, _ := json.Marshal(b.Payload)
	ctx := r.Context()
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		s.fail(w, e)
		return
	}
	defer tx.Rollback(ctx)
	id := platform.ID()
	var returned string
	e = tx.QueryRow(ctx, `INSERT INTO manual_records(id,kind,cow_id,observed_at,created_by,payload,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(created_by,idempotency_key) DO UPDATE SET idempotency_key=EXCLUDED.idempotency_key RETURNING id::text`, id, manualKind(r), b.Cow, b.Observed, who(r).ID, raw, key).Scan(&returned)
	if e != nil {
		s.fail(w, e)
		return
	}
	if returned == id {
		_, e = tx.Exec(ctx, `INSERT INTO manual_revisions SELECT id,version,payload,observed_at,created_by,created_at,'Создание' FROM manual_records WHERE id=$1`, id)
		if e == nil {
			_, e = tx.Exec(ctx, `INSERT INTO outbox_events(id,event_type,payload,owner) SELECT $1,'manual_measurement.created.v1',jsonb_build_object('cow_id',c.id,'farm_id',c.farm_id,'record_id',$2::text),'api-service' FROM cows c WHERE c.id=$3`, platform.ID(), id, b.Cow)
		}
	} else {
		var same bool
		e = tx.QueryRow(ctx, `SELECT cow_id=$2 AND kind=$3 AND observed_at=$4 AND payload=$5 FROM manual_records WHERE id=$1`, returned, b.Cow, manualKind(r), b.Observed, raw).Scan(&same)
		if e == nil && !same {
			platform.Error(w, 409, "CONFLICT", "Ключ уже использован для другого запроса")
			return
		}
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		s.fail(w, e)
		return
	}
	platform.JSON(w, 201, map[string]any{"id": returned})
}
func (s *Server) manualGet(w http.ResponseWriter, r *http.Request) {
	s.one(w, r, `SELECT to_jsonb(m)||jsonb_build_object('created_by_name',(SELECT username FROM users WHERE id=m.created_by)) FROM manual_records m JOIN cows c ON c.id=m.cow_id WHERE m.id::text=$1 AND m.kind=$2 AND c.farm_id=ANY($3)`, r.PathValue("id"), manualKind(r), who(r).Farms)
}
func (s *Server) timeline(w http.ResponseWriter, r *http.Request) {
	if !s.cowAllowed(w, r, r.PathValue("id")) {
		return
	}
	s.paged(w, r, `SELECT item,row_number() OVER(ORDER BY event_at DESC,id) ordinal FROM (
 SELECT m.id::text id,m.observed_at event_at,jsonb_build_object('entry_type','manual','event_at',m.observed_at,'data',to_jsonb(m)||jsonb_build_object('created_by_name',(SELECT username FROM users WHERE id=m.created_by))) item FROM manual_records m WHERE cow_id=$1
 UNION ALL SELECT p.id::text,p.window_end,jsonb_build_object('entry_type','prediction','event_at',p.window_end,'data',to_jsonb(p)) FROM ml_predictions p WHERE cow_id=$1
 UNION ALL SELECT a.id::text,a.created_at,jsonb_build_object('entry_type','alert','event_at',a.created_at,'data',to_jsonb(a)) FROM alerts a WHERE cow_id=$1
 )timeline`, r.PathValue("id"))
}
func (s *Server) revisions(w http.ResponseWriter, r *http.Request) {
	s.list(w, r, `SELECT to_jsonb(v)||jsonb_build_object('created_by_name',(SELECT username FROM users WHERE id=v.created_by)) FROM manual_revisions v JOIN manual_records m ON m.id=v.record_id JOIN cows c ON c.id=m.cow_id WHERE m.id::text=$1 AND m.kind=$2 AND c.farm_id=ANY($3) ORDER BY v.version DESC`, r.PathValue("id"), manualKind(r), who(r).Farms)
}
func (s *Server) manualEdit(w http.ResponseWriter, r *http.Request) {
	var b manualInput
	if !platform.Decode(w, r, &b) {
		return
	}
	if msg := validateManual(manualKind(r), b); msg != "" || strings.TrimSpace(b.Reason) == "" {
		platform.Error(w, 422, "VALIDATION_ERROR", "Проверьте поля и укажите причину исправления. "+msg)
		return
	}
	ctx := r.Context()
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		s.fail(w, e)
		return
	}
	defer tx.Rollback(ctx)
	raw, _ := json.Marshal(b.Payload)
	tag, e := tx.Exec(ctx, `UPDATE manual_records m SET payload=$1,observed_at=$2,version=version+1 FROM cows c WHERE m.cow_id=c.id AND c.farm_id=ANY($3) AND m.id::text=$4 AND m.kind=$5 AND m.version=$6 AND m.cow_id=$7`, raw, b.Observed, who(r).Farms, r.PathValue("id"), manualKind(r), b.Version, b.Cow)
	if e != nil {
		s.fail(w, e)
		return
	}
	if tag.RowsAffected() == 0 {
		platform.Error(w, 409, "CONFLICT", "Запись изменилась или недоступна. Обновите данные")
		return
	}
	_, e = tx.Exec(ctx, `INSERT INTO manual_revisions SELECT id,version,payload,observed_at,$2,now(),$3 FROM manual_records WHERE id::text=$1`, r.PathValue("id"), who(r).ID, b.Reason)
	if e == nil {
		_, e = tx.Exec(ctx, "INSERT INTO audit_log VALUES($1,$2,'manual.corrected',$3,now())", platform.ID(), who(r).ID, r.PathValue("id"))
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		s.fail(w, e)
		return
	}
	platform.JSON(w, 200, map[string]any{"version": b.Version + 1})
}
