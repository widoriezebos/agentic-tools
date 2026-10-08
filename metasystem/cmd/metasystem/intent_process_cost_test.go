package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processchange"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
)

func processPublishedSubject(t *testing.T, bed *workBed, run string) string {
	t.Helper()
	runner := &launch.UnitRunner{Root: bed.unitRoot, Manager: bed.manager}
	commit := "subject-" + run
	err := runner.ReviewSubject(run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
		return retain(launch.UnitSubject{Round: review.Round.Number, ExpectedParent: review.Head, DiffDigest: review.DiffDigest, Commit: commit, Published: "unit-tip"})
	})
	if err != nil {
		t.Fatal(err)
	}
	return commit
}

func processReadOwners(bed *workBed, fail *bool, state ...string) intentOwners {
	owners := bed.workOwners()
	owners.delivery = &intentDeliveryOwners{
		branchState: func(string, string) (intentBranchState, error) { return intentBranchState{}, nil },
		branchRead: func([]string) (branch.BranchReadResult, int, error) {
			return branch.BranchReadResult{State: "collected", AttestationCommit: "read-tip"}, 0, nil
		},
		publishRead: func(string, string, string) (branch.PublishReadResult, error) {
			if *fail {
				return branch.PublishReadResult{}, fmt.Errorf("publication unavailable")
			}
			published := "current"
			if len(state) > 0 {
				published = state[0]
			}
			return branch.PublishReadResult{State: published, RemoteTip: "read-tip"}, nil
		},
	}
	owners.work.inspectRead = func(string, string, string) (branch.BranchReadResult, error) {
		return branch.BranchReadResult{State: "collected", Published: !*fail}, nil
	}
	return owners
}

func processReportMeasures(t *testing.T, value any) processmeasure.Measures {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var m processmeasure.Measures
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestProcessCostPublicReport(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	start := time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
	clock := &workClock{now: start}
	bed.manager.Now = clock.Now
	bed.manager.Supervisor = processCostStarter{bed.starter, clock}
	page, accepted := processEstimatePage(t, bed, "70", "10")
	accepted = []byte(strings.ReplaceAll(string(accepted), "| other | 999 |", "| other | 20 |"))
	if err := os.WriteFile(page, accepted, 0600); err != nil {
		t.Fatal(err)
	}
	processCommittedPage(t, bed, page, accepted)
	check := slices.Insert(workArgv, 2, "-timeout", "30m")
	gitHook := bed.workOwnersHook
	bed.workOwnersHook = func(owners *intentWorkOwners) {
		gitHook(owners)
		git := owners.git
		owners.git = func(root string, args ...string) ([]byte, error) {
			if len(args) == 2 && args[0] == "show" && strings.HasSuffix(args[1], ":metasystem.conf") {
				return []byte("proof.full=" + strings.Join(check, " ") + "\n"), nil
			}
			return git(root, args...)
		}
	}
	build := func(unit string) string {
		t.Helper()
		brief := bed.brief(unit+".md", "Build the unit.\n")
		code, built, output := bed.work(append([]string{"work", "build", bed.id, "--work", unit, "--brief", brief, "--check"}, check...)...)
		if code != 0 {
			t.Fatalf("build: %d %+v %s", code, built, output)
		}
		return resultData(t, built)["run"].(string)
	}
	first := build("evidence")
	fix := bed.brief("fix.md", "Correct the unit.\n")
	if code, result, output := bed.work("work", "revise", bed.id, "--work", "evidence", "--after", "1", "--brief", fix); code != 0 {
		t.Fatalf("revise: %d %+v %s", code, result, output)
	}
	fail := false
	publish := func(run string) time.Time {
		t.Helper()
		processPublishedSubject(t, bed, run)
		at := clock.Now()
		if code, result := bed.runJSON(processReadOwners(bed, &fail, "pushed"), "work", "review", "run:"+run); code != 0 {
			t.Fatalf("publish: %d %+v", code, result)
		}
		return at
	}
	publish(first)
	third := build("other")
	publish(third)
	// The same unit is readmitted in a new worktree; the old worktree vanishes.
	old := bed.worktree
	bed.worktree = filepath.Join(filepath.Dir(old), "new-work")
	if err := os.MkdirAll(bed.worktree, 0700); err != nil {
		t.Fatal(err)
	}
	second := build("evidence")
	finish := publish(second)
	if err := os.RemoveAll(old); err != nil {
		t.Fatal(err)
	}
	processCostQuestion(t, bed, channel.Question{ID: "person", Goal: bed.id, State: "answered", OpenedAt: start.Add(5 * time.Minute), Answer: &channel.Answer{At: start.Add(15 * time.Minute)}})
	actsDir := filepath.Join(bed.root(), "process", "acts")
	if err := os.MkdirAll(actsDir, 0700); err != nil {
		t.Fatal(err)
	}
	act := processchange.ProcessAct{ID: "own-act", Goal: bed.id, Actor: "agent", Key: "launch.limit", Status: "proposed", ProposedAt: start, Reason: "recorded process choice"}
	body, _ := json.Marshal(act)
	if err := os.WriteFile(filepath.Join(actsDir, "own-act.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	grantAct := act
	grantAct.ID, grantAct.Actor = "grant-act", "direct-person"
	grantAct.Proof.Helm = &humanauthority.HelmGrant{By: "Wido", Grant: "process-choice"}
	grantBody, err := json.Marshal(grantAct)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(actsDir, "grant-act.json"), grantBody, 0600); err != nil {
		t.Fatal(err)
	}
	report := func() (processmeasure.Measures, map[string]processmeasure.Measures, map[string]any) {
		t.Helper()
		code, result, _ := bed.work("goal", "status", bed.id)
		if code != 0 {
			t.Fatalf("goal status: %d %+v", code, result)
		}
		data := resultData(t, result)
		units := map[string]processmeasure.Measures{}
		for _, raw := range data["work"].([]any) {
			view := raw.(map[string]any)
			units[view["work"].(string)] = processReportMeasures(t, view["measures"])
			if view["work"] == "evidence" && (view["state"] != "awaiting-judgement" || view["run"] != second) {
				t.Fatalf("old run hid current state: %+v", view)
			}
		}
		return processReportMeasures(t, data["measures"]), units, data
	}
	total, units, data := report()
	for kind, minutes := range map[string]float64{"build": 3, "attest": 4, "read": 0, "correction": 1, "person": 10} {
		if total.Hours[kind] == nil || fmt.Sprintf("%.6f", *total.Hours[kind]) != fmt.Sprintf("%.6f", minutes/60) {
			t.Fatalf("goal %s: %+v", kind, total)
		}
	}
	if len(units) != 2 || units["evidence"].Hours["build"] == nil || *units["evidence"].Hours["build"] != (2*time.Minute).Hours() || *units["other"].Hours["build"] != (time.Minute).Hours() {
		t.Fatalf("unit costs: %+v", units)
	}
	if total.EstimateMinutes == nil || *total.EstimateMinutes != 90 || total.Corrections != 1 || total.SuiteMinutes == nil || *total.SuiteMinutes != 4 || total.ElapsedFinish != nil || total.ElapsedHours == nil || *total.ElapsedHours != clock.Now().Sub(start.Add(2*time.Minute)).Hours() || !total.ElapsedLowerBound || units["evidence"].ElapsedFinish == nil || !units["evidence"].ElapsedFinish.Equal(finish) || !slices.Contains(total.Unknown, "fix units unavailable") {
		t.Fatalf("goal cost: %+v", total)
	}
	acts := data["processActs"].([]any)
	if len(acts) != 2 || !slices.ContainsFunc(acts, func(raw any) bool { return raw.(map[string]any)["ID"] == "own-act" }) || !slices.ContainsFunc(acts, func(raw any) bool { return raw.(map[string]any)["ID"] == "grant-act" }) {
		t.Fatalf("process acts: %+v", data)
	}
	// A status read cannot retain derived totals. New execution evidence changes it.
	source := units["evidence"].Sources[0]
	if _, err := bed.manager.Store.Update(source, func(r *launch.Record) error {
		at, _ := time.Parse(time.RFC3339Nano, r.FinishedAt)
		r.FinishedAt = at.Add(time.Minute).Format(time.RFC3339Nano)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	updated, _, _ := report()
	if *updated.Hours["build"] != (4 * time.Minute).Hours() {
		t.Fatalf("total was stored instead of derived: %+v", updated)
	}
	code, text, stderr := bed.run(processReadOwners(bed, &fail), "goal", "status", bed.id)
	if code != 0 {
		t.Fatalf("text status: %d %s %s", code, text, stderr)
	}
	for _, fragment := range []string{"process act own-act: own-caused agent", "process act grant-act: own-caused agent", "evidence: hours:", "other: hours:", "goal total:", "estimate 90 minutes", "fix units unavailable", "elapsed finish " + finish.In(time.Local).Format(time.RFC3339)} {
		if !strings.Contains(strings.Join(strings.Fields(text), " "), fragment) {
			t.Fatalf("missing %q: %s", fragment, text)
		}
	}
	if strings.Index(text, "own-caused agent") > strings.Index(text, "goal total:") {
		t.Fatalf("own process act was not first: %s", text)
	}
	// A newer revision has no published read; an older publication cannot finish it.
	fix = bed.brief("next-fix.md", "Correct the current unit.\n")
	if code, result, output := bed.work("work", "revise", bed.id, "--work", "evidence", "--after", "1", "--brief", fix); code != 0 {
		t.Fatalf("new revision: %d %+v %s", code, result, output)
	}
	open, _, _ := report()
	if open.ElapsedFinish != nil || !open.ElapsedLowerBound || open.Corrections != 2 {
		t.Fatalf("old publication hid open unit: %+v", open)
	}
	// Damaged advisory history is visible without refusing status.
	actPath := filepath.Join(actsDir, "own-act.json")
	if err := os.WriteFile(actPath, []byte("damaged"), 0600); err != nil {
		t.Fatal(err)
	}
	if code, result, _ := bed.work("goal", "status", bed.id); code != 0 || !strings.Contains(jsonText(result.Data), "own-act.json") {
		t.Fatalf("bad process act hidden: %d %+v", code, result)
	}
	if err := os.WriteFile(actPath, body, 0600); err != nil {
		t.Fatal(err)
	}
	// Corrupt retained evidence is explicitly unknown, never a read failure.
	path := filepath.Join(bed.unitRoot, first, "run.json")
	if err := os.WriteFile(path, []byte("damaged"), 0600); err != nil {
		t.Fatal(err)
	}
	if code, result, _ := bed.work("goal", "status", bed.id); code != 0 || !strings.Contains(jsonText(result.Data), first) {
		t.Fatalf("bad historical run hidden: %d %+v", code, result)
	}
}

func TestProcessPublicationPublicReview(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"run", "commit"} {
		for _, publication := range []string{"current", "reconciled", "adopted", "pushed"} {
			t.Run(verb+"/"+publication, func(t *testing.T) {
				t.Parallel()
				bed := newWorkBed(t)
				clock := &workClock{now: time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)}
				bed.manager.Now = clock.Now
				bed.manager.Supervisor = processCostStarter{bed.starter, clock}
				code, built, _ := processEvidenceBuild(bed)
				if code != 0 {
					t.Fatalf("build: %d %+v", code, built)
				}
				run := resultData(t, built)["run"].(string)
				commit := processPublishedSubject(t, bed, run)
				runner := &launch.UnitRunner{Root: bed.unitRoot}
				stamp := func() string {
					t.Helper()
					r, err := runner.Status(run)
					if err != nil {
						t.Fatal(err)
					}
					return r.Subjects[0].PublishedAt
				}
				args := []string{"work", "review", "run:" + run}
				replay := []string{"work", "review", "--commit", commit, "--goal", bed.id}
				if verb == "commit" {
					args, replay = replay, args
				}
				fail := true
				if code, result := bed.runJSON(processReadOwners(bed, &fail), args...); code == 0 || result.Outcome != intentPartial || stamp() != "" {
					t.Fatalf("failed publication got a finish: %d %+v %s", code, result, stamp())
				}
				clock.Sleep(time.Hour)
				fail = false
				at := clock.Now()
				want := ""
				if publication == "pushed" {
					want = at.Format(time.RFC3339Nano)
				}
				if code, result := bed.runJSON(processReadOwners(bed, &fail, publication), args...); code != 0 || stamp() != want {
					t.Fatalf("publication time: %d %+v got %s want %s", code, result, stamp(), want)
				}
				clock.Sleep(time.Hour)
				if code, result := bed.runJSON(processReadOwners(bed, &fail, publication), replay...); code != 0 || stamp() != want {
					t.Fatalf("publication replay changed time: %d %+v %s", code, result, stamp())
				}
				code, result, _ := bed.work("work", "status", bed.id, "--work", "evidence")
				m := processReportMeasures(t, resultData(t, result)["work"].([]any)[0].(map[string]any)["measures"])
				if code != 0 || (want == "" && (m.ElapsedFinish != nil || !m.ElapsedLowerBound)) || (want != "" && (m.ElapsedFinish == nil || !m.ElapsedFinish.Equal(at) || m.ElapsedLowerBound)) {
					t.Fatalf("named status publication: %d %+v", code, m)
				}
			})
		}
	}
}

func TestProcessStatusDamagedUnrelatedHistory(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	if code, result, _ := processEvidenceBuild(bed); code != 0 {
		t.Fatalf("build: %d %+v", code, result)
	}
	if err := os.MkdirAll(filepath.Join(bed.unitRoot, "unrelated-run"), 0700); err != nil {
		t.Fatal(err)
	}
	acts := filepath.Join(bed.root(), "process", "acts")
	if err := os.MkdirAll(acts, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(acts, "other-goal.json"), []byte(`{"Goal":"another-goal","Status":`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(bed.unitRoot, "damaged-run"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bed.unitRoot, "damaged-run", "run.json"), []byte(`{"Goal":"another-goal","Rounds":`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"status", bed.id}, {"goal", "status", bed.id}} {
		code, result, _ := bed.work(args...)
		if code != 0 {
			t.Fatalf("%v: %d %+v", args, code, result)
		}
		m := processReportMeasures(t, resultData(t, result)["measures"])
		if !strings.Contains(jsonText(m.Unknown), "other-goal.json") || !strings.Contains(jsonText(m.Unknown), "damaged-run") {
			t.Fatalf("missing unknown history: %+v", m)
		}
	}
	code, output, stderr := bed.run(bed.workOwners(), "status", bed.id)
	if code != 0 || !strings.Contains(output, "run damaged-run unavailable") || !strings.Contains(output, "process act other-goal.json unavailable") || strings.Contains(output, "unrelated-run") {
		t.Fatalf("text status: %d %s %s", code, output, stderr)
	}
}

func TestProcessGoalFinishPublicStatus(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	clock := &workClock{now: time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)}
	bed.manager.Now = clock.Now
	bed.manager.Supervisor = processCostStarter{bed.starter, clock}
	page, accepted := processEstimatePage(t, bed, "70", "10")
	processCommittedPage(t, bed, page, accepted)
	code, built, _ := processEvidenceBuild(bed)
	if code != 0 {
		t.Fatalf("build: %d %+v", code, built)
	}
	run := resultData(t, built)["run"].(string)
	processPublishedSubject(t, bed, run)
	fail := false
	if code, result := bed.runJSON(processReadOwners(bed, &fail, "pushed"), "work", "review", "run:"+run); code != 0 {
		t.Fatalf("publish: %d %+v", code, result)
	}
	for _, done := range []bool{false, true} {
		if done {
			file := bed.goalFile(bed.id)
			file.State = goal.StateDone
			file.Claimed, file.StopCapability = nil, nil
			file.Conclude = "recorded completion"
			bed.addGoal(file)
		}
		code, result, _ := bed.work("goal", "status", bed.id)
		if code != 0 {
			t.Fatalf("status: %d %+v", code, result)
		}
		data := resultData(t, result)
		total := processReportMeasures(t, data["measures"])
		unit := processReportMeasures(t, data["work"].([]any)[0].(map[string]any)["measures"])
		if (total.ElapsedFinish != nil) != done || total.ElapsedLowerBound == done || unit.ElapsedFinish == nil || unit.ElapsedLowerBound {
			t.Fatalf("done=%t goal=%+v unit=%+v", done, total, unit)
		}
	}
}

func TestProcessManualPublicStatus(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	owners := bed.workOwners()
	owners.connection.endpointTip = func(string, goal.Endpoint) (string, error) { return "base", nil }
	git := owners.work.git
	owners.work.git = func(root string, args ...string) ([]byte, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --verify HEAD^{commit}":
			return []byte("manual"), nil
		case "merge-base base manual":
			return []byte("base"), nil
		case "rev-list --first-parent --reverse --parents base..manual", "rev-list --parents -n 1 manual":
			return []byte("manual base"), nil
		case "show -s --format=%(trailers:only,unfold=true) manual":
			return []byte("Goal-Unit: " + bed.id + "/handmade"), nil
		case "diff-tree -r -z --no-renames --full-index manual^ manual":
			return []byte(":000000 100644 " + strings.Repeat("0", 40) + " " + strings.Repeat("a", 40) + " A\x00manual.go\x00"), nil
		}
		return git(root, args...)
	}
	owners.work.inspectRead = func(string, string, string) (branch.BranchReadResult, error) { return branch.BranchReadResult{}, nil }
	for _, args := range [][]string{{"status", bed.id}, {"goal", "status", bed.id}} {
		code, result := bed.runJSON(owners, args...)
		if code != 0 || !strings.Contains(result.Summary, "work handmade") || result.Next == nil || !slices.Equal(result.Next.Argv[1:], []string{"work", "review", bed.id, "--work", "handmade"}) {
			t.Fatalf("manual status: %d %+v", code, result)
		}
	}
}
