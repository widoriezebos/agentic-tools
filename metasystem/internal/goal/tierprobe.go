package goal

import (
	"sort"
	"strings"
)

// TierProbe is the backlog's tier spread: how many open goals carry each
// recorded tier, how many each derived tier, and which goals record a tier
// above their derivation, so a person can lower them (a lowering after
// claim is the human's act alone). Goal tier-from-severity-and-novelty
// names it as the proof that exposure no longer lifts the backlog.
type TierProbe struct {
	Open      int             `json:"open"`
	Recorded  map[uint8]int   `json:"recorded"`
	Derived   map[uint8]int   `json:"derived"`
	Lowerable []TierLowerable `json:"lowerable"`
}

// TierLowerable is one open goal whose recorded tier exceeds the tier its
// risk answers derive.
type TierLowerable struct {
	ID       string `json:"id"`
	State    string `json:"state"`
	Recorded uint8  `json:"recorded"`
	Derived  uint8  `json:"derived"`
	// Override reports a deliberate tier above the derivation on the
	// goal's own history (a TierOverride line with its why), as opposed to a
	// tier the earlier formula set.
	Override bool `json:"override"`
}

// Tier3Share reports the percentage of open goals at tier 3 by recorded
// and by derived tier, rounded to whole percent; zero when nothing is open.
func (p TierProbe) Tier3Share() (recorded, derived int) {
	if p.Open == 0 {
		return 0, 0
	}
	return (p.Recorded[3]*100 + p.Open/2) / p.Open, (p.Derived[3]*100 + p.Open/2) / p.Open
}

// ProbeTiers reads every live goal that carries a risk record.
func ProbeTiers(t *TreeGoals) TierProbe {
	probe := TierProbe{Recorded: map[uint8]int{}, Derived: map[uint8]int{}, Lowerable: []TierLowerable{}}
	if t == nil {
		return probe
	}
	for _, id := range sortedGoalIds(t.Live) {
		f := t.Live[id]
		if f.Risk == nil {
			continue
		}
		derived := f.Risk.DerivedTier()
		probe.Open++
		probe.Recorded[f.Tier]++
		probe.Derived[derived]++
		if f.Tier > derived {
			override := false
			for _, h := range f.History {
				if strings.HasPrefix(h.Reason, "TierOverride:") {
					override = true
				}
			}
			probe.Lowerable = append(probe.Lowerable, TierLowerable{ID: id, State: f.State, Recorded: f.Tier, Derived: derived, Override: override})
		}
	}
	sort.Slice(probe.Lowerable, func(i, j int) bool { return probe.Lowerable[i].ID < probe.Lowerable[j].ID })
	return probe
}
