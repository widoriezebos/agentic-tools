package gittree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// recordedLayout builds one tuple's filesystem shape without Git.
func recordedLayout(t *testing.T, common, name string) WorktreeTuple {
	t.Helper()
	scratch, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(scratch, "wt-"+name)
	tuple := WorktreeTuple{Parent: parent, Top: filepath.Join(parent, "worktree-"+name), Control: filepath.Dir(common), Common: common}
	if err := os.MkdirAll(tuple.Top, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tuple.Top, "payload"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return tuple
}

func writeEntry(t *testing.T, common, base, gitdir string) string {
	t.Helper()
	entry := filepath.Join(common, "worktrees", base)
	if err := os.MkdirAll(entry, 0o700); err != nil {
		t.Fatal(err)
	}
	if gitdir != "" {
		if err := os.WriteFile(filepath.Join(entry, "gitdir"), []byte(gitdir+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(entry, "commondir"), []byte("../..\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return entry
}

func TestRemoveRecordedWorktreeClassifiesOnlyTheRecordedEntry(t *testing.T) {
	t.Parallel()
	control, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	common := filepath.Join(control, ".git")
	if err := os.MkdirAll(filepath.Join(common, "worktrees"), 0o700); err != nil {
		t.Fatal(err)
	}
	sibling := writeEntry(t, common, "worktree-sibling", "/elsewhere/.git")
	var removes []string
	stub := Workspace{RawSource: func(request RawRequest) RawResult {
		args := strings.Join(request.Args, " ")
		if !strings.Contains(args, "worktree remove --force --force") {
			t.Fatalf("unexpected git %s", args)
		}
		top := request.Args[len(request.Args)-1]
		removes = append(removes, top)
		_ = os.RemoveAll(filepath.Join(common, "worktrees", filepath.Base(top)))
		_ = os.RemoveAll(top)
		return RawResult{}
	}}

	reciprocal := recordedLayout(t, common, "reciprocal")
	entry := writeEntry(t, common, filepath.Base(reciprocal.Top), filepath.Join(reciprocal.Top, ".git"))
	if err := os.WriteFile(filepath.Join(reciprocal.Top, ".git"), []byte("gitdir: "+entry+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if action, err := RemoveRecordedWorktree(reciprocal, stub); action != WorktreeRemoved || err != nil {
		t.Fatalf("reciprocal = %s, %v", action, err)
	}
	if len(removes) != 1 || removes[0] != reciprocal.Top {
		t.Fatalf("worktree remove calls = %v", removes)
	}

	unregistered := recordedLayout(t, common, "unregistered")
	if action, err := RemoveRecordedWorktree(unregistered, stub); action != WorktreeRemoved || err != nil {
		t.Fatalf("unregistered = %s, %v", action, err)
	}

	// An add killed after the admin entry exists but before .git was
	// written is recovered only through its recorded tuple.
	interrupted := recordedLayout(t, common, "interrupted")
	interruptedEntry := writeEntry(t, common, filepath.Base(interrupted.Top), filepath.Join(interrupted.Top, ".git"))
	if action, err := RemoveRecordedWorktree(interrupted, stub); action != WorktreeRemoved || err != nil {
		t.Fatalf("interrupted = %s, %v", action, err)
	}
	if _, err := os.Lstat(interruptedEntry); !os.IsNotExist(err) {
		t.Fatalf("interrupted entry survived: %v", err)
	}

	foreign := recordedLayout(t, common, "foreign")
	if err := os.WriteFile(filepath.Join(foreign.Top, ".git"), []byte("gitdir: "+sibling+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if action, err := RemoveRecordedWorktree(foreign, stub); action != WorktreeRefused || err == nil {
		t.Fatalf("foreign = %s, %v", action, err)
	}
	if _, err := os.Stat(filepath.Join(foreign.Top, "payload")); err != nil {
		t.Fatalf("refused tuple lost its bytes: %v", err)
	}

	misnamed := recordedLayout(t, common, "misnamed")
	writeEntry(t, common, filepath.Base(misnamed.Top), "/another/top/.git")
	if action, _ := RemoveRecordedWorktree(misnamed, stub); action != WorktreeRefused {
		t.Fatalf("entry naming another top = %s", action)
	}

	for _, gone := range []string{reciprocal.Parent, unregistered.Parent, interrupted.Parent} {
		if _, err := os.Lstat(gone); !os.IsNotExist(err) {
			t.Fatalf("%s survived: %v", gone, err)
		}
	}
	if _, err := os.Stat(filepath.Join(sibling, "gitdir")); err != nil {
		t.Fatalf("unrecorded sibling entry touched: %v", err)
	}
	if len(removes) != 1 {
		t.Fatalf("git ran for a non-reciprocal tuple: %v", removes)
	}
}

// The native Git child receives the caller's inherited descriptor and the
// hooks-off pin: a fake git on PATH prints its arguments and reads a token
// from descriptor 3. The call is synchronous; the parent closes its file only
// after the child exited. Lock lifetime across a parent close is claimed by
// the scratch inherited-writer and custodian tests, not here.
func TestRunRawPassesInheritFilesAndHooksPathToTheNativeChild(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\"\ncat <&3\n"
	if err := testexec.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	tokenPath := filepath.Join(dir, "writer-token")
	if err := os.WriteFile(tokenPath, []byte("inherited-writer-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	token, err := os.Open(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	defer token.Close()
	hooks := filepath.Join(dir, "no-hooks")
	workspace := Workspace{Dir: dir, Materialize: &Materialization{InheritFiles: []*os.File{token}, HooksPath: hooks}}
	output, err := workspace.git(nil, "status")
	if err != nil {
		t.Fatalf("fake git with an inherited descriptor: %v", err)
	}
	if !strings.Contains(string(output), "core.hooksPath="+hooks) || !strings.HasSuffix(string(output), "inherited-writer-token\n") {
		t.Fatalf("fake git output = %q", output)
	}
	// Without InheritFiles the child must not see our writer's token. The
	// claim is about our file, not about descriptor 3 in general: the test
	// process or a shell may hold an unrelated, empty descriptor 3, so the
	// child may exit zero; an absent descriptor 3 fails the read instead,
	// and neither outcome is the defect. The second script reproduces the
	// benign case by opening /dev/null only when descriptor 3 is not already
	// open, so a leaked writer would still be read and caught.
	for _, script := range []string{
		"#!/bin/sh\nprintf '%s\\n' \"$*\"\ncat <&3\n",
		"#!/bin/sh\n{ : <&3; } 2>/dev/null || exec 3</dev/null\nprintf '%s\\n' \"$*\"\ncat <&3\n",
	} {
		if err := testexec.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		// Rewind the shared offset the positive child consumed, so a leak
		// would yield the token rather than an empty read.
		if _, err := token.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		bare := Workspace{Dir: dir, Materialize: &Materialization{HooksPath: hooks}}
		output, err := bare.git(nil, "status")
		if strings.Contains(string(output), "inherited-writer-token") {
			t.Fatalf("child read our writer without InheritFiles, script %q: %q", script, output)
		}
		if err == nil && !strings.Contains(string(output), "core.hooksPath="+hooks) {
			t.Fatalf("fake git without InheritFiles, script %q: output = %q", script, output)
		}
	}
}

// Adapter integration: real Git registration is the claim. A planned
// worktree runs with repository hooks off, registers exactly the recorded
// tuple, and a crashed owner's tuple is removed through the record alone.
func TestPlannedWorktreeRealGitRegistrationAndRecordedRecovery(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git adapter integration needs git")
	}
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		command := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@invalid"}, args...)...)
		command.Env = ScrubbedEnviron()
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return string(out)
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "file"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(repo, "hook-ran")
	hook := filepath.Join(repo, ".git", "hooks", "post-checkout")
	if err := testexec.WriteFile(hook, []byte("#!/bin/sh\ntouch "+sentinel+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	run("add", "file")
	run("commit", "-q", "-m", "one")
	tree := strings.TrimSpace(run("rev-parse", "HEAD^{tree}"))
	scratch, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	noHooks := filepath.Join(scratch, "no-hooks")
	if err := os.Mkdir(noHooks, 0o700); err != nil {
		t.Fatal(err)
	}
	workspace := Workspace{Dir: repo, Materialize: &Materialization{HooksPath: noHooks, TempDir: scratch}}
	plan, err := workspace.PlanDetachedWorktreeIn(scratch)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Common != filepath.Join(repo, ".git") || plan.Control != repo || filepath.Dir(plan.Top) != plan.Parent {
		t.Fatalf("plan tuple = %+v", plan.WorktreeTuple)
	}
	detached, err := plan.Create(tree)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("repository hook ran during materialization: %v", err)
	}
	if !strings.Contains(run("worktree", "list", "--porcelain"), plan.Top) {
		t.Fatal("planned worktree is not registered")
	}
	// The owner dies here: no Close. Recovery uses the recorded tuple.
	_ = detached
	if action, err := RemoveRecordedWorktree(plan.WorktreeTuple, Workspace{}); action != WorktreeRemoved || err != nil {
		t.Fatalf("recorded recovery = %s, %v", action, err)
	}
	if strings.Contains(run("worktree", "list", "--porcelain"), plan.Top) {
		t.Fatal("recovered worktree is still registered")
	}
	if _, err := os.Lstat(plan.Parent); !os.IsNotExist(err) {
		t.Fatalf("recovered parent survived: %v", err)
	}
}

// A worktree add that fails after writing its admin entry leaves the tuple
// unresolved: Close reports it, keeps the parent, and the recorded tuple
// alone recovers it.
func TestPlannedWorktreeFailedAddStaysUnresolvedUntilRecordedRecovery(t *testing.T) {
	t.Parallel()
	control, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	common := filepath.Join(control, ".git")
	if err := os.MkdirAll(filepath.Join(common, "worktrees"), 0o700); err != nil {
		t.Fatal(err)
	}
	stub := Workspace{Dir: control, RawSource: func(request RawRequest) RawResult {
		args := strings.Join(request.Args, " ")
		switch {
		case strings.HasSuffix(args, "rev-parse --show-toplevel"):
			return RawResult{Stdout: []byte(control + "\n")}
		case strings.HasSuffix(args, "rev-parse --show-prefix"):
			return RawResult{Stdout: []byte("\n")}
		case strings.HasSuffix(args, "rev-parse --git-common-dir"):
			return RawResult{Stdout: []byte(common + "\n")}
		case strings.Contains(args, "worktree add --detach"):
			top := request.Args[len(request.Args)-2]
			writeEntry(t, common, filepath.Base(top), filepath.Join(top, ".git"))
			return RawResult{ExitCode: 128, Stderr: []byte("fatal: interrupted")}
		}
		t.Errorf("unexpected git %s", args)
		return RawResult{ExitCode: 1}
	}}
	scratch := t.TempDir()
	plan, err := stub.PlanDetachedWorktreeIn(scratch)
	if err != nil {
		t.Fatal(err)
	}
	var closeResult error
	closed := false
	plan.AfterClose = func(err error) error { closed, closeResult = true, err; return nil }
	if _, err := plan.Create(strings.Repeat("a", 40)); err == nil || !strings.Contains(err.Error(), "unresolved") {
		t.Fatalf("failed add = %v", err)
	}
	if !closed || closeResult == nil {
		t.Fatalf("Close reported a failed add's registration as resolved: %v", closeResult)
	}
	entry := filepath.Join(common, "worktrees", filepath.Base(plan.Top))
	for _, kept := range []string{plan.Parent, entry} {
		if _, err := os.Lstat(kept); err != nil {
			t.Fatalf("%s: %v", kept, err)
		}
	}
	if action, err := RemoveRecordedWorktree(plan.WorktreeTuple, stub); action != WorktreeRemoved || err != nil {
		t.Fatalf("recorded recovery = %s, %v", action, err)
	}
	for _, gone := range []string{plan.Parent, entry} {
		if _, err := os.Lstat(gone); !os.IsNotExist(err) {
			t.Fatalf("%s survived: %v", gone, err)
		}
	}
}
