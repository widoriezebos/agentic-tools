package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type hazardClosureFixture struct {
	t              *testing.T
	repo           string
	agents         string
	jobs           string
	implementer    string
	critic         string
	rootRecord     map[string]any
	memberIDs      map[string]bool
	memberSessions map[string]bool
	finalState     hazardFinalWorkState
	subject        ReadSubject
}

func newHazardClosureFixture(t *testing.T) *hazardClosureFixture {
	t.Helper()
	repo := t.TempDir()
	agents := filepath.Join(repo, "artifacts", "agents")
	fixture := &hazardClosureFixture{
		t:              t,
		repo:           repo,
		agents:         agents,
		jobs:           filepath.Join(agents, "jobs"),
		implementer:    "implementation",
		critic:         "critic",
		memberIDs:      map[string]bool{"implementation": true},
		memberSessions: map[string]bool{"builder-session": true},
	}
	fixture.subject = fixture.writeImplementerRound("implementation", 1, nil, "completed", strings.Repeat("a", 40), []byte("same patch\n"), true)
	fixture.finalState = hazardFinalWorkState{job: "implementation", round: 1, endedAt: mustHazardTime(t, "2026-08-30T10:00:00Z")}
	fixture.rootRecord = map[string]any{"independentCritiqueJobRef": fixture.critic}
	writeHazardEvidenceJob(t, repo, fixture.critic, HazardDesignBearing, map[string]any{
		"role": "code-critic", "reviews": fixture.implementer,
		"sessionId": "critic-session", "reasoningEffort": "xhigh",
		"endedAt":                  "2026-08-30T10:01:00Z",
		"configurationObligations": requiredConfigurationByHazard[HazardDesignBearing],
	})
	fixture.writeClosure(1, fixture.critic, fixture.subject, "completed")
	return fixture
}

func mustHazardTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func (f *hazardClosureFixture) writeImplementerRound(job string, round int64, parent any, status, tree string, patch []byte, artifacts bool) ReadSubject {
	f.t.Helper()
	record := map[string]any{
		"jobId": job, "role": "implementer", "round": round, "parentJob": parent,
		"status": status, "sessionId": "builder-session", "endedAt": "2026-08-30T10:02:00Z",
	}
	writeJSONFile(f.t, f.jobs, job+".json", record)
	f.memberIDs[job] = true
	if !artifacts {
		return ReadSubject{}
	}
	dir := filepath.Join(f.agents, f.implementer, "rounds", strconv.FormatInt(round, 10))
	writeJSONFile(f.t, dir, "review.json", map[string]any{"reviewedTree": tree})
	if err := osWriteFile(filepath.Join(dir, "diff.patch"), patch); err != nil {
		f.t.Fatal(err)
	}
	digest := sha256.Sum256(patch)
	return ReadSubject{
		Kind: SubjectLive, ImplementerRoot: f.implementer, ReviewedMember: job,
		ReviewedProjectTree: tree, DiffDigest: hex.EncodeToString(digest[:]),
	}
}

func osWriteFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (f *hazardClosureFixture) writeClosure(round int64, roundJob string, subject ReadSubject, status string) {
	f.t.Helper()
	if round == 2 && !fileExists(filepath.Join(f.jobs, roundJob+".json")) {
		f.writeCriticFollowUp(roundJob, round, f.critic, "resumed", "critic-session", "follow-up", nil)
	}
	rootPath := filepath.Join(f.jobs, f.critic+".json")
	root := readJSONFile(f.t, rootPath)
	root["chainClosed"] = true
	root[findingRegisterField] = []any{}
	root[findingRegisterRoundField] = round
	root[findingRegisterSubjectDigestField] = subject.Digest()
	root[closureField] = encodeClosure(Closure{CriticRoot: f.critic, Round: round, Subject: subject, Mechanism: "clean"})
	writeRecord(rootPath, root)
	if round > 1 && status != "completed" {
		path := filepath.Join(f.jobs, roundJob+".json")
		record := readJSONFile(f.t, path)
		record["status"] = status
		writeRecord(path, record)
	}
	dir := filepath.Join(f.agents, f.critic, "rounds", strconv.FormatInt(round, 10))
	writeJSONFile(f.t, dir, "subject.json", subject)
	result := map[string]any{
		"jobId": roundJob, "round": round, "reviewedTree": subject.ReviewedProjectTree,
		"findings": []any{}, "verdictMaterialCount": 0,
	}
	if subject.Kind == SubjectDesign {
		delete(result, "reviewedTree")
		result["reviewedCommit"] = subject.ReviewedCommit
	}
	writeJSONFile(f.t, dir, "return.json", result)
}

func (f *hazardClosureFixture) writeCriticFollowUp(job string, round int64, parent, resumeMode, session, adapterVerb string, overrides map[string]any) {
	f.t.Helper()
	record := map[string]any{
		"role": "code-critic", "round": round, "parentJob": parent,
		"resumeMode": resumeMode, "sessionId": session,
		"reviews": f.implementer, "endedAt": "2026-08-30T10:03:00Z",
		"configurationObligations": requiredConfigurationByHazard[HazardDesignBearing],
		"reasoningEffort":          "xhigh",
	}
	for key, value := range overrides {
		record[key] = value
	}
	writeHazardFollowUpEvidenceJob(f.t, f.repo, job, HazardDesignBearing, record, adapterVerb)
}

func writeHazardFollowUpEvidenceJob(t *testing.T, repo, job string, class HazardClass, record map[string]any, adapterVerb string) {
	t.Helper()
	jobs := filepath.Join(repo, "artifacts", "agents", "jobs")
	parentID := asString(record["parentJob"])
	parent := readJSONFile(t, filepath.Join(jobs, parentID+".json"))
	round, ok := numInt(record["round"])
	if !ok || round < 2 {
		t.Fatalf("follow-up fixture %s has invalid round %v", job, record["round"])
	}
	runtimeName := asString(record["runtime"])
	if runtimeName == "" {
		runtimeName = asString(parent["runtime"])
	}
	model := asString(record["requestedModel"])
	if model == "" {
		model = asString(parent["requestedModel"])
	}
	canonicalModel := asString(record["canonicalModelKey"])
	if canonicalModel == "" {
		canonicalModel = model
	}
	resumedSession := asString(record["resumedSessionId"])
	if _, present := record["resumedSessionId"]; !present {
		resumedSession = asString(parent["sessionId"])
	}
	digest := strings.Repeat("a", 64)
	request := CanonicalLaunchRequest{
		SessionKey: "session-" + job, DispatchMode: DispatchModeFollowUp, ResumedSessionID: resumedSession,
		Runtime: runtimeName, CanonicalModelKey: canonicalModel, Role: asString(record["role"]),
		LaunchMode: LaunchModeSharedCheckout, PermissionEnvelopeDigest: digest,
		ProductRoots: []string{repo}, CapMinutes: 30, InputHash: strings.Repeat("b", 64),
		DestructiveReach: class,
	}
	fingerprint, err := LaunchFingerprintV2(request)
	if err != nil {
		t.Fatal(err)
	}
	tag := "metasystem-job-" + job + "-evidence"
	record["jobId"] = job
	record["operationId"] = job
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
		"instanceTag": tag, "adapterVerb": adapterVerb, "status": "consumed",
		"mintedAt": "2026-08-30T09:58:00Z", "consumedAt": "2026-08-30T09:59:00Z",
		"supervisor": exactIdentityFields(nativeTestExact(7001, 3).Ref()),
	}
	writeHandwrittenHazardEvidenceJob(t, repo, job, record)
}

func (f *hazardClosureFixture) addTerminalWork(job, status, tree string, patch []byte, artifacts bool) ReadSubject {
	f.t.Helper()
	subject := f.writeImplementerRound(job, 2, f.implementer, status, tree, patch, artifacts)
	f.finalState = hazardFinalWorkState{job: job, round: 2, endedAt: mustHazardTime(f.t, "2026-08-30T10:02:00Z")}
	return subject
}

func (f *hazardClosureFixture) alignLegacyCritic(job string) {
	f.t.Helper()
	path := filepath.Join(f.jobs, f.critic+".json")
	record := readJSONFile(f.t, path)
	record["reviews"] = job
	record["endedAt"] = "2026-08-30T10:03:00Z"
	writeRecord(path, record)
}

func (f *hazardClosureFixture) validate() error {
	f.t.Helper()
	return validateIndependentCritiqueReference(
		f.repo, f.jobs, f.rootRecord, f.memberIDs, f.memberSessions,
		requiredConfigurationByHazard[HazardDesignBearing], f.finalState,
	)
}

func (f *hazardClosureFixture) requireAccepted() {
	f.t.Helper()
	if err := f.validate(); err != nil {
		f.t.Fatalf("closure-backed critique refused: %v", err)
	}
}

func (f *hazardClosureFixture) requireRefused(reason, contains string) {
	f.requireRefusedFields(reason, contains)
}

func (f *hazardClosureFixture) requireRefusedFields(reason string, contains ...string) {
	f.t.Helper()
	err := f.validate()
	wantHazardClosureRefusal(f.t, err, reason)
	for _, field := range contains {
		if field != "" && !strings.Contains(err.Error(), field) {
			f.t.Fatalf("closure refusal %q does not contain %q", err, field)
		}
	}
}

func TestHazardCloseReadsClosureSubject(t *testing.T) {
	t.Run("no-op terminal implementer uses earlier clean read", func(t *testing.T) {
		fixture := newHazardClosureFixture(t)
		fixture.addTerminalWork("implementation-r2", "completed", fixture.subject.ReviewedProjectTree, []byte("same patch\n"), true)
		fixture.requireAccepted()
	})

	t.Run("last critic follow-up binds changed terminal work", func(t *testing.T) {
		fixture := newHazardClosureFixture(t)
		terminal := fixture.addTerminalWork("implementation-r2", "completed", strings.Repeat("b", 40), []byte("changed patch\n"), true)
		fixture.writeClosure(2, "critic-r2", terminal, "completed")
		fixture.requireAccepted()
	})

	t.Run("last critic follow-up must carry its own session", func(t *testing.T) {
		fixture := newHazardClosureFixture(t)
		terminal := fixture.addTerminalWork("implementation-r2", "completed", strings.Repeat("b", 40), []byte("changed patch\n"), true)
		fixture.writeClosure(2, "critic-r2", terminal, "completed")
		path := filepath.Join(fixture.jobs, "critic-r2.json")
		proving := readJSONFile(t, path)
		delete(proving, "sessionId")
		writeRecord(path, proving)
		fixture.requireRefusedFields(hazardCritiqueClosureRefusal, fixture.critic, "critic-r2", "round 2", "sessionId")
	})

	t.Run("stamp must equal closure root", func(t *testing.T) {
		fixture := newHazardClosureFixture(t)
		path := filepath.Join(fixture.jobs, fixture.critic+".json")
		root := readJSONFile(t, path)
		root[closureField].(map[string]any)["criticRoot"] = "other-critic"
		writeRecord(path, root)
		fixture.requireRefused(hazardCritiqueStaleRefusal, "other-critic")
	})

	for _, change := range []struct {
		name  string
		tree  string
		patch []byte
	}{
		{name: "changed tree", tree: strings.Repeat("b", 40), patch: []byte("same patch\n")},
		{name: "changed diff", tree: strings.Repeat("a", 40), patch: []byte("different patch\n")},
	} {
		t.Run(change.name, func(t *testing.T) {
			fixture := newHazardClosureFixture(t)
			fixture.addTerminalWork("implementation-r2", "completed", change.tree, change.patch, true)
			fixture.alignLegacyCritic("implementation-r2")
			fixture.requireRefused(hazardCritiqueStaleRefusal, "subject")
		})
	}

	t.Run("wrong subject kind", func(t *testing.T) {
		fixture := newHazardClosureFixture(t)
		design := ReadSubject{
			Kind: SubjectDesign, DesignPath: "metasystem/plans/design.md",
			ContentDigest: "content", DeclaredOutputsDigest: "outputs", ReviewedCommit: strings.Repeat("c", 40),
		}
		fixture.writeClosure(1, fixture.critic, design, "completed")
		fixture.requireRefused(hazardCritiqueStaleRefusal, "live subject")
	})

	t.Run("independence still applies", func(t *testing.T) {
		fixture := newHazardClosureFixture(t)
		path := filepath.Join(fixture.jobs, fixture.critic+".json")
		root := readJSONFile(t, path)
		root["sessionId"] = "builder-session"
		writeRecord(path, root)
		fixture.requireRefused(hazardCritiqueClosureRefusal, "distinct fresh session")
	})

	t.Run("maximum effort still applies", func(t *testing.T) {
		fixture := newHazardClosureFixture(t)
		path := filepath.Join(fixture.jobs, fixture.critic+".json")
		root := readJSONFile(t, path)
		root["reasoningEffort"] = "medium"
		root["configurationObligations"] = requiredConfigurationByHazard[HazardMechanical]
		writeRecord(path, root)
		fixture.requireRefused(hazardCritiqueClosureRefusal, "required maximum critic effort")
	})
}

func TestHazardClosureProvingRoundContext(t *testing.T) {
	t.Run("stamped root remains fresh", func(t *testing.T) {
		fixture := newHazardClosureFixture(t)
		fixture.writeClosure(2, "critic-r2", fixture.subject, "completed")
		path := filepath.Join(fixture.jobs, fixture.critic+".json")
		root := readJSONFile(t, path)
		root["parentJob"] = "earlier-critic"
		writeRecord(path, root)
		fixture.requireRefusedFields(hazardCritiqueClosureRefusal, fixture.critic, "fresh-context chain")
	})

	for _, test := range []struct {
		name   string
		mutate func(*hazardClosureFixture, string)
		field  string
	}{
		{name: "missing proving session", field: "sessionId", mutate: func(f *hazardClosureFixture, job string) {
			path := filepath.Join(f.jobs, job+".json")
			record := readJSONFile(f.t, path)
			delete(record, "sessionId")
			writeRecord(path, record)
		}},
		{name: "proving session belongs to implementation", field: "sessionId", mutate: func(f *hazardClosureFixture, job string) {
			path := filepath.Join(f.jobs, job+".json")
			record := readJSONFile(f.t, path)
			record["sessionId"] = "builder-session"
			writeRecord(path, record)
		}},
		{name: "wrong proving role", field: "role", mutate: func(f *hazardClosureFixture, job string) {
			path := filepath.Join(f.jobs, job+".json")
			record := readJSONFile(f.t, path)
			record["role"] = "verifier"
			writeRecord(path, record)
		}},
		{name: "resumed session does not name parent session", field: "resumedSessionId", mutate: func(f *hazardClosureFixture, job string) {
			path := filepath.Join(f.jobs, job+".json")
			record := readJSONFile(f.t, path)
			record["resumedSessionId"] = "other-session"
			writeRecord(path, record)
		}},
		{name: "missing resume mode", field: "resumeMode", mutate: func(f *hazardClosureFixture, job string) {
			path := filepath.Join(f.jobs, job+".json")
			record := readJSONFile(f.t, path)
			delete(record, "resumeMode")
			writeRecord(path, record)
		}},
		{name: "unsupported resume mode", field: "resumeMode", mutate: func(f *hazardClosureFixture, job string) {
			path := filepath.Join(f.jobs, job+".json")
			record := readJSONFile(f.t, path)
			record["resumeMode"] = "other"
			writeRecord(path, record)
		}},
		{name: "resumed mode opens another session", field: "sessionId", mutate: func(f *hazardClosureFixture, job string) {
			path := filepath.Join(f.jobs, job+".json")
			record := readJSONFile(f.t, path)
			record["sessionId"] = "other-critic-session"
			writeRecord(path, record)
		}},
		{name: "fresh context reuses parent session", field: "sessionId", mutate: func(f *hazardClosureFixture, job string) {
			path := filepath.Join(f.jobs, job+".json")
			record := readJSONFile(f.t, path)
			record["resumeMode"] = "fresh-context"
			record["launchCapability"].(map[string]any)["adapterVerb"] = "dispatch"
			writeRecord(path, record)
		}},
		{name: "wrong follow-up dispatch mode", field: "dispatchMode", mutate: func(f *hazardClosureFixture, job string) {
			path := filepath.Join(f.jobs, job+".json")
			record := readJSONFile(f.t, path)
			record["dispatchMode"] = DispatchModeFresh
			writeRecord(path, record)
		}},
		{name: "runtime differs from parent", field: "runtime", mutate: func(f *hazardClosureFixture, job string) {
			path := filepath.Join(f.jobs, job+".json")
			record := readJSONFile(f.t, path)
			record["runtime"] = "other-runtime"
			writeRecord(path, record)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newHazardClosureFixture(t)
			fixture.writeClosure(2, "critic-r2", fixture.subject, "completed")
			test.mutate(fixture, "critic-r2")
			fixture.requireRefusedFields(hazardCritiqueClosureRefusal, fixture.critic, "critic-r2", "round 2", test.field)
		})
	}

	t.Run("intermediate round uses implementation session", func(t *testing.T) {
		fixture := newHazardClosureFixture(t)
		fixture.writeCriticFollowUp("critic-r2", 2, fixture.critic, "resumed", "builder-session", "follow-up", nil)
		fixture.writeCriticFollowUp("critic-r3", 3, "critic-r2", "fresh-context", "critic-r3-session", "dispatch", nil)
		fixture.writeClosure(3, "critic-r3", fixture.subject, "completed")
		fixture.requireRefusedFields(hazardCritiqueClosureRefusal, fixture.critic, "critic-r3", "round 3", "critic-r2", "sessionId")
	})

	t.Run("round three skips the preceding member", func(t *testing.T) {
		fixture := newHazardClosureFixture(t)
		fixture.writeCriticFollowUp("critic-r2", 2, fixture.critic, "resumed", "critic-session", "follow-up", nil)
		fixture.writeCriticFollowUp("critic-r3", 3, fixture.critic, "resumed", "critic-session", "follow-up", nil)
		fixture.writeClosure(3, "critic-r3", fixture.subject, "completed")
		fixture.requireRefusedFields(hazardCritiqueClosureRefusal, fixture.critic, "critic-r3", "round 3", "parentJob")
	})

	t.Run("proving record path must match its job", func(t *testing.T) {
		fixture := newHazardClosureFixture(t)
		fixture.writeClosure(2, "critic-r2", fixture.subject, "completed")
		if err := os.Rename(filepath.Join(fixture.jobs, "critic-r2.json"), filepath.Join(fixture.jobs, "critic-r2-alias.json")); err != nil {
			t.Fatal(err)
		}
		fixture.requireRefusedFields(hazardCritiqueStaleRefusal, fixture.critic, "critic-r2", "round 2", "record path")
	})

	t.Run("proving job must be outside implementation ids", func(t *testing.T) {
		fixture := newHazardClosureFixture(t)
		fixture.writeClosure(2, "critic-r2", fixture.subject, "completed")
		fixture.memberIDs["critic-r2"] = true
		fixture.requireRefusedFields(hazardCritiqueStaleRefusal, fixture.critic, "critic-r2", "round 2", "implementation chain")
	})

	t.Run("valid round three native resume chain", func(t *testing.T) {
		fixture := newHazardClosureFixture(t)
		fixture.writeCriticFollowUp("critic-r2", 2, fixture.critic, "resumed", "critic-session", "follow-up", nil)
		fixture.writeCriticFollowUp("critic-r3", 3, "critic-r2", "resumed", "critic-session", "follow-up", nil)
		fixture.writeClosure(3, "critic-r3", fixture.subject, "completed")
		fixture.requireAccepted()
	})
}

func TestHazardClosureProvingRoundAdmission(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*hazardClosureFixture, map[string]any)
		field  string
	}{
		{name: "missing admission", field: "launch admission", mutate: func(_ *hazardClosureFixture, record map[string]any) {
			delete(record, "launchCapability")
		}},
		{name: "corrupt fingerprint", field: "fingerprint", mutate: func(_ *hazardClosureFixture, record map[string]any) {
			record["fingerprint"] = strings.Repeat("d", 64)
		}},
		{name: "copied root capability", field: "launch admission", mutate: func(f *hazardClosureFixture, record map[string]any) {
			root := readJSONFile(f.t, filepath.Join(f.jobs, f.critic+".json"))
			record["launchCapability"] = root["launchCapability"]
		}},
		{name: "unconsumed capability", field: "launch admission", mutate: func(_ *hazardClosureFixture, record map[string]any) {
			record["launchCapability"].(map[string]any)["status"] = "minted"
		}},
		{name: "resumed mode with dispatch verb", field: "launch admission", mutate: func(_ *hazardClosureFixture, record map[string]any) {
			record["launchCapability"].(map[string]any)["adapterVerb"] = "dispatch"
		}},
		{name: "fresh context with follow-up verb", field: "launch admission", mutate: func(_ *hazardClosureFixture, record map[string]any) {
			record["resumeMode"] = "fresh-context"
			record["sessionId"] = "critic-r2-fresh-session"
			record["launchCapability"].(map[string]any)["adapterVerb"] = "follow-up"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newHazardClosureFixture(t)
			fixture.writeClosure(2, "critic-r2", fixture.subject, "completed")
			path := filepath.Join(fixture.jobs, "critic-r2.json")
			proving := readJSONFile(t, path)
			test.mutate(fixture, proving)
			writeRecord(path, proving)
			fixture.requireRefusedFields(hazardEvidenceProvenanceRefusal, fixture.critic, "critic-r2", "round 2", test.field)
		})
	}

	t.Run("native resume", func(t *testing.T) {
		fixture := newHazardClosureFixture(t)
		fixture.writeClosure(2, "critic-r2", fixture.subject, "completed")
		fixture.requireAccepted()
	})

	t.Run("fresh context", func(t *testing.T) {
		fixture := newHazardClosureFixture(t)
		fixture.writeCriticFollowUp("critic-r2", 2, fixture.critic, "fresh-context", "critic-r2-fresh-session", "dispatch", nil)
		fixture.writeClosure(2, "critic-r2", fixture.subject, "completed")
		fixture.requireAccepted()
	})
}

func TestHazardEvidenceAdmissionAcceptsFreshContextFollowUp(t *testing.T) {
	fixture := newHazardClosureFixture(t)
	fixture.writeCriticFollowUp("critic-r2", 2, fixture.critic, "fresh-context", "critic-r2-fresh-session", "dispatch", nil)
	proving := readJSONFile(t, filepath.Join(fixture.jobs, "critic-r2.json"))
	if detail := validateHazardEvidenceAdmissionProvenance(proving, "critic-r2"); detail != "" {
		t.Fatalf("fresh-context follow-up admission refused: %s", detail)
	}
}

func TestHazardClosureProvingRoundEffort(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
		field  string
	}{
		{name: "missing builder effort tier", field: "configurationObligations.builderEffortTier", mutate: func(record map[string]any) {
			delete(record["configurationObligations"].(map[string]any), "builderEffortTier")
		}},
		{name: "weak builder effort tier", field: "configurationObligations.builderEffortTier", mutate: func(record map[string]any) {
			record["configurationObligations"].(map[string]any)["builderEffortTier"] = "ordinary"
		}},
		{name: "missing builder reasoning effort", field: "configurationObligations.builderReasoningEffort", mutate: func(record map[string]any) {
			delete(record["configurationObligations"].(map[string]any), "builderReasoningEffort")
		}},
		{name: "weak builder reasoning effort", field: "configurationObligations.builderReasoningEffort", mutate: func(record map[string]any) {
			record["configurationObligations"].(map[string]any)["builderReasoningEffort"] = "medium"
		}},
		{name: "missing reasoning effort", field: "reasoningEffort", mutate: func(record map[string]any) {
			delete(record, "reasoningEffort")
		}},
		{name: "weak reasoning effort", field: "reasoningEffort", mutate: func(record map[string]any) {
			record["reasoningEffort"] = "medium"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newHazardClosureFixture(t)
			fixture.writeClosure(2, "critic-r2", fixture.subject, "completed")
			path := filepath.Join(fixture.jobs, "critic-r2.json")
			proving := readJSONFile(t, path)
			test.mutate(proving)
			writeRecord(path, proving)
			fixture.requireRefusedFields(hazardCritiqueClosureRefusal, fixture.critic, "critic-r2", "round 2", test.field)
		})
	}

	t.Run("missing requested model", func(t *testing.T) {
		fixture := newHazardClosureFixture(t)
		fixture.writeClosure(2, "critic-r2", fixture.subject, "completed")
		path := filepath.Join(fixture.jobs, "critic-r2.json")
		proving := readJSONFile(t, path)
		delete(proving, "requestedModel")
		writeRecord(path, proving)
		fixture.requireRefusedFields(hazardCritiqueClosureRefusal, fixture.critic, "critic-r2", "round 2", "runtime/requestedModel")
	})

	t.Run("runtime mapping excludes proving model", func(t *testing.T) {
		fixture := newHazardClosureFixture(t)
		if err := os.WriteFile(filepath.Join(fixture.repo, "metasystem.conf"), []byte("runtime.claude.maximal-models=root-maximal\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		rootPath := filepath.Join(fixture.jobs, fixture.critic+".json")
		root := readJSONFile(t, rootPath)
		root["runtime"] = "claude"
		root["requestedModel"] = "root-maximal"
		stampHazardEvidenceAdmission(t, fixture.repo, fixture.critic, HazardDesignBearing, root)
		writeHandwrittenHazardEvidenceJob(t, fixture.repo, fixture.critic, root)
		fixture.writeCriticFollowUp("critic-r2", 2, fixture.critic, "resumed", "critic-session", "follow-up", map[string]any{
			"requestedModel": "proving-not-maximal", "canonicalModelKey": "proving-not-maximal",
		})
		fixture.writeClosure(2, "critic-r2", fixture.subject, "completed")
		fixture.requireRefusedFields(hazardCritiqueClosureRefusal, fixture.critic, "critic-r2", "round 2", "runtime/requestedModel")
	})

	t.Run("fully proven effort", func(t *testing.T) {
		fixture := newHazardClosureFixture(t)
		fixture.writeClosure(2, "critic-r2", fixture.subject, "completed")
		fixture.requireAccepted()
	})
}

func TestCloseCheckRejectsUnprovenClosureRound(t *testing.T) {
	t.Run("proving round needs its own admission", func(t *testing.T) {
		repo, evidence, job := closeReadyHazardChain(t, HazardDesignBearing)
		agents := filepath.Join(repo, "artifacts", "agents")
		jobs := filepath.Join(agents, "jobs")
		tree := strings.Repeat("a", 40)
		writeJSONFile(t, filepath.Join(agents, job, "rounds", "1"), "review.json", map[string]any{"reviewedTree": tree})
		patch, err := os.ReadFile(filepath.Join(agents, job, "rounds", "1", "diff.patch"))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(patch)
		subject := ReadSubject{
			Kind: SubjectLive, ImplementerRoot: job, ReviewedMember: job,
			ReviewedProjectTree: tree, DiffDigest: hex.EncodeToString(digest[:]),
		}
		fixture := &hazardClosureFixture{
			t: t, repo: repo, agents: agents, jobs: jobs, implementer: job, critic: "critic-close",
			rootRecord: map[string]any{"independentCritiqueJobRef": "critic-close"},
			memberIDs:  map[string]bool{job: true}, memberSessions: map[string]bool{"builder-session": true},
			finalState: hazardFinalWorkState{job: job, round: 1, endedAt: mustHazardTime(t, "2026-08-30T10:00:00Z")},
			subject:    subject,
		}
		writeHazardEvidenceJob(t, repo, fixture.critic, HazardDesignBearing, map[string]any{
			"role": "code-critic", "reviews": job,
			"sessionId": "critic-session", "reasoningEffort": "xhigh",
			"configurationObligations": requiredConfigurationByHazard[HazardDesignBearing],
		})
		fixture.writeClosure(2, "critic-close-r2", subject, "completed")
		rootPath := filepath.Join(jobs, job+".json")
		root := readJSONFile(t, rootPath)
		root["independentCritiqueJobRef"] = fixture.critic
		writeRecord(rootPath, root)
		refreshHazardMirror(t, repo, evidence, job)

		provingPath := filepath.Join(jobs, "critic-close-r2.json")
		proving := readJSONFile(t, provingPath)
		admission := proving["launchCapability"]
		delete(proving, "launchCapability")
		writeRecord(provingPath, proving)
		err = CloseCheck(repo, job)
		wantHazardClosureRefusal(t, err, hazardEvidenceProvenanceRefusal)
		for _, field := range []string{fixture.critic, "critic-close-r2", "round 2", "launch admission"} {
			if !strings.Contains(err.Error(), field) {
				t.Fatalf("CloseCheck refusal %q does not contain %q", err, field)
			}
		}

		proving = readJSONFile(t, provingPath)
		proving["launchCapability"] = admission
		writeRecord(provingPath, proving)
		if err := CloseCheck(repo, job); err != nil {
			t.Fatalf("CloseCheck refused a fully proven closure round: %v", err)
		}
	})

	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
		field  string
	}{
		{name: "absent closure retains exact root review", field: "reviews", mutate: func(root map[string]any) {
			root["reviews"] = "other-work"
		}},
		{name: "absent closure retains root end time", field: "did not end", mutate: func(root map[string]any) {
			root["endedAt"] = "2026-08-30T09:59:00Z"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newHazardClosureFixture(t)
			path := filepath.Join(fixture.jobs, fixture.critic+".json")
			root := readJSONFile(t, path)
			delete(root, closureField)
			delete(root, "chainClosed")
			delete(root, findingRegisterField)
			delete(root, findingRegisterRoundField)
			delete(root, findingRegisterSubjectDigestField)
			test.mutate(root)
			writeRecord(path, root)
			fixture.requireRefusedFields(hazardCritiqueStaleRefusal, fixture.critic, test.field)
		})
	}
}

func TestHazardCloseUsesTerminalWorkSubject(t *testing.T) {
	for _, status := range []string{"completed", "cancelled"} {
		t.Run(status+" terminal without artifacts", func(t *testing.T) {
			fixture := newHazardClosureFixture(t)
			fixture.addTerminalWork("implementation-r2", status, strings.Repeat("a", 40), nil, false)
			fixture.alignLegacyCritic("implementation-r2")
			fixture.requireRefused(hazardCritiqueStaleRefusal, "no readable live subject")
		})
	}

	t.Run("invalid work round", func(t *testing.T) {
		_, detail := finalHazardWorkState([]chainMember{{record: map[string]any{
			"jobId": "implementation", "role": "implementer", "round": 0,
		}}})
		if !strings.Contains(detail, "positive round") {
			t.Fatalf("invalid round detail = %q", detail)
		}
	})

	t.Run("tied work round", func(t *testing.T) {
		endedAt := "2026-08-30T10:00:00Z"
		_, detail := finalHazardWorkState([]chainMember{
			{record: map[string]any{"jobId": "implementation-a", "role": "implementer", "round": 2, "endedAt": endedAt}},
			{record: map[string]any{"jobId": "implementation-b", "role": "implementer", "round": 2, "endedAt": endedAt}},
		})
		if !strings.Contains(detail, "more than one terminal work record at round 2") {
			t.Fatalf("tied round detail = %q", detail)
		}
	})

	t.Run("running member is rejected before hazard evidence", func(t *testing.T) {
		repo, _, root := closeReadyHazardChain(t, HazardDesignBearing)
		writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), root+"-r2.json", map[string]any{
			"jobId": root + "-r2", "role": "implementer", "round": 2,
			"parentJob": root, "status": "running",
		})
		if err := CloseCheck(repo, root); err == nil || !strings.Contains(err.Error(), "non-terminal record") {
			t.Fatalf("running work member close = %v", err)
		}
	})
}

func TestHazardCloseRejectsStaleCriticClosure(t *testing.T) {
	for _, status := range []string{"running", "completed", "failed", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			fixture := newHazardClosureFixture(t)
			writeJSONFile(t, fixture.jobs, "critic-r2.json", map[string]any{
				"jobId": "critic-r2", "role": "code-critic", "round": 2,
				"parentJob": fixture.critic, "status": status,
			})
			fixture.requireRefused(hazardCritiqueStaleRefusal, "round")
		})

	}
}
