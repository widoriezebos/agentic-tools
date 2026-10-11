package rehearsal

import (
	"time"
)

// Breakdown partitions scenario time, including failed and cancelled steps.
type Breakdown struct {
	Setup     int64 `json:"setup"`
	Bootstrap int64 `json:"bootstrap"`
	HandIn    int64 `json:"hand-in"`
	Gate      int64 `json:"gate"`
	Proof     int64 `json:"proof"`
	Push      int64 `json:"push"`
	Total     int64 `json:"total"`
}

func (b Breakdown) stages() []struct {
	name string
	ms   int64
} {
	return []struct {
		name string
		ms   int64
	}{{"setup", b.Setup}, {"bootstrap", b.Bootstrap}, {"hand-in", b.HandIn}, {"gate", b.Gate}, {"proof", b.Proof}, {"push", b.Push}}
}

func (b Breakdown) Slowest() (string, time.Duration) {
	name, ms := "setup", b.Setup
	for _, stage := range b.stages() {
		if stage.ms > ms {
			name, ms = stage.name, stage.ms
		}
	}
	return name, time.Duration(ms) * time.Millisecond
}

func (s *Scratch) beginStage(name string, now time.Time) {
	ms := now.Sub(s.stageStart).Milliseconds()
	switch s.stage {
	case "setup":
		s.Breakdown.Setup += ms
	case "bootstrap":
		s.Breakdown.Bootstrap += ms
	case "hand-in":
		s.Breakdown.HandIn += ms
	case "gate":
		s.Breakdown.Gate += ms
	case "proof":
		s.Breakdown.Proof += ms
	case "push":
		s.Breakdown.Push += ms
	}
	s.stage, s.stageStart = name, now
}
