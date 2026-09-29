package steward

// The record kinds that can still need a proof attempt (design
// engine-owns-disk-lifetimes Part B, 3.5 "retention roots", U5b). Each kind
// reads its own records, typed, and names the attempts its live records
// hold; an unreadable kind stops attempt retention for the pass. The
// landing kinds read the landing packages' records through local types of
// the fields they need, because those packages import this one; a test in
// cmd/metasystem marshals the real types through these readers.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// attemptIDs keeps every non-empty value: a reference of any shape keeps
// the attempt it names.
func attemptIDs(values ...string) []string {
	var ids []string
	for _, value := range values {
		if value != "" {
			ids = append(ids, value)
		}
	}
	return ids
}

// ledgerView reads a checkout's accepted goal ledger without a fetch. It
// keeps a projection only while the accepted tip it was read at stays the
// same: every read checks the tip and projects again when it moved, so an
// apply never judges on what a plan read (Round B3-4). Without a tip
// reader it projects on every read.
type ledgerView struct {
	project    func() (goal.Projection, error)
	tip        func() (string, error)
	projection goal.Projection
	readTip    string
	err        error
	read       bool
}

func (v *ledgerView) get() (goal.Projection, error) {
	current := ""
	if v.tip != nil {
		tip, err := v.tip()
		if err != nil {
			return goal.Projection{}, err
		}
		current = tip
		if v.read && v.err == nil && tip == v.readTip {
			return v.projection, nil
		}
	}
	v.read, v.readTip = true, current
	v.projection, v.err = v.project()
	if v.err == nil && v.projection.Tree == nil {
		v.err = errors.New("the goal ledger projection has no tree")
	}
	return v.projection, v.err
}

func checkoutLedger(top string, now time.Time) *ledgerView {
	return &ledgerView{tip: func() (string, error) {
		tip, _, err := goal.AcceptedLedgerTip(top)
		return tip, err
	}, project: func() (goal.Projection, error) {
		endpoint, err := goal.ResolveEndpoint(top)
		if err != nil {
			return goal.Projection{}, err
		}
		return goal.Project(endpoint, false, now)
	}}
}

// stopBatchNamer: a goal stop batch names its proof attempts while it is
// not complete or while its goal is open.
type stopBatchNamer struct {
	Root   string
	Ledger *ledgerView
}

func (stopBatchNamer) Kind() string { return "goal stop batches" }

func (n stopBatchNamer) Named(context.Context, time.Time) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(n.Root, "artifacts", "agents", "goal-stops", "*.json"))
	if err != nil || len(paths) == 0 {
		return nil, err
	}
	projection, err := n.Ledger.get()
	if err != nil {
		return nil, err
	}
	var named []string
	for _, path := range paths {
		var batch goal.StopBatch
		if err := readRecordJSON(path, &batch); err != nil {
			return nil, err
		}
		if batch.State == goal.StopBatchComplete && projection.Tree.Live[batch.GoalID] == nil {
			continue
		}
		named = append(named, attemptIDs(append(batch.PendingProofs, batch.TerminalProofs...)...)...)
		for _, proof := range batch.ObservedProofs {
			named = append(named, attemptIDs(proof.AttemptID)...)
		}
	}
	return named, nil
}

// trunkRedNamer: an open trunk-red entry names the attempts of its
// sightings and fix passes; the latest cadence status names its attempt
// and the attempts its groups reuse.
type trunkRedNamer struct{ Ledger *ledgerView }

func (trunkRedNamer) Kind() string { return "trunk-red register" }

func (n trunkRedNamer) Named(context.Context, time.Time) ([]string, error) {
	projection, err := n.Ledger.get()
	if err != nil {
		return nil, err
	}
	var named []string
	for _, entry := range projection.Tree.TrunkRed {
		if entry.Closed != nil {
			continue
		}
		for _, sighting := range entry.Sightings {
			named = append(named, attemptIDs(sighting.Attempt)...)
			if sighting.Rerun != nil {
				named = append(named, attemptIDs(sighting.Rerun.Attempt)...)
			}
		}
		if entry.FixProof != nil {
			for _, pass := range entry.FixProof.Passes {
				named = append(named, attemptIDs(pass.Attempt)...)
			}
		}
	}
	if cadence := projection.Tree.Cadence; cadence != nil {
		named = append(named, attemptIDs(cadence.AttemptID)...)
		for _, group := range cadence.Groups {
			named = append(named, attemptIDs(group.ReuseSource)...)
		}
	}
	return named, nil
}

// landingBatchRecord is the part of a landing batch record (landing/batch
// Record) that names attempts.
type landingBatchRecord struct {
	State string `json:"state"`
	Units []struct {
		Admission *struct {
			AttemptID string `json:"attemptId"`
		} `json:"admission"`
	} `json:"units"`
	TrunkRed *struct {
		Red struct {
			AttemptID string `json:"attemptId"`
		} `json:"red"`
	} `json:"trunkRed"`
	Proof *struct {
		AttemptID string            `json:"attemptId"`
		Reuse     map[string]string `json:"reuse"`
		Sources   map[string]struct {
			Attempt string `json:"attempt"`
		} `json:"sources"`
	} `json:"proof"`
	Wait *struct {
		For []struct {
			Proof *struct {
				Attempt string `json:"attempt"`
			} `json:"proof"`
		} `json:"for"`
	} `json:"wait"`
	Receipts map[string]struct {
		AttemptID string
		Reused    map[string]string
	} `json:"receipts"`
	// Early is the owner's early acts on the batch's wait (D14, U10b-3):
	// the early proof's attempt and a red finding's attempt.
	Early *struct {
		Attempt string `json:"attempt"`
		Finding *struct {
			Attempt string `json:"attempt"`
		} `json:"finding"`
	} `json:"early"`
}

// landingBatchNamer: a landing batch that has not landed or dissolved
// names the attempts of its admissions, its proof and its sources, its
// trunk-red hold and its prefix receipts.
type landingBatchNamer struct {
	Roots []string
	// LaneErr is why the landing lane's checkout could not be resolved,
	// which holds the kind.
	LaneErr error
}

func (landingBatchNamer) Kind() string { return "landing batches" }

func (n landingBatchNamer) Named(context.Context, time.Time) ([]string, error) {
	if n.LaneErr != nil {
		return nil, fmt.Errorf("the landing lane's checkout cannot be resolved: %w", n.LaneErr)
	}
	var paths []string
	for _, root := range n.Roots {
		found, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "landing-batches", "*.json"))
		if err != nil {
			return nil, err
		}
		paths = append(paths, found...)
	}
	var named []string
	for _, path := range paths {
		var record landingBatchRecord
		if err := readRecordJSON(path, &record); err != nil {
			return nil, err
		}
		if record.State == "landed" || record.State == "dissolved" {
			continue
		}
		for _, unit := range record.Units {
			if unit.Admission != nil {
				named = append(named, attemptIDs(unit.Admission.AttemptID)...)
			}
		}
		if record.TrunkRed != nil {
			named = append(named, attemptIDs(record.TrunkRed.Red.AttemptID)...)
		}
		if record.Proof != nil {
			named = append(named, attemptIDs(record.Proof.AttemptID)...)
			for _, source := range record.Proof.Sources {
				named = append(named, attemptIDs(source.Attempt)...)
			}
			for _, reused := range record.Proof.Reuse {
				named = append(named, attemptIDs(reused)...)
			}
		}
		if record.Wait != nil {
			for _, waited := range record.Wait.For {
				if waited.Proof != nil {
					named = append(named, attemptIDs(waited.Proof.Attempt)...)
				}
			}
		}
		if record.Early != nil {
			named = append(named, attemptIDs(record.Early.Attempt)...)
			if record.Early.Finding != nil {
				named = append(named, attemptIDs(record.Early.Finding.Attempt)...)
			}
		}
		for _, receipt := range record.Receipts {
			named = append(named, attemptIDs(receipt.AttemptID)...)
			for _, reused := range receipt.Reused {
				named = append(named, attemptIDs(reused)...)
			}
		}
	}
	return named, nil
}

// landingReceipt is the part of a landing test receipt (landing
// TestReceipt) that names attempts.
type landingReceipt struct {
	Time       string   `json:"time"`
	AttemptIDs []string `json:"attemptIds"`
	Proof      *struct {
		AttemptID string `json:"attemptId"`
	} `json:"proof"`
	Coverage *struct {
		AttemptID string `json:"attemptId"`
	} `json:"coverage"`
	Testing *proofrun.TestResult `json:"testing"`
}

// landingReceiptNamer: a test receipt names its attempts, and the attempts
// its result reuses, while it is younger than disk.proof-keep-days (its
// proof is consumed while fresh; afterwards it is history), and a hand
// landing's receipt whatever its age while that landing has not landed.
type landingReceiptNamer struct {
	Installation string
	Keep         time.Duration
}

func (landingReceiptNamer) Kind() string { return "landing test receipts" }

func (n landingReceiptNamer) Named(_ context.Context, now time.Time) ([]string, error) {
	agents := filepath.Join(n.Installation, "artifacts", "agents")
	stored, err := filepath.Glob(filepath.Join(agents, "landing", "receipts", "*.json"))
	if err != nil {
		return nil, err
	}
	intent, err := filepath.Glob(filepath.Join(agents, "landing-intent", "*", "*", "receipt-*.json"))
	if err != nil {
		return nil, err
	}
	var named []string
	for _, path := range append(stored, intent...) {
		var receipt landingReceipt
		if err := readRecordJSON(path, &receipt); err != nil {
			return nil, err
		}
		pending := false
		if filepath.Base(filepath.Dir(filepath.Dir(path))) != "landing" {
			_, landedErr := os.Stat(filepath.Join(filepath.Dir(path), "landed.json"))
			pending = errors.Is(landedErr, os.ErrNotExist)
		}
		at, timeErr := time.Parse(time.RFC3339Nano, receipt.Time)
		if !pending && timeErr == nil && now.Sub(at) >= n.Keep {
			continue
		}
		named = append(named, attemptIDs(receipt.AttemptIDs...)...)
		if receipt.Proof != nil {
			named = append(named, attemptIDs(receipt.Proof.AttemptID)...)
		}
		if receipt.Coverage != nil {
			named = append(named, attemptIDs(receipt.Coverage.AttemptID)...)
		}
		if receipt.Testing != nil {
			named = append(named, attemptIDs(proofrun.TestResultReferences(*receipt.Testing)...)...)
		}
	}
	return named, nil
}

// validationWindowNamer: the steward's direct-validation window names the
// attempts of the observations it keeps.
type validationWindowNamer struct{ Root string }

func (validationWindowNamer) Kind() string { return "direct-validation window" }

func (n validationWindowNamer) Named(context.Context, time.Time) ([]string, error) {
	state, err := loadValidationWindow(n.Root)
	if err != nil {
		return nil, err
	}
	var named []string
	for _, observation := range state.Observations {
		named = append(named, attemptIDs(observation.AttemptID)...)
	}
	return named, nil
}

func dedupe(paths ...string) []string {
	var unique []string
	for _, path := range paths {
		if !containsPath(unique, path) {
			unique = append(unique, path)
		}
	}
	return unique
}

// readRecordJSON reads one record and names its path in an error.
func readRecordJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// attemptRetention is the proof attempt class of a checkout pass with
// every record kind that can name an attempt.
func attemptRetention(top string, now time.Time, target int64, keep time.Duration) *proofrun.Retention {
	installation := top
	if layout, err := stateroot.ResolveLayout(top); err == nil {
		installation = layout.InstallationRoot
	}
	ledger := checkoutLedger(top, now)
	lane, laneErr := landingLaneRoots(installation, now)
	prober := identity.KernelProber{}
	return &proofrun.Retention{Control: top, Target: target, Keep: keep,
		Alive: func(ref identity.Ref) identity.Liveness { return identity.AliveRef(prober, ref) },
		Namers: []proofrun.AttemptNamer{
			proofrun.ScratchNamer{Control: top},
			stopBatchNamer{Root: top, Ledger: ledger},
			trunkRedNamer{Ledger: ledger},
			landingBatchNamer{Roots: nonEmpty(dedupe(append([]string{top, installation}, lane...)...)...), LaneErr: laneErr},
			landingReceiptNamer{Installation: installation, Keep: keep},
			validationWindowNamer{Root: top},
		}}
}

// LandingAttemptNamers are the two landing record kinds, for the
// conformance test that marshals the landing packages' own types.
func LandingAttemptNamers(batchRoots []string, installation string, keep time.Duration) []proofrun.AttemptNamer {
	return []proofrun.AttemptNamer{landingBatchNamer{Roots: batchRoots}, landingReceiptNamer{Installation: installation, Keep: keep}}
}

// landingLaneRoots are the landing lane's checkout, resolved through the
// host lane resolver the command layer binds (LandingLaneRoot: the seat's
// setting against the host's one lane record, U12), and its installation,
// whose landing batches can name this checkout's attempts; none when the
// host has no lane for this seat.
func landingLaneRoots(installation string, now time.Time) ([]string, error) {
	return landingLaneRootsWith(LandingLaneRoot, installation, now)
}

// landingLaneRootsWith fails closed: an unbound resolver, an unresolvable
// lane, or a configured lane without a root is an error, never "no lane".
func landingLaneRootsWith(resolve func(string, time.Time) (string, bool, error), installation string, now time.Time) ([]string, error) {
	if resolve == nil {
		return nil, errors.New("no landing lane resolver is bound in this process")
	}
	root, configured, err := resolve(installation, now)
	if err != nil {
		return nil, err
	}
	if !configured {
		return nil, nil
	}
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("the landing lane is configured but names no checkout")
	}
	roots := []string{root}
	if layout, err := stateroot.ResolveLayout(root); err == nil {
		roots = append(roots, layout.InstallationRoot)
	}
	return roots, nil
}
