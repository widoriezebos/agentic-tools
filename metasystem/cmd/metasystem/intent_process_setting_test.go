package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processchange"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func processSettingBed(t *testing.T) (*policyBed, string) {
	t.Helper()
	b := newPolicyBed(t)
	b.owners.prove = fixedFixtureGoalAuthority
	b.owners.processes.question = channel.ReadQuestion
	b.owners.dependencies.ownerLineage = func() string { return "builder-session" }
	// Settings proposals observe only this fixture's retained executions.
	unitRoot := t.TempDir()
	manager := &launch.Manager{Store: launch.Store{Root: t.TempDir()}, Now: func() time.Time { return b.now }}
	b.owners.work.units = func(stateroot.Layout) *launch.UnitRunner {
		return &launch.UnitRunner{Root: unitRoot, Manager: manager}
	}
	b.owners.policies.ConfPath = func(checkout string) (string, error) { return filepath.Join(checkout, "settings.conf"), nil }
	local := filepath.Join(b.seat, "settings.conf.local")
	if err := os.WriteFile(local, []byte("# preserve unrelated bytes\nsecret=not-a-real-secret\nlaunch.codex.sandbox=workspace-write\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return b, local
}

func TestProcessSettingRecoveryAndPreservation(t *testing.T) {
	t.Parallel()
	b, local := processSettingBed(t)
	_, result, _ := b.run(t, b.seat, "set", "launch.codex.sandbox", "danger-full-access")
	act := processAct(t, result)
	before, _ := os.ReadFile(local)
	// The configuration write survived; the applied record did not.
	if err := os.WriteFile(local, bytes.ReplaceAll(before, []byte("workspace-write"), []byte("danger-full-access")), 0600); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2002, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(local, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	b.owners.policies.Helm = func(string) helm.State {
		return helm.State{Active: true, Record: helm.Record{By: "Wido"}, Since: b.now}
	}
	code, result, _ := b.run(t, b.seat, "set", act.Key, act.After, "--act", act.ID)
	info, err := os.Stat(local)
	applied := processAct(t, result)
	if code != 0 || err != nil || !info.ModTime().Equal(stamp) || applied.Status != "applied" || applied.AppliedProof.Helm != nil || applied.AppliedProof.TerminalGeneration == 0 {
		t.Fatalf("crash reconciliation under the helm: %d %+v %v", code, result, err)
	}

	if code, result, _ := b.run(t, b.seat, "set", act.Key, "workspace-write"); code != 0 {
		t.Fatalf("person replacement: %+v", result)
	}
	if code, result, _ := b.run(t, b.seat, "set", act.Key, act.After, "--act", act.ID); code == 0 || !strings.Contains(result.Summary, "superseded") {
		t.Fatalf("an older applied act overwrote current state: %+v", result)
	}
	q, _ := channel.ReadQuestion(b.seat, act.Question)
	if q.State != "closed" {
		t.Fatalf("reconciled act did not close its question: %+v", q)
	}
	// Corrupt advisory policy and act history cannot veto an explicit person.
	for _, path := range []string{filepath.Join(b.seat, ".git", "metasystem", "policy", "process.change.json"), filepath.Join(b.seat, "process", "acts", act.ID+".json")} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("damaged metadata"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	b.owners.policies.Registry = func(string) (config.PolicyRegistry, error) { return config.PolicyRegistry{}, os.ErrPermission }
	if code, result, _ := b.run(t, b.seat, "set", act.Key, act.After); code != 0 {
		t.Fatalf("advisory damage vetoed a person: %+v", result)
	}
	history, _ := os.ReadFile(filepath.Join(b.seat, "process", "acts", act.ID+".json"))
	if string(history) != "damaged metadata" {
		t.Fatal("person replacement discarded unreadable history")
	}
	// An actual inability to preserve the target is still an effect failure.
	if err := os.Remove(local); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(local, 0700); err != nil {
		t.Fatal(err)
	}
	if code, result, _ := b.run(t, b.seat, "set", act.Key, "workspace-write"); code == 0 {
		t.Fatalf("preservation failure reported success: %+v", result)
	}
	if err := os.Remove(local); err != nil {
		t.Fatal(err)
	}
	// A missing-layer proposal requires readable process history.
	b, local = processSettingBed(t)
	if err := os.Remove(local); err != nil {
		t.Fatal(err)
	}
	b.owners.prove = fixedFixtureGoalAuthority
	_, result, _ = b.run(t, b.seat, "set", act.Key, "workspace-write")
	absent := processAct(t, result)
	if absent.ID == "" || absent.Before != nil {
		t.Fatalf("missing local layer was not proposed: %+v", result)
	}
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	if code, result, _ := b.run(t, b.seat, "set", absent.Key, absent.After, "--act", absent.ID); code != 0 {
		t.Fatalf("missing layer remedy: %+v", result)
	}
	if data, err := os.ReadFile(local); err != nil || !bytes.Contains(data, []byte("launch.codex.sandbox=workspace-write")) {
		t.Fatalf("missing layer was not created: %s %v", data, err)
	}

}

func TestProcessOrdinaryCheckPublicBuild(t *testing.T) {
	t.Parallel()
	for _, selection := range []string{"^One$", "^Two$"} {
		bed := newWorkBed(t)
		bed.lineage = "builder"
		brief := bed.brief("ordinary.md", "Build this unit.\n")
		check := []string{"go", "test", "-timeout", "30m", "-run", selection, "./fixture"}
		bed.declaredCheap = shellCommand(check)
		args := []string{"work", "build", bed.id, "--work", strings.Trim(selection, "^$"), "--brief", brief, "--lines", "5"}
		code, result, output := bed.work(args...)
		if code != 0 {
			t.Fatalf("ordinary check was held: exit %d %+v %s", code, result, output)
		}
		plan, err := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
		if err != nil || plan.Check == nil || plan.Check.Cheap != shellCommand(check) || len(plan.Proof) != 1 || plan.Proof[0].Name != "unit-check" {
			t.Fatalf("ordinary check was not retained: %+v %v", plan, err)
		}
	}
}

func processAct(t *testing.T, result intentResult) processchange.ProcessAct {
	t.Helper()
	data, err := json.Marshal(result.Data)
	if err != nil {
		t.Fatal(err)
	}
	var act processchange.ProcessAct
	if err := json.Unmarshal(data, &act); err != nil {
		t.Fatal(err)
	}
	return act
}

func TestProcessSettingActPublicRemedy(t *testing.T) {
	t.Parallel()
	b, local := processSettingBed(t)
	if policy := b.show(t, "process.change"); policy.Value != "person" {
		t.Fatalf("process default = %+v", policy)
	}
	before, _ := os.ReadFile(local)
	code, result, raw := b.run(t, b.seat, "set", "launch.codex.sandbox", "danger-full-access")
	act := processAct(t, result)
	if code == 0 || act.ID == "" || act.Status != "proposed" || act.Actor != "agent" || act.Lineage != "builder-session" {
		t.Fatalf("proposal exit %d: %s", code, raw)
	}
	after, _ := os.ReadFile(local)
	if !bytes.Equal(before, after) {
		t.Fatal("an agent applied its own setting under person policy")
	}
	_, repeated, _ := b.run(t, b.seat, "set", act.Key, act.After)
	if again := processAct(t, repeated); again.ID != act.ID || again.Question != act.Question {
		t.Fatalf("proposal replay duplicated the act/question: %+v", again)
	}
	questions, _ := filepath.Glob(filepath.Join(b.seat, "artifacts", "agents", "channel", "questions", "*.json"))
	if len(questions) != 1 {
		t.Fatalf("proposal wrote %d questions", len(questions))
	}
	var envelope struct {
		Next struct {
			Argv []string `json:"argv"`
		} `json:"next"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatal(err)
	}
	want := []string{"metasystem", "settings", "set", act.Key, act.After, "--repo", b.seat, "--act", act.ID}
	if shellCommand(envelope.Next.Argv) != shellCommand(want) {
		t.Fatalf("remedy = %v", envelope.Next.Argv)
	}
	var stdout, stderr bytes.Buffer
	code = runIntentIn(mustIntentCommand(t, "question show"), []string{"channel:" + act.Question}, &stdout, &stderr, b.seat, b.owners)
	if code != 0 || !strings.Contains(stdout.String(), shellCommand(want)) {
		t.Fatalf("question show exit %d: %s %s", code, &stdout, &stderr)
	}
	t.Logf("question show: %s", &stdout)
	// Possession of an act id is never a person's proof.
	if code, result, _ := b.run(t, b.seat, "set", act.Key, act.After, "--act", act.ID); code == 0 || processAct(t, result).Status == "applied" {
		t.Fatalf("an agent used --act as authority: %+v", result)
	}
	q, err := channel.ReadQuestion(b.seat, act.Question)
	if err != nil || q.ProcessAct != act.ID || q.Wants != shellCommand(want) {
		t.Fatalf("bound question = %+v, %v", q, err)
	}
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	code, result, _ = b.run(t, b.seat, "set", want[3:]...)
	if code != 0 || processAct(t, result).Status != "applied" {
		t.Fatalf("exact person remedy exit %d: %+v", code, result)
	}
	after, _ = os.ReadFile(local)
	if !bytes.Contains(after, []byte("launch.codex.sandbox=danger-full-access\n")) || !bytes.Contains(after, []byte("secret=not-a-real-secret\n")) {
		t.Fatalf("setting or unrelated bytes lost: %s", after)
	}
	q, _ = channel.ReadQuestion(b.seat, act.Question)
	if q.State != "closed" {
		t.Fatalf("successful exact act did not close its question: %+v", q)
	}
	stamp := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(local, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	code, result, _ = b.run(t, b.seat, "set", want[3:]...)
	info, err := os.Stat(local)
	if code != 0 || err != nil || !info.ModTime().Equal(stamp) {
		t.Fatalf("replay rewrote its effect: exit %d, %v, %+v", code, err, result)
	}
	t.Log("settings set person remedy applied and closed its exact question; replay left the target modification time unchanged")
}

func TestProcessSettingGrantAuthority(t *testing.T) {
	t.Parallel()
	b, local := processSettingBed(t)
	b.owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
		return humanauthority.HelmProof(root, humanauthority.HelmGrant{By: "Wido", Grant: "attorney-identity", Class: "DELEGATE"}, at)
	}
	code, result, _ := b.run(t, b.seat, "set", "launch.codex.sandbox", "danger-full-access")
	act := processAct(t, result)
	if code == 0 || act.Actor != "agent" || act.Proof.Helm == nil || act.Proof.Helm.Grant != "attorney-identity" {
		t.Fatalf("grant became direct person or lost provenance: %+v", result)
	}
	before, _ := os.ReadFile(local)
	if code, result, _ := b.run(t, b.seat, "set", "process.change", "auto"); code == 0 {
		t.Fatalf("agent changed its own policy: %+v", result)
	}
	after, _ := os.ReadFile(local)
	if !bytes.Equal(before, after) {
		t.Fatal("person-only policy refusal changed bytes")
	}
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	if code, result, _ := b.run(t, b.seat, "set", "process.change", "auto"); code != 0 {
		t.Fatalf("person could not set auto: %+v", result)
	}
	b.owners.prove = fixedFixtureGoalAuthority
	if code, result, _ := b.run(t, b.seat, "set", "launch.codex.sandbox", "danger-full-access"); code == 0 {
		t.Fatalf("uncited automatic setting applied: %+v", result)
	}
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	if code, result, _ := b.run(t, b.seat, "set", "launch.codex.sandbox", "workspace-write", "--repo", b.lane); code != 0 {
		t.Fatalf("calling terminal could not apply at the destination: %+v", result)
	}
	b.owners.prove = fixedFixtureGoalAuthority

	// Automatic citation admission is outside this first part: an agent
	// still gets an executable person proposal after the policy is set to auto.
	code, result, _ = b.run(t, b.seat, "set", "launch.codex.sandbox", "danger-full-access")
	if code == 0 || processAct(t, result).Question == "" {
		t.Fatalf("automatic proposal did not remain held: %+v", result)
	}
}

func TestProcessSettingSupersededRemedy(t *testing.T) {
	t.Parallel()
	b, local := processSettingBed(t)
	if err := os.WriteFile(local, []byte("# preserve unrelated bytes\nlaunch.contract=before.json\n"), 0600); err != nil {
		t.Fatal(err)
	}
	code, proposed, raw := b.run(t, b.seat, "set", "launch.contract", "after.json")
	act := processAct(t, proposed)
	if code != 1 || act.Question == "" {
		t.Fatalf("proposal: exit %d %+v", code, proposed)
	}
	var envelope struct {
		Next struct {
			Argv []string `json:"argv"`
		} `json:"next"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatal(err)
	}
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	if code, result, _ := b.run(t, b.seat, "set", act.Key, "third.json"); code != 0 {
		t.Fatalf("person's third value: %+v", result)
	}
	before, err := os.ReadFile(local)
	if err != nil {
		t.Fatal(err)
	}
	code, result, raw := b.run(t, b.seat, "set", envelope.Next.Argv[3:]...)
	if code != 1 || !strings.Contains(result.Summary, "superseded") {
		t.Fatalf("stale act: exit %d %+v", code, result)
	}
	q, err := channel.ReadQuestion(b.seat, act.Question)
	if err != nil || q.State != "closed" {
		t.Fatalf("superseded question remains open: %+v %v", q, err)
	}
	history, err := os.ReadFile(filepath.Join(b.seat, "process", "acts", act.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var retained processchange.ProcessAct
	if err := json.Unmarshal(history, &retained); err != nil {
		t.Fatal(err)
	}
	if retained.Status != "superseded" {
		t.Fatalf("stale act status: %+v", retained)
	}
	after, err := os.ReadFile(local)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("stale act changed the person's setting: %s %v", after, err)
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatal(err)
	}
	want := []string{"metasystem", "settings", "set", act.Key, act.After, "--repo", b.seat}
	if shellCommand(envelope.Next.Argv) != shellCommand(want) {
		t.Fatalf("replacement remedy: %v", envelope.Next.Argv)
	}
	if code, result, _ := b.run(t, b.seat, "set", envelope.Next.Argv[3:]...); code != 0 || processAct(t, result).Status != "applied" {
		t.Fatalf("replacement remedy: exit %d %+v", code, result)
	}
	after, err = os.ReadFile(local)
	if err != nil || !bytes.Contains(after, []byte(act.Key+"="+act.After+"\n")) {
		t.Fatalf("replacement remedy did not apply: %s %v", after, err)
	}
	t.Log("stale act refused without changing bytes; question closed; plain person remedy applied")
}

func TestProcessSettingReproposal(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"applied", "superseded"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			b, local := processSettingBed(t)
			if err := os.WriteFile(local, []byte("launch.contract=before.json\n"), 0600); err != nil {
				t.Fatal(err)
			}
			code, result, _ := b.run(t, b.seat, "set", "launch.contract", "after.json")
			previous := processAct(t, result)
			if code != 1 || previous.Status != "proposed" || previous.Key != "launch.contract" || previous.After != "after.json" || previous.ID == "" || previous.Question == "" {
				t.Fatalf("initial proposal: exit %d %+v", code, result)
			}
			for cycle := 0; cycle < 2; cycle++ {
				b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
				if status == "superseded" {
					if code, result, _ := b.run(t, b.seat, "set", previous.Key, "third.json"); code != 0 {
						t.Fatalf("third value: %+v", result)
					}
				}
				code, result, _ := b.run(t, b.seat, "set", previous.Key, previous.After, "--act", previous.ID)
				if (status == "applied" && code != 0) || (status == "superseded" && code != 1) {
					t.Fatalf("finish act: exit %d %+v", code, result)
				}
				if code, result, _ := b.run(t, b.seat, "set", previous.Key, "before.json"); code != 0 {
					t.Fatalf("restore original value: %+v", result)
				}
				b.owners.prove = fixedFixtureGoalAuthority
				code, result, _ = b.run(t, b.seat, "set", previous.Key, previous.After)
				fresh := processAct(t, result)
				if code != 1 || fresh.ID == "" || fresh.ID == previous.ID || fresh.Question == "" || fresh.Question == previous.Question || fresh.Status != "proposed" {
					t.Fatalf("reproposal collided: exit %d %+v", code, result)
				}
				_, replay, _ := b.run(t, b.seat, "set", fresh.Key, fresh.After)
				if again := processAct(t, replay); again.ID != fresh.ID || again.Question != fresh.Question {
					t.Fatalf("pending replay duplicated: %+v", again)
				}
				questions, err := filepath.Glob(filepath.Join(b.seat, "artifacts", "agents", "channel", "questions", "*.json"))
				if err != nil || len(questions) != cycle+2 {
					t.Fatalf("question count: %d %v", len(questions), err)
				}
				open := 0
				for _, path := range questions {
					q, err := channel.ReadQuestion(b.seat, strings.TrimSuffix(filepath.Base(path), ".json"))
					if err != nil {
						t.Fatal(err)
					}
					if q.State == "open" {
						open++
					}
				}
				if open != 1 {
					t.Fatalf("reproposal has %d open questions", open)
				}
				previous = fresh
			}
			t.Log("completed proposals renewed twice; each pending replay retained exactly one open question")
		})
	}
}
