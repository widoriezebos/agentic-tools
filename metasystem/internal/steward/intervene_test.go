package steward

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFileRaw(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func testIntent(nonce string) Intent {
	return Intent{
		Nonce: nonce, RepoIdentity: "repo", InstallGen: 1, Goal: "fix-it",
		RoleDigest: "r", BriefDigest: "b", PermsDigest: "p",
		Runtime: "fake", Model: "fixture", JobId: "job-1", MintedAtTick: 7,
	}
}

func TestIntentExistsBeforeAnythingElse(t *testing.T) {
	root := t.TempDir()
	if err := MintIntent(root, testIntent("n1")); err != nil {
		t.Fatal(err)
	}
	live, err := LiveIntents(root)
	if err != nil || len(live) != 1 || live[0].Goal != "fix-it" {
		t.Fatalf("the intent must be durably live: %v %v", live, err)
	}
	if err := MintIntent(root, testIntent("n1")); err == nil {
		t.Fatal("re-minting the same nonce must refuse")
	}
}

func TestPreparedIntentAuthorizesHealingWithoutNotification(t *testing.T) {
	root := t.TempDir()
	if err := MintIntent(root, testIntent("n2")); err != nil {
		t.Fatal(err)
	}
	if _, err := ConsumeIntent(root, "n2"); err != nil {
		t.Fatalf("notification cannot gate an automatic repair: %v", err)
	}
}

func TestConsumedIntentRefusesReplay(t *testing.T) {
	root := t.TempDir()
	it := testIntent("n3")
	it.Notified = true
	if err := MintIntent(root, it); err != nil {
		t.Fatal(err)
	}
	if _, err := ConsumeIntent(root, "n3"); err != nil {
		t.Fatalf("a delivered intent authorizes once: %v", err)
	}
	if _, err := ConsumeIntent(root, "n3"); err == nil {
		t.Fatal("the second consumption is a replay and must refuse")
	}
	if live, _ := LiveIntents(root); len(live) != 0 {
		t.Fatalf("a consumed intent leaves the live set: %v", live)
	}
}

func TestNotificationQueueSurvivesUntilDelivery(t *testing.T) {
	root := t.TempDir()
	if err := QueueNotification(root, PendingNotification{Nonce: "n4", Message: "worker dead; reviving fix-it"}); err != nil {
		t.Fatal(err)
	}
	pending, err := PendingNotifications(root)
	if err != nil || len(pending) != 1 {
		t.Fatalf("queued must persist: %v %v", pending, err)
	}
	if err := MarkDelivered(root, "n4"); err != nil {
		t.Fatal(err)
	}
	if pending, _ := PendingNotifications(root); len(pending) != 0 {
		t.Fatalf("delivered leaves the queue: %v", pending)
	}
}

func TestMalformedIntentSurfacesByName(t *testing.T) {
	root := t.TempDir()
	if err := MintIntent(root, testIntent("n5")); err != nil {
		t.Fatal(err)
	}
	// Tear the live record's bytes.
	path := intentsDir(root) + "/n5.json"
	if err := writeFileRaw(path, "{torn"); err != nil {
		t.Fatal(err)
	}
	if _, err := LiveIntents(root); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("a torn intent must surface, never be skipped: %v", err)
	}
}

func TestUnreadableStoresFailClosed(t *testing.T) {
	root := t.TempDir()
	if err := MintIntent(root, testIntent("fc-1")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(intentsDir(root), 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(intentsDir(root), 0o755)
	if _, err := LiveIntents(root); err == nil {
		t.Fatal("an unreadable intent store must fail the read, not look empty")
	}
	if err := QueueNotification(root, PendingNotification{Nonce: "fc-p", Message: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(pendingDir(root), 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(pendingDir(root), 0o755)
	if _, err := PendingNotifications(root); err == nil {
		t.Fatal("an unreadable pending store must fail the read, not look empty")
	}
	if err := os.MkdirAll(consumedDir(root), 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(consumedDir(root), 0o755)
	if _, err := ConsumedActive(root); err == nil {
		t.Fatal("an unreadable consumed store must fail the read, not look empty")
	}
}

func TestFailedCancelStillRetiresTheAuthorization(t *testing.T) {
	root := t.TempDir()
	if err := MintIntent(root, testIntent("fc-2")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cancelledDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cancelledDir(root), 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(cancelledDir(root), 0o755)
	if err := CancelIntent(root, "fc-2", "test"); err == nil {
		t.Fatal("an unwritable tombstone store must surface as an error")
	}
	// The safe direction: the live authorization died before the
	// tombstone failed — a retry cannot resume a half-preparation.
	if live, _ := LiveIntents(root); len(live) != 0 {
		t.Fatalf("the live authorization must be gone even when the tombstone fails: %v", live)
	}
}

func TestConsumptionRestampsTheSetupGrace(t *testing.T) {
	root := t.TempDir()
	it := testIntent("fc-3")
	it.Notified = true
	if err := MintIntent(root, it); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(filepath.Join(intentsDir(root), "fc-3.json"), old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := ConsumeIntent(root, "fc-3"); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(consumedDir(root), "fc-3.json"))
	if err != nil {
		t.Fatal(err)
	}
	// The reaper's grace measures from this mtime: an intent that sat
	// through a notifier outage must still get its full setup window.
	if time.Since(fi.ModTime()) > time.Minute {
		t.Fatalf("consumption must restamp the grace anchor: %v", fi.ModTime())
	}
}

func TestConsumptionFailureLeavesTheIntentLive(t *testing.T) {
	root := t.TempDir()
	it := testIntent("fc-4")
	it.Notified = true
	if err := MintIntent(root, it); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(consumedDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(consumedDir(root), 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(consumedDir(root), 0o755)
	if _, err := ConsumeIntent(root, "fc-4"); err == nil {
		t.Fatal("a consumption that cannot complete must refuse")
	}
	// Fail-closed: the authorization is still live for the next
	// tick, and nothing half-consumed can launch.
	live, err := LiveIntents(root)
	if err != nil || len(live) != 1 || live[0].Nonce != "fc-4" {
		t.Fatalf("a refused consumption leaves the intent live: %v %v", live, err)
	}
	if consumed, _ := ConsumedActive(root); len(consumed) != 0 {
		t.Fatalf("nothing may appear consumed after a refusal: %v", consumed)
	}
}

func TestUpdateIntentRequiresALiveRecordAndPersistsMutableState(t *testing.T) {
	root := t.TempDir()
	it := testIntent("update-1")
	if err := UpdateIntent(root, it); err == nil || !strings.Contains(err.Error(), "is not live") {
		t.Fatalf("missing intent was updated: %v", err)
	}
	if err := MintIntent(root, it); err != nil {
		t.Fatal(err)
	}
	it.Notified = true
	it.DispatchedAt = 9
	if err := UpdateIntent(root, it); err != nil {
		t.Fatal(err)
	}
	live, err := LiveIntents(root)
	if err != nil || len(live) != 1 || !live[0].Notified || live[0].DispatchedAt != 9 {
		t.Fatalf("mutable intent state was not persisted: intents=%+v err=%v", live, err)
	}
}

func TestConsumedActiveJobRefusesAmbiguityAndNamesOneJob(t *testing.T) {
	root := t.TempDir()
	if job, ok, err := ConsumedActiveJob(root); err != nil || ok || job != "" {
		t.Fatalf("empty consumed store named a job: job=%q ok=%v err=%v", job, ok, err)
	}
	first := testIntent("active-1")
	first.JobId = "job-one"
	if err := MintIntent(root, first); err != nil {
		t.Fatal(err)
	}
	if _, err := ConsumeIntent(root, first.Nonce); err != nil {
		t.Fatal(err)
	}
	if job, ok, err := ConsumedActiveJob(root); err != nil || !ok || job != "job-one" {
		t.Fatalf("one active continuation was not named: job=%q ok=%v err=%v", job, ok, err)
	}
	second := testIntent("active-2")
	second.JobId = "job-two"
	if err := MintIntent(root, second); err != nil {
		t.Fatal(err)
	}
	if _, err := ConsumeIntent(root, second.Nonce); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ConsumedActiveJob(root); err == nil || !strings.Contains(err.Error(), "guard was bypassed") {
		t.Fatalf("two active continuations were collapsed to one: %v", err)
	}
}

func TestConsumedActiveJobPreservesUnreadableStoreFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(consumedDir(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(consumedDir(root), []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ConsumedActiveJob(root); err == nil {
		t.Fatal("an unreadable consumed store looked empty")
	}
}
