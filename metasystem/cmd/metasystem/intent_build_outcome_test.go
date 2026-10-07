package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

type outcomeGit struct {
	workGit
	diff string
}

func (g outcomeGit) Run(dir string, env []string, args ...string) ([]byte, error) {
	if len(args) > 2 && args[0] == "diff" && args[2] == "--binary" {
		return []byte(g.diff), nil
	}
	return g.workGit.Run(dir, env, args...)
}

type outcomeStarter struct {
	bed   *workBed
	proof func()
	build func()
}

func (s outcomeStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	r, _ := s.bed.manager.Store.Read(id)
	if r.Kind == "proof" && s.proof != nil {
		s.proof()
	}
	if r.Kind == "build" {
		if s.build != nil {
			s.build()
		}
		name, message := "last-message.txt", "There is a missing requirement in the plan.\n"
		if r.Adapter == "claude-headless" {
			name, message = "result.json", `{"type":"result","result":"There is a missing requirement in the plan.","is_error":false}`
		}
		if err := os.WriteFile(filepath.Join(state, name), []byte(message), 0600); err != nil {
			return identity.Ref{}, err
		}
	}
	return s.bed.starter.StartSupervisor(id, state)
}

func outcomeBed(t *testing.T, diff string) *workBed {
	t.Helper()
	b := newWorkBed(t)
	b.workOwnersHook = func(o *intentWorkOwners) {
		units := o.units
		o.units = func(layout stateroot.Layout) *launch.UnitRunner {
			r := units(layout)
			r.Git = outcomeGit{workGit{b}, diff}
			return r
		}
	}
	b.manager.Supervisor = outcomeStarter{bed: b}
	return b
}

func outcomeBuild(t *testing.T, b *workBed) (int, intentResult, string) {
	t.Helper()
	brief := b.brief("build.md", "Build the declared requirement.\n")
	return b.work(append([]string{"work", "build", b.id, "outcome", "--brief", brief, "--lines", "1"}, workCheck...)...)
}

func outcomeReview(t *testing.T, b *workBed, person bool, stderr *bytes.Buffer) (int, intentResult) {
	t.Helper()
	owners := b.workOwners()
	if !person {
		owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
			return humanauthority.Proof{}, errors.New("an agent process is an ancestor of this shell")
		}
	}
	command, rest, _ := resolveIntentArgv([]string{"work", "review", b.id, "--work", "outcome", "--reason", "Accept the complete change", "--by", "Wido"})
	var out bytes.Buffer
	code := runIntentIn(command, append([]string{"--json"}, rest...), &out, stderr, b.root(), owners)
	var result intentResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err, out.String())
	}
	return code, result
}

func TestIntentBuildEmptyStopsBeforeProofAndRetainsGap(t *testing.T) {
	t.Parallel()
	b := outcomeBed(t, "")
	code, result, _ := outcomeBuild(t, b)
	if code != 1 || !strings.Contains(result.Summary, "stopped (gap)") || !slices.Equal(b.starter.launched(), []string{"build"}) {
		t.Fatalf("empty build launched proof/read: %d %+v %v", code, result, b.starter.launched())
	}

	questions, unreadable := channel.WalkOpenQuestions(b.root())
	if len(unreadable) != 0 || len(questions) != 1 || questions[0].UnitStop == nil || !strings.Contains(channel.ActCommand(questions[0]), "work revise") || !slices.Equal(questions[0].UnitStop.AcceptableActs, []string{"work-revise"}) {
		t.Fatalf("gap question has no executable revision: %+v %v", questions, unreadable)
	}
	run := resultData(t, result)["run"].(string)
	for _, verb := range []string{"wait", "review"} {
		code, recovered, _ := b.work("work", verb, "run:"+run)
		if code != 1 || !strings.Contains(recovered.Summary, "stopped (gap)") || recovered.Next == nil || !slices.Contains(recovered.Next.Argv, "revise") || len(b.starter.launched()) != 1 {
			t.Fatalf("restart lost the gap: %d %+v", code, recovered)
		}
	}
	r, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if err != nil || r.Rounds[0].Stop == nil || r.Rounds[0].Stop.Decision != "stop" {
		t.Fatalf("gap was not retained: %+v %v", r, err)
	}
	message, err := os.ReadFile(r.Rounds[0].GapMessage)
	if err != nil || !strings.Contains(string(message), "missing requirement") {
		t.Fatalf("builder's last message lost: %q %v", message, err)
	}
	if _, err := os.Stat(filepath.Join(r.Rounds[0].Directory, "stop-register.json")); err != nil {
		t.Fatal(err)
	}
	brief := b.brief("gap.md", "Correct the missing requirement in the retained plan.\n")
	code, revised, _ := b.work("work", "revise", b.id, "--work", "outcome", "--after", "1", "--brief", brief)
	if code != 1 || !strings.Contains(revised.Summary, "stopped (gap)") || !slices.Equal(b.starter.launched(), []string{"build", "build"}) {
		t.Fatalf("gap remedy failed: %d %+v %v", code, revised, b.starter.launched())
	}
	closed, err := channel.ReadQuestion(b.root(), questions[0].ID)
	if err != nil || closed.State != "closed" {
		t.Fatalf("successful gap revision left its question open: %+v %v", closed, err)
	}
	register, err := os.ReadFile(filepath.Join(r.Rounds[0].Directory, "stop-register.json"))
	if err != nil || !strings.Contains(string(register), `"status":"cleared"`) {
		t.Fatalf("older gap hid the current round: %s %v", register, err)
	}
	current, _ := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	build, _ := b.manager.Store.Read(current.Rounds[1].Steps[0].LaunchID)
	var inputs []string
	for _, input := range build.Inputs {
		inputs = append(inputs, input.Path)
	}
	if !slices.Contains(inputs, r.Plan) || !slices.Contains(inputs, filepath.Join(r.Rounds[0].Directory, "worktree.diff")) || !slices.Contains(inputs, r.Rounds[0].GapMessage) {
		t.Fatalf("gap correction lost its retained plan or gap: %v", inputs)
	}
}

func TestIntentBuildSizeAcceptanceResumesProofAfterRecordedImpact(t *testing.T) {
	t.Parallel()
	diff := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n-old\n+new\ndiff --git a/a_test.go b/a_test.go\n--- /dev/null\n+++ b/a_test.go\n+test\n"
	b := outcomeBed(t, diff)
	code, result, _ := outcomeBuild(t, b)
	if code != 1 || !slices.Equal(b.starter.launched(), []string{"build"}) {
		t.Fatalf("oversized build started proof: %d %+v %v", code, result, b.starter.launched())
	}

	questions, unreadable := channel.WalkOpenQuestions(b.root())
	if len(unreadable) != 0 || len(questions) != 1 || questions[0].UnitStop == nil || !strings.Contains(channel.ActCommand(questions[0]), "work review") || !strings.Contains(channel.ActCommand(questions[0]), "--reason TEXT --by NAME") || !slices.Equal(questions[0].UnitStop.AcceptableActs, []string{"work-review-size"}) {
		t.Fatalf("size question has no person's act: %+v %v", questions, unreadable)
	}
	run := resultData(t, result)["run"].(string)
	for _, verb := range []string{"wait", "review"} {
		code, recovered, _ := b.work("work", verb, "run:"+run)
		if code != 1 || recovered.Next == nil || !slices.Contains(recovered.Next.Argv, "--reason") || len(b.starter.launched()) != 1 {
			t.Fatalf("restart lost the size hold: %d %+v", code, recovered)
		}
	}
	r, _ := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if r.Rounds[0].BuildLines != 3 || r.Rounds[0].DeclaredLines != 1 || r.Rounds[0].Stop == nil {
		t.Fatalf("size omitted deletion or tests: %+v", r)
	}
	code, status, _ := b.work("status", b.id, "--work", "outcome")
	if code != 0 || status.Next == nil || !slices.Contains(status.Next.Argv, "--reason") {
		t.Fatalf("status offered a build instead of size acceptance: %d %+v", code, status)
	}
	var stderr bytes.Buffer
	code, denied := outcomeReview(t, b, false, &stderr)
	if code == 0 || len(b.starter.launched()) != 1 {
		t.Fatalf("agent accepted the size: %d %+v", code, denied)
	}
	b.manager.Supervisor = outcomeStarter{bed: b, proof: func() {
		if !strings.Contains(stderr.String(), "Impact: accept 3 changed lines against 1 declared") {
			t.Error("proof started before impact was printed")
		}
		impacts, _ := filepath.Glob(filepath.Join(b.root(), "artifacts", "agents", "channel", "unit-stop-overrides", "*.json"))

		if len(impacts) != 1 {
			t.Errorf("proof started before impact was recorded: %v", impacts)
		} else {
			data, err := os.ReadFile(impacts[0])
			var impact struct {
				Goal, Kind, Reason, Impact, Who string
				At                              time.Time
			}
			if err != nil || json.Unmarshal(data, &impact) != nil || impact.Goal != b.id || impact.Kind != "work-review-size" || impact.Who != "Wido" || impact.Reason != "Accept the complete change" || impact.At.IsZero() || !strings.Contains(impact.Impact, "3 changed lines against 1 declared") {
				t.Errorf("incomplete impact: %s %v", data, err)
			}
		}
	}}
	code, accepted := outcomeReview(t, b, true, &stderr)
	if code != 0 || !strings.Contains(stderr.String(), "Impact: accept 3 changed lines against 1 declared") || !slices.Equal(b.starter.launched(), []string{"build", "proof"}) {
		t.Fatalf("acceptance rebuilt or skipped impact: %d %+v %q %v", code, accepted, stderr.String(), b.starter.launched())
	}
	closed, err := channel.ReadQuestion(b.root(), questions[0].ID)
	if err != nil || closed.State != "closed" {
		t.Fatalf("size acceptance left its question open: %+v %v", closed, err)
	}
	r, _ = (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if len(r.Rounds) != 1 || r.Rounds[0].SizeAcceptedBy != "Wido" || r.Rounds[0].Outcome != "green" {
		t.Fatalf("acceptance lost its round: %s", fmt.Sprint(r))
	}
	register, err := os.ReadFile(filepath.Join(r.Rounds[0].Directory, "stop-register.json"))
	if err != nil || !strings.Contains(string(register), `"status":"cleared"`) {
		t.Fatalf("old size stop hid acceptance: %s %v", register, err)
	}
}

func TestIntentBuildHoldQuestionRecoversAfterWriteFailure(t *testing.T) {
	t.Parallel()
	b := outcomeBed(t, "")
	lock := filepath.Join(b.root(), "artifacts", "agents", "channel", "unit-stops.lock")
	if err := os.MkdirAll(lock, 0700); err != nil {
		t.Fatal(err)
	}
	code, result, _ := outcomeBuild(t, b)
	if code != 1 || len(b.starter.launched()) != 1 {
		t.Fatalf("question failure launched more work: %d %+v", code, result)
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	code, result, _ = b.work("work", "wait", b.id, "--work", "outcome")
	questions, unreadable := channel.WalkOpenQuestions(b.root())
	if code != 1 || !strings.Contains(result.Summary, "stopped (gap)") || len(questions) != 1 || len(unreadable) != 0 || len(b.starter.launched()) != 1 {
		t.Fatalf("retained hold lost during recovery: %d %+v %+v %v", code, result, questions, unreadable)
	}
}

func TestIntentBuildSizeMovedChangeStaysHeld(t *testing.T) {
	t.Parallel()
	diff := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n+one\n+two\n+three\n"
	b := outcomeBed(t, diff)
	code, result, _ := outcomeBuild(t, b)
	if code != 1 {
		t.Fatalf("size did not hold: %d %+v", code, result)
	}
	questions, _ := channel.WalkOpenQuestions(b.root())
	b.workOwnersHook = func(o *intentWorkOwners) {
		units := o.units
		o.units = func(layout stateroot.Layout) *launch.UnitRunner {
			runner := units(layout)
			runner.Git = outcomeGit{workGit{b}, diff + "+moved\n"}
			return runner
		}
	}
	var stderr bytes.Buffer
	code, result = outcomeReview(t, b, true, &stderr)
	open, _ := channel.WalkOpenQuestions(b.root())
	if code != 1 || len(b.starter.launched()) != 1 || len(questions) != 1 || len(open) != 1 || open[0].ID != questions[0].ID {
		t.Fatalf("changed result was accepted: %d %+v %v", code, result, b.starter.launched())
	}
	impacts, _ := filepath.Glob(filepath.Join(b.root(), "artifacts", "agents", "channel", "unit-stop-overrides", "*.json"))
	if len(impacts) != 0 {
		t.Fatalf("refused acceptance recorded an impact: %v", impacts)
	}
	code, result, _ = b.work("work", "wait", b.id, "--work", "outcome")
	if code != 1 || result.Next == nil || !slices.Contains(result.Next.Argv, "--reason") || len(b.starter.launched()) != 1 {
		t.Fatalf("failed acceptance undid the hold: %d %+v", code, result)
	}
}

func TestIntentBuildGapCorrectionClearsHoldWithoutARead(t *testing.T) {
	t.Parallel()
	b := outcomeBed(t, "")
	code, held, _ := outcomeBuild(t, b)
	if code != 1 {
		t.Fatalf("empty build did not hold: %d %+v", code, held)
	}
	run := resultData(t, held)["run"].(string)
	before, _ := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	b.workOwnersHook = func(o *intentWorkOwners) {
		units := o.units
		o.units = func(layout stateroot.Layout) *launch.UnitRunner {
			runner := units(layout)
			runner.Git = outcomeGit{workGit{b}, "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n+implemented\n"}
			return runner
		}
	}
	brief := b.brief("corrected-gap.md", "Supply the missing implementation.\n")
	code, corrected, _ := b.work("work", "revise", b.id, "--work", "outcome", "--after", "1", "--brief", brief)
	if code != 0 || resultData(t, corrected)["outcome"] != "green" || !slices.Equal(b.starter.launched(), []string{"build", "build", "proof"}) {
		t.Fatalf("gap correction failed: %d %+v %v", code, corrected, b.starter.launched())
	}
	current, _ := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	for _, name := range []string{"read-decision.json", "stop-register.json"} {
		if _, err := os.Stat(filepath.Join(current.Rounds[1].Directory, name)); !os.IsNotExist(err) {
			t.Fatalf("unexamined correction wrote %s: %v", name, err)
		}
	}
	register, err := os.ReadFile(filepath.Join(before.Rounds[0].Directory, "stop-register.json"))
	if err != nil || !strings.Contains(string(register), `"status":"cleared"`) {
		t.Fatalf("older gap remained open: %s %v", register, err)
	}
	questions, unreadable := channel.WalkOpenQuestions(b.root())
	if len(questions) != 0 || len(unreadable) != 0 {
		t.Fatalf("corrected gap still asks for revision: %+v %v", questions, unreadable)
	}
}

func TestIntentBuildGapAtRoundLimitNamesExecutablePersonRevision(t *testing.T) {
	t.Parallel()
	b := outcomeBed(t, "")
	code, first, _ := outcomeBuild(t, b)
	if code != 1 {
		t.Fatalf("empty build did not hold: %d %+v", code, first)
	}
	brief := b.brief("second-gap.md", "Correct the missing requirement.\n")
	code, held, _ := b.work("work", "revise", b.id, "--work", "outcome", "--after", "1", "--brief", brief)
	if code != 1 || held.Next == nil || !slices.Contains(held.Next.Argv, "--reason") || !slices.Contains(held.Next.Argv, "--by") {
		t.Fatalf("exhausted gap offered an impossible revision: %d %+v", code, held)
	}
	questions, _ := channel.WalkOpenQuestions(b.root())
	if len(questions) != 1 || !strings.Contains(channel.ActCommand(questions[0]), "--reason TEXT --by NAME") {
		t.Fatalf("gap question omitted the person: %+v", questions)
	}
	argv := slices.Clone(held.Next.Argv[1:])
	for i, v := range argv {
		switch v {
		case "FILE":
			argv[i] = b.brief("person-gap.md", "Revise the missing requirement again.\n")
		case "TEXT":
			argv[i] = "Admit one more gap correction"
		case "NAME":
			argv[i] = "Wido"
		}
	}
	code, continued, stderr := b.work(argv...)
	if code != 1 || resultData(t, continued)["round"] != float64(3) || resultData(t, continued)["outcome"] != "build-gap" || !strings.Contains(stderr, "Impact:") || !slices.Equal(b.starter.launched(), []string{"build", "build", "build"}) {
		t.Fatalf("printed person remedy failed: %d %+v %s %v", code, continued, stderr, b.starter.launched())
	}
	closed, err := channel.ReadQuestion(b.root(), questions[0].ID)
	if err != nil || closed.State != "closed" {
		t.Fatalf("admitted correction left its gap ask open: %+v %v", closed, err)
	}
}

func TestIntentBuildSizeCorrectionRecordsRevisionWithoutAcceptance(t *testing.T) {
	t.Parallel()
	for _, empty := range []bool{false, true} {
		t.Run(fmt.Sprint(empty), func(t *testing.T) {
			t.Parallel()
			diff := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n+one\n+two\n+three\n"
			b := outcomeBed(t, diff)
			code, held, _ := outcomeBuild(t, b)
			if code != 1 {
				t.Fatalf("size did not hold: %d %+v", code, held)
			}
			run := resultData(t, held)["run"].(string)
			before, _ := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
			b.workOwnersHook = func(o *intentWorkOwners) {
				units := o.units
				o.units = func(layout stateroot.Layout) *launch.UnitRunner {
					runner := units(layout)
					if empty {
						diff = ""
					} else {
						diff = "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n+one\n"
					}
					runner.Git = outcomeGit{workGit{b}, diff}
					return runner
				}
			}
			brief := b.brief("narrow-size.md", "Narrow the implementation to its declared size.\n")
			code, corrected, _ := b.work("work", "revise", b.id, "--work", "outcome", "--after", "1", "--brief", brief, "--reason", "Correct the oversized change", "--by", "Wido")
			want := 0
			if empty {
				want = 1
			}
			if code != want || resultData(t, corrected)["round"] != float64(2) {
				t.Fatalf("size correction failed: %d %+v", code, corrected)
			}
			register, err := os.ReadFile(filepath.Join(before.Rounds[0].Directory, "stop-register.json"))
			if err != nil || !strings.Contains(string(register), `"status":"cleared"`) {
				t.Fatalf("older size hold hides correction: %s %v", register, err)
			}
			acts, _ := filepath.Glob(filepath.Join(b.root(), "artifacts", "agents", "channel", "unit-stop-acts", "*.json"))
			if len(acts) != 1 {
				t.Fatalf("correction omitted its act: %v", acts)
			}
			data, err := os.ReadFile(acts[0])
			var act channel.UnitStopAct
			if err != nil || json.Unmarshal(data, &act) != nil || act.Kind != "work-revise" {
				t.Fatalf("revision recorded a size acceptance: %s %v", data, err)
			}
			current, _ := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
			if current.Rounds[0].SizeAcceptedBy != "" {
				t.Fatal("correction accepted the old size")
			}
		})
	}
}

// realOutcomeBed keeps the public command fixture's synthetic authority and
// model executions, and measures actual files with production Git.
func realOutcomeBed(t *testing.T) *workBed {
	t.Helper()
	b := newWorkBed(t)
	git := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", b.worktree, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "goal/"+b.id)
	if err := os.WriteFile(filepath.Join(b.worktree, "unit.go"), []byte("old\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "unit.go")
	git("commit", "-q", "-m", "fixture base")
	b.head = git("rev-parse", "HEAD")
	b.workOwnersHook = func(o *intentWorkOwners) {
		units := o.units
		o.units = func(layout stateroot.Layout) *launch.UnitRunner {
			r := units(layout)
			r.Git = launch.OSGitRunner{}
			return r
		}
	}
	return b
}

func TestIntentBuildUnchangedCorrectionIsGap(t *testing.T) {
	t.Parallel()
	b := realOutcomeBed(t)
	b.manager.Supervisor = outcomeStarter{bed: b, build: func() {
		if err := os.WriteFile(filepath.Join(b.worktree, "unit.go"), []byte("new\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}}
	code, built, _ := outcomeBuild(t, b)
	if code != 0 {
		t.Fatalf("first build: %d %+v", code, built)
	}
	code, held, _ := b.work("work", "revise", b.id, "--work", "outcome", "--after", "1", "--brief", b.brief("unchanged.md", "Correct the implementation.\n"), "--reason", "Revisit the result", "--by", "Wido")
	if code != 1 || resultData(t, held)["outcome"] != "build-gap" || !slices.Equal(b.starter.launched(), []string{"build", "proof", "build"}) {
		t.Fatalf("unchanged correction launched checks or review: %d %+v %v", code, held, b.starter.launched())
	}
	run := resultData(t, built)["run"].(string)
	r, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if err != nil || r.Rounds[1].BuildLines != 0 {
		t.Fatalf("unchanged builder counted old lines: %+v %v", r, err)
	}
}

func TestIntentBuildSizeExcludesRecordsAndPreexistingChanges(t *testing.T) {
	t.Parallel()
	b := realOutcomeBed(t)
	if err := os.WriteFile(filepath.Join(b.worktree, "other.go"), []byte(strings.Repeat("other unit\n", 40)), 0600); err != nil {
		t.Fatal(err)
	}
	b.manager.Supervisor = outcomeStarter{bed: b, build: func() {
		for _, path := range []string{"records/reads/read.json", "metasystem/records/reads/read.json"} {
			full := filepath.Join(b.worktree, path)
			if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte(strings.Repeat("record\n", 40)), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(b.worktree, "unit.go"), []byte("new\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}}
	code, built, _ := outcomeBuild(t, b)
	if code != 0 || !slices.Equal(b.starter.launched(), []string{"build", "proof"}) {
		t.Fatalf("unrelated files caused a size hold: %d %+v", code, built)
	}
	r, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(resultData(t, built)["run"].(string))
	if err != nil || r.Rounds[0].BuildLines != 2 {
		t.Fatalf("size must count only builder additions and deletions: %+v %v", r, err)
	}
}

func TestIntentBuildGapRevisionHonorsPolicy(t *testing.T) {
	t.Parallel()
	for _, policy := range []string{"person", "unreadable"} {
		t.Run(policy, func(t *testing.T) {
			t.Parallel()
			b := outcomeBed(t, "")
			hook := b.workOwnersHook
			b.workOwnersHook = func(o *intentWorkOwners) {
				hook(o)
				units := o.units
				o.units = func(layout stateroot.Layout) *launch.UnitRunner {
					r := units(layout)
					r.ReviewPolicy = func() (string, error) {
						if policy == "unreadable" && len(b.starter.launched()) > 0 {
							return "", fmt.Errorf("review.stop cannot be read: invalid policy value")
						}
						if policy == "unreadable" {
							return "auto", nil
						}
						return policy, nil
					}
					return r
				}
			}
			code, held, _ := outcomeBuild(t, b)
			if code != 1 || held.Next == nil || !slices.Contains(held.Next.Argv, "--reason") || !slices.Contains(held.Next.Argv, "--by") {
				t.Fatalf("gap omitted person remedy: %d %+v", code, held)
			}
			questions, unreadable := channel.WalkOpenQuestions(b.root())
			if len(unreadable) != 0 || len(questions) != 1 || !strings.Contains(channel.ActCommand(questions[0]), "--reason TEXT --by NAME") {
				t.Fatalf("gap question omitted person remedy: %+v %v", questions, unreadable)
			}
			brief := b.brief("policy-gap.md", "Supply the missing implementation.\n")
			code, refused, _ := b.work("work", "revise", b.id, "--work", "outcome", "--after", "1", "--brief", brief)
			if code != 1 || refused.Next == nil || !slices.Contains(refused.Next.Argv, "--reason") || !slices.Contains(refused.Next.Argv, "--by") || !slices.Equal(b.starter.launched(), []string{"build"}) {
				t.Fatalf("agent bypassed gap policy: %d %+v %v", code, refused, b.starter.launched())
			}
			r, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(resultData(t, held)["run"].(string))
			if err != nil || len(r.Rounds) != 1 || len(r.Revisions) != 0 {
				t.Fatalf("refused agent revision changed the run: %+v %v", r, err)
			}
			code, admitted, stderr := b.work("work", "revise", b.id, "--work", "outcome", "--after", "1", "--brief", brief, "--reason", "Admit the missing implementation", "--by", "Wido")
			if code != 1 || resultData(t, admitted)["round"] != float64(2) || !strings.Contains(stderr, "Impact:") || !slices.Equal(b.starter.launched(), []string{"build", "build"}) {
				t.Fatalf("person could not use printed gap remedy: %d %+v %s", code, admitted, stderr)
			}
		})
	}
}
