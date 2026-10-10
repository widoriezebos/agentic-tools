package plain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGitAdapterSeatMergeWhitespaceAndMarkers(t *testing.T) {
	t.Parallel()
	for _, resolution := range []string{"main  \ngoal\n", "<<<<<<< main\n", "=======\n", ">>>>>>> goal\n", "||||||| base\n"} {
		t.Run(strings.TrimSpace(resolution), func(t *testing.T) {
			t.Parallel()
			b := newResolveGitAdapterFixture(t, false)
			b.contract.Generated, b.seams.AsSeat = nil, true
			if _, err := b.resolve(); err != nil {
				t.Fatal(err)
			}
			fix, err := ReadFix(b.install)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(b.install, "out/conflict"), []byte(resolution), 0o600); err != nil {
				t.Fatal(err)
			}
			b.mustGit("add", "metasystem/out/conflict")
			// The index, rather than a clean working copy, owns marker admission.
			if err := os.WriteFile(filepath.Join(b.install, "out/conflict"), []byte("main\ngoal\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			b.seams.Run = func(_ []string, _ string, _ *os.File, started func(int64) error) error {
				if err := started(0); err != nil {
					return err
				}
				if strings.HasPrefix(resolution, "main") {
					return os.WriteFile(filepath.Join(b.install, "out/conflict"), []byte(resolution), 0o600)
				}
				return nil
			}
			err = CompleteMerge(b.home, b.install, b.checkout, fix, "metasystem test impact", b.seams)
			if strings.HasPrefix(resolution, "main") {
				if err != nil || fix.State != "reviewing" {
					t.Fatalf("whitespace resolution did not commit: %+v %v", fix, err)
				}
				if got := b.mustGit("show", "HEAD:metasystem/out/conflict"); got != strings.TrimSuffix(resolution, "\n") {
					t.Fatalf("committed resolution=%q", got)
				}
			} else if err == nil || !strings.Contains(err.Error(), "conflict markers") || b.mustGit("rev-parse", "HEAD") != fix.Commit || b.mustGit("rev-parse", "MERGE_HEAD") != fix.Tip {
				t.Fatalf("staged marker admitted or merge changed: %+v %v", fix, err)
			}
		})
	}
}

func TestReadStatusAbandonedMergeYieldsToNewProof(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	fix := &Fix{Goal: "goal-a", Units: []string{"lane-merge-1"}, Commit: "old-batch", Tip: "old-tip", State: "abandoned", Attempt: "old-attempt", Reason: "the pending merge is gone; its resolution was abandoned"}
	if err := WriteFix(b.install, fix); err != nil {
		t.Fatal(err)
	}
	read := func(tip string) Status {
		return b.read(ProveSeams{Git: func(string, ...string) (string, error) { return tip, nil }, Alive: func(Running) bool { return true }}, b.git(nil, nil))
	}
	if status := read(fix.Commit); status.Summary != fix.Reason {
		t.Fatalf("current abandonment lost its reason: %+v", status)
	}
	if status := read("new-batch"); status.Summary == fix.Reason || status.ProofHeadline == fix.Reason || status.RunningFix != nil {
		t.Fatalf("stale abandonment replaced status: %+v", status)
	}
	for _, tip := range []string{fix.Commit, "new-batch"} {
		b.lines("running.json", Running{Commit: tip, Tree: "proof-tree", Attempt: "new-proof", Pid: 42, Since: time.Now().UTC().Format(time.RFC3339)})
		status := read(tip)
		if !strings.HasPrefix(status.ProofHeadline, "Proving "+tip+";") || status.Summary != status.ProofHeadline || status.RunningFix != nil {
			t.Fatalf("abandonment replaced new proof: %+v", status)
		}
	}
}

func TestSeatMergeStateErrorNamesState(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"before check", "during check"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			b := newResolveGitAdapterFixture(t, false)
			b.contract.Generated, b.seams.AsSeat = nil, true
			if _, err := b.resolve(); err != nil {
				t.Fatal(err)
			}
			fix, err := ReadFix(b.install)
			if err != nil {
				t.Fatal(err)
			}
			if phase == "before check" {
				fix.State = "returned"
			} else {
				if err := os.WriteFile(filepath.Join(b.install, "out/conflict"), []byte("main\ngoal\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				b.mustGit("add", "metasystem/out/conflict")
				b.seams.Run = func(_ []string, _ string, _ *os.File, started func(int64) error) error {
					if err := started(0); err != nil {
						return err
					}
					current := *fix
					current.State = "returned"
					return WriteFix(b.install, &current)
				}
			}
			err = CompleteMerge(b.home, b.install, b.checkout, fix, "metasystem test impact", b.seams)
			if err == nil || !strings.Contains(err.Error(), "returned") || !strings.Contains(err.Error(), "resolution") {
				t.Fatalf("state refusal has no explanation: %v", err)
			}
		})
	}
}

func TestReadStatusResolvedMergeNamesReadError(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	fix := &Fix{Goal: "goal-a", Units: []string{"lane-merge-1"}, Commit: "merge", Job: "build", State: "resolved", Attempt: "attempt", Reason: "the merge read could not complete: reader unavailable"}
	if err := WriteFix(b.install, fix); err != nil {
		t.Fatal(err)
	}
	status := b.read(ProveSeams{}, b.git(nil, nil))
	if status.RunningFix == nil || status.RunningFix.State != "resolved" || !strings.Contains(status.ProofHeadline, "read pending") || !strings.Contains(status.Summary, fix.Reason) {
		t.Fatalf("pending read or its error disappeared: %+v", status)
	}
	if err := proofFixReady(b.install, ProveSeams{}); err == nil || !strings.Contains(err.Error(), fix.Reason) {
		t.Fatalf("pending read admitted a proof or lost its reason: %v", err)
	}
}
