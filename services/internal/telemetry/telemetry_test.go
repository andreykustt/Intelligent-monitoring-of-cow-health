package telemetry

import (
	"cowhealth/internal/platform"
	"testing"
	"time"
)

func TestEnvelope(t *testing.T) {
	valid := Event{Version: 1, ID: platform.ID(), Simulation: platform.ID(), Cow: "COW-001", Sensor: "COW-001:activity_index", Metric: "activity_index", Value: .5, Recorded: time.Now().UTC(), Source: "synthetic"}
	if valid.Validate() != nil {
		t.Fatal("valid rejected")
	}
	tests := []struct {
		name   string
		change func(*Event)
	}{{"source", func(e *Event) { e.Source = "hardware" }}, {"unknown metric", func(e *Event) { e.Metric = "diagnosis" }}, {"activity bounds", func(e *Event) { e.Value = 1.01 }}, {"future", func(e *Event) { e.Recorded = time.Now().Add(time.Hour) }}, {"sensor scope", func(e *Event) { e.Sensor = "COW-002:activity_index" }}, {"missing id", func(e *Event) { e.ID = "" }}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := valid
			test.change(&e)
			if e.Validate() == nil {
				t.Fatal("invalid accepted")
			}
		})
	}
}
