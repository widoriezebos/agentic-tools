package census

import (
	"encoding/json"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"os"
	"path/filepath"
	"testing"
)

// TestAgentAncestorWireShape pins `proc find-ancestor`'s JSON bytes to what
// the map[string]any form produced: sorted keys. The struct's field order IS
// the wire order.
func TestAgentAncestorWireShape(t *testing.T) {
	got, err := json.Marshal(AgentAncestor{Argv: "claude -p", Pgid: 2, Pid: 3, PidStartedAt: 4, Runtime: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"argv":"claude -p","pgid":2,"pid":3,"pidStartedAt":4,"runtime":"claude"}`
	if string(got) != want {
		t.Fatalf("wire shape changed:\n got %s\nwant %s", got, want)
	}
}

func TestAuthIdentityFixtureFile(t *testing.T) {
	dir := t.TempDir()
	idFile := filepath.Join(dir, "id.json")
	os.WriteFile(idFile, []byte(`{"42":{"started":100,"command":"claude serve"}}`), 0o644)
	os.WriteFile(filepath.Join(dir, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644)
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", idFile)
	authorization, err := fixtureauth.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	probe := authorization.Identity()
	id, err := AuthIdentity(42, probe)
	if err != nil {
		t.Fatal(err)
	}
	if id.PidStartedAt != 100 || id.Command != "claude serve" {
		t.Fatalf("wrong identity: %+v", id)
	}
	// A pid absent from the fixture falls through to ps (and fails for a
	// non-existent pid).
	if _, err := AuthIdentity(999999, probe); err == nil {
		t.Fatal("a non-existent pid must error")
	}
	// The authorization REFUSES construction outside a fake checkout —
	// the leaked-fixture fence at every entry point (agnosticism B1).
	os.WriteFile(filepath.Join(dir, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644)
	if _, err := fixtureauth.New(dir); err == nil {
		t.Fatal("a leaked fixture in a non-fake checkout was authorized")
	}
}

func TestSignatureCheckContract(t *testing.T) {
	dir := t.TempDir()
	adapter := filepath.Join(dir, "fake.sh")
	os.WriteFile(adapter, []byte("#!/bin/sh\nprintf 'match (^|[[:space:]/-])metasystem-fake-agent([[:space:]]|$)\\nexclude fake\\.sh\\n'\n"), 0o755)
	// positive classifies, lookalike does not: contract holds.
	if err := SignatureCheck(adapter, "metasystem-fake-agent job", "unrelated proc"); err != nil {
		t.Fatalf("valid contract rejected: %v", err)
	}
	// A lookalike that DOES classify breaks the contract.
	if err := SignatureCheck(adapter, "metasystem-fake-agent job", "another metasystem-fake-agent"); err == nil {
		t.Fatal("a matching lookalike must fail the contract")
	}
	// A positive that does NOT classify breaks the contract.
	if err := SignatureCheck(adapter, "not an agent", "unrelated"); err == nil {
		t.Fatal("a non-matching positive must fail the contract")
	}
}

func TestAliveNonexistentPid(t *testing.T) {
	if Alive(999999, 1, nil) {
		t.Fatal("a non-existent pid is not alive")
	}
}

// TestAlivePairSurvivesBtimeDrift is the issue #1 sweep-3 / KI-37 regression:
// on a time-synced guest the btime-derived start SECOND of a live process
// differs between two reads, so seconds-equality false-deaths it. The
// clock-step-immune pair (StartTicks+BootID) does not move, so AlivePair with
// the pair must read the live process alive even when the expected SECOND is
// wrong — while a wrong pair (a genuinely different/reused process) still
// reads dead. Uses this test process's own real identity (Linux only; on
// darwin the pair is absent and the seconds path is exact, which the other
// tests cover).
func TestAlivePairSurvivesBtimeDrift(t *testing.T) {
	pid := int64(os.Getpid())
	exact, state, err := identity.KernelProber{}.Probe(pid)
	if err != nil || state != identity.Alive {
		t.Fatalf("probing self failed: state=%v err=%v", state, err)
	}
	if exact.StartTicks == 0 || exact.BootID == "" {
		t.Skip("no clock-step-immune pair on this platform (darwin); seconds path is exact")
	}
	sec := exact.StartedAt.Unix()

	// The exact truth is alive by every path.
	if !AlivePair(pid, sec, exact.StartTicks, exact.BootID, nil) {
		t.Fatal("exact identity must read alive")
	}
	// A btime step moved the recorded second by a few seconds; the pair is
	// unchanged. Seconds-only reads dead (the bug); the pair reads alive.
	if AlivePair(pid, sec+3, 0, "", nil) {
		t.Fatal("a wrong second with no pair must read dead (the seconds path)")
	}
	if !AlivePair(pid, sec+3, exact.StartTicks, exact.BootID, nil) {
		t.Fatal("a wrong second WITH the matching pair must read alive (drift-immune)")
	}
	// A genuinely different process (wrong ticks) is dead even at the right second.
	if AlivePair(pid, sec, exact.StartTicks+1, exact.BootID, nil) {
		t.Fatal("a mismatched pair (reused pid) must read dead")
	}
}
