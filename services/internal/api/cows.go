package api

import (
	"cowhealth/internal/platform"
	"net/http"
	"strings"
	"time"
)

const cowSelect = `SELECT c.*,COALESCE(p.status,'insufficient_data') status,p.window_end analysis_at,p.model_version, (j.last_error IS NOT NULL) analysis_delayed,COALESCE((SELECT jsonb_object_agg(l.metric_type,jsonb_build_object('value',l.value,'unit',m.unit,'recorded_at',l.recorded_at,'received_at',l.received_at,'source',l.source,'stale',l.recorded_at<now()-make_interval(secs=>3*COALESCE((sim.config->'interval_ms_by_metric'->>l.metric_type)::double precision/1000,5)))) FROM cow_latest_state l JOIN metric_definitions m ON m.code=l.metric_type LEFT JOIN simulations sim ON sim.id=l.simulation_id WHERE l.cow_id=c.id),'{}') latest FROM cows c LEFT JOIN LATERAL(SELECT status,window_end,model_version FROM ml_predictions WHERE cow_id=c.id ORDER BY window_end DESC LIMIT 1)p ON true LEFT JOIN LATERAL(SELECT last_error FROM analysis_jobs WHERE cow_id=c.id AND completed_at IS NULL ORDER BY window_end DESC LIMIT 1)j ON true`

func (s *Server) cows(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	if status != "" && status != "anomaly" && status != "no_anomaly" && status != "insufficient_data" {
		platform.Error(w, 422, "VALIDATION_ERROR", "Неизвестный статус")
		return
	}
	sort := q.Get("sort")
	order := map[string]string{"": "id", "id": "id", "-id": "id DESC", "name": "name", "-name": "name DESC", "status": "status,id"}[sort]
	if order == "" {
		platform.Error(w, 422, "VALIDATION_ERROR", "Недопустимая сортировка")
		return
	}
	s.paged(w, r, `SELECT to_jsonb(t) item,row_number() OVER(ORDER BY `+order+`) ordinal FROM (`+cowSelect+`)t WHERE farm_id=ANY($1) AND ($2='' OR farm_id=$2) AND ($3='' OR id ILIKE '%'||$3||'%' OR name ILIKE '%'||$3||'%') AND ($4='' OR status=$4)`, who(r).Farms, q.Get("farm_id"), q.Get("q"), status)
}
func (s *Server) cow(w http.ResponseWriter, r *http.Request) {
	s.one(w, r, `SELECT to_jsonb(t) FROM (`+cowSelect+`)t WHERE id=$1 AND farm_id=ANY($2)`, r.PathValue("id"), who(r).Farms)
}
func (s *Server) saveCow(w http.ResponseWriter, r *http.Request) {
	if !admin(w, r) {
		return
	}
	var b struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Farm string `json:"farm_id"`
	}
	if !platform.Decode(w, r, &b) {
		return
	}
	if strings.TrimSpace(b.Name) == "" || len(b.Name) > 120 || !farmAllowed(r, b.Farm) {
		platform.Error(w, 422, "VALIDATION_ERROR", "Проверьте имя и доступную ферму")
		return
	}
	id := r.PathValue("id")
	if id != "" {
		if !s.cowAllowed(w, r, id) {
			return
		}
		b.ID = id
	}
	if len(b.ID) < 1 || len(b.ID) > 64 {
		platform.Error(w, 422, "VALIDATION_ERROR", "Нужен идентификатор до 64 символов")
		return
	}
	tx, e := s.DB.Begin(r.Context())
	if e != nil {
		s.fail(w, e)
		return
	}
	defer tx.Rollback(r.Context())
	if id == "" {
		_, e = tx.Exec(r.Context(), "INSERT INTO cows(id,name,farm_id) VALUES($1,$2,$3)", b.ID, b.Name, b.Farm)
	} else {
		_, e = tx.Exec(r.Context(), "UPDATE cows SET name=$2,farm_id=$3 WHERE id=$1", b.ID, b.Name, b.Farm)
	}
	if e != nil {
		platform.Error(w, 409, "CONFLICT", "Идентификатор уже существует или данные некорректны")
		return
	}
	_, e = tx.Exec(r.Context(), "INSERT INTO sensors SELECT $1||':'||code,$1,code FROM metric_definitions ON CONFLICT DO NOTHING", b.ID)
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e != nil {
		s.fail(w, e)
		return
	}
	platform.JSON(w, 200, b)
}
func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	s.list(w, r, "SELECT to_jsonb(m) FROM metric_definitions m ORDER BY code")
}
func (s *Server) latest(w http.ResponseWriter, r *http.Request) {
	s.paged(w, r, `SELECT to_jsonb(t) item,row_number() OVER(ORDER BY cow_id,metric_type) ordinal FROM (SELECT l.*,m.unit FROM cow_latest_state l JOIN cows c ON c.id=l.cow_id JOIN metric_definitions m ON m.code=l.metric_type WHERE c.farm_id=ANY($1) AND ($2='' OR c.farm_id=$2))t`, who(r).Farms, r.URL.Query().Get("farm_id"))
}
func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	metric := q.Get("metric_type")
	if r.PathValue("metric") != "" {
		metric = r.PathValue("metric")
	}
	var exists bool
	if e := s.DB.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM metric_definitions WHERE code=$1)", metric).Scan(&exists); e != nil {
		s.fail(w, e)
		return
	}
	if !exists && metric != "milk_yield_l" {
		platform.Error(w, 422, "VALIDATION_ERROR", "Выберите известную метрику")
		return
	}
	end := time.Now().UTC()
	start := end.Add(-time.Hour)
	var e error
	if v := q.Get("from"); v != "" {
		start, e = time.Parse(time.RFC3339, v)
		if e != nil {
			platform.Error(w, 422, "VALIDATION_ERROR", "Некорректное начало периода")
			return
		}
	}
	if v := q.Get("to"); v != "" {
		end, e = time.Parse(time.RFC3339, v)
		if e != nil {
			platform.Error(w, 422, "VALIDATION_ERROR", "Некорректный конец периода")
			return
		}
	}
	if !end.After(start) || end.Sub(start) > 90*24*time.Hour {
		platform.Error(w, 422, "VALIDATION_ERROR", "Диапазон должен быть от 1 секунды до 90 дней")
		return
	}
	cow := r.PathValue("id")
	if cow == "" {
		cow = q.Get("cow_id")
	}
	if cow != "" && !s.cowAllowed(w, r, cow) {
		return
	}
	bin := int(end.Sub(start).Seconds()/999) + 1
	if metric == "milk_yield_l" {
		s.list(w, r, `WITH bins AS(SELECT generate_series(date_bin(make_interval(secs=>$5),$3::timestamptz,'2000-01-01'),$4::timestamptz,make_interval(secs=>$5)) t), agg AS(SELECT date_bin(make_interval(secs=>$5),observed_at,'2000-01-01') t,sum((payload->>'yield_l')::double precision) value,count(*) count FROM manual_records m JOIN cows c ON c.id=m.cow_id WHERE c.farm_id=ANY($1) AND ($2='' OR cow_id=$2) AND m.kind='milkings' AND observed_at>=$3 AND observed_at<$4 AND ($6='' OR c.farm_id=$6) GROUP BY 1) SELECT jsonb_build_object('recorded_at',b.t,'value',a.value,'min',NULL,'max',NULL,'count',COALESCE(a.count,0)) FROM bins b LEFT JOIN agg a USING(t) ORDER BY b.t`, who(r).Farms, cow, start, end, bin, q.Get("farm_id"))
		return
	}
	s.list(w, r, `WITH bins AS(SELECT generate_series(date_bin(make_interval(secs=>$6),$4::timestamptz,'2000-01-01'),$5::timestamptz,make_interval(secs=>$6)) t), agg AS(SELECT date_bin(make_interval(secs=>$6),recorded_at,'2000-01-01') t,avg(value) mean,min(value) min,max(value) max,count(*) count FROM sensor_measurements l JOIN cows c ON c.id=l.cow_id WHERE c.farm_id=ANY($1) AND ($2='' OR cow_id=$2) AND metric_type=$3 AND recorded_at>=$4 AND recorded_at<$5 AND ($7='' OR c.farm_id=$7) GROUP BY 1) SELECT jsonb_build_object('recorded_at',b.t,'value',a.mean,'min',a.min,'max',a.max,'count',COALESCE(a.count,0)) FROM bins b LEFT JOIN agg a USING(t) ORDER BY b.t`, who(r).Farms, cow, metric, start, end, bin, q.Get("farm_id"))
}
func (s *Server) predictions(w http.ResponseWriter, r *http.Request) {
	if !s.cowAllowed(w, r, r.PathValue("id")) {
		return
	}
	s.paged(w, r, `SELECT to_jsonb(p) item,row_number() OVER(ORDER BY window_end DESC) ordinal FROM ml_predictions p WHERE cow_id=$1`, r.PathValue("id"))
}
func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	s.one(w, r, `WITH herd AS(`+cowSelect+`) SELECT jsonb_build_object('total',count(*),'anomaly',count(*) FILTER(WHERE status='anomaly'),'no_anomaly',count(*) FILTER(WHERE status='no_anomaly'),'insufficient_data',count(*) FILTER(WHERE status='insufficient_data'),'with_data',count(*) FILTER(WHERE latest!='{}'),'alerts',(SELECT count(*) FROM alerts a JOIN cows c ON c.id=a.cow_id WHERE c.farm_id=ANY($1) AND a.state!='resolved' AND ($2='' OR c.farm_id=$2))) FROM herd WHERE farm_id=ANY($1) AND ($2='' OR farm_id=$2)`, who(r).Farms, r.URL.Query().Get("farm_id"))
}
