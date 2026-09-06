package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func stewardVerbGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func TestHumanStewardVerbSeedsTheCheckedOutUpstreamOnce(t *testing.T) {
	root := t.TempDir()
	stewardVerbGit(t, root, "init", "-q", "-b", "release")
	stewardVerbGit(t, root, "config", "user.name", "fixture")
	stewardVerbGit(t, root, "config", "user.email", "fixture@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "README"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stewardVerbGit(t, root, "add", "README")
	stewardVerbGit(t, root, "commit", "-qm", "fixture")
	remote := filepath.Join(t.TempDir(), "origin.git")
	if err := os.Mkdir(remote, 0o755); err != nil {
		t.Fatal(err)
	}
	stewardVerbGit(t, remote, "init", "--bare", "-q")
	stewardVerbGit(t, root, "remote", "add", "origin", remote)
	stewardVerbGit(t, root, "push", "-q", "-u", "origin", "release")
	seeded, err := seedStewardLandingRef(root)
	if err != nil || seeded.Ref != "refs/remotes/origin/release" || seeded.NotSeeded != "" {
		t.Fatalf("checked-out upstream was not seeded: %+v %v", seeded, err)
	}
	if got := stewardVerbGit(t, root, "config", "--local", "--get", "metasystem.steward.landing-ref"); got != seeded.Ref {
		t.Fatalf("local landing ref = %q, want %q", got, seeded.Ref)
	}
	stewardVerbGit(t, root, "config", "--local", "metasystem.steward.landing-ref", "refs/remotes/fork/operator-choice")
	seeded, err = seedStewardLandingRef(root)
	if err != nil || seeded.Ref != "" || seeded.NotSeeded != "" {
		t.Fatalf("an existing local value was reseeded: %+v %v", seeded, err)
	}
	if got := stewardVerbGit(t, root, "config", "--local", "--get", "metasystem.steward.landing-ref"); got != "refs/remotes/fork/operator-choice" {
		t.Fatalf("existing operator choice changed to %q", got)
	}
}

func TestHumanStewardVerbDoesNotSeedABranchWithoutAnUpstream(t *testing.T) {
	root := t.TempDir()
	stewardVerbGit(t, root, "init", "-q", "-b", "release")
	seeded, err := seedStewardLandingRef(root)
	if err != nil || seeded.Ref != "" || !strings.Contains(seeded.NotSeeded, "branch release has no upstream") {
		t.Fatalf("branch without an upstream did not preserve human arming: %+v %v", seeded, err)
	}
	if _, err := exec.Command("git", "-C", root, "config", "--local", "--get", "metasystem.steward.landing-ref").Output(); err == nil {
		t.Fatal("branch without an upstream invented a landing ref")
	}
}

func TestHumanStewardVerbCannotInventABranchWhileDetached(t *testing.T) {
	root := t.TempDir()
	stewardVerbGit(t, root, "init", "-q", "-b", "release")
	stewardVerbGit(t, root, "config", "user.name", "fixture")
	stewardVerbGit(t, root, "config", "user.email", "fixture@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "README"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stewardVerbGit(t, root, "add", "README")
	stewardVerbGit(t, root, "commit", "-qm", "fixture")
	stewardVerbGit(t, root, "checkout", "--detach", "-q")
	seeded, err := seedStewardLandingRef(root)
	if err != nil || seeded.Ref != "" || !strings.Contains(seeded.NotSeeded, "checkout is detached") {
		t.Fatalf("detached checkout did not preserve human arming: %+v %v", seeded, err)
	}
	if _, err := exec.Command("git", "-C", root, "config", "--local", "--get", "metasystem.steward.landing-ref").Output(); err == nil {
		t.Fatal("detached checkout invented a landing ref")
	}
}
