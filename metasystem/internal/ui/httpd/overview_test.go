package httpd

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/overview"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// The Overview route: one read of five answers this server already has.

// overviewNow is the clock the route is given, so a composed page can be read
// against an instant rather than against whenever the test ran.
var overviewNow = time.Date(2026, 9, 23, 14, 37, 0, 0, time.UTC)

// overviewInfo is a server that can answer the landing page: a project, a
// ledger, a marker, and a clock.
func overviewInfo(t *testing.T) (Info, *int) {
	t.Helper()
	visits := 0
	root := t.TempDir()
	return Info{
		Project: func() (project.Pane, error) { return describedPane(), nil },
		Observe: func() snapshot.Observation { return overviewObservation() },
		Now:     func() time.Time { return overviewNow },
		Visit: func(human string, now time.Time) (time.Time, bool, error) {
			visits++
			return overview.Visit(root, human, now)
		},
	}, &visits
}

func overviewObservation() snapshot.Observation {
	return snapshot.Observation{
		ObservedAt: overviewNow, State: snapshot.StateRead, StateRoot: "/work/repository",
		Tip: "c5d517f", CommittedAt: overviewNow.Add(-time.Minute), SyncMode: goal.SyncLocal,
		Tree: &goal.TreeGoals{
			Root:      &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "2", SyncMode: goal.SyncLocal, Revision: 1},
			Live:      map[string]*goal.GoalFile{"g1-s22": {Id: "g1-s22", State: goal.StateQueued, Intent: "The pane briefs", Priority: 2, Sequence: 1, Revision: 3}},
			Done:      map[string]*goal.GoalFile{},
			Abandoned: map[string]*goal.GoalFile{},
		},
		Admission: backlog.Admission{Answered: true, Ready: map[string]bool{}, Blocked: map[string]bool{}, Awaiting: map[string]bool{}, Refused: map[string]string{}},
		Fetch: snapshot.FetchState{
			Outcome: snapshot.OutcomeCurrent, FinishedAt: overviewNow.Add(-time.Second),
			Tip: "c5d517f", Detail: "already at the canonical tip",
			SucceededAt: overviewNow.Add(-time.Second), SucceededTip: "c5d517f",
		},
	}
}

// The route answers the composed page, with the project's records and the
// ledger's lanes in it, and records the visit while it does.
func TestOverviewPayload(t *testing.T) {
	t.Parallel()

	info, visits := overviewInfo(t)
	served := New(info, loopback(), testBundle())

	response := request(t, served, http.MethodGet, "/api/overview", "127.0.0.1:7878", nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "content type", response.Header().Get("Content-Type"), "application/json")
	var page overview.Page
	testutil.Require(t, "decode the response", json.Unmarshal(response.Body.Bytes(), &page), nil)
	testutil.Expect(t, "the schema version", page.SchemaVersion, overview.SchemaVersion)
	testutil.Expect(t, "when it was read", page.ReadAt, overviewNow.Format(time.RFC3339))
	testutil.Expect(t, "the first visit's window", page.Since, overviewNow.Add(-24*time.Hour).Format(time.RFC3339))
	testutil.Expect(t, "and that it says so", page.First, true)
	testutil.Expect(t, "the visit was recorded", *visits, 1)
	// The project's own records reached the page: the pane's one design is a
	// draft nobody has accepted.
	testutil.Expect(t, "drafts waiting on a human", page.NeedsYou.Drafts.Count, 0)
	testutil.Expect(t, "goals waiting on an approval", page.NeedsYou.Approvals.Count, 1)
	testutil.Expect(t, "a seat nothing proves asks for a sign-in", page.NeedsYou.SignIn, true)
	// The lane strip is the projection's own counts, with Done today beside
	// them.
	testutil.Expect(t, "the lane strip", page.Work.Lanes, []overview.Lane{
		{ID: "to-do", Count: 1}, {ID: "ready"}, {ID: "in-progress"},
		{ID: "review"}, {ID: "waiting"}, {ID: overview.LaneDoneToday},
	})
	testutil.Expect(t, "health", page.Health.OK, true)
}

// The page's health is judged on the same freshness the board's chip is: one
// rule, derived once, so that Overview and the Backlog cannot say different
// things about the same fetch loop.
func TestOverviewHealthReadsTheSameFreshnessTheBoardDoes(t *testing.T) {
	t.Parallel()

	for _, one := range []struct {
		name  string
		shape func(*snapshot.Observation)
		state snapshot.Freshness
		title string
		ok    bool
	}{
		{
			name:  "a look that landed on the accepted tip",
			shape: func(*snapshot.Observation) {},
			state: snapshot.FreshnessCurrent,
			ok:    true,
		},
		{
			name: "an accepted commit three hours old, which is no problem at all",
			shape: func(observation *snapshot.Observation) {
				observation.CommittedAt = observation.ObservedAt.Add(-3 * time.Hour)
			},
			state: snapshot.FreshnessCurrent,
			ok:    true,
		},
		{
			name: "a loop that has not landed a look for longer than the threshold",
			shape: func(observation *snapshot.Observation) {
				observation.Fetch.SucceededAt = observation.ObservedAt.Add(-2 * time.Hour)
			},
			state: snapshot.FreshnessBehind,
			title: "the last fetch of the canonical branch landed more than 30 minutes ago",
		},
		{
			name: "a loop whose last look failed",
			shape: func(observation *snapshot.Observation) {
				observation.Fetch.Outcome = snapshot.OutcomeFailed
				observation.Fetch.Tip, observation.Fetch.Detail = "", ""
				observation.Fetch.Message = "ssh: connect: host unreachable"
				observation.Fetch.Failures = 2
			},
			state: snapshot.FreshnessFailed,
			title: "ssh: connect: host unreachable (2 fetches in a row have failed)",
		},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()

			info, _ := overviewInfo(t)
			observation := overviewObservation()
			one.shape(&observation)
			info.Observe = func() snapshot.Observation { return observation }
			served := New(info, loopback(), testBundle())

			page := overviewPage(t, served, "the read")

			testutil.Expect(t, "the freshness the pill says", page.Health.Freshness, one.state)
			testutil.Expect(t, "health", page.Health.OK, one.ok)
			if one.ok {
				return
			}
			testutil.Require(t, "how many problems", page.Health.Problems.Count, 1)
			testutil.Expect(t, "the problem's title is the freshness detail",
				page.Health.Problems.Items[0].Title, one.title)
			testutil.Expect(t, "where it is fixed",
				page.Health.Problems.Items[0].Where, overview.Where{Kind: overview.WhereBacklog})
		})
	}
}

// A second read inside the visit is answered over the same window, because
// that is what the marker is for.
func TestOverviewReadAgainKeepsTheWindow(t *testing.T) {
	t.Parallel()

	info, _ := overviewInfo(t)
	served := New(info, loopback(), testBundle())

	first := overviewPage(t, served, "the first read")
	again := overviewPage(t, served, "the second read")

	testutil.Expect(t, "the window", again.Since, first.Since)
	testutil.Expect(t, "and that it is still a first visit", again.First, true)
}

// Every reader is required, and a reader that fails is the reason rather than
// a calm page that could not look.
func TestOverviewRefusesWhenAReaderCannotAnswer(t *testing.T) {
	t.Parallel()

	for _, one := range []struct {
		name   string
		info   func(Info) Info
		reason string
	}{
		{
			name:   "no project reader",
			info:   func(info Info) Info { info.Project = nil; return info },
			reason: "this engine was built without a project reader",
		},
		{
			name:   "no ledger reader",
			info:   func(info Info) Info { info.Observe = nil; return info },
			reason: "this engine was built without a ledger reader",
		},
		{
			name: "a project reader that fails",
			info: func(info Info) Info {
				info.Project = func() (project.Pane, error) { return project.Pane{}, errFailed }
				return info
			},
			reason: errFailed.Error(),
		},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()

			base, _ := overviewInfo(t)
			served := New(one.info(base), loopback(), testBundle())

			response := request(t, served, http.MethodGet, "/api/overview", "127.0.0.1:7878", nil)

			testutil.Require(t, "status", response.Code, http.StatusInternalServerError)
			var refusal struct {
				Error string `json:"error"`
			}
			testutil.Require(t, "decode the refusal", json.Unmarshal(response.Body.Bytes(), &refusal), nil)
			testutil.Expect(t, "the reason", refusal.Error, one.reason)
		})
	}
}

// The marker is preference state: a build that keeps none, and a marker that
// could not be written, both render the page over a first visit's window.
func TestOverviewWithoutAMarkerStillAnswers(t *testing.T) {
	t.Parallel()

	for _, one := range []struct {
		name  string
		visit func(string, time.Time) (time.Time, bool, error)
	}{
		{name: "a build that keeps none", visit: nil},
		{
			name: "a marker that could not be written",
			visit: func(_ string, now time.Time) (time.Time, bool, error) {
				return now.Add(-24 * time.Hour), true, errors.New("the marker is on a read-only volume")
			},
		},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()

			info, _ := overviewInfo(t)
			info.Visit = one.visit
			served := New(info, loopback(), testBundle())

			page := overviewPage(t, served, "the read")

			testutil.Expect(t, "the window", page.Since, overviewNow.Add(-24*time.Hour).Format(time.RFC3339))
			testutil.Expect(t, "and that it says so", page.First, true)
		})
	}
}

// Nothing lies beneath the resource, and it takes the same method policy every
// other read takes.
func TestOverviewIsMatchedExactlyAndReadsOnly(t *testing.T) {
	t.Parallel()

	info, _ := overviewInfo(t)
	served := New(info, loopback(), testBundle())

	for _, path := range []string{"/api/overview/x", "/api/overview/", "/api/overviews"} {
		response := request(t, served, http.MethodGet, path, "127.0.0.1:7878",
			map[string]string{"Accept": "application/json"})
		testutil.Expect(t, "the status for "+path, response.Code, http.StatusNotFound)
	}
	posted := post(t, served, "/api/overview", `{}`, nil)
	testutil.Expect(t, "posting to a read", posted.Code, http.StatusMethodNotAllowed)
}

// The marker the engine wires sits beside the sessions file the lifecycle
// slice owns. The two packages spell that directory separately, because taking
// the name across would close an import cycle, so the agreement is asserted
// here, where both are in scope.
func TestTheMarkerSitsInTheInterfacesOwnDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	testutil.Expect(t, "where the marker is written", overview.VisitsPath(root),
		filepath.Join(lifecycle.Dir(root), "visits.json"))
}

// overviewPage reads the route once. The label is the caller's, because the
// expectation register names each assertion once per test and one test reads
// this route twice.
func overviewPage(t *testing.T, served http.Handler, what string) overview.Page {
	t.Helper()
	response := request(t, served, http.MethodGet, "/api/overview", "127.0.0.1:7878", nil)
	testutil.Require(t, what+": status", response.Code, http.StatusOK)
	var page overview.Page
	testutil.Require(t, what+": decode the response", json.Unmarshal(response.Body.Bytes(), &page), nil)
	return page
}

func TestHumanDurationSpeaksLikeAPerson(t *testing.T) {
	t.Parallel()
	cases := map[time.Duration]string{30 * time.Minute: "30 minutes", 2 * time.Hour: "2 hours", 90 * time.Minute: "1 hour 30 minutes", 45 * time.Second: "45 seconds", time.Minute: "1 minute"}
	for d, want := range cases {
		if got := humanDuration(d); got != want {
			t.Errorf("humanDuration(%s) = %q, want %q", d, got, want)
		}
	}
}
