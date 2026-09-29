package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// TestLandingGuardOwnersYieldAtTheHelmWithoutATerminal drives the enrolled
// hook through a real git commit with no controlling terminal, as GitHub
// Desktop's git runs it: the production guard classifies the caller
// UNTRUSTED and refuses on main; once the person holds the helm the same
// commit is admitted with one yield line; a linked worktree of the seat
// still refuses and records nothing.
func TestLandingGuardOwnersYieldAtTheHelmWithoutATerminal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runReceiptGit(t, root, "init", "-q", "-b", "main")
	runReceiptGit(t, root, "config", "user.name", "helm fixture")
	runReceiptGit(t, root, "config", "user.email", "helm@example.invalid")
	// The suite's own agent ancestry is not an agent runtime here, and no
	// identity table stages the terminal: the kernel answers it.
	writeReceiptFixture(t, root, "metasystem.conf", "metasystem.runtimes=fake\n")
	writeReceiptFixture(t, root, "readme.txt", "first\n")
	runReceiptGit(t, root, "add", ".")
	runReceiptGit(t, root, "commit", "-qm", "initial")
	stub := "#!/usr/bin/env bash\nset -euo pipefail\n" + preCommitEngineDispatch(t) + "exit 1\n"
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(root, "bin", "metasystem"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(root, ".git", "hooks", "pre-commit"), []byte("#!/bin/sh\nexec \""+filepath.Join(root, "bin", "metasystem")+"\" internal pre-commit --root \""+root+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runReceiptGit(t, root, "add", "bin")
	// commit runs git commit in its own session: no controlling terminal.
	commit := func(dir, file string) (string, error) {
		t.Helper()
		writeReceiptFixture(t, dir, file, file+"\n")
		runReceiptGit(t, dir, "add", file)
		command := exec.Command("git", "-C", dir, "commit", "-qm", "commit "+file)
		command.Env = testenv.WithoutInheritedControls(gittree.ScrubbedEnviron())
		command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		out, err := command.CombinedOutput()
		return string(out), err
	}
	yields := filepath.Join(root, ".git", "metasystem", "helm-yields.log")

	head := runReceiptGit(t, root, "rev-parse", "HEAD")
	out, err := commit(root, "control.txt")
	if err == nil || !strings.Contains(out, "the live wrapper ancestry token is missing") || !strings.Contains(out, "refs/heads/main is the published line") {
		t.Fatalf("control commit without the helm: %v\n%s", err, out)
	}
	if after := runReceiptGit(t, root, "rev-parse", "HEAD"); after != head {
		t.Fatalf("the refused commit advanced HEAD to %s", after)
	}

	seat, err := helm.Locate(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := helm.Write(root, helm.Record{By: "wido", At: time.Now().UTC().Format(time.RFC3339), Reason: "e2e", Checkout: seat.Checkout}); err != nil {
		t.Fatal(err)
	}
	out, err = commit(root, "helm.txt")
	if err != nil {
		t.Fatalf("commit at the helm: %v\n%s", err, out)
	}
	if after := runReceiptGit(t, root, "rev-parse", "HEAD"); after == head {
		t.Fatalf("the admitted commit left HEAD at %s", head)
	}
	if strings.Count(out, "pre-commit guard: HUMAN AT THE HELM (wido): the wrapper-fence yields; recorded in ") != 1 || !strings.Contains(out, "helm-yields.log") {
		t.Fatalf("commit at the helm printed %q", out)
	}
	lines := yieldLines(t, yields)
	if len(lines) != 1 {
		t.Fatalf("yield lines %q, want one", lines)
	}
	var y helm.Yield
	if err := json.Unmarshal([]byte(lines[0]), &y); err != nil {
		t.Fatal(err)
	}
	if y.Boundary != "pre-commit" || y.Gate != "wrapper-fence" || y.Would != "refuse" || y.By != "wido" ||
		!strings.Contains(y.Subject, "branch=refs/heads/main") || !strings.Contains(y.Subject, "class=UNTRUSTED") {
		t.Fatalf("yield %+v", y)
	}

	// A linked worktree of the seat: the helm is active there too, but it is
	// not the checkout the person sits in.
	if err := os.MkdirAll(filepath.Join(root, "artifacts", "agents", "mains"), 0o755); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(t.TempDir(), "linked")
	runReceiptGit(t, root, "worktree", "add", "-q", "-b", "feature", linked)
	if !helm.Active(linked).Active {
		t.Fatal("the helm is not active in the linked worktree")
	}
	linkedHead := runReceiptGit(t, linked, "rev-parse", "HEAD")
	out, err = commit(linked, "linked.txt")
	if err == nil || !strings.Contains(out, "the live wrapper ancestry token is missing") || strings.Contains(out, "HUMAN AT THE HELM") {
		t.Fatalf("linked worktree commit at the helm: %v\n%s", err, out)
	}
	if after := runReceiptGit(t, linked, "rev-parse", "HEAD"); after != linkedHead {
		t.Fatalf("the refused linked commit advanced HEAD to %s", after)
	}
	if lines := yieldLines(t, yields); len(lines) != 1 {
		t.Fatalf("the linked worktree added a yield: %q", lines)
	}
}

func yieldLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(data) == 0 {
		return nil
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}
