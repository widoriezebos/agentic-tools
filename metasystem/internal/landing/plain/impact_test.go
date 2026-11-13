package plain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

func TestReadImpactPlanHashTracksContentAcrossRuns(t *testing.T) {
	t.Parallel()
	selection := "internal/a=TestA"
	pretty := false
	scope := "impact"
	seams := ProveSeams{
		Executable: func() (string, error) { return "/fixture/engine", nil },
		Command: func(command *exec.Cmd) error {
			if !reflect.DeepEqual(command.Args[1:], []string{"test", "impact", "--plan", "--json", "--base", "base"}) || command.Dir != "fixture" {
				t.Fatalf("plan command: %v dir=%q", command.Args, command.Dir)
			}
			if !slices.Contains(command.Env, "LANDING_PROOF_SCOPE="+scope) {
				t.Fatalf("plan scope %q missing from environment", scope)
			}
			plan := ImpactPlan{Base: "base", Subject: "batch base", Selections: []string{selection}}
			result := verbresult.FromError("test impact", 0, nil, plan)
			result.Summary = fmt.Sprintf("format %v", pretty)
			data, err := json.Marshal(result)
			if pretty {
				// Both data and envelope whitespace vary, as does envelope metadata.
				var indented bytes.Buffer
				err = json.Indent(&indented, data, "", "  ")
				data = indented.Bytes()
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := command.Stdout.Write(data); err != nil {
				t.Fatal(err)
			}
			ended := exec.Command("/usr/bin/true")
			if err := ended.Run(); err != nil {
				t.Fatal(err)
			}
			command.ProcessState = ended.ProcessState
			return nil
		},
	}
	hash := func() string {
		t.Helper()
		decision := scopeDecision{scopeRecord: scopeRecord{BaseCommit: "base", Scope: "impact"}}
		if err := decision.impactPlan(seams, "fixture"); err != nil || decision.PlanHash == "" {
			t.Fatalf("plan hash=%q error=%v", decision.PlanHash, err)
		}
		return decision.PlanHash
	}
	first := hash()
	if second := hash(); second != first {
		t.Fatalf("same plan changed hash: %s / %s", first, second)
	}
	pretty = true
	if formatted := hash(); formatted != first {
		t.Fatalf("formatting or summary changed hash: %s / %s", first, formatted)
	}
	selection = "internal/a=TestB"
	if changed := hash(); changed == first {
		t.Fatalf("changed selection kept hash %s", first)
	}
	scope = "gate"
	if _, err := ReadImpactPlan(seams, "fixture", "base", scope); err != nil {
		t.Fatal(err)
	}
}

func TestReadImpactPlanRejectsUnreadableEnvelope(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, text string }{
		{"human text", "plan: base base\nselection: internal/a\n"},
		{"missing envelope", ""},
		{"null plan", `{"schemaVersion":1,"verb":"test impact","outcome":"confirmed","data":null}`},
		{"unknown plan field", `{"schemaVersion":1,"verb":"test impact","outcome":"confirmed","data":{"base":"base","subject":"","selections":[],"unknown":true}}`},
		{"outcome contradicts exit", `{"schemaVersion":1,"verb":"test impact","outcome":"refused"}`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			seams := ProveSeams{
				Executable: func() (string, error) { return "/fixture/engine", nil },
				Command: func(command *exec.Cmd) error {
					fmt.Fprint(command.Stdout, row.text)
					ended := exec.Command("/usr/bin/true")
					if err := ended.Run(); err != nil {
						t.Fatal(err)
					}
					command.ProcessState = ended.ProcessState
					return nil
				},
			}
			decision := scopeDecision{scopeRecord: scopeRecord{BaseCommit: "base"}}
			if err := decision.impactPlan(seams, "fixture"); err == nil || decision.PlanHash != "" || !strings.Contains(err.Error(), "impact") {
				t.Fatalf("invalid plan authorized hash=%q error=%v", decision.PlanHash, err)
			}
		})
	}
}

// TestAGateStandsForASecondMergeOfItsTree merges one member again on the same
// base, as after an abandoned resolution: the gate recorded for the first
// merge commit is taken for the second, and another base or a changed impact
// plan runs the gate again (finding 556).
func TestAGateStandsForASecondMergeOfItsTree(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, verdict, parents, selection string
		reused                            bool
	}{
		{"skipped", Skipped, "main tip", "internal/a=TestA", true},
		{"green", Green, "main tip", "internal/a=TestA", true},
		{"another base", Skipped, "other tip", "internal/a=TestA", false},
		{"another plan", Skipped, "main tip", "internal/a=TestB", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			notAncestor := exec.Command("/usr/bin/false").Run()
			selection := "internal/a=TestA"
			seams := ProveSeams{
				Gate:       true,
				Now:        func() time.Time { return bedNow },
				BudgetHome: t.TempDir(),
				Executable: func() (string, error) { return "fixture-engine", nil },
				Launch:     func([]string, string, string) (int64, error) { return int64(os.Getpid()), nil },
				Incidents:  func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil },
				Command: func(command *exec.Cmd) error {
					if !slices.Contains(command.Args, "--plan") || !slices.Contains(command.Env, "LANDING_PROOF_SCOPE=gate") {
						t.Errorf("unexpected command: %v", command.Args)
					}
					data, err := json.Marshal(verbresult.FromError("test impact", 0, nil, ImpactPlan{Base: "main", Subject: "batch base", Selections: []string{selection}}))
					if err == nil {
						_, err = command.Stdout.Write(data)
					}
					ended := exec.Command("/usr/bin/true")
					if err == nil {
						err = ended.Run()
					}
					command.ProcessState = ended.ProcessState
					return err
				},
				Git: func(_ string, args ...string) (string, error) {
					switch args[0] {
					case "cat-file":
						return "", nil
					case "show":
						return row.parents, nil
					case "rev-parse":
						if args[len(args)-1] == "HEAD^{commit}" {
							return "candidate", nil
						}
						if args[len(args)-1] == "HEAD^{tree}" {
							return "tree", nil
						}
						return "main", nil
					case "rev-list":
						return "candidate main tip", nil
					case "merge-base":
						if args[3] == "candidate" && (args[2] == "main" || args[2] == "tip") {
							return "", nil
						}
						return "", notAncestor
					default:
						return "", fmt.Errorf("unexpected Git: %v", args)
					}
				},
			}
			if _, _, err := HandIn(install, Line{Goal: "goal", SHA: "tip"}); err != nil {
				t.Fatal(err)
			}
			batch := &Batch{ID: "batch", Base: "main", State: BatchPrepared, Open: true, Selector: PolicyValue{Value: "auto"}, Lane: lane.Record{Root: install, Install: install, CustodyEpoch: 1}, Members: []GoalSHA{{Goal: "goal", SHA: "tip"}}}
			if err := writeBatch(install, batch); err != nil {
				t.Fatal(err)
			}
			planned := scopeDecision{scopeRecord: scopeRecord{Scope: "gate", BaseCommit: "main"}}
			if err := planned.impactPlan(seams, install); err != nil {
				t.Fatal(err)
			}
			first := Result{Attempt: "first-gate", Requested: "first-merge", Attributed: "first-merge", Commit: "first-merge", Tree: "tree", Result: row.verdict, Static: Green, Depth: "impact",
				Scope: "gate", BaseCommit: "main", PlanHash: planned.PlanHash, At: bedNow.Format(time.RFC3339)}
			if err := appendLine(gatesPath(install), first); err != nil {
				t.Fatal(err)
			}
			selection = row.selection
			started, already, err := Start(install, install, seams)
			if err != nil || already != row.reused || row.reused && started.Attempt != first.Attempt {
				t.Fatalf("second merge of the tree: attempt=%q already=%v err=%v, want reused=%v", started.Attempt, already, err, row.reused)
			}
		})
	}
}
