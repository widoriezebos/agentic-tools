package census

import (
	"encoding/json"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
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
