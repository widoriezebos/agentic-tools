package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

// All proof, incident, contract, tree and clock facts belong to this bed.
type standingBed struct {
	*scopeBed
	red               Result
	incidents         []goal.TrunkRedEntry
	launches, fetches int
	environment, fail string
}

func newStandingBed(t *testing.T, green bool) *standingBed {
	t.Helper()
	b := &standingBed{scopeBed: changedMainBed(t), environment: "image toolchain"}
	if !green {
		if err := os.Remove(resultsPath(b.install)); err != nil {
			t.Fatal(err)
		}
	}
	b.contract.Groups[0].Requires = nil
	b.contract.Groups[1].Inputs = []string{"metasystem/records/image/**"}
	b.contract.Groups[0].Inputs = []string{"metasystem/plans/**"}
	b.contract.Groups[2].Inputs = []string{"metasystem/records/shared/**"}
	pkg := b.contract.Groups[0]
	pkg.ID = "packages"
	pkg.Adapter = "go"
	pkg.Argv = nil
	pkg.Format = ""
	pkg.Packages = nil
	pkg.Tests = json.RawMessage(`"all"`)
	pkg.PackageSelection = "changed-and-consumers"
	pkg.CWD = "metasystem"
	pkg.Inputs = []string{"metasystem/internal/**"}
	b.contract.Groups = append(b.contract.Groups, pkg)
	b.red = Result{Trunk: true, Attempt: "standing-red", Commit: "red-commit", Tree: "red-tree", Scope: "full", Result: Red, Environment: b.environment, At: bedNow.Add(-5 * time.Minute).Format(time.RFC3339), CountedFull: true, Cause: &Cause{Kind: "main"}, Failed: []FailedUnit{{Unit: "plans"}, {Unit: "metasystem/cmd/metasystem", Tests: []string{"TestX"}}}}
	b.incidents = []goal.TrunkRedEntry{{ID: "plans-incident", Identity: "red:plans", Sightings: []goal.TrunkRedSighting{{BaseCommit: b.red.Commit}}}, {ID: "package-incident", Identity: "red:metasystem/cmd/metasystem:TestX", Sightings: []goal.TrunkRedSighting{{BaseCommit: b.red.Commit}}}}
	b.save(t, b.red)
	b.git.changed[[2]string{b.red.Tree, b.git.tree}] = "metasystem/records/shared/page.md"
	b.seams.Closure = func(string, string, string) (adapter.Closure, error) { return adapter.Closure{}, nil }
	git := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "fetch" {
			b.fetches++
			return "", nil
		}
		if args[0] == "ls-tree" {
			return "metasystem/cmd/metasystem/main.go\nmetasystem/internal/other/other.go", nil
		}
		return git(dir, args...)
	}
	b.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return b.incidents, nil }
	b.seams.Launch = func([]string, string, string) (int64, error) { b.launches++; return int64(os.Getpid()), nil }
	b.seams.Alive = func(r Running) bool { return !r.Checkpoint }
	b.seams.Command = func(cmd *exec.Cmd) error {
		b.calls = append(b.calls, cmd)
		fmt.Fprintln(cmd.Stdout, "landing environment "+b.environment)
		ids := strings.Fields(commandEnv(cmd, "LANDING_PROOF_GROUPS"))
		if commandEnv(cmd, "LANDING_PROOF_SCOPE") == "full" {
			ids = []string{"plans", "unrelated", "shared", "unit/cmd/metasystem"}
		}
		for _, id := range ids {
			fmt.Fprintf(cmd.Stdout, "landing group %s green 1\n", id)
		}
		if b.fail != "" {
			fmt.Fprintf(cmd.Stdout, "LANDING-FAILED\t%s\t\nLANDING-CHECKED\t1\n", b.fail)
			return errors.New("fixture failed")
		}
		fmt.Fprintln(cmd.Stdout, "LANDING-CHECKED\t0")
		return nil
	}
	b.setContract(t)
	return b
}

func (b *standingBed) detached(t *testing.T) Result {
	t.Helper()
	running, already, err := Start(b.install, b.checkout, b.seams)
	if err != nil || already {
		t.Fatalf("start: %+v %v already=%v", running, err, already)
	}
	// Pinned facts survive a detached child; selection must not consult them again.
	log, err := os.Create(running.Log)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	r, err := Run(b.install, b.checkout, "fixture-proof", running.Attempt, log, b.seams)
	if err != nil {
		t.Fatal(err)
	}
	if r.didNotRun() {
		data, _ := os.ReadFile(r.Log)
		t.Logf("proof log: %s", data)
	}
	return r
}

func (b *standingBed) provenance(t *testing.T, r Result, want map[string]string) {
	t.Helper()
	data, err := os.ReadFile(scopePath(b.install, r.Attempt))
	if err != nil {
		t.Fatal(err)
	}
	var scope scopeRecord
	if err := json.Unmarshal(data, &scope); err != nil {
		t.Fatal(err)
	}
	if len(scope.Groups) != len(want) {
		t.Fatalf("groups: %+v want %v", scope.Groups, want)
	}
	for _, g := range scope.Groups {
		if want[g.ID] != g.State {
			t.Fatalf("%s provenance %+v want %s", g.ID, g, want[g.ID])
		}
		if g.State == "standing" && g.From != b.red.Tree {
			t.Fatalf("standing source %+v", g)
		}
		if g.State == "kept" && g.From != b.base.FullTree {
			t.Fatalf("green source %+v", g)
		}
	}
	if r.CountedFull || r.OwedSince.Format(time.RFC3339) != b.red.At {
		t.Fatalf("credit or debt: %+v", r)
	}
	if r.FullTree == b.red.Tree || r.FullTree != "" && (r.FullTree != b.base.FullTree || r.FullAt != b.base.FullAt) {
		t.Fatalf("coverage from red: %+v", r)
	}
}

func TestStandingRedAffectedChecksAndCoverage(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"no green", "green", "green changed inputs", "green other environment", "green before code change", "plans affected", "prerequisite", "new failure"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := newStandingBed(t, mode != "no green" && mode != "new failure")
			state := "kept"
			if mode == "no green" || mode == "new failure" {
				state = "unproved"
			}
			if mode == "green changed inputs" || mode == "green before code change" {
				b.git.changed[[2]string{b.base.FullTree, b.git.tree}] = "metasystem/records/image/facts.json"
			}
			if mode == "green other environment" {
				b.base.Environment = "older toolchain"
				if err := os.Remove(resultsPath(b.install)); err != nil {
					t.Fatal(err)
				}
				b.save(t, b.base)
				b.save(t, b.red)
				state = "unproved"
			}
			if mode == "green before code change" {
				b.git.changed[[2]string{b.base.FullTree, b.git.tree}] = "metasystem/internal/other/other.go"
				b.contract.Groups[1].Inputs = append(b.contract.Groups[1].Inputs, "metasystem/internal/other/**")
				b.setContract(t)
			}
			if mode == "plans affected" || mode == "prerequisite" {
				b.git.changed[[2]string{b.red.Tree, b.git.tree}] = "metasystem/plans/designs/page.md"
			}
			if mode == "prerequisite" {
				b.contract.Groups[0].Requires = []string{"shared"}
				b.setContract(t)
			}
			sightings := 0
			b.seams.RecordMain = func(checks []Result) error {
				for _, c := range checks {
					if c.Commit != b.git.commit || c.Tree != b.git.tree {
						t.Fatalf("executed sighting has inherited coordinates %+v", c)
					}
					for _, f := range c.Failed {
						if f.Inherited || f.Unit != "shared" {
							t.Fatalf("carried sighting: %+v", checks)
						}
						sightings++
					}
				}
				return nil
			}
			if mode == "new failure" {
				b.fail = "shared"
			}
			r := b.detached(t)
			want := map[string]string{"plans": "standing", "shared": "ran", "unrelated": state, "unit/cmd/metasystem": "standing"}
			if mode == "green changed inputs" || mode == "green before code change" {
				want["unrelated"] = "unproved"
			}
			if mode == "plans affected" || mode == "prerequisite" {
				want["plans"] = "ran"
				want["shared"] = state
			}
			if mode == "prerequisite" {
				want["shared"] = "ran"
				if !strings.Contains(r.ScopeReason, "prerequisites repeated: shared") {
					t.Fatal(r.ScopeReason)
				}
			}
			b.provenance(t, r, want)
			if r.Result != Red || r.Scope != "scoped" || b.launches != 1 || len(b.calls) != 1 {
				t.Fatalf("result %+v launches=%d calls=%d", r, b.launches, len(b.calls))
			}
			carried := 2
			if mode == "plans affected" || mode == "prerequisite" {
				carried = 1
			}
			if len(r.Failed) != carried+map[bool]int{true: 1}[mode == "new failure"] {
				t.Fatalf("failures %+v", r.Failed)
			}
			for _, f := range r.Failed {
				if f.Unit != "shared" && (!f.Inherited || f.Attempt != b.red.Attempt) {
					t.Fatalf("lost inheritance %+v", f)
				}
			}
			if (sightings == 1) != (mode == "new failure") {
				t.Fatalf("sightings=%d", sightings)
			}
		})
	}
}

func TestStandingRedEmptySelectionObservesEnvironment(t *testing.T) {
	t.Parallel()
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprint(changed), func(t *testing.T) {
			t.Parallel()
			b := newStandingBed(t, false)
			b.git.changed[[2]string{b.red.Tree, b.git.tree}] = ""
			b.git.tree = b.red.Tree
			b.setContract(t)
			if changed {
				b.environment = "new toolchain"
			}
			r := b.detached(t)
			if changed {
				if r.Scope != "full" || len(b.calls) != 2 || !r.CountedFull {
					t.Fatalf("environment fallback %+v calls=%d", r, len(b.calls))
				}
				return
			}
			b.provenance(t, r, map[string]string{"plans": "standing", "shared": "unproved", "unrelated": "unproved", "unit/cmd/metasystem": "standing"})
			if r.Result != Red || len(r.Ran) != 0 || len(b.calls) != 1 || !strings.HasSuffix(r.ScopeReason, "nothing to run") {
				t.Fatalf("empty selection %+v calls=%d", r, len(b.calls))
			}
			if err := RecoverMovedMain(b.install, b.checkout, b.seams); err != nil {
				t.Fatal(err)
			}
			if b.launches != 1 {
				t.Fatal("exact inherited red restarted")
			}
		})
	}
}

func TestStandingRedIdentityClosuresAndPendingClassification(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"one closed unit", "one closed sibling", "green closed sibling", "pending", "all closed", "unknown", "unclassified"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := newStandingBed(t, false)
			if mode == "one closed sibling" || mode == "green closed sibling" {
				b.red.Failed[1].Tests = append(b.red.Failed[1].Tests, "TestClosed")
				b.incidents = append(b.incidents, goal.TrunkRedEntry{ID: "closed", Identity: "red:metasystem/cmd/metasystem:TestClosed", Closed: &goal.TrunkRedClosure{How: "hand"}})
			}
			if mode == "green closed sibling" {
				b.incidents[len(b.incidents)-1].Closed.How = "green"
			}
			if mode == "one closed unit" || mode == "all closed" {
				b.incidents[1].Closed = &goal.TrunkRedClosure{How: "hand"}
			}
			if mode == "all closed" {
				b.incidents[0].Closed = &goal.TrunkRedClosure{How: "hand"}
			}
			if mode == "pending" {
				b.red.ClassificationPending = true
			}
			if mode == "unclassified" {
				b.red.Cause.Kind = "unclassified"
			}
			if mode == "unknown" {
				b.red.Failed[0].Unit = "unknown"
				b.incidents[0].Identity = "red:unknown"
			}
			b.save(t, b.red)
			r := b.detached(t)
			if mode == "one closed unit" {
				if len(r.Failed) != 1 || r.Failed[0].Unit != "plans" {
					t.Fatalf("closed unit carried %+v", r)
				}
				return
			}
			if mode == "one closed sibling" || mode == "green closed sibling" {
				if len(r.Failed) != 2 || !slices.Equal(r.Failed[1].Tests, []string{"TestX"}) {
					t.Fatalf("closed sibling carried %+v", r)
				}
				return
			}
			if r.Scope != "full" {
				t.Fatalf("ineligible red %+v", r)
			}
			if mode == "unknown" && r.ScopeReason != "a standing failure cannot be resolved to a check: unknown" {
				t.Fatal(r.ScopeReason)
			}
		})
	}
}

func TestStandingRedFullFallbacks(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"code", "code no green", "tree code", "contract", "uncovered", "partly uncovered", "template", "closure", "fresh", "classification"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := newStandingBed(t, mode != "code no green")
			switch mode {
			case "code", "code no green":
				b.git.changed[[2]string{b.red.Commit, b.git.commit}] = "metasystem/internal/other/code.go"
				b.git.changed[[2]string{b.red.Tree, b.git.tree}] = "metasystem/internal/other/code.go"
			case "tree code":
				b.git.changed[[2]string{b.red.Tree, b.git.tree}] = "metasystem/internal/other/code.go"
			case "contract":
				b.git.changed[[2]string{b.red.Tree, b.git.tree}] = "metasystem/testing.json"
			case "uncovered":
				b.git.changed[[2]string{b.red.Tree, b.git.tree}] = "metasystem/memory/uncovered.txt"
			case "partly uncovered":
				b.git.changed[[2]string{b.red.Tree, b.git.tree}] += "\nmetasystem/memory/uncovered.txt"
			case "template":
				b.git.changed[[2]string{b.red.Tree, b.git.tree}] = "metasystem/internal/other/page.md"
			case "closure":
				b.seams.Closure = func(string, string, string) (adapter.Closure, error) {
					return adapter.Closure{}, errors.New("unreadable")
				}
			case "fresh":
				b.seams.Fresh = true
			case "classification":
				b.seams.ClassificationOf = "authorised"
			}
			r := b.detached(t)
			if r.Scope != "full" || len(b.calls) != 1 || !r.CountedFull {
				t.Fatalf("fallback %+v calls=%d", r, len(b.calls))
			}
			if mode == "tree code" && !strings.HasPrefix(r.ScopeReason, "an executable input changed:") {
				t.Fatal(r.ScopeReason)
			}
			if (mode == "uncovered" || mode == "partly uncovered") && !strings.Contains(r.ScopeReason, "no declared test inputs") {
				t.Fatal(r.ScopeReason)
			}
		})
	}
}

func TestStandingFailureResolution(t *testing.T) {
	t.Parallel()
	b := newStandingBed(t, false)
	named := b.contract.Groups[0]
	named.ID = "agent-protocol-standard"
	named.Adapter = "go"
	named.Packages = []string{"cmd/metasystem"}
	named.Tests = json.RawMessage(`["TestProtocolCheckersRefuseMissingArgumentsAsUsage"]`)
	b.contract.Groups = append(b.contract.Groups, named)
	for _, row := range []struct {
		unit FailedUnit
		want string
		ok   bool
	}{{FailedUnit{Unit: "plans"}, "plans", true}, {FailedUnit{Unit: "metasystem/cmd/metasystem", Tests: []string{"TestProtocolCheckersRefuseMissingArgumentsAsUsage"}}, "agent-protocol-standard", true}, {FailedUnit{Unit: "metasystem/cmd/metasystem", Tests: []string{"TestProtocolCheckersRefuseMissingArgumentsAsUsage", "TestOutsideSelection"}}, "agent-protocol-standard unit/cmd/metasystem", true},
		{FailedUnit{Unit: "metasystem/cmd/metasystem", Tests: []string{"TestProtocolCheckersRefuseMissingArgumentsAsUsage/subcase"}}, "agent-protocol-standard", true},
		{FailedUnit{Unit: "metasystem/internal/other", Tests: []string{"TestOther"}}, "unit/internal/other", true}, {FailedUnit{Unit: "unknown"}, "", false}, {FailedUnit{Unit: "metasystem/absent"}, "", false}} {
		ids, ok := resolveFailure(b.contract, row.unit, []string{"cmd/metasystem", "internal/other"})
		if ok != row.ok || strings.Join(ids, " ") != row.want {
			t.Fatalf("resolve %+v = %v %v", row.unit, ids, ok)
		}
	}
}

func TestStandingRedPreservesHourlyDebtAcrossTwoMoves(t *testing.T) {
	t.Parallel()
	b := newStandingBed(t, false)
	if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("testing.contract=testing.json\nproof.trunk-every=4h\n"), 0600); err != nil {
		t.Fatal(err)
	}
	first := b.detached(t)
	b.git.commit, b.git.tree = "next-record", "next-tree"
	b.git.changed[[2]string{first.Tree, b.git.tree}] = "metasystem/records/shared/next.md"
	b.setContract(t)
	b.seams.NewID = func() string { return "next-attempt" }
	b.seams.Now = func() time.Time { return bedNow.Add(10 * time.Minute) }
	second := b.detached(t)
	if second.OwedSince != first.OwedSince || len(second.Ran) != 1 || second.Ran[0] != "shared" {
		t.Fatalf("renewed debt or repeated failures %+v", second)
	}
	b.seams.Now = func() time.Time { return bedNow.Add(time.Hour) }
	b.seams.NewID = func() string { return "hourly" }
	if due, err := scopedProofDue(b.install, b.seams.now()); err != nil || !due {
		t.Fatalf("no green lost hourly debt %v %v", due, err)
	}
	b.fail = "plans"
	hold, err := KeeperClock(lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1}, func(lane.Record) error { return nil }, b.seams)
	if err != nil || !strings.Contains(hold, "hourly debt") {
		t.Fatalf("clock %s %v", hold, err)
	}
	running, _, _, err := ReadRunning(b.install, b.seams)
	if err != nil || running.Admission.Scope != "full" || !running.Fresh {
		t.Fatalf("hourly admission %+v %v", running, err)
	}
	log, err := os.Create(running.Log)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	r, err := Run(b.install, b.checkout, "fixture-proof", running.Attempt, log, b.seams)
	if err != nil || !r.CountedFull || r.Result != Red {
		t.Fatalf("full red %+v %v", r, err)
	}
	due, err := scopedProofDue(b.install, b.seams.now())
	if err != nil || due {
		t.Fatalf("executed counted red did not discharge debt: %v %v", due, err)
	}
	before := b.launches
	_, err = KeeperClock(lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1}, func(lane.Record) error { return nil }, b.seams)
	if err != nil || b.launches != before {
		t.Fatalf("counted red did not discharge debt launches=%d %v", b.launches, err)
	}
}

func TestStandingRedMissingCoverageAndEnvironment(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"missing environment", "first partial", "expired green"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := newStandingBed(t, false)
			if mode == "missing environment" {
				b.red.Environment = ""
				b.git.tree = b.red.Tree
				b.setContract(t)
			}
			if mode == "first partial" {
				b.red.Scope = "scoped"
				b.red.CountedFull = false
			}
			if mode == "expired green" {
				if err := os.Remove(resultsPath(b.install)); err != nil {
					t.Fatal(err)
				}
				b.base.FullAt = bedNow.Add(-2 * time.Hour).Format(time.RFC3339)
				b.save(t, b.base)
			}
			b.save(t, b.red)
			r := b.detached(t)
			if mode == "missing environment" {
				if r.Scope != "full" || len(b.calls) != 2 || !r.CountedFull {
					t.Fatalf("missing environment inherited %+v", r)
				}
				return
			}
			b.provenance(t, r, map[string]string{"plans": "standing", "shared": "ran", "unrelated": "unproved", "unit/cmd/metasystem": "standing"})
			if r.FullAt != "" || r.FullTree != "" || r.OwedSince.Format(time.RFC3339) != b.red.At {
				t.Fatalf("missing coverage lost original debt %+v", r)
			}
		})
	}
}

func TestStandingRedDebtRequiresExecutedTrunkCheck(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"candidate red", "candidate green", "trunk red", "trunk green", "partial red", "nonrun"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := newStandingBed(t, false)
			origin := b.detached(t)
			paid := Result{Trunk: !strings.HasPrefix(mode, "candidate"), Attempt: "payment", Commit: "payment", Tree: "payment", Scope: "full", Result: Red, CountedFull: true, Cause: &Cause{Kind: "main"}, At: bedNow.Add(55 * time.Minute).Format(time.RFC3339)}
			if strings.HasSuffix(mode, "green") {
				paid.Result = Green
			}
			if mode == "partial red" {
				paid.Scope = "scoped"
				paid.OwedSince = origin.OwedSince
				paid.CountedFull = false
			}
			if mode == "nonrun" {
				paid.Cause = &Cause{Kind: "environment"}
				paid.CountedFull = false
			}
			b.save(t, paid)
			due, err := scopedProofDue(b.install, bedNow.Add(time.Hour))
			if err != nil || due != (mode != "trunk red" && mode != "trunk green") {
				t.Fatalf("debt paid by %s: due=%v %v", mode, due, err)
			}
		})
	}
}

func TestStandingRedCountedCandidateDoesNotPayScopedPushDebt(t *testing.T) {
	t.Parallel()
	b := newStandingBed(t, false)
	scoped := b.detached(t)
	scoped.Trunk = false
	b.save(t, scoped)
	writePushRecords(t, b.install, Pushed{Tree: scoped.Tree, At: scoped.At})
	b.save(t, Result{Attempt: "candidate-red", Tree: "candidate-tree", Scope: "full", Result: Red, CountedFull: true, Cause: &Cause{Kind: "main"}, At: bedNow.Add(55 * time.Minute).Format(time.RFC3339)})
	due, err := scopedProofDue(b.install, bedNow.Add(time.Hour))
	if err != nil || !due {
		t.Fatalf("candidate red paid main's scoped-push debt: %v %v", due, err)
	}
}
