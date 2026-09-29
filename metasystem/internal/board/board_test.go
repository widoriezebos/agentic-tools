package board

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// fakeProber answers from a table: pid -> start seconds; absent is dead.
type fakeProber map[int64]int64

func (p fakeProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	started, ok := p[pid]
	if !ok {
		return identity.Exact{}, identity.Dead, nil
	}
	return identity.Exact{Pid: pid, StartedAt: time.Unix(started, 0)}, identity.Alive, nil
}

// aliveProber reports every pid alive with the recorded start of 1000.
type aliveProber struct{}

func (aliveProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{Pid: pid, StartedAt: time.Unix(1000, 0)}, identity.Alive, nil
}

func put(t *testing.T, home string, card Card) {
	t.Helper()
	// Written as a seat's writer would, then stamped as the fixture needs:
	// the board owns Since and LastProgressAt, so the fixture sets them after.
	if err := WriteAt(home, card); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(Dir(home), card.Seat.Machine, card.Goal+".json")
	stored, ok := readCardFile(path)
	if !ok {
		t.Fatal("unreadable")
	}
	stored.SchemaVersion = max(card.SchemaVersion, SchemaVersion)
	stored.Since, stored.LastProgressAt = card.Since, card.LastProgressAt
	if card.Seat.Installation != stored.Seat.Installation {
		stored.Seat = card.Seat
	}
	rewrite(t, path, stored)
}

func rewrite(t *testing.T, path string, card Card) {
	t.Helper()
	data, err := jsonMarshal(card)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestUnknownIsNeverNear (R24, U10a-1 part): a newer schema, a dead owner, a
// reused owner, a stalled card and a stamp from the future are each Unknown
// with a reason; the same cards with a live owner and a fresh stamp are
// believed; Classify answers the same for cards from disk and as values.
func TestUnknownIsNeverNear(t *testing.T) {
	t.Parallel()
	home := fixtureHome(t)
	seats := []Seat{seatOf("m1b"), seatOf("m1c")}
	now := t0.Add(30 * time.Minute)
	stall := 20 * time.Minute
	live := &Owner{Pid: 41, PidStartedAt: 1000}
	prober := fakeProber{41: 1000, 42: 2000}
	fresh := now.Add(-time.Minute)
	cases := []struct {
		goal   string
		card   Card
		reason string
	}{
		{"newer-schema", Card{SchemaVersion: 2, Stage: StageBuild, Owner: live}, "schema 2"},
		{"dead-owner", Card{Stage: StageBuild, Owner: &Owner{Pid: 43, PidStartedAt: 1000}}, "writer dead"},
		{"reused-owner", Card{Stage: StageReview, Owner: &Owner{Pid: 42, PidStartedAt: 1000}}, "reused"},
		{"stalled", Card{Stage: StageUnitProof, Owner: live, LastProgressAt: now.Add(-25 * time.Minute)}, ReasonStalled},
		{"future", Card{Stage: StageBuild, Owner: live, LastProgressAt: now.Add(25 * time.Minute)}, "malformed"},
		{"ownerless", Card{Stage: StageBuild}, "no owner"},
	}
	for _, c := range cases {
		card := c.card
		card.Seat, card.Goal = seatOf("m1b"), c.goal
		if card.Since.IsZero() {
			card.Since = t0
		}
		if card.LastProgressAt.IsZero() {
			card.LastProgressAt = fresh
		}
		put(t, home, card)
	}
	for _, goal := range []string{"believed-build", "believed-land-ready"} {
		stage, owner := StageBuild, live
		if goal == "believed-land-ready" {
			stage, owner = StageLandReady, nil
		}
		put(t, home, Card{Seat: seatOf("m1c"), Goal: goal, Stage: stage, Owner: owner, Since: t0, LastProgressAt: fresh})
	}
	picture, unreadable := Read(home, seats, prober, now, stall)
	if len(unreadable) != 0 {
		t.Fatalf("unexpected unreadable: %+v", unreadable)
	}
	reasons := map[string]string{}
	for _, u := range picture.Unknown {
		reasons[u.Goal] = u.Reason
	}
	for _, c := range cases {
		if !strings.Contains(reasons[c.goal], c.reason) {
			t.Errorf("%s: reason %q, want it to name %q", c.goal, reasons[c.goal], c.reason)
		}
	}
	var believed []string
	for _, card := range picture.Cards {
		believed = append(believed, card.Goal)
	}
	if !reflect.DeepEqual(believed, []string{"believed-build", "believed-land-ready"}) {
		t.Fatalf("believed %v, want only the live, fresh cards", believed)
	}

	// The same cards handed in as values classify identically.
	all := append([]Card(nil), picture.Cards...)
	for _, u := range picture.Unknown {
		all = append(all, *u.Card)
	}
	byValue := Classify(seats, all, prober, now, stall)
	if !reflect.DeepEqual(byValue.Cards, picture.Cards) || len(byValue.Unknown) != len(picture.Unknown) {
		t.Fatalf("Classify of values differs from the read: %+v vs %+v", byValue, picture)
	}
	for _, u := range byValue.Unknown {
		if reasons[u.Goal] != u.Reason {
			t.Errorf("%s: by value %q, from disk %q", u.Goal, u.Reason, reasons[u.Goal])
		}
	}
}

// TestBoardReadsOnlyTheSeatsItIsGiven (R24, U10a-1 part): a seat directory
// with no armed seat is reported once and not read; a card whose installation
// is not its armed checkout's is Unknown; two armed checkouts with one
// nickname are reported and both Unknown; board imports nothing above it.
func TestBoardReadsOnlyTheSeatsItIsGiven(t *testing.T) {
	t.Parallel()
	home := fixtureHome(t)
	now := t0.Add(time.Minute)
	put(t, home, Card{Seat: seatOf("m1b"), Goal: "goal-x", Stage: StageLandReady, Since: t0, LastProgressAt: t0})
	put(t, home, Card{Seat: seatOf("stranger"), Goal: "goal-s", Stage: StageLandReady, Since: t0, LastProgressAt: t0})
	put(t, home, Card{Seat: Seat{Machine: "m1c", Installation: "/elsewhere/metasystem"}, Goal: "goal-y", Stage: StageLandReady, Since: t0, LastProgressAt: t0})
	put(t, home, Card{Seat: seatOf("m1e"), Goal: "goal-z", Stage: StageLandReady, Since: t0, LastProgressAt: t0})
	seats := []Seat{seatOf("m1b"), seatOf("m1c"), seatOf("m1e"), {Machine: "m1e", Installation: "/second/metasystem"}}
	picture, unreadable := Read(home, seats, aliveProber{}, now, 20*time.Minute)
	if len(picture.Cards) != 1 || picture.Cards[0].Goal != "goal-x" {
		t.Fatalf("believed %+v, want goal-x alone", picture.Cards)
	}
	var stranger, shared int
	for _, u := range unreadable {
		if strings.HasSuffix(u.Path, "stranger") {
			stranger++
		}
		if strings.HasSuffix(u.Path, "m1e") {
			shared++
		}
	}
	if stranger != 1 || shared != 1 {
		t.Fatalf("unreadable %+v: want the unregistered directory and the shared nickname once each", unreadable)
	}
	var foreign, sharedUnknown int
	for _, u := range picture.Unknown {
		if u.Goal == "goal-y" && strings.Contains(u.Reason, "not the armed checkout") {
			foreign++
		}
		if u.Seat.Machine == "m1e" && strings.Contains(u.Reason, "shared") {
			sharedUnknown++
		}
		if u.Goal == "goal-s" || u.Goal == "goal-z" {
			t.Errorf("a card of an unread directory was read: %+v", u)
		}
	}
	if foreign != 1 || sharedUnknown != 2 {
		t.Fatalf("unknown %+v: want the foreign installation and both shared checkouts", picture.Unknown)
	}
}

// TestBoardImportsNothingAboveIt: board imports only the standard library,
// atomicfile and identity, so every stage owner can import it.
func TestBoardImportsNothingAboveIt(t *testing.T) {
	t.Parallel()
	allowed := map[string]bool{
		"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile": true,
		"github.com/widoriezebos/agentic-tools/metasystem/internal/identity":   true,
	}
	files, _ := filepath.Glob("*.go")
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range parsed.Imports {
			path, _ := strconv.Unquote(spec.Path.Value)
			if strings.Contains(path, ".") && !allowed[path] {
				t.Errorf("%s imports %s; board may import only the standard library, atomicfile and identity", file, path)
			}
		}
		source, _ := os.ReadFile(file)
		if strings.Contains(string(source), "internal/config") {
			t.Errorf("%s reads a setting; Write and Read take their inputs as values", file)
		}
	}
}

func jsonMarshal(card Card) ([]byte, error) { return json.MarshalIndent(card, "", "  ") }
