package main

import (
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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

type treeProber struct{ dead bool }

func (p *treeProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if p.dead {
		return identity.Exact{}, identity.Dead, nil
	}
	return workProber{}.Probe(pid)
}

func treeBuild(t *testing.T, b *workBed, name string, read bool) (int, intentResult) {
	t.Helper()
	text := "Build the declared requirement.\n"
	if read {
		text = "Read each round: yes\n" + text
	}
	brief := b.brief(name+".md", text)
	code, result, _ := b.work(append([]string{"work", "build", b.id, "--work", name, "--brief", brief, "--lines", "5"}, workCheck...)...)
	return code, result
}

func TestIntentTreeGhostChildCanBeStoppedAndReleasesTree(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	var run, child string
	interrupted := new(int)
	b.workOwnersHook = func(owners *intentWorkOwners) {
		units := owners.units
		owners.units = func(layout stateroot.Layout) *launch.UnitRunner {
			runner := units(layout)
			runner.AfterWrite = func(record launch.UnitRunRecord) error {
				if len(record.Rounds) > 0 && record.Rounds[0].Steps[0].State == launch.StepStarting {
					run, child = record.ID, record.Rounds[0].Steps[0].LaunchID
					panic(interrupted)
				}
				return nil
			}
			return runner
		}
	}
	func() {
		defer func() {
			if caught := recover(); caught != interrupted {
				t.Fatalf("command did not die at the saved launch id: %v", caught)
			}
		}()
		treeBuild(t, b, "u", false)
	}()
	b.workOwnersHook = nil
	if _, err := b.manager.Store.Read(child); !os.IsNotExist(err) || len(b.starter.launched()) != 0 {
		t.Fatalf("interruption created a launch: %v %v", err, b.starter.launched())
	}
	code, waiting := treeBuild(t, b, "v", false)
	act := "metasystem work stop run:" + run
	if code != 3 || !strings.Contains(waiting.Summary, act) {
		t.Errorf("never-started owner has no executable recovery: %d %+v", code, waiting)
	}
	owners := b.workOwners()
	owners.processes = defaultProcessIntentOwners()
	owners.processes.process.repositoryTop = fakeTop(b.root())
	now, err := b.commandNow(b.root())
	if err != nil {
		t.Fatal(err)
	}
	owners.prove = enrolledPersonProver(t, b.root(), now)
	code, stopped := b.runJSON(owners, strings.Fields(act)[1:]...)
	if code != 0 {
		t.Fatalf("person could not stop the never-started child: %d %+v", code, stopped)
	}
	current, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	paths, globErr := filepath.Glob(filepath.Join(b.unitRoot, ".trees", "*.json"))
	if err != nil || globErr != nil || current.State != "cancelled" || len(paths) != 0 {
		t.Fatalf("stop did not release the tree: %+v %v %v %v", current, paths, err, globErr)
	}
	code, next := treeBuild(t, b, "v", false)
	if code != 0 || !slices.Equal(b.starter.launched(), []string{"build", "proof"}) {
		t.Fatalf("next writer could not proceed: %d %+v %v", code, next, b.starter.launched())
	}
	code, resumed, _ := b.work("work", "wait", "run:"+run)
	if code != 1 || !strings.Contains(resultWords(resumed), "UNIT_CANCELLED") || len(b.starter.launched()) != 2 {
		t.Fatalf("cancelled owner resumed: %d %+v", code, resumed)
	}
}

func TestIntentTreeReservationWaitsAcrossCommandsAndExactCancellation(t *testing.T) {
	t.Parallel()
	for _, terminalRecord := range []bool{false, true} {
		name := "running-child"
		if terminalRecord {
			name = "terminal-record-live-child"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newWorkBedWith(t, func(file *goal.GoalFile) {
				workApprovedBox(file)
				file.Budget.ReservedJobMinutesLimit = 600
				file.NormApproval = &goal.GoalNormApprovalClaim{ApprovedRef: "R-fixture", Minutes: 600, ReviewRounds: 2, GoalRevision: file.Revision}
				file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
			})
			// Manual capture reads an actual source HEAD before it asks for tree custody.
			connectionGit(t, b.root(), "init", "-q", "-b", "main")
			object := func(kind, contents string) string {
				t.Helper()
				command := exec.Command("git", "-C", b.root(), "hash-object", "-t", kind, "-w", "--stdin")
				command.Stdin = strings.NewReader(contents)
				out, err := command.CombinedOutput()
				if err != nil {
					t.Fatalf("source %s object: %s %v", kind, out, err)
				}
				return strings.TrimSpace(string(out))
			}
			tree := object("tree", "")
			head := object("commit", "tree "+tree+"\nauthor Fixture <fixture@example.invalid> 0 +0000\ncommitter Fixture <fixture@example.invalid> 0 +0000\n\nfixture source\n")
			connectionGit(t, b.root(), "update-ref", "HEAD", head)
			b.starter.hold = "build"
			prober := &treeProber{}
			b.manager.Prober = prober
			code, started := treeBuild(t, b, "u", false)
			if code != 3 {
				t.Fatalf("start: %d %+v", code, started)
			}
			run := resultData(t, started)["run"].(string)
			runner := &launch.UnitRunner{Manager: b.manager, Root: b.unitRoot}
			record, err := runner.Status(run)
			if err != nil {
				t.Fatal(err)
			}
			child := record.Rounds[0].Steps[0].LaunchID
			code, own, _ := b.work("work", "review", "run:"+run)
			if code != 1 || !strings.Contains(own.Summary, "still running") {
				t.Fatalf("owner waited on its own reservation: %d %+v", code, own)
			}

			// A terminal record cannot release custody while its exact child is live.
			if terminalRecord {
				b.manager.Store.Update(child, func(r *launch.Record) error {
					r.State = launch.Cancelled
					r.FinishedAt = b.manager.Now().UTC().Format(time.RFC3339Nano)
					return nil
				})
			}
			assertWait := func(result intentResult, code int) {
				t.Helper()
				if code != 3 || result.Outcome != intentInProgress || result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "work", "wait", "run:" + run}) || !strings.Contains(result.Summary, run) {
					t.Fatalf("no executable wait for the owner: %d %+v", code, result)
				}
			}
			code, blocked := treeBuild(t, b, "v", false)
			assertWait(blocked, code)
			owners := b.workOwners()
			mutations := 0
			owners.connection.endpointTip = func(string, goal.Endpoint) (string, error) { return strings.Repeat("a", 40), nil }
			owners.connection.section = func(_ string, body func(func(func() error) error) error) error {
				return body(func(f func() error) error { return f() })
			}
			owners.connection.rebase = func(branch.RebaseRequest) (branch.RebaseResult, error) {
				mutations++
				return branch.RebaseResult{State: "held"}, nil
			}
			code, blocked = b.runJSON(owners, "work", "rebase", b.id)
			assertWait(blocked, code)
			code, blocked = b.runJSON(owners, "work", "review", b.id, "--work", "manual", "--changes", "--brief", b.brief("manual.md", "Submit a separate change.\n"))
			assertWait(blocked, code)
			if mutations != 0 || len(b.starter.launched()) != 1 {
				t.Fatal("a waiting writer started work")
			}
			paths, _ := filepath.Glob(filepath.Join(b.unitRoot, ".trees", "*.json"))
			data, err := os.ReadFile(paths[0])
			var reservation struct {
				Worktree, Run, Phase string
				Round                int
				Children             []string
			}
			if err != nil || json.Unmarshal(data, &reservation) != nil || !sameDirectory(reservation.Worktree, b.worktree) || reservation.Run != run || reservation.Round != 1 || !slices.Contains(reservation.Children, child) {
				t.Fatalf("incomplete durable reservation: %s %v", data, err)
			}
			// An already terminal launch still waits until its exact custody ends.
			owners.processes.launches = func() *launch.Manager { return b.manager }
			code, terminal := b.runJSON(owners, "work", "stop", "j1:"+child)
			if terminalRecord && (code != 0 || terminal.Outcome != intentUnchanged) || !terminalRecord && code != 1 {
				t.Fatalf("terminal cancellation: %d %+v", code, terminal)
			}
			code, blocked = treeBuild(t, b, "v", false)
			assertWait(blocked, code)
			now, err := b.commandNow(b.root())
			if err != nil {
				t.Fatal(err)
			}
			owners.prove = enrolledPersonProver(t, b.root(), now)
			code, pending := b.runJSON(owners, "work", "stop", "run:"+run)
			if code != 1 {
				t.Fatalf("run released live custody: %d %+v", code, pending)
			}
			code, blocked = treeBuild(t, b, "v", false)
			assertWait(blocked, code)
			prober.dead = true
			code, cancelled := b.runJSON(owners, "work", "stop", "j1:"+child)
			if code != 0 {
				t.Fatalf("exact cancellation: %d %+v", code, cancelled)
			}
			// A stopped run never starts another step when collected.
			code, collected, _ := b.work("work", "wait", "run:"+run)
			if code != 1 || len(b.starter.launched()) != 1 {
				t.Fatalf("cancelled collection started more work: %d %+v", code, collected)
			}
			code, cancelled = b.runJSON(owners, "work", "stop", "run:"+run)
			if code != 0 {
				t.Fatalf("run cancellation: %d %+v", code, cancelled)
			}
			b.starter.hold = ""
			code, next := treeBuild(t, b, "v", false)
			if code != 0 || len(b.starter.launched()) != 3 {
				t.Fatalf("quiescent cancellation did not release: %d %+v %v", code, next, b.starter.launched())
			}
			current, _ := runner.Status(run)
			if current.State != "cancelled" {
				t.Fatalf("old owner may resume writing: %+v", current)
			}
			code, resumed, _ := b.work("work", "wait", "run:"+run)
			if code == 0 || len(b.starter.launched()) != 3 {
				t.Fatalf("cancelled owner resumed: %d %+v", code, resumed)
			}
		})
	}

}

type boundaryGit struct {
	bed  *workBed
	tree *string
}

func (g boundaryGit) Run(dir string, env []string, args ...string) ([]byte, error) {
	if slices.Equal(args, []string{"diff", "--cached", "--raw", "-z", "--no-abbrev", "HEAD", "--", "."}) {
		return []byte(*g.tree), nil
	}
	return workGit{g.bed}.Run(dir, env, args...)
}

type boundaryStarter struct {
	bed  *workBed
	read func()
}

func (s boundaryStarter) StartSupervisor(id, dir string) (identity.Ref, error) {
	record, _ := s.bed.manager.Store.Read(id)
	if record.Kind == "read" && s.read != nil {
		s.read()
	}
	return s.bed.starter.StartSupervisor(id, dir)
}

func boundaryBed(t *testing.T) (*workBed, *string) {
	t.Helper()
	b := newWorkBed(t)
	tree := new(string)
	b.workOwnersHook = func(owners *intentWorkOwners) {
		units := owners.units
		owners.units = func(layout stateroot.Layout) *launch.UnitRunner {
			r := units(layout)
			r.Git = boundaryGit{b, tree}
			return r
		}
	}
	return b, tree
}

func TestIntentTreeBoundaryIsFreshAfterYield(t *testing.T) {
	t.Parallel()
	b, tree := boundaryBed(t)
	b.starter.hold = "proof"
	code, started := treeBuild(t, b, "fresh", true)
	if code != 3 {
		t.Fatalf("start: %d %+v", code, started)
	}
	run := resultData(t, started)["run"].(string)
	runner := &launch.UnitRunner{Root: b.unitRoot}
	before, err := runner.Status(run)
	if err != nil {
		t.Fatal(err)
	}
	step := before.Rounds[0].Steps[1]
	*tree = ":100644 100644 " + strings.Repeat("a", 40) + " " + strings.Repeat("b", 40) + " M\x00a.go\x00"
	b.manager.Store.Update(step.LaunchID, func(r *launch.Record) error {
		zero := 0
		r.State, r.ExitCode = launch.Completed, &zero
		r.FinishedAt = b.manager.Now().UTC().Format(time.RFC3339Nano)
		return nil
	})
	code, result, _ := b.work("work", "wait", "run:"+run)
	current, err := runner.Status(run)
	if err != nil || current.Rounds[0].Outcome != "proof-wrote" || current.Rounds[0].Cause != "environment" || current.Rounds[0].Steps[1].State != launch.StepFailed || len(b.starter.launched()) != 2 {
		t.Fatalf("resume used stale proof boundary: %d %+v %+v %v", code, result, current, err)
	}
}

func TestIntentTreeMovementDuringReadCannotPublishClean(t *testing.T) {
	t.Parallel()
	b, tree := boundaryBed(t)
	b.manager.Supervisor = boundaryStarter{b, func() {
		*tree = ":100644 100644 " + strings.Repeat("a", 40) + " " + strings.Repeat("b", 40) + " M\x00a.go\x00"
	}}
	code, built := treeBuild(t, b, "readmove", true)
	if code != 1 {
		t.Fatalf("build: %d %+v", code, built)
	}
	run := resultData(t, built)["run"].(string)
	current, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if err != nil || current.Rounds[0].Outcome != "proof-wrote" || current.Rounds[0].Cause != "environment" || current.Rounds[0].Material != -1 || len(current.Rounds[0].Reads) != 0 {
		t.Fatalf("read of changed bytes counted clean: %+v %v", current, err)
	}
	code, review, _ := b.work("work", "review", "run:"+run)
	if code == 0 || !strings.Contains(resultWords(review), "environment") || len(current.Subjects) != 0 {
		t.Fatalf("changed read was published: %d %+v", code, review)
	}
}

func TestIntentTreeStoppedChildKeepsOwnerForPersonRevision(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	b.starter.hold = "build"
	prober := &treeProber{}
	b.manager.Prober = prober
	code, started := treeBuild(t, b, "u", false)
	if code != 3 {
		t.Fatalf("start: %d %+v", code, started)
	}
	run := resultData(t, started)["run"].(string)
	runner := &launch.UnitRunner{Manager: b.manager, Root: b.unitRoot}
	record, err := runner.Status(run)
	if err != nil {
		t.Fatal(err)
	}
	owners := b.workOwners()
	owners.processes = defaultProcessIntentOwners()
	owners.processes.process.repositoryTop = fakeTop(b.root())
	owners.processes.launches = func() *launch.Manager { return b.manager }
	now, err := b.commandNow(b.root())
	if err != nil {
		t.Fatal(err)
	}
	owners.prove = enrolledPersonProver(t, b.root(), now)
	prober.dead = true
	code, stopped := b.runJSON(owners, "work", "stop", "j1:"+record.Rounds[0].Steps[0].LaunchID)
	if code != 0 {
		t.Fatalf("stop step: %d %+v", code, stopped)
	}
	b.work("work", "wait", "run:"+run)
	code, waiting := treeBuild(t, b, "v", false)
	if code != 3 {
		t.Fatalf("competing build must wait: %d %+v", code, waiting)
	}
	after, err := runner.Status(run)
	if err != nil || after.State == "cancelled" {
		t.Fatalf("gate cancelled owner: %+v %v", after, err)
	}
	b.starter.hold = ""
	code, revised := b.runJSON(owners, "work", "revise", b.id, "--work", "u", "--after", "1", "--brief", b.brief("repair.md", "Repair the stopped builder.\n"), "--reason", "The hung builder was stopped; continue its work")
	after, err = runner.Status(run)
	if code != 0 || err != nil || len(after.Rounds) != 2 || len(b.starter.launched()) != 3 {
		t.Fatalf("person revision lost ownership: %d %+v %+v %v", code, revised, after, err)
	}
}

func TestIntentTreeEndedOwnerCanBeCancelledOnlyByPerson(t *testing.T) {
	t.Parallel()
	for _, empty := range []bool{false, true} {
		t.Run(fmt.Sprintf("no-children=%t", empty), func(t *testing.T) {
			t.Parallel()
			b := newWorkBedWith(t, func(file *goal.GoalFile) {
				workApprovedBox(file)
				file.Budget.ReservedJobMinutesLimit = 600
				file.NormApproval = &goal.GoalNormApprovalClaim{ApprovedRef: "R-fixture", Minutes: 600, ReviewRounds: 2, GoalRevision: file.Revision}
				file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
			})
			b.starter.fail["build"] = true
			code, ended := treeBuild(t, b, "u", false)
			if code != 1 {
				t.Fatalf("ended build: %d %+v", code, ended)
			}
			run := resultData(t, ended)["run"].(string)
			runner := &launch.UnitRunner{Manager: b.manager, Root: b.unitRoot}
			if empty {
				// An interrupted admission has durable ownership but no round or launch.
				record, err := runner.Status(run)
				if err != nil {
					t.Fatal(err)
				}
				record.Rounds = nil
				data, _ := json.Marshal(record)
				if err := os.WriteFile(filepath.Join(b.unitRoot, run, "run.json"), data, 0600); err != nil {
					t.Fatal(err)
				}
				paths, _ := filepath.Glob(filepath.Join(b.unitRoot, ".trees", "*.json"))
				data, _ = os.ReadFile(paths[0])
				var owner map[string]any
				if err := json.Unmarshal(data, &owner); err != nil {
					t.Fatal(err)
				}
				owner["children"] = []string{}
				data, _ = json.Marshal(owner)
				if err := os.WriteFile(paths[0], data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			code, waiting := treeBuild(t, b, "v", false)
			act := "metasystem work stop run:" + run
			if code != 3 || !strings.Contains(waiting.Summary, act) {
				t.Fatalf("ended owner has no executable recovery: %d %+v", code, waiting)
			}
			owners := b.workOwners()
			owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent invocation")
			}
			code, refused := b.runJSON(owners, strings.Fields(act)[1:]...)
			if code == 0 || refused.Outcome != intentRefused {
				t.Fatalf("agent cancelled another owner: %d %+v", code, refused)
			}
			current, _ := runner.Status(run)
			if current.State == "cancelled" {
				t.Fatal("agent changed owner's record")
			}
			now, err := b.commandNow(b.root())
			if err != nil {
				t.Fatal(err)
			}
			owners.prove = enrolledPersonProver(t, b.root(), now)
			code, stopped := b.runJSON(owners, strings.Fields(act)[1:]...)
			if code != 0 {
				t.Fatalf("printed person act failed: %d %+v", code, stopped)
			}
			current, _ = runner.Status(run)
			paths, _ := filepath.Glob(filepath.Join(b.unitRoot, ".trees", "*.json"))
			if current.State != "cancelled" || len(paths) != 0 {
				t.Fatalf("cancel did not release: %+v %v", current, paths)
			}
			b.starter.fail["build"] = false
			code, next := treeBuild(t, b, "v", false)
			if code != 0 {
				t.Fatalf("next writer did not proceed: %d %+v", code, next)
			}
			code, repeat := b.runJSON(owners, strings.Fields(act)[1:]...)
			if code != 0 {
				t.Fatalf("repeat cancellation touched new owner: %d %+v", code, repeat)
			}
		})
	}
}

func treeLockPaths(t *testing.T, b *workBed) []string {
	t.Helper()
	trees, err := filepath.Glob(filepath.Join(b.unitRoot, ".trees", "*.json.lock"))
	if err != nil || len(trees) != 1 {
		t.Fatalf("tree transition locks: %v %v", trees, err)
	}
	return trees
}

func assertTreeLocksFree(t *testing.T, paths []string) {
	t.Helper()
	for _, path := range paths {
		held, err := lock.File(path, 0600, lock.TryExclusive)
		if err != nil {
			t.Fatalf("command waited while holding %s: %v", path, err)
		}
		if err := held.Release(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestIntentTreeLockOrderAndWaitAllowOwnerToContinue(t *testing.T) {
	t.Parallel()
	b, rebaseOwners, _ := rebaseIntentBed(t)
	b.starter.hold = "build"
	var run string
	saves := 0
	b.workOwnersHook = func(owners *intentWorkOwners) {
		units := owners.units
		owners.units = func(layout stateroot.Layout) *launch.UnitRunner {
			runner := units(layout)
			runner.AfterWrite = func(record launch.UnitRunRecord) error {
				run = record.ID
				saves++
				// Saving a unit cannot acquire the tree after its lower locks.
				for _, path := range treeLockPaths(t, b) {
					held, err := lock.File(path, 0600, lock.TryExclusive)
					if held != nil {
						held.Release()
					}
					if !lock.Busy(err) {
						t.Fatalf("unit saved without first holding the tree: %v", err)
					}
				}
				return nil
			}
			return runner
		}
	}
	sleep := b.manager.Sleep
	observed := false
	b.manager.Sleep = func(d time.Duration) {
		if !observed {
			observed = true
			paths := treeLockPaths(t, b)
			names, _ := filepath.Glob(filepath.Join(b.unitRoot, ".named", "*.lock"))
			paths = append(paths, names...)
			paths = append(paths, filepath.Join(b.unitRoot, run, ".lock"))
			assertTreeLocksFree(t, paths)
			code, waiting := b.runJSON(rebaseOwners, "work", "rebase", b.id)
			if code != 3 || waiting.Next == nil || !slices.Equal(waiting.Next.Argv, []string{"metasystem", "work", "wait", "run:" + run}) {
				t.Fatalf("competing rebase did not wait for the unit: %d %+v", code, waiting)
			}
		}
		sleep(d)
	}
	code, started := treeBuild(t, b, "u", false)
	if code != 3 || !observed || saves == 0 {
		t.Fatalf("build did not exercise waiting: %d %+v", code, started)
	}
	b.manager.Sleep = sleep
	record, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.manager.Store.Update(record.Rounds[0].Steps[0].LaunchID, func(r *launch.Record) error {
		zero := 0
		r.State, r.ExitCode, r.Supervisor, r.Child = launch.Completed, &zero, nil, nil
		r.FinishedAt = b.manager.Now().UTC().Format(time.RFC3339Nano)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	b.starter.hold = ""
	code, continued, _ := b.work("work", "wait", "run:"+run)
	if code != 0 || !slices.Equal(b.starter.launched(), []string{"build", "proof"}) {
		t.Fatalf("owner's next command failed: %d %+v %v", code, continued, b.starter.launched())
	}
}

func TestIntentTreeBranchMutationsReserveWholeOperation(t *testing.T) {
	t.Parallel()
	treeMutationScenario(t, "rebase")
}

// Manual submission captures its source through Git subprocesses with no
// injectable capture seam. This adapter test follows that capture into the
// reservation and checks custody during the remote read and after its failure.
func TestIntentTreeSubmissionGitAdapterReservesThroughRemoteRead(t *testing.T) {
	t.Parallel()
	treeMutationScenario(t, "submission")
}

func treeMutationScenario(t *testing.T, operation string) {
	t.Helper()
	b, owners, _ := rebaseIntentBed(t)
	invoked := false
	var owner string
	inspect := func() {
		invoked = true
		assertTreeLocksFree(t, treeLockPaths(t, b))
		code, waiting := treeBuild(t, b, "v", false)
		if code != 3 || waiting.Next == nil || len(waiting.Next.Argv) != 4 || !strings.HasPrefix(waiting.Next.Argv[3], "run:") {
			t.Fatalf("build entered an ongoing %s: %d %+v", operation, code, waiting)
		}
		owner = strings.TrimPrefix(waiting.Next.Argv[3], "run:")
		// Releasing a live branch command would admit a second writer.
		person := b.workOwners()
		now, err := b.commandNow(b.root())
		if err != nil {
			t.Fatal(err)
		}
		person.prove = enrolledPersonProver(t, b.root(), now)
		stopCode, stopped := b.runJSON(person, "work", "stop", "run:"+owner)
		if stopCode != 1 || !strings.Contains(resultWords(stopped), "remains reserved") {
			t.Fatalf("live branch command was released: %d %+v", stopCode, stopped)
		}
		if len(b.starter.launched()) != 0 {
			t.Fatal("waiting build launched a child")
		}
		code, pending, _ := b.work(waiting.Next.Argv[1:]...)
		if code != 3 || pending.Next == nil || !slices.Equal(pending.Next.Argv, waiting.Next.Argv) {
			t.Fatalf("operation wait cannot be followed: %d %+v", code, pending)
		}
	}
	if operation == "rebase" {
		owners.connection.rebase = func(branch.RebaseRequest) (branch.RebaseResult, error) {
			inspect()
			return branch.RebaseResult{State: "held"}, nil
		}
		code, result := b.runJSON(owners, "work", "rebase", b.id)
		if code != 0 {
			t.Fatalf("rebase failed: %d %+v", code, result)
		}
	} else {
		// Manual capture reads a real temporary checkout before admission.
		connectionGit(t, b.root(), "init", "-q")
		connectionGit(t, b.root(), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-qm", "fixture base")
		owners.connection.endpointTip = func(string, goal.Endpoint) (string, error) {
			inspect()
			return "", errors.New("the remote is unavailable")
		}
		code, result := b.runJSON(owners, "work", "review", b.id, "--work", "manual", "--changes", "--brief", b.brief("manual.md", "Submit manual work.\n"))
		if code != 1 || !strings.Contains(resultWords(result), "remote is unavailable") {
			t.Fatalf("submission did not reach its remote boundary: %d %+v", code, result)
		}
	}
	if !invoked {
		t.Fatalf("%s did not reach its operation boundary", operation)
	}
	code, ended, _ := b.work("work", "wait", "run:"+owner)
	if code != 0 || len(b.starter.launched()) != 0 {
		t.Fatalf("ended operation wait started work: %d %+v", code, ended)
	}
	code, next := treeBuild(t, b, "v", false)
	if code != 0 || !slices.Equal(b.starter.launched(), []string{"build", "proof"}) {
		t.Fatalf("operation did not release after return: %d %+v %v", code, next, b.starter.launched())
	}
}

func TestIntentTreeWaitDoesNotOverwritePersonCancellation(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	b.starter.hold = "build"
	var run string
	b.workOwnersHook = func(owners *intentWorkOwners) {
		units := owners.units
		owners.units = func(layout stateroot.Layout) *launch.UnitRunner {
			r := units(layout)
			r.AfterWrite = func(record launch.UnitRunRecord) error { run = record.ID; return nil }
			return r
		}
	}
	sleep := b.manager.Sleep
	cancelled := false
	b.manager.Sleep = func(d time.Duration) {
		if !cancelled {
			cancelled = true
			owners := b.workOwners()
			now, err := b.commandNow(b.root())
			if err != nil {
				t.Fatal(err)
			}
			owners.prove = enrolledPersonProver(t, b.root(), now)
			code, result := b.runJSON(owners, "work", "stop", "run:"+run)
			if code != 1 || !strings.Contains(resultWords(result), "remains reserved") {
				t.Fatalf("live-child cancellation released custody: %d %+v", code, result)
			}
		}
		sleep(d)
	}
	code, result := treeBuild(t, b, "u", false)
	record, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if code != 1 || !cancelled || err != nil || record.State != "cancelled" || len(b.starter.launched()) != 1 {
		t.Fatalf("wait overwrote cancellation: %d %+v %+v %v", code, result, record, err)
	}
}

type resolvingTreeGit struct{ workGit }

func (g resolvingTreeGit) Run(dir string, env []string, args ...string) ([]byte, error) {
	if slices.Equal(args, []string{"rev-parse", "--verify", "HEAD"}) {
		return []byte(g.bed.head), nil
	}
	if slices.Equal(args, []string{"rev-parse", "--verify", "REBASE_HEAD"}) {
		return []byte(strings.Repeat("c", 40)), nil
	}
	return g.workGit.Run(dir, env, args...)
}

func TestIntentTreeRebaseCorrectionStaysInOperationCustody(t *testing.T) {
	t.Parallel()
	b, owners, _ := rebaseIntentBed(t)
	b.head = strings.Repeat("b", 40)
	b.workOwnersHook = func(owners *intentWorkOwners) {
		units := owners.units
		owners.units = func(layout stateroot.Layout) *launch.UnitRunner {
			r := units(layout)
			r.Git = resolvingTreeGit{workGit{b}}
			return r
		}
	}
	code, built := treeBuild(t, b, "u", false)
	if code != 0 {
		t.Fatalf("build: %d %+v", code, built)
	}
	run := resultData(t, built)["run"].(string)
	record, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if err != nil {
		t.Fatal(err)
	}
	// A completed transfer is a lawful closed owner before branch mutation.
	record.Rounds[0].Transferred = true
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.unitRoot, run, "run.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	owners = b.workOwners()
	owners.delivery = &intentDeliveryOwners{laneRoot: func(string, time.Time) (string, bool, error) { return "", false, nil }, now: b.manager.Now}
	owners.connection.recordRebase = func(*intentInvocation, string, branch.RebaseResult) error { return nil }
	owners.connection.endpointTip = func(string, goal.Endpoint) (string, error) { return strings.Repeat("a", 40), nil }
	owners.connection.section = func(_ string, body func(func(func() error) error) error) error {
		return body(func(fn func() error) error { return fn() })
	}
	var operation string
	owners.connection.rebase = func(req branch.RebaseRequest) (branch.RebaseResult, error) {
		paths, _ := filepath.Glob(filepath.Join(b.unitRoot, ".trees", "*.json"))
		data, err := os.ReadFile(paths[0])
		var owner struct {
			Run      string
			Children []string
		}
		if err != nil || json.Unmarshal(data, &owner) != nil {
			t.Fatalf("operation owner: %s %v", data, err)
		}
		operation = owner.Run
		if _, err := req.Resolve(branch.RebaseResolution{Unit: "u", Worktree: b.worktree, Base: b.head, Commit: strings.Repeat("c", 40), Conflicts: "Resolve the conflict."}); err != nil {
			return branch.RebaseResult{}, err
		}
		data, err = os.ReadFile(paths[0])
		if err != nil || json.Unmarshal(data, &owner) != nil || owner.Run != operation || len(owner.Children) < 2 {
			t.Fatalf("correction displaced operation custody: %s %v", data, err)
		}
		code, waiting := treeBuild(t, b, "v", false)
		if code != 3 || waiting.Next == nil || waiting.Next.Argv[3] != "run:"+operation {
			t.Fatalf("competing build did not wait for rebase: %d %+v", code, waiting)
		}
		return branch.RebaseResult{State: "held"}, nil
	}
	code, rebased := b.runJSON(owners, "work", "rebase", b.id)
	record, err = (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if code != 0 || err != nil || len(record.Rounds) != 2 || !slices.Equal(b.starter.launched(), []string{"build", "proof", "build", "proof"}) {
		t.Fatalf("rebase correction waited on itself: %d %+v %+v %v", code, rebased, record, err)
	}
	code, ended, _ := b.work("work", "wait", "run:"+operation)
	if code != 0 {
		t.Fatalf("rebase wait after correction: %d %+v", code, ended)
	}
}

func publicationLockPaths(t *testing.T, b *workBed, run string) []string {
	t.Helper()
	paths := treeLockPaths(t, b)
	named, err := filepath.Glob(filepath.Join(b.unitRoot, ".named", "*.lock"))
	if err != nil || len(named) != 1 {
		t.Fatalf("named locks: %v %v", named, err)
	}
	return append(append(paths, named...), filepath.Join(b.unitRoot, run, ".lock"))
}

func TestIntentTreePublicationComparesFreshBytesUnderOwnerLocks(t *testing.T) {
	t.Parallel()
	for _, movement := range []string{"bytes", "head"} {
		t.Run(movement, func(t *testing.T) {
			t.Parallel()

			b, owners, run, _ := unitCarryIntentBed(t, false)
			tree := new(string)
			b.head = "replayed-tip"
			owners.work.units = func(stateroot.Layout) *launch.UnitRunner {
				return &launch.UnitRunner{Manager: b.manager, Root: b.unitRoot, Git: boundaryGit{b, tree}}
			}
			paths := publicationLockPaths(t, b, run)
			remoteReads := 0
			owners.connection.endpointTip = func(string, goal.Endpoint) (string, error) {
				remoteReads++
				assertTreeLocksFree(t, paths)
				code, waiting := treeBuild(t, b, "competitor", false)
				if code != 3 || waiting.Next == nil || waiting.Next.Argv[3] != "run:"+run {
					t.Fatalf("remote read lost the reservation: %d %+v", code, waiting)
				}
				return strings.Repeat("a", 40), nil
			}
			owners.connection.push = func(branch.PushRequest) (branch.PushResult, error) {
				assertTreeLocksFree(t, paths)
				return branch.PushResult{Tip: b.head}, nil
			}
			reads, published := 0, 0
			owners.delivery.branchRead = func([]string) (branch.BranchReadResult, int, error) {
				reads++
				code, competing := b.runJSON(owners, "work", "rebase", b.id)
				if code != 3 || competing.Next == nil || !strings.Contains(competing.Summary, "another command") {
					t.Fatalf("a locked publication did not give rebase a retry: %d %+v", code, competing)
				}
				for _, path := range paths {
					held, err := lock.File(path, 0600, lock.TryExclusive)
					if held != nil {
						held.Release()
					}
					if !lock.Busy(err) {
						t.Fatalf("publication did not hold %s: %v", path, err)
					}
				}
				if movement == "bytes" {
					*tree = ":100644 100644 " + strings.Repeat("a", 40) + " " + strings.Repeat("b", 40) + " M\x00unit.go\x00"
				} else {
					b.head = "another-commit"
				}
				return branch.BranchReadResult{State: "collected", AttestationCommit: "read-commit"}, 0, nil
			}
			owners.delivery.publishRead = func(string, string, string) (branch.PublishReadResult, error) {
				published++
				return branch.PublishReadResult{State: "current"}, nil
			}
			code, result := b.runJSON(owners, "work", "review", "run:"+run)
			if code != 1 || reads != 1 || published != 0 || remoteReads != 1 || !strings.Contains(result.Summary, "changed after the read") || resultData(t, result)["cause"] != "environment" {
				t.Fatalf("changed bytes published clean: %d %+v reads=%d publications=%d remote=%d", code, result, reads, published, remoteReads)
			}
			*tree, b.head = "", "replayed-tip"
			owners.delivery.branchRead = func([]string) (branch.BranchReadResult, int, error) {
				return branch.BranchReadResult{State: "already-collected", AttestationCommit: "read-commit"}, 0, nil
			}
			code, result = b.runJSON(owners, "work", "review", "run:"+run)
			if code != 0 || published != 1 {
				t.Fatalf("restoring the result did not permit publication: %d %+v", code, result)
			}
			assertTreeLocksFree(t, paths)
		})
	}
}

func TestIntentTreeCollectedReadPushReleasesCommandLocks(t *testing.T) {
	t.Parallel()
	b, owners, run, _ := unitCarryIntentBed(t, false)
	b.head = "replayed-tip"
	owners.work.units = func(stateroot.Layout) *launch.UnitRunner {
		return &launch.UnitRunner{Manager: b.manager, Root: b.unitRoot, Git: boundaryGit{b, new(string)}}
	}
	paths := publicationLockPaths(t, b, run)
	publications := 0
	owners.delivery.publishRead = func(root, goalID, unit string) (branch.PublishReadResult, error) {
		publications++
		if !sameDirectory(root, b.worktree) || goalID != b.id || unit != "corrected-commit" {
			t.Fatalf("publication subject: root=%s goal=%s unit=%s", root, goalID, unit)
		}
		code, waiting := treeBuild(t, b, "competitor", false)
		if code != 3 || waiting.Outcome != intentInProgress || waiting.Next == nil ||
			!slices.Equal(waiting.Next.Argv, []string{"metasystem", "work", "wait", "run:" + run}) ||
			strings.Contains(resultWords(waiting), "UNIT_RUN_BUSY") {
			t.Fatalf("competing writer did not wait for the publication's owner: %d %+v", code, waiting)
		}
		assertTreeLocksFree(t, paths)
		return branch.PublishReadResult{State: "current"}, nil
	}
	code, result := b.runJSON(owners, "work", "review", "run:"+run)
	if code != 0 || result.Outcome != intentConfirmed || publications != 1 || len(b.starter.launched()) != 2 {
		t.Fatalf("publication did not complete without starting competing work: %d %+v publications=%d launches=%v", code, result, publications, b.starter.launched())
	}
	assertTreeLocksFree(t, paths)
}

func TestIntentTreeSupervisorStartupReleasesCommandLocks(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	b.manager.StartCap = 10 * time.Second
	var child, run string
	slept := false
	sleep := b.manager.Sleep
	b.manager.Supervisor = supervisorStart(func(id, state string) (identity.Ref, error) {
		record, err := b.manager.Store.Read(id)
		if err != nil {
			t.Fatal(err)
		}
		if record.Kind != "build" {
			return b.starter.StartSupervisor(id, state)
		}
		child = id
		rows, err := filepath.Glob(filepath.Join(b.unitRoot, "*", "run.json"))
		if err != nil || len(rows) != 1 {
			t.Fatalf("runs: %v %v", rows, err)
		}
		run = filepath.Base(filepath.Dir(rows[0]))
		assertTreeLocksFree(t, publicationLockPaths(t, b, run))
		return workProcessRef(10), nil
	})
	b.manager.Sleep = func(d time.Duration) {
		if !slept {
			slept = true
			assertTreeLocksFree(t, publicationLockPaths(t, b, run))
			code, waiting := treeBuild(t, b, "competitor", false)
			if code != 3 || waiting.Next == nil || waiting.Next.Argv[3] != "run:"+run || len(b.starter.launched()) != 0 {
				t.Fatalf("competing command blocked or started during startup: %d %+v", code, waiting)
			}
			if _, err := b.starter.StartSupervisor(child, ""); err != nil {
				t.Fatal(err)
			}
		}
		sleep(d)
	}
	code, built := treeBuild(t, b, "owner", false)
	if code != 0 || !slept || !slices.Equal(b.starter.launched(), []string{"build", "proof"}) {
		t.Fatalf("startup did not complete: %d %+v %v", code, built, b.starter.launched())
	}
}

func TestIntentTreeLiveRebaseWaitPollsCustodyAndDeadline(t *testing.T) {
	t.Parallel()
	for _, completes := range []bool{false, true} {
		name := "deadline"
		if completes {
			name = "completion"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, owners, _ := rebaseIntentBed(t)
			owners.connection.rebase = func(branch.RebaseRequest) (branch.RebaseResult, error) {
				code, waiting := treeBuild(t, b, "competitor", false)
				if code != 3 || waiting.Next == nil {
					t.Fatalf("rebase reservation: %d %+v", code, waiting)
				}
				run := strings.TrimPrefix(waiting.Next.Argv[3], "run:")
				sleeps := 0
				sleep := b.manager.Sleep
				b.manager.Sleep = func(d time.Duration) {
					sleeps++
					assertTreeLocksFree(t, treeLockPaths(t, b))
					if completes {
						record, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
						if err != nil {
							t.Fatal(err)
						}
						record.State = "completed"
						data, err := json.Marshal(record)
						if err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(filepath.Join(b.unitRoot, run, "run.json"), data, 0600); err != nil {
							t.Fatal(err)
						}
					}
					sleep(d)
				}
				defer func() { b.manager.Sleep = sleep }()
				code, result, _ := b.work("work", "wait", "run:"+run, "--timeout", "2s")
				expected := 3
				if completes {
					expected = 0
				}
				if code != expected || sleeps == 0 || len(b.starter.launched()) != 0 {
					t.Fatalf("wait did not observe live rebase: %d %+v sleeps=%d", code, result, sleeps)
				}
				if !completes && (result.Next == nil || result.Next.Argv[3] != "run:"+run) {
					t.Fatalf("no continuation: %+v", result)
				}
				return branch.RebaseResult{State: "held"}, nil
			}
			if code, result := b.runJSON(owners, "work", "rebase", b.id); code != 0 {
				t.Fatalf("rebase: %d %+v", code, result)
			}
		})
	}
}

func TestIntentTreeStartupLockRetakePrintsRetryWithoutSaving(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	var held *lock.FileLock
	var run string
	b.manager.Supervisor = supervisorStart(func(id, state string) (identity.Ref, error) {
		child, err := b.manager.Store.Read(id)
		if err != nil {
			t.Fatal(err)
		}
		if child.Kind != "build" {
			return b.starter.StartSupervisor(id, state)
		}
		rows, err := filepath.Glob(filepath.Join(b.unitRoot, "*", "run.json"))
		if err != nil || len(rows) != 1 {
			t.Fatalf("runs: %v %v", rows, err)
		}
		run = filepath.Base(filepath.Dir(rows[0]))
		paths := publicationLockPaths(t, b, run)
		assertTreeLocksFree(t, paths)
		held, err = lock.File(paths[1], 0600, lock.TryExclusive)
		if err != nil {
			t.Fatal(err)
		}
		return b.starter.StartSupervisor(id, state)
	})
	code, result := treeBuild(t, b, "owner", false)
	if held != nil {
		held.Release()
	}
	record, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if code != 1 || err != nil || !strings.Contains(resultWords(result), "repeat the same command") || record.Rounds[0].Steps[0].State != launch.StepStarting {
		t.Fatalf("lock retake failed without a safe retry: %d %+v %+v %v", code, result, record, err)
	}
	b.manager.Supervisor = b.starter
	code, result, _ = b.work("work", "wait", "run:"+run)
	if code != 0 || !slices.Equal(b.starter.launched(), []string{"build", "proof"}) {
		t.Fatalf("retry repeated the build: %d %+v", code, result)
	}
}

func TestIntentTreeStartupCannotOverwriteReleasedCancellation(t *testing.T) {
	t.Parallel()
	b := newWorkBedWith(t, func(file *goal.GoalFile) {
		workApprovedBox(file)
		file.Budget.ReservedJobMinutesLimit = 600
		file.NormApproval = &goal.GoalNormApprovalClaim{ApprovedRef: "R-fixture", Minutes: 600, ReviewRounds: 2, GoalRevision: file.Revision}
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	var run string
	b.manager.Supervisor = supervisorStart(func(id, state string) (identity.Ref, error) {
		rows, err := filepath.Glob(filepath.Join(b.unitRoot, "*", "run.json"))
		if err != nil || len(rows) != 1 {
			t.Fatalf("runs: %v %v", rows, err)
		}
		run = filepath.Base(filepath.Dir(rows[0]))
		assertTreeLocksFree(t, publicationLockPaths(t, b, run))
		if _, err := b.starter.StartSupervisor(id, state); err != nil {
			t.Fatal(err)
		}
		owners := b.workOwners()
		owners.processes = defaultProcessIntentOwners()
		owners.processes.process.repositoryTop = fakeTop(b.root())
		now, err := b.commandNow(b.root())
		if err != nil {
			t.Fatal(err)
		}
		owners.prove = enrolledPersonProver(t, b.root(), now)
		code, stopped := b.runJSON(owners, "work", "stop", "run:"+run)
		if code != 0 {
			t.Fatalf("person could not cancel quiescent startup: %d %+v", code, stopped)
		}
		return workProcessRef(30), nil
	})
	code, built := treeBuild(t, b, "owner", false)
	record, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if code != 1 || err != nil || record.State != "cancelled" || !strings.Contains(resultWords(built), "repeat the same command") || len(b.starter.launched()) != 1 {
		t.Fatalf("startup overwrote the person's cancellation: %d %+v %+v %v", code, built, record, err)
	}
	b.manager.Supervisor = b.starter
	code, result := treeBuild(t, b, "competitor", false)
	if code != 0 || !slices.Equal(b.starter.launched(), []string{"build", "build", "proof"}) {
		t.Fatalf("cancelled custody did not release: %d %+v", code, result)
	}
}
