package steward

// The intervention record is minted before anything happens and consumed
// exactly once at launch. The record is the one-shot authorization whose
// staged digests bind what runs, making every repair reconcilable after a
// crash without requiring an alert before action.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// Intent is one intervention's durable record.
type Intent struct {
	Nonce         string `json:"nonce"` // also the record's filename
	RepoIdentity  string `json:"repoIdentity"`
	InstallGen    int    `json:"installGeneration"`
	Goal          string `json:"goal"`        // the claim being continued
	Role          string `json:"role"`        // always steward-continuation; recorded, never chosen
	Permissions   string `json:"permissions"` // the preset name whose expansion PermsDigest pins
	RoleDigest    string `json:"roleDigest"`
	ReqDigest     string `json:"reqDigest"`
	SchemaDigest  string `json:"schemaDigest"`
	BriefDigest   string `json:"briefDigest"`
	PermsDigest   string `json:"permsDigest"`
	Runtime       string `json:"runtime"` // roster-resolved, recorded not chosen
	Model         string `json:"model"`
	JobId         string `json:"jobId"`
	MintedAtTick  int    `json:"mintedAtTick"`
	FenceAtMint   int64  `json:"fenceAtMint"`
	Notified      bool   `json:"notified"`      // retained for pre-heal-first records; never gates launch
	DispatchedAt  int    `json:"dispatchedAt"`  // tick; zero = never
	LaunchStamped bool   `json:"launchStamped"` // dispatch returned
	ReapedAt      string `json:"reapedAt,omitempty"`
	Outcome       string `json:"outcome,omitempty"`
}

// Paths under the steward's own artifact directory.
func intentsDir(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "intents")
}
func consumedDir(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "consumed")
}
func cancelledDir(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "cancelled")
}

// CancelIntent retires a live intent WITHOUT authorizing anything:
// the record moves to the cancelled store, which no authorization
// path reads — a cancelled reservation can never be replayed into a
// launch.
func CancelIntent(repoRoot, nonce, reason string) error {
	live := filepath.Join(intentsDir(repoRoot), nonce+".json")
	data, err := os.ReadFile(live)
	if err != nil {
		return fmt.Errorf("intent %s cannot cancel: %w", nonce, err)
	}
	var it Intent
	if err := json.Unmarshal(data, &it); err != nil {
		return fmt.Errorf("intent %s malformed: %w", nonce, err)
	}
	it.Outcome = "cancelled: " + reason
	out, err := json.MarshalIndent(it, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cancelledDir(repoRoot), 0o755); err != nil {
		return err
	}
	// The live authorization dies FIRST: a crash between the remove
	// and the tombstone loses only bookkeeping, never leaves a live
	// half-prepared intent a retry could resume.
	if err := os.Remove(live); err != nil {
		return err
	}
	target := filepath.Join(cancelledDir(repoRoot), nonce+".json")
	if err := os.WriteFile(target+".tmp", out, 0o644); err != nil {
		return err
	}
	return os.Rename(target+".tmp", target)
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
// consumed/ succeeds once and a replay finds the record gone. Recovery is
// deliberately not notification-gated; the operator is contacted only when
// the machinery cannot complete the repair.
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
	if err := os.MkdirAll(consumedDir(repoRoot), 0o755); err != nil {
		return Intent{}, err
	}
	// The reaper's setup grace measures from the record's mtime, and
	// a rename would keep the mint time — restamp BEFORE the
	// irreversible move, so a restamp that cannot happen refuses
	// consumption instead of launching behind a grace an outage may
	// already have spent.
	now := time.Now()
	if err := os.Chtimes(live, now, now); err != nil {
		return Intent{}, fmt.Errorf("intent %s: the setup grace could not anchor at consumption; launch refused: %w", nonce, err)
	}
	consumed := filepath.Join(consumedDir(repoRoot), nonce+".json")
	if err := os.Rename(live, consumed); err != nil {
		return Intent{}, fmt.Errorf("intent %s already consumed or gone: %w", nonce, err)
	}
	return it, nil
}

// ConsumedIntent reads one consumed record — the dispatch
// authorization's source of staged values.
func ConsumedIntent(repoRoot, nonce string) (Intent, error) {
	data, err := os.ReadFile(filepath.Join(consumedDir(repoRoot), nonce+".json"))
	if err != nil {
		return Intent{}, fmt.Errorf("no consumed intent %s: %w", nonce, err)
	}
	var it Intent
	if err := json.Unmarshal(data, &it); err != nil {
		return Intent{}, fmt.Errorf("consumed intent %s malformed: %w", nonce, err)
	}
	return it, nil
}

// StampLaunch marks a consumed intent's dispatch as returned — the
// crash boundary the next tick reconciles against: consumed without
// a stamp is an unknown launch outcome.
func StampLaunch(repoRoot, nonce string) error {
	path := filepath.Join(consumedDir(repoRoot), nonce+".json")
	it, err := ConsumedIntent(repoRoot, nonce)
	if err != nil {
		return err
	}
	it.LaunchStamped = true
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

// ConsumedActive lists consumed intents not yet reaped — the window
// of the steward's authority over its continuation, from consumption
// through the reaper's close.
func ConsumedActive(repoRoot string) ([]Intent, error) {
	entries, err := os.ReadDir(consumedDir(repoRoot))
	if err != nil && !os.IsNotExist(err) {
		// An unreadable store must fail the read, not look empty:
		// a hidden authorization would bypass the one-continuation
		// guard, and hidden incidents would go silent.
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			paths = append(paths, filepath.Join(consumedDir(repoRoot), entry.Name()))
		}
	}
	var out []Intent
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var it Intent
		if err := json.Unmarshal(data, &it); err != nil {
			return nil, fmt.Errorf("consumed intent %s malformed: %w", filepath.Base(p), err)
		}
		if it.ReapedAt == "" {
			out = append(out, it)
		}
	}
	return out, nil
}

// ConsumedActiveJob names the one active continuation's job. More
// than one is the guard bypass it would be, reported not picked from.
func ConsumedActiveJob(repoRoot string) (string, bool, error) {
	active, err := ConsumedActive(repoRoot)
	if err != nil {
		return "", false, err
	}
	if len(active) > 1 {
		return "", false, fmt.Errorf("two consumed intents are active (%s and %s); the one-active-continuation guard was bypassed", active[0].JobId, active[1].JobId)
	}
	if len(active) == 0 {
		return "", false, nil
	}
	return active[0].JobId, true, nil
}

// LiveIntents lists unconsumed records — the next tick's
// reconciliation input and the one-active-continuation guard.
func LiveIntents(repoRoot string) ([]Intent, error) {
	entries, err := os.ReadDir(intentsDir(repoRoot))
	if err != nil && !os.IsNotExist(err) {
		// An unreadable store must fail the read, not look empty:
		// a hidden authorization would bypass the one-continuation
		// guard, and hidden incidents would go silent.
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			paths = append(paths, filepath.Join(intentsDir(repoRoot), entry.Name()))
		}
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
	Nonce         string `json:"nonce"`
	Message       string `json:"message"`
	DeliveryOwner string `json:"deliveryOwner,omitempty"`
}

// L3 used this literal to mark held health findings for L4. The episode
// migration recognizes it only long enough to preserve and retire that shape.
const legacyHealthDeliveryOwner = "L4"

func pendingDir(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "pending")
}

// QueueNotification stores the message durably before any delivery
// attempt.
func QueueNotification(repoRoot string, n PendingNotification) error {
	dir := pendingDir(repoRoot)
	data, err := json.Marshal(n)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, n.Nonce+".json")
	durable, err := atomicfile.WriteText(path, string(append(data, '\n')), repoRoot)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("pending notification %s was published with durability unknown", n.Nonce)
	}
	return nil
}

// PendingNotifications lists what still awaits delivery.
func PendingNotifications(repoRoot string) ([]PendingNotification, error) {
	entries, err := os.ReadDir(pendingDir(repoRoot))
	if err != nil && !os.IsNotExist(err) {
		// An unreadable store must fail the read, not look empty:
		// a hidden authorization would bypass the one-continuation
		// guard, and hidden incidents would go silent.
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			paths = append(paths, filepath.Join(pendingDir(repoRoot), entry.Name()))
		}
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
