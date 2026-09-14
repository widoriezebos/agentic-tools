package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDispatchBriefBoundsAdmission(t *testing.T) {
	must := func(t *testing.T, ok bool, format string, args ...any) {
		t.Helper()
		if !ok {
			t.Fatalf(format, args...)
		}
	}
	installRoot, _ := filepath.Abs(filepath.Join("..", ".."))
	repoRoot := filepath.Dir(installRoot)
	brief := filepath.Join(t.TempDir(), "brief.md")
	script, _ := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "dispatch.sh"))
	briefSource, _ := os.ReadFile(filepath.Join("..", "..", "internal", "dispatch", "brief.go"))
	run := func(t *testing.T, body string, extra ...string) (int, string, string) {
		must(t, os.WriteFile(brief, []byte(body), 0o600) == nil, "write brief")
		args := append([]string{"--brief", brief, "--root", installRoot}, extra...)
		return captureCommandOutput(t, true, true, func() int { return runDispatchBriefMode(args) })
	}
	t.Run("authority-only", func(t *testing.T) {
		flags := []string{"--authority-only", "--base-tree", repoRoot, "--disk-root", repoRoot}
		code, _, problem := run(t, "Boundary: []\nCeiling: 1", flags...)
		must(t, code == 0, "valid exit %d: %s", code, problem)
		code, _, problem = run(t, "Boundary: []", flags...)
		must(t, code != 0 && strings.Contains(problem, "BRIEF_BOUNDS_INVALID"), "partial exit %d: %s", code, problem)
	})
	for _, tc := range []struct {
		name, body string
		extra      []string
		want       string
	}{
		{"normal", "Working Mode: implement\nBoundary: []", nil, "BRIEF_BOUNDS_INVALID"},
		{"mode-stdout", "Working Mode: implement\nBoundary: []\nCeiling: 1", nil, "implement\n"}, {"mode-empty", "Working Mode:", nil, ""},
		{"authority-before-mode", "Boundary: []\nCeiling: 1\nRead metasystem/internal/u1b-missing", []string{"--base-tree", repoRoot, "--disk-root", repoRoot}, "u1b-missing"}, {"authority-before-mode-required", "Boundary: []\nCeiling: 1", []string{"--base-tree", repoRoot, "--disk-root", repoRoot}, ""},
		{"mode-only", "Working Mode: implement\nBoundary: []", []string{"--mode-only", "--root", filepath.Join(repoRoot, "absent")}, "implement\n"},
		{"mode-only-conflict", "Working Mode: implement", []string{"--mode-only", "--authority-only"}, "cannot be combined"}, {"mode-only-base-conflict", "Working Mode: implement", []string{"--mode-only", "--base-tree", repoRoot}, "cannot be combined"}, {"mode-only-disk-conflict", "Working Mode: implement", []string{"--mode-only", "--disk-root", repoRoot}, "cannot be combined"},
		{"root-prefix", "Working Mode: implement\nBoundary: [\"internal/dispatch/brief.go\"]\nCeiling: 1", nil, "BRIEF_BOUNDS_INVALID"},
		{"root-prefix-admitted", "Working Mode: implement\nBoundary: [\"metasystem/internal/dispatch/brief.go\"]\nCeiling: 1", nil, "implement\n"},
		{"headerless-no-prefix", "Working Mode: implement", []string{"--root", filepath.Join(repoRoot, "absent")}, "implement\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, problem := run(t, tc.body, tc.extra...)
			if tc.name == "normal" || tc.name == "mode-empty" || tc.name == "root-prefix" || strings.HasPrefix(tc.name, "authority-before-mode") || strings.Contains(tc.name, "conflict") {
				must(t, code != 0 && strings.Contains(problem, tc.want), "exit %d, stdout %q, stderr %q", code, out, problem)
			} else {
				must(t, code == 0 && out == tc.want && problem == "", "exit %d, stdout %q, stderr %q", code, out, problem)
			}
			must(t, tc.name != "mode-only" || strings.Contains(string(script), `job brief-mode --mode-only`), "early mode discovery does not use mode-only")
			must(t, tc.name != "root-prefix" || strings.Contains(string(script), `job brief-mode --authority-only --brief "$1" --root "$root"`), "authority admission does not pass root")
			must(t, tc.name != "mode-stdout" || strings.Count(string(briefSource), "os.ReadFile(briefPath)") == 3 && strings.Contains(string(briefSource), "validateBriefAuthority(admitted, bounds"), "public admission can reread authority bytes")
		})
	}
}

func TestDispatchBriefBoundsPrefixLookupOrdering(t *testing.T) {
	directory := t.TempDir()
	t.Chdir(directory)
	brief := filepath.Join(directory, "brief.md")
	for _, tc := range []struct {
		name, body, want string
		silent           bool
	}{
		{"mode-before-prefix", "Boundary: []\nCeiling: 1", "", true},
		{"partial-before-prefix", "Working Mode: implement\nBoundary: []", "BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary", false},
		{"malformed-before-prefix", "Working Mode: implement\nBoundary: bad\nCeiling: 1", "BRIEF_BOUNDS_INVALID: Boundary: expected a JSON array of paths", false},
		{"malformed-ceiling-before-prefix", "Working Mode: implement\nBoundary: []\nCeiling: -1", "BRIEF_BOUNDS_INVALID: Ceiling: expected a nonnegative decimal integer", false},
		{"invalid-member-before-prefix", "Working Mode: implement\nBoundary: [\"/abs\"]\nCeiling: 1", "BRIEF_BOUNDS_INVALID: Boundary: invalid path or pattern \"/abs\"", false},
		{"valid-pair-uses-prefix", "Working Mode: implement\nBoundary: []\nCeiling: 1", "brief admission cannot resolve installation prefix", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(brief, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			code, out, problem := captureCommandOutput(t, true, true, func() int {
				return runDispatchBriefMode([]string{"--brief", brief})
			})
			if code != 1 || out != "" || (tc.silent && problem != "") || (!tc.silent && (!strings.Contains(problem, tc.want) || strings.Contains(problem, "authority admission"))) {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, out, problem)
			}
		})
	}
}

func TestDispatchBriefBoundsFailureSuffix(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "dispatch.sh"))
	if err != nil || !strings.Contains(string(data), "brief headers are invalid; Working Mode must be filled and Boundary and Ceiling must appear together") {
		t.Fatalf("dispatch suffix missing: %v", err)
	}
}
