package httpd

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/fleet"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/overview"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// The fleet at the boundary: what the resource answers, and that the flag the
// board and the Overview carry is joined from the SAME observation their rows
// were projected from. What a standing means is proved in internal/ui/fleet.

var fleetNow = time.Date(2026, 9, 25, 14, 37, 0, 0, time.UTC)

// fleetSeen records every observation the fleet composer was handed, so a
// test can prove that the board's rows and their holders came from one.
type fleetSeen struct {
	observations []snapshot.Observation
	boards       []backlog.Board
	calls        int
	fail         error
}

func (f *fleetSeen) compose(observed snapshot.Observation, board backlog.Board, _ time.Time) (fleet.Page, error) {
	f.calls++
	f.observations = append(f.observations, observed)
	f.boards = append(f.boards, board)
	if f.fail != nil {
		return fleet.Page{}, f.fail
	}
	return fleet.Compose(fleet.Inputs{
		This: "m1u",
		Presence: seat.Copy{
			Records: map[string]seat.Record{
				"m1c": {
					PresenceSchema: seat.RecordSchema, Machine: "m1c", RepoIdentity: "r", Generation: 4,
					Engine: "3f9c1e2", ArmedLineage: seat.NoLease, TickSeconds: 600,
					TickAt: seat.FormatTime(fleetNow.Add(-6 * time.Hour)),
				},
			},
			Malformed: map[string]string{},
		},
		Copy:        fleet.Copy{Source: fleet.SourceInterface, SucceededAt: seat.FormatTime(fleetNow)},
		Observation: observed, Board: board,
		Previous: map[string]seat.Observation{
			"m1c": {Standing: seat.Unreachable, Since: seat.FormatTime(fleetNow.Add(-5 * time.Hour))},
		},
		Window: seat.DefaultStaleMinutes * time.Minute,
	}, fleetNow), nil
}

// silentHolder is a ledger with one In Progress goal held by m1c.
func silentHolder() snapshot.Observation {
	claimed := &goal.GoalFile{
		Id: "tests-parallel-and-deterministic", State: goal.StateClaimed,
		Intent: "Run the suite in parallel, deterministically.", Origin: "main",
		NextStep: "Land the runner.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 9,
		Claimed: &goal.ClaimRecord{Machine: "m1c", Lineage: "coordinator", At: "2026-09-25T08:00:00Z"},
		History: []goal.HistoryLine{{At: "2026-09-25T08:00:00Z", Opid: "op-1", Verb: "claim", Actor: "m1c+coordinator"}},
	}
	tree := &goal.TreeGoals{
		Root:      &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "2", SyncMode: goal.SyncLocal, Revision: 1},
		Live:      map[string]*goal.GoalFile{"tests-parallel-and-deterministic": claimed},
		Done:      map[string]*goal.GoalFile{},
		Abandoned: map[string]*goal.GoalFile{},
	}
	horizon := goal.NewApprovalHorizon(tree, fleetNow)
	return snapshot.Observation{
		ObservedAt: fleetNow, State: snapshot.StateRead, Tip: "5b9d958",
		Tree: tree, Horizon: horizon, SyncMode: goal.SyncLocal,
		Admission: backlog.Admit(goal.Projection{Root: "/work/repository", Tree: tree, Horizon: horizon}),
	}
}

func TestTheFleetResourceAnswersTheComposedPage(t *testing.T) {
	t.Parallel()
	seen := &fleetSeen{}
	served := New(Info{Observe: silentHolder, Fleet: seen.compose, Now: func() time.Time { return fleetNow }},
		loopback(), testBundle())

	response := request(t, served, http.MethodGet, fleetPath, "127.0.0.1:7878", nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "content type", response.Header().Get("Content-Type"), "application/json")
	var page fleet.Page
	testutil.Require(t, "decode", json.Unmarshal(response.Body.Bytes(), &page), nil)
	testutil.Expect(t, "the schema a reader parses", page.SchemaVersion, fleet.SchemaVersion)
	testutil.Expect(t, "the claims name the tip they were read at", page.Claims.Tip, "5b9d958")
	testutil.Expect(t, "one silent machine is reported", len(page.Machines), 1)
	testutil.Expect(t, "and it is unreachable", page.Machines[0].Standing, "unreachable")
}

func TestABuildWithNoFleetReaderSaysSo(t *testing.T) {
	t.Parallel()
	served := New(Info{Observe: silentHolder}, loopback(), testBundle())

	response := request(t, served, http.MethodGet, fleetPath, "127.0.0.1:7878", nil)

	testutil.Expect(t, "status", response.Code, http.StatusInternalServerError)
	testutil.Expect(t, "the reason a human acts on",
		strings.Contains(response.Body.String(), "this engine was built without a fleet reader"), true)
}

func TestNothingLivesBeneathTheFleetResource(t *testing.T) {
	t.Parallel()
	seen := &fleetSeen{}
	served := New(Info{Observe: silentHolder, Fleet: seen.compose}, loopback(), testBundle())

	beneath := request(t, served, http.MethodGet, fleetPath+"/m1c", "127.0.0.1:7878", nil)
	written := request(t, served, http.MethodPost, fleetPath, "127.0.0.1:7878", nil)

	testutil.Expect(t, "an unserved path under /api", beneath.Code, http.StatusNotFound)
	testutil.Expect(t, "a write is refused", written.Code, http.StatusMethodNotAllowed)
}

/* ------------------------------------------------ the flag on the board -- */

func TestTheBoardCarriesTheHolderFromTheObservationItsRowsCameFrom(t *testing.T) {
	t.Parallel()
	seen := &fleetSeen{}
	observed := silentHolder()
	served := New(Info{
		Observe: func() snapshot.Observation { return observed },
		Fleet:   seen.compose, Now: func() time.Time { return fleetNow },
	}, loopback(), testBundle())

	response := request(t, served, http.MethodGet, backlogPath, "127.0.0.1:7878", nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	var payload backlogPayload
	testutil.Require(t, "decode", json.Unmarshal(response.Body.Bytes(), &payload), nil)
	held := holderIn(payload.Rows, "tests-parallel-and-deterministic")
	testutil.Require(t, "the claimed row carries a holder", held != nil, true)
	testutil.Expect(t, "naming the machine", held.Machine, "m1c")
	testutil.Expect(t, "its standing", held.Standing, "unreachable")
	testutil.Expect(t, "when the silence began", held.Since, seat.FormatTime(fleetNow.Add(-5*time.Hour)))
	// The words carry no clock: the instant is the field beside them, and
	// the browser renders it in the viewer's own zone.
	testutil.Expect(t, "and the words the card shows", held.Flag, "held by m1c, unreachable")
	testutil.Require(t, "the fleet was composed once", seen.calls, 1)
	testutil.Expect(t, "from the observation the rows were projected from",
		seen.observations[0].Tip, observed.Tip)
	testutil.Expect(t, "and over those same rows",
		len(seen.boards[0].Rows), len(payload.Rows))
}

func TestAFleetReaderThatFailsCostsTheBoardAFlagAndNotARow(t *testing.T) {
	t.Parallel()
	seen := &fleetSeen{fail: errors.New("the presence namespace could not be read")}
	served := New(Info{Observe: silentHolder, Fleet: seen.compose, Now: func() time.Time { return fleetNow }},
		loopback(), testBundle())

	response := request(t, served, http.MethodGet, backlogPath, "127.0.0.1:7878", nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	var payload backlogPayload
	testutil.Require(t, "decode", json.Unmarshal(response.Body.Bytes(), &payload), nil)
	testutil.Expect(t, "the rows are all there", len(payload.Rows) > 0, true)
	testutil.Expect(t, "and none of them carries a holder",
		holderIn(payload.Rows, "tests-parallel-and-deterministic"), (*holderPayload)(nil))
}

/* --------------------------------------------- the flag beside the seat -- */

func TestTheOverviewCarriesTheHolderBesideTheSeat(t *testing.T) {
	t.Parallel()
	seen := &fleetSeen{}
	served := New(Info{
		Observe: silentHolder, Fleet: seen.compose, Now: func() time.Time { return fleetNow },
		Project: func() (project.Pane, error) { return project.Pane{SchemaVersion: project.SchemaVersion}, nil },
	}, loopback(), testBundle())

	response := request(t, served, http.MethodGet, overviewPath, "127.0.0.1:7878", nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	var page overview.Page
	testutil.Require(t, "decode", json.Unmarshal(response.Body.Bytes(), &page), nil)
	testutil.Require(t, "one goal is in progress", len(page.Work.InProgress), 1)
	claimed := page.Work.InProgress[0]
	testutil.Expect(t, "the seat is still named", claimed.Seat.Machine, "m1c")
	testutil.Require(t, "and the holder beside it", claimed.Holder != nil, true)
	testutil.Expect(t, "with its standing", claimed.Holder.Standing, "unreachable")
	testutil.Expect(t, "and the flag the row shows", claimed.Holder.Flag, "held by m1c, unreachable")
	testutil.Expect(t, "with the instant beside it for the browser to render",
		claimed.Holder.Since, seat.FormatTime(fleetNow.Add(-5*time.Hour)))
}

/* -------------------------------------------- the one event on the stream -- */

// The stream is the server's one explicit connection signal, and the same
// registration carries the fleet event back. Without the event a mounted
// Fleet page would read once and never learn of new presence.
func TestAnOpenStreamIsAConnectionAndIsToldOfEveryAttempt(t *testing.T) {
	t.Parallel()
	watch := fleet.NewWatch()
	path := journalWith(t, journalLine(t, "S1", "steward", "already in the history", true))

	testutil.Expect(t, "no browser, no connection", watch.Connected(), false)
	stream := streamedFrom(t, Info{NotificationJournal: path, Watch: watch}, "")
	testutil.Expect(t, "the stream opens with a comment",
		strings.HasPrefix(stream.line(t), ": "), true)
	testutil.Expect(t, "and a retry hint",
		strings.HasPrefix(stream.line(t), "retry: "), true)
	testutil.Expect(t, "an open stream is a connection", watch.Connected(), true)

	watch.Announce()
	id, name, data := stream.event(t)
	testutil.Expect(t, "the fleet event carries no id", id, "")
	testutil.Expect(t, "it is named for what the page re-reads", name, "fleet")
	testutil.Expect(t, "and carries no payload", data, "{}")

	stream.stop()
}

func holderIn(rows []boardRow, id string) *holderPayload {
	for _, row := range rows {
		if row.ID == id {
			return row.Holder
		}
	}
	return nil
}
