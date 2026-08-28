package steward

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// notifyRepo is a git repository whose notify-command appends to a
// sink file — a fully observable delivery channel.
func notifyRepo(t *testing.T, command string) (string, string) {
	t.Helper()
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	sink := filepath.Join(t.TempDir(), "delivered.log")
	if command == "" {
		command = `printf '%s\n' "$STEWARD_MESSAGE" >> ` + sink
	}
	if out, err := exec.Command("git", "-C", root, "config", "metasystem.steward.notify-command", command).CombinedOutput(); err != nil {
		t.Fatalf("config: %v\n%s", err, out)
	}
	return root, sink
}

func TestDeliveryMeansTheCommandSucceeded(t *testing.T) {
	root, sink := notifyRepo(t, "")
	if err := Deliver(root, "worker dead; reviving fix-it"); err != nil {
		t.Fatalf("a zero exit is a delivery: %v", err)
	}
	data, err := os.ReadFile(sink)
	if err != nil || !strings.Contains(string(data), "worker dead") {
		t.Fatalf("the message must reach the channel: %q %v", data, err)
	}
}

func TestFailedChannelIsNotADelivery(t *testing.T) {
	root, _ := notifyRepo(t, "exit 1")
	if err := Deliver(root, "anything"); err == nil {
		t.Fatal("a failing notifier must not claim delivery")
	}
}

func TestPendingQueueDrainsOnDeliveryAndHoldsOnFailure(t *testing.T) {
	root, sink := notifyRepo(t, "")
	for _, n := range []string{"a", "b"} {
		if err := QueueNotification(root, PendingNotification{Nonce: n, Message: "msg-" + n}); err != nil {
			t.Fatal(err)
		}
	}
	delivered, err := DeliverPending(root)
	if err != nil || delivered != 2 {
		t.Fatalf("both must deliver: %d %v", delivered, err)
	}
	if pending, _ := PendingNotifications(root); len(pending) != 0 {
		t.Fatalf("delivered messages leave the queue: %v", pending)
	}
	data, _ := os.ReadFile(sink)
	if !strings.Contains(string(data), "msg-a") || !strings.Contains(string(data), "msg-b") {
		t.Fatalf("both messages must reach the channel: %q", data)
	}

	// Break the channel: the queue holds and reports.
	if out, err := exec.Command("git", "-C", root, "config", "metasystem.steward.notify-command", "exit 1").CombinedOutput(); err != nil {
		t.Fatalf("reconfig: %v\n%s", err, out)
	}
	if err := QueueNotification(root, PendingNotification{Nonce: "c", Message: "msg-c"}); err != nil {
		t.Fatal(err)
	}
	if _, err := DeliverPending(root); err == nil {
		t.Fatal("a down channel must surface, not silently drop")
	}
	if pending, _ := PendingNotifications(root); len(pending) != 1 {
		t.Fatalf("undelivered messages stay queued: %v", pending)
	}
}

func TestHealthFindingsStayPendingByDistinctDigest(t *testing.T) {
	root, sink := notifyRepo(t, "")
	first := evidenceDigest("runner stale")
	second := evidenceDigest("narrator stale")
	if err := QueueHealthNotification(root, first, "HEALTH unhealthy — runner stale"); err != nil {
		t.Fatal(err)
	}
	if err := QueueHealthNotification(root, first, "HEALTH unhealthy — runner stale"); err != nil {
		t.Fatal(err)
	}
	if err := QueueHealthNotification(root, second, "HEALTH unhealthy — narrator stale"); err != nil {
		t.Fatal(err)
	}
	if _, err := DeliverPending(root); err != nil {
		t.Fatal(err)
	}
	pending, err := PendingNotifications(root)
	if err != nil || len(pending) != 2 {
		t.Fatalf("one held notification per distinct finding must remain for L4: %v %v", pending, err)
	}
	for _, notification := range pending {
		if notification.Nonce != first && notification.Nonce != second {
			t.Fatalf("the finding digest is the notification nonce: %+v", notification)
		}
		if notification.DeliveryOwner != healthDeliveryOwner {
			t.Fatalf("health delivery remains owned by L4: %+v", notification)
		}
	}
	if data, err := os.ReadFile(sink); err == nil && len(data) > 0 {
		t.Fatalf("L3 must not drain health notifications through the notifier: %q", data)
	}
	if err := QueueHealthNotification(root, first, "a later rendering of the same finding"); err != nil {
		t.Fatal(err)
	}
	after, err := PendingNotifications(root)
	if err != nil || len(after) != 2 {
		t.Fatalf("a repeated finding must retain the existing pending record: %v %v", after, err)
	}
	for _, notification := range after {
		if notification.Nonce == first && notification.Message != "HEALTH unhealthy — runner stale" {
			t.Fatalf("a repeated finding must not overwrite its pending notification: %+v", notification)
		}
	}
	occupied := evidenceDigest("occupied nonce")
	if err := QueueNotification(root, PendingNotification{Nonce: occupied, Message: "an unrelated pending message"}); err != nil {
		t.Fatal(err)
	}
	if err := QueueHealthNotification(root, occupied, "HEALTH unhealthy — another finding"); err == nil {
		t.Fatal("a health finding must not overwrite a different pending notification")
	}
}
