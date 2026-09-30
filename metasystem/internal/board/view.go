package board

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The bridge's state as a one-shot view reports it: from the presence of
// the socket at its constant path, never from a connection (D14-r2, R25:
// one-shot commands never use the socket).
const (
	BridgeLive   = "live"
	BridgeAbsent = "absent"
)

// SocketPath is the bridge's socket under home: a constant path in the
// board's private host directory.
func SocketPath(home string) string { return filepath.Join(home, "host", "bridge.sock") }

// BridgeState is live when a socket stands at the bridge's path, absent
// otherwise. It connects to nothing.
func BridgeState(home string) string {
	info, err := os.Lstat(SocketPath(home))
	if err == nil && info.Mode()&fs.ModeSocket != 0 {
		return BridgeLive
	}
	return BridgeAbsent
}

// CheckClaims applies the checks only a reader of the goal ledger can make
// (D14B-04): a live card whose goal the live claim gives to another machine
// (claim moved: a delayed writer of a seat that handed over cannot
// resurrect it), a live card for a goal nobody claims (not claimed), and a
// goal a seat of this host claims with no card at all (no card: an older
// engine, or a writer that failed). Each is Unknown, never near. claims
// maps every live claimed goal to its holder.
func CheckClaims(picture Picture, seats []Seat, claims map[string]string) Picture {
	carded := map[string]bool{}
	var believed []Card
	for _, card := range picture.Cards {
		carded[card.Goal] = true
		if card.Stage.Terminal() {
			believed = append(believed, card)
			continue
		}
		holder, claimed := claims[card.Goal]
		kept := card
		switch {
		case !claimed:
			picture.Unknown = append(picture.Unknown, Unknown{Seat: card.Seat, Goal: card.Goal, Reason: "not claimed", Card: &kept})
		case holder != card.Seat.Machine:
			picture.Unknown = append(picture.Unknown, Unknown{Seat: card.Seat, Goal: card.Goal, Reason: "claim moved to " + holder, Card: &kept})
		default:
			believed = append(believed, card)
		}
	}
	for _, unknown := range picture.Unknown {
		carded[unknown.Goal] = true
	}
	goals := make([]string, 0, len(claims))
	for goalID := range claims {
		goals = append(goals, goalID)
	}
	sort.Strings(goals)
	for _, goalID := range goals {
		if carded[goalID] {
			continue
		}
		for _, seat := range seats {
			if seat.Machine == claims[goalID] {
				picture.Unknown = append(picture.Unknown, Unknown{Seat: seat, Goal: goalID, Reason: "no card"})
			}
		}
	}
	picture.Cards = believed
	return picture
}

// View is the classified board as every view shows it (D14-r2, R23): one
// entry per armed seat of this host, each with its goals. Readable and
// Bridge are the caller's: whether the registry could be read, and the
// bridge's state.
type View struct {
	Readable bool       `json:"readable"`
	Reason   string     `json:"reason,omitempty"`
	Bridge   string     `json:"bridge"`
	Seats    []SeatView `json:"seats"`
}

// SeatView is one armed seat and its goals; Unknown is a reason the whole
// seat cannot be read (a nickname two armed checkouts share).
type SeatView struct {
	Machine      string     `json:"machine"`
	Installation string     `json:"installation"`
	Unknown      string     `json:"unknown,omitempty"`
	Goals        []GoalView `json:"goals"`
}

// GoalView is one goal on a seat: its card's identifiers, numbers and times,
// or the reason it is Unknown.
type GoalView struct {
	Goal           string    `json:"goal"`
	Stage          Stage     `json:"stage,omitempty"`
	Round          *Round    `json:"round,omitempty"`
	Proof          *Proof    `json:"proof,omitempty"`
	Batch          string    `json:"batch,omitempty"`
	Since          time.Time `json:"since,omitzero"`
	LastProgressAt time.Time `json:"lastProgressAt,omitzero"`
	Unknown        string    `json:"unknown,omitempty"`
}

// NewView groups a classified picture by the armed seats it was read for.
// An Unknown whose seat is not among them is dropped: the view shows this
// host's seats.
func NewView(seats []Seat, picture Picture) View {
	view := View{Seats: []SeatView{}}
	index := map[string]int{}
	ordered := append([]Seat(nil), seats...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Machine < ordered[j].Machine })
	for _, seat := range ordered {
		if _, seen := index[seat.Machine]; seen {
			continue
		}
		index[seat.Machine] = len(view.Seats)
		view.Seats = append(view.Seats, SeatView{Machine: seat.Machine, Installation: seat.Installation, Goals: []GoalView{}})
	}
	for _, card := range picture.Cards {
		if at, ok := index[card.Seat.Machine]; ok {
			view.Seats[at].Goals = append(view.Seats[at].Goals, goalView(card, ""))
		}
	}
	for _, unknown := range picture.Unknown {
		at, ok := index[unknown.Seat.Machine]
		if !ok {
			continue
		}
		if unknown.Goal == "" {
			view.Seats[at].Unknown = unknown.Reason
			continue
		}
		entry := GoalView{Goal: unknown.Goal, Unknown: unknown.Reason}
		if unknown.Card != nil {
			entry = goalView(*unknown.Card, unknown.Reason)
		}
		view.Seats[at].Goals = append(view.Seats[at].Goals, entry)
	}
	return view
}

func goalView(card Card, unknown string) GoalView {
	return GoalView{Goal: card.Goal, Stage: card.Stage, Round: card.Round, Proof: card.Proof, Batch: card.Batch,
		Since: card.Since, LastProgressAt: card.LastProgressAt, Unknown: unknown}
}

// Lines renders the view for a person, in local time: a header with the
// bridge's state, then one line per seat naming its underway and Unknown
// goals and counting the claimed-idle and finished ones. Verbose adds one
// line per goal beneath its seat.
func (view View) Lines(now time.Time, location *time.Location, verbose bool) []string {
	bridge := " (bridge " + view.Bridge + ")"
	switch {
	case !view.Readable:
		return []string{"board unreadable (" + view.Reason + ")" + bridge}
	case len(view.Seats) == 0:
		return []string{"board: no armed seat on this host" + bridge}
	}
	lines := []string{fmt.Sprintf("board: %d seat%s on this host%s", len(view.Seats), plural(len(view.Seats)), bridge)}
	for _, seat := range view.Seats {
		if !verbose || seat.Unknown != "" || len(seat.Goals) == 0 {
			lines = append(lines, "  "+seat.Machine+": "+seatText(seat, now, location))
			continue
		}
		// Verbose gives each goal its own line; the seat line counts them
		// instead of naming them a second time (F5).
		lines = append(lines, fmt.Sprintf("  %s: %d goal%s", seat.Machine, len(seat.Goals), plural(len(seat.Goals))))
		for _, entry := range seat.Goals {
			lines = append(lines, "    "+goalText(entry, now, location))
		}
	}
	return lines
}

// GoalLine is one goal's own line, naming its seat; false when no seat of
// this host holds it.
func (view View) GoalLine(goalID string, now time.Time, location *time.Location) (string, bool) {
	for _, seat := range view.Seats {
		for _, entry := range seat.Goals {
			if entry.Goal != goalID {
				continue
			}
			text := goalText(entry, now, location)
			rest := strings.TrimPrefix(text, entry.Goal)
			return "board: " + entry.Goal + " on " + seat.Machine + rest, true
		}
	}
	return "", false
}

// Text is the seat's line after its nickname, as Lines prints it.
func (seat SeatView) Text(now time.Time, location *time.Location) string {
	return seatText(seat, now, location)
}

func seatText(seat SeatView, now time.Time, location *time.Location) string {
	if seat.Unknown != "" {
		return "unknown: " + seat.Unknown
	}
	var shown []string
	idle, finished := 0, 0
	for _, entry := range seat.Goals {
		switch {
		case entry.Unknown != "":
			shown = append(shown, goalText(entry, now, location))
		case entry.Stage == StageClaimedIdle:
			idle++
		case entry.Stage.Terminal():
			finished++
		default:
			shown = append(shown, goalText(entry, now, location))
		}
	}
	text := strings.Join(shown, "; ")
	if text == "" {
		text = "nothing underway"
	}
	var counts []string
	if idle > 0 {
		counts = append(counts, fmt.Sprintf("%d claimed idle", idle))
	}
	if finished > 0 {
		counts = append(counts, fmt.Sprintf("%d finished", finished))
	}
	if len(counts) > 0 {
		text += " (" + strings.Join(counts, ", ") + ")"
	}
	return text
}

// goalText is one goal as a person reads it: goal-x, review round 2 of 3
// since 10:12; goal-z unknown: writer dead (pid 77) since 09:40.
func goalText(entry GoalView, now time.Time, location *time.Location) string {
	if entry.Unknown != "" {
		text := entry.Goal + " unknown: " + entry.Unknown
		if !entry.LastProgressAt.IsZero() {
			text += " since " + clock(entry.LastProgressAt, now, location)
		}
		return text
	}
	text := entry.Goal + ", " + StageText(entry.Stage, entry.Round, entry.Proof)
	if entry.Stage == StageJoined && entry.Batch != "" {
		text += " batch " + entry.Batch
	}
	if !entry.Since.IsZero() {
		text += " since " + clock(entry.Since, now, location)
	}
	return text
}

// StageText is a stage as a person reads it: review round 2 of 3, unit
// proof 120 of 189, claimed idle.
func StageText(stage Stage, round *Round, proof *Proof) string {
	text := strings.ReplaceAll(string(stage), "-", " ")
	if proof != nil && proof.Planned > 0 && stage == StageUnitProof {
		text += fmt.Sprintf(" %d of %d", proof.Done, proof.Planned)
	}
	if round != nil && (stage == StageReview || stage == StageRevise) {
		if round.Max != nil {
			text += fmt.Sprintf(" round %d of %d", round.N, *round.Max)
		} else {
			text += fmt.Sprintf(" round %d", round.N)
		}
	}
	return text
}

// clock is a local time of day, with the date when it is not today's.
func clock(at, now time.Time, location *time.Location) string {
	if location == nil {
		location = time.Local
	}
	local, today := at.In(location), now.In(location)
	if local.Year() == today.Year() && local.YearDay() == today.YearDay() {
		return local.Format("15:04")
	}
	return local.Format("Jan 2 15:04")
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
