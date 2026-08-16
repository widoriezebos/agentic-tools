package runtimes

import (
	"strings"
	"testing"
)

// The shipped rows validate, the wire format is exact, and the
// collision proof holds — including the one sanctioned exception.
func TestRegistrationRows(t *testing.T) {
	if problems := ValidateRegistration(); len(problems) != 0 {
		t.Fatalf("shipped rows invalid: %v", problems)
	}
	wire := RegistrationV1("codex")
	lines := strings.Split(strings.TrimSuffix(wire, "\n"), "\n")
	if lines[0] != "registration/v1" {
		t.Fatalf("wire header wrong: %q", lines[0])
	}
	if len(lines) != 1+len(RegistrationRows("codex")) {
		t.Fatalf("wire row count wrong: %d", len(lines))
	}
	for _, line := range lines[1:] {
		if got := len(strings.Split(line, "\t")); got != 12 {
			t.Fatalf("wire arity %d != 12: %q", got, line)
		}
	}
	if !strings.Contains(wire, ".codex/hooks.json\tpresence-only\tinstruction-bearing\ttrue") {
		t.Fatalf("the codex exception row drifted:\n%s", wire)
	}
	if RegistrationV1("fake") != "registration/v1\n" {
		t.Fatalf("fake must have a header-only wire: %q", RegistrationV1("fake"))
	}
	// The dirs view and the rows agree on every declared destination
	// directory (the pre-row mirror must not drift while it survives).
	for _, runtime := range []string{"claude", "codex", "devin"} {
		declaration, _ := Lookup(runtime)
		for _, dir := range declaration.RegistrationDirs {
			found := false
			for _, row := range RegistrationRows(runtime) {
				if row.Destination == dir || strings.HasPrefix(row.Destination, dir+"/") {
					found = true
				}
			}
			if !found {
				t.Fatalf("%s: dirs view entry %s has no backing row", runtime, dir)
			}
		}
	}
}

// The validator rejects hostile rows in both directions.
func TestValidateRegistrationCounterexamples(t *testing.T) {
	saved := registrationRows
	defer func() { registrationRows = saved }()
	registrationRows = map[string][]RegistrationRow{
		"claude": {
			{ID: "a", Operation: OpTree, Policy: PolicyTransformedBytes,
				Requiredness: Requiredness{TemplateSource: "required", AdoptedDestination: "required"},
				Destination:  ".claude/x", InstructionBearing: true, Source: "skills"},
			{ID: "a", Operation: OpCopyFile, Policy: PolicyPresenceOnly,
				Requiredness: Requiredness{TemplateSource: "required", AdoptedDestination: "required"},
				Destination:  "loose/INSTR.md", InstructionBearing: true, Source: "x"},
			{ID: "b", Operation: OpCopyFile, Policy: PolicyPresenceOnly,
				Requiredness: Requiredness{TemplateSource: "required", AdoptedDestination: "required"},
				Destination:  ".mystery/hooks.json", InstructionBearing: true, UncoveredException: true, Source: "x"},
		},
	}
	problems := ValidateRegistration()
	for _, want := range []string{"duplicate artifact role", "not legal for operation",
		"lies under no contributed collision root", "sanctioned only for .codex/hooks.json"} {
		found := false
		for _, p := range problems {
			if strings.Contains(p, want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("validator missed %q in %v", want, problems)
		}
	}
}
