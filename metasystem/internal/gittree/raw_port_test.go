package gittree

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
)

type rawPortStep struct {
	dir, operation string
	args           []string
	stdin          []byte
	result         RawResult
	check          func(RawRequest)
}

func rawPortScript(t *testing.T, workspaceDir string, steps ...rawPortStep) func(RawRequest) RawResult {
	t.Helper()
	next := 0
	t.Cleanup(func() {
		if next != len(steps) {
			t.Errorf("raw Git operations consumed %d of %d", next, len(steps))
		}
	})
	return func(request RawRequest) RawResult {
		t.Helper()
		if next >= len(steps) {
			t.Fatalf("unexpected raw Git operation: %+v", request)
		}
		step := steps[next]
		next++
		if step.dir != "" && request.Dir != step.dir {
			t.Fatalf("raw directory = %q, want %q", request.Dir, step.dir)
		}
		if step.args != nil {
			wantArgs := append(append([]string{"-C", request.Dir}, configPins...), step.args...)
			if !reflect.DeepEqual(request.Args, wantArgs) {
				t.Fatalf("raw argv = %q, want %q", request.Args, wantArgs)
			}
		}
		if !bytes.Equal(request.Stdin, step.stdin) || (request.Stdin == nil) != (step.stdin == nil) {
			t.Fatalf("raw stdin = %q, want %q", request.Stdin, step.stdin)
		}
		if step.operation != "" && request.Operation != step.operation {
			t.Fatalf("raw operation = %q, want %q", request.Operation, step.operation)
		}
		if step.dir != "" {
			wantBound := boundedexec.Timeout(filepath.Join(workspaceDir, "metasystem.conf"), boundedexec.Local)
			if request.Timeout != wantBound {
				t.Fatalf("raw timeout = %+v, want %+v", request.Timeout, wantBound)
			}
		}
		if step.check != nil {
			step.check(request)
		} else if !reflect.DeepEqual(request.Env, ScrubbedEnviron()) {
			t.Fatalf("raw environment differs from scrubbed environment")
		}
		return step.result
	}
}

func rawPortIndex(t *testing.T, index *string) func(RawRequest) {
	t.Helper()
	return func(request RawRequest) {
		t.Helper()
		var path string
		for _, entry := range request.Env {
			if strings.HasPrefix(entry, "GIT_INDEX_FILE=") {
				path = strings.TrimPrefix(entry, "GIT_INDEX_FILE=")
			}
		}
		if path == "" {
			t.Fatal("missing isolated index")
		}
		if *index == "" {
			*index = path
		} else if path != *index {
			t.Fatalf("isolated index changed: %q, want %q", path, *index)
		}
		if !reflect.DeepEqual(request.Env, ScrubbedEnviron("GIT_INDEX_FILE="+path)) {
			t.Fatalf("raw environment did not scrub steering variables or retain only the isolated index")
		}
	}
}

func TestWorkspaceRawSourceNonzeroAndSpawnDistinct(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	spawn := errors.New("cannot start Git")
	base := strings.Repeat("a", 40)
	patch := []byte("exact patch\x00bytes")
	firstIndex, secondIndex, thirdIndex := "", "", ""
	w := Workspace{Dir: dir, RawSource: rawPortScript(t, dir,
		rawPortStep{dir: dir, operation: "git cat-file -t missing", args: []string{"cat-file", "-t", "missing"}, result: RawResult{Stderr: []byte("missing tree\n"), ExitCode: 128}},
		rawPortStep{dir: dir, operation: "git cat-file -t missing", args: []string{"cat-file", "-t", "missing"}, result: RawResult{Err: spawn}},
		rawPortStep{dir: dir, operation: "git merge-base --is-ancestor a b", args: []string{"merge-base", "--is-ancestor", "a", "b"}, result: RawResult{ExitCode: 1}},
		rawPortStep{dir: dir, operation: "git merge-base --is-ancestor a b", args: []string{"merge-base", "--is-ancestor", "a", "b"}, result: RawResult{Err: spawn}},
		rawPortStep{dir: dir, operation: "git status --porcelain", args: []string{"status", "--porcelain"}, result: RawResult{Stdout: []byte{'x', 0, 'y'}, Stderr: []byte("warning\n"), ExitCode: 2}},
		rawPortStep{dir: dir, operation: "git read-tree " + base, args: []string{"read-tree", base}, check: rawPortIndex(t, &firstIndex)},
		rawPortStep{dir: dir, operation: "git rev-parse --show-toplevel", args: []string{"rev-parse", "--show-toplevel"}, result: RawResult{Stdout: []byte(dir + "\n")}},
		rawPortStep{dir: dir, operation: "git apply --cached", args: []string{"apply", "--cached", "--binary", "--whitespace=nowarn", "-"}, stdin: patch, check: rawPortIndex(t, &firstIndex), result: RawResult{Stderr: []byte("patch rejected\n"), ExitCode: 1}},
		rawPortStep{dir: dir, operation: "git read-tree " + base, args: []string{"read-tree", base}, check: rawPortIndex(t, &secondIndex)},
		rawPortStep{dir: dir, operation: "git rev-parse --show-toplevel", args: []string{"rev-parse", "--show-toplevel"}, result: RawResult{Stdout: []byte(dir + "\n")}},
		rawPortStep{dir: dir, operation: "git apply --cached", args: []string{"apply", "--cached", "--binary", "--whitespace=nowarn", "-"}, stdin: patch, check: rawPortIndex(t, &secondIndex), result: RawResult{Err: spawn}},
		rawPortStep{dir: dir, operation: "git read-tree " + base, args: []string{"read-tree", base}, check: rawPortIndex(t, &thirdIndex)},
		rawPortStep{dir: dir, operation: "git rev-parse --show-toplevel", args: []string{"rev-parse", "--show-toplevel"}, result: RawResult{Stdout: []byte(dir + "\n")}},
		rawPortStep{dir: dir, operation: "git apply --cached", args: []string{"apply", "--cached", "--binary", "--whitespace=nowarn", "-"}, stdin: patch, check: rawPortIndex(t, &thirdIndex), result: RawResult{ExitCode: -1, ExitDetail: "signal: terminated"}},
	)}
	if _, err := w.git(nil, "cat-file", "-t", "missing"); err == nil || err.Error() != "git cat-file: missing tree" {
		t.Fatalf("executed nonzero = %v", err)
	} else {
		var failure *RunFailure
		if errors.As(err, &failure) {
			t.Fatalf("executed nonzero became a RunFailure: %v", err)
		}
	}
	if _, err := w.git(nil, "cat-file", "-t", "missing"); err == nil {
		t.Fatal("missing spawn failure")
	} else {
		var failure *RunFailure
		if !errors.As(err, &failure) || failure.Op != "cat-file" || !strings.Contains(err.Error(), spawn.Error()) {
			t.Fatalf("could-not-run mapping = %v", err)
		}
	}
	if yes, err := w.IsAncestor("a", "b"); yes || err != nil {
		t.Fatalf("executed ancestry refusal = %v, %v", yes, err)
	}
	if _, err := w.IsAncestor("a", "b"); err == nil || !errors.Is(err, spawn) {
		t.Fatalf("probe could-not-run mapping = %v", err)
	} else {
		var failure *RunFailure
		if !errors.As(err, &failure) || failure.Op != "merge-base" {
			t.Fatalf("probe failure type = %v", err)
		}
	}
	stdout, stderr, code, err := w.gitProbe(dir, nil, nil, "status", "--porcelain")
	if err != nil || code != 2 || !bytes.Equal([]byte(stdout), []byte{'x', 0, 'y'}) || stderr != "warning\n" {
		t.Fatalf("probe byte output = %q, %q, %d, %v", stdout, stderr, code, err)
	}
	if _, err := w.Apply(base, patch); err == nil || !strings.Contains(err.Error(), "patch does not apply exactly: patch rejected") {
		t.Fatalf("executed apply refusal = %v", err)
	} else {
		var failure *RunFailure
		if errors.As(err, &failure) {
			t.Fatalf("executed apply refusal became RunFailure: %v", err)
		}
	}
	if _, err := w.Apply(base, patch); err == nil || !errors.Is(err, spawn) {
		t.Fatalf("apply spawn failure = %v", err)
	} else {
		var failure *RunFailure
		if !errors.As(err, &failure) || failure.Op != "apply --cached" {
			t.Fatalf("apply failure type = %v", err)
		}
	}
	if _, err := w.Apply(base, patch); err == nil || !strings.Contains(err.Error(), "patch does not apply exactly: signal: terminated") {
		t.Fatalf("apply signal detail = %v", err)
	}
}

func TestWorkspaceRawSourceNativeSignalDetail(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\nkill -TERM $$\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	w := Workspace{Dir: dir}
	_, err := w.git(nil, "cat-file", "-t", "missing")
	if err == nil || !strings.Contains(err.Error(), "signal:") || strings.Contains(err.Error(), "exit status -1") {
		t.Fatalf("native signal detail = %v", err)
	}
	if got := rawCommandError(RawResult{ExitCode: -1, ExitDetail: "signal: terminated"}); got != "signal: terminated" {
		t.Fatalf("transfer signal detail = %q", got)
	}
}

func TestWorkspaceRawSourceInputEnvironmentAndInstances(t *testing.T) {
	t.Setenv("GIT_DIR", "/wrong/repository")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.useReplaceRefs")
	t.Setenv("GIT_CONFIG_VALUE_0", "true")
	first, second := t.TempDir(), t.TempDir()
	base, result := strings.Repeat("a", 40), strings.Repeat("b", 40)
	patch := []byte("diff --git a/file b/file\nindex 111..222\n")
	index := ""
	checkIndex := rawPortIndex(t, &index)
	w1 := Workspace{Dir: first, RawSource: rawPortScript(t, first,
		rawPortStep{dir: first, operation: "git read-tree " + base, args: []string{"read-tree", base}, check: checkIndex},
		rawPortStep{dir: first, operation: "git rev-parse --show-toplevel", args: []string{"rev-parse", "--show-toplevel"}, result: RawResult{Stdout: []byte(first + "\n")}},
		rawPortStep{dir: first, operation: "git apply --cached", args: []string{"apply", "--cached", "--binary", "--whitespace=nowarn", "-"}, stdin: patch, check: checkIndex},
		rawPortStep{dir: first, operation: "git write-tree", args: []string{"write-tree"}, check: checkIndex, result: RawResult{Stdout: []byte(result + "\n")}},
	)}
	w2 := Workspace{Dir: second, RawSource: rawPortScript(t, second,
		rawPortStep{dir: second, operation: "git cat-file blob object", args: []string{"cat-file", "blob", "object"}, result: RawResult{Stdout: []byte{'x', 0, 'y', '\n'}}},
	)}
	if out, err := w2.git(nil, "cat-file", "blob", "object"); err != nil || !bytes.Equal(out, []byte{'x', 0, 'y', '\n'}) {
		t.Fatalf("second workspace byte output = %q, %v", out, err)
	}
	if got, err := w1.Apply(base, patch); err != nil || got != result {
		t.Fatalf("first workspace applied tree = %q, %v", got, err)
	}
	if index == "" {
		t.Fatal("no isolated index observed")
	}
}

func TestTransferTreeClosureRawSourceAndDestination(t *testing.T) {
	t.Parallel()
	source, destination := t.TempDir(), t.TempDir()
	tree := strings.Repeat("c", 40)
	packBytes := []byte{'P', 'A', 'C', 'K', 0, 1, 2, 255}
	sourceWorkspace := Workspace{Dir: source, RawSource: rawPortScript(t, source,
		rawPortStep{dir: source, operation: "git cat-file -t " + tree, args: []string{"cat-file", "-t", tree}, result: RawResult{Stdout: []byte("tree\n")}},
		rawPortStep{dir: source, operation: "git rev-parse --show-toplevel", args: []string{"rev-parse", "--show-toplevel"}, result: RawResult{Stdout: []byte(source + "\n")}},
		rawPortStep{dir: source, operation: "git pack-objects", args: []string{"pack-objects", "--stdout", "--revs"}, stdin: []byte(tree + "\n"), result: RawResult{Stdout: packBytes}},
	)}
	destinationWorkspace := Workspace{Dir: destination, RawSource: rawPortScript(t, destination,
		rawPortStep{dir: destination, operation: "git rev-parse --show-toplevel", args: []string{"rev-parse", "--show-toplevel"}, result: RawResult{Stdout: []byte(destination + "\n")}},
		rawPortStep{dir: destination, operation: "git index-pack", args: []string{"index-pack", "--stdin"}, stdin: packBytes},
		rawPortStep{dir: destination, operation: "git cat-file -t " + tree, args: []string{"cat-file", "-t", tree}, result: RawResult{Stdout: []byte("tree\n")}},
		rawPortStep{dir: destination, operation: "git fsck --connectivity-only --no-dangling " + tree, args: []string{"fsck", "--connectivity-only", "--no-dangling", tree}},
	)}
	if err := sourceWorkspace.TransferTreeClosure(tree, destinationWorkspace); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(destination, ".metasystem-tree-closure-*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary pack remains: %v, %v", files, err)
	}
}

func TestDetachedWorkspaceRetainsRawSource(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	common := filepath.Join(repo, ".git")
	if err := os.Mkdir(common, 0o755); err != nil {
		t.Fatal(err)
	}
	tree, base, commit := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	var detachedTop string
	index := ""
	checkDetachedDir := func(r RawRequest) {
		t.Helper()
		if r.Dir != detachedTop {
			t.Fatalf("derived workspace dir = %q, want %q", r.Dir, detachedTop)
		}
		wantBound := boundedexec.Timeout(filepath.Join(detachedTop, "metasystem.conf"), boundedexec.Local)
		if r.Timeout != wantBound {
			t.Fatalf("derived timeout = %+v, want %+v", r.Timeout, wantBound)
		}
	}
	checkDetached := func(r RawRequest) {
		checkDetachedDir(r)
		if !reflect.DeepEqual(r.Env, ScrubbedEnviron()) {
			t.Fatal("derived workspace environment was not scrubbed")
		}
	}
	checkDetachedIndex := func(r RawRequest) {
		checkDetachedDir(r)
		rawPortIndex(t, &index)(r)
	}
	steps := []rawPortStep{
		{dir: repo, args: []string{"rev-parse", "--show-toplevel"}, operation: "git rev-parse --show-toplevel", result: RawResult{Stdout: []byte(repo + "\n")}},
		{dir: repo, args: []string{"rev-parse", "--show-prefix"}, operation: "git rev-parse --show-prefix"},
		{dir: repo, args: []string{"rev-parse", "--git-common-dir"}, operation: "git rev-parse --git-common-dir", result: RawResult{Stdout: []byte(common + "\n")}},
		{dir: repo, check: func(r RawRequest) {
			args := r.Args[2+len(configPins):]
			if len(args) != 5 || args[0] != "worktree" || args[1] != "add" || args[2] != "--detach" || args[4] != "HEAD" {
				t.Fatalf("worktree add argv = %q", args)
			}
			detachedTop = args[3]
			if r.Operation != "git worktree add --detach "+detachedTop+" HEAD" {
				t.Fatalf("worktree add operation = %q", r.Operation)
			}
		}},
		{args: []string{"read-tree", "--reset", "-u", tree}, operation: "git read-tree --reset -u " + tree, check: checkDetached},
		{args: []string{"rev-parse", "--verify", "HEAD^{commit}"}, operation: "git rev-parse --verify HEAD^{commit}", check: checkDetached, result: RawResult{Stdout: []byte(base + "\n")}},
		{args: []string{"-c", "user.name=MetaSystem", "-c", "user.email=metasystem@invalid", "commit-tree", tree, "-p", base, "-m", "temporary candidate snapshot"}, operation: "git -c user.name=MetaSystem -c user.email=metasystem@invalid commit-tree " + tree + " -p " + base + " -m temporary candidate snapshot", check: checkDetached, result: RawResult{Stdout: []byte(commit + "\n")}},
		{args: []string{"update-ref", "--no-deref", "HEAD", commit, base}, operation: "git update-ref --no-deref HEAD " + commit + " " + base, check: checkDetached},
		{args: []string{"rev-parse", "HEAD^{tree}"}, operation: "git rev-parse HEAD^{tree}", check: checkDetached, result: RawResult{Stdout: []byte(tree + "\n")}},
		{args: []string{"write-tree"}, operation: "git write-tree", check: checkDetached, result: RawResult{Stdout: []byte(tree + "\n")}},
		{args: []string{"diff-index", "--quiet", "HEAD", "--"}, operation: "git diff-index --quiet HEAD --", check: checkDetached},
		{args: []string{"cat-file", "-t", tree}, operation: "git cat-file -t " + tree, check: checkDetached, result: RawResult{Stdout: []byte("tree\n")}},
		{args: []string{"read-tree", "HEAD"}, operation: "git read-tree HEAD", check: checkDetachedIndex},
		{args: []string{"rm", "-r", "--cached", "-f", "--ignore-unmatch", "--", "ws"}, operation: "git rm -r --cached -f --ignore-unmatch -- ws", check: checkDetachedIndex},
		{args: []string{"read-tree", "--prefix=ws/", tree}, operation: "git read-tree --prefix=ws/ " + tree, check: checkDetachedIndex},
		{args: []string{"write-tree"}, operation: "git write-tree", check: checkDetachedIndex, result: RawResult{Stdout: []byte(tree + "\n")}},
		{args: []string{"-c", "merge.driver=fixture", "rebase", base}, operation: "git -c merge.driver=fixture rebase " + base, check: checkDetached},
		{args: []string{"rev-parse", "--verify", "--quiet", "HEAD^{commit}"}, operation: "git rev-parse --verify --quiet HEAD^{commit}", check: checkDetached, result: RawResult{Stdout: []byte(commit + "\n")}},
		{dir: repo, args: []string{"rev-parse", "--git-common-dir"}, operation: "git rev-parse --git-common-dir", result: RawResult{Stdout: []byte(common + "\n")}},
		{dir: repo, check: func(r RawRequest) {
			args := r.Args[2+len(configPins):]
			if !reflect.DeepEqual(args, []string{"worktree", "remove", "--force", "--force", detachedTop}) {
				t.Fatalf("worktree remove argv = %q", args)
			}
		}},
	}
	w := Workspace{Dir: repo, RawSource: rawPortScript(t, repo, steps...)}
	d, err := w.NewDetachedWorktree(tree)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := d.Workspace().gitLine(nil, "cat-file", "-t", tree); err != nil || got != "tree" {
		t.Fatalf("derived Workspace = %q, %v", got, err)
	}
	if got, err := d.graftSubtree("ws/", tree); err != nil || got != tree {
		t.Fatalf("derived graft = %q, %v", got, err)
	}
	if got, err := d.Rebase(base, "-c", "merge.driver=fixture"); err != nil || got.Head != commit {
		t.Fatalf("derived rebase = %+v, %v", got, err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTransferTreeClosureNativeAdapter(t *testing.T) {
	t.Parallel()
	source := newTreeFixture(t)
	destination := t.TempDir()
	runRawAdapterGit(t, destination, "init", "-q", "-b", "main")
	source.write("nested/blob.txt", "closure survives source removal\n")
	tree := source.snapshot()
	w := Workspace{Dir: destination}
	if err := source.w.TransferTreeClosure(tree, w); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(source.w.Dir); err != nil {
		t.Fatal(err)
	}
	got, ok, err := w.FileAt(tree, "nested/blob.txt")
	if err != nil || !ok || string(got) != "closure survives source removal\n" {
		t.Fatalf("destination blob = %q, present=%t, err=%v", got, ok, err)
	}
	if _, err := w.git(nil, "fsck", "--connectivity-only", "--no-dangling", tree); err != nil {
		t.Fatalf("destination tree closure: %v", err)
	}
}

func runRawAdapterGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = ScrubbedEnviron()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(fmt.Errorf("git %v: %w: %s", args, err, out))
	}
}
