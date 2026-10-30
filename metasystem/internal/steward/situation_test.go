package steward

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

var situationNow = claimQuotaClock

// situationQuotaBed is the claim-quota ledger with this machine's claim on
// held-h, three ready priority-1 goals ranked c, a, b and one unranked goal,
// and a saved health record whose claim-quota role reads alive.
func situationQuotaBed(t *testing.T) string {
	t.Helper()
	ready := func(id string, sequence uint64) *goal.GoalFile {
		file := claimQuotaReady(id, 1, "m1e")
		file.Sequence = sequence
		return file
	}
	root := claimQuotaTickBed(t, nil, claimQuotaHeld("held-h"), ready("ready-c", 1), ready("ready-a", 2), ready("ready-b", 3),
		approvedStewardGoal("aaa-unranked", "Wait unranked", "Build it.", "2026-08-23T00:00:00Z"))
	if _, err := observePartialHealth(root, situationNow.Add(-time.Hour), []RoleVerdict{roleAlive(RoleClaimQuota, "saved by a tick")}); err != nil {
		t.Fatal(err)
	}
	return root
}

// situationLaneHome is a host home with the lane registered on a checkout.
func situationLaneHome(t *testing.T) string {
	t.Helper()
	home, checkout := t.TempDir(), t.TempDir()
	data, err := json.Marshal(lane.Record{Root: checkout, Install: filepath.Join(checkout, "metasystem"), CustodyEpoch: 1, RegisteredBy: "Wido"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(lane.HostDir(home), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lane.RecordPath(home), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return home
}

// countedProject is the production projection counted per call.
func countedProject(calls *int) func(goal.Endpoint, bool, time.Time) (goal.Projection, error) {
	return func(endpoint goal.Endpoint, fetch bool, now time.Time) (goal.Projection, error) {
		*calls++
		return goal.Project(endpoint, fetch, now)
	}
}

func TestTheSituationReadsTheLedgerAndTheQuotaLive(t *testing.T) {
	t.Parallel()
	root := situationQuotaBed(t)
	states := map[string]string{}
	projections := 0
	tip := seatGit(t, root, "rev-parse", goal.AcceptedRef)
	units := func(gotRoot, id string, projection goal.Projection, now time.Time) ([]UnitStage, error) {
		if gotRoot != root || id != "held-h" || projection.Tip != tip || !now.Equal(situationNow) {
			t.Errorf("units read root=%q goal=%q tip=%q now=%v, want the read's projection %q at %v", gotRoot, id, projection.Tip, now, tip, situationNow)
		}
		return []UnitStage{{Unit: "first", Stage: "built"}, {Unit: "second", Stage: "reading"}}, nil
	}
	src := withEmptyHost(t, SituationSources{Now: func() time.Time { return situationNow }, LaneHome: func() (string, error) { return t.TempDir(), nil },
		Project: countedProject(&projections), ClaimLaneState: claimQuotaLane(t, root, states), Units: units,
		BoardHome: func() (string, error) { return t.TempDir(), nil },
		Waiting:   func(string) ([]string, error) { return []string{"a durable wait"}, nil }})
	release := ClaimQuotaBlock{Held: "held-h", Ready: []string{"ready-c", "ready-a", "ready-b"}, Command: "metasystem goal release held-h --reason TEXT"}

	s := readSituation(t, root, src)
	if !slices.Equal(s.Next.Claimed, []string{"held-h"}) || s.Ledger.Tip == "" || !s.At.Equal(situationNow) || s.Machine != "m1e" {
		t.Fatalf("the ledger part: claimed=%v tip=%q at=%v machine=%q unavailable=%v", s.Next.Claimed, s.Ledger.Tip, s.At, s.Machine, s.Unavailable)
	}
	if want := []string{"ready-c", "ready-a", "ready-b"}; len(s.Order) < 3 || !slices.Equal(s.Order[:3], want) || slices.Index(s.Order, "aaa-unranked") < 3 {
		t.Fatalf("order = %v, want %v before any unranked goal", s.Order, want)
	}
	if !reflect.DeepEqual(s.Quota, []ClaimQuotaBlock{release}) || s.Unavailable["quota"] != "" {
		t.Fatalf("quota = %+v (unavailable %q), want one block %+v", s.Quota, s.Unavailable["quota"], release)
	}
	if !slices.Equal(s.Waiting, []string{"a durable wait"}) || projections != 1 {
		t.Fatalf("waiting=%v projections=%d, want the wait and one projection", s.Waiting, projections)
	}
	facts := s.GoalFacts["held-h"]
	for _, want := range []string{"Next step: Build it.", "first: built", "second: reading", "Hand-in"} {
		if !strings.Contains(facts, want) {
			t.Fatalf("the held goal's facts lack %q:\n%s", want, facts)
		}
	}

	states["held-h"] = "waiting"
	if s = readSituation(t, root, src); len(s.Quota) != 0 || projections != 2 {
		t.Fatalf("a waiting hand-in frees the claim: quota=%+v projections=%d", s.Quota, projections)
	}
	states["held-h"] = "returned"
	land := release
	land.Command = "metasystem work land held-h"
	if s = readSituation(t, root, src); !reflect.DeepEqual(s.Quota, []ClaimQuotaBlock{land}) || projections != 3 {
		t.Fatalf("a returned hand-in blocks again: quota=%+v projections=%d, want %+v", s.Quota, projections, land)
	}

	src.ClaimLaneState = nil
	if s = readSituation(t, root, src); s.Unavailable["quota"] != "no lane reader" || len(s.Quota) != 0 {
		t.Fatalf("no lane reader: quota=%+v unavailable=%q, want none and no saved verdict", s.Quota, s.Unavailable["quota"])
	}
}

func TestASituationPartFailsAlone(t *testing.T) {
	t.Parallel()
	root := situationQuotaBed(t)
	laneHome, boardHome := situationLaneHome(t), t.TempDir()
	if _, err := board.Publish(boardHome, board.Request{Kind: board.KindAsk, From: board.Sender{Machine: "m1x"}, To: board.Address{Machine: "m1e"}, Text: "Is it built?"}, situationNow); err != nil {
		t.Fatal(err)
	}
	// Units is nil: the seat brief's own reader says the units are unavailable.
	src := func() SituationSources {
		return withEmptyHost(t, SituationSources{Now: func() time.Time { return situationNow }, LaneHome: func() (string, error) { return laneHome, nil },
			LaneStatus:     func(string, time.Time) plain.Status { return plain.Status{Problems: []string{"the lane read"}} },
			HostWork:       func() (int, int, error) { return 0, 0, nil },
			BoardHome:      func() (string, error) { return boardHome, nil },
			ClaimLaneState: claimQuotaLane(t, root, nil), Waiting: func(string) ([]string, error) { return []string{"a durable wait"}, nil }})
	}
	read := func(s Situation) (lane, ledger, quota, waiting, asks bool) {
		return slices.Equal(s.Lane.Problems, []string{"the lane read"}), slices.Equal(s.Next.Claimed, []string{"held-h"}), len(s.Quota) == 1, len(s.Waiting) == 1, len(s.Asks) == 1
	}

	whole := src()
	s := readSituation(t, root, whole)
	if lane, ledger, quota, waiting, asks := read(s); !lane || !ledger || !quota || !waiting || !asks {
		t.Fatalf("the whole read: lane=%v ledger=%v quota=%v waiting=%v asks=%v unavailable=%v", lane, ledger, quota, waiting, asks, s.Unavailable)
	}
	if facts := s.GoalFacts["held-h"]; !strings.Contains(facts, "Units unavailable") || !strings.Contains(facts, "Next step:") {
		t.Fatalf("no unit reader: the held goal's facts are %q, want the units unavailable and the rest", facts)
	}

	noLane := src()
	noLane.LaneHome = func() (string, error) { return "", errors.New("no host home") }
	s = readSituation(t, root, noLane)
	if lane, ledger, quota, waiting, asks := read(s); lane || !ledger || !quota || !waiting || !asks || s.Unavailable["lane"] == "" {
		t.Fatalf("a failed lane home: lane=%v ledger=%v quota=%v waiting=%v asks=%v unavailable=%v", lane, ledger, quota, waiting, asks, s.Unavailable)
	}

	noBoard := src()
	noBoard.BoardHome = func() (string, error) { return "", errors.New("no board home") }
	s = readSituation(t, root, noBoard)
	if lane, ledger, quota, waiting, asks := read(s); !lane || !ledger || !quota || !waiting || asks || s.Unavailable["asks"] != "no board home" || s.GoalFacts["held-h"] == "" {
		t.Fatalf("a failed board home: lane=%v ledger=%v quota=%v waiting=%v asks=%v unavailable=%v", lane, ledger, quota, waiting, asks, s.Unavailable)
	}

	noMachine := src()
	noMachine.Machine = func(string) (string, error) { return "", errors.New("no nickname") }
	s = readSituation(t, root, noMachine)
	if lane, ledger, quota, _, asks := read(s); !lane || ledger || quota || asks || len(s.GoalFacts) != 0 || s.Unavailable["lane"] != "" {
		t.Fatalf("a failed machine read: lane=%v ledger=%v quota=%v asks=%v facts=%d unavailable=%v", lane, ledger, quota, asks, len(s.GoalFacts), s.Unavailable)
	}
	for _, part := range []string{"ledger", "quota", "facts", "asks"} {
		if s.Unavailable[part] == "" {
			t.Fatalf("a failed machine read leaves %s readable: %v", part, s.Unavailable)
		}
	}

	noWaiting := src()
	noWaiting.Waiting = func(string) ([]string, error) { return nil, errors.New("the waits cannot be read") }
	s = readSituation(t, root, noWaiting)
	if lane, ledger, quota, _, asks := read(s); !lane || !ledger || !quota || !asks || s.Unavailable["waiting"] != "the waits cannot be read" {
		t.Fatalf("a failed waiting read: lane=%v ledger=%v quota=%v asks=%v unavailable=%v", lane, ledger, quota, asks, s.Unavailable)
	}
}

// boardTree is every path under the board home with its size.
func boardTree(t *testing.T, home string) map[string]int64 {
	t.Helper()
	tree := map[string]int64{}
	if err := filepath.Walk(home, func(path string, info os.FileInfo, err error) error {
		if err == nil {
			tree[path] = info.Size()
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestTheSituationKeepsEveryAskOfThisSeat(t *testing.T) {
	t.Parallel()
	root, home := situationQuotaBed(t), t.TempDir()
	if err := askOnce(askedReceipts(root), home, board.Sender{Machine: "m1e"}, board.Address{Goal: "held-h"}, "the claim on held-h", "Release held-h?", situationNow); err != nil {
		t.Fatal(err)
	}
	publish := func(request board.Request, at time.Time) board.Message {
		t.Helper()
		published, err := board.Publish(home, request, at)
		if err != nil {
			t.Fatal(err)
		}
		return published.Message
	}
	deadline := publish(board.Request{Kind: board.KindAsk, From: board.Sender{Machine: "m1x"}, To: board.Address{Machine: "m1e"}, Text: "Is it built?", Deadline: 30 * time.Minute}, situationNow)
	if err := board.Mark(deadline, "m1e", "coordinator", "inbox", situationNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	publish(board.Request{Kind: board.KindAsk, From: board.Sender{Machine: "m1x"}, To: board.Address{Goal: "ready-a"}, Text: "Who takes ready-a?"}, situationNow)
	outgoing := publish(board.Request{Kind: board.KindAsk, From: board.Sender{Machine: "m1e"}, To: board.Address{Machine: "m1x"}, Text: "Can you read it?"}, situationNow)
	now := situationNow.Add(10 * time.Minute)
	src := withEmptyHost(t, SituationSources{Now: func() time.Time { return now }, LaneHome: func() (string, error) { return t.TempDir(), nil },
		ClaimLaneState: claimQuotaLane(t, root, nil), BoardHome: func() (string, error) { return home, nil }})
	read := func() map[string]SituationAsk {
		t.Helper()
		before := boardTree(t, home)
		s := readSituation(t, root, src)
		if after := boardTree(t, home); !reflect.DeepEqual(before, after) {
			t.Fatalf("the read changed the board:\nbefore %v\nafter  %v", before, after)
		}
		for path := range boardTree(t, home) {
			if strings.Contains(path, "concluded") {
				t.Fatalf("the read concluded a message: %s", path)
			}
		}
		asks := map[string]SituationAsk{}
		for _, ask := range s.Asks {
			asks[ask.Thread.Messages[0].PeerText()] = ask
		}
		if s.Unavailable["asks"] != "" {
			t.Fatalf("the asks do not read: %q", s.Unavailable["asks"])
		}
		return asks
	}
	asks := read()
	if len(asks) != 3 {
		t.Fatalf("asks = %v, want this seat's three and not the other seat's", asks)
	}
	if ask := asks["Is it built?"]; ask.State != board.ThreadOpen || !ask.Delivered {
		t.Fatalf("the read deadline ask: %+v, want open and delivered", ask)
	}
	if ask := asks["Release held-h?"]; ask.State != board.ThreadOpen || ask.Delivered {
		t.Fatalf("the goal's deadline-free ask: %+v, want open and not delivered", ask)
	}
	if ask := asks["Can you read it?"]; ask.State != board.ThreadOpen {
		t.Fatalf("this seat's own ask: %+v, want open", ask)
	}

	// Another machine reads the goal's ask: it is still not delivered here.
	if err := board.Mark(*asks["Release held-h?"].Thread.Root, "ui", "coordinator", "inbox", now); err != nil {
		t.Fatal(err)
	}
	if ask := read()["Release held-h?"]; ask.Delivered || ask.State != board.ThreadOpen {
		t.Fatalf("the goal's ask marked for ui: %+v, want open and not delivered for m1e", ask)
	}

	now = situationNow.Add(31 * time.Minute)
	publish(board.Request{Kind: board.KindReply, From: board.Sender{Machine: "m1x"}, To: board.Address{Machine: "m1e"}, Thread: outgoing.ID, Text: "Yes."}, now)
	asks = read()
	if ask := asks["Is it built?"]; ask.State != board.ThreadExpired || !ask.Since.Equal(situationNow.Add(30*time.Minute)) {
		t.Fatalf("the deadline ask at +31m: %+v, want expired at +30m", ask)
	}
	if ask := asks["Can you read it?"]; ask.State != board.ThreadReplied || !ask.Since.Equal(now) || len(asks) != 3 {
		t.Fatalf("the answered ask: %+v (%d asks), want replied at %v", ask, len(asks), now)
	}
}
