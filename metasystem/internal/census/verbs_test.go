package census

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuthIdentityFixtureFile(t *testing.T) {
	dir := t.TempDir()
	idFile := filepath.Join(dir, "id.json")
	os.WriteFile(idFile, []byte(`{"42":{"started":100,"command":"claude serve"}}`), 0o644)
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", idFile)
	id, err := AuthIdentity(42)
	if err != nil {
		t.Fatal(err)
	}
	if id["pidStartedAt"].(int64) != 100 || id["command"].(string) != "claude serve" {
		t.Fatalf("wrong identity: %+v", id)
	}
	// A pid absent from the fixture falls through to ps (and fails for a
	// non-existent pid).
	if _, err := AuthIdentity(999999); err == nil {
		t.Fatal("a non-existent pid must error")
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
	if Alive(999999, 1) {
		t.Fatal("a non-existent pid is not alive")
	}
}
