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
	// Batches, Helm and Pause are the readers; nil reads the retained
	// records.
	Batches BatchReader
	Helm    HelmReader
	Pause   PauseReader
}

func (p Pass) withDefaults() Pass {
	if p.Home == nil {
		p.Home = board.Home
	}
	if p.Batches == nil {
		p.Batches = ReadBatchStore
	}
	if p.Helm == nil {
		p.Helm = ReadHelm
	}
	if p.Pause == nil {
		p.Pause = ReadPause
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
	batches, batchErr := p.Batches(laneRoot)
	if batchErr != nil {
		signals.BatchesUnreadable = true
	} else {
		paused, pauseErr := p.Pause(home)
		landingHelm, landingReadable := p.Helm(laneRoot)
		markHolds(batches, paused, pauseErr, landingHelm, landingReadable, p.Helm)
		signals.Batches = batches
	}
	maxGap := MaxGap(repoRoot)
	_, err = steward.UpdatePatterns(repoRoot, steward.PatternCycle{
		Now: now, ClearTicks: setting(conf, ClearTicksKey, 2), Deliver: p.Deliver,
		Step: func(raw []byte) ([]byte, []steward.PatternRun, error) {
			current, err := decodeState(raw)
			if err != nil {
				// A torn state is started over: every interval since is
				// unobserved, which can only delay a report.
				current, _ = decodeState(nil)
			}
			if signals.BatchesUnreadable {
				current.unobserved(now)
			} else {
				current.account(signals.Batches, now, maxGap)
			}
			var runs []steward.PatternRun
			for _, pattern := range Registry() {
				if mode(conf, pattern.Name) != ModeReport {
					continue
				}
				runs = append(runs, steward.PatternRun{Pattern: pattern.Name,
					Observations: pattern.Detect(signals, now, thresholds(conf, pattern))})
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
