package missionrunner

import (
	"os"
	"os/exec"
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
	command := exec.Command("sh", "-c", "yes >/dev/null & sleep 3; kill $!")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
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
