package adopt

// Git integration tests. Adoption's claims are about git itself: the payload
// is exactly what `git archive` exports from the template's tracked HEAD
// (including from a template vendored below its git toplevel), the hook is
// installed where git resolves hooks and runs as git runs it, and the landing
// ref comes from the branch's upstream. A stub cannot prove what git exports
// or which hook git runs, so these tests drive real git against a committed
// snapshot of this module's working tree. Everything else is stubbed in
// adopt_test.go.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/audit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostsetup"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ledgerfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

type snapshot struct {
	root string // the template installation adopted from
	top  string // its git toplevel
	sha  string
}

var shared struct {
	once        sync.Once
	dir         string
	flat        snapshot
	nested      snapshot
	err         error
	engineOnce  sync.Once
	engine      string
	engineError error
}

var fixedNow = func() time.Time { return time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC) }

// testGenesis is the genesis the command layer performs, with the caller a
// non-holder machine session admitted under the genesis mode: adoption must
// work from agent ancestry, and the store re-judges the adoption shape.
func testGenesis(target string) error {
	if err := ledgerfence.Ensure(target); err != nil {
		return err
	}
	_, err := (&goal.Store{Root: target}).Reconcile(goal.Caller{Class: "MAIN", Holder: false, Genesis: true})
	return err
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitCommand(dir, args...)
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimRight(out, "\n")
}

func gitCommand(dir string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-c", "user.name=adoption fixture", "-c", "user.email=fixture@example.invalid", "-c", "init.defaultBranch=main", "-C", dir}, args...)...)
	command.Env = ledgerfence.EnvironWithoutGitSteering()
	out, err := command.CombinedOutput()
	return string(out), err
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// copyModule links this module's working tree without runtime state: the
// snapshot is the implementation under review, never a clone of HEAD. The
// template's concluded history (plans/, records/) is represented by a few
// entries only: adoption drops it, and the tests prove it is dropped.
func copyModule(from, to string) error {
	history := map[string]bool{"plans/README.md": true, "plans/designs/verbs-object-action.md": true,
		"records/misc/fleet-coordinator-brain-role-packet.md": true, "records/README.md": true}
	return filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		slashed := filepath.ToSlash(rel)
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "artifacts", "node_modules":
				return filepath.SkipDir
			}
			if rel == "bin" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(to, rel), 0o755)
		}
		if !entry.Type().IsRegular() || rel == "metasystem" || rel == "metasystem.conf.local" {
			return nil
		}
		if first, _, _ := strings.Cut(slashed, "/"); (first == "plans" || first == "records") && !history[slashed] {
			return nil
		}
		destination := filepath.Join(to, rel)
		if os.Link(path, destination) == nil {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return testexec.WriteFile(destination, data, info.Mode().Perm())
	})
}

func commitSnapshot(top string) (string, error) {
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"config", "metasystem.goal.machine", "fixture-machine"}, {"add", "-A"}, {"commit", "-qm", "snapshot"}} {
		if out, err := gitCommand(top, args...); err != nil {
			return "", fmt.Errorf("git %v: %v: %s", args, err, out)
		}
	}
	sha, err := gitCommand(top, "rev-parse", "HEAD")
	return strings.TrimSpace(sha), err
}

// templates returns the flat snapshot (the template at its repository root)
// and the vendored one (the template one level below its git toplevel, the
// real repository's own layout).
func templates(t *testing.T) (snapshot, snapshot) {
	t.Helper()
	module := moduleRoot(t)
	shared.once.Do(func() {
		dir, err := os.MkdirTemp("", "adopt-templates-")
		if err != nil {
			shared.err = err
			return
		}
		shared.dir = dir
		flat := filepath.Join(dir, "flat")
		if shared.err = copyModule(module, flat); shared.err != nil {
			return
		}
		// Ignored content must never ride along into the payload.
		if shared.err = appendMissingLines(filepath.Join(flat, ".gitignore"), []string{"ignored-fixture.txt"}); shared.err != nil {
			return
		}
		if shared.err = os.WriteFile(filepath.Join(flat, "ignored-fixture.txt"), []byte("junk\n"), 0o644); shared.err != nil {
			return
		}
		if shared.flat.sha, shared.err = commitSnapshot(flat); shared.err != nil {
			return
		}
		shared.flat.root, shared.flat.top = flat, flat
		// The vendored layout reads the flat snapshot's objects: the same
		// tree, checked out one level below a new toplevel.
		nestedTop := filepath.Join(dir, "nested")
		nested := filepath.Join(nestedTop, "vendored")
		for _, args := range [][]string{{"init", "-q", "-b", "main", nestedTop}} {
			if out, err := gitCommand(dir, args...); err != nil {
				shared.err = fmt.Errorf("git %v: %v: %s", args, err, out)
				return
			}
		}
		if shared.err = os.WriteFile(filepath.Join(nestedTop, ".git", "objects", "info", "alternates"), []byte(filepath.Join(flat, ".git", "objects")+"\n"), 0o644); shared.err != nil {
			return
		}
		for _, args := range [][]string{{"config", "metasystem.goal.machine", "fixture-machine"}, {"read-tree", "--prefix=vendored/", "-u", shared.flat.sha + "^{tree}"}, {"commit", "-qm", "nested"}} {
			if out, err := gitCommand(nestedTop, args...); err != nil {
				shared.err = fmt.Errorf("git %v: %v: %s", args, err, out)
				return
			}
		}
		out, err := gitCommand(nestedTop, "rev-parse", "HEAD")
		if shared.err = err; err != nil {
			return
		}
		shared.nested.sha = strings.TrimSpace(out)
		shared.nested.root, shared.nested.top = nested, nestedTop
		for _, root := range []string{flat, nested} {
			if shared.err = os.MkdirAll(filepath.Join(root, "bin"), 0o755); shared.err != nil {
				return
			}
			if shared.err = testexec.WriteFile(filepath.Join(root, "bin", "metasystem"), []byte("#!/bin/sh\nexit 0\n"), 0o755); shared.err != nil {
				return
			}
		}
	})
	if shared.err != nil {
		t.Fatalf("template snapshots: %v", shared.err)
	}
	return shared.flat, shared.nested
}

// realEngine builds this module's engine once, for the tests that run the
// adopted target's own binary.
func realEngine(t *testing.T) string {
	t.Helper()
	templates(t)
	shared.engineOnce.Do(func() {
		shared.engine = filepath.Join(shared.dir, "engine", "metasystem")
		build := exec.Command("go", "build", "-buildvcs=false", "-o", shared.engine, "./cmd/metasystem")
		build.Dir = moduleRoot(t)
		if out, err := build.CombinedOutput(); err != nil {
			shared.engineError = fmt.Errorf("%v: %s", err, out)
		}
	})
	if shared.engineError != nil {
		t.Fatalf("build the engine: %v", shared.engineError)
	}
	return shared.engine
}

// cloneTemplate is a template variant: a clone of source with its own
// commits, for the tier-tailoring and dirty-template legs.
func cloneTemplate(t *testing.T, source snapshot) snapshot {
	t.Helper()
	clone := filepath.Join(t.TempDir(), "template")
	gitIn(t, filepath.Dir(clone), "clone", "-q", source.top, clone)
	gitIn(t, clone, "config", "metasystem.goal.machine", "fixture-machine")
	if err := os.MkdirAll(filepath.Join(clone, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(clone, "bin", "metasystem"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return snapshot{root: clone, top: clone, sha: gitIn(t, clone, "rev-parse", "HEAD")}
}

func optionsFor(source snapshot, target string, output io.Writer) Options {
	return Options{Source: source.root, Target: target, Stdout: output, Stderr: output,
		Deps: Deps{EngineStamp: source.sha, Build: func(string) error { return nil }, Genesis: testGenesis, Now: fixedNow}}
}

func adoptWith(t *testing.T, options Options) Result {
	t.Helper()
	result, err := Adopt(options)
	if err != nil {
		var refusal *Refusal
		if errors.As(err, &refusal) {
			t.Fatalf("adoption into %s refused: %s %v (%s)", options.Target, refusal.Message, refusal.Detail, refusal.Remedy)
		}
		t.Fatalf("adoption into %s failed: %v", options.Target, err)
	}
	return result
}

func refusedWith(t *testing.T, options Options, want string) *Refusal {
	t.Helper()
	_, err := Adopt(options)
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("adoption into %s was not refused: %v", options.Target, err)
	}
	text := refusal.Message + "\n" + strings.Join(refusal.Detail, "\n") + "\n" + refusal.Remedy
	if !strings.Contains(text, want) {
		t.Fatalf("refusal does not say %q:\n%s", want, text)
	}
	if refusal.Remedy == "" {
		t.Fatalf("refusal %q names no way forward", refusal.Message)
	}
	return refusal
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func confLines(t *testing.T, target string) []string {
	return strings.Split(readText(t, filepath.Join(target, "metasystem.conf")), "\n")
}

func hasLine(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}

func names(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, entry := range entries {
		out = append(out, entry.Name())
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

// assertShipsNoTemplateProjectState holds what an application must never
// inherit from the template: this repository's own project homes (intent,
// doctrine, decisions) and history under docs/ (the journey, the reviews,
// the Stop-surface declarations of its own goals), the goals migration
// manifest of its own ledger, and a pointer at the template's local
// development rules in any instruction file. The Stop-surface protocol ships
// without a declaration.
func assertShipsNoTemplateProjectState(t *testing.T, target string) {
	t.Helper()
	for _, rel := range []string{"docs/intent", "docs/doctrine", "docs/decisions", "docs/journey.md", "docs/reviews",
		"records/misc/goals-migration-manifest.md"} {
		if exists(filepath.Join(target, filepath.FromSlash(rel))) {
			t.Errorf("adoption shipped the template's own %s", rel)
		}
	}
	if got := names(t, filepath.Join(target, "docs", "stop-decision-moves")); got != "README.md" {
		t.Errorf("docs/stop-decision-moves/ carries %q, want the protocol README alone", got)
	}
	if !exists(filepath.Join(target, "docs", "project-rules.md")) || !exists(filepath.Join(target, "docs", "orchestration.md")) {
		t.Error("adoption dropped the shipped documentation with the template's project state")
	}
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		path := filepath.Join(target, name)
		if exists(path) && strings.Contains(readText(t, path), "development/project-rules-local.md") {
			t.Errorf("%s points at the template's local development rules", name)
		}
	}
}

// treeState is every entry below root: symlink targets and file bytes.
func treeState(t *testing.T, root string) map[string]string {
	t.Helper()
	state := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		switch {
		case entry.Type()&fs.ModeSymlink != 0:
			link, linkErr := os.Readlink(path)
			if linkErr != nil {
				return linkErr
			}
			state[rel] = "link:" + link
		case entry.IsDir():
			state[rel] = "dir"
		default:
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			info, _ := entry.Info()
			state[rel] = fmt.Sprintf("file:%o:%s", info.Mode().Perm(), data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func sameTree(t *testing.T, before, after map[string]string, what string) {
	t.Helper()
	for rel, value := range before {
		if after[rel] != value {
			t.Fatalf("%s changed %s", what, rel)
		}
	}
	for rel := range after {
		if _, ok := before[rel]; !ok {
			t.Fatalf("%s added %s", what, rel)
		}
	}
}

func TestAdoptGitIntegrationDefaultInstallsTheWholePayload(t *testing.T) {
	t.Parallel()
	source, _ := templates(t)
	target := filepath.Join(t.TempDir(), "adopt-default")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "README.md"), []byte("project readme\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	result := adoptWith(t, optionsFor(source, target, &output))
	if result.Already || result.SHA != source.sha {
		t.Fatalf("first adoption result: %+v", result)
	}
	for _, path := range []string{".github/workflows/metasystem.yml", ".claude/agents/verify.md", ".claude/agents/code-critique.md",
		"metasystem.conf", "go.mod", "cmd/metasystem/main.go", "bin/metasystem"} {
		if !exists(filepath.Join(target, path)) {
			t.Fatalf("adoption did not install %s", path)
		}
	}
	// The parallel ratchet is the template's own development test policy,
	// like the coverage floors: it stays home.
	if exists(filepath.Join(target, "testing-parallel-ratchet.json")) {
		t.Fatal("adoption shipped the template's parallel ratchet")
	}
	for _, dir := range []string{"internal", "cmd", "artifacts"} {
		if info, err := os.Stat(filepath.Join(target, dir)); err != nil || !info.IsDir() {
			t.Fatalf("adoption did not install the %s directory", dir)
		}
	}
	// The engine's data is compiled in: adoption ships no scripts tree, and
	// the workflow it installs is the engine's own.
	if exists(filepath.Join(target, "scripts")) {
		t.Fatal("adoption shipped a scripts tree")
	}
	if workflow, err := os.ReadFile(filepath.Join(target, ".github", "workflows", "metasystem.yml")); err != nil || !bytes.Equal(workflow, githubActionsWorkflow) {
		t.Fatalf("the installed workflow is not the engine's: %v", err)
	}
	for _, link := range []string{".claude/skills/verify", ".claude/skills/code-critique"} {
		if info, err := os.Lstat(filepath.Join(target, link)); err != nil || info.Mode()&fs.ModeSymlink == 0 {
			t.Fatalf("claude skill registration %s is not a link", link)
		}
	}
	lines := confLines(t, target)
	for _, want := range []string{"metasystem.runtimes=claude", "role.default.runtime=claude", "suite.progress-silence-min=30",
		"suite.section-cap-min=45", "suite.evidence-copy-timeout-sec=60", "suite.evidence-copy-max-mb=512"} {
		if !hasLine(lines, want) {
			t.Fatalf("tailored configuration lacks %s", want)
		}
	}
	unselected := regexp.MustCompile(`(^|\.)model\.(codex|devin)=|\.runtime=(codex|devin)$`)
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") && unselected.MatchString(line) {
			t.Fatalf("an unselected runtime key survived: %s", line)
		}
	}
	if settings := readText(t, filepath.Join(target, ".claude", "settings.json")); !strings.Contains(settings, "SessionStart") || !regexp.MustCompile(`internal hook claude start;`).MatchString(settings) {
		t.Fatal("the Claude session-start supervision hook is missing")
	}
	if _, err := hostsetup.Setup(hostsetup.Options{RepositoryPath: target, Runtimes: []string{"claude"}, Check: true}); err != nil {
		t.Fatalf("the default Claude registration fails the shared setup check: %v", err)
	}
	if exists(filepath.Join(target, "optional-skills")) || exists(filepath.Join(target, "ignored-fixture.txt")) ||
		exists(filepath.Join(target, "benchmark")) || exists(filepath.Join(target, "development")) {
		t.Fatal("content outside the payload allowlist shipped")
	}
	if readText(t, filepath.Join(target, "README.md")) != "project readme\n" {
		t.Fatal("the project's own README was touched")
	}
	assertShipsNoTemplateProjectState(t, target)
	if got := names(t, filepath.Join(target, "plans")); got != "README.md goals-accepted.json goals.md" {
		t.Fatalf("plans/ carries %q", got)
	}
	if got := names(t, filepath.Join(target, "memory")); got != "README.md instruction-ledger.md known-issues.md rulings.md" {
		t.Fatalf("memory/ carries %q", got)
	}
	if !regexp.MustCompile(`(?m)^## Goal-free: declared \S+ by human over [0-9a-f]{64}$`).MatchString(readText(t, filepath.Join(target, "plans", "goals.md"))) {
		t.Fatal("the seeded ledger lacks its digest-pinned goal-free declaration")
	}
	if store := (&goal.Store{Root: target}); !store.BaselinePresent() || !store.BaselineMatches() {
		t.Fatal("the seeded goal pair is not reconciled")
	}
	if !hasLine(strings.Split(readText(t, filepath.Join(target, ".gitignore")), "\n"), "artifacts/") {
		t.Fatal("artifacts/ is not ignored")
	}
	rules := readText(t, filepath.Join(target, "docs", "project-rules.md"))
	if !strings.Contains(rules, source.sha) || strings.Contains(rules, Placeholder) {
		t.Fatal("the template SHA was not recorded")
	}
	if len(result.Notes) == 0 || !strings.Contains(result.Notes[0], "file-shaped") {
		t.Fatalf("the detectability note is missing: %v", result.Notes)
	}

	// A second run over the healthy installation changes nothing.
	before := treeState(t, target)
	again := adoptWith(t, optionsFor(source, target, io.Discard))
	if !again.Already {
		t.Fatalf("a re-adoption at the same SHA did not recognize the installation: %+v", again)
	}
	sameTree(t, before, treeState(t, target), "a second adoption")

	// Unreplaced placeholders fail the full audit, which the structural
	// pass tolerated, and the audit names them as the adopted placeholder
	// refusal (the retired validator's static scan carried this message;
	// the audit owns it) in the project rules. With those filled the audit
	// passes: the configuration carries nothing to fill, since the evidence
	// root has a compiled-in default (ERD-04).
	audited, err := audit.AuditMetasystem(target, audit.AuditOptions{})
	if err != nil || !hasLine(audited.Violations, "adopted repository has unreplaced placeholders in docs/project-rules.md or metasystem.conf") {
		t.Fatalf("the audit did not name the adopted placeholder refusal in the project rules: %v %q", err, audited.Violations)
	}
	path := filepath.Join(target, "docs", "project-rules.md")
	filled := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(readText(t, path), "filled")
	if err := os.WriteFile(path, []byte(filled), 0o644); err != nil {
		t.Fatal(err)
	}
	if audited, err := audit.AuditMetasystem(target, audit.AuditOptions{}); err != nil || len(audited.Violations) != 0 {
		t.Fatalf("the filled adoption still fails the audit: %v %q", err, audited.Violations)
	}
	if last := result.Notes[len(result.Notes)-1]; !strings.HasPrefix(last, "evidence root: ") {
		t.Fatalf("adoption did not say the evidence root: %v", result.Notes)
	}
}

func TestAdoptGitIntegrationRuntimeSelections(t *testing.T) {
	t.Parallel()
	source, _ := templates(t)
	base := t.TempDir()
	t.Run("devin", func(t *testing.T) {
		t.Parallel()
		target := filepath.Join(base, "adopt-devin")
		options := optionsFor(source, target, io.Discard)
		options.Runtimes = "devin"
		adoptWith(t, options)
		for _, path := range []string{".devin/agents/verify/AGENT.md", ".devin/agents/code-critique/AGENT.md", ".devin/config.json"} {
			if !exists(filepath.Join(target, path)) {
				t.Fatalf("devin registration lacks %s", path)
			}
		}
		for _, link := range []string{".agents/skills/verify", ".agents/skills/code-critique"} {
			if info, err := os.Lstat(filepath.Join(target, link)); err != nil || info.Mode()&fs.ModeSymlink == 0 {
				t.Fatalf("devin skill registration %s is missing", link)
			}
		}
		// Devin discovers project skills through .agents/skills; a second
		// tree under .devin/skills would list every skill twice.
		if exists(filepath.Join(target, ".devin", "skills")) {
			t.Fatal("devin skills were registered twice, under .devin/skills too")
		}
		if !hasLine(confLines(t, target), "metasystem.runtimes=devin") || !hasLine(confLines(t, target), "role.default.runtime=devin") {
			t.Fatal("the devin selection was not recorded as the default")
		}
		if !regexp.MustCompile(`internal hook devin start;`).MatchString(readText(t, filepath.Join(target, ".devin", "config.json"))) || exists(filepath.Join(target, ".claude")) {
			t.Fatal("a devin-only target has the wrong hook configuration")
		}
		if _, err := hostsetup.Setup(hostsetup.Options{RepositoryPath: target, Runtimes: []string{"devin"}, Check: true}); err != nil {
			t.Fatalf("devin registration fails the shared setup check: %v", err)
		}
	})
	t.Run("codex", func(t *testing.T) {
		t.Parallel()
		target := filepath.Join(base, "adopt-codex")
		options := optionsFor(source, target, io.Discard)
		options.Runtimes = "codex"
		adoptWith(t, options)
		for _, link := range []string{".agents/skills/verify", ".agents/skills/code-critique"} {
			if info, err := os.Lstat(filepath.Join(target, link)); err != nil || info.Mode()&fs.ModeSymlink == 0 {
				t.Fatalf("codex skill registration %s is missing", link)
			}
		}
		if !hasLine(confLines(t, target), "metasystem.runtimes=codex") || !hasLine(confLines(t, target), "role.default.runtime=codex") {
			t.Fatal("the codex selection was not recorded as the default")
		}
		if !regexp.MustCompile(`internal hook codex start;`).MatchString(readText(t, filepath.Join(target, ".codex", "hooks.json"))) {
			t.Fatal("the Codex session-start hook is missing")
		}
		if _, err := hostsetup.Setup(hostsetup.Options{RepositoryPath: target, Runtimes: []string{"codex"}, Check: true}); err != nil {
			t.Fatalf("codex registration fails the shared setup check: %v", err)
		}
	})
	t.Run("none", func(t *testing.T) {
		t.Parallel()
		target := filepath.Join(base, "adopt-none")
		options := optionsFor(source, target, io.Discard)
		options.Runtimes = "none"
		adoptWith(t, options)
		for _, path := range []string{".claude", ".devin", ".agents"} {
			if exists(filepath.Join(target, path)) {
				t.Fatalf("--runtimes none registered %s", path)
			}
		}
		if !exists(filepath.Join(target, ".github", "workflows", "metasystem.yml")) || !hasLine(confLines(t, target), "metasystem.runtimes=") {
			t.Fatal("--runtimes none skipped the workflow or recorded a selection")
		}
		for _, line := range confLines(t, target) {
			if strings.HasPrefix(line, "role.") || regexp.MustCompile(`^mode\..*\.role\.`).MatchString(line) {
				t.Fatalf("--runtimes none kept roster line %s", line)
			}
		}
		if _, err := hostsetup.Setup(hostsetup.Options{RepositoryPath: target, Runtimes: []string{"none"}, Check: true}); err != nil {
			t.Fatalf("none registration fails the shared setup check: %v", err)
		}
	})
	t.Run("copied skills and precedence", func(t *testing.T) {
		t.Parallel()
		target := filepath.Join(base, "adopt-copy")
		options := optionsFor(source, target, io.Discard)
		options.Runtimes, options.CopySkills = "claude,codex", true
		adoptWith(t, options)
		for _, dir := range []string{".claude/skills/verify", ".agents/skills/verify"} {
			if info, err := os.Lstat(filepath.Join(target, dir)); err != nil || !info.IsDir() {
				t.Fatalf("--copy-skills did not copy %s", dir)
			}
		}
		if _, err := hostsetup.Setup(hostsetup.Options{RepositoryPath: target, Runtimes: []string{"claude", "codex"}, CopySkills: true, Check: true}); err != nil {
			t.Fatalf("copied registrations fail the shared setup check: %v", err)
		}
		if !hasLine(confLines(t, target), "metasystem.runtimes=claude,codex") || !hasLine(confLines(t, target), "role.default.runtime=codex") {
			t.Fatal("the multi-runtime selection or the codex, devin, claude precedence was not recorded")
		}
	})
	t.Run("enable an optional skill", func(t *testing.T) {
		t.Parallel()
		target := filepath.Join(base, "adopt-java")
		options := optionsFor(source, target, io.Discard)
		options.Enable = []string{"debug-java"}
		adoptWith(t, options)
		if !exists(filepath.Join(target, "skills", "debug-java", "SKILL.md")) || exists(filepath.Join(target, "optional-skills")) {
			t.Fatal("--enable did not move the optional skill")
		}
	})
	t.Run("model tiers keep only selected runtimes", func(t *testing.T) {
		t.Parallel()
		tier := cloneTemplate(t, source)
		conf := filepath.Join(tier.root, "metasystem.conf")
		var rewritten []string
		for _, line := range strings.Split(readText(t, conf), "\n") {
			for runtime, model := range map[string]string{"claude": "claude-model", "codex": "codex-model", "devin": "devin-model"} {
				if strings.HasSuffix(line, ".model."+runtime+"=") {
					line += model
				}
			}
			rewritten = append(rewritten, line)
		}
		text := strings.Join(rewritten, "\n") + "\nmodel.tier.1=claude:claude-model,codex:codex-model,devin:devin-model\n"
		if err := os.WriteFile(conf, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		gitIn(t, tier.root, "commit", "-qam", "tier fixture")
		tier.sha = gitIn(t, tier.root, "rev-parse", "HEAD")
		target := filepath.Join(base, "adopt-tier-claude")
		adoptWith(t, optionsFor(tier, target, io.Discard))
		lines := confLines(t, target)
		if !hasLine(lines, "model.tier.1=claude:claude-model") {
			t.Fatal("the model tier kept an unselected runtime")
		}
		concrete := regexp.MustCompile(`^(role\.|mode\..*\.role\.).*(\.model\.(codex|devin)=|\.runtime=(codex|devin)$)`)
		for _, line := range lines {
			if concrete.MatchString(line) {
				t.Fatalf("a concrete unselected key survived pruning: %s", line)
			}
		}
	})
}

func TestAdoptGitIntegrationRefusesWithoutDamage(t *testing.T) {
	t.Parallel()
	source, _ := templates(t)
	base := t.TempDir()
	t.Run("foreign instruction asset", func(t *testing.T) {
		t.Parallel()
		target := filepath.Join(base, "adopt-foreign")
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(target, ".cursorrules"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		refusedWith(t, optionsFor(source, target, io.Discard), "docs/metasystem-reconciliation.md")
		if exists(filepath.Join(target, "wow.md")) {
			t.Fatal("a refused foreign target was written")
		}
	})
	t.Run("colliding payload path", func(t *testing.T) {
		t.Parallel()
		target := filepath.Join(base, "adopt-collide")
		if err := os.MkdirAll(filepath.Join(target, "docs"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(target, "docs", "collaboration.md"), []byte("different\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		refusedWith(t, optionsFor(source, target, io.Discard), "collision: docs/collaboration.md")
		if readText(t, filepath.Join(target, "docs", "collaboration.md")) != "different\n" || exists(filepath.Join(target, "wow.md")) {
			t.Fatal("a colliding target was overwritten or partly written")
		}
	})
	for _, selection := range []struct{ name, runtimes, want string }{
		{"unknown runtime", "codez", "unknown or non-adoptable runtime: codez"},
		{"none mixed with a runtime", "none,claude", "cannot be combined"},
		{"duplicate runtime", "codex,codex", "duplicate"},
	} {
		t.Run(selection.name, func(t *testing.T) {
			t.Parallel()
			target := filepath.Join(base, strings.ReplaceAll(selection.name, " ", "-"))
			options := optionsFor(source, target, io.Discard)
			options.Runtimes = selection.runtimes
			if refusal := refusedWith(t, options, selection.want); refusal.Code != CodeUsage {
				t.Fatalf("a runtime selection error exits %d, want %d", refusal.Code, CodeUsage)
			}
			if exists(target) {
				t.Fatal("a rejected runtime selection touched the target")
			}
		})
	}
	for _, broken := range []struct{ name, remove string }{
		{"incomplete installation", ".github/workflows/metasystem.yml"},
		{"structurally broken installation", "AGENTS.md"},
	} {
		t.Run(broken.name, func(t *testing.T) {
			t.Parallel()
			target := filepath.Join(base, strings.ReplaceAll(broken.name, " ", "-"))
			adoptWith(t, optionsFor(source, target, io.Discard))
			if err := os.Remove(filepath.Join(target, broken.remove)); err != nil {
				t.Fatal(err)
			}
			refusedWith(t, optionsFor(source, target, io.Discard), "docs/project-adaptation.md")
		})
	}
	t.Run("dirty template", func(t *testing.T) {
		t.Parallel()
		dirty := cloneTemplate(t, source)
		if err := os.WriteFile(filepath.Join(dirty.root, "wow.md"), []byte("dirty\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(base, "adopt-dirty")
		refusal := refusedWith(t, optionsFor(dirty, target, io.Discard), "dirty")
		if len(refusal.Argv) == 0 || exists(filepath.Join(target, "wow.md")) {
			t.Fatalf("a dirty template refusal named no command or wrote the target: %+v", refusal)
		}
	})
	t.Run("engine built from another commit", func(t *testing.T) {
		t.Parallel()
		target := filepath.Join(base, "adopt-stale-engine")
		options := optionsFor(source, target, io.Discard)
		options.Deps.EngineStamp = strings.Repeat("0", 40)
		refusal := refusedWith(t, options, "not from the template's HEAD")
		if len(refusal.Argv) < 4 || refusal.Argv[0] != filepath.Join(source.root, "bin", "metasystem") || refusal.Argv[1] != "system" || refusal.Argv[2] != "adopt" {
			t.Fatalf("the stale-engine refusal does not name the rebuilt engine's adoption: %v", refusal.Argv)
		}
		if exists(filepath.Join(target, "wow.md")) {
			t.Fatal("a stale-engine refusal wrote the target")
		}
	})
}

func TestAdoptGitIntegrationSeedsOrKeepsTheLandingRef(t *testing.T) {
	t.Parallel()
	source, _ := templates(t)
	base := t.TempDir()
	repository := func(t *testing.T, name string) string {
		target := filepath.Join(base, name)
		gitIn(t, base, "init", "-q", "-b", "trunk", target)
		if err := os.WriteFile(filepath.Join(target, "README.md"), []byte(name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitIn(t, target, "add", "README.md")
		gitIn(t, target, "commit", "-qm", "base")
		return target
	}
	landingRef := func(t *testing.T, target string) string {
		out, _ := gitCommand(target, "config", "--local", "--no-includes", "--get", "metasystem.steward.landing-ref")
		return strings.TrimSpace(out)
	}
	notes := func(result Result) string { return strings.Join(result.Notes, "\n") }
	t.Run("detached", func(t *testing.T) {
		t.Parallel()
		target := repository(t, "detached")
		gitIn(t, target, "checkout", "-q", "--detach")
		options := optionsFor(source, target, io.Discard)
		options.Runtimes = "none"
		result := adoptWith(t, options)
		if !strings.Contains(notes(result), "landing ref was not seeded: the target checkout is detached; automatic machine re-arm remains disabled until the key is configured") || landingRef(t, target) != "" {
			t.Fatalf("a detached target: notes=%q ref=%q", notes(result), landingRef(t, target))
		}
	})
	t.Run("preset is kept", func(t *testing.T) {
		t.Parallel()
		target := repository(t, "preset")
		gitIn(t, target, "remote", "add", "origin", target)
		gitIn(t, target, "fetch", "-q", "origin")
		gitIn(t, target, "branch", "--set-upstream-to=origin/trunk", "trunk")
		gitIn(t, target, "config", "--local", "metasystem.steward.landing-ref", "refs/remotes/kept/stable")
		options := optionsFor(source, target, io.Discard)
		options.Runtimes = "none"
		result := adoptWith(t, options)
		if !strings.Contains(notes(result), "landing ref was kept: metasystem.steward.landing-ref=refs/remotes/kept/stable") || landingRef(t, target) != "refs/remotes/kept/stable" {
			t.Fatalf("a preset landing ref: notes=%q ref=%q", notes(result), landingRef(t, target))
		}
	})
	t.Run("detached preset is kept", func(t *testing.T) {
		t.Parallel()
		target := repository(t, "detached-preset")
		gitIn(t, target, "config", "--local", "metasystem.steward.landing-ref", "refs/remotes/kept/detached")
		gitIn(t, target, "checkout", "-q", "--detach")
		options := optionsFor(source, target, io.Discard)
		options.Runtimes = "none"
		result := adoptWith(t, options)
		if !strings.Contains(notes(result), "landing ref was kept: metasystem.steward.landing-ref=refs/remotes/kept/detached") ||
			strings.Contains(notes(result), "not seeded") || landingRef(t, target) != "refs/remotes/kept/detached" {
			t.Fatalf("a detached preset landing ref: notes=%q ref=%q", notes(result), landingRef(t, target))
		}
	})
	t.Run("remote upstream is seeded", func(t *testing.T) {
		t.Parallel()
		target := repository(t, "upstream")
		gitIn(t, target, "remote", "add", "origin", target)
		gitIn(t, target, "fetch", "-q", "origin")
		gitIn(t, target, "branch", "--set-upstream-to=origin/trunk", "trunk")
		options := optionsFor(source, target, io.Discard)
		options.Runtimes = "none"
		adoptWith(t, options)
		if got := landingRef(t, target); got != "refs/remotes/origin/trunk" {
			t.Fatalf("the upstream was not seeded as the landing ref: %q", got)
		}
	})
}

func TestAdoptGitIntegrationFromAVendoredTemplate(t *testing.T) {
	t.Parallel()
	_, nested := templates(t)
	base := t.TempDir()
	t.Run("installation beneath an existing application", func(t *testing.T) {
		t.Parallel()
		application := filepath.Join(base, "application")
		gitIn(t, base, "init", "-q", "-b", "main", application)
		if err := os.WriteFile(filepath.Join(application, "README.md"), []byte("application readme\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitIn(t, application, "add", "README.md")
		gitIn(t, application, "commit", "-qm", "application base")
		install := filepath.Join(application, "vendor", "metasystem runtime")
		adoptWith(t, optionsFor(nested, install, io.Discard))
		if !exists(filepath.Join(install, "metasystem.conf")) || !exists(filepath.Join(install, ".claude", "settings.json")) {
			t.Fatal("the nested installation lacks its local Claude registration")
		}
		if info, err := os.Lstat(filepath.Join(install, ".claude", "skills", "verify")); err != nil || info.Mode()&fs.ModeSymlink == 0 {
			t.Fatal("the nested installation lacks its skill link")
		}
		if exists(filepath.Join(application, ".claude")) || readText(t, filepath.Join(application, "README.md")) != "application readme\n" {
			t.Fatal("adoption beneath an application modified the application's own files")
		}
		if exists(filepath.Join(application, "AGENTS.md")) || exists(filepath.Join(application, "CLAUDE.md")) {
			t.Fatal("adoption beneath an application wrote instruction pointers into the application's root")
		}
		assertShipsNoTemplateProjectState(t, install)
		if _, err := hostsetup.Setup(hostsetup.Options{RepositoryPath: filepath.Join(install, "skills", "verify"), Runtimes: []string{"claude"}, Check: true}); err != nil {
			t.Fatalf("the nested installation fails the shared setup check: %v", err)
		}
	})
	t.Run("a nested prefix exports the whole payload and fresh ledgers", func(t *testing.T) {
		t.Parallel()
		target := filepath.Join(base, "nested-target")
		gitIn(t, base, "init", "-q", "-b", "main", target)
		adoptWith(t, optionsFor(nested, target, io.Discard))
		if !exists(filepath.Join(target, "metasystem.conf")) || !exists(filepath.Join(target, "internal", "protocol")) {
			t.Fatal("a nested-prefix adoption staged an empty payload")
		}
		if exists(filepath.Join(target, "benchmark")) || exists(filepath.Join(target, "development")) {
			t.Fatal("adoption leaked the measuring kit or development/")
		}
		assertShipsNoTemplateProjectState(t, target)
		for _, register := range []string{"instruction-ledger.md", "known-issues.md"} {
			if regexp.MustCompile(`(?m)^\| (IL|KI)-[0-9]`).MatchString(readText(t, filepath.Join(target, "memory", register))) {
				t.Fatalf("adoption shipped the template's own %s rows", register)
			}
		}
		if exists(filepath.Join(target, "memory", "receipts.log")) || exists(filepath.Join(target, "README.md")) {
			t.Fatal("adoption shipped template-repository state")
		}
		if info, err := os.Stat(filepath.Join(target, ".git", "hooks", "pre-commit")); err != nil || info.Mode()&0o111 == 0 {
			t.Fatal("adoption did not install the pre-commit guard hook")
		}
	})
	t.Run("an existing hook is composed behind the guard, once", func(t *testing.T) {
		t.Parallel()
		target := filepath.Join(base, "compose-target")
		gitIn(t, base, "init", "-q", "-b", "main", target)
		hook := filepath.Join(target, ".git", "hooks", "pre-commit")
		if err := writeExecutable(hook, []byte("#!/usr/bin/env bash\ntouch \"$(git rev-parse --show-toplevel)/.project-hook-ran\"\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		result := adoptWith(t, optionsFor(nested, target, io.Discard))
		if !strings.Contains(readText(t, hook), "internal pre-commit") {
			t.Fatal("the engine guard was not enrolled over the existing hook")
		}
		if info, err := os.Stat(hook + ".local"); err != nil || info.Mode()&0o111 == 0 {
			t.Fatal("the existing hook was not preserved as pre-commit.local")
		}
		if !strings.Contains(strings.Join(result.Notes, "\n"), "pre-commit.local") {
			t.Fatalf("the composition was not reported: %v", result.Notes)
		}
		// The composed hook proves composition, not the guard's rules: an
		// admitting stand-in answers the engine's pre-commit entry for this
		// one run, and the target's own engine goes back before re-adoption.
		engine := filepath.Join(target, "bin", "metasystem")
		kept, err := os.ReadFile(engine)
		if err != nil {
			t.Fatalf("the adopted target has no engine to run its guard: %v", err)
		}
		if err := writeExecutable(engine, []byte("#!/usr/bin/env bash\n[[ \"$1 $2\" == \"internal pre-commit\" ]] || exit 3\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		run := exec.Command(hook)
		run.Dir = target
		run.Env = ledgerfence.EnvironWithoutGitSteering()
		out, runErr := run.CombinedOutput()
		if err := writeExecutable(engine, kept, 0o755); err != nil {
			t.Fatal(err)
		}
		if runErr != nil {
			t.Fatalf("the composed hook refused a clean tree: %v\n%s", runErr, out)
		}
		if !exists(filepath.Join(target, ".project-hook-ran")) {
			t.Fatal("the preserved project hook no longer runs")
		}
		if err := os.Remove(filepath.Join(target, ".project-hook-ran")); err != nil {
			t.Fatal(err)
		}
		again := adoptWith(t, optionsFor(nested, target, io.Discard))
		if !again.Already {
			t.Fatalf("re-adoption over the composed hook did not recognize the installation: %+v", again)
		}
		local := readText(t, hook+".local")
		if strings.Contains(local, "internal pre-commit") || !strings.Contains(local, "touch") {
			t.Fatal("re-adoption stacked the composer or clobbered the preserved hook")
		}
	})
	t.Run("both hooks without the guard refuse before any write", func(t *testing.T) {
		t.Parallel()
		target := filepath.Join(base, "both-hooks")
		gitIn(t, base, "init", "-q", "-b", "main", target)
		hooks := filepath.Join(target, ".git", "hooks")
		if err := writeExecutable(filepath.Join(hooks, "pre-commit"), []byte("#!/bin/sh\necho main-hook\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := writeExecutable(filepath.Join(hooks, "pre-commit.local"), []byte("#!/bin/sh\necho local-hook\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		refusedWith(t, optionsFor(nested, target, io.Discard), "compose them by hand")
		if readText(t, filepath.Join(hooks, "pre-commit")) != "#!/bin/sh\necho main-hook\n" || readText(t, filepath.Join(hooks, "pre-commit.local")) != "#!/bin/sh\necho local-hook\n" {
			t.Fatal("the refused adoption changed a hook")
		}
		if got := names(t, target); got != ".git" {
			t.Fatalf("the refused adoption wrote the worktree: %s", got)
		}
		if _, err := gitCommand(target, "rev-parse", "--verify", "HEAD"); err == nil {
			t.Fatal("the refused adoption created a commit")
		}
	})
}

// The adopted guard's rules (IL-14's new-plan acknowledgment, the unborn
// exception, the ledger fence) are the engine's `internal pre-commit` entry
// since U5, proved by internal/landing/landpath TestGuard*; adoption owes the
// enrollment, which the composition leg above proves.

// Writers of an installation vendored below the application keep their state
// in the application's trees, never under the vendored prefix. They run as
// the adopted engine binary, whose installation is the vendored prefix.
func TestAdoptGitIntegrationVendoredWritersUseApplicationState(t *testing.T) {
	t.Parallel()
	_, nested := templates(t)
	engine := realEngine(t)
	base := t.TempDir()
	target := filepath.Join(base, "application")
	gitIn(t, base, "init", "-q", "-b", "main", target)
	gitIn(t, target, "config", "metasystem.goal.machine", "fixture-machine")
	adoptWith(t, optionsFor(nested, target, io.Discard))
	prefix := filepath.Join(target, "metasystem")
	data, err := os.ReadFile(engine)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeExecutable(filepath.Join(prefix, "bin", "metasystem"), data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prefix, "metasystem.conf"), []byte(readText(t, filepath.Join(target, "metasystem.conf"))), 0o644); err != nil {
		t.Fatal(err)
	}
	// A commitless repository ticks degraded by design and a degraded tick
	// never persists; real adopted repositories have commits.
	gitIn(t, target, "add", "-A")
	gitIn(t, target, "-c", "core.hooksPath=/dev/null", "commit", "-qm", "adopted base")
	run := func(args ...string) string {
		command := exec.Command(filepath.Join(prefix, "bin", "metasystem"), args...)
		command.Dir = target
		command.Env = ledgerfence.EnvironWithoutGitSteering()
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	run("receipt", "add", "--type", "implement", "--outcome", "shipped", "--skills", "none", "--verify", "clean",
		"--corrections", "0", "--stop-loss", "no", "--note", "adopted state-root fixture", "--root", prefix)
	if tick := run("internal", "steward", "tick", "--repo", target); strings.Contains(tick, `"verdict": "degraded"`) {
		t.Fatalf("the adopted steward tick degraded: %s", tick)
	}
	if !exists(filepath.Join(target, "memory", "receipts.log")) || !exists(filepath.Join(target, "artifacts", "agents", "steward", "highwater.json")) {
		t.Fatal("the vendored writers did not use the application's state trees")
	}
	for _, register := range []string{"known-issues.md", "flake-registry.md", "rulings.md", "instruction-ledger.md", "receipts.log", "backlog-notes.md", "proposal-drafts.md", "llm-wiki-pattern.md"} {
		for _, dir := range []string{"", "plans", "memory"} {
			if exists(filepath.Join(prefix, dir, register)) {
				t.Fatalf("a writer placed %s under the vendored prefix", filepath.Join(dir, register))
			}
		}
	}
}

// The two-part law, the half adoption proves: the app-owned covenant, its
// doctrine, its evidence table and a covenant-referenced net file survive the
// first adoption and a same-version re-run byte for byte.
func TestAdoptGitIntegrationKeepsAppOwnedCovenantFiles(t *testing.T) {
	t.Parallel()
	source, _ := templates(t)
	target := filepath.Join(t.TempDir(), "adopt-covenant")
	files := map[string]string{
		"covenant.json":             `{"identity": {"name": "the-app"}, "guardrails": ["goldens/", "gate.sh"]}` + "\n",
		"docs/app-doctrine.md":      "# the-app doctrine\npatterns the delegates honor\n",
		"docs/covenant-evidence.md": "# Covenant evidence — the-app\n\n| criterion id | criterion | proof id | kind | exact command | repo deps | evidence source | status |\n| --- | --- | --- | --- | --- | --- | --- | --- |\n| 1 | The score holds | score | repo | bash gate.sh | gate.sh | gate.sh emits the metric | observed |\n\nWired: 1. Floating: 0.\n",
		"gate.sh":                   "#!/usr/bin/env bash\nprintf \"metric=score=1\\n\"\n",
	}
	for rel, content := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(target, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(target, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, run := range []string{"adoption", "a same-version re-run"} {
		adoptWith(t, optionsFor(source, target, io.Discard))
		for rel, content := range files {
			if readText(t, filepath.Join(target, rel)) != content {
				t.Fatalf("%s altered the app-owned %s", run, rel)
			}
		}
	}
}

// The declared touch list: adoption's writes are exactly the source's
// shippable inventory plus the enumerated seeds, and pre-existing
// application bytes are unchanged (modification and deletion are caught, not
// only creation). Two runtimes exercise the branching writers.
func TestAdoptGitIntegrationWritesOnlyTheDeclaredInventory(t *testing.T) {
	t.Parallel()
	_, nested := templates(t)
	expected := map[string]bool{}
	err := filepath.WalkDir(nested.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(nested.root, path)
		first, _, _ := strings.Cut(filepath.ToSlash(rel), "/")
		if entry.IsDir() {
			switch first {
			case ".git", "artifacts", "bin", "development", "memory", "plans", "records":
				return filepath.SkipDir
			}
			return nil
		}
		expected[filepath.ToSlash(rel)] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	seeds := map[string]bool{
		"src/app.txt": true, "docs/app.md": true, ".gitignore": true, ".gitattributes": true,
		"metasystem.conf": true, "plans/goals-accepted.json": true, "bin/metasystem": true, ".github/workflows/metasystem.yml": true,
		"memory/known-issues.md": true, "memory/instruction-ledger.md": true, "memory/rulings.md": true,
		"plans/goals.md": true, "plans/README.md": true, "memory/README.md": true, "records/README.md": true,
		"records/misc/fleet-coordinator-brain-role-packet.md": true,
	}
	for _, runtime := range []string{"claude", "codex"} {
		t.Run(runtime, func(t *testing.T) {
			t.Parallel()
			target := filepath.Join(t.TempDir(), "tracer-"+runtime)
			gitIn(t, filepath.Dir(target), "init", "-q", "-b", "main", target)
			app := map[string]string{"src/app.txt": "app content\n", "docs/app.md": "app doc\n"}
			for rel, content := range app {
				if err := os.MkdirAll(filepath.Dir(filepath.Join(target, rel)), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(target, rel), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			options := optionsFor(nested, target, io.Discard)
			options.Runtimes = runtime
			adoptWith(t, options)
			for rel, content := range app {
				if readText(t, filepath.Join(target, rel)) != content {
					t.Fatalf("adoption altered or deleted app-owned %s", rel)
				}
			}
			err := filepath.WalkDir(target, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				rel, _ := filepath.Rel(target, path)
				rel = filepath.ToSlash(rel)
				if entry.IsDir() {
					if rel == ".git" || rel == "artifacts" {
						return filepath.SkipDir
					}
					return nil
				}
				switch {
				case expected[rel], seeds[rel], strings.HasPrefix(rel, "plans/goals/"),
					strings.HasPrefix(rel, ".claude/"), strings.HasPrefix(rel, ".agents/"), strings.HasPrefix(rel, ".devin/"), rel == ".codex/hooks.json":
					return nil
				}
				return fmt.Errorf("adoption wrote outside the computed inventory (%s): %s", runtime, rel)
			})
			if err != nil {
				t.Fatal(err)
			}
			if hooks := names(t, filepath.Join(target, ".git", "hooks")); !strings.Contains(" "+hooks+" ", " pre-commit ") {
				t.Fatalf("adoption installed no hook (%s): %s", runtime, hooks)
			}
			if exists(filepath.Join(target, "metasystem", "memory")) || !exists(filepath.Join(target, "memory", "rulings.md")) {
				t.Fatal("the landing rulings are not at the application root")
			}
		})
	}
}

// writeExecutable writes an executable file, creating its directory.
func writeExecutable(path string, data []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return testexec.WriteFile(path, data, mode)
}
