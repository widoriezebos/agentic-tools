package board

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// ReasonStalled is the reason of a card whose last real progress is older
// than the stall bound; the card carries the stamp a view renders.
const ReasonStalled = "stalled"

// Unknown is a card, or a seat, the reader cannot believe: never near, and
// always named with its reason.
type Unknown struct {
	Seat   Seat   `json:"seat"`
	Goal   string `json:"goal,omitempty"`
	Reason string `json:"reason"`
	Card   *Card  `json:"card,omitempty"`
}

// Picture is the classified board: the cards the reader believes, and the
// ones it cannot.
type Picture struct {
	Cards   []Card    `json:"cards"`
	Unknown []Unknown `json:"unknown"`
}

// Unreadable is a part of the board the reader reports once and does not
// read.
type Unreadable struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
	// Stray is a seat directory no armed seat names: a leftover the reader
	// skips, not a part of the board it failed to read.
	Stray bool `json:"stray,omitempty"`
}

// The reasons a seat or a card is Unknown because it could not be read,
// which a reader tells apart from the Unknowns classification decides.
const (
	ReasonSeatUnreadable = "seat directory unreadable"
	ReasonCardUnreadable = "card unreadable"
)

// Read reads the cards of exactly the armed seats its caller gives and
// classifies them. A seat directory with no armed seat behind it is reported
// once and not read; two armed checkouts with one nickname are reported and
// both read as Unknown; a directory whose name fails the safe-name rule is
// not listed at all. Read reads no setting: every input is a value.
func Read(home string, seats []Seat, prober identity.Prober, now time.Time, stall time.Duration) (Picture, []Unreadable) {
	var unreadable []Unreadable
	var picture Picture
	board := Dir(home)
	byMachine := map[string][]Seat{}
	for _, seat := range seats {
		byMachine[seat.Machine] = append(byMachine[seat.Machine], seat)
	}
	var cards []Card
	for _, machine := range sortedKeys(byMachine) {
		armed := byMachine[machine]
		if !SafeName(machine) {
			unreadable = append(unreadable, Unreadable{Path: machine, Reason: checkName("seat nickname", machine).Error()})
			continue
		}
		if len(armed) > 1 {
			unreadable = append(unreadable, Unreadable{Path: filepath.Join(board, machine), Reason: fmt.Sprintf("the nickname %s names %d armed checkouts; neither is read", machine, len(armed))})
			for _, seat := range armed {
				picture.Unknown = append(picture.Unknown, Unknown{Seat: seat, Reason: fmt.Sprintf("nickname %s shared by %d armed checkouts", machine, len(armed))})
			}
			continue
		}
		read, problems := readSeat(filepath.Join(board, machine), armed[0])
		cards = append(cards, read...)
		picture.Unknown = append(picture.Unknown, problems...)
	}
	entries, err := os.ReadDir(board)
	switch {
	case err == nil:
		for _, entry := range entries {
			name := entry.Name()
			if !entry.IsDir() || !SafeName(name) || name == GoalNamespace {
				continue
			}
			if _, armed := byMachine[name]; !armed {
				unreadable = append(unreadable, Unreadable{Path: filepath.Join(board, name), Reason: "no armed seat on this host is named " + name, Stray: true})
			}
		}
	case !errors.Is(err, fs.ErrNotExist):
		unreadable = append(unreadable, Unreadable{Path: board, Reason: "BOARD_UNREADABLE: the board cannot be listed: " + err.Error()})
	}
	classified := Classify(seats, cards, prober, now, stall)
	picture.Cards = classified.Cards
	picture.Unknown = append(picture.Unknown, classified.Unknown...)
	return picture, unreadable
}

// readSeat reads one seat's card files; a malformed card or one filed under
// another goal's name is Unknown.
func readSeat(dir string, seat Seat) ([]Card, []Unknown) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, []Unknown{{Seat: seat, Reason: ReasonSeatUnreadable + ": " + err.Error()}}
	}
	var cards []Card
	var unknown []Unknown
	for _, entry := range entries {
		name := entry.Name()
		goal, ok := strings.CutSuffix(name, ".json")
		if !ok || entry.IsDir() || !SafeName(goal) {
			continue
		}
		card, readable := readCardFile(filepath.Join(dir, name))
		switch {
		case !readable:
			unknown = append(unknown, Unknown{Seat: seat, Goal: goal, Reason: ReasonCardUnreadable})
		case card.Goal != goal:
			unknown = append(unknown, Unknown{Seat: seat, Goal: goal, Reason: "card names goal " + card.Goal, Card: &card})
		case card.Seat.Machine != seat.Machine:
			unknown = append(unknown, Unknown{Seat: seat, Goal: goal, Reason: "card names seat " + card.Seat.Machine, Card: &card})
		default:
			cards = append(cards, card)
		}
	}
	return cards, unknown
}

// Classify is the one function that turns cards into believed and Unknown,
// used by every reader, whether the cards came from disk or as values. A card
// is Unknown when its schema is newer than this reader, its seat is not the
// armed checkout of its nickname, its owner is dead, reused or cannot be
// probed, its last real progress is older than stall, or any stamp lies
// further ahead of now than stall.
func Classify(seats []Seat, cards []Card, prober identity.Prober, now time.Time, stall time.Duration) Picture {
	armed := map[string][]Seat{}
	for _, seat := range seats {
		armed[seat.Machine] = append(armed[seat.Machine], seat)
	}
	var picture Picture
	for _, card := range cards {
		if reason := unknownReason(card, armed[card.Seat.Machine], prober, now, stall); reason != "" {
			kept := card
			picture.Unknown = append(picture.Unknown, Unknown{Seat: card.Seat, Goal: card.Goal, Reason: reason, Card: &kept})
			continue
		}
		picture.Cards = append(picture.Cards, card)
	}
	sort.SliceStable(picture.Cards, func(i, j int) bool {
		if picture.Cards[i].Seat.Machine != picture.Cards[j].Seat.Machine {
			return picture.Cards[i].Seat.Machine < picture.Cards[j].Seat.Machine
		}
		return picture.Cards[i].Goal < picture.Cards[j].Goal
	})
	return picture
}

func unknownReason(card Card, armed []Seat, prober identity.Prober, now time.Time, stall time.Duration) string {
	if card.SchemaVersion > SchemaVersion {
		return fmt.Sprintf("schema %d is newer than this reader's %d", card.SchemaVersion, SchemaVersion)
	}
	if !card.Stage.Valid() {
		return fmt.Sprintf("stage %q is not in the vocabulary", card.Stage)
	}
	switch {
	case len(armed) == 0:
		return "seat " + card.Seat.Machine + " is not armed on this host"
	case len(armed) > 1:
		return fmt.Sprintf("nickname %s shared by %d armed checkouts", card.Seat.Machine, len(armed))
	case filepath.Clean(armed[0].Installation) != filepath.Clean(card.Seat.Installation):
		return "installation " + card.Seat.Installation + " is not the armed checkout " + armed[0].Installation
	}
	for _, stamp := range []time.Time{card.Since, card.LastProgressAt, card.Writer.At} {
		if stamp.Sub(now) > stall {
			return "malformed: a stamp lies " + roundMinutes(stamp.Sub(now)) + " ahead of this clock"
		}
	}
	if card.Stage.Terminal() {
		return ""
	}
	if card.Stage.processBound() {
		if card.Owner == nil || card.Owner.Pid <= 0 {
			return "no owner"
		}
		if reason := ownerReason(*card.Owner, prober); reason != "" {
			return reason
		}
	}
	if card.Stage.progressing() && now.Sub(card.LastProgressAt) > stall {
		return ReasonStalled
	}
	return ""
}

// ownerReason probes the owner with the recorded start, the rule of the
// owner invocation: a pid that is gone is dead, a pid whose start differs was
// reused, and a probe that cannot decide is never belief.
func ownerReason(owner Owner, prober identity.Prober) string {
	if prober == nil {
		return "owner unprobeable"
	}
	exact, state, err := prober.Probe(owner.Pid)
	switch {
	case err != nil || state == identity.Unknown:
		return fmt.Sprintf("owner pid %d unprobeable", owner.Pid)
	case state == identity.Dead:
		return fmt.Sprintf("writer dead (pid %d)", owner.Pid)
	case owner.PidStartedAt != 0 && exact.StartedAt.Unix() != owner.PidStartedAt:
		return fmt.Sprintf("owner pid %d reused", owner.Pid)
	}
	return ""
}

func roundMinutes(d time.Duration) string {
	return fmt.Sprintf("%d min", int(d.Round(time.Minute)/time.Minute))
}

func sortedKeys(values map[string][]Seat) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
