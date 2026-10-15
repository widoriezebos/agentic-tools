package plain

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestChangedMainRepairOverrideRecordsCompletedEffects(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"notification", "first write", "second write", "launch", "success"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			b := changedMainBed(t)
			checkpoint := Running{Checkpoint: true, Attempt: "candidate", Commit: "candidate", Tree: "candidate-tree"}
			b.save(t, Result{Attempt: checkpoint.Attempt, Tree: checkpoint.Tree, Commit: checkpoint.Commit, Result: Red})
			if err := writeRunning(b.install, checkpoint); err != nil {
				t.Fatal(err)
			}
			if err := WriteFix(b.install, &Fix{Attempt: checkpoint.Attempt, Goal: "repair", Units: []string{"unit"}, Job: "job", Commit: "candidate", State: "building"}); err != nil {
				t.Fatal(err)
			}
			b.seams.Person = &ActProvenance{Person: "Wido"}
			notified, launched := false, false
			b.seams.NotifyRepairOverride = func(string) error {
				notified = true
				if failure == "notification" {
					return errors.New("notification failed")
				}
				if failure == "first write" {
					return os.Mkdir(runningPath(b.install)+".tmp", 0700)
				}
				return nil
			}
			if failure == "second write" {
				blocked := false
				b.seams.Now = func() time.Time {
					current, present, _, err := ReadRunning(b.install, ProveSeams{})
					if err != nil {
						t.Fatal(err)
					}
					if present && current.Attempt == "attempt" && !blocked {
						blocked = true
						if err := os.Mkdir(runningPath(b.install)+".tmp", 0700); err != nil {
							t.Fatal(err)
						}
					}
					return bedNow
				}
			}
			b.seams.Launch = func([]string, string, string) (int64, error) {
				launched = true
				if !notified {
					t.Fatal("launch preceded notification")
				}
				if failure == "launch" {
					return 0, errors.New("launch refused")
				}
				return int64(os.Getpid()), nil
			}
			_, _, err := Start(b.install, b.checkout, b.seams)
			if (err == nil) != (failure == "success") {
				t.Fatalf("Start error = %v", err)
			}
			stored, present, _, readErr := ReadRunning(b.install, b.seams)
			fix, fixErr := FixForAttempt(b.install, checkpoint.Attempt)
			closed := failure != "notification"
			replaced := closed && failure != "first write"
			if readErr != nil || fixErr != nil || !present || fix == nil || (fix.State == "closed") != closed || (stored.Attempt != checkpoint.Attempt) != replaced || !replaced && !reflect.DeepEqual(stored, checkpoint) {
				t.Fatalf("disk effects: running=%+v fix=%+v errors=%v/%v", stored, fix, readErr, fixErr)
			}
			want := "fix not closed, checkpoint retained, launch not attempted"
			if closed {
				want = "fix closed, checkpoint retained, launch not attempted"
			}
			if replaced {
				want = "fix closed, checkpoint replaced, launch not attempted"
			}
			if launched {
				want = "fix closed, checkpoint replaced, launch succeeded"
				if failure == "launch" {
					want = "fix closed, checkpoint replaced, launch failed: launch refused"
				}
			}
			entries, err := readLines[Intervention](interventionsPath(b.install))
			if err != nil || len(entries) != 2 {
				t.Fatalf("request and effects: %+v %v", entries, err)
			}
			for _, entry := range entries {
				if entry.Row != "repair-override" || entry.Subject != "attempt" || entry.Person == nil || entry.Person.Person != "Wido" || !strings.Contains(entry.What, "the candidate's repair checkpoint of tree candidate-tr (fix repair) is replaced by this check at Wido's request") {
					t.Fatalf("lost request: %+v", entry)
				}
			}
			if entries[0].Outcome != "requested; fix not closed, checkpoint retained, launch not attempted" {
				t.Fatalf("request effects: %+v", entries[0])
			}
			if entries[1].Outcome != want {
				t.Fatalf("effects = %q, want %q", entries[1].Outcome, want)
			}
			// Leave the injected write barrier out of the fixture's final observation.
			if failure == "first write" || failure == "second write" {
				if err := os.Remove(filepath.Join(Dir(b.install), "running.json.tmp")); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestChangedMainOrdinaryRunningFixRefusalNamesNoCheckpoint(t *testing.T) {
	t.Parallel()
	b := changedMainBed(t)
	running := Running{Attempt: "ordinary", Tree: "ordinary-tree"}
	if err := writeRunning(b.install, running); err != nil {
		t.Fatal(err)
	}
	if err := WriteFix(b.install, &Fix{Attempt: running.Attempt, Goal: "repair", Units: []string{"unit"}, Job: "job", Commit: "candidate", State: "building"}); err != nil {
		t.Fatal(err)
	}
	pending, err := candidateRepairPending(b.install, running, true)
	if err != nil || pending == nil || pending.Tree != "" || pending.Error() != "main's check waits for the pending fix of repair" {
		t.Fatalf("ordinary running record became a checkpoint: %+v %v", pending, err)
	}
}
