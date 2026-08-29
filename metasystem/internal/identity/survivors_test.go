package identity

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestTaggedSurvivorsSeesARealTaggedProcess(t *testing.T) {
	// No tag recorded: nothing to scan for, no claim either way.
	if alive, certain := TaggedSurvivors("", 0, 0); alive || !certain {
		t.Fatalf("an empty tag scans nothing: %v %v", alive, certain)
	}

	// A real child carrying the tag in its argv IS a survivor —
	// exactly the orphaned `util hold` shape the kill-less reapers
	// must defer on. The tag rides argv[0] via a symlink: a shell
	// wrapper's trailing argument is NOT durable (bash exec-optimizes
	// `-c 'sleep 30'` into a bare sleep and the tag vanishes — this
	// leg went red exactly that way inside an adopted target's gate).
	tag := "metasystem-survivor-scan-fixture"
	link := filepath.Join(t.TempDir(), tag)
	if err := os.Symlink("/bin/sleep", link); err != nil {
		t.Fatal(err)
	}
	child := exec.Command(link, "30")
	// Its own process group: the indeterminacy scope the reapers
	// pass is the recorded group, and the test's assertions must not
	// depend on whatever else this machine runs under go test's
	// group.
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	childGroup := int64(child.Process.Pid)
	if _, err := AllPids(); errors.Is(err, syscall.EPERM) {
		prior := survivorPids
		survivorPids = func() ([]int64, error) { return []int64{childGroup}, nil }
		t.Cleanup(func() { survivorPids = prior })
	}
	defer func() {
		_ = child.Process.Kill()
		_, _ = child.Process.Wait()
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		alive, certain := TaggedSurvivors(tag, 0, childGroup)
		if alive && certain {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the live tagged child is a survivor: %v %v", alive, certain)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// The recorded custodian itself is excluded: a scan that counted
	// the dead-but-probed custodian would defer forever.
	if alive, certain := TaggedSurvivors(tag, int64(child.Process.Pid), childGroup); alive || !certain {
		t.Fatalf("the custodian pid is not its own survivor: %v %v", alive, certain)
	}

	// With the child gone the scan comes back clear and the verdict
	// may conclude.
	_ = child.Process.Kill()
	_, _ = child.Process.Wait()
	deadline = time.Now().Add(5 * time.Second)
	for {
		alive, certain := TaggedSurvivors(tag, 0, childGroup)
		if !alive && certain {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("a dead group scans clear: %v %v", alive, certain)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
