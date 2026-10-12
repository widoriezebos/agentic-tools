package plain

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestLandingGiveBackCancellationRecovery(t *testing.T) {
	t.Parallel()
	install, batch, _, seams, ended := restartBed(t, 10, false)
	member := batch.Members[0]
	person := ActProvenance{Person: "Wido", CheckedAt: restartNow}
	stop := seams.EndProof
	seams.EndProof = func(Running) error { return errors.New("cannot stop process") }
	_, _, err := GiveBack(install, member.Goal, "continue at seat", person, seams)
	if err == nil {
		t.Fatal("failed termination reported done")
	}
	saved, err := ReadBatch(install)
	if err != nil || len(saved.Actions) != 1 || !saved.Actions[0].Pending || saved.Actions[0].Person.Person != "Wido" || saved.Actions[0].Reason != "continue at seat" || !reflect.DeepEqual(saved.Members, batch.Members) {
		t.Fatalf("request lost: %+v %v", saved, err)
	}
	seams.EndProof = stop
	// A new keeper consumes the saved decision; no repeated person command.
	if err := withLock(install, func() error { return finishGiveBacksLocked(install, seams) }); err != nil {
		t.Fatal(err)
	}
	entry, _, err := Latest(install, member.Goal)
	if err != nil || entry.State != StateReturned || entry.Reason != "continue at seat" || entry.ReturnOrigin.Person.Person != "Wido" || *ended != 1 {
		t.Fatalf("recovery: %+v %v ends %d", entry, err, *ended)
	}
	result, known, err := LastResult(install)
	if err != nil || !known || result.Result != "cancelled" {
		t.Fatalf("cancel result: %+v %v", result, err)
	}
	if _, err := os.Stat(runningPath(install)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("execution slot not released: %v", err)
	}
}

func TestLandingLandAnywayPushRecord(t *testing.T) {
	t.Parallel()
	for _, currentException := range []bool{true, false} {
		t.Run(fmt.Sprint(currentException), func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			member := GoalSHA{Goal: "fixture-goal", SHA: "fixture-sha"}
			decision := &Exception{Code: "GOAL_LAND_TRUNK_RED", Reason: "ship independent fix", By: "Wido", Goal: member.Goal, SHA: member.SHA, Person: &ActProvenance{Person: "Wido", CheckedAt: bedNow}, Incidents: &IncidentCoverage{Open: []IncidentIdentity{{Identity: "fixture-incident", Opened: bedNow.Format(time.RFC3339), FirstSeen: "sighting"}}}}
			decision.Impact = "This lands fixture-goal while main fails fixture-incident; those failures stay open and later goals are still held by them; undo with `landing give-back fixture-goal` before the push, or a revert after."
			old := *decision
			old.Reason, old.By = "withdrawn decision", "previous person"
			if _, _, err := HandIn(install, Line{Goal: member.Goal, SHA: member.SHA, Exception: &old}); err != nil {
				t.Fatal(err)
			}
			if err := appendLine(queuePath(install), Line{Goal: member.Goal, SHA: member.SHA, Outcome: StateReturned, At: bedNow.Format(time.RFC3339)}); err != nil {
				t.Fatal(err)
			}
			if !currentException {
				decision = nil
			}
			if _, _, err := HandIn(install, Line{Goal: member.Goal, SHA: member.SHA, Again: true, Exception: decision}); err != nil {
				t.Fatal(err)
			}
			if err := appendLine(resultsPath(install), Result{Result: Green, Attempt: "fixture-proof", Tree: "tree", Commit: "head", Goals: []GoalSHA{member}}); err != nil {
				t.Fatal(err)
			}
			seams := ProveSeams{BudgetHome: t.TempDir(), Now: func() time.Time { return bedNow }, Git: func(string, ...string) (string, error) { return "", errors.New("no git needed for saved decision") }}
			if err := completePush(install, install, nil, "main", "head", "tree", bedNow, seams); err != nil {
				t.Fatal(err)
			}
			push, known, err := LastPush(install)
			want := []*Exception{}
			if currentException {
				want = append(want, decision)
			}
			if err != nil || !known || len(push.Exceptions) != len(want) || len(want) > 0 && !reflect.DeepEqual(push.Exceptions, want) {
				t.Fatalf("push decision missing or withdrawn decision reused: %+v %v", push, err)
			}
		})
	}
}

func TestLandingGiveBackPersonProofCompletion(t *testing.T) {
	t.Parallel()
	install, batch, running, seams, ended := restartBed(t, 10, false)
	running.Person = &ActProvenance{Person: "Wido"}
	if err := writeRunning(install, running); err != nil {
		t.Fatal(err)
	}
	member := batch.Members[0]
	_, message, err := GiveBack(install, member.Goal, "person's choice", ActProvenance{Person: "Wido", CheckedAt: restartNow}, seams)
	if err != nil || !strings.Contains(message, "proof continues") || *ended != 0 {
		t.Fatalf("person proof: %s %v", message, err)
	}
	seams.Alive = func(Running) bool { return false }
	if err := appendLine(resultsPath(install), Result{Attempt: running.Attempt, Tree: running.Tree, Result: Green, Goals: batch.Members}); err != nil {
		t.Fatal(err)
	}
	if err := withLock(install, func() error { _, _, _, err := checkState(install, install, "", seams); return err }); err != nil {
		t.Fatal(err)
	}
	entry, _, err := Latest(install, member.Goal)
	if err != nil || entry.State != StateReturned || *ended != 0 {
		t.Fatalf("completion: %+v %v", entry, err)
	}
	// The old green merge is no longer admitted for publication.
	baseGit := seams.Git
	seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "rev-list" {
			return running.Commit + " main " + member.SHA, nil
		}
		if args[0] == "merge-base" && args[2] == "main" {
			return "", nil
		}
		return baseGit(dir, args...)
	}
	seams.Lane = func() (lane.Record, error) { return batch.Lane, nil }
	if _, err := CheckBatch(install, install, running.Commit, "main", false, seams); err == nil || !strings.Contains(err.Error(), "outside the ordered selection") {
		t.Fatalf("withdrawn green candidate still admitted: %v", err)
	}
}

func TestLandingGiveBackSupersededPersonProof(t *testing.T) {
	t.Parallel()
	for _, changed := range []string{"new-hand-in", "removed-member"} {
		t.Run(changed, func(t *testing.T) {
			t.Parallel()
			install, batch, running, seams, ended := restartBed(t, 10, false)
			running.Person = &ActProvenance{Person: "Wido"}
			if err := writeRunning(install, running); err != nil {
				t.Fatal(err)
			}
			member := batch.Members[0]
			if _, _, err := GiveBack(install, member.Goal, "person's choice", ActProvenance{Person: "Wido", CheckedAt: restartNow}, seams); err != nil {
				t.Fatal(err)
			}
			wantSHA := member.SHA
			if changed == "new-hand-in" {
				wantSHA = "sha2"
				if _, _, err := HandIn(install, Line{Goal: member.Goal, SHA: wantSHA}); err != nil {
					t.Fatal(err)
				}
			} else {
				saved, err := ReadBatch(install)
				if err != nil {
					t.Fatal(err)
				}
				saved.Members = nil
				if err := writeBatch(install, saved); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(queuePath(install))
			if err != nil {
				t.Fatal(err)
			}
			seams.Alive = func(Running) bool { return false }
			if err := appendLine(resultsPath(install), Result{Attempt: running.Attempt, Tree: running.Tree, Result: Green, Goals: batch.Members}); err != nil {
				t.Fatal(err)
			}
			for pass := 0; pass < 2; pass++ {
				if err := withLock(install, func() error { _, _, _, err := checkState(install, install, "", seams); return err }); err != nil {
					t.Fatal(err)
				}
				saved, err := ReadBatch(install)
				if err != nil || len(saved.Actions) != 1 || saved.Actions[0].Pending || saved.Actions[0].Impact != "superseded by a newer hand-in; nothing returned" {
					t.Fatalf("stale request not cleared: %+v %v", saved, err)
				}
			}
			entry, _, err := Latest(install, member.Goal)
			if err != nil || entry.SHA != wantSHA || entry.State != StateWaiting || entry.ReturnOrigin != nil || *ended != 0 {
				t.Fatalf("stale request returned a hand-in: %+v %v ends %d", entry, err, *ended)
			}
			after, err := os.ReadFile(queuePath(install))
			if err != nil || string(before) != string(after) {
				t.Fatalf("completion wrote a return: %v", err)
			}
		})
	}
}
