package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func requiredScopeFixture(t *testing.T, inherited string) (*dropFixture, string) {
	t.Helper()
	f := newDropFixture(t)
	f.connect()
	f.bed.manager.Seat = board.Seat{Machine: filepath.Base(filepath.Dir(t.TempDir())), Installation: f.bed.root()}
	if code, result := f.review(t); code != 1 {
		t.Fatalf("prepare: %d %+v", code, result)
	}
	body, err := os.ReadFile(f.page)
	if err != nil {
		t.Fatal(err)
	}
	if inherited == "" {
		body = append(body, []byte("| stopped | Required behavior | 5 |\n")...)
	}
	if err := os.WriteFile(f.page, body, 0600); err != nil {
		t.Fatal(err)
	}
	if inherited != "" {
		file := f.bed.goalFile(f.bed.id)
		source, target := "stopped", "required-other"
		if inherited == "target" {
			source, target = "retained-source", "stopped"
		}
		finding := stopFinding("regression", "source.go")
		finding.ID = "source-read:1"
		file.ReviewObligations = append(file.ReviewObligations, goal.ReviewObligation{Finding: finding.ID, Chain: "source-read", Artifact: "source.go", Test: "TestInherited", State: "open", SourceUnit: source, TargetUnit: target, OriginalRead: "source-read", OriginalFinding: finding.ID, StopReference: "source-stop", TransferredOnce: true, SourceCommit: f.commit, OriginalEvidence: finding})
		f.bed.addGoal(file)
	}
	legacy := f.bed.goalFile(f.bed.id)
	if len(legacy.ScopeExclusions) != 0 || legacy.ExcludesScope("stopped", "") {
		t.Fatal("older file acquired scope exclusions")
	}
	f.bed.head = f.v
	path := filepath.Join(f.retained(t).Rounds[1].Directory, "stop-dispositions.md")
	return f, path
}

func TestPersonDropUpdatesRequiredCompletion(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"declared", "enrolled-terminal", "inherited-source", "inherited-target", "forged-person", "unreadable-policy", "notification-repair", "red-proof", "lost-goal-response"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			inherited := strings.TrimPrefix(scenario, "inherited-")
			if inherited == scenario {
				inherited = ""
			}
			f, path := requiredScopeFixture(t, inherited)
			if code, result := f.review(t, "--dispositions", path); code == 0 || f.inversions != 0 || result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), "--by") {
				t.Fatalf("agent excluded required work: %d %+v", code, result)
			}
			f.person = true
			if scenario == "enrolled-terminal" {
				enrollGoalSyncTerminal(t, f.bed.root(), "ttys:scope_drop")
				f.owners.dependencies.ownerLineage = func() string { return "" }
			}
			if scenario == "forged-person" {
				f.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
					return humanauthority.Proof{}, errors.New("no enrolled terminal is an ancestor of this caller")
				}
			}
			if scenario == "unreadable-policy" {
				units := f.owners.work.units
				f.owners.work.units = func(layout stateroot.Layout) *launch.UnitRunner {
					runner := units(layout)
					runner.ReviewPolicy = func() (string, error) { return "", errors.New("review.stop is malformed") }
					return runner
				}
			}
			if scenario == "red-proof" {
				f.bed.starter.fail["proof"] = true
			}
			questions, bad := channel.WalkOpenQuestions(f.bed.stateRoot())
			if len(bad) != 0 || len(questions) != 2 {
				t.Fatalf("questions: %v %v", questions, bad)
			}
			questionPath := filepath.Join(f.bed.stateRoot(), "artifacts", "agents", "channel", "questions", questions[0].ID+".json")
			var saved []byte
			if scenario == "notification-repair" {
				saved, _ = os.ReadFile(questionPath)
				if err := os.WriteFile(questionPath, []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "lost-goal-response" {
				endpoint := f.owners.dependencies.endpoint
				lost := &lostDropOutcome{Repository: f.bed.repo}
				f.owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) { e, err := endpoint(root); e.Repository = lost; return e, err }
			}
			args := []string{"--dispositions", path, "--by", "Wido", "--reason", "Remove the required behavior"}
			code, stdout, stderr := f.bed.run(f.owners, append([]string{"work", "review", f.bed.id, "--work", "stopped", "--json"}, args...)...)
			var result intentResult
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("%v: %s / %s", err, stdout, stderr)
			}
			if scenario == "forged-person" || scenario == "red-proof" {
				if code == 0 || len(f.bed.goalFile(f.bed.id).ScopeExclusions) != 0 {
					t.Fatalf("failed act excluded scope: %d %+v", code, result)
				}
				return
			}
			if scenario == "lost-goal-response" {
				if code == 0 || result.Next == nil || !slices.Contains(result.Next.Argv, "--recover") {
					t.Fatalf("lost outcome: %d %+v", code, result)
				}
				if c, r := transferPublic(t, f.bed, f.owners, "goal", "sync", "--recover"); c != 0 {
					t.Fatalf("recover: %d %+v", c, r)
				}
				code, result = f.review(t, args...)
			}
			if scenario == "notification-repair" {
				if code == 0 || !strings.Contains(result.Summary, "repair remains pending") || len(f.bed.goalFile(f.bed.id).ScopeExclusions) != 1 {
					t.Fatalf("notification vetoed effect: %d %+v", code, result)
				}
				if err := os.WriteFile(questionPath, saved, 0600); err != nil {
					t.Fatal(err)
				}
				code, result = f.review(t, args...)
			}
			if code != 0 || f.inversions != 1 || f.checks != 1 || f.commits != 1 {
				t.Fatalf("person drop: %d %+v", code, result)
			}
			if !strings.Contains(stderr, "Impact:") || !strings.Contains(stderr, "goal scope restore") {
				t.Fatalf("prior impact not printed: %s", stderr)
			}
			file := f.bed.goalFile(f.bed.id)
			if len(file.ScopeExclusions) != 1 || !file.ExcludesScope("stopped", "") || file.ExcludesScope("required-other", "") || file.ScopeExclusions[0].Actor != "Wido" || file.ScopeExclusions[0].Authority == "" {
				t.Fatalf("scope outcome: %+v", file.ScopeExclusions)
			}
			if scenario == "enrolled-terminal" {
				last := file.History[len(file.History)-1]
				claim := file.Claimed
				if last.Actor != "human:Wido" || last.Displaced != claim.Machine+"+"+claim.Lineage+"@"+claim.At {
					t.Fatalf("foreign person act lost its claim displacement: %+v", last)
				}
			}
			qs, damaged := channel.WalkOpenQuestions(f.bed.stateRoot())
			if len(qs) != 0 || len(damaged) != 0 {
				t.Fatalf("successful drop asks remain: %v %v", qs, damaged)
			}

		})
	}
}

type scopeReadRepository struct{ *dropFixture }

func (r scopeReadRepository) Range(repo, endpoint, tip, goalID string) ([]branch.Commit, error) {
	return branch.ValidateRangeWithGit(repo, endpoint, tip, goalID, r.raw)
}
func (r scopeReadRepository) Subject(string, string) (branch.AttestationSubject, error) {
	return branch.AttestationSubject{}, fmt.Errorf("subject is unavailable in this checkout's object database")
}
func (r scopeReadRepository) CommonDir(string) (string, error)               { return r.scratch, nil }
func (r scopeReadRepository) Entries(string, string) ([]branch.Entry, error) { return nil, nil }
func (r scopeReadRepository) Detached(string, string) (string, func() error, error) {
	return r.scratch, func() error { return nil }, nil
}

// scopeCompletionBed keeps real branch objects and ledger snapshots; only
// proof execution and final delivery effects use the existing owner fixtures.
func scopeCompletionBed(t *testing.T, f *dropFixture, file *goal.GoalFile) (*deliveryBed, func(), func(*goal.GoalFile)) {
	t.Helper()
	bed, landing, _ := plainLaneBedWith(t, true, "reader-record")
	root := bed.root()
	git := func(args ...string) string { return goalSyncMutationGit(t, root, args...) }
	git("init", "-q", "--initial-branch=main")
	git("config", "user.name", "Scope reader")
	git("config", "user.email", "scope@example.invalid")
	git("config", "remote.origin.url", root)
	git("config", "goal.human.Wido", "Wido <wido@example.invalid>")
	rootPath := filepath.Join(root, "plans", "goals", "backlog.md")
	rootBytes, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	rootRecord, problems := goal.ParseRoot(rootBytes)
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	rootRecord.SyncMode = goal.SyncRemote
	bed.writeFile(rootPath, string(goal.RenderRoot(rootRecord)))
	commit := func(parent, message string) string {
		tree := git("write-tree")
		args := []string{"commit-tree", tree, "-m", message}
		if parent != "" {
			args = append(args, "-p", parent)
		}
		return git(args...)
	}
	design, err := os.ReadFile(f.page)
	if err != nil {
		t.Fatal(err)
	}
	bed.writeFile(filepath.Join(root, "plans", "designs", "landing-work.md"), string(design))
	bed.writeFile(filepath.Join(root, "metasystem", "unit.go"), "base\n")
	bed.writeFile(filepath.Join(root, "metasystem", "memory", "receipts.log"), "")
	git("add", "plans/goals", "metasystem/unit.go", "metasystem/memory/receipts.log")
	base := commit("", "scope reader base")
	git("update-ref", "refs/heads/main", base)
	bed.writeFile(filepath.Join(root, "metasystem", "unit.go"), "required behavior\n")
	git("add", "metasystem/unit.go")
	original := commit(base, "required build\n\nGoal-Unit: "+f.bed.id+"/stopped")
	bed.writeFile(filepath.Join(root, "metasystem", "other.go"), "independent behavior\n")
	git("add", "metasystem/other.go")
	independent := commit(original, "independent build\n\nGoal-Unit: "+f.bed.id+"/required-other")
	bed.writeFile(filepath.Join(root, "metasystem", "unit.go"), "base\n")
	git("add", "metasystem/unit.go")
	inverse := commit(independent, "drop required behavior\n\n"+f.trailer)
	git("update-ref", "refs/heads/goal/"+f.bed.id, inverse)
	// Keep ledger publications on main, beside the goal's immutable builds.
	git("read-tree", base)
	main := base
	publishScope := func(current *goal.GoalFile) {
		t.Helper()
		copy, problems := goal.ParseFile(goal.RenderFile(current))
		if len(problems) != 0 {
			t.Fatal(problems)
		}
		copy.UnitDrops[0].Covered, copy.UnitDrops[0].Commit = []string{original}, inverse
		copy.UnitDrops[0].Tree = git("rev-parse", inverse+"^{tree}")
		copy.ScopeExclusions[0].Result = inverse
		bed.addGoal(copy)
		git("read-tree", main)
		git("add", "plans/goals/"+f.bed.id+".md")
		main = commit(main, "publish current scope")
		git("update-ref", "refs/heads/main", main)
		git("update-ref", goal.AcceptedRef, main)
	}
	publishScope(file)
	bed.owners.branchState = productionIntentBranchState
	bed.owners.laneRoot = func(string, time.Time) (string, bool, error) { return "", false, nil }
	var candidate string
	bed.owners.landCandidate = func(args []string) (goalBranchLandPrepOutcome, int, error) {
		state, err := productionIntentBranchState(root, f.bed.id)
		if err != nil {
			return goalBranchLandPrepOutcome{}, 1, err
		}
		current := bed.goalFile(f.bed.id)
		result, err := branch.PrepareLanding(branch.LandRequest{Repo: root, Remote: "origin", EndpointTip: state.EndpointTip, BranchTip: state.BranchTip, GoalID: f.bed.id, Last: true, LandingReady: true, GoalPage: string(goal.RenderFile(current)), ApprovedBy: "human:Wido", Seat: "scope-reader", CandidateOnly: true, CheckClaim: func() error { return nil }})
		candidate = result.Candidate
		if err != nil {
			return goalBranchLandPrepOutcome{}, 1, err
		}
		if got := git("show", candidate+":metasystem/unit.go"); got != "base" {
			t.Fatalf("landing candidate retained excluded code: %q", got)
		}
		if got := git("show", candidate+":metasystem/other.go"); got != "independent behavior" {
			t.Fatalf("landing candidate lost independent code: %q", got)
		}
		return goalBranchLandPrepOutcome{Result: result}, 0, nil
	}
	bed.owners.landPrep = func(args []string) (goalBranchLandPrepOutcome, int, error) {
		var receipt struct{ Tree string }
		data, err := os.ReadFile(flagValue(args, "--test-receipt"))
		if err != nil || json.Unmarshal(data, &receipt) != nil || receipt.Tree != candidate {
			t.Fatalf("landing proof: %s %v", data, err)
		}
		if err := os.MkdirAll(flagValue(args, "--out"), 0700); err != nil {
			t.Fatal(err)
		}
		return goalBranchLandPrepOutcome{Result: branch.LandResult{Landing: "land1", Candidate: candidate}}, 0, nil
	}
	readIndependent := func() {
		t.Helper()
		if err := os.Remove(filepath.Join(root, "metasystem", "other.go")); err != nil {
			t.Fatal(err)
		}
		git("checkout", "-q", "goal/"+f.bed.id)
		digest, err := branch.UnitDigest(root, independent)
		if err != nil {
			t.Fatal(err)
		}
		record := "metasystem/records/misc/independent-read.md"
		bed.writeFile(filepath.Join(root, record), "Read "+independent+" with digest "+digest+" and found it clean.\n")
		read, _, err := branch.CommitRead(branch.CommitReadRequest{Repo: root, Remote: "origin", EndpointTip: main, GoalID: f.bed.id, Unit: "required-other", OpID: "scope-independent-read", ReaderRecord: record, GateRunID: "scope-independent-proof", GateTree: git("rev-parse", independent+"^{tree}"), CheckClaim: func() error { return nil }})
		if err != nil {
			t.Fatal(err)
		}
		git("update-ref", "refs/heads/goal/"+f.bed.id, read)
		// The delivery fixture uses this only for its final sweep effect.
		landing.status.EndpointTip = main
	}
	return bed, readIndependent, publishScope
}

func TestPersonScopeCompletionReadersAndRestore(t *testing.T) {
	t.Parallel()
	f, path := requiredScopeFixture(t, "target")
	f.person = true
	if code, result := f.review(t, "--dispositions", path, "--by", "Wido", "--reason", "Drop required scope"); code != 0 {
		t.Fatalf("drop: %d %+v", code, result)
	}
	file, problems := goal.ParseFile(goal.RenderFile(f.bed.goalFile(f.bed.id)))
	if len(problems) != 0 || len(file.ScopeExclusions) != 1 {
		t.Fatalf("fresh reload: %v %+v", problems, file)
	}
	bed, readIndependent, publishScope := scopeCompletionBed(t, f, file)
	state, err := bed.owners.branchState(bed.root(), f.bed.id)
	if err != nil {
		t.Fatal(err)
	}
	status := state.Status
	if status.Prefix != 1 || status.Units[0].ReadState != "dropped" || status.Units[0].PriorReadState != "built" || status.Units[1].ReadState != "built" {
		t.Fatalf("fresh scope status: %+v", status)
	}
	secondDesign := filepath.Join(t.TempDir(), "second.md")
	bed.writeFile(secondDesign, "## Units\n\n| Unit | Lines |\n| --- | ---: |\n| unrelated-design | 5 |\n")
	progress, err := goalProgress([]string{f.page, secondDesign}, state)
	if err != nil || progress.Declared != 1 || slices.Contains(progress.Unbuilt, "unrelated-design") || slices.Contains(progress.Unbuilt, "stopped") || slices.Contains(progress.Unread, "stopped") || !slices.Contains(progress.Unread, "required-other") {
		t.Fatalf("progress: %+v %v", progress, err)
	}
	exact, liveness, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || liveness != identity.Alive {
		t.Fatalf("self identity: %s %v", liveness, err)
	}
	holder := t.TempDir()
	if _, err := lease.AnnounceWithPair(holder, "scope-reader", int64(os.Getpid()), exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "fixture", "fake", f.bed.lineage); err != nil {
		t.Fatal(err)
	}
	raw := &goalBranchRawDependencies{GoalRepository: f.bed.repo, Config: func(_ string, key string) (string, error) {
		switch key {
		case "goal.sync-branch":
			return "refs/heads/main", nil
		case "goal.sync-remote":
			return "local", nil
		case "metasystem.goal.machine":
			return "mac-cli", nil
		}
		return "", nil
	}, EndpointTip: func(string, goal.Endpoint) (string, error) { return f.base, nil }, OriginTip: func(string, goal.Endpoint, string) (string, bool, error) { return f.bed.head, true, nil }, ResolveCommit: func(_, commit string) (string, error) { return commit, nil }, HolderRoot: func(string) string { return holder }, Linked: func(string) bool { return true }, HeadRef: func(string) ([]byte, error) { return []byte("refs/heads/main"), nil }, ReadRepository: scopeReadRepository{f}, ReadInputs: &branch.ReadCommitInputs{}, Transport: dropTransport{}}
	f.owners.delivery.branchRead = func(args []string) (branch.BranchReadResult, int, error) {
		return goalBranchReadRun(args, goalBranchReadDependencies{Raw: raw})
	}
	if code, result := transferPublic(t, f.bed, f.owners, "work", "review", "--commit", f.commit, "--goal", f.bed.id); code != 0 || !strings.Contains(result.Summary, "dropped") {
		t.Fatalf("read admission: %d %+v", code, result)
	}
	if code, result := transferPublic(t, f.bed, f.owners, "work", "review", "--commit", f.v, "--goal", f.bed.id); code == 0 {
		t.Fatalf("independent unread work waived: %d %+v", code, result)
	}
	for _, independent := range []bool{false, true} {
		bed := newIntentBed(t, false, func(current *goal.GoalFile) {
			current.UnitDrops, current.ScopeExclusions, current.ReviewObligations = file.UnitDrops, file.ScopeExclusions, slices.Clone(file.ReviewObligations)
			if independent {
				current.ReviewObligations = append(current.ReviewObligations, goal.ReviewObligation{Finding: "independent", Chain: "independent-read", Artifact: "other.go", Test: "TestOther", State: "open"})
			}
		})
		_, reader := enrollGoalSyncTerminal(t, bed.root(), "ttys:scope_done")
		bed.facts.reader = &reader
		owners := bed.owners()
		owners.work.git = func(string, ...string) ([]byte, error) { return []byte("worktree " + bed.root() + "\n"), nil }
		code, result := bed.runJSON(owners, "goal", "done", "standing-validation", "--reason", "Required scope was explicitly excluded")
		if (code == 0) == independent {
			t.Fatalf("goal done independent=%t: %d %+v", independent, code, result)
		}
	}
	if code, result := bed.do("work", "land", f.bed.id); code == 0 || !strings.Contains(result.Summary, "required-other") {
		t.Fatalf("dependent work landed: %d %+v", code, result)
	}
	readIndependent()
	if code, result := bed.do("work", "land", f.bed.id); code != 0 {
		t.Fatalf("completed scope did not land: %d %+v", code, result)
	}
	prove := f.owners.prove
	f.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("an agent process is an ancestor of this shell")
	}
	if code, result := transferPublic(t, f.bed, f.owners, "goal", "scope", "restore", f.bed.id, "stopped"); code == 0 {
		t.Fatalf("agent restored scope: %d %+v", code, result)
	}
	f.owners.prove = prove
	if code, result := transferPublic(t, f.bed, f.owners, "goal", "scope", "restore", f.bed.id, "stopped", "--by", "Wido"); code != 0 {
		t.Fatalf("person restore: %d %+v", code, result)
	}
	restored := f.bed.goalFile(f.bed.id)
	if restored.ExcludesScope("stopped", "") || restored.ScopeExclusions[0].RestoredBy != "Wido" || restored.ScopeExclusions[0].RestoredAt == "" {
		t.Fatalf("restore not retained: %+v", restored.ScopeExclusions)
	}
	publishScope(restored)
	state, err = bed.owners.branchState(bed.root(), f.bed.id)
	if err != nil {
		t.Fatal(err)
	}
	status = state.Status
	if status.Units[0].ReadState != "built" || status.Prefix != 0 {
		t.Fatalf("restore kept completion waiver: %+v", status)
	}
	freshStatus, err := branch.InspectStatus(bed.root(), state.EndpointTip, state.BranchTip, f.bed.id)
	if err != nil || freshStatus.Prefix != 0 || freshStatus.Units[0].ReadState != "built" || freshStatus.Units[0].Drop == nil || freshStatus.Units[0].PriorReadState != "built" {
		t.Fatalf("fresh branch status kept restored scope waived or lost its drop: %+v %v", freshStatus, err)
	}
	_, reader := enrollGoalSyncTerminal(t, f.bed.root(), "ttys:scope_restore_done")
	f.bed.facts.reader = &reader
	if code, result := transferPublic(t, f.bed, f.owners, "goal", "done", f.bed.id, "--reason", "Required behavior finished"); code == 0 || !strings.Contains(result.Summary, "open review obligation") {
		t.Fatalf("restored obligation hidden: %d %+v", code, result)
	}
}
