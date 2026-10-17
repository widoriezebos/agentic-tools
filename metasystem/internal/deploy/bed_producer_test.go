package deploy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

type heldAwaitRecorder struct {
	t        *testing.T
	failures []string
}

func (*heldAwaitRecorder) Helper() {}
func (r *heldAwaitRecorder) Fatalf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}
func (r *heldAwaitRecorder) Deadline() (time.Time, bool) { return r.t.Deadline() }

func TestAwaitHeldFailsAtOnceWhenTheAdapterIsGone(t *testing.T) {
	t.Parallel()
	child := exec.Command("/bin/sh", "-c", "exit 0")
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	pid := child.Process.Pid
	if err := child.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := testenv.AwaitProcessTargetGone(-pid); err != nil {
		t.Fatal(err)
	}
	b := newBed(t)
	b.fifo("stalled")
	b.fifo("verify")
	if err := os.WriteFile(filepath.Join(b.state, "calls"), []byte("build c2\nversion c1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	recorder := &heldAwaitRecorder{t: t}
	b.awaitHeld(recorder, pid)
	if len(recorder.failures) != 1 {
		t.Fatalf("failures=%q, want exactly one", recorder.failures)
	}
	for _, want := range []string{"its producer is gone", fmt.Sprintf("process group %d has no member", pid), "stalled not held", "verify not held", "build c2", "version c1"} {
		if !strings.Contains(recorder.failures[0], want) {
			t.Errorf("failure=%q, want %q", recorder.failures[0], want)
		}
	}
}
