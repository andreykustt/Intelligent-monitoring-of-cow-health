package api

import (
	"cowhealth/internal/platform"
	"encoding/json"
	"github.com/gorilla/websocket"
	"net/http"
	"time"
)

// Each browser receives a bounded best-effort broadcast queue; REST repairs delivery gaps.
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	u := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == s.Origin }}
	farm := r.URL.Query().Get("farm_id")
	if farm != "" && !farmAllowed(r, farm) {
		platform.Error(w, 403, "FORBIDDEN", "Ферма недоступна")
		return
	}
	broker, ch, e := platform.Connect()
	if e != nil {
		platform.Error(w, 503, "NOT_READY", "Поток временно недоступен")
		return
	}
	defer broker.Close()
	q, e := ch.QueueDeclare("", false, true, true, false, nil)
	if e != nil {
		s.fail(w, e)
		return
	}
	for _, key := range []string{"telemetry.saved.v1", "prediction.created.v1", "alert.created.v1", "alert.updated.v1", "simulation.status_changed.v1", "manual_measurement.created.v1"} {
		if e = ch.QueueBind(q.Name, key, platform.Exchange, false, nil); e != nil {
			s.fail(w, e)
			return
		}
	}
	_ = ch.Qos(100, 0, false)
	events, e := ch.Consume(q.Name, "", false, true, false, false, nil)
	if e != nil {
		s.fail(w, e)
		return
	}
	ws, e := u.Upgrade(w, r, nil)
	if e != nil {
		return
	}
	defer ws.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ws.SetReadLimit(4096)
		_ = ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		ws.SetPongHandler(func(string) error { return ws.SetReadDeadline(time.Now().Add(60 * time.Second)) })
		for {
			if _, _, e := ws.ReadMessage(); e != nil {
				return
			}
		}
	}()
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	authTick := time.NewTicker(15 * time.Second)
	defer authTick.Stop()
	batch := []json.RawMessage{}
	for {
		select {
		case <-done:
			return
		case <-r.Context().Done():
			return
		case d, ok := <-events:
			if !ok {
				return
			}
			var env struct {
				Type string `json:"type"`
				Data struct {
					Farm string `json:"farm_id"`
				} `json:"data"`
			}
			if json.Unmarshal(d.Body, &env) == nil && farmAllowed(r, env.Data.Farm) && (farm == "" || farm == env.Data.Farm) {
				var event map[string]any
				_ = json.Unmarshal(d.Body, &event)
				if env.Type == "telemetry.saved.v1" {
					event["type"] = "telemetry.update.v1"
				}
				b, _ := json.Marshal(event)
				batch = append(batch, b)
			}
			_ = d.Ack(false)
			if len(batch) > 1000 {
				return
			}
		case <-tick.C:
			if len(batch) > 0 {
				_ = ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if ws.WriteJSON(batch) != nil {
					return
				}
				batch = batch[:0]
			}
		case <-authTick.C:
			current, e := s.session(r)
			old := who(r)
			if e != nil || current.Role != old.Role || !sameFarms(current.Farms, old.Farms) {
				return
			}
			if ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)) != nil {
				return
			}
		}
	}
}
func sameFarms(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
