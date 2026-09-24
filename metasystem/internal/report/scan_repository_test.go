package report

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const scanFixtureLedger = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
const scanFixtureTip = "accepted-report-scan-tip"

// scanReadFixture verifies the raw scan inputs and the committed facts read by
// the real goal projector. An unexpected repository method reaches the nil
// embedded interface and fails the test immediately.
type scanReadFixture struct {
	goal.Repository
	t           *testing.T
	root        string
	files       map[string][]byte
	committed   time.Time
	world       bool
	machine     string
	absent      bool
	endpointErr error
	acceptedErr error
	filesErr    error
	want        []string
	next        int
	commitCalls int
}

func newScanReadFixture(t *testing.T, root string) *scanReadFixture {
	t.Helper()
	return &scanReadFixture{t: t, root: root, committed: time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC), machine: "bed-m1"}
}

func absentScanReads(t *testing.T, root string) *scanReadFixture {
	t.Helper()
	f := newScanReadFixture(t, resolveRepo(root))
	f.world = false
	f.absent = true
	return f
}

func openWorkWithoutGoal(t *testing.T, root string) []string {
	t.Helper()
	f := absentScanReads(t, root)
	f.expect("identity", "identity")
	lines := openWorkReportWithReads(root, f.reads())
	f.checked(0)
	return lines
}

func scanWithoutGoal(t *testing.T, root string) goal.ScanResult {
	t.Helper()
	f := absentScanReads(t, root)
	f.expect("identity", "endpoint", "accepted", "world")
	scan, _ := scanWithCompletionWithReads(root, f.reads())
	f.checked(0)
	return scan
}

func scanWithProberWithoutGoal(t *testing.T, root string, prober identity.Prober) goal.ScanResult {
	t.Helper()
	return scanWithProberAtWithoutGoal(t, root, prober, time.Now().UTC())
}

func scanWithProberAtWithoutGoal(t *testing.T, root string, prober identity.Prober, at time.Time) goal.ScanResult {
	t.Helper()
	f := absentScanReads(t, root)
	f.expect("identity", "endpoint", "accepted", "world")
	scan := scanWithProberAtWithReads(root, prober, at, f.reads())
	f.checked(0)
	return scan
}

func runningWorkClauseWithoutGoal(t *testing.T, root string) string {
	t.Helper()
	f := absentScanReads(t, root)
	f.expect("identity", "endpoint", "accepted", "world")
	clause := runningWorkClauseWithReads(root, f.reads())
	f.checked(0)
	return clause
}

func (f *scanReadFixture) expect(calls ...string) {
	f.t.Helper()
	f.want, f.next, f.commitCalls = calls, 0, 0
}

func (f *scanReadFixture) call(name string) {
	f.t.Helper()
	if f.next >= len(f.want) || f.want[f.next] != name {
		f.t.Fatalf("unexpected scan goal read %q at %d; expected calls: %v", name, f.next, f.want)
	}
	f.next++
}

func (f *scanReadFixture) checked(commitCalls int) {
	f.t.Helper()
	if f.next != len(f.want) || f.commitCalls != commitCalls {
		f.t.Fatalf("scan goal reads: consumed %d of %d calls, CommitTime called %d times; want %d; sequence %v",
			f.next, len(f.want), f.commitCalls, commitCalls, f.want)
	}
}

func (f *scanReadFixture) checkRoot(root string) {
	f.t.Helper()
	if root != f.root {
		f.t.Fatalf("goal read root = %q, want physical fixture root %q", root, f.root)
	}
}

func (f *scanReadFixture) reads() scanGoalReads {
	return scanGoalReads{
		existingLedgerIdentity: func(root string) string {
			f.call("identity")
			f.checkRoot(root)
			if !f.world {
				return ""
			}
			return scanFixtureLedger
		},
		resolveEndpoint: func(root string) (goal.Endpoint, error) {
			f.call("endpoint")
			f.checkRoot(root)
			if f.endpointErr != nil {
				return goal.Endpoint{}, f.endpointErr
			}
			return goal.Endpoint{Root: root, Remote: "local", Repository: f}, nil
		},
		newWorld: func(root string) bool {
			f.call("world")
			f.checkRoot(root)
			return f.world
		},
		resolveMachine: func(root string) (string, error) {
			f.call("machine")
			f.checkRoot(root)
			if f.machine == "" {
				return "", errors.New("no machine nickname is enrolled")
			}
			return f.machine, nil
		},
	}
}

func (f *scanReadFixture) Accepted() (string, bool, error) {
	f.call("accepted")
	if f.acceptedErr != nil {
		return "", false, f.acceptedErr
	}
	if f.absent {
		return "", false, nil
	}
	return scanFixtureTip, true, nil
}

func (f *scanReadFixture) Files(tip string, prefixes ...string) (map[string][]byte, error) {
	f.call("files")
	if tip != scanFixtureTip || !reflect.DeepEqual(prefixes, []string{"plans/goals/", "records/goals/"}) {
		f.t.Fatalf("committed file read at tip %q prefixes %v", tip, prefixes)
	}
	if f.filesErr != nil {
		return nil, f.filesErr
	}
	return f.files, nil
}

func (f *scanReadFixture) CommitTime(tip string) (time.Time, error) {
	f.call("commitTime")
	if tip != scanFixtureTip {
		f.t.Fatalf("commit time read at tip %q, want %q", tip, scanFixtureTip)
	}
	f.commitCalls++
	return f.committed, nil
}

func (f *scanReadFixture) setDraftFiles() {
	f.t.Helper()
	f.files = map[string][]byte{
		"plans/goals/backlog.md": goal.RenderRoot(&goal.RootRecord{
			Identity: scanFixtureLedger, FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1,
		}),
		"plans/goals/draft-here.md": goal.RenderFile(&goal.GoalFile{
			Id: "draft-here", State: goal.StateQueued, Intent: "Test the draft scan", Origin: goal.OriginMain,
			NextStep: "Have a human approve this draft.", OpenedAt: "2026-09-07T00:00:00Z", Revision: 1,
			History: []goal.HistoryLine{{
				At: "2026-09-07T00:00:00Z", Opid: goal.Opid(scanFixtureLedger, "bed-m1", "coordinator"),
				Verb: "open", Actor: "bed-m1+coordinator", Targets: []string{"draft-here"}, Keep: -1,
			}},
		}),
	}
}
