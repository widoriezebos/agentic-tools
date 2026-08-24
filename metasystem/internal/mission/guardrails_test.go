package mission

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// One grammar for the net's declaration: files and directory prefixes
// cover, everything the host-artifact grammar refuses is refused here
// too, and the protected predicate binds when supplied.
func TestParseGuardrailsGrammar(t *testing.T) {
	class, violation := ParseGuardrails(ContractGuardrailSubject, "specs/gate.sh, goldens/, budgets.json, data/v1..v2.json", nil)
	if violation != "" {
		t.Fatal(violation)
	}
	for path, want := range map[string]bool{
		"specs/gate.sh":      true,
		"goldens/case.txt":   true,
		"goldens/deep/a.bin": true,
		"budgets.json":       true,
		"data/v1..v2.json":   true,
		"specs/other.sh":     false,
		"goldensx/case.txt":  false,
	} {
		if class.Covers(path) != want {
			t.Fatalf("Covers(%q) = %v, want %v", path, !want, want)
		}
	}
	if class.Empty() {
		t.Fatal("a declared class is not empty")
	}
	empty, _ := ParseGuardrails(ContractGuardrailSubject, "  ", nil)
	if !empty.Empty() || empty.Covers("anything") {
		t.Fatal("an empty declaration covers nothing beyond the covenant")
	}
	// The one exception is by construction, not declaration: the
	// covenant custodies itself in EVERY class, the empty one included.
	if !empty.Covers(CovenantFilename) || !class.Covers(CovenantFilename) {
		t.Fatal("every class must custody the covenant by construction")
	}
	var nilClass *GuardrailClass
	if !nilClass.Empty() || nilClass.Covers("anything") || nilClass.Covers(CovenantFilename) {
		t.Fatal("the nil class is empty and covers nothing — no class, no custody")
	}
	for name, value := range map[string]string{
		"empty path":  "goldens/,,specs.sh",
		"absolute":    "/etc/goldens",
		"traversal":   "goldens/../secrets",
		"backslash":   "goldens\\case",
		"glob":        "goldens/*.txt",
		"dot segment": "goldens/./",
		"doubled":     "goldens//deep",
		"lone dot":    ".",
	} {
		if _, violation := ParseGuardrails(ContractGuardrailSubject, value, nil); violation == "" {
			t.Fatalf("%s must refuse", name)
		}
	}
	protected := func(path string) bool { return strings.HasPrefix(path, "sealed/") }
	if _, violation := ParseGuardrails(ContractGuardrailSubject, "sealed/gate.sh", protected); violation == "" {
		t.Fatal("a protected file must refuse")
	}
	if _, violation := ParseGuardrails(ContractGuardrailSubject, "sealed/", protected); violation == "" {
		t.Fatal("a protected prefix must refuse")
	}
}

// A path declared as both host artifact and guardrail contradicts the
// contract; disjoint declarations compose.
func TestGuardrailContradiction(t *testing.T) {
	guardrails, _ := ParseGuardrails(ContractGuardrailSubject, "goldens/", nil)
	declared := map[string]bool{"goldens/case.txt": true, "notes.md": true}
	if v := GuardrailContradiction(declared, guardrails); !strings.Contains(v, "goldens/case.txt") {
		t.Fatalf("the contradiction must refuse by name: %q", v)
	}
	if v := GuardrailContradiction(map[string]bool{"notes.md": true}, guardrails); v != "" {
		t.Fatalf("disjoint declarations must compose: %q", v)
	}
}

// The verified read path: no fences means an unsealed bed (empty class),
// a digest that matches yields the declared class, and a live contract
// that no longer matches the approved digest is a tamper refusal.
func TestVerifiedGuardrails(t *testing.T) {
	repo := t.TempDir()
	missionID := "m-vg"

	class, err := VerifiedGuardrails(repo, missionID)
	if err != nil || !class.Empty() {
		t.Fatalf("no fences must mean the empty class: %v %v", class, err)
	}

	contractText := "```mission\n" +
		"fence.wall-clock-hours=2\nfence.cycles=10\nfence.jobs=20\n" +
		"fence.concurrency=2\nfence.job-cap-min=5\n" +
		"wall.guardrails=goldens/\n" +
		"```\n"
	contractPath := filepath.Join(repo, "plans", "mission-"+missionID+".contract.md")
	os.MkdirAll(filepath.Dir(contractPath), 0o755)
	if err := os.WriteFile(contractPath, []byte(contractText), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(contractText))
	fences := map[string]any{
		"schemaVersion": 1, "missionId": missionID,
		"startedAt": "2026-08-20T00:00:00Z", "cycles": 0,
		"reservations": map[string]any{},
	}
	writeFences := func() {
		data, _ := json.Marshal(fences)
		dir := filepath.Join(repo, "artifacts", "agents", "missions", missionID)
		os.MkdirAll(dir, 0o755)
		if err := os.WriteFile(filepath.Join(dir, "fences.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Fences without an approved digest: unsealed, empty class.
	writeFences()
	class, err = VerifiedGuardrails(repo, missionID)
	if err != nil || !class.Empty() {
		t.Fatalf("no approved digest must mean the empty class: %v %v", class, err)
	}

	// The approved digest matches: the declared class arrives.
	fences["approvedContractSha256"] = hex.EncodeToString(sum[:])
	writeFences()
	class, err = VerifiedGuardrails(repo, missionID)
	if err != nil {
		t.Fatal(err)
	}
	if !class.Covers("goldens/case.txt") || class.Covers("main.go") {
		t.Fatalf("the declared class must arrive verified: %+v", class)
	}

	// A live contract that drifted from the approved digest is tamper.
	if err := os.WriteFile(contractPath, []byte(contractText+"\n<!-- drifted -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifiedGuardrails(repo, missionID); err == nil || !strings.Contains(err.Error(), "cannot trust the contract") {
		t.Fatalf("contract drift must refuse: %v", err)
	}
}
