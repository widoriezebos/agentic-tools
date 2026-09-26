package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/metrics"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// intentBed is one isolated goal ledger driven through the public commands.
// The goal owners are the real ones; only the repository, clock, binding and
// human proof are the existing per-test fakes.
type intentBed struct {
	*goalBudgetResumeFixture
	t       *testing.T
	reports int
	lineage string
}

func newIntentBed(t *testing.T, stopped bool, amend func(*goal.GoalFile)) *intentBed {
	t.Helper()
	fixture := newGoalBudgetResumeFixture(t, stopped, amend)
	writeFixtureEnrollment(t, fixture.root(), "Wido")
	return &intentBed{goalBudgetResumeFixture: fixture, t: t}
}

func withinPath(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// fakeTop answers the repository top for paths under root, as Git would.
func fakeTop(root string) func(string) (string, error) {
	resolved, _ := filepath.EvalSymlinks(root)
	return func(path string) (string, error) {
		if withinPath(path, root) || withinPath(path, resolved) {
			return root, nil
		}
		return "", fmt.Errorf("fatal: not a git repository: %s", path)
	}
}

func noExecutable() (string, error) { return "", errors.New("tests select no executable installation") }

func (b *intentBed) owners() intentOwners {
	dependencies := b.dependencies()
	dependencies.ownerLineage = func() string { return b.lineage }
	return intentOwners{
		resolver:     stateroot.NewResolver(fakeTop(b.root()), noExecutable),
		prove:        fixedFixtureGoalAuthority,
		commandNow:   b.commandNow,
		dependencies: dependencies,
		binding:      b.binding,
		parkBranchCheck: func(string, goal.Endpoint) func(string, string) (string, error) {
			return func(string, string) (string, error) { return "", nil }
		},
		completion: completionInputs{
			localTip: func(string, string) (string, bool, error) { return "", false, nil },
			reporter: func(metrics.Options) (metrics.Result, error) { b.reports++; return metrics.Result{}, nil },
		},
	}
}

func (b *intentBed) run(owners intentOwners, args ...string) (int, string, string) {
	b.t.Helper()
	command, ok := findIntentCommand(args[0])
	if !ok {
		b.t.Fatalf("no public command %q", args[0])
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, args[1:], &stdout, &stderr, b.root(), owners)
	return code, stdout.String(), stderr.String()
}

func (b *intentBed) runJSON(owners intentOwners, args ...string) (int, intentResult) {
	b.t.Helper()
	code, stdout, stderr := b.run(owners, append(args, "--json")...)
	var result intentResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		b.t.Fatalf("%v printed no JSON result: %v; stdout=%q stderr=%q", args, err, stdout, stderr)
	}
	if result.SchemaVersion != 1 || result.Verb != args[0] || result.Outcome == "" || result.Summary == "" {
		b.t.Fatalf("%v envelope incomplete: %+v", args, result)
	}
	return code, result
}

func (b *intentBed) goalFile(id string) *goal.GoalFile {
	b.t.Helper()
	endpoint, err := b.dependencies().endpoint(b.root())
	if err != nil {
		b.t.Fatal(err)
	}
	projection, err := goal.Project(endpoint, false, time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC))
	if err != nil {
		b.t.Fatal(err)
	}
	file, _ := goalRecord(projection, id)
	if file == nil {
		b.t.Fatalf("goal %s is missing from the accepted ledger", id)
	}
	return file
}

// addGoal places one more goal on the accepted ledger.
func (b *intentBed) addGoal(file *goal.GoalFile) {
	b.t.Helper()
	path := "plans/goals/" + file.Id + ".md"
	rendered := goal.RenderFile(file)
	if err := os.WriteFile(filepath.Join(b.root(), filepath.FromSlash(path)), rendered, 0o644); err != nil {
		b.t.Fatal(err)
	}
	b.repo.commit(b.repo.accepted).files[path] = rendered
}

func (b *intentBed) setRoot(record *goal.RootRecord) {
	b.t.Helper()
	b.repo.commit(b.repo.accepted).files["plans/goals/backlog.md"] = goal.RenderRoot(record)
}

func queuedIntentGoal(id string, tier uint8) *goal.GoalFile {
	openedAt := "2026-08-30T08:00:00Z"
	file := &goal.GoalFile{
		Id: id, State: goal.StateQueued, Tier: tier, Intent: "Exercise " + id + ".", Origin: goal.OriginMain,
		NextStep: "Approve it.", OpenedAt: openedAt, Revision: 1,
		History: []goal.HistoryLine{{At: openedAt, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FB0", "mac-cli", "m1"),
			Verb: "open", Actor: "mac-cli+m1", Targets: []string{id}, Keep: -1}},
	}
	if tier != 0 {
		file.Risk = &goal.RiskRecord{Severity: tier, Novelty: tier, Exposure: 1, Accumulation: 1, Basis: "The fixture exercises a queued goal."}
	}
	return file
}

func makeQueued(file *goal.GoalFile) {
	file.State, file.Claimed, file.Approved, file.Budget, file.StopCapability = goal.StateQueued, nil, nil, nil, nil
}

func makeParked(file *goal.GoalFile) {
	makeQueued(file)
	file.State = goal.StateParked
	file.Parked = &goal.ParkRecord{By: "mac-cli+m1", At: "2026-08-30T08:30:00Z", Because: "waits for the design review"}
}

func tierBox(t *testing.T, root string, tier uint8) goal.Budget {
	t.Helper()
	box, err := config.TierBox(filepath.Join(root, "metasystem.conf"), tier)
	if err != nil {
		t.Fatal(err)
	}
	return box
}

const bedGoal = "standing-validation"

func TestIntentHelpAndCompatibility(t *testing.T) {
	t.Parallel()
	registered := families()
	code, root, problem := runCLIHelp([]string{"help"}, registered)
	if code != 0 || problem != "" || strings.Contains(root, "usage: metasystem <family> <verb>") {
		t.Fatalf("root help = code %d stderr %q; it must be the concise page: %q", code, problem, root)
	}
	for _, command := range intentCommands() {
		if !strings.Contains(root, "  "+command.name) {
			t.Errorf("root help does not list %s", command.name)
		}
		if isFamilyName(registered, command.name) {
			// ui is also a family: its own help and verbs win (checked below
			// with every family), so the public page is not routed by name.
			continue
		}
		code, page, problem := runCLIHelp([]string{"help", command.name}, registered)
		if code != 0 || problem != "" || !strings.Contains(page, command.usage[0]) || !strings.Contains(page, "--repo PATH") {
			t.Errorf("help %s = code %d stderr %q page %q", command.name, code, problem, page)
		}
		// Command help needs no repository, identity or writes.
		var stdout, stderr bytes.Buffer
		failing := intentOwners{resolver: stateroot.NewResolver(func(string) (string, error) { return "", errors.New("no repository") }, noExecutable)}
		if code := runIntentIn(command, []string{"some-goal", "--help"}, &stdout, &stderr, t.TempDir(), failing); code != 0 || stdout.String() != page {
			t.Errorf("%s --help = code %d stdout %q stderr %q", command.name, code, stdout.String(), stderr.String())
		}
	}
	// Commands whose implementation has not landed are not advertised.
	// start, enroll, fleet and doctor landed with the process slice; claim and
	// open with the planning slice; build, brief, fold and wait with the work
	// slice; review, land and close with the delivery slice.
	for _, absent := range []string{} {
		if strings.Contains(root, "\n  "+absent+" ") {
			t.Errorf("root help advertises unimplemented %s", absent)
		}
		if _, ok := findIntentCommand(absent); ok {
			t.Errorf("router accepts unimplemented %s", absent)
		}
	}
	for _, topic := range []string{"human", "agent", "internal"} {
		code, page, problem := runCLIHelp([]string{"help", topic}, registered)
		if code != 0 || problem != "" || page == "" {
			t.Errorf("help %s = code %d stderr %q", topic, code, problem)
		}
	}
	_, agent, _ := runCLIHelp([]string{"help", "agent"}, registered)
	if !strings.Contains(agent, "metasystem help internal") || strings.Contains(agent, "approve G") {
		t.Errorf("help agent must point at the internal catalogue and list only agent commands: %q", agent)
	}
	_, internal, _ := runCLIHelp([]string{"help", "internal"}, registered)
	var legacy bytes.Buffer
	writeUsage(&legacy, registered)
	if !strings.Contains(internal, legacy.String()) {
		t.Error("help internal does not carry the complete legacy catalogue")
	}
	// Every family keeps its help, directly and through the internal alias.
	for _, fam := range registered {
		var want bytes.Buffer
		if command, public := findIntentCommand(fam.name); public {
			writeIntentHelpWithFamily(&want, command, registered)
		} else {
			writeFamilyHelp(&want, fam)
		}
		var familyOnly bytes.Buffer
		writeFamilyHelp(&familyOnly, fam)
		for _, args := range [][]string{{"help", fam.name}, {fam.name, "--help"}, {"internal", fam.name, "--help"}} {
			expected := want.String()
			if args[0] == "internal" {
				expected = familyOnly.String()
			}
			if code, got, problem := runCLIHelp(args, registered); code != 0 || got != expected || problem != "" {
				t.Errorf("%v = code %d stderr %q, stdout equal %t", args, code, problem, got == expected)
			}
		}
	}
	// The alias routes to the family handler unchanged.
	called := []string{}
	fake := []family{{name: "safe", summary: "fixture", verbs: []verb{{"run", "records its arguments", func(args []string) int { called = args; return 7 }}}}}
	if code, _, _ := runCLIHelp([]string{"internal", "safe", "run", "--x", "y"}, fake); code != 7 || !slices.Equal(called, []string{"--x", "y"}) {
		t.Errorf("internal alias = code %d args %v", code, called)
	}
	if code, _, _ := runCLIHelp([]string{"safe", "run", "z"}, fake); code != 7 || !slices.Equal(called, []string{"z"}) {
		t.Errorf("direct family call = code %d args %v", code, called)
	}
	// The Partner catalogue is the public table plus every family, unchanged.
	catalogue := commandCatalogue()
	if len(catalogue) != len(registered)+1 || catalogue[0].Name != "metasystem" {
		t.Fatalf("catalogue has %d families, want %d with the public table first", len(catalogue), len(registered)+1)
	}
	names := func(index int) []string {
		var list []string
		for _, one := range catalogue[index].Verbs {
			list = append(list, one.Name)
		}
		return list
	}
	for _, want := range []string{"goals", "show", "approve", "budget", "pause", "resume", "done", "help", "internal"} {
		if !slices.Contains(names(0), want) {
			t.Errorf("public catalogue lacks %s", want)
		}
	}
	for index, fam := range registered {
		if catalogue[index+1].Name != fam.name || len(catalogue[index+1].Verbs) != len(fam.verbs) {
			t.Errorf("catalogue family %d = %s with %d verbs, want %s with %d", index+1, catalogue[index+1].Name, len(catalogue[index+1].Verbs), fam.name, len(fam.verbs))
		}
		if fam.name == "goal" {
			for _, want := range []string{"set-pin", "grant", "reopen", "list", "show"} {
				if !slices.Contains(names(index+1), want) {
					t.Errorf("internal goal family lost %s", want)
				}
			}
		}
	}
}

func TestIntentRepositorySelection(t *testing.T) {
	t.Parallel()
	writeTree := func(root string, files ...string) {
		for _, name := range files {
			path := filepath.Join(root, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("fixture\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	template := filepath.Join(base, "template repo")
	writeTree(template, "development/metasystem-design.md", "metasystem/metasystem.conf",
		"metasystem/scripts/agents/guard.sh", "metasystem/plans/goals/backlog.md", "docs/space dir/note.md")
	adopted := filepath.Join(base, "app")
	writeTree(adopted, "metasystem.conf", "scripts/agents/guard.sh", "plans/goals/backlog.md", "src/child/file.go")
	link := filepath.Join(base, "link-to-installation")
	if err := os.Symlink(filepath.Join(template, "metasystem"), link); err != nil {
		t.Fatal(err)
	}
	top := func(path string) (string, error) {
		for _, repository := range []string{template, adopted} {
			if withinPath(path, repository) {
				return repository, nil
			}
		}
		return "", fmt.Errorf("fatal: not a git repository: %s", path)
	}
	owners := intentOwners{resolver: stateroot.NewResolver(top, noExecutable)}
	selected := func(cwd string, args ...string) (string, *intentResult) {
		command, _ := findIntentCommand("goals")
		input, inputErr := parseIntentArgs(command, args)
		if inputErr != nil {
			t.Fatalf("%v: %s", args, inputErr.summary)
		}
		inv := &intentInvocation{command: command, input: input, cwd: cwd, owners: owners}
		problem := inv.selectRoot()
		return inv.stateRoot, problem
	}
	for _, test := range []struct {
		name, want, cwd string
		args            []string
	}{
		{"template top", filepath.Join(template, "metasystem"), template, nil},
		{"template installation", filepath.Join(template, "metasystem"), filepath.Join(template, "metasystem"), nil},
		{"template child", filepath.Join(template, "metasystem"), filepath.Join(template, "metasystem", "plans", "goals"), nil},
		{"directory with a space", filepath.Join(template, "metasystem"), filepath.Join(template, "docs", "space dir"), nil},
		{"file inside", filepath.Join(template, "metasystem"), base, []string{"--repo", filepath.Join(template, "docs", "space dir", "note.md")}},
		{"symbolic link", filepath.Join(template, "metasystem"), base, []string{"--repo", link}},
		{"relative --root", filepath.Join(template, "metasystem"), base, []string{"--root=template repo"}},
		{"adopted top", adopted, adopted, nil},
		{"adopted child", adopted, filepath.Join(adopted, "src", "child"), nil},
		{"adopted by --repo", adopted, template, []string{"--repo", filepath.Join(adopted, "src")}},
	} {
		got, problem := selected(test.cwd, test.args...)
		if problem != nil || got != test.want {
			t.Errorf("%s: state root %q problem %+v, want %q", test.name, got, problem, test.want)
		}
	}
	outside := filepath.Join(base, "elsewhere")
	writeTree(outside, "note.md")
	if got, problem := selected(outside); problem == nil || problem.code != 2 || problem.Decision == "" || got != "" {
		t.Errorf("a path outside every installation selected %q problem %+v", got, problem)
	}
	var stdout, stderr bytes.Buffer
	command, _ := findIntentCommand("goals")
	if code := runIntentIn(command, []string{"--json"}, &stdout, &stderr, outside, owners); code != 2 || !strings.Contains(stdout.String(), `"outcome": "refused"`) {
		t.Errorf("missing installation = code %d stdout %q", code, stdout.String())
	}
}

func TestIntentGoalAuthorityAndState(t *testing.T) {
	t.Parallel()
	human := []string{"--fixture-human-authority", "--lineage", "m1"}

	t.Run("reads", func(t *testing.T) {
		bed := newIntentBed(t, true, nil)
		code, result := bed.runJSON(bed.owners(), "show", bedGoal)
		if code != 0 || result.Outcome != intentConfirmed || result.Next == nil || result.Next.Argv[1] != "resume" {
			t.Fatalf("show of a stopped goal = %d %+v", code, result)
		}
		data := result.Data.(map[string]any)
		shown, _ := json.Marshal(data["budget"])
		code, budget := bed.runJSON(bed.owners(), "budget", "--id", bedGoal)
		read, _ := json.Marshal(budget.Data.(map[string]any)["budget"])
		if code != 0 || string(read) != string(shown) || !strings.Contains(string(read), `"box":"4h/4/240m/2/3"`) {
			t.Fatalf("budget read %s differs from show's block %s", read, shown)
		}
		code, list := bed.runJSON(bed.owners(), "goals", "--all")
		if code != 0 || list.Data.(map[string]any)["done"] == nil {
			t.Fatalf("goals --all did not include the archive: %+v", list)
		}
		if bed.repo.publications != 0 {
			t.Fatal("a read published")
		}
	})

	t.Run("queued approval under each tier's box in one act", func(t *testing.T) {
		bed := newIntentBed(t, false, makeQueued)
		bed.addGoal(queuedIntentGoal("second-goal", 2))
		code, result := bed.runJSON(bed.owners(), append([]string{"approve", bedGoal, "second-goal"}, human...)...)
		if code != 0 || result.Outcome != intentConfirmed || len(result.Targets) != 2 {
			t.Fatalf("approve = %d %+v", code, result)
		}
		for id, tier := range map[string]uint8{bedGoal: 3, "second-goal": 2} {
			file := bed.goalFile(id)
			if file.State != goal.StateApproved || file.Budget == nil || *file.Budget != tierBox(t, bed.root(), tier) || file.Approved == nil {
				t.Fatalf("%s after approval: state %s budget %+v approved %+v", id, file.State, file.Budget, file.Approved)
			}
		}
		if bed.repo.publications != 1 {
			t.Fatalf("approval published %d times, want one atomic act", bed.repo.publications)
		}
	})

	t.Run("wrong and foreign human proof refuse without effect", func(t *testing.T) {
		bed := newIntentBed(t, false, makeQueued)
		wrong := bed.owners()
		wrong.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
			return humanauthority.Proof{}, errors.New("process 42 is not the enrolled terminal")
		}
		code, result := bed.runJSON(wrong, "approve", bedGoal, "--lineage", "m1")
		if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "not the enrolled terminal") || result.Decision != "run this at the enrolled terminal" {
			t.Fatalf("wrong terminal = %d %+v", code, result)
		}
		foreign := bed.owners()
		other := t.TempDir()
		foreign.prove = func(_ string, pid int64, reader humanauthority.Reader, word, by string, now time.Time) (humanauthority.Proof, error) {
			return fixedFixtureGoalAuthority(other, pid, reader, word, by, now)
		}
		code, result = bed.runJSON(foreign, "approve", bedGoal, "--lineage", "m1")
		if code == 0 || result.Outcome != intentRefused {
			t.Fatalf("foreign proof = %d %+v", code, result)
		}
		if bed.repo.publications != 0 || bed.goalFile(bedGoal).State != goal.StateQueued {
			t.Fatal("a refused approval changed the ledger")
		}
	})

	t.Run("running goal takes a new box", func(t *testing.T) {
		bed := newIntentBed(t, false, func(file *goal.GoalFile) {
			file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1}
		})
		code, result := bed.runJSON(bed.owners(), append([]string{"budget", bedGoal, "5h/5/300m/2/3"}, human...)...)
		file := bed.goalFile(bedGoal)
		if code != 0 || result.Outcome != intentConfirmed || file.State != goal.StateClaimed || goalBoxString(file) != "5h/5/300m/2/3" {
			t.Fatalf("budget change = %d %+v, record %s %s", code, result, file.State, goalBoxString(file))
		}
	})

	t.Run("stopped goal refuses a changed box and resumes under its standing box", func(t *testing.T) {
		bed := newIntentBed(t, true, nil)
		code, result := bed.runJSON(bed.owners(), append([]string{"budget", bedGoal, "5h/5/300m/2/3"}, human...)...)
		if code != 1 || result.Outcome != intentRefused || result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "resume", bedGoal}) {
			t.Fatalf("changed box on a stopped goal = %d %+v", code, result)
		}
		if bed.repo.publications != 0 || bed.goalFile(bedGoal).StopFence == nil {
			t.Fatal("the refused box resumed the goal first")
		}
		bed.expectBindings(0)
		// The printed remedy is the next command, and it works.
		code, result = bed.runJSON(bed.owners(), append(result.Next.Argv[1:], human...)...)
		file := bed.goalFile(bedGoal)
		if code != 0 || result.Outcome != intentConfirmed || file.StopFence != nil || goalBoxString(file) != "4h/4/240m/2/3" {
			t.Fatalf("resume = %d %+v; fence %+v box %s", code, result, file.StopFence, goalBoxString(file))
		}
		bed.expectBindings(1)
		if bed.repo.publications != 1 {
			t.Fatalf("resume published %d times", bed.repo.publications)
		}
	})

	t.Run("parked goal resumes without approval", func(t *testing.T) {
		bed := newIntentBed(t, false, makeParked)
		code, result := bed.runJSON(bed.owners(), "resume", bedGoal, "--lineage", "m1")
		file := bed.goalFile(bedGoal)
		if code != 0 || result.Outcome != intentConfirmed || file.State != goal.StateQueued || file.Approved != nil || file.Budget != nil {
			t.Fatalf("unpark = %d %+v; record %s approved %+v", code, result, file.State, file.Approved)
		}
		code, result = bed.runJSON(bed.owners(), "resume", bedGoal, "--lineage", "m1")
		if code != 1 || result.Next == nil || result.Next.Argv[1] != "approve" {
			t.Fatalf("resume of a queued goal must name approval as its own act: %d %+v", code, result)
		}
	})

	t.Run("pause keeps the literal reason", func(t *testing.T) {
		bed := newIntentBed(t, false, makeQueued)
		reason := `-waits for "review" -- and it's quoted`
		code, result := bed.runJSON(bed.owners(), "pause", "--reason", reason, bedGoal, "--lineage", "m1")
		file := bed.goalFile(bedGoal)
		if code != 0 || result.Outcome != intentConfirmed || file.State != goal.StateParked || file.Parked == nil || file.Parked.Because != reason {
			t.Fatalf("pause = %d %+v; record %s %+v", code, result, file.State, file.Parked)
		}
	})

	t.Run("done checks its obligations then concludes", func(t *testing.T) {
		bed := newIntentBed(t, false, nil)
		code, result := bed.runJSON(bed.owners(), "done", bedGoal, "--reason", "shipped; residue remains in the retry path", "--lineage", "m1")
		if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "residue") || bed.repo.publications != 0 {
			t.Fatalf("done with unscheduled residue = %d %+v", code, result)
		}
		code, result = bed.runJSON(bed.owners(), "done", bedGoal, "--reason", "shipped and verified", "--lineage", "m1")
		if code != 0 || result.Outcome != intentConfirmed || bed.reports != 1 {
			t.Fatalf("done = %d %+v reports %d", code, result, bed.reports)
		}
		if file := bed.goalFile(bedGoal); file.State != goal.StateDone {
			t.Fatalf("done left %s", file.State)
		}
	})
}

func goalBoxString(file *goal.GoalFile) string {
	if file.Budget == nil {
		return ""
	}
	return goalbudget.FormatBox(*file.Budget)
}

func TestIntentArgumentsAndRemedies(t *testing.T) {
	t.Parallel()
	pause, _ := findIntentCommand("pause")
	for _, args := range [][]string{
		{"g", "--reason", "x y"},
		{"--reason", "x y", "g"},
		{"--reason=x y", "g"},
		{"-reason", "x y", "--", "g"},
		{"g", "--reason", "x y", "--reason=x y"},
		{"--id", "g", "--because", "x y"},
	} {
		input, problem := parseIntentArgs(pause, args)
		target := input.text("id")
		if len(input.args) > 0 {
			target = input.args[0]
		}
		if problem != nil || target != "g" || input.text("reason") != "x y" {
			t.Errorf("%q parsed to %+v problem %+v", args, input, problem)
		}
	}
	input, problem := parseIntentArgs(pause, []string{"--reason", "--literal", "--", "-g"})
	if problem != nil || input.text("reason") != "--literal" || !slices.Equal(input.args, []string{"-g"}) {
		t.Errorf("leading dashes were not kept literally: %+v %+v", input, problem)
	}

	bed := newIntentBed(t, false, makeQueued)
	// Conflicts, unknown flags and missing inputs refuse before any owner.
	code, result := bed.runJSON(bed.owners(), "pause", bedGoal, "--reason", "a", "--reason", "b", "--lineage", "m1")
	if code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "a and b") {
		t.Fatalf("conflicting duplicates = %d %+v", code, result)
	}
	code, result = bed.runJSON(bed.owners(), "pause", bedGoal, "other-goal", "--lineage", "m1")
	if code != 2 || !strings.Contains(result.Summary, "at most 1") {
		t.Fatalf("extra positional = %d %+v", code, result)
	}
	code, result = bed.runJSON(bed.owners(), "pause", "--reason", "x", "--lineage", "m1")
	if code != 2 || !strings.Contains(result.Summary, "needs a goal") || !strings.Contains(fmt.Sprint(result.Data), bedGoal) {
		t.Fatalf("missing target = %d %+v", code, result)
	}
	unenrolled := goalSyncTerminalReader(t, bed.root(), "ttys:not_enrolled")
	bed.facts.reader = &unenrolled
	code, result = bed.runJSON(bed.owners(), "pause", bedGoal, "--reason", "x")
	if code != 1 || !strings.Contains(result.Decision, "--lineage") {
		t.Fatalf("unknown actor = %d %+v", code, result)
	}
	reason := "it's -waiting"
	code, result = bed.runJSON(bed.owners(), "pause", bedGoal, "--reson", reason, "--lineage", "m1")
	if code != 2 || result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "pause", bedGoal, "--reason", reason, "--lineage", "m1", "--json"}) {
		t.Fatalf("typo = %d %+v", code, result)
	}
	if bed.repo.publications != 0 || bed.goalFile(bedGoal).State != goal.StateQueued {
		t.Fatal("an input mistake changed the ledger")
	}
	// The offered correction is the command that works.
	code, result = bed.runJSON(bed.owners(), result.Next.Argv[1:len(result.Next.Argv)-1]...)
	if file := bed.goalFile(bedGoal); code != 0 || result.Outcome != intentConfirmed || file.Parked == nil || file.Parked.Because != reason {
		t.Fatalf("corrected command = %d %+v", code, result)
	}
	// Text output: the refusal and its remedy on standard error, quoted.
	code, stdout, stderr := bed.run(bed.owners(), "show", "no-such-goal")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "run: metasystem goals --all") {
		t.Fatalf("unknown goal text = %d %q %q", code, stdout, stderr)
	}
	code, stdout, _ = bed.run(bed.owners(), "show", bedGoal)
	if code != 0 || !strings.Contains(stdout, bedGoal+"  parked  tier 3") || !strings.Contains(stdout, "next: metasystem resume "+bedGoal) {
		t.Fatalf("show text = %d %q", code, stdout)
	}
	if got := shellWords(shellCommand([]string{"metasystem", "goal", "budget", "--by", "it's me", "", "1d/2/3m/1/0"})); !slices.Equal(got, []string{"metasystem", "goal", "budget", "--by", "it's me", "", "1d/2/3m/1/0"}) {
		t.Fatalf("shellWords did not read back shellCommand: %q", got)
	}
}

func TestIntentResumeAttorney(t *testing.T) {
	t.Parallel()
	attorneyID := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC0", "mac-cli", "m1")
	grant := &goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1,
		PowerOfAttorney: []goal.PowerOfAttorneyEntry{{
			ID: attorneyID, By: "human:Wido", Tiers: []uint8{1}, Verbs: []string{"unpark"},
			Since: "2026-09-01T08:00:00Z", Expires: "2026-09-05",
		}},
	}
	// The owner's law: an attorney lifts a person's park on tier-one goals.
	tierOne := func(file *goal.GoalFile) {
		file.Tier = 1
		file.Risk = &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "The fixture is a tier-one goal."}
	}
	parked := newIntentBed(t, false, func(file *goal.GoalFile) { makeParked(file); tierOne(file) })
	parked.setRoot(grant)
	code, result := parked.runJSON(parked.owners(), "resume", bedGoal, "--under", attorneyID, "--verified", "the review landed", "--lineage", "m1")
	file := parked.goalFile(bedGoal)
	if code != 0 || result.Outcome != intentConfirmed || file.State != goal.StateQueued || file.Approved != nil {
		t.Fatalf("unpark under attorney = %d %+v; record %s approved %+v", code, result, file.State, file.Approved)
	}
	if last := file.History[len(file.History)-1]; last.Verb != "unpark" || !strings.Contains(fmt.Sprint(last), attorneyID) {
		t.Fatalf("the unpark does not name its power of attorney: %+v", last)
	}

	stopped := newIntentBed(t, true, nil)
	stopped.setRoot(grant)
	code, result = stopped.runJSON(stopped.owners(), "resume", bedGoal, "--under", attorneyID, "--verified", "the review landed", "--lineage", "m1")
	if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "power of attorney") || !strings.Contains(result.Decision, "metasystem resume "+bedGoal) {
		t.Fatalf("stopped resume under attorney = %d %+v", code, result)
	}
	if stopped.repo.publications != 0 || stopped.goalFile(bedGoal).StopFence == nil {
		t.Fatal("a refused attorney resume changed the stopped goal")
	}
	stopped.expectBindings(0)
}

func TestIntentTierlessApprovalRemedy(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, makeQueued)
	bed.addGoal(queuedIntentGoal("tierless-goal", 0))
	code, result := bed.runJSON(bed.owners(), "approve", bedGoal, "tierless-goal", "--fixture-human-authority", "--lineage", "m1")
	if code != 1 || result.Outcome != intentRefused || result.Next != nil {
		t.Fatalf("tierless approval = %d %+v", code, result)
	}
	for _, want := range []string{"severity", "novelty", "exposure", "accumulation", "basis"} {
		if !strings.Contains(result.Decision, want) {
			t.Errorf("the missing decision does not name %s: %q", want, result.Decision)
		}
	}
	if strings.Contains(result.Decision, "--tier") || strings.Contains(result.Decision, " N ") {
		t.Errorf("the remedy invents a tier: %q", result.Decision)
	}
	if bed.repo.publications != 0 || bed.goalFile(bedGoal).State != goal.StateQueued {
		t.Fatal("a tierless member approved part of the set")
	}
}

func humanOrigin(file *goal.GoalFile) { file.Origin = goal.OriginHuman }

func TestIntentDoneNeedsTheHumansProof(t *testing.T) {
	t.Parallel()
	// A typed name beside the owning session's lineage is not the person.
	forged := newIntentBed(t, false, humanOrigin)
	owners := forged.owners()
	owners.dependencies.proveHuman = func(string, int64, humanauthority.Reader, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("an agent process is an ancestor of this shell")
	}
	owners.dependencies.proveTerminal = func(string, int64, humanauthority.Reader, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("an agent process is an ancestor of this shell")
	}
	code, result := forged.runJSON(owners, "done", bedGoal, "--reason", "closed", "--by", "Wido", "--lineage", "m1")
	if code == 0 || result.Outcome != intentRefused || forged.repo.publications != 0 || forged.goalFile(bedGoal).State != goal.StateClaimed {
		t.Fatalf("forged human done = %d %+v", code, result)
	}
	// The owning agent without a name cannot conclude the person's goal either.
	code, result = forged.runJSON(forged.owners(), "done", bedGoal, "--reason", "closed", "--lineage", "m1")
	if code == 0 || !strings.Contains(result.Summary, "opened by the human") || forged.repo.publications != 0 {
		t.Fatalf("agent done of a human-origin goal = %d %+v", code, result)
	}
	// The person at the enrolled terminal concludes it with no name typed.
	human := newIntentBed(t, false, humanOrigin)
	_, reader := enrollGoalSyncTerminal(t, human.root(), "ttys:fixture_done")
	human.facts.reader = &reader
	code, result = human.runJSON(human.owners(), "done", bedGoal, "--reason", "closed at the terminal")
	if code != 0 || result.Outcome != intentConfirmed || human.goalFile(bedGoal).State != goal.StateDone {
		t.Fatalf("enrolled human done = %d %+v", code, result)
	}
}

func TestIntentHumanPauseNeedsNoTypedName(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, makeQueued)
	_, reader := enrollGoalSyncTerminal(t, bed.root(), "ttys:fixture_pause")
	bed.facts.reader = &reader
	code, result := bed.runJSON(bed.owners(), "pause", bedGoal, "--reason", "waits for Wido")
	file := bed.goalFile(bedGoal)
	if code != 0 || result.Outcome != intentConfirmed || file.State != goal.StateParked || file.Parked == nil || !strings.Contains(file.Parked.By, "Wido") {
		t.Fatalf("enrolled human pause = %d %+v; record %s %+v", code, result, file.State, file.Parked)
	}

	wrong := newIntentBed(t, false, makeQueued)
	enrollGoalSyncTerminal(t, wrong.root(), "ttys:fixture_enrolled")
	other := goalSyncTerminalReader(t, wrong.root(), "ttys:fixture_other")
	wrong.facts.reader = &other
	code, result = wrong.runJSON(wrong.owners(), "pause", bedGoal, "--reason", "waits for Wido")
	if code != 1 || result.Outcome != intentRefused || wrong.repo.publications != 0 || wrong.goalFile(bedGoal).State != goal.StateQueued {
		t.Fatalf("pause from a shell outside the enrolled terminal = %d %+v", code, result)
	}
}

func TestIntentAttorneyBudgetKeepsAuthorityInputs(t *testing.T) {
	t.Parallel()
	attorneyID := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC1", "mac-cli", "m1")
	grant := &goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1,
		PowerOfAttorney: []goal.PowerOfAttorneyEntry{{
			ID: attorneyID, By: "human:Wido", Tiers: []uint8{1}, Verbs: []string{"approve", "set-budget"},
			Since: "2026-09-01T08:00:00Z", Expires: "2026-09-05",
		}},
	}
	tierOne := func(file *goal.GoalFile) {
		file.Tier = 1
		file.Risk = &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "The fixture is a tier-one goal."}
		if file.Approved != nil {
			file.Approved.Digest = goal.ApprovalDigest(file.Intent, 1, *file.Budget, file.Risk)
		}
	}
	for _, test := range []struct {
		name  string
		amend func(*goal.GoalFile)
		extra []string
	}{
		{"queued approval with a typed name", func(file *goal.GoalFile) { makeQueued(file); tierOne(file) }, []string{"--by", "Wido"}},
		{"running box with a human proof", func(file *goal.GoalFile) {
			tierOne(file)
			file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1}
		}, []string{"--fixture-human-authority"}},
	} {
		bed := newIntentBed(t, false, test.amend)
		bed.setRoot(grant)
		before := bed.goalFile(bedGoal)
		code, result := bed.runJSON(bed.owners(), append([]string{"budget", bedGoal, "4h/4/240m/2/1", "--under", attorneyID, "--lineage", "m1"}, test.extra...)...)
		after := bed.goalFile(bedGoal)
		if code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "--under is the seat's own act") ||
			bed.repo.publications != 0 || after.State != before.State || goalBoxString(after) != goalBoxString(before) {
			t.Fatalf("%s: code %d %+v", test.name, code, result)
		}
	}
}

func TestIntentTierlessBudgetNormRefuses(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, func(file *goal.GoalFile) { makeQueued(file); file.Tier, file.Risk = 0, nil })
	code, result := bed.runJSON(bed.owners(), "budget", bedGoal, "norm", "--fixture-human-authority", "--lineage", "m1")
	if code != 1 || result.Outcome != intentRefused || result.Next != nil {
		t.Fatalf("tierless budget norm = %d %+v", code, result)
	}
	for _, want := range []string{"severity", "novelty", "exposure", "accumulation", "basis"} {
		if !strings.Contains(result.Decision, want) {
			t.Errorf("the missing decision does not name %s: %q", want, result.Decision)
		}
	}
	if file := bed.goalFile(bedGoal); bed.repo.publications != 0 || file.State != goal.StateQueued || file.Budget != nil {
		t.Fatal("a tierless norm box was granted")
	}
}

func TestIntentLandedActsReportPartialFollowUp(t *testing.T) {
	t.Parallel()
	t.Run("branch sweep", func(t *testing.T) {
		bed := newIntentBed(t, false, nil)
		owners := bed.owners()
		owners.completion.localTip = func(string, string) (string, bool, error) {
			return "", false, errors.New("the local branch ref is unreadable")
		}
		code, result := bed.runJSON(owners, "done", bedGoal, "--reason", "shipped", "--lineage", "m1")
		data, _ := result.Data.(map[string]any)
		if code != 1 || result.Outcome != intentPartial || !strings.Contains(result.Summary, "branch was not swept") || data["owner"] == nil ||
			bed.goalFile(bedGoal).State != goal.StateDone || bed.repo.publications != 1 {
			t.Fatalf("done with a failed sweep = %d %+v", code, result)
		}
	})
	t.Run("metrics report", func(t *testing.T) {
		bed := newIntentBed(t, false, nil)
		owners := bed.owners()
		owners.completion.reporter = func(metrics.Options) (metrics.Result, error) { return metrics.Result{}, errors.New("disk full") }
		code, result := bed.runJSON(owners, "done", bedGoal, "--reason", "shipped", "--lineage", "m1")
		data, _ := result.Data.(map[string]any)
		if code != 1 || result.Outcome != intentPartial || !strings.Contains(fmt.Sprint(data["incomplete"]), "disk full") ||
			result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "metrics", "report", "--goal", bedGoal}) ||
			bed.goalFile(bedGoal).State != goal.StateDone {
			t.Fatalf("done with a failed metrics report = %d %+v", code, result)
		}
	})
	t.Run("approval proof record", func(t *testing.T) {
		bed := newIntentBed(t, false, makeQueued)
		blocked := filepath.Join(bed.root(), "artifacts", "agents", "authority", "proofs")
		if err := os.WriteFile(blocked, []byte("not a directory\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		code, result := bed.runJSON(bed.owners(), "approve", bedGoal, "--fixture-human-authority", "--lineage", "m1")
		data, _ := result.Data.(map[string]any)
		if code != 1 || result.Outcome != intentPartial || data["owner"] == nil || !strings.Contains(result.Decision, "do not run it again") ||
			bed.goalFile(bedGoal).State != goal.StateApproved || bed.repo.publications != 1 {
			t.Fatalf("approval whose proof record failed = %d %+v", code, result)
		}
	})
}

func TestIntentParkedResumeActors(t *testing.T) {
	t.Parallel()
	t.Run("enrolled human without a typed name", func(t *testing.T) {
		bed := newIntentBed(t, false, makeParked)
		_, reader := enrollGoalSyncTerminal(t, bed.root(), "ttys:fixture_resume")
		bed.facts.reader = &reader
		code, result := bed.runJSON(bed.owners(), "resume", bedGoal)
		file := bed.goalFile(bedGoal)
		if code != 0 || result.Outcome != intentConfirmed || file.State != goal.StateQueued || file.Approved != nil || bed.repo.publications != 1 {
			t.Fatalf("enrolled human resume = %d %+v; record %s", code, result, file.State)
		}
	})
	t.Run("wrong terminal", func(t *testing.T) {
		bed := newIntentBed(t, false, makeParked)
		enrollGoalSyncTerminal(t, bed.root(), "ttys:fixture_enrolled")
		other := goalSyncTerminalReader(t, bed.root(), "ttys:fixture_elsewhere")
		bed.facts.reader = &other
		code, result := bed.runJSON(bed.owners(), "resume", bedGoal)
		if code != 1 || result.Outcome != intentRefused || bed.repo.publications != 0 || bed.goalFile(bedGoal).State != goal.StateParked {
			t.Fatalf("resume outside the enrolled terminal = %d %+v", code, result)
		}
	})
	for _, flags := range [][]string{
		{"--temporary-human-word", "Wido says resume it"},
		{"--review-by", "2026-09-10"},
		{"--temporary-human-word", "Wido says resume it", "--review-by", "2026-09-10"},
	} {
		t.Run("stopped-only "+flags[0]+fmt.Sprint(len(flags)), func(t *testing.T) {
			bed := newIntentBed(t, false, makeParked)
			code, result := bed.runJSON(bed.owners(), append([]string{"resume", bedGoal, "--lineage", "m1"}, flags...)...)
			if code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "parked goal's unpark takes neither") ||
				bed.repo.publications != 0 || bed.goalFile(bedGoal).State != goal.StateParked {
				t.Fatalf("parked resume with %v = %d %+v", flags, code, result)
			}
		})
	}
	t.Run("fixture authority on the exact fake root", func(t *testing.T) {
		bed := newIntentBed(t, false, makeParked)
		code, result := bed.runJSON(bed.owners(), "resume", bedGoal, "--fixture-human-authority", "--lineage", "m1")
		if code != 0 || result.Outcome != intentConfirmed || bed.goalFile(bedGoal).State != goal.StateQueued {
			t.Fatalf("fixture parked resume = %d %+v", code, result)
		}
		if _, err := parseSyncFlagValues("unpark", []string{"--id", bedGoal, "--fixture-human-authority"}); err != nil {
			t.Fatalf("goal unpark does not accept the fixture proof flag: %v", err)
		}
		// The fixture proof stays bound to a fake-runtime checkout.
		real := t.TempDir()
		if err := os.WriteFile(filepath.Join(real, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := proveFixtureGoalAuthorityAt("unpark", &syncFlags{root: real, fixtureHumanAuthority: true}, func(string) (time.Time, error) {
			return time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC), nil
		}); err == nil {
			t.Fatal("fixture authority was granted outside a fake-runtime root")
		}
	})
}
