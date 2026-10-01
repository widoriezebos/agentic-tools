package pattern

import (
	"path/filepath"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// Pass is the lane steward's pattern pass: one call per steward cycle. The
// zero value is production.
type Pass struct {
	// Home is the host home the lane record lives under (board.Home).
	Home func() (string, error)
	// Deliver is the notifier; nil is the steward's own.
	Deliver func(repoRoot, message string) error
	// Git runs the trunk reader's git; nil is RunGit.
	Git GitRunner
	// LaneLineages are the lane's identities, the only lineages churn counts
	// (by their hash); nil is the stable identity and, until the lane
	// cutover, the old owner's.
	LaneLineages []string
}

func (p Pass) withDefaults() Pass {
	if p.Home == nil {
		p.Home = board.Home
	}
	if p.Git == nil {
		p.Git = RunGit
	}
	if p.LaneLineages == nil {
		p.LaneLineages = []string{LaneLineage}
	}
	return p
}

// Run runs every enabled pattern once, in the steward of the checkout the
// host's lane record names and nowhere else (D6). Without a lane on the host
// no pattern runs.
func (p Pass) Run(repoRoot string, now time.Time) error {
	p = p.withDefaults()
	home, err := p.Home()
	if err != nil {
		return nil
	}
	record, registered, err := lane.Read(home)
	if err != nil || !registered || !LaneSteward(record.Root, repoRoot) {
		return nil
	}
	laneRoot := record.Root
	conf := filepath.Join(repoRoot, "metasystem.conf")

	var signals Signals
	maxGap := MaxGap(repoRoot)
	clearTicks := setting(conf, ClearTicksKey, 2)
	// The fetch is the one network step: it runs before the alerts lock.
	churn := mode(conf, Churn) == ModeReport
	fetched := false
	if churn {
		fetched = fetchTrunk(p.Git, laneRoot) == nil
	}
	laneHashes := map[string]bool{}
	for _, lineage := range p.LaneLineages {
		laneHashes[hash8(lineage)] = true
	}
	_, err = steward.UpdatePatterns(repoRoot, steward.PatternCycle{
		Now: now, ClearTicks: clearTicks, Deliver: p.Deliver,
		Step: func(raw []byte) ([]byte, []steward.PatternRun, error) {
			current, err := decodeState(raw)
			if err != nil {
				// A torn state is started over: every interval since is
				// unobserved, which can only delay a report.
				current, _ = decodeState(nil)
			}
			cycleSignals := signals
			if churn {
				if current.Trunk == nil {
					current.Trunk = &trunkState{}
				}
				window := time.Duration(setting(conf, ThresholdKey(Churn, "window-min"), 10)) * time.Minute
				cycleSignals.Trunk = current.Trunk.read(p.Git, laneRoot, fetched, now, maxGap, window)
				cycleSignals.Trunk.LaneHashes = laneHashes
				for work := range current.Trunk.Watched {
					cycleSignals.Trunk.Watched = append(cycleSignals.Trunk.Watched, work)
				}
			}
			var runs []steward.PatternRun
			for _, pattern := range Registry() {
				if mode(conf, pattern.Name) != ModeReport {
					continue
				}
				observations := pattern.Detect(cycleSignals, now, thresholds(conf, pattern))
				if pattern.Name == Churn && current.Trunk != nil {
					current.Trunk.watch(observations, clearTicks)
				}
				runs = append(runs, steward.PatternRun{Pattern: pattern.Name, Observations: observations})
			}
			next, err := current.encode()
			return next, runs, err
		},
	})
	return err
}

// LaneSteward reports whether the steward serving repoRoot is the host
// lane's: repoRoot is the lane checkout or its installation.
func LaneSteward(laneRoot, repoRoot string) bool {
	here := realpath.Resolve(repoRoot)
	if realpath.Resolve(laneRoot) == here {
		return true
	}
	layout, err := stateroot.ResolveLayout(laneRoot)
	return err == nil && realpath.Resolve(layout.InstallationRoot) == here
}

// MaxGap is the longest gap between two cycles that still counts as observed
// time: steward.pattern.max-gap-sec, else the effective steward cadence plus
// half for cycle overhead (15 minutes at the default 600 s).
func MaxGap(repoRoot string) time.Duration {
	conf := filepath.Join(repoRoot, "metasystem.conf")
	if seconds := setting(conf, MaxGapKey, 0); seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(steward.TickSeconds(repoRoot)) * time.Second * 3 / 2
}

func lookup(conf, key, fallback string) string {
	value, _, err := config.Get(config.GetParams{Key: key, ConfPath: conf, Default: fallback, DefaultSet: true})
	if err != nil || value == "" {
		return fallback
	}
	return value
}

func setting(conf, key string, fallback int) int {
	value, err := strconv.Atoi(lookup(conf, key, strconv.Itoa(fallback)))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func mode(conf, name string) string {
	return lookup(conf, ModeKey(name), ModeReport)
}

func thresholds(conf string, pattern Pattern) Thresholds {
	values := Thresholds{}
	for name, fallback := range pattern.Defaults {
		values[name] = lookup(conf, ThresholdKey(pattern.Name, name), fallback)
	}
	return values
}
