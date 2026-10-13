package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestSharedEnrollmentGoalDoneAndLandingStart(t *testing.T) {
	t.Parallel()
	if testenv.IsolateMachine(t) {
		return
	}
	landed := func(file *goal.GoalFile) {
		humanOrigin(file)
		file.Landing = &goal.LandingRecord{At: file.Claimed.At, Opid: "01J5X00000000000000000FD00-mac-a-12345678"}
	}
	seat := newIntentBed(t, false, landed)
	laneInstall := newIntentBed(t, false, landed)
	// Both installations observe the same repository's immutable commits.
	laneInstall.repo = seat.repo
	_, reader := enrollGoalSyncTerminal(t, seat.root(), "ttys:shared-shell")
	prove := func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
		return humanauthority.Prove(root, reader.exact.Pid, reader, at)
	}
	for _, install := range []*intentBed{laneInstall, seat} {
		install.facts.reader = &reader
		owners := install.owners()
		owners.prove = prove
		code, result := install.runJSON(owners, "goal", "done", bedGoal, "--reason", "The landed work is complete")
		if code != 0 || (result.Outcome != intentConfirmed && result.Outcome != intentUnchanged) {
			t.Fatalf("goal done from %s: exit=%d %+v", install.root(), code, result)
		}
		if file := install.goalFile(bedGoal); file.State != goal.StateDone || file.History[len(file.History)-1].Actor != "human:Wido" {
			t.Fatalf("conclusion lost the machine's person: %+v", file)
		}
	}
	landing := newLaneVerbBed(t)
	if code, _, stderr := landing.run(t, "landing", "set", landing.landingA); code != 0 {
		t.Fatalf("register test lane: %d %s", code, stderr)
	}
	for _, root := range []string{landing.landingA, landing.landingB} {
		landing.cwd = root
		if _, err := lane.SetPause(landing.home, "Wido", laneTestNow); err != nil {
			t.Fatal(err)
		}
		owners := landing.owners()
		owners.landing.person = provenPerson(reader, func() int64 { return reader.exact.Pid }, owners.commandNow)
		command := mustIntentCommand(t, "landing start")
		var output bytes.Buffer
		if code := runIntentIn(command, nil, &output, &output, root, owners); code != 0 {
			t.Fatalf("landing start from %s: exit=%d %s", root, code, output.String())
		}
		if _, paused := lane.ReadPause(landing.home); paused {
			t.Fatal("the person's start left the lane paused")
		}
		output.Reset()
		if code := runIntentIn(mustIntentCommand(t, "landing stop"), []string{"--by", "Another person"}, &output, &output, root, owners); code != 0 {
			t.Fatalf("landing stop from %s: exit=%d %s", root, code, output.String())
		}
		if pause, paused := lane.ReadPause(landing.home); !paused || pause.By != "Wido" {
			t.Fatalf("the stop recorded a typed name instead of the enrolled person: %+v", pause)
		}
	}
}

type sharedEnrollmentAgentReader struct{ goalSyncEnrollmentReader }

func (r sharedEnrollmentAgentReader) Read(pid int64) (humanauthority.Snapshot, error) {
	snapshot, err := r.goalSyncEnrollmentReader.Read(pid)
	if pid == r.exact.Pid {
		snapshot.Exact.Argv = []string{"codex", "exec", "metasystem-fake-agent"}
		snapshot.Executable = "/fixture/codex"
	}
	return snapshot, err
}

func TestSharedEnrollmentRefusesAgentPersonActs(t *testing.T) {
	t.Parallel()
	if testenv.IsolateMachine(t) {
		return
	}
	source := t.TempDir()
	_, terminal := enrollGoalSyncTerminal(t, source, "ttys:shared-shell")
	agent := sharedEnrollmentAgentReader{terminal}
	prove := func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
		return humanauthority.Prove(root, terminal.exact.Pid, agent, at)
	}
	for _, verb := range []string{"done", "resume", "budget", "approve", "claim"} {
		t.Run(verb, func(t *testing.T) {
			t.Parallel()
			bed := newIntentBed(t, verb == "resume", humanOrigin)
			bed.lineage = "agent-session"
			owners := bed.owners()
			owners.prove = prove
			owners.dependencies.proveHuman = func(root string, pid int64, reader humanauthority.Reader, at time.Time) (humanauthority.Proof, error) {
				return prove(root, pid, reader, "", "", at)
			}
			owners.dependencies.proveTerminal = owners.dependencies.proveHuman
			args := []string{"goal", verb, bedGoal, "--by", "Wido"}
			if verb == "done" {
				args = append(args, "--reason", "The work is complete")
			}
			if verb == "budget" {
				args = append(args, "48h/20/2400m/4/5")
			}
			code, result := bed.runJSON(owners, args...)
			if code == 0 || !strings.Contains(strings.ToLower(result.Summary), "agent") || bed.repo.publications != 0 {
				t.Fatalf("agent's person act %s: exit=%d %+v publications=%d", verb, code, result, bed.repo.publications)
			}
		})
	}
	for _, verb := range []string{"start"} {
		t.Run("landing "+verb, func(t *testing.T) {
			t.Parallel()
			bed := newLaneVerbBed(t)
			if code, _, text := bed.run(t, "landing", "set", bed.landingA); code != 0 {
				t.Fatalf("set: %d %s", code, text)
			}
			if verb == "start" {
				if _, err := lane.SetPause(bed.home, "Wido", laneTestNow); err != nil {
					t.Fatal(err)
				}
			}
			owners := bed.owners()
			owners.landing.person = provenPerson(agent, func() int64 { return terminal.exact.Pid }, owners.commandNow)
			var output bytes.Buffer
			if code := runIntentIn(mustIntentCommand(t, "landing "+verb), nil, &output, &output, bed.landingA, owners); code == 0 || !strings.Contains(output.String(), "agent") {
				t.Fatalf("agent's landing %s: %d %s", verb, code, output.String())
			}
			if _, paused := lane.ReadPause(bed.home); paused != (verb == "start") {
				t.Fatal("refused person act changed the pause")
			}
		})
	}

}

func TestSharedEnrollmentRefusesAgentWorkLand(t *testing.T) {
	t.Parallel()
	if testenv.IsolateMachine(t) {
		return
	}
	bed, _, install := plainLaneBedWith(t, true, "critic-root", "critic-root")
	bed.lineage = "agent-session"
	_, terminal := enrollGoalSyncTerminal(t, t.TempDir(), "ttys:shared-shell")
	agent := sharedEnrollmentAgentReader{terminal}
	owners := bed.intentBed.owners()
	owners.delivery, owners.connection, owners.work = bed.owners, bed.connection, bed.work
	owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
		return humanauthority.Prove(root, terminal.exact.Pid, agent, at)
	}
	code, result := bed.runJSON(owners, "work", "land", bedGoal, "--exception", goal.LandTrunkRedCode, "--reason", "Ship", "--by", "Wido")
	if code == 0 || !strings.Contains(strings.ToLower(result.Summary), "agent") {
		t.Fatalf("agent's work land: %d %+v", code, result)
	}
	if entries, err := plain.Entries(install); err != nil || len(entries) != 0 {
		t.Fatalf("refused agent wrote a landing: %+v %v", entries, err)
	}
}

func TestSharedEnrollmentRefusesAgentLandingReturn(t *testing.T) {
	t.Parallel()
	if testenv.IsolateMachine(t) {
		return
	}
	bed, red := redPermissionBed(t)
	bed.success(t, "landing", "prove", "--classify", red.Attempt)
	_, terminal := enrollGoalSyncTerminal(t, t.TempDir(), "ttys:shared-shell")
	agent := sharedEnrollmentAgentReader{terminal}
	bed.owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
		return humanauthority.Prove(root, terminal.exact.Pid, agent, at)
	}
	if code, text := bed.run(t, "landing", "return", "a", "--cause", "own", "--reason", "Ship", "--by", "Wido"); code == 0 || !strings.Contains(strings.ToLower(text), "agent") {
		t.Fatalf("agent's landing return: %d %s", code, text)
	}
	if entry, _, err := plain.Latest(bed.installation, "a"); err != nil || entry.State == plain.StateReturned {
		t.Fatalf("refused agent returned the goal: %+v %v", entry, err)
	}
}

func TestSharedEnrollmentRealLedgerAndTestingPlan(t *testing.T) {
	t.Parallel()
	if testenv.IsolateMachine(t) {
		return
	}
	data, err := os.ReadFile("../../plans/goals/builder-proves-each-rule-by-mutation.md")
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 || file.Risk == nil || file.Origin != goal.OriginHuman {
		t.Fatalf("real ledger shape is unusable: %v", problems)
	}
	contract, err := testpolicy.Load("../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := testpolicy.Select(contract, testpolicy.SelectionRequest{ChangedPaths: []string{"metasystem/internal/humanauthority/authority.go"}, RequestedMode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery,
		GoalRisk: testpolicy.GoalRisk{Severity: int(file.Risk.Severity), Novelty: int(file.Risk.Novelty), Exposure: int(file.Risk.Exposure), Accumulation: int(file.Risk.Accumulation)}})
	if err != nil || !slices.Contains(plan.AffectedSurfaces, "human-authority-and-channel") || len(plan.SelectedGroups) == 0 {
		t.Fatalf("real testing plan did not cover human authority: %+v %v", plan, err)
	}
	encoded, _ := json.Marshal(plan)
	t.Logf("real testing plan: %s", encoded)
	source := t.TempDir()
	_, reader := enrollGoalSyncTerminal(t, source, "ttys:real-ledger")
	bed := newIntentBed(t, false, func(target *goal.GoalFile) {
		// Keep the real goal's fields and history; the fixture's manifest names
		// this goal, so its identity and history targets follow that manifest.
		*target = *file
		target.Id = bedGoal
		target.Sequence = 1
		for index := range target.History {
			target.History[index].Targets = []string{bedGoal}
		}
	})
	bed.facts.reader = &reader
	owners := bed.owners()
	owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
		return humanauthority.Prove(root, reader.exact.Pid, reader, at)
	}
	if code, result := bed.runJSON(owners, "goal", "done", bedGoal, "--reason", "The requested outcome exists"); code != 0 {
		t.Fatalf("real ledger conclusion from another install: %d %+v", code, result)
	}
	if entries, err := goal.Entries(bed.root()); err != nil || len(entries) == 0 {
		t.Fatalf("the acting install did not retain custody: %v %v", entries, err)
	}
}

func TestAuditSharedEnrollmentAgentAudiences(t *testing.T) {
	t.Parallel()
	if testenv.IsolateMachine(t) {
		return
	}
	_, terminal := enrollGoalSyncTerminal(t, t.TempDir(), "ttys:agent-audiences")
	agent := sharedEnrollmentAgentReader{terminal}
	prove := func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
		return humanauthority.Prove(root, terminal.exact.Pid, agent, at)
	}
	admitAgent := func(owners *intentOwners) {
		owners.prove = prove
		owners.dependencies.ownerLineage = func() string { return "agent-session" }
		owners.landing.person = provenPerson(agent, func() int64 { return terminal.exact.Pid }, owners.commandNow)
	}
	t.Run("landing stop", func(t *testing.T) {
		t.Parallel()
		bed := newLaneVerbBed(t)
		if code, _, text := bed.run(t, "landing", "set", bed.landingA); code != 0 {
			t.Fatalf("set: %d %s", code, text)
		}
		owners := bed.owners()
		admitAgent(&owners)
		owners.resolver = stateroot.NewResolver(fakeTop(bed.landingA), noExecutable)
		if err := os.WriteFile(filepath.Join(bed.landingA, "metasystem", "metasystem.conf"), []byte("metasystem.template=true\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := owners.resolver.ResolveLayout(bed.landingA); err != nil {
			t.Fatalf("the agent's calling checkout is unreadable: %v", err)
		}
		var output bytes.Buffer
		if code := runIntentIn(mustIntentCommand(t, "landing stop"), nil, &output, &output, bed.landingA, owners); code != 0 {
			t.Fatalf("agent could not pause the lane: %d %s", code, output.String())
		}
		if pause, paused := lane.ReadPause(bed.home); !paused || pause.By != "seat" {
			t.Fatalf("agent pause did not record its seat: %+v paused=%t", pause, paused)
		}
		output.Reset()
		if code := runIntentIn(mustIntentCommand(t, "landing start"), nil, &output, &output, bed.landingA, owners); code == 0 {
			t.Fatalf("agent resumed its own pause: %s", output.String())
		}
		if _, paused := lane.ReadPause(bed.home); !paused {
			t.Fatal("refused agent resume cleared the pause")
		}
	})
	t.Run("landing resolve", func(t *testing.T) {
		t.Parallel()
		bed := newResolveVerbFixture(t)
		admitAgent(&bed.owners)
		bed.owners.resolver = stateroot.NewResolver(fakeTop(bed.root), noExecutable)
		if err := os.WriteFile(filepath.Join(bed.install, "metasystem.conf"), []byte("metasystem.template=true\ntesting.contract=testing.json\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := bed.owners.resolver.ResolveLayout(bed.root); err != nil {
			t.Fatalf("the landing agent's calling checkout is unreadable: %v", err)
		}
		bed.owners.landing.plainResolve.Git = func(_ string, args ...string) (string, error) {
			switch strings.Join(args, " ") {
			case "show HEAD:metasystem/testing.json":
				return bed.contract, nil
			case "diff --name-only --diff-filter=U -z":
				return "", nil
			default:
				t.Fatalf("unexpected resolve Git: %v", args)
				return "", nil
			}
		}
		if code, text := bed.run(t, bed.root, "resolve"); code != 0 {
			t.Fatalf("agent could not resolve the lane: %d %s", code, text)
		}
	})
	t.Run("question withdraw", func(t *testing.T) {
		t.Parallel()
		bed := newIntentBed(t, false, nil)
		bed.lineage = "agent-session"
		owners := bed.owners()
		admitAgent(&owners)
		owners.processes.question = channel.ReadQuestion
		owners.processes.channelLink = func(string) (channel.Provider, channel.DestinationConfig) {
			return &questionProvider{}, channel.DestinationConfig{}
		}
		writeQuestionFixture(t, filepath.Join(bed.root(), "artifacts", "agents", "channel", "questions", "agent-question.json"),
			map[string]any{"id": "agent-question", "goal": bedGoal, "kind": "other", "state": "open"})
		if code, result := bed.runJSON(owners, "question", "withdraw", "agent-question", "--reason", "The agent no longer needs an answer"); code != 0 {
			t.Fatalf("agent could not withdraw its question: %d %+v", code, result)
		}
		if question, err := channel.ReadQuestion(bed.root(), "agent-question"); err != nil || question.State != "closed" {
			t.Fatalf("agent withdrawal did not close the question: %+v %v", question, err)
		}
	})
}
