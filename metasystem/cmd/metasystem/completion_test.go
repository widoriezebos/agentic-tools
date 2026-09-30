package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// The shell Tab completion's proof (design shell-tab-completion, section 4).

// completionFixtureGoals are the fixture ledger's goals and their states.
var completionFixtureGoals = map[string]string{"first-goal": "queued", "fix-later": "parked", "other-work": "claimed"}

// completionNoExecutable is the executable seam of a completion test's
// resolver: completion never reads it.
func completionNoExecutable() (string, error) {
	return "", errors.New("completion reads no executable")
}

// completionTemp is a temporary directory with its symbolic links resolved,
// as git and the resolver name it.
func completionTemp(t testing.TB) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func completionWrite(t testing.TB, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// completionLedger writes goal files as the ledger's writer does, and the
// ledger's root record, which is not a goal.
func completionLedger(t testing.TB, goals string, states map[string]string) {
	t.Helper()
	for id, state := range states {
		completionWrite(t, filepath.Join(goals, id+".md"), "# "+id+"\n\n- State: "+state+"\n- Priority: 2\n\n## Intent\n\nfixture\n")
	}
	completionWrite(t, filepath.Join(goals, "backlog.md"), "# Backlog\n\n- Identity: P700PZP7S9CSCKSR9J0WSVZ263\n- FormatVersion: 1\n")
}

// completionTemplateCheckout is a nested template checkout, built without
// git: a .git entry at the top and the template's installation under
// metasystem/, with its ledger.
func completionTemplateCheckout(t testing.TB, states map[string]string) string {
	t.Helper()
	repo := completionTemp(t)
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	completionWrite(t, filepath.Join(repo, "metasystem", "metasystem.conf"), "metasystem.template=true\n")
	completionLedger(t, filepath.Join(repo, "metasystem", "plans", "goals"), states)
	return repo
}

func completionOwnersIn(cwd string) completeOwners {
	return completeOwners{cwd: cwd, resolver: stateroot.NewResolver(nearestGitTop, completionNoExecutable)}
}

// completeLines runs one completion through the entry's protocol and returns
// its output lines; every completion exits 0 with nothing on stderr.
func completeLines(t testing.TB, owners completeOwners, words ...string) []string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runComplete(append([]string{"--"}, words...), &stdout, &stderr, owners)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("complete %q: code %d stderr %q", words, code, stderr.String())
	}
	text := strings.TrimSuffix(stdout.String(), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// completedWords are the candidates' first columns; directives are kept as
// they are.
func completedWords(lines []string) []string {
	var words []string
	for _, line := range lines {
		word, _, _ := strings.Cut(line, "\t")
		words = append(words, word)
	}
	return words
}

func TestCompleteOffersEveryRoutableWordAndNothingElse(t *testing.T) {
	t.Parallel()
	owners := completionOwnersIn(completionTemp(t))
	offered := func(words ...string) []string { return completedWords(completeLines(t, owners, words...)) }
	sentinel := &sentinelFamilies{}
	registered := sentinel.registry()
	routes := func(want string, args ...string) {
		t.Helper()
		code, page, problem := routeWith(registered, args...)
		if code != 0 || problem != "" || (want != "" && page != want) {
			t.Errorf("%q is offered but does not route: code %d stderr %q", args, code, problem)
		}
	}
	names := func(commands []intentCommand) []string {
		var actions []string
		for _, command := range commands {
			actions = append(actions, command.action)
		}
		return actions
	}
	page := func(write func(w *bytes.Buffer)) string {
		var buffer bytes.Buffer
		write(&buffer)
		return buffer.String()
	}

	first := offered("")
	if want := append(intentObjects(), "status", "help", "internal"); !slices.Equal(first, want) {
		t.Fatalf("word 1 offers %v, want %v", first, want)
	}
	for _, word := range first {
		switch word {
		case "status":
			command, _ := findIntentCommand("status")
			routes(page(func(w *bytes.Buffer) { writeIntentHelp(w, command) }), "status", "--help")
		case "help", "internal":
			routes("", word)
		default:
			routes(page(func(w *bytes.Buffer) { writeIntentObjectHelp(w, word) }), word)
			actions := offered(word, "")
			if want := names(objectActions(word)); !slices.Equal(actions, want) {
				t.Errorf("%s offers %v, want its public actions %v", word, actions, want)
			}
			for _, action := range actions {
				command, _ := findIntentAction(word, action)
				routes(page(func(w *bytes.Buffer) { writeIntentHelp(w, command) }), word, action, "--help")
			}
		}
	}
	if got := offered("go"); !slices.Equal(got, []string{"goal"}) {
		t.Errorf("go offers %v, want goal", got)
	}

	helpObjects := offered("help", "")
	if !slices.Equal(helpObjects, intentObjects()) {
		t.Errorf("help offers %v, want the objects", helpObjects)
	}
	for _, object := range helpObjects {
		routes("", "help", object)
		actions := offered("help", object, "")
		if want := names(objectActions(object)); !slices.Equal(actions, want) {
			t.Errorf("help %s offers %v, want %v", object, actions, want)
		}
		for _, action := range actions {
			routes("", "help", object, action)
		}
		if got := offered("help", object, actions[0], ""); len(got) != 0 {
			t.Errorf("help %s %s offers %v, want nothing", object, actions[0], got)
		}
	}

	var wantInternal []string
	for _, fam := range families() {
		wantInternal = append(wantInternal, fam.name)
	}
	for _, entry := range topLevelEntries() {
		wantInternal = append(wantInternal, entry.name)
	}
	internal := offered("internal", "")
	if !slices.Equal(internal, wantInternal) {
		t.Errorf("internal offers %v, want every family and entry %v", internal, wantInternal)
	}
	if !slices.Contains(internal, "__complete") {
		t.Errorf("internal does not offer __complete: %v", internal)
	}
	for _, word := range internal {
		if isTopLevelEntry(word) {
			// An entry answers help as TestEveryInternalEntrypointAnswersHelp
			// holds it: exit 0 and its usage on stdout. pre-commit's parser
			// also repeats its usage on stderr, which this slice leaves alone.
			code, page, problem := routeWith(registered, "internal", word, "--help")
			if code != 0 || !strings.Contains(page, "usage: metasystem internal "+word) {
				t.Errorf("internal %s --help is offered but does not route: code %d stdout %q stderr %q", word, code, page, problem)
			}
		} else {
			routes("", "internal", word, "--help")
		}
		var verbs []string
		for _, fam := range families() {
			if fam.name == word {
				for _, v := range fam.verbs {
					verbs = append(verbs, v.name)
				}
			}
		}
		got := offered("internal", word, "")
		if !slices.Equal(got, verbs) {
			t.Errorf("internal %s offers %v, want %v", word, got, verbs)
		}
		for _, v := range got {
			routes("", "internal", word, v, "--help")
			if deeper := offered("internal", word, v, ""); len(deeper) != 0 {
				t.Errorf("internal %s %s offers %v; its parser is its own", word, v, deeper)
			}
		}
	}
	if calls := sentinel.taken(); len(calls) != len(familyVerbCount()) {
		t.Errorf("the oracle reached %d family verbs, want %d", len(calls), len(familyVerbCount()))
	}

	goalActions := offered("goal", "")
	if slices.Contains(goalActions, "fetch") || slices.Contains(goalActions, "next") {
		t.Errorf("goal offers hidden entries: %v", goalActions)
	}
	if internalGoal := offered("internal", "goal", ""); !slices.Contains(internalGoal, "fetch") || !slices.Contains(internalGoal, "next") {
		t.Errorf("internal goal lacks its entries: %v", internalGoal)
	}
	if got := offered("goal", "fetch", ""); len(got) != 0 {
		t.Errorf("a hidden row offers %v", got)
	}
	if got := offered("goall", ""); len(got) != 0 {
		t.Errorf("a typo offers %v", got)
	}
	if got := offered("goal", "aprove", "--"); len(got) != 0 {
		t.Errorf("a mistyped action offers %v", got)
	}
}

func familyVerbCount() []string {
	var verbs []string
	for _, fam := range families() {
		for _, v := range fam.verbs {
			verbs = append(verbs, fam.name+" "+v.name)
		}
	}
	return verbs
}

func TestCompleteOffersEachCommandsOwnFlags(t *testing.T) {
	t.Parallel()
	owners := completionOwnersIn(completionTemp(t))
	for _, command := range publicIntentCommands() {
		got := completedWords(completeLines(t, owners, append(command.words(), "--")...))
		var want []string
		declared := false
		for _, definition := range command.helpFlags() {
			declared = declared || definition.name == "lineage"
			if !definition.hidden {
				want = append(want, "--"+definition.name)
			}
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s -- offers %v, want %v", command.name, got, want)
		}
		if slices.Contains(got, "--fixture-human-authority") {
			t.Errorf("%s offers the hidden --fixture-human-authority", command.name)
		}
		if shown := slices.Contains(got, "--lineage"); shown != (declared && (command.audience == "agent" || command.audience == "both")) {
			t.Errorf("%s: --lineage offered %v, declared %v, audience %s", command.name, shown, declared, command.audience)
		}
		if command.passthrough != nil {
			continue
		}
		for _, flag := range got {
			definition, _ := command.lookupFlag(strings.TrimPrefix(flag, "--"))
			raw := []string{flag}
			if definition.value != "" {
				raw = append(raw, "x")
			}
			if _, problem := parseIntentArgs(command, raw); problem != nil && strings.Contains(problem.summary, "does not take") {
				t.Errorf("%s: offered %s does not parse: %s", command.name, flag, problem.summary)
			}
		}
	}
	if got := completedWords(completeLines(t, owners, "goal", "approve", "--bu")); !slices.Equal(got, []string{"--budget"}) {
		t.Errorf("goal approve --bu offers %v", got)
	}
}

func TestCompleteValuesFollowTheParser(t *testing.T) {
	t.Parallel()
	repo := completionTemplateCheckout(t, completionFixtureGoals)
	owners := completionOwnersIn(filepath.Join(repo, "metasystem"))
	all := []string{"first-goal\tqueued", "fix-later\tparked", "other-work\tclaimed"}
	fi := []string{"first-goal\tqueued", "fix-later\tparked"}
	for _, row := range []struct {
		words []string
		want  []string
	}{
		{[]string{"goal", "approve", ""}, all},
		{[]string{"goal", "approve", "fi"}, fi},
		{[]string{"goal", "approve", "first-goal", "o"}, []string{"other-work\tclaimed"}},
		{[]string{"goal", "approve", "b"}, nil},
		{[]string{"goal", "budget", "--id", ""}, all},
		{[]string{"status", ""}, all},
		{[]string{"goal", "approve", "--", ""}, all},
		{[]string{"goal", "budget", "--id=fi"}, append([]string{":prefix 5"}, fi...)},
		{[]string{"goal", "pause", "G", "--reason", ""}, nil},
		{[]string{"goal", "pause", "G", "--reason", "-"}, nil},
		{[]string{"work", "review", "G", "--critic", ""}, nil},
		{[]string{"work", "wait", "G", "--chain", ""}, nil},
		{[]string{"work", "build", "G", "--brief", "FILE", "--check", "go", "test", "--"}, nil},
		{[]string{"work", "build", "G", "--brief", ""}, []string{":files"}},
		{[]string{"work", "build", "G", "--brief=des"}, []string{":prefix 8", ":files"}},
		{[]string{"work", "build", "G", "--brief", "--draft=v1/de"}, []string{":files"}},
		{[]string{"goal", "show", "G", ""}, nil},
		{[]string{"goal", "budget", "G", ""}, nil},
		// A passthrough row's owner parses for itself: an option the row does
		// not document is read as a switch, and the walk goes on.
		{[]string{"test", "plan", "--json", "--goal", "fi"}, fi},
		// A parsed row refuses an unknown option, so nothing follows it.
		{[]string{"goal", "approve", "--nosuch", ""}, nil},
	} {
		if got := completeLines(t, owners, row.words...); !slices.Equal(got, row.want) {
			t.Errorf("%q offers %q, want %q", row.words, got, row.want)
		}
	}
	for _, row := range []struct {
		words []string
		want  []string
	}{
		{[]string{"goal", "open", "G", "--origin", ""}, []string{"human", "main"}},
		{[]string{"goal", "prioritize", "G", ""}, []string{"1", "2", "3"}},
		{[]string{"system", "completion", ""}, []string{"zsh", "bash"}},
		{[]string{"system", "completion", "z"}, []string{"zsh"}},
	} {
		if got := completedWords(completeLines(t, owners, row.words...)); !slices.Equal(got, row.want) {
			t.Errorf("%q offers %q, want %q", row.words, got, row.want)
		}
	}
}

// TestCompleteDirectivePrefixOnlyForParserJoinedValues: the :prefix
// directive follows the walk, not the spelling (TC-08 confirmation).
func TestCompleteDirectivePrefixOnlyForParserJoinedValues(t *testing.T) {
	t.Parallel()
	repo := completionTemplateCheckout(t, completionFixtureGoals)
	owners := completionOwnersIn(repo)
	fi := []string{"first-goal\tqueued", "fix-later\tparked"}
	for _, row := range []struct {
		words []string
		want  []string
	}{
		{[]string{"work", "build", "G", "--brief=notes=v1/de"}, []string{":prefix 8", ":files"}},
		{[]string{"work", "build", "G", "--brief", "notes=v1/de"}, []string{":files"}},
		{[]string{"work", "build", "G", "--brief", "--draft=v1/de"}, []string{":files"}},
		{[]string{"design", "check", "notes=v1/de"}, []string{":files"}},
		{[]string{"goal", "pause", "G", "--reason", "--id=fi"}, nil},
		{[]string{"goal", "approve", "--", "--id=fi"}, nil},
		{[]string{"goal", "approve", "-id=fi"}, append([]string{":prefix 4"}, fi...)},
		{[]string{"goal", "approve", "--goal=fi"}, append([]string{":prefix 7"}, fi...)},
		{[]string{"work", "build", "G", "--nosuch=x"}, nil},
	} {
		if got := completeLines(t, owners, row.words...); !slices.Equal(got, row.want) {
			t.Errorf("%q answers %q, want %q", row.words, got, row.want)
		}
	}
}

// completionGoalDirOf is the goal directory the verbs would select for path,
// computed with the same two resolver calls selectRoot makes.
func completionGoalDirOf(t *testing.T, resolver stateroot.Resolver, path string) string {
	t.Helper()
	layout, err := resolver.ResolveLayout(path)
	if err != nil {
		t.Fatalf("ResolveLayout(%s): %v", path, err)
	}
	root, err := resolver.RootForInstallation(layout.InstallationRoot)
	if err != nil {
		t.Fatalf("RootForInstallation(%s): %v", layout.InstallationRoot, err)
	}
	return filepath.Join(root, "plans", "goals")
}

// completionFixtures are the checkout shapes of TC-01, built without git:
// each names the directory a completion runs in, an optional --repo, the goal
// directory the verbs would select, and the ids that must come back.
type completionCheckoutFixture struct {
	name       string
	cwd, repo  string
	goals      string
	ids        []string
	executable string
}

func completionCheckoutFixtures(t *testing.T) []completionCheckoutFixture {
	t.Helper()
	nested := completionTemplateCheckout(t, map[string]string{"nested-goal": "queued"})

	adopted := completionTemp(t)
	if err := os.Mkdir(filepath.Join(adopted, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	completionWrite(t, filepath.Join(adopted, "vendor", "ms", "metasystem.conf"), "metasystem.runtimes=claude\n")
	completionLedger(t, filepath.Join(adopted, "plans", "goals"), map[string]string{"adopted-goal": "approved"})
	if err := os.MkdirAll(filepath.Join(adopted, "vendor", "ms", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	a := completionTemplateCheckout(t, map[string]string{"a-goal": "queued"})
	b := completionTemplateCheckout(t, map[string]string{"b-goal": "queued"})

	return []completionCheckoutFixture{
		{name: "nested template from the repository top with --repo .", cwd: nested, repo: ".", goals: filepath.Join(nested, "metasystem", "plans", "goals"), ids: []string{"nested-goal"}},
		{name: "nested template from inside metasystem/", cwd: filepath.Join(nested, "metasystem"), goals: filepath.Join(nested, "metasystem", "plans", "goals"), ids: []string{"nested-goal"}},
		{name: "adopted installation beneath the repository, from a subdirectory", cwd: filepath.Join(adopted, "vendor", "ms", "sub"), goals: filepath.Join(adopted, "plans", "goals"), ids: []string{"adopted-goal"}},
		{name: "another checkout's executable", cwd: filepath.Join(b, "metasystem"), goals: filepath.Join(b, "metasystem", "plans", "goals"), ids: []string{"b-goal"},
			executable: filepath.Join(a, "metasystem", "bin", "metasystem")},
	}
}

func TestCompleteGoalsComeFromTheCommandsCheckout(t *testing.T) {
	t.Parallel()
	for _, fixture := range completionCheckoutFixtures(t) {
		executable := completionNoExecutable
		if fixture.executable != "" {
			executable = func() (string, error) { return fixture.executable, nil }
		}
		resolver := stateroot.NewResolver(nearestGitTop, executable)
		owners := completeOwners{cwd: fixture.cwd, resolver: resolver}
		words := []string{"goal", "approve", ""}
		selected := fixture.cwd
		if fixture.repo != "" {
			words = []string{"goal", "approve", "--repo", fixture.repo, ""}
			selected = filepath.Join(fixture.cwd, fixture.repo)
		}
		if want := completionGoalDirOf(t, resolver, selected); want != fixture.goals {
			t.Fatalf("%s: the verbs select %s, the fixture expects %s", fixture.name, want, fixture.goals)
		}
		dir, ok := completionGoalDir(owners, fixture.repo)
		if !ok || dir != fixture.goals {
			t.Errorf("%s: completion reads %q (%v), want %s", fixture.name, dir, ok, fixture.goals)
		}
		if got := completedWords(completeLines(t, owners, words...)); !slices.Equal(got, fixture.ids) {
			t.Errorf("%s: offers %v, want %v", fixture.name, got, fixture.ids)
		}
	}
	outside := completionTemp(t)
	completionLedger(t, filepath.Join(outside, "plans", "goals"), map[string]string{"stray-goal": "queued"})
	if got := completeLines(t, completionOwnersIn(outside), "goal", "approve", ""); len(got) != 0 {
		t.Errorf("a path in no repository offers %v", got)
	}
}

// TestCompletionRootWalkMatchesGitToplevel is the one completion test whose
// claim is the git interaction: the pure repository walk answers what git
// rev-parse --show-toplevel answers on the same fixtures.
func TestCompletionRootWalkMatchesGitToplevel(t *testing.T) {
	t.Parallel()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("git is not on PATH: %v", err)
	}
	initialized := map[string]bool{}
	for _, fixture := range completionCheckoutFixtures(t) {
		top := fixture.cwd
		for {
			if _, err := os.Lstat(filepath.Join(top, ".git")); err == nil {
				break
			}
			top = filepath.Dir(top)
		}
		if !initialized[top] {
			initialized[top] = true
			if err := os.Remove(filepath.Join(top, ".git")); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command(gitPath, "init", "-q", top).CombinedOutput(); err != nil {
				t.Fatalf("git init %s: %v %s", top, err, out)
			}
		}
		for _, path := range []string{fixture.cwd, top, fixture.goals} {
			pure, pureErr := nearestGitTop(path)
			real, realErr := stateroot.RepositoryTop(path)
			if pureErr != nil || realErr != nil || pure != real {
				t.Errorf("%s: %s: pure walk %q (%v), git %q (%v)", fixture.name, path, pure, pureErr, real, realErr)
			}
		}
	}
}

// completionTreeDigest hashes every path, mode and content under root.
func completionTreeDigest(t *testing.T, root string) string {
	t.Helper()
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(hash, "%s %v %d\n", path, info.Mode(), info.Size())
		if entry.Type().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			hash.Write(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func TestCompleteReadsWithoutGitOrRecords(t *testing.T) {
	t.Parallel()
	repo := completionTemplateCheckout(t, completionFixtureGoals)
	var executableCalls atomic.Int32
	resolver := stateroot.NewResolver(nearestGitTop, func() (string, error) {
		executableCalls.Add(1)
		return "", errors.New("unexpected")
	})
	before := completionTreeDigest(t, repo)
	for _, words := range [][]string{{"goal", "approve", ""}, {"status", "f"}, {"goal", "approve", "--repo", ".", ""}, {"work", "build", "G", "--brief=x"}, {""}} {
		completeLines(t, completeOwners{cwd: repo, resolver: resolver}, words...)
	}
	if got := completeLines(t, completeOwners{cwd: repo, resolver: resolver}, "goal", "approve", ""); len(got) != len(completionFixtureGoals) {
		t.Errorf("the completion read %d goals, want %d: %q", len(got), len(completionFixtureGoals), got)
	}
	if after := completionTreeDigest(t, repo); after != before {
		t.Errorf("a completion changed the fixture tree")
	}
	if calls := executableCalls.Load(); calls != 0 {
		t.Errorf("completion read the executable %d times", calls)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "completion.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		if path == "os/exec" || strings.HasSuffix(path, "/internal/goal") || strings.Contains(path, "/internal/goal/") {
			t.Errorf("completion.go imports %s", path)
		}
	}
}

// BenchmarkCompleteGoalIds is evidence for D-fast, not a gate: goal ids from
// a ledger of 1000 goals.
func BenchmarkCompleteGoalIds(b *testing.B) {
	states := map[string]string{}
	for index := range 1000 {
		states[fmt.Sprintf("goal-%04d", index)] = "queued"
	}
	repo := completionTemplateCheckout(b, states)
	owners := completionOwnersIn(filepath.Join(repo, "metasystem"))
	for b.Loop() {
		var stdout, stderr bytes.Buffer
		if code := runComplete([]string{"--", "goal", "approve", ""}, &stdout, &stderr, owners); code != 0 || strings.Count(stdout.String(), "\n") != 1000 {
			b.Fatalf("code %d, %d lines", code, strings.Count(stdout.String(), "\n"))
		}
	}
}

func TestCompletionScriptsCallBackThroughTheTypedCommand(t *testing.T) {
	t.Parallel()
	registered := (&sentinelFamilies{}).registry()
	code, zsh, problem := routeWith(registered, "system", "completion", "zsh")
	if code != 0 || problem != "" {
		t.Fatalf("system completion zsh: code %d stderr %q", code, problem)
	}
	for _, want := range []string{`"${(Q)words[1]}" __complete --`, `"${(@Q)words[2,CURRENT]}"`, "compset -p", "compdef _metasystem metasystem"} {
		if !strings.Contains(zsh, want) {
			t.Errorf("the zsh script lacks %q:\n%s", want, zsh)
		}
	}
	for _, refused := range []string{"compset -P", "'*='", "(#", "=*"} {
		if strings.Contains(zsh, refused) {
			t.Errorf("the zsh script holds a pattern over the word, %q:\n%s", refused, zsh)
		}
	}
	code, bash, problem := routeWith(registered, "system", "completion", "bash")
	if code != 0 || problem != "" {
		t.Fatalf("system completion bash: code %d stderr %q", code, problem)
	}
	for _, want := range []string{`"${COMP_WORDS[0]}" __complete --`, "complete -o filenames -F _metasystem metasystem"} {
		if !strings.Contains(bash, want) {
			t.Errorf("the bash script lacks %q:\n%s", want, bash)
		}
	}
	if strings.Contains(bash, "compopt") {
		t.Errorf("the bash script uses compopt, which bash 3.2 lacks")
	}
	for _, args := range [][]string{{"system", "completion", "fish"}, {"system", "completion"}} {
		code, _, problem = routeWith(registered, args...)
		if code != 2 || !strings.Contains(problem, "zsh") || !strings.Contains(problem, "bash") {
			t.Errorf("%q: code %d stderr %q; want exit 2 naming zsh and bash", args, code, problem)
		}
	}
	code, encoded, problem := routeWith(registered, "system", "completion", "zsh", "--json")
	var result struct {
		Verb    string `json:"verb"`
		Outcome string `json:"outcome"`
		Data    struct {
			Script string `json:"script"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(encoded), &result); err != nil || code != 0 || problem != "" {
		t.Fatalf("--json: code %d err %v stderr %q stdout %q", code, err, problem, encoded)
	}
	if result.Verb != "system completion" || result.Outcome != intentConfirmed || result.Data.Script != zsh {
		t.Errorf("--json envelope %+v does not carry the zsh script", result)
	}
}

// zshGlueDriver writes a zsh -f script that stubs the completion system's
// functions, loads the glue that system completion zsh prints, and runs the
// cases; it returns the script's output.
func zshGlueDriver(t *testing.T, dir, cases string) string {
	t.Helper()
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skipf("zsh is not on PATH: %v", err)
	}
	_, glue, _ := routeWith((&sentinelFamilies{}).registry(), "system", "completion", "zsh")
	completionWrite(t, filepath.Join(dir, "glue.zsh"), glue)
	driver := `compdef() { print -r -- "compdef $*" }
compset() {
  print -r -- "compset $*"
  if [[ $1 == -p ]] && (( ${#PREFIX} >= $2 )); then IPREFIX+=${PREFIX[1,$2]}; PREFIX=${PREFIX[$2+1,-1]}; return 0; fi
  return 1
}
_files() { print -r -- "files PREFIX=$PREFIX IPREFIX=$IPREFIX" }
_describe() { print -r -- "describe $1 $2 $3 ${(j:,:)${(@P)4}}" }
source ./glue.zsh
` + cases
	completionWrite(t, filepath.Join(dir, "driver.zsh"), driver)
	command := exec.Command(zsh, "-f", "./driver.zsh")
	command.Dir = dir
	command.Env = append(os.Environ(), "ZDOTDIR="+dir)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("zsh driver: %v\n%s", err, out)
	}
	return string(out)
}

// completionEngineStub is an executable that records its argument vector, one
// per line, in argv.record and prints the answer file $ANSWER names.
func completionEngineStub(t *testing.T, path, dir string) {
	t.Helper()
	completionWrite(t, path, "#!/bin/sh\nprintf '%s\\n' \"$@\" > '"+filepath.Join(dir, "argv.record")+"'\ncat \"$ANSWER\"\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestZshGlueUnquotesAndKeepsEmptyWords(t *testing.T) {
	t.Parallel()
	dir := completionTemp(t)
	completionEngineStub(t, filepath.Join(dir, "my bin", "metasystem"), dir)
	completionWrite(t, filepath.Join(dir, "answer"), "first-goal\tqueued\n")
	out := zshGlueDriver(t, dir, `export ANSWER=./answer
words=('./my bin/metasystem' goal approve '--repo' "'dir with space'" '')
CURRENT=6; PREFIX=''; IPREFIX=''
_metasystem
`)
	if !strings.Contains(out, "describe -t metasystem metasystem first-goal:queued") {
		t.Errorf("the glue did not describe the engine's candidates:\n%s", out)
	}
	if strings.Contains(out, "compset") {
		t.Errorf("the glue moved a prefix without a directive:\n%s", out)
	}
	record, err := os.ReadFile(filepath.Join(dir, "argv.record"))
	if err != nil {
		t.Fatalf("the engine was not called back: %v\n%s", err, out)
	}
	argv := strings.Split(strings.TrimSuffix(string(record), "\n"), "\n")
	if want := []string{"__complete", "--", "goal", "approve", "--repo", "dir with space", ""}; !slices.Equal(argv, want) {
		t.Errorf("the engine received %q, want %q", argv, want)
	}
}

// TestZshCompletionPreservesEqualsInPaths is TC-08's fixture obligation: the
// engine stub answers what the Go walk answers for each line, and only a
// parser-joined value has its --name= moved out of the completed word.
func TestZshCompletionPreservesEqualsInPaths(t *testing.T) {
	t.Parallel()
	dir := completionTemp(t)
	engine := filepath.Join(dir, "bin", "metasystem")
	completionEngineStub(t, engine, dir)
	owners := completionOwnersIn(dir)
	var cases strings.Builder
	var wants []string
	for index, row := range []struct {
		words  []string
		answer []string
		want   string
	}{
		{[]string{"work", "build", "G", "--brief=notes=v1/de"}, []string{":prefix 8", ":files"}, "files PREFIX=notes=v1/de IPREFIX=--brief="},
		{[]string{"work", "build", "G", "--brief", "notes=v1/de"}, []string{":files"}, "files PREFIX=notes=v1/de IPREFIX="},
		{[]string{"work", "build", "G", "--brief", "--draft=v1/de"}, []string{":files"}, "files PREFIX=--draft=v1/de IPREFIX="},
		{[]string{"design", "check", "notes=v1/de"}, []string{":files"}, "files PREFIX=notes=v1/de IPREFIX="},
	} {
		answer := completeLines(t, owners, row.words...)
		if !slices.Equal(answer, row.answer) {
			t.Fatalf("the Go walk answers %q for %q, want %q", answer, row.words, row.answer)
		}
		name := fmt.Sprintf("answer%d", index)
		completionWrite(t, filepath.Join(dir, name), strings.Join(answer, "\n")+"\n")
		quoted := []string{"'" + engine + "'"}
		for _, word := range row.words {
			quoted = append(quoted, "'"+word+"'")
		}
		fmt.Fprintf(&cases, "export ANSWER=./%s\nwords=(%s)\nCURRENT=%d; PREFIX=$words[CURRENT]; IPREFIX=''\nprint -r -- \"case %d\"\n_metasystem\n",
			name, strings.Join(quoted, " "), len(quoted), index)
		wants = append(wants, row.want)
	}
	out := zshGlueDriver(t, dir, cases.String())
	blocks := strings.Split(out, "case ")
	if len(blocks) != len(wants)+1 {
		t.Fatalf("driver output has %d cases:\n%s", len(blocks)-1, out)
	}
	for index, want := range wants {
		lines := strings.Split(strings.TrimSpace(blocks[index+1]), "\n")
		if last := lines[len(lines)-1]; last != want {
			t.Errorf("case %d: %q, want %q\n%s", index, last, want, out)
		}
	}
}

// TestBashGlueCompletesPathsWithSpaces (SOL-TC-01): on :files the bash glue
// looks up the current word unquoted and keeps each file name one candidate,
// and registers with -o filenames so bash escapes what it inserts.
func TestBashGlueCompletesPathsWithSpaces(t *testing.T) {
	t.Parallel()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("bash is not on PATH: %v", err)
	}
	dir := completionTemp(t)
	engine := filepath.Join(dir, "bin", "metasystem")
	completionEngineStub(t, engine, dir)
	answer := completeLines(t, completionOwnersIn(dir), "work", "build", "G", "--brief", "dir wi")
	if !slices.Equal(answer, []string{":files"}) {
		t.Fatalf("the Go walk answers %q, want :files", answer)
	}
	completionWrite(t, filepath.Join(dir, "answer"), ":files\n")
	completionWrite(t, filepath.Join(dir, "dir with space", "file.md"), "")
	completionWrite(t, filepath.Join(dir, "dirt.md"), "")
	_, glue, _ := routeWith((&sentinelFamilies{}).registry(), "system", "completion", "bash")
	completionWrite(t, filepath.Join(dir, "glue.bash"), glue)
	driver := `source ./glue.bash
complete -p metasystem
export ANSWER=./answer
try() { COMP_WORDS=('` + engine + `' work build G --brief "$1"); COMP_CWORD=5; _metasystem; echo "case ${#COMPREPLY[@]}:$(printf '[%s]\n' "${COMPREPLY[@]}" | sort | tr -d '\n')"; }
try 'dir\ wi'
try '"dir wi'
try "'dir wi"
try 'dir'
`
	completionWrite(t, filepath.Join(dir, "driver.bash"), driver)
	command := exec.Command(bash, "--norc", "--noprofile", "./driver.bash")
	command.Dir = dir
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("bash driver: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	want := []string{"complete -o filenames -F _metasystem metasystem",
		"case 1:[dir with space]", "case 1:[dir with space]", "case 1:[dir with space]", "case 2:[dir with space][dirt.md]"}
	if !slices.Equal(lines, want) {
		t.Errorf("bash glue answers\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}
