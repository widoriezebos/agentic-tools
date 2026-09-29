package contractmerge

import (
	"errors"
	"reflect"
	"testing"
)

func TestInputAndSurfacePathEditsAreIdempotentAndRefuseUnknownOwners(t *testing.T) {
	t.Parallel()
	contract := mergeFixture()
	contract.Surfaces[0].DependsOn = []string{}
	contract, err := AddInputs(contract, "app-group", []string{"floors.json", " go.mod "})
	if err != nil || !reflect.DeepEqual(contract.Groups[0].Inputs, []string{"go.mod", "floors.json"}) {
		t.Fatalf("add inputs = %v, %v", contract.Groups[0].Inputs, err)
	}
	contract, err = RemoveInputs(contract, "app-group", []string{"floors.json", "never-listed"})
	if err != nil || !reflect.DeepEqual(contract.Groups[0].Inputs, []string{"go.mod"}) {
		t.Fatalf("remove inputs = %v, %v", contract.Groups[0].Inputs, err)
	}
	contract, err = AddSurfacePaths(contract, "app", []string{"lib/**", "app/**"})
	if err != nil || !reflect.DeepEqual(contract.Surfaces[0].Paths, []string{"app/**", "lib/**"}) {
		t.Fatalf("add surface paths = %v, %v", contract.Surfaces[0].Paths, err)
	}
	contract, err = RemoveSurfacePaths(contract, "app", []string{"lib/**"})
	if err != nil || !reflect.DeepEqual(contract.Surfaces[0].Paths, []string{"app/**"}) {
		t.Fatalf("remove surface paths = %v, %v", contract.Surfaces[0].Paths, err)
	}
	extra := fixtureSurface("extra", "app-group")
	contract.Surfaces = appendBeforeFallback(contract.Surfaces, extra)
	contract.Surfaces[0].DependsOn = []string{"extra"}
	contract, err = RemoveSurface(contract, "extra")
	if err != nil || len(contract.Surfaces) != 2 || len(contract.Surfaces[0].DependsOn) != 0 {
		t.Fatalf("remove surface = %+v, %v", contract.Surfaces, err)
	}
	if again, err := RemoveSurface(contract, "extra"); err != nil || !reflect.DeepEqual(again, contract) {
		t.Fatalf("a repeated surface removal changed the contract: %v", err)
	}
	var refusal *Refusal
	for name, edit := range map[string]func() error{
		"unknown group":   func() error { _, err := AddInputs(contract, "no-group", []string{"x"}); return err },
		"unknown surface": func() error { _, err := RemoveSurfacePaths(contract, "no-surface", []string{"x"}); return err },
		"empty path":      func() error { _, err := RemoveInputs(contract, "app-group", []string{" "}); return err },
		"no path":         func() error { _, err := AddSurfacePaths(contract, "app", nil); return err },
	} {
		if err := edit(); !errors.As(err, &refusal) || refusal.Code != AddTestsCode {
			t.Errorf("%s: err = %v, want a %s refusal", name, err, AddTestsCode)
		}
	}
	if _, err := RemoveSurface(contract, "residual"); !errors.As(err, &refusal) || refusal.Code != InvalidContractCode {
		t.Fatalf("removing the fallback surface = %v, want the contract refused as invalid", err)
	}
}
