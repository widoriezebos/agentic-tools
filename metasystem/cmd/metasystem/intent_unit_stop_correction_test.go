package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func TestIntentContinuingUnitRefusesDropAndSplitBeforeRetention(t *testing.T) {
	t.Parallel()
	for _, decision := range []string{"dropped: person proposes dropping the finding", "split: destination"} {
		t.Run(strings.Split(decision, ":")[0], func(t *testing.T) {
			t.Parallel()
			fixture := newTransferScenarioFixture(t, true)
			b, owners := fixture.bed, fixture.owners
			// The first examination had more material and no matching class.
			record, err := fixture.runner.Status(fixture.run)
			if err != nil {
				t.Fatal(err)
			}
			record.Rounds[0].Reads[0].Findings = nil
			transferWriteJSON(t, filepath.Join(b.unitRoot, fixture.run, "run.json"), record)
			code, pending := transferPublic(t, b, owners, "work", "review", b.id, "--work", "stopped")
			if code != 1 || pending.Outcome != intentInProgress {
				t.Fatalf("decisions template: %d %+v", code, pending)
			}
			record, err = fixture.runner.Status(fixture.run)
			if err != nil || record.Rounds[1].Stop.Decision != "continue" {
				t.Fatalf("unit did not continue: %+v %v", record, err)
			}
			path := resultData(t, pending)["template"].(string)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			proposal := b.brief("person-decisions.md", strings.ReplaceAll(string(before), "DECIDE", decision))
			source := filepath.Join(fixture.agents, "jobs", fixture.critic+".json")
			sourceBefore, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			code, refused := transferPublic(t, b, owners, "work", "review", b.id, "--work", "stopped", "--dispositions", proposal)
			if code == 0 || refused.Outcome != intentRefused {
				t.Fatalf("unsupported decision retained: %d %+v", code, refused)
			}
			if strings.HasPrefix(decision, "dropped:") && (!strings.Contains(strings.ToLower(refused.Summary), "cannot be dropped yet") || refused.Next == nil || !strings.Contains(strings.Join(refused.Next.Argv, " "), "work revise")) {
				t.Fatalf("drop refusal lacks working act: %+v", refused)
			}
			if strings.HasPrefix(decision, "split:") && !strings.Contains(refused.Summary, "recorded stop") {
				t.Fatalf("split refusal lacks stop requirement: %+v", refused)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("refusal retained decisions: %s %v", after, err)
			}
			sourceAfter, err := os.ReadFile(source)
			if err != nil || !bytes.Equal(sourceBefore, sourceAfter) || fixture.observer.publications != 0 || len(b.goalFile(b.id).ReviewObligations) != 0 {
				t.Fatalf("refusal changed source or completion debt: %v", err)
			}
		})
	}
}

func TestIntentRoundTwoCloseWithDispositionsCollectsExamination(t *testing.T) {
	t.Parallel()
	fixture := newTransferScenarioFixture(t, true)
	b, owners := fixture.bed, fixture.owners
	record, err := fixture.runner.Status(fixture.run)
	if err != nil {
		t.Fatal(err)
	}
	// The committed examination has finished but has not been collected.
	record.Rounds[0].Reads[0].Findings = nil
	record.Rounds[1].Stop = nil
	transferWriteJSON(t, filepath.Join(b.unitRoot, fixture.run, "run.json"), record)
	returnPath := filepath.Join(fixture.agents, fixture.critic, "rounds", "1", "return.json")
	digest, _, err := reviewReturnDigest(returnPath)
	if err != nil {
		t.Fatal(err)
	}
	findings, _, err := readIntentFindings(returnPath)
	if err != nil {
		t.Fatal(err)
	}
	binding := reviewBinding{Goal: b.id, Work: "stopped", Attempt: 2, Subject: fixture.commit, Examination: fixture.critic, Round: 1, Return: digest}
	decisions := strings.ReplaceAll(decisionsDocument(binding, findings), "DECIDE", "accepted")
	path := b.brief("close-decisions.md", decisions)
	code, result := transferPublic(t, b, owners, "work", "review", b.id, "--work", "stopped", "--dispositions", path)
	if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "need a fix") || result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), "work revise") {
		t.Fatalf("first close did not decide the accepted findings: %d %+v", code, result)
	}
	after, err := fixture.runner.Status(fixture.run)
	if err != nil || after.Rounds[1].Stop == nil || after.Rounds[1].Stop.Decision != "continue" || len(after.Subjects) != 1 || after.Subjects[0].Examination != fixture.critic || after.Subjects[0].ExaminationReturnPath != returnPath || after.Subjects[0].Drop != nil {
		t.Fatalf("first close did not collect the examination: %+v %v", after, err)
	}
	saved, err := os.ReadFile(filepath.Join(filepath.Dir(returnPath), "decisions.md"))
	if err != nil || string(saved) != decisions || fixture.observer.publications != 0 || len(b.goalFile(b.id).ReviewObligations) != 0 {
		t.Fatalf("ordinary decisions were not retained without a transfer: %q %v", saved, err)
	}
}

func TestWorkReviewUnknownCommittedUnitLaunchesFreshExamination(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	brief := bed.brief("build.md", "Build the unit.\n")
	code, built, _ := bed.work(append([]string{"work", "build", bed.id, "--work", "u1", "--brief", brief, "--lines", "5"}, workCheck...)...)
	if code != 0 || built.Outcome != intentConfirmed {
		t.Fatalf("build: exit=%d result=%+v", code, built)
	}
	run := resultData(t, built)["run"].(string)
	unit := strings.Repeat("b", 40)
	const critic = "unknown-committed-reader"
	repository := correctedBriefRepository{root: bed.worktree, unit: unit, goal: bed.id}
	subject, err := repository.Subject(bed.worktree, unit)
	if err != nil {
		t.Fatal(err)
	}
	roundDir := filepath.Join(bed.worktree, "artifacts", "agents", critic, "rounds", "1")
	returnPath := filepath.Join(roundDir, "return.json")
	request := branch.BranchReadRequest{Repo: bed.worktree, Remote: "origin", EndpointTip: subject.Parent, BranchTip: unit,
		GoalID: bed.id, UnitCommit: unit, Repository: repository, BriefPath: filepath.Join(bed.root(), brief),
		CheckClaim: func() error { return nil }, Gate: func(string) (string, error) { return "green", nil },
		Delegate: func(string, string, string, string, string) (string, error) {
			transferWriteJSON(t, filepath.Join(bed.worktree, "artifacts", "agents", "jobs", critic+".json"), map[string]any{
				"jobId": critic, "role": "code-critic", "round": 1, "status": "completed", "reviews": "commit:" + unit,
				"goalId": bed.id, "engineBuild": "fixture-engine", "effectiveModel": "fixture-reader", "findingRegister": []any{},
			})
			transferWriteJSON(t, filepath.Join(roundDir, "subject.json"), readsubject.ReadSubject{
				Kind: readsubject.SubjectCommit, Commit: unit, Parent: subject.Parent, Tree: subject.Tree, DiffDigest: subject.PatchDigest,
			})
			// A material finding without its class makes the stop inputs unknown.
			finding := stopFinding("", "unit.go")
			finding.ID = "F1"
			transferWriteJSON(t, returnPath, map[string]any{"jobId": critic, "round": 1, "reviewedTree": subject.Tree,
				"verdictMaterialCount": 1, "findings": []readsubject.Finding{finding}})
			if err := os.WriteFile(filepath.Join(roundDir, "return.md"), []byte("VERDICT: REVISE material=1\n"), 0600); err != nil {
				t.Fatal(err)
			}
			return critic, nil
		},
	}
	if result, err := branch.RunBranchRead(request); err != nil || result.State != "dispatched" {
		t.Fatalf("initial examination: result=%+v err=%v", result, err)
	}
	runner := &launch.UnitRunner{Root: bed.unitRoot, Manager: bed.manager, Git: workGit{bed},
		ReviewPolicy: func() (string, error) { return "auto", nil }}
	if err := runner.ReviewSubject(run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
		return retain(launch.UnitSubject{Round: review.Round.Number, Commit: unit, Tip: unit, Published: unit,
			StagedTree: subject.Tree, DiffDigest: review.DiffDigest, Examination: critic, ExaminationJob: critic,
			ExaminationRound: 1, ExaminationReturnPath: returnPath})
	}); err != nil {
		t.Fatal(err)
	}
	before, err := runner.Status(run)
	if err != nil || len(before.Subjects) != 1 || before.Subjects[0].Commit != unit || before.Rounds[0].Stop == nil ||
		before.Rounds[0].Stop.Decision != "stop" || before.Rounds[0].Stop.Handoff != "stopped unavailable-stop-inputs" || before.Rounds[0].UnknownRetries != 0 {
		t.Fatalf("committed unit did not stop for unknown inputs: record=%+v err=%v", before, err)
	}
	bed.head = unit
	launches := bed.starter.launched()
	owners := bed.workOwners()
	owners.work.inspectRead = func(string, string, string) (branch.BranchReadResult, error) {
		return branch.BranchReadResult{RootJob: critic}, nil
	}
	fresh := 0
	request.FollowUp = func(root, frozenBrief string) (string, error) {
		if root != critic || len(mustRead(t, frozenBrief)) == 0 {
			t.Fatalf("fresh examination has wrong chain or no brief: root=%q brief=%q", root, frozenBrief)
		}
		fresh++
		child := critic + "-r2"
		transferWriteJSON(t, filepath.Join(bed.worktree, "artifacts", "agents", "jobs", child+".json"), map[string]any{
			"jobId": child, "role": "code-critic", "round": 2, "parentJob": critic, "status": "running", "reviews": "commit:" + unit,
		})
		return child, nil
	}
	owners.delivery = &intentDeliveryOwners{branchRead: func(args []string) (branch.BranchReadResult, int, error) {
		if flagValue(args, "--unit") != unit || flagValue(args, "--goal") != bed.id || flagValue(args, "--root") != bed.worktree {
			t.Fatalf("retry changed the committed subject: %v", args)
		}
		retried := request
		retried.BriefPath = flagValue(args, "--brief")
		retried.Join = slices.Contains(args, "--join")
		retryRound, err := strconv.ParseInt(flagValue(args, "--retry"), 10, 64)
		if err != nil || retryRound != 1 {
			t.Fatalf("retry did not name examination round 1: args=%v err=%v", args, err)
		}
		retried.Retry = retryRound
		result, err := branch.RunBranchRead(retried)
		if err != nil {
			return result, 1, err
		}
		return result, 0, nil
	}}
	for _, state := range []string{"dispatched", "retry-joined"} {
		code, result := bed.runJSON(owners, "work", "review", bed.id, "--work", "u1", "--retry", "1")
		if code != 1 || result.Outcome != intentInProgress || resultData(t, result)["state"] != state || fresh != 1 {
			t.Fatalf("fresh examination %s: exit=%d result=%+v launches=%d", state, code, result, fresh)
		}
	}
	after, err := runner.Status(run)
	if err != nil || len(after.Rounds) != len(before.Rounds) || len(after.Revisions) != len(before.Revisions) ||
		after.Subjects[0].Commit != unit || !slices.Equal(launches, bed.starter.launched()) {
		t.Fatalf("examination retry changed the unit or launched another build: record=%+v err=%v", after, err)
	}
}

func TestIntentPersonStopTransferRequiresProvenPerson(t *testing.T) {
	t.Parallel()
	fixture := newTransferScenarioFixture(t, true)
	b, owners := fixture.bed, fixture.owners
	owners.lookupEnv = func(key string) (string, bool) { return "person", key == config.EnvName("review.stop") }
	personProver := enrolledPersonProver(t, b.root(), b.manager.Now())
	owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, humanauthority.Refusedf(humanauthority.OutcomeAgent, "an agent started this shell")
	}
	code, pending := transferPublic(t, b, owners, "work", "review", b.id, "--work", "stopped")
	if code != 1 || pending.Outcome != intentInProgress || pending.Next == nil {
		t.Fatalf("person policy did not ask: %d %+v", code, pending)
	}
	questions, damaged := channel.WalkOpenQuestions(b.stateRoot())
	if len(damaged) != 0 || len(questions) != 2 {
		t.Fatalf("missing transfer asks: %+v %v", questions, damaged)
	}
	for _, q := range questions {
		if q.UnitStop == nil || !strings.Contains(q.UnitStop.Needs, "--by NAME") {
			t.Fatalf("ask omitted person: %+v", q)
		}
	}
	args := append([]string(nil), pending.Next.Argv[1:]...)
	code, refused := transferPublic(t, b, owners, args...)
	if code == 0 || refused.Outcome != intentRefused || fixture.observer.publications != 0 || len(b.goalFile(b.id).ReviewObligations) != 0 {
		t.Fatalf("agent transferred person-held findings: %d %+v", code, refused)
	}
	owners.prove = personProver
	for i := range args {
		if args[i] == "NAME" {
			args[i] = "Wido"
		}
	}
	code, transferred := transferPublic(t, b, owners, args...)
	if code != 0 || transferred.Outcome != intentConfirmed || fixture.observer.publications != 1 || len(b.goalFile(b.id).ReviewObligations) != 2 {
		t.Fatalf("proven person could not transfer: %d %+v", code, transferred)
	}
}

func TestIntentCorruptReviewStopNamesSettingRepair(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"env", "conf-local", "conf"} {
		for _, admitted := range []bool{false, true} {
			t.Run(source+"/"+map[bool]string{false: "admission", true: "collected review"}[admitted], func(t *testing.T) {
				t.Parallel()
				fixture := newTransferScenarioFixture(t, true)
				b, owners := fixture.bed, fixture.owners
				owners.lookupEnv = func(key string) (string, bool) {
					return "corrupt", source == "env" && key == config.EnvName("review.stop")
				}
				layout, err := owners.resolver.ResolveLayout(b.root())
				if err != nil {
					t.Fatal(err)
				}
				conf := intentConfPath(layout)
				want := []string{"unset", config.EnvName("review.stop")}
				if source != "env" {
					path := conf
					if source == "conf-local" {
						path += ".local"
						want = []string{"metasystem", "settings", "set", "review.stop", "auto", "--repo", filepath.Dir(conf)}
					} else {
						want = []string{"edit", conf}
					}
					file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
					if err != nil {
						t.Fatal(err)
					}
					_, writeErr := file.WriteString("\nreview.stop=corrupt\n")
					closeErr := file.Close()
					if writeErr != nil || closeErr != nil {
						t.Fatalf("corrupt fixture policy: %v %v", writeErr, closeErr)
					}
				}
				args := []string{"work", "review", b.id, "--work", "stopped"}
				if !admitted {
					if _, err := (&launch.UnitRunner{Manager: b.manager, Root: b.unitRoot, Git: workGit{b}}).CancelRun(fixture.run); err != nil {
						t.Fatal(err)
					}
					args = append([]string{"work", "build", b.id, "other", "--brief", b.brief("other.md", "Build another unit.\n"), "--lines", "5"}, workCheck...)
				}
				launches := len(b.starter.launched())
				code, refused := transferPublic(t, b, owners, args...)
				if code == 0 || !strings.Contains(refused.Summary, shellCommand(want)) || len(b.starter.launched()) != launches || fixture.observer.publications != 0 {
					t.Fatalf("corrupt policy has no actionable repair or changed work: %d %+v", code, refused)
				}
				if admitted && (refused.Next == nil || !slices.Equal(refused.Next.Argv, want)) {
					t.Fatalf("printed next act differs from the repair: %+v, want %v", refused.Next, want)
				}
			})
		}
	}
}

func TestIntentReviewProceedsAfterPolicyRepair(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"conf-local", "conf-local-from-subdirectory", "env"} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			testIntentReviewPolicyRepair(t, source)
		})
	}
}

func testIntentReviewPolicyRepair(t *testing.T, source string) {
	t.Helper()
	fixture := newTransferScenarioFixture(t, true)
	b, owners := fixture.bed, fixture.owners
	layout, err := owners.resolver.ResolveLayout(b.root())
	if err != nil {
		t.Fatal(err)
	}
	// Each parallel fixture has its own environment and configuration files.
	env := map[string]string{}
	owners.lookupEnv = func(key string) (string, bool) { value, ok := env[key]; return value, ok }
	if source == "env" {
		env[config.EnvName("review.stop")] = "corrupt"
	} else if err := os.WriteFile(intentConfPath(layout)+".local", []byte("review.stop=corrupt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	now, err := owners.commandNow(b.root())
	if err != nil {
		t.Fatal(err)
	}
	owners.prove = enrolledPersonProver(t, b.root(), now)
	args := []string{"work", "review", b.id, "--work", "stopped"}
	if source == "conf-local-from-subdirectory" {
		// The review's own --repo differs from the owner checkout the repair must name once.
		sub := filepath.Join(b.root(), "sub")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		args = append(args, "--repo", sub)
	}
	launches := len(b.starter.launched())
	code, refused := transferPublic(t, b, owners, args...)
	if code != 1 || refused.Outcome != intentRefused || refused.Next == nil {
		t.Fatalf("corrupt policy did not refuse with its repair: %d %+v", code, refused)
	}
	before, err := fixture.runner.Status(fixture.run)
	if err != nil || before.Rounds[1].Stop == nil || before.Rounds[1].Stop.Handoff != "stopped unreadable-policy" {
		t.Fatalf("unreadable policy was not retained: %+v %v", before, err)
	}
	repair := refused.Next.Argv
	t.Logf("work review refused: %s; printed repair: %s", refused.Summary, shellCommand(repair))
	if repair[0] == "unset" {
		if !slices.Equal(repair, []string{"unset", config.EnvName("review.stop")}) {
			t.Fatalf("repair unsets the wrong environment variable: %v", repair)
		}
		delete(env, repair[1])
	} else {
		if repair[0] != "metasystem" {
			t.Fatalf("repair is not a public command: %v", repair)
		}
		code, repaired := transferPublic(t, b, owners, repair[1:]...)
		if code != 0 || repaired.Outcome != intentConfirmed {
			t.Fatalf("printed repair failed: %d %+v", code, repaired)
		}
		t.Logf("printed repair succeeded: %s", repaired.Summary)
	}
	code, reviewed := transferPublic(t, b, owners, args...)
	if code != 0 || reviewed.Outcome != intentConfirmed || fixture.observer.publications != 1 || len(b.goalFile(b.id).ReviewObligations) != 2 {
		t.Fatalf("same review stayed blocked after policy repair: %d %+v", code, reviewed)
	}
	t.Logf("work review proceeded after repair: %s", reviewed.Summary)
	after, err := fixture.runner.Status(fixture.run)
	if err != nil || len(after.Rounds) != len(before.Rounds) || len(b.starter.launched()) != launches || after.Rounds[1].Stop.Attempt != before.Rounds[1].Stop.Attempt || after.Subjects[0].Examination != before.Subjects[0].Examination {
		t.Fatalf("repair changed the attempt or examination: %+v %v", after, err)
	}
}
