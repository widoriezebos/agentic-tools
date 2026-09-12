package missionrunner

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestParsePSCPUTime(t *testing.T) {
	for value, want := range map[string]float64{"0:00.12": 0.12, "1:02.50": 62.5, "1:01:01": 3661, "2-01:00:00": 176400} {
		got, err := parsePSCPUTime(value)
		if err != nil || got != want {
			t.Fatalf("%s: got %v err=%v want %v", value, got, err, want)
		}
	}
	for _, value := range []string{"", "12", "a:b", "1:-2"} {
		if _, err := parsePSCPUTime(value); err == nil {
			t.Fatalf("%q parsed", value)
		}
	}
}

func TestProcessTreeCPUSecondsCountsDescendants(t *testing.T) {
	// A busy child under a shell: the tree's CPU time grows while the shell
	// itself sits in wait, so descendant time must be part of the sum.
	// The shell and its burner share a process group of their own, and the
	// group is what ends. A bare Process.Kill ended only the shell, before
	// its own `kill $!` ran, and every run of this test left a `yes` at a
	// full core behind it: ten of them burned through the night of
	// 2026-09-11 under every cadence measurement (found 2026-09-12 08:40,
	// parent launchd, start times matching the runs).
	command := exec.Command("sh", "-c", "yes >/dev/null & sleep 3; kill $!")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	group := command.Process.Pid
	t.Cleanup(func() {
		_ = syscall.Kill(-group, syscall.SIGKILL)
		_ = command.Wait()
		if err := syscall.Kill(-group, 0); err == nil {
			t.Errorf("the burner outlived the test in process group %d", group)
		}
	})
	first, ok := processTreeCPUSeconds(command.Process.Pid)
	if !ok {
		t.Skip("platform process reader is unavailable on this test host")
	}
	time.Sleep(1500 * time.Millisecond)
	second, ok := processTreeCPUSeconds(command.Process.Pid)
	if !ok || second <= first {
		t.Fatalf("descendant CPU did not grow: first=%v second=%v ok=%v", first, second, ok)
	}
	progress := newTreeCPUProgress(command.Process.Pid)
	if progress.advanced(time.Now()) {
		t.Fatal("the first sample has nothing to compare with")
	}
	time.Sleep(1100 * time.Millisecond)
	if !progress.advanced(time.Now()) {
		t.Fatal("a growing tree must count as progress")
	}
	if _, ok := processTreeCPUSeconds(os.Getpid() + 1_000_000); ok {
		t.Fatal("an absent process must report no sample")
	}
}
