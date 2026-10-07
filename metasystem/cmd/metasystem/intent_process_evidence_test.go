package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/designgate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter/fakeadapter"
)

func processEstimatePage(t *testing.T, bed *workBed, elapsed, check string) (string, []byte) {
	t.Helper()
	path, data := designGatePage(t, bed, "- Critique: ruled by Wido 2026-10-07, accepted\n")
	data = append(data, []byte("\n## Estimates\n\n| Unit | Estimated elapsed minutes | Expected impacted-check minutes | Production lines |\n| --- | ---: | ---: | ---: |\n| other | 999 | 99 | 250 |\n| evidence | "+elapsed+" | "+check+" | 250 |\n")...)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	processCommittedPage(t, bed, path, data)
	return path, data
}

func processCommittedPage(t *testing.T, bed *workBed, path string, data []byte) {
	t.Helper()
	relative, err := filepath.Rel(bed.root(), path)
	if err != nil {
		t.Fatal(err)
	}
	bed.workOwnersHook = func(owners *intentWorkOwners) {
		git := owners.git
		owners.git = func(root string, args ...string) ([]byte, error) {
			if slices.Equal(args, []string{"show", "origin/main:" + filepath.ToSlash(relative)}) {
				if root != bed.root() {
					t.Fatalf("accepted page read from %s, want %s", root, bed.root())
				}
				return append([]byte(nil), data...), nil
			}
			return git(root, args...)
		}
	}
}

func processEvidenceBuild(bed *workBed) (int, intentResult, string) {
	brief := bed.brief("estimate.md", "Build the unit.\n\n| Unit | Lines |\n| --- | --- |\n| evidence | 40 |\n")
	check := append([]string{"--check"}, slices.Insert(workArgv, 2, "-timeout", "30m")...)
	return bed.work(append([]string{"work", "build", bed.id, "evidence", "--brief", brief}, check...)...)
}

func TestProcessEstimatePublicBuild(t *testing.T) {
	t.Parallel()
	first := newWorkBed(t)
	first.lineage = "builder"
	_, data := processEstimatePage(t, first, "17.5", "2.5")
	code, built, output := processEvidenceBuild(first)
	if code != 0 {
		t.Fatalf("build exit %d: %+v %s", code, built, output)
	}
	wantDigest := fmt.Sprintf("%x", sha256.Sum256(data))
	for _, bed := range []*workBed{first, newWorkBed(t)} {
		if bed != first {
			bed.unitRoot = first.unitRoot
			bed.lineage = "builder"
			processEstimatePage(t, bed, "90", "-")
		}
		code, result, output := processEvidenceBuild(bed)
		if code != 0 {
			t.Fatalf("repeat/new run exit %d: %+v %s", code, result, output)
		}
		plan, err := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
		if err != nil {
			t.Fatal(err)
		}
		estimate := plan.Estimate
		if estimate == nil || estimate.DesignID != "gate-design" || estimate.Unit != "evidence" || estimate.SourceSHA256 != wantDigest || estimate.BodySHA256 == "" || estimate.ElapsedMinutes != 17.5 || estimate.CheckMinutes == nil || *estimate.CheckMinutes != 2.5 {
			t.Fatalf("lost first design-row estimate (brief says 40 lines): %+v", estimate)
		}
		if bed == first && resultData(t, result)["run"] != resultData(t, built)["run"] {
			t.Fatal("repeat created another run")
		}
		if bed != first && resultData(t, result)["run"] == resultData(t, built)["run"] {
			t.Fatal("new worktree did not create a distinct run")
		}
	}
	planPath := resultData(t, built)["plan"].(string)
	planBytes := mustRead(t, planPath)
	for _, damaged := range []string{
		strings.Replace(string(planBytes), `"elapsedMinutes": 17.5`, `"elapsedMinutes": -1`, 1),
		strings.Replace(string(planBytes), `"designId": "gate-design"`, `"designId": "gate-design", "unknownEstimateField": true`, 1),
	} {
		if err := os.WriteFile(planPath, []byte(damaged), 0o600); err != nil {
			t.Fatal(err)
		}
		if code, result, _ := first.work("work", "build", "run:"+resultData(t, built)["run"].(string)); code != 1 || !strings.Contains(resultWords(result), "UNIT_PLAN_INVALID") {
			t.Fatalf("strict plan decoder accepted damaged estimate: %d %+v", code, result)
		}
	}
}

func TestProcessEstimateAdmissionPublicBuild(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"missing", "changed", "bad minutes", "optional check"} {
		for _, person := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/person=%t", scenario, person), func(t *testing.T) {
				t.Parallel()
				bed := newWorkBed(t)
				if !person {
					bed.lineage = "builder"
				}
				elapsed, check := "17", "2"
				if scenario == "bad minutes" {
					elapsed = "NaN"
				}
				if scenario == "optional check" {
					check = "-"
				}
				if scenario != "missing" {
					path, data := processEstimatePage(t, bed, elapsed, check)
					if scenario == "changed" {
						bed.designGate.chains = func(string, string, string) ([]designgate.Chain, error) {
							if err := os.WriteFile(path, append(data, []byte("Changed after the accepted snapshot.\n")...), 0o600); err != nil {
								return nil, err
							}
							return nil, nil
						}
					}
				}
				code, result, output := processEvidenceBuild(bed)
				if !person && scenario == "changed" {
					if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "estimate unavailable") || len(bed.starter.launched()) != 0 || len(bed.runDirectories()) != 0 {
						t.Fatalf("agent was not held before launch: %d %+v %s", code, result, output)
					}
					return
				}
				if code != 0 || !slices.Contains(bed.starter.launched(), "build") {
					t.Fatalf("person/optional estimate held: %d %+v %s", code, result, output)
				}
				plan, err := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
				if err != nil {
					t.Fatal(err)
				}
				if scenario == "optional check" {
					if plan.Estimate == nil || plan.Estimate.CheckMinutes != nil {
						t.Fatalf("optional check minutes: %+v", plan.Estimate)
					}
				} else if plan.Estimate != nil || !strings.Contains(output, "estimate unavailable") {
					t.Fatalf("unavailable estimate was fabricated: %+v %s", plan.Estimate, output)
				}
				if scenario == "missing" {
					if person {
						// An older admission may have retained a null estimate.
						key := fmt.Sprintf("%x", sha256.Sum256([]byte("01M4189Q0RH1NSPD3PNAS6G177\x00"+bed.id+"\x00evidence")))
						path := filepath.Join(filepath.Dir(filepath.Dir(plan.Path)), ".estimates", key+".json")
						if err := os.WriteFile(path, []byte("null\n"), 0o600); err != nil {
							t.Fatal(err)
						}
					}
					next := newWorkBed(t)
					next.unitRoot, next.lineage = bed.unitRoot, "builder"
					processEstimatePage(t, next, "90", "2")
					code, result, _ := processEvidenceBuild(next)
					if code != 0 || len(next.starter.launched()) == 0 {
						t.Fatalf("later row did not repair unavailable first admission: %d %+v", code, result)
					}
					retained, err := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
					if err != nil || retained.Estimate == nil || retained.Estimate.ElapsedMinutes != 90 {
						t.Fatalf("later row did not become the denominator: %+v %v", retained.Estimate, err)
					}
				}
			})
		}
	}
}

func TestProcessEstimateCommittedPagePublicBuild(t *testing.T) {
	t.Parallel()
	for _, person := range []bool{false, true} {
		t.Run(fmt.Sprintf("person=%t", person), func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			if !person {
				bed.lineage = "builder"
			}
			path, committed := processEstimatePage(t, bed, "17", "2")
			changed := strings.Replace(string(committed), "| evidence | 17 |", "| evidence | 90 |", 1)
			if err := os.WriteFile(path, []byte(changed), 0o600); err != nil {
				t.Fatal(err)
			}
			code, result, output := processEvidenceBuild(bed)
			if !person {
				if code != 1 || !strings.Contains(result.Summary, "estimate unavailable") || len(bed.starter.launched()) != 0 || len(bed.runDirectories()) != 0 {
					t.Fatalf("edited estimate admitted an agent: %d %+v %s", code, result, output)
				}
				// Restoring the accepted bytes is a usable remedy for the hold.
				if err := os.WriteFile(path, committed, 0o600); err != nil {
					t.Fatal(err)
				}
				code, result, output = processEvidenceBuild(bed)
			}
			if code != 0 || !slices.Contains(bed.starter.launched(), "build") {
				t.Fatalf("person/restored build held: %d %+v %s", code, result, output)
			}
			plan, err := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
			if err != nil {
				t.Fatal(err)
			}
			if person {
				if plan.Estimate != nil || !strings.Contains(output, "estimate unavailable") {
					t.Fatalf("person retained an edited estimate: %+v %s", plan.Estimate, output)
				}
			} else if plan.Estimate == nil || plan.Estimate.ElapsedMinutes != 17 || plan.Estimate.SourceSHA256 != fmt.Sprintf("%x", sha256.Sum256(committed)) {
				t.Fatalf("restored build lost committed estimate: %+v", plan.Estimate)
			}
		})
	}
	bed := newWorkBed(t)
	bed.lineage = "builder"
	path, committed := designGatePage(t, bed, "- Critique: ruled by Wido 2026-10-07, accepted\n")
	processCommittedPage(t, bed, path, committed)
	if code, result, _ := processEvidenceBuild(bed); code != 0 {
		t.Fatalf("missing committed row held: %d %+v", code, result)
	}
	next := newWorkBed(t)
	next.lineage = "builder"
	bed = next
	path, committed = designGatePage(t, bed, "- Critique: ruled by Wido 2026-10-07, accepted\n")
	_, local := processEstimatePage(t, bed, "90", "2")
	processCommittedPage(t, bed, path, committed)
	if code, result, _ := processEvidenceBuild(bed); code != 1 || !strings.Contains(result.Summary, "estimate unavailable") || len(bed.starter.launched()) != 0 {
		t.Fatalf("local row bypassed held admission: %d %+v local=%s", code, result, local)
	}
}

func TestProcessEstimatePlanAdmissionPublicBuild(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"missing agent", "missing person", "accepted", "forged", "null"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			if scenario != "missing person" {
				bed.lineage = "builder"
			}
			if scenario != "missing agent" && scenario != "missing person" {
				processEstimatePage(t, bed, "17", "2")
			}
			brief := filepath.Join(bed.root(), bed.brief("plan-brief.md", "| Unit | Lines |\n| --- | --- |\n| evidence | 40 |\n"))
			plan := launch.UnitPlan{Unit: "evidence", Goal: bed.id, Worktree: bed.worktree, Base: bed.head,
				Build: launch.UnitBuildPlan{Brief: brief, Inputs: []string{}, Outputs: []string{}, UnitsPage: brief, Units: []string{"evidence"}},
				Proof: []launch.ProofCommand{{Name: "check", Dir: bed.worktree, Argv: slices.Insert(workArgv, 2, "-timeout", "30m"), Env: []string{}}}}
			data, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "forged" || scenario == "null" {
				estimate := "null"
				if scenario == "forged" {
					encoded, err := json.Marshal(launch.UnitEstimate{DesignID: "gate-design", Unit: "evidence", SourceSHA256: strings.Repeat("a", 64), BodySHA256: strings.Repeat("b", 64), ElapsedMinutes: 900})
					if err != nil {
						t.Fatal(err)
					}
					estimate = string(encoded)
				}
				data = []byte(strings.TrimSuffix(string(data), "}") + `,"estimate":` + estimate + "}")
			}
			path := filepath.Join(bed.root(), "caller-plan.json")
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			code, result, output := bed.work("work", "build", "--plan", path)
			switch scenario {
			case "forged", "null":
				if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, `unknown field "estimate"`) || len(bed.starter.launched()) != 0 || len(bed.runDirectories()) != 0 {
					t.Fatalf("caller estimate field accepted: %d %+v %s", code, result, output)
				}
			default:
				if code != 0 || !slices.Contains(bed.starter.launched(), "build") {
					t.Fatalf("lawful plan held: %d %+v %s", code, result, output)
				}
				retained, err := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
				if err != nil {
					t.Fatal(err)
				}
				if scenario == "accepted" && (retained.Estimate == nil || retained.Estimate.ElapsedMinutes != 17) || (scenario == "missing person" || scenario == "missing agent") && (retained.Estimate != nil || !strings.Contains(output, "estimate unavailable")) {
					t.Fatalf("wrong plan estimate: %+v %s", retained.Estimate, output)
				}
			}
		})
	}
}

type evidenceStarter struct {
	*workStarter
	start, end string
}

func (s evidenceStarter) StartSupervisor(id, root string) (identity.Ref, error) {
	ref, err := s.workStarter.StartSupervisor(id, root)
	if err == nil {
		_, err = s.m.Store.Update(id, func(record *launch.Record) error {
			record.StartedAt, record.FinishedAt = s.start, s.end
			return nil
		})
	}
	return ref, err
}

func TestProcessStepEvidencePublicBuild(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	start := time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
	end, collected := start.Add(time.Minute), start.Add(3*time.Minute)
	stamp := func(value time.Time) string { return value.Format(time.RFC3339Nano) }
	bed.manager.Now = func() time.Time { return collected }
	bed.manager.Supervisor = evidenceStarter{bed.starter, stamp(start), stamp(end)}
	fake := fakeadapter.New()
	fake.Steps = []adapter.GateStep{{Name: "display-name", Args: []string{"selected-tool", "--one", "two words"}}}
	fake.Scripted.Root = t.TempDir()
	bed.testingAdapter = workTestingAdapter{fake, func(string, string, string) (adapter.Closure, error) { return fake.Scripted, nil }}
	brief := bed.brief("evidence.md", "Read each round: yes\nBuild the unit.\n")
	selectedArgv := slices.Insert(workArgv, 2, "-timeout", "30m")
	args := append([]string{"work", "build", bed.id, "evidence", "--brief", brief, "--lines", "40", "--read-tool-calls", "12", "--check"}, selectedArgv...)
	code, built, output := bed.work(args...)
	if code != 0 {
		t.Fatalf("build exit %d: %+v %s", code, built, output)
	}
	run := resultData(t, built)["run"].(string)
	runner := &launch.UnitRunner{Manager: bed.manager, Root: bed.unitRoot}
	check := func(roundNumber int) {
		t.Helper()
		record, err := runner.Status(run)
		if err != nil {
			t.Fatal(err)
		}
		round := record.Rounds[roundNumber-1]
		if len(round.Steps) != 4 {
			t.Fatalf("step count: %+v", round.Steps)
		}
		for index, step := range round.Steps {
			kind := []string{"build", "attest", "attest", "read"}[index]
			if roundNumber == 2 && index == 0 {
				kind = "correction"
				if step.RevisionAfter != 1 {
					t.Fatalf("correction lost admitted revision: %+v", step)
				}
			}
			if step.Kind != kind || step.ExecutionStartedAt != stamp(start) || step.ExecutionEndedAt != stamp(end) || step.FinishedAt != stamp(collected) {
				t.Fatalf("step lost execution or collection evidence: %+v", step)
			}
			if index == 1 || index == 2 {
				want := fake.Steps[0].Args
				if index == 2 {
					want = selectedArgv
				}
				if step.Command == nil || !slices.Equal(step.Command.Argv, want) || "proof:"+step.Command.Name != step.Name || step.Command.Dir != []string{fake.Scripted.Root, bed.worktree}[index-1] {
					t.Fatalf("selected command identity: %+v", step)
				}
			}
		}
	}
	check(1)
	if code, _, _ := bed.work(args...); code != 0 {
		t.Fatalf("repeat exit %d", code)
	}
	check(1)
	fix := bed.brief("correction.md", "Correct the unit.\n")
	if code, result, output := bed.work("work", "revise", bed.id, "--work", "evidence", "--after", "1", "--brief", filepath.Join(bed.root(), fix)); code != 0 {
		t.Fatalf("correction exit %d: %+v %s", code, result, output)
	}
	check(2)
}
