package launch

import (
	"encoding/json"
	"errors"
	"fmt"
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
		git.baseSeen = args[3] == "new-base"
		args = append([]string(nil), args...)
		args[3] = "base"
	}
	return git.GitRunner.Run(dir, env, args...)
}

func TestRevisionOnStoppedRebase(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "branch", "round", "round", "round")
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
	for _, source := range []string{"markdown", "committed", "unread"} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			events := []string{"branch", "round", "branch", "round"}
			if source != "unread" {
				events = append(events, "warm")
			}
			fixture := newUnitFixture(t, "", events...)
			fixture.starter.readVerdict = "VERDICT: fix first (2 material findings)"
			first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
			require(t, err != nil, "%v", err)
			read := filepath.Join(t.TempDir(), "findings.md")
			writeFile(t, read, "1. Broken fold\n2. Lost result\n3. Non-gating observation\n")
			if source == "markdown" {
				_, err = fixture.manager.Store.Update(first.Record.Rounds[0].Steps[2].LaunchID, func(record *Record) error { record.Outputs = []Output{{Path: read}}; return nil })
				require(t, err != nil, "%v", err)
			} else if source == "committed" {
				writeFile(t, read, `{"findings":[{"id":"1","material":true},{"id":"2","material":true},{"id":"3","material":false}]}`)
				first.Record.Subjects = []UnitSubject{{Round: 1, Examination: "critic", ExaminationReturnPath: read}}
			} else {
				first.Record.Rounds[0].Steps = first.Record.Rounds[0].Steps[:2]
			}
			require(t, fixture.runner.save(first.Record) != nil, "save run")
			brief := "Declared size: 1 changed lines\n## Decisions on round 1\n| 1 | fixed | file.go:12 |\n"
			if source != "unread" {
				before, _ := os.ReadFile(filepath.Join(fixture.runner.runDir(first.Record.ID), "run.json"))
				launched := len(fixture.starter.order)
				_, err = fixture.runner.Revise(UnitRevisionRequest{Run: first.Record.ID, After: 1, Brief: []byte(brief)})
				if !IsCode(err, "UNIT_REVISE_UNDECIDED") || ErrorDetail(err) != fmt.Sprintf("UNIT_REVISE_UNDECIDED run=%s round=1 finding=2: finding 2 of round 1 has no decision; nothing was started", first.Record.ID) || err.Error() != "finding 2 of round 1 has no decision; nothing was started" {
					t.Fatalf("refusal: %v", err)
				}
				after, _ := os.ReadFile(filepath.Join(fixture.runner.runDir(first.Record.ID), "run.json"))
				if string(before) != string(after) || len(fixture.starter.order) != launched {
					t.Fatal("undecided correction changed the run or launched")
				}
				brief += "| 2 | refuted | result is retained |\n"
			} else {
				brief = "Declared size: 1 changed lines\nretry\n"
			}
			if _, err := fixture.runner.Revise(UnitRevisionRequest{Run: first.Record.ID, After: 1, Brief: []byte(brief)}); err != nil {
				t.Fatal(err)
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
	decisions := "## Decisions on round 1\n| 1 | follow-up | tracked in goal notes |\n"
	second, err := fixture.runner.Revise(UnitRevisionRequest{Run: first.Record.ID, After: 1, Brief: []byte("Declared size: 1 changed lines\n" + decisions)})
	require(t, err != nil, "%v", err)
	read, err := fixture.manager.Store.Read(second.Record.Rounds[1].Steps[2].LaunchID)
	require(t, err != nil, "%v", err)
	brief, err := os.ReadFile(readString(read.AdapterData, "brief"))
	require(t, err != nil, "%v", err)
	for _, want := range []string{findings, decisions, "Check every fold first, citing the line", "Never re-raise a refuted finding without new evidence", "changed lines only", "fold-not-holding", "same-rule-as N", "Diff since round 1's tree", "+fixed line", "Proof result of round 2", `"state": "passed"`} {
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
	yes, no := true, false
	read := UnitStep{Name: "read", VerdictCounts: &yes, Verdict: "VERDICT: fix first (3 material findings)"}
	record := UnitRunRecord{ID: "run", Unit: "U", Goal: "goal", Rounds: []UnitRound{
		{Number: 1, Steps: []UnitStep{read}}, {Number: 2, Steps: []UnitStep{{Name: "read", VerdictCounts: &no, Verdict: read.Verdict}}},
		{Number: 3}, {Number: 4, Steps: []UnitStep{read}}}}
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
			t.Parallel()
			var events []string
			for index := range counts {
				events = append(events, "branch", "round")
				if index > 0 {
					events = append(events, "warm")
				}
			}
			if admission == "follow-up" || !refused {
				events = append(events, "branch")
			}
			if !refused {
				events = append(events, "round", "warm")
			}
			fixture := newUnitFixture(t, "", events...)
			fixture.starter.readOutput = filepath.Join(t.TempDir(), "findings.md")
			var result UnitResult
			for index, material := range counts {
				fixture.starter.readVerdict = fmt.Sprintf("VERDICT: fix first (%d material findings)", material)
				writeFile(t, fixture.starter.readOutput, "1. Finding\nRELATION: "+relation+"\n2. Observation\n")
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
					findings := []map[string]any{{"material": false}, {"material": false}}
					for n := 0; n < material; n++ {
						findings = append(findings, map[string]any{"material": true})
					}
					if relation != "" {
						findings[0]["relation"] = relation
					}
					data, _ := json.Marshal(map[string]any{"findings": findings})
					path := filepath.Join(t.TempDir(), "return.json")
					writeFile(t, path, string(data))
					result.Record.Subjects = append(result.Record.Subjects, UnitSubject{Round: index + 1,
						Examination: "critic", ExaminationRound: 1, ExaminationReturnPath: path})
					if err := fixture.runner.save(result.Record); err != nil {
						t.Fatal(err)
					}
				}
			}
			// The kept relation must survive replacement of the shared report.
			writeFile(t, fixture.starter.readOutput, "RELATION: new\n")
			run := result.Record.ID
			before, _ := os.ReadFile(filepath.Join(fixture.runner.runDir(run), "run.json"))
			launched := len(fixture.starter.order)
			var err error
			if admission == "revise" {
				brief := fmt.Sprintf("Declared size: 1 changed lines\n## Decisions on round %d\n", len(counts))
				for finding := 1; finding <= 5; finding++ {
					brief += fmt.Sprintf("| %d | follow-up | tracked |\n", finding)
				}
				_, err = fixture.runner.Revise(UnitRevisionRequest{Run: run, Brief: []byte(brief)})
			} else {
				_, err = fixture.runner.Advance(UnitRequest{Resume: run, FollowUp: writeFollowUp(t)})
			}
			if !refused {
				if err != nil || len(fixture.starter.order) <= launched {
					t.Fatalf("admission: %v; launches=%v", err, fixture.starter.order)
				}
				return
			}
			repeats := 0
			if relation != "" && relation != "new" {
				repeats = 1
			}
			facts := fmt.Sprintf("run=%s previous=%d newest=%d repeats=%d", run, counts[0], counts[1], repeats)
			if !IsCode(err, "UNIT_ROUND_DIVERGENT") || !strings.Contains(ErrorDetail(err), facts) ||
				!strings.Contains(err.Error(), fmt.Sprintf("the last two reads of U found %d, then %d material findings, %d of them repeats; nothing was started", counts[0], counts[1], repeats)) {
				t.Fatalf("refusal: %v; detail=%s", err, ErrorDetail(err))
			}
			for _, next := range []string{"take-a-step-back", "work review goal --work U", "work build goal --work NEW --brief FILE --check ..."} {
				if !strings.Contains(ErrorDetail(err)+err.Error(), next) {
					t.Errorf("refusal lacks %q: %v", next, err)
				}
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
	fixture := newUnitFixture(t, "", "branch", "branch", "round", "branch", "branch", "round")
	first, err := fixture.runner.AdvanceNamed(fixture.plan)
	if err != nil || first.Record.State != "awaiting-judgement" {
		t.Fatalf("first attempt: %+v %v", first, err)
	}
	run := first.Record.ID
	brief := []byte("Declared size: 1 changed lines\nfix the finding\n")

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
	fixture := newUnitFixture(t, "", "branch", "branch", "round", "branch", "branch", "round")
	first, err := fixture.runner.AdvanceNamed(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	run := first.Record.ID
	brief := []byte("Declared size: 1 changed lines\nfix F-1\n")
	decisions := []byte("| Finding id | Disposition |\n| F-1 | accepted |\n")
	fixture.starter.failKind = "build"
	failed, err := fixture.runner.Revise(UnitRevisionRequest{Run: run, Brief: brief, Decisions: decisions})
	if err != nil || failed.Revision.Attempt != 2 || failed.Record.Rounds[1].Outcome == "green" {
		t.Fatalf("failed correction: %+v %v", failed, err)
	}
	fixture.starter.failKind = ""
	retried, err := fixture.runner.Revise(UnitRevisionRequest{Run: run, After: 2, Brief: brief, Decisions: decisions})
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
