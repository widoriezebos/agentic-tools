package plain

import (
	"errors"
	"io"
	"reflect"
	"testing"
	"time"
)

func TestProofCauseRecordsExistingDecisions(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"incomplete", "not-run", "killed", "no worktree", "complete unknown", "flake record failed"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			command, kind := "exit 1", "unclassified"
			switch name {
			case "not-run":
				command, kind = "printf 'LANDING-NOT-RUN\\tbusy\\n'; exit 1", "environment"
			case "killed":
				command, kind = "kill -KILL $$", "environment"
			case "no worktree":
				b.seams.Git, kind = (stubGit{commit: "commit", tree: "tree", addErr: errors.New("cannot create worktree")}).run, "environment"
			case "complete unknown":
				// The classifier proves an unknown red on main by replay.
				command, b.seams.Judge, kind = failedReport, nil, "main"
			case "flake record failed":
				command = "if [ -z \"$LANDING_ONLY\" ]; then " + failedReport + "; fi; printf 'LANDING-CHECKED\\t0\\n'"
				b.seams.RecordFlake, kind = nil, "flake"
			}
			proof := b.run(command)
			if proof.Result != Red || proof.Cause == nil || proof.Cause.Kind != kind || proof.Cause.Evidence == "" {
				t.Fatalf("missing cause: %+v", proof)
			}
			if kind == "environment" && proof.Cause.Name != "lost-process" {
				t.Fatalf("environment kind lost: %+v", proof.Cause)
			}
			if name == "complete unknown" && !reflect.DeepEqual(proof.Cause.Tests, []string{"u/a TestA", "u/a TestB", "u/b TestC"}) {
				t.Fatalf("failed tests lost: %+v", proof.Cause)
			}
			for _, line := range b.lines() {
				if line.Result == Red && line.Cause == nil {
					t.Fatalf("red record lacks cause: %+v", line)
				}
			}
		})
	}
}

func TestRepeatedLegacyRedGainsCause(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	legacy := Result{Tree: "tree", Commit: "commit", Result: Red, Repeat: "allowed", Log: "legacy.log", Failed: []FailedUnit{{Unit: "u/a", Tests: []string{"TestA"}}}}
	if err := withLock(b.install, func() error { return appendLine(resultsPath(b.install), legacy) }); err != nil {
		t.Fatal(err)
	}
	b.seams.RecordFlake = nil
	if _, err := Run(b.install, b.checkout, "exit 1", "", io.Discard, b.seams); err != nil {
		t.Fatal(err)
	}
	lines := b.lines()
	if len(lines) < 2 || lines[1].Cause == nil || lines[1].Cause.Kind != "unclassified" || lines[1].Cause.Evidence != "legacy.log" || !reflect.DeepEqual(lines[1].Cause.Tests, []string{"u/a TestA"}) {
		t.Fatalf("repeated legacy red lacks cause: %+v", lines)
	}
}

func TestDeadProofRecordsEnvironmentCause(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	if err := withLock(b.install, func() error {
		return writeRunning(b.install, Running{Attempt: "dead", Commit: "commit", Tree: "tree", Log: "dead.log"})
	}); err != nil {
		t.Fatal(err)
	}
	b.seams.Alive = func(Running) bool { return false }
	if _, _, _, err := checkState(b.install, b.checkout, "", b.seams); err != nil {
		t.Fatal(err)
	}
	proof, ok, err := LastResult(b.install)
	if err != nil || !ok || proof.Cause == nil || proof.Cause.Kind != "environment" || proof.Cause.Name != "lost-process" || proof.Cause.Evidence != "dead.log" || proof.Repeat != "allowed" {
		t.Fatalf("dead proof: %+v %v", proof, err)
	}
}

func TestDesignRefusalReturnsOnlyCheckedHandIn(t *testing.T) {
	t.Parallel()
	for _, newer := range []bool{false, true} {
		t.Run(map[bool]string{false: "checked", true: "newer"}[newer], func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			if _, _, err := HandIn(install, Line{Goal: "goal", SHA: "checked"}); err != nil {
				t.Fatal(err)
			}
			if newer {
				if _, _, err := HandIn(install, Line{Goal: "goal", SHA: "newer"}); err != nil {
					t.Fatal(err)
				}
			}
			check := DesignCheck{Goal: "goal", Commit: "checked", Reason: "accepted design changed"}
			_, changed, err := ReturnDesignRefused(install, check, time.Now())
			entry, ok, readErr := Latest(install, "goal")
			if readErr != nil || !ok {
				t.Fatalf("return cannot be read: %+v ok=%v err=%v", entry, ok, readErr)
			}
			if newer {
				if !errors.Is(err, ErrNotWaiting) || changed || entry.State != StateWaiting || entry.SHA != "newer" {
					t.Fatalf("stale design refusal changed a newer hand-in: %+v changed=%v err=%v", entry, changed, err)
				}
				return
			}
			if err != nil || !changed || entry.State != StateReturned || entry.Cause == nil || !reflect.DeepEqual(entry.Cause, &Cause{Kind: "own", Goal: "goal", SHA: "checked", Evidence: check.Reason}) {
				t.Fatalf("design refusal: %+v changed=%v err=%v", entry, changed, err)
			}
			before, err := Entries(install)
			if err != nil {
				t.Fatal(err)
			}
			_, changed, err = ReturnDesignRefused(install, check, time.Now())
			after, readErr := Entries(install)
			if err != nil || changed || readErr != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("repeat return changed the queue: before=%+v after=%+v changed=%v err=%v readErr=%v", before, after, changed, err, readErr)
			}
		})
	}
}
