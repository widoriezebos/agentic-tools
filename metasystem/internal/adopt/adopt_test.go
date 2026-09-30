package adopt

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// fakeGit answers git calls from a table keyed by the argument words; an
// unexpected call fails the test. One instance per test.
type fakeGit struct {
	t       *testing.T
	answers map[string]fakeAnswer
	calls   []string
}

type fakeAnswer struct {
	out string
	err error
}

func newFakeGit(t *testing.T, answers map[string]fakeAnswer) *fakeGit {
	return &fakeGit{t: t, answers: answers}
}

func (g *fakeGit) run(dir string, scrub bool, args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	g.calls = append(g.calls, key)
	answer, ok := g.answers[key]
	if !ok {
		g.t.Errorf("unexpected git call in %s: %s", dir, key)
		return nil, errors.New("unexpected git call")
	}
	return []byte(answer.out), answer.err
}

func lookAll(string) (string, error) { return "/usr/bin/found", nil }

func TestSelectRuntimesRefusesBeforeAnyEffect(t *testing.T) {
	t.Parallel()
	adoptable := runtimes.Adoptable()
	if len(adoptable) == 0 {
		t.Fatal("no adoptable runtime is declared")
	}
	for _, valid := range []string{"none", adoptable[0], strings.Join(adoptable, ",")} {
		if _, refusal := SelectRuntimes(valid); refusal != nil {
			t.Fatalf("%q refused: %s", valid, refusal.Message)
		}
	}
	for input, want := range map[string]string{
		"":                                "cannot be empty",
		"codez":                           "unknown or non-adoptable runtime: codez",
		"fake":                            "unknown or non-adoptable runtime: fake",
		"none," + adoptable[0]:            "cannot be combined",
		adoptable[0] + "," + "none":       "cannot be combined",
		adoptable[0] + "," + adoptable[0]: "duplicate",
	} {
		_, refusal := SelectRuntimes(input)
		if refusal == nil || refusal.Code != CodeUsage || !strings.Contains(refusal.Message, want) {
			t.Fatalf("%q: refusal %+v, want %q", input, refusal, want)
		}
		if len(refusal.Argv) == 0 || refusal.Argv[0] != "metasystem" || !strings.Contains(refusal.Remedy, adoptable[0]) {
			t.Fatalf("%q: the refusal names no way forward: %+v", input, refusal)
		}
	}
}

func TestAdoptRefusesMissingToolsBeforeTouchingTheTarget(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "target")
	noGo := func(name string) (string, error) {
		if name == "go" {
			return "", errors.New("not found")
		}
		return "/usr/bin/" + name, nil
	}
	_, err := Adopt(Options{Source: t.TempDir(), Target: target, Deps: Deps{LookPath: noGo, Git: newFakeGit(t, nil).run}})
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.Code != CodeRefused || !strings.Contains(refusal.Message, "Go toolchain") || refusal.Remedy == "" {
		t.Fatalf("a machine without Go: %v", err)
	}
	onlyGo := func(name string) (string, error) {
		if name == "go" {
			return "/usr/bin/go", nil
		}
		return "", errors.New("not found")
	}
	_, err = Adopt(Options{Source: t.TempDir(), Target: target, Deps: Deps{LookPath: onlyGo, Git: newFakeGit(t, nil).run}})
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Message, "production commands") || len(refusal.Detail) == 0 || !strings.Contains(refusal.Detail[0], "package:") {
		t.Fatalf("a host without production commands: %v", err)
	}
	if exists(target) {
		t.Fatal("a tool refusal created the target")
	}
}

func templateAnswers(sha string) map[string]fakeAnswer {
	return map[string]fakeAnswer{
		"rev-parse --is-inside-work-tree": {out: "true\n"},
		"status --porcelain -- .":         {},
		"rev-parse HEAD":                  {out: sha + "\n"},
		"rev-parse --show-prefix":         {out: "\n"},
	}
}

func TestAdoptRefusesADirtyTemplateNamingTheCommand(t *testing.T) {
	t.Parallel()
	answers := templateAnswers(strings.Repeat("a", 40))
	answers["status --porcelain -- ."] = fakeAnswer{out: " M wow.md\n"}
	source, target := t.TempDir(), t.TempDir()
	_, err := Adopt(Options{Source: source, Target: target, Deps: Deps{LookPath: lookAll, Git: newFakeGit(t, answers).run}})
	var refusal *Refusal
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Message, "dirty") || strings.Join(refusal.Argv, " ") != "git -C "+source+" status" {
		t.Fatalf("a dirty template: %+v", err)
	}
}

func TestAdoptRefusesAnEngineFromAnotherCommitAfterBuilding(t *testing.T) {
	t.Parallel()
	sha := strings.Repeat("b", 40)
	answers := templateAnswers(sha)
	answers["rev-parse --is-inside-work-tree"] = fakeAnswer{out: "true\n"}
	answers["rev-parse --path-format=absolute --git-path hooks"] = fakeAnswer{out: "/nonexistent/hooks\n"}
	source, target := t.TempDir(), t.TempDir()
	built := 0
	options := Options{Source: source, Target: target, Runtimes: "none", Deps: Deps{LookPath: lookAll, Git: newFakeGit(t, answers).run,
		Build: func(string) error { built++; return nil }, EngineStamp: "dev-" + sha + "-dirty"}}
	_, err := Adopt(options)
	var refusal *Refusal
	if !errors.As(err, &refusal) || built != 1 || !strings.Contains(refusal.Message, "dev-"+sha+"-dirty") {
		t.Fatalf("a stale engine: built=%d %v", built, err)
	}
	resolved, _ := filepath.EvalSymlinks(target)
	want := []string{filepath.Join(source, "bin", "metasystem"), "system", "adopt", resolved, "--repo", source, "--runtimes", "none"}
	if strings.Join(refusal.Argv, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("the stale-engine refusal names %q, want %q", refusal.Argv, want)
	}
	if names(t, target) != "" {
		t.Fatal("the stale-engine refusal wrote the target")
	}
	options.Deps.Build = func(string) error { return errors.New("compile error") }
	options.Deps.Git = newFakeGit(t, answers).run
	if _, err := Adopt(options); !errors.As(err, &refusal) || !strings.Contains(refusal.Message, "compile error") || refusal.Argv[0] != "go" {
		t.Fatalf("a failed build: %v", err)
	}
}

func TestRecognizeClassifiesTheTarget(t *testing.T) {
	t.Parallel()
	sha := strings.Repeat("c", 40)
	write := func(t *testing.T, root, rel, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if already, refusal := recognize(t.TempDir(), sha); already || refusal != nil {
		t.Fatalf("an empty target: %v %+v", already, refusal)
	}
	for _, asset := range ForeignAssets {
		root := t.TempDir()
		write(t, root, asset, "x")
		if _, refusal := recognize(root, sha); refusal == nil || !strings.Contains(refusal.Message, asset) || !strings.Contains(strings.Join(refusal.Argv, " "), "metasystem-reconciliation.md") {
			t.Fatalf("foreign asset %s: %+v", asset, refusal)
		}
	}
	root := t.TempDir()
	write(t, root, workflowPath, "x")
	if _, refusal := recognize(root, sha); refusal == nil || !strings.Contains(refusal.Message, workflowPath) {
		t.Fatalf("an existing workflow: %+v", refusal)
	}
	other := t.TempDir()
	write(t, other, "wow.md", "x")
	write(t, other, "docs/project-rules.md", MarkerPrefix+" `"+strings.Repeat("d", 40)+"`.\n")
	if _, refusal := recognize(other, sha); refusal == nil || !strings.Contains(refusal.Message, "another template SHA") || !strings.Contains(refusal.Remedy, "upgrade") {
		t.Fatalf("an installation at another SHA: %+v", refusal)
	}
	incomplete := t.TempDir()
	write(t, incomplete, "wow.md", "x")
	write(t, incomplete, "docs/project-rules.md", MarkerPrefix+" `"+sha+"`.\n")
	if _, refusal := recognize(incomplete, sha); refusal == nil || !strings.Contains(refusal.Message, "half-finished installation") || !strings.Contains(strings.Join(refusal.Argv, " "), "project-adaptation.md") {
		t.Fatalf("an incomplete installation at the same SHA: %+v", refusal)
	}
}

func TestHookPreflightReadsWithoutExecuting(t *testing.T) {
	t.Parallel()
	hooks := t.TempDir()
	answers := map[string]fakeAnswer{
		"rev-parse --is-inside-work-tree":                   {out: "true\n"},
		"rev-parse --path-format=absolute --git-path hooks": {out: hooks + "\n"},
	}
	// The hooks write a marker if they are ever executed.
	marker := filepath.Join(t.TempDir(), "ran")
	body := "#!/bin/sh\ntouch " + marker + "\n"
	if err := writeExecutable(filepath.Join(hooks, "pre-commit"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	if refusal := hookPreflight(Deps{Git: newFakeGit(t, answers).run}, t.TempDir()); refusal != nil {
		t.Fatalf("one project hook is composable: %+v", refusal)
	}
	if err := writeExecutable(filepath.Join(hooks, "pre-commit.local"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	refusal := hookPreflight(Deps{Git: newFakeGit(t, answers).run}, t.TempDir())
	if refusal == nil || !strings.Contains(refusal.Remedy, "compose them by hand") {
		t.Fatalf("both hooks without the guard: %+v", refusal)
	}
	composer := "#!/usr/bin/env bash\nguard=\"$(git rev-parse --show-toplevel)/\"'scripts/agents/pre-commit-guard.sh'\n"
	if err := writeExecutable(filepath.Join(hooks, "pre-commit"), []byte(composer), 0o755); err != nil {
		t.Fatal(err)
	}
	if refusal := hookPreflight(Deps{Git: newFakeGit(t, answers).run}, t.TempDir()); refusal != nil {
		t.Fatalf("our retired composer beside a local hook is already enrolled: %+v", refusal)
	}
	// The engine composer (U5) is ours too.
	engineComposer := "#!/usr/bin/env bash\nprefix=''\ninstallation=\"$(git rev-parse --show-toplevel)/$prefix\"\n\"$engine\" internal pre-commit --root \"$installation\"\n"
	if err := writeExecutable(filepath.Join(hooks, "pre-commit"), []byte(engineComposer), 0o755); err != nil {
		t.Fatal(err)
	}
	if refusal := hookPreflight(Deps{Git: newFakeGit(t, answers).run}, t.TempDir()); refusal != nil {
		t.Fatalf("our engine composer beside a local hook is already enrolled: %+v", refusal)
	}
	if exists(marker) {
		t.Fatal("the preflight executed a hook")
	}
	// A git that ran and exited 128, as it does outside any repository.
	exited := exec.Command("sh", "-c", "exit 128").Run()
	notGit := map[string]fakeAnswer{"rev-parse --is-inside-work-tree": {err: fmt.Errorf("git rev-parse --is-inside-work-tree: %w", exited)}}
	if refusal := hookPreflight(Deps{Git: newFakeGit(t, notGit).run}, t.TempDir()); refusal != nil {
		t.Fatalf("a target without git: %+v", refusal)
	}
	// A repository is told by its .git on the filesystem, never by git's
	// words: a target that has one is a repository whatever git said, and
	// its failure is a malformed setup the preflight refuses.
	repository := t.TempDir()
	if err := os.Mkdir(filepath.Join(repository, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	broken := map[string]fakeAnswer{"rev-parse --is-inside-work-tree": {err: errors.New("fatal: bad config line 1")}}
	if refusal := hookPreflight(Deps{Git: newFakeGit(t, broken).run}, repository); refusal == nil || !strings.Contains(refusal.Message, "cannot be proven") {
		t.Fatalf("a malformed repository: %+v", refusal)
	}
	if refusal := hookPreflight(Deps{Git: newFakeGit(t, notGit).run}, repository); refusal == nil || !strings.Contains(refusal.Message, "cannot be proven") {
		t.Fatalf("a repository whose git said it is none: %+v", refusal)
	}
}

func TestSeedLandingRefDecisions(t *testing.T) {
	t.Parallel()
	const get = "config --local --no-includes --get metasystem.steward.landing-ref"
	const branch = "symbolic-ref --quiet --short HEAD"
	const upstream = "rev-parse --symbolic-full-name @{upstream}"
	missing := fakeAnswer{err: errors.New("exit status 1")}
	for _, test := range []struct {
		name    string
		answers map[string]fakeAnswer
		note    string
	}{
		{"kept", map[string]fakeAnswer{get: {out: "refs/remotes/kept/stable\n"}}, "landing ref was kept: metasystem.steward.landing-ref=refs/remotes/kept/stable"},
		{"detached", map[string]fakeAnswer{get: missing, branch: missing}, "the target checkout is detached"},
		{"no upstream", map[string]fakeAnswer{get: missing, branch: {out: "trunk\n"}, upstream: missing}, "target branch trunk has no upstream"},
		{"local upstream", map[string]fakeAnswer{get: missing, branch: {out: "trunk\n"}, upstream: {out: "refs/heads/base\n"}}, "has upstream refs/heads/base, not refs/remotes/<remote>/<branch>"},
		{"seeded", map[string]fakeAnswer{get: missing, branch: {out: "trunk\n"}, upstream: {out: "refs/remotes/origin/trunk\n"},
			"config --local metasystem.steward.landing-ref refs/remotes/origin/trunk": {}}, ""},
	} {
		git := newFakeGit(t, test.answers)
		notes := seedLandingRef(Deps{Git: git.run}, "/target")
		if strings.Join(notes, "\n") != "" && test.note == "" || test.note != "" && (len(notes) != 1 || !strings.Contains(notes[0], test.note)) {
			t.Fatalf("%s: notes %q, want %q", test.name, notes, test.note)
		}
	}
}

func TestGoalFreeLedgerPinsTheSeededScanDigest(t *testing.T) {
	t.Parallel()
	digest := sha256.Sum256([]byte("README.md"))
	want := "# Goals\n\n## Goal-free: declared 2026-09-27T12:00:00Z by human over " + hex.EncodeToString(digest[:]) + "\n"
	if got := GoalFreeLedger(time.Date(2026, 9, 27, 14, 0, 0, 0, time.FixedZone("CEST", 7200))); got != want {
		t.Fatalf("ledger = %q, want %q", got, want)
	}
}

func TestCollisionsNeverOverwriteAndSeedsOnlyOnce(t *testing.T) {
	t.Parallel()
	stage, target := t.TempDir(), t.TempDir()
	put := func(root, rel, content string) {
		if err := writeFile(filepath.Join(root, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put(stage, "docs/a.md", "payload\n")
	put(stage, "docs/same.md", "same\n")
	put(stage, ".gitignore", "artifacts/\nbin/\n")
	put(stage, "plans/goals.md", "seed\n")
	put(stage, "memory/rulings.md", "prepared\n")
	put(target, "docs/same.md", "same\n")
	put(target, ".gitignore", "node_modules/")
	put(target, "memory/rulings.md", "application rulings\n")
	if got := collide(stage, target); len(got) != 0 {
		t.Fatalf("identical files, merged files and the prepared rulings collided: %v", got)
	}
	put(target, "docs/a.md", "different\n")
	if got := collide(stage, target); strings.Join(got, ",") != "docs/a.md" {
		t.Fatalf("collisions = %v", got)
	}
	if err := os.Remove(filepath.Join(target, "docs", "a.md")); err != nil {
		t.Fatal(err)
	}
	put(target, "plans/goals.md", "the project's own ledger\n")
	if got := collide(stage, target); len(got) != 0 {
		t.Fatalf("the seed-once ledger collided: %v", got)
	}
	if err := copyPayload(stage, target); err != nil {
		t.Fatal(err)
	}
	if readText(t, filepath.Join(target, "plans", "goals.md")) != "the project's own ledger\n" {
		t.Fatal("a re-adoption rewrote the project's goal ledger")
	}
	if readText(t, filepath.Join(target, ".gitignore")) != "node_modules/\nartifacts/\nbin/\n" {
		t.Fatalf("merged .gitignore = %q", readText(t, filepath.Join(target, ".gitignore")))
	}
	if readText(t, filepath.Join(target, "memory", "rulings.md")) != "prepared\n" || readText(t, filepath.Join(target, "docs", "a.md")) != "payload\n" {
		t.Fatal("the payload was not copied")
	}
}

func TestExtractKeepsModesAndRefusesEscapes(t *testing.T) {
	t.Parallel()
	archive := func(entries ...tar.Header) []byte {
		var buffer bytes.Buffer
		writer := tar.NewWriter(&buffer)
		for _, header := range entries {
			header := header
			body := []byte("content of " + header.Name)
			if header.Typeflag == tar.TypeReg {
				header.Size = int64(len(body))
			}
			if err := writer.WriteHeader(&header); err != nil {
				t.Fatal(err)
			}
			if header.Typeflag == tar.TypeReg {
				if _, err := writer.Write(body); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		return buffer.Bytes()
	}
	dir := t.TempDir()
	data := archive(
		tar.Header{Name: "scripts/", Typeflag: tar.TypeDir, Mode: 0o755},
		tar.Header{Name: "scripts/tool.sh", Typeflag: tar.TypeReg, Mode: 0o755},
		tar.Header{Name: "benchmark/grader.py", Typeflag: tar.TypeReg, Mode: 0o644},
		tar.Header{Name: "scripts/link", Typeflag: tar.TypeSymlink, Linkname: "tool.sh"},
	)
	if err := extract(data, dir, func(name string) bool { return !strings.HasPrefix(name, "benchmark") }); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "scripts", "tool.sh"))
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("the executable bit was lost: %v %v", info, err)
	}
	if exists(filepath.Join(dir, "benchmark")) || exists(filepath.Join(dir, "scripts", "link")) {
		t.Fatal("a filtered entry or a symbolic link was extracted")
	}
	if err := extract(archive(tar.Header{Name: "../escape", Typeflag: tar.TypeReg, Mode: 0o644}), dir, func(string) bool { return true }); err == nil {
		t.Fatal("an escaping archive entry was accepted")
	}
}

func TestAppendMissingLinesKeepsExistingBytes(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), ".gitignore")
	if err := appendMissingLines(path, nil); err != nil || readText(t, path) != "" {
		t.Fatalf("a missing file is created empty: %v", err)
	}
	if err := os.WriteFile(path, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := appendMissingLines(path, []string{"a", "", "b", "b"}); err != nil {
		t.Fatal(err)
	}
	if got := readText(t, path); got != "a\nb\n" {
		t.Fatalf("got %q", got)
	}
	if err := appendMissingLines(path, []string{"b"}); err != nil || readText(t, path) != "a\nb\n" {
		t.Fatalf("a present line was appended again: %v", err)
	}
}
