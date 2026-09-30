package pattern

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Churn is the same record written to main again and again (design §2b).
const Churn = "churn"

// churnWork is one (machine, lineage-hash, path).
func churnWork(machine, lineage, path string) string { return machine + "-" + lineage + ":" + path }

// DetectChurn observes every (machine, lineage-hash, path) main was written
// by: a Finding when commits or more of them fall inside one window-min
// window by committer time (a burst observed late still counts); a Clear
// only when this cycle's fetch was fresh and the last window is below the
// threshold; Unknown when main could not be read or the fetch is stale.
func DetectChurn(signals Signals, now time.Time, thresholds Thresholds) []Observation {
	trunk := signals.Trunk
	if !trunk.Read {
		return nil
	}
	limit := thresholds.int("commits")
	window := time.Duration(thresholds.int("window-min")) * time.Minute
	groups := map[string][]TrunkCommit{}
	for _, commit := range trunk.Commits {
		for _, path := range commit.Paths {
			work := churnWork(commit.Machine, commit.Lineage, path)
			groups[work] = append(groups[work], commit)
		}
	}
	works := map[string]bool{}
	for work := range groups {
		works[work] = true
	}
	for _, work := range trunk.Watched {
		works[work] = true
	}
	var ordered []string
	for work := range works {
		ordered = append(ordered, work)
	}
	sort.Strings(ordered)
	var observations []Observation
	for _, work := range ordered {
		observation := Observation{Work: work}
		commits := groups[work]
		dense, crossing := denseWindows(commits, limit, window, now)
		switch {
		case trunk.Unreadable:
			observation.Kind = Unknown
		case limit > 0 && window > 0 && len(dense) > 0:
			observation.Kind = Finding
			observation.Since = crossing.at
			for _, commit := range dense {
				observation.Evidence = append(observation.Evidence, Evidence{Record: commit.SHA, At: stamp(commit.At),
					Fact: fmt.Sprintf("%s-%s wrote %s", commit.Machine, commit.Lineage, work[strings.Index(work, ":")+1:])})
			}
			observation.Message = churnMessage(trunk, commits[0], limit, crossing)
		case trunk.Fresh:
			observation.Kind = Clear
		default:
			observation.Kind = Unknown
		}
		observations = append(observations, observation)
	}
	return observations
}

type churnCrossing struct {
	at   time.Time
	span time.Duration
}

// denseWindows returns the commits inside every window of at least limit
// commits that holds a commit new this cycle or ends in the last window,
// and where the first such window crossed the threshold.
func denseWindows(commits []TrunkCommit, limit int, window time.Duration, now time.Time) ([]TrunkCommit, churnCrossing) {
	if limit < 1 || window <= 0 {
		return nil, churnCrossing{}
	}
	in := map[int]bool{}
	var crossing churnCrossing
	for start := range commits {
		end := start
		for end+1 < len(commits) && commits[end+1].At.Sub(commits[start].At) < window {
			end++
		}
		if end-start+1 < limit {
			continue
		}
		current := false
		for index := start; index <= end; index++ {
			current = current || commits[index].New || now.Sub(commits[index].At) < window
		}
		if !current {
			continue
		}
		if crossing.at.IsZero() {
			crossed := commits[start+limit-1].At
			crossing = churnCrossing{at: crossed, span: crossed.Sub(commits[start].At)}
		}
		for index := start; index <= end; index++ {
			in[index] = true
		}
	}
	var dense []TrunkCommit
	for index, commit := range commits {
		if in[index] {
			dense = append(dense, commit)
		}
	}
	return dense, crossing
}

// churnMessage names the writer by its lineage hash alone (never a name
// prefix) and states both times: when the burst crossed the threshold and
// when the steward saw it.
func churnMessage(trunk TrunkSignal, commit TrunkCommit, limit int, crossing churnCrossing) string {
	who := "Machine " + commit.Machine
	if trunk.LaneHashes[commit.Lineage] {
		who = "The landing lane"
	}
	minutes := int(math.Ceil(crossing.span.Minutes()))
	if minutes < 1 {
		minutes = 1
	}
	return fmt.Sprintf("%s wrote the same record to main %d times in %d minute%s (by %s; seen %s).",
		who, limit, minutes, plural(minutes), crossing.at.Local().Format("15:04"), trunk.SeenAt.Local().Format("15:04"))
}

// watch keeps the works a churn finding named until they have read clear
// for clearTicks cycles, so their episodes can clear once main is quiet.
func (s *trunkState) watch(observations []Observation, clearTicks int) {
	if s.Watched == nil {
		s.Watched = map[string]int{}
	}
	for _, observation := range observations {
		switch observation.Kind {
		case Finding:
			s.Watched[observation.Work] = 0
		case Clear:
			if count, watched := s.Watched[observation.Work]; watched {
				if count+1 >= clearTicks {
					delete(s.Watched, observation.Work)
				} else {
					s.Watched[observation.Work] = count + 1
				}
			}
		}
	}
}
