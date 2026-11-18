package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/checkenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// Scan complete artifacts, without printing their contents on failure.
func assertNoAmbientBytes(t *testing.T, root string, values ...string) map[string]bool {
	t.Helper()
	found := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, value := range values {
			if bytes.Contains(data, []byte(value)) {
				t.Fatalf("ambient value in artifact %s", filepath.Base(path))
			}
		}
		found[filepath.Base(path)] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func TestDeclaredEnvironmentPublicWritersAndAttribution(t *testing.T) {
	first := rand.Text()
	second := rand.Text()
	unrelated := rand.Text()
	t.Setenv("E1_AMBIENT", first)
	t.Setenv("GOOGLE_API_KEY", unrelated)
	t.Setenv("BASH_FUNC_which%%", rand.Text())
	for _, flake := range []bool{false, true} {
		t.Run(map[bool]string{false: "base comparison", true: "isolated flake"}[flake], func(t *testing.T) {
			t.Setenv("E1_AMBIENT", first)
			bed := unitFlakeBuild(t, true, false, flake, func(b *workBed, script *string) []string {
				*script = strings.Replace(*script, "#!/bin/sh\n", "#!/bin/sh\n[ -z \"${E1_AMBIENT+x}\" ] && [ -z \"${GOOGLE_API_KEY+x}\" ] || exit 3\n", 1)
				b.manager.Supervisor = unitProofStarter{bed: b, t: t, before: func(record launch.Record) {
					if strings.HasSuffix(record.ID, "-alone") {
						t.Setenv("E1_AMBIENT", second)
					} else if strings.HasSuffix(record.ID, "-base") {
						if err := os.Unsetenv("E1_AMBIENT"); err != nil {
							t.Fatal(err)
						}
					}
				}}
				return nil
			})
			found := assertNoAmbientBytes(t, bed.unitRoot, first, second, unrelated)
			for _, name := range []string{"plan.json", "run.json", "proof-unit-check.json", "result.json", "alone-1.json"} {
				if !found[name] {
					t.Fatalf("writer not exercised: %s", name)
				}
			}
			if !flake && !found["base-1.json"] {
				t.Fatal("base writer not exercised")
			}
			paths, _ := filepath.Glob(filepath.Join(bed.unitRoot, "*", "plan.json"))
			if len(paths) != 1 {
				t.Fatal("missing top-level plan")
			}
			plan, err := launch.ReadUnitPlan(paths[0])
			if err != nil {
				t.Fatal(err)
			}
			if plan.Proof[0].Input == nil || !slices.Contains(plan.Proof[0].Input.Dropped, "E1_AMBIENT") || !slices.Contains(plan.Proof[0].Input.Dropped, "GOOGLE_API_KEY") || !slices.Contains(plan.Proof[0].Input.Dropped, "BASH_FUNC_which%%") {
				t.Fatal("dropped names not retained")
			}
		})
	}
}

func TestDeclaredEnvironmentCallerAdmission(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"declared", "flat", "GOPROXY", "GOFLAGS", "GOAUTH", "current-policy", "wrong-policy"} {
		t.Run(scenario, func(t *testing.T) {
			b := newWorkBed(t)
			b.declaredCheap = `test "$E1_CUSTOM" = declared-synthetic`
			command := launch.ProofCommand{Name: "check", Dir: b.worktree, Argv: []string{"/bin/true"}, Env: []string{}}
			switch scenario {
			case "declared":
				command.DeclaredEnv = []string{"E1_CUSTOM=declared-synthetic"}
			case "flat":
				command.Env = []string{"E1_CUSTOM=flat-synthetic"}
			case "current-policy":
				command.Input = &checkenv.Input{Version: checkenv.Version, Entries: []checkenv.Entry{{Name: "GOPROXY", Value: "https://user:unsupported-synthetic@host", Source: "host"}}}
			case "wrong-policy":
				command.Input = &checkenv.Input{Version: checkenv.Version + 1}
			case "GOAUTH":
				command.Env = []string{"GOAUTH=unsupported-synthetic"}
			default:
				command.Env = []string{scenario + "=https://user:unsupported-synthetic@host"}
			}
			plan := launch.UnitPlan{Goal: b.id, Unit: "environment", Worktree: b.worktree, Base: b.head,
				Build: launch.UnitBuildPlan{Brief: filepath.Join(b.root(), b.brief("environment.md", "Build.\n")), UnitsPage: filepath.Join(b.root(), b.brief("environment-units.md", "| Unit | Changed lines |\n| --- | --- |\n| environment | 5 |\n")), Inputs: []string{}, Outputs: []string{}, Units: []string{"environment"}}, Proof: []launch.ProofCommand{command}}
			before, _ := json.Marshal(plan)
			path := filepath.Join(t.TempDir(), "caller.json")
			if err := os.WriteFile(path, before, 0600); err != nil {
				t.Fatal(err)
			}
			code, built := checkActBuild(t, b, b.workOwners(), "work", "build", b.id, "--plan", path)
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("caller plan changed")
			}
			if scenario == "declared" {
				if code != 0 {
					t.Fatalf("declaration refused: %s", built.Summary)
				}
				run := resultData(t, built)["run"].(string)
				code, _ = declaredCheckAt(t, b, b.worktree, "", "test", "run", "--unit-run", run)
				if code != 0 {
					t.Fatal("declared name did not reach ordinary check")
				}
			} else {
				if code == 0 || strings.Contains(built.Summary, "synthetic") {
					t.Fatal("caller input was accepted or its value disclosed")
				}
				if _, err := os.Stat(b.unitRoot); err == nil {
					assertNoAmbientBytes(t, b.unitRoot, "flat-synthetic", "unsupported-synthetic")
				}
				copies, _ := filepath.Glob(filepath.Join(b.unitRoot, ".named", "*.plan"))
				if len(copies) != 0 {
					t.Fatal("refused plan was staged")
				}
			}
		})
	}
}
func TestDeclaredEnvironmentGeneratedUnsupported(t *testing.T) {
	for _, name := range []string{"GOPROXY", "GOFLAGS", "GOAUTH"} {
		t.Run(name, func(t *testing.T) {
			value := "https://user:unsupported-generated-synthetic@host"
			if name == "GOAUTH" {
				value = "unsupported-generated-synthetic"
			}
			t.Setenv(name, value)
			b := newWorkBed(t)
			code, result, _ := b.work("work", "build", b.id, "unsupported", "--brief", b.brief("unsupported.md", "Build.\n"), "--lines", "5")
			if code == 0 || strings.Contains(result.Summary, "synthetic") || !strings.Contains(result.Summary, name) {
				t.Fatal("generated input refusal is missing or disclosed a value")
			}
			if _, err := os.Stat(b.unitRoot); err == nil {
				assertNoAmbientBytes(t, b.unitRoot, value)
			}
		})
	}
}
func TestDeclaredEnvironmentDamagedCarryNoArchive(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"damaged", "legacy", "invalid-policy"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b, owners, args, _ := carryRecoveryBed(t)
			if code, _ := b.runJSON(owners, args...); code != 0 {
				t.Fatal("initial carry failed")
			}
			records := filepath.Join(b.root(), "artifacts", "unit-checks", "carry", args[3])
			path := filepath.Join(records, "subject.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var prior launch.UnitCheck
			if json.Unmarshal(data, &prior) != nil {
				t.Fatal("initial snapshot unreadable")
			}
			prior.Environment = []string{"E1_OLD=synthetic-old-carry"}
			prior.Input = nil
			if kind == "invalid-policy" {
				prior.Input = &checkenv.Input{Version: checkenv.Version + 1}
			}
			before, err := json.Marshal(prior)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "damaged" {
				before = []byte(`{"environment":["E1_OLD=synthetic-old-carry"],`)
			}
			if err := os.WriteFile(path, before, 0600); err != nil {
				t.Fatal(err)
			}
			code, refused := b.runJSON(owners, args...)
			diagnostic := refused.Summary
			if code == 0 || !strings.Contains(diagnostic, "run:") || strings.Contains(diagnostic, "declaredEnv") || strings.Contains(diagnostic, "synthetic-old-carry") {
				t.Fatalf("carry refusal: exit=%d repairHint=%t callerAdvice=%t disclosed=%t snapshotUnavailable=%t", code, strings.Contains(diagnostic, "run:"), strings.Contains(diagnostic, "declaredEnv"), strings.Contains(diagnostic, "synthetic-old-carry"), strings.Contains(diagnostic, "carry snapshot is unavailable"))
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("refusal changed snapshot")
			}
			engine := publicationEngineStub(t)
			owners.processes.executable = func() (string, error) { return engine, nil }
			repair := append(withoutOption(args, "act"), "--reason", "Repair unavailable snapshot", "--by", "Wido", "--check", "true")
			if code, _, _ := b.run(owners, repair...); code != 0 {
				t.Fatal("explicit carry repair failed")
			}
			data, err = os.ReadFile(path)
			var repaired launch.UnitCheck
			if err != nil || json.Unmarshal(data, &repaired) != nil || repaired.Input == nil || repaired.Input.Validate() != nil || repaired.Environment != nil {
				t.Fatal("repair did not publish a current approved snapshot")
			}
			archives, err := filepath.Glob(path + ".damaged-*")
			if err != nil || len(archives) != 0 {
				t.Fatal("repair archived the old payload")
			}
			assertNoAmbientBytes(t, records, "synthetic-old-carry")
		})
	}
}
func TestDeclaredEnvironmentPreservesHostSelection(t *testing.T) {
	for _, name := range []string{"GOENV", "GOWORK", "XDG_CONFIG_HOME"} {
		t.Setenv(name, "/synthetic-selection")
	}
	b := newWorkBed(t)
	code, built, _ := b.work("work", "build", b.id, "host", "--brief", b.brief("host.md", "Build.\n"), "--lines", "5")
	if code != 0 {
		t.Fatalf("host selection refused: %s", built.Summary)
	}
	plan, err := launch.ReadUnitPlan(resultData(t, built)["plan"].(string))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"GOENV", "GOWORK", "XDG_CONFIG_HOME"} {
		if !slices.Contains(plan.Check.EnvironmentValues(), name+"=/synthetic-selection") {
			t.Fatalf("host name lost: %s", name)
		}
	}
	if err := plan.Check.Input.Validate(); err != nil || plan.Check.Input.Version != checkenv.Version {
		t.Fatal("wrong approved representation")
	}
}

func TestDeclaredEnvironmentPublicRerunSeesDeclaredInput(t *testing.T) {
	t.Setenv("E1_DECLARED", "live-synthetic")
	unitFlakeBuild(t, true, false, false, func(b *workBed, script *string) []string {
		*script = strings.Replace(*script, "#!/bin/sh\n", "#!/bin/sh\n[ \"$E1_DECLARED\" = frozen-synthetic ] || exit 3\n", 1)
		b.manager.Supervisor = unitProofStarter{bed: b, t: t, before: func(record launch.Record) {
			if strings.HasSuffix(record.ID, "-alone") {
				t.Setenv("E1_DECLARED", "changed-synthetic")
			}
		}}
		return []string{"E1_DECLARED=frozen-synthetic"}
	})
}
func TestDeclaredEnvironmentPublicCarryWriters(t *testing.T) {
	value := rand.Text()
	t.Setenv("E1_CARRY_AMBIENT", value)
	t.Setenv("BASH_FUNC_which%%", value)
	b, owners, args, _ := carryRecoveryBed(t)
	code, _ := b.runJSON(owners, args...)
	if code != 0 {
		t.Fatal("carry check failed")
	}
	root := filepath.Join(b.root(), "artifacts", "unit-checks", "carry", args[3])
	found := assertNoAmbientBytes(t, root, value)
	if !found["subject.json"] || !found["result.json"] {
		t.Fatal("carry writers not exercised")
	}
	data, err := os.ReadFile(filepath.Join(root, "subject.json"))
	var check launch.UnitCheck
	if err != nil || json.Unmarshal(data, &check) != nil || check.Input == nil || !slices.Contains(check.Input.Dropped, "BASH_FUNC_which%%") {
		t.Fatal("carry snapshot lost the dropped ambient name")
	}
}
