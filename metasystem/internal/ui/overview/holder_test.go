package overview

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The flag beside the seat: a claim whose machine has gone quiet says so on
// the row that names the machine, and a machine nothing knows about leaves
// the row exactly as it was. What a standing means is proved in
// internal/ui/fleet; this is the join and nothing else.
func TestAnInProgressRowCarriesItsHoldersStandingWhereOneIsKnown(t *testing.T) {
	t.Parallel()

	claim := func(in *Inputs) {
		claimed := goalRow("g1-s15", backlog.LaneInProgress, "The Decisions section")
		claimed.Claim = &backlog.Claim{Machine: "m1c", Lineage: "coordinator", At: ago(5 * time.Hour)}
		in.Rows = []backlog.Row{claimed}
	}
	silent := composed(t, func(in *Inputs) {
		claim(in)
		in.Holders = map[string]Holder{"m1c": {
			Machine: "m1c", Standing: "unreachable", Since: ago(6 * time.Hour),
			Flag: "held by m1c, unreachable since 08:37",
		}}
	})
	unknown := composed(t, claim)

	testutil.Require(t, "the claimed row is there", len(silent.Work.InProgress), 1)
	testutil.Require(t, "with a holder beside its seat", silent.Work.InProgress[0].Holder != nil, true)
	testutil.Expect(t, "naming the standing", silent.Work.InProgress[0].Holder.Standing, "unreachable")
	testutil.Expect(t, "and the words the row shows",
		silent.Work.InProgress[0].Holder.Flag, "held by m1c, unreachable since 08:37")
	testutil.Expect(t, "the seat itself is untouched", silent.Work.InProgress[0].Seat,
		Seat{Machine: "m1c", Lineage: "coordinator"})
	testutil.Require(t, "a build that read no presence still shows the row",
		len(unknown.Work.InProgress), 1)
	testutil.Expect(t, "with no holder on it", unknown.Work.InProgress[0].Holder, (*Holder)(nil))
}
