package diskstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// CheckoutFacts are a checkout's four facts (3.12 "Checkout identity"): its
// git root, its installation, its root commit and its ledger identity (the
// accepted goal tree's RootRecord.Identity). An empty root commit or ledger
// identity is unknown.
type CheckoutFacts struct {
	GitRoot        string `json:"gitRoot"`
	Installation   string `json:"installation"`
	RootCommit     string `json:"rootCommit,omitempty"`
	LedgerIdentity string `json:"ledgerIdentity,omitempty"`
}

// SuiteFailures is the checkout pass's evidence ageing (3.3 trigger 3, 3.5):
// a bundle idle disk.suite-failure-distill-hours is distilled; a distilled
// bundle disk.suite-failure-move-days old by its recorded creation stamp is
// moved into the git root's segment of the evidence root. Nothing here
// deletes a unique byte: the distiller replaces with verified recipes and
// the move removes a source only after its copy verified.
type SuiteFailures struct {
	// Dir is <control>/artifacts/agents/suite-failures.
	Dir string
	// SegmentDir is <the evidence root>/suite-failures/<segment of the git root>;
	// empty when the evidence root is unknown, and then nothing moves.
	SegmentDir    string
	Installation  string
	GitRoot       string
	Blobs         BlobStore
	CompressAbove int64
	DistillAfter  time.Duration
	MoveAfter     time.Duration
	// Target is disk.suite-failure-target-gib: over it is reported.
	Target int64
	Known  func(ctx context.Context, paths []string) (map[string]bool, error)
	// Facts completes a bundle's owner file with the checkout's root commit
	// and ledger identity; nil leaves them absent.
	Facts func(ctx context.Context) (CheckoutFacts, error)
	// AttemptGoal reads an attempt record's accounted goal ("" when it names
	// none); found false when the record is gone.
	AttemptGoal func(attempt string) (goal string, found bool)
	Entropy     io.Reader
	Sync        Syncer
}

// Name names the class.
func (s SuiteFailures) Name() string { return "suite-failure bundles" }

const (
	actionDistil = "distil:"
	actionMove   = "move:"
	actionFinish = "finish-move:"
)

// Plan observes every bundle and creates nothing.
func (s SuiteFailures) Plan(ctx context.Context, pass *Pass) ([]Item, error) {
	entries, err := os.ReadDir(s.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	distillAfter, moveAfter := s.DistillAfter, s.MoveAfter
	if pass.Floor && pass.FloorMinAge > 0 {
		distillAfter, moveAfter = minDuration(distillAfter, pass.FloorMinAge), minDuration(moveAfter, pass.FloorMinAge)
	}
	var items []Item
	var total int64
	for _, entry := range entries {
		if ctx.Err() != nil {
			break
		}
		name := entry.Name()
		path := filepath.Join(s.Dir, name)
		if bundle, ok := MovedSource(name); ok {
			item := Item{Class: s.Name(), Key: actionFinish + name, Path: path}
			if s.SegmentDir != "" && fileExists(filepath.Join(s.SegmentDir, bundle)) {
				item.Verdict = Verdict{Decision: Release, Reason: "finish removing a moved bundle's source; its copy is in " + s.SegmentDir}
			} else {
				item.Verdict = Verdict{Decision: Pending, Reason: "a moved bundle's source was set aside but its copy is not in the segment; it is kept",
					Command: "metasystem evidence show " + path}
			}
			items = append(items, item)
			continue
		}
		if !entry.IsDir() || IsPartial(name) {
			continue
		}
		bytes, newest, _ := Measure(ctx, path)
		total += bytes
		item := Item{Class: s.Name(), Path: path, Bytes: bytes, Verdict: Verdict{Decision: Wait}}
		header, lines, present, readErr := ReadDistilled(path)
		switch {
		case readErr != nil:
			item.Key = name
			item.Verdict = Verdict{Decision: Pending, Reason: "its DISTILLED.txt is unreadable: " + readErr.Error(), Command: "metasystem evidence show " + path}
		case !present || openTransaction(path, lines):
			item.Key = actionDistil + name
			if pass.Now.Sub(newest) >= distillAfter {
				item.Verdict = Verdict{Decision: Release, Reason: "distil: idle since " + newest.UTC().Format(time.RFC3339)}
			}
		case pass.Now.Sub(header.Created) >= moveAfter:
			item.Key = actionMove + name
			if s.SegmentDir == "" {
				item.Verdict = Verdict{Decision: Pending, Reason: "due to move, but this checkout's evidence root is unknown", Command: "metasystem settings check"}
			} else {
				item.Verdict = Verdict{Decision: Release, Reason: "move into " + s.SegmentDir}
			}
		default:
			item.Key = name
		}
		items = append(items, item)
	}
	if s.Target > 0 && total > s.Target {
		items = append(items, Item{Class: s.Name(), Key: "over-target", Path: s.Dir, Verdict: Verdict{Decision: Keep,
			Reason:  fmt.Sprintf("suite-failure bundles hold %s, over their target of %s; each is distilled when idle and moved into the evidence root when old enough", formatBytes(total), formatBytes(s.Target)),
			Command: "metasystem disk show"}})
	}
	return items, nil
}

func minDuration(a, b time.Duration) time.Duration {
	if b < a {
		return b
	}
	return a
}

// openTransaction reports a manifest line whose original is still present:
// an interrupted distillation the next run finishes.
func openTransaction(bundle string, lines []RecipeLine) bool {
	for _, line := range lines {
		if line.Restores() && fileExists(filepath.Join(bundle, filepath.FromSlash(line.Path))) {
			return true
		}
	}
	return false
}

// Apply distils, moves, or finishes one bundle.
func (s SuiteFailures) Apply(ctx context.Context, pass *Pass, item Item) Verdict {
	stage, err := NewID(pass.Now, s.Entropy)
	if err != nil {
		return Verdict{Decision: Pending, Reason: err.Error(), Command: "metasystem disk clean"}
	}
	switch {
	case strings.HasPrefix(item.Key, actionFinish):
		// A source set aside by a move cut short is re-checked against its
		// copy first, and its references go to the copy only then (Round
		// B2-4).
		bundle, _ := MovedSource(filepath.Base(item.Path))
		source := filepath.Join(filepath.Dir(item.Path), bundle)
		rules := MoveRules{SegmentDir: s.SegmentDir, Blobs: s.Blobs, Referrer: s.referrer(source), Installation: s.Installation,
			Segment: Segment(s.GitRoot), Stage: movedStage(item.Path), Sync: s.Sync, Stores: CheckoutRegistry(s.Installation)}
		kept, err := rules.FinishMovedSource(ctx, item.Path, filepath.Join(s.SegmentDir, bundle))
		switch {
		case err != nil:
			return s.pending("finishing the move", err)
		case kept != "":
			return Verdict{Decision: Keep, Reason: kept, Command: "metasystem evidence show " + source}
		}
		return Verdict{Decision: Release, Reason: "removed a moved bundle's source"}
	case strings.HasPrefix(item.Key, actionDistil):
		rules := s.distillRules(item.Path, stage)
		result, err := Distill(ctx, item.Path, rules, pass.Now)
		if err != nil {
			return s.pending("distillation", err)
		}
		s.complete(ctx, item.Path, pass.Now)
		reason := fmt.Sprintf("distilled: %d replaced, %d finished", len(result.Replaced), len(result.Finished))
		if len(result.Kept) > 0 {
			reason += fmt.Sprintf(", %d kept (%s)", len(result.Kept), result.Kept[0])
		}
		return Verdict{Decision: Release, Reason: reason}
	case strings.HasPrefix(item.Key, actionMove):
		if s.SegmentDir == "" {
			return Verdict{Decision: Pending, Reason: "this checkout's evidence root is unknown", Command: "metasystem settings check"}
		}
		if _, err := os.Lstat(filepath.Join(item.Path, OwnerFileName)); errors.Is(err, os.ErrNotExist) {
			if _, err := WriteBundleOwner(item.Path, s.legacyOwner(item.Path, pass.Now), s.Sync); err != nil {
				return s.pending("the owner file", err)
			}
		}
		s.complete(ctx, item.Path, pass.Now)
		result, err := MoveBundle(ctx, item.Path, MoveRules{SegmentDir: s.SegmentDir, Blobs: s.Blobs, Referrer: s.referrer(item.Path),
			Installation: s.Installation, Segment: Segment(s.GitRoot), Stage: stage, Sync: s.Sync, Stores: CheckoutRegistry(s.Installation)})
		switch {
		case err != nil:
			return s.pending("the move", err)
		case result.Kept != "":
			return Verdict{Decision: Keep, Reason: result.Kept, Command: "metasystem evidence show " + result.Destination}
		}
		return Verdict{Decision: Release, Reason: "moved to " + result.Destination}
	}
	return Verdict{Decision: Pending, Reason: "no action for " + item.Key, Command: "metasystem disk show"}
}

func (s SuiteFailures) pending(what string, err error) Verdict {
	if errors.Is(err, ErrBlobStoreBusy) {
		return Verdict{Decision: Pending, Reason: "the blob store is in a reference check; " + what + " waits for the next pass", Command: "metasystem disk clean"}
	}
	return Verdict{Decision: Pending, Reason: what + " stopped: " + err.Error(), Command: "metasystem disk clean"}
}

func (s SuiteFailures) referrer(bundle string) string {
	return Segment(s.GitRoot) + "-" + filepath.Base(bundle)
}

func (s SuiteFailures) distillRules(bundle, stage string) DistillRules {
	return DistillRules{CompressAbove: s.CompressAbove, Blobs: s.Blobs, Referrer: s.referrer(bundle), Installation: s.Installation,
		Segment: Segment(s.GitRoot), Stage: stage, Known: s.Known, Sync: s.Sync,
		Owner: func(bundle string) (BundleOwner, error) { return s.legacyOwner(bundle, time.Time{}), nil }}
}

// attemptInName finds a proof attempt's id in a bundle's name; bundles
// written before names carried one have none.
var attemptInName = regexp.MustCompile(`proof-[a-z0-9]+-[0-9a-f]{16}`)

// legacyOwner is the owner of a bundle written without OWNER.json (3.5): the
// attempt its name carries and that attempt's accounted goal while the
// record exists; otherwise goal unknown, which the exclusions hold.
func (s SuiteFailures) legacyOwner(bundle string, now time.Time) BundleOwner {
	owner := BundleOwner{Attempt: GoalUnknown, Goal: GoalUnknown, GitRoot: s.GitRoot, Installation: s.Installation,
		WrittenBy: "distiller", WrittenAt: now.UTC()}
	attempt := attemptInName.FindString(filepath.Base(bundle))
	if attempt == "" || s.AttemptGoal == nil {
		return owner
	}
	owner.Attempt = attempt
	if goal, found := s.AttemptGoal(attempt); found {
		owner.Goal = goal
		if goal == "" {
			owner.Goal = GoalNone
		}
	}
	return owner
}

// complete fills a bundle owner's absent checkout facts: the bundle writer
// runs no git on the failure path, so the root commit and the ledger
// identity are recorded by the checkout pass that distils or moves it.
func (s SuiteFailures) complete(ctx context.Context, bundle string, now time.Time) {
	if s.Facts == nil {
		return
	}
	owner, err := ReadBundleOwner(bundle)
	if err != nil || owner.RootCommit != "" && owner.LedgerIdentity != "" {
		return
	}
	facts, err := s.Facts(ctx)
	if err != nil {
		return
	}
	changed := false
	for _, field := range []struct {
		have *string
		want string
	}{{&owner.GitRoot, facts.GitRoot}, {&owner.Installation, facts.Installation}, {&owner.RootCommit, facts.RootCommit}, {&owner.LedgerIdentity, facts.LedgerIdentity}} {
		if *field.have == "" && field.want != "" {
			*field.have, changed = field.want, true
		}
	}
	if !changed {
		return
	}
	data, err := json.MarshalIndent(owner, "", "  ")
	if err == nil {
		_ = s.Sync.WriteDurable(filepath.Join(bundle, OwnerFileName), append(data, '\n'), fmt.Sprintf("owner-%d", now.UnixNano()))
	}
}
