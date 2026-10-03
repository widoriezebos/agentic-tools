package designgate

import (
	"errors"
	"testing"
)

func TestDesignGateVerdicts(t *testing.T) {
	t.Parallel()
	closed := Design{ID: "design-id", Path: "design.md", Name: "Design", Status: "accepted", SHA256: "design-sha256", Critique: "closed at round 2 on 0 material findings (WHO)"}
	missing, open, ruled, draft := closed, closed, closed, closed
	missing.Critique = ""
	open.Chains = []Chain{{Closed: true, Round: 1}, {Round: 3}}
	ruled.Critique, ruled.Chains = "ruled by Wido", open.Chains
	draft.Status = "draft"
	for _, tc := range []struct {
		name    string
		facts   Facts
		verdict string
		refuse  bool
		pair    [2]string
	}{
		{"tier one", Facts{Goal: "G", Tier: 1}, "not-design-bearing", false, [2]string{}},
		{"no design", Facts{Goal: "G", Tier: 2}, "no-accepted-design", true, [2]string{"warning: goal G has no accepted design; this build runs on its brief alone", "metasystem design write G --brief FILE"}},
		{"draft", Facts{Goal: "G", Tier: 3, Designs: []Design{draft}}, "no-accepted-design", true, [2]string{"warning: goal G has no accepted design; this build runs on its brief alone", "metasystem design review design.md"}},
		{"missing critique", Facts{Tier: 2, Designs: []Design{missing}}, "critique-not-recorded", true, [2]string{"warning: the accepted design Design does not say its critique closed; the build goes on", "edit design.md: add \"- Critique: closed at round N on 0 material findings (WHO)\" under its Goals line"}},
		{"open critique", Facts{Tier: 3, Designs: []Design{open}}, "critique-open", true, [2]string{"warning: the accepted design Design says its critique closed, but its review here is open at round 3", "metasystem design review design.md --dispositions FILE --after 3"}},
		{"missing precedes open", Facts{Tier: 2, Designs: []Design{open, missing}}, "critique-not-recorded", true, [2]string{"warning: the accepted design Design does not say its critique closed; the build goes on", "edit design.md: add \"- Critique: closed at round N on 0 material findings (WHO)\" under its Goals line"}},
		{"closed", Facts{Tier: 2, Designs: []Design{closed}}, "ok", false, [2]string{}},
		{"ruled", Facts{Tier: 3, Designs: []Design{ruled}}, "ok", false, [2]string{}},
		{"unchecked", Facts{Goal: "G", Tier: 2, Error: errors.New("unreadable")}, "unchecked", false, [2]string{"warning: the design check could not run (unreadable); this build was not checked", "metasystem design list --goal G"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := Check(tc.facts)
			if r.Verdict != tc.verdict || r.WouldRefuse != tc.refuse || r.Mode != "warn" || r.Warning != tc.pair {
				t.Fatalf("got %+v; want %s, wouldRefuse=%v, pair=%q", r, tc.verdict, tc.refuse, tc.pair)
			}
		})
	}
}
