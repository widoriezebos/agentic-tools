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
		{Name: "dup", TailoringPriority: 0, InstructionFile: "./dot.md",
			SelfCheck: &LiveSelfCheck{VendoredMarker: ""}},
	}
	problems := Validate()
	for _, want := range []string{"shell-safe grammar", "variable grammar", "clean-relative",
		"already belongs", "not a permission field", "empty residual",
		"declared twice", "must be positive", "ascending priority order",
		"nonblank vendored marker"} {
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
	// Populations DERIVE from the declarations (a new runtime must not
	// fail this test); only the RELATIONAL policies below are pinned.
	if len(Names()) != len(All()) {
		t.Fatalf("Names/All disagree: %v", Names())
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
	for _, name := range Adoptable() {
		d, _ := Lookup(name)
		if !d.Adoptable || d.SynthesizedModel != "" {
			t.Fatalf("adoptable filter leaked %s", name)
		}
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
	files := InstructionFiles()
	if len(files) == 0 {
		t.Fatal("no instruction files declared")
	}
	for i := 1; i < len(files); i++ {
		if files[i-1] >= files[i] {
			t.Fatalf("instruction files not sorted-unique: %v", files)
		}
	}
	for _, d := range All() {
		found := false
		for _, f := range files {
			if f == d.InstructionFile {
				found = true
			}
		}
		if d.InstructionFile != "" && !found {
			t.Fatalf("%s's instruction file missing from the set", d.Name)
		}
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

// The C2 declaration surfaces: filters, vectors, collision roots —
// derived relationally, with the validator's new rows exercised both
// ways.
func TestDeclaredSurfaces(t *testing.T) {
	for _, name := range WithAdapter() {
		d, _ := Lookup(name)
		if !d.HasAdapter {
			t.Fatalf("with-adapter leaked %s", name)
		}
		if d.SignatureVectors.Positive == "" || d.SignatureVectors.Lookalike == "" {
			t.Fatalf("%s lacks signature vectors", name)
		}
	}
	for _, name := range WithHost() {
		d, _ := Lookup(name)
		if !d.HasHostLauncher {
			t.Fatalf("with-host leaked %s", name)
		}
	}
	commonLifecycle := WithCommonLifecycle()
	for _, name := range commonLifecycle {
		if name == "fake" {
			t.Fatal("fake claimed the common lifecycle shape")
		}
	}
	if len(commonLifecycle) == 0 {
		t.Fatal("no common-lifecycle adapters declared")
	}
	roots := CollisionRootsAll()
	if len(roots) == 0 {
		t.Fatal("no collision roots contributed")
	}
	for i, root := range roots {
		if root[0] != '.' {
			t.Fatalf("collision root %q not dot-prefixed", root)
		}
		if i > 0 && roots[i-1] >= root {
			t.Fatalf("collision roots not sorted-unique: %v", roots)
		}
	}
	// Today's exact full population is a CURRENT-declaration fact the
	// installer relies on; pinned here so drift refuses by name.
	if len(roots) != 3 || roots[0] != ".agents" || roots[1] != ".claude" || roots[2] != ".devin" {
		t.Fatalf("collision-root population drifted: %v", roots)
	}
}

// The validator rejects vectorless adapters and hostile collision
// roots.
func TestValidateC2Rows(t *testing.T) {
	saved := declarations
	defer func() { declarations = saved }()
	declarations = []Declaration{
		{Name: "noveec", TailoringPriority: 1, HasAdapter: true, AdoptionDefault: true, Adoptable: true,
			InstructionFile: "AGENTS.md",
			CollisionRoots:  []string{"noleadingdot", ".ok/../escape"}},
	}
	problems := Validate()
	for _, want := range []string{"requires signature vectors", "clean dot-prefixed"} {
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
