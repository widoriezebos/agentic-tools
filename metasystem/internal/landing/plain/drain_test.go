package plain

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

func drainFixture(t *testing.T, install string) Drain {
	t.Helper()
	drain, _, err := SetDrain(install, Drain{By: "Wido", At: bedNow.UTC().Format(time.RFC3339), Source: DrainSource{Kind: "person"}, Reason: "maintenance"})
	if err != nil {
		t.Fatal(err)
	}
	return drain
}

func TestDrainHandInAdmission(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	line := Line{Goal: "a", SHA: "a1", Delivered: "first"}
	if _, _, err := HandIn(install, line); err != nil {
		t.Fatal(err)
	}
	if _, _, err := HandIn(install, Line{Goal: "returned", SHA: "r1"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReturnProven(install, "returned", "unclassified", "own", true, "fixture", bedNow, ProveSeams{Person: &ActProvenance{Kind: "return", Person: "fixture"}}); err != nil {
		t.Fatal(err)
	}
	first := drainFixture(t, install)
	if drain, changed, err := SetDrain(install, Drain{By: "someone else"}); err != nil || changed || drain != first {
		t.Fatalf("repeat replaced author: %+v %v %v", drain, changed, err)
	}
	line.Delivered = "metadata"
	if e, added, err := HandIn(install, line); err != nil || added || e.Delivered != "metadata" {
		t.Fatalf("repeat: %+v %v %v", e, added, err)
	}
	if e, added, err := HandIn(install, Line{Goal: "returned", SHA: "r1"}); err != nil || added || e.State != StateReturned {
		t.Fatalf("returned repeat: %+v %v", e, err)
	}
	for _, proposed := range []Line{{Goal: "b", SHA: "b1"}, {Goal: "a", SHA: "a2"}, {Goal: "returned", SHA: "r1", Again: true}, {Goal: "records", SHA: "rec", Records: true, DrainBy: "forged", WholeBy: "Wido", Exception: &Exception{By: "Wido"}}} {
		_, _, err := HandIn(install, proposed)
		var closed *AdmissionClosed
		if !errors.As(err, &closed) || closed.Drain.By != "Wido" {
			t.Fatalf("admitted new work: %+v %v", proposed, err)
		}
	}
	if _, _, err := ReturnProven(install, "a", "unclassified", "done", true, "fixture", bedNow, ProveSeams{Person: &ActProvenance{Kind: "return", Person: "fixture"}}); err != nil {
		t.Fatal(err)
	}
	seams := ProveSeams{Git: func(string, ...string) (string, error) { return "main", nil }}
	progress, err := AdvanceDrain(install, "checkout", seams)
	if err != nil || progress.Drain.State != DrainHeld {
		t.Fatalf("empty drain: %+v %v", progress, err)
	}
	proposed := Line{Goal: "person", SHA: "p1", DrainBy: "forged"}
	e, added, err := HandIn(install, proposed, "Wido")
	drain, _ := ReadDrain(install)
	if err != nil || !added || e.DrainBy != "Wido" || drain.State != DrainDraining {
		t.Fatalf("person override: %+v %+v %v", e, drain, err)
	}
	if changed, err := ClearDrain(install); err != nil || !changed {
		t.Fatal(changed, err)
	}
	if _, added, err := HandIn(install, Line{Goal: "b", SHA: "b1"}); err != nil || !added {
		t.Fatal(added, err)
	}
}

func TestHandInSkipsDamagedQueueWithoutDrain(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	if _, _, err := HandIn(install, Line{Goal: "known", SHA: "known-sha"}); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(queuePath(install), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.WriteString("{broken\n")
	if err := errors.Join(writeErr, file.Close()); err != nil {
		t.Fatal(err)
	}
	line := Line{Goal: "new", SHA: "new-sha"}
	if entry, added, err := HandIn(install, line); err != nil || !added || entry.State != StateWaiting {
		t.Fatalf("damaged queue refused a hand-in with admission open: %+v added=%v err=%v", entry, added, err)
	}
	if _, added, err := HandIn(install, line); err != nil || added {
		t.Fatalf("repeat was not preserved: added=%v err=%v", added, err)
	}
	entries, err := Entries(install)
	if err != nil || len(entries) != 2 || entries[1].Goal != line.Goal || entries[1].SHA != line.SHA {
		t.Fatalf("hand-in was not persisted: %+v %v", entries, err)
	}
}

func TestRecordsHandInSupersedesOlderWaitingLine(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	for _, line := range []Line{{Goal: "goal", SHA: "code"}, {Goal: "goal", SHA: "records", Records: true}} {
		if _, _, err := HandIn(install, line); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := Entries(install)
	if err != nil || len(entries) != 2 || entries[0].State != StateSuperseded || entries[1].State != StateWaiting || !entries[1].Records {
		t.Fatalf("records hand-in did not supersede older work of its goal: %+v %v", entries, err)
	}
	waiting, err := Waiting(install)
	if err != nil || len(waiting) != 1 || waiting[0].SHA != "records" {
		t.Fatalf("older work still waits: %+v %v", waiting, err)
	}
}

func TestDrainMembershipIncludesHeldAndDamage(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	if _, _, err := HandIn(install, Line{Goal: "held", SHA: "h"}); err != nil {
		t.Fatal(err)
	}
	if err := appendLine(queuePath(install), Line{Goal: "held", SHA: "h", Outcome: StateWaiting, Held: true}); err != nil {
		t.Fatal(err)
	}
	drainFixture(t, install)
	seams := ProveSeams{Git: func(_ string, args ...string) (string, error) {
		if args[0] == "cat-file" {
			return "", os.ErrNotExist
		}
		return "main", nil
	}}
	p, err := AdvanceDrain(install, "checkout", seams)
	if err != nil || p.Waiting != 1 || p.Drain.State != DrainDraining {
		t.Fatalf("held work disappeared: %+v %v", p, err)
	}
	if _, _, err := ReturnProven(install, "held", "unclassified", "returned", true, "fixture", bedNow, ProveSeams{Person: &ActProvenance{Kind: "return", Person: "fixture"}}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queuePath(install), []byte("{broken\n{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p, err = AdvanceDrain(install, "checkout", seams)
	if err != nil || p.Unknown == "" || p.Drain.State != DrainDraining {
		t.Fatalf("unknown membership held or returned error: %+v %v", p, err)
	}
	if _, _, err := HandIn(install, Line{Goal: "agent", SHA: "a"}); err == nil {
		t.Fatal("agent crossed the standing drain with unknown membership")
	}
	if entry, added, err := HandIn(install, Line{Goal: "person", SHA: "p"}, "Wido"); err != nil || !added || entry.DrainBy != "Wido" {
		t.Fatalf("damaged membership blocked a person's hand-in: %+v added=%v err=%v", entry, added, err)
	}
	if _, added, err := HandIn(install, Line{Goal: "person", SHA: "p"}); err != nil || added {
		t.Fatalf("known repeat was blocked by damaged membership: added=%v err=%v", added, err)
	}
	p, err = AdvanceDrain(install, "checkout", seams)
	if err != nil || p.Unknown == "" || p.Waiting != 1 || p.Drain.State != DrainDraining {
		t.Fatalf("person's hand-in hid damage or waiting work: %+v %v", p, err)
	}
}

func TestDrainProgressWaitsForProofAndRegeneration(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"proof", "regeneration", "undecidable"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			drainFixture(t, install)
			log := filepath.Join(install, "log")
			if err := os.WriteFile(log, nil, 0600); err != nil {
				t.Fatal(err)
			}
			path := runningPath(install)
			if name == "regeneration" {
				path = regenerationRunningPath(install)
				if err := writeRegeneration(install, RunningRegeneration{Goal: "g", SHA: "s", Log: log, Pid: 42, Process: "unknown"}); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := writeRunning(install, Running{Attempt: "proof", Pid: 42, Process: "unknown"}); err != nil {
					t.Fatal(err)
				}
			}
			seams := ProveSeams{Git: func(string, ...string) (string, error) { return "main", nil }, Alive: func(Running) bool { return true }}
			if name == "undecidable" {
				seams.Alive = nil
			}
			p, err := AdvanceDrain(install, "checkout", seams)
			if err != nil || p.Drain.State != DrainDraining || name == "undecidable" && p.Unknown == "" {
				t.Fatalf("unfinished progress: %+v %v", p, err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			p, err = AdvanceDrain(install, "checkout", seams)
			if err != nil || p.Drain.State != DrainHeld {
				t.Fatalf("finished: %+v %v", p, err)
			}
		})
	}
}

func TestDrainAndHandInSerialize(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	if err := os.MkdirAll(Dir(install), 0755); err != nil {
		t.Fatal(err)
	}
	held, err := lock.File(lockPath(install), 0600, lock.Exclusive)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	started := make(chan struct{}, 2)
	var handErr, drainErr error
	go func() {
		defer wg.Done()
		started <- struct{}{}
		_, _, handErr = HandIn(install, Line{Goal: "g", SHA: "s"})
	}()
	go func() {
		defer wg.Done()
		started <- struct{}{}
		_, _, drainErr = SetDrain(install, Drain{By: "Wido", At: bedNow.Format(time.RFC3339), Source: DrainSource{Kind: "person"}})
	}()
	<-started
	<-started
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	entries, err := Entries(install)
	var closed *AdmissionClosed
	if err != nil || drainErr != nil || handErr != nil && !errors.As(handErr, &closed) || (handErr == nil) != (len(entries) == 1) {
		t.Fatalf("partial admission: %+v hand=%v drain=%v read=%v", entries, handErr, drainErr, err)
	}
	if _, _, err := HandIn(install, Line{Goal: "later", SHA: "s2"}); err == nil {
		t.Fatal("locked drain check was bypassed")
	}
}

func TestDrainProofAdmissionAndTimer(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	drainFixture(t, install)
	launches := 0
	// Missing objects and an ancestor exit use production Git's actual error shapes.
	notAncestor := exec.Command("/usr/bin/false").Run()
	seams := ProveSeams{Now: func() time.Time { return bedNow }, Git: func(_ string, args ...string) (string, error) {
		if args[0] == "merge-base" {
			return "", notAncestor
		}
		if args[0] == "ls-tree" {
			return "", nil
		}
		return "main", nil
	}, Executable: func() (string, error) { return "engine", nil }, Launch: func([]string, string, string) (int64, error) { launches++; return int64(os.Getpid()), nil }, Alive: func(Running) bool { return false }}
	if _, _, err := Start(install, "checkout", seams); err == nil || launches != 0 {
		t.Fatal("idle proof was admitted", err, launches)
	}
	if _, _, err := HandIn(install, Line{Goal: "g", SHA: "s"}, "Wido"); err != nil {
		t.Fatal(err)
	}
	seams.Git = func(_ string, args ...string) (string, error) {
		if args[0] == "ls-tree" {
			return "", nil
		}
		return "main", nil
	}
	if _, _, err := Start(install, "checkout", seams); err != nil || launches != 1 {
		t.Fatal("admitted proof stopped", err, launches)
	}
	if err := os.Remove(runningPath(install)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReturnProven(install, "g", "unclassified", "own", true, "fixture", bedNow, ProveSeams{Person: &ActProvenance{Kind: "return", Person: "fixture"}}); err != nil {
		t.Fatal(err)
	}
	reasons, err := WakeReasons(install, "checkout", time.Time{}, bedNow, seams)
	if err != nil || len(reasons) != 0 {
		t.Fatal("timer woke drained work", reasons, err)
	}
	seams.Trunk = true
	if _, _, err := Start(install, "checkout", seams); err == nil {
		t.Fatal("agent admitted trunk timer proof")
	}
	if _, _, err := HandIn(install, Line{Goal: "g", SHA: "s", Again: true}, "Wido"); err != nil {
		t.Fatal(err)
	}
	seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
		return []goal.TrunkRedEntry{{Identity: "incident", Opened: bedNow.Format(time.RFC3339), Class: goal.TrunkRedClassTrunkRed}}, nil
	}
	if _, _, err := Start(install, "checkout", seams); err != nil {
		t.Fatal("incident proof blocked", err)
	}
}

// TestHandInUnderDrainWithDamagedQueueRefusesAsClosed: an agent's hand-in
// under a drain is refused as admission closed even when a queue line cannot
// be decoded, so its printed remedy (wait until admission opens) is the one
// that works.
func TestHandInUnderDrainWithDamagedQueueRefusesAsClosed(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	if err := os.MkdirAll(Dir(install), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queuePath(install), []byte("{not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := SetDrain(install, Drain{By: "Wido", At: bedNow.UTC().Format(time.RFC3339), Source: DrainSource{Kind: "person"}, Reason: "maintenance"}); err != nil {
		t.Fatal(err)
	}
	_, _, err := HandIn(install, Line{Goal: "g", SHA: "s"})
	var closed *AdmissionClosed
	if !errors.As(err, &closed) {
		t.Fatalf("an agent's hand-in under a drain with a damaged queue line: %v", err)
	}
}
