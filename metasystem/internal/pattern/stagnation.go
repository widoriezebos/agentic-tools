package pattern

import (
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

// Stagnation is lane work that does not move (design §2a).
const Stagnation = "stagnation"

// highWaterRank ranks the states a batch moves forward through; diagnosing
// and the held states carry no rank.
var highWaterRank = map[string]int{
	batch.StateOpen: 0, batch.StateSealed: 1, batch.StateProving: 2, batch.StateLanding: 3, batch.StateLanded: 4,
}

// DetectStagnation observes every lane batch. An unfinished batch is a
// Finding when its counted active time reaches batch-hours (S1) or when
// repeat distinct attempts failed since its high-water state last rose (S2).
func DetectStagnation(signals Signals, _ time.Time, thresholds Thresholds) []Observation {
	limit := time.Duration(thresholds.float("batch-hours") * float64(time.Hour))
	repeat := thresholds.int("repeat")
	observations := make([]Observation, 0, len(signals.Batches))
	for _, b := range signals.Batches {
		observation := Observation{Work: b.ID, Since: b.Since}
		switch {
		case b.Unreadable:
			observation.Kind = Unknown
		case b.finished():
			observation.Kind = Clear
		case b.Held:
			observation.Kind = Held
		case b.HoldUnreadable:
			observation.Kind = Unknown
		default:
			observation.Kind = Clear
			aged := limit > 0 && b.Active >= limit
			attempts := attemptsSinceRise(b.Steps)
			retried := repeat > 0 && len(attempts) >= repeat
			if aged {
				observation.Evidence = append(observation.Evidence, Evidence{Record: b.Record, At: stamp(b.Since),
					Fact: fmt.Sprintf("counted active time reached batch-hours=%s", thresholds["batch-hours"])})
			}
			if retried {
				for _, step := range attempts {
					observation.Evidence = append(observation.Evidence, Evidence{Record: b.Record, At: stamp(step.At),
						Fact: fmt.Sprintf("%s %s→%s", step.Verb, step.From, step.To)})
				}
			}
			if aged || retried {
				observation.Kind = Finding
				observation.Message = stagnationMessage(b, aged, len(attempts))
			}
		}
		observations = append(observations, observation)
	}
	return observations
}

// attemptsSinceRise are the distinct failed attempts (by time and verb)
// after the entry that last raised the batch's high-water state: a refusal,
// a verifier that was unavailable, or a red. A retry that repeats
// sealed→proving leaves the high-water mark where it is, so it is never
// progress.
func attemptsSinceRise(steps []Step) []Step {
	high, rise := -1, -1
	for index, step := range steps {
		if rank, ranked := highWaterRank[step.To]; ranked && rank > high {
			high, rise = rank, index
		}
	}
	type key struct {
		at   time.Time
		verb string
	}
	seen := map[key]bool{}
	var attempts []Step
	for _, step := range steps[rise+1:] {
		failed := strings.HasSuffix(step.Verb, "-refused") || step.Verb == "verifier-unavailable" ||
			(step.Verb == "prove" && step.To == batch.StateDiagnosing)
		if !failed || seen[key{step.At, step.Verb}] {
			continue
		}
		seen[key{step.At, step.Verb}] = true
		attempts = append(attempts, step)
	}
	return attempts
}

func stagnationMessage(b BatchSignal, aged bool, attempts int) string {
	short := b.ID
	if len(short) > 5 {
		short = short[:5]
	}
	if aged {
		hours := int(b.Active / time.Hour)
		return fmt.Sprintf("The landing lane has not moved batch %s forward for %d hour%s.", short, hours, plural(hours))
	}
	return fmt.Sprintf("The landing lane tried batch %s %d times without moving it forward.", short, attempts)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func stamp(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339Nano)
}
