package landing

import (
	"errors"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/designgate"
)

func TestLandingDesignCheck(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name, verdict     string
		compared, refuses bool
	}{
		{"unchanged", "ok", true, false},
		{"head only", "ok", true, false},
		{"no critique", "critique-not-recorded", false, true},
		{"open chain", "critique-open", false, true},
		{"changed body", "design-changed", true, true},
		{"superseded", "design-missing", true, true},
		{"absent record", "ok", false, false},
		{"facts fail", "unchecked", false, false},
		{"record fails", "unchecked", false, false},
		{"no design", "no-accepted-design", false, true},
		{"tier 1", "not-design-bearing", false, false},
		{"allowed", "allowed", false, false},
	} {
		for _, mode := range []string{"warn", "refuse"} {
			for _, person := range []bool{false, true} {
				t.Run(scenario.name+"/"+mode+"/"+map[bool]string{false: "agent", true: "person"}[person], func(t *testing.T) {
					t.Parallel()
					d := designgate.Design{ID: "d", Name: "Gate", Path: "plans/designs/gate.md", Status: "accepted", Critique: "closed at round 2", SHA256: "file"}
					f := DesignFacts{Facts: designgate.Facts{Goal: "g", Tier: 2, Mode: mode, Designs: []designgate.Design{d}},
						Recorded: []DesignRecord{{Design: d, BodySHA256: "body"}}, Digests: map[string]string{"d": "body"}}
					switch scenario.name {
					case "head only":
						f.Designs[0].SHA256 = "head edited"
					case "no critique":
						f.Designs[0].Critique = ""
						f.Digests["d"] = "changed too"
					case "open chain":
						f.Designs[0].Chains = []designgate.Chain{{Round: 3}}
						f.Digests["d"] = "changed too"
					case "changed body":
						f.Digests["d"] = "changed"
					case "superseded":
						f.Designs[0].Status = "superseded"
						d.ID = "replacement"
						f.Designs = append(f.Designs, d)
					case "absent record":
						f.Recorded = nil
					case "facts fail":
						f.Error = errors.New("facts unavailable\nnow")
					case "record fails":
						f.RecordError = errors.New("record unavailable")
					case "no design":
						f.Designs = nil
					case "tier 1":
						f.Tier = 1
					case "allowed":
						f.Allowed = true
					}
					r := ObserveDesign(f, person)
					if r.Verdict != scenario.verdict || r.Compared != scenario.compared || r.WouldRefuse != scenario.refuses || r.Person != person || r.RefusesAgent != (mode == "refuse" && !person && scenario.refuses) {
						t.Fatalf("design observation: %+v", r)
					}
					warns := scenario.refuses || scenario.verdict == "unchecked"
					if !warns {
						if r.Pair != [2]string{} {
							t.Fatalf("silent verdict printed %+v", r.Pair)
						}
						return
					}
					if r.Pair[1] == "" || strings.Contains(r.Pair[0], "\n") || strings.Contains(r.Pair[0], "; the build goes on") || strings.Contains(r.Pair[0], "this build") {
						t.Fatalf("landing pair: %q", r.Pair)
					}
					ending := "; the landing goes on"
					if person && scenario.refuses {
						ending = "; it goes on at your word"
					}
					if r.RefusesAgent {
						ending = "; nothing was landed"
						if r.Pair[1] != "metasystem goal allow g build-without-design --reason TEXT" {
							t.Fatalf("appeal: %q", r.Pair[1])
						}
					}
					if !strings.HasSuffix(r.Pair[0], ending) {
						t.Fatalf("landing pair: %q", r.Pair)
					}
				})
			}
		}
	}
}

func TestLandingDesignCheckObservation(t *testing.T) {
	t.Parallel()
	for _, lineage := range []string{"L", "human"} {
		t.Run(lineage, func(t *testing.T) {
			t.Parallel()
			fixture := newRepositoryObservationFixture(t)
			path := "plans/handoff-fixture-1.md"
			c := fixture.comparison(observeTreeB, "diff --git a/"+path+" b/"+path+"\n+# handoff\n", path)
			c.declare(path, nil, observationText("# handoff\n"))
			held := observationText(string(observationHeldGoal("g", "m1", lineage)))
			c.declare("plans/goals/g.md", held, held)
			calls := 0
			params := ObserveParams{RepoRoot: fixture.root, CandidateTree: c.candidate, Goal: "g", Actor: "m1+" + lineage, DirectFix: "register-carriage", DesignFacts: func() DesignFacts {
				calls++
				return DesignFacts{Facts: designgate.Facts{Goal: "g", Tier: 2, Mode: "refuse"}}
			}}
			r := c.observe(params)
			person := lineage == "human"
			if calls != 1 || r.Design == nil || r.Design.Verdict != "no-accepted-design" || r.Design.Person != person || r.RefusesAgent == person {
				t.Fatalf("attached verdict: calls=%d observation=%+v", calls, r)
			}
			if person {
				if r.Code != "register-carriage" || !strings.HasSuffix(r.Design.Pair[0], "; it goes on at your word") {
					t.Fatalf("person's observation: %+v", r)
				}
			} else if r.Code != "LANDING_DESIGN_NOT_STANDING" || !knownRefusalCode(r.Code) {
				t.Fatalf("agent's observation: %+v", r)
			}
		})
	}
}
