package launch

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type rebaseFixtureGit struct {
	GitRunner
	source, worktree string
	baseSeen         bool
}

func (git *rebaseFixtureGit) Run(dir string, env []string, args ...string) ([]byte, error) {
	if dir == git.worktree {
		if strings.Join(args, " ") == "rev-parse --verify HEAD" {
			return []byte("new-base"), nil
		}
		if strings.Join(args, " ") == "rev-parse --verify REBASE_HEAD" {
			return []byte("stopped-unit"), nil
		}
		dir = git.source
	}
	if len(args) > 3 && args[0] == "diff" && args[1] == "--cached" && args[2] == "--binary" {
		if args[3] != "previous-tree" {
			git.baseSeen = git.baseSeen || args[3] == "new-base"
			args = append([]string(nil), args...)
			args[3] = "base"
		}
	}
	if slices.Equal(args, []string{"read-tree", "new-base"}) {
		args = []string{"read-tree", "base"}
	}
	return git.GitRunner.Run(dir, env, args...)
}

func TestRevisionOnStoppedRebase(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "branch", "round", "resolve", "before", "resolve", "round", "resolve", "round")
	first, err := fixture.runner.AdvanceNamed(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	worktree := t.TempDir()
	git := &rebaseFixtureGit{GitRunner: fixture.git, source: fixture.worktree, worktree: worktree}
	fixture.runner.Git = git
	fixture.starter.onStart = func(record Record) error {
		if record.Kind == "read" {
			t.Fatal("resolve round launched a read")
		}
		if record.Kind == "build" && record.WorkingDirectory != worktree {
			t.Fatalf("build directory %s", record.WorkingDirectory)
		}
		if record.Kind == "build" {
			data, err := os.ReadFile(record.Inputs[0].Path)
			if err != nil || !strings.Contains(string(data), "Declared size: 2 changed lines") || !strings.Contains(string(data), "resolve hunks") {
				t.Fatalf("resolve brief %s %v", data, err)
			}
		}
		if record.Kind == "proof" {
			data, err := os.ReadFile(record.Inputs[0].Path)
			if err != nil {
				t.Fatal(err)
			}
			var proof PlainBrief
			if err := json.Unmarshal(data, &proof); err != nil || proof.Dir != worktree {
				t.Fatalf("proof %s %v", data, err)
			}
		}
		return nil
	}
	request := UnitRevisionRequest{Run: first.Record.ID, Brief: []byte("Declared size: 1 changed lines\nresolve hunks\n"), Rebase: &UnitRebasePlan{Worktree: worktree, Base: "new-base", Commit: "stopped-unit"}}
	fixture.runner.BeforeModelLaunch = func(UnitRunRecord, StartSpec) error { return errors.New("interrupted before launch") }
	if _, err := fixture.runner.Revise(request); err == nil {
		t.Fatal("the interrupted correction passed")
	}
	fixture.runner.BeforeModelLaunch = nil
	got, err := fixture.runner.Continue(UnitRequest{Resume: first.Record.ID})
	if err != nil || got.Round != 2 || got.Record.Rounds[1].Outcome != "green" || !git.baseSeen {
		t.Fatalf("resolve %+v %v base seen %v", got, err, git.baseSeen)
	}
	launched := len(fixture.starter.order)
	again, err := fixture.runner.Revise(request)
	if err != nil || !again.Rejoined || len(fixture.starter.order) != launched {
		t.Fatalf("repeat %+v %v", again, err)
	}
	fixture.starter.failKind = "proof"
	request.Brief = []byte("resolve hunks that remain\n")
	red, err := fixture.runner.Revise(request)
	if err != nil || red.Record.Rounds[red.Round-1].Outcome != "proof-red" {
		t.Fatalf("red resolve %+v %v", red, err)
	}
}

func TestReviseRefusesUndecidedFindings(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"launch", "committed", "unread"} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			events := []string{"branch", "round"}
			if source != "unread" {
				events = append(events, "branch", "round")
			}
			fixture := newUnitFixture(t, "", events...)
			fixture.starter.readVerdict = "fix first (2 material findings)"
			fixture.starter.skipStructured = source == "unread"
			first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
			if err != nil {
				t.Fatal(err)
			}
			if source == "committed" {
				path := filepath.Join(t.TempDir(), "return.json")
				writeFile(t, path, structuredUnitReturn(2, "regression", "code.go"))
				if err := fixture.runner.ReviewSubject(first.Record.ID, func(review UnitReview, retain func(UnitSubject) error) error {
					return retain(UnitSubject{Round: 1, DiffDigest: review.DiffDigest, Examination: "critic", ExaminationRound: 1, ExaminationReturnPath: path})
				}); err != nil {
					t.Fatal(err)
				}
				first.Record, err = fixture.runner.Status(first.Record.ID)
				if err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(filepath.Join(fixture.runner.runDir(first.Record.ID), "run.json"))
			launched := len(fixture.starter.order)
			brief := []byte("Declared size: 1 changed lines\n")
			want := "UNIT_STOPPED"
			if source != "unread" {
				findings := first.Record.Rounds[0].Reads[0].Findings
				brief = []byte(fmt.Sprintf("Declared size: 1 changed lines\n## Decisions on round 1\n| %s | fixed | file.go:12 |\n", findings[0].ID))
				want = "UNIT_REVISE_UNDECIDED"
			}
			_, err = fixture.runner.Revise(UnitRevisionRequest{Run: first.Record.ID, After: 1, Brief: brief})
			if !IsCode(err, want) {
				t.Fatalf("refusal: %v want %s", err, want)
			}
			if source != "unread" && !strings.Contains(err.Error(), first.Record.Rounds[0].Reads[0].Findings[1].ID) {
				t.Fatalf("missing canonical unresolved finding: %v", err)
			}
			after, _ := os.ReadFile(filepath.Join(fixture.runner.runDir(first.Record.ID), "run.json"))
			if string(before) != string(after) || len(fixture.starter.order) != launched {
				t.Fatal("undecided correction changed the run or launched")
			}
			if source != "unread" {
				if _, err := fixture.runner.Revise(UnitRevisionRequest{Run: first.Record.ID, After: 1, Brief: correctionBrief(first.Record, "Declared size: 1 changed lines\n")}); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestReviseJoinsSuppliedDispositions(t *testing.T) {
	t.Parallel()
	for _, complete := range []bool{true, false} {
		t.Run(fmt.Sprintf("complete=%t", complete), func(t *testing.T) {
			t.Parallel()
			events := []string{"branch", "round"}
			if complete {
				events = append(events, "branch", "round")
			}
			fixture := newUnitFixture(t, "", events...)
			fixture.starter.readVerdict = "fix first (2 material findings)"
			first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
			if err != nil {
				t.Fatal(err)
			}
			findings := first.Record.Rounds[0].Reads[0].Findings
			dispositions := fmt.Sprintf("| %s | fixed | file.go:12 |\n", findings[0].ID)
			if complete {
				dispositions += fmt.Sprintf("| %s | fixed | file.go:20 |\n", findings[1].ID)
			}
			before, err := os.ReadFile(filepath.Join(fixture.runner.runDir(first.Record.ID), "run.json"))
			if err != nil {
				t.Fatal(err)
			}
			launched := len(fixture.starter.order)
			fixture.starter.readVerdict = "fix first (1 material findings)"
			corrected, err := fixture.runner.Revise(UnitRevisionRequest{Run: first.Record.ID, After: 1, Brief: []byte("Declared size: 1 changed lines\nPreserve the result\n"), Decisions: []byte(dispositions)})
			if !complete {
				if !IsCode(err, "UNIT_REVISE_UNDECIDED") || !strings.Contains(err.Error(), findings[1].ID) {
					t.Fatalf("incomplete supplied dispositions admitted: %v", err)
				}
				after, readErr := os.ReadFile(filepath.Join(fixture.runner.runDir(first.Record.ID), "run.json"))
				if readErr != nil || string(after) != string(before) || len(fixture.starter.order) != launched {
					t.Fatalf("refused supplied dispositions changed the run: %v", readErr)
				}
				return
			}
			if err != nil || corrected.Round != 2 || len(corrected.Record.Rounds) != 2 || len(fixture.starter.order) != launched+3 {
				t.Fatalf("complete supplied dispositions did not launch one correction: %+v %v", corrected, err)
			}
			kept, err := os.ReadFile(corrected.Revision.Decisions)
			if err != nil || string(kept) != dispositions {
				t.Fatalf("supplied dispositions were not retained: %s %v", kept, err)
			}
		})
	}
}

func TestFollowUpReadIsToldThePreviousDecisions(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "round", "branch", "round", "warm")
	fixture.starter.readOutput = filepath.Join(t.TempDir(), "findings.md")
	findings := "1. Result is lost at `deleted.go:12-13`\nRELATION: new\n"
	writeFile(t, fixture.starter.readOutput, findings)
	fixture.starter.readVerdict = "VERDICT: fix first (1 material findings)"
	first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	require(t, err != nil, "%v", err)
	decisions := string(correctionBrief(first.Record, ""))
	second, err := fixture.runner.Revise(UnitRevisionRequest{Run: first.Record.ID, After: 1, Brief: []byte("Declared size: 1 changed lines\n" + decisions)})
	require(t, err != nil, "%v", err)
	read, err := fixture.manager.Store.Read(second.Record.Rounds[1].Steps[2].LaunchID)
	require(t, err != nil, "%v", err)
	brief, err := os.ReadFile(readString(read.AdapterData, "brief"))
	require(t, err != nil, "%v", err)
	for _, want := range []string{"Finding 1", "The result is discarded", decisions, "Check every fold first, citing the line", "Never re-raise a refuted finding without new evidence", "changed lines only", "fold-not-holding", "same-rule-as N", "Diff since round 1's tree", "+fixed line", "Proof result of round 2", `"state": "passed"`} {
		if !strings.Contains(string(brief), want) {
			t.Errorf("read brief lacks %q: %s", want, brief)
		}
	}
}

func TestReviseRefusedWhenMaterialDoesNotFall(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"markdown", "committed", "mixed"} {
		for _, newest := range []int{3, 2} {
			t.Run(fmt.Sprintf("%s/3-to-%d", source, newest), func(t *testing.T) {
				t.Parallel()
				checkDivergenceAdmissions(t, []int{3, newest}, "", source, newest == 3)
			})
		}
	}
}

func TestReviseRefusedOnARepeatedRule(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"markdown", "committed"} {
		for _, relation := range []string{"same-rule-as 1", "fold-not-holding"} {
			t.Run(source+"/"+relation, func(t *testing.T) {
				t.Parallel()
				checkDivergenceAdmissions(t, []int{3, 2}, relation, source, true)
			})
		}
	}
}

func TestReviseNeverDivergentOnNonMaterialOrOneRead(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"markdown", "committed"} {
		for _, counts := range [][]int{{0, 0}, {3, 0}, {3}} {
			t.Run(fmt.Sprint(source, counts), func(t *testing.T) {
				t.Parallel()
				checkDivergenceAdmissions(t, counts, "fold-not-holding", source, false)
			})
		}
	}
}

func TestRoundDivergenceSkipsUnreadRounds(t *testing.T) {
	t.Parallel()
	first, err := readsubject.Collect("first", readsubject.ReadSubject{}, "engine", "model", "return.json", []byte(structuredUnitReturn(3, "regression", "first.go")), "")
	if err != nil {
		t.Fatal(err)
	}
	last, err := readsubject.Collect("last", readsubject.ReadSubject{}, "engine", "model", "return.json", []byte(structuredUnitReturn(3, "scope", "last.go")), "")
	if err != nil {
		t.Fatal(err)
	}
	record := UnitRunRecord{ID: "run", Unit: "U", Goal: "goal", Rounds: []UnitRound{{Number: 1, Reads: []readsubject.Read{first}}, {Number: 2, Cause: "provider-limit"}, {Number: 3}, {Number: 4, Reads: []readsubject.Read{last}}}}
	if err := (&UnitRunner{}).roundDivergent(record); !IsCode(err, "UNIT_ROUND_DIVERGENT") {
		t.Fatalf("the last two reads must compare across unread rounds: %v", err)
	}
	record.Rounds = record.Rounds[1:]
	if err := (&UnitRunner{}).roundDivergent(record); err != nil {
		t.Fatalf("one counting read must not refuse: %v", err)
	}
}

func checkDivergenceAdmissions(t *testing.T, counts []int, relation, source string, refused bool) {
	t.Helper()
	for _, admission := range []string{"revise", "follow-up"} {
		t.Run(admission, func(t *testing.T) {
			counts := append([]int(nil), counts...)
			t.Parallel()
			// A clean read closes immediately; another automatic attempt cannot be started.
			if counts[0] == 0 {
				counts = counts[:1]
			}
			closed := counts[len(counts)-1] == 0
			var events []string
			for range counts {
				events = append(events, "branch", "round")
			}
			if admission == "follow-up" || !refused && !closed {
				events = append(events, "branch")
			}
			if !refused && !closed {
				events = append(events, "round")
			}
			fixture := newUnitFixture(t, "", events...)
			var result UnitResult
			for index, material := range counts {
				fixture.starter.readVerdict = fmt.Sprintf("fix first (%d material findings)", material)
				fixture.starter.readWhere = fmt.Sprintf("round-%d.go", index)
				fixture.starter.readClass = []string{"regression", "scope", "incomplete-item"}[index%3]
				if relation != "" && len(counts) > 1 {
					fixture.starter.readWhere = "repeated.go"
					fixture.starter.readClass = "regression"
					if index > 0 && relation == "same-rule-as 1" {
						fixture.starter.readWhere = "other.go"
					}
				}
				request := UnitRequest{Plan: fixture.plan}
				if index > 0 {
					request = UnitRequest{Resume: result.Record.ID, FollowUp: writeFollowUp(t)}
				}
				var err error
				result, err = fixture.runner.Advance(request)
				if err != nil {
					t.Fatal(err)
				}
				if source == "committed" || source == "mixed" && index == 1 {
					path := filepath.Join(t.TempDir(), "return.json")
					writeFile(t, path, structuredUnitReturn(material, fixture.starter.readClass, fixture.starter.readWhere))
					if err := fixture.runner.ReviewSubject(result.Record.ID, func(review UnitReview, retain func(UnitSubject) error) error {
						return retain(UnitSubject{Round: index + 1, DiffDigest: review.DiffDigest, Examination: fmt.Sprintf("critic-%d", index), ExaminationRound: 1, ExaminationReturnPath: path})
					}); err != nil {
						t.Fatal(err)
					}
					result.Record, err = fixture.runner.Status(result.Record.ID)
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			run := result.Record.ID
			before, _ := os.ReadFile(filepath.Join(fixture.runner.runDir(run), "run.json"))
			launched := len(fixture.starter.order)
			var err error
			if admission == "revise" {
				_, err = fixture.runner.Revise(UnitRevisionRequest{Run: run, Brief: correctionBrief(result.Record, "Declared size: 1 changed lines\n")})
			} else {
				_, err = fixture.runner.Advance(UnitRequest{Resume: run, FollowUp: writeFollowUp(t)})
			}
			if !refused && !closed {
				if err != nil || len(fixture.starter.order) <= launched {
					t.Fatalf("admission: %v; launches=%v; stop=%+v", err, fixture.starter.order, result.Record.Rounds[len(result.Record.Rounds)-1].Stop)
				}
				return
			}
			if !IsCode(err, "UNIT_STOPPED") {
				t.Fatalf("refusal: %v; detail=%s", err, ErrorDetail(err))
			}
			stop := result.Record.Rounds[len(result.Record.Rounds)-1].Stop
			if stop == nil || closed && stop.Decision != "close" || refused && stop.Decision != "stop" {
				t.Fatalf("wrong decision: %+v", stop)
			}
			after, _ := os.ReadFile(filepath.Join(fixture.runner.runDir(run), "run.json"))
			if string(after) != string(before) || len(fixture.starter.order) != launched {
				t.Fatal("a refused correction changed the run or launched work")
			}
		})
	}
}

// TestRevisionRequestReplay: a correction request is retained before its
// attempt launches, so an interrupted request, a lost response and a busy
// concurrent caller all reach one attempt; a different request for the same
// attempt and a request against an older attempt launch nothing.
func TestRevisionRequestReplay(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "branch", "round", "branch", "before", "branch", "round")
	first, err := fixture.runner.AdvanceNamed(fixture.plan)
	if err != nil || first.Record.State != "awaiting-judgement" {
		t.Fatalf("first attempt: %+v %v", first, err)
	}
	run := first.Record.ID
	brief := correctionBrief(first.Record, "Declared size: 1 changed lines\nfix the finding\n")

	// Interrupted after the request is retained and the attempt added,
	// before any launch.
	interrupted := *fixture.runner
	interrupted.BeforeModelLaunch = func(UnitRunRecord, StartSpec) error { return errors.New("the process died here") }
	if _, err := interrupted.Revise(UnitRevisionRequest{Run: run, After: 1, Brief: brief}); err == nil {
		t.Fatal("the interrupted request reported success")
	}
	record, _ := fixture.runner.read(run)
	if len(record.Revisions) != 1 || record.Revisions[0].After != 1 || record.Revisions[0].Attempt != 2 {
		t.Fatalf("the request was not retained before launch: %+v", record.Revisions)
	}
	requireLaunchedOnce(t, fixture, 3)

	// A concurrent caller while the unit is held is told to repeat.
	_, key, _ := namedUnitIdentity(UnitPlan{Worktree: fixture.worktree, Goal: "goal", Unit: "U"})
	held, err := fixture.runner.namedLock(key, UnitPlan{Unit: "U", Goal: "goal"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.runner.Revise(UnitRevisionRequest{Run: run, After: 1, Brief: brief}); err == nil || !strings.HasPrefix(ErrorDetail(err), "UNIT_RUN_BUSY") {
		t.Fatalf("a concurrent identical request = %v, want busy", err)
	}
	releaseUnitLock(held)
	requireLaunchedOnce(t, fixture, 3)

	// The retry resumes that attempt and launches it once.
	second, err := fixture.runner.Revise(UnitRevisionRequest{Run: run, After: 1, Brief: brief})
	if err != nil || second.Revision.Attempt != 2 || len(second.Record.Rounds) != 2 || second.Record.State != "awaiting-judgement" {
		t.Fatalf("retry: %+v %v", second, err)
	}
	launched := len(fixture.starter.order)

	// A lost response: the same request, with or without --after, rejoins.
	for _, after := range []int{1, 0} {
		again, err := fixture.runner.Revise(UnitRevisionRequest{Run: run, After: after, Brief: brief})
		if err != nil || !again.Rejoined || again.Revision.Attempt != 2 || again.Current != 2 {
			t.Fatalf("replay after=%d: %+v %v", after, again, err)
		}
	}
	// A different request for attempt 1 and a request against an older
	// attempt are refused.
	if _, err := fixture.runner.Revise(UnitRevisionRequest{Run: run, After: 1, Brief: []byte("another fix\n")}); err == nil || !strings.HasPrefix(ErrorDetail(err), "UNIT_REVISION_CONFLICT") {
		t.Fatalf("a different request for attempt 1 = %v", err)
	}
	if _, err := fixture.runner.Revise(UnitRevisionRequest{Run: run, After: 5, Brief: brief}); !errors.Is(err, ErrUnitRevisionStale) {
		t.Fatalf("a request against attempt 5 = %v", err)
	}
	if len(fixture.starter.order) != launched {
		t.Fatalf("a replayed or refused request launched: %v", fixture.starter.order[launched:])
	}
}

// TestRevisionRetainsReviewedFindingsAfterFailure: the reviewed findings and
// decisions frozen with a correction stay the builder's input when the
// deliberate retry follows a failed correction, whose own read outputs are
// empty.
func TestRevisionRetainsReviewedFindingsAfterFailure(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "branch", "round", "branch", "before", "branch", "before", "round")
	first, err := fixture.runner.AdvanceNamed(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	run := first.Record.ID
	brief := correctionBrief(first.Record, "Declared size: 1 changed lines\nfix F-1\n")
	decisions := []byte("| Finding id | Disposition |\n| F-1 | accepted |\n")
	fixture.starter.failKind = "build"
	failed, err := fixture.runner.Revise(UnitRevisionRequest{Run: run, Brief: brief, Decisions: decisions})
	if err != nil || failed.Revision.Attempt != 2 || failed.Record.Rounds[1].Outcome == "green" {
		t.Fatalf("failed correction: %+v %v", failed, err)
	}
	fixture.starter.failKind = ""
	retried, err := fixture.runner.Revise(UnitRevisionRequest{Run: run, After: 2, Brief: brief, Decisions: decisions, Person: "Wido", Reason: "Retry the failed builder with the retained decisions", Impact: "Start a new correction after a failed build with no read"})
	if err != nil || retried.Rejoined || retried.Revision.Attempt != 3 {
		t.Fatalf("deliberate retry after 2: %+v %v", retried, err)
	}
	frozen := retried.Revision.Decisions
	if data, err := os.ReadFile(frozen); err != nil || string(data) != string(decisions) {
		t.Fatalf("frozen decisions %s = %q %v", frozen, data, err)
	}
	build := retried.Record.Rounds[2].Steps[0]
	launched, err := fixture.manager.Store.Read(build.LaunchID)
	if err != nil {
		t.Fatal(err)
	}
	var inputs []string
	for _, input := range launched.Inputs {
		inputs = append(inputs, input.Path)
	}
	if !slices.ContainsFunc(inputs, func(path string) bool { return filepath.Clean(path) == filepath.Clean(frozen) }) {
		t.Fatalf("attempt 3's build inputs %v lack the frozen findings and decisions %s", inputs, frozen)
	}
}

// TestNamedWorkListsOneGoalsWork: the read API lists a goal's named work in
// one worktree with each run's record, leaves other goals and worktrees out,
// and refuses an entry whose run belongs elsewhere instead of skipping it.
func TestNamedWorkListsOneGoalsWork(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "branch", "round")
	first, err := fixture.runner.AdvanceNamed(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	work, err := fixture.runner.NamedWork(fixture.worktree, "goal")
	if err != nil || len(work) != 1 || work[0].Unit != "U" || work[0].Run != first.Record.ID || work[0].Record == nil || work[0].Running() {
		t.Fatalf("work = %+v %v", work, err)
	}
	if other, err := fixture.runner.NamedWork(fixture.worktree, "another-goal"); err != nil || len(other) != 0 {
		t.Fatalf("another goal's work = %+v %v", other, err)
	}
	elsewhere := t.TempDir()
	if other, err := fixture.runner.NamedWork(elsewhere, "goal"); err != nil || len(other) != 0 {
		t.Fatalf("another worktree's work = %+v %v", other, err)
	}
	_, key, _ := namedUnitIdentity(UnitPlan{Worktree: fixture.worktree, Goal: "goal", Unit: "U"})
	entry, _, _ := fixture.runner.readNamed(key)
	record, _ := fixture.runner.read(entry.Run)
	record.Goal = "stolen"
	if err := fixture.runner.save(record); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.runner.NamedWork(fixture.worktree, "goal"); err == nil || !strings.HasPrefix(ErrorDetail(err), "UNIT_NAMED_ENTRY_CORRUPT") {
		t.Fatalf("a run of another goal behind the entry = %v", err)
	}
}
