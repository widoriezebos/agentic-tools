package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/metrics"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
)

// goalCLIBed is the Git-free successor of the goal CLI shell bed: one
// physical checkout, an in-memory accepted ledger (testgoal.Repository) and a
// clock the test moves by hand. It seeds the ledger the shell bed's migration
// produced: ship-widget claimed by fixture-machine/fixture-lineage, fix-docs
// queued, perf-pass parked and port-engine done. The goal owners are the real
// ones; public commands run through the public router with injected streams.
type goalCLIBed struct {
	t       *testing.T
	root    string
	repo    *testgoal.Repository
	mu      sync.Mutex
	now     time.Time
	machine string
	lineage string
	remote  string
	prove   goalAuthorityProver
	caller  ownercall.Process
}

// goalCLISeedNow is the clock the seed claim was taken at.
var goalCLISeedNow = time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)

type goalCLISeed struct {
	// remote is the endpoint's sync remote; "origin" (two-machine mode) by default.
	remote string
	// config is appended to metasystem.conf after metasystem.runtimes=fake.
	config string
	// amend edits the seeded goals before they are rendered.
	amend func(map[string]*goal.GoalFile)
	// root edits the seeded root record before it is rendered.
	rootRecord func(*goal.RootRecord)
	// noEnrollment leaves the checkout without the local terminal enrollment
	// (by default Wido is enrolled, as the fixture-human acts need a name).
	noEnrollment bool
}

func newGoalCLIBed(t *testing.T, seed goalCLISeed) *goalCLIBed {
	t.Helper()
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	remote := seed.remote
	if remote == "" {
		remote = "origin"
	}
	at := goalCLISeedNow.Format(time.RFC3339)
	opid := func(ulid string) string { return goal.Opid(ulid, "fixture-machine", "fixture-lineage") }
	goals := map[string]*goal.GoalFile{
		"ship-widget": {
			Id: "ship-widget", State: goal.StateClaimed, Intent: "Ship the widget end to end", Origin: goal.OriginHuman,
			NextStep: "Wire the widget into the release train.", OpenedAt: at, Revision: 2,
			Claimed: &goal.ClaimRecord{Machine: "fixture-machine", Lineage: "fixture-lineage", At: at, Revision: 2},
			History: []goal.HistoryLine{
				{At: at, Opid: opid("01ARZ3NDEKTSV4RRFFQ69G5FA1"), Verb: "migrate", Actor: "fixture-machine+fixture-lineage", Targets: []string{"ship-widget"}, Keep: -1},
				{At: at, Opid: opid("01ARZ3NDEKTSV4RRFFQ69G5FA2"), Verb: "claim", Actor: "fixture-machine+fixture-lineage", Targets: []string{"ship-widget"}, Keep: -1},
			},
		},
		"fix-docs": {
			Id: "fix-docs", State: goal.StateQueued, Intent: "Bring the docs current", Origin: goal.OriginMain,
			NextStep: "The amended next step.", OpenedAt: at, Revision: 1,
			History: []goal.HistoryLine{
				{At: at, Opid: opid("01ARZ3NDEKTSV4RRFFQ69G5FA3"), Verb: "migrate", Actor: "fixture-machine+fixture-lineage", Targets: []string{"fix-docs"}, Keep: -1},
			},
		},
		"perf-pass": {
			Id: "perf-pass", State: goal.StateParked, Intent: "Cut p99 latency in half", Origin: goal.OriginMain,
			NextStep: "Re-profile once the vendor ships.", OpenedAt: at, Revision: 1,
			Parked: &goal.ParkRecord{By: "human:Wido", At: at, Because: "Blocked on the vendor's profiler fix."},
			History: []goal.HistoryLine{
				{At: at, Opid: opid("01ARZ3NDEKTSV4RRFFQ69G5FA4"), Verb: "migrate", Actor: "fixture-machine+fixture-lineage", Targets: []string{"perf-pass"}, Keep: -1},
			},
		},
		"port-engine": {
			Id: "port-engine", State: goal.StateDone, Intent: "Port the engine to Go", Origin: goal.OriginHuman,
			NextStep: "Landed and gated on both hosts.", Conclude: "Landed and gated on both hosts.", OpenedAt: at, Revision: 1,
			History: []goal.HistoryLine{
				{At: at, Opid: opid("01ARZ3NDEKTSV4RRFFQ69G5FA5"), Verb: "migrate", Actor: "fixture-machine+fixture-lineage", Targets: []string{"port-engine"}, Keep: -1},
			},
		},
	}
	if seed.amend != nil {
		seed.amend(goals)
	}
	record := &goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncRemote,
		MigrationEpoch: "2026-08-20T00:00:00Z", MigrationMode: "manifest", Revision: 1,
	}
	if remote == "local" {
		record.SyncMode = goal.SyncLocal
	}
	if seed.rootRecord != nil {
		seed.rootRecord(record)
	}
	files := map[string][]byte{
		"metasystem.conf":        []byte("metasystem.runtimes=fake\n" + seed.config),
		"plans/goals/backlog.md": goal.RenderRoot(record),
	}
	for id, file := range goals {
		if file == nil {
			continue
		}
		path := "plans/goals/" + id + ".md"
		if file.State == goal.StateDone || file.State == goal.StateAbandoned {
			path = "records/goals/" + id + ".md"
		}
		files[path] = goal.RenderFile(file)
	}
	for path, data := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		write, mode := os.WriteFile, os.FileMode(0o644)
		if strings.HasSuffix(path, ".sh") {
			write, mode = testexec.WriteFile, 0o755
		}
		if err := write(full, data, mode); err != nil {
			t.Fatal(err)
		}
	}
	// scripts/agents marks the installation for the state root; the bed
	// once planted the retired pre-commit-guard.sh there (U5 moved the guard
	// into the engine).
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if marker, err := os.OpenFile(filepath.Join(root, "metasystem.conf"), os.O_CREATE|os.O_WRONLY, 0o644); err != nil {
		t.Fatal(err)
	} else {
		marker.Close()
	}
	if !seed.noEnrollment {
		writeFixtureEnrollment(t, root, "Wido")
	}
	bed := &goalCLIBed{
		t: t, root: root, now: goalCLISeedNow.Add(time.Minute), machine: "fixture-machine", lineage: "fixture-lineage",
		remote: remote, prove: fixedFixtureGoalAuthority, caller: ownercall.EntryCaller(),
		repo: testgoal.New(files, goalCLISeedNow, "0000000000000000000000000000000000000001"),
	}
	return bed
}

// announceHolder records this test process as the checkout's lease holder
// under the bed's lineage, as the shell bed's lease announce of its own shell
// did, so claim-bearing acts carry a real claim epoch.
func (b *goalCLIBed) announceHolder() {
	b.t.Helper()
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		b.t.Fatalf("probe the test process: state=%s err=%v", state, err)
	}
	if _, err := lease.AnnounceWithPair(b.root, "goal-cli-fixture", exact.Pid, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID,
		"goal-cli-fixture", "fake", b.lineage); err != nil {
		b.t.Fatal(err)
	}
	b.caller = ownercall.Process{Pid: exact.Pid, StartedAt: exact.StartedAt.Unix()}
}

// setNow moves the bed's clock; every later command reads it.
func (b *goalCLIBed) setNow(at time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.now = at.UTC()
}

func (b *goalCLIBed) clock() time.Time {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.now
}

func (b *goalCLIBed) commandNow(string) (time.Time, error) { return b.clock(), nil }

func (b *goalCLIBed) endpoint(root string) (goal.Endpoint, error) {
	branch := "refs/heads/main"
	if b.remote == "local" {
		branch = goal.LocalLedgerBranch
	}
	return goal.Endpoint{Root: root, Remote: b.remote, Branch: branch, Repository: b.repo}, nil
}

// dependencies are the owners' request facts for this bed, printing on the
// given streams.
func (b *goalCLIBed) dependencies(stdout, stderr *bytes.Buffer) syncRequestDependencies {
	return syncRequestDependencies{
		authorityFacts: goalAuthorityReadFacts{
			caller:         b.caller,
			repositoryTop:  func(string) (string, error) { return b.root, nil },
			ledgerIdentity: func(string) string { return "01ARZ3NDEKTSV4RRFFQ69G5FAV" },
		},
		endpoint:     b.endpoint,
		machine:      func(string) (string, error) { return b.machine, nil },
		ensureGuard:  func(string) error { return nil },
		ownerLineage: func() string { return b.lineage },
		// A person's act in this bed is proven as the shell bed's
		// --fixture-human-authority was: a fixture-only proof for the root.
		proveHuman: func(root string, pid int64, reader humanauthority.Reader, now time.Time) (humanauthority.Proof, error) {
			return b.prove(root, pid, reader, "", "", now)
		},
		proveTerminal: func(string, int64, humanauthority.Reader, time.Time) (humanauthority.Proof, error) {
			b.t.Fatal("goal CLI bed: unexpected terminal proof")
			return humanauthority.Proof{}, nil
		},
		presence: func(string, goal.Endpoint) (seat.Copy, error) { return seat.Copy{}, nil },
		stdout:   stdout, stderr: stderr,
	}
}

func (b *goalCLIBed) owners(stdout, stderr *bytes.Buffer) intentOwners {
	return intentOwners{
		resolver:     stateroot.NewResolver(fakeTop(b.root), noExecutable),
		prove:        b.prove,
		commandNow:   b.commandNow,
		dependencies: b.dependencies(stdout, stderr),
		binding:      nil,
		parkBranchCheck: func(string, goal.Endpoint) func(string, string) (string, error) {
			return func(string, string) (string, error) { return "", nil }
		},
		completion: completionInputs{
			localTip: func(string, string) (string, bool, error) { return "", false, nil },
			reporter: func(metrics.Options) (metrics.Result, error) { return metrics.Result{}, nil },
		},
	}
}

// public runs one public command (for example "goal", "approve", "fix-docs")
// from the checkout and returns its exit code and streams.
func (b *goalCLIBed) public(args ...string) (int, string, string) {
	b.t.Helper()
	command, rest, ok := resolveIntentArgv(args)
	if !ok {
		b.t.Fatalf("no public command %q", args)
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, rest, &stdout, &stderr, b.root, b.owners(&stdout, &stderr))
	return code, stdout.String(), stderr.String()
}

// owner runs one in-process owner runner with this bed's dependencies.
func (b *goalCLIBed) owner(run func(syncRequestDependencies) int) (int, string, string) {
	b.t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(b.dependencies(&stdout, &stderr))
	return code, stdout.String(), stderr.String()
}

// accepted returns the accepted ledger's bytes at path ("" when absent).
func (b *goalCLIBed) accepted(path string) string {
	b.t.Helper()
	tip, _, err := b.repo.Accepted()
	if err != nil {
		b.t.Fatal(err)
	}
	files, err := b.repo.Files(tip, path)
	if err != nil {
		b.t.Fatal(err)
	}
	return string(files[path])
}

// goalRecord is the accepted live record of id, or its archive when concluded.
func (b *goalCLIBed) goalRecord(id string) string {
	b.t.Helper()
	if text := b.accepted("plans/goals/" + id + ".md"); text != "" {
		return text
	}
	return b.accepted("records/goals/" + id + ".md")
}

func (b *goalCLIBed) tip() string {
	b.t.Helper()
	tip, _, err := b.repo.Accepted()
	if err != nil {
		b.t.Fatal(err)
	}
	return tip
}

// line returns the record's first line with prefix, or "".
func goalCLILine(record, prefix string) string {
	for _, line := range strings.Split(record, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}
