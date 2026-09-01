package main

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

func TestMainSplitRatificationRefusesHolderWithoutLeaseEpoch(t *testing.T) {
	if _, err := mainSplitRatification("parent", strings.Repeat("a", 64), lease.ClassifyResult{Class: lease.ClassMain, Holder: false, MainId: "non-holder"}); err == nil ||
		!strings.Contains(err.Error(), "MAIN checkout-lease holder") {
		t.Fatalf("non-holder MAIN classification did not refuse toward the authenticated holder: %v", err)
	}
	// ClassHuman authenticates terminal ancestry but carries no human name.
	// The accepted tier=human token deliberately names --by, so this path
	// fails closed toward that explicit mint instead of fabricating identity.
	if _, err := mainSplitRatification("parent", strings.Repeat("a", 64), lease.ClassifyResult{Class: lease.ClassHuman}); err == nil ||
		!strings.Contains(err.Error(), "must name its person") || !strings.Contains(err.Error(), "--by") {
		t.Fatalf("human classification without the token's --by coordinate did not name the lawful path: %v", err)
	}
	classification := lease.ClassifyResult{Class: lease.ClassMain, Holder: true, MainId: "main-without-lease"}
	if _, err := mainSplitRatification("parent", strings.Repeat("a", 64), classification); err == nil ||
		!strings.Contains(err.Error(), "no checkout lease epoch") || !strings.Contains(err.Error(), "metasystem up") {
		t.Fatalf("MAIN holder without an authenticated coordinate did not refuse toward lease establishment: %v", err)
	}
	epoch := int64(7)
	classification.ClaimEpoch = &epoch
	token, err := mainSplitRatification("parent", strings.Repeat("b", 64), classification)
	if err != nil || token.MainID != classification.MainId || token.ClaimEpoch != epoch {
		t.Fatalf("authenticated MAIN coordinates did not mint the token: %+v %v", token, err)
	}
}
