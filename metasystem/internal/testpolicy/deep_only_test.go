package testpolicy

import (
	"os"
	"reflect"
	"testing"
)

func TestDeepOnlySectionsUseComputedStandardClosure(t *testing.T) {
	contract := Contract{
		Surfaces: []Surface{
			{ID: "provider", Paths: []string{"provider/**"}, Standard: []string{"ordinary"}, Deep: []string{"section/deep-only"}, Critical: []string{"critical-proof"}},
			{ID: "consumer", Paths: []string{"consumer/**"}, DependsOn: []string{"provider"}, Standard: []string{"section/consumer-only"}},
			{ID: "fallback", Standard: []string{"ordinary"}},
		},
		Fallback: "fallback",
		Groups: []Group{
			{ID: "canary", Adapter: "go"},
			{ID: "ordinary", Adapter: "go"},
			{ID: "section/critical-only", Adapter: "section", Obligations: []string{"critical-proof"}},
			{ID: "section/consumer-only", Adapter: "section"},
			{ID: "section/deep-only", Adapter: "section"},
		},
		Always: Always{Canary: []string{"canary"}},
	}
	got, err := DeepOnlySectionGroupIDs(contract)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"section/deep-only"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deep-only groups = %v, want %v; critical providers and dependency consumers must be standard-reachable", got, want)
	}
}

func TestEveryDeepOnlySectionIsCadenceCovered(t *testing.T) {
	data, err := os.ReadFile("../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	contract, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	deepOnly, err := DeepOnlySectionGroupIDs(contract)
	if err != nil {
		t.Fatal(err)
	}
	cadence := set(contract.Cadence)
	var missing []string
	for _, id := range deepOnly {
		if !cadence[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("deep-only section groups missing from cadence: %v; computed deep-only list: %v", missing, deepOnly)
	}
}
