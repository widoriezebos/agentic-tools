package testenv

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// The stale registrations m1e's disk show printed were fixture checkouts
// (mktemp tmp.* repositories, operator-repo and supervision fixtures,
// scratchpad repositories, all written 2026-08-10 to 2026-09-01 by an
// engine built as "dev") armed into the account's own
// ~/.metasystem/armed-checkouts.jsonl by harnesses that ran the engine
// without the test registry home. A test process under Main arms a fixture
// into its own run-scoped registry: the path it selects is never under the
// account's home, and arming there leaves the account's registry exactly as
// it was.
func TestFixtureArmingLeavesTheAccountRegistryUntouched(t *testing.T) {
	account, err := user.Current()
	if err != nil {
		t.Fatalf("read the account: %v", err)
	}
	if account.HomeDir == "" || account.HomeDir == os.Getenv("HOME") {
		t.Fatalf("the account's home %q is not distinct from the test HOME %q: the witness cannot see a leak", account.HomeDir, os.Getenv("HOME"))
	}
	accountRegistry := filepath.Join(account.HomeDir, ".metasystem", "armed-checkouts.jsonl")

	path, err := registry.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(path, account.HomeDir+string(filepath.Separator)) {
		t.Fatalf("a test process selects the registry %s under the account's home %s", path, account.HomeDir)
	}
	fixture := t.TempDir()
	payload, err := json.Marshal(map[string]any{"schemaVersion": 1, "event": registry.EventArming, "checkoutPath": fixture,
		"ownerTag": "metasystem-supervision-owner-isolation-witness", "at": time.Now().UTC().Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := registry.AppendFrame(path, payload); err != nil {
		t.Fatal(err)
	}
	// The account's registry is live (the person's own owners append to
	// it), so the witness asks whether it names the fixture, not whether it
	// changed.
	data, err := os.ReadFile(accountRegistry)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(fixture)) {
		t.Fatalf("arming the fixture %s wrote it into the account's registry %s", fixture, accountRegistry)
	}
	if own, err := os.ReadFile(path); err != nil || !bytes.Contains(own, []byte(fixture)) {
		t.Fatalf("the test's own registry %s does not name the fixture: %v", path, err)
	}
}
