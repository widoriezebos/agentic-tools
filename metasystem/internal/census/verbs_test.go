package census

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
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
	// the leaked-fixture fence at every entry point.
	os.WriteFile(filepath.Join(dir, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644)
	if _, err := fixtureauth.New(dir); err == nil {
		t.Fatal("a leaked fixture in a non-fake checkout was authorized")
	}
}

func TestSignatureCheckContract(t *testing.T) {
	// positive classifies, lookalike does not: contract holds.
	if err := SignatureCheck("fake", "metasystem-fake-agent job", "unrelated proc"); err != nil {
		t.Fatalf("valid contract rejected: %v", err)
	}
	// A lookalike that DOES classify breaks the contract.
	if err := SignatureCheck("fake", "metasystem-fake-agent job", "another metasystem-fake-agent"); err == nil {
		t.Fatal("a matching lookalike must fail the contract")
	}
	// A positive that does NOT classify breaks the contract.
	if err := SignatureCheck("fake", "not an agent", "unrelated"); err == nil {
		t.Fatal("a non-matching positive must fail the contract")
	}
	// The supervisor process launches the CLI; it is never the CLI.
	if err := SignatureCheck("fake", "metasystem-fake-agent job",
		"/repo/bin/metasystem delegate-supervisor fake dispatch --job j metasystem-fake-agent"); err != nil {
		t.Fatalf("the supervisor argv must stay out of the fake signature: %v", err)
	}
}

// S4-7 (supervision-fixtures.sh): every declared adapter runtime's registry
// signature holds its PROVIDER-OWNED positive/lookalike vectors. The loop
// iterates the declared population with no runtime branch: a future runtime
// joins by declaration.
func TestSignatureCheckEveryDeclaredRuntime(t *testing.T) {
	t.Parallel()
	names := runtimes.WithAdapter()
	if len(names) < 4 {
		t.Fatalf("only %d adapter runtimes exercised — the declared population went missing", len(names))
	}
	for _, runtime := range names {
		declaration, ok := runtimes.Lookup(runtime)
		if !ok {
			t.Fatalf("%s is listed but not declared", runtime)
		}
		vectors := declaration.SignatureVectors
		if vectors.Positive == "" || vectors.Lookalike == "" {
			t.Fatalf("%s declared no signature vectors", runtime)
		}
		if err := SignatureCheck(runtime, vectors.Positive, vectors.Lookalike); err != nil {
			t.Fatalf("%s: %v", runtime, err)
		}
	}
}

// S4-7's bad-adapter refusals, on the declaration side that replaced the
// adapter scripts: every bad declaration fails closed.
func TestBadSignatureDeclarationsFailClosed(t *testing.T) {
	t.Parallel()
	// invalid-ere: a declaration that does not compile.
	if _, err := CompileSignature("bad", []string{"["}, nil); err == nil {
		t.Fatal("invalid-ere signature did not fail closed")
	}
	if _, err := CompileSignature("bad", []string{"^ok$"}, []string{"["}); err == nil {
		t.Fatal("invalid-ere exclude did not fail closed")
	}
	// exclude-tie: exclude wins ties, so the positive cannot classify.
	tie, err := CompileSignature("tie", []string{"^tie$"}, []string{"^tie$"})
	if err != nil {
		t.Fatal(err)
	}
	if err := signatureContract("tie", tie, "tie", "lookalike"); err == nil {
		t.Fatal("exclude-tie signature did not fail closed")
	}
	// malformed: a line that is neither match nor exclude declares nothing,
	// so the contract's positive cannot classify.
	matches, excludes := ParseSignatureText("bogus .*\n")
	malformed, err := CompileSignature("malformed", matches, excludes)
	if err != nil {
		t.Fatal(err)
	}
	if err := signatureContract("malformed", malformed, "tie", "lookalike"); err == nil {
		t.Fatal("malformed signature did not fail closed")
	}
	// adapter-failed: a runtime with no declaration is refused.
	if err := SignatureCheck("no-such-runtime", "tie", "lookalike"); err == nil {
		t.Fatal("an undeclared runtime's signature check did not fail closed")
	}
}

func TestAliveNonexistentPid(t *testing.T) {
	if Alive(999999, 1, nil) {
		t.Fatal("a non-existent pid is not alive")
	}
}

// The signature verbs were retired (U6a); the positive/lookalike contract
// stays a test of every live registry signature.
// SignatureCheck is the positive/lookalike contract of one runtime's
// registry signature: the positive argv must classify as the runtime and
// the lookalike must NOT — the proof that a signature is neither too loose
// nor too tight. Returns an error when the contract fails.
func SignatureCheck(runtime, positive, lookalike string) error {
	sig, _, err := RuntimeSignature(runtime)
	if err != nil {
		return err
	}
	return signatureContract(runtime, sig, positive, lookalike)
}

func signatureContract(runtime string, sig Signature, positive, lookalike string) error {
	positiveOK := sig.matches(positive)
	lookalikeOK := sig.matches(lookalike)
	if !positiveOK || lookalikeOK {
		return fmt.Errorf("signature positive/lookalike contract failed for %s", runtime)
	}
	return nil
}
