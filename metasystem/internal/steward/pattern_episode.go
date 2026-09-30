package steward

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// Behaviour patterns (design steward-acts-on-behaviour-patterns §1): a
// detector's observation of one piece of work becomes, through the lifecycle
// here, at most one durable AlertEpisode per (pattern, work) and one logical
// notification. v1 reports only: nothing here acts on the work.

// ObsKind is what a detector observed about one piece of work in one cycle.
type ObsKind string

const (
	// ObsFinding is a pattern that holds for the work.
	ObsFinding ObsKind = "finding"
	// ObsClear is a complete, fresh negative: every signal was readable and
	// the pattern does not hold. Only a Clear moves an episode toward cleared.
	ObsClear ObsKind = "clear"
	// ObsHeld is work a person holds (the lane paused, a seat at the helm).
	ObsHeld ObsKind = "held"
	// ObsUnknown is work a signal could not be read for.
	ObsUnknown ObsKind = "unknown"
)

// AlertEvidence is one typed fact behind an episode: the record (a path or a
// SHA), when it happened (UTC), and the typed fact itself.
type AlertEvidence struct {
	Record string `json:"record"`
	At     string `json:"at"`
	Fact   string `json:"fact"`
}

// PatternObservation is one detector observation (the design's Observation).
type PatternObservation struct {
	Kind     ObsKind
	Work     string
	Evidence []AlertEvidence
	// Since is when the observed condition began; it is part of the
	// episode's digest.
	Since time.Time
	// Message is a Finding's line 1: the plain situation a person reads when
	// the episode opens.
	Message string
}

// PatternRun is one pattern's observations of one cycle.
type PatternRun struct {
	Pattern      string
	Observations []PatternObservation
}

// PatternCycle is one cycle of the pattern pass. Step runs under the alerts
// lock: it receives the pattern state the last cycle kept (nil when none)
// and returns its successor with this cycle's runs.
type PatternCycle struct {
	Now        time.Time
	ClearTicks int
	// Deliver is the notifier; nil is the steward's own.
	Deliver func(repoRoot, message string) error
	Step    func(state []byte) ([]byte, []PatternRun, error)
}

// PatternReport is what one cycle changed.
type PatternReport struct {
	Opened []AlertEpisode
	Open   int
}

// AlertOpening is one request to open (or extend) an episode for a piece of
// work: OpenAlert is the one opening path, for patterns and any other opener
// (the lane's stop-loss opens with Owner "lane:stoploss").
type AlertOpening struct {
	Owner    string
	Work     string
	Since    time.Time
	Evidence []AlertEvidence
	// Message is line 1, the plain situation; the notification adds line 2.
	Message string
	Now     time.Time
	Deliver func(repoRoot, message string) error
}

const (
	patternOwnerPrefix = "pattern:"
	// maxAlertEvidence caps an episode's evidence; the first and the latest
	// items are kept.
	maxAlertEvidence = 20
	// StandingHeld and StandingUnreadable say why an open episode is neither
	// growing nor clearing.
	StandingHeld       = "held"
	StandingUnreadable = "unreadable"
)

// alertListCommand is line 2 of every opened episode's notification.
const alertListCommand = "metasystem alert list"

// PatternOwner is the episode owner of a pattern.
func PatternOwner(name string) string { return patternOwnerPrefix + name }

// IsPatternOwner reports whether an episode owner is a pattern.
func IsPatternOwner(owner string) bool { return strings.HasPrefix(owner, patternOwnerPrefix) }

// PatternStatePath is the pattern state the lane steward keeps between
// cycles, written under the alerts lock.
func PatternStatePath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "patterns.json")
}

func openingDigest(owner, work string, since time.Time) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{owner, work, since.UTC().Format(time.RFC3339Nano)}, "\n")))
	return hex.EncodeToString(sum[:])
}

// mergeEvidence appends the items not already held, keeping the first and
// the latest when the cap is reached; it reports whether anything changed.
func mergeEvidence(held, add []AlertEvidence) ([]AlertEvidence, bool) {
	changed := false
	for _, item := range add {
		seen := false
		for _, have := range held {
			if have == item {
				seen = true
				break
			}
		}
		if !seen {
			held = append(held, item)
			changed = true
		}
	}
	if len(held) > maxAlertEvidence {
		held = append(held[:1:1], held[len(held)-(maxAlertEvidence-1):]...)
	}
	return held, changed
}

// OpenAlert opens one episode for a piece of work, persists it, and then
// submits its one notification. An open episode for the same owner and work
// gains the evidence instead; a work a person cleared stays suppressed until
// its opener reports it clear (ClearSuppression). The bool reports a new
// episode.
func OpenAlert(repoRoot string, opening AlertOpening) (AlertEpisode, bool, error) {
	held, err := lockAlerts(repoRoot, lock.Exclusive)
	if err != nil {
		return AlertEpisode{}, false, err
	}
	defer unlockAlerts(held)
	episodes, err := loadAlertEpisodesUnlocked(repoRoot)
	if err != nil {
		return AlertEpisode{}, false, err
	}
	return openAlertLocked(repoRoot, &episodes, opening)
}

func openAlertLocked(repoRoot string, episodes *[]AlertEpisode, opening AlertOpening) (AlertEpisode, bool, error) {
	if opening.Owner == "" || opening.Work == "" || strings.TrimSpace(opening.Message) == "" {
		return AlertEpisode{}, false, errors.New("an alert needs an owner, a work identity and a message")
	}
	now := opening.Now
	if now.IsZero() {
		now = time.Now()
	}
	deliverTo := opening.Deliver
	if deliverTo == nil {
		deliverTo = deliver
	}
	for index := range *episodes {
		episode := &(*episodes)[index]
		if episode.Owner != opening.Owner || episode.ScopeID != opening.Work {
			continue
		}
		if !episode.Cleared {
			merged, changed := mergeEvidence(episode.Evidence, opening.Evidence)
			if changed {
				episode.Evidence = merged
				if err := saveAlertEpisode(repoRoot, *episode); err != nil {
					return AlertEpisode{}, false, err
				}
			}
			if episode.TransportResult != TransportSubmitted {
				if err := submitEpisode(repoRoot, episode, now, deliverTo); err != nil {
					return AlertEpisode{}, false, err
				}
			}
			return *episode, false, nil
		}
		if episode.Suppressed {
			return AlertEpisode{}, false, nil
		}
	}
	digest := openingDigest(opening.Owner, opening.Work, opening.Since)
	evidence, _ := mergeEvidence(nil, opening.Evidence)
	episode := AlertEpisode{
		Schema: 1, EpisodeID: nextEpisodeID(digest, *episodes), Digest: digest,
		Owner: opening.Owner, ScopeID: opening.Work, Evidence: evidence,
		Message:  strings.TrimSpace(opening.Message) + "\nrun: " + alertListCommand,
		OpenedAt: now.UTC(), Attempts: []AlertAttempt{}, TransportResult: TransportPending,
	}
	// Persist first; the notification is derived from the persisted record.
	if err := saveAlertEpisode(repoRoot, episode); err != nil {
		return AlertEpisode{}, false, err
	}
	*episodes = append(*episodes, episode)
	stored := &(*episodes)[len(*episodes)-1]
	if err := submitEpisode(repoRoot, stored, now, deliverTo); err != nil {
		return *stored, true, err
	}
	return *stored, true, nil
}

// UpdatePatterns runs one pattern cycle under the alerts lock: the step
// reads and replaces the pattern state, and each observation moves its
// (pattern, work) episode by the lifecycle table of design §1.
func UpdatePatterns(repoRoot string, cycle PatternCycle) (PatternReport, error) {
	if cycle.Step == nil {
		return PatternReport{}, errors.New("a pattern cycle needs a step")
	}
	if cycle.ClearTicks < 1 {
		cycle.ClearTicks = 1
	}
	held, err := lockAlerts(repoRoot, lock.Exclusive)
	if err != nil {
		return PatternReport{}, err
	}
	defer unlockAlerts(held)
	state, err := os.ReadFile(PatternStatePath(repoRoot))
	if err != nil && !os.IsNotExist(err) {
		return PatternReport{}, err
	}
	next, runs, err := cycle.Step(state)
	if err != nil {
		return PatternReport{}, err
	}
	episodes, err := loadAlertEpisodesUnlocked(repoRoot)
	if err != nil {
		return PatternReport{}, err
	}
	var report PatternReport
	for _, run := range runs {
		for _, observation := range run.Observations {
			opened, err := applyPatternObservation(repoRoot, &episodes, run, observation, cycle)
			if err != nil {
				return report, err
			}
			if opened != nil {
				report.Opened = append(report.Opened, *opened)
			}
		}
	}
	if next != nil {
		durable, err := atomicfile.WriteText(PatternStatePath(repoRoot), string(next), repoRoot)
		if err != nil {
			return report, err
		}
		if !durable {
			return report, fmt.Errorf("the pattern state was published with durability unknown")
		}
	}
	for _, episode := range episodes {
		if IsPatternOwner(episode.Owner) && !episode.Cleared {
			report.Open++
		}
	}
	return report, nil
}

func applyPatternObservation(repoRoot string, episodes *[]AlertEpisode, run PatternRun, observation PatternObservation, cycle PatternCycle) (*AlertEpisode, error) {
	owner := PatternOwner(run.Pattern)
	var open *AlertEpisode
	var suppressed []*AlertEpisode
	for index := range *episodes {
		episode := &(*episodes)[index]
		if episode.Owner != owner || episode.ScopeID != observation.Work {
			continue
		}
		switch {
		case !episode.Cleared:
			open = episode
		case episode.Suppressed:
			suppressed = append(suppressed, episode)
		}
	}
	save := func(episode *AlertEpisode) error { return saveAlertEpisode(repoRoot, *episode) }
	switch observation.Kind {
	case ObsFinding:
		if open == nil {
			// openAlertLocked opens nothing for suppressed work.
			episode, created, err := openAlertLocked(repoRoot, episodes, AlertOpening{Owner: owner, Work: observation.Work, Since: observation.Since,
				Evidence: observation.Evidence, Message: observation.Message, Now: cycle.Now, Deliver: cycle.Deliver})
			if err != nil || !created {
				return nil, err
			}
			return &episode, nil
		}
		merged, changed := mergeEvidence(open.Evidence, observation.Evidence)
		if changed || open.CleanCount != 0 || open.Standing != "" {
			open.Evidence, open.CleanCount, open.Standing = merged, 0, ""
			if err := save(open); err != nil {
				return nil, err
			}
		}
		if open.TransportResult != TransportSubmitted {
			deliverTo := cycle.Deliver
			if deliverTo == nil {
				deliverTo = deliver
			}
			if err := submitEpisode(repoRoot, open, cycle.Now, deliverTo); err != nil {
				return nil, err
			}
		}
	case ObsClear:
		for _, episode := range suppressed {
			episode.Suppressed = false
			if err := save(episode); err != nil {
				return nil, err
			}
		}
		if open != nil {
			open.CleanCount++
			open.Standing = ""
			if open.CleanCount >= cycle.ClearTicks {
				open.Resolved, open.ResolvedAt = true, cycle.Now.UTC()
				open.Cleared, open.ClearedAt = true, cycle.Now.UTC()
			}
			if err := save(open); err != nil {
				return nil, err
			}
		}
	case ObsHeld, ObsUnknown:
		standing := StandingHeld
		if observation.Kind == ObsUnknown {
			standing = StandingUnreadable
		}
		if open != nil && open.Standing != standing {
			open.Standing = standing
			if err := save(open); err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("pattern %s observed %q, which is not an observation kind", run.Pattern, observation.Kind)
	}
	return nil, nil
}

// ClearAlert is a person resolving an episode. A pattern episode stays
// suppressed for its work until its detector reports that work clear; a
// repeated clear succeeds and changes nothing.
func ClearAlert(repoRoot, episodeID string, invoker AlertInvoker, now time.Time) (AlertEpisode, bool, error) {
	if !validEpisodeID(episodeID) {
		return AlertEpisode{}, false, fmt.Errorf("alert episode id is invalid")
	}
	held, err := lockAlerts(repoRoot, lock.Exclusive)
	if err != nil {
		return AlertEpisode{}, false, err
	}
	defer unlockAlerts(held)
	episode, err := loadAlertEpisode(alertPath(repoRoot, episodeID))
	if err != nil {
		return AlertEpisode{}, false, err
	}
	if episode.Cleared {
		return episode, false, nil
	}
	episode.Resolved, episode.ResolvedAt = true, now.UTC()
	episode.Cleared, episode.ClearedAt = true, now.UTC()
	episode.ClearedBy = &invoker
	episode.Suppressed = IsPatternOwner(episode.Owner)
	if err := saveAlertEpisode(repoRoot, episode); err != nil {
		return AlertEpisode{}, false, err
	}
	return episode, true, nil
}

// OpenPatternEpisodes counts the open pattern episodes of one store.
func OpenPatternEpisodes(repoRoot string) (int, error) {
	episodes, err := AlertEpisodes(repoRoot)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, episode := range episodes {
		if IsPatternOwner(episode.Owner) && !episode.Cleared {
			count++
		}
	}
	return count, nil
}
