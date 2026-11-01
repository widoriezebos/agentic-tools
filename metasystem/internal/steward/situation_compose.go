package steward

// The composition is the page one situation makes: the first thing to do,
// each part's lines, and the wake fields a wait compares. It reads nothing:
// every input is in the situation and every time comes from its clock.

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// Composition is one situation's page. Wake are its semantic facts, one per
// line and sorted, Digest their hash, and Deadlines the clock times after
// ReadAt at which a wake field changes with no file change.
type Composition struct {
	ReadAt    time.Time
	Machine   string
	First     FirstThing
	Parts     []SituationPart
	Wake      []string
	Digest    string
	Deadlines []time.Time
}

// FirstThing is what this seat does first. Kind is ask, red, continue,
// claim or wait, or unknown when a part the rule needs could not be read.
type FirstThing struct{ Kind, Subject, Why, Command string }

// SituationPart is one part's lines: Summary fits a screen, Detail holds
// every item.
type SituationPart struct {
	Name            string
	Summary, Detail []string
	Unavailable     string
}

// Compose makes the page from one situation.
func Compose(s Situation) Composition {
	c := Composition{ReadAt: s.At, Machine: s.Machine, First: s.firstThing()}
	c.Wake, c.Deadlines = s.wake()
	sum := sha256.Sum256([]byte(strings.Join(c.Wake, "\n")))
	c.Digest = hex.EncodeToString(sum[:])
	c.Parts = s.parts(c.First)
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
	if h := s.Lane.Health; h != nil {
		for _, row := range h.Rows() {
			add("row|%s|%s", row.Row, row.Subject)
		}
		// A row that fires on its threshold changes with no file change: its
		// deadline is the first second past the threshold, from the minutes
		// the lane read measured at this clock.
		cross := func(minutes *float64, threshold float64) {
			if minutes != nil {
				later(s.At.Add(time.Duration((threshold-*minutes)*float64(time.Minute)) + time.Second))
			}
		}
		if h.Paused != nil {
			cross(h.Paused.Minutes, 30)
		}
		for _, question := range h.Questions {
			cross(&question.Minutes, 30)
		}
		if h.Agent.Alive && h.Agent.WaitingOn == "" {
			cross(h.Agent.Minutes, 20)
			cross(h.Agent.Minutes, 80)
		}
		// Only a waiting or proving hand-in's minutes run on with the clock.
		for _, g := range h.Goals {
			if g.State == plain.StateWaiting || g.State == "proving" {
				cross(g.Minutes, 60)
				cross(g.Minutes, 120)
			}
		}
		// A person's act fires at five minutes, a proof's drift at 1.5 medians.
		if h.PendingAct != nil {
			cross(h.PendingAct.Minutes, 5)
		}
		if p := h.Proofs; p.Median != nil && !p.Drift {
			cross(p.Running, 1.5**p.Median)
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

// parts are the page's parts in order, each with the reasons its readers
// failed. A summary counts; the detail names every item.
func (s Situation) parts(first FirstThing) []SituationPart {
	env, live := s.env(), map[string]*goal.GoalFile{}
	several := func(n int, one, many string) string {
		return fmt.Sprint(n, " ", map[bool]string{true: one, false: many}[n == 1])
	}
	if s.Ledger.Tree != nil {
		live = s.Ledger.Tree.Live
	}
	var parts []SituationPart
	// An unreadable engine reader leaves only the seat holders unread; the
	// executions are read before them.
	labels := map[string]string{"engine": "seat holders"}
	add := func(name string, readers []string, summary, detail []string) {
		var reasons []string
		for _, reader := range readers {
			if reason := s.Unavailable[reader]; reason != "" {
				reasons = append(reasons, cmp.Or(labels[reader], reader)+": "+reason)
			}
		}
		parts = append(parts, SituationPart{Name: name, Summary: summary, Detail: detail, Unavailable: strings.Join(reasons, "; ")})
	}
	firstLines := []string{strings.TrimSpace(first.Kind+" "+first.Subject) + ": " + first.Why}
	if first.Kind == "unknown" {
		firstLines[0] = first.Why
	}
	if first.Command != "" {
		firstLines = append(firstLines, "next: "+first.Command)
	}
	add("first", nil, firstLines, nil)

	lane, rows := []string{"the lane is " + cmp.Or(s.Lane.Owner.State, "unread")}, s.Lane.Problems
	if proof := s.Lane.RunningProof; proof != nil {
		lane = append(lane, "proof "+proof.Attempt+" is "+proof.State)
	}
	hold := "hold: none"
	if s.Hold != nil {
		hold = "hold: " + s.Hold.Subject + ", " + s.Hold.Outcome + ", " + env.Until(s.Hold.Until)
	}
	lane = append(lane, hold)
	if s.Lane.Health != nil {
		for _, row := range s.Lane.Health.Rows() {
			rows = append(rows, fmt.Sprintf("%s on %s: %s", row.Row, row.Subject, row.Act))
		}
	}
	add("lane", []string{"lane", "hold"}, lane, rows)
	add("ledger", []string{"ledger", "waiting"}, []string{fmt.Sprintf("claimed %s; landing %s; fenced %s; %d ready, %d blocked",
		names(s.Next.Claimed), names(s.Next.Landing), names(s.Next.Fenced), len(s.Next.Ready), len(s.Next.Blocked))}, slices.Concat(s.ClaimLines, s.Waiting))

	var program, order, goals, facts []string
	for _, block := range s.Quota {
		waits := fmt.Sprintf("%d ready goals wait", len(block.Ready))
		if len(block.Ready) == 1 {
			waits = "1 ready goal waits"
		}
		program = append(program, fmt.Sprintf("%s behind this seat's claim on %s; next: %s", waits, block.Held, block.Command))
	}
	for index, id := range s.Order {
		if file := live[id]; file != nil {
			order = append(order, strings.TrimSuffix(fmt.Sprintf("%d. %s, priority %d, pinned %s", index+1, id, file.Priority, file.Pinned), ", pinned "))
		}
	}
	add("program", []string{"quota", "ledger"}, append(program, fmt.Sprintf("%d open goals in the queue", len(s.Order))), order)
	for _, id := range s.seatGoals() {
		if file := live[id]; file != nil {
			goals = append(goals, id+": "+file.NextStep)
		}
		if text := strings.TrimSpace(s.GoalFacts[id]); text != "" {
			facts = append(facts, strings.Split(text, "\n")...)
		}
	}
	if len(goals) == 0 {
		goals = []string{"this seat holds no goal"}
	}
	add("goal", []string{"facts"}, goals, facts)

	asked, asks := map[string]int{}, []string(nil)
	for _, ask := range s.Asks {
		if asked[ask.State]++; ask.Thread.Root != nil {
			asks = append(asks, ask.Thread.ID+" "+ask.State+": "+s.askLine(ask))
		}
	}
	add("asks", []string{"asks"}, []string{several(len(s.Asks), "ask", "asks") + ": " + counts(asked)}, asks)
	launched, launches := map[string]int{}, []string(nil)
	for _, record := range s.Launches {
		launched[string(record.State)]++
		line := fmt.Sprintf("%s %s for %s: %s", record.ID, record.Kind, cmp.Or(record.Goal, "no goal"), record.State)
		for _, output := range record.Outputs {
			line += ", " + output.Path
		}
		launches = append(launches, line)
	}
	add("launches", []string{"launches"}, []string{several(len(s.Launches), "launch", "launches") + ": " + counts(launched)}, launches)
	statuses, designs := map[string]int{}, []string(nil)
	for _, design := range s.Designs {
		statuses[design.Record.Status]++
		designs = append(designs, fmt.Sprintf("%s: %s, revision %s, review round %d", design.Record.Path, design.Record.Status, cmp.Or(design.Revision, "none"), newestRound(design)))
	}
	add("designs", []string{"designs"}, []string{several(len(s.Designs), "design", "designs") + ": " + counts(statuses)}, designs)
	states, questions := map[string]int{}, []string(nil)
	for _, store := range slices.Sorted(maps.Keys(s.Questions)) {
		for _, question := range s.Questions[store] {
			states[question.State]++
			questions = append(questions, fmt.Sprintf("%s %s %s: %s", store, question.ID, question.State, cmp.Or(question.Goal, question.About)))
		}
	}
	add("questions", []string{"questions"}, []string{several(len(questions), "question", "questions") + ": " + counts(states)}, questions)

	byCheckout, processes := map[string]int{}, []string(nil)
	for _, process := range s.Processes {
		byCheckout[process.Checkout]++
		processes = append(processes, fmt.Sprintf("pid %d %s in %s, started %s", process.Pid, process.Kind, strings.TrimSpace(process.Checkout+" "+process.Workspace), env.Ago(process.Started)))
	}
	load := "load unread"
	if s.Load.Available {
		load = fmt.Sprintf("load %.2f", s.Load.Load1m)
	}
	add("host", []string{"host"}, []string{fmt.Sprintf("%s; %d test processes: %s", load, len(s.Processes), counts(byCheckout))}, processes)
	var roles, holders []string
	for _, role := range slices.Sorted(maps.Keys(s.Executions)) {
		roles = append(roles, fmt.Sprintf("%s generation %d", role, s.Executions[role].Generation))
	}
	for _, holder := range s.Holders {
		holders = append(holders, strings.TrimSuffix(fmt.Sprintf("%s: main %s, lineage %s, pid %d %s", holder.Checkout, cmp.Or(holder.MainId, "none"), holder.Lineage, holder.Pid, holder.Unavailable), " "))
	}
	add("engine", []string{"engine"}, []string{names(roles), fmt.Sprintf("%d seats", len(s.Holders))}, holders)
	authority, newest := []string(nil), "none"
	for _, grant := range s.Grants {
		authority = append(authority, "grant "+grant.ID)
	}
	for _, ruling := range s.Rulings {
		authority, newest = append(authority, ruling.ID+": "+ruling.Words), ruling.ID
	}
	add("authority", []string{"authority"}, []string{fmt.Sprintf("%d live grants; newest ruling %s; %d answered lines", len(s.Grants), newest, len(s.Answered))},
		append(authority, s.Answered...))
	var alerts []string
	for _, store := range slices.Sorted(maps.Keys(s.Alerts)) {
		for _, episode := range s.Alerts[store] {
			alerts = append(alerts, store+" "+episode.EpisodeID+": "+episode.Message)
		}
	}
	add("alerts", []string{"alerts"}, []string{several(len(alerts), "open alert", "open alerts")}, alerts)
	return parts
}

// names are ids joined, or none.
func names(ids []string) string { return cmp.Or(strings.Join(ids, ", "), "none") }

// counts are a tally's keys with their counts, sorted, or none.
func counts(tally map[string]int) string {
	var words []string
	for _, key := range slices.Sorted(maps.Keys(tally)) {
		words = append(words, fmt.Sprintf("%d %s", tally[key], key))
	}
	return names(words)
}
