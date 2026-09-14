package dispatch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type reviewReferenceClosureFixture struct {
	t               *testing.T
	repo            string
	evidence        string
	agents          string
	jobs            string
	workRoot        string
	workTerminal    string
	criticRoot      string
	criticTerminal  string
	role            string
	terminalSubject ReadSubject
}

func newReviewReferenceClosureFixture(t *testing.T, role string, rootReviewsTerminal, freshContext bool) *reviewReferenceClosureFixture {
	t.Helper()
	f := &reviewReferenceClosureFixture{
		t: t, repo: t.TempDir(), evidence: t.TempDir(),
		workRoot: "work-r4", workTerminal: "work-r6",
		criticRoot: "critic-read", criticTerminal: "critic-read-r3", role: role,
	}
	f.agents = filepath.Join(f.repo, "artifacts", "agents")
	f.jobs = filepath.Join(f.agents, "jobs")
	snapshot := "artifacts/agents/capabilities/review-reference.json"
	writeJSONFile(t, filepath.Join(f.agents, "capabilities"), "review-reference.json", map[string]any{"fixture": true})
	if err := os.MkdirAll(filepath.Join(f.agents, f.workRoot, "rounds"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.agents, f.workRoot, "brief.md"), []byte("implementation brief\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	work := []struct {
		job, parent, ended, tree string
		round                    int64
		patch                    []byte
	}{
		{job: f.workRoot, round: 4, ended: "2026-09-14T10:04:00Z", tree: strings.Repeat("a", 40), patch: []byte("round four patch\n")},
		{job: "work-r5", parent: f.workRoot, round: 5, ended: "2026-09-14T10:05:00Z", tree: strings.Repeat("b", 40), patch: []byte("terminal patch\n")},
		{job: f.workTerminal, parent: "work-r5", round: 6, ended: "2026-09-14T10:06:00Z", tree: strings.Repeat("b", 40), patch: []byte("terminal patch\n")},
	}
	workSubjects := make([]ReadSubject, 0, len(work))
	for _, member := range work {
		parent := any(nil)
		if member.parent != "" {
			parent = member.parent
		}
		record := map[string]any{
			"jobId": member.job, "role": "implementer", "round": member.round,
			"parentJob": parent, "status": "completed", "sessionId": "builder-session",
			"endedAt": member.ended, "capabilitySnapshot": snapshot,
		}
		if member.job == f.workRoot {
			record["mirror"] = nil
			record["destructiveReach"] = HazardDesignBearing
			record["goalTier"] = nil
			record["configurationObligations"] = requiredConfigurationByHazard[HazardDesignBearing]
			record["dispatchMode"] = DispatchModeFresh
			record["resumedSessionId"] = nil
		}
		writeJSONFile(t, f.jobs, member.job+".json", record)
		roundDir := filepath.Join(f.agents, f.workRoot, "rounds", strconv.FormatInt(member.round, 10))
		writeJSONFile(t, roundDir, "review.json", map[string]any{"reviewedTree": member.tree})
		if err := osWriteFile(filepath.Join(roundDir, "diff.patch"), member.patch); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(member.patch)
		workSubjects = append(workSubjects, ReadSubject{
			Kind: SubjectLive, ImplementerRoot: f.workRoot, ReviewedMember: member.job,
			ReviewedProjectTree: member.tree, DiffDigest: hex.EncodeToString(digest[:]),
		})
	}
	if !workSubjects[1].Equal(workSubjects[2]) || workSubjects[1].ReviewedMember == workSubjects[2].ReviewedMember {
		t.Fatal("fixture terminal round is not a no-op member with distinct provenance")
	}
	f.terminalSubject = workSubjects[2]

	reviews := f.workRoot
	if rootReviewsTerminal {
		reviews = f.workTerminal
	}
	writeHazardEvidenceJob(t, f.repo, f.criticRoot, HazardDesignBearing, map[string]any{
		"role": role, "reviews": reviews, "sessionId": "critic-session",
		"reasoningEffort": "xhigh", "endedAt": "2026-09-14T10:03:00Z",
		"configurationObligations": requiredConfigurationByHazard[HazardDesignBearing],
	})
	writeHazardFollowUpEvidenceJob(t, f.repo, "critic-read-r2", HazardDesignBearing, map[string]any{
		"role": role, "round": 2, "parentJob": f.criticRoot, "reviews": reviews,
		"resumeMode": "resumed", "sessionId": "critic-session",
		"reasoningEffort": "xhigh", "endedAt": "2026-09-14T10:05:30Z",
		"configurationObligations": requiredConfigurationByHazard[HazardDesignBearing],
	}, "follow-up")
	lastResumeMode, lastSession, lastVerb := "resumed", "critic-session", "follow-up"
	if freshContext {
		lastResumeMode, lastSession, lastVerb = "fresh-context", "critic-fresh-session", "dispatch"
	}
	writeHazardFollowUpEvidenceJob(t, f.repo, f.criticTerminal, HazardDesignBearing, map[string]any{
		"role": role, "round": 3, "parentJob": "critic-read-r2", "reviews": reviews,
		"resumeMode": lastResumeMode, "sessionId": lastSession,
		"reasoningEffort": "xhigh", "endedAt": "2026-09-14T10:07:00Z",
		"configurationObligations": requiredConfigurationByHazard[HazardDesignBearing],
	}, lastVerb)
	for index, subject := range workSubjects {
		writeSubjectReturn(t, f.repo, f.criticRoot, []string{f.criticRoot, "critic-read-r2", f.criticTerminal}[index], int64(index+1), subject)
	}
	rootPath := filepath.Join(f.jobs, f.criticRoot+".json")
	root := readJSONFile(t, rootPath)
	root["chainClosed"] = true
	root[findingRegisterField] = []any{}
	root[findingRegisterRoundField] = int64(3)
	root[findingRegisterSubjectDigestField] = f.terminalSubject.Digest()
	root[closureField] = encodeClosure(Closure{
		CriticRoot: f.criticRoot, Round: 3, Subject: f.terminalSubject, Mechanism: "clean",
	})
	if err := writeRecord(rootPath, root); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *reviewReferenceClosureFixture) rootRecord() map[string]any {
	f.t.Helper()
	return readJSONFile(f.t, filepath.Join(f.jobs, f.criticRoot+".json"))
}

func (f *reviewReferenceClosureFixture) writeRoot(record map[string]any) {
	f.t.Helper()
	if err := writeRecord(filepath.Join(f.jobs, f.criticRoot+".json"), record); err != nil {
		f.t.Fatal(err)
	}
}

func (f *reviewReferenceClosureFixture) setStamp(value string) {
	f.t.Helper()
	path := filepath.Join(f.jobs, f.workRoot+".json")
	record := readJSONFile(f.t, path)
	if value == "" {
		delete(record, independentCritiqueReferenceField)
	} else {
		record[independentCritiqueReferenceField] = value
	}
	if err := writeRecord(path, record); err != nil {
		f.t.Fatal(err)
	}
}

func (f *reviewReferenceClosureFixture) mirrorImplementation() {
	f.t.Helper()
	result := filepath.Join(f.t.TempDir(), "mirror.json")
	for _, job := range []string{f.workRoot, "work-r5", f.workTerminal} {
		if err := Mirror(f.repo, f.repo, f.evidence, f.workRoot, job, result); err != nil {
			f.t.Fatal(err)
		}
	}
	mirrored := readJSONFile(f.t, result)
	path := filepath.Join(f.jobs, f.workRoot+".json")
	record := readJSONFile(f.t, path)
	record["mirror"] = map[string]any{"path": asString(mirrored["path"]), "manifest": mirrored["manifest"]}
	if err := writeRecord(path, record); err != nil {
		f.t.Fatal(err)
	}
	for _, job := range []string{f.workRoot, "work-r5", f.workTerminal} {
		if err := Mirror(f.repo, f.repo, f.evidence, f.workRoot, job, result); err != nil {
			f.t.Fatal(err)
		}
	}
}

func (f *reviewReferenceClosureFixture) criticEvidence() map[string][]byte {
	f.t.Helper()
	paths := []string{
		filepath.Join(f.jobs, f.criticRoot+".json"),
		filepath.Join(f.jobs, "critic-read-r2.json"),
		filepath.Join(f.jobs, f.criticTerminal+".json"),
	}
	if err := filepath.Walk(filepath.Join(f.agents, f.criticRoot), func(path string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			paths = append(paths, path)
		}
		return err
	}); err != nil {
		f.t.Fatal(err)
	}
	sort.Strings(paths)
	snapshot := make(map[string][]byte, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			f.t.Fatal(err)
		}
		rel, err := filepath.Rel(f.repo, path)
		if err != nil {
			f.t.Fatal(err)
		}
		snapshot[rel] = data
	}
	return snapshot
}

func (f *reviewReferenceClosureFixture) assertCriticEvidence(snapshot map[string][]byte) {
	f.t.Helper()
	after := f.criticEvidence()
	if len(after) != len(snapshot) {
		f.t.Fatalf("critic evidence file count changed: before=%d after=%d", len(snapshot), len(after))
	}
	for path, before := range snapshot {
		if !bytes.Equal(before, after[path]) {
			f.t.Fatalf("reconciliation changed immutable critic evidence %s", path)
		}
	}
	for _, job := range []string{f.criticRoot, "critic-read-r2", f.criticTerminal} {
		if got := asString(readJSONFile(f.t, filepath.Join(f.jobs, job+".json"))["reviews"]); got != asString(f.rootRecord()["reviews"]) {
			f.t.Fatalf("critic member %s reviews changed to %q", job, got)
		}
	}
}

func TestReconcileReviewReferenceAcceptsClosedFollowUpSubject(t *testing.T) {
	for _, role := range []string{"code-critic", "warden"} {
		for _, evidence := range []string{"root", "round-three"} {
			for _, prior := range []string{"missing", "incorrect"} {
				t.Run(strings.Join([]string{role, evidence, prior}, "/"), func(t *testing.T) {
					fixture := newReviewReferenceClosureFixture(t, role, false, false)
					if prior == "incorrect" {
						fixture.setStamp("older-critic")
					}
					before := fixture.criticEvidence()
					evidenceJob := fixture.criticRoot
					if evidence == "round-three" {
						evidenceJob = fixture.criticTerminal
					}
					if err := ReconcileReviewReference(fixture.repo, fixture.workRoot, evidenceJob); err != nil {
						t.Fatalf("closed follow-up reconciliation: %v", err)
					}
					if got := asString(readJSONFile(t, filepath.Join(fixture.jobs, fixture.workRoot+".json"))[independentCritiqueReferenceField]); got != fixture.criticRoot {
						t.Fatalf("reconciled stamp = %q, want canonical root %q", got, fixture.criticRoot)
					}
					if err := ReconcileReviewReference(fixture.repo, fixture.workRoot, evidenceJob); err != nil {
						t.Fatalf("repeated reconciliation: %v", err)
					}
					fixture.assertCriticEvidence(before)
					fixture.mirrorImplementation()
					if err := CloseCheck(fixture.repo, fixture.workRoot); err != nil {
						t.Fatalf("close check after reconciliation: %v", err)
					}
				})
			}
		}
	}
}

func TestReconcileReviewReferenceRejectsInvalidClosure(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*reviewReferenceClosureFixture)
	}{
		{name: "wrong kind", mutate: func(f *reviewReferenceClosureFixture) {
			subject := ReadSubject{Kind: SubjectDesign, DesignPath: "metasystem/plans/design.md", ContentDigest: "content", DeclaredOutputsDigest: "outputs", ReviewedCommit: strings.Repeat("c", 40)}
			f.replaceClosureSubject(subject)
		}},
		{name: "wrong root", mutate: func(f *reviewReferenceClosureFixture) {
			root := f.rootRecord()
			root[closureField].(map[string]any)["criticRoot"] = "other-critic"
			f.writeRoot(root)
		}},
		{name: "wrong tree", mutate: func(f *reviewReferenceClosureFixture) {
			subject := f.terminalSubject
			subject.ReviewedProjectTree = strings.Repeat("c", 40)
			f.replaceClosureSubject(subject)
		}},
		{name: "wrong diff", mutate: func(f *reviewReferenceClosureFixture) {
			subject := f.terminalSubject
			subject.DiffDigest = strings.Repeat("d", 64)
			f.replaceClosureSubject(subject)
		}},
		{name: "unclosed root", mutate: func(f *reviewReferenceClosureFixture) {
			root := f.rootRecord()
			root["chainClosed"] = false
			f.writeRoot(root)
		}},
		{name: "stale critic round", mutate: func(f *reviewReferenceClosureFixture) {
			writeJSONFile(f.t, f.jobs, "critic-read-r4.json", map[string]any{
				"jobId": "critic-read-r4", "role": f.role, "round": 4,
				"parentJob": f.criticTerminal, "status": "completed", "reviews": f.workTerminal,
			})
		}},
		{name: "tied critic rounds", mutate: func(f *reviewReferenceClosureFixture) {
			writeJSONFile(f.t, f.jobs, "critic-read-other.json", map[string]any{
				"jobId": "critic-read-other", "role": f.role, "round": 3,
				"parentJob": "critic-read-r2", "status": "completed", "reviews": f.workTerminal,
			})
		}},
		{name: "unbound return", mutate: func(f *reviewReferenceClosureFixture) {
			path := filepath.Join(f.agents, f.criticRoot, "rounds", "3", "return.json")
			result := readJSONFile(f.t, path)
			result["reviewedTree"] = strings.Repeat("c", 40)
			if err := writeRecord(path, result); err != nil {
				f.t.Fatal(err)
			}
		}},
		{name: "missing persisted subject", mutate: func(f *reviewReferenceClosureFixture) {
			if err := os.Remove(filepath.Join(f.agents, f.criticRoot, "rounds", "3", "subject.json")); err != nil {
				f.t.Fatal(err)
			}
		}},
		{name: "missing terminal review", mutate: func(f *reviewReferenceClosureFixture) {
			if err := os.Remove(filepath.Join(f.agents, f.workRoot, "rounds", "6", "review.json")); err != nil {
				f.t.Fatal(err)
			}
		}},
		{name: "missing terminal patch", mutate: func(f *reviewReferenceClosureFixture) {
			if err := os.Remove(filepath.Join(f.agents, f.workRoot, "rounds", "6", "diff.patch")); err != nil {
				f.t.Fatal(err)
			}
		}},
		{name: "missing modern clean closure", mutate: func(f *reviewReferenceClosureFixture) {
			root := f.rootRecord()
			delete(root, closureField)
			f.writeRoot(root)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newReviewReferenceClosureFixture(t, "code-critic", true, false)
			fixture.setStamp("sentinel-critic")
			test.mutate(fixture)
			err := ReconcileReviewReference(fixture.repo, fixture.workRoot, fixture.criticTerminal)
			if err == nil {
				t.Fatal("invalid closure reconciled")
			}
			for _, detail := range []string{fixture.criticTerminal, fixture.criticRoot, "round 3"} {
				if !strings.Contains(err.Error(), detail) {
					t.Fatalf("refusal %q does not name %q", err, detail)
				}
			}
			if got := asString(readJSONFile(t, filepath.Join(fixture.jobs, fixture.workRoot+".json"))[independentCritiqueReferenceField]); got != "sentinel-critic" {
				t.Fatalf("failed reconciliation changed sentinel stamp to %q", got)
			}
		})
	}
}

func (f *reviewReferenceClosureFixture) replaceClosureSubject(subject ReadSubject) {
	f.t.Helper()
	root := f.rootRecord()
	root[closureField] = encodeClosure(Closure{CriticRoot: f.criticRoot, Round: 3, Subject: subject, Mechanism: "clean"})
	root[findingRegisterSubjectDigestField] = subject.Digest()
	f.writeRoot(root)
	writeSubjectReturn(f.t, f.repo, f.criticRoot, f.criticTerminal, 3, subject)
}

func TestReconcileReviewReferenceKeepsVerifierTerminalBinding(t *testing.T) {
	fixture := newReviewReferenceClosureFixture(t, "code-critic", false, false)
	writeJSONFile(t, fixture.jobs, "stale-verifier.json", map[string]any{
		"jobId": "stale-verifier", "role": "verifier", "round": 1, "parentJob": nil,
		"status": "completed", "reviews": "work-r5",
	})
	if err := ReconcileReviewReference(fixture.repo, fixture.workRoot, "stale-verifier"); err == nil || !strings.Contains(err.Error(), "terminal work round") {
		t.Fatalf("stale verifier reconciliation = %v", err)
	}
	if got := asString(readJSONFile(t, filepath.Join(fixture.jobs, fixture.workRoot+".json"))[liveProofReferenceField]); got != "" {
		t.Fatalf("stale verifier stamped %q", got)
	}
	writeJSONFile(t, fixture.jobs, "terminal-verifier.json", map[string]any{
		"jobId": "terminal-verifier", "role": "verifier", "round": 1, "parentJob": nil,
		"status": "completed", "reviews": fixture.workTerminal,
	})
	if err := ReconcileReviewReference(fixture.repo, fixture.workRoot, "terminal-verifier"); err != nil {
		t.Fatalf("terminal verifier reconciliation: %v", err)
	}
}

func TestReconcileReviewReferenceKeepsLawfulNonCleanAbsence(t *testing.T) {
	for _, disposition := range []struct{ status, resolution string }{
		{status: "accepted-risk", resolution: "accepted-risk"},
		{status: "deferred", resolution: "deferred"},
		{status: "resolved", resolution: "out-of-scope"},
	} {
		t.Run(disposition.status, func(t *testing.T) {
			fixture := newReviewReferenceClosureFixture(t, "code-critic", true, false)
			root := fixture.rootRecord()
			delete(root, closureField)
			delete(root, findingRegisterSubjectDigestField)
			root[findingRegisterField] = encodeFindingRegister([]registerFinding{{
				FindingID: "F-1", Critic: fixture.criticTerminal, RigorClass: "bounded",
				FactsDigest: digestJSON(nil), Status: disposition.status, Resolution: disposition.resolution,
				DecisionOpID: "decision-op", EvidenceDigest: digestJSON(nil), Multiplicity: 1,
			}})
			fixture.writeRoot(root)
			if err := ReconcileReviewReference(fixture.repo, fixture.workRoot, fixture.criticTerminal); err != nil {
				t.Fatalf("lawful %s absence: %v", disposition.status, err)
			}
			if got := asString(readJSONFile(t, filepath.Join(fixture.jobs, fixture.workRoot+".json"))[independentCritiqueReferenceField]); got != fixture.criticTerminal {
				t.Fatalf("lawful %s absence stamped %q, want supplied member %q", disposition.status, got, fixture.criticTerminal)
			}
		})
	}
}

func TestReconciledFollowUpClosePreservesHazardProof(t *testing.T) {
	for _, mode := range []struct {
		name         string
		freshContext bool
	}{
		{name: "native resume"},
		{name: "fresh-context fallback", freshContext: true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			fixture := newReviewReferenceClosureFixture(t, "code-critic", false, mode.freshContext)
			if err := ReconcileReviewReference(fixture.repo, fixture.workRoot, fixture.criticTerminal); err != nil {
				t.Fatal(err)
			}
			fixture.mirrorImplementation()
			if err := CloseCheck(fixture.repo, fixture.workRoot); err != nil {
				t.Fatalf("valid reconciled close: %v", err)
			}
		})
	}

	tests := []struct {
		name, field string
		mutate      func(*reviewReferenceClosureFixture)
	}{
		{name: "missing proving admission", field: "launch admission", mutate: func(f *reviewReferenceClosureFixture) {
			path := filepath.Join(f.jobs, f.criticTerminal+".json")
			record := readJSONFile(f.t, path)
			delete(record, "launchCapability")
			_ = writeRecord(path, record)
		}},
		{name: "weaker proving effort", field: "reasoningEffort", mutate: func(f *reviewReferenceClosureFixture) {
			path := filepath.Join(f.jobs, f.criticTerminal+".json")
			record := readJSONFile(f.t, path)
			record["reasoningEffort"] = "medium"
			_ = writeRecord(path, record)
		}},
		{name: "implementation session in ancestry", field: "sessionId", mutate: func(f *reviewReferenceClosureFixture) {
			path := filepath.Join(f.jobs, "critic-read-r2.json")
			record := readJSONFile(f.t, path)
			record["sessionId"] = "builder-session"
			_ = writeRecord(path, record)
		}},
		{name: "broken parent join", field: "parentJob", mutate: func(f *reviewReferenceClosureFixture) {
			path := filepath.Join(f.jobs, f.criticTerminal+".json")
			record := readJSONFile(f.t, path)
			record["parentJob"] = f.criticRoot
			_ = writeRecord(path, record)
		}},
		{name: "broken session join", field: "resumedSessionId", mutate: func(f *reviewReferenceClosureFixture) {
			path := filepath.Join(f.jobs, f.criticTerminal+".json")
			record := readJSONFile(f.t, path)
			record["resumedSessionId"] = "unrelated-session"
			_ = writeRecord(path, record)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newReviewReferenceClosureFixture(t, "code-critic", false, false)
			if err := ReconcileReviewReference(fixture.repo, fixture.workRoot, fixture.criticTerminal); err != nil {
				t.Fatal(err)
			}
			test.mutate(fixture)
			err := CloseCheck(fixture.repo, fixture.workRoot)
			if err == nil || !strings.Contains(err.Error(), fixture.criticTerminal) || !strings.Contains(err.Error(), test.field) {
				t.Fatalf("hazard proof mutation %s = %v", test.name, err)
			}
		})
	}
}
