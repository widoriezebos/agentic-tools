package dispatch

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGitCritiqueSubjectFactsReadsCommit(t *testing.T) {
	repo := initReadSubjectRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "metasystem", "next.go"), []byte("package next\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitReadSubject(t, repo, "add", "metasystem/next.go")
	gitReadSubject(t, repo, "-c", "user.name=test", "-c", "user.email=test@example.invalid", "commit", "-qm", "next")
	commit := gitReadSubject(t, repo, "rev-parse", "HEAD")
	facts := gitCritiqueSubjectFacts{}
	paths, err := facts.ChangedPaths(repo, commit)
	if err != nil || !reflect.DeepEqual(paths, []string{"metasystem/next.go"}) {
		t.Fatalf("changed paths = %v, %v", paths, err)
	}
	tree, err := facts.CommitTree(repo, commit)
	if want := gitReadSubject(t, repo, "rev-parse", "HEAD^{tree}"); err != nil || tree != want {
		t.Fatalf("commit tree = %q, %v; want %q", tree, err, want)
	}
	first := gitReadSubject(t, repo, "rev-list", "--max-parents=0", "HEAD")
	if _, err := facts.ChangedPaths(repo, first); err == nil {
		t.Fatal("root commit had changed paths without a parent")
	}
	if _, err := facts.ChangedPaths(repo, strings.Repeat("f", 40)); err == nil {
		t.Fatal("unreadable commit had changed paths")
	}
	if _, err := facts.CommitTree(repo, strings.Repeat("f", 40)); err == nil {
		t.Fatal("unreadable commit had a tree")
	}
	if prefix, err := facts.InstallPrefix(repo); err != nil || prefix != "" {
		t.Fatalf("root install prefix = %q, %v", prefix, err)
	}
	if prefix, err := facts.InstallPrefix(filepath.Join(repo, "metasystem")); err != nil || prefix != "metasystem" {
		t.Fatalf("nested install prefix = %q, %v", prefix, err)
	}
}

func TestGitCritiqueSubjectFactsChecksArtifactAbsence(t *testing.T) {
	repo := initReadSubjectRepo(t)
	tree := gitReadSubject(t, repo, "rev-parse", "HEAD^{tree}")
	facts := gitCritiqueSubjectFacts{}
	if facts.ArtifactAbsent(repo, strings.Repeat("f", 40), "metasystem/page.md") {
		t.Fatal("invalid tree reported an absent artifact")
	}
	if facts.ArtifactAbsent(repo, tree, "metasystem/page.md") {
		t.Fatal("present artifact reported absent")
	}
	if !facts.ArtifactAbsent(repo, tree, "metasystem/new.go") {
		t.Fatal("missing artifact was not reported absent")
	}
}
