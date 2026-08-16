package missionrunner

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"golang.org/x/sys/unix"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The host process lifecycle, driven with real processes (Phase 3b's
// coverage hardening: these paths run under every mission and had none).

func TestStartProcessLifecycle(t *testing.T) {
	process, err := startProcess(exec.Command("sleep", "0.1"))
	if err != nil {
		t.Fatal(err)
	}
	if process.exited() {
		t.Fatal("reported exited while sleeping")
	}
	if !process.waitFor(5 * time.Second) {
		t.Fatal("never reaped")
	}
	if !process.exited() {
		t.Fatal("reaped but not exited")
	}
	if code := process.exitCode(); code != 0 {
		t.Fatalf("clean exit read as %d", code)
	}
}

func TestWaitForBoundsItsWait(t *testing.T) {
	process, err := startProcess(exec.Command("sleep", "30"))
	if err != nil {
		t.Fatal(err)
	}
	defer process.cmd.Process.Kill()
	started := time.Now()
	if process.waitFor(200 * time.Millisecond) {
		t.Fatal("a running child reported reaped")
	}
	if time.Since(started) > 5*time.Second {
		t.Fatal("the bound did not release the caller")
	}
}

func TestExitCodeShapes(t *testing.T) {
	// A failing child reports its code.
	process, _ := startProcess(exec.Command("false"))
	process.waitFor(5 * time.Second)
	if code := process.exitCode(); code != 1 {
		t.Fatalf("false exited %d", code)
	}
	// A signaled child reads as -1, the plain-failure convention.
	process, _ = startProcess(exec.Command("sleep", "30"))
	process.cmd.Process.Signal(syscall.SIGKILL)
	process.waitFor(5 * time.Second)
	if code := process.exitCode(); code != -1 {
		t.Fatalf("a signaled child read as %d", code)
	}
	// exitCode before the reap is not exercised: the type's contract is
	// waitFor-then-exitCode (every production caller does), and reading
	// ProcessState while the reaper goroutine may write it is a data race
	// the race detector rightly rejects.
}

func TestHostStartVerifiedMatrix(t *testing.T) {
	if !hostStartVerified(100, 100, "metasystem host --tag mr-x1", "mr-x1", false) {
		t.Fatal("a group-leading tagged host must verify")
	}
	if hostStartVerified(100, 99, "metasystem host --tag mr-x1", "mr-x1", false) {
		t.Fatal("a non-leader must not verify")
	}
	if hostStartVerified(100, 100, "unrelated command", "mr-x1", false) {
		t.Fatal("an untagged command must not verify")
	}
	if hostStartVerified(100, 100, "metasystem host --tag mr-x1", "mr-x1", true) {
		t.Fatal("the fixture force-unverified path must refuse")
	}
}

// The group probes against real processes: our own group is alive; a
// pgid that cannot exist is not; ownership needs the tag on a live member.
func TestGroupProbes(t *testing.T) {
	self, err := syscall.Getpgid(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if !groupAlive(self) {
		t.Fatal("our own group read dead")
	}
	if groupAlive(0) || groupAlive(-1) {
		t.Fatal("a nonsense pgid read alive")
	}
	if groupOwned(self, "tag-that-cannot-appear-77aa", fixtureauth.GroupOwnershipGrant{}) {
		t.Fatal("ownership proven by a tag no member carries")
	}
}

// TestAssembleHostCommandExportsMissionLineage pins the succession wiring:
// every turn's host process must inherit METASYSTEM_OWNER_LINEAGE derived
// from the mission id, or each turn's host takes the lease from its own
// dead predecessor and sweeps the previous turn's delegates (the loop that
// cost bm-2 two of three delegates). This replaces the shell fixture's
// grep of host.go source text (script-fixtures-019).
func TestAssembleHostCommandExportsMissionLineage(t *testing.T) {
	root := t.TempDir()
	engine := &Engine{Mission: "mr-lineage", Root: root}
	adapterDir := filepath.Join(root, "scripts", "agents", "hosts")
	if err := os.MkdirAll(adapterDir, 0o755); err != nil {
		t.Fatal(err)
	}
	adapter := filepath.Join(adapterDir, "fake.sh")
	if err := os.WriteFile(adapter, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	launch := &hostLaunch{
		turnID:    "turn-1",
		turnDir:   t.TempDir(),
		turn:      map[string]any{"runtime": "fake"},
		leasePath: filepath.Join(root, "lease.json"),
	}
	if err := engine.assembleHostCommand(launch); err != nil {
		t.Fatalf("assemble: %v", err)
	}
	want := "METASYSTEM_OWNER_LINEAGE=" + MissionLineage("mr-lineage")
	for _, entry := range launch.command.Env {
		if entry == want {
			return
		}
	}
	t.Fatalf("host environment must carry %s, got:\n%s", want, strings.Join(launch.command.Env, "\n"))
}

// terminateGroup's three verdicts: a dead group is a no-op; an unowned live
// group is skipped with the census left to catch strays; an owned group is
// signaled and reaped within the grace window.
func TestTerminateGroup(t *testing.T) {
	engine := &Engine{Mission: "mr-test", Root: t.TempDir()}

	// Dead group: nothing to do, no error.
	if err := engine.terminateGroup(1<<28, "any", false); err != nil {
		t.Fatalf("a dead group errored: %v", err)
	}

	// Live but unowned: never signaled. The child survives the call.
	unowned := exec.Command("sleep", "5")
	unowned.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := unowned.Start(); err != nil {
		t.Fatal(err)
	}
	defer unowned.Process.Kill()
	defer unowned.Wait()
	pgid, _ := syscall.Getpgid(unowned.Process.Pid)
	if err := engine.terminateGroup(pgid, "tag-no-member-carries", false); err != nil {
		t.Fatalf("skipping an unowned group errored: %v", err)
	}
	if err := unowned.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatal("an unowned group was signaled")
	}

	// Owned: the tag is on the live member's command line, so the group is
	// wound down within the grace window.
	tag := "mr-owned-4c1d"
	owned := exec.Command("bash", "-c", "exec -a sleep-"+tag+" sleep 30")
	owned.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := owned.Start(); err != nil {
		t.Fatal(err)
	}
	// Reap concurrently: a deferred Wait would hold the child as a zombie,
	// and a zombie keeps its group technically alive after the wind-down's
	// SIGTERM — the group then reads alive with its ownership proof gone,
	// which is the test harness's artifact, not the wind-down's failure.
	go owned.Wait()
	ownedPgid, _ := syscall.Getpgid(owned.Process.Pid)
	// The child execs twice (bash, then the tagged sleep). Ownership is
	// briefly provable through bash's transitional argv — which still
	// carries the tag — and terminateGroup's own re-check can then land in
	// the inner execve window (the flake dossier's mechanism) and rightly
	// skip the signal. Wait for the proof in its final, stable form: a
	// member whose argv[0] IS the tagged name, after which no exec
	// transition remains.
	stablyOwned := func() bool {
		pids, err := identity.AllPids()
		if err != nil {
			return false
		}
		for _, pid := range pids {
			if memberGroup, err := syscall.Getpgid(int(pid)); err != nil || memberGroup != ownedPgid {
				continue
			}
			exact, state, err := identity.KernelProber{}.Probe(pid)
			if err != nil || state != identity.Alive || len(exact.Argv) == 0 {
				continue
			}
			if filepath.Base(exact.Argv[0]) == "sleep-"+tag {
				return true
			}
		}
		return false
	}
	stableBy := time.Now().Add(2 * time.Second)
	for !stablyOwned() {
		if time.Now().After(stableBy) {
			owned.Process.Kill()
			t.Skip("argv tagging not visible on this host; the owned path needs it")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := engine.terminateGroup(ownedPgid, tag, false); err != nil {
		t.Fatalf("terminating an owned group errored: %v", err)
	}
	if groupAlive(ownedPgid) {
		owned.Process.Kill()
		t.Fatal("the owned group survived its wind-down")
	}
}

func TestSmallPureHelpers(t *testing.T) {
	if scaledDuration(3) != 3*time.Second {
		t.Fatal("scaledDuration")
	}
	env := gitAuthorEnvironment("mission-alpha")
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "GIT_AUTHOR_NAME=mission-alpha") ||
		!strings.Contains(joined, "GIT_AUTHOR_EMAIL=mission-alpha@metasystem.invalid") {
		t.Fatalf("author environment wrong: %v", env[len(env)-2:])
	}
	if firstDetail(" stderr wins \n", "stdout") != "stderr wins" {
		t.Fatal("firstDetail stderr")
	}
	if firstDetail("  \n", " stdout speaks ") != "stdout speaks" {
		t.Fatal("firstDetail stdout fallback")
	}
	first, second := randomHex(8), randomHex(8)
	if len(first) != 16 || first == second {
		t.Fatal("randomHex shape or uniqueness")
	}
}

// B1 critique finding 15: denied capabilities leave OBSERVABLE
// nothing — a refused publication writes no file, and a refused group
// grant never proves ownership of a live group.
func TestDeniedCapabilitiesActNowhere(t *testing.T) {
	table := filepath.Join(t.TempDir(), "table.json")
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)
	if err := publishFakeIdentity(1234, 100, 1234, "t", fixtureauth.PublicationGrant{}); err != nil {
		t.Fatalf("an unauthorized publication must be a silent no-op for the env-absent CALLER shape: %v", err)
	}
	if _, statErr := os.Stat(table); statErr == nil {
		t.Fatal("a denied publication wrote the fixture file")
	}
	self, err := unix.Getpgid(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if groupOwned(self, "any-tag", fixtureauth.GroupOwnershipGrant{}) {
		t.Fatal("a zero grant proved ownership of a live group")
	}
}

// Construction refusal PROPAGATES on the engine's fixture paths
// (finding 4): a leaked fixture makes publication and group
// termination errors, and the command probe refuses.
func TestEngineFixtureConstructionRefusal(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644)
	table := filepath.Join(t.TempDir(), "table.json")
	os.WriteFile(table, []byte(`{}`), 0o644)
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)
	engine := &Engine{Root: root}
	if err := publishFakeIdentityForEngine(engine, 1234, 100, "t"); err == nil {
		t.Fatal("a leaked fixture did not refuse publication")
	}
	if probe := hostCommandProbe(engine, true); func() bool { _, ok := probe.FixtureCommand(1234); return ok }() {
		t.Fatal("a leaked fixture served a command probe")
	}
	self, err := unix.Getpgid(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.terminateGroup(self, "any-tag", true); err == nil {
		t.Fatal("a leaked fixture did not refuse the fake-mode terminate path")
	}
	// The custodian degrades to Unknown, which authorizes nothing.
	if verdict := engine.custodian(int64(os.Getpid()), 1, "t"); verdict != identity.Unknown {
		t.Fatalf("leaked-fixture custodian verdict: %v", verdict)
	}
}
