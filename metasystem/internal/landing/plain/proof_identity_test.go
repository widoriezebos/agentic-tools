package plain

import (
	"os"
	"testing"
)

func TestDeadProofReplayCannotKeepItsRunningRecord(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	running := Running{Attempt: "dead", Commit: "commit", Tree: "tree", Log: "dead.log"}
	if err := withLock(b.install, func() error { return writeRunning(b.install, running) }); err != nil {
		t.Fatal(err)
	}
	b.seams.Alive = func(Running) bool { return false }
	var recorded, alive bool
	err := withLock(b.install, func() error {
		var err error
		_, recorded, alive, err = checkState(b.install, b.checkout, running.Attempt, b.seams)
		return err
	})
	if err != nil || recorded || alive {
		t.Fatalf("replayed dead proof remains running: recorded=%v alive=%v err=%v", recorded, alive, err)
	}
	last, found, err := LastResult(b.install)
	if err != nil || !found || last.Result != Red || last.Attempt != running.Attempt || last.Cause == nil || last.Cause.Name != "lost-process" {
		t.Fatalf("dead proof lacks its lost-process result: %+v found=%v err=%v", last, found, err)
	}
}

func TestRunningOwnProcessRequiresPIDAndExactIdentity(t *testing.T) {
	t.Parallel()
	pid := int64(os.Getpid())
	ref := processRef(pid)
	if ref == "" {
		t.Fatal("cannot read this process's identity")
	}
	for _, tc := range []struct {
		name    string
		running Running
		want    bool
	}{
		{"own process", Running{Pid: pid, Process: ref}, true},
		{"other pid", Running{Pid: 0, Process: ref}, false},
		{"other identity", Running{Pid: pid, Process: "another-process"}, false},
		{"missing identity", Running{Pid: pid}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.running.OwnProcess(); got != tc.want {
				t.Fatalf("OwnProcess() = %v, want %v", got, tc.want)
			}
		})
	}
}
