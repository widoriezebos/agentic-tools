package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

func TestSTR3Tier1ReceiptProof06RefusesMismatchedIndex(t *testing.T) {
	root := t.TempDir()
	runReceiptGit(t, root, "init", "-q", "-b", "main")
	runReceiptGit(t, root, "config", "user.name", "receipt fixture")
	runReceiptGit(t, root, "config", "user.email", "receipt@example.invalid")
	writeReceiptFixture(t, root, "payload.txt", "base\n")
	runReceiptGit(t, root, "add", "payload.txt")
	runReceiptGit(t, root, "commit", "-qm", "base")

	writeReceiptFixture(t, root, "payload.txt", "candidate\n")
	runReceiptGit(t, root, "add", "payload.txt")
	candidate := runReceiptGit(t, root, "write-tree")

	// The supplied candidate remains a valid Git tree, but the real index is
	// moved before receipt creation. The command must not run and no stale or
	// newly labelled receipt may remain for the supplied tree.
	writeReceiptFixture(t, root, "payload.txt", "different index\n")
	runReceiptGit(t, root, "add", "payload.txt")
	if code := runLandingTestReceipt([]string{
		"--root", root,
		"--tree", candidate,
		"--command", "touch command-must-not-run",
	}); code == 0 {
		t.Fatal("receipt creation accepted a supplied tree that differed from the real index")
	}
	if _, err := os.Stat(filepath.Join(root, "command-must-not-run")); !os.IsNotExist(err) {
		t.Fatalf("test command ran despite the mismatched index: %v", err)
	}
	if _, err := os.Stat(landing.TestReceiptPath(root, candidate)); !os.IsNotExist(err) {
		t.Fatalf("receipt exists after mismatched-index refusal: %v", err)
	}
}

func TestLandingTestReceiptRefusesMismatchedWorkingTreeBeforeCommand(t *testing.T) {
	root := t.TempDir()
	runReceiptGit(t, root, "init", "-q", "-b", "main")
	runReceiptGit(t, root, "config", "user.name", "receipt fixture")
	runReceiptGit(t, root, "config", "user.email", "receipt@example.invalid")
	writeReceiptFixture(t, root, "payload.txt", "base\n")
	runReceiptGit(t, root, "add", "payload.txt")
	runReceiptGit(t, root, "commit", "-qm", "base")

	writeReceiptFixture(t, root, "payload.txt", "candidate\n")
	runReceiptGit(t, root, "add", "payload.txt")
	candidate := runReceiptGit(t, root, "write-tree")
	writeReceiptFixture(t, root, "payload.txt", "different working tree\n")

	if code := runLandingTestReceipt([]string{
		"--root", root,
		"--tree", candidate,
		"--command", "touch command-must-not-run",
	}); code == 0 {
		t.Fatal("receipt creation accepted a working tree that differed from the supplied tree")
	}
	if got := runReceiptGit(t, root, "write-tree"); got != candidate {
		t.Fatalf("fixture index tree moved: got %s, want %s", got, candidate)
	}
	if _, err := os.Stat(filepath.Join(root, "command-must-not-run")); !os.IsNotExist(err) {
		t.Fatalf("test command ran despite the mismatched working tree: %v", err)
	}
	if _, err := os.Stat(landing.TestReceiptPath(root, candidate)); !os.IsNotExist(err) {
		t.Fatalf("receipt exists after mismatched-working-tree refusal: %v", err)
	}
}

func TestLandingTestReceiptRefusesPostCommandTreeDrift(t *testing.T) {
	root := t.TempDir()
	runReceiptGit(t, root, "init", "-q", "-b", "main")
	runReceiptGit(t, root, "config", "user.name", "receipt fixture")
	runReceiptGit(t, root, "config", "user.email", "receipt@example.invalid")
	writeReceiptFixture(t, root, "payload.txt", "base\n")
	runReceiptGit(t, root, "add", "payload.txt")
	runReceiptGit(t, root, "commit", "-qm", "base")
	writeReceiptFixture(t, root, "payload.txt", "candidate\n")
	runReceiptGit(t, root, "add", "payload.txt")
	candidate := runReceiptGit(t, root, "write-tree")

	if code := runLandingTestReceipt([]string{
		"--root", root,
		"--tree", candidate,
		"--command", "printf 'drift\\n' > payload.txt",
	}); code == 0 {
		t.Fatal("receipt creation accepted a working tree changed by the command")
	}
	if _, err := os.Stat(landing.TestReceiptPath(root, candidate)); !os.IsNotExist(err) {
		t.Fatalf("receipt exists after post-command tree drift: %v", err)
	}
}

func runReceiptGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeReceiptFixture(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
