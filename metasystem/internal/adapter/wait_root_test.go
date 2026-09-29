package adapter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// An installation root answers wait delivery through its runtime registry:
// every built-in with an adapter declares it, an unknown name does not, and
// an unreadable adapter directory answers no one.
func TestWaitDeliveryAtAnInstallationRoot(t *testing.T) {
	t.Parallel()
	request := WaitDeliveryRequest{
		WaitID:   "0123456789abcdef0123456789abcdef",
		Nonce:    "fedcba9876543210fedcba9876543210",
		Deadline: time.Date(2026, 9, 28, 12, 0, 0, 0, time.FixedZone("CEST", 2*60*60)),
		Session:  "session-1",
	}
	root := t.TempDir()
	for _, runtime := range []string{"claude", "codex", "devin", "fake"} {
		if !WaitDeliveryRuntime(root, runtime) {
			t.Errorf("built-in %s does not answer wait delivery at a plain root", runtime)
		}
		answer, err := DeliverWaitAt(context.Background(), root, runtime, request)
		if err != nil || answer != "blocking" {
			t.Errorf("DeliverWaitAt(%s) = %q, %v; want blocking", runtime, answer, err)
		}
	}
	if WaitDeliveryRuntime(root, "no-such-runtime") {
		t.Error("an undeclared runtime answered wait delivery")
	}
	_, err := DeliverWaitAt(context.Background(), root, "no-such-runtime", request)
	if err == nil || errors.Is(err, ErrWaitDeliveryDeclined) || !strings.Contains(err.Error(), "no-such-runtime is unavailable") {
		t.Fatalf("undeclared runtime = %v, want the named unavailability", err)
	}

	broken := t.TempDir()
	if err := os.WriteFile(filepath.Join(broken, "adapters"), []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if WaitDeliveryRuntime(broken, "claude") {
		t.Fatal("a root whose adapter directory is unreadable answered wait delivery")
	}
}

func TestDeliverWaitRefusesAnIncompleteRequest(t *testing.T) {
	t.Parallel()
	complete := WaitDeliveryRequest{WaitID: "w", Nonce: "n", Deadline: time.Unix(1, 0), Session: "s"}
	for name, request := range map[string]WaitDeliveryRequest{
		"no wait id":  {Nonce: "n", Deadline: complete.Deadline, Session: "s"},
		"no nonce":    {WaitID: "w", Deadline: complete.Deadline, Session: "s"},
		"no deadline": {WaitID: "w", Nonce: "n", Session: "s"},
		"no session":  {WaitID: "w", Nonce: "n", Deadline: complete.Deadline},
	} {
		if _, err := DeliverWaitAt(context.Background(), "", "claude", request); err == nil || !strings.Contains(err.Error(), "requires an adapter") {
			t.Errorf("%s: incomplete request = %v, want the named refusal", name, err)
		}
	}
	if _, err := DeliverWaitAt(context.Background(), "", "", complete); err == nil || !strings.Contains(err.Error(), "requires an adapter") {
		t.Errorf("no runtime = %v, want the named refusal", err)
	}
}
