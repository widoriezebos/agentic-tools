package dispatch

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// wantRun asserts a refusal's line 2: the command that resolves it or shows
// what it is about, with the job it concerns filled in.
func wantRun(t *testing.T, err error, run string) {
	t.Helper()
	var op *OpError
	if !errors.As(err, &op) {
		t.Fatalf("expected an OpError, got %v", err)
	}
	if op.Run != run || !strings.HasSuffix(op.Error(), "\nrun: "+run) {
		t.Fatalf("run = %q, text %q; want %q", op.Run, op.Error(), run)
	}
}

// A record operation's refusal names the job's status as its line 2.
func TestRecordRefusalsNameTheJobStatus(t *testing.T) {
	root := sandbox(t)
	createPending(t, root, "job-a")
	setupPending(t, root, "job-a")
	patch := writeJSON(t, filepath.Join(t.TempDir(), "p.json"), map[string]any{"note": "x"})
	_, err := RecordCAS(root, "job-a", "pending", "completed", patch)
	wantRun(t, err, "metasystem work status j2:job-a")

	source := writeJSON(t, filepath.Join(t.TempDir(), "again.json"), map[string]any{"jobId": "job-a", "status": "pending-setup"})
	wantRun(t, RecordCreate(root, "job-a", source), "metasystem work status j2:job-a")
}

// A chain that cannot close for want of an independent critique names the
// review of its final work round.
func TestHazardClosureRefusalNamesTheReview(t *testing.T) {
	fixture := newHazardClosureFixture(t)
	fixture.rootRecord = map[string]any{}
	err := validateIndependentCritiqueReference(fixture.repo, fixture.jobs, fixture.rootRecord, fixture.memberIDs, fixture.memberSessions,
		requiredConfigurationByHazard[HazardDesignBearing], fixture.finalState)
	wantRun(t, err, "metasystem work review j2:implementation")
}

// A redundant read names what closes the clean chain.
func TestRedundantReadNamesTheFinish(t *testing.T) {
	err := redundantReadError(ReadAdmissionResult{SubjectDigest: "d"}, cleanReadCandidate{root: "crit-1", closeable: true}, "")
	wantRun(t, err, "metasystem work finish j2:crit-1")
	err = redundantReadError(ReadAdmissionResult{SubjectDigest: "d"}, cleanReadCandidate{root: "crit-1"}, "")
	wantRun(t, err, "metasystem work status j2:crit-1")
}
