package runtimes

import (
	"reflect"
	"strings"
	"testing"
)

// The declaration invariants hold for the shipped universe — and the
// validator actually catches violations (both directions, so a hostile
// declaration cannot land).
func TestDeclarationInvariants(t *testing.T) {
	if problems := Validate(); len(problems) != 0 {
		t.Fatalf("shipped declarations invalid: %v", problems)
	}
	saved := declarations
	defer func() { declarations = saved }()
	declarations = []Declaration{
		{Name: "Bad Name", TailoringPriority: 1, AdoptionDefault: true, Adoptable: true,
			SessionEnv: "lower", InstructionFile: "../escape.md",
			PermissionResiduals: map[string]string{"bogus": ""}},
		{Name: "dup", TailoringPriority: 1},
	}
	problems := Validate()
	for _, want := range []string{"shell-safe grammar", "variable grammar", "clean-relative",
		"already belongs", "not a permission field", "empty residual"} {
		found := false
		for _, p := range problems {
			if contains(p, want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("validator missed %q in %v", want, problems)
		}
	}
}

// The pinned policy facts: tailoring precedence codex > devin > claude
// > fake, fake never outranking a real runtime, claude the one adoption
// default, fake never adoptable, the fake-model synthesis value.
func TestPinnedPolicies(t *testing.T) {
	if got := Names(); !reflect.DeepEqual(got, []string{"codex", "devin", "claude", "fake"}) {
		t.Fatalf("priority order wrong: %v", got)
	}
	if got := DefaultFor(map[string]bool{"claude": true, "devin": true, "codex": true, "fake": true}); got != "codex" {
		t.Fatalf("strongest of all = %s, want codex", got)
	}
	if got := DefaultFor(map[string]bool{"claude": true, "fake": true}); got != "claude" {
		t.Fatalf("fake outranked claude: %s", got)
	}
	if got := DefaultFor(map[string]bool{"fake": true}); got != "fake" {
		t.Fatalf("sole fake selection must default fake: %s", got)
	}
	if got := Adoptable(); !reflect.DeepEqual(got, []string{"codex", "devin", "claude"}) {
		t.Fatalf("adoptable wrong: %v", got)
	}
	if AdoptionDefault() != "claude" {
		t.Fatalf("adoption default wrong: %s", AdoptionDefault())
	}
	fake, _ := Lookup("fake")
	if fake.Adoptable || fake.SynthesizedModel != "fake-model" {
		t.Fatalf("fake declaration drifted: %+v", fake)
	}
}

// The hook capabilities are independent: all three real runtimes ship
// an enforcement config; only claude declares a live self-check.
func TestHookCapabilitiesIndependent(t *testing.T) {
	shipped := map[string]bool{}
	for _, d := range All() {
		if d.ShippedEnforcementConfig != "" {
			shipped[d.Name] = true
		}
		if d.SelfCheck != nil && d.Name != "claude" {
			t.Fatalf("%s declares a live self-check", d.Name)
		}
	}
	if !shipped["claude"] || !shipped["codex"] || !shipped["devin"] || shipped["fake"] {
		t.Fatalf("shipped enforcement coverage wrong: %v", shipped)
	}
	claude, _ := Lookup("claude")
	if claude.SelfCheck.VendoredMarker != "$CLAUDE_PROJECT_DIR/metasystem" {
		t.Fatalf("claude vendored marker drifted: %q", claude.SelfCheck.VendoredMarker)
	}
}

// The instruction-file set feeds every consumer from one list.
func TestInstructionFiles(t *testing.T) {
	if got := InstructionFiles(); !reflect.DeepEqual(got, []string{"AGENTS.md", "CLAUDE.md"}) {
		t.Fatalf("instruction files wrong: %v", got)
	}
}

// Residuals are field-bound and fail closed: devin's two identifiers
// exist, and undeclared lookups return empty.
func TestResiduals(t *testing.T) {
	if got := ResidualFor("devin", "readRoots"); got != "devin-read-roots-unenforced" {
		t.Fatalf("devin readRoots residual: %q", got)
	}
	if got := ResidualFor("devin", "network"); got != "" {
		t.Fatalf("undeclared field returned a residual: %q", got)
	}
	if got := ResidualFor("codex", "readRoots"); got != "" {
		t.Fatalf("codex declares no residual yet got: %q", got)
	}
	if got := ResidualFor("nope", "readRoots"); got != "" {
		t.Fatalf("unknown runtime returned a residual: %q", got)
	}
}

// The envelope-enforcement declarations match the adapters' live
// snapshot shapes exactly (the suite greps these same triples).
func TestEnvelopeEnforcementPinned(t *testing.T) {
	want := map[string]map[string]Enforcement{
		"claude": {"writeRoots": Mapped, "readRoots": Mapped, "network": Mapped},
		"codex":  {"writeRoots": Mapped, "readRoots": NotEnforced, "network": Mapped},
		"devin":  {"writeRoots": NotEnforced, "readRoots": NotEnforced, "network": NotEnforced},
	}
	for name, expected := range want {
		d, _ := Lookup(name)
		if !reflect.DeepEqual(map[string]Enforcement(d.ExpectedEnvelopeEnforcement), expected) {
			t.Fatalf("%s enforcement drifted: %v", name, d.ExpectedEnvelopeEnforcement)
		}
	}
	fake, _ := Lookup("fake")
	if fake.ExpectedEnvelopeEnforcement != nil {
		t.Fatal("fake's enforcement is profile-driven and must stay undeclared")
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
