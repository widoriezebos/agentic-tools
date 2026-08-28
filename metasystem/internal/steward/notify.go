package steward

// Operator notification, delivery-gated: a message is DELIVERED only
// when the configured command exits zero — that is the acknowledgment
// the launch gate requires. The command comes from the repository's
// local git configuration (metasystem.steward.notify-command); darwin
// falls back to the platform notifier; anywhere else, no configured
// command means delivery cannot be claimed and installation refuses
// up front (the glue enforces that; this layer just reports).

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// notifyTimeout bounds one delivery attempt; a hung notifier is a
// failed attempt, retried next tick, never a wedged tick.
const notifyTimeout = 15 * time.Second

// NotifyCommand resolves the configured delivery command.
func NotifyCommand(repoRoot string) (string, bool) {
	out, err := exec.Command("git", "-C", repoRoot, "config", "--get", "metasystem.steward.notify-command").Output()
	if err == nil {
		if cmd := strings.TrimSpace(string(out)); cmd != "" {
			return cmd, true
		}
	}
	if runtime.GOOS == "darwin" {
		return "", true // the platform notifier below
	}
	return "", false
}

// Deliver attempts one delivery. Returning nil MEANS delivered — the
// caller may gate a launch on it.
func Deliver(repoRoot, message string) error {
	command, ok := NotifyCommand(repoRoot)
	if !ok {
		return fmt.Errorf("no notification channel is configured and this platform has no default; the operator cannot be reached")
	}
	ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
	defer cancel()
	var cmd *exec.Cmd
	if command == "" {
		script := fmt.Sprintf("display notification %q with title %q", message, "metasystem steward")
		cmd = exec.CommandContext(ctx, "osascript", "-e", script)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", command)
		cmd.Env = append(cmd.Environ(), "STEWARD_MESSAGE="+message)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("notification not delivered: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// DeliverPending retries the queue: each delivered message leaves it;
// the first failure stops the pass (the channel is down — one named
// failure beats a burst of them). Returns how many were delivered.
func DeliverPending(repoRoot string) (int, error) {
	pending, err := PendingNotifications(repoRoot)
	if err != nil {
		return 0, err
	}
	delivered := 0
	for _, n := range pending {
		if n.DeliveryOwner == legacyHealthDeliveryOwner {
			continue
		}
		if n.Nonce == "verdict-"+string(VerdictStalledDead) {
			// Older runners queued proven-death alerts before attempting
			// revival. Retire that obsolete intent instead of delivering it
			// after an upgrade or after the condition has already healed.
			if err := MarkDelivered(repoRoot, n.Nonce); err != nil {
				return delivered, err
			}
			continue
		}
		if err := Deliver(repoRoot, n.Message); err != nil {
			return delivered, err
		}
		// The intent acknowledges FIRST: a crash between these two
		// writes then repeats a delivery (benign) instead of
		// stranding an undelivered-looking intent with no pending
		// message (a permanent suppression).
		if err := markIntentNotified(repoRoot, n.Nonce); err != nil {
			return delivered, err
		}
		if err := MarkDelivered(repoRoot, n.Nonce); err != nil {
			return delivered, err
		}
		delivered++
	}
	return delivered, nil
}

// markIntentNotified flips the live intent matching a delivered
// message; a nonce with no live intent is ordinary (verdict and reap
// messages have none).
func markIntentNotified(repoRoot, nonce string) error {
	live, err := LiveIntents(repoRoot)
	if err != nil {
		return err
	}
	for _, it := range live {
		if it.Nonce == nonce && !it.Notified {
			it.Notified = true
			return UpdateIntent(repoRoot, it)
		}
	}
	return nil
}
