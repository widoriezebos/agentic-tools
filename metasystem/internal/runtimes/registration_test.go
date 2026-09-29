package runtimes

import (
	"strings"
	"testing"
)

// The shipped rows validate and the collision proof holds — including the
// one sanctioned exception.
func TestRegistrationRows(t *testing.T) {
	if problems := ValidateRegistration(); len(problems) != 0 {
		t.Fatalf("shipped rows invalid: %v", problems)
	}
	exception := false
	for _, row := range RegistrationRows("codex") {
		if row.Destination == ".codex/hooks.json" {
			exception = row.Policy == PolicyPresenceOnly && row.InstructionBearing && row.UncoveredException
		}
	}
	if !exception {
		t.Fatalf("the codex exception row drifted: %+v", RegistrationRows("codex"))
	}
	if rows := RegistrationRows("fake"); len(rows) != 0 {
		t.Fatalf("fake must declare no registration rows: %+v", rows)
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
