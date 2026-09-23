package httpd

import (
	"encoding/json"
	"net"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

var (
	observedAt  = time.Date(2026, 9, 21, 19, 5, 12, 0, time.UTC)
	committedAt = time.Date(2026, 9, 21, 18, 39, 42, 0, time.UTC)
)

func observedTree() *goal.TreeGoals {
	tree := &goal.TreeGoals{
		Root:      &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "2", SyncMode: goal.SyncLocal, Revision: 1},
		Live:      map[string]*goal.GoalFile{},
		Done:      map[string]*goal.GoalFile{},
		Abandoned: map[string]*goal.GoalFile{},
	}
	claimed := routeGoal("running", goal.StateClaimed)
	claimed.Claimed = &goal.ClaimRecord{Machine: "m1e", Lineage: "coordinator", At: "2026-09-20T09:00:00Z"}
	tree.Live["running"] = claimed
	tree.Live["waiting"] = routeGoal("waiting", goal.StateQueued)
	tree.Done["finished"] = routeGoal("finished", goal.StateDone)
	return tree
}

func routeGoal(id, state string) *goal.GoalFile {
	return &goal.GoalFile{
		Id: id, State: state, Intent: "Do " + id, Origin: "main",
		NextStep: "Start " + id + ".", OpenedAt: "2026-08-23T00:00:00Z", Revision: 9,
		History: []goal.HistoryLine{{At: "2026-09-20T09:00:00Z", Opid: "op-1", Verb: "claim", Actor: "m1e+coordinator"}},
	}
}

func readObservation() snapshot.Observation {
	tree := observedTree()
	horizon := goal.NewApprovalHorizon(tree, observedAt)
	return snapshot.Observation{
		ObservedAt: observedAt, StateRoot: "/work/repository/metasystem",
		State: snapshot.StateRead, Tip: "c5d517f427e35e19c2944ab9aefee4d9c9cd9e6c",
		CommittedAt: committedAt, Tree: tree, Horizon: horizon, SyncMode: goal.SyncLocal,
		Admission: backlog.Admit(goal.Projection{Root: "/work/repository/metasystem", Tree: tree, Horizon: horizon}),
		Fetch:     landedFetch(),
	}
}

// landedFetch is a freshness loop whose last look landed a second ago, on the
// tip this clone has accepted: the state everything else is a departure from.
func landedFetch() snapshot.FetchState {
	return snapshot.FetchState{
		Outcome:   snapshot.OutcomeCurrent,
		StartedAt: observedAt.Add(-2 * time.Second), FinishedAt: observedAt.Add(-time.Second),
		Tip: "c5d517f427e35e19c2944ab9aefee4d9c9cd9e6c", Detail: "already at the canonical tip",
		SucceededAt:  observedAt.Add(-time.Second),
		SucceededTip: "c5d517f427e35e19c2944ab9aefee4d9c9cd9e6c",
		Failures:     0, Cadence: snapshot.CadenceConnected, NextAt: observedAt.Add(5 * time.Second),
	}
}

func servedBacklog(t *testing.T, observe func() snapshot.Observation) http.Handler {
	t.Helper()
	bound := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7878}
	return New(Info{Observe: observe}, bound, testBundle())
}

func decodeBacklog(t *testing.T, response *http.Response) map[string]json.RawMessage {
	t.Helper()
	var payload map[string]json.RawMessage
	testutil.Require(t, "decode", json.NewDecoder(response.Body).Decode(&payload), nil)
	return payload
}

func keysOf(object map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func TestBacklogPayload(t *testing.T) {
	t.Parallel()

	calls := 0
	served := servedBacklog(t, func() snapshot.Observation {
		calls++
		return readObservation()
	})

	response := request(t, served, http.MethodGet, "/api/backlog", "127.0.0.1:7878", nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "content type", response.Header().Get("Content-Type"), "application/json")

	payload := decodeBacklog(t, response.Result())
	testutil.Expect(t, "the payload's field names", keysOf(payload), []string{
		"admission", "authority", "budgetDefaults", "closed", "counts", "draft",
		"ledger", "observedAt", "rows", "schemaVersion", "workingTree",
	})

	var schemaVersion int
	testutil.Require(t, "decode the schema version", json.Unmarshal(payload["schemaVersion"], &schemaVersion), nil)
	testutil.Expect(t, "schema version", schemaVersion, 1)

	var observed string
	testutil.Require(t, "decode the observation time", json.Unmarshal(payload["observedAt"], &observed), nil)
	testutil.Expect(t, "observed at", observed, "2026-09-21T19:05:12Z")

	var ledger map[string]json.RawMessage
	testutil.Require(t, "decode the ledger", json.Unmarshal(payload["ledger"], &ledger), nil)
	testutil.Expect(t, "the ledger's field names", keysOf(ledger), []string{
		"committedAt", "fetch", "freshness", "message", "problems", "stale", "staleAfterSeconds", "state", "stateRoot", "syncMode", "tip",
	})

	var fetch map[string]json.RawMessage
	testutil.Require(t, "decode the fetch clause", json.Unmarshal(ledger["fetch"], &fetch), nil)
	testutil.Expect(t, "the fetch clause's field names", keysOf(fetch), []string{
		"cadence", "detail", "failures", "finishedAt", "message", "nextAt", "outcome", "startedAt",
		"succeededAt", "succeededTip", "tip",
	})

	var freshness map[string]json.RawMessage
	testutil.Require(t, "decode the freshness", json.Unmarshal(ledger["freshness"], &freshness), nil)
	testutil.Expect(t, "the freshness's field names", keysOf(freshness), []string{"detail", "since", "state"})

	var admission map[string]json.RawMessage
	testutil.Require(t, "decode the admission", json.Unmarshal(payload["admission"], &admission), nil)
	testutil.Expect(t, "the admission's field names", keysOf(admission), []string{"answered", "message"})

	var workingTree map[string]json.RawMessage
	testutil.Require(t, "decode the working tree", json.Unmarshal(payload["workingTree"], &workingTree), nil)
	testutil.Expect(t, "the working tree's field names", keysOf(workingTree), []string{"archivedFiles", "liveFiles"})

	// Two names an earlier shape carried and this one must not: nothing
	// records when a clone accepted a tip, and the drafts directory has no
	// reader to count.
	for _, absent := range []string{"draftFiles", "acceptedAt"} {
		if _, present := payload[absent]; present {
			t.Fatalf("the payload carries %s", absent)
		}
		if _, present := ledger[absent]; present {
			t.Fatalf("the ledger carries %s", absent)
		}
	}

	second := request(t, served, http.MethodGet, "/api/backlog", "127.0.0.1:7878", nil)
	testutil.Expect(t, "the second status", second.Code, http.StatusOK)
	testutil.Expect(t, "observations", calls, 2)
}

func TestBacklogCarriesTheLedgersOwnFacts(t *testing.T) {
	t.Parallel()

	served := servedBacklog(t, readObservation)
	response := request(t, served, http.MethodGet, "/api/backlog", "127.0.0.1:7878", nil)
	testutil.Require(t, "status", response.Code, http.StatusOK)

	var payload backlogPayload
	testutil.Require(t, "decode", json.Unmarshal(response.Body.Bytes(), &payload), nil)

	testutil.Expect(t, "state", payload.Ledger.State, "read")
	testutil.Expect(t, "tip", payload.Ledger.Tip, "c5d517f427e35e19c2944ab9aefee4d9c9cd9e6c")
	testutil.Expect(t, "committed at", payload.Ledger.CommittedAt, "2026-09-21T18:39:42Z")
	testutil.Expect(t, "stale", payload.Ledger.Stale, false)
	testutil.Expect(t, "stale after", payload.Ledger.StaleAfterSeconds, 1800)
	testutil.Expect(t, "sync mode", payload.Ledger.SyncMode, goal.SyncLocal)
	testutil.Expect(t, "state root", payload.Ledger.StateRoot, "/work/repository/metasystem")
	testutil.Expect(t, "problems", payload.Ledger.Problems, []string{})
	testutil.Expect(t, "the fetch clause", payload.Ledger.Fetch, fetchPayload{
		Outcome: "current", StartedAt: "2026-09-21T19:05:10Z", FinishedAt: "2026-09-21T19:05:11Z",
		Tip: "c5d517f427e35e19c2944ab9aefee4d9c9cd9e6c", Detail: "already at the canonical tip",
		SucceededAt:  "2026-09-21T19:05:11Z",
		SucceededTip: "c5d517f427e35e19c2944ab9aefee4d9c9cd9e6c",
		Failures:     0, Cadence: "connected", NextAt: "2026-09-21T19:05:17Z",
	})
	testutil.Expect(t, "admission", payload.Admission, admissionPayload{Answered: true})
	testutil.Expect(t, "live rows", len(payload.Rows), 2)
	testutil.Expect(t, "closed rows", len(payload.Closed), 1)
	testutil.Expect(t, "the claimed row", payload.Rows[0].Ref, backlog.Ref{Kind: "goal", ID: "running", Revision: 9})
	testutil.Expect(t, "the claimed lane", payload.Rows[0].Lane, backlog.LaneInProgress)
	testutil.Expect(t, "the claimed phase", payload.Rows[0].Phase, backlog.PhaseNotRecorded)
	testutil.Expect(t, "the claimed gaps", payload.Rows[0].Gaps, []string{"phase not recorded"})
	testutil.Expect(t, "the in-progress count", payload.Counts[backlog.LaneInProgress], 1)
	testutil.Expect(t, "the draft statement", payload.Draft.Statement, backlog.DraftStatement)
}

// The freshness rule: this interface judges how current it is by its own
// fetch loop, and by nothing else.
//
// The commit's age is deliberately absent from every case below. A tree
// nobody has committed to for three hours is a quiet project, and a warning
// about it is a warning Refresh cannot clear, because no fetch can make the
// last commit younger.
func TestFreshnessIsJudgedByTheFetchLoop(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		shape   func(*snapshot.Observation)
		state   snapshot.Freshness
		since   string
		detail  string
		wantAge bool
	}{
		{
			name:   "a look that landed a second ago on the accepted tip",
			shape:  func(*snapshot.Observation) {},
			state:  snapshot.FreshnessCurrent,
			since:  "2026-09-21T19:05:11Z",
			detail: "already at the canonical tip",
		},
		{
			name: "an accepted tree older than the threshold, which is not a freshness at all",
			shape: func(observation *snapshot.Observation) {
				observation.CommittedAt = observation.ObservedAt.Add(-3 * time.Hour)
			},
			state:   snapshot.FreshnessCurrent,
			since:   "2026-09-21T19:05:11Z",
			detail:  "already at the canonical tip",
			wantAge: true,
		},
		{
			name: "a loop that has never landed a look",
			shape: func(observation *snapshot.Observation) {
				observation.Fetch = snapshot.FetchState{Outcome: snapshot.OutcomeNever, Cadence: snapshot.CadenceConnected}
			},
			state:  snapshot.FreshnessBehind,
			detail: "no fetch of the canonical branch has completed on this clone yet",
		},
		{
			name: "a look that landed longer ago than the threshold",
			shape: func(observation *snapshot.Observation) {
				observation.Fetch.SucceededAt = observation.ObservedAt.Add(-90 * time.Minute)
			},
			state:  snapshot.FreshnessBehind,
			since:  "2026-09-21T17:35:12Z",
			detail: "the last fetch of the canonical branch landed more than 30 minutes ago",
		},
		{
			name: "a look that found a tip this clone has not caught up with",
			shape: func(observation *snapshot.Observation) {
				observation.Fetch.SucceededTip = "9f3c1ab7d20e4c5f8a1b2c3d4e5f60718293a4b5"
			},
			state:  snapshot.FreshnessBehind,
			since:  "2026-09-21T19:05:11Z",
			detail: "the last fetch found the canonical branch at 9f3c1ab and this clone has accepted c5d517f",
		},
		{
			name: "a look that failed, with how many failed before it",
			shape: func(observation *snapshot.Observation) {
				observation.Fetch.Outcome = snapshot.OutcomeFailed
				observation.Fetch.Tip, observation.Fetch.Detail = "", ""
				observation.Fetch.Message = "ssh: connect: host unreachable"
				observation.Fetch.Failures = 3
			},
			state:  snapshot.FreshnessFailed,
			since:  "2026-09-21T19:05:11Z",
			detail: "ssh: connect: host unreachable (3 fetches in a row have failed)",
		},
		{
			name: "one look that failed, which says the cause and counts nothing",
			shape: func(observation *snapshot.Observation) {
				observation.Fetch.Outcome = snapshot.OutcomeFailed
				observation.Fetch.Message = "ssh: connect: host unreachable"
				observation.Fetch.Failures = 1
			},
			state:  snapshot.FreshnessFailed,
			since:  "2026-09-21T19:05:11Z",
			detail: "ssh: connect: host unreachable",
		},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()

			observation := readObservation()
			one.shape(&observation)
			served := servedBacklog(t, func() snapshot.Observation { return observation })

			var payload backlogPayload
			response := request(t, served, http.MethodGet, "/api/backlog", "127.0.0.1:7878", nil)
			testutil.Require(t, "decode", json.Unmarshal(response.Body.Bytes(), &payload), nil)

			testutil.Expect(t, "the freshness", payload.Ledger.Freshness, freshnessPayload{
				State: one.state, Since: one.since, Detail: one.detail,
			})
			// The alias every reader written against the old shape still
			// parses: not current, whatever the reason.
			testutil.Expect(t, "stale, which mirrors the state",
				payload.Ledger.Stale, one.state != snapshot.FreshnessCurrent)
			if one.wantAge {
				testutil.Expect(t, "the commit's age, reported and never warned about",
					payload.Ledger.CommittedAt, "2026-09-21T16:05:12Z")
			}
		})
	}
}

// A tick in flight is not a clone that has never fetched: the look that
// landed before it is still the last thing this clone knows.
func TestAFetchInFlightKeepsTheFreshnessTheLastLookLeft(t *testing.T) {
	t.Parallel()

	observation := readObservation()
	observation.Fetch.Outcome = snapshot.OutcomeRunning
	observation.Fetch.FinishedAt = time.Time{}
	served := servedBacklog(t, func() snapshot.Observation { return observation })

	var payload backlogPayload
	response := request(t, served, http.MethodGet, "/api/backlog", "127.0.0.1:7878", nil)
	testutil.Require(t, "decode", json.Unmarshal(response.Body.Bytes(), &payload), nil)

	testutil.Expect(t, "the state", payload.Ledger.Freshness.State, snapshot.FreshnessCurrent)
	testutil.Expect(t, "measured from the look that landed", payload.Ledger.Freshness.Since, "2026-09-21T19:05:11Z")
}

// Refresh asks whether this clone is current now, so it looks and then
// observes: an answer composed from a loop that last looked half a cadence
// ago would be an answer about a moment that has passed.
func TestRefreshLooksBeforeItObserves(t *testing.T) {
	t.Parallel()

	t.Run("the ask that carries the query", func(t *testing.T) {
		t.Parallel()

		order := []string{}
		bound := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7878}
		served := New(Info{
			Observe: func() snapshot.Observation {
				order = append(order, "observe")
				return readObservation()
			},
			Fetch: func() { order = append(order, "fetch") },
		}, bound, testBundle())

		response := request(t, served, http.MethodGet, "/api/backlog?fetch=1", "127.0.0.1:7878", nil)

		testutil.Expect(t, "status", response.Code, http.StatusOK)
		testutil.Expect(t, "what the request did, in order", order, []string{"fetch", "observe"})
	})

	t.Run("every other read, which observes and starts nothing", func(t *testing.T) {
		t.Parallel()

		order := []string{}
		bound := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7878}
		served := New(Info{
			Observe: func() snapshot.Observation {
				order = append(order, "observe")
				return readObservation()
			},
			Fetch: func() { order = append(order, "fetch") },
		}, bound, testBundle())

		response := request(t, served, http.MethodGet, "/api/backlog", "127.0.0.1:7878", nil)

		testutil.Expect(t, "status", response.Code, http.StatusOK)
		testutil.Expect(t, "what the request did", order, []string{"observe"})
	})

	t.Run("a build that cannot look, which still answers", func(t *testing.T) {
		t.Parallel()

		served := servedBacklog(t, readObservation)

		response := request(t, served, http.MethodGet, "/api/backlog?fetch=1", "127.0.0.1:7878", nil)

		testutil.Expect(t, "status", response.Code, http.StatusOK)
	})
}

// TestBacklogAnswersEveryLedgerState: a ledger that cannot be read is still
// an answer from the workspace, so it is a 200 carrying why.
func TestBacklogAnswersEveryLedgerState(t *testing.T) {
	t.Parallel()

	live, archived := 155, 430
	cases := []struct {
		name        string
		observation snapshot.Observation
		wantMessage string
	}{
		{
			name: "a clone that has not fetched the ref",
			observation: snapshot.Observation{
				ObservedAt: observedAt, StateRoot: "/work", State: snapshot.StateAbsent,
				LiveFiles: &live, ArchivedFiles: &archived,
				Fetch: snapshot.FetchState{Outcome: snapshot.OutcomeNever, Cadence: snapshot.CadenceConnected, NextAt: observedAt.Add(5 * time.Second)},
			},
		},
		{
			name: "a ref whose commit carries no ledger",
			observation: snapshot.Observation{
				ObservedAt: observedAt, StateRoot: "/work", State: snapshot.StateNoLedger, Tip: "abc",
				LiveFiles: &live, ArchivedFiles: &archived,
				Fetch: snapshot.FetchState{Outcome: snapshot.OutcomeFailed, Message: "the accepted tree's identity cannot be read"},
			},
		},
		{
			name: "a ref that cannot be read",
			observation: snapshot.Observation{
				ObservedAt: observedAt, StateRoot: "/work", State: snapshot.StateBroken,
				Message: "the accepted ref's file exists but git reports no valid ref",
				Fetch:   snapshot.FetchState{Outcome: snapshot.OutcomeFailed, Message: "the accepted ref CAS lost five times"},
			},
			wantMessage: "the accepted ref's file exists but git reports no valid ref",
		},
		{
			name: "a tree the at-rest rules refuse",
			observation: snapshot.Observation{
				ObservedAt: observedAt, StateRoot: "/work", State: snapshot.StateUnreadable, Tip: "abc",
				Message:  "the ledger tree at abc does not parse:\nplans/goals/torn.md: missing Integrity line",
				Problems: []goal.Problem{"plans/goals/torn.md: missing Integrity line"},
				Fetch:    snapshot.FetchState{Outcome: snapshot.OutcomeCurrent, Detail: "already at the canonical tip"},
			},
			wantMessage: "the ledger tree at abc does not parse:\nplans/goals/torn.md: missing Integrity line",
		},
		{
			name: "a configuration the engine will not project from",
			observation: snapshot.Observation{
				ObservedAt: observedAt, StateRoot: "/work", State: snapshot.StateRefused, Tip: "abc",
				Message: "sync-mode mismatch refused: the ledger is committed local, the config says remote",
				Fetch:   snapshot.FetchState{Outcome: snapshot.OutcomeFailed, Message: "sync-mode mismatch refused"},
			},
			wantMessage: "sync-mode mismatch refused: the ledger is committed local, the config says remote",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			served := servedBacklog(t, func() snapshot.Observation { return testCase.observation })
			response := request(t, served, http.MethodGet, "/api/backlog", "127.0.0.1:7878", nil)

			testutil.Require(t, "status", response.Code, http.StatusOK)
			var payload backlogPayload
			testutil.Require(t, "decode", json.Unmarshal(response.Body.Bytes(), &payload), nil)

			testutil.Expect(t, "state", payload.Ledger.State, string(testCase.observation.State))
			testutil.Expect(t, "message", payload.Ledger.Message, testCase.wantMessage)
			testutil.Expect(t, "rows", payload.Rows, []backlog.Row{})
			testutil.Expect(t, "closed", payload.Closed, []backlog.Row{})
			testutil.Expect(t, "counts", payload.Counts, map[backlog.Lane]int{})
			testutil.Expect(t, "admission", payload.Admission, admissionPayload{})
			testutil.Expect(t, "the loop's outcome", payload.Ledger.Fetch.Outcome, string(testCase.observation.Fetch.Outcome))
		})
	}
}

func TestBacklogCarriesTheWorkingTreeCountsOnlyWhenTheyAreKnown(t *testing.T) {
	t.Parallel()

	live, archived := 155, 430
	served := servedBacklog(t, func() snapshot.Observation {
		return snapshot.Observation{
			ObservedAt: observedAt, State: snapshot.StateAbsent,
			LiveFiles: &live, ArchivedFiles: &archived,
			Fetch: snapshot.FetchState{Outcome: snapshot.OutcomeNever},
		}
	})
	response := request(t, served, http.MethodGet, "/api/backlog", "127.0.0.1:7878", nil)

	var payload backlogPayload
	testutil.Require(t, "decode", json.Unmarshal(response.Body.Bytes(), &payload), nil)
	testutil.Require(t, "the live count is present", payload.WorkingTree.LiveFiles != nil, true)
	testutil.Expect(t, "live files", *payload.WorkingTree.LiveFiles, 155)
	testutil.Expect(t, "archived files", *payload.WorkingTree.ArchivedFiles, 430)

	unknown := servedBacklog(t, readObservation)
	unknownResponse := request(t, unknown, http.MethodGet, "/api/backlog", "127.0.0.1:7878", nil)
	var read backlogPayload
	testutil.Require(t, "decode the read state", json.Unmarshal(unknownResponse.Body.Bytes(), &read), nil)
	testutil.Expect(t, "the counts of a readable ledger", read.WorkingTree, workingTreePayload{})
}

func TestBacklogSerializesAnUnansweredAdmission(t *testing.T) {
	t.Parallel()

	observation := readObservation()
	observation.Admission = backlog.Admission{Message: "cannot answer claimable backlog: resolve metasystem.budget.tier-3"}
	served := servedBacklog(t, func() snapshot.Observation { return observation })

	response := request(t, served, http.MethodGet, "/api/backlog", "127.0.0.1:7878", nil)
	var payload backlogPayload
	testutil.Require(t, "decode", json.Unmarshal(response.Body.Bytes(), &payload), nil)

	testutil.Expect(t, "admission", payload.Admission, admissionPayload{
		Answered: false, Message: "cannot answer claimable backlog: resolve metasystem.budget.tier-3",
	})
	testutil.Expect(t, "ready is empty", payload.Counts[backlog.LaneReady], 0)
}

func TestBacklogWithoutALedgerReader(t *testing.T) {
	t.Parallel()

	bound := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7878}
	served := New(Info{}, bound, testBundle())

	response := request(t, served, http.MethodGet, "/api/backlog", "127.0.0.1:7878", nil)

	testutil.Expect(t, "status", response.Code, http.StatusInternalServerError)
	testutil.Expect(t, "content type", response.Header().Get("Content-Type"), "application/json")
	testutil.Expect(t, "body", response.Body.String(), "{\"error\":\"this engine was built without a ledger reader\"}\n")
}

// The route is exact: what lies beneath it, or beside it, belongs to no
// resource and says so rather than answering with the page.
func TestBacklogRouteIsExact(t *testing.T) {
	t.Parallel()

	paths := []struct{ name, path string }{
		{name: "beneath the route", path: "/api/backlog/x"},
		{name: "a trailing slash", path: "/api/backlog/"},
		{name: "a longer name", path: "/api/backlogs"},
	}
	served := servedBacklog(t, readObservation)
	for _, testCase := range paths {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			response := request(t, served, http.MethodGet, testCase.path, "127.0.0.1:7878", map[string]string{"Accept": "text/html"})
			testutil.Expect(t, "status", response.Code, http.StatusNotFound)
			testutil.Expect(t, "body", response.Body.String(), "404 page not found\n")
		})
	}
}

// The route is behind the same checks and carries the same headers as every
// other response: no check is loosened for the API.
func TestBacklogRouteIsBehindTheChecks(t *testing.T) {
	t.Parallel()

	const withoutNonce = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; font-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
	served := servedBacklog(t, readObservation)

	allowed := request(t, served, http.MethodGet, "/api/backlog", "127.0.0.1:7878", map[string]string{"Sec-Fetch-Site": "same-origin"})
	testutil.Require(t, "same-origin status", allowed.Code, http.StatusOK)
	testutil.Expect(t, "content security policy", allowed.Header().Get("Content-Security-Policy"), withoutNonce)
	testutil.Expect(t, "no store", allowed.Header().Get("Cache-Control"), "no-store")
	testutil.Expect(t, "no sniff", allowed.Header().Get("X-Content-Type-Options"), "nosniff")

	crossSite := request(t, served, http.MethodGet, "/api/backlog", "127.0.0.1:7878", map[string]string{"Sec-Fetch-Site": "cross-site"})
	testutil.Expect(t, "cross-site status", crossSite.Code, http.StatusForbidden)

	foreignHost := request(t, served, http.MethodGet, "/api/backlog", "attacker.invalid", nil)
	testutil.Expect(t, "foreign host status", foreignHost.Code, http.StatusForbidden)

	post := request(t, served, http.MethodPost, "/api/backlog", "127.0.0.1:7878", nil)
	testutil.Expect(t, "POST status", post.Code, http.StatusMethodNotAllowed)
}
