package httpd

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/rulings"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/overview"
)

// The Decisions page's own last-visit entry, end to end through the route.
//
// Reading the page IS the visit, so the route records one and composes over
// the window it established. What matters here is whose window it is: the
// landing page keeps its own entry, and a read of Decisions must not move it.

// visiting is a server that answers Decisions and keeps both markers in one
// state root, through the same owner the engine wires.
func decisionsVisiting(root string, clock *time.Time) Info {
	info := decisionsInfo()
	info.Now = func() time.Time { return *clock }
	info.Visit = func(human string, now time.Time) (time.Time, bool, error) {
		return overview.Visit(root, human, now)
	}
	info.VisitDecisions = func(human string, now time.Time) (time.Time, bool, error) {
		return overview.VisitPage(root, overview.PageDecisions, human, now)
	}
	return info
}

func TestDecisionsRecordsItsOwnVisitAndLeavesOverviewsAlone(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	clock := overviewNow.Add(-2 * time.Hour)
	served := New(decisionsVisiting(root, &clock), loopback(), testBundle())

	// The first read is a first visit: a day back, and it says so.
	first := decisionsPage(t, served, "the first read")
	testutil.Expect(t, "the first visit's window", first.Visit.Since,
		clock.Add(-24*time.Hour).Format(time.RFC3339))
	testutil.Expect(t, "and that it is a first visit", first.Visit.First, true)

	// Two hours later, which is past the gap, the window is the end of the
	// visit before it — the read above.
	before := clock
	clock = overviewNow
	second := decisionsPage(t, served, "the second read")
	testutil.Expect(t, "the second visit's window", second.Visit.Since, before.Format(time.RFC3339))
	testutil.Expect(t, "which is no longer a first visit", second.Visit.First, false)

	// And Overview's own entry was never written, because nothing has read
	// Overview: a page that advanced it from here would move a boundary the
	// human never saw anything across.
	data, err := os.ReadFile(overview.VisitsPath(root))
	testutil.Require(t, "reading the marker file", err, nil)
	var held overview.Visits
	testutil.Require(t, "decoding the marker file", json.Unmarshal(data, &held), nil)
	testutil.Expect(t, "the landing page has no row", len(held.Humans), 0)
	testutil.Expect(t, "the page's own row is there", held.Pages[overview.PageDecisions][""].Seen,
		overviewNow.Format(time.RFC3339))
}

// A read that ends in a 500 is a page nobody saw, so it is not a visit. The
// marker is recorded after every reader that can fail: an advance here would
// spend this human's "new since your last visit" on a page that never
// rendered, and the window cannot be given back.
func TestAFailedReadOfDecisionsDoesNotAdvanceTheVisit(t *testing.T) {
	t.Parallel()

	for _, one := range []struct {
		name  string
		shape func(*Info)
	}{
		{
			name: "the channel reader fails",
			shape: func(info *Info) {
				info.Asks = func() ([]channel.Question, error) { return nil, errors.New("the channel file is unreadable") }
			},
		},
		{
			name: "the register reader fails",
			shape: func(info *Info) {
				info.Rulings = func() (rulings.Register, error) { return rulings.Register{}, errors.New("the register is unreadable") }
			},
		},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			clock := overviewNow
			info := decisionsVisiting(root, &clock)
			one.shape(&info)
			served := New(info, loopback(), testBundle())

			response := request(t, served, http.MethodGet, "/api/decisions", "127.0.0.1:7878", nil)
			testutil.Expect(t, "the status", response.Code, http.StatusInternalServerError)

			// Nothing was written at all: the first visit is still to come.
			_, err := os.ReadFile(overview.VisitsPath(root))
			testutil.Expect(t, "the marker file was never written", os.IsNotExist(err), true)
		})
	}
}

// A build with no marker store still answers a page: the marker is preference
// state, and losing it changes a window and nothing else.
func TestDecisionsWithNoMarkerStoreIsAFirstVisit(t *testing.T) {
	t.Parallel()

	served := New(decisionsInfo(), loopback(), testBundle())
	page := decisionsPage(t, served, "the read")

	testutil.Expect(t, "the window", page.Visit.Since,
		overviewNow.Add(-24*time.Hour).Format(time.RFC3339))
	testutil.Expect(t, "and that it is a first visit", page.Visit.First, true)
	// Which is a window, so the rows recorded inside it are new.
	newRows := 0
	for _, need := range page.NeedsYou {
		if need.New {
			newRows++
		}
	}
	testutil.Expect(t, "the seat's ask, two hours old, is new", newRows > 0, true)
}
