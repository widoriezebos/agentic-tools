package identity

import (
	"os/exec"
	"testing"
	"time"
)

func TestTaggedSurvivorsSeesARealTaggedProcess(t *testing.T) {
	// No tag recorded: nothing to scan for, no claim either way.
	if alive, certain := TaggedSurvivors("", 0); alive || !certain {
		t.Fatalf("an empty tag scans nothing: %v %v", alive, certain)
	}

	// A real child carrying the tag in its argv IS a survivor —
	// exactly the orphaned `util hold` shape the kill-less reapers
	// must defer on.
	tag := "metasystem-survivor-scan-fixture"
	child := exec.Command("/bin/sh", "-c", "sleep 30", tag)
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = child.Process.Kill()
		_, _ = child.Process.Wait()
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		alive, certain := TaggedSurvivors(tag, 0)
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
	if alive, certain := TaggedSurvivors(tag, int64(child.Process.Pid)); alive || !certain {
		t.Fatalf("the custodian pid is not its own survivor: %v %v", alive, certain)
	}

	// With the child gone the scan comes back clear and the verdict
	// may conclude.
	_ = child.Process.Kill()
	_, _ = child.Process.Wait()
	deadline = time.Now().Add(5 * time.Second)
	for {
		alive, certain := TaggedSurvivors(tag, 0)
		if !alive && certain {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("a dead group scans clear: %v %v", alive, certain)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
