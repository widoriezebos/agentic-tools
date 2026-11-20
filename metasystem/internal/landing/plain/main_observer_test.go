package plain

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestMovedMainSuccessiveSubjectsShareTheProofSlot(t *testing.T) {
	t.Parallel()
	b := changedMainBed(t)
	now := bedNow
	b.seams.Now = func() time.Time { return now }
	launches := 0
	b.seams.Launch = func([]string, string, string) (int64, error) { launches++; return int64(os.Getpid()), nil }
	git := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "fetch" {
			return "", nil
		}
		return git(dir, args...)
	}
	pass := func() {
		t.Helper()
		if err := ObserveMovedMain(b.install, b.checkout, b.seams); err != nil {
			t.Fatal(err)
		}
	}
	pass()
	first := b.git.commit
	now = now.Add(5 * time.Second)
	b.git.commit, b.git.tree = "second", "second-tree"
	b.setContract(t)
	pass()
	var sight mainSighting
	if err := readAgentRecord(b.install, mainSightingFile, &sight); err != nil || sight.Commit != "second" || !sight.Seen.Equal(now) || !strings.Contains(sight.line(now), "proof of "+first+" holds the slot") || launches != 1 {
		t.Fatalf("second main: %+v, launches=%d, err=%v", sight, launches, err)
	}
	if err := removeRunning(b.install); err != nil {
		t.Fatal(err)
	}
	now = now.Add(15 * time.Second)
	pass()
	r, _, _, err := ReadRunning(b.install, b.seams)
	if err != nil || r.Commit != "second" || launches != 2 {
		t.Fatalf("next slot: %+v launches=%d err=%v", r, launches, err)
	}
}

func TestMovedMainRacingSubjectWaitsForNextObservation(t *testing.T) {
	t.Parallel()
	b := changedMainBed(t)
	git := b.seams.Git
	reads, starts := 0, 0
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "fetch" {
			return "", nil
		}
		if args[len(args)-1] == "origin/main^{commit}" {
			reads++
			if reads == 2 {
				b.git.commit = "next-main"
				b.git.tree = "next-tree"
				b.setContract(t)
			}
		}
		return git(dir, args...)
	}
	b.seams.Launch = func([]string, string, string) (int64, error) { starts++; return int64(os.Getpid()), nil }
	for pass := 0; pass < 2; pass++ {
		err := ObserveMovedMain(b.install, b.checkout, b.seams)
		if err != nil || starts != pass {
			t.Fatalf("pass=%d starts=%d err=%v", pass, starts, err)
		}
	}
}

func TestMovedMainConcurrentKeepersHaveOneAdmission(t *testing.T) {
	t.Parallel()
	b := changedMainBed(t)
	entered, resume := make(chan struct{}), make(chan struct{})
	git := b.seams.Git
	fetches, launches := 0, 0
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "fetch" {
			fetches++
			if fetches == 1 {
				close(entered)
				<-resume
			}
			return "", nil
		}
		return git(dir, args...)
	}
	b.seams.Launch = func([]string, string, string) (int64, error) { launches++; return int64(os.Getpid()), nil }
	done := make(chan error, 2)
	go func() { done <- ObserveMovedMain(b.install, b.checkout, b.seams) }()
	<-entered
	go func() { done <- RecoverMovedMain(b.install, b.checkout, b.seams) }()
	close(resume)
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if launches != 1 {
		t.Fatalf("launches=%d", launches)
	}
}
