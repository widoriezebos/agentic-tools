package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

type pipelineProber struct{}

func (pipelineProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if pid == 999 {
		return identity.Exact{}, identity.Dead, nil
	}
	return identity.Exact{Pid: pid, StartedAt: time.Unix(1000, 0)}, identity.Alive, nil
}

// pipelineHost is a fixture host: a registry, checkout directories, their
// nicknames and a board, all under one temporary home.
type pipelineHost struct {
	root, registryPath, home string
	nicknames                map[string]string
}

func newPipelineHost(t *testing.T) *pipelineHost {
	t.Helper()
	root := t.TempDir()
	host := &pipelineHost{root: root, registryPath: filepath.Join(root, "armed-checkouts.jsonl"), home: filepath.Join(root, ".metasystem"), nicknames: map[string]string{}}
	return host
}

// checkout creates an armed (or, closed, disarmed) checkout named machine.
func (host *pipelineHost) checkout(t *testing.T, name, machine string, closed, gone bool) string {
	t.Helper()
	path := filepath.Join(host.root, name, "metasystem")
	if !gone {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	host.nicknames[path] = machine
	events := []string{registry.EventRelaunched}
	if closed {
		events = append(events, registry.EventExited)
	}
	for _, event := range events {
		payload, _ := json.Marshal(map[string]any{"schemaVersion": 1, "event": event, "checkoutPath": path, "ownerTag": "tag-" + name,
			"at": "2026-09-29T08:00:00Z", "generation": 1, "watcherTag": "w-" + name, "reaperTag": "r-" + name, "retiredThrough": 0,
			"reason": "shutdown", "teardownComplete": true})
		if err := registry.AppendFrame(host.registryPath, payload); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func (host *pipelineHost) card(t *testing.T, machine, installation, goalID string, stage board.Stage, at time.Time) {
	t.Helper()
	card := board.Card{Seat: board.Seat{Machine: machine, Installation: installation}, Goal: goalID, Stage: stage, Writer: board.Writer{At: at}}
	if stage == board.StageBuild {
		card.Owner = &board.Owner{Pid: 41, PidStartedAt: 1000}
	}
	if err := board.WriteAt(host.home, card); err != nil {
		t.Fatal(err)
	}
}

func (host *pipelineHost) source(claims map[string]string) batchowner.HostPipeline {
	source := batchowner.ProductionPipeline(20*time.Minute, func() (map[string]string, error) { return claims, nil })
	source.Registry = func() (string, error) { return host.registryPath, nil }
	source.Home = func() (string, error) { return host.home, nil }
	source.Prober = pipelineProber{}
	source.Machine = func(checkout string) (string, error) {
		if machine, ok := host.nicknames[checkout]; ok && machine != "" {
			return machine, nil
		}
		return "", errors.New("no nickname")
	}
	return source
}

func unknownReasons(picture batch.BoardPicture) map[string]string {
	reasons := map[string]string{}
	for _, unknown := range picture.Unknown {
		reasons[unknown.Seat.Machine+"/"+unknown.Goal] = unknown.Reason
	}
	return reasons
}

// TestBatchPipelineReadsOnlyTheArmedSeats (R24, U10b-1; the production
// source's half of TestBoardReadsOnlyTheSeatsItIsGiven): a fixture registry
// with two open seats, a closed one, one whose directory is gone, a board
// directory with no registration, a card of a foreign installation and two
// armed checkouts with one nickname: the source hands board.Read exactly
// the armed, present seats with their nicknames, the foreign card is
// Unknown with its reason, and the shared nickname makes both Unknown.
func TestBatchPipelineReadsOnlyTheArmedSeats(t *testing.T) {
	t.Parallel()
	host := newPipelineHost(t)
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	m1b := host.checkout(t, "b", "m1b", false, false)
	m1c := host.checkout(t, "c", "m1c", false, false)
	closed := host.checkout(t, "closed", "m1x", true, false)
	host.checkout(t, "gone", "m1g", false, true)
	host.checkout(t, "e1", "m1e", false, false)
	host.checkout(t, "e2", "m1e", false, false)
	host.card(t, "m1b", m1b, "goal-b", board.StageBuild, now)
	host.card(t, "m1c", "/elsewhere/metasystem", "goal-c", board.StageBuild, now)
	host.card(t, "m1x", closed, "goal-x", board.StageBuild, now)
	host.card(t, "m1g", "/gone", "goal-g", board.StageBuild, now)
	host.card(t, "stranger", "/stranger", "goal-s", board.StageBuild, now)
	_ = m1c
	picture := host.source(map[string]string{"goal-b": "m1b", "goal-c": "m1c", "goal-x": "m1x", "goal-g": "m1g", "goal-s": "stranger"}).Board(now)
	if !picture.Readable || len(picture.Cards) != 1 || picture.Cards[0].Goal != "goal-b" {
		t.Fatalf("believed %+v unknown %v (%v %s)", picture.Cards, unknownReasons(picture), picture.Readable, picture.Reason)
	}
	reasons := unknownReasons(picture)
	if !strings.Contains(reasons["m1c/goal-c"], "not the armed checkout") {
		t.Fatalf("the foreign installation: %v", reasons)
	}
	shared := 0
	for _, unknown := range picture.Unknown {
		if unknown.Seat.Machine == "m1e" && strings.Contains(unknown.Reason, "shared") {
			shared++
		}
		if unknown.Goal == "goal-x" || unknown.Goal == "goal-g" || unknown.Goal == "goal-s" {
			t.Fatalf("a closed, gone or unregistered seat was read: %+v", unknown)
		}
	}
	if shared != 2 {
		t.Fatalf("both checkouts of the shared nickname are Unknown: %v", reasons)
	}
}

// TestBatchPipelineKeepsTheRegistryError (R24, U10b-1; the source's half of
// TestArmedCheckoutsKeepTheirError): an unreadable registry is an
// unreadable picture with the cause; an empty registry is a readable,
// empty one.
func TestBatchPipelineKeepsTheRegistryError(t *testing.T) {
	t.Parallel()
	host := newPipelineHost(t)
	if err := os.Mkdir(host.registryPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if picture := host.source(nil).Board(time.Now()); picture.Readable || !strings.HasPrefix(picture.Reason, "registry: ") {
		t.Fatalf("an unreadable registry = %+v", picture)
	}
	empty := newPipelineHost(t)
	if picture := empty.source(nil).Board(time.Now()); !picture.Readable || len(picture.Cards)+len(picture.Unknown) != 0 {
		t.Fatalf("an empty registry = %+v", picture)
	}
}

// TestBatchPipelineChecksTheLedger (R24, U10b-1; the source's half of
// TestUnknownIsNeverNear): a card under m1b for a goal the ledger's live
// claim gives to m1c is claim moved; a live card for a goal nobody claims is
// not claimed; a goal claimed by a seat of this host with no card is no
// card; the same card with a matching claim is believed.
func TestBatchPipelineChecksTheLedger(t *testing.T) {
	t.Parallel()
	host := newPipelineHost(t)
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	m1b := host.checkout(t, "b", "m1b", false, false)
	host.checkout(t, "c", "m1c", false, false)
	host.card(t, "m1b", m1b, "goal-moved", board.StageBuild, now)
	host.card(t, "m1b", m1b, "goal-unclaimed", board.StageBuild, now)
	host.card(t, "m1b", m1b, "goal-kept", board.StageBuild, now)
	host.card(t, "m1b", m1b, "goal-done", board.StageReleased, now)
	picture := host.source(map[string]string{"goal-moved": "m1c", "goal-kept": "m1b", "goal-silent": "m1c", "goal-elsewhere": "m1z"}).Board(now)
	reasons := unknownReasons(picture)
	if reasons["m1b/goal-moved"] != "claim moved to m1c" || reasons["m1b/goal-unclaimed"] != "not claimed" || reasons["m1c/goal-silent"] != "no card" {
		t.Fatalf("ledger reasons %v", reasons)
	}
	if _, named := reasons["m1z/goal-elsewhere"]; named {
		t.Fatal("a claim of another host's machine is none of this board's business")
	}
	var believed []string
	for _, card := range picture.Cards {
		believed = append(believed, card.Goal)
	}
	if strings.Join(believed, ",") != "goal-done,goal-kept" {
		t.Fatalf("believed %v", believed)
	}
}

// emptyHostBoard is a readable host board with nothing underway: an owner
// built for a test decides every start from its own joined units.
type emptyHostBoard struct{}

func (emptyHostBoard) Board(time.Time) batch.BoardPicture { return batch.BoardPicture{Readable: true} }
