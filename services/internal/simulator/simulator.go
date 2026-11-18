package simulator

import (
	"bytes"
	"context"
	"cowhealth/internal/platform"
	"cowhealth/internal/telemetry"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	"math/rand"
	"net/http"
	"sort"
	"time"
)

type run struct {
	ID, Farm, State string
	Version         int
	Config          platform.SimConfig
	Cows            []string
	Started         time.Time
	Last            map[string]time.Time
	RNG             *rand.Rand
	Sent, Retries   int
	ReplayIndex     int
	ReplayStart     time.Time
}

func Run(ctx context.Context, db *pgxpool.Pool) {
	p := &platform.Publisher{}
	defer p.Close()
	client := &http.Client{Timeout: 7 * time.Second}
	runs := map[string]*run{}
	for ctx.Err() == nil {
		rows, e := db.Query(ctx, `SELECT id::text,farm_id,config,version,created_at,state FROM simulations WHERE state IN ('starting','running','paused') ORDER BY created_at`)
		if e != nil {
			platform.Pause(ctx, time.Second)
			continue
		}
		for rows.Next() {
			var id, farm, state string
			var raw []byte
			var v int
			var created time.Time
			if rows.Scan(&id, &farm, &raw, &v, &created, &state) != nil {
				continue
			}
			a := runs[id]
			if a == nil {
				a = &run{ID: id, Farm: farm, Version: 0, Started: time.Now(), State: "starting", Last: map[string]time.Time{}}
				runs[id] = a
				if state != "starting" {
					a.State = "failed"
				}
			}
			if a.Version != v {
				_ = json.Unmarshal(raw, &a.Config)
				if a.RNG == nil {
					a.RNG = rand.New(rand.NewSource(a.Config.Seed))
				}
				a.Version = v
				var action string
				if db.QueryRow(ctx, "SELECT action FROM simulation_commands WHERE simulation_id=$1 AND version=$2", id, v).Scan(&action) != nil {
					continue
				}
				if a.State != "failed" {
					switch action {
					case "start", "resume":
						a.State = "running"
					case "pause":
						a.State = "paused"
					case "stop":
						a.State = "stopped"
					}
				}
				if a.Cows == nil {
					cr, e := db.Query(ctx, "SELECT id FROM cows WHERE farm_id=$1 ORDER BY id LIMIT $2", farm, a.Config.CowCount)
					if e == nil {
						for cr.Next() {
							var cow string
							if cr.Scan(&cow) == nil {
								a.Cows = append(a.Cows, cow)
							}
						}
						cr.Close()
					}
				}
				sort.SliceStable(a.Config.Replay, func(i, j int) bool { return a.Config.Replay[i].Recorded.Before(a.Config.Replay[j].Recorded) })
			}
		}
		rows.Close()
		for _, a := range runs {
			if a.State == "running" {
				if time.Since(a.Started) > time.Duration(a.Config.Duration)*time.Second {
					a.State = "completed"
				} else if a.Config.Mode == "replay" {
					replay(ctx, a, client)
				} else {
					generate(ctx, a, client)
				}
			}
			if status(ctx, a, p) && (a.State == "completed" || a.State == "stopped" || a.State == "failed") {
				delete(runs, a.ID)
			}
		}
		platform.Pause(ctx, time.Second)
	}
}
func status(ctx context.Context, a *run, p *platform.Publisher) bool {
	b, _ := json.Marshal(map[string]any{"event_id": platform.ID(), "type": "simulation.status_changed.v1", "occurred_at": time.Now().UTC(), "data": map[string]any{"id": a.ID, "farm_id": a.Farm, "version": a.Version, "state": a.State, "progress": map[string]any{"sent": a.Sent, "retries": a.Retries, "elapsed_seconds": int(time.Since(a.Started).Seconds())}}})
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return p.Publish(c, "simulation.status_changed.v1", b) == nil
}
func generate(ctx context.Context, a *run, client *http.Client) {
	base := map[string]float64{"body_temperature_c": 38.6, "heart_rate_bpm": 65, "respiratory_rate_bpm": 25, "activity_index": 0.5, "steps_per_min": 12, "rumen_ph": 6.4, "rumination_min_1h": 25, "feed_intake_kg_1h": 2, "water_intake_l_1h": 4}
	for _, m := range a.Config.Metrics {
		if time.Since(a.Last[m]) < time.Duration(a.Config.Intervals[m])*time.Millisecond {
			continue
		}
		a.Last[m] = time.Now()
		for i, cow := range a.Cows {
			if ctx.Err() != nil {
				return
			}
			if a.Config.Scenario == "missing" && i%5 == 0 {
				continue
			}
			v := base[m] * (1 + a.RNG.NormFloat64()*.025)
			if a.Config.Scenario == "anomaly" && i%10 == 0 {
				switch m {
				case "body_temperature_c":
					v += 2
				case "heart_rate_bpm":
					v += 40
				case "activity_index":
					v = .1
				}
			}
			v = math.Max(0, v)
			e := telemetry.Event{Version: 1, ID: platform.ID(), Simulation: a.ID, Cow: cow, Sensor: cow + ":" + m, Recorded: time.Now().UTC(), Metric: m, Value: v, Source: "synthetic"}
			if !send(ctx, a, e, client) {
				a.State = "failed"
				return
			}
		}
	}
}
func replay(ctx context.Context, a *run, client *http.Client) {
	if a.ReplayStart.IsZero() {
		a.ReplayStart = time.Now()
	}
	for a.ReplayIndex < len(a.Config.Replay) {
		r := a.Config.Replay[a.ReplayIndex]
		if time.Since(a.ReplayStart).Seconds() < r.Recorded.Sub(a.Config.Replay[0].Recorded).Seconds()/a.Config.Speed {
			return
		}
		valid := false
		for _, c := range a.Cows {
			if c == r.CowID {
				valid = true
			}
		}
		if !valid {
			a.State = "failed"
			return
		}
		e := telemetry.Event{Version: 1, ID: platform.ID(), Simulation: a.ID, Cow: r.CowID, Sensor: r.CowID + ":" + r.Metric, Recorded: r.Recorded, Metric: r.Metric, Value: r.Value, Source: "replay"}
		if !send(ctx, a, e, client) {
			a.State = "failed"
			return
		}
		a.ReplayIndex++
	}
	a.State = "completed"
}
func send(ctx context.Context, a *run, e telemetry.Event, client *http.Client) bool {
	b, _ := json.Marshal(e)
	for attempt := 0; attempt < 5; attempt++ {
		r, err := http.NewRequestWithContext(ctx, "POST", platform.Env("INGEST_URL", "http://ingest-service:8080")+"/internal/v1/telemetry", bytes.NewReader(b))
		if err != nil {
			return false
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+platform.Required("INTERNAL_TOKEN"))
		resp, err := client.Do(r)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 202 {
				a.Sent++
				return true
			}
			if resp.StatusCode < 500 {
				return false
			}
		}
		a.Retries++
		platform.Pause(ctx, time.Duration(1<<attempt)*time.Second)
	}
	return false
}
