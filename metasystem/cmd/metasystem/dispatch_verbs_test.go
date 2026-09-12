package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestComposeRolePacketCommandCarriesGoalTier(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	temp := t.TempDir()
	brief := filepath.Join(temp, "brief.md")
	if err := os.WriteFile(brief, []byte("Build the focused change.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	composition := filepath.Join(temp, "composition.json")
	code := runDispatchComposeRolePacket([]string{
		"--root", root, "--role", "implementer", "--brief", brief,
		"--job", "compose-tier-3", "--runtime", "fake", "--model", "fake-model",
		"--tool-policy", "read-write", "--round", "1", "--destructive-reach", "MECHANICAL",
		"--goal-tier", "3", "--output", filepath.Join(temp, "prompt.md"), "--composition", composition,
	})
	if code != 0 {
		t.Fatalf("compose-role-packet exit = %d", code)
	}
	stored, err := os.ReadFile(composition)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(stored, &record); err != nil {
		t.Fatal(err)
	}
	obligations, ok := record["configurationObligations"].(map[string]any)
	if !ok || obligations["independentCritiqueRequired"] != true ||
		obligations["independentCritiqueEffortTier"] != "maximal" ||
		obligations["independentCritiqueReasoningEffort"] != "xhigh" {
		t.Fatalf("tier-3 command obligations = %#v", record["configurationObligations"])
	}
}

func TestGoalRevisionAdmissionCommandMarksThenEnforcesWithExplicitDispatchContext(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	amendSyncedGoalFixture(t, root, "breach-stop capable admission fixture", func(file *goal.GoalFile) {
		file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1}
	})
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-08-30T09:00:00Z")
	args := []string{"--root", root, "--goal", "standing-validation", "--revision", "2", "--proposed-cap", "5", "--role", "implementer", "--dispatch-mode", "fresh", "--destructive-reach", "MECHANICAL"}
	marked, markCode := captureStdout(t, func() int { return runDispatchGoalRevisionAdmission(args) })
	want := "RISK_UNANSWERED goal=standing-validation tier=3 next: goal edit --risk"
	if markCode != 0 || strings.TrimSpace(marked) != want {
		t.Fatalf("mark-mode command did not print its notice and proceed: code=%d output=%q", markCode, marked)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\nmetasystem.governance.correlation-policy=A\nmetasystem.budget.risk-gate=enforce\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	refusal, enforceCode := captureStderr(t, func() int { return runDispatchGoalRevisionAdmission(args) })
	if enforceCode != 9 || strings.TrimSpace(refusal) != want {
		t.Fatalf("enforce-mode command did not refuse with the same code: code=%d output=%q", enforceCode, refusal)
	}
}

func TestGoalRevisionAdmissionCommandRefusesExhaustedCodeCriticClass(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	amendSyncedGoalFixture(t, root, "critic class admission fixture", func(file *goal.GoalFile) {
		file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1}
		file.Budget.AttemptLimit = 20
		file.Budget.ReservedJobMinutesLimit = 1000
		file.Budget.ActiveJobLimit = 10
		file.Budget.ReviewRoundLimit = 2
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, job := range []string{"code-one", "code-two"} {
		writeTemp(t, jobs, job+".json", map[string]any{
			"jobId": job, "operationId": job, "role": "code-critic", "parentJob": nil,
			"goalId": "standing-validation", "goalRevision": 2, "capMin": 1, "status": "completed",
			"reviewChainCounted": true,
		})
	}
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-08-30T09:00:00Z")
	args := []string{"--root", root, "--goal", "standing-validation", "--revision", "2", "--proposed-cap", "1", "--role", "code-critic", "--dispatch-mode", "fresh", "--destructive-reach", "MECHANICAL"}
	refusal, code := captureStdout(t, func() int { return runDispatchGoalRevisionAdmission(args) })
	if code != 9 || !strings.Contains(refusal, "codeCritiques=2/2") {
		t.Fatalf("command did not refuse the exhausted code-critic class: code=%d stdout=%q", code, refusal)
	}
}

func TestGoalRevisionAdmissionCommandJSONCarriesBudgetExtensionOffer(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\nmetasystem.governance.correlation-policy=A\nmetasystem.budget.tier-3=8h/1/1200m/1/3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	amendSyncedGoalFixture(t, root, "budget extension command fixture", func(file *goal.GoalFile) {
		file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1}
		file.Budget.AttemptLimit = 1
		file.Budget.ReservedJobMinutesLimit = 10000
		file.Budget.ActiveJobLimit = 10
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	now := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	receipt := fmt.Sprintf("%d|%s|RECEIPT|type=implement|outcome=shipped|goal=standing-validation|note=fixture\n",
		now.Add(-time.Hour).Unix(), now.Add(-time.Hour).Format(time.RFC3339))
	if err := os.MkdirAll(filepath.Join(root, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "memory", "receipts.log"), []byte(receipt), 0o644); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, root, "add", "memory/receipts.log", "metasystem.conf")
	goalSyncMutationGit(t, root, "commit", "-q", "-m", "extension receipt")
	goalSyncMutationGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, jobs, "spent.json", map[string]any{
		"jobId": "spent", "operationId": "spent", "goalId": "standing-validation", "goalRevision": 2,
		"capMin": 1, "status": "completed", "startedAt": "2026-08-30T08:20:00Z", "endedAt": "2026-08-30T08:21:00Z",
		"pid": 4242,
	})
	t.Setenv("METASYSTEM_GOAL_NOW", now.Format(time.RFC3339))
	args := []string{"--root", root, "--goal", "standing-validation", "--revision", "2", "--proposed-cap", "1",
		"--role", "implementer", "--dispatch-mode", "fresh", "--destructive-reach", "MECHANICAL", "--format", "json"}
	output, code := captureStdout(t, func() int { return runDispatchGoalRevisionAdmission(args) })
	if code != 9 {
		t.Fatalf("JSON offer command exit=%d output=%s", code, output)
	}
	var verdict dispatchcore.GoalRevisionAdmission
	if err := json.Unmarshal([]byte(output), &verdict); err != nil || verdict.Extension == nil || verdict.Extension.EvidenceKind != "landing" {
		t.Fatalf("JSON verdict lost the offer: %+v err=%v output=%s", verdict, err, output)
	}

	t.Setenv("METASYSTEM_OWNER_LINEAGE", "m1")
	extendArgs := []string{"--root", root, "--id", "standing-validation", "--revision", "2", "--proposed-cap", "1",
		"--role", "implementer", "--dispatch-mode", "fresh", "--destructive-reach", "MECHANICAL"}
	extended, extendCode := captureStdout(t, func() int { return runGoalExtendBudget(extendArgs) })
	if extendCode != 0 || !strings.Contains(extended, `"outcome":"confirmed"`) {
		t.Fatalf("extend-budget command did not replay and apply the offer: code=%d output=%s", extendCode, extended)
	}
	tip := goalSyncMutationGit(t, root, "rev-parse", goal.AcceptedRef)
	record := goalSyncMutationGit(t, root, "cat-file", "-p", tip+":plans/goals/standing-validation.md")
	if !strings.Contains(record, "- BudgetExtension: ") || !strings.Contains(record, "attemptLimit=1->2") {
		t.Fatalf("extend-budget command did not persist its marker: %s", record)
	}

	writeTemp(t, jobs, "spent-again.json", map[string]any{
		"jobId": "spent-again", "operationId": "spent-again", "goalId": "standing-validation", "goalRevision": 2,
		"capMin": 1, "status": "completed", "startedAt": "2026-08-30T08:30:00Z", "endedAt": "2026-08-30T08:31:00Z",
	})
	second, secondCode := captureStderr(t, func() int { return runGoalExtendBudget(extendArgs) })
	if secondCode != 1 || !strings.Contains(second, "extended once at 2026-08-30T09:00:00Z") {
		t.Fatalf("second extend-budget command did not name its marker: code=%d output=%s", secondCode, second)
	}
}

func TestGoalExtendBudgetRefusesSeamsThatAreNotExtendable(t *testing.T) {
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "m1")
	baseArgs := func(root string) []string {
		return []string{"--root", root, "--id", "standing-validation", "--revision", "2", "--proposed-cap", "1",
			"--role", "implementer", "--dispatch-mode", "fresh", "--destructive-reach", "MECHANICAL"}
	}
	t.Run("zero proposed cap", func(t *testing.T) {
		root := syncedClaimedGoalFixture(t)
		args := baseArgs(root)
		for index := range args {
			if args[index] == "--proposed-cap" {
				args[index+1] = "0"
			}
		}
		output, code := captureStderr(t, func() int { return runGoalExtendBudget(args) })
		if code != 2 || !strings.Contains(output, "positive --proposed-cap") {
			t.Fatalf("zero-cap extension refusal: code=%d output=%s", code, output)
		}
	})
	t.Run("admitted", func(t *testing.T) {
		root := syncedClaimedGoalFixture(t)
		amendSyncedGoalFixture(t, root, "admitted extension refusal", func(file *goal.GoalFile) {
			file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1}
		})
		t.Setenv("METASYSTEM_GOAL_NOW", "2026-08-30T09:00:00Z")
		output, code := captureStderr(t, func() int { return runGoalExtendBudget(baseArgs(root)) })
		if code != 1 || !strings.Contains(output, "is admitted; there is no budget refusal to extend") {
			t.Fatalf("admitted seam refusal: code=%d output=%s", code, output)
		}
	})
	t.Run("active job", func(t *testing.T) {
		root := syncedClaimedGoalFixture(t)
		amendSyncedGoalFixture(t, root, "active job extension refusal", func(file *goal.GoalFile) {
			file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1}
			file.Budget.AttemptLimit = 20
			file.Budget.ReservedJobMinutesLimit = 10000
			file.Budget.ActiveJobLimit = 1
			file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
		})
		jobs := filepath.Join(root, "artifacts", "agents", "jobs")
		if err := os.MkdirAll(jobs, 0o755); err != nil {
			t.Fatal(err)
		}
		writeTemp(t, jobs, "active.json", map[string]any{
			"jobId": "active", "operationId": "active", "goalId": "standing-validation", "goalRevision": 2,
			"capMin": 1, "status": "running",
		})
		t.Setenv("METASYSTEM_GOAL_NOW", "2026-08-30T09:00:00Z")
		output, code := captureStderr(t, func() int { return runGoalExtendBudget(baseArgs(root)) })
		if code != 1 || !strings.Contains(output, "activeJobLimit") || !strings.Contains(output, "no consumption-earned budget extension offer") {
			t.Fatalf("active-job seam refusal: code=%d output=%s", code, output)
		}
	})
	t.Run("live stop", func(t *testing.T) {
		root := syncedClaimedGoalFixture(t)
		amendSyncedGoalFixture(t, root, "live stop extension refusal", func(file *goal.GoalFile) {
			file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1}
			file.Budget.ElapsedLimit = "1h"
			file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
		})
		t.Setenv("METASYSTEM_GOAL_NOW", "2026-08-30T10:00:00Z")
		output, code := captureStderr(t, func() int { return runGoalExtendBudget(baseArgs(root)) })
		if code != 1 || !strings.Contains(output, "names a live stop, not an extendable exhaustion") {
			t.Fatalf("live-stop seam refusal: code=%d output=%s", code, output)
		}
	})
}

func TestCommandTaggedProcessScannerUsesAuthorizedCompleteFixtureTable(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	processes := filepath.Join(root, "processes.json")
	if err := os.WriteFile(processes, []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", processes)
	result := (commandTaggedProcessScanner{root: root}).ScanTag("reservation-tag", time.Now())
	if !result.Complete() || result.EnumerationError != "" || len(result.Tagged) != 0 {
		t.Fatalf("complete empty fixture table = %+v", result)
	}
}

// writeTemp writes a JSON file and returns its path.
func writeTemp(t *testing.T, dir, name string, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// captureStdout runs fn with stdout redirected and returns what it printed.
func captureStdout(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	original := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = write
	code := fn()
	write.Close()
	os.Stdout = original
	out, _ := io.ReadAll(read)
	return string(out), code
}

func TestResolveModelAliasVerb(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	if err := os.WriteFile(conf, []byte("metasystem.runtimes=claude\nruntime.claude.model-alias.claude-fable-5=claude-fable-5-1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := captureStdout(t, func() int {
		return runDispatchResolveModelAlias([]string{"--conf", conf, "--runtime", "claude", "--model", "claude-fable-5"})
	})
	if code != 0 {
		t.Fatalf("resolve-model-alias exit = %d", code)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got["model"] != "claude-fable-5-1" || got["aliasedFrom"] != "claude-fable-5" || len(got) != 2 {
		t.Fatalf("resolve-model-alias output = %v", got)
	}
}

func TestCommandTaggedProcessScannerHonorsEmptyConfiguredUniverse(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	processes := writeTemp(t, t.TempDir(), "processes.json", []any{})
	identities := writeTemp(t, t.TempDir(), "identities.json", map[string]any{})
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", processes)
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", identities)

	result := (commandTaggedProcessScanner{root: root}).ScanTag("metasystem-job-empty-nonce", time.Time{})
	if !result.Complete() || result.EnumerationError != "" || len(result.Tagged) != 0 {
		t.Fatalf("empty configured process universe was not a complete absence proof: %+v", result)
	}
}

// TestDispatchRecordVerbsPath drives the whole record lifecycle through the CLI
// verbs the shell invokes, proving the flag parsing, exit-code mapping, and the
// lost-compare stdout witness all work end to end.
func TestDispatchRecordVerbsPath(t *testing.T) {
	root := t.TempDir()
	tmp := t.TempDir()
	job := "job-cli"

	create := writeTemp(t, tmp, "create.json", map[string]any{
		"jobId": job, "status": "pending-setup", "mainId": "main-1", "claimEpoch": 7,
	})
	if code := runDispatchRecordCreate([]string{"--root", root, "--job", job, "--source", create}); code != 0 {
		t.Fatalf("record-create exit = %d, want 0", code)
	}
	// A second create on the same id is a collision (exit 1).
	if code := runDispatchRecordCreate([]string{"--root", root, "--job", job, "--source", create}); code != 1 {
		t.Fatalf("record-create collision exit = %d, want 1", code)
	}

	setup := writeTemp(t, tmp, "setup.json", map[string]any{
		"jobId": job, "status": "pending", "mainId": "main-1", "claimEpoch": 7,
		"startedAt": "2026-08-10T00:00:00Z",
	})
	if code := runDispatchRecordSetup([]string{"--root", root, "--job", job, "--source", setup}); code != 0 {
		t.Fatalf("record-setup exit = %d, want 0", code)
	}

	run := writeTemp(t, tmp, "run.json", map[string]any{"sessionId": "s"})
	if code := runDispatchRecordCAS([]string{"--root", root, "--job", job, "--expect", "pending", "--status", "running", "--patch", run}); code != 0 {
		t.Fatalf("record-cas pending->running exit = %d, want 0", code)
	}

	// A lost compare prints the observed status on stdout and exits 3.
	stale := writeTemp(t, tmp, "stale.json", map[string]any{"note": "x"})
	out, code := captureStdout(t, func() int {
		return runDispatchRecordCAS([]string{"--root", root, "--job", job, "--expect", "pending", "--status", "failed", "--patch", stale})
	})
	if code != 3 {
		t.Fatalf("stale record-cas exit = %d, want 3", code)
	}
	if strings.TrimSpace(out) != "observed=running" {
		t.Fatalf("stale record-cas stdout = %q, want observed=running", out)
	}

	// A missing required flag is a usage error (exit 2).
	if code := runDispatchRecordCAS([]string{"--root", root, "--job", job, "--expect", "running"}); code != 2 {
		t.Fatalf("record-cas missing-flags exit = %d, want 2", code)
	}
}

func TestDispatchCritiqueAdvanceVerbsPath(t *testing.T) {
	repo := t.TempDir()
	agents := filepath.Join(repo, "artifacts", "agents")
	jobs := filepath.Join(agents, "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	facts := map[string]any{
		"local": true, "recoverable": true,
		"proofBoundaryCrossed": false, "authorityBoundaryCrossed": false,
		"secretsBoundaryCrossed": false, "irreversibleDataBoundaryCrossed": false,
		"externalSideEffectBoundaryCrossed": false,
	}
	for round := 1; round <= 3; round++ {
		job := "critic"
		parent := any(nil)
		if round > 1 {
			job = "critic-r" + strconv.Itoa(round)
			if round == 2 {
				parent = "critic"
			} else {
				parent = "critic-r2"
			}
		}
		record := map[string]any{
			"jobId": job, "role": "design-critic", "round": round,
			"parentJob": parent, "status": "completed",
		}
		if round == 1 {
			record["findingRegister"] = []any{}
			record["findingRegisterRound"] = 0
			record["reviewRoundLimit"] = 3
			record["criticRoundsConsumed"] = 0
			record["demotions"] = []any{}
			record["critiqueExhaustions"] = []any{}
		}
		writeTemp(t, jobs, job+".json", record)
		findings := []any{}
		rigor := []any{}
		if round == 1 {
			findings = []any{map[string]any{
				"id": "S-1", "severity": "high", "material": true,
				"claim": "severe finding", "evidence": "direct evidence",
			}}
			rigor = []any{map[string]any{
				"findingId": "S-1", "rigorClass": "severe", "facts": facts,
				"artifact":         "metasystem/test.go",
				"reopeningTrigger": "reopen if the defect recurs",
			}}
		}
		roundDir := filepath.Join(agents, "critic", "rounds", strconv.Itoa(round))
		if err := os.MkdirAll(roundDir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeTemp(t, roundDir, "return.json", map[string]any{
			"schemaVersion": 3, "jobId": job, "round": round,
			"findings": findings, "rigor": rigor,
		})
		out, code := captureStdout(t, func() int {
			return runDispatchCritiqueRegisterAdvance([]string{"--repo", repo, "--root-job", "critic", "--round-job", job})
		})
		if code != 0 || strings.TrimSpace(out) != "advanced" {
			t.Fatalf("register round %d: exit=%d out=%q", round, code, out)
		}
	}
	out, code := captureStdout(t, func() int {
		return runDispatchCritiqueOpenFindingIDs([]string{"--repo", repo, "--root-job", "critic"})
	})
	if code != 0 || strings.TrimSpace(out) != "S-1" {
		t.Fatalf("open finding identifiers: exit=%d out=%q", code, out)
	}
	message := filepath.Join(t.TempDir(), "message.md")
	if err := os.WriteFile(message, []byte("Address S-1.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr, code := captureStderr(t, func() int {
		return runDispatchCritiqueExhaustionAdvance([]string{
			"--repo", repo, "--root-job", "critic", "--role", "design-critic",
			"--message", message, "--successor", "critic-r4",
		})
	})
	wantStderr := "reason=cap-exhausted-human-raise the review-round limit is exhausted with a severe or unproven finding; waiting on the human is the only remedy at terminal round 3 with open finding identifiers: S-1\n"
	if code != 10 || stderr != wantStderr {
		t.Fatalf("terminal exhaustion: exit=%d stderr=%q want=%q", code, stderr, wantStderr)
	}
	rootRecord, err := os.ReadFile(filepath.Join(jobs, "critic.json"))
	if err != nil || strings.Contains(string(rootRecord), `"successorJobId": "critic-r4"`) {
		t.Fatalf("terminal exhaustion wrote legacy state: %v, %s", err, rootRecord)
	}
	if code := runDispatchCritiqueRegisterAdvance([]string{"--repo", repo}); code != 2 {
		t.Fatalf("register usage error exit=%d, want 2", code)
	}
	if code := runDispatchCritiqueOpenFindingIDs([]string{"--repo", repo}); code != 2 {
		t.Fatalf("open finding identifiers usage error exit=%d, want 2", code)
	}
}

func TestDispatchCritiqueRegisterCloseKeepsRegisterlessCompatibility(t *testing.T) {
	repo := t.TempDir()
	jobs := filepath.Join(repo, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, jobs, "legacy-critic.json", map[string]any{
		"jobId": "legacy-critic", "role": "code-critic", "round": 1,
		"parentJob": nil, "status": "completed",
	})
	out, code := captureStdout(t, func() int {
		return runDispatchCritiqueRegisterClose([]string{"--repo", repo, "--root-job", "legacy-critic"})
	})
	if code != 0 || strings.TrimSpace(out) != "closed" {
		t.Fatalf("register-less close verb = exit %d output %q", code, out)
	}
}
