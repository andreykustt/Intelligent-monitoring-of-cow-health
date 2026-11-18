package platform

import "testing"

func TestSimBounds(t *testing.T) {
	c := SimConfig{CowCount: 200, Metrics: []string{"activity_index"}, Intervals: map[string]int{"activity_index": 5000}, Duration: 3600, Mode: "synthetic", Scenario: "normal"}
	if c.Validate() != nil {
		t.Fatal("valid config rejected")
	}
	c.Metrics = append(c.Metrics, "activity_index")
	if c.Validate() == nil {
		t.Fatal("duplicate metrics accepted")
	}
	c.Metrics = c.Metrics[:1]
	c.Intervals["activity_index"] = 0
	if c.Validate() == nil {
		t.Fatal("zero interval accepted")
	}
}
