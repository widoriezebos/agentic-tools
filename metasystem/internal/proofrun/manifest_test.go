package proofrun

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestManifestRecordsSymlinkExecutableAndRawSort(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "internal", "z-plain"), []byte("plain"), 0644)
	writeTestFile(t, filepath.Join(root, "internal", "a-exec"), []byte("exec"), 0755)
	writeTestFile(t, filepath.Join(root, "internal", "b-group-other-exec"), []byte("not owner executable"), 0611)
	if err := os.Symlink("z-plain", filepath.Join(root, "internal", "m-link")); err != nil {
		t.Fatal(err)
	}
	rawName := "~raw"
	writeTestFile(t, filepath.Join(root, "internal", rawName), []byte("raw"), 0644)

	m, err := readManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, len(m.entries))
	for i, item := range m.entries {
		paths[i] = item.path
	}
	wantOrder := []string{"internal/a-exec", "internal/b-group-other-exec", "internal/m-link", "internal/z-plain", "internal/" + rawName}
	if !reflect.DeepEqual(paths, wantOrder) {
		t.Fatalf("raw path order = %q, want %q", paths, wantOrder)
	}
	if !m.entries[0].executable || m.entries[0].kind != 'f' {
		t.Fatalf("executable file record lost its kind or executable bit: %+v", m.entries[0])
	}
	if m.entries[1].executable {
		t.Fatalf("group/other-only execute permissions set the normative executable bit: %+v", m.entries[1])
	}
	if m.entries[3].executable {
		t.Fatalf("plain file record acquired an executable bit: %+v", m.entries[3])
	}
	if m.entries[2].kind != 'l' || !bytes.Equal(m.entries[2].target, []byte("z-plain")) {
		t.Fatalf("symlink record = %+v, want raw target z-plain", m.entries[2])
	}
	if err := os.Chmod(filepath.Join(root, "internal", "b-group-other-exec"), 0600); err != nil {
		t.Fatal(err)
	}
	withoutGroupOtherExec, err := readManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if withoutGroupOtherExec.digest != m.digest {
		t.Fatalf("group/other-only execute permissions changed the manifest digest: %s != %s", withoutGroupOtherExec.digest, m.digest)
	}
}

func TestManifestLengthFramingAndDigest(t *testing.T) {
	root := t.TempDir()
	content := []byte("framed content")
	writeTestFile(t, filepath.Join(root, "internal", "one"), content, 0644)

	m, err := readManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.records) < recordLengthBytes {
		t.Fatalf("framed manifest is only %d bytes", len(m.records))
	}
	bodyLength := binary.BigEndian.Uint64(m.records[:recordLengthBytes])
	if int(bodyLength) != len(m.records)-recordLengthBytes {
		t.Fatalf("record length prefix = %d, body has %d bytes", bodyLength, len(m.records)-recordLengthBytes)
	}
	fileHash := sha256.Sum256(content)
	wantBody := append([]byte("internal/one\x00f\x00"), fileHash[:]...)
	if got := m.records[recordLengthBytes:]; !bytes.Equal(got, wantBody) {
		t.Fatalf("record body = %x, want %x", got, wantBody)
	}
	wantDigest := sha256.Sum256(m.records)
	if m.digest != stringHex(wantDigest[:]) {
		t.Fatalf("manifest digest = %s, want %x", m.digest, wantDigest)
	}
}

func TestManifestExcludesOnlyDeclaredRootClosures(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"artifacts/runtime", "bin/metasystem", ".git/index"} {
		writeTestFile(t, filepath.Join(root, path), []byte(path), 0644)
	}
	writeTestFile(t, filepath.Join(root, "internal/artifacts/input"), []byte("included"), 0644)

	m, err := readManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, len(m.entries))
	for i, item := range m.entries {
		paths[i] = item.path
	}
	// Directories are pruned only at the declared runtime roots; a
	// nested artifacts name inside a projection member is ordinary
	// content, and directories themselves are no longer entries.
	want := []string{"internal/artifacts/input"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("manifest paths = %q, want %q", paths, want)
	}
}

func TestFreezeExportsIdenticalProjection(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "internal/dir/plain"), []byte("plain"), 0644)
	writeTestFile(t, filepath.Join(root, "cmd/tool"), []byte("tool"), 0755)
	if err := os.Symlink("dir/plain", filepath.Join(root, "internal/link")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "artifacts/ignored"), []byte("runtime"), 0644)

	frozen, err := Freeze(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = frozen.Close() })
	if _, err := Verify(frozen.Root, frozen.Digest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(frozen.Root, "artifacts")); !os.IsNotExist(err) {
		t.Fatalf("excluded artifacts closure was exported: %v", err)
	}
	if target, err := os.Readlink(filepath.Join(frozen.Root, "internal/link")); err != nil || target != "dir/plain" {
		t.Fatalf("exported symlink target = %q, %v", target, err)
	}
}

func TestFreezeIncludesLiveGoalsInCompleteDigestWithoutGit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "internal", "source.go"), []byte("package internal\n"), 0o644)
	writeTestFile(t, filepath.Join(root, "plans", "goals", "active.md"), []byte("active goal\n"), 0o644)
	writeTestFile(t, filepath.Join(root, "plans", "goals", "backlog.md"), []byte("ledger root\n"), 0o644)
	writeTestFile(t, filepath.Join(root, "records", "goals", "done.md"), []byte("concluded goal\n"), 0o644)
	writeTestFile(t, filepath.Join(root, "artifacts", "runtime"), []byte("runtime\n"), 0o644)
	writeTestFile(t, filepath.Join(root, "metasystem.conf.local"), []byte("secret fixture\n"), 0o600)

	frozen, err := Freeze(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = frozen.Close() })
	for _, path := range []string{"plans/goals/active.md", "records/goals/done.md"} {
		if _, err := os.Stat(filepath.Join(frozen.Root, filepath.FromSlash(path))); err != nil {
			t.Fatalf("frozen goal %s: %v", path, err)
		}
	}
	for _, path := range []string{"plans/goals/backlog.md", "artifacts/runtime", "metasystem.conf.local"} {
		if _, err := os.Lstat(filepath.Join(frozen.Root, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Fatalf("excluded frozen input %s exists: %v", path, err)
		}
	}
	if _, err := Verify(frozen.Root, frozen.Digest); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "plans", "goals", "backlog.md"), []byte("changed ledger root\n"), 0o644)
	writeTestFile(t, filepath.Join(root, "artifacts", "runtime"), []byte("changed runtime\n"), 0o644)
	writeTestFile(t, filepath.Join(root, "metasystem.conf.local"), []byte("changed secret fixture\n"), 0o600)
	if _, err := Verify(root, frozen.Digest); err != nil {
		t.Fatalf("excluded state invalidated frozen digest: %v", err)
	}
	writeTestFile(t, filepath.Join(root, "plans", "goals", "active.md"), []byte("changed goal\n"), 0o644)
	if _, err := Verify(root, frozen.Digest); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("live goal mutation did not invalidate frozen digest: %v", err)
	}
}

func TestCompleteManifestSelectsOnlyStateRootGoalsWithoutGit(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, installationPrefix, stateRootPrefix, liveGoal, concludedGoal, wrongHome string
	}{
		{"self-hosted", "metasystem", "metasystem", "metasystem/plans/goals/live.md", "metasystem/records/goals/done.md", "plans/goals/wrong.md"},
		{"adopted", "tools/metasystem", "", "plans/goals/live.md", "records/goals/done.md", "tools/metasystem/plans/goals/wrong.md"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			for _, path := range []string{test.liveGoal, test.concludedGoal, test.wrongHome} {
				writeTestFile(t, filepath.Join(root, filepath.FromSlash(path)), []byte(path), 0o644)
			}
			goalsDir := filepath.Dir(test.liveGoal)
			for _, path := range []string{goalsDir + "/backlog.md", goalsDir + "/nested/deep.md", goalsDir + "/notes.txt"} {
				writeTestFile(t, filepath.Join(root, filepath.FromSlash(path)), []byte(path), 0o644)
			}
			m, err := readManifestAtWithHook(root, test.installationPrefix, test.stateRootPrefix, true, nil)
			if err != nil {
				t.Fatal(err)
			}
			paths := make(map[string]bool, len(m.entries))
			for _, item := range m.entries {
				paths[item.path] = true
			}
			for _, path := range []string{test.liveGoal, test.concludedGoal} {
				if !paths[path] {
					t.Errorf("state-root input %s omitted", path)
				}
			}
			for _, path := range []string{test.wrongHome, goalsDir + "/backlog.md", goalsDir + "/nested/deep.md", goalsDir + "/notes.txt"} {
				if paths[path] {
					t.Errorf("non-goal input %s entered complete manifest", path)
				}
			}
		})
	}
}

func TestFreezePreservesCompleteNestedProjectAndPrivateGit(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	installation := filepath.Join(project, "tools", "metasystem")
	writeTestFile(t, filepath.Join(project, ".gitattributes"), []byte("testing.json merge=metasystem-testing\n"), 0o644)
	writeTestFile(t, filepath.Join(project, ".claude", "agents", "review.md"), []byte("profile\n"), 0o644)
	writeTestFile(t, filepath.Join(project, "application", "old.txt"), []byte("parent input\n"), 0o644)
	writeTestFile(t, filepath.Join(project, "bin", "host-tool"), []byte("host input\n"), 0o755)
	writeTestFile(t, filepath.Join(installation, "go.mod"), []byte("module example.invalid/frozen\n"), 0o644)
	writeTestFile(t, filepath.Join(installation, "internal", "source.go"), []byte("package internal\n"), 0o644)
	runFreezeTestGit(t, project, "init", "-q", "-b", "main")
	runFreezeTestGit(t, project, "add", ".")
	runFreezeTestGit(t, project, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "base")

	if err := os.Rename(filepath.Join(project, "application", "old.txt"), filepath.Join(project, "application", "moved.txt")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(project, "application", "untracked.txt"), []byte("untracked parent\n"), 0o644)
	writeTestFile(t, filepath.Join(installation, "internal", "source.go"), []byte("package internal\nvar Dirty = true\n"), 0o644)
	for _, path := range []string{
		filepath.Join(project, "metasystem.conf.local"), filepath.Join(project, "nested", "metasystem.conf.local"),
		filepath.Join(installation, "metasystem.conf.local"), filepath.Join(installation, "nested", "metasystem.conf.local"),
	} {
		writeTestFile(t, path, []byte("secret fixture bytes\n"), 0o000)
	}
	for _, path := range []string{
		filepath.Join(project, "artifacts", "runtime"), filepath.Join(project, "plans", "goals", "backlog.md"),
		filepath.Join(installation, "artifacts", "runtime"), filepath.Join(installation, "bin", "metasystem"),
		filepath.Join(installation, "plans", "goals", "backlog.md"),
	} {
		writeTestFile(t, path, []byte("runtime or ledger\n"), 0o644)
	}
	writeTestFile(t, filepath.Join(project, "plans", "goals", "active.md"), []byte("active goal\n"), 0o644)
	writeTestFile(t, filepath.Join(project, "records", "goals", "concluded.md"), []byte("concluded goal\n"), 0o644)
	writeTestFile(t, filepath.Join(installation, "plans", "goals", "wrong.md"), []byte("wrong installation home\n"), 0o644)
	sourceHead := runFreezeTestGit(t, project, "rev-parse", "HEAD")
	sourceRefs := runFreezeTestGit(t, project, "for-each-ref", "--format=%(refname) %(objectname)")
	sourceStatus := runFreezeTestGit(t, project, "status", "--short", "--untracked-files=all")

	frozen, err := Freeze(installation)
	if err != nil {
		t.Fatal(err)
	}
	owner := filepath.Dir(frozen.SnapshotRoot)
	t.Cleanup(func() { _ = frozen.Close() })
	wantRoot := filepath.Join(frozen.ProjectRoot, "tools", "metasystem")
	if frozen.Root != wantRoot || frozen.SnapshotRoot != frozen.ProjectRoot {
		t.Fatalf("frozen roots = execution %q project %q snapshot %q, want %q beneath one project", frozen.Root, frozen.ProjectRoot, frozen.SnapshotRoot, wantRoot)
	}
	for path, want := range map[string]string{
		".gitattributes":                      "testing.json merge=metasystem-testing\n",
		".claude/agents/review.md":            "profile\n",
		"application/moved.txt":               "parent input\n",
		"application/untracked.txt":           "untracked parent\n",
		"bin/host-tool":                       "host input\n",
		"plans/goals/active.md":               "active goal\n",
		"records/goals/concluded.md":          "concluded goal\n",
		"tools/metasystem/internal/source.go": "package internal\nvar Dirty = true\n",
	} {
		data, readErr := os.ReadFile(filepath.Join(frozen.ProjectRoot, filepath.FromSlash(path)))
		if readErr != nil || string(data) != want {
			t.Fatalf("frozen project input %s = %q, %v; want %q", path, data, readErr, want)
		}
	}
	for _, path := range []string{
		"application/old.txt", "artifacts/runtime", "plans/goals/backlog.md", "metasystem.conf.local", "nested/metasystem.conf.local",
		"tools/metasystem/artifacts/runtime", "tools/metasystem/bin/metasystem", "tools/metasystem/plans/goals/backlog.md",
		"tools/metasystem/plans/goals/wrong.md",
		"tools/metasystem/metasystem.conf.local", "tools/metasystem/nested/metasystem.conf.local",
	} {
		if _, statErr := os.Lstat(filepath.Join(frozen.ProjectRoot, filepath.FromSlash(path))); !os.IsNotExist(statErr) {
			t.Fatalf("excluded frozen input %s exists: %v", path, statErr)
		}
	}
	if got := runFreezeTestGit(t, frozen.ProjectRoot, "rev-parse", "--is-inside-work-tree"); got != "true" {
		t.Fatalf("frozen project is not a Git worktree: %q", got)
	}
	if output, err := exec.Command("git", "-C", frozen.ProjectRoot, "symbolic-ref", "-q", "HEAD").CombinedOutput(); err == nil {
		t.Fatalf("frozen project HEAD remains symbolic: %s", output)
	} else if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("inspect frozen project detached HEAD: %v: %s", err, output)
	}
	if got := runFreezeTestGit(t, frozen.ProjectRoot, "rev-parse", "refs/heads/main"); got != sourceHead {
		t.Fatalf("private source branch moved from %s to %s", sourceHead, got)
	}
	origin := runFreezeTestGit(t, frozen.ProjectRoot, "remote", "get-url", "origin")
	if origin == project || !strings.HasPrefix(origin, owner+string(filepath.Separator)) {
		t.Fatalf("frozen origin %q is not private beneath %q", origin, owner)
	}
	runFreezeTestGit(t, frozen.ProjectRoot, "config", "fixture.private", "yes")
	command := exec.Command("git", "-C", project, "config", "--local", "--get", "fixture.private")
	if err := command.Run(); err == nil {
		t.Fatal("frozen Git configuration mutated source repository metadata")
	}
	if got := runFreezeTestGit(t, project, "for-each-ref", "--format=%(refname) %(objectname)"); got != sourceRefs {
		t.Fatalf("freeze changed source refs:\nbefore:\n%s\nafter:\n%s", sourceRefs, got)
	}
	if got := runFreezeTestGit(t, project, "status", "--short", "--untracked-files=all"); got != sourceStatus {
		t.Fatalf("freeze changed source worktree or index:\nbefore:\n%s\nafter:\n%s", sourceStatus, got)
	}
	if _, err := Verify(frozen.Root, frozen.Digest); err != nil {
		t.Fatalf("frozen project does not verify through its nested installation: %v", err)
	}
	writeTestFile(t, filepath.Join(project, "application", "untracked.txt"), []byte("moved after freeze\n"), 0o644)
	if _, err := Verify(installation, frozen.Digest); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("dirty parent input did not invalidate proof identity: %v", err)
	}
	if err := frozen.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(owner); !os.IsNotExist(err) {
		t.Fatalf("nested execution-root cleanup left owned snapshot %s: %v", owner, err)
	}

	t.Run("installation prefix is part of identity", func(t *testing.T) {
		project := t.TempDir()
		for _, prefix := range []string{"a", "b"} {
			writeTestFile(t, filepath.Join(project, prefix, "internal", "engine.go"), []byte("package engine\n"), 0o644)
		}
		runFreezeTestGit(t, project, "init", "-q", "-b", "main")
		runFreezeTestGit(t, project, "add", ".")
		runFreezeTestGit(t, project, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "prefixes")
		digests := map[string]string{}
		for _, prefix := range []string{"a", "b"} {
			digests[prefix] = freezeTestFullDigest(t, filepath.Join(project, prefix))
		}
		if digests["a"] == digests["b"] {
			t.Fatal("different executed installation prefixes shared a complete manifest")
		}
		if _, err := Verify(filepath.Join(project, "b"), digests["a"]); !errors.Is(err, ErrDigestMismatch) {
			t.Fatalf("installation b accepted installation a's witness: %v", err)
		}
	})

	t.Run("Git capability is part of identity", func(t *testing.T) {
		noGit, gitRoot := t.TempDir(), t.TempDir()
		for _, root := range []string{noGit, gitRoot} {
			writeTestFile(t, filepath.Join(root, "internal", "same.go"), []byte("package internal\n"), 0o644)
		}
		runFreezeTestGit(t, gitRoot, "init", "-q", "-b", "main")
		runFreezeTestGit(t, gitRoot, "add", ".")
		runFreezeTestGit(t, gitRoot, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "same bytes")
		noGitDigest, gitDigest := freezeTestFullDigest(t, noGit), freezeTestFullDigest(t, gitRoot)
		if noGitDigest == gitDigest {
			t.Fatal("Git and no-Git installations with the same source bytes shared a complete manifest")
		}
	})
}

func freezeTestFullDigest(t *testing.T, root string) string {
	t.Helper()
	digest, err := FullDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := Freeze(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(frozen.Root, digest); err != nil {
		_ = frozen.Close()
		t.Fatalf("own frozen export did not preserve complete identity for %s: %v", root, err)
	}
	if err := frozen.Close(); err != nil {
		t.Fatal(err)
	}
	return digest
}

func TestFreezeRefusesBeforeExportAfterMismatchAndCleans(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		hook func(source string) func(string)
	}{
		{name: "source moved", hook: func(source string) func(string) {
			return func(string) {
				writeTestFile(t, filepath.Join(source, "internal", "source.go"), []byte("after\n"), 0o644)
			}
		}},
		{name: "export moved", hook: func(string) func(string) {
			return func(export string) {
				writeTestFile(t, filepath.Join(export, "internal", "source.go"), []byte("export changed\n"), 0o644)
			}
		}},
		{name: "source goal moved", hook: func(source string) func(string) {
			return func(string) {
				writeTestFile(t, filepath.Join(source, "plans", "goals", "active.md"), []byte("after\n"), 0o644)
			}
		}},
		{name: "export goal moved", hook: func(string) func(string) {
			return func(export string) {
				writeTestFile(t, filepath.Join(export, "plans", "goals", "active.md"), []byte("export changed\n"), 0o644)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			temp := t.TempDir()
			source := filepath.Join(temp, "source")
			writeTestFile(t, filepath.Join(source, "internal", "source.go"), []byte("before\n"), 0o644)
			writeTestFile(t, filepath.Join(source, "plans", "goals", "active.md"), []byte("before\n"), 0o644)
			if _, err := freezeWithHookAt(source, temp, test.hook(source)); err == nil || !strings.Contains(err.Error(), "before") || !strings.Contains(err.Error(), "export") || !strings.Contains(err.Error(), "after") {
				t.Fatalf("freeze mismatch = %v", err)
			}
			matches, err := filepath.Glob(filepath.Join(temp, "metasystem-witness-freeze-*"))
			if err != nil || len(matches) != 0 {
				t.Fatalf("failed freeze left snapshots %q: %v", matches, err)
			}
		})
	}

	t.Run("excluded runtime closures are pruned before descent", func(t *testing.T) {
		root := t.TempDir()
		writeTestFile(t, filepath.Join(root, "application", "input.txt"), []byte("application\n"), 0o644)
		writeTestFile(t, filepath.Join(root, "artifacts", "runtime", "state"), []byte("runtime\n"), 0o644)
		if err := os.Chmod(filepath.Join(root, "artifacts", "runtime"), 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "artifacts", "runtime"), 0o755) })
		if _, _, err := readCompleteManifest(root); err != nil {
			t.Fatalf("inaccessible excluded runtime child invalidated complete input: %v", err)
		}
		if err := os.Chmod(filepath.Join(root, "artifacts", "runtime"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := readManifestAtWithHook(root, "", "", true, func(rel string, entry os.DirEntry) error {
			if rel == "artifacts" && entry.IsDir() {
				return os.RemoveAll(filepath.Join(root, "artifacts"))
			}
			return nil
		}); err != nil {
			t.Fatalf("removed excluded runtime closure invalidated complete input: %v", err)
		}
	})

	t.Run("ordinary application closures remain required", func(t *testing.T) {
		root := t.TempDir()
		writeTestFile(t, filepath.Join(root, "application", "input.txt"), []byte("application\n"), 0o644)
		_, err := readManifestAtWithHook(root, "", "", true, func(rel string, entry os.DirEntry) error {
			if rel == "application" && entry.IsDir() {
				return os.RemoveAll(filepath.Join(root, "application"))
			}
			return nil
		})
		if err == nil {
			t.Fatal("removed application input was pruned as runtime state")
		}
	})
}

func runFreezeTestGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func writeTestFile(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(path, content, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func stringHex(value []byte) string {
	const digits = "0123456789abcdef"
	result := make([]byte, len(value)*2)
	for i, b := range value {
		result[i*2] = digits[b>>4]
		result[i*2+1] = digits[b&0xf]
	}
	return string(result)
}
