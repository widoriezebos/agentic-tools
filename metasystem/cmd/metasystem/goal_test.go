package main

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// The genesis effective-class rule (D84 as bounded by D93's C'
// ruling): only an announced MAIN at the source raises; every other
// source view — HUMAN, or machinery — falls through to the TARGET's
// own view. The fallthrough for machinery is DELIBERATE (a
// machinery-refusal was tried and reverted: it broke the delegated
// validation flows whose announcement-free snapshots classify
// DELEGATE at the source); the target view still refuses a real
// delegate against a signature-carrying target, and the remaining
// crafted-root and virgin-target holes are the recorded cooperative
// posture, not a claim of unforgeability.
func TestGenesisEffective(t *testing.T) {
	target := lease.ClassifyResult{Class: "DELEGATE", Holder: false}
	human := lease.ClassifyResult{Class: "HUMAN", Holder: false}
	virgin := lease.ClassifyResult{Class: "HUMAN", Holder: false}
	cases := []struct {
		name      string
		from      lease.ClassifyResult
		target    lease.ClassifyResult
		wantClass string
	}{
		{"announced main raises", lease.ClassifyResult{Class: "MAIN", Holder: true}, target, "MAIN"},
		{"source human falls to target", human, human, "HUMAN"},
		{"source human cannot mask a target delegate", human, target, "DELEGATE"},
		{"source delegate falls to the target view (delegated validation)", lease.ClassifyResult{Class: "DELEGATE"}, virgin, "HUMAN"},
		{"source delegate against a signature-carrying target stays refused-by-target", lease.ClassifyResult{Class: "DELEGATE"}, target, "DELEGATE"},
		{"adapter supervisor falls to the target view", lease.ClassifyResult{Class: "ADAPTER-SUPERVISOR"}, target, "DELEGATE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := genesisEffective(tc.target, tc.from)
			if err != nil {
				t.Fatalf("unexpected refusal: %v", err)
			}
			if got.Class != tc.wantClass {
				t.Fatalf("effective class = %s, want %s", got.Class, tc.wantClass)
			}
		})
	}
}
