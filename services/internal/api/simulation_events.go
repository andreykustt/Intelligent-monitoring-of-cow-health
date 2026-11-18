package api

import (
	"context"
	"cowhealth/internal/platform"
	"encoding/json"
	"time"
)

func (s *Server) ConsumeSimulationStates(ctx context.Context) {
	for ctx.Err() == nil {
		func() {
			conn, ch, e := platform.Connect()
			if e != nil {
				return
			}
			defer conn.Close()
			q, e := ch.QueueDeclare("simulation.states.api", true, false, false, false, nil)
			if e != nil {
				return
			}
			if ch.QueueBind(q.Name, "simulation.status_changed.v1", platform.Exchange, false, nil) != nil {
				return
			}
			ds, e := ch.Consume(q.Name, "", false, false, false, false, nil)
			if e != nil {
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
					var env struct {
						Data struct {
							ID       string          `json:"id"`
							State    string          `json:"state"`
							Version  int             `json:"version"`
							Progress json.RawMessage `json:"progress"`
						} `json:"data"`
					}
					if json.Unmarshal(d.Body, &env) != nil {
						_ = d.Reject(false)
						continue
					}
					v := env.Data
					if v.State != "running" && v.State != "paused" && v.State != "stopped" && v.State != "completed" && v.State != "failed" {
						_ = d.Reject(false)
						continue
					}
					_, e = s.DB.Exec(ctx, "UPDATE simulations SET state=$2,progress=$3,updated_at=now() WHERE id::text=$1 AND version=$4", v.ID, v.State, v.Progress, v.Version)
					if e != nil {
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
