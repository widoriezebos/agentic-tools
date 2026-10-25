package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// deadlineCheckProcesses runs each unit check launch as a check that reached
// proof.deadline: it leaves an observation of its own launch and exits 1. A
// launch with no outcomes passes.
type deadlineCheckProcesses struct {
	workProcesses
	bed      *workBed
	outcomes func(id string) []string
	launches []string
}

func (p *deadlineCheckProcesses) StartChild(c launch.Command) (launch.Child, identity.Ref, error) {
	var outcomes []string
	for dir := filepath.Dir(c.LogPath); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		record, err := p.bed.manager.Store.Read(filepath.Base(dir))
		if err != nil || record.Kind != "proof" || len(record.Inputs) == 0 {
			continue
		}
		p.launches = append(p.launches, record.ID)
		if outcomes = p.outcomes(record.ID); len(outcomes) > 0 {
			name := fmt.Sprintf("check-%d", len(p.launches))
			execution := launch.CheckExecution{ExecutionID: name, Role: "builder"}
			for _, outcome := range outcomes {
				execution.Steps = append(execution.Steps, processmeasure.Step{Parent: record.ID, Start: fmt.Sprintf("2026-10-23T09:%02d:00Z", len(p.launches)), Outcome: outcome})
			}
			directory := filepath.Join(filepath.Dir(filepath.Dir(record.Inputs[0].Path)), name)
			data, _ := json.Marshal(execution)
			if err := os.MkdirAll(directory, 0700); err != nil {
				return nil, identity.Ref{}, err
			}
			if err := os.WriteFile(filepath.Join(directory, "observation.json"), data, 0600); err != nil {
				return nil, identity.Ref{}, err
			}
		}
		break
	}
	return failureChild{passed: len(outcomes) == 0}, workProcessRef(20), nil
}

func everyCheckDeadline(string) []string { return []string{"deadline", "passed"} }

// deadlineCheckBed is a work bed whose goal allows attempts and whose host
// lane proves while lane is true.
func deadlineCheckBed(t *testing.T, attempts uint64, outcomes func(string) []string) (*workBed, *deadlineCheckProcesses, *bool) {
	t.Helper()
	b := newWorkBedWith(t, func(file *goal.GoalFile) {
		file.Budget = &goal.Budget{ElapsedLimit: "4h", AttemptLimit: attempts, ReservedJobMinutesLimit: 1000, ActiveJobLimit: 2}
		workApprovedBox(file)
	})
	lane := new(bool)
	b.workOwnersHook = func(o *intentWorkOwners) {
		units := o.units
		o.units = func(layout stateroot.Layout) *launch.UnitRunner {
			r := units(layout)
			r.Git = outcomeGit{workGit{b}, "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n+new\n"}
			return r
		}
		o.laneProof = func() *plain.RunningProof {
			if *lane {
				return &plain.RunningProof{State: "running"}
			}
			return nil
		}
	}
	p := &deadlineCheckProcesses{bed: b, outcomes: outcomes}
	b.manager.Processes = p
	b.manager.Adapters["plain-exec"] = launch.PlainExec{}
	b.manager.Supervisor = failureSupervisor{bed: b}
	return b, p, lane
}

// enrolledWork runs a public verb from the person's own enrolled terminal.
func enrolledWork(t *testing.T, b *workBed, args ...string) (int, intentResult, string) {
	t.Helper()
	owners := b.workOwners()
	owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
	command, rest, _ := resolveIntentArgv(args)
	var out, stderr bytes.Buffer
	code := runIntentIn(command, append([]string{"--json"}, rest...), &out, &stderr, b.root(), owners)
	var result intentResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("%v printed no JSON result: %v; %s %s", args, err, out.String(), stderr.String())
	}
	return code, result, stderr.String()
}

// agentWork runs a public verb from a shell an agent started.
func agentWork(t *testing.T, b *workBed, args ...string) (int, intentResult) {
	t.Helper()
	owners := b.workOwners()
	owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("an agent started this shell")
	}
	command, rest, _ := resolveIntentArgv(args)
	var out, stderr bytes.Buffer
	code := runIntentIn(command, append([]string{"--json"}, rest...), &out, &stderr, b.root(), owners)
	var result intentResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("%v printed no JSON result: %v; %s %s", args, err, out.String(), stderr.String())
	}
	return code, result
}

// onlyRun is the bed's one unit run, read from its store.
func onlyRun(t *testing.T, b *workBed) launch.UnitRunRecord {
	t.Helper()
	runs, err := filepath.Glob(filepath.Join(b.unitRoot, "*", "run.json"))
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs=%v err=%v", runs, err)
	}
	record, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(filepath.Base(filepath.Dir(runs[0])))
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func agentBuild(t *testing.T, b *workBed) (int, intentResult) {
	t.Helper()
	return agentWork(t, b, "work", "build", b.id, "outcome", "--brief", b.brief("build.md", "Build the declared requirement.\n"), "--lines", "1")
}

func reservation(t *testing.T, b *workBed, id string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(b.root(), "artifacts", "agents", "jobs", id+".json"))
	if os.IsNotExist(err) {
		return nil
	}
	var record map[string]any
	if err != nil || json.Unmarshal(data, &record) != nil {
		t.Fatalf("reservation %s: %s %v", id, data, err)
	}
	return record
}

func TestUnitCheckRetryWaitsForTheLaneProof(t *testing.T) {
	t.Parallel()
	t.Run("machine", func(t *testing.T) {
		t.Parallel()
		b, p, lane := deadlineCheckBed(t, 4, everyCheckDeadline)
		*lane = true
		code, held := agentBuild(t, b)
		r := onlyRun(t, b)
		step := r.Rounds[0].Steps[1]
		if code != 1 || held.Outcome != intentRefused || !strings.HasSuffix(held.Summary, "; the unit check waits for the host") || step.State != launch.StepStarting || len(step.LaunchIDs) != 2 || !strings.HasSuffix(step.LaunchID, "-retry") || step.RetryBy != "machine" || len(p.launches) != 1 {
			t.Fatalf("the retry started while the lane proves: %d %+v step=%+v launches=%v", code, held, step, p.launches)
		}
		*lane = false
		code, stopped := agentWork(t, b, "work", "build", "run:"+r.ID)
		r = onlyRun(t, b)
		if code != 1 || !slices.Equal(p.launches, step.LaunchIDs) || r.Rounds[0].Stop == nil || r.Rounds[0].Stop.Handoff != "stopped deadline" {
			t.Fatalf("the idle lane did not start the retry: %d %+v launches=%v", code, stopped, p.launches)
		}
		questions, damaged := channel.WalkOpenQuestions(b.root())
		if len(damaged) != 0 || len(questions) != 1 || !slices.Contains(questions[0].UnitStop.AcceptableActs, "work-review-retry") {
			t.Fatalf("the second deadline did not ask one question: %+v %v", questions, damaged)
		}
	})
	t.Run("person-review", func(t *testing.T) {
		t.Parallel()
		b, p, lane := deadlineCheckBed(t, 4, func(id string) []string {
			if strings.HasSuffix(id, "-retry-2") {
				return nil
			}
			return everyCheckDeadline(id)
		})
		code, stopped := agentBuild(t, b)
		r := onlyRun(t, b)
		if code != 1 || len(p.launches) != 2 || r.Rounds[0].Stop == nil {
			t.Fatalf("no second deadline: %d %+v launches=%v", code, stopped, p.launches)
		}
		*lane = true
		b.manager.CapacitySources.Load = loadedHost
		code, retried, output := b.work("work", "review", "run:"+r.ID, "--reason", "The host is quiet again", "--by", "Wido")
		if code != 0 || len(p.launches) != 3 || !strings.HasSuffix(p.launches[2], "-retry-2") || onlyRun(t, b).Rounds[0].Outcome != "green" {
			t.Fatalf("the person's retry did not start the check: %d %+v launches=%v %s", code, retried, p.launches, output)
		}
	})
	t.Run("held-retry", func(t *testing.T) {
		t.Parallel()
		b, p, lane := deadlineCheckBed(t, 4, func(id string) []string {
			if strings.HasSuffix(id, "-retry") {
				return nil
			}
			return everyCheckDeadline(id)
		})
		*lane = true
		code, held := agentBuild(t, b)
		r := onlyRun(t, b)
		retry := r.Rounds[0].Steps[1].LaunchID
		if code != 1 || r.Rounds[0].Steps[1].State != launch.StepStarting || len(p.launches) != 1 {
			t.Fatalf("the retry was not held: %d %+v launches=%v", code, held, p.launches)
		}
		code, continued, output := enrolledWork(t, b, "work", "build", "run:"+r.ID, "--by", "Wido")
		r = onlyRun(t, b)
		if code != 0 || len(p.launches) != 2 || p.launches[1] != retry || len(r.Rounds[0].Steps[1].LaunchIDs) != 2 || r.Rounds[0].Steps[1].State != launch.StepPassed {
			t.Fatalf("a person's continuation did not start the held retry: %d %+v launches=%v %s", code, continued, p.launches, output)
		}
	})
}

func TestUnitCheckRetryIsChargedAsAnAttempt(t *testing.T) {
	t.Parallel()
	t.Run("at-the-limit", func(t *testing.T) {
		t.Parallel()
		b, p, _ := deadlineCheckBed(t, 1, everyCheckDeadline)
		code, refused := agentBuild(t, b)
		r := onlyRun(t, b)
		step := r.Rounds[0].Steps[1]
		details := strings.Join(refused.Details, "\n")
		if code != 1 || refused.Outcome != intentRefused || refused.Next == nil || shellCommand(refused.Next.Argv) != shellCommand([]string{"metasystem", "goal", "budget", b.id, "BOX"}) || !slices.Contains(refused.Targets, intentTarget{Kind: "goal", ID: b.id}) {
			t.Fatalf("the retry's refusal did not name the budget remedy: %d %+v", code, refused)
		}
		if !strings.Contains(details, "BUDGET_REFUSED") || !strings.Contains(details, "attemptLimit used=1 limit=1") || strings.Contains(details, "reservedJobMinutesLimit") {
			t.Fatalf("the refusal does not say the attempts are used up: %s", details)
		}
		if step.State != launch.StepStarting || step.RetryBy != "machine" || len(step.LaunchIDs) != 2 || len(p.launches) != 1 || reservation(t, b, step.LaunchID) != nil {
			t.Fatalf("a refused retry launched or reserved: step=%+v launches=%v", step, p.launches)
		}
		projection := dispatchcore.ProjectBudget(b.root(), b.goalFile(b.id), b.manager.Now())
		if projection.Status != dispatchcore.BudgetKnown || projection.Attempts != projection.Limits.AttemptLimit || len(projection.Breaches) != 0 {
			t.Fatalf("the goal's attempts: %+v", projection)
		}
	})
	t.Run("charged", func(t *testing.T) {
		t.Parallel()
		b, p, lane := deadlineCheckBed(t, 4, func(id string) []string {
			if strings.HasSuffix(id, "-retry-2") {
				return nil
			}
			return everyCheckDeadline(id)
		})
		*lane = true
		code, _ := agentBuild(t, b)
		r := onlyRun(t, b)
		original, retry := r.Rounds[0].Steps[1].LaunchIDs[0], r.Rounds[0].Steps[1].LaunchIDs[1]
		if code != 1 || reservation(t, b, retry)["unitRetryAttempt"] != true || reservation(t, b, original)["unitEnvironment"] != true || reservation(t, b, original)["unitRetryAttempt"] != nil {
			t.Fatalf("the machine retry is not charged, or its first deadline not excluded: %+v %+v", reservation(t, b, retry), reservation(t, b, original))
		}
		// A known-flake repeat keeps the machine's RetryBy under a -retry-2
		// launch; the machine does not charge it. The held retry never ran,
		// so the repeat stands in its place.
		r.Rounds[0].Steps[1].LaunchID = original + "-retry-2"
		r.Rounds[0].Steps[1].LaunchIDs = []string{original, original + "-retry-2"}
		r.Rounds[0].Steps[1].FlakeRepeat = true
		data, _ := json.Marshal(r)
		if err := os.WriteFile(filepath.Join(b.unitRoot, r.ID, "run.json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
		*lane = false
		if code, repeated := agentWork(t, b, "work", "build", "run:"+r.ID); code != 0 || len(p.launches) != 2 {
			t.Fatalf("the flake repeat did not run: %d %s %v launches=%v", code, repeated.Summary, repeated.Details, p.launches)
		}
		if repeat := reservation(t, b, original+"-retry-2"); repeat == nil || repeat["unitRetryAttempt"] != nil {
			t.Fatalf("the flake repeat was charged as the machine's retry: %+v", repeat)
		}
	})
	t.Run("person", func(t *testing.T) {
		t.Parallel()
		b, p, _ := deadlineCheckBed(t, 4, func(id string) []string {
			if strings.HasSuffix(id, "-retry-2") {
				return nil
			}
			return everyCheckDeadline(id)
		})
		agentBuild(t, b)
		r := onlyRun(t, b)
		if code, _, output := b.work("work", "review", "run:"+r.ID, "--reason", "Retry the stopped check", "--by", "Wido"); code != 0 || len(p.launches) != 3 {
			t.Fatalf("the person's retry did not run: %d launches=%v %s", code, p.launches, output)
		}
		if retry := reservation(t, b, p.launches[2]); retry == nil || retry["unitRetryAttempt"] != nil {
			t.Fatalf("the person's retry was charged as the machine's: %+v", retry)
		}
	})
}

// partialBuildBed is a work bed whose builder reports PARTIAL with message
// and leaves a diff of lines changed lines.
func partialBuildBed(t *testing.T, lines int, message string) *workBed {
	t.Helper()
	diff := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n"
	for line := range lines {
		diff += fmt.Sprintf("+line %d\n", line)
	}
	b := outcomeBed(t, diff)
	b.manager.Settings.BuildLinesCap = 1500
	b.manager.Supervisor = partialStarter{bed: b, message: message}
	return b
}

type partialStarter struct {
	bed     *workBed
	message string
	// block puts a folder where the correction brief would be written.
	block bool
}

func (s partialStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	r, _ := s.bed.manager.Store.Read(id)
	if r.Kind == "build" && s.block {
		rounds, _ := filepath.Glob(filepath.Join(s.bed.unitRoot, "*", "round-1"))
		for _, round := range rounds {
			if err := os.MkdirAll(filepath.Join(round, "correction-brief.md"), 0700); err != nil {
				return identity.Ref{}, err
			}
		}
	}
	if r.Kind == "build" {
		result, _ := json.Marshal(map[string]any{"type": "result", "result": s.message, "is_error": false})
		for name, data := range map[string][]byte{"last-message.txt": []byte(s.message), "result.json": result} {
			if err := os.WriteFile(filepath.Join(state, name), data, 0600); err != nil {
				return identity.Ref{}, err
			}
		}
	}
	return s.bed.starter.StartSupervisor(id, state)
}

func enrol(t *testing.T, b *workBed, human string) {
	t.Helper()
	enrollment := humanauthority.Enrollment{Schema: 1, EnrolledAt: time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC), Generation: 1, Human: human, TerminalID: "ttys001",
		TerminalRef: humanauthority.ProcessRef{PID: 10, PIDStartedAt: 1}, SessionLeader: humanauthority.ProcessRef{PID: 11, PIDStartedAt: 1}}
	data, _ := json.Marshal(enrollment)
	path := filepath.Join(b.root(), "artifacts", "agents", "authority", "human-terminal.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := humanauthority.ReadEnrollment(b.root()); err != nil || got.Human != human {
		t.Fatalf("enrolment fixture: %+v %v", got, err)
	}
}

// unenrol sets the bed's enrolment aside, so nobody is known.
func unenrol(t *testing.T, b *workBed) {
	t.Helper()
	path := filepath.Join(b.root(), "artifacts", "agents", "authority", "human-terminal.json")
	if err := os.Rename(path, path+".aside"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func partialQuestion(t *testing.T, b *workBed) (channel.Question, launch.UnitRunRecord) {
	t.Helper()
	code, held := agentWork(t, b, "work", "build", b.id, "outcome", "--brief", b.brief("build.md", "Build the declared requirement.\n"), "--lines", "750")
	r := onlyRun(t, b)
	questions, damaged := channel.WalkOpenQuestions(b.root())
	if code != 1 || r.Rounds[0].Outcome != "build-partial" || len(damaged) != 0 || len(questions) != 1 {
		t.Fatalf("the partial build did not ask one question: %d %+v %+v %v", code, held, questions, damaged)
	}
	return questions[0], r
}

const partialMessage = "I built the parser and its tests.\n\nThe error table is still open: it needs the codes from the design.\nRESULT: PARTIAL\n"

func TestFailedBuildQuestionSaysWhatHappened(t *testing.T) {
	t.Parallel()
	for _, enrolled := range []bool{true, false} {
		t.Run(fmt.Sprintf("enrolled=%t", enrolled), func(t *testing.T) {
			t.Parallel()
			b := partialBuildBed(t, 474, partialMessage)
			if enrolled {
				enrol(t, b, "Wido")
			} else {
				unenrol(t, b)
			}
			q, r := partialQuestion(t, b)
			command := channel.ActCommand(q)
			last := "The builder's last words: The error table is still open: it needs the codes from the design.\nRESULT: PARTIAL"
			if q.Plain == nil || q.Plain.Decision != "Unit outcome of goal "+b.id+" stopped after build round 1: "+r.Rounds[0].Stop.Class || slices.Contains(q.Facts, r.Rounds[0].Stop.Class) {
				t.Fatalf("the decision does not name the class: %+v", q)
			}
			if !slices.Contains(q.Facts, "474 changed lines against 750 declared") || !slices.Contains(q.Facts, last) {
				t.Fatalf("the facts do not count or quote the build: %q", q.Facts)
			}
			words := shellWords(command)
			if slices.Contains(words, "TEXT") || slices.Contains(words, "NAME") != !enrolled || enrolled && !slices.Contains(words, "Wido") {
				t.Fatalf("the command keeps its placeholders (enrolled %t): %s", enrolled, command)
			}
		})
	}
}

func TestLastParagraph(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	long := strings.Repeat("é", 400)
	result, _ := json.Marshal(map[string]any{"result": "First words.\n\nLast words of the result."})
	for _, row := range []struct{ name, data, want string }{
		{"last-message.txt", "First paragraph.\n\n  \nThe last paragraph,\nin two lines.\n\n\n", "The last paragraph,\nin two lines."},
		{"result.json", string(result), "Last words of the result."},
		{"long.txt", "Before.\n\n" + long, strings.Repeat("é", 300)},
		{"invalid.txt", "Before.\n\n" + strings.Repeat("\x80", 700), strings.Repeat("\x80", 600)},
		{"missing.txt", "", "The builder left no readable last message."},
	} {
		path := filepath.Join(directory, row.name)
		if row.name != "missing.txt" {
			if err := os.WriteFile(path, []byte(row.data), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if got := lastParagraph(path); got != row.want {
			t.Fatalf("%s: %q, want %q", row.name, got, row.want)
		}
	}
}

func failedBuildQuestionBed(t *testing.T) *workBed {
	t.Helper()
	return partialBuildBed(t, 3, partialMessage)
}

func TestFailedBuildQuestionCommandIsReadyToPaste(t *testing.T) {
	t.Parallel()
	t.Run("prepared", func(t *testing.T) {
		t.Parallel()
		b := failedBuildQuestionBed(t)
		enrol(t, b, "Wido")
		q, r := partialQuestion(t, b)
		command := channel.ActCommand(q)
		words := shellWords(command)
		for _, word := range words[1:] {
			if regexp.MustCompile(`^[A-Z]+$`).MatchString(word) {
				t.Fatalf("the command keeps the placeholder %s: %s", word, command)
			}
		}
		brief := words[slices.Index(words, "--brief")+1]
		data, err := os.ReadFile(brief)
		if !filepath.IsAbs(brief) || err != nil || len(data) == 0 || !strings.Contains(string(data), r.Rounds[0].Stop.Class) || !slices.Contains(q.Facts, "A correction brief is prepared at "+brief+"; edit it before you run the command if you want a different correction.") {
			t.Fatalf("the prepared brief %s: %q %v facts=%q", brief, data, err, q.Facts)
		}
		edited := "Correct only the error table.\n"
		if err := os.WriteFile(brief, []byte(edited), 0600); err != nil {
			t.Fatal(err)
		}
		owners := b.workOwners()
		layout, err := owners.resolver.ResolveLayout(b.root())
		if err != nil {
			t.Fatal(err)
		}
		if err := (&intentInvocation{owners: owners, layout: layout, cwd: b.root(), stateRoot: b.root(), input: intentInput{values: map[string][]string{}}}).syncBuildHolds(r); err != nil {
			t.Fatal(err)
		}
		again, _ := channel.WalkOpenQuestions(b.root())
		kept, _ := os.ReadFile(brief)
		if len(again) != 1 || channel.ActCommand(again[0]) != command || string(kept) != edited {
			t.Fatalf("a later sync changed the brief or the command: %q %+v", kept, again)
		}
		code, revised, output := b.work(words[1:]...)
		after := onlyRun(t, b)
		if len(after.Revisions) != 1 || after.Revisions[0].Person != "Wido" || len(after.Rounds) != 2 {
			t.Fatalf("the pasted command did not admit the revision: %d %+v %s", code, revised, output)
		}
	})
	t.Run("unwritable", func(t *testing.T) {
		t.Parallel()
		b := failedBuildQuestionBed(t)
		b.manager.Supervisor = partialStarter{bed: b, message: partialMessage, block: true}
		q, _ := partialQuestion(t, b)
		if words := shellWords(channel.ActCommand(q)); !slices.Contains(words, "FILE") || !slices.ContainsFunc(q.Facts, func(fact string) bool { return strings.HasPrefix(fact, "A correction brief could not be prepared") }) {
			t.Fatalf("an unprepared brief was named: %s %q", channel.ActCommand(q), q.Facts)
		}
	})
}

func TestIntentDeclaredCheckRunsFromTheInstallation(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, folder string
		check        []string
		want         string
		laneFix      bool
	}{
		{name: "checkout-root", folder: "."},
		{name: "below-the-installation", folder: "module"},
		{name: "manual-check", folder: "module", check: []string{"--check", "true"}, want: "module"},
		{name: "lane-fix-check", folder: "module", check: []string{"--check", "metasystem test impact"}, want: "module", laneFix: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := declaredCheckBed(t, "proof.cheap=true\nproof.audits=true\nproof.deadline=15\n")
			for _, tree := range []string{b.root(), b.worktree} {
				if err := os.MkdirAll(filepath.Join(tree, "module"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			b.manager.Supervisor = &declaredCheckStarter{bed: b, t: t}
			args := []string{"work", "build", b.id, "installed", "--brief", filepath.Join(b.root(), b.brief("installed.md", "Build.\n")), "--lines", "100"}
			var code int
			var built intentResult
			if row.laneFix {
				// The lane's own fix check runs where the lane's caller stands;
				// the plan is frozen before the build launch is admitted.
				laneCheckAt(t, b, filepath.Join(b.root(), row.folder), append(args, row.check...)...)
				plan, err := launch.ReadUnitPlan(filepath.Join(onlyRun(t, b).Rounds[0].Directory, "plan.json"))
				if err != nil || plan.Check == nil || plan.Check.Cheap != "metasystem test impact --check" || plan.Check.Directory != filepath.Join(b.worktree, row.want) {
					t.Fatalf("the lane-fix check is frozen at %+v, want the worktree's %q: %v", plan.Check, row.want, err)
				}
				return
			}
			if row.check == nil {
				code, built = declaredCheckAt(t, b, filepath.Join(b.root(), row.folder), "", args...)
			} else {
				code, built = manualCheckAt(t, b, filepath.Join(b.root(), row.folder), append(append(args, "--reason", "Run my own check", "--by", "Wido"), row.check...)...)
			}
			if _, ok := resultData(t, built)["plan"].(string); !ok {
				t.Fatalf("no plan: %d %s %v", code, built.Summary, built.Details)
			}
			plan, err := launch.ReadUnitPlan(resultData(t, built)["plan"].(string))
			if err != nil || plan.Check == nil || plan.Check.Directory != filepath.Join(b.worktree, row.want) {
				t.Fatalf("%d %s: the check is frozen at %+v, want the worktree's %q: %v", code, built.Summary, plan.Check, row.want, err)
			}
		})
	}
}

// manualCheckAt is declaredCheckAt from the person's enrolled terminal, the
// only caller whose own --check runs.
func manualCheckAt(t *testing.T, b *workBed, directory string, args ...string) (int, intentResult) {
	t.Helper()
	owners := b.workOwners()
	owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
	return checkAt(t, b, owners, directory, args...)
}

// laneCheckAt is declaredCheckAt for a worktree the landing lane records a
// fix for.
func laneCheckAt(t *testing.T, b *workBed, directory string, args ...string) (int, intentResult) {
	t.Helper()
	owners := b.workOwners()
	owners.connection.laneFix = func(_, checkout, id string) *plain.Fix {
		if !sameCanonicalPath(checkout, b.worktree) {
			return nil
		}
		return &plain.Fix{Goal: id, Parent: "base-commit", Round: 1}
	}
	return checkAt(t, b, owners, directory, args...)
}

func checkAt(t *testing.T, b *workBed, owners intentOwners, directory string, args ...string) (int, intentResult) {
	t.Helper()
	command, rest, _ := resolveIntentArgv(args)
	git := owners.work.git
	owners.work.git = func(root string, args ...string) ([]byte, error) {
		if slices.Equal(args, []string{"rev-parse", "--show-toplevel"}) {
			for _, tree := range []string{b.root(), b.worktree} {
				if _, err := fakeTop(tree)(root); err == nil {
					return []byte(tree), nil
				}
			}
		}
		return git(root, args...)
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, append([]string{"--json"}, rest...), &stdout, &stderr, directory, owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("result: %s stderr=%s: %v", stdout.String(), stderr.String(), err)
	}
	return code, result
}
