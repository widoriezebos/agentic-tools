package adapter

import (
	"reflect"
	"strings"
	"testing"
)

// The probe table: devin's probe is registered with its exact labels,
// cross-runtime and undeclared lookups refuse, the list view serves
// conformance, and registration guards reject incomplete/duplicate
// probes.
func TestSelftestProbeTable(t *testing.T) {
	probe, err := SelftestProbeFor("devin", "symlinked-skill-discovery")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(probe.BehaviorLabels,
		[]string{"documented-exit-status-observation", "symlinked-skill-discovery"}) {
		t.Fatalf("devin probe labels drifted: %v", probe.BehaviorLabels)
	}
	if _, err := SelftestProbeFor("codex", "symlinked-skill-discovery"); err == nil {
		t.Fatal("a cross-runtime probe name was honored")
	}
	if _, err := SelftestProbeFor("devin", "nope"); err == nil {
		t.Fatal("an undeclared probe name was honored")
	}
	if got := SelftestProbeList(); !reflect.DeepEqual(got, []string{"devin/symlinked-skill-discovery"}) {
		t.Fatalf("probe list wrong: %v", got)
	}
}

func TestProbeRegistrationGuards(t *testing.T) {
	expectPanic := func(name string, f func()) {
		defer func() {
			if r := recover(); r == nil || !strings.Contains(r.(string), "probe") {
				t.Fatalf("%s did not panic usefully: %v", name, r)
			}
		}()
		f()
	}
	expectPanic("incomplete probe", func() { RegisterSelftestProbe("x", SelftestProbe{Name: "p"}) })
	complete := SelftestProbe{
		Name:           "symlinked-skill-discovery",
		PrepareScratch: func(string, string) error { return nil },
		PromptText:     func(string) string { return "" },
		VerifyEvidence: func(string, string) error { return nil },
	}
	expectPanic("duplicate probe", func() { RegisterSelftestProbe("devin", complete) })
}
