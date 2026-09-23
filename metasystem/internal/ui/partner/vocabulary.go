package partner

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"sync"
)

// The interface's own words, carried into the standing rule.
//
// A Partner that calls Ready for Work "the ready queue", a tier "a risk level"
// and an arc "an epic" is a Partner explaining a different workspace than the
// one on the screen. The interface already writes every one of those
// explanations down, once, in the help register beside the names it shows:
// src/help/terms.ts. This is that register, as JSON, so the Go side can carry
// it without a second copy anybody has to keep in step by hand.
//
// It is a generated file, and the smallest of the three ways the design left
// open. The bundle would have meant a build step and a served asset; handing
// the texts from the page with the first turn would have meant a payload on
// the wire and a page that can silently stop sending them. A committed JSON
// beside the register, embedded here, costs one file and one assertion:
// src/help/terms.test.ts reads this file and fails when the register and it
// disagree, so the drift is caught by the test run that already exists rather
// than by a reader noticing.

//go:embed vocabulary.json
var vocabularyJSON []byte

// Term is one word this interface uses in its own way, and what it means.
type Term struct {
	ID   string `json:"id"`
	Term string `json:"term"`
	Text string `json:"text"`
}

var (
	vocabularyOnce  sync.Once
	vocabularyBlock string
)

// Vocabulary is the register as it stands, in the order the file carries it.
func Vocabulary() []Term {
	var terms []Term
	if err := json.Unmarshal(vocabularyJSON, &terms); err != nil {
		return nil
	}
	return terms
}

// vocabulary is the block every turn carries, composed once.
//
// The terms are ordered by name so the block is stable between builds: a
// prompt that reshuffles itself for no reason is a prompt no cache can hold.
func vocabulary() string {
	vocabularyOnce.Do(func() {
		terms := Vocabulary()
		if len(terms) == 0 {
			vocabularyBlock = ""
			return
		}
		sort.Slice(terms, func(left, right int) bool { return terms[left].ID < terms[right].ID })
		var built strings.Builder
		built.WriteString("What this interface's words mean, as it explains them to the human\n")
		for _, term := range terms {
			built.WriteString("- " + term.Term + ": " + term.Text + "\n")
		}
		vocabularyBlock = built.String()
	})
	return vocabularyBlock
}
