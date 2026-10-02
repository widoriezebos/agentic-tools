package httpd

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// TestBoardCarriesTheLandingLane (U12): /api/board carries the host's
// landing lane as a top-level "lane", the one view landing status renders,
// read on every request; with no lane reader, and with an unreadable
// registry, the key is still there.
func TestBoardCarriesTheLandingLane(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	root := "/lanes/landing"
	reads := 0
	laneView := func(now time.Time) plain.Status {
		reads++
		testutil.Expect(t, "the lane is read at the server's clock", now, fleetNow)
		return plain.Status{View: lane.View{Root: &root, Owner: lane.OwnerView{State: lane.OwnerRunning}, Summary: "landing lane " + root + ": owner running"}}
	}
	seats := func() ([]board.Seat, error) { return nil, nil }
	served := New(Info{Observe: silentHolder, Now: func() time.Time { return fleetNow },
		Board: &BoardSource{Home: home, Seats: seats, Prober: boardProber{}, Stall: 20 * time.Minute, Lane: laneView}}, loopback(), testBundle())
	var payload map[string]json.RawMessage
	testutil.Require(t, "decode", json.Unmarshal(request(t, served, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &payload), nil)
	var got plain.Status
	testutil.Require(t, "lane", json.Unmarshal(payload["lane"], &got), nil)
	testutil.Expect(t, "a registered lane is an object", string(payload["lane"]) != "null", true)
	testutil.Expect(t, "lane summary", got.Summary, "landing lane /lanes/landing: owner running")
	testutil.Expect(t, "lane read once per request", reads, 1)

	bare := New(Info{Observe: silentHolder, Now: func() time.Time { return fleetNow },
		Board: &BoardSource{Home: home, Seats: seats, Prober: boardProber{}, Stall: 20 * time.Minute}}, loopback(), testBundle())
	payload = nil
	testutil.Require(t, "decode bare", json.Unmarshal(request(t, bare, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &payload), nil)
	testutil.Expect(t, "no lane reader: lane is null", string(payload["lane"]), "null")

	unregistered := New(Info{Observe: silentHolder, Now: func() time.Time { return fleetNow },
		Board: &BoardSource{Home: home, Seats: seats, Prober: boardProber{}, Stall: 20 * time.Minute,
			Lane: func(time.Time) plain.Status {
				return plain.Status{View: lane.View{Owner: lane.OwnerView{State: lane.OwnerUnready}, Summary: "no landing lane is registered"}}
			}}}, loopback(), testBundle())
	payload = nil
	testutil.Require(t, "decode unregistered", json.Unmarshal(request(t, unregistered, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &payload), nil)
	testutil.Expect(t, "no lane registered: lane is null", string(payload["lane"]), "null")
}

// TestBoardLaneCarriesThePlainLane (goal fleet-card-can-land-now): the
// board's lane is landing status --json's data, field for field — whether
// the lane is paused and its agent alive, the queue, the running proof, the
// last proof and the last push — so the card reads what the terminal reads.
func TestBoardLaneCarriesThePlainLane(t *testing.T) {
	t.Parallel()
	root := "/lanes/landing"
	seats := func() ([]board.Seat, error) { return nil, nil }
	served := New(Info{Observe: silentHolder, Now: func() time.Time { return fleetNow },
		Board: &BoardSource{Home: t.TempDir(), Seats: seats, Prober: boardProber{}, Stall: 20 * time.Minute,
			Lane: func(time.Time) plain.Status {
				return plain.Status{View: lane.View{Root: &root, Owner: lane.OwnerView{State: lane.OwnerIdle}, Summary: "landing lane " + root + ": idle"},
					Queue:     []plain.Entry{{Goal: "goal-a", Branch: "goal/goal-a", SHA: "abc", Seat: "m1e", State: plain.StateWaiting}},
					LastProof: &plain.Result{Tree: "t1", Commit: "c1", Result: plain.Green}, LastPush: &plain.Pushed{Old: "c0", Commit: "c1"}}
			}}}, loopback(), testBundle())
	var payload map[string]json.RawMessage
	testutil.Require(t, "decode", json.Unmarshal(request(t, served, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &payload), nil)
	var carried map[string]json.RawMessage
	testutil.Require(t, "lane", json.Unmarshal(payload["lane"], &carried), nil)
	for _, key := range []string{"paused", "agent_alive", "queue", "running_proof", "last_proof", "last_push", "owner", "summary", "wake"} {
		_, present := carried[key]
		testutil.Expect(t, "the lane carries "+key, present, true)
	}
	var got plain.Status
	testutil.Require(t, "lane as status", json.Unmarshal(payload["lane"], &got), nil)
	testutil.Expect(t, "the queue as the reader read it", got.Queue, []plain.Entry{{Goal: "goal-a", Branch: "goal/goal-a", SHA: "abc", Seat: "m1e", State: plain.StateWaiting}})
	testutil.Expect(t, "the last proof", got.LastProof.Commit, "c1")
	testutil.Expect(t, "the last push", got.LastPush.Commit, "c1")
}

// TestBoardNamesTheGoalsItCarriesByTitle: /api/board carries the title of
// every goal its seats, its lane's queue and its questions name, from the
// one ledger observation it checks the cards against — live, done and
// abandoned goals alike, since a landed hand-in's goal is usually done. A
// goal the ledger does not carry has no title rather than an invented one.
func TestBoardNamesTheGoalsItCarriesByTitle(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	m1c := board.Seat{Machine: "m1c", Installation: "/c/c/metasystem"}
	card := board.Card{Goal: "tests-parallel-and-deterministic", Stage: board.StageBuild, Owner: &board.Owner{Pid: 41, PidStartedAt: 1000},
		Seat: m1c, Writer: board.Writer{At: fleetNow.Add(-2 * time.Minute)}}
	testutil.Require(t, "write card", board.WriteAt(home, card), nil)
	root := "/lanes/landing"
	observe := func() snapshot.Observation {
		observed := silentHolder()
		observed.Tree.Done["plain-lane-landing"] = &goal.GoalFile{Id: "plain-lane-landing", Intent: "Land work through the plain lane. It replaces the batch lane."}
		observed.Tree.Abandoned["dropped"] = &goal.GoalFile{Id: "dropped", Intent: "Something nobody will build."}
		return observed
	}
	served := New(Info{Observe: observe, Now: func() time.Time { return fleetNow },
		Asks: func() ([]channel.Question, error) {
			return []channel.Question{{ID: "q-1", Goal: "dropped", Machine: "m1f", Facts: []string{"  Land slice 2 now?  ", "a fact"}, OpenedAt: fleetNow.Add(-time.Hour)}}, nil
		},
		Board: &BoardSource{Home: home, Seats: func() ([]board.Seat, error) { return []board.Seat{m1c}, nil }, Prober: boardProber{}, Stall: 20 * time.Minute,
			Lane: func(time.Time) plain.Status {
				return plain.Status{View: lane.View{Root: &root, Owner: lane.OwnerView{State: lane.OwnerIdle}},
					Queue: []plain.Entry{{Goal: "plain-lane-landing", SHA: "abc", State: plain.StateLanded}, {Goal: "not-in-the-ledger", SHA: "def", State: plain.StateWaiting}}}
			}}}, loopback(), testBundle())

	var payload struct {
		Titles    map[string]string `json:"titles"`
		Questions []struct {
			ID       string `json:"id"`
			Goal     string `json:"goal"`
			Machine  string `json:"machine"`
			Question string `json:"question"`
			OpenedAt string `json:"openedAt"`
		} `json:"questions"`
		QuestionsProblem *string `json:"questionsProblem"`
	}
	testutil.Require(t, "decode", json.Unmarshal(request(t, served, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &payload), nil)
	testutil.Expect(t, "titles", payload.Titles, map[string]string{
		"tests-parallel-and-deterministic": "Run the suite in parallel, deterministically.",
		"plain-lane-landing":               "Land work through the plain lane.",
		"dropped":                          "Something nobody will build.",
	})
	testutil.Require(t, "one question", len(payload.Questions), 1)
	testutil.Expect(t, "the question as the person reads it", payload.Questions[0].Question, "Land slice 2 now?")
	testutil.Expect(t, "who asks", payload.Questions[0].Machine, "m1f")
	testutil.Expect(t, "when", payload.Questions[0].OpenedAt, "2026-09-25T13:37:00Z")
	testutil.Expect(t, "questions were read", payload.QuestionsProblem != nil && *payload.QuestionsProblem == "", true)
}

// TestBoardSaysWhenItCannotReadQuestions: a question reader that fails, and
// a build with none, are said in questionsProblem with an empty list, never
// read as a checkout nobody asked anything in.
func TestBoardSaysWhenItCannotReadQuestions(t *testing.T) {
	t.Parallel()
	seats := func() ([]board.Seat, error) { return nil, nil }
	read := func(label string, asks func() ([]channel.Question, error)) (string, string) {
		served := New(Info{Observe: silentHolder, Now: func() time.Time { return fleetNow }, Asks: asks,
			Board: &BoardSource{Home: t.TempDir(), Seats: seats, Prober: boardProber{}, Stall: time.Minute}}, loopback(), testBundle())
		var payload map[string]json.RawMessage
		testutil.Require(t, "decode "+label, json.Unmarshal(request(t, served, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &payload), nil)
		return string(payload["questions"]), string(payload["questionsProblem"])
	}
	list, problem := read("failing", func() ([]channel.Question, error) { return nil, errors.New("channel folder unreadable") })
	testutil.Expect(t, "a failing reader's list", list, "[]")
	testutil.Expect(t, "a failing reader's words", problem, `"channel folder unreadable"`)
	list, problem = read("none", nil)
	testutil.Expect(t, "no reader's list", list, "[]")
	testutil.Expect(t, "no reader's words", problem, `"this server reads no questions"`)

	unregistered := New(Info{Now: func() time.Time { return fleetNow },
		Board: &BoardSource{Home: t.TempDir(), Seats: func() ([]board.Seat, error) { return nil, errors.New("permission denied") }, Prober: boardProber{}, Stall: time.Minute}},
		loopback(), testBundle())
	var payload map[string]json.RawMessage
	testutil.Require(t, "decode unregistered", json.Unmarshal(request(t, unregistered, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &payload), nil)
	testutil.Expect(t, "an unreadable registry still carries titles", string(payload["titles"]), "{}")
	testutil.Expect(t, "and questions", string(payload["questions"]), "[]")
}

// TestBoardNamesTheQuestionRecordsItCouldNotRead: a walk that read some
// question records and not others carries the readable questions and says,
// in questionsProblem, how many records it could not read.
func TestBoardNamesTheQuestionRecordsItCouldNotRead(t *testing.T) {
	t.Parallel()
	seats := func() ([]board.Seat, error) { return nil, nil }
	read := func(label string, records []string) (int, string) {
		served := New(Info{Observe: silentHolder, Now: func() time.Time { return fleetNow },
			Asks: func() ([]channel.Question, error) {
				return []channel.Question{{ID: "q-1", Machine: "m1f", Facts: []string{"Land it?"}, OpenedAt: fleetNow}}, &UnreadQuestions{Records: records}
			},
			Board: &BoardSource{Home: t.TempDir(), Seats: seats, Prober: boardProber{}, Stall: time.Minute}}, loopback(), testBundle())
		var payload struct {
			Questions        []json.RawMessage `json:"questions"`
			QuestionsProblem string            `json:"questionsProblem"`
		}
		testutil.Require(t, "decode "+label, json.Unmarshal(request(t, served, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &payload), nil)
		return len(payload.Questions), payload.QuestionsProblem
	}
	count, problem := read("one", []string{"/c/q-2.json: unexpected end of JSON input"})
	testutil.Expect(t, "the readable question is kept", count, 1)
	testutil.Expect(t, "one record named", problem, "1 of its records can't be read: /c/q-2.json: unexpected end of JSON input")
	count, problem = read("three", []string{"/c/q-2.json: torn", "/c/q-3.json: torn", "/c/q-4.json: torn"})
	testutil.Expect(t, "still kept", count, 1)
	testutil.Expect(t, "three counted, the first named", problem, "3 of its records can't be read; the first: /c/q-2.json: torn")
}

// TestBoardCarriesALaneItCouldNotRead: a lane status with no root that
// carries a read problem reaches the page as a lane, so the page says it
// could not read it; a lane that is simply not registered is still null.
func TestBoardCarriesALaneItCouldNotRead(t *testing.T) {
	t.Parallel()
	seats := func() ([]board.Seat, error) { return nil, nil }
	served := New(Info{Observe: silentHolder, Now: func() time.Time { return fleetNow },
		Board: &BoardSource{Home: t.TempDir(), Seats: seats, Prober: boardProber{}, Stall: time.Minute,
			Lane: func(time.Time) plain.Status {
				return plain.Status{View: lane.View{Owner: lane.OwnerView{State: lane.OwnerUnready}, Unreadable: "permission denied"},
					Queue: []plain.Entry{}, Problems: []string{"the lane's registration can't be read: permission denied"}}
			}}}, loopback(), testBundle())
	var payload struct {
		Lane *plain.Status `json:"lane"`
	}
	testutil.Require(t, "decode", json.Unmarshal(request(t, served, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &payload), nil)
	testutil.Require(t, "an unreadable lane is carried", payload.Lane != nil, true)
	testutil.Expect(t, "with its root absent", payload.Lane.Root == nil, true)
	testutil.Expect(t, "and its problem", payload.Lane.Problems, []string{"the lane's registration can't be read: permission denied"})
}

// TestBoardSaysWhichOfItsPartsItCouldNotRead: a nickname two checkouts share,
// a seat directory that can't be listed and a card that can't be parsed are
// parts of this computer's board that were not read, carried in unreadable;
// a directory no armed seat names is a leftover, not a failed read.
func TestBoardSaysWhichOfItsPartsItCouldNotRead(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	m1c := board.Seat{Machine: "m1c", Installation: "/c/c/metasystem"}
	stranger := board.Seat{Machine: "stranger", Installation: "/s/s/metasystem"}
	for _, card := range []board.Card{
		{Goal: "tests-parallel-and-deterministic", Stage: board.StageBuild, Owner: &board.Owner{Pid: 41, PidStartedAt: 1000}, Seat: m1c},
		{Goal: "goal-s", Stage: board.StageLandReady, Seat: stranger},
	} {
		card.Writer = board.Writer{At: fleetNow.Add(-2 * time.Minute)}
		testutil.Require(t, "write "+card.Goal, board.WriteAt(home, card), nil)
	}
	torn := filepath.Join(board.Dir(home), "m1c", "goal-torn.json")
	testutil.Require(t, "tear a card", os.WriteFile(torn, []byte("{"), 0o600), nil)
	// A seat whose directory is a file cannot be listed.
	testutil.Require(t, "block a seat", os.WriteFile(filepath.Join(board.Dir(home), "m1d"), []byte("x"), 0o600), nil)
	m1d := board.Seat{Machine: "m1d", Installation: "/d/d/metasystem"}
	seats := []board.Seat{m1c, m1d, {Machine: "m1e", Installation: "/e/one/metasystem"}, {Machine: "m1e", Installation: "/e/two/metasystem"}}
	served := New(Info{Observe: silentHolder, Now: func() time.Time { return fleetNow },
		Board: &BoardSource{Home: home, Seats: func() ([]board.Seat, error) { return seats, nil }, Prober: boardProber{}, Stall: 20 * time.Minute}},
		loopback(), testBundle())

	var payload struct {
		Readable   bool     `json:"readable"`
		Unreadable []string `json:"unreadable"`
	}
	testutil.Require(t, "decode", json.Unmarshal(request(t, served, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &payload), nil)
	joined := strings.Join(payload.Unreadable, "\n")
	testutil.Expect(t, "the registry was read", payload.Readable, true)
	testutil.Expect(t, "three parts not read", len(payload.Unreadable), 3)
	testutil.Expect(t, "the shared nickname", strings.Contains(joined, "names 2 armed checkouts"), true)
	testutil.Expect(t, "the seat that can't be listed", strings.Contains(joined, "m1d: seat directory unreadable"), true)
	testutil.Expect(t, "the torn card", strings.Contains(joined, "m1c: goal-torn: card unreadable"), true)
	testutil.Expect(t, "never the leftover directory", strings.Contains(joined, "stranger"), false)

	calm := New(Info{Observe: silentHolder, Now: func() time.Time { return fleetNow },
		Board: &BoardSource{Home: t.TempDir(), Seats: func() ([]board.Seat, error) { return nil, nil }, Prober: boardProber{}, Stall: time.Minute}},
		loopback(), testBundle())
	var raw map[string]json.RawMessage
	testutil.Require(t, "decode calm", json.Unmarshal(request(t, calm, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &raw), nil)
	testutil.Expect(t, "a board read whole carries an empty list", string(raw["unreadable"]), "[]")
}

// TestBoardSaysALedgerItCouldNotRead: a board read whose ledger observation
// failed checked no card against the claims and titled no goal, so the
// ledger is one of the parts it says it could not read.
func TestBoardSaysALedgerItCouldNotRead(t *testing.T) {
	t.Parallel()
	seats := func() ([]board.Seat, error) { return nil, nil }
	read := func(label string, observe func() snapshot.Observation) []string {
		served := New(Info{Observe: observe, Now: func() time.Time { return fleetNow },
			Board: &BoardSource{Home: t.TempDir(), Seats: seats, Prober: boardProber{}, Stall: time.Minute}}, loopback(), testBundle())
		var payload struct {
			Readable   bool     `json:"readable"`
			Unreadable []string `json:"unreadable"`
		}
		testutil.Require(t, "decode "+label, json.Unmarshal(request(t, served, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &payload), nil)
		return payload.Unreadable
	}
	broken := func() snapshot.Observation {
		return snapshot.Observation{State: snapshot.StateBroken, Message: "the accepted ref can't be read"}
	}
	testutil.Expect(t, "a broken ledger", read("broken", broken), []string{"the goal ledger can't be read: the accepted ref can't be read"})
	testutil.Expect(t, "a ledger in no state with no words", read("absent", func() snapshot.Observation { return snapshot.Observation{State: snapshot.StateAbsent} }),
		[]string{"the goal ledger can't be read: the accepted ledger could not be read"})
	testutil.Expect(t, "a build with no ledger reader", read("none", nil), []string{"the goal ledger can't be read: this server reads no goal ledger"})
	testutil.Expect(t, "a ledger that was read", read("read", silentHolder), []string{})
}
