package main

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stoptransition"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// statusBoard is what status reads besides the checkout's processes: the
// host board, the landing lane and the peer message counts, read once; the
// status view draws them.
type statusBoard struct {
	now   time.Time
	view  board.View
	lane  *lane.View
	peers []string
}

func (inv *intentInvocation) readStatusBoard() statusBoard {
	now := inv.boardNow()
	return statusBoard{now: now, view: inv.hostBoardView(now), lane: inv.statusLane(),
		peers: inv.peerStatusLines(inv.layout.GitRoot)}
}

// statusSeatName is the checkout as a person names it: its machine
// nickname, or its directory's name.
func (inv *intentInvocation) statusSeatName(checkout string) string {
	if name, err := inv.owners.helm.withDefaults().machine(checkout); err == nil && strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return filepath.Base(checkout)
}

// statusView is status's page (output-style §6.1, 6.2): the headline says
// whether the checkout runs, with its helpers, jobs and changes waiting to
// land; then what needs a person, the work underway, the seats of this
// host, the landing lane and peer messages. --verbose adds the checkout's
// path, each helper, the lane's path and owner, and the processes that are
// not ours. Every line an owner wrote that the view has no typed place for
// is printed as it was.
func (inv *intentInvocation) statusView(checkout string, report stoptransition.Report, reading statusBoard) func(*textui.Page) {
	name := inv.statusSeatName(checkout)
	return func(page *textui.Page) {
		env := page.Env()
		var helpers, work, others []stoptransition.StatusItem
		used := map[int]bool{}
		if len(report.Lines) > 0 && strings.HasPrefix(report.Lines[0], "checkout ") {
			used[0] = true
		}
		for _, item := range report.Items {
			for index, line := range report.Lines {
				if !used[index] && strings.HasPrefix(line, item.Line) {
					item.Line, used[index] = line, true
					break
				}
			}
			switch item.Family {
			case "untracked":
				others = append(others, item)
			case "steward", "supervision":
				helpers = append(helpers, item)
			default:
				work = append(work, item)
			}
		}
		var notes []string
		for index, line := range report.Lines {
			if !used[index] && line != "nothing is running" {
				notes = append(notes, line)
			}
		}

		state := name + " has nothing running"
		switch {
		case report.ExitCode != 0:
			state = name + "'s status is incomplete"
		case report.FenceState == stopfence.StateClosed:
			state = name + " is stopped"
		case len(helpers) > 0:
			state = name + " is running"
		}
		helperFact := ""
		if len(helpers) > 0 {
			helperFact = textui.Count(len(helpers), "helper", "helpers")
		}
		page.Headline(state, helperFact, textui.Count(len(work), "job", "jobs"))
		if page.Verbose() {
			page.Facts(textui.KV{Key: "checkout", Value: []textui.Span{textui.Plain(env.Path(checkout))}})
		}
		if len(notes) > 0 {
			section := page.Section("", "")
			for _, note := range notes {
				section.Text(note)
			}
		}
		if len(work) > 0 {
			section := page.Section("Work", "")
			for _, item := range work {
				section.Item(textui.Running, strings.TrimSuffix(item.Line, ": running"))
			}
		}
		if page.Verbose() && len(helpers) > 0 {
			aside := ""
			if first := earliestStart(helpers); first > 0 {
				aside = env.Since(time.Unix(first, 0))
			}
			table := page.Section("Machinery", aside).Table(textui.Column{}, textui.Column{Flex: true})
			for _, item := range helpers {
				table.Row(textui.Marked(textui.Running, helperName(item.Component)), textui.Dim("pid "+strconv.FormatInt(item.Pid, 10)))
			}
		}
		reading.drawSeats(page, env)
		reading.drawLane(page, env)
		if len(reading.peers) > 0 {
			section := page.Section("Messages", "")
			for _, line := range reading.peers {
				section.Text(line)
			}
		}
		if page.Verbose() && len(others) > 0 {
			table := page.Section("Not ours, left alone", "").Table(textui.Column{Right: true}, textui.Column{}, textui.Column{Flex: true})
			for _, item := range others {
				runtime, argv := untrackedCommand(item.Line)
				table.Row(textui.Plain(strconv.FormatInt(item.Pid, 10)), textui.Plain(runtime), textui.Plain(argv))
			}
		}
	}
}

// helperName is a helper as a person knows it.
func helperName(component string) string {
	if name, ok := machineComponentNames[component]; ok {
		return name
	}
	return strings.ReplaceAll(component, "-", " ")
}

func earliestStart(items []stoptransition.StatusItem) int64 {
	var first int64
	for _, item := range items {
		if item.PidStartedAt > 0 && (first == 0 || item.PidStartedAt < first) {
			first = item.PidStartedAt
		}
	}
	return first
}

// untrackedCommand is a process that is not ours as its line names it:
// untracked pid N RUNTIME ARGV: running.
func untrackedCommand(line string) (string, string) {
	fields := strings.Fields(strings.TrimSuffix(line, ": running"))
	if len(fields) < 4 {
		return "", strings.Join(fields, " ")
	}
	return fields[3], strings.Join(fields[4:], " ")
}

// drawSeats is the host board: one row per armed seat with what it works
// on; --verbose one more per goal.
func (b statusBoard) drawSeats(page *textui.Page, env textui.Env) {
	aside := ""
	if b.view.Bridge != "" {
		aside = "bridge " + b.view.Bridge
	}
	switch {
	case !b.view.Readable:
		text := "the board is unreadable"
		if b.view.Reason != "" {
			text += ": " + b.view.Reason
		}
		page.Section("Seats on this host", aside).Item(textui.Unknown, text)
		return
	case len(b.view.Seats) == 0:
		page.Section("Seats on this host", aside).Text("no armed seat on this host")
		return
	}
	table := page.Section("Seats on this host", aside).Table(textui.Column{}, textui.Column{Flex: true, Wrap: true})
	for _, seat := range b.view.Seats {
		table.Row(textui.Marked(seatState(seat), seat.Machine), textui.Plain(seat.Text(b.now, env.Zone)))
		if !page.Verbose() {
			continue
		}
		// The seat's row already names its underway and unknown goals;
		// --verbose adds the ones it only counts.
		for _, entry := range seat.Goals {
			if entry.Unknown == "" && (entry.Stage == board.StageClaimedIdle || entry.Stage.Terminal()) {
				table.Row(textui.Plain(""), textui.Dim(entry.Text(b.now, env.Zone)))
			}
		}
	}
}

// seatState is unknown when the seat or one of its goals cannot be read,
// running while a goal is underway, idle otherwise.
func seatState(seat board.SeatView) textui.State {
	if seat.Unknown != "" {
		return textui.Unknown
	}
	state := textui.Idle
	for _, entry := range seat.Goals {
		switch {
		case entry.Unknown != "":
			return textui.Unknown
		case entry.Stage != board.StageClaimedIdle && !entry.Stage.Terminal():
			state = textui.Running
		}
	}
	return state
}

// drawLane is the landing lane: its owner when it does not run (or always
// with --verbose).
func (b statusBoard) drawLane(page *textui.Page, env textui.Env) {
	if b.lane == nil {
		return
	}
	aside := ""
	if b.lane != nil && b.lane.Root != nil && page.Verbose() {
		aside = env.Path(*b.lane.Root)
	}
	table := page.Section("Landing lane", aside).Table(textui.Column{}, textui.Column{Flex: true, Wrap: true})
	{
		owner := b.lane.Owner
		switch {
		case (owner.State != lane.OwnerRunning && owner.State != lane.OwnerIdle) || strings.Contains(b.lane.Summary, "unreadable"):
			table.Row(textui.Marked(textui.Alert, "owner"), textui.Plain(b.lane.Summary))
		case owner.State == lane.OwnerIdle:
			if page.Verbose() {
				table.Row(textui.Marked(textui.Idle, "owner"), textui.Plain("idle; its agent starts when there is work"))
			}
		case page.Verbose():
			detail := "running"
			if owner.PID != nil {
				detail += ", pid " + strconv.FormatInt(*owner.PID, 10)
			}
			table.Row(textui.Marked(textui.Running, "owner"), textui.Plain(detail))
		}
	}
}
