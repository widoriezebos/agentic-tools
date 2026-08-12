package atomicfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteTextReplacesContentAndReportsDurable(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "deeper", "file.txt")
	durable, err := WriteText(path, "first\n", root)
	if err != nil || !durable {
		t.Fatalf("clean write: durable=%v err=%v", durable, err)
	}
	if data, _ := os.ReadFile(path); string(data) != "first\n" {
		t.Fatalf("got %q", data)
	}
	if durable, err := WriteText(path, "second\n", root); err != nil || !durable {
		t.Fatalf("replacement: durable=%v err=%v", durable, err)
	}
	if data, _ := os.ReadFile(path); string(data) != "second\n" {
		t.Fatalf("replacement got %q", data)
	}
}

// Pre-publication failure: no new bytes at the target, the prior content
// untouched, a non-nil error, and no temp residue.
func TestPrePublicationFailureLeavesEverythingAlone(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file.txt")
	WriteText(path, "original\n", root)
	blocked := filepath.Join(root, "blocked")
	os.MkdirAll(blocked, 0o755)
	durable, err := WriteText(blocked, "x", root)
	if err == nil || durable {
		t.Fatalf("writing over a directory must fail: durable=%v err=%v", durable, err)
	}
	if data, _ := os.ReadFile(path); string(data) != "original\n" {
		t.Fatalf("the untouched file changed: %q", data)
	}
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("temp residue: %s", entry.Name())
		}
	}
}

// Fault injection, post-publication: the directory sync AFTER the rename
// fails. The file must be visible with the NEW content, and the outcome must
// be committed-with-doubt — (false, nil) — never an error, because the
// transition already happened.
func TestPostPublicationSyncFailureIsCommittedWithDoubt(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file.txt")
	WriteText(path, "before\n", root)

	published := false
	original := syncDir
	syncDir = func(dir string) error {
		// Let the pre-publication chain succeed; fail the one that follows
		// the rename. The rename is what makes the new content visible, so
		// checking the file distinguishes the two phases exactly.
		if data, err := os.ReadFile(path); err == nil && string(data) == "after\n" {
			published = true
			return errors.New("injected: directory sync failed")
		}
		return original(dir)
	}
	defer func() { syncDir = original }()

	durable, err := WriteText(path, "after\n", root)
	if !published {
		t.Fatal("the injection never fired; the test proves nothing")
	}
	if err != nil {
		t.Fatalf("a COMMITTED transition was reported as a failure: %v", err)
	}
	if durable {
		t.Fatal("durability was claimed over a failed sync")
	}
	if data, _ := os.ReadFile(path); string(data) != "after\n" {
		t.Fatalf("the committed content is not visible: %q", data)
	}
}

// Fault injection, pre-publication: the directory CHAIN sync fails. Nothing
// may be published, and the caller must see a plain error.
func TestChainSyncFailureIsPrePublication(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "file.txt")
	original := syncDir
	syncDir = func(dir string) error { return errors.New("injected: chain sync failed") }
	defer func() { syncDir = original }()

	durable, err := WriteText(path, "never\n", root)
	if err == nil || durable {
		t.Fatalf("a failed chain sync must be a plain error: durable=%v err=%v", durable, err)
	}
	if !strings.Contains(err.Error(), "directory chain") {
		t.Fatalf("the failure does not name its cause: %v", err)
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Fatal("content was published despite a failed chain sync")
	}
}

// The two-call retry test (r7/R7-F1, r8/R8-F1): the first call fails its
// chain sync and publishes nothing; the RETRY must re-sync the chain — the
// directories are now visible, and a rule conditioned on "did this call
// create them" would skip the chain and wrongly report durable.
func TestRetryReSyncsTheChain(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "fresh", "deeper", "file.txt")
	original := syncDir
	failing := true
	var synced []string
	syncDir = func(dir string) error {
		if failing {
			return errors.New("injected: chain sync failed")
		}
		synced = append(synced, dir)
		return original(dir)
	}
	defer func() { syncDir = original }()

	if durable, err := WriteText(path, "x\n", root); err == nil || durable {
		t.Fatalf("first call must fail pre-publication: durable=%v err=%v", durable, err)
	}
	failing = false
	synced = nil
	durable, err := WriteText(path, "y\n", root)
	if err != nil || !durable {
		t.Fatalf("retry: durable=%v err=%v", durable, err)
	}
	// The chain — target, its parent, up to and INCLUDING the anchor — must
	// have been synced again even though the directories already existed.
	wantAnchor := false
	for _, dir := range synced {
		if dir == filepath.Clean(root) {
			wantAnchor = true
		}
	}
	if !wantAnchor {
		t.Fatalf("the retry did not re-sync the chain through its anchor: %v", synced)
	}
	if len(synced) < 3 {
		t.Fatalf("the chain is shorter than target..anchor: %v", synced)
	}
}

func TestChainBounds(t *testing.T) {
	// An empty anchor syncs only the target's own directory.
	if got := chain("/a/b/c", ""); len(got) != 1 || got[0] != "/a/b/c" {
		t.Fatalf("empty anchor: %v", got)
	}
	// The anchor is INCLUSIVE.
	got := chain("/a/b/c", "/a")
	want := []string{"/a/b/c", "/a/b", "/a"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	// An anchor that is not an ancestor cannot loop; the walk is bounded.
	if got := chain("/a/b", "/x/y"); len(got) == 0 || len(got) > 4 {
		t.Fatalf("unrelated anchor did not terminate sanely: %v", got)
	}
}
