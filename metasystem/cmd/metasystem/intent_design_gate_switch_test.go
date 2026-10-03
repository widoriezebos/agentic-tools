package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/designgate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
)

func setDesignGateMode(t *testing.T, bed *workBed, mode string) {
	t.Helper()
	path := filepath.Join(bed.stateRoot(), "metasystem.conf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, []byte("\ndesign.gate.mode="+mode+"\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDesignGateRefusesOnlyWhenSwitchedOn(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"warn", "refuse", "standing", "allowed", "person", "explicit lineage", "missing critique", "open critique"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed := newDesignGateBed(t, 2)
			bed.lineage = "builder"
			mode := "refuse"
			if scenario == "warn" {
				mode = "warn"
			}
			setDesignGateMode(t, bed, mode)
			if scenario == "standing" || scenario == "open critique" {
				designGatePage(t, bed, "- Critique: closed at round 2 on 0 material findings (WHO)")
			} else if scenario == "missing critique" {
				designGatePage(t, bed, "")
			}
			if scenario == "open critique" {
				bed.designGate.chains = func(string, string, string) ([]designgate.Chain, error) { return []designgate.Chain{{Round: 3}}, nil }
			}
			if scenario == "allowed" {
				bed.lineage = ""
				code, result := bed.runJSON(bed.terminalOwners(), "goal", "allow", bed.id, goal.PermissionBuildWithoutDesign, "--reason", "Use the brief", "--fixture-human-authority")
				if code != 0 || result.Outcome != intentConfirmed {
					t.Fatalf("allow: %d %+v", code, result)
				}
				bed.lineage = "builder"
			}
			if scenario == "person" || scenario == "explicit lineage" {
				bed.lineage = ""
			}
			brief := bed.brief("gate-brief.md", "Build the gate.\n")
			args := []string{"work", "build", bed.id, "u", "--brief", brief, "--lines", "40"}
			if scenario == "explicit lineage" {
				args = append(args, "--lineage", "builder")
			}
			args = append(args, workCheck...)
			code, result, output := bed.work(args...)
			refused := scenario == "refuse" || scenario == "explicit lineage" || scenario == "missing critique" || scenario == "open critique"
			if refused {
				want := "metasystem goal allow " + bed.id + " build-without-design --reason TEXT"
				if code != 1 || result.Outcome != intentRefused || !strings.HasSuffix(result.Summary, "; nothing was built") || result.Next == nil || strings.Join(result.Next.Argv, " ") != want || len(bed.starter.launched()) != 0 {
					t.Fatalf("refusal: code=%d result=%+v launches=%v output=%q", code, result, bed.starter.launched(), output)
				}
				if _, err := os.Stat(bed.unitRoot); !os.IsNotExist(err) {
					t.Fatalf("refused build created a unit store: %v", err)
				}
				verboseArgs := append(append([]string(nil), args[:len(args)-len(workCheck)]...), "--verbose")
				code, _, text := bed.run(bed.workOwners(), append(verboseArgs, workCheck...)...)
				if code != 1 || !strings.Contains(text, want) || !strings.Contains(text, "BUILD_DESIGN_NOT_ACCEPTED goal="+bed.id+" verdict=") || !strings.Contains(text, "governed-by=") {
					t.Fatalf("verbose refusal: code=%d text=%q", code, text)
				}
				return
			}
			if code != 0 || len(bed.starter.launched()) == 0 {
				t.Fatalf("build: code=%d result=%+v output=%q", code, result, output)
			}
			identity, _ := bed.designGate.identity(bed.stateRoot())
			_, record := designGateRead(t, bed, identity, "u")
			if record.Mode != mode || record.Person != (scenario == "person") {
				t.Fatalf("dispatch record: %+v", record)
			}
			if scenario == "allowed" && (record.Verdict != "allowed" || record.WouldRefuse || output != "") {
				t.Fatalf("allowed build: record=%+v output=%q", record, output)
			}
			if scenario == "person" && !strings.Contains(output, "; it goes on at your word\n") {
				t.Fatalf("person's warning: %q", output)
			}
		})
	}
}

func TestDesignGateNeverStallsWhenItBreaksUnderRefuse(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"chain", "jobs folder", "identity", "invalid identity", "record", "digest", "unreadable mode", "invalid mode"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			bed := newDesignGateBed(t, 2)
			bed.lineage = "builder"
			mode := "refuse"
			if failure == "invalid mode" {
				mode = "invalid"
			}
			setDesignGateMode(t, bed, mode)
			designGatePage(t, bed, "- Critique: closed at round 2 on 0 material findings (WHO)")
			problem := errors.New("fixture failure")
			brokenCheck := failure == "chain" || failure == "jobs folder" || failure == "digest" || failure == "unreadable mode" || failure == "invalid mode"
			switch failure {
			case "chain", "digest":
				bed.designGate.chains = func(string, string, string) ([]designgate.Chain, error) { return nil, problem }
				if failure == "digest" {
					bed.designGate.digest = func(string, narratordigest.Entry, time.Time) error { return problem }
				}
			case "jobs folder":
				bed.designGate.chains = nil
				parent := filepath.Join(bed.stateRoot(), "artifacts", "agents")
				if err := os.MkdirAll(parent, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(parent, "jobs"), []byte("not a folder"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "identity":
				bed.designGate.identity = func(string) (string, error) { return "", problem }
			case "invalid identity":
				bed.designGate.identity = func(string) (string, error) { return "../../escape", nil }
			case "record":
				bed.designGate.record = func(string, string, string) (bool, error) { return false, problem }
			case "unreadable mode":
				bed.config = func(string, string) (string, string, int, error) { return "", "", 1, problem }
			}
			result, output := designGateBuild(t, bed, "u")
			if brokenCheck {
				if !strings.Contains(output, "warning: the design check could not run (") || !strings.Contains(output, "); this build was not checked\nmetasystem design list --goal "+bed.id+"\n") {
					t.Fatalf("broken check pair: %q", output)
				}
				gate := resultData(t, result)["designGate"].(map[string]any)
				if gate["verdict"] != "unchecked" || gate["wouldRefuse"] != false || (failure == "invalid mode" || failure == "unreadable mode") && gate["mode"] != "warn" {
					t.Fatalf("broken check verdict: %v", gate)
				}
			}
			if !brokenCheck || failure == "digest" {
				if !strings.Contains(output, "warning: the design check's record could not be written (") || !strings.Contains(output, "); the build goes on\nnothing to do: the landing check runs without it\n") {
					t.Fatalf("broken writer pair: %q", output)
				}
			}
			inputs := resultData(t, result)["inputs"].(string)
			for _, name := range []string{"request.json", "plan.json", "build-brief.md", "read-brief.md"} {
				if _, err := os.Stat(filepath.Join(inputs, name)); err != nil {
					t.Fatalf("launch input %s: %v", name, err)
				}
			}
		})
	}
}

func TestAllowBuildWithoutDesignShowsAndRecordsItsImpact(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, makeQueued)
	want := "This lets goal " + bedGoal + " be built and landed without an accepted, reviewed design.\nThe risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands.\nIt holds for every later build and landing of " + bedGoal + " until withdrawn.\nTo undo: metasystem goal disallow " + bedGoal + " build-without-design\n"
	args := []string{"goal", "allow", bedGoal, "build-without-design"}
	before := bed.publications()
	code, _, output := bed.run(bed.terminalOwners(), args...)
	if code != 2 || !strings.HasPrefix(output, want) || bed.publications() != before || bed.goalFile(bedGoal).DesignGateOff {
		t.Fatalf("without reason: code=%d output=%q", code, output)
	}
	bed.lineage = "builder"
	code, result := bed.runJSON(bed.owners(), append(args, "--reason", "Use the brief")...)
	if code != 1 || bed.publications() != before || result.Outcome != intentRefused {
		t.Fatalf("agent allowance: code=%d result=%+v", code, result)
	}
	bed.lineage = ""
	command, rest, _ := resolveIntentArgv(append(args, "--reason", "Use the brief", "--fixture-human-authority"))
	var combined bytes.Buffer
	code = runIntentIn(command, rest, &combined, &combined, bed.root(), bed.terminalOwners())
	file := bed.goalFile(bedGoal)
	impact := strings.Join(strings.Fields(want), " ")
	if code != 0 || !strings.HasPrefix(combined.String(), want) || combined.Len() <= len(want) || bed.publications() != before+1 || !file.DesignGateOff || !strings.Contains(string(goal.RenderFile(file)), "\n- DesignGate: off\n") || file.History[len(file.History)-1].Reason != "Allowed: build-without-design why=Use the brief impact="+impact {
		t.Fatalf("allowance: code=%d output=%q record=%s", code, combined.String(), goal.RenderFile(file))
	}
	bed.lineage = "builder"
	code, result = bed.runJSON(bed.owners(), "goal", "disallow", bedGoal, "build-without-design")
	if code != 0 || result.Outcome != intentConfirmed || bed.goalFile(bedGoal).DesignGateOff || strings.Contains(string(goal.RenderFile(bed.goalFile(bedGoal))), "- DesignGate:") {
		t.Fatalf("withdrawal: code=%d result=%+v", code, result)
	}
}
