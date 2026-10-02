package main

import (
	"os"
	"strings"
	"testing"
)

// Every generated review brief names the default threat model, so a critic
// never assumes an adversary nobody named when the page states none.
func TestGeneratedReviewBriefsNameTheDefaultThreatModel(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("intent_delivery.go")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(source), "where it states none: our own agents and operators make mistakes, nobody attacks") +
		strings.Count(string(source), "where they state none: our own agents and operators make mistakes, nobody attacks"); got != 2 {
		t.Fatalf("the design and job review briefs must both name the default threat model; found %d", got)
	}
}
