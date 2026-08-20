package steward

import (
	"os"
	"strings"
	"testing"
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

func TestUndeliveredIntentCannotAuthorizeALaunch(t *testing.T) {
	root := t.TempDir()
	if err := MintIntent(root, testIntent("n2")); err != nil {
		t.Fatal(err)
	}
	if _, err := ConsumeIntent(root, "n2"); err == nil || !strings.Contains(err.Error(), "not yet delivered") {
		t.Fatalf("consumption before delivery must refuse by name: %v", err)
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
