package steward

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// attentionPolicyBed owns a declared accepted world and a strict record of
// repository calls. Projection replies always come from parsed goal files.
type attentionPolicyBed struct {
	t                *testing.T
	root             string
	now              time.Time
	accepted, remote string
	local, migrated  bool
	worlds           map[string]map[string][]byte
	changes          map[string][]goal.LedgerChange
	stageTime        map[string]time.Time
	entries          []goal.Entry
	repairBaseline   []string
	failure          map[string]error
	calls            []string
}

func newAttentionPolicyBed(t *testing.T) *attentionPolicyBed {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\nsteward.ledger-attention-stale-minutes=30\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := &attentionPolicyBed{t: t, root: root, now: bedTime(), accepted: "base", remote: "base", migrated: true,
		worlds: map[string]map[string][]byte{}, changes: map[string][]goal.LedgerChange{}, stageTime: map[string]time.Time{}, failure: map[string]error{}}
	b.world("base", nil)
	return b
}

func (b *attentionPolicyBed) world(tip string, files map[string]*goal.GoalFile) {
	b.t.Helper()
	root := &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncRemote, Revision: 1}
	rendered := map[string][]byte{"plans/goals/backlog.md": goal.RenderRoot(root)}
	for id, file := range files {
		rendered["plans/goals/"+id+".md"] = goal.RenderFile(file)
	}
	b.worlds[tip] = rendered
}

func (b *attentionPolicyBed) move(tip string, files map[string]*goal.GoalFile, from string, changes ...goal.LedgerChange) {
	b.world(tip, files)
	b.remote = tip
	if len(changes) == 0 {
		changes = []goal.LedgerChange{{Tip: tip, Consecutive: true}}
	}
	b.changes[from+">"+tip] = changes
}

func (b *attentionPolicyBed) checkRoot(root string) {
	b.t.Helper()
	if root != b.root {
		b.t.Fatalf("repository root = %q, want %q", root, b.root)
	}
}

func (b *attentionPolicyBed) checkEndpoint(endpoint goal.Endpoint) {
	b.t.Helper()
	b.checkRoot(endpoint.Root)
	if endpoint.Remote != "origin" || endpoint.Branch != "refs/heads/main" {
		b.t.Fatalf("repository endpoint = %+v, want origin refs/heads/main", endpoint)
	}
}

func (b *attentionPolicyBed) call(name string) error {
	b.calls = append(b.calls, name)
	return b.failure[name]
}

func (b *attentionPolicyBed) projection(tip string, at time.Time) goal.Projection {
	b.t.Helper()
	files, ok := b.worlds[tip]
	if !ok {
		b.t.Fatalf("undeclared goal tree at %q", tip)
	}
	tree, problems := goal.ParseTreeFiles(files)
	if len(problems) != 0 {
		b.t.Fatalf("rendered goal tree at %q: %v", tip, problems)
	}
	horizon := goal.ApprovalHorizon{Now: at}
	if tree.Root.FleetEnrollment != nil {
		horizon.EnrolledAt, _ = time.Parse(time.RFC3339, tree.Root.FleetEnrollment.At)
	}
	return goal.Projection{Root: b.root, Tip: tip, Tree: tree, Horizon: horizon}
}

func fixtureAdded(before, after []string) []string {
	var added []string
	for _, candidate := range after {
		found := false
		for _, existing := range before {
			if candidate == existing {
				found = true
				break
			}
		}
		if !found {
			added = append(added, candidate)
		}
	}
	return added
}

func (b *attentionPolicyBed) expectedStage(state ledgerAttentionState, tip string) ledgerAttentionStage {
	b.t.Helper()
	at := b.stageTime[tip]
	changes := b.changes[b.accepted+">"+tip]
	if at.IsZero() || len(changes) == 0 {
		b.t.Fatalf("no declared stage time or changes for %s > %s", b.accepted, tip)
	}
	before, err := snapshotLedger(b.projection(b.accepted, at), "mac-a")
	if err != nil {
		b.t.Fatal(err)
	}
	want := ledgerAttentionStage{From: b.accepted, Tip: tip, TopologyEpoch: state.TopologyEpoch, RepairBaseline: append([]string(nil), b.repairBaseline...), RepairBaselineReady: true}
	for _, change := range changes {
		if !change.Consecutive {
			want.TopologyEpoch++
		}
		after, err := snapshotLedger(b.projection(change.Tip, at), "mac-a")
		if err != nil {
			b.t.Fatal(err)
		}
		event := LedgerAttentionEvent{SourceID: eventSourceID(change.Tip, want.TopologyEpoch), Tip: change.Tip, At: at.UTC().Format(time.RFC3339Nano),
			Claimable: fixtureAdded(before.Ready, after.Ready), Pins: fixtureAdded(before.Pinned, after.Pinned)}
		if !reflect.DeepEqual(before.Queue, after.Queue) {
			event.QueueWas = append([]string(nil), before.Queue...)
			event.QueueNow = append([]string(nil), after.Queue...)
		}
		if len(event.Claimable) > 0 || len(event.Pins) > 0 || event.QueueWas != nil || event.QueueNow != nil {
			want.Events = append(want.Events, event)
		}
		before = after
	}
	want.Ready, want.Pinned, want.Queue = before.Ready, before.Pinned, before.Queue
	return want
}

func (b *attentionPolicyBed) repository() *ledgerAttentionRepository {
	return &ledgerAttentionRepository{
		ResolveEndpoint: func(root string) (goal.Endpoint, error) {
			b.checkRoot(root)
			if err := b.call("endpoint"); err != nil {
				return goal.Endpoint{}, err
			}
			remote := "origin"
			if b.local {
				remote = "local"
			}
			return goal.Endpoint{Root: root, Remote: remote, Branch: "refs/heads/main"}, nil
		},
		AcceptedLedgerTip: func(root string) (string, bool, error) {
			b.checkRoot(root)
			if err := b.call("accepted"); err != nil {
				return "", false, err
			}
			return b.accepted, b.migrated, nil
		},
		ResolveMachine: func(root string) (string, error) {
			b.checkRoot(root)
			if err := b.call("machine"); err != nil {
				return "", err
			}
			return "mac-a", nil
		},
		ProjectAt: func(root, tip string, at ...time.Time) (goal.Projection, error) {
			b.checkRoot(root)
			call := "project:" + tip
			if len(at) > 1 {
				b.t.Fatalf("ProjectAt time arguments: %v", at)
			}
			if len(at) == 1 {
				call += "@" + at[0].UTC().Format(time.RFC3339Nano)
			}
			if err := b.call(call); err != nil {
				return goal.Projection{}, err
			}
			now := time.Now().UTC()
			if len(at) > 0 {
				now = at[0]
			}
			return b.projection(tip, now), nil
		},
		Entries: func(root string) ([]goal.Entry, error) {
			b.checkRoot(root)
			if err := b.call("entries"); err != nil {
				return nil, err
			}
			return append([]goal.Entry(nil), b.entries...), nil
		},
		LedgerChanges: func(root, before, after string) ([]goal.LedgerChange, error) {
			b.checkRoot(root)
			if err := b.call("changes:" + before + ">" + after); err != nil {
				return nil, err
			}
			changes, ok := b.changes[before+">"+after]
			if !ok {
				b.t.Fatalf("undeclared ledger changes %s > %s", before, after)
			}
			return append([]goal.LedgerChange(nil), changes...), nil
		},
		CaptureTipBounded: func(endpoint goal.Endpoint, budget time.Duration) (goal.BoundedCapture, error) {
			b.checkEndpoint(endpoint)
			if budget != ledgerAttentionFetchBudget {
				b.t.Fatalf("capture arguments: endpoint=%+v budget=%s", endpoint, budget)
			}
			if err := b.call("capture"); err != nil {
				return goal.BoundedCapture{}, err
			}
			return goal.BoundedCapture{Tip: b.remote, OperationID: "capture-1"}, nil
		},
		CleanupRefs: func(endpoint goal.Endpoint, opid string) {
			b.checkEndpoint(endpoint)
			if opid != "capture-1" {
				b.t.Fatalf("cleanup arguments: %+v %q", endpoint, opid)
			}
			b.call("cleanup")
		},
		SyncModeGate: func(endpoint goal.Endpoint, tip string) error {
			b.checkEndpoint(endpoint)
			if tip != b.remote {
				b.t.Fatalf("sync tip = %q, want %q", tip, b.remote)
			}
			return b.call("sync")
		},
		AcceptanceGates: func(root, accepted, fetched string) error {
			b.checkRoot(root)
			if accepted != b.accepted || fetched != b.remote {
				b.t.Fatalf("gate tips = %q, %q, want %q, %q", accepted, fetched, b.accepted, b.remote)
			}
			return b.call("gates")
		},
		ValidateCommit: func(root, tip string) error {
			b.checkRoot(root)
			if tip != b.remote {
				b.t.Fatalf("validate tip = %q, want %q", tip, b.remote)
			}
			return b.call("validate")
		},
		AdvanceAccepted: func(root, tip string) error {
			b.checkRoot(root)
			if tip != b.remote && b.worlds[tip] == nil {
				b.t.Fatalf("advance to undeclared tip %q", tip)
			}
			state, exists, err := loadLedgerAttentionState(root)
			if err != nil || !exists || state.Staged == nil || state.Staged.From != b.accepted || state.Staged.Tip != tip {
				b.t.Fatalf("accepted movement preceded durable stage: state=%+v exists=%t err=%v", state, exists, err)
			}
			want := b.expectedStage(state, tip)
			if !reflect.DeepEqual(*state.Staged, want) {
				b.t.Fatalf("durable staged content disagrees with rendered goal bytes: got=%+v want=%+v", *state.Staged, want)
			}
			if err := b.call("advance:" + tip); err != nil {
				return err
			}
			b.accepted = tip
			return nil
		},
		IsAncestor: func(root, ancestor, descendant string) (bool, error) {
			b.checkRoot(root)
			if err := b.call("ancestor:" + ancestor + ">" + descendant); err != nil {
				return false, err
			}
			return ancestor == descendant, nil
		},
	}
}

func (b *attentionPolicyBed) run(at time.Time, want string) LedgerAttentionReport {
	return b.runWith(at, want, func(repository *ledgerAttentionRepository) LedgerAttentionReport {
		return runLedgerAttentionWithRepository(b.root, at, repository)
	})
}

func (b *attentionPolicyBed) runWithWriter(at time.Time, want string, writer ledgerAttentionStateWriter) LedgerAttentionReport {
	return b.runWith(at, want, func(repository *ledgerAttentionRepository) LedgerAttentionReport {
		return runLedgerAttentionWithRepositoryAndWriter(b.root, at, repository, writer)
	})
}

func (b *attentionPolicyBed) runWith(at time.Time, want string, run func(*ledgerAttentionRepository) LedgerAttentionReport) LedgerAttentionReport {
	b.t.Helper()
	b.calls = nil
	if b.remote != b.accepted && b.stageTime[b.remote].IsZero() {
		b.stageTime[b.remote] = at
	}
	report := run(b.repository())
	if expected := strings.Fields(want); !reflect.DeepEqual(b.calls, expected) {
		b.t.Fatalf("repository calls: got %v, want %v; report=%+v", b.calls, expected, report)
	}
	state, exists, err := loadLedgerAttentionState(b.root)
	if err != nil || (!exists && report.FailureKind != ledgerAttentionStateWriteFailed) {
		b.t.Fatalf("attention state missing after pass: exists=%t err=%v", exists, err)
	}
	if report.Outcome == "advanced" || report.Outcome == "current" {
		if state.Staged != nil || state.DiffedTip != b.accepted || !reflect.DeepEqual(state.Pending, report.Pending) {
			b.t.Fatalf("after-state disagrees with accepted effect or report: state=%+v accepted=%q report=%+v", state, b.accepted, report)
		}
	}
	return report
}

func (b *attentionPolicyBed) state() ledgerAttentionState {
	b.t.Helper()
	state, exists, err := loadLedgerAttentionState(b.root)
	if err != nil || !exists {
		b.t.Fatalf("attention state missing: exists=%t err=%v", exists, err)
	}
	return state
}

func (b *attentionPolicyBed) fail(call, message string) { b.failure[call] = errors.New(message) }

func queuedAttentionGoal(id string) *goal.GoalFile {
	return &goal.GoalFile{Id: id, State: goal.StateQueued, Intent: "Work on " + id + " safely.", Origin: goal.OriginMain,
		NextStep: "Work on " + id + ".", OpenedAt: "2026-08-23T00:00:00Z", Revision: 1, History: bedHistory(id, "open")}
}

func approvedAttentionGoal(id string) *goal.GoalFile {
	return approvedStewardGoal(id, "Work on "+id+" safely.", "Work on "+id+".", "2026-08-23T00:00:00Z")
}

func attentionWorld(files ...*goal.GoalFile) map[string]*goal.GoalFile {
	world := make(map[string]*goal.GoalFile, len(files))
	for _, file := range files {
		world[file.Id] = file
	}
	return world
}

func attentionMoveCalls(from string, tips ...string) string {
	parts := []string{"endpoint", "accepted", "machine", "capture", "sync", "accepted", "gates", "validate", "entries", "project:" + from, "changes:" + from + ">" + tips[len(tips)-1]}
	for _, tip := range tips {
		parts = append(parts, "project:"+tip)
	}
	parts = append(parts, "advance:"+tips[len(tips)-1], "entries", "cleanup")
	return strings.Join(parts, " ")
}

func attentionBaselineCalls() string {
	return "endpoint accepted machine project:base capture sync accepted validate entries cleanup"
}

func attentionMovedCalls(from string, tips ...string) string {
	return fmt.Sprintf("endpoint accepted machine entries %s", strings.TrimPrefix(attentionMoveCalls(from, tips...), "endpoint accepted machine "))
}
