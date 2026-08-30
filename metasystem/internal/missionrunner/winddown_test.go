package missionrunner

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
)

// spawnTaggedGroup starts a real process group whose leader's argv carries
// the tag in the janitor's tagged-hold positional shape. termImmune leaders
// ignore SIGTERM so only the kill-through path can end them.
func spawnTaggedGroup(t *testing.T, tag string, termImmune bool) *exec.Cmd {
	t.Helper()
	script := "sleep 30"
	if termImmune {
		script = `trap "" TERM; sleep 30`
	}
	cmd := exec.Command("bash", "-c", script, "metasystem", "util", "hold", "--tag", tag)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Reap concurrently: an unreaped leader is a zombie that keeps its
	// group technically alive after the kill, which is this harness's
	// artifact, not the wind-down's failure (the same rule the older
	// TestTerminateGroup records).
	go func() { _ = cmd.Wait() }()
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	})
	return cmd
}

func waitGroupDead(pgid int, patience time.Duration) bool {
	deadline := time.Now().Add(patience)
	for time.Now().Before(deadline) {
		if !groupAlive(pgid) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return !groupAlive(pgid)
}

func TestTerminateGroupKillsThroughATermImmuneOwnedGroup(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "mr-winddown"}
	tag := fmt.Sprintf("metasystem-job-winddown-%d", os.Getpid())
	cmd := spawnTaggedGroup(t, tag, true)
	pgid := cmd.Process.Pid
	if err := engine.terminateGroup(pgid, tag, false); err != nil {
		t.Fatalf("kill-through wind-down failed: %v", err)
	}
	if !waitGroupDead(pgid, 3*time.Second) {
		t.Fatal("a TERM-immune owned group must die through the SIGKILL path")
	}
}

func TestTerminateGroupNeverSignalsAForeignGroup(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "mr-winddown"}
	foreign := exec.Command("sleep", "30")
	foreign.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := foreign.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-foreign.Process.Pid, syscall.SIGKILL)
		_, _ = foreign.Process.Wait()
	})
	tag := fmt.Sprintf("metasystem-job-foreign-%d", os.Getpid())
	if err := engine.terminateGroup(foreign.Process.Pid, tag, false); err != nil {
		t.Fatalf("foreign wind-down must skip without error: %v", err)
	}
	if !groupAlive(foreign.Process.Pid) {
		t.Fatal("a group without the positioned tag was signaled")
	}
}

func TestTerminateGroupLeaksNoGroupsUnderCompression(t *testing.T) {
	// The slice-2 accounting: repeated wind-downs under an aggressive
	// compression scale must abandon ZERO groups — the real-fact grace
	// floor holds even when scaled waits shrink to their 10ms floor.
	t.Setenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI", "20") // scale 50
	engine := &Engine{Root: t.TempDir(), Mission: "mr-winddown-scale"}
	leaked := 0
	for cycle := 0; cycle < 4; cycle++ {
		tag := fmt.Sprintf("metasystem-job-scale-%d-%d", os.Getpid(), cycle)
		cmd := spawnTaggedGroup(t, tag, cycle%2 == 1)
		pgid := cmd.Process.Pid
		if err := engine.terminateGroup(pgid, tag, false); err != nil {
			leaked++
			continue
		}
		if !waitGroupDead(pgid, 3*time.Second) {
			leaked++
		}
	}
	if leaked != 0 {
		t.Fatalf("compressed wind-down abandoned %d of 4 groups", leaked)
	}
}

func TestGroupOwnershipFixtureFallbackStaysExact(t *testing.T) {
	// A zero grant must refuse the fixture path entirely; the kernel
	// tri-state alone decides.
	var grant fixtureauth.GroupOwnershipGrant
	if got := groupOwnership(1, "metasystem-job-x", grant); got == "OWNED" {
		t.Fatalf("pgid 1 with a zero grant = %s", got)
	}
}
