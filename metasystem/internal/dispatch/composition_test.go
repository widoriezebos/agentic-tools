package dispatch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func compositionRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestComposeRolePacketFollowsClosedRecipeAndRecordsEveryRange(t *testing.T) {
	root := compositionRepoRoot(t)
	temp := t.TempDir()
	brief := filepath.Join(temp, "brief.md")
	if err := os.WriteFile(brief, []byte("Do the focused task.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prompt := filepath.Join(temp, "prompt.md")
	composition := filepath.Join(temp, "composition.json")
	record, err := ComposeRolePacket(ComposeRolePacketParams{
		Root: root, Role: "verifier", Brief: brief, JobID: "verify-a", Runtime: "fake",
		Model: "fake-model", ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardMechanical, Output: prompt, CompositionOutput: composition,
		ExtraSources: []string{"skills/verify/SKILL.md"},
	})
	if err != nil {
		t.Fatal(err)
	}
	packet, err := os.ReadFile(prompt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(packet), "# Task Direction\n\nDo the focused task.\n") {
		t.Fatalf("packet does not begin with the exact task direction:\n%s", packet)
	}
	wantSlots := []string{"task-direction", "role-instructions", "required-skill", "response-contract", "tool-names", "generated-runtime-notice"}
	if len(record.Sources) != len(wantSlots) {
		t.Fatalf("source count = %d, want %d", len(record.Sources), len(wantSlots))
	}
	for index, source := range record.Sources {
		if source.Slot != wantSlots[index] {
			t.Fatalf("source %d slot = %s, want %s", index, source.Slot, wantSlots[index])
		}
		if source.StartByte < 0 || source.EndByte > len(packet) || source.StartByte >= source.EndByte {
			t.Fatalf("source %s has invalid range %d:%d", source.Slot, source.StartByte, source.EndByte)
		}
		if digestBytes(packet[source.StartByte:source.EndByte]) != source.DeliveredDigest {
			t.Fatalf("source %s delivered digest does not match its packet range", source.Slot)
		}
	}
	if digestBytes(packet) != record.PacketDigest || record.ContextProof.Classification != "advisory" ||
		record.ContextProof.ProofState != "no-leak-not-proven" || record.ContextProof.ReasonCode != "BROAD-READ-RUNTIME" {
		t.Fatalf("packet digest or bootstrap-honest classification is wrong: %+v", record)
	}
	if record.MachineSlot.Outcome != "DEFERRED" || record.MachineSlot.OwnerGoal != "machine-concurrency-governor" {
		t.Fatalf("machine governor seam was not retained: %+v", record.MachineSlot)
	}
	if record.ToolSurface.Policy != "read-only" || record.ToolSurface.NameState != "exact" || len(record.ToolSurface.Names) != 0 {
		t.Fatalf("fake runtime tool surface is not exact: %+v", record.ToolSurface)
	}
	if record.DestructiveReach != HazardMechanical || record.ConfigurationObligations.BuilderEffortTier != "ordinary" {
		t.Fatalf("hazard configuration was not recorded: %+v", record.ConfigurationObligations)
	}
	stored, err := os.ReadFile(composition)
	if err != nil || !json.Valid(stored) {
		t.Fatalf("composition record is not stored JSON: %v", err)
	}
}

func TestComposeRolePacketRefusesCallerSourceOutsideRecipeBeforeWriting(t *testing.T) {
	root := compositionRepoRoot(t)
	temp := t.TempDir()
	brief := filepath.Join(temp, "brief.md")
	if err := os.WriteFile(brief, []byte("Do the focused task.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prompt := filepath.Join(temp, "prompt.md")
	composition := filepath.Join(temp, "composition.json")
	_, err := ComposeRolePacket(ComposeRolePacketParams{
		Root: root, Role: "verifier", Brief: brief, JobID: "verify-a", Runtime: "fake",
		Model: "fake-model", ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardMechanical, Output: prompt, CompositionOutput: composition,
		ExtraSources: []string{"plans/role-context-composition-design.md"},
	})
	var refusal *CompositionRefusal
	if !errors.As(err, &refusal) || refusal.Code != "REFUSED-CONTEXT-SOURCE" {
		t.Fatalf("forbidden source result = %T %v", err, err)
	}
	for _, path := range []string{prompt, composition} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("refused composition wrote %s", path)
		}
	}
}

func TestJobRecordRejectsExpandedOrDishonestComposition(t *testing.T) {
	root := compositionRepoRoot(t)
	temp := t.TempDir()
	brief := filepath.Join(temp, "brief.md")
	prompt := filepath.Join(temp, "prompt.md")
	composition := filepath.Join(temp, "composition.json")
	if err := os.WriteFile(brief, []byte("Do the focused task.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record, err := ComposeRolePacket(ComposeRolePacketParams{
		Root: root, Role: "verifier", Brief: brief, JobID: "verify-a", Runtime: "fake",
		Model: "fake-model", ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardMechanical, Output: prompt, CompositionOutput: composition,
	})
	if err != nil {
		t.Fatal(err)
	}
	packet, err := os.ReadFile(prompt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readCompositionForJob(composition, "verify-a", "verifier", "fake", "fake-model", "", HazardMechanical, 1, int64(len(packet)), record.PacketDigest); err != nil {
		t.Fatalf("generated composition did not validate: %v", err)
	}

	var expanded map[string]any
	stored, err := os.ReadFile(composition)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(stored, &expanded); err != nil {
		t.Fatal(err)
	}
	expanded["undeclaredContext"] = "must not enter the job record"
	tampered, err := json.Marshal(expanded)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(composition, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readCompositionForJob(composition, "verify-a", "verifier", "fake", "fake-model", "", HazardMechanical, 1, int64(len(packet)), record.PacketDigest); err == nil || !strings.Contains(err.Error(), "expanded") {
		t.Fatalf("expanded composition result = %v", err)
	}
}

func TestRolePacketTableCoversEveryDispatchableRole(t *testing.T) {
	root := compositionRepoRoot(t)
	tableBytes, err := os.ReadFile(filepath.Join(root, rolePacketTablePath))
	if err != nil {
		t.Fatal(err)
	}
	var table rolePacketTable
	if err := json.Unmarshal(tableBytes, &table); err != nil {
		t.Fatal(err)
	}
	entries, err := filepath.Glob(filepath.Join(root, "scripts", "agents", "roles", "*.requirements.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := make([]string, 0, len(entries))
	for _, entry := range entries {
		want = append(want, strings.TrimSuffix(filepath.Base(entry), ".requirements.json"))
	}
	sort.Strings(want)
	got := make([]string, 0, len(table.Roles))
	for role := range table.Roles {
		got = append(got, role)
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("role packet table = %v, dispatchable roles = %v", got, want)
	}
	if len(table.DestructiveReach) != 3 {
		t.Fatalf("destructiveReach class count = %d, want 3", len(table.DestructiveReach))
	}
}

func TestHazardConfigurationRefusesAWeakenedMinimum(t *testing.T) {
	root := compositionRepoRoot(t)
	destructive, err := ResolveHazardConfiguration(root, HazardDestructiveReach)
	if err != nil {
		t.Fatal(err)
	}
	if destructive.BuilderEffortTier != "maximal" || destructive.BuilderReasoningEffort != "xhigh" ||
		!destructive.IndependentCritiqueRequired || destructive.IndependentCritiqueEffortTier != "maximal" ||
		destructive.IndependentCritiqueReasoningEffort != "xhigh" || !destructive.LiveProofRequired {
		t.Fatalf("destructive-reach minimum is incomplete: %+v", destructive)
	}

	tableBytes, err := os.ReadFile(filepath.Join(root, rolePacketTablePath))
	if err != nil {
		t.Fatal(err)
	}
	var table map[string]any
	if err := json.Unmarshal(tableBytes, &table); err != nil {
		t.Fatal(err)
	}
	classes := table["destructiveReach"].(map[string]any)
	weakened := classes[string(HazardDestructiveReach)].(map[string]any)
	weakened["builderReasoningEffort"] = "medium"
	encoded, err := json.Marshal(table)
	if err != nil {
		t.Fatal(err)
	}
	tamperedRoot := t.TempDir()
	path := filepath.Join(tamperedRoot, rolePacketTablePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveHazardConfiguration(tamperedRoot, HazardDestructiveReach); err == nil || !strings.Contains(err.Error(), "does not match its required configuration") {
		t.Fatalf("weakened destructive-reach configuration result = %v", err)
	}
}

func TestHazardConfigurationRefusesRuntimeWithoutExecutableMaximum(t *testing.T) {
	root := compositionRepoRoot(t)
	temp := t.TempDir()
	brief := filepath.Join(temp, "brief.md")
	if err := os.WriteFile(brief, []byte("Do the focused task.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prompt := filepath.Join(temp, "prompt.md")
	composition := filepath.Join(temp, "composition.json")
	_, err := ComposeRolePacket(ComposeRolePacketParams{
		Root: root, Role: "verifier", Brief: brief, JobID: "verify-a", Runtime: "claude",
		Model: "claude-non-maximal", ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardDestructiveReach,
		Output: prompt, CompositionOutput: composition,
	})
	var refusal *CompositionRefusal
	if !errors.As(err, &refusal) || refusal.Code != "REFUSED-HAZARD-CONFIGURATION" || !strings.Contains(refusal.Detail, "no executable maximal-effort mapping") {
		t.Fatalf("unsupported runtime hazard result = %T %v", err, err)
	}
	for _, path := range []string{prompt, composition} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("refused hazard composition wrote %s", path)
		}
	}
}

func TestHazardConfigurationAcceptsConfiguredMaximalModel(t *testing.T) {
	root := compositionRepoRoot(t)
	temp := t.TempDir()
	brief := filepath.Join(temp, "brief.md")
	if err := os.WriteFile(brief, []byte("Working Mode: review\nReview the focused change.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record, err := ComposeRolePacket(ComposeRolePacketParams{
		Root: root, Role: "code-critic", Brief: brief, JobID: "claude-maximal", Runtime: "claude",
		Model: "claude-fable-5-1", ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardDesignBearing,
		Output: filepath.Join(temp, "prompt.md"), CompositionOutput: filepath.Join(temp, "composition.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Runtime != "claude" || record.Model != "claude-fable-5-1" ||
		record.ConfigurationObligations.BuilderEffortTier != "maximal" ||
		record.ConfigurationObligations.BuilderReasoningEffort != "xhigh" {
		t.Fatalf("configured maximal-model composition = %+v", record)
	}
}

func TestHazardConfigurationUsesResolvedLocalMaximalModelMapping(t *testing.T) {
	root := t.TempDir()
	conf := filepath.Join(root, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("runtime.claude.maximal-models=claude-base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conf+".local", []byte("runtime.claude.maximal-models=claude-fable-5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRuntimeHazardConfiguration(root, "claude", "claude-fable-5", HazardDesignBearing); err != nil {
		t.Fatalf("resolved local maximal model refused: %v", err)
	}
	if err := ValidateRuntimeHazardConfiguration(root, "claude", "claude-base", HazardDesignBearing); err == nil || !strings.Contains(err.Error(), "no executable maximal-effort mapping") {
		t.Fatalf("shadowed committed maximal model result = %v", err)
	}
}

func closeReadyHazardChain(t *testing.T, class HazardClass) (repo, evidence, job string) {
	t.Helper()
	repo, evidence, job = mirrorFixture(t)
	path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
	record := readJSONFile(t, path)
	record["destructiveReach"] = class
	record["configurationObligations"] = requiredConfigurationByHazard[class]
	record["dispatchMode"] = DispatchModeFresh
	record["resumedSessionId"] = nil
	record["sessionId"] = "builder-session"
	record["endedAt"] = "2026-08-30T10:00:00Z"
	writeRecord(path, record)
	refreshHazardMirror(t, repo, evidence, job)
	return repo, evidence, job
}

func refreshHazardMirror(t *testing.T, repo, evidence, job string) {
	t.Helper()
	result := filepath.Join(t.TempDir(), "mirror.json")
	if err := Mirror(repo, repo, evidence, job, job, result); err != nil {
		t.Fatal(err)
	}
	mirrored := readJSONFile(t, result)
	path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
	record := readJSONFile(t, path)
	record["mirror"] = map[string]any{"path": asString(mirrored["path"]), "manifest": mirrored["manifest"]}
	writeRecord(path, record)
	if err := Mirror(repo, repo, evidence, job, job, result); err != nil {
		t.Fatal(err)
	}
}

func writeHazardEvidenceJob(t *testing.T, repo, job string, class HazardClass, record map[string]any) {
	t.Helper()
	stampHazardEvidenceAdmission(t, repo, job, class, record)
	writeHandwrittenHazardEvidenceJob(t, repo, job, record)
}

func writeHandwrittenHazardEvidenceJob(t *testing.T, repo, job string, record map[string]any) {
	t.Helper()
	record["jobId"] = job
	record["status"] = "completed"
	if _, present := record["endedAt"]; !present {
		record["endedAt"] = "2026-08-30T10:01:00Z"
	}
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), job+".json", record)
}

func stampHazardEvidenceAdmission(t *testing.T, repo, job string, class HazardClass, record map[string]any) {
	t.Helper()
	digest := strings.Repeat("a", 64)
	runtimeName := asString(record["runtime"])
	if runtimeName == "" {
		runtimeName = "fake"
	}
	model := asString(record["requestedModel"])
	if model == "" {
		model = "fake-model"
	}
	request := CanonicalLaunchRequest{
		SessionKey: "session-" + job, DispatchMode: DispatchModeFresh, ResumedSessionID: "",
		Runtime: runtimeName, CanonicalModelKey: model, Role: asString(record["role"]),
		LaunchMode: LaunchModeSharedCheckout, PermissionEnvelopeDigest: digest,
		ProductRoots: []string{repo}, CapMinutes: 30, InputHash: strings.Repeat("b", 64),
		DestructiveReach: class,
	}
	fingerprint, err := LaunchFingerprintV2(request)
	if err != nil {
		t.Fatal(err)
	}
	tag := "metasystem-job-" + job + "-evidence"
	record["operationId"] = job
	record["round"] = 1
	if _, present := record["parentJob"]; !present {
		record["parentJob"] = nil
	}
	record["proofLevel"] = "proven"
	record["sessionKey"] = request.SessionKey
	record["dispatchMode"] = request.DispatchMode
	record["resumedSessionId"] = request.ResumedSessionID
	record["runtime"] = request.Runtime
	record["canonicalModelKey"] = request.CanonicalModelKey
	record["requestedModel"] = model
	record["launchMode"] = request.LaunchMode
	record["permissionEnvelopeDigest"] = request.PermissionEnvelopeDigest
	record["productRoots"] = request.ProductRoots
	record["capMin"] = request.CapMinutes
	record["capRequest"] = map[string]any{"minutes": request.CapMinutes}
	record["inputHash"] = request.InputHash
	record["goalId"] = nil
	record["goalRevision"] = nil
	record["destructiveReach"] = request.DestructiveReach
	record["fingerprintVersion"] = fingerprint.Version
	record["fingerprint"] = fingerprint.Digest
	record["instanceTag"] = tag
	record["launchCapability"] = map[string]any{
		"digest": strings.Repeat("c", 64), "jobId": job, "operationId": job,
		"instanceTag": tag, "adapterVerb": "dispatch", "status": "consumed",
		"mintedAt": "2026-08-30T09:58:00Z", "consumedAt": "2026-08-30T09:59:00Z",
		"supervisor": exactIdentityFields(nativeTestExact(7001, 3).Ref()),
	}
	if _, present := record["configurationObligations"]; !present {
		record["configurationObligations"] = requiredConfigurationByHazard[class]
	}
	if _, present := record["reasoningEffort"]; !present {
		record["reasoningEffort"] = requiredConfigurationByHazard[class].BuilderReasoningEffort
	}
}

func wantHazardClosureRefusal(t *testing.T, err error, reason string) {
	t.Helper()
	var refusal *OpError
	if !errors.As(err, &refusal) || refusal.Code != 9 || refusal.Reason != reason {
		t.Fatalf("closure refusal = %T %v, want typed %s", err, err, reason)
	}
}

func TestHazardDutiesGateChainCompletion(t *testing.T) {
	t.Run("destructive reach requires critique and live proof", func(t *testing.T) {
		repo, evidence, job := closeReadyHazardChain(t, HazardDestructiveReach)
		wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-INDEPENDENT-CRITIQUE")

		writeHazardEvidenceJob(t, repo, "critic-job", HazardDestructiveReach, map[string]any{
			"role": "code-critic", "reviews": job, "parentJob": nil,
			"dispatchMode": DispatchModeFresh, "resumedSessionId": nil,
			"sessionId": "critic-session", "reasoningEffort": "xhigh",
			"configurationObligations": requiredConfigurationByHazard[HazardDestructiveReach],
		})
		if err := StampClaimedReviewReference(repo, "critic-job"); err != nil {
			t.Fatalf("could not attach post-work critique evidence: %v", err)
		}
		refreshHazardMirror(t, repo, evidence, job)
		wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-LIVE-PROOF")

		writeHazardEvidenceJob(t, repo, "live-proof-job", HazardDestructiveReach, map[string]any{
			"role": "verifier", "reviews": job,
		})
		if err := StampClaimedReviewReference(repo, "live-proof-job"); err != nil {
			t.Fatalf("could not attach post-work live proof evidence: %v", err)
		}
		refreshHazardMirror(t, repo, evidence, job)
		if err := CloseCheck(repo, job); err != nil {
			t.Fatalf("complete destructive-reach evidence did not discharge the chain: %v", err)
		}
	})

	t.Run("mechanical needs neither", func(t *testing.T) {
		repo, _, job := closeReadyHazardChain(t, HazardMechanical)
		if err := CloseCheck(repo, job); err != nil {
			t.Fatalf("mechanical chain required hazard evidence: %v", err)
		}
	})

	t.Run("configured claude maximal model closes critique duty", func(t *testing.T) {
		repo, _, job := closeReadyHazardChain(t, HazardDesignBearing)
		if err := os.WriteFile(filepath.Join(repo, "metasystem.conf"), []byte("runtime.claude.maximal-models=claude-fable-5\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		writeHazardEvidenceJob(t, repo, "claude-critic", HazardDesignBearing, map[string]any{
			"role": "code-critic", "reviews": job, "parentJob": nil,
			"dispatchMode": DispatchModeFresh, "resumedSessionId": nil,
			"runtime": "claude", "requestedModel": "claude-fable-5",
			"sessionId": "claude-critic-session", "reasoningEffort": "xhigh",
			"configurationObligations": requiredConfigurationByHazard[HazardDesignBearing],
		})
		path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
		record := readJSONFile(t, path)
		record["independentCritiqueJobRef"] = "claude-critic"
		writeRecord(path, record)
		if err := CloseCheck(repo, job); err != nil {
			t.Fatalf("configured Claude maximal critic did not discharge the chain: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repo, "metasystem.conf"), []byte("runtime.claude.maximal-models=claude-other\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-INDEPENDENT-CRITIQUE")
	})

	t.Run("dangling critique reference", func(t *testing.T) {
		repo, _, job := closeReadyHazardChain(t, HazardDestructiveReach)
		path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
		record := readJSONFile(t, path)
		record["independentCritiqueJobRef"] = "missing-critic"
		record["liveProofEvidenceRef"] = "missing-proof"
		writeRecord(path, record)
		wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-INDEPENDENT-CRITIQUE")
	})

	for _, invalid := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "critic resumed instead of fresh", mutate: func(record map[string]any) { record["parentJob"] = "older-critic" }},
		{name: "critic reused builder session", mutate: func(record map[string]any) { record["sessionId"] = "builder-session" }},
		{name: "critic below maximum effort", mutate: func(record map[string]any) {
			record["reasoningEffort"] = "medium"
			record["configurationObligations"] = requiredConfigurationByHazard[HazardMechanical]
		}},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			repo, _, job := closeReadyHazardChain(t, HazardDesignBearing)
			critic := map[string]any{
				"role": "code-critic", "reviews": job, "parentJob": nil,
				"dispatchMode": DispatchModeFresh, "resumedSessionId": nil,
				"sessionId": "critic-session", "reasoningEffort": "xhigh",
				"configurationObligations": requiredConfigurationByHazard[HazardDesignBearing],
			}
			invalid.mutate(critic)
			writeHazardEvidenceJob(t, repo, "critic-job", HazardDesignBearing, critic)
			path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
			record := readJSONFile(t, path)
			record["independentCritiqueJobRef"] = "critic-job"
			writeRecord(path, record)
			wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-INDEPENDENT-CRITIQUE")
		})
	}

	t.Run("critic must be distinct", func(t *testing.T) {
		repo, _, job := closeReadyHazardChain(t, HazardDesignBearing)
		path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
		record := readJSONFile(t, path)
		record["independentCritiqueJobRef"] = job
		writeRecord(path, record)
		wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-INDEPENDENT-CRITIQUE")
	})

	t.Run("dangling live-proof reference", func(t *testing.T) {
		repo, _, job := closeReadyHazardChain(t, HazardDestructiveReach)
		writeHazardEvidenceJob(t, repo, "critic-job", HazardDestructiveReach, map[string]any{
			"role": "code-critic", "reviews": job, "parentJob": nil,
			"dispatchMode": DispatchModeFresh, "resumedSessionId": nil,
			"sessionId": "critic-session", "reasoningEffort": "xhigh",
			"configurationObligations": requiredConfigurationByHazard[HazardDestructiveReach],
		})
		path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
		record := readJSONFile(t, path)
		record["independentCritiqueJobRef"] = "critic-job"
		record["liveProofEvidenceRef"] = "missing-proof"
		writeRecord(path, record)
		wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-LIVE-PROOF")
	})
}

func TestHazardEvidenceRequiresAdmissionProvenance(t *testing.T) {
	repo, evidence, job := closeReadyHazardChain(t, HazardDestructiveReach)
	critic := map[string]any{
		"role": "code-critic", "reviews": job, "parentJob": nil,
		"dispatchMode": DispatchModeFresh, "resumedSessionId": nil,
		"sessionId": "critic-session", "reasoningEffort": "xhigh",
		"configurationObligations": requiredConfigurationByHazard[HazardDestructiveReach],
	}
	proof := map[string]any{"role": "verifier", "reviews": job}
	writeHandwrittenHazardEvidenceJob(t, repo, "critic-c", critic)
	writeHandwrittenHazardEvidenceJob(t, repo, "proof-c", proof)
	path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
	record := readJSONFile(t, path)
	record["independentCritiqueJobRef"] = "critic-c"
	record["liveProofEvidenceRef"] = "proof-c"
	writeRecord(path, record)
	refreshHazardMirror(t, repo, evidence, job)
	wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-EVIDENCE-PROVENANCE")

	writeHazardEvidenceJob(t, repo, "critic-c", HazardDestructiveReach, critic)
	wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-EVIDENCE-PROVENANCE")

	writeHazardEvidenceJob(t, repo, "proof-c", HazardDestructiveReach, proof)
	proofPath := filepath.Join(repo, "artifacts", "agents", "jobs", "proof-c.json")
	proof = readJSONFile(t, proofPath)
	proof["fingerprint"] = strings.Repeat("d", 64)
	writeRecord(proofPath, proof)
	wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-EVIDENCE-PROVENANCE")

	writeHazardEvidenceJob(t, repo, "proof-c", HazardDestructiveReach, proof)
	proof = readJSONFile(t, proofPath)
	capability := proof["launchCapability"].(map[string]any)
	delete(capability, "consumedAt")
	writeRecord(proofPath, proof)
	wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-EVIDENCE-PROVENANCE")
}

func TestHazardEvidenceMustCoverFinalWorkState(t *testing.T) {
	repo, evidence, job := closeReadyHazardChain(t, HazardDestructiveReach)
	agents := filepath.Join(repo, "artifacts", "agents")
	terminalJob := job + "-r2"
	writeJSONFile(t, filepath.Join(agents, "jobs"), terminalJob+".json", map[string]any{
		"jobId": terminalJob, "round": 2, "parentJob": job, "status": "completed",
		"role": "implementer", "endedAt": "2026-08-30T10:02:00Z",
		"destructiveReach":         HazardDestructiveReach,
		"configurationObligations": requiredConfigurationByHazard[HazardDestructiveReach],
		"capabilitySnapshot":       "artifacts/agents/capabilities/snap.json",
	})
	if err := os.MkdirAll(filepath.Join(agents, job, "rounds", "2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agents, job, "rounds", "2", "diff.patch"), []byte("round two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := filepath.Join(t.TempDir(), "mirror.json")
	if err := Mirror(repo, repo, evidence, job, terminalJob, result); err != nil {
		t.Fatal(err)
	}

	critic := map[string]any{
		"role": "code-critic", "reviews": job, "parentJob": nil,
		"sessionId": "critic-session", "reasoningEffort": "xhigh",
		"configurationObligations": requiredConfigurationByHazard[HazardDestructiveReach],
		"endedAt":                  "2026-08-30T10:01:00Z",
	}
	writeHazardEvidenceJob(t, repo, "critic-final", HazardDestructiveReach, critic)
	proof := map[string]any{
		"role": "verifier", "reviews": terminalJob,
		"endedAt": "2026-08-30T10:03:00Z",
	}
	writeHazardEvidenceJob(t, repo, "proof-final", HazardDestructiveReach, proof)
	rootPath := filepath.Join(agents, "jobs", job+".json")
	root := readJSONFile(t, rootPath)
	root["independentCritiqueJobRef"] = "critic-final"
	root["liveProofEvidenceRef"] = "proof-final"
	writeRecord(rootPath, root)
	refreshHazardMirror(t, repo, evidence, job)
	wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-CRITIQUE-STALE")

	critic["reviews"] = terminalJob
	writeHazardEvidenceJob(t, repo, "critic-final", HazardDestructiveReach, critic)
	wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-CRITIQUE-STALE")

	critic["endedAt"] = "2026-08-30T10:02:00Z"
	writeHazardEvidenceJob(t, repo, "critic-final", HazardDestructiveReach, critic)
	if err := CloseCheck(repo, job); err != nil {
		t.Fatalf("critique and live proof of the terminal work round did not discharge the chain: %v", err)
	}

	proof["endedAt"] = "2026-08-30T10:01:00Z"
	writeHazardEvidenceJob(t, repo, "proof-final", HazardDestructiveReach, proof)
	wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-LIVE-PROOF-STALE")

	proof["reviews"] = job
	proof["endedAt"] = "2026-08-30T10:03:00Z"
	writeHazardEvidenceJob(t, repo, "proof-final", HazardDestructiveReach, proof)
	wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-LIVE-PROOF-STALE")
}

func TestDefaultOperationIdentityAndFingerprintBindGoalRevision(t *testing.T) {
	digest := strings.Repeat("a", 64)
	one, err := DefaultOperationID("goal-a", 7, DispatchModeFresh, "verifier", digest, "")
	if err != nil {
		t.Fatal(err)
	}
	two, err := DefaultOperationID("goal-a", 8, DispatchModeFresh, "verifier", digest, "")
	if err != nil {
		t.Fatal(err)
	}
	if one == two {
		t.Fatal("goal revision change did not mint a distinct operation id")
	}
	followOne, err := DefaultOperationID("goal-a", 7, DispatchModeFollowUp, "verifier", digest, "chain-a")
	if err != nil {
		t.Fatal(err)
	}
	followTwo, err := DefaultOperationID("goal-a", 8, DispatchModeFollowUp, "verifier", digest, "chain-a")
	if err != nil {
		t.Fatal(err)
	}
	if followOne == followTwo || followOne == one {
		t.Fatal("follow-up operation identity did not bind mode and goal revision")
	}
	followOtherChain, err := DefaultOperationID("goal-a", 7, DispatchModeFollowUp, "verifier", digest, "chain-b")
	if err != nil {
		t.Fatal(err)
	}
	if followOtherChain == followOne {
		t.Fatal("follow-up operation identity did not bind the direct parent")
	}
	request := launchFingerprintRequestForTest()
	request.GoalID, request.GoalRevision = "goal-a", 7
	root := t.TempDir()
	fingerprintOne, err := CanonicalizeLaunchFingerprint(root, request, 120)
	if err != nil {
		t.Fatal(err)
	}
	request.GoalRevision = 8
	fingerprintTwo, err := CanonicalizeLaunchFingerprint(root, request, 120)
	if err != nil {
		t.Fatal(err)
	}
	if fingerprintOne.Version != 2 || fingerprintOne.Digest == fingerprintTwo.Digest {
		t.Fatalf("v2 fingerprints did not bind revision: %+v %+v", fingerprintOne, fingerprintTwo)
	}
}
