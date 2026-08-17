package main

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// The genesis effective-class rule (D84, corrected by the opus-window
// re-review finding 3): the source can RAISE only via an announced
// MAIN, a source HUMAN never raises (crafted roots launder to HUMAN),
// and a positive machinery classification at the source disqualifies
// outright — it must never fall through to the target view, where a
// virgin target's ancestry condition can read HUMAN and admit it.
func TestGenesisEffective(t *testing.T) {
	target := lease.ClassifyResult{Class: "DELEGATE", Holder: false}
	human := lease.ClassifyResult{Class: "HUMAN", Holder: false}
	cases := []struct {
		name      string
		from      lease.ClassifyResult
		target    lease.ClassifyResult
		wantClass string
		wantErr   string
	}{
		{"announced main raises", lease.ClassifyResult{Class: "MAIN", Holder: true}, target, "MAIN", ""},
		{"source human falls to target", human, human, "HUMAN", ""},
		{"source human cannot mask a target delegate", human, target, "DELEGATE", ""},
		{"adapter supervisor disqualifies", lease.ClassifyResult{Class: "ADAPTER-SUPERVISOR"}, human, "", "machinery never seeds"},
		{"delegate disqualifies", lease.ClassifyResult{Class: "DELEGATE"}, human, "", "machinery never seeds"},
		{"supervision disqualifies", lease.ClassifyResult{Class: "SUPERVISION"}, human, "", "machinery never seeds"},
		{"unknown class disqualifies", lease.ClassifyResult{Class: "SOMETHING-NEW"}, human, "", "machinery never seeds"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := genesisEffective(tc.target, tc.from)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want refusal containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected refusal: %v", err)
			}
			if got.Class != tc.wantClass {
				t.Fatalf("effective class = %s, want %s", got.Class, tc.wantClass)
			}
		})
	}
}
