package steward

// The intervention record: minted before anything happens, notified
// before anything launches, consumed exactly once at launch. The
// record IS the one-shot authorization (its staged digests bind what
// runs), and its lifecycle makes every intervention visible-before-
// action and reconcilable after a crash.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Intent is one intervention's durable record.
type Intent struct {
	Nonce         string `json:"nonce"` // also the record's filename
	RepoIdentity  string `json:"repoIdentity"`
	InstallGen    int    `json:"installGeneration"`
	Goal          string `json:"goal"` // the claim being continued
	RoleDigest    string `json:"roleDigest"`
	BriefDigest   string `json:"briefDigest"`
	PermsDigest   string `json:"permsDigest"`
	Runtime       string `json:"runtime"` // roster-resolved, recorded not chosen
	Model         string `json:"model"`
	JobId         string `json:"jobId"`
	MintedAtTick  int    `json:"mintedAtTick"`
	Notified      bool   `json:"notified"`      // delivery confirmed
	DispatchedAt  int    `json:"dispatchedAt"`  // tick; zero = never
	LaunchStamped bool   `json:"launchStamped"` // dispatch returned
}

// Paths under the steward's own artifact directory.
func intentsDir(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "intents")
}
func consumedDir(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "consumed")
}

// MintIntent writes the record durably. Nothing may launch before
// this exists; the receipt and notification reference its nonce.
func MintIntent(repoRoot string, it Intent) error {
	if it.Nonce == "" {
		return fmt.Errorf("an intent needs a nonce")
	}
	dir := intentsDir(repoRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, it.Nonce+".json")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("intent %s already exists", it.Nonce)
	}
	data, err := json.MarshalIndent(it, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// UpdateIntent rewrites a live intent's mutable fields (notified,
// dispatched, stamped) in place, atomically.
func UpdateIntent(repoRoot string, it Intent) error {
	path := filepath.Join(intentsDir(repoRoot), it.Nonce+".json")
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("intent %s is not live: %w", it.Nonce, err)
	}
	data, err := json.MarshalIndent(it, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ConsumeIntent authorizes exactly one launch: the atomic move to
// consumed/ succeeds once; a replay finds the record gone and
// refuses. Consumption requires the prerequisites the design pins —
// delivered notification first.
func ConsumeIntent(repoRoot, nonce string) (Intent, error) {
	live := filepath.Join(intentsDir(repoRoot), nonce+".json")
	data, err := os.ReadFile(live)
	if err != nil {
		return Intent{}, fmt.Errorf("intent %s cannot authorize: %w", nonce, err)
	}
	var it Intent
	if err := json.Unmarshal(data, &it); err != nil {
		return Intent{}, fmt.Errorf("intent %s malformed: %w", nonce, err)
	}
	if !it.Notified {
		return Intent{}, fmt.Errorf("intent %s is not yet delivered to the operator; launch refused", nonce)
	}
	if err := os.MkdirAll(consumedDir(repoRoot), 0o755); err != nil {
		return Intent{}, err
	}
	consumed := filepath.Join(consumedDir(repoRoot), nonce+".json")
	if err := os.Rename(live, consumed); err != nil {
		return Intent{}, fmt.Errorf("intent %s already consumed or gone: %w", nonce, err)
	}
	return it, nil
}

// LiveIntents lists unconsumed records — the next tick's
// reconciliation input and the one-active-continuation guard.
func LiveIntents(repoRoot string) ([]Intent, error) {
	paths, err := filepath.Glob(filepath.Join(intentsDir(repoRoot), "*.json"))
	if err != nil {
		return nil, err
	}
	var out []Intent
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var it Intent
		if err := json.Unmarshal(data, &it); err != nil {
			return nil, fmt.Errorf("intent %s malformed: %w", filepath.Base(p), err)
		}
		out = append(out, it)
	}
	return out, nil
}

// PendingNotification is one undelivered operator message, durable
// until delivery succeeds; retried every tick, never redispatching.
type PendingNotification struct {
	Nonce   string `json:"nonce"`
	Message string `json:"message"`
}

func pendingDir(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "pending")
}

// QueueNotification stores the message durably before any delivery
// attempt.
func QueueNotification(repoRoot string, n PendingNotification) error {
	dir := pendingDir(repoRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(n)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, n.Nonce+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// PendingNotifications lists what still awaits delivery.
func PendingNotifications(repoRoot string) ([]PendingNotification, error) {
	paths, err := filepath.Glob(filepath.Join(pendingDir(repoRoot), "*.json"))
	if err != nil {
		return nil, err
	}
	var out []PendingNotification
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var n PendingNotification
		if err := json.Unmarshal(data, &n); err != nil {
			return nil, fmt.Errorf("pending notification %s malformed: %w", filepath.Base(p), err)
		}
		out = append(out, n)
	}
	return out, nil
}

// MarkDelivered removes a delivered notification from the queue.
func MarkDelivered(repoRoot, nonce string) error {
	return os.Remove(filepath.Join(pendingDir(repoRoot), nonce+".json"))
}
