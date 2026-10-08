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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
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
				b.manager.Store.Update(child, func(r *launch.Record) error { r.State = launch.Cancelled; return nil })
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
