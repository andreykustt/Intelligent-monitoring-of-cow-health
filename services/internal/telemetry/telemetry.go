package telemetry

import (
	"context"
	"cowhealth/internal/platform"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	"net/http"
	"regexp"
	"time"
)

type Event struct {
	Version    int       `json:"schema_version"`
	ID         string    `json:"event_id"`
	Simulation string    `json:"simulation_id"`
	Cow        string    `json:"cow_id"`
	Sensor     string    `json:"sensor_id"`
	Recorded   time.Time `json:"recorded_at"`
	Received   time.Time `json:"received_at"`
	Metric     string    `json:"metric_type"`
	Value      float64   `json:"value"`
	Source     string    `json:"source"`
}

var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var ranges = map[string][2]float64{"body_temperature_c": {20, 50}, "respiratory_rate_bpm": {0, 200}, "heart_rate_bpm": {0, 300}, "activity_index": {0, 1}, "steps_per_min": {0, 300}, "rumen_ph": {0, 14}, "rumination_min_1h": {0, 60}, "feed_intake_kg_1h": {0, 50}, "water_intake_l_1h": {0, 100}}

func (e Event) Validate() error {
	b, ok := ranges[e.Metric]
	if e.Version != 1 || !uuid.MatchString(e.ID) || !uuid.MatchString(e.Simulation) || e.Cow == "" || e.Sensor != e.Cow+":"+e.Metric || !ok || math.IsNaN(e.Value) || math.IsInf(e.Value, 0) || e.Value < b[0] || e.Value > b[1] || e.Recorded.IsZero() || e.Recorded.After(time.Now().Add(time.Minute)) || (e.Source != "synthetic" && e.Source != "replay") {
		return fmt.Errorf("invalid telemetry envelope")
	}
	return nil
}
func Ingest(ctx context.Context) {
	p := &platform.Publisher{}
	defer p.Close()
	token := platform.Required("INTERNAL_TOKEN")
	m := http.NewServeMux()
	m.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) { platform.JSON(w, 200, map[string]bool{"live": true}) })
	m.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		c, _, e := platform.Connect()
		if e != nil {
			platform.Error(w, 503, "NOT_READY", "Broker unavailable")
			return
		}
		c.Close()
		platform.JSON(w, 200, map[string]bool{"ready": true})
	})
	m.HandleFunc("POST /internal/v1/telemetry", func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			platform.Error(w, 401, "UNAUTHORIZED", "Service authentication required")
			return
		}
		var e Event
		if !platform.Decode(w, r, &e) {
			return
		}
		if err := e.Validate(); err != nil {
			platform.Error(w, 422, "VALIDATION_ERROR", err.Error())
			return
		}
		e.Received = time.Now().UTC()
		e.Recorded = e.Recorded.UTC()
		b, _ := json.Marshal(e)
		c, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if p.Publish(c, "telemetry.received.v1", b) != nil {
			platform.Error(w, 503, "BROKER_UNAVAILABLE", "Повторите с прежним event_id")
			return
		}
		platform.JSON(w, 202, map[string]string{"event_id": e.ID, "accepted_by": "broker"})
	})
	platform.Serve(ctx, m)
}

var ErrUnknown = errors.New("unregistered cow, sensor or simulation")

func Save(ctx context.Context, db *pgxpool.Pool, e Event) error {
	if err := e.Validate(); err != nil {
		return err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var valid bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sensors se JOIN cows c ON c.id=se.cow_id JOIN simulations sim ON sim.farm_id=c.farm_id WHERE se.id=$1 AND se.cow_id=$2 AND se.metric_type=$3 AND sim.id=$4)`, e.Sensor, e.Cow, e.Metric, e.Simulation).Scan(&valid); err != nil {
		return err
	}
	if !valid {
		return ErrUnknown
	}
	tag, err := tx.Exec(ctx, `INSERT INTO sensor_measurements(event_id,simulation_id,cow_id,sensor_id,metric_type,value,recorded_at,received_at,source) SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9 WHERE EXISTS(SELECT 1 FROM sensors se JOIN cows c ON c.id=se.cow_id JOIN simulations sim ON sim.farm_id=c.farm_id WHERE se.id=$4 AND se.cow_id=$3 AND se.metric_type=$5 AND sim.id=$2) ON CONFLICT(event_id) DO NOTHING`, e.ID, e.Simulation, e.Cow, e.Sensor, e.Metric, e.Value, e.Recorded, e.Received, e.Source)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, `INSERT INTO cow_latest_state SELECT * FROM sensor_measurements WHERE event_id=$1 ON CONFLICT(cow_id,metric_type) DO UPDATE SET event_id=EXCLUDED.event_id,simulation_id=EXCLUDED.simulation_id,sensor_id=EXCLUDED.sensor_id,value=EXCLUDED.value,recorded_at=EXCLUDED.recorded_at,received_at=EXCLUDED.received_at,source=EXCLUDED.source WHERE cow_latest_state.recorded_at<EXCLUDED.recorded_at`, e.ID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO outbox_events(id,event_type,payload,owner) SELECT $1,'telemetry.saved.v1',jsonb_build_object('cow_id',c.id,'farm_id',c.farm_id,'measurement',to_jsonb(m)),'data-worker' FROM sensor_measurements m JOIN cows c ON c.id=m.cow_id WHERE m.event_id=$2`, platform.ID(), e.ID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func Worker(ctx context.Context, db *pgxpool.Pool) {
	go platform.Outbox(ctx, db, "data-worker")
	for ctx.Err() == nil {
		func() {
			conn, ch, err := platform.Connect()
			if err != nil {
				return
			}
			defer conn.Close()
			_ = ch.Qos(100, 0, false)
			ds, err := ch.Consume("telemetry.received.v1", "", false, false, false, false, nil)
			if err != nil {
				return
			}
			for {
				select {
				case <-ctx.Done():
					return
				case d, ok := <-ds:
					if !ok {
						return
					}
					var e Event
					if json.Unmarshal(d.Body, &e) != nil || e.Validate() != nil {
						_ = d.Reject(false)
						continue
					}
					if err = Save(ctx, db, e); errors.Is(err, ErrUnknown) {
						_ = d.Reject(false)
					} else if err != nil {
						_ = d.Nack(false, true)
						platform.Pause(ctx, time.Second)
					} else {
						_ = d.Ack(false)
					}
				}
			}
		}()
		platform.Pause(ctx, time.Second)
	}
}
