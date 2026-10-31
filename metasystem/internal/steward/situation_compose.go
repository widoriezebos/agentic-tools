package steward

// The composition is the page one situation makes: the first thing to do
// and the wake fields a wait compares. It reads nothing: every input is in
// the situation and every time comes from its clock.

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// Composition is one situation's page. Wake are its semantic facts, one per
// line and sorted, Digest their hash, and Deadlines the clock times after
// ReadAt at which a wake field changes with no file change.
type Composition struct {
	ReadAt    time.Time
	Machine   string
	First     FirstThing
	Wake      []string
	Digest    string
	Deadlines []time.Time
}

// FirstThing is what this seat does first. Kind is ask, red, continue,
// claim or wait, or unknown when a part the rule needs could not be read.
type FirstThing struct{ Kind, Subject, Why, Command string }

// Compose makes the page from one situation.
func Compose(s Situation) Composition {
	c := Composition{ReadAt: s.At, Machine: s.Machine, First: s.firstThing()}
	c.Wake, c.Deadlines = s.wake()
	sum := sha256.Sum256([]byte(strings.Join(c.Wake, "\n")))
	c.Digest = hex.EncodeToString(sum[:])
	return c
}

// env words times in the read's own zone and ages from its clock.
func (s Situation) env() textui.Env { return textui.Env{Now: s.At, Zone: s.At.Location()} }

// outstanding are the asks to this seat or its goals with no reply, open or
// past their deadline: the recipient decides, never the sender. Past the
// deadline first, oldest deadline first; then the rest, oldest first.
func (s Situation) outstanding() []SituationAsk {
	var asks []SituationAsk
	for _, ask := range s.Asks {
		root := ask.Thread.Root
		incoming := root != nil && root.Kind == board.KindAsk &&
			(root.To.Machine != "" && root.To.Machine == s.Machine || root.To.Goal != "" && slices.Contains(s.seatGoals(), root.To.Goal))
		if incoming && (ask.State == board.ThreadOpen || ask.State == board.ThreadExpired) {
			asks = append(asks, ask)
		}
	}
	slices.SortStableFunc(asks, func(a, b SituationAsk) int {
		switch aExpired, bExpired := a.State == board.ThreadExpired, b.State == board.ThreadExpired; {
		case aExpired && bExpired:
			return a.Thread.Root.DeadlineAt.Compare(*b.Thread.Root.DeadlineAt)
		case aExpired != bExpired:
			return map[bool]int{true: -1, false: 1}[aExpired]
		}
		return a.Thread.Root.At.Compare(b.Thread.Root.At)
	})
	return asks
}

// askLine names an ask's sender and recipient, its age and its deadline.
func (s Situation) askLine(ask SituationAsk) string {
	root, env := ask.Thread.Root, s.env()
	to, due := cmp.Or(root.To.Machine, "goal "+root.To.Goal), "no deadline"
	if ask.State == board.ThreadExpired {
		due = "past its deadline " + env.Time(*root.DeadlineAt)
	} else if root.DeadlineAt != nil {
		due = "due " + env.Time(*root.DeadlineAt)
	}
	return fmt.Sprintf("from %s to %s, asked %s, %s", root.From.Machine, to, env.Ago(root.At), due)
}

// firstThing is the first of: an outstanding ask, a red this seat owns, the
// claim to continue, a goal to claim; else wait. A part the rule needs that
// failed is named, and never read as nothing to do.
func (s Situation) firstThing() FirstThing {
	var missing []string
	for _, part := range []string{"asks", "ledger"} {
		if reason := s.Unavailable[part]; reason != "" {
			missing = append(missing, part+" unavailable: "+reason)
		}
	}
	first := FirstThing{Kind: "wait", Why: "nothing; wait"}
	if asks := s.outstanding(); len(asks) > 0 {
		first = FirstThing{Kind: "ask", Subject: asks[0].Thread.ID, Why: "an ask " + s.askLine(asks[0]), Command: "metasystem agent inbox"}
	} else if len(s.Next.TrunkRedOwned) > 0 {
		red := s.Next.TrunkRedOwned[0]
		first = FirstThing{Kind: "red", Subject: red.ID, Why: cmp.Or(s.RedLine, "trunk red "+red.ID+"; fix it under "+cmp.Or(red.FixGoal, "a goal of its own"))}
	} else if next := goal.SelectNext(s.Next); next.Kind == goal.NextSelectionContinue {
		first = FirstThing{Kind: "continue", Subject: next.GoalID, Why: "this seat holds " + next.GoalID, Command: "metasystem goal claim " + next.GoalID}
	} else if next.Kind == goal.NextSelectionReady {
		first = FirstThing{Kind: "claim", Subject: next.GoalID, Why: next.GoalID + " is ready to claim", Command: "metasystem goal claim " + next.GoalID}
	} else if len(missing) > 0 {
		return FirstThing{Kind: "unknown", Why: "unknown: " + strings.Join(missing, "; ")}
	}
	if len(missing) > 0 {
		first.Why += "; " + strings.Join(missing, "; ")
	}
	return first
}

// wake are the situation's semantic facts and the deadlines that change one.
// Times, load, process starts, minutes and reasons are never among them.
func (s Situation) wake() (lines []string, deadlines []time.Time) {
	add := func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) }
	later := func(at time.Time) {
		if at.After(s.At) {
			deadlines = append(deadlines, at)
		}
	}
	for _, ask := range s.Asks {
		add("ask|%s|%s", ask.Thread.ID, ask.State)
		if root := ask.Thread.Root; ask.State == board.ThreadOpen && root != nil && root.DeadlineAt != nil {
			later(*root.DeadlineAt)
		}
	}
	add("lane|%s", s.Lane.Owner.State)
	if proof := s.Lane.RunningProof; proof != nil {
		add("proof|%s|%s", proof.Attempt, proof.State)
	}
	if s.Lane.Health != nil {
		for _, row := range s.Lane.Health.Rows() {
			add("row|%s|%s", row.Row, row.Subject)
		}
	}
	hold := "none"
	if s.Hold != nil {
		hold = s.Hold.Subject
		later(s.Hold.Until)
	}
	add("hold|%s", hold)
	if s.Ledger.Tree != nil {
		for id, file := range s.Ledger.Tree.Live {
			machine := ""
			if file.Claimed != nil {
				machine = file.Claimed.Machine
			}
			add("goal|%s|%s|%s|%t", id, file.State, machine, file.Landing != nil)
		}
	}
	for _, block := range s.Quota {
		add("quota|%s|%d", block.Held, len(block.Ready))
	}
	for _, record := range s.Launches {
		if record.State.Terminal() {
			add("launch|%s|%s", record.ID, record.State)
		}
	}
	for store, questions := range s.Questions {
		for _, question := range questions {
			add("question|%s|%s|%s", store, question.ID, question.State)
		}
	}
	for _, design := range s.Designs {
		add("design|%s|%s|%d", design.Record.ID, design.Record.Status, newestRound(design))
	}
	for _, grant := range s.Grants {
		add("grant|%s", grant.ID)
	}
	if len(s.Rulings) > 0 {
		add("ruling|%s", s.Rulings[len(s.Rulings)-1].ID)
	}
	if len(s.Answered) > 0 {
		add("answered|%x", sha256.Sum256([]byte(s.Answered[len(s.Answered)-1])))
	}
	for role, execution := range s.Executions {
		add("engine|%s|%d", role, execution.Generation)
	}
	for _, holder := range s.Holders {
		add("seat|%s|%s", holder.Checkout, holder.MainId)
	}
	for store, episodes := range s.Alerts {
		for _, episode := range episodes {
			add("alert|%s|%s|%t", store, episode.EpisodeID, episode.Cleared)
		}
	}
	for part := range s.Unavailable {
		add("unavailable|%s", part)
	}
	slices.Sort(lines)
	slices.SortFunc(deadlines, time.Time.Compare)
	return lines, deadlines
}

// newestRound is a design's newest review round; zero is none.
func newestRound(design SituationDesign) (round int64) {
	for _, review := range design.Reviews {
		round = max(round, review.NewestRound)
	}
	return round
}
