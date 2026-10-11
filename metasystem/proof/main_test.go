package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/repoproof"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// This integration drives the reporter's real process entrypoint and marker
// lifetime; a stubbed launcher cannot prove the child's kernel ancestry.
func TestReporterRegistersLiveAncestorAndRemovesMarker(t *testing.T) {
	t.Parallel()
	reporter := filepath.Join(t.TempDir(), "reporter")
	build := testenv.Go("build", "-buildvcs=false", "-o", reporter, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build reporter: %v\n%s", err, output)
	}
	module, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	contract, err := testpolicy.Load("testdata/testing.json")
	if err != nil {
		t.Fatal(err)
	}
	const id = "section/go-engine-gate"
	for i, group := range contract.Groups {
		if group.ID == id {
			group.Argv = []string{binary, "section-marker-probe", module}
			group.CWD = "."
			group.Inputs = []string{"bed.txt"}
			group.Tools = nil
			contract.Groups[i] = group
		}
	}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{"testing.json": data, "bed.txt": []byte("section input")} {
		if err := os.WriteFile(filepath.Join(module, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, status := range []string{"0", "1"} {
		command := exec.Command(reporter, "--section", id)
		command.Dir = module
		command.Env = append(os.Environ(), "SECTION_MARKER_PROBE_EXIT="+status)
		output, err := command.CombinedOutput()
		if (err == nil) != (status == "0") || !strings.Contains(string(output), "authenticated reporter ancestor") {
			t.Fatalf("section status=%s err=%v output=%s", status, err, output)
		}
		survey := gaterun.Survey(module)
		if len(survey.Live) != 0 || len(survey.Unreadable) != 0 {
			t.Fatalf("reporter left its marker: %+v", survey)
		}
	}
}

func TestMain(m *testing.M) {
	if len(os.Args) == 3 && os.Args[1] == "section-marker-probe" {
		for _, marker := range gaterun.Survey(os.Args[2]).Live {
			if marker.Gate == "repository proof" && proofrun.AuthenticateAncestor(int64(os.Getpid()),
				proofrun.ProcessIdentity{Pid: marker.Pid, PidStartedAt: marker.PidStartedAt}) == nil {
				fmt.Print("authenticated reporter ancestor\n")
				if os.Getenv("SECTION_MARKER_PROBE_EXIT") == "1" {
					os.Exit(1)
				}
				os.Exit(0)
			}
		}
		fmt.Fprintln(os.Stderr, "no live reporter ancestor")
		os.Exit(1)
	}
	if exit, handled := repoproof.Custodian(os.Args[1:], os.Stderr); handled {
		os.Exit(exit)
	}
	os.Exit(testenv.Main(m))
}

func TestReporterRunsNamedReplayAndCompilesUnselectedTests(t *testing.T) {
	t.Parallel()
	reporter := filepath.Join(t.TempDir(), "reporter")
	build := testenv.Go("build", "-buildvcs=false", "-o", reporter, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	contract, err := os.ReadFile("testdata/testing.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"named", "named group", "whole", "broken compile", "missing name", "mixed missing", "missing broken compile"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			module := t.TempDir()
			unit := "metasystem/cmd/metasystem"
			bedContract := contract
			if mode == "named group" {
				unit = "go-batchtest"
				decoded, err := testpolicy.Decode(contract)
				if err != nil {
					t.Fatal(err)
				}
				for i := range decoded.Groups {
					group := &decoded.Groups[i]
					if group.ID == unit {
						group.CWD = "."
						group.Packages = []string{"cmd/metasystem"}
						group.BuildTags = nil
						group.Inputs = []string{"go.mod", "cmd/metasystem/**"}
						group.Tools = nil
					}
				}
				bedContract, err = json.Marshal(decoded)
				if err != nil {
					t.Fatal(err)
				}
			}
			files := map[string][]byte{
				"testing.json":                 bedContract,
				"go.mod":                       []byte("module example.com/replay\n\ngo 1.23\n"),
				"cmd/metasystem/named_test.go": []byte("package main\nimport (\"testing\"; \"os\")\nfunc record(t *testing.T, name string) { t.Helper(); if err:=os.WriteFile(name+\".ran\",[]byte(name),0600); err!=nil { t.Fatal(err) } }\nfunc TestA(t *testing.T) { t.Parallel(); record(t,\"TestA\") }\nfunc TestB(t *testing.T) { t.Parallel(); record(t,\"TestB\") }\nfunc TestC(t *testing.T) { t.Parallel(); record(t,\"TestC\") }\nfunc TestD(t *testing.T) { t.Parallel(); record(t,\"TestD\") }\nfunc TestE(t *testing.T) { t.Parallel(); record(t,\"TestE\") }\nfunc TestAB(t *testing.T) { t.Parallel(); t.Fatal(\"unselected\") }\n"),
			}
			if mode == "broken compile" || mode == "missing broken compile" {
				files["cmd/metasystem/broken_test.go"] = []byte("package main\nimport \"testing\"\nfunc TestUnselected(t *testing.T) { t.Parallel(); doesNotCompile() }\n")
			}
			for name, data := range files {
				path := filepath.Join(module, name)
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			names := "TestA TestB TestC TestD TestE"
			if mode == "whole" {
				names = ""
			}
			if mode == "missing name" || mode == "missing broken compile" {
				names = "TestAbsent"
			}
			if mode == "mixed missing" {
				names += " TestAbsent"
			}
			command := exec.Command(reporter, "--host")
			command.Dir = module
			command.Env = proofrun.TestingEnvironment(os.Environ(), map[string]string{"LANDING_ONLY": unit, "LANDING_ONLY_TESTS": names, "LANDING_PROOF_SCOPE": "full", "GOWORK": "off", "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOSUMDB": "off", "METASYSTEM_TESTING_WORKERS": "9"})
			output, err := command.CombinedOutput()
			if mode == "named" || mode == "named group" || mode == "missing name" || mode == "mixed missing" {
				if err != nil || mode == "named" && !strings.Contains(string(output), "landing planned 1\n") || !strings.HasSuffix(string(output), "LANDING-CHECKED\t0\n") {
					t.Fatalf("named replay: %v\n%s", err, output)
				}
				runs := map[string]int{}
				for _, line := range strings.Split(string(output), "\n") {
					var event struct{ Action, Test string }
					if json.Unmarshal([]byte(line), &event) == nil && event.Action == "run" && event.Test != "" {
						runs[event.Test]++
					}
				}
				want := map[string]int{"TestA": 1, "TestB": 1, "TestC": 1, "TestD": 1, "TestE": 1}
				if mode == "missing name" {
					want = map[string]int{}
					if !strings.Contains(string(output), `"Action":"pass","Package":"example.com/replay/cmd/metasystem"`) {
						t.Fatalf("compile did not complete: %s", output)
					}
				}
				if mode == "named group" {
					for name := range want {
						data, err := os.ReadFile(filepath.Join(module, "cmd/metasystem", name+".ran"))
						if err != nil || string(data) != name {
							t.Fatalf("group omitted %s: %v", name, err)
						}
					}
					return
				}
				if !reflect.DeepEqual(runs, want) {
					t.Fatalf("replay ran=%v want=%v", runs, want)
				}
			} else if err == nil || strings.HasSuffix(string(output), "LANDING-CHECKED\t0\n") {
				t.Fatalf("boundary %s falsely passed: %v\n%s", mode, err, output)
			}
		})
	}
}

func TestReporterReplaysRealRedAgainstTheRepositoryContract(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../internal/landing/plain/testdata/named-replay-red.json")
	if err != nil {
		t.Fatal(err)
	}
	var red struct {
		Failed []struct {
			Unit  string
			Tests []string
		}
		BatchMembers []struct {
			Goal string `json:"goal"`
		} `json:"batch-members"`
	}
	if err := json.Unmarshal(data, &red); err != nil {
		t.Fatal(err)
	}
	if len(red.Failed) != 1 || len(red.BatchMembers) != 1 {
		t.Fatalf("red shape=%+v", red)
	}
	ledger, err := os.ReadFile("../internal/landing/plain/testdata/named-replay-goal.md")
	if err != nil {
		t.Fatal(err)
	}
	record, problems := goal.ParseFile(ledger)
	if len(problems) != 0 || record.Id != red.BatchMembers[0].Goal {
		t.Fatalf("real ledger=%+v problems=%v", record, problems)
	}
	reporter := filepath.Join(t.TempDir(), "reporter")
	build := testenv.Go("build", "-buildvcs=false", "-o", reporter, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	contract, err := os.ReadFile("testdata/testing.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, present := range []bool{true, false} {
		t.Run(fmt.Sprintf("present=%t", present), func(t *testing.T) {
			t.Parallel()
			module := t.TempDir()
			name := red.Failed[0].Tests[0]
			source := "package plain\nimport \"testing\"\nfunc TestUnselected(t *testing.T) { t.Parallel(); t.Fatal(\"unselected\") }\n"
			if present {
				source += "func " + name + "(t *testing.T) { t.Parallel(); t.Log(\"real red identity ran\") }\n"
			}
			files := map[string][]byte{
				"go.mod":                                []byte("module example.com/replay\n\ngo 1.23\n"),
				"testing.json":                          contract,
				"plans/goals/" + record.Id + ".md":      ledger,
				"internal/landing/plain/replay_test.go": []byte(source),
			}
			for name, data := range files {
				path := filepath.Join(module, name)
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			command := exec.Command(reporter, "--host")
			command.Dir = module
			command.Env = proofrun.TestingEnvironment(os.Environ(), map[string]string{"LANDING_ONLY": red.Failed[0].Unit, "LANDING_ONLY_TESTS": name, "GOWORK": "off", "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOSUMDB": "off", "METASYSTEM_TESTING_WORKERS": "9"})
			output, err := command.CombinedOutput()
			if err != nil || !strings.Contains(string(output), "landing planned 1\n") || !strings.HasSuffix(string(output), "LANDING-CHECKED\t0\n") {
				t.Fatalf("real named replay: %v\n%s", err, output)
			}
			ran := []string{}
			compiled := false
			for _, line := range strings.Split(string(output), "\n") {
				var event struct{ Action, Test, Package string }
				if json.Unmarshal([]byte(line), &event) != nil {
					continue
				}
				if event.Action == "run" && event.Test != "" && !strings.Contains(event.Test, "/") {
					ran = append(ran, event.Test)
				}
				compiled = compiled || event.Action == "pass" && event.Test == "" && event.Package == "example.com/replay/internal/landing/plain"
			}
			want := []string{}
			if present {
				want = red.Failed[0].Tests
			}
			if !compiled || !reflect.DeepEqual(ran, want) {
				t.Fatalf("real replay compiled=%t ran=%v want=%v\n%s", compiled, ran, want, output)
			}
		})
	}
}
