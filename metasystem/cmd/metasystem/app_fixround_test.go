package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/applaunch"
)

// withEvidenceRoot names a durable evidence root in the bed's settings and
// commits it, so that a test moving between branches keeps it.
func (b *appBed) withEvidenceRoot() string {
	b.t.Helper()
	evidence := b.t.TempDir()
	conf := filepath.Join(b.installation, "metasystem.conf")
	body, err := os.ReadFile(conf)
	if err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(conf, append(body, []byte("evidence.root="+evidence+"\n")...), 0o644); err != nil {
		b.t.Fatal(err)
	}
	b.git("commit", "--quiet", "-m", "the evidence root", "--", conf)
	return evidence
}

func evidenceCopies(evidence, goal, ref string) []string {
	copies, _ := filepath.Glob(filepath.Join(evidence, "goals", goal, "app", applaunch.KeyFor(ref), "*", "_source-*-run.txt"))
	return copies
}

// --at goal/G is the same run as --goal G, so a stop through either spelling
// copies the goal run's evidence: the record's own goal decides.
func TestAppStopThroughTheAtSpellingCopiesTheGoalRunsEvidence(t *testing.T) {
	address := appFreePort(t)
	bed := newAppBed(t, appHTTPContract(appFixtureApp(t), address))
	evidence := bed.withEvidenceRoot()
	bed.git("branch", "goal/g1")
	t.Cleanup(func() { bed.run("app", "stop", "--goal", "g1", "--clean") })
	if code, out := bed.run("app", "start", "--goal", "g1"); code != 0 {
		t.Fatalf("app start --goal g1: %d\n%s", code, out)
	}
	code, out := bed.run("app", "stop", "--at", "goal/g1")
	if code != 0 || !strings.Contains(out, "copied to") {
		t.Fatalf("a stop through --at goal/g1 copies the goal run's evidence: %d\n%s", code, out)
	}
	if copies := evidenceCopies(evidence, "g1", "goal/g1"); len(copies) != 1 {
		t.Fatalf("one evidence copy under the goal, got %v\n%s", copies, out)
	}
}

// A run started through --at goal/G records the goal as --goal G does, so a
// stop through the same spelling copies the goal run's evidence.
func TestAppStartThroughTheAtSpellingRecordsTheGoal(t *testing.T) {
	address := appFreePort(t)
	bed := newAppBed(t, appHTTPContract(appFixtureApp(t), address))
	evidence := bed.withEvidenceRoot()
	bed.git("branch", "goal/g1")
	t.Cleanup(func() { bed.run("app", "stop", "--goal", "g1", "--clean") })
	if code, out := bed.run("app", "start", "--at", "goal/g1"); code != 0 {
		t.Fatalf("app start --at goal/g1: %d\n%s", code, out)
	}
	record, err := applaunch.ReadRecord(bed.installation, applaunch.KeyFor("goal/g1"))
	if err != nil || record.Goal != "g1" {
		t.Fatalf("a start through --at goal/g1 records the goal: %+v %v", record, err)
	}
	code, out := bed.run("app", "stop", "--at", "goal/g1")
	if code != 0 || !strings.Contains(out, "copied to") {
		t.Fatalf("a stop through --at goal/g1 copies the goal run's evidence: %d\n%s", code, out)
	}
	if copies := evidenceCopies(evidence, "g1", "goal/g1"); len(copies) != 1 {
		t.Fatalf("one evidence copy under the goal, got %v\n%s", copies, out)
	}
}

// A goal run whose evidence cannot be copied is not closed: with an invalid
// evidence root (a relative one; an unset root has a default) the process is
// ended, the record survives, and the next stop with a valid root closes it.
func TestAppGoalRunWithAnInvalidEvidenceRootIsNotClosed(t *testing.T) {
	address := appFreePort(t)
	bed := newAppBed(t, appHTTPContract(appFixtureApp(t), address))
	bed.git("branch", "goal/g1")
	t.Cleanup(func() { bed.run("app", "stop", "--goal", "g1", "--clean") })
	if code, out := bed.run("app", "start", "--goal", "g1"); code != 0 {
		t.Fatalf("app start --goal g1: %d\n%s", code, out)
	}
	key := applaunch.KeyFor("goal/g1")
	started, err := applaunch.ReadRecord(bed.installation, key)
	if err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(bed.installation, "metasystem.conf.local")
	if err := os.WriteFile(local, []byte("evidence.root=relative\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := bed.run("app", "stop", "--goal", "g1")
	if code == 0 || !strings.Contains(out, `evidence.root must be absolute (metasystem.conf.local reads "relative")`) || !strings.Contains(out, "stopped, but its run could not be closed") {
		t.Fatalf("a goal run with an invalid evidence root refuses closure by naming the setting: %d\n%s", code, out)
	}
	if err := os.Remove(local); err != nil {
		t.Fatal(err)
	}
	if answered(started.Address) {
		t.Fatalf("the process is ended even though the run is not closed:\n%s", out)
	}
	if record, err := applaunch.ReadRecord(bed.installation, key); err != nil || record.Ended == nil {
		t.Fatalf("the record survives, ended, for the retry: %+v %v", record, err)
	}
	if _, err := os.Stat(started.Tree); err != nil {
		t.Fatalf("the tree survives: %v", err)
	}
	evidence := bed.withEvidenceRoot()
	if code, out := bed.run("app", "stop", "--goal", "g1"); code != 0 || !strings.Contains(out, "copied to") {
		t.Fatalf("with the root configured the retry closes the run: %d\n%s", code, out)
	}
	if copies := evidenceCopies(evidence, "g1", "goal/g1"); len(copies) != 1 {
		t.Fatalf("the retry made the copy, got %v", copies)
	}
	if _, err := applaunch.ReadRecord(bed.installation, key); !os.IsNotExist(err) {
		t.Fatalf("the closed run's record is removed: %v", err)
	}
}
