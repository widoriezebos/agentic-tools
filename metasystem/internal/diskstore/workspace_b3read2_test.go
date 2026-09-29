package diskstore

// Witnesses of the second B3 read (the reader's probes, kept as they
// failed on 118b71a4f).

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// R2-W1 (F2 variant): a release whose archive was cut short leaves an intact
// workspace in releasing; work written into it afterwards (uncommitted) is
// never judged: the next release removes it with --force.
func TestB3Read2ReleasingSkipsContentJudgement(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	ws := r.obtain("w", r.base)
	critical, err := r.registry.TryCritical(ws.Record.ID)
	if err != nil {
		t.Fatal(err)
	}
	rec := critical.Record()
	rec.State = StateReleasing
	if err := critical.Write(rec); err != nil {
		t.Fatal(err)
	}
	critical.Release()
	os.WriteFile(filepath.Join(ws.Record.Path, "new-work.go"), []byte("package x // uncommitted, 3 hours of edits\n"), 0o600)
	out := r.release(ws.Record.ID)
	t.Logf("outcome %+v", out)
	if _, err := os.Stat(filepath.Join(ws.Record.Path, "new-work.go")); err != nil {
		t.Fatalf("uncommitted file written into a releasing workspace removed without a keep: %v", err)
	}
}

// R2-W2 (F4 variant): the set is selected when all work landed; the agent
// then commits unlanded work on the workspace branch; the recorded set runs
// later (P6 / after the push / sweeper retry) and releases it without
// re-checking that its tips lie in the landed tip.
func TestB3Read2ReleaseSetRunDoesNotRecheckLanded(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	ws := r.obtain("s1", r.base)
	set, err := SelectReleaseSet(context.Background(), r.registry, r.repo, goalG.Ref, r.base, realWorkspaceGit)
	if err != nil || len(set.Stores) != 1 {
		t.Fatalf("selection: %+v %v", set, err)
	}
	r.git(ws.Record.Path, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "slice 2, not landed")
	RunReleaseSet(context.Background(), WorkspaceReleaseRequest{Registry: r.registry, GitRoot: r.repo, Git: realWorkspaceGit,
		Census: &UseCensus{Taken: true}, By: "landing", Now: testNow, IgnoredReleaseBytes: 1 << 20}, &set)
	t.Logf("set after run: %+v", set.Stores)
	if _, err := os.Stat(ws.Record.Path); err != nil {
		t.Fatalf("workspace with an unlanded commit released by the recorded set (%s)", set.Stores[0].State)
	}
}
