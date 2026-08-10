package lease

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// announceSelf announces this test process as a main and returns its mainId.
func announceSelf(t *testing.T, root string) string {
	t.Helper()
	self := int64(os.Getpid())
	if _, err := Announce(root, "my sess", self, selfStart(t), "tag", "fake", ""); err != nil {
		t.Fatalf("announce self: %v", err)
	}
	got, err := ClassifyVerb(root, self)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := got["mainId"].(string)
	if id == "" {
		t.Fatalf("announced main has no mainId: %v", got)
	}
	return id
}

func TestAnnounceMakesUsMainAndHolder(t *testing.T) {
	root := t.TempDir()
	announceSelf(t, root)
	self := int64(os.Getpid())

	got, err := ClassifyVerb(root, self)
	if err != nil {
		t.Fatal(err)
	}
	if got["class"] != ClassMain || got["holder"] != true {
		t.Fatalf("announcer should be MAIN and holder: %v", got)
	}
	if epoch, _ := jsonInt(got["claimEpoch"]); epoch != 1 {
		t.Fatalf("first claim should be epoch 1: %v", got["claimEpoch"])
	}

	held, err := RequireHolder(root, self, nil)
	if err != nil {
		t.Fatalf("require-holder refused the holder: %v", err)
	}
	if held["class"] != "HOLDER" || held["holder"] != true {
		t.Fatalf("require-holder should confirm HOLDER: %v", held)
	}
}

func TestAnnounceFillsAbsentOwnerLineageOnce(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	if _, err := Announce(root, "sess", self, selfStart(t), "tag", "fake", ""); err != nil {
		t.Fatal(err)
	}
	// Re-announce with a lineage: the absent-to-present fill is allowed.
	if _, err := Announce(root, "sess", self, selfStart(t), "tag", "fake", "mission-x"); err != nil {
		t.Fatalf("absent-to-present lineage fill should succeed: %v", err)
	}
	// Changing an established lineage is refused.
	if _, err := Announce(root, "sess", self, selfStart(t), "tag", "fake", "mission-y"); err == nil {
		t.Fatal("changing an established owner lineage must be refused")
	}
	lease, _ := loadLease(root, true)
	if lease.OwnerLineage != "mission-x" {
		t.Fatalf("lease should carry the filled lineage: %q", lease.OwnerLineage)
	}
}

func TestRenewBumpsRevision(t *testing.T) {
	root := t.TempDir()
	announceSelf(t, root)
	self := int64(os.Getpid())
	out, err := Renew(root, self)
	if err != nil {
		t.Fatalf("renew refused: %v", err)
	}
	if rev, _ := jsonInt(out["revision"]); rev != 2 {
		t.Fatalf("renew should bump revision to 2, got %v", out["revision"])
	}
}

func TestRunHeldRunsForHolder(t *testing.T) {
	root := t.TempDir()
	announceSelf(t, root)
	self := int64(os.Getpid())
	code, err := RunHeld(root, self, nil, []string{"/bin/sh", "-c", "exit 7"})
	if err != nil {
		t.Fatalf("run-held errored: %v", err)
	}
	if code != 7 {
		t.Fatalf("run-held should return the child's exit code, got %d", code)
	}
}

func TestRetireRemovesAnnouncement(t *testing.T) {
	root := t.TempDir()
	announceSelf(t, root)
	self := int64(os.Getpid())
	if err := Retire(root, "my sess", self, selfStart(t)); err != nil {
		t.Fatalf("retire: %v", err)
	}
	recs, err := readAnnouncements(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 0 {
		t.Fatalf("retire should remove the announcement, %d remain", len(recs))
	}
}

func TestProtocolGrowthAndAdvance(t *testing.T) {
	root := t.TempDir()
	mainID := announceSelf(t, root)
	self := int64(os.Getpid())

	// A job carrying a protocol error the main has not yet acknowledged.
	writeJSON(t, filepath.Join(root, "artifacts/agents/jobs/job-1.json"),
		`{"jobId":"job-1","protocolError":{"key":"K1"}}`)

	growth, err := ProtocolGrowth(root, mainID)
	if err != nil {
		t.Fatal(err)
	}
	if msg, _ := growth["message"].(string); !strings.Contains(msg, "PROTOCOL-ERRORS") {
		t.Fatalf("growth should report the new error: %v", growth["message"])
	}

	if err := ProtocolAdvance(root, mainID, self, `{"job-1":1}`); err != nil {
		t.Fatalf("advance: %v", err)
	}
	growth, err = ProtocolGrowth(root, mainID)
	if err != nil {
		t.Fatal(err)
	}
	if msg, _ := growth["message"].(string); msg != "" {
		t.Fatalf("after advancing, there should be no new growth: %q", msg)
	}
}
