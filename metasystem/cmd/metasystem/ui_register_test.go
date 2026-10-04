package main

// Where the interface reads the rulings register from, and where it says that
// register is.
//
// The two are different roots and the page needs both. The register is read
// from the INSTALLATION, because that is the kit's memory home; a destination
// naming it is opened by the document reader, which resolves against the
// CHECKOUT. Reading from the checkout answered an empty register for a file
// that exists, and naming the installation's own path would answer a
// destination that opens nothing.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/rulings"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/fleet"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/notifications"
)

const registerFixture = "# Standing rulings register\n\n" +
	"| ID | Date | Decision | Evidence | Owner | Review |\n|---|---|---|---|---|---|\n" +
	"| R-1 | 2026-09-01 | The register is read from where it is | Given with g1-s46 | Wido |  |\n"

// The layout this interface most often serves: a checkout whose installation
// is a directory inside it. The register is under the installation, the
// checkout has none of its own, and the path the payload carries opens.
func TestTheRegisterIsReadFromTheInstallationAndNamedFromTheCheckout(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	installation := filepath.Join(checkout, "metasystem")
	plantRegister(t, installation)
	roots := lifecycle.Roots{Checkout: checkout, Installation: stateroottest.Installation(t, installation), StateRoot: stateroottest.State(t, checkout)}

	// Read from the installation, which is the root the wiring hands over.
	fromInstallation, err := rulings.Read(roots.Installation.Path())
	if err != nil {
		t.Fatalf("reading the register from the installation: %v", err)
	}
	testutil.Expect(t, "the installation's register has its row", len(fromInstallation.Rows), 1)

	// The root the wiring used to hand over, which is why the page showed
	// zero rulings: no file, no rows, and no error either.
	fromCheckout, err := rulings.Read(roots.Checkout)
	if err != nil {
		t.Fatalf("reading the register from the checkout: %v", err)
	}
	testutil.Expect(t, "the checkout has no register of its own", len(fromCheckout.Rows), 0)

	// And the path the payload carries, which the document reader opens
	// against the checkout.
	named := registerFromCheckout(roots)
	testutil.Expect(t, "the payload names the register from the checkout", named, "metasystem/memory/rulings.md")
	if _, statErr := os.Stat(filepath.Join(roots.Checkout, filepath.FromSlash(named))); statErr != nil {
		t.Fatalf("the destination the payload names does not open from the checkout: %v", statErr)
	}
}

// The self-hosted layout, where the two roots are one directory, keeps the
// path every reader saw before this field existed.
func TestTheSelfHostedLayoutNamesTheRegisterWhereItAlwaysWas(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	plantRegister(t, root)
	roots := lifecycle.Roots{Checkout: root, Installation: stateroottest.Installation(t, root), StateRoot: stateroottest.State(t, root)}

	named := registerFromCheckout(roots)

	testutil.Expect(t, "the register's own path", named, "memory/rulings.md")
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(named))); err != nil {
		t.Fatalf("the destination the payload names does not open: %v", err)
	}
}

// The self-hosted layout keeps the steward's records beneath the state root,
// a directory inside the Git checkout. A notice, a health verdict and an open
// question written where the steward and the channel write them are what the
// interface's readers return, and the checkout holds none of them.
func TestTheSelfHostedLayoutReadsWhatTheStewardWroteBeneathTheStateRoot(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	state := filepath.Join(checkout, "metasystem")
	roots := lifecycle.Roots{Checkout: checkout, Installation: stateroottest.Installation(t, state), StateRoot: stateroottest.State(t, state)}
	plant := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	plant(steward.NotificationJournalPath(state),
		`{"id":"n1","at":"2026-10-03T08:00:00Z","message":"the seat is waiting","source":"steward","delivered":true}`+"\n")
	plant(steward.HealthRecordPath(state),
		`{"verdict":{"observedAt":"2026-10-03T08:00:00Z","aggregate":"alive","roles":[]}}`)
	plant(filepath.Join(state, "artifacts", "agents", "channel", "questions", "q1.json"),
		`{"id":"q1","goal":"g","openedAt":"2026-10-03T08:00:00Z","state":"open"}`)

	notices, err := notifications.Page(notificationJournal(roots), 10, "")
	if err != nil {
		t.Fatalf("reading the journal: %v", err)
	}
	testutil.Expect(t, "the notice the steward journalled", len(notices), 1)
	health := seatHealth(roots)
	if health == nil {
		t.Fatal("the recorded health verdict was not read")
	}
	testutil.Expect(t, "the recorded verdict", health.State, "alive")
	testutil.Expect(t, "the open question", len(openAsks(roots)), 1)

	if _, statErr := os.Stat(steward.NotificationJournalPath(checkout)); !os.IsNotExist(statErr) {
		t.Fatalf("the checkout has a journal of its own: %v", statErr)
	}
	testutil.Expect(t, "the checkout records no verdict", fleet.ReadHealth(checkout) == nil, true)
}

// An installation outside the checkout has no checkout-relative path at all.
// Naming one that walked out of the repository would be a destination that
// cannot open dressed up as one that can, so nothing is named.
func TestAnInstallationOutsideTheCheckoutNamesNoRegisterPath(t *testing.T) {
	t.Parallel()
	roots := lifecycle.Roots{
		Checkout: filepath.Join(t.TempDir(), "app"), Installation: stateroottest.Installation(t, filepath.Join(t.TempDir(), "kit")),
	}

	testutil.Expect(t, "nothing is named", registerFromCheckout(roots), "")
}

func plantRegister(t *testing.T, installation string) {
	t.Helper()
	path := rulings.Path(installation)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(registerFixture), 0o644); err != nil {
		t.Fatal(err)
	}
}
