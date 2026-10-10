package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// The repository observer checks publication ordering without replacing the
// transfer or source-closure owners.
type transferPublicationObserver struct {
	goal.Repository
	t                    *testing.T
	root, critic, goalID string
	publications         int
}

func (r *transferPublicationObserver) Publish(parent, commit string) (goal.CASOutcome, error) {
	files, err := r.Repository.Files(commit, "plans/goals/")
	if err != nil {
		return "", err
	}
	for name, data := range files {
		if !strings.HasSuffix(name, "/"+r.goalID+".md") {
			continue
		}
		file, problems := goal.ParseFile(data)
		if len(problems) > 0 {
			r.t.Fatalf("candidate goal: %v", problems)
		}
		if len(file.History) > 0 && file.History[len(file.History)-1].Verb == "defer-findings" {
			r.publications++
			var record map[string]any
			body, err := os.ReadFile(filepath.Join(r.root, "artifacts", "agents", "jobs", r.critic+".json"))
			if err != nil || json.Unmarshal(body, &record) != nil {
				r.t.Fatalf("source record before publication: %v", err)
			}
			if record["chainClosed"] == true {
				r.t.Fatal("source closed before destination obligations were published")
			}
			if len(file.ReviewObligations) != 2 || file.ReviewObligations[0].State != "open" {
				r.t.Fatalf("publication lacks open inherited evidence: %+v", file.ReviewObligations)
			}
		}
	}
	return r.Repository.Publish(parent, commit)
}

func transferWriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
}

func transferPublic(t *testing.T, b *workBed, owners intentOwners, args ...string) (int, intentResult) {
	t.Helper()
	command, rest, ok := resolveIntentArgv(args)
	if !ok {
		t.Fatalf("unknown public command: %v", args)
	}
	var out, errors bytes.Buffer
	code := runIntentIn(command, append([]string{"--json"}, rest...), &out, &errors, b.root(), owners)
	var result intentResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("public %v: %v output=%q errors=%q", args, err, out.String(), errors.String())
	}
	b.recordReadDirs(result)
	return code, result
}

type transferCritiqueFacts struct {
	paths []string
	tree  string
}

func (f transferCritiqueFacts) ChangedPaths(string, string) ([]string, error) {
	return append([]string(nil), f.paths...), nil
}
func (f transferCritiqueFacts) CommitTree(string, string) (string, error)  { return f.tree, nil }
func (f transferCritiqueFacts) InstallPrefix(string) (string, error)       { return "", nil }
func (f transferCritiqueFacts) ArtifactAbsent(string, string, string) bool { return false }

type transferScenarioFixture struct {
	bed                               *workBed
	owners                            intentOwners
	runner                            *launch.UnitRunner
	run, page, commit, critic, agents string
	findings                          []readsubject.Finding
	admitted                          map[string]any
	observer                          *transferPublicationObserver
}

func newTransferScenarioFixture(t *testing.T, required bool, goalID ...string) transferScenarioFixture {
	t.Helper()
	b := newStopWorkBed(t)
	if len(goalID) > 0 {
		file := b.goalFile(b.id)
		oldPath := "plans/goals/" + b.id + ".md"
		delete(b.repo.commit(b.repo.accepted).files, oldPath)
		if err := os.Remove(filepath.Join(b.root(), filepath.FromSlash(oldPath))); err != nil {
			t.Fatal(err)
		}
		file.Id, b.id = goalID[0], goalID[0]
		for i := range file.History {
			file.History[i].Targets = []string{b.id}
		}
		b.addGoal(file)
		b.initialBinding.GoalID, b.initialBinding.File = b.id, file
	}
	b.lineage = b.goalFile(b.id).Claimed.Lineage
	b.manager.Supervisor = &stopReadStarter{bed: b, reads: [][]readsubject.Finding{{stopFinding("regression", "source.go"), stopFinding("weakened-test", "source_test.go"), stopFinding("incomplete-item", "requirement.go")}, {stopFinding("regression", "newfile.go"), stopFinding("missing-reader", "reader.go")}}}
	declared := "stopped"
	if !required {
		declared = "required-other"
	}
	page, data := designGatePage(t, b, "- Critique: closed at round 1 on 0 material findings (reader)",
		"\n| Unit | Purpose | Estimated changed lines |\n| --- | --- | --- |\n| "+declared+" | Complete the required behavior | 5 |\n")
	processCommittedPage(t, b, page, data)
	run, before, _ := stopBuild(t, b, "auto")
	if before.Rounds[0].Stop == nil || before.Rounds[0].Stop.Decision != "continue" {
		t.Fatalf("source did not admit its first correction: %+v", before)
	}
	var correction strings.Builder
	correction.WriteString("Correct the findings.\n\n## Decisions on round 1\n\n| Finding | Decision | Evidence |\n| --- | --- | --- |\n")
	for _, finding := range before.Rounds[0].Reads[0].Findings {
		fmt.Fprintf(&correction, "| %s | fixed | %s:12 |\n", finding.ID, finding.Where)
	}
	brief := b.brief("transfer-correction.md", correction.String())
	code, revised, _ := stopPublic(t, b, "auto", "work", "revise", b.id, "--work", "stopped", "--brief", brief)
	if code != 0 {
		t.Fatalf("first correction: %d %+v", code, revised)
	}
	runner := &launch.UnitRunner{Root: b.unitRoot, Manager: b.manager, Git: workGit{b}}
	before, err := runner.Status(run)
	if err != nil || len(before.Rounds) != 2 || before.Rounds[1].Stop == nil || before.Rounds[1].Stop.Decision != "stop" || before.Rounds[1].Material != 2 {
		t.Fatalf("repeated regression did not stop the source: %+v %v", before, err)
	}
	const critic = "transfer-source-reader"
	commit, tree := strings.Repeat("a", 40), strings.Repeat("b", 40)
	if err := runner.ReviewSubject(run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
		return retain(launch.UnitSubject{Round: review.Round.Number, Commit: commit, Published: commit, Tip: commit, StagedTree: tree, DiffDigest: review.DiffDigest})
	}); err != nil {
		t.Fatal(err)
	}
	b.head = commit
	// The model transport completes a canonical examination. Immutable
	// repository facts bind the real register to the source commit.
	agents := filepath.Join(b.worktree, "artifacts", "agents")
	roundDir := filepath.Join(agents, critic, "rounds", "1")
	subject := readsubject.ReadSubject{Kind: readsubject.SubjectCommit, Commit: commit, Tree: tree, Parent: strings.Repeat("c", 40), DiffDigest: strings.Repeat("d", 64)}
	transferWriteJSON(t, filepath.Join(roundDir, "subject.json"), subject)
	findings := []readsubject.Finding{stopFinding("regression", "newfile.go"), stopFinding("missing-reader", "reader.go")}
	for index := range findings {
		findings[index].ID = critic + ":" + fmt.Sprint(index+1)
	}
	var rigor []any
	for _, finding := range findings {
		rigor = append(rigor, map[string]any{"findingId": finding.ID, "artifact": "metasystem/" + finding.Where, "rigorClass": "bounded", "facts": map[string]any{"local": true, "recoverable": true, "proofBoundaryCrossed": false, "authorityBoundaryCrossed": false, "secretsBoundaryCrossed": false, "irreversibleDataBoundaryCrossed": false, "externalSideEffectBoundaryCrossed": false}, "reopeningTrigger": "The required behavior fails again"})
	}
	transferWriteJSON(t, filepath.Join(roundDir, "return.json"), map[string]any{"schemaVersion": 6, "jobId": critic, "round": 1, "findings": findings, "verdictMaterialCount": 2, "rigor": rigor, "reviewedTree": tree, "verdict": "material=2"})
	if err := os.WriteFile(filepath.Join(roundDir, "return.md"), []byte("VERDICT: REVISE material=2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	readerRecord := map[string]any{"runtime": "local", "jobId": critic, "role": "code-critic", "status": "completed", "round": 1, "goalId": b.id, "engineBuild": "fixture-engine", "effectiveModel": "fixture-read-model", "findingRegister": []any{}, "reviews": "commit:" + commit, "findingRegisterRound": 0, "reviewRoundLimit": 6, "criticRoundsConsumed": 0,
		"operationId": "fixture-critic:" + critic, "goalRevision": b.goalFile(b.id).Claimed.Revision, "capMin": 1,
		"startedAt": b.manager.Now().UTC().Format(time.RFC3339Nano), "endedAt": b.manager.Now().UTC().Format(time.RFC3339Nano),
		"instanceTag": "fixture-critic-" + critic, "pid": int64(20), "pgid": int64(20), "pidStartedAt": int64(400)}
	if runtime.GOOS == "darwin" {
		readerRecord["pidStartedAtExactMicro"] = int64(400_000_001)
	} else {
		readerRecord["pidStartTicks"], readerRecord["bootId"] = int64(400), "fixture-boot"
	}
	transferWriteJSON(t, filepath.Join(agents, "jobs", critic+".json"), readerRecord)
	if outcome, err := dispatchcore.CritiqueRegisterAdvanceWithFacts(b.worktree, critic, critic, transferCritiqueFacts{paths: []string{"metasystem/newfile.go", "metasystem/reader.go"}, tree: tree}); err != nil || outcome != "advanced" {
		t.Fatalf("canonical register: %s %v", outcome, err)
	}
	var admitted map[string]any
	admittedBytes, err := os.ReadFile(filepath.Join(agents, "jobs", critic+".json"))
	if err != nil || json.Unmarshal(admittedBytes, &admitted) != nil {
		t.Fatal(err)
	}
	entries, _ := admitted["findingRegister"].([]any)
	if len(entries) != 2 {
		t.Fatalf("source examination did not enter register: %+v", admitted)
	}
	hook := b.workOwnersHook
	b.workOwnersHook = func(o *intentWorkOwners) {
		hook(o)
		o.criticDeath = dispatchcore.CustodyDeathDependencies{
			Reader: &criticCustodyReader{dead: true}, Processes: identity.FixedProcessTable{},
			MatchesTag: func([]string, string) bool { return true },
			TaggedScan: func(string) census.TaggedProcessCensus { return census.TaggedProcessCensus{} },
		}
	}
	owners := b.workOwners()
	units := owners.work.units
	owners.work.units = func(layout stateroot.Layout) *launch.UnitRunner {
		runner := units(layout)
		runner.ReviewPolicy = nil
		return runner
	}
	owners.policies.Registry = func(string) (config.PolicyRegistry, error) { return config.PolicyRegistry{}, nil }
	owners.lookupEnv = func(key string) (string, bool) { return "0", key == config.EnvName("review.stop") }
	owners.work.inspectRead = func(string, string, string) (branch.BranchReadResult, error) {
		return branch.BranchReadResult{RootJob: critic}, nil
	}
	owners.delivery = &intentDeliveryOwners{branchRead: func([]string) (branch.BranchReadResult, int, error) {
		return branch.BranchReadResult{State: "closed", RootJob: critic}, 0, nil
	}}
	observer := &transferPublicationObserver{Repository: b.repo, t: t, root: b.worktree, critic: critic, goalID: b.id}
	endpoint := owners.dependencies.endpoint
	owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
		e, err := endpoint(root)
		e.Repository = observer
		return e, err
	}
	return transferScenarioFixture{b, owners, runner, run, page, commit, critic, agents, findings, admitted, observer}
}

func TestIntentRequiredStoppedUnitPublishesTransferOnce(t *testing.T) {
	t.Parallel()
	fixture := newTransferScenarioFixture(t, true)
	b, owners, runner := fixture.bed, fixture.owners, fixture.runner
	run, page, commit, critic, agents := fixture.run, fixture.page, fixture.commit, fixture.critic, fixture.agents
	findings, admitted, observer := fixture.findings, fixture.admitted, fixture.observer
	code, result := transferPublic(t, b, owners, "work", "review", b.id, "--work", "stopped")
	if code != 0 || result.Outcome != intentConfirmed || result.Next == nil {
		t.Fatalf("automatic required transfer: %d %+v register=%+v obligations=%+v", code, result, admitted, b.goalFile(b.id).ReviewObligations)
	}
	obligations := b.goalFile(b.id).ReviewObligations
	if len(obligations) != 2 || obligations[0].OriginalEvidence != findings[0] || obligations[1].OriginalEvidence != findings[1] || obligations[0].State != "open" || obligations[1].State != "open" {
		t.Fatalf("canonical transfer not durable: %+v", obligations)
	}
	o := obligations[0]
	after, err := runner.Status(run)
	if err != nil || !after.Rounds[1].Transferred || len(after.Subjects) != 1 || !slices.Equal(after.Subjects[0].TransferredTo, []string{o.TargetUnit}) {
		t.Fatalf("source transfer not retained: %+v %v", after, err)
	}
	var sourceRecord map[string]any
	body, err := os.ReadFile(filepath.Join(agents, "jobs", critic+".json"))
	if err != nil || json.Unmarshal(body, &sourceRecord) != nil {
		t.Fatal(err)
	}
	if sourceRecord["chainCloseReason"] != "transferred" || sourceRecord["closure"] != nil {
		t.Fatalf("source falsely acquired a clean read: %+v", sourceRecord)
	}
	questions, damaged := channel.WalkQuestions(b.stateRoot())
	if len(damaged) != 0 || len(questions) != 2 || questions[0].State == "open" {
		t.Fatalf("matching stop question not closed: %+v %v", questions, damaged)
	}
	generated := filepath.Join(after.Rounds[1].Directory, "stop-dispositions.md")
	code, repeated := transferPublic(t, b, owners, "work", "review", b.id, "--work", "stopped", "--dispositions", generated)
	if code != 0 || repeated.Outcome != intentConfirmed || observer.publications != 1 || len(b.goalFile(b.id).ReviewObligations) != 2 {
		t.Fatalf("repeat duplicated transfer: %d %+v publications=%d", code, repeated, observer.publications)
	}
	questions, damaged = channel.WalkQuestions(b.stateRoot())
	if len(damaged) != 0 || len(questions) != 2 {
		t.Fatalf("repeat duplicated question: %+v %v", questions, damaged)
	}
	progress, err := goalProgress([]string{page}, intentBranchState{Status: branch.Status{Units: []branch.UnitStatus{{Unit: "stopped", Commit: commit, ReadState: "built"}}, ReviewObligations: b.goalFile(b.id).ReviewObligations}})
	if err != nil || !slices.Contains(progress.Unbuilt, o.TargetUnit) || !slices.Contains(progress.Unread, "stopped") {
		t.Fatalf("transfer bypassed required destination/source coverage: %+v %v", progress, err)
	}
	// Execute the generated command using the same real build, admission and
	// proof owners as the source. The external model resolves its inherited row.
	resolved := append([]readsubject.Finding(nil), findings...)
	for index := range resolved {
		resolved[index].ID = ""
		resolved[index].Material = false
		resolved[index].Resolves = findings[index].ID
	}
	b.manager.Supervisor = &stopReadStarter{bed: b, reads: [][]readsubject.Finding{resolved}}
	code, built, _ := stopPublic(t, b, "auto", result.Next.Argv[1:]...)
	if code != 0 {
		t.Fatalf("generated destination command cannot run: %d %+v argv=%v", code, built, result.Next.Argv)
	}
	destinationRun, ok := resultData(t, built)["run"].(string)
	if !ok {
		t.Fatalf("generated command did not build destination: %+v argv=%v", built, result.Next.Argv)
	}
	destination, err := runner.Status(destinationRun)
	if err != nil || destination.Unit != o.TargetUnit || destination.Rounds[0].Stop == nil || destination.Rounds[0].Stop.Decision != "close" {
		t.Fatalf("destination inherited resolution: %+v %v", destination, err)
	}
	plan, err := launch.ReadUnitPlan(filepath.Join(destination.Rounds[0].Directory, "plan.json"))
	if err != nil || slices.Contains(result.Next.Argv, "--check") || plan.Check == nil || plan.Check.Cheap != shellCommand(workArgv) || plan.Check.Audits != "true" || plan.Check.Minutes != 15 || len(plan.Proof) != 1 || plan.Proof[0].Name != "unit-check" {
		t.Fatalf("generated destination lost proof: %+v %v", plan, err)
	}
	for _, step := range destination.Rounds[0].Steps {
		if strings.HasPrefix(step.Name, "build") {
			record, err := b.manager.Store.Read(step.LaunchID)
			if err != nil || record.DeclaredLines != 5 {
				t.Fatalf("generated destination lost source estimate: %+v %v", record, err)
			}
		}
	}
	e, err := b.dependencies().endpoint(b.root())
	if err != nil {
		t.Fatal(err)
	}
	file := b.goalFile(b.id)
	req := goal.VerbRequest{Endpoint: e, Actor: goal.Actor{Machine: file.Claimed.Machine, Lineage: file.Claimed.Lineage}, Now: time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC), Ulid: "01J5X00000000000000000TT01"}
	if done, err := goal.Done(req, b.id, "Completed source requirement"); err != nil || done.Outcome != goal.OutcomeRejected || !strings.Contains(done.Detail, "open review obligation") {
		t.Fatalf("goal completed without destination coverage: %+v %v", done, err)
	}
	coverage := goal.TransferCoverage{TargetUnit: o.TargetUnit, ReadID: "destination-reader", Commit: strings.Repeat("e", 40), Findings: []string{"unrelated:1"}, SourceCommits: []string{commit}}
	req.Ulid = "01J5X00000000000000000TT02"
	if done, err := goal.CompleteTransfers(req, b.id, o.TargetUnit, coverage); err != nil || done.Outcome != goal.OutcomeRejected {
		t.Fatalf("unrelated clean destination discharged debt: %+v %v", done, err)
	}
	repository, readRequest, destinationCritic := transferDestinationReader(t, b, obligations, resolved)
	if started, err := branch.RunBranchRead(readRequest); err != nil || started.State != "dispatched" {
		t.Fatalf("destination critic dispatch: %+v %v", started, err)
	}
	if outcome, err := dispatchcore.CritiqueRegisterAdvanceWithFacts(b.worktree, destinationCritic, destinationCritic, transferCritiqueFacts{paths: []string{repository.unitPath}, tree: repository.tree}); err != nil || outcome != "advanced" {
		t.Fatalf("destination canonical resolution: %s %v", outcome, err)
	}
	if err := dispatchcore.CritiqueChainClose(b.worktree, destinationCritic, false); err != nil {
		t.Fatalf("destination clean close: %v", err)
	}
	readRequest.Collect = true
	collected, err := branch.RunBranchRead(readRequest)
	if err != nil || collected.State != "collected" || collected.TransferCoverage == nil || !slices.Equal(collected.TransferCoverage.Findings, []string{obligations[0].OriginalFinding, obligations[1].OriginalFinding}) {
		t.Fatalf("destination inherited attestation: %+v %v", collected, err)
	}
	// The selected checkout reads the source's mirrored job identity; the
	// destination's published immutable blobs supply the coverage proof.
	transferWriteJSON(t, filepath.Join(b.root(), "artifacts", "agents", "jobs", critic+".json"), sourceRecord)
	owners.delivery.branchState = func(string, string) (intentBranchState, error) {
		return intentBranchState{EndpointTip: repository.base, BranchTip: repository.read, Status: branch.Status{Units: []branch.UnitStatus{{Unit: "stopped", Commit: commit, ReadState: "built"}, {Unit: o.TargetUnit, Commit: repository.unit, ReadState: "read clean"}}, ReviewObligations: b.goalFile(b.id).ReviewObligations}}, nil
	}
	owners.delivery.transferCoverage = func(repo, snapshot, endpoint, goalID, unit, commit string) (goal.TransferCoverage, error) {
		return branch.VerifyTransferCoverageWithReads(repository, repo, snapshot, endpoint, goalID, unit, commit)
	}
	code, selected := transferPublic(t, b, owners, "work", "review", b.id, "--review", critic, "--finding", o.OriginalFinding, "--test", "TestTransferredSourceCovered")
	if code != 0 || selected.Outcome != intentConfirmed {
		t.Fatalf("retained public finding resolution: %d %+v", code, selected)
	}
	partial := b.goalFile(b.id).ReviewObligations
	if partial[0].State != "discharged" || partial[1].State != "open" {
		t.Fatalf("one finding resolution completed other findings: %+v", partial)
	}
	if err := runner.ReviewSubject(destinationRun, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
		return retain(launch.UnitSubject{Round: review.Round.Number, Commit: repository.unit, Published: repository.unit, Tip: repository.unit, StagedTree: repository.tree, DiffDigest: review.DiffDigest})
	}); err != nil {
		t.Fatal(err)
	}
	b.head = repository.unit
	owners.work.inspectRead = func(string, string, string) (branch.BranchReadResult, error) { return collected, nil }
	owners.delivery.branchRead = func(args []string) (branch.BranchReadResult, int, error) {
		request := readRequest
		request.Collect = slices.Contains(args, "--collect")
		result, err := branch.RunBranchRead(request)
		if err != nil {
			return result, 1, err
		}
		return result, 0, nil
	}
	owners.delivery.publishRead = func(string, string, string) (branch.PublishReadResult, error) {
		return branch.PublishReadResult{Attestation: repository.read, State: "published", RemoteTip: repository.read}, nil
	}
	code, completed := transferPublic(t, b, owners, "work", "review", b.id, "--work", o.TargetUnit)
	if code != 0 || completed.Outcome != intentConfirmed {
		t.Fatalf("public clean destination completion: %d %+v", code, completed)
	}
	for index, got := range b.goalFile(b.id).ReviewObligations {
		if got.State != "discharged" || got.CoverageRead != destinationCritic || got.CoverageCommit != repository.unit || got.OriginalEvidence != findings[index] {
			t.Fatalf("completion discarded original evidence: %+v", got)
		}
	}
	code, replayed := transferPublic(t, b, owners, "work", "review", b.id, "--review", critic, "--finding", obligations[1].OriginalFinding, "--test", "TestTransferredSourceCovered")
	if code != 0 || replayed.Outcome != intentConfirmed {
		t.Fatalf("retained finding route after completion: %d %+v", code, replayed)
	}
	code, person := transferPublic(t, b, owners, "work", "review", b.id, "--review", critic, "--finding", obligations[0].OriginalFinding, "--test", "TestTransferredSourceCovered", "--by", "Wido")
	if code != 0 || person.Outcome != intentConfirmed {
		t.Fatalf("person's retained finding resolution: %d %+v", code, person)
	}

}

// transferBranchRepository supplies immutable Git facts while the real
// branch reader and read-commit owner collect and publish their own evidence.
type transferBranchRepository struct{ *branchRawFixture }

func (r transferBranchRepository) IsAncestor(_ string, source, destination string) (bool, error) {
	return source == r.base && destination == r.unit, nil
}
func (r transferBranchRepository) SnapshotFile(_ string, snapshot, path string) ([]byte, error) {
	if snapshot != r.read {
		r.t.Fatalf("unpublished snapshot %s", snapshot)
	}
	data, ok := r.generated[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), data...), nil
}

func transferDestinationReader(t *testing.T, b *workBed, obligations []goal.ReviewObligation, resolved []readsubject.Finding) (transferBranchRepository, branch.BranchReadRequest, string) {
	t.Helper()
	f := &branchRawFixture{t: t, project: b.worktree, installation: b.worktree, base: obligations[0].SourceCommit, unit: branchRawID("e"), tree: branchRawID("f"), read: branchRawID("9"), tip: branchRawID("e"), unitPath: "metasystem/code.go", responses: map[string][]byte{}, used: map[string]int{}}
	if err := os.MkdirAll(filepath.Join(f.project, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	f.add(f.base+"\n", "merge-base", f.base, f.unit)
	f.add(f.unit+" "+f.base+"\n", "rev-list", "--first-parent", "--reverse", "--parents", f.base+".."+f.unit)
	f.add("Goal-Unit: "+b.id+"/"+obligations[0].TargetUnit+"\n", "show", "-s", "--format=%(trailers:only,unfold=true)", f.unit)
	f.add(f.unit+" "+f.base+"\n", "rev-list", "--parents", "-n", "1", f.unit)
	f.responses[branchRawKey("diff-tree", "-r", "-z", "--no-renames", "--full-index", f.unit+"^", f.unit)] = branchRawEntry(f.unitPath)
	f.add("unit patch\n", "diff", "--no-color", "--no-ext-diff", "--no-textconv", "--no-relative", "--binary", "--full-index", "--no-renames", "--src-prefix=a/", "--dst-prefix=b/", "-U3", f.unit+"^", f.unit)
	r := transferBranchRepository{f}
	inputs := f.inputs()
	inputs.Reads = r
	inputs.Effects.Commit = func(_, subject, trailer string, amend bool) error {
		wantSubject := "goal " + b.id + " read " + obligations[0].TargetUnit
		wantTrailer := "Goal-Read: " + b.id + "/" + obligations[0].TargetUnit + " " + f.unit
		if subject != wantSubject || trailer != wantTrailer || amend {
			t.Fatalf("destination commit: %q %q", subject, trailer)
		}
		f.trailer = trailer
		f.readTranscript()
		return nil
	}
	const critic = "transfer-destination-reader"
	request := branch.BranchReadRequest{Repo: f.installation, Remote: "upstream", EndpointTip: f.base, BranchTip: f.unit, GoalID: b.id, UnitCommit: f.unit, Repository: r, InheritedFindings: obligations, CheckClaim: func() error { return nil }, Gate: func(string) (string, error) { return "destination-fast-check", nil }, NewID: func(string) (string, error) { return "destination-read-operation", nil }, Commit: func(req branch.CommitReadRequest) (string, branch.Attestation, error) {
		req.Inputs = inputs
		req.Transport = branchRawTransport{f}
		req.GateRepository = r
		return branch.CommitRead(req)
	}}
	request.Delegate = func(brief, goalID, commit, runtime, model string) (string, error) {
		data, err := os.ReadFile(brief)
		if err != nil || !strings.Contains(string(data), "git diff "+f.base+"^ "+f.unit) {
			t.Fatalf("destination read omitted retained source: %s %v", data, err)
		}
		agents := filepath.Join(f.installation, "artifacts", "agents")
		dir := filepath.Join(agents, critic, "rounds", "1")
		subject, err := f.ReadSubject(f.installation, f.unit)
		if err != nil {
			t.Fatal(err)
		}
		transferWriteJSON(t, filepath.Join(dir, "subject.json"), subject)
		transferWriteJSON(t, filepath.Join(dir, "return.json"), map[string]any{"schemaVersion": 6, "jobId": critic, "round": 1, "findings": resolved, "verdictMaterialCount": 0, "reviewedTree": f.tree, "verdict": "LAND", "rigor": []any{}})
		if err := os.WriteFile(filepath.Join(dir, "return.md"), []byte("VERDICT: LAND\n"), 0600); err != nil {
			t.Fatal(err)
		}
		mirror := t.TempDir()
		transferWriteJSON(t, filepath.Join(mirror, "manifest.json"), map[string]any{"files": map[string]any{"jobs/" + critic + ".json": map[string]any{}}})
		transferWriteJSON(t, filepath.Join(agents, "jobs", critic+".json"), map[string]any{"jobId": critic, "role": "code-critic", "status": "completed", "round": 1, "reviews": "commit:" + f.unit, "goalId": b.id, "engineBuild": "fixture-engine", "effectiveModel": "fixture-read-model", "findingRegister": []any{}, "findingRegisterRound": 0, "reviewRoundLimit": 6, "criticRoundsConsumed": 0, "mirror": map[string]any{"path": mirror}})
		return critic, nil
	}
	return r, request, critic
}

func TestIntentExtraStoppedUnitRetainsCodeAndAsksWorkingAct(t *testing.T) {
	t.Parallel()
	fixture := newTransferScenarioFixture(t, false)
	b, owners := fixture.bed, fixture.owners
	code, result := transferPublic(t, b, owners, "work", "review", b.id, "--work", "stopped")
	if code != 1 || result.Outcome != intentInProgress {
		t.Fatalf("extra stopped unit did not request its acts: %d %+v", code, result)
	}
	questions, damaged := channel.WalkOpenQuestions(b.stateRoot())
	if len(damaged) != 0 || len(questions) != 2 {
		t.Fatalf("unrequired findings have no executable asks: %+v %v", questions, damaged)
	}
	for _, question := range questions {
		if question.UnitStop == nil || !strings.Contains(question.UnitStop.Needs, "work review "+b.id+" --work stopped") || !strings.Contains(question.UnitStop.Needs, "--dispositions") || !slices.Contains(question.UnitStop.AcceptableActs, "work-drop") || !slices.Contains(question.UnitStop.AcceptableActs, "work-revise") {
			t.Fatalf("finding ask has no existing matching act: %+v", question)
		}
		owners.processes.question = channel.ReadQuestion
		code, out, _ := b.run(owners, "question", "show", "channel:"+question.ID)
		if code != 0 || !strings.Contains(strings.Join(strings.Fields(out), " "), question.UnitStop.Needs) {
			t.Fatalf("question show omitted its correction command: %d %q", code, out)
		}
	}
	before, err := fixture.runner.Status(fixture.run)
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(fixture.agents, "jobs", fixture.critic+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var sourceRecord map[string]any
	if err = json.Unmarshal(source, &sourceRecord); err != nil {
		t.Fatal(err)
	}
	transferWriteJSON(t, filepath.Join(b.root(), "artifacts", "agents", "jobs", fixture.critic+".json"), sourceRecord)
	originalDiff, err := os.ReadFile(filepath.Join(before.Rounds[1].Directory, "worktree.diff"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(before.Rounds[1].Directory, "stop-dispositions.md")
	owners.connection.endpointTip = func(string, goal.Endpoint) (string, error) { return "", fmt.Errorf("branch endpoint unavailable") }
	code, dropped := transferPublic(t, b, owners, "work", "review", b.id, "--work", "stopped", "--dispositions", path)
	if code != 1 || dropped.Outcome != intentInProgress || dropped.Summary != "the drop remains pending" || !slices.Equal(dropped.Details, []string{"branch endpoint unavailable"}) {
		t.Fatalf("extra drop silently changed retained work: %d %+v", code, dropped)
	}
	after, err := fixture.runner.Status(fixture.run)
	if err != nil || after.Rounds[1].Transferred || after.Subjects[0].Commit != before.Subjects[0].Commit || len(b.goalFile(b.id).ReviewObligations) != 0 || fixture.observer.publications != 0 {
		t.Fatalf("extra drop changed source/completion: %+v %v", after, err)
	}
	retained, err := os.ReadFile(filepath.Join(after.Rounds[1].Directory, "worktree.diff"))
	if err != nil || !bytes.Equal(retained, originalDiff) {
		t.Fatalf("extra drop changed retained diff: %v", err)
	}
	questions, damaged = channel.WalkOpenQuestions(b.stateRoot())
	if len(damaged) != 0 || len(questions) != 2 {
		t.Fatalf("unsupported drop closed a question: %+v %v", questions, damaged)
	}
	unrelated, err := channel.Ask(channel.AskRequest{RepoRoot: b.root(), Goal: b.id, Kind: "stop", Machine: "machine", Facts: []string{"another unit still needs correction"}, Now: b.manager.Now(), UnitStop: &channel.UnitStopQuestion{Loop: "unit-round", Subject: b.id + "/another/run", Attempt: 2, Finding: "another-reader:1", Review: "another-reader", Needs: "metasystem work revise " + b.id + " --work another --brief FILE --reason TEXT --by NAME", AcceptableActs: []string{"work-revise"}}})
	if err != nil {
		t.Fatal(err)
	}
	var correction strings.Builder
	correction.WriteString("Correct the extra work.\n\n## Decisions on round 2\n\n| Finding | Decision | Evidence |\n| --- | --- | --- |\n")
	for _, finding := range fixture.findings {
		fmt.Fprintf(&correction, "| %s | fixed | %s:12 |\n", finding.ID, finding.Where)
	}
	brief := b.brief("extra-person-correction.md", correction.String())
	b.manager.Supervisor = &stopReadStarter{bed: b, reads: [][]readsubject.Finding{{}}}
	code, corrected := transferPublic(t, b, owners, "work", "revise", b.id, "--work", "stopped", "--brief", brief, "--reason", "Correct the remaining extra findings", "--by", "Wido")
	if code != 0 || corrected.Outcome != intentConfirmed {
		t.Fatalf("printed reasoned correction act cannot run: %d %+v", code, corrected)
	}
	questions, damaged = channel.WalkOpenQuestions(b.stateRoot())
	if len(damaged) != 0 || len(questions) != 1 || questions[0].ID != unrelated.ID {
		t.Fatalf("recorded reasoned correction did not close only its matching asks: %+v %v", questions, damaged)
	}

}
