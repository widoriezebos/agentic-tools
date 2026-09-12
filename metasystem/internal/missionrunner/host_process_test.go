package missionrunner

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/janitor"
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
	if !process.waitFor(wiringBound) {
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
	if process.exited() {
		t.Fatal("reported exited while sleeping")
	}
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
	process.waitFor(wiringBound)
	if code := process.exitCode(); code != 1 {
		t.Fatalf("false exited %d", code)
	}
	// A signaled child reads as -1, the plain-failure convention.
	process, _ = startProcess(exec.Command("sleep", "30"))
	process.cmd.Process.Signal(syscall.SIGKILL)
	process.waitFor(wiringBound)
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
	if groupOwnership(self, "tag-that-cannot-appear-77aa", fixtureauth.GroupOwnershipGrant{}) == janitor.GroupOwned {
		t.Fatal("ownership proven by a tag no member carries")
	}
}

// TestAssembleHostCommandExportsMissionLineage pins the succession wiring:
// every turn's host process must inherit METASYSTEM_OWNER_LINEAGE derived
// from the mission id, or each turn's host takes the lease from its own
// dead predecessor and sweeps the previous turn's delegates.
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
	// The sealed-cap pass-through reads the PINNED approved snapshot and
	// fails the launch without one: the fixture pins a minimal
	// sealed contract exactly as the launcher does.
	missionDir := filepath.Join(root, "artifacts", "agents", "missions", "mr-lineage")
	if err := os.MkdirAll(missionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	contract := "```mission\ncandidate.branch=main\nstream.alpha=Do alpha\n```\n\n```mission-seal\nsealed.baseline.score=0\n```\n"
	if err := os.WriteFile(filepath.Join(missionDir, "mission-mr-lineage.contract.md"), []byte(contract), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(contract))
	writeJSONFile(t, engine.fencesPath(), map[string]any{
		"schemaVersion": 1, "missionId": "mr-lineage", "startedAt": "2026-08-18T00:00:00Z",
		"cycles": 0, "reservations": map[string]any{},
		"approvedContractSha256": hex.EncodeToString(sum[:]),
	})
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

func TestSmallPureHelpers(t *testing.T) {
	t.Setenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI", "1000")
	if w, err := ScaledWait(3); err != nil || w != 3*time.Second {
		t.Fatalf("ScaledWait at explicit default scale: %v %v", w, err)
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

// Denied capabilities leave OBSERVABLE
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
	if groupOwnership(self, "any-tag", fixtureauth.GroupOwnershipGrant{}) == janitor.GroupOwned {
		t.Fatal("a zero grant proved ownership of a live group")
	}
}

// Construction refusal PROPAGATES on the engine's fixture
// paths: a leaked fixture makes publication and group
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
	if _, err := engine.terminateGroup(self, "any-tag", true); err == nil {
		t.Fatal("a leaked fixture did not refuse the fake-mode terminate path")
	}
	// The custodian degrades to Unknown, which authorizes nothing.
	if verdict := engine.custodian(int64(os.Getpid()), 1, "t"); verdict != identity.Unknown {
		t.Fatalf("leaked-fixture custodian verdict: %v", verdict)
	}
}
