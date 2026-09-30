package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stoptransition"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// The output-style audit (plans/designs/output-style.md §5): each converted
// verb's default output is a golden under testdata/layout, driven in-process
// with fake owners; its --json is a golden taken from main before the
// conversion, so a conversion cannot move a field.

// updateLayoutJSON rewrites the --json goldens. They are main's output: run
// it only on a tree whose render is main's.
var updateLayoutJSON = flag.Bool("update-layout-json", false, "rewrite testdata/layout/*.json from this tree")

// layoutNow is the goldens' clock: Wednesday 30 September 2026, 10:58 CEST.
var layoutNow = time.Date(2026, 9, 30, 8, 58, 0, 0, time.UTC)

// layoutCase is one golden: a verb, its fixture, and the stable names its
// temporary paths and ids are replaced with.
type layoutCase struct {
	name string
	args []string
	bed  func(t *testing.T) layoutBed
	// help draws a help page instead of running a verb; help pages carry
	// no --json of this shape and are always enforced.
	help func(env textui.Env) *textui.Page
	// measured names why a case's --json is not byte-stable (a duration it
	// measures), so the JSON golden is not kept for it.
	measured string
	// noJSON is a passthrough action that takes no --json.
	noJSON bool
	// fileSystem names why a case's goldens hold what the macOS file system
	// allocates and reports (st_blocks sizes, no file generation): on
	// another system the bytes are not compared; the shape rules still run.
	fileSystem string
}

// layoutBytesCompared says whether this system prints a case's goldens
// byte for byte.
func layoutBytesCompared(t *testing.T, c layoutCase) bool {
	if c.fileSystem != "" && runtime.GOOS != "darwin" {
		t.Logf("%s: goldens not compared on %s: %s", c.name, runtime.GOOS, c.fileSystem)
		return false
	}
	return true
}

// addLayoutCases adds a group's cases from its own file through the group
// hook, so parallel builders never edit one list.
func addLayoutCases(cases ...layoutCase) bool {
	layoutGroupCases = append(layoutGroupCases, func() []layoutCase { return cases })
	return true
}

type layoutBed struct {
	owners  intentOwners
	cwd     string
	replace []string  // old, new pairs applied to every output
	now     time.Time // the bed's clock; zero is layoutNow
	// words fill a case's placeholder words with the bed's values (a
	// temporary directory), the way replace takes them out again.
	words map[string]string
}

// layoutGroupCases are the goldens a conversion group adds from its own
// file, so parallel groups never edit one list.
var layoutGroupCases []func() []layoutCase

func layoutCases() []layoutCase {
	cases := []layoutCase{
		{name: "status", args: []string{"status"}, bed: statusLayoutBed(true)},
		{name: "status-verbose", args: []string{"status", "--verbose"}, bed: statusLayoutBed(true)},
		{name: "status-quiet", args: []string{"status"}, bed: statusLayoutBed(false)},
		{name: "status-refusal", args: []string{"status", "goal-a", "goal-b"}, bed: statusLayoutBed(false)},
		{name: "grant-list", args: []string{"grant", "list"}, bed: grantListLayoutBed(1, false)},
		{name: "grant-list-all", args: []string{"grant", "list", "--all"}, bed: grantListLayoutBed(2, true)},
		{name: "grant-list-empty", args: []string{"grant", "list"}, bed: grantListLayoutBed(0, false)},
		{name: "grant-list-refusal", args: []string{"grant", "list"}, bed: outsideLayoutBed},
		// The help pages (§6.12, 6.13): the top level, an object grouped by
		// intent and one listed plainly, and action pages, one of them with
		// administration forms and forms wider than the page.
		{name: "help", help: intentRootHelpPage},
		{name: "help-goal", help: objectHelpLayout("goal")},
		{name: "help-system", help: objectHelpLayout("system")},
		{name: "help-ui", help: objectHelpLayout("ui")},
		{name: "help-goal-approve", help: actionHelpLayout("goal approve")},
		{name: "help-system-stop", help: actionHelpLayout("system stop")},
		{name: "help-work-land", help: actionHelpLayout("work land")},
		{name: "help-status", help: actionHelpLayout("status")},
	}
	for _, group := range layoutGroupCases {
		cases = append(cases, group()...)
	}
	return cases
}

func objectHelpLayout(object string) func(textui.Env) *textui.Page {
	return func(env textui.Env) *textui.Page { return intentObjectHelpPage(env, object) }
}

func actionHelpLayout(name string) func(textui.Env) *textui.Page {
	return func(env textui.Env) *textui.Page {
		command, ok := findIntentCommand(name)
		if !ok {
			panic("no public command " + name)
		}
		return intentCommandHelpPage(env, command)
	}
}

// helpLayoutBed is no bed: a help page reads nothing.
func helpLayoutBed(t *testing.T) layoutBed { return layoutBed{cwd: t.TempDir()} }

// statusLayoutBed is a checkout with its five helpers and two processes
// that are not ours, the host board, a landing lane with one batch
// collecting, and (busy) the helm taken and a general grant live.
func statusLayoutBed(busy bool) func(t *testing.T) layoutBed {
	return func(t *testing.T) layoutBed {
		b := newProcessBed(t)
		root := realpath.Resolve(b.root())
		if busy {
			item := func(family, component string, pid int64, line string) *machineItem {
				return machineFixtureItem(family, component, "", pid, line)
			}
			b.families = []stoptransition.Family{
				&machineFamily{name: "steward", items: []*machineItem{item("steward", "steward-runner", 39052, "steward-runner pid 39052 started 1790758391: running")}},
				&machineFamily{name: "supervision", items: []*machineItem{
					item("supervision", "supervision-owner", 39060, "supervision-owner pid 39060 tag metasystem-supervision-owner-m1e-1790758391-38939 generation 205: running"),
					item("supervision", "repo-watcher", 39061, "repo-watcher pid 39061: running"),
					item("supervision", "job-reaper", 39062, "job-reaper pid 39062: running"),
					item("supervision", "landing-batch-owner", 39063, "landing-batch-owner pid 39063: running")}},
				&machineFamily{name: "untracked", items: []*machineItem{
					item("untracked", "untracked", 2878, "untracked pid 2878 claude claude --allow-dangerously-skip-permissions: running"),
					item("untracked", "untracked", 10947, "untracked pid 10947 codex codex app-server: running")}},
			}
		}
		owners := b.owners()
		owners.commandNow = func(string) (time.Time, error) { return layoutNow, nil }
		owners.helm.zone = layoutZone(t)
		owners.helm.now = func() time.Time { return layoutNow }
		owners.delivery.now = func() time.Time { return layoutNow }
		owners.delivery.boardView = func(string, time.Time) board.View {
			return board.View{Readable: true, Bridge: "live", Seats: []board.SeatView{
				{Machine: "landing", Goals: []board.GoalView{}},
				{Machine: "m1e", Goals: []board.GoalView{{Goal: "switch-on-trial", Unknown: "not claimed", LastProgressAt: layoutNow.Add(-time.Minute)}}},
				{Machine: "ui", Goals: []board.GoalView{}}}}
		}
		owners.helm.machine = func(string) (string, error) { return "m1e", nil }
		base := t.TempDir()
		home, landing := filepath.Join(base, "home"), filepath.Join(filepath.Dir(root), "agentic-tools-landing")
		for _, dir := range []string{home, landing} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		home, landing = realpath.Resolve(home), realpath.Resolve(landing)
		records := []batch.Record{}
		owners.landing = laneVerbOwners{
			home: func() (string, error) { return home, nil },
			probe: func(string) (lane.OwnerProbe, error) {
				return lane.OwnerProbe{Alive: true, PID: 38928, Since: layoutNow.Add(-5 * time.Minute)}, nil
			},
			records: func(string) ([]batch.Record, error) { return records, nil },
			now:     func() time.Time { return layoutNow },
		}
		owners.delivery.batchRoot = func(string, time.Time) (string, bool, error) { return "", false, nil }
		if busy {
			registerLane(t, home, landing, "Wido", layoutNow.Add(-2*time.Hour))
			collecting := batch.Record{Schema: 1, BatchID: "4gr18nm8t3nyev9sssda9jgtsq", State: batch.StateOpen,
				Units:   []batch.Unit{{GoalID: "533209e6d", Chain: "c", SeatRoot: root, State: batch.UnitJoined, Claim: batch.Claim{Machine: "m1e"}}},
				History: []batch.HistoryEntry{{At: layoutNow.Add(-time.Minute).Format(time.RFC3339Nano), Verb: "open", To: batch.StateOpen}}}
			records = append(records, collecting)
			if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
			if _, err := helm.Write(root, helm.Record{By: "wido", At: time.Date(2026, 9, 28, 10, 39, 0, 0, time.UTC).Format(time.RFC3339),
				Reason: "coordinating the verb and machinery batches", Checkout: root, Leader: "login", LeaderRef: "34285@1790591981"}); err != nil {
				t.Fatal(err)
			}
			entry := goal.PowerOfAttorneyEntry{ID: "YY6RQ7765HX224V3NT4TN67JFR-m1e-b6a4eb0a", By: "human:wido", Verbs: []string{goal.GeneralAct},
				Since: layoutNow.Add(-2 * time.Minute).Format(time.RFC3339), For: "m1e", Checkout: root, Lineage: "lin-main",
				Until: time.Date(2026, 10, 1, 8, 56, 0, 0, time.UTC).Format(time.RFC3339)}
			owners.attorney.entries = func(string) ([]goal.PowerOfAttorneyEntry, error) { return []goal.PowerOfAttorneyEntry{entry}, nil }
		} else {
			owners.attorney.entries = func(string) ([]goal.PowerOfAttorneyEntry, error) { return nil, nil }
		}
		return layoutBed{owners: owners, cwd: b.root(), replace: layoutPaths(b.root(), root, "/Users/wido/GitHub/agentic-tools-m1e",
			home, "/Users/wido/.metasystem-home", landing, "/Users/wido/GitHub/agentic-tools-landing", "~/agentic-tools-landing", "~/GitHub/agentic-tools-landing")}
	}
}

// grantListLayoutBed is a ledger holding count general grants, the first
// revoked when revoke is set.
func grantListLayoutBed(count int, revoke bool) func(t *testing.T) layoutBed {
	return func(t *testing.T) layoutBed {
		b := newGrantEverythingBed(t)
		owners := b.owners()
		owners.helm.zone = layoutZone(t)
		var replace []string
		for index, spell := range []string{"24h", "8h"}[:count] {
			// Each grant is made a minute after the one before, the last now, so
			// the ledger's order (by start, then by the random id) holds.
			acting := owners
			acting.commandNow = func(root string) (time.Time, error) {
				at, err := b.commandNow(root)
				return at.Add(time.Duration(index-count+1) * time.Minute), err
			}
			code, result := b.runJSON(acting, "grant", "add", "--acts", "everything", "--for", spell)
			if code != 0 || len(result.Targets) != 1 {
				t.Fatalf("grant add = %d %+v", code, result)
			}
			id := result.Targets[0].ID
			replace = append(replace, id, []string{"YY6RQ7765HX224V3NT4TN67JFR-m1e-b6a4eb0a", "01K6D2S9W0Q5Y3M7N8P4R2T6V1-m1e-7c1d9e20"}[index])
			if index == 0 && revoke {
				if code, result := b.runJSON(owners, "grant", "revoke", id); code != 0 {
					t.Fatalf("grant revoke = %d %+v", code, result)
				}
			}
		}
		root := realpath.Resolve(b.root())
		now, err := b.commandNow(b.root())
		if err != nil {
			t.Fatal(err)
		}
		return layoutBed{owners: owners, cwd: b.root(), now: now, replace: append(replace, layoutPaths(b.root(), root, "/Users/wido/GitHub/agentic-tools-m1e")...)}
	}
}

// notInRepository is a fake repository lookup's refusal: its words are the
// fake's, its type stateroot.ErrNotInRepository's, as the real lookup's.
type notInRepository struct{ words string }

func (e notInRepository) Error() string      { return e.words }
func (notInRepository) Is(target error) bool { return target == stateroot.ErrNotInRepository }

// outsideLayoutBed runs from the file system's root, which is no
// repository.
func outsideLayoutBed(t *testing.T) layoutBed {
	notARepository := func(string) (string, error) { return "", notInRepository{"not a git repository"} }
	b := newIntentBed(t, false, nil)
	owners := b.owners()
	owners.resolver = stateroot.NewResolver(notARepository, noExecutable)
	return layoutBed{owners: owners, cwd: "/"}
}

// layoutPaths are the replacements of a bed's paths: its root as given and
// resolved, then further old/new pairs.
func layoutPaths(given, resolved, stable string, more ...string) []string {
	pairs := []string{resolved, stable}
	if given != resolved {
		pairs = append(pairs, given, stable)
	}
	return append(pairs, more...)
}

func layoutZone(t *testing.T) *time.Location {
	zone, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Fatal(err)
	}
	return zone
}

func runLayoutCase(t *testing.T, c layoutCase, bed layoutBed, args ...string) (int, string, string) {
	t.Helper()
	if c.help != nil {
		env := layoutEnv(t, bed.now, "", "")
		if bed.owners.textEnv != nil {
			env = bed.owners.textEnv(nil)
		}
		return 0, c.help(env).String(), ""
	}
	words := append(append([]string{}, c.args...), args...)
	for index, word := range words {
		if value, ok := bed.words[word]; ok {
			words[index] = value
		}
	}
	command, rest, ok := resolveIntentArgv(words)
	if !ok {
		t.Fatalf("no public command %q", c.args)
	}
	stdout, stderr := &layoutStream{env: bed.owners.textEnv}, &layoutStream{env: bed.owners.textEnv}
	var code int
	if command.run == nil && command.passthrough != nil {
		// A passthrough prints from its own handler, in the layout its
		// stream carries; its bed names the installation in the words.
		code = command.passthrough(rest, stdout, stderr)
	} else {
		code = runIntentIn(command, rest, stdout, stderr, bed.cwd, bed.owners)
	}
	replacer := strings.NewReplacer(bed.replace...)
	return code, replacer.Replace(stdout.String()), replacer.Replace(stderr.String())
}

// layoutStream is a golden's output stream: it carries the golden's text
// layout to a passthrough handler, which has no owners to ask.
type layoutStream struct {
	bytes.Buffer
	env func(io.Writer) textui.Env
}

func (s *layoutStream) TextEnv() textui.Env {
	if s.env == nil {
		return textui.DetectWith(false, 0, func(string) string { return "" }, layoutNow, time.UTC)
	}
	return s.env(s)
}

func layoutGolden(name string) string { return filepath.Join("testdata", "layout", name) }

// TestAuditOutputLayoutJSONUnchanged: every golden verb's --json is byte for
// byte what main printed before its text was converted.
func TestAuditOutputLayoutJSONUnchanged(t *testing.T) {
	t.Parallel()
	for _, c := range layoutCases() {
		if c.help != nil || c.measured != "" || c.noJSON {
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			code, stdout, stderr := runLayoutCase(t, c, c.bed(t), "--json")
			got := strings.Join([]string{"exit " + strconv.Itoa(code), stdout, stderr}, "\n--\n")
			path := layoutGolden(c.name + ".json")
			if *updateLayoutJSON {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (write main's with -update-layout-json)", err)
			}
			if got != string(want) && layoutBytesCompared(t, c) {
				t.Errorf("%s --json moved:\n%s\nmain printed:\n%s", c.name, got, want)
			}
		})
	}
}

// updateLayout rewrites the text goldens from this tree.
var updateLayout = flag.Bool("update-layout", false, "rewrite testdata/layout/*.txt from this tree")

// layoutEnv is the goldens' fixed text layout (§5): width 100, no colour,
// the symbols, the fixed clock in Amsterdam, and the bed's own home and
// checkout for short paths.
func layoutEnv(t *testing.T, now time.Time, home, repo string) textui.Env {
	if now.IsZero() {
		now = layoutNow
	}
	return textui.Env{Width: textui.MaxWidth, Now: now, Zone: layoutZone(t), Home: home, Repo: repo, InRepo: true}
}

// TestAuditOutputLayout checks the shape of every golden verb's default
// output (§5 rules 1 to 5) and compares it with its golden; a verb whose
// run function the mode table enforces fails on a broken rule, the others
// log it. The coverage and static rules run over the command table and the
// source (rules 6 and 7).
func TestAuditOutputLayout(t *testing.T) {
	t.Parallel()
	for _, c := range layoutCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.bed == nil {
				c.bed = helpLayoutBed
			}
			bed, _ := layoutPrepared(t, c, false)
			_, stdout, stderr := runLayoutCase(t, c, bed)
			got := stdout + stderr
			path := layoutGolden(c.name + ".txt")
			if *updateLayout {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			} else if want, err := os.ReadFile(path); (err != nil || got != string(want)) && layoutBytesCompared(t, c) {
				t.Errorf("%s printed:\n%s\nthe golden %s holds:\n%s (%v)", c.name, got, path, want, err)
			}
			problems := layoutProblems(got, textui.MaxWidth, bed.replace)
			if mode := layoutModeOf(t, c); mode == auditEnforce {
				for _, problem := range problems {
					t.Errorf("%s: %s", c.name, problem)
				}
			} else if len(problems) > 0 {
				t.Logf("%s (report): %s", c.name, strings.Join(problems, "; "))
			}

			// On a terminal the same page is coloured; with colour off not
			// one escape is printed.
			// A fresh bed: a verb that acts meets the same world twice.
			again, _ := layoutPrepared(t, c, true)
			_, coloured, colouredErr := runLayoutCase(t, c, again)
			if plain := layoutStripANSI(coloured + colouredErr); plain != got && layoutModeOf(t, c) == auditEnforce {
				t.Errorf("%s: the coloured page is not the plain page in colour:\n%s", c.name, plain)
			}
			if strings.Contains(got, "\x1b") {
				t.Errorf("%s: an escape without colour: %q", c.name, got)
			}
		})
	}
}

// layoutPrepared is a fresh bed for one case, laid out by the goldens'
// fixed layout (on a terminal when tty is set), with the replacements of
// its short paths.
func layoutPrepared(t *testing.T, c layoutCase, tty bool) (layoutBed, textui.Env) {
	t.Helper()
	bed := c.bed(t)
	root := realpath.Resolve(bed.cwd)
	env := layoutEnv(t, bed.now, filepath.Dir(root), root)
	env.TTY, env.Color = tty, tty
	bed.owners.textEnv = func(io.Writer) textui.Env { return env }
	bed.replace = append([]string{"~/" + filepath.Base(root), "~/GitHub/" + layoutStableBase(bed.replace, root)}, bed.replace...)
	return bed, env
}

// layoutStableBase is the stable name a bed's root is replaced with.
func layoutStableBase(replace []string, root string) string {
	for index := 0; index+1 < len(replace); index += 2 {
		if replace[index] == root {
			return filepath.Base(replace[index+1])
		}
	}
	return filepath.Base(root)
}

var layoutANSI = regexp.MustCompile("\x1b\\[[0-9;]*m")

func layoutStripANSI(text string) string { return layoutANSI.ReplaceAllString(text, "") }

// layoutModeOf is a verb's layout mode: its run function's, by the table.
// Help pages are always enforced.
func layoutModeOf(t *testing.T, c layoutCase) string {
	t.Helper()
	if c.help != nil {
		return auditEnforce
	}
	command, _, ok := resolveIntentArgv(c.args)
	if !ok {
		t.Fatalf("no public command %q", c.args)
	}
	file, function := layoutRunSource(command)
	return layoutModeFor(file, function)
}

// layoutRunSource is the module-relative file and name of the function a
// public command runs: its handler, or a passthrough's owner.
func layoutRunSource(command intentCommand) (string, string) {
	var pointer uintptr
	switch {
	case command.owner != nil:
		pointer = reflect.ValueOf(command.owner).Pointer()
	case command.run != nil:
		pointer = reflect.ValueOf(command.run).Pointer()
	default:
		return "", ""
	}
	fn := runtime.FuncForPC(pointer)
	if fn == nil {
		return "", ""
	}
	file, _ := fn.FileLine(pointer)
	// main.runX, or path/to/cmd/metasystem.runX in a test binary.
	name := fn.Name()
	name = name[strings.LastIndex(name, "/")+1:]
	_, name, _ = strings.Cut(name, ".")
	file = filepath.ToSlash(file)
	if index := strings.LastIndex(file, "/cmd/metasystem/"); index >= 0 {
		file = file[index+1:]
	} else if index := strings.LastIndex(file, "cmd/metasystem/"); index >= 0 {
		file = file[index:]
	}
	return file, name
}

var (
	layoutEpoch         = regexp.MustCompile(`\b1[6-9]\d{8}\b`)
	layoutHex           = regexp.MustCompile(`\b[0-9a-f]{40}(?:[0-9a-f]{24})?\b`)
	layoutUTC           = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2}(\.\d+)?)?Z\b`)
	layoutKeyValues     = regexp.MustCompile(`\b[a-z][a-zA-Z_-]*=\S+\s+[a-z][a-zA-Z_-]*=\S+`)
	layoutErrorChain    = regexp.MustCompile(`\b(open|stat|read|lstat) /\S*: no such file`)
	layoutSymbols       = "●○!✗✓?"
	layoutBannerSymbols = "●!?"
	// layoutLoneCommand is a line that holds one command a person pastes,
	// after its indent and a usage key: P9's lone token, never broken.
	layoutLoneCommand = regexp.MustCompile(`^\s*(usage\s+)?metasystem \S`)
)

// layoutProblems are the §5 shape rules a page breaks: the headline first
// (after an optional banner), the width, no raw dumps, breathing room.
// replace names the bed's stable paths: printed whole they are a path the
// page should have shortened.
func layoutProblems(page string, width int, replace []string) []string {
	var problems []string
	if page == "" {
		return []string{"prints nothing"}
	}
	if !strings.HasSuffix(page, "\n") {
		problems = append(problems, "the last line has no newline")
	}
	lines := strings.Split(strings.TrimSuffix(page, "\n"), "\n")
	headline := 0
	// A banner is attention (!, ?) or a live condition (●); an act's ✓ or
	// a refusal's ✗ is the headline itself.
	if first := []rune(lines[0]); len(first) > 1 && strings.ContainsRune(layoutBannerSymbols, first[0]) && first[1] == ' ' {
		for index, line := range lines {
			if line == "" {
				headline = index + 1
				break
			}
			if index > 0 && !strings.HasPrefix(line, "  ") && !(len([]rune(line)) > 1 && strings.ContainsRune(layoutSymbols, []rune(line)[0])) {
				break
			}
		}
	}
	if headline < len(lines) {
		line := lines[headline]
		switch {
		case line == "" || strings.HasPrefix(line, " "):
			problems = append(problems, "the headline is not at column 0")
		case strings.HasPrefix(line, "usage:"):
			problems = append(problems, "the page opens with usage")
		case layoutKeyValues.MatchString(line):
			problems = append(problems, "the headline is a key=value run")
		}
	}
	for index, line := range lines {
		if n := utf8.RuneCountInString(line); n > width && len(strings.Fields(line)) > 1 && !layoutLoneCommand.MatchString(line) && !layoutFixedLine(line) {
			problems = append(problems, fmt.Sprintf("line %d is %d columns: %q", index+1, n, line))
		}
		if strings.HasSuffix(line, " ") {
			problems = append(problems, fmt.Sprintf("line %d ends in a space", index+1))
		}
		if line == "" && (index == 0 || index == len(lines)-1 || lines[index-1] == "") {
			problems = append(problems, fmt.Sprintf("line %d is a blank line at an edge or a second one", index+1))
		}
		for _, rule := range []struct {
			pattern *regexp.Regexp
			what    string
		}{{layoutEpoch, "a Unix epoch"}, {layoutHex, "a whole hex digest"}, {layoutUTC, "a UTC stamp"}, {layoutKeyValues, "a key=value run"}, {layoutErrorChain, "a Go error chain"}} {
			if rule.pattern.MatchString(line) {
				problems = append(problems, fmt.Sprintf("line %d holds %s: %q", index+1, rule.what, line))
			}
		}
		if strings.Contains(line, "(s)") {
			problems = append(problems, fmt.Sprintf("line %d counts with (s): %q", index+1, line))
		}
		for pair := 0; pair+1 < len(replace); pair += 2 {
			if stable := replace[pair+1]; strings.HasPrefix(stable, "/Users/wido/") && strings.Contains(line, stable) {
				problems = append(problems, fmt.Sprintf("line %d holds %s whole, not ~ or repo-relative", index+1, stable))
			}
		}
	}
	return problems
}

// The shape rules catch what they name.
func TestAuditOutputLayoutRulesCatchTheirTriggers(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ page, want string }{
		{"a headline\n", ""},
		{"! the helm is taken\n  → metasystem helm return\n\nm1e is running\n", ""},
		{"✓ wido has the helm\n\n  terminal   the enrolled one\n", ""},
		{"! the helm is taken\n\n  indented\n", "column 0"},
		{"  indented\n", "column 0"},
		{"usage: metasystem goal\n", "usage"},
		{"claimed=3 approved=8\n", "key=value"},
		{"x " + strings.Repeat("y", 100) + "\n", "columns"},
		{"a headline\n  usage   metasystem goal approve" + strings.Repeat(" G", 40) + "\n", ""},
		{"a headline\n  metasystem work land" + strings.Repeat(" --x", 30) + "\n", ""},
		{"a headline\n  the words metasystem" + strings.Repeat(" y", 50) + "\n", "columns"},
		{strings.Repeat("y", 120) + "\n", ""},
		{"started 1790758391\n", "epoch"},
		{"tip " + strings.Repeat("a", 40) + "\n", "hex"},
		{"at 2026-09-30T08:53:15Z\n", "UTC"},
		{"0 item(s)\n", "(s)"},
		{"a\n\n\nb\n", "second one"},
		{"a \n", "space"},
		{"x: open /tmp/y: no such file or directory\n", "error chain"},
		{"checkout /Users/wido/GitHub/agentic-tools-m1e\n", "whole"},
		{"no newline", "newline"},
		{"", "nothing"},
	} {
		problems := strings.Join(layoutProblems(c.page, 100, []string{"/tmp/m1e", "/Users/wido/GitHub/agentic-tools-m1e"}), "; ")
		if (c.want == "") != (problems == "") || !strings.Contains(problems, c.want) {
			t.Errorf("%q: problems %q, want one naming %q", c.page, problems, c.want)
		}
	}
}

// Rule 6: every public action is either enforced with a golden or
// reported with the group that owns its run function's file.
func TestAuditOutputLayoutCoversEveryPublicAction(t *testing.T) {
	t.Parallel()
	golden := map[string]bool{}
	for _, c := range layoutCases() {
		if c.help != nil {
			continue
		}
		command, _, _ := resolveIntentArgv(c.args)
		golden[command.name] = true
	}
	groups := map[string][]string{}
	for _, command := range publicIntentCommands() {
		file, function := layoutRunSource(command)
		group := verbGroup(command.name)
		if group == "" {
			group = auditGroupFor(file, function)
		}
		switch mode := layoutModeFor(file, function); {
		case mode == auditEnforce && !golden[command.name]:
			t.Errorf("%s is enforced but has no golden under testdata/layout", command.name)
		case mode == auditReport && group == "":
			t.Errorf("%s runs %s in %s, which no group of the mode table owns", command.name, function, file)
		default:
			groups[group+" "+mode] = append(groups[group+" "+mode], command.name)
		}
	}
	for key, names := range groups {
		t.Logf("%s: %s", key, strings.Join(names, ", "))
	}
}

// Rule 7: a function or file the table enforces writes nothing to the
// invocation's streams itself (the view is the only writer), and no
// source outside textui writes a raw escape.
func TestAuditOutputLayoutStatic(t *testing.T) {
	t.Parallel()
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	err = filepath.WalkDir(module, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return walkErr
		}
		rel, _ := filepath.Rel(module, path)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "internal/textui/") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(data), `\x1b[`) || strings.Contains(string(data), `\033[`) {
			t.Errorf("%s writes a raw terminal escape; colour belongs to textui", rel)
		}
		if !strings.HasPrefix(rel, "cmd/metasystem/") {
			return nil
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, data, 0)
		if parseErr != nil {
			return parseErr
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := fn.Name.Name
			if fn.Recv != nil && len(fn.Recv.List) == 1 {
				receiver := fn.Recv.List[0].Type
				if star, ok := receiver.(*ast.StarExpr); ok {
					receiver = star.X
				}
				if ident, ok := receiver.(*ast.Ident); ok {
					name = ident.Name + "." + name
				}
			}
			if layoutModeFor(rel, name) != auditEnforce {
				continue
			}
			checked++
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if pkg, isIdent := selectorPackage(selector); !ok || !isIdent || pkg != "fmt" || !strings.HasPrefix(selector.Sel.Name, "Fprint") {
					return true
				}
				if stream, ok := call.Args[0].(*ast.SelectorExpr); ok && (stream.Sel.Name == "stdout" || stream.Sel.Name == "stderr") {
					t.Errorf("%s#%s writes to the invocation's %s itself; the view is the only writer", rel, name, stream.Sel.Name)
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no enforced function was checked; the table or the scan is broken")
	}
}

func selectorPackage(selector *ast.SelectorExpr) (string, bool) {
	if selector == nil {
		return "", false
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	return ident.Name, true
}

// layoutFixedLine is a peer message's or note's fixed frame line, which
// the seat's agent reads verbatim: printed whole, so exempt from the width
// rule (Wido's ruling, 2026-09-30).
func layoutFixedLine(line string) bool {
	for _, frame := range []string{board.Preface, board.NotePreface, board.NoteClosing} {
		if fixed, _, _ := strings.Cut(frame, "%"); strings.HasPrefix(line, fixed) {
			return true
		}
	}
	return false
}
