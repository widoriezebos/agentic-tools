package plain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

type flakeMoveBed struct {
	*keeperClockBed
	at         time.Time
	main       string
	trees      map[string]map[string]string
	argv       [][]string
	unreadable bool
}

func newFlakeMoveBed(t *testing.T) *flakeMoveBed {
	t.Helper()
	b := &flakeMoveBed{keeperClockBed: newKeeperClockBed(t), at: bedNow, main: "a", trees: map[string]map[string]string{}}
	b.batch = nil
	for _, tree := range []string{"a", "b", "c"} {
		b.trees[tree] = map[string]string{}
		for _, name := range []string{"trunk-red.json", "tests-observe-only-what-they-inject-2.md"} {
			path := "plans/goals/" + name
			data, err := os.ReadFile(filepath.Join("testdata", "flake-record", tree, name))
			if err != nil {
				t.Fatal(err)
			}
			b.trees[tree]["metasystem/"+path] = string(data)
		}
	}
	b.base = Result{Trunk: true, Commit: "commit-a", Tree: "tree-a", Result: Green, Scope: "full", FullTree: "tree-a", At: bedNow.Format(time.RFC3339), FullAt: bedNow.Format(time.RFC3339), Attempt: "full-a", Environment: "image toolchain", CountedFull: true}
	b.save(t, b.base)
	b.seams.Trunk = true
	b.seams.Now = func() time.Time { return b.at }
	ids := 0
	b.seams.NewID = func() string { ids++; return fmt.Sprintf("attempt-%d", ids) }
	b.seams.Alive = func(Running) bool { return false }
	const child = int64(1 << 29)
	ownerRef, childRef := syntheticTrunkProcess(t, int64(os.Getpid())), syntheticTrunkProcess(t, child)
	b.seams.ProcessRef = func(pid int64) string {
		if pid == int64(os.Getpid()) {
			return ownerRef
		}
		if pid == child {
			return childRef
		}
		t.Fatalf("unexpected process identity: %d", pid)
		return ""
	}
	b.seams.Launch = func(argv []string, _, _ string) (int64, error) {
		b.trunks++
		b.argv = append(b.argv, slices.Clone(argv))
		return child, nil
	}
	b.seams.Git = func(dir string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if len(args) > 0 && args[0] == "fetch" {
			return "", nil
		}
		if joined == "rev-parse --verify origin/main^{commit}" {
			return "commit-" + b.main, nil
		}
		if len(args) == 3 && args[0] == "rev-parse" && strings.HasSuffix(args[2], "^{tree}") {
			return "tree-" + strings.TrimSuffix(strings.TrimPrefix(args[2], "commit-"), "^{tree}"), nil
		}
		if len(args) == 3 && args[0] == "cat-file" {
			return "", nil
		}
		if len(args) == 4 && args[0] == "merge-base" {
			if args[2] == "commit-unrelated" {
				return "", errors.New("unrelated commit")
			}
			return "", nil
		}
		if len(args) == 5 && args[0] == "diff" {
			from, to := strings.TrimPrefix(args[3], "tree-"), strings.TrimPrefix(args[4], "tree-")
			if from == "origin/main" {
				from = b.main
			}
			paths := map[string]bool{}
			for path, old := range b.trees[from] {
				if b.trees[to][path] != old {
					paths[path] = true
				}
			}
			for path, data := range b.trees[to] {
				if b.trees[from][path] != data {
					paths[path] = true
				}
			}
			changed := []string{}
			for path := range paths {
				changed = append(changed, path)
			}
			sort.Strings(changed)
			return strings.Join(changed, "\n"), nil
		}
		if len(args) == 2 && args[0] == "show" {
			tree, path, _ := strings.Cut(args[1], ":")
			if b.unreadable && strings.HasSuffix(path, ".md") {
				return "", errors.New("unreadable goal")
			}
			if path == "metasystem/testing.json" {
				return b.git.shows[b.git.tree+":metasystem/testing.json"], nil
			}
			data, ok := b.trees[strings.TrimPrefix(tree, "tree-")][path]
			if !ok {
				return "", errors.New("missing file")
			}
			return data, nil
		}
		return b.git.run(dir, args...)
	}
	b.seams.Closure = func(string, string, string) (adapter.Closure, error) {
		return adapter.Closure{Unowned: []string{"metasystem/plans/goals/trunk-red.json"}}, nil
	}
	b.restart()
	return b
}

func (b *flakeMoveBed) restart() {
	previous := b.keeper
	b.keeper = lane.AgentKeeper{Home: previous.Home, Self: previous.Self, Prepare: previous.Prepare, Holds: previous.Holds, Running: previous.Running, Start: previous.Start,
		Now:     func() time.Time { return b.at },
		Observe: func(r lane.Record) error { return RecoverMovedMain(r.Install, r.Root, b.seams) },
		Clock: func(r lane.Record, prepare func(lane.Record) error) (string, error) {
			return KeeperClock(r, prepare, b.seams)
		},
		Sources: lane.WakeSources{Reasons: func(string) ([]string, error) { return WakeReasons(b.install, b.checkout, time.Time{}, b.at, b.seams) }},
	}
}

func (b *flakeMoveBed) move(tree string, age time.Duration) { b.main, b.at = tree, bedNow.Add(age) }
func (b *flakeMoveBed) rows(t *testing.T) []Result {
	t.Helper()
	rows, err := Results(b.install)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
func (b *flakeMoveBed) inherited(t *testing.T, tree string, want int) {
	t.Helper()
	count := 0
	for _, r := range b.rows(t) {
		if r.Scope == "inherited" && r.ScopeReason == flakeRecordsReason && r.Tree == "tree-"+tree {
			count++
			if r.Result != Green || !r.Trunk || r.FullTree != "tree-a" || r.FullAt != b.base.FullAt || r.Environment != b.base.Environment || r.Commit != "commit-"+tree || !strings.HasPrefix(r.Reason, "inherits green from tree ") {
				t.Fatalf("inherited row lost full-proof facts: %+v", r)
			}
		}
	}
	if count != want {
		t.Fatalf("inherited rows for %s = %d, want %d; rows=%+v", tree, count, want, b.rows(t))
	}
}
func (b *flakeMoveBed) checkWake(t *testing.T, launches int, due bool) {
	t.Helper()
	b.wake(t)
	reasons, err := WakeReasons(b.install, b.checkout, time.Time{}, b.at, b.seams)
	if err != nil || b.trunks != launches || slices.Contains(reasons, WakeFullDue) != due {
		t.Fatalf("at %s: launches=%d want %d; wake reasons=%v want full-due=%v; error=%v", b.at.Sub(bedNow), b.trunks, launches, reasons, due, err)
	}
}

func TestFlakeRecordMoveKeepsMainsFullGreen(t *testing.T) {
	t.Parallel()
	cases := []string{"flake-record", "stacked", "design-edit-flake-fix", "code-change", "later-red", "did-not-run-red", "unreadable", "person", "person-scoped-then-flake", "newer-environment", "missing-environment", "legacy-inheritance-evidence", "scoped-push-then-flake", "missing-full-at", "malformed-full-at", "future-full-at", "future-marked-row", "helpers-refuse-future", "legacy-row", "flake-after-design-edit", "later-red-after-inheritance"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newFlakeMoveBed(t)
			b.move("b", 5*time.Minute)
			switch name {
			case "missing-full-at", "malformed-full-at", "future-full-at":
				switch name {
				case "missing-full-at":
					b.base.FullAt = ""
				case "malformed-full-at":
					b.base.FullAt = "not a time"
				case "future-full-at":
					b.base.FullAt = bedNow.Add(time.Hour).Format(time.RFC3339)
				}
				if err := os.Remove(resultsPath(b.install)); err != nil {
					t.Fatal(err)
				}
				b.save(t, b.base)
			case "newer-environment", "missing-environment", "legacy-inheritance-evidence":
				r := b.base
				r.Commit, r.Tree, r.Scope, r.FullTree = "commit-unrelated", "tree-unrelated", "scoped", "tree-a"
				r.Environment = "other image"
				if name == "missing-environment" {
					r.Environment = ""
				}
				if name == "legacy-inheritance-evidence" {
					r.Scope, r.Reason = "inherited", "inherits green from tree old"
				}
				b.save(t, r)
			case "later-red", "did-not-run-red":
				r := b.base
				r.Result = Red
				r.Cause = &Cause{Kind: "main"}
				r.Attempt = "later-red"
				r.At = b.at.Format(time.RFC3339)
				if name == "did-not-run-red" {
					r.Cause.Kind = "environment"
					r.CountedFull = false
				}
				b.save(t, r)
			case "unreadable":
				b.unreadable = true
			case "code-change":
				b.trees["b"]["metasystem/internal/x/x.go"] = "package x"
			case "design-edit-flake-fix", "flake-after-design-edit":
				old := "metasystem/plans/goals/tests-observe-only-what-they-inject-2.md"
				path := "metasystem/plans/goals/flake-fix-metasystem-internal-x-testy.md"
				for _, tree := range []string{"a", "b", "c"} {
					f, problems := goal.ParseFile([]byte(b.trees[tree][old]))
					if len(problems) != 0 {
						t.Fatal(problems)
					}
					f.Id = "flake-fix-metasystem-internal-x-testy"
					for i := range f.History {
						f.History[i].Targets = []string{f.Id}
					}
					if tree != "a" {
						f.Intent = "Change the design."
						f.Revision++
						f.History = append(f.History, goal.HistoryLine{At: b.at.Format(time.RFC3339), Opid: f.History[0].Opid, Actor: "lane+lin-1", Verb: "edit", Targets: []string{f.Id}, Keep: -1})
					}
					b.trees[tree][path] = strings.TrimSpace(string(goal.RenderFile(f)))
					delete(b.trees[tree], old)
					entries, problems := goal.ParseTrunkRed([]byte(b.trees[tree]["metasystem/plans/goals/trunk-red.json"]))
					if len(problems) != 0 {
						t.Fatal(problems)
					}
					for i := range entries {
						entries[i].FixGoal = f.Id
					}
					b.trees[tree]["metasystem/plans/goals/trunk-red.json"] = strings.TrimSpace(string(goal.RenderTrunkRed(entries)))
				}
				if name == "flake-after-design-edit" {
					path := "metasystem/plans/goals/flake-fix-metasystem-internal-x-testy.md"
					a, _ := goal.ParseFile([]byte(b.trees["a"][path]))
					c, _ := goal.ParseFile([]byte(b.trees["b"][path]))
					edit := c.History[len(c.History)-1]
					base := *a
					base.Intent, base.Revision = c.Intent, a.Revision+1
					base.History = append(slices.Clone(a.History), edit)
					b.trees["b"][path] = strings.TrimSpace(string(goal.RenderFile(&base)))
					c.Revision = base.Revision + 1
					c.History = append(slices.Clone(base.History), c.History[len(a.History)])
					b.trees["c"][path] = strings.TrimSpace(string(goal.RenderFile(c)))
					b.trees["b"]["metasystem/plans/goals/trunk-red.json"] = b.trees["a"]["metasystem/plans/goals/trunk-red.json"]
					entries, _ := goal.ParseTrunkRed([]byte(b.trees["c"]["metasystem/plans/goals/trunk-red.json"]))
					entries[0].Sightings = entries[0].Sightings[:2]
					b.trees["c"]["metasystem/plans/goals/trunk-red.json"] = strings.TrimSpace(string(goal.RenderTrunkRed(entries)))
				}

			case "future-marked-row", "helpers-refuse-future", "legacy-row":
				r := b.base
				r.Commit, r.Tree, r.Scope, r.ScopeReason, r.Reason = "commit-b", "tree-b", "inherited", flakeRecordsReason, "inherits green from tree old"
				r.FullAt = bedNow.Add(time.Hour).Format(time.RFC3339)
				if name == "legacy-row" {
					r.ScopeReason = "legacy"
					r.FullAt = bedNow.Add(-2 * time.Hour).Format(time.RFC3339)
				}
				if name == "helpers-refuse-future" {
					r.Reason = flakeRecordsReason
				}
				b.save(t, r)
				if name == "helpers-refuse-future" {
					if _, ok := applicableMainProof(b.install, b.checkout, r.Commit, r.Tree, b.seams, true); ok {
						t.Fatal("applicableMainProof reused a future full time")
					}
					if r.fullCurrent(b.at) {
						t.Fatal("fullCurrent accepted a future full time")
					}
					b.at = bedNow.Add(5 * time.Hour)
					if _, ok := applicableMainProof(b.install, b.checkout, r.Commit, r.Tree, b.seams, true); !ok {
						t.Fatal("applicableMainProof refused four-hour-old inheritance")
					}
					if !r.fullCurrent(b.at) {
						t.Fatal("fullCurrent refused four-hour-old inheritance")
					}
					return
				}
			}
			if name == "person" || name == "person-scoped-then-flake" || name == "flake-after-design-edit" {
				b.seams.Person = &ActProvenance{Person: "Wido"}
				running, already, err := Start(b.install, b.checkout, b.seams)
				if err != nil || already || b.trunks != 1 || running.Reused != nil {
					t.Fatalf("person admission launches=%d already=%v running=%+v error=%v", b.trunks, already, running, err)
				}
				b.inherited(t, "b", 0)
				if name == "person" {
					return
				}
				if running.Admission.Scope != "scoped" {
					t.Fatalf("person check scope=%s, want scoped", running.Admission.Scope)
				}
				r := b.base
				r.Commit, r.Tree, r.Scope, r.At = "commit-b", "tree-b", "scoped", b.at.Format(time.RFC3339)
				b.save(t, r)
				b.seams.Person = nil
				b.move("c", 30*time.Minute)
				if name == "flake-after-design-edit" {
					b.at = bedNow.Add(10 * time.Minute)
					b.checkWake(t, 2, false)
					b.inherited(t, "c", 0)
					return
				}
				b.checkWake(t, 1, false)
				b.inherited(t, "c", 1)
				for _, age := range []time.Duration{time.Hour + time.Second, 3*time.Hour + 59*time.Minute} {
					b.at = bedNow.Add(age)
					b.checkWake(t, 1, false)
				}
				b.at = bedNow.Add(4 * time.Hour)
				b.checkWake(t, 2, true)
				return
			}
			if name == "scoped-push-then-flake" {
				r := b.base
				r.Trunk = false
				r.Commit, r.Tree, r.Scope, r.At = "commit-b", "tree-b", "scoped", b.at.Format(time.RFC3339)
				b.save(t, r)
				writePushRecords(t, b.install, Pushed{Tree: r.Tree, At: r.At, BatchID: "batch-b"})
				b.move("c", 30*time.Minute)
				b.checkWake(t, 0, false)
				b.inherited(t, "c", 1)
				b.at = bedNow.Add(time.Hour + time.Second)
				b.checkWake(t, 1, true)
				return
			}
			accepted := slices.Contains([]string{"flake-record", "stacked", "did-not-run-red", "legacy-inheritance-evidence", "later-red-after-inheritance"}, name)
			if !accepted {
				b.checkWake(t, 1, name == "legacy-row")
				if name != "future-marked-row" {
					b.inherited(t, "b", 0)
				}
				return
			}
			b.checkWake(t, 0, false)
			b.inherited(t, "b", 1)
			if name == "later-red-after-inheritance" {
				r := b.base
				r.Trunk = false
				r.Result = Red
				r.Cause = &Cause{Kind: "main"}
				r.At = b.at.Add(time.Minute).Format(time.RFC3339)
				r.Attempt = "batch-replay"
				b.save(t, r)
				for _, restart := range []bool{false, true} {
					if restart {
						b.restart()
					}
					before := b.trunks
					b.wake(t)
					if b.trunks != before+1 {
						t.Fatalf("recovery reused inheritance after executed ancestor red: launches=%d want %d restart=%v", b.trunks, before+1, restart)
					}
					if err := removeRunning(b.install, b.seams); err != nil {
						t.Fatal(err)
					}
					running, already, err := Start(b.install, b.checkout, b.seams)
					if err != nil || already || running.Reused != nil || b.trunks != before+2 {
						t.Fatalf("direct admission reused inheritance after red: %+v already=%v launches=%d error=%v", running, already, b.trunks, err)
					}
					if err := removeRunning(b.install, b.seams); err != nil {
						t.Fatal(err)
					}
				}
				b.inherited(t, "b", 1)
				return
			}
			if name == "stacked" {
				b.move("c", 30*time.Minute)
				b.checkWake(t, 0, false)
				b.inherited(t, "c", 1)
			}
			for _, age := range []time.Duration{time.Hour + time.Second, 3*time.Hour + 59*time.Minute} {
				b.at = bedNow.Add(age)
				b.checkWake(t, 0, false)
			}
			b.restart()
			b.checkWake(t, 0, false)
			b.inherited(t, b.main, 1)
			b.at = bedNow.Add(4 * time.Hour)
			b.checkWake(t, 1, true)
			if len(b.argv) != 1 || !slices.Contains(b.argv[0], "--trunk") || !slices.Contains(b.argv[0], "--fresh") {
				t.Fatalf("four-hour launch argv=%v", b.argv)
			}
		})
	}
}
