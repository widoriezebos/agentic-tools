package tickrequest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }

func TestRequestCheckoutAliasesPreserveThePendingBatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "checkout")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 20, 5, 0, 0, 0, time.UTC)
	if err := Write(root, "first", now); err != nil {
		t.Fatal(err)
	}
	first, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(alias, "second", now); err != nil {
		t.Fatal(err)
	}
	next, err := Read(root)
	if err != nil || next.Batch != first.Batch || next.Count != 2 || next.Generation != 2 {
		t.Fatalf("alias replaced the pending batch: %+v %v", next, err)
	}
	// A request may name either spelling of the checkout.
	next.Root = alias
	if err := write(root, next); err != nil {
		t.Fatal(err)
	}
	if err := Acknowledge(alias, first); err != nil {
		t.Fatal(err)
	}
	pending, err := Read(root)
	if err != nil || pending.Count != 1 || pending.Reason != "second" {
		t.Fatalf("alias acknowledgement lost a completion: %+v %v", pending, err)
	}
	if err := Acknowledge(root, pending); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(alias); !os.IsNotExist(err) {
		t.Fatalf("alias still has served request: %v", err)
	}
}

func TestForeignRequestWriterPreservesThePendingBatch(t *testing.T) {
	t.Parallel()
	root, other := t.TempDir(), t.TempDir()
	now := time.Date(2026, 11, 20, 5, 0, 0, 0, time.UTC)
	if err := Write(root, "first", now); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(Path(root))
	if err != nil {
		t.Fatal(err)
	}
	// Both paths reach the same state directory but identify distinct checkouts.
	if err := os.Symlink(filepath.Join(root, "artifacts"), filepath.Join(other, "artifacts")); err != nil {
		t.Fatal(err)
	}
	if err := Write(other, "foreign", now); err == nil {
		t.Fatal("foreign writer accepted")
	}
	after, err := os.ReadFile(Path(root))
	if err != nil || string(before) != string(after) {
		t.Fatal("foreign writer destroyed the valid pending request")
	}
	var request Request
	if err := json.Unmarshal(after, &request); err != nil || request.Count != 1 {
		t.Fatalf("pending batch changed: %+v %v", request, err)
	}
}

func TestRequestsSurviveRestartAndAcknowledgeOnlyTheServedBatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Date(2026, 11, 20, 5, 0, 0, 0, time.UTC)
	if err := Write(root, "launch-ended:first", now); err != nil {
		t.Fatal(err)
	}
	served, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := Write(root, "job-ended:next", now); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if err := Acknowledge(root, served); err != nil {
		t.Fatal(err)
	}
	next, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if next.Count != 50 || next.Reason != "job-ended:next" {
		t.Fatalf("pending after cycle: %+v", next)
	}
	// A fresh reader models a restarted runner; reading never consumes a request.
	restart, err := Read(root)
	if err != nil || restart != next {
		t.Fatalf("restart: %+v, %v", restart, err)
	}
	if err := Acknowledge(root, restart); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root); !os.IsNotExist(err) {
		t.Fatalf("served request remains: %v", err)
	}
	if err := Write(root, "new batch", now); err != nil {
		t.Fatal(err)
	}
	if err := Acknowledge(root, served); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root); err != nil {
		t.Fatalf("old acknowledgement erased new batch: %v", err)
	}
}

func TestRequestIsolationAndUnsafeRecords(t *testing.T) {
	t.Parallel()
	root, other := t.TempDir(), t.TempDir()
	now := time.Now()
	if err := Write(root, "done\x1b[31m\nnext", now); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(other); !os.IsNotExist(err) {
		t.Fatal("request crossed checkouts")
	}
	request, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if request.Reason != "done[31m next" {
		t.Fatalf("unsafe reason: %q", request.Reason)
	}
	data, err := os.ReadFile(Path(root))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(Path(other)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(other), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(other); err == nil {
		t.Fatal("accepted foreign checkout")
	}
	if err := os.Remove(Path(other)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(Path(root), Path(other)); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(other); err == nil {
		t.Fatal("followed symlink")
	}
	if err := DiscardInvalid(other, now); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root); err != nil {
		t.Fatalf("discard followed symlink: %v", err)
	}
}

func TestRequestReasonRemainsSmall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := Write(root, strings.Repeat("a\x1b\n", 1<<19), time.Now()); err != nil {
		t.Fatal(err)
	}
	r, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Reason) > 200 || strings.ContainsAny(r.Reason, "\x1b\n") {
		t.Fatalf("unbounded reason of %d bytes", len(r.Reason))
	}
	info, err := os.Stat(Path(root))
	if err != nil || info.Size() > 4096 {
		t.Fatalf("request is not small: %v", err)
	}
}
