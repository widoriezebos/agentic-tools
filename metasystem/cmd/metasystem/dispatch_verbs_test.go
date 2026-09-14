package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
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

func TestVerifyReferencesVerbExitsNineAndPrintsTheLine(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	testRoot := filepath.Join(root, "artifacts", "agents", "test-"+t.Name())
	t.Cleanup(func() { os.RemoveAll(testRoot) })
	stageDir := filepath.Join(testRoot, "record-locks", "staged")
	referenceDir := filepath.Join(testRoot, "rounds", "1", "staged")
	temp := t.TempDir()
	brief := filepath.Join(temp, "brief.md")
	if err := os.WriteFile(brief, []byte(strings.Repeat("b", 40*1024)), 0o644); err != nil {
		t.Fatal(err)
	}
	composition := filepath.Join(temp, "composition.json")
	code := runDispatchComposeRolePacket([]string{
		"--root", root, "--role", "verifier", "--brief", brief,
		"--job", "verify-references", "--runtime", "fake", "--model", "fake-model",
		"--tool-policy", "read-only", "--round", "1", "--destructive-reach", "MECHANICAL",
		"--output", filepath.Join(temp, "prompt.md"), "--composition", composition,
		"--stage-dir", stageDir, "--reference-dir", referenceDir,
	})
	if code != 0 {
		t.Fatalf("compose-role-packet exit = %d", code)
	}
	if err := os.MkdirAll(filepath.Dir(referenceDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(stageDir, referenceDir); err != nil {
		t.Fatal(err)
	}
	out, code := captureStdout(t, func() int {
		return runDispatchVerifyReferences([]string{"--root", root, "--composition", composition})
	})
	if code != 0 || strings.TrimSpace(out) != "references-verified count=1" {
		t.Fatalf("verified command = exit %d, output %q", code, out)
	}
	staged := filepath.Join(referenceDir, "task-direction.md")
	handle, err := os.OpenFile(staged, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handle.WriteString("tampered\n"); err != nil {
		handle.Close()
		t.Fatal(err)
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	out, code = captureStdout(t, func() int {
		return runDispatchVerifyReferences([]string{"--root", root, "--composition", composition})
	})
	if code != 9 || strings.Count(strings.TrimSpace(out), "\n") != 0 || !strings.HasPrefix(out, "REFERENCE_"+"MISMATCH path=") {
		t.Fatalf("mismatch command = exit %d, output %q", code, out)
	}
	_, code = captureStderr(t, func() int {
		return runDispatchVerifyReferences([]string{"--root", root, "--composition", filepath.Join(temp, "missing.json")})
	})
	if code != 1 {
		t.Fatalf("missing composition exit = %d, want 1", code)
	}
}

func TestComposeRolePacketCommandEnforcesPacketCap(t *testing.T) {
	const limitEnv = "METASYSTEM_DISPATCH_MAX_INLINE_INPUT_KB"
	if original, present := os.LookupEnv(limitEnv); present {
		t.Cleanup(func() { _ = os.Setenv(limitEnv, original) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv(limitEnv) })
	}
	if err := os.Unsetenv(limitEnv); err != nil {
		t.Fatal(err)
	}
	sourceRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	newRoot := func(t *testing.T) string {
		t.Helper()
		root := t.TempDir()
		copyFile := func(path string) {
			t.Helper()
			content, err := os.ReadFile(filepath.Join(sourceRoot, filepath.FromSlash(path)))
			if err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(root, filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(destination, content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		for _, path := range []string{
			"scripts/agents/role-packets.json",
			"scripts/agents/roles/implementer.md",
			"docs/orchestration.md",
			"scripts/agents/schemas/implementer.schema.json",
		} {
			copyFile(path)
		}
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		return root
	}
	digest := func(data []byte) string {
		sum := sha256.Sum256(data)
		return fmt.Sprintf("%x", sum)
	}
	readRecord := func(t *testing.T, path string) dispatchcore.CompositionRecord {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var record dispatchcore.CompositionRecord
		if err := json.Unmarshal(data, &record); err != nil {
			t.Fatal(err)
		}
		return record
	}
	verifyPacket := func(t *testing.T, root, packetPath string, record dispatchcore.CompositionRecord, rawBySlot map[string][]byte, wantReferences []string) []byte {
		t.Helper()
		packet, err := os.ReadFile(packetPath)
		if err != nil {
			t.Fatal(err)
		}
		previousEnd := 0
		for _, source := range record.Sources {
			raw, ok := rawBySlot[source.Slot]
			if !ok {
				t.Fatalf("missing independent source bytes for %s", source.Slot)
			}
			if source.SourceDigest != digest(raw) || source.SourceBytes != len(raw) {
				t.Fatalf("source %s lost raw provenance: %+v", source.Slot, source)
			}
			if source.StartByte != previousEnd || source.EndByte <= source.StartByte || source.EndByte > len(packet) || source.DeliveredDigest != digest(packet[source.StartByte:source.EndByte]) {
				t.Fatalf("source %s has invalid delivered range or digest: %+v", source.Slot, source)
			}
			previousEnd = source.EndByte
		}
		if previousEnd != len(packet) || record.PacketDigest != digest(packet) {
			t.Fatalf("packet provenance ends at %d of %d with digest %s", previousEnd, len(packet), record.PacketDigest)
		}
		if len(record.References) != len(wantReferences) {
			t.Fatalf("references = %+v, want %v", record.References, wantReferences)
		}
		for index, slot := range wantReferences {
			if record.References[index].Slot != slot {
				t.Fatalf("reference %d = %s, want %s", index, record.References[index].Slot, slot)
			}
		}
		return packet
	}
	promoteAndVerify := func(t *testing.T, root, stageDir, referenceDir, composition string, count int) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(referenceDir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(stageDir, referenceDir); err != nil {
			t.Fatal(err)
		}
		out, code := captureStdout(t, func() int {
			return runDispatchVerifyReferences([]string{"--root", root, "--composition", composition})
		})
		want := fmt.Sprintf("references-verified count=%d", count)
		if code != 0 || strings.TrimSpace(out) != want {
			t.Fatalf("verify-references = exit %d, output %q; want %q", code, out, want)
		}
	}
	generatedBodies := func(job string, round int64) map[string][]byte {
		toolNotice := "Permission tool policy: read-write\nTool-name observation: exact\nTool names: (none; the fake runtime opens no model tool channel)\n"
		runtimeNotice := fmt.Sprintf("Job-Id: %s\nRole: implementer\nRuntime: fake\nModel: fake-model\nRound: %d\nMission: none\nDestructive reach class: MECHANICAL\nBuilder effort tier: ordinary\nBuilder reasoning effort: medium\nIndependent critique required: false\nIndependent critique effort tier: none\nIndependent critique reasoning effort: none\nLive proof required: false\nContext classification: advisory. This broad-read runtime does not prove context isolation or independent examination.\n", job, round)
		return map[string][]byte{"tool-names": []byte(toolNotice), "generated-runtime-notice": []byte(runtimeNotice)}
	}
	addFixedBodies := func(t *testing.T, root string, rawBySlot map[string][]byte) {
		t.Helper()
		for slot, path := range map[string]string{
			"role-instructions": "scripts/agents/roles/implementer.md",
			"required-skill":    "docs/orchestration.md",
			"response-contract": "scripts/agents/schemas/implementer.schema.json",
		} {
			raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
			if err != nil {
				t.Fatal(err)
			}
			rawBySlot[slot] = raw
		}
	}
	composeArgs := func(root, job, brief, output, composition, stageDir, referenceDir string, round int64, continuations []string) []string {
		args := []string{
			"--root", root, "--role", "implementer", "--brief", brief, "--job", job,
			"--runtime", "fake", "--model", "fake-model", "--tool-policy", "read-write",
			"--round", strconv.FormatInt(round, 10), "--destructive-reach", "MECHANICAL",
			"--output", output, "--composition", composition, "--stage-dir", stageDir, "--reference-dir", referenceDir,
		}
		for _, continuation := range continuations {
			args = append(args, "--continuation", continuation)
		}
		return args
	}
	assertImpossible := func(t *testing.T, args []string, output, composition, stageDir string, capBytes int64) {
		t.Helper()
		out, code := captureStdout(t, func() int { return runDispatchComposeRolePacket(args) })
		if code != 9 {
			t.Fatalf("impossible packet exit = %d, want 9; output %q", code, out)
		}
		var refusal map[string]any
		if err := json.Unmarshal([]byte(out), &refusal); err != nil {
			t.Fatal(err)
		}
		if refusal["outcome"] != "REFUSED-INLINE-INPUT-LIMIT" || refusal["headline"] != "refused" || refusal["source"] != "dispatch.max-inline-input-kb" {
			t.Fatalf("unexpected command refusal: %v", refusal)
		}
		detail, _ := refusal["detail"].(string)
		var packetBytes, gotCap, overhead int64
		if matched, err := fmt.Sscanf(detail, "role packet exceeds dispatch.max-inline-input-kb: %d bytes > %d bytes; fixed/reference overhead %d bytes", &packetBytes, &gotCap, &overhead); err != nil || matched != 3 || packetBytes <= gotCap || gotCap != capBytes || overhead <= 0 {
			t.Fatalf("refusal detail does not carry measured counts: %q (%d, %v)", detail, matched, err)
		}
		if detail != fmt.Sprintf("role packet exceeds dispatch.max-inline-input-kb: %d bytes > %d bytes; fixed/reference overhead %d bytes", packetBytes, gotCap, overhead) || strings.Contains(detail, "pass a file reference") {
			t.Fatalf("refusal detail has the wrong template: %q", detail)
		}
		for _, path := range []string{output, composition, stageDir, filepath.Join(stageDir, "task-direction.md")} {
			if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
				t.Fatalf("impossible command published %s", path)
			}
		}
	}

	t.Run("fresh", func(t *testing.T) {
		root := newRoot(t)
		briefBody := bytes.Repeat([]byte("b"), 16*1024)
		brief := filepath.Join(root, "fresh-brief.md")
		if err := os.WriteFile(brief, briefBody, 0o644); err != nil {
			t.Fatal(err)
		}
		output := filepath.Join(root, "fresh-prompt.md")
		composition := filepath.Join(root, "fresh-composition.json")
		stageDir := filepath.Join(root, "record-locks", "fresh")
		referenceDir := filepath.Join(root, "artifacts", "fresh", "rounds", "1", "staged")
		args := composeArgs(root, "fresh", brief, output, composition, stageDir, referenceDir, 1, nil)
		if code := runDispatchComposeRolePacket(args); code != 0 {
			t.Fatalf("fresh composition exit = %d", code)
		}
		rawBySlot := generatedBodies("fresh", 1)
		rawBySlot["task-direction"] = briefBody
		addFixedBodies(t, root, rawBySlot)
		record := readRecord(t, composition)
		packet := verifyPacket(t, root, output, record, rawBySlot, []string{"task-direction"})
		if len(packet) > 65536 {
			t.Fatalf("fresh packet has %d bytes, cap is 65536", len(packet))
		}
		staged, err := os.ReadFile(filepath.Join(stageDir, "task-direction.md"))
		if err != nil || !bytes.Equal(staged, briefBody) {
			t.Fatalf("fresh command staged wrong task direction: %v", err)
		}
		promoteAndVerify(t, root, stageDir, referenceDir, composition, 1)

		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("dispatch.max-inline-input-kb=1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		lowOutput := filepath.Join(root, "fresh-low-prompt.md")
		lowComposition := filepath.Join(root, "fresh-low-composition.json")
		lowStage := filepath.Join(root, "record-locks", "fresh-low")
		lowArgs := composeArgs(root, "fresh-low", brief, lowOutput, lowComposition, lowStage, filepath.Join(root, "artifacts", "fresh-low", "rounds", "1", "staged"), 1, nil)
		assertImpossible(t, lowArgs, lowOutput, lowComposition, lowStage, 1024)
	})

	t.Run("continuations", func(t *testing.T) {
		root := newRoot(t)
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("dispatch.max-inline-input-kb=24\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		fixed := map[string][]byte{
			"role-instructions": bytes.Repeat([]byte("i"), 1024),
			"required-skill":    bytes.Repeat([]byte("s"), 4096),
			"response-contract": bytes.Repeat([]byte("c"), 1024),
		}
		for slot, path := range map[string]string{
			"role-instructions": "scripts/agents/roles/implementer.md",
			"required-skill":    "docs/orchestration.md",
			"response-contract": "scripts/agents/schemas/implementer.schema.json",
		} {
			if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), fixed[slot], 0o644); err != nil {
				t.Fatal(err)
			}
		}
		taskBody := bytes.Repeat([]byte("t"), 8*1024)
		brief := filepath.Join(root, "continuation-brief.md")
		if err := os.WriteFile(brief, taskBody, 0o644); err != nil {
			t.Fatal(err)
		}
		bodyFixtures := []struct {
			slot string
			raw  []byte
		}{
			{slot: "prior-brief", raw: bytes.Repeat([]byte("b"), 12*1024)},
			{slot: "prior-return", raw: bytes.Repeat([]byte("r"), 16*1024)},
			{slot: "critique-register", raw: bytes.Repeat([]byte("q"), 1024)},
			{slot: "prior-worktree", raw: bytes.Repeat([]byte("w"), 1024)},
		}
		continuations := make([]string, 0, len(bodyFixtures))
		rawBySlot := generatedBodies("round-two", 2)
		rawBySlot["task-direction"] = taskBody
		for slot, raw := range fixed {
			rawBySlot[slot] = raw
		}
		for _, body := range bodyFixtures {
			path := filepath.Join(root, body.slot+".md")
			if err := os.WriteFile(path, body.raw, 0o644); err != nil {
				t.Fatal(err)
			}
			continuations = append(continuations, body.slot+"="+path)
			rawBySlot[body.slot] = body.raw
		}
		output := filepath.Join(root, "round-two-prompt.md")
		composition := filepath.Join(root, "round-two-composition.json")
		stageDir := filepath.Join(root, "record-locks", "round-two")
		referenceDir := filepath.Join(root, "artifacts", "round-two", "rounds", "2", "staged")
		args := composeArgs(root, "round-two", brief, output, composition, stageDir, referenceDir, 2, continuations)
		if code := runDispatchComposeRolePacket(args); code != 0 {
			t.Fatalf("round-two composition exit = %d", code)
		}
		record := readRecord(t, composition)
		packet := verifyPacket(t, root, output, record, rawBySlot, []string{"prior-brief", "prior-return"})
		if len(packet) > 24*1024 {
			t.Fatalf("round-two packet has %d bytes, cap is %d", len(packet), 24*1024)
		}
		for _, slot := range []string{"prior-brief", "prior-return"} {
			staged, err := os.ReadFile(filepath.Join(stageDir, slot+".md"))
			if err != nil || !bytes.Equal(staged, rawBySlot[slot]) {
				t.Fatalf("round-two command staged wrong %s: %v", slot, err)
			}
		}
		promoteAndVerify(t, root, stageDir, referenceDir, composition, 2)

		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("dispatch.max-inline-input-kb=1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		lowOutput := filepath.Join(root, "round-two-low-prompt.md")
		lowComposition := filepath.Join(root, "round-two-low-composition.json")
		lowStage := filepath.Join(root, "record-locks", "round-two-low")
		lowArgs := composeArgs(root, "round-two-low", brief, lowOutput, lowComposition, lowStage, filepath.Join(root, "artifacts", "round-two-low", "rounds", "2", "staged"), 2, continuations)
		assertImpossible(t, lowArgs, lowOutput, lowComposition, lowStage, 1024)
	})
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
	code, stdout, _ := captureCommandOutput(t, true, false, fn)
	return stdout, code
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
	if code := runDispatchCritiqueClose([]string{"--repo", repo}); code != 2 {
		t.Fatalf("critic chain close usage error exit=%d, want 2", code)
	}
	for _, args := range [][]string{
		{"--repo", repo, "--root-job", "authorized", "--root-job", "redirected"},
		{"--repo", repo, "--root-job", "authorized", "--repo", t.TempDir()},
	} {
		if code := runDispatchCritiqueClose(args); code != 2 {
			t.Fatalf("critic chain close repeated authority flag %v exit=%d, want 2", args, code)
		}
	}
}

func TestDispatchCritiqueReadAdmissionResult(t *testing.T) {
	subject := dispatchcore.ReadSubject{
		Kind:                dispatchcore.SubjectLive,
		ImplementerRoot:     "implementer",
		ReviewedMember:      "implementer",
		ReviewedProjectTree: strings.Repeat("a", 40),
		DiffDigest:          strings.Repeat("b", 64),
	}

	readResult := func(t *testing.T, path string) dispatchcore.ReadAdmissionResult {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var result dispatchcore.ReadAdmissionResult
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	writeSubject := func(t *testing.T, dir string, value dispatchcore.ReadSubject) string {
		t.Helper()
		return writeTemp(t, dir, "subject.json", value)
	}
	seedScope := func(t *testing.T, repo string) string {
		t.Helper()
		jobs := filepath.Join(repo, "artifacts", "agents", "jobs")
		if err := os.MkdirAll(jobs, 0o755); err != nil {
			t.Fatal(err)
		}
		writeTemp(t, jobs, "implementer.json", map[string]any{
			"jobId": "implementer", "role": "implementer", "round": 1,
			"parentJob": nil, "status": "completed",
		})
		return jobs
	}
	seedRead := func(t *testing.T, repo, root, status string, clean bool) {
		t.Helper()
		jobs := seedScope(t, repo)
		record := map[string]any{
			"jobId": root, "role": "code-critic", "round": 1,
			"parentJob": nil, "status": status, "reviews": "implementer",
			"findingRegister": []any{}, "findingRegisterRound": 0,
		}
		if clean {
			record["findingRegisterRound"] = 1
			record["findingRegisterSubjectDigest"] = subject.Digest()
			record["cleanReadRounds"] = []any{map[string]any{"round": 1, "subject": subject}}
		}
		writeTemp(t, jobs, root+".json", record)
		roundDir := filepath.Join(repo, "artifacts", "agents", root, "rounds", "1")
		if err := os.MkdirAll(roundDir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeTemp(t, roundDir, "subject.json", subject)
		writeTemp(t, roundDir, "return.json", map[string]any{
			"schemaVersion": 3, "jobId": root, "round": 1,
			"reviewedTree": subject.ReviewedProjectTree,
			"findings":     []any{}, "rigor": []any{},
		})
	}
	run := func(t *testing.T, repo, role, rootJob, round, subjectFile, resultFile string) (string, int) {
		t.Helper()
		return captureStderr(t, func() int {
			return runDispatchCritiqueReadAdmission([]string{
				"--repo", repo, "--role", role, "--root-job", rootJob,
				"--round", round, "--subject-file", subjectFile, "--result", resultFile,
			})
		})
	}

	t.Run("admitted", func(t *testing.T) {
		repo := t.TempDir()
		subjectFile := writeSubject(t, t.TempDir(), subject)
		resultFile := filepath.Join(t.TempDir(), "result.json")
		stderr, code := run(t, repo, "code-critic", "candidate", "1", subjectFile, resultFile)
		result := readResult(t, resultFile)
		if code != 0 || stderr != "" || result.Decision != "ADMITTED" || result.SubjectDigest != subject.Digest() {
			t.Fatalf("admitted command = exit %d stderr %q result %+v", code, stderr, result)
		}
	})

	t.Run("redundant", func(t *testing.T) {
		repo := t.TempDir()
		seedRead(t, repo, "prior", "completed", true)
		resultFile := filepath.Join(t.TempDir(), "result.json")
		stderr, code := run(t, repo, "code-critic", "candidate", "1", writeSubject(t, t.TempDir(), subject), resultFile)
		result := readResult(t, resultFile)
		if code != 11 || !strings.Contains(stderr, "REDUNDANT_READ") || result.Decision != "REDUNDANT_READ" ||
			result.CriticRoot != "prior" || result.Round != 1 || result.EventID == "" || !result.EventRecorded || !result.EventDurable {
			t.Fatalf("redundant command = exit %d stderr %q result %+v", code, stderr, result)
		}
		events, err := dispatchcore.LoadReadRefusals(filepath.Join(repo, "artifacts", "agents", "prior", "reads-refused.jsonl"))
		if err != nil || len(events) != 1 || events[0].ID != result.EventID {
			t.Fatalf("recorded event = %+v, %v; result id %q", events, err, result.EventID)
		}
	})

	t.Run("concurrent", func(t *testing.T) {
		repo := t.TempDir()
		seedRead(t, repo, "outstanding", "running", false)
		resultFile := filepath.Join(t.TempDir(), "result.json")
		stderr, code := run(t, repo, "code-critic", "candidate", "1", writeSubject(t, t.TempDir(), subject), resultFile)
		result := readResult(t, resultFile)
		if code != 11 || !strings.Contains(stderr, "CONCURRENT_READ") || result.Decision != "CONCURRENT_READ" ||
			result.CriticRoot != "outstanding" || result.Round != 1 || result.EventRecorded || result.EventID != "" {
			t.Fatalf("concurrent command = exit %d stderr %q result %+v", code, stderr, result)
		}
	})

	t.Run("event-write-failure", func(t *testing.T) {
		repo := t.TempDir()
		seedRead(t, repo, "prior", "completed", true)
		refusalPath := filepath.Join(repo, "artifacts", "agents", "prior", "reads-refused.jsonl")
		if err := os.Mkdir(refusalPath, 0o755); err != nil {
			t.Fatal(err)
		}
		resultFile := filepath.Join(t.TempDir(), "result.json")
		stderr, code := run(t, repo, "code-critic", "candidate", "1", writeSubject(t, t.TempDir(), subject), resultFile)
		result := readResult(t, resultFile)
		if code != 11 || !strings.Contains(stderr, "refusal event was not recorded") || result.Decision != "REDUNDANT_READ" ||
			result.EventID == "" || result.EventRecorded || result.EventDurable {
			t.Fatalf("event failure command = exit %d stderr %q result %+v", code, stderr, result)
		}
	})

	t.Run("malformed-input", func(t *testing.T) {
		for name, input := range map[string]string{
			"malformed-json":    "{",
			"malformed-subject": `{"kind":"live"}`,
		} {
			t.Run(name, func(t *testing.T) {
				inputPath := filepath.Join(t.TempDir(), "subject.json")
				if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
					t.Fatal(err)
				}
				resultFile := filepath.Join(t.TempDir(), "result.json")
				_, code := run(t, t.TempDir(), "code-critic", "candidate", "1", inputPath, resultFile)
				_ = readResult(t, resultFile)
				if code == 0 {
					t.Fatal("malformed subject was admitted")
				}
			})
		}
	})

	t.Run("strict-flags-and-coordinates", func(t *testing.T) {
		repo := t.TempDir()
		subjectFile := writeSubject(t, t.TempDir(), subject)
		resultFile := filepath.Join(t.TempDir(), "result.json")
		base := []string{"--repo", repo, "--role", "code-critic", "--root-job", "candidate", "--round", "1", "--subject-file", subjectFile, "--result", resultFile}
		for flagName, value := range map[string]string{
			"repo": repo, "role": "code-critic", "root-job": "redirected",
			"round": "2", "subject-file": subjectFile, "result": filepath.Join(t.TempDir(), "other.json"),
		} {
			t.Run("repeated-"+flagName, func(t *testing.T) {
				args := append(append([]string{}, base...), "--"+flagName, value)
				if _, code := captureStderr(t, func() int { return runDispatchCritiqueReadAdmission(args) }); code != 2 {
					t.Fatalf("exit = %d, want 2", code)
				}
			})
		}
		if _, code := captureStderr(t, func() int {
			return runDispatchCritiqueReadAdmission(append(append([]string{}, base...), "extra"))
		}); code != 2 {
			t.Fatalf("positional argument exit = %d, want 2", code)
		}
		for name, values := range map[string][3]string{
			"invalid-id":    {"code-critic", "Bad_ID", "1"},
			"invalid-round": {"code-critic", "candidate", "0"},
			"role-mismatch": {"design-critic", "candidate", "1"},
		} {
			t.Run(name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "result.json")
				_, code := run(t, repo, values[0], values[1], values[2], subjectFile, path)
				_ = readResult(t, path)
				if code == 0 {
					t.Fatal("invalid request was admitted")
				}
			})
		}
	})

	t.Run("unreadable-subject-and-result-failure", func(t *testing.T) {
		resultFile := filepath.Join(t.TempDir(), "result.json")
		_, code := run(t, t.TempDir(), "code-critic", "candidate", "1", filepath.Join(t.TempDir(), "missing.json"), resultFile)
		_ = readResult(t, resultFile)
		if code == 0 {
			t.Fatal("unreadable subject was admitted")
		}
		resultDir := t.TempDir()
		_, code = run(t, t.TempDir(), "code-critic", "candidate", "1", writeSubject(t, t.TempDir(), subject), resultDir)
		if code == 0 {
			t.Fatal("result-file failure admitted the read")
		}
	})
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
