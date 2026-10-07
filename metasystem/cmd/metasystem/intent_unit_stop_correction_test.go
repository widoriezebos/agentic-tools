package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

func TestIntentContinuingUnitRefusesDropAndSplitBeforeRetention(t *testing.T) {
	t.Parallel()
	for _, decision := range []string{"dropped: person proposes dropping the finding", "split: destination"} {
		t.Run(strings.Split(decision, ":")[0], func(t *testing.T) {
			t.Parallel()
			fixture := newTransferScenarioFixture(t, true)
			b, owners := fixture.bed, fixture.owners
			// The first examination had more material and no matching class.
			record, err := fixture.runner.Status(fixture.run)
			if err != nil {
				t.Fatal(err)
			}
			record.Rounds[0].Reads[0].Findings = nil
			transferWriteJSON(t, filepath.Join(b.unitRoot, fixture.run, "run.json"), record)
			code, pending := transferPublic(t, b, owners, "work", "review", b.id, "--work", "stopped")
			if code != 1 || pending.Outcome != intentInProgress {
				t.Fatalf("decisions template: %d %+v", code, pending)
			}
			record, err = fixture.runner.Status(fixture.run)
			if err != nil || record.Rounds[1].Stop.Decision != "continue" {
				t.Fatalf("unit did not continue: %+v %v", record, err)
			}
			path := resultData(t, pending)["template"].(string)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			proposal := b.brief("person-decisions.md", strings.ReplaceAll(string(before), "DECIDE", decision))
			source := filepath.Join(fixture.agents, "jobs", fixture.critic+".json")
			sourceBefore, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			code, refused := transferPublic(t, b, owners, "work", "review", b.id, "--work", "stopped", "--dispositions", proposal)
			if code == 0 || refused.Outcome != intentRefused {
				t.Fatalf("unsupported decision retained: %d %+v", code, refused)
			}
			if strings.HasPrefix(decision, "dropped:") && (!strings.Contains(strings.ToLower(refused.Summary), "drop effects are not built yet") || refused.Next == nil || !strings.Contains(strings.Join(refused.Next.Argv, " "), "work revise")) {
				t.Fatalf("drop refusal lacks working act: %+v", refused)
			}
			if strings.HasPrefix(decision, "split:") && !strings.Contains(refused.Summary, "recorded stop") {
				t.Fatalf("split refusal lacks stop requirement: %+v", refused)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("refusal retained decisions: %s %v", after, err)
			}
			sourceAfter, err := os.ReadFile(source)
			if err != nil || !bytes.Equal(sourceBefore, sourceAfter) || fixture.observer.publications != 0 || len(b.goalFile(b.id).ReviewObligations) != 0 {
				t.Fatalf("refusal changed source or completion debt: %v", err)
			}
		})
	}
}

func TestIntentPersonStopTransferRequiresProvenPerson(t *testing.T) {
	t.Parallel()
	fixture := newTransferScenarioFixture(t, true)
	b, owners := fixture.bed, fixture.owners
	owners.lookupEnv = func(key string) (string, bool) { return "person", key == config.EnvName("review.stop") }
	personProver := enrolledPersonProver(t, b.root(), b.manager.Now())
	owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, humanauthority.Refusedf(humanauthority.OutcomeAgent, "an agent started this shell")
	}
	code, pending := transferPublic(t, b, owners, "work", "review", b.id, "--work", "stopped")
	if code != 1 || pending.Outcome != intentInProgress || pending.Next == nil {
		t.Fatalf("person policy did not ask: %d %+v", code, pending)
	}
	questions, damaged := channel.WalkOpenQuestions(b.stateRoot())
	if len(damaged) != 0 || len(questions) != 2 {
		t.Fatalf("missing transfer asks: %+v %v", questions, damaged)
	}
	for _, q := range questions {
		if q.UnitStop == nil || !strings.Contains(q.UnitStop.Needs, "--by NAME") {
			t.Fatalf("ask omitted person: %+v", q)
		}
	}
	args := append([]string(nil), pending.Next.Argv[1:]...)
	code, refused := transferPublic(t, b, owners, args...)
	if code == 0 || refused.Outcome != intentRefused || fixture.observer.publications != 0 || len(b.goalFile(b.id).ReviewObligations) != 0 {
		t.Fatalf("agent transferred person-held findings: %d %+v", code, refused)
	}
	owners.prove = personProver
	for i := range args {
		if args[i] == "NAME" {
			args[i] = "Wido"
		}
	}
	code, transferred := transferPublic(t, b, owners, args...)
	if code != 0 || transferred.Outcome != intentConfirmed || fixture.observer.publications != 1 || len(b.goalFile(b.id).ReviewObligations) != 2 {
		t.Fatalf("proven person could not transfer: %d %+v", code, transferred)
	}
}

func TestIntentCorruptReviewStopNamesSettingRepair(t *testing.T) {
	t.Parallel()
	for _, admitted := range []bool{false, true} {
		t.Run(map[bool]string{false: "admission", true: "collected review"}[admitted], func(t *testing.T) {
			t.Parallel()
			fixture := newTransferScenarioFixture(t, true)
			b, owners := fixture.bed, fixture.owners
			owners.lookupEnv = func(key string) (string, bool) { return "corrupt", key == config.EnvName("review.stop") }
			args := []string{"work", "review", b.id, "--work", "stopped"}
			if !admitted {
				args = append([]string{"work", "build", b.id, "other", "--brief", b.brief("other.md", "Build another unit.\n"), "--lines", "5"}, workCheck...)
			}
			launches := len(b.starter.launched())
			code, refused := transferPublic(t, b, owners, args...)
			if code == 0 || !strings.Contains(refused.Summary, "settings set review.stop auto") || len(b.starter.launched()) != launches || fixture.observer.publications != 0 {
				t.Fatalf("corrupt policy has no actionable repair or changed work: %d %+v", code, refused)
			}
		})
	}
}

func TestIntentReviewProceedsAfterPolicyRepair(t *testing.T) {
	t.Parallel()
	fixture := newTransferScenarioFixture(t, true)
	b, owners := fixture.bed, fixture.owners
	policy := "corrupt"
	owners.lookupEnv = func(key string) (string, bool) { return policy, key == config.EnvName("review.stop") }
	args := []string{"work", "review", b.id, "--work", "stopped"}
	launches := len(b.starter.launched())
	code, refused := transferPublic(t, b, owners, args...)
	if code != 1 || refused.Outcome != intentRefused || !strings.Contains(refused.Summary, "settings set review.stop auto") {
		t.Fatalf("corrupt policy did not refuse with its repair: %d %+v", code, refused)
	}
	before, err := fixture.runner.Status(fixture.run)
	if err != nil || before.Rounds[1].Stop == nil || before.Rounds[1].Stop.Handoff != "stopped unreadable-policy" {
		t.Fatalf("unreadable policy was not retained: %+v %v", before, err)
	}
	policy = "auto"
	code, reviewed := transferPublic(t, b, owners, args...)
	if code != 0 || reviewed.Outcome != intentConfirmed || fixture.observer.publications != 1 || len(b.goalFile(b.id).ReviewObligations) != 2 {
		t.Fatalf("same review stayed blocked after policy repair: %d %+v", code, reviewed)
	}
	after, err := fixture.runner.Status(fixture.run)
	if err != nil || len(after.Rounds) != len(before.Rounds) || len(b.starter.launched()) != launches || after.Rounds[1].Stop.Attempt != before.Rounds[1].Stop.Attempt || after.Subjects[0].Examination != before.Subjects[0].Examination {
		t.Fatalf("repair changed the attempt or examination: %+v %v", after, err)
	}
}
