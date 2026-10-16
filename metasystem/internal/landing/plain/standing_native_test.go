package plain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/repoproof"
)

// The package and test identities mirror the protocol standard's concrete
// selection. The module and test bodies are small, independent fixtures.
func standingNativeBed(t *testing.T, mode string) *standingBed {
	t.Helper()
	b := newStandingBed(t, false)
	group := b.contract.Groups[0]
	group.ID, group.Adapter, group.Argv, group.Format, group.CWD = "agent-protocol-standard", "go", nil, "", "."
	group.Inputs = []string{"metasystem/plans/designs/**"}
	group.Packages = []string{"cmd/metasystem"}
	group.Tests = json.RawMessage(`["TestProtocolCheckersRefuseMissingArgumentsAsUsage","TestOther"]`)
	b.contract.Groups[0].Inputs = []string{"metasystem/plans/goals/**"}
	if mode == "blocked" {
		group.Requires = []string{"shared"}
		b.contract.Groups[2].Argv = []string{"/usr/bin/false"}
	} else {
		b.contract.Groups[2].Argv = []string{"/usr/bin/true"}
	}
	b.contract.Groups = append(b.contract.Groups, group)
	b.red.Failed[1].Tests = []string{"TestProtocolCheckersRefuseMissingArgumentsAsUsage"}
	b.incidents[1].Identity = "red:metasystem/cmd/metasystem:TestProtocolCheckersRefuseMissingArgumentsAsUsage"
	if mode == "flake" {
		b.red.Failed = b.red.Failed[:1]
	}
	b.save(t, b.red)
	b.git.changed[[2]string{b.red.Tree, b.git.tree}] = "metasystem/plans/designs/protocol.md"
	b.setContract(t)
	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	tools := t.TempDir()
	if err := os.Symlink(goPath, filepath.Join(tools, "go")); err != nil {
		t.Fatal(err)
	}
	environment := proofrun.TestingEnvironment(os.Environ(), map[string]string{"PATH": tools, "GOWORK": "off", "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOSUMDB": "off", "METASYSTEM_TESTING_WORKERS": "2", "METASYSTEM_TEST_WORKERS": "2"})
	// Each detached worktree gets its own generated module. The same directory
	// is reused for its permitted isolated confirmation and diagnostic reruns.
	b.seams.Command = func(cmd *exec.Cmd) error {
		b.calls = append(b.calls, cmd)
		contract := b.contract
		for i := range contract.Groups {
			contract.Groups[i].CWD = "."
		}
		data, err := json.Marshal(contract)
		if err != nil {
			return err
		}
		files := map[string]string{"go.mod": "module github.com/widoriezebos/agentic-tools/metasystem\n\ngo 1.23\n", "testing.json": string(data), "cmd/metasystem/mode": mode, "cmd/metasystem/native_test.go": `package main
import ("testing";"os")
func TestProtocolCheckersRefuseMissingArgumentsAsUsage(t *testing.T) {
 t.Parallel()
 mode,_:=os.ReadFile("mode")
 if string(mode)=="fail again" {t.Fatal("protocol failed")}
}
func TestOutsideSelection(t *testing.T) { t.Parallel();t.Fatal("unselected standing test ran") }
func TestOther(t *testing.T) {
 t.Parallel()
 mode,_:=os.ReadFile("mode")
 if string(mode)=="flake" {
  f,err:=os.OpenFile("first-failure",os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600)
  if err!=nil {return};f.Close()
 }
 if string(mode)!="all pass" {t.Fatal("other failed")}
}
`}
		for name, contents := range files {
			p := filepath.Join(cmd.Dir, name)
			if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
				return err
			}
			if err := os.WriteFile(p, []byte(contents), 0600); err != nil {
				return err
			}
		}
		getenv := func(key string) string {
			value := commandEnv(cmd, key)
			if value == "missing" {
				return ""
			}
			return value
		}
		hooks := repoproof.HostRunners{Environment: func() (string, error) { return b.environment, nil }, Groups: func(ids []string) ([]proofrun.NamedGroupResult, error) {
			selected := contract
			if names := commandEnv(cmd, "LANDING_ONLY_TESTS"); names != "" && names != "missing" {
				for i := range selected.Groups {
					if slices.Contains(ids, selected.Groups[i].ID) {
						selected.Groups[i].Tests, _ = json.Marshal(strings.Fields(names))
					}
				}
			}
			return proofrun.RunNamedGroups(context.Background(), cmd.Dir, selected, ids, environment)
		}, Native: func(r proofrun.NativeInventoryRequest) (proofrun.NativeInventoryResult, error) {
			r.Root = cmd.Dir
			r.Environment = environment
			return proofrun.RunNativeInventory(context.Background(), r)
		}}
		code := repoproof.RunHostAt(cmd.Dir, cmd.Stdout, cmd.Stderr, getenv, func([]string, io.Writer, io.Writer) error { return fmt.Errorf("unexpected full section") }, hooks)
		if code != 0 {
			return exec.Command("/usr/bin/false").Run()
		}
		return nil
	}
	return b
}

func TestStandingNativeNamedGroupOutcomes(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"pass beside fail", "fail again", "blocked", "flake", "all pass"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := standingNativeBed(t, mode)
			sightings := map[string]int{}
			b.seams.RecordMain = func(checks []Result) error {
				for _, c := range checks {
					for _, f := range c.Failed {
						if f.Inherited {
							t.Fatal("carried sighting")
						}
						for _, name := range f.Tests {
							sightings[name]++
						}
					}
				}
				return nil
			}
			flakes := 0
			b.seams.RecordFlake = func(f FlakeRecord) (FlakeRecorded, error) {
				flakes++
				if mode == "flake" {
					history, err := Results(b.install)
					if err != nil {
						t.Fatal(err)
					}
					for _, observation := range history {
						if observation.Attempt == "attempt" && !slices.ContainsFunc(observation.Failed, func(unit FailedUnit) bool { return unit.Unit == "plans" && unit.Inherited }) {
							t.Fatalf("classification lost carried failure in durable record %+v", observation)
						}
					}
				}
				if len(f.Outputs) != 1 || len(f.RepeatOutputs) != 1 || f.RepeatOutputs["TestOther"].Outcome != "pass" {
					t.Fatalf("native evidence missing %+v", f)
				}
				return FlakeRecorded{Seen: 1}, nil
			}
			r := b.detached(t)
			if r.Scope != "scoped" || r.CountedFull {
				t.Fatalf("depth %+v", r)
			}
			protocol := []FailedUnit{}
			for _, f := range r.Failed {
				if slices.Contains(f.Tests, "TestProtocolCheckersRefuseMissingArgumentsAsUsage") {
					protocol = append(protocol, f)
				}
			}
			if mode == "blocked" {
				if len(protocol) != 1 || !protocol[0].Inherited || slices.Contains(r.Ran, "agent-protocol-standard") {
					t.Fatalf("blocked group supplied outcomes %+v", r)
				}
			} else if mode == "fail again" {
				if len(protocol) != 1 || protocol[0].Inherited || sightings["TestProtocolCheckersRefuseMissingArgumentsAsUsage"] != 1 || len(r.FlakeRepeats) == 0 {
					t.Fatalf("executed failure duplicated or not confirmed %+v sightings=%v", r, sightings)
				}
			} else if len(protocol) != 0 {
				t.Fatalf("passing test stayed standing %+v", r)
			}
			if mode == "flake" && (flakes != 1 || len(r.Failed) != 1 || !r.Failed[0].Inherited || r.Failed[0].Unit != "plans") {
				t.Fatalf("confirmation cleared carried failure %+v flakes=%d", r, flakes)
			}
			if mode == "pass beside fail" && (sightings["TestOther"] != 1 || len(r.Failed) != 2 || r.Failed[0].Inherited || !r.Failed[1].Inherited) {
				t.Fatalf("mixed outcomes %+v sightings=%v", r, sightings)
			}
			if !slices.ContainsFunc(r.Failed, func(f FailedUnit) bool { return f.Unit == "plans" && f.Inherited }) {
				t.Fatalf("plans lost %+v", r)
			}
		})
	}
}

func TestStandingPassDoesNotResurrectOrGrantCoverage(t *testing.T) {
	t.Parallel()
	for _, green := range []bool{false, true} {
		t.Run(fmt.Sprint(green), func(t *testing.T) {
			t.Parallel()
			b := standingNativeBed(t, "all pass")
			b.red.Failed = b.red.Failed[1:]
			if green {
				if err := os.Remove(resultsPath(b.install)); err != nil {
					t.Fatal(err)
				}
				b.save(t, b.base)
				b.git.changed[[2]string{b.base.FullTree, b.git.tree}] = "metasystem/records/image/changed.md"
			}
			b.save(t, b.red)
			r := b.detached(t)
			if r.Result != Green || len(r.Failed) != 0 || len(r.Unproved) == 0 || r.reusableGreen(b.seams.now()) {
				t.Fatalf("incomplete scoped green %+v", r)
			}
			if green && r.FullAt != b.base.FullAt {
				t.Fatalf("fixture lost qualifying green: %+v", r)
			}
			if _, ok := applicableMainProof(b.install, b.checkout, r.Commit, r.Tree, b.seams, true); ok {
				t.Fatal("incomplete scoped green admitted as complete")
			}
			if provenGreen(b.install, r.Tree) == nil {
				t.Fatal("incomplete scoped green authorised publication")
			}
			b.git.commit, b.git.tree = "next-record", "next-tree"
			if green {
				b.git.changed[[2]string{b.base.FullTree, b.git.tree}] = "metasystem/records/image/changed.md"
			}
			b.setContract(t)
			b.seams.NewID = func() string { return "after-pass" }
			next := b.detached(t)
			if next.Result != Green || len(next.Failed) != 0 || len(next.Ran) != 0 || len(next.Unproved) == 0 || len(b.incidents) != 2 {
				t.Fatalf("standing failure resurrected or coverage invented %+v", next)
			}
		})
	}
}

func TestStandingNativePartiallySelectedUnitKeepsOnlyUnexecutedTest(t *testing.T) {
	t.Parallel()
	b := standingNativeBed(t, "all pass")
	b.red.Failed[1].Tests = append(b.red.Failed[1].Tests, "TestOutsideSelection")
	b.incidents = append(b.incidents, goal.TrunkRedEntry{ID: "outside", Identity: "red:metasystem/cmd/metasystem:TestOutsideSelection"})
	b.save(t, b.red)
	r := b.detached(t)
	if len(r.Failed) != 2 || !slices.Equal(r.Failed[1].Tests, []string{"TestOutsideSelection"}) || !r.Failed[1].Inherited {
		t.Fatalf("selection membership invented evidence %+v", r)
	}
	b.provenance(t, r, map[string]string{"plans": "standing", "shared": "unproved", "unrelated": "unproved", "agent-protocol-standard": "ran", "unit/cmd/metasystem": "standing"})
}
