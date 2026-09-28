package stateroot

import (
	"errors"
	"path/filepath"
	"testing"
)

// EM-36: outside a repository the refusal carried "state root:" and Git's
// own "fatal:" line. It says the directory is not inside a Git repository;
// any other Git failure keeps Git's words, since they are the reason.
func TestRepositoryTopOutsideARepositorySaysSo(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	notRepository := func(commandRequest) ([]byte, error) {
		return []byte("fatal: not a git repository (or any of the parent directories): .git\n"), errors.New("exit status 128")
	}
	if _, err := repositoryTopWith(directory, nil, notRepository); err == nil || err.Error() != directory+" is not inside a Git repository" {
		t.Fatalf("outside a repository = %v", err)
	}
	dubious := func(commandRequest) ([]byte, error) {
		return []byte("fatal: detected dubious ownership in repository at '/x'\n"), errors.New("exit status 128")
	}
	if _, err := repositoryTopWith(directory, nil, dubious); err == nil || err.Error() != "cannot find the Git repository of "+directory+": fatal: detected dubious ownership in repository at '/x'" {
		t.Fatalf("other Git failure = %v", err)
	}
}

// EM-37: a binary that is not an installation's bin/metasystem was refused
// with a literal "<installation>" placeholder and no way forward.
func TestUninstalledExecutableRefusalNamesTheWayForward(t *testing.T) {
	t.Parallel()
	executable := filepath.Join(t.TempDir(), "ms")
	resolver := NewResolver(func(string) (string, error) { t.Fatal("unexpected Git lookup"); return "", nil }, func() (string, error) { return executable, nil })
	_, err := resolver.installationRoot()
	want := executable + " is not an installed metasystem (an installation runs its own bin/metasystem); run the installation's bin/metasystem instead"
	if err == nil || err.Error() != want {
		t.Fatalf("uninstalled executable = %v, want %q", err, want)
	}
}
