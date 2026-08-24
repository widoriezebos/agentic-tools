package covenant

// The hardening rows: the one-open no-follow read, the contract
// grammars the battery must fit, and the protected-path denial the
// covenant net shares with the contract side.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func loadableDoc(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "taskrun-covenant.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func writeDoc(t *testing.T, doc map[string]any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), Filename)
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A symlink is never a covenant: the no-follow open refuses it in the
// same act that would have read it — there is no window where a shape
// check passes and a swapped link serves other bytes.
func TestLoadRefusesSymlink(t *testing.T) {
	real := writeDoc(t, loadableDoc(t))
	link := filepath.Join(t.TempDir(), Filename)
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(link); err == nil {
		t.Fatal("a symlinked covenant must refuse")
	}
	// The real file still loads: the refusal is the link, not the bytes.
	if _, err := Load(real); err != nil {
		t.Fatal(err)
	}
}

// A non-regular shape refuses on the held handle.
func TestLoadRefusesDirectory(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir); err == nil ||
		!strings.Contains(err.Error(), "regular file") {
		t.Fatalf("a directory must refuse by shape: %v", err)
	}
}

// The battery must fit the contract's grammars beyond non-emptiness:
// the metric forms a contract key, and the command must be carryable
// as a contract value.
func TestBatteryContractGrammars(t *testing.T) {
	doc := loadableDoc(t)
	doc["battery"].(map[string]any)["metric"] = "Tests_Passed"
	if _, err := Load(writeDoc(t, doc)); err == nil ||
		!strings.Contains(err.Error(), "metric grammar") {
		t.Fatalf("an out-of-grammar metric must refuse: %v", err)
	}
	doc = loadableDoc(t)
	doc["battery"].(map[string]any)["command"] = "line one\nline two"
	if _, err := Load(writeDoc(t, doc)); err == nil ||
		!strings.Contains(err.Error(), "single line") {
		t.Fatalf("a multi-line command must refuse: %v", err)
	}
	doc = loadableDoc(t)
	doc["battery"].(map[string]any)["command"] = "run\x00gate"
	if _, err := Load(writeDoc(t, doc)); err == nil ||
		!strings.Contains(err.Error(), "NUL") {
		t.Fatalf("a NUL in the command must refuse: %v", err)
	}
	doc = loadableDoc(t)
	doc["battery"].(map[string]any)["command"] = " bash gate.sh "
	if _, err := Load(writeDoc(t, doc)); err == nil ||
		!strings.Contains(err.Error(), "rejects padded values") {
		t.Fatalf("a padded command must refuse: %v", err)
	}
}

// The covenant net answers to the same protected-path denial as the
// contract's declaration: a wall-custodied path can never be an
// app-declared guardrail.
func TestGuardrailsRefuseProtectedPaths(t *testing.T) {
	for _, entry := range []string{"scripts/agents/gate.sh", "plans/goals/", "plans/known-issues.md"} {
		doc := loadableDoc(t)
		doc["guardrails"] = []any{entry}
		if _, err := Load(writeDoc(t, doc)); err == nil ||
			!strings.Contains(err.Error(), "protected path") {
			t.Fatalf("the protected path %q must refuse: %v", entry, err)
		}
	}
}

// A FIFO can neither hang the open nor pass the shape check: the
// non-blocking open succeeds and the held handle's shape refuses.
func TestLoadRefusesFIFOWithoutHanging(t *testing.T) {
	path := filepath.Join(t.TempDir(), Filename)
	if err := unix.Mkfifo(path, 0o644); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := Load(path)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("a FIFO must refuse by shape: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the FIFO open hung; the read must be non-blocking")
	}
}

// The symlink refusal names the rule, not an errno.
func TestSymlinkRefusalNamesTheRule(t *testing.T) {
	real := writeDoc(t, loadableDoc(t))
	link := filepath.Join(t.TempDir(), Filename)
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	_, err := Load(link)
	want := "covenant refused: " + link + " is a symlink, and a symlink is never a covenant — the one home holds the bytes themselves"
	if err == nil || err.Error() != want {
		t.Fatalf("the refusal must carry the whole symlink rule:\ngot:  %v\nwant: %s", err, want)
	}
}
