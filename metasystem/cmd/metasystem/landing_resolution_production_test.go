package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

func TestLandingResolutionReadProductionGate(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		paths    []string
		read     bool
		fallback int
	}{
		{"tests", []string{"cmd/main_test.go", "internal/other_test.go"}, false, 0},
		{"kept goal tests", []string{"cmd/main_test.go"}, true, 1},
		{"kept main tests", []string{"cmd/main_test.go"}, true, 2},
		{"production", []string{"cmd/main_test.go", "internal/other.go"}, true, 0},
		{"person policy", []string{"internal/other.go"}, true, 0},
		{"person production", []string{"internal/other.go"}, true, 0},
		{"plans", []string{"plans/designs/change.md"}, false, 0},
		{"records", []string{"memory/record.md", "records/reads/result.json"}, false, 0},
		{"skills", []string{"skills/landing-agent/SKILL.md"}, false, 0},
		{"testdata", []string{"internal/fixture/testdata/record.json"}, false, 0},
		{"go in testdata", []string{"internal/fixture/testdata/main.go"}, true, 0},
		{"other production", []string{"docs/guide.md"}, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newWorkBed(t)
			owners := b.workOwners()
			layout, err := owners.resolver.ResolveLayout(b.root())
			if err != nil {
				t.Fatal(err)
			}
			root, checkout := layout.InstallationRoot.Path(), layout.GitRoot
			pid := int64(os.Getpid())
			exact, _, err := (identity.KernelProber{}).Probe(pid)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := lease.AnnounceWithPair(root, "lane-session", pid, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "lane-session", "metasystem", lane.AgentLineage); err != nil {
				t.Fatal(err)
			}
			if holder, err := lease.RequireHolder(root, pid, nil); err != nil || !holder.Holder {
				t.Fatalf("holder=%+v %v", holder, err)
			}
			home := t.TempDir()
			registered := lane.Record{Root: checkout, Install: root, CustodyEpoch: 1}
			data, _ := json.Marshal(registered)
			if err := os.MkdirAll(filepath.Dir(lane.RecordPath(home)), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(lane.RecordPath(home), data, 0600); err != nil {
				t.Fatal(err)
			}
			owners.landing.home = func() (string, error) { return home, nil }
			owners.policies = config.PolicyReaders{
				Registry: func(string) (config.PolicyRegistry, error) { return config.PolicyRegistry{Lane: checkout}, nil },
				Helm:     func(string) helm.State { return helm.State{} },
				ConfPath: func(string) (string, error) { return filepath.Join(root, "metasystem.conf"), nil },
			}
			now := laneTestNow
			owners.landing.now = func() time.Time { return now }
			owners.landing.plainProve.BudgetHome = t.TempDir()
			pushes := 0
			owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
				pushes++
				if dir != checkout || strings.Join(args, " ") != "push origin merge:refs/heads/lane-merges/"+b.id {
					t.Fatalf("unexpected publication: %s %v", dir, args)
				}
				return "", nil
			}
			fix := &plain.Fix{Attempt: "resolution", Goal: b.id, Tip: "member", Commit: "merge", Job: "build", Units: []string{"lane-merge-1"}, State: "reviewing", CheckResult: "green", Brief: filepath.Join(b.root(), b.brief("merge.md", "Resolve conflicts.\n")), Paths: []conflict.Path{{Path: "source.go", Class: conflict.Builder}}}
			if err := plain.WriteFix(root, fix); err != nil {
				t.Fatal(err)
			}
			owners.work.git = func(_ string, args ...string) ([]byte, error) {
				switch args[0] {
				case "rev-list":
					return []byte("merge first second\n"), nil
				case "merge-tree":
					return []byte("automatic\n"), nil
				case "diff":
					if tc.fallback != 0 && (args[2] == "automatic" || tc.fallback == 2 && args[2] == "first") {
						return nil, nil
					}
					if args[1] == "--name-only" {
						base := "automatic"
						if tc.fallback != 0 {
							base = []string{"", "first", "second"}[tc.fallback]
						}
						if args[4] != base {
							t.Fatalf("classification used another diff: %v", args)
						}
						return []byte(strings.Join(tc.paths, "\x00") + "\x00"), nil
					}
					var patch strings.Builder
					for _, path := range tc.paths {
						fmt.Fprintf(&patch, "diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\n@@ -0,0 +1 @@\n+resolution\n", path, path, path, path)
					}
					return []byte(patch.String()), nil
				default:
					t.Fatalf("unexpected git: %v", args)
					return nil, nil
				}
			}
			b.starter.hold = "read"
			command, _ := findIntentAction("work", "review")
			var out, problem bytes.Buffer
			code := runIntentIn(command, []string{b.id, "--work", "lane-merge-1"}, &out, &problem, checkout, owners)
			active, err := plain.FixForAttempt(root, fix.Attempt)
			if err != nil {
				t.Fatal(err)
			}
			if tc.read {
				if code != 124 || !slices.Contains(b.starter.launched(), "read") || active.State != "reviewing" || !active.ReadLaneOwned || active.ReadStartedAt == "" {
					t.Fatalf("read exit=%d launches=%v record=%+v output=%s %s", code, b.starter.launched(), active, &out, &problem)
				}
				if tc.name == "production" || tc.name == "person production" || tc.name == "person policy" {
					if _, _, err := plain.HandIn(root, plain.Line{Goal: b.id, SHA: "member"}); err != nil {
						t.Fatal(err)
					}
					if tc.name == "person production" {
						active.ReadLaneOwned = false
						if err := plain.WriteFix(root, active); err != nil {
							t.Fatal(err)
						}
					}
					if tc.name == "person policy" {
						if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("landing.on-red=person\n"), 0600); err != nil {
							t.Fatal(err)
						}
					}
					sleep := b.manager.Sleep
					b.manager.Sleep = func(d time.Duration) {
						sleep(d)
						jobs, err := b.manager.Store.List()
						if err != nil {
							t.Fatal(err)
						}
						for _, job := range jobs {
							if job.Reason == "cancel-requested" {
								if _, err := b.manager.Store.Update(job.ID, func(r *launch.Record) error { r.State = launch.Cancelled; r.Child, r.Supervisor = nil, nil; return nil }); err != nil {
									t.Fatal(err)
								}
							}
						}
					}
					now = now.Add(21 * time.Minute)
					out.Reset()
					problem.Reset()
					code = runIntentIn(command, []string{b.id, "--work", "lane-merge-1", "--json"}, &out, &problem, checkout, owners)
					entry, _, err := plain.Latest(root, b.id)
					if err != nil {
						t.Fatal(err)
					}
					read, err := owners.work.units(layout).InspectRead(active.Read)
					if err != nil {
						t.Fatal(err)
					}
					if tc.name == "production" {
						if code != 0 || pushes != 1 || entry.State != plain.StateReturned || !strings.Contains(entry.Reason, "origin refs/heads/lane-merges/"+b.id) || !read.Stopping || !slices.ContainsFunc(read.Reports, func(r launch.ReadReport) bool { return r.LaunchState == launch.Cancelled }) {
							t.Fatalf("expire exit=%d entry=%+v read=%+v output=%s %s", code, entry, read, &out, &problem)
						}
					} else if code != 124 || pushes != 0 || entry.State != plain.StateWaiting || read.Attempt.State != "running" || read.Stopping {
						t.Fatalf("person read ended: exit=%d entry=%+v read=%+v", code, entry, read)
					}
					if tc.name == "person policy" {
						retained, err := plain.FixForAttempt(root, fix.Attempt)
						if err != nil || !strings.Contains(retained.Reason, "merge read ran 21 min, bound 20") || !strings.Contains(retained.Reason, "landing give-back "+b.id) || !strings.Contains(out.String(), "landing give-back "+b.id) {
							t.Fatalf("person finding=%+v %v", retained, err)
						}
					}
				}
			} else {
				want := fmt.Sprintf("resolution of %s reviewed by its check only: %d test/records files, no production code", b.id, len(tc.paths))
				if code != 0 || len(b.starter.launched()) != 0 || active.State != "done" || active.Reason != want || !strings.Contains(oneSpaced(out.String()), want) {
					t.Fatalf("skip exit=%d launches=%v record=%+v output=%s %s", code, b.starter.launched(), active, &out, &problem)
				}
			}
		})
	}
}

func TestSkillLandingAgentFirstJobBeforeReading(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../skills/landing-agent/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	sentence := "When this step has a member to merge or a conflict to resolve, launch that job (the resolution job first) before any other reading; the engine records the step at launch."
	if strings.Count(string(data), sentence) != 1 {
		t.Fatal("skill omits the first job and launch record instruction")
	}
	step := strings.Index(string(data), "3. Otherwise:")
	next := strings.Index(string(data), "4. ")
	instruction := strings.Index(string(data), sentence)
	if step < 0 || next < 0 || instruction < step || instruction > next {
		t.Fatal("first job instruction must be inside step 3, after pause and proof handling")
	}
	if count := len(strings.Fields(string(data))); count > 3000 {
		t.Fatalf("skill words=%d, budget=3000", count)
	}
}
