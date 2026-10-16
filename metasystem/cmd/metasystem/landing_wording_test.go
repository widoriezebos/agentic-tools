package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestLandingStatusToDoImmediatelyFollowsHeadline(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"held", "stopped", "unready", "unready spaced", "question", "proving", "starting", "failed", "resolving", "fixing", "resolved", "reviewing", "idle"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			bed, _, _, _, _ := landingRestartBed(t)
			owners := bed.owners()
			owners.landing.status = func(_ string, _ lane.Record, view lane.View) landingStatusData {
				view.Owner.State = lane.OwnerIdle
				status := plain.Status{View: view}
				switch state {
				case "held":
					status.Owner.State = lane.OwnerHeld
					status.Owner.Barren = 2
					status.Summary = view.Summary
				case "stopped":
					by := "person"
					status.Owner.State = lane.OwnerStopped
					status.Owner.StoppedBy = &by
					status.Summary = view.Summary
				case "unready", "unready spaced":
					status.Owner.State = lane.OwnerUnready
					status.Owner.Fix = []string{"metasystem", "system", "start", "--repo", "fixture"}
					if state == "unready spaced" {
						status.Owner.Fix[4] = "fixture with space"
					}
					status.Summary = view.Summary
				case "question":
					status.Summary = "Waiting for a person's answer to question q; run: metasystem landing resume"
				case "resolving", "fixing", "resolved", "reviewing":
					status.Summary = state + " g; nothing to do"
				case "proving":
					status.Summary = "proving main (7.0 min); nothing to do"
					status.RunningProof = &plain.RunningProof{State: "running"}
				case "starting":
					status.Summary = "starting the check; nothing to do"
					status.RunningProof = &plain.RunningProof{State: "pending"}
				case "failed":
					status.Summary = "the lane's check failed; run: metasystem landing resume"
					status.RunningProof = &plain.RunningProof{State: "failed"}
				}
				return status
			}
			command, _ := findIntentAction("landing", "status")
			var out, errOut bytes.Buffer
			code := runIntentIn(command, nil, &out, &errOut, bed.cwd, owners)
			sections := strings.Split(strings.TrimSpace(out.String()), "\n\n")
			want := "metasystem landing resume"
			if state == "stopped" {
				want = "metasystem landing resume"
			}
			if state == "unready" {
				want = "metasystem system start --repo fixture"
			}
			if state == "unready spaced" {
				want = "metasystem system start --repo 'fixture with space'"
			}
			act := state == "failed" || state == "held" || state == "stopped" || state == "question" || state == "unready" || state == "unready spaced"
			if code != 0 || act && (len(sections) < 2 || strings.TrimSpace(sections[1]) != "to do: "+want) || !act && strings.Contains(out.String(), "to do:") {
				t.Fatalf("state=%s exit=%d output=%q error=%q", state, code, out.String(), errOut.String())
			}
			t.Logf("%s: exit=%d output=%s", state, code, out.String())
		})
	}
}

func TestLandingRefusalTextNamesIntentAndJSONRetainsCommand(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"metasystem landing run --goals g", "metasystem landing prove --gate", "metasystem landing prove --classify 20261012T123456", "metasystem landing prove --trunk", "git config metasystem.goal.machine fixture", "metasystem landing return g --cause other --reason TEXT", "metasystem work land g --exception GOAL_LAND_TRUNK_RED --reason TEXT", "metasystem landing push", "metasystem system start --repo fixture", "metasystem helm return --repo fixture", "sh -c go run ./cmd/devgate build", "metasystem landing set PATH", "metasystem landing unset --force"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			bed, _, _, _, _ := landingRestartBed(t)
			descriptor, _ := findIntentAction("landing", "run")
			input, problem := parseIntentArgs(descriptor, nil)
			if problem != nil {
				t.Fatal(problem)
			}
			var out, errOut bytes.Buffer
			inv := &intentInvocation{command: descriptor, input: input, stdout: &out, stderr: &errOut, cwd: bed.cwd, owners: bed.owners(), laneCallerObserved: true, laneCallerPerson: true}
			result := intentResult{Outcome: intentRefused, code: 1, Summary: "the lane needs a decision", next: strings.Fields(command), nextReason: "use the enrolled terminal; then retry"}
			if code := inv.render(result); code != 1 || !strings.Contains(oneSpaced(errOut.String()), lane.Remedy(command)) || !strings.Contains(oneSpaced(errOut.String()), "use the enrolled terminal; then retry") {
				t.Fatalf("text exit=%d out=%q err=%q", code, out.String(), errOut.String())
			}
			if command != lane.Remedy(command) && strings.Contains(errOut.String(), command) {
				t.Fatalf("mechanism command in text: %s", errOut.String())
			}
			out.Reset()
			errOut.Reset()
			input, problem = parseIntentArgs(descriptor, []string{"--json"})
			if problem != nil {
				t.Fatal(problem)
			}
			inv.input = input
			if code := inv.render(result); code != 1 {
				t.Fatal(code)
			}
			var decoded struct{ Next *intentNext }
			if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Next == nil || strings.Join(decoded.Next.Argv, " ") != command {
				t.Fatalf("lost execution command: %s", out.String())
			}
		})
	}
}

func TestLandingWordingRealRefusalProducers(t *testing.T) {
	t.Parallel()
	for _, row := range layoutCases() {
		if len(row.args) < 2 || row.args[0] != "landing" || strings.Contains(row.name, "verbose") {
			continue
		}
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			bed, _ := layoutPrepared(t, row, false)
			_, out, errOut := runLayoutCase(t, row, bed)
			text := oneSpaced(out + errOut)
			for _, word := range []string{"barren", "recorded selection", "selection recorded", "recorded selection changed", "fingerprint", "sealed window", "admission", "GOAL_LAND_TRUNK_RED", "--exception", "landing run --goals"} {
				if strings.Contains(text, word) {
					t.Errorf("mechanism %q: %s", word, text)
				}
			}
			if row.name == "landing-start-refusal" && !strings.Contains(text, "metasystem landing set PATH") {
				t.Fatal("lost actual prerequisite: " + text)
			}
			if row.name == "landing-stop" || row.name == "landing-drain" || row.name == "landing-return-refusal" {
				if strings.Contains(text, "→") {
					t.Fatal("finished or absent work asks for another act: " + text)
				}
			}
		})
	}
}

func TestLandingWordingAgentRemediesStayExecutable(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"prove", "push", "resolve", "return"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			bed, _, _, _, _ := landingRestartBed(t)
			descriptor, _ := findIntentAction("landing", action)
			input, problem := parseIntentArgs(descriptor, nil)
			if problem != nil {
				t.Fatal(problem)
			}
			var out, errOut bytes.Buffer
			inv := &intentInvocation{command: descriptor, input: input, stdout: &out, stderr: &errOut, cwd: bed.cwd, owners: bed.owners(), laneCallerObserved: true, laneCallerPerson: false}
			command := "metasystem landing return g --cause own --reason defect"
			result := intentResult{Outcome: intentRefused, code: 1, Summary: "the proof failed", next: strings.Fields(command), nextReason: "returns the defective goal"}
			if code := inv.render(result); code != 1 || !strings.Contains(oneSpaced(errOut.String()), command) || !strings.Contains(oneSpaced(errOut.String()), result.nextReason) {
				t.Fatalf("%d: %s", code, &errOut)
			}
		})
	}
}

func TestLandingWordingKeepsPrerequisiteArgumentBoundaries(t *testing.T) {
	t.Parallel()
	bed, _, _, _, _ := landingRestartBed(t)
	descriptor, _ := findIntentAction("landing", "run")
	input, problem := parseIntentArgs(descriptor, nil)
	if problem != nil {
		t.Fatal(problem)
	}
	var out, errOut bytes.Buffer
	inv := &intentInvocation{command: descriptor, input: input, stdout: &out, stderr: &errOut, cwd: bed.cwd, owners: bed.owners(), laneCallerObserved: true, laneCallerPerson: true}
	result := intentResult{Outcome: intentRefused, code: 1, Summary: "the checkout needs its machine name", next: []string{"git", "-C", "fixture with space", "config", "metasystem.goal.machine", "landing"}, nextReason: "names this checkout"}
	if code := inv.render(result); code != 1 || !strings.Contains(oneSpaced(errOut.String()), "git -C 'fixture with space' config metasystem.goal.machine landing") || !strings.Contains(errOut.String(), result.nextReason) {
		t.Fatalf("%d: %s", code, &errOut)
	}
}
