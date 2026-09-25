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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/knownissues"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/application"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/fleet"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/overview"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/workspace"
)

// The Application route: one read of five answers this server already has.

var applicationNow = time.Date(2026, 9, 25, 11, 0, 0, 0, time.UTC)

// An observation carrying one concluded goal and one live one, so the page has
// a row to answer with and the done lane is not the whole tree.
func applicationObservation() snapshot.Observation {
	concluded := &goal.GoalFile{
		Id: "g1-s48", State: goal.StateDone, Intent: "The Decisions inbox is done",
		Conclude: "landed in 9017baa; the inbox groups collapse", Revision: 4,
		History: []goal.HistoryLine{
			{At: "2026-09-25T09:00:00Z", Opid: "op-done", Verb: "done", Actor: "m1e+coordinator"},
		},
	}
	return snapshot.Observation{
		ObservedAt: applicationNow, State: snapshot.StateRead, StateRoot: "/work/repository",
		Tip: "c5d517f", CommittedAt: applicationNow.Add(-time.Minute), SyncMode: goal.SyncLocal,
		Tree: &goal.TreeGoals{
			Root:      &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "2", SyncMode: goal.SyncLocal, Revision: 1},
			Live:      map[string]*goal.GoalFile{"g1-s49": {Id: "g1-s49", State: goal.StateQueued, Intent: "Application, what it does now", Priority: 2, Sequence: 1, Revision: 3}},
			Done:      map[string]*goal.GoalFile{"g1-s48": concluded},
			Abandoned: map[string]*goal.GoalFile{},
		},
		Admission: backlog.Admission{Answered: true, Ready: map[string]bool{}, Blocked: map[string]bool{}, Awaiting: map[string]bool{}, Refused: map[string]string{}},
	}
}

// applicationInfo is a server that can answer the page: a workspace, a ledger,
// a project, the register, this seat's own presence row, a marker and a clock.
func applicationInfo(t *testing.T) (Info, *int) {
	t.Helper()
	visits := 0
	root := t.TempDir()
	return Info{
		Describe: func() (workspace.Workspace, error) {
			return workspace.Workspace{
				SchemaVersion: workspace.SchemaVersion,
				Subject:       "MetaSystem", Mode: workspace.ModeSelfHosted,
			}, nil
		},
		Observe: func() snapshot.Observation { return applicationObservation() },
		Project: func() (project.Pane, error) { return describedPane(), nil },
		Now:     func() time.Time { return applicationNow },
		Fleet: func(snapshot.Observation, backlog.Board, time.Time) (fleet.Page, error) {
			return fleet.Page{
				SchemaVersion: fleet.SchemaVersion,
				Machines: []fleet.Machine{
					{Machine: "m2a", Standing: "reachable", Engine: "0000000", Generation: 9, Seen: "2026-09-25T10:00:00Z"},
					{Machine: "m1e", Standing: "reachable", This: true, Engine: "5b9d958", Generation: 4, Seen: "2026-09-25T10:45:00Z"},
				},
			}, nil
		},
		KnownIssues: func() (knownissues.Register, error) {
			return knownissues.Register{
				Columns: []string{"Id", "Date", "Symptom and evidence", "Cost when it bites", "Fix direction or lever", "Status"},
				Open: []knownissues.Row{{
					ID: "KI-25", Date: "2026-08-07", What: "The follow-up round reviews a stale tree",
					Consequence: "A critic re-reports a folded finding", Lever: "Sync the worktree on follow-up",
					Status: "OPEN", Open: true,
				}},
				Concluded: []knownissues.Row{{
					ID: "KI-1", Date: "2026-08-04", What: "last-census.json records durationMs",
					Consequence: "A reader looks for the wrong field", Lever: "Align the documentation",
					Status: "FIXED 2026-08-06",
				}},
				Unread:  2,
				Defects: []string{"row=19: wrong column count: got 3, want 6"},
			}, nil
		},
		KnownIssuesPath: "metasystem/memory/known-issues.md",
		VisitApplication: func(human string, now time.Time) (time.Time, bool, error) {
			visits++
			return overview.VisitPage(root, application.PageName, human, now)
		},
	}, &visits
}

func applicationPage(t *testing.T, served http.Handler, named string) application.Page {
	t.Helper()
	response := request(t, served, http.MethodGet, applicationPath, "127.0.0.1:7878", nil)
	testutil.Require(t, named+" status", response.Code, http.StatusOK)
	testutil.Expect(t, named+" content type", response.Header().Get("Content-Type"), "application/json")
	var page application.Page
	testutil.Require(t, "decode "+named, json.Unmarshal(response.Body.Bytes(), &page), nil)
	return page
}

// The route answers the composed page, with the five answers in it, and
// records the visit while it does.
func TestApplicationPayload(t *testing.T) {
	t.Parallel()

	info, visits := applicationInfo(t)
	served := New(info, loopback(), testBundle())
	page := applicationPage(t, served, "the read")

	testutil.Expect(t, "the schema a reader parses", page.SchemaVersion, application.SchemaVersion)
	testutil.Expect(t, "when it was read", page.ReadAt, applicationNow.Format(time.RFC3339))
	testutil.Expect(t, "the subject", page.Subject, "MetaSystem")
	testutil.Expect(t, "the mode", page.Mode, workspace.ModeSelfHosted)
	// The done lane's one row, in the ledger's own words; the queued goal is
	// not an account of what this workspace has concluded.
	testutil.Expect(t, "how many concluded", page.Counts.Landed, 1)
	testutil.Require(t, "a row to read", len(page.Landed), 1)
	testutil.Expect(t, "the row's id", page.Landed[0].ID, "g1-s48")
	testutil.Expect(t, "the row's conclusion", page.Landed[0].Concluded,
		"landed in 9017baa; the inbox groups collapse")
	// The register, whole, with the rows it could not read counted and named.
	testutil.Expect(t, "the open problems", len(page.Problems.Open), 1)
	testutil.Expect(t, "the concluded problems", len(page.Problems.Concluded), 1)
	testutil.Expect(t, "how many could not be read", page.Problems.Unread, 2)
	testutil.Expect(t, "where the register is from the checkout", page.Problems.Register,
		"metasystem/memory/known-issues.md")
	// The visit was recorded as part of answering, because reading the page is
	// the visit.
	testutil.Expect(t, "the visit was recorded", *visits, 1)
	testutil.Expect(t, "and the page says it was a first one", page.Visit.First, true)
}

// The build this page names is this seat's own row of the fleet, read through
// the one composition, so the two pages cannot name two builds.
func TestApplicationNamesThisSeatsOwnLastPublishedEngine(t *testing.T) {
	t.Parallel()

	info, _ := applicationInfo(t)
	served := New(info, loopback(), testBundle())
	page := applicationPage(t, served, "the read")

	testutil.Require(t, "an engine build", page.Engine != nil, true)
	testutil.Expect(t, "the build, the generation and its tick", *page.Engine,
		application.Engine{Build: "5b9d958", Generation: 4, PublishedAt: "2026-09-25T10:45:00Z"})
}

// A build with no fleet reader, and a seat whose row carries no record, both
// leave the page with no engine build, which it says in words.
func TestApplicationCarriesNoEngineWhereThisSeatPublishedNone(t *testing.T) {
	t.Parallel()

	info, _ := applicationInfo(t)
	info.Fleet = nil
	page := applicationPage(t, New(info, loopback(), testBundle()), "with no fleet reader")
	if page.Engine != nil {
		t.Fatalf("a build with no fleet reader names no engine: %+v", page.Engine)
	}

	silent, _ := applicationInfo(t)
	silent.Fleet = func(snapshot.Observation, backlog.Board, time.Time) (fleet.Page, error) {
		return fleet.Page{Machines: []fleet.Machine{{Machine: "m1e", This: true}}}, nil
	}
	unarmed := applicationPage(t, New(silent, loopback(), testBundle()), "with no presence record")
	if unarmed.Engine != nil {
		t.Fatalf("a seat that has published nothing names no engine: %+v", unarmed.Engine)
	}
}

// A register this seat could not read is the reason the page could not be
// composed, not an empty block that reads like a project with no known
// defects.
func TestApplicationRefusesWhenTheRegisterCouldNotBeRead(t *testing.T) {
	t.Parallel()

	info, _ := applicationInfo(t)
	info.KnownIssues = func() (knownissues.Register, error) {
		return knownissues.Register{}, errors.New("memory/known-issues.md could not be read")
	}
	served := New(info, loopback(), testBundle())

	response := request(t, served, http.MethodGet, applicationPath, "127.0.0.1:7878", nil)

	testutil.Expect(t, "status", response.Code, http.StatusInternalServerError)
	testutil.Expect(t, "the reason a human acts on",
		strings.Contains(response.Body.String(), "memory/known-issues.md could not be read"), true)
}

// A build with no register reader at all is an ordinary adopted repository:
// the page renders with an empty register rather than refusing.
func TestApplicationRendersWithNoRegisterReader(t *testing.T) {
	t.Parallel()

	info, _ := applicationInfo(t)
	info.KnownIssues = nil
	page := applicationPage(t, New(info, loopback(), testBundle()), "with no register reader")

	testutil.Expect(t, "the columns", page.Problems.Columns, []string{})
	testutil.Expect(t, "the open problems", page.Problems.Open, []knownissues.Row{})
	testutil.Expect(t, "how many could not be read", page.Problems.Unread, 0)
}

// The three readers the page cannot be composed without say which one this
// build lacks, because the reason is what a human acts on.
func TestApplicationSaysWhichReaderThisBuildLacks(t *testing.T) {
	t.Parallel()

	for _, one := range []struct {
		name   string
		strip  func(*Info)
		reason string
	}{
		{"no workspace", func(in *Info) { in.Describe = nil }, "this engine was built without a workspace description"},
		{"no ledger", func(in *Info) { in.Observe = nil }, "this engine was built without a ledger reader"},
		{"no project", func(in *Info) { in.Project = nil }, "this engine was built without a project reader"},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			info, _ := applicationInfo(t)
			one.strip(&info)
			response := request(t, New(info, loopback(), testBundle()),
				http.MethodGet, applicationPath, "127.0.0.1:7878", nil)
			testutil.Expect(t, "status", response.Code, http.StatusInternalServerError)
			testutil.Expect(t, "the reason", strings.Contains(response.Body.String(), one.reason), true)
		})
	}
}

// The read routes' own policy: the resource is matched exactly, what lies
// beneath it belongs to no resource, and a write is refused.
func TestApplicationTakesTheReadRoutesPolicy(t *testing.T) {
	t.Parallel()

	info, _ := applicationInfo(t)
	served := New(info, loopback(), testBundle())

	beneath := request(t, served, http.MethodGet, applicationPath+"/KI-1", "127.0.0.1:7878", nil)
	written := request(t, served, http.MethodPost, applicationPath, "127.0.0.1:7878", nil)
	foreign := request(t, served, http.MethodGet, applicationPath, "attacker.invalid", nil)
	crossSite := request(t, served, http.MethodGet, applicationPath, "127.0.0.1:7878",
		map[string]string{"Sec-Fetch-Site": "cross-site"})

	testutil.Expect(t, "an unserved path under /api", beneath.Code, http.StatusNotFound)
	testutil.Expect(t, "a write is refused", written.Code, http.StatusMethodNotAllowed)
	testutil.Expect(t, "a foreign host is refused", foreign.Code, http.StatusForbidden)
	testutil.Expect(t, "a cross-site read is refused", crossSite.Code, http.StatusForbidden)
}

// The entry is this page's own: reading Application must not move the window
// the Decisions page or the landing page compares against.
func TestApplicationKeepsItsOwnVisitEntry(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	info, _ := applicationInfo(t)
	info.VisitApplication = func(human string, now time.Time) (time.Time, bool, error) {
		return overview.VisitPage(root, application.PageName, human, now)
	}
	applicationPage(t, New(info, loopback(), testBundle()), "the read")

	// The landing page's own marker was not written by that read, so it is
	// still a first visit there.
	_, first, err := overview.Visit(root, "", applicationNow)
	testutil.Require(t, "the landing page's marker", err, nil)
	testutil.Expect(t, "reading Application left the landing page's window alone", first, true)
	// And Decisions' entry is untouched for the same reason.
	_, decisionsFirst, err := overview.VisitPage(root, overview.PageDecisions, "", applicationNow)
	testutil.Require(t, "the Decisions marker", err, nil)
	testutil.Expect(t, "and Decisions' window too", decisionsFirst, true)
}
