package lease

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
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
	if got.MainId == "" {
		t.Fatalf("announced main has no mainId: %+v", got)
	}
	return got.MainId
}

// TestVerbResultWireShapes pins the CLI JSON bytes of the typed verb
// results to what the map[string]any form produced: sorted keys, absent
// keys stay absent, and require-holder's ungated paths keep their explicit
// nulls. Shell gates parse these shapes; a struct field reorder or an
// omitempty change would silently break them.
func TestVerbResultWireShapes(t *testing.T) {
	epoch, revision := int64(1), int64(3)
	mainID := "main-x"
	cases := []struct {
		value any
		want  string
	}{
		{
			ClassifyResult{Class: ClassDelegate, Pid: 7, ClaimEpoch: &epoch, Revision: &revision},
			`{"claimEpoch":1,"class":"DELEGATE","holder":false,"pid":7,"revision":3}`,
		},
		{
			ClassifyResult{Class: ClassAdapterSupervisor, Pid: 7, JobId: "job-1"},
			`{"class":"ADAPTER-SUPERVISOR","holder":false,"jobId":"job-1","pid":7}`,
		},
		{
			HolderView{Class: ClassHuman, Holder: true},
			`{"claimEpoch":null,"class":"HUMAN","holder":true,"mainId":null}`,
		},
		{
			HolderView{Class: "HOLDER", Holder: true, ClaimEpoch: &epoch, Revision: &revision, MainId: &mainID},
			`{"claimEpoch":1,"class":"HOLDER","holder":true,"mainId":"main-x","revision":3}`,
		},
		{
			RenewResult{ClaimEpoch: 1, Revision: 2},
			`{"claimEpoch":1,"revision":2}`,
		},
		{
			GrowthReport{Counts: map[string]int{"b": 2, "a": 1}},
			`{"counts":{"a":1,"b":2},"message":""}`,
		},
	}
	for _, c := range cases {
		got, err := json.Marshal(c.value)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != c.want {
			t.Fatalf("wire shape changed:\n got %s\nwant %s", got, c.want)
		}
	}
}

func TestAnnounceMakesUsMainAndHolder(t *testing.T) {
	root := t.TempDir()
	mainID := announceSelf(t, root)
	self := int64(os.Getpid())

	got, err := ClassifyVerb(root, self)
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != ClassMain || !got.Holder {
		t.Fatalf("announcer should be MAIN and holder: %+v", got)
	}
	if got.ClaimEpoch == nil || *got.ClaimEpoch != 1 {
		t.Fatalf("first claim should be epoch 1: %+v", got.ClaimEpoch)
	}

	held, err := RequireHolder(root, self, nil)
	if err != nil {
		t.Fatalf("require-holder refused the holder: %v", err)
	}
	if held.Class != "HOLDER" || !held.Holder {
		t.Fatalf("require-holder should confirm HOLDER: %+v", held)
	}
	current, err := CurrentHolder(root)
	if err != nil {
		t.Fatalf("current holder: %v", err)
	}
	if current.MainId != mainID || current.SessionId != "my sess" || current.Pid != self {
		t.Fatalf("current holder did not name the holder session: %+v", current)
	}
}

func TestDuplicateAnnouncementDoesNotBumpTheWorkerEnrollmentFence(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	started := selfStart(t)
	if _, err := Announce(root, "same-session", self, started, "tag", "fake", ""); err != nil {
		t.Fatal(err)
	}
	before, err := steward.ReadEnrollmentFence(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Announce(root, "same-session", self, started, "tag", "fake", ""); err != nil {
		t.Fatal(err)
	}
	after, err := steward.ReadEnrollmentFence(root)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("verifying the same announcement advanced the worker enrollment fence: %d -> %d", before, after)
	}
	if _, err := AnnounceWithProof(root, "same-session", self, started, 0, "", "tag", "fake", "", &IdentityProvenance{
		Source: "explicit-ancestry-fallback", CallerPid: self, CallerPidStartedAt: started,
	}); err != nil {
		t.Fatal(err)
	}
	afterEnrichment, err := steward.ReadEnrollmentFence(root)
	if err != nil {
		t.Fatal(err)
	}
	if afterEnrichment != before {
		t.Fatalf("enriching the same announcement advanced the worker enrollment fence: %d -> %d", before, afterEnrichment)
	}
}

func TestAnnouncementRecordsExplicitFallbackProvenance(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	started := selfStart(t)
	path, err := AnnounceWithProof(root, "fallback", self, started, 0, "", "tag", "fake", "", &IdentityProvenance{
		Source: "explicit-ancestry-fallback", CallerPid: self, CallerPidStartedAt: started,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var announcement Announcement
	if err := json.Unmarshal(data, &announcement); err != nil {
		t.Fatal(err)
	}
	if announcement.IdentityProvenance == nil ||
		announcement.IdentityProvenance.Source != "explicit-ancestry-fallback" ||
		announcement.IdentityProvenance.CallerPid != self ||
		announcement.IdentityProvenance.CallerPidStartedAt != started {
		t.Fatalf("fallback provenance was not recorded: %+v", announcement.IdentityProvenance)
	}
}

func TestAnnouncementSeparatesVendoredInstallFromStateRoot(t *testing.T) {
	stateRoot := t.TempDir()
	metasystemRoot := filepath.Join(stateRoot, "metasystem")
	if err := os.MkdirAll(metasystemRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metasystemRoot, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	identityTable := filepath.Join(t.TempDir(), "identities.json")
	if err := os.WriteFile(identityTable, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", identityTable)
	self := int64(os.Getpid())
	started := selfStart(t)
	path, err := AnnounceWithProofAt(stateRoot, metasystemRoot, "vendored", self, started, 0, "", "tag", "fake", "", &IdentityProvenance{
		Source: "explicit-ancestry-fallback", CallerPid: self, CallerPidStartedAt: started,
	})
	if err != nil {
		t.Fatal(err)
	}
	canonicalStateRoot, err := filepath.EvalSymlinks(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	wantDirectory := filepath.Join(canonicalStateRoot, "artifacts", "agents", "mains")
	if filepath.Dir(path) != wantDirectory {
		t.Fatalf("announcement escaped the repository state root: %s", path)
	}
	if _, err := os.Stat(filepath.Join(metasystemRoot, "artifacts")); !os.IsNotExist(err) {
		t.Fatalf("announcement split state under the vendored install: %v", err)
	}
	holder, err := RequireHolderAt(stateRoot, metasystemRoot, self, nil)
	if err != nil || holder.Class != "HOLDER" {
		t.Fatalf("vendored holder classification did not use the installation root: %+v %v", holder, err)
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
	if out.Revision != 2 {
		t.Fatalf("renew should bump revision to 2, got %d", out.Revision)
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
	if !strings.Contains(growth.Message, "PROTOCOL-ERRORS") {
		t.Fatalf("growth should report the new error: %q", growth.Message)
	}

	if err := ProtocolAdvance(root, mainID, self, `{"job-1":1}`); err != nil {
		t.Fatalf("advance: %v", err)
	}
	growth, err = ProtocolGrowth(root, mainID)
	if err != nil {
		t.Fatal(err)
	}
	if growth.Message != "" {
		t.Fatalf("after advancing, there should be no new growth: %q", growth.Message)
	}
}
