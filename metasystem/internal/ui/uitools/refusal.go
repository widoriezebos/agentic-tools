package uitools

import (
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

// The refusal register as a reading (g1-s68 D4).
//
// A human who asks what happened after the engine refused an act is asking the
// question the register already answers: who owns the refusal, what kind of
// refusal it is, and which human verb carries past it. The register is compiled
// into this build, so the reading names the build as its source; the Partner
// quotes the row and adds the state it read elsewhere.

// registerSource is what every reading of the register is stamped with.
const registerSource = "the refusal register compiled into this build (internal/refusal/register.go)"

// shapeMeans is what each of the register's shapes means, in the register's own
// terms, so the Partner can say what kind of refusal it is without guessing.
var shapeMeans = map[refusal.Shape]string{
	refusal.Identity: "the machine establishes that the word is the human's; proving who is asking is the way past it",
	refusal.Warning:  "the machine prints and records the warning, then complies",
	refusal.Question: "the word is stale or names nothing, so the machine asks for it again",
	refusal.Agent:    "the refusal binds an agent's act only; the human verb below carries past it",
}

// standingMeans is what each H1 standing asks of the person who met the refusal.
var standingMeans = map[refusal.Standing]string{
	refusal.StandingInput:    "the word was malformed, stale or named nothing; re-read and issue it again",
	refusal.StandingAgent:    "it binds a caller that is not the authenticated person",
	refusal.StandingIdentity: "it establishes that the caller is the person",
	refusal.StandingGuide:    "the act as asked would damage the system; the forward command achieves the intent the right way",
}

// refusalRow answers one code with its register row, or says the register has
// none. Exclusions, prose rows and the interface's own codes have no row and
// get the same honest line.
func (r Readers) refusalRow(code string) Result {
	code = strings.TrimSpace(code)
	if code == "" {
		return Result{Source: registerSource, Problem: "this tool needs the code a refusal named"}
	}
	for _, row := range refusal.Rows {
		if row.Code != code {
			continue
		}
		var built strings.Builder
		built.WriteString("- Code: " + row.Code + "\n")
		built.WriteString("- Owner: " + row.Owner + "\n")
		built.WriteString("- Emitted at: " + row.Site + "\n")
		built.WriteString("- Shape: " + string(row.Shape) + " — " + shapeMeans[row.Shape] + "\n")
		if row.Override != "" {
			built.WriteString("- Carried past by: " + row.Override + "\n")
			built.WriteString("- Commands: " + strconv.Itoa(row.Commands) + "\n")
		} else {
			built.WriteString("- Carried past by: no human verb; the refusal is answered by a corrected word or proof of who is asking\n")
		}
		if row.Pending != "" {
			built.WriteString("- Pending: the override is not yet in the tree (" + row.Pending + ")\n")
		}
		if standing := row.Standing(); standing != "" {
			built.WriteString("- H1 standing: " + string(standing) + " — " + standingMeans[standing] + "\n")
		}
		if row.Forward != "" {
			built.WriteString("- Forward command: metasystem " + row.Forward + "\n")
		}
		return bounded(registerSource, built.String(), 1, 1)
	}
	return Result{Source: registerSource, Problem: "the register has no row for " + code}
}
