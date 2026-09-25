// Package fleet composes the Fleet page: who is doing what, and whether
// execution is healthy.
//
// It answers from two sources and nothing else — the presence copy this
// interface fetched for itself, for standings, and one captured accepted tip,
// for who holds what. It judges nothing the seat package judges: standings
// come from seat.Fleet, claims from one snapshot.Observation and its board
// projection, and this file joins them.
//
// Two rules of the design are load-bearing here.
//
// One captured ledger. Claims, the tip, titles and lanes all come from ONE
// observation, taken once per request, never from seat.Claims, which captures
// its own tip and returns neither the tip nor the projection: two captures
// could name a holder from one commit and a title from another.
//
// The interface never writes the steward's files. seat.Fleet freezes `since`
// against the standings file the tick keeps, and mints the request's own
// instant as a first observation where that file has nothing to say. On an
// interface checkout — which may be unarmed, and which never writes that file
// — that minted instant would be a lie dated now. So `since` survives only
// where the tick's own observation names the standing this page just judged,
// and the flag words are regenerated after that normalisation rather than
// taken from seat.SilentHolder, whose "no presence record" branch fires for
// any machine without a `since` and is false for an unreachable machine with
// a perfectly valid record.
//
// The design is plans/designs/user-interface/g1-s42-the-fleet-panel.md.
package fleet

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// SchemaVersion is the shape of the resource a reader parses.
const SchemaVersion = 1

// The three provenances of the presence copy a page was composed from.
const (
	// SourceInterface is the interface's own namespace, after a fetch of its
	// own succeeded.
	SourceInterface = "the interface"
	// SourceTick is the steward tick's canonical copy, read while this
	// interface's own fetch has not yet succeeded. It carries no succeededAt:
	// the tick's standings ReadAt is an observation time and not a fetch time.
	SourceTick = "the tick"
	// SourceLocal is LocalMode, where nothing is fetched and the publishing
	// refs are read in place.
	SourceLocal = "local refs"
)

/* ----------------------------------------------------------- the payload -- */

// Page is GET /api/fleet: one reading, composed on the server, which the
// browser draws and judges nothing about.
type Page struct {
	SchemaVersion int    `json:"schemaVersion"`
	ReadAt        string `json:"readAt"`
	Copy          Copy   `json:"copy"`
	Claims        Claims `json:"claims"`
	This          This   `json:"this"`
	// NeedsYou is every hold whose machine is unreachable, or unknown by the
	// absence of a record: the goals a human may want to steal or resume.
	NeedsYou []Held `json:"needsYou"`
	// Machines is one row per machine: this seat first, then machines with a
	// flagged hold, then reachable, then unreachable, then unknown.
	Machines []Machine `json:"machines"`
}

// Copy is what the fetch owner knows about the presence copy this page was
// composed from. Attempt, success and failure are kept apart: a failure
// preserves the last success, and a success that brought nothing is a success
// with an empty copy rather than a clone that has never fetched.
type Copy struct {
	Source      string `json:"source"`
	AttemptedAt string `json:"attemptedAt"`
	SucceededAt string `json:"succeededAt"`
	FailedAt    string `json:"failedAt"`
	// Problem is the last failure's own words, kept while the previously
	// fetched refs still stand and are still reported.
	Problem string `json:"problem"`
}

// Claims is the one captured accepted tip every holder on this page was read
// from, and the reason it could not be read.
type Claims struct {
	Tip         string `json:"tip"`
	Unavailable string `json:"unavailable"`
}

// This is the seat the interface is running on: its nickname, whether
// supervision is armed here, what it last published, what it is running, and
// the steward's LAST RECORDED health verdict, presented as such.
type This struct {
	Machine    string `json:"machine"`
	NoNickname bool   `json:"noNickname"`
	// Armed is a word and not a boolean: armed, not armed, stale, or
	// unreadable. A verdict older than the presence threshold is stale rather
	// than false, and a health file that cannot be read is named rather than
	// read as absent.
	Armed       string       `json:"armed"`
	Health      *Health      `json:"health"`
	Publication *Publication `json:"publication"`
	// PublicationProblem is a publication state that could not be read, which
	// is not the same as a seat that has never published.
	PublicationProblem string `json:"publicationProblem"`
	// Running is this seat's own newest delegate chain, from the local jobs.
	Running *Running `json:"running"`
	// RunningProblem is the jobs reader's own explanation, carried rather
	// than shown as idle: a machine that cannot say what it is running says
	// so.
	RunningProblem string `json:"runningProblem"`
}

// The four words Armed takes.
const (
	ArmedYes        = "armed"
	ArmedNo         = "not armed"
	ArmedStale      = "stale"
	ArmedUnreadable = "unreadable"
)

// Health is the steward's last recorded verdict, with the instant it was
// recorded at, so the page can say "last recorded 12 min ago" rather than
// presenting a past verdict as the present.
type Health struct {
	State      string `json:"state"`
	ObservedAt string `json:"observedAt"`
	// Problem names a health file that could not be read or parsed.
	Problem string `json:"problem"`
	Roles   []Role `json:"roles"`
}

// Role is one line of that verdict.
type Role struct {
	Role   string `json:"role"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// Publication is this machine's own publishing, as the seat package records
// it. It is local and never published.
type Publication struct {
	LastAttemptAt string `json:"lastAttemptAt"`
	LastSuccessAt string `json:"lastSuccessAt"`
	LastOutcome   string `json:"lastOutcome"`
	Rung          int    `json:"rung"`
	Detail        string `json:"detail"`
}

// Running is the newest non-terminal delegate chain of one machine. StartedAt
// is nullable on purpose: a pending-setup reservation has none, and a page
// that invented one would say a job was running that has not begun.
type Running struct {
	Job       string  `json:"job"`
	Role      string  `json:"role"`
	Round     int     `json:"round"`
	Goal      string  `json:"goal"`
	StartedAt *string `json:"startedAt"`
}

// Held is one goal a machine holds, with the flag its holder's standing
// raises. A flag is never an act: the goal stays claimed.
type Held struct {
	Goal  string `json:"goal"`
	Title string `json:"title"`
	// Lane is the projection's own lane id; the page titles it from the lane
	// register the board reads, so there is one owner for a lane's name.
	Lane     string `json:"lane"`
	Machine  string `json:"machine"`
	Standing string `json:"standing"`
	Since    string `json:"since"`
	Flag     string `json:"flag"`
}

// Machine is one row of the fleet.
type Machine struct {
	Machine  string `json:"machine"`
	Standing string `json:"standing"`
	Reason   string `json:"reason"`
	// AgeSeconds is the reader's clock minus the record's tickAt, negative
	// for a record dated ahead. Null without a record.
	AgeSeconds *int64 `json:"ageSeconds"`
	// Seen is the record's own tickAt, whatever the standing, so a row can
	// say when a machine was last seen even while it is unreachable.
	Seen string `json:"seen"`
	// Since is the first observation of this standing, and is empty where
	// this interface has no frozen observation to name.
	Since      string   `json:"since"`
	Running    *Running `json:"running"`
	Engine     string   `json:"engine"`
	Generation int      `json:"generation"`
	Holds      []Held   `json:"holds"`
	This       bool     `json:"this"`
}

/* ------------------------------------------------------- the composition -- */

// Inputs is everything one reading of the page needs, gathered by the caller
// so that this package opens nothing and fetches nothing.
type Inputs struct {
	// This and NoNickname are the checkout's own nickname.
	This       string
	NoNickname bool
	// Presence is the joined presence copy, read from one namespace.
	Presence seat.Copy
	// Copy is what the fetch owner knows about that copy.
	Copy Copy
	// Observation is the one captured accepted ledger, and Board its
	// projection. Both are of the same commit.
	Observation snapshot.Observation
	Board       backlog.Board
	// Previous is the tick's standings file, the only frozen `since` this
	// interface may name.
	Previous map[string]seat.Observation
	Window   time.Duration
	// Publication and PublicationProblem are seat.LoadPublicationState's two
	// answers: absent, and unreadable.
	Publication        *seat.PublicationState
	PublicationProblem string
	// Health is the steward's last recorded verdict for this seat.
	Health *Health
	// Running and RunningProblem are seat.NewestChain over the local jobs.
	Running        *seat.Chain
	RunningProblem string
}

// Compose is the whole page, from the inputs above, as they stood at now.
func Compose(in Inputs, now time.Time) Page {
	now = now.UTC()
	claims, unavailable := claimsOf(in.Observation, in.Board)
	standings := seat.Fleet(seat.FleetInput{
		This: in.This, Copy: in.Presence, Claims: claims, ClaimsUnavailable: unavailable,
		Previous: in.Previous, Now: now, Window: in.Window,
	})
	held := holdsOf(in.Board)
	machines := make([]Machine, 0, len(standings))
	needs := []Held{}
	for _, line := range standings {
		// D4: the interface never writes the steward's files, so a `since`
		// the tick did not observe is a `since` this page does not have.
		since := Since(in.Previous, line)
		row := Machine{
			Machine: line.Machine, Standing: string(line.Standing),
			Reason: reasonOf(line), AgeSeconds: line.AgeSeconds,
			Since: since, This: line.This, Holds: []Held{},
		}
		if line.Record != nil {
			row.Seen = line.Record.TickAt
			row.Engine = line.Record.Engine
			row.Generation = line.Record.Generation
			row.Running = runningOf(line.Record.Chain)
		}
		flag := ""
		if unavailable == "" {
			flag = Flag(line, since, now)
		}
		for _, goal := range line.Holds {
			one := held[goal]
			one.Goal, one.Machine = goal, line.Machine
			one.Standing, one.Since, one.Flag = string(line.Standing), since, flag
			row.Holds = append(row.Holds, one)
			if needsYou(line) {
				needs = append(needs, one)
			}
		}
		machines = append(machines, row)
	}
	sort.SliceStable(machines, func(i, j int) bool {
		left, right := rankOf(machines[i]), rankOf(machines[j])
		if left != right {
			return left < right
		}
		return machines[i].Machine < machines[j].Machine
	})
	sortSilences(needs)
	return Page{
		SchemaVersion: SchemaVersion,
		ReadAt:        seat.FormatTime(now),
		Copy:          in.Copy,
		Claims:        Claims{Tip: in.Observation.Tip, Unavailable: unavailable},
		This:          thisSeat(in, now),
		NeedsYou:      needs,
		Machines:      machines,
	}
}

// claimsOf reads the holders out of ONE observation's board projection, so a
// holder and the title beside it are always from the same commit. A ledger
// this interface could not read raises no flag at all: standings still stand,
// but nothing is said about who holds what.
func claimsOf(observed snapshot.Observation, board backlog.Board) (map[string][]string, string) {
	if observed.State != snapshot.StateRead {
		reason := observed.Message
		if reason == "" {
			reason = "the accepted ledger could not be read"
		}
		return map[string][]string{}, reason
	}
	claims := map[string][]string{}
	for _, row := range board.Rows {
		if row.Claim == nil || row.Claim.Machine == "" {
			continue
		}
		claims[row.Claim.Machine] = append(claims[row.Claim.Machine], row.ID)
	}
	for machine := range claims {
		sort.Strings(claims[machine])
	}
	return claims, ""
}

// holdsOf is every live row by its goal id, with the title and lane the same
// projection carries.
func holdsOf(board backlog.Board) map[string]Held {
	rows := make(map[string]Held, len(board.Rows))
	for _, row := range board.Rows {
		rows[row.ID] = Held{Goal: row.ID, Title: lede(row.Intent), Lane: string(row.Lane)}
	}
	return rows
}

// Since keeps the tick's own first observation of a standing, and nothing
// else. seat.Fleet mints now where the file says nothing, and a reader that
// writes no standings file of its own has no standing to mint from: a silence
// dated the instant it was first noticed would be a lie about when it began.
//
// It is exported because three surfaces carry the same flag — this page, the
// board's card line and `goal next` — and a second normalisation written
// beside one of them would be a second account of when a machine went quiet.
func Since(previous map[string]seat.Observation, line seat.MachineStanding) string {
	was, seen := previous[line.Machine]
	if !seen || was.Standing != line.Standing {
		return ""
	}
	return was.Since
}

// reasonOf keeps the three reasons an unknown machine can have apart: no
// record at all, a record that could not be read, and a record from the
// future. A reachable or unreachable machine keeps the judgement's own words.
func reasonOf(line seat.MachineStanding) string {
	if line.Standing != seat.Unknown {
		return line.Reason
	}
	switch {
	case line.Malformed != "":
		return "presence unreadable: " + line.Malformed
	case line.Record == nil:
		return "no presence record"
	default:
		return line.Reason
	}
}

// Flag is the one sentence a claim's row carries when its holder has gone
// silent, composed after the Since normalisation above rather than taken from
// seat.SilentHolder, whose "no presence record" branch fires for any machine
// without a frozen since and is false for an unreachable machine holding a
// perfectly valid record.
//
// A machine known only by the absence of a record has not "gone silent": it
// has published nothing, which is a different thing and is said differently.
func Flag(line seat.MachineStanding, since string, now time.Time) string {
	if line.Standing == seat.Reachable || len(line.Holds) == 0 {
		return ""
	}
	if line.Standing == seat.Unknown {
		switch {
		case line.Malformed != "":
			return "held by " + line.Machine + ", whose presence record is unreadable"
		case line.Record == nil:
			return "held by " + line.Machine + ", which has published no presence"
		default:
			return "held by " + line.Machine + ", " + line.Reason
		}
	}
	if since == "" {
		return "held by " + line.Machine + ", unreachable"
	}
	return "held by " + line.Machine + ", unreachable since " + clockWords(since, now)
}

// needsYou selects the holds a human may want to act on: a machine that has
// gone silent with a valid record, and a machine that holds a goal while
// having published no presence at all.
func needsYou(line seat.MachineStanding) bool {
	if len(line.Holds) == 0 {
		return false
	}
	if line.Standing == seat.Unreachable {
		return true
	}
	return line.Standing == seat.Unknown && line.Record == nil && line.Malformed == ""
}

// sortSilences puts the dated silences first, oldest first, and the undated
// after them in a stable order of their own: an unknown `since` cannot be
// compared with a known one, and sorting it as though it were the epoch would
// put the least-known row at the top.
func sortSilences(needs []Held) {
	sort.SliceStable(needs, func(i, j int) bool {
		left, right := needs[i], needs[j]
		if (left.Since == "") != (right.Since == "") {
			return left.Since != ""
		}
		if left.Since != right.Since {
			return left.Since < right.Since
		}
		if left.Machine != right.Machine {
			return left.Machine < right.Machine
		}
		return left.Goal < right.Goal
	})
}

// rankOf is the row order of the table: this seat first, then the machines
// holding a flagged goal, then the reachable, then the silent, then the
// unknown.
func rankOf(row Machine) int {
	switch {
	case row.This:
		return 0
	case flagged(row):
		return 1
	case row.Standing == string(seat.Reachable):
		return 2
	case row.Standing == string(seat.Unreachable):
		return 3
	default:
		return 4
	}
}

func flagged(row Machine) bool {
	for _, hold := range row.Holds {
		if hold.Flag != "" {
			return true
		}
	}
	return false
}

func runningOf(chain *seat.Chain) *Running {
	if chain == nil {
		return nil
	}
	return &Running{
		Job: chain.Job, Role: chain.Role, Round: chain.Round,
		Goal: chain.Goal, StartedAt: chain.StartedAt,
	}
}

// thisSeat is the checkout the interface is serving: what it is called, what
// it last published, what it is running, and what the steward last recorded
// about it.
func thisSeat(in Inputs, now time.Time) This {
	seat_ := This{
		Machine: in.This, NoNickname: in.NoNickname,
		Health: in.Health, PublicationProblem: in.PublicationProblem,
		Running: runningOf(in.Running), RunningProblem: in.RunningProblem,
	}
	if in.Publication != nil {
		seat_.Publication = &Publication{
			LastAttemptAt: in.Publication.LastAttemptAt,
			LastSuccessAt: in.Publication.LastSuccessAt,
			LastOutcome:   in.Publication.LastOutcome,
			Rung:          in.Publication.Rung,
			Detail:        in.Publication.Detail,
		}
	}
	seat_.Armed = armedWord(in, now)
	return seat_
}

// armedWord tells the four cases apart. A verdict is a PAST observation, so
// one older than the presence threshold — the same max(window, three of this
// machine's own ticks) every standing is judged by — is stale rather than a
// statement about now, and a file that could not be read is named rather than
// read as an unarmed seat.
func armedWord(in Inputs, now time.Time) string {
	if in.Health == nil {
		return ArmedNo
	}
	if in.Health.Problem != "" {
		return ArmedUnreadable
	}
	tickSeconds := 0
	if in.Publication != nil {
		tickSeconds = in.Publication.TickSeconds
	}
	if at, err := time.Parse(time.RFC3339, in.Health.ObservedAt); err == nil {
		if now.Sub(at.UTC()) > seat.Threshold(in.Window, tickSeconds) {
			return ArmedStale
		}
	}
	for _, role := range in.Health.Roles {
		if role.Role == "steward-runner" {
			if role.Status == "alive" {
				return ArmedYes
			}
			return ArmedNo
		}
	}
	return ArmedNo
}

/* ---------------------------------------------------------------- words -- */

// clockWords says an instant the way the flag does: the time of day, with the
// date before it only where it was not today, because a silence that began
// this morning and one that began last week must not read alike.
func clockWords(at string, now time.Time) string {
	parsed, err := time.Parse(time.RFC3339, at)
	if err != nil {
		return at
	}
	parsed = parsed.UTC()
	clock := twoDigits(parsed.Hour()) + ":" + twoDigits(parsed.Minute())
	if parsed.Year() == now.Year() && parsed.YearDay() == now.YearDay() {
		return clock
	}
	return parsed.Format("2006-01-02") + " " + clock
}

func twoDigits(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}

// ledeRunes is how much of a goal's intent a chip carries, which is what
// Overview's own rows carry.
const ledeRunes = 80

// lede is the first sentence of an intent, bounded, the way every other list
// of goals in this interface writes one.
func lede(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if at := strings.Index(text, ". "); at >= 0 {
		text = text[:at+1]
	}
	runes := []rune(text)
	if len(runes) <= ledeRunes {
		return text
	}
	cut := ledeRunes
	for cut > 0 && runes[cut] != ' ' {
		cut--
	}
	if cut == 0 {
		cut = ledeRunes
	}
	return strings.TrimRight(string(runes[:cut]), " ,;:") + "…"
}
