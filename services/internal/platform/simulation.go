package platform

import (
	"fmt"
	"time"
)

var Metrics = []string{"body_temperature_c", "respiratory_rate_bpm", "heart_rate_bpm", "activity_index", "steps_per_min", "rumen_ph", "rumination_min_1h", "feed_intake_kg_1h", "water_intake_l_1h"}

type ReplayReading struct {
	CowID    string    `json:"cow_id"`
	Metric   string    `json:"metric_type"`
	Value    float64   `json:"value"`
	Recorded time.Time `json:"recorded_at"`
}
type SimConfig struct {
	CowCount  int             `json:"cow_count"`
	Metrics   []string        `json:"metrics_enabled"`
	Intervals map[string]int  `json:"interval_ms_by_metric"`
	Duration  int             `json:"duration_seconds"`
	Mode      string          `json:"mode"`
	Speed     float64         `json:"playback_speed"`
	Scenario  string          `json:"scenario"`
	Seed      int64           `json:"seed"`
	Replay    []ReplayReading `json:"replay"`
}

func (c SimConfig) Validate() error {
	if c.CowCount < 1 || c.CowCount > 1000 || c.Duration < 1 || c.Duration > 86400 {
		return fmt.Errorf("от 1 до 1000 животных, длительность 1–86400 секунд")
	}
	if c.Mode != "synthetic" && c.Mode != "replay" {
		return fmt.Errorf("неизвестный режим")
	}
	if c.Scenario != "normal" && c.Scenario != "anomaly" && c.Scenario != "missing" {
		return fmt.Errorf("неизвестный сценарий")
	}
	if len(c.Metrics) == 0 {
		return fmt.Errorf("выберите метрики")
	}
	seen := map[string]bool{}
	for _, m := range c.Metrics {
		valid := false
		for _, v := range Metrics {
			if m == v {
				valid = true
			}
		}
		if !valid || seen[m] || c.Intervals[m] < 1000 || c.Intervals[m] > 3600000 {
			return fmt.Errorf("метрики уникальны, интервал 1000–3600000 мс")
		}
		seen[m] = true
	}
	if c.Mode == "replay" && (len(c.Replay) == 0 || len(c.Replay) > 1000 || c.Speed <= 0 || c.Speed > 100) {
		return fmt.Errorf("replay: 1–1000 записей, скорость (0,100]")
	}
	return nil
}
