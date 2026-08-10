package gaterun

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRegisterThenRunningIsTrueForLiveGate(t *testing.T) {
	root := t.TempDir()
	path, err := Register(root, int64(os.Getpid()), "go-gate")
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("registering a live process should write a marker")
	}
	if !Running(root) {
		t.Fatal("a live gate marker should read as running")
	}
}

func TestRunningPrunesDeadMarker(t *testing.T) {
	root := t.TempDir()
	// A dead pid: spawn, reap, then hand-write a marker for it.
	cmd := exec.Command("/bin/sleep", "1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadPid := cmd.Process.Pid
	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	dir := markerDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "dead.json")
	if err := os.WriteFile(marker, []byte(`{"pid":`+itoa(deadPid)+`,"pidStartedAt":111}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if Running(root) {
		t.Fatal("a dead gate marker must not read as running")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("a dead gate marker should be pruned")
	}
}

func TestRunningDropsUnparsableMarker(t *testing.T) {
	root := t.TempDir()
	dir := markerDir(root)
	_ = os.MkdirAll(dir, 0o755)
	marker := filepath.Join(dir, "junk.json")
	_ = os.WriteFile(marker, []byte(`{not json`), 0o644)
	if Running(root) {
		t.Fatal("an unparsable marker is not a running gate")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("an unparsable marker should be pruned")
	}
}

func TestRegisterSkipsDeadProcess(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command("/bin/sleep", "1")
	_ = cmd.Start()
	deadPid := int64(cmd.Process.Pid)
	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	path, err := Register(root, deadPid, "go-gate")
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Fatal("a dead process has no verifiable start, so no marker should be written")
	}
	if Running(root) {
		t.Fatal("nothing was registered, so nothing runs")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
