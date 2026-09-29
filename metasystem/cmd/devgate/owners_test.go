package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// ownerTree writes a small module tree for the in-process owners to judge.
func ownerTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := testexec.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestDependencyRatchetPassesDeclaredScriptsAndNamesABannedInterpreter(t *testing.T) {
	t.Parallel()
	clean := ownerTree(t, map[string]string{"skills/fixture/ok.sh": "#!/usr/bin/env bash\ngit status\n"})
	if out, ok := dependencyRatchet(clean); !ok || out != "dependency ratchet passed\n" {
		t.Fatalf("clean tree: %v %q", ok, out)
	}
	banned := ownerTree(t, map[string]string{"optional-skills/fixture/bad.sh": "#!/usr/bin/env bash\necho x\nnode run.js\n"})
	if out, ok := dependencyRatchet(banned); ok || out != "dependency ratchet: banned interpreter node: optional-skills/fixture/bad.sh:3\n" {
		t.Fatalf("banned interpreter: %v %q", ok, out)
	}
	if out, ok := dependencyRatchet(t.TempDir()); !ok || out != "dependency ratchet passed\n" {
		t.Fatalf("a tree shipping no shell: %v %q", ok, out)
	}
	unreadable := ownerTree(t, map[string]string{"skills": "not a directory"})
	if out, ok := dependencyRatchet(unreadable); ok || !strings.Contains(out, "dependency audit shell root is not a directory") {
		t.Fatalf("a skills file: %v %q", ok, out)
	}
}

func TestParallelRatchetRefusesARaisedSerialCount(t *testing.T) {
	t.Parallel()
	source := "package fixture\n\nimport \"testing\"\n\nfunc TestSerial(t *testing.T) {}\n\nfunc TestParallel(t *testing.T) { t.Parallel() }\n"
	files := map[string]string{
		"go.mod":                           "module example.com/fixture\n\ngo 1.24\n",
		"internal/fixture/fixture_test.go": source,
		"testing-parallel-ratchet.json":    `{"packages":{"example.com/fixture/internal/fixture":1},"exempt":[]}` + "\n",
	}
	if out, ok := parallelRatchet(ownerTree(t, files)); !ok || out != "parallel ratchet passed\n" {
		t.Fatalf("at the ceiling: %v %q", ok, out)
	}
	files["testing-parallel-ratchet.json"] = `{"packages":{"example.com/fixture/internal/fixture":0},"exempt":[]}` + "\n"
	out, ok := parallelRatchet(ownerTree(t, files))
	if ok || out != "parallel ratchet: package example.com/fixture/internal/fixture test TestSerial at internal/fixture/fixture_test.go:5: recorded serial count 0, actual 1\n"+
		"PARALLEL_RATCHET_REFUSED: serial Go test count increased\n" {
		t.Fatalf("raised count: %v %q", ok, out)
	}
	delete(files, "testing-parallel-ratchet.json")
	if out, ok := parallelRatchet(ownerTree(t, files)); ok || !strings.HasPrefix(out, "PARALLEL_RATCHET_REFUSED: parallel ratchet baseline unreadable") {
		t.Fatalf("missing baseline: %v %q", ok, out)
	}
	files["testing-parallel-ratchet.json"] = `{"packages":{}}` + "\n"
	delete(files, "go.mod")
	if out, ok := parallelRatchet(ownerTree(t, files)); ok || !strings.HasPrefix(out, "PARALLEL_RATCHET_REFUSED: parallel ratchet module path unreadable") {
		t.Fatalf("missing go.mod: %v %q", ok, out)
	}
}

func TestInstallationAuditsRefuseATreeWithoutTheirSources(t *testing.T) {
	t.Parallel()
	empty := t.TempDir()
	if out, ok := hookStartExits(empty); ok || !strings.Contains(out, "hook start exit audit could not read") {
		t.Fatalf("hook start exits: %v %q", ok, out)
	}
	if out, ok := projectCheck(filepath.Join(empty, "missing")); ok || out == "" {
		t.Fatalf("project check on a missing tree: %v %q", ok, out)
	}
}

func TestExitStatusNamesAChildThatCouldNotStart(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		err  error
		want int
	}{
		{nil, 0},
		{exitCode(3), 3},
		{exitCode(-1), 1},
		{errors.New("exec: not found"), 127},
	} {
		if got := exitStatus(test.err); got != test.want {
			t.Fatalf("exitStatus(%v) = %d, want %d", test.err, got, test.want)
		}
	}
}
