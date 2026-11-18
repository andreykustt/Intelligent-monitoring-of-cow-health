package analyzer

import (
	"bytes"
	"context"
	"cowhealth/internal/platform"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"time"
)

type result struct {
	Status string   `json:"status"`
	Score  *float64 `json:"anomaly_score"`
	Model  string   `json:"model_version"`
}

func Run(ctx context.Context, db *pgxpool.Pool) {
	go platform.Outbox(ctx, db, "ml-analyzer")
	client := &http.Client{Timeout: 5 * time.Second}
	for ctx.Err() == nil {
		_, _ = db.Exec(ctx, `INSERT INTO analysis_jobs(cow_id,window_end) SELECT cow_id,date_bin(interval '5 minutes',recorded_at,'2000-01-01')+interval '5 minutes' FROM sensor_measurements WHERE recorded_at>now()-interval '90 days' AND recorded_at<date_bin(interval '5 minutes',now(),'2000-01-01') GROUP BY 1,2 HAVING max(received_at)<now()-interval '10 seconds' ON CONFLICT DO NOTHING`)
		rows, e := db.Query(ctx, "SELECT cow_id,window_end FROM analysis_jobs WHERE completed_at IS NULL AND next_attempt_at<=now() ORDER BY window_end LIMIT 200")
		if e != nil {
			platform.Pause(ctx, time.Second)
			continue
		}
		type job struct {
			Cow string
			End time.Time
		}
		jobs := []job{}
		for rows.Next() {
			var j job
			if rows.Scan(&j.Cow, &j.End) == nil {
				jobs = append(jobs, j)
			}
		}
		rows.Close()
		for _, j := range jobs {
			e = analyze(ctx, db, client, j.Cow, j.End)
			if e != nil {
				_, _ = db.Exec(ctx, "UPDATE analysis_jobs SET attempts=attempts+1,last_error=$3,next_attempt_at=now()+interval '30 seconds' WHERE cow_id=$1 AND window_end=$2", j.Cow, j.End, e.Error())
			}
		}
		platform.Pause(ctx, 5*time.Second)
	}
}
func analyze(ctx context.Context, db *pgxpool.Pool, client *http.Client, cow string, end time.Time) error {
	features := []float64{}
	complete := true
	for _, metric := range []string{"body_temperature_c", "heart_rate_bpm", "activity_index"} {
		var count int
		var mean, std, slope float64
		e := db.QueryRow(ctx, `WITH minutes AS(SELECT date_trunc('minute',recorded_at) t,avg(value) v FROM sensor_measurements WHERE cow_id=$1 AND metric_type=$2 AND recorded_at>=$3 AND recorded_at<$4 GROUP BY 1) SELECT count(*),COALESCE(avg(v),0),COALESCE(stddev_pop(v),0),COALESCE(regr_slope(v,extract(epoch FROM t)/60),0) FROM minutes`, cow, metric, end.Add(-time.Hour), end).Scan(&count, &mean, &std, &slope)
		if e != nil {
			return e
		}
		if count < 48 {
			complete = false
		}
		features = append(features, mean, std, slope)
	}
	if !complete {
		features = nil
	}
	b, _ := json.Marshal(map[string]any{"cow_id": cow, "window_end": end, "window_start": end.Add(-time.Hour), "features": features, "feature_schema_version": 1})
	req, e := http.NewRequestWithContext(ctx, "POST", platform.Env("ML_URL", "http://ml-service:8000")+"/predict", bytes.NewReader(b))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	resp, e := client.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("inference HTTP %d", resp.StatusCode)
	}
	var p result
	if e = json.NewDecoder(resp.Body).Decode(&p); e != nil {
		return e
	}
	if p.Model == "" || (p.Status != "anomaly" && p.Status != "no_anomaly" && p.Status != "insufficient_data") {
		return fmt.Errorf("invalid model response")
	}
	tx, e := db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	pid := platform.ID()
	var inserted string
	e = tx.QueryRow(ctx, `INSERT INTO ml_predictions VALUES($1,$2,$3,$4,$5,$6,$7,1,now()) ON CONFLICT(cow_id,window_end,model_version,schema_version) DO UPDATE SET status=ml_predictions.status RETURNING id::text`, pid, cow, end.Add(-time.Hour), end, p.Status, p.Score, p.Model).Scan(&inserted)
	if e != nil {
		return e
	}
	if inserted == pid {
		_, e = tx.Exec(ctx, `INSERT INTO outbox_events(id,event_type,payload,owner) SELECT $1,'prediction.created.v1',jsonb_build_object('cow_id',id,'farm_id',farm_id,'status',$3::text),'ml-analyzer' FROM cows WHERE id=$2`, platform.ID(), cow, p.Status)
		if e != nil {
			return e
		}
		if p.Status == "anomaly" {
			aid := platform.ID()
			var alertID string
			e = tx.QueryRow(ctx, `INSERT INTO alerts(id,cow_id,prediction_id,last_seen) VALUES($1,$2,$3,$4) ON CONFLICT(cow_id,condition) WHERE state!='resolved' DO UPDATE SET last_seen=GREATEST(alerts.last_seen,EXCLUDED.last_seen),prediction_id=CASE WHEN EXCLUDED.last_seen>=alerts.last_seen THEN EXCLUDED.prediction_id ELSE alerts.prediction_id END RETURNING id::text`, aid, cow, pid, end).Scan(&alertID)
			if e != nil {
				return e
			}
			key := "alert.updated.v1"
			if alertID == aid {
				key = "alert.created.v1"
			}
			_, e = tx.Exec(ctx, `INSERT INTO outbox_events(id,event_type,payload,owner) SELECT $1,$2,jsonb_build_object('id',$3::text,'cow_id',id,'farm_id',farm_id),'ml-analyzer' FROM cows WHERE id=$4`, platform.ID(), key, alertID, cow)
			if e != nil {
				return e
			}
		}
	}
	_, e = tx.Exec(ctx, "UPDATE analysis_jobs SET completed_at=now(),last_error=NULL WHERE cow_id=$1 AND window_end=$2", cow, end)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
