package act

import (
	"errors"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
)

// On separated roots a browser act hands the ledger fence the installation
// the server runs for, never the state root its ledger lives under.
func TestSessionActLedgerFenceSeparatedRoots(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	installation := stateroottest.Installation(t, t.TempDir())
	proof, err := humanauthority.SignedInSessionProof(bed.root, "Wido", testSession, "browser", fixtureNow)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := SignedIn(bed.root, installation, "Wido", testSession, proof)
	if err != nil {
		t.Fatal(err)
	}
	authority = bed.acting(t, authority)
	var fenced []stateroot.Installation
	authority.reads.fence = func(at stateroot.Installation) error {
		fenced = append(fenced, at)
		return errors.New("fence stub")
	}
	if err := authority.Withdraw("ui-fence", "separated roots"); err == nil || len(fenced) != 1 || fenced[0] != installation {
		t.Fatalf("withdraw = %v, fenced %q, want only %q", err, fenced, installation)
	}
}
