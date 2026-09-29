package landing

import (
	_ "embed"
	"strings"
)

// policyRulingsSource holds the rows of the human rulings the compiled
// landing classes cite as their authority (authorizedBy). They are landing
// policy compiled into the engine: memory/rulings.md stays the human
// register, and the engine never reads its landing authority from it.
//
//go:embed policy-rulings.md
var policyRulingsSource string

// PolicyRuling is one compiled landing authority: its id and its register
// row, verbatim.
type PolicyRuling struct {
	ID  string
	Row string
}

// PolicyRulings returns the compiled landing authorities in source order.
func PolicyRulings() []PolicyRuling {
	var rulings []PolicyRuling
	for _, line := range strings.Split(policyRulingsSource, "\n") {
		if id, ok := rulingRowID(line); ok {
			rulings = append(rulings, PolicyRuling{ID: id, Row: line})
		}
	}
	return rulings
}

// policyRuling reports whether id is a compiled landing authority.
func policyRuling(id string) bool {
	for _, ruling := range PolicyRulings() {
		if ruling.ID == id {
			return true
		}
	}
	return false
}
