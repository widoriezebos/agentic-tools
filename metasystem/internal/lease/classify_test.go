package lease

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

// childOf spawns a child whose parent is this test process, so classifying the
// child walks up through us — letting us stage what the ancestor resolves to.
func childOf(t *testing.T) int64 {
	t.Helper()
	cmd := exec.Command("/bin/sleep", "120")
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn child: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	return int64(cmd.Process.Pid)
}

func selfStart(t *testing.T) int64 {
	t.Helper()
	s, ok := StartedAt(int64(os.Getpid()), nil)
	if !ok {
		t.Fatal("could not read our own start")
	}
	return s
}

func writeJSON(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestClassifyHumanWhenNoAncestorRecognised(t *testing.T) {
	root := t.TempDir()
	got, err := Classify(root, childOf(t))
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != ClassHuman {
		t.Fatalf("want HUMAN, got %+v", got)
	}
}

func TestClassifyMainThroughAnnouncedAncestor(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	if _, err := Announce(root, "sess", self, selfStart(t), "tag", "fake", ""); err != nil {
		t.Fatalf("announce self: %v", err)
	}
	got, err := Classify(root, childOf(t))
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != ClassMain {
		t.Fatalf("a child of an announced main is that main's work; got %+v", got)
	}
}

func TestClassifySupervisionThroughAncestor(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	writeJSON(t, filepath.Join(root, "artifacts/agents/supervision/state.json"),
		fmt.Sprintf(`{"owner":{"pid":%d,"pidStartedAt":%d,"instanceTag":"o"},"components":{}}`, self, selfStart(t)))
	got, err := Classify(root, childOf(t))
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != ClassSupervision || got.Pid != self {
		t.Fatalf("want SUPERVISION at our pid, got %+v", got)
	}
}

func TestClassifyAdapterSupervisorThroughAncestor(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	writeJSON(t, filepath.Join(root, "artifacts/agents/jobs/job-1.json"),
		fmt.Sprintf(`{"jobId":"job-1","pid":%d,"pidStartedAt":%d}`, self, selfStart(t)))
	got, err := Classify(root, childOf(t))
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != ClassAdapterSupervisor || got.JobId != "job-1" {
		t.Fatalf("want ADAPTER-SUPERVISOR of job-1, got %+v", got)
	}
}

func TestClassifyDelegateThroughSignedAncestor(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	command, ok := ProcessCommand(self, nil)
	if !ok {
		t.Skip("cannot read our own command to build a matching signature")
	}
	// An adapter whose signature matches our exact command, so the child's
	// ancestor (us) classifies as a delegate of that runtime.
	adapterDir := filepath.Join(root, "scripts/agents/adapters")
	if err := os.MkdirAll(adapterDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// printf the pattern as an argument, not a format string, so QuoteMeta's
	// backslashes reach the signature registry intact.
	line := "match " + regexp.QuoteMeta(command)
	script := "#!/bin/sh\n[ \"$1\" = signature ] && printf '%s\\n' '" + line + "'\n"
	if err := os.WriteFile(filepath.Join(adapterDir, "fake.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Classify(root, childOf(t))
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != ClassDelegate || got.Pid != self {
		t.Fatalf("want DELEGATE at our pid, got %+v", got)
	}
}

func TestReadAnnouncementsSkipsIncompleteButRefusesTampered(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "artifacts/agents/mains")
	// An older record without identity fields: skipped, not an error.
	writeJSON(t, filepath.Join(dir, "old.json"),
		`{"sessionId":"s","pid":1,"pidStartedAt":1,"pgid":1,"runtime":"fake","instanceTag":"t","announcedAt":"a"}`)
	if recs, err := readAnnouncements(root, true); err != nil || len(recs) != 0 {
		t.Fatalf("an identity-less announcement should be skipped, not refuse: %v %d", err, len(recs))
	}
	// A tampering-shaped record (bad mainId) refuses a strict read.
	writeJSON(t, filepath.Join(dir, "bad.json"),
		`{"sessionId":"s","pid":1,"pidStartedAt":1,"pgid":1,"runtime":"fake","instanceTag":"t","announcedAt":"a","mainId":"NOPE","commandHash":"`+CommandHash("x")+`"}`)
	if _, err := readAnnouncements(root, true); err == nil {
		t.Fatal("a malformed main identity must refuse a strict read")
	}
	// The same malformed record is skipped by a lax read.
	if _, err := readAnnouncements(root, false); err != nil {
		t.Fatalf("a lax read must skip a malformed record, not error: %v", err)
	}
}
