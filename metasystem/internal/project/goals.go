package project

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// The goal ledger, read the way the rest of the interface already reads it.
//
// Goals are the project's one subdivision: a record says which of them it is
// about, and a record that names none is about the project as a whole. The
// ledger owns them — this package only reads what it declares, and it reads it
// through the engine's own parser, so a goal file's grammar is stated once.

// The two directories the ledger keeps its goals in, beneath the state root.
const (
	liveGoalsDir      = "plans/goals"
	concludedGoalsDir = "records/goals"
)

// rootRecordName is the ledger's root record. It sits among the goal files and
// is not a goal, so it is not read as one.
const rootRecordName = "backlog.md"

// Where a goal was read from: the live ledger, or the records of what closed.
const (
	GoalLive      = "live"
	GoalConcluded = "concluded"
)

// Goal is one goal of the ledger as the project needs it: what it is called,
// where it stands, and why it is open. Everything else the ledger carries —
// the budget, the claim, the history — belongs to the ledger's own views.
type Goal struct {
	ID     string // the ledger's id, which is the file's own name
	Title  string // the goal file's heading, or the id where it has none
	State  string // queued | approved | claimed | parked | done | abandoned
	Intent string // the goal's own Intent line
	Where  string // GoalLive or GoalConcluded
	Path   string // checkout-relative
	// Sliced is the goal's own `- Sliced:` line, where it carries one.
	Sliced *Slicing
}

// Slicing is the irreversible pre-reservation boundary a goal file records:
// when slicing started, and which seat started it. Once it is present the goal
// can only advance through a split, which is why a reader of the plan wants to
// know whether it has happened and when.
type Slicing struct {
	At      string
	Machine string
	Lineage string
}

// LiveStates are the states a goal is still worked under, in the order the
// ledger lives them. A goal in none of them is concluded.
var LiveStates = []string{goal.StateQueued, goal.StateApproved, goal.StateClaimed, goal.StateParked}

// Live reports whether this goal is still worked under.
func (g Goal) Live() bool { return contains(LiveStates, g.State) }

// readGoals reads both directories once: the live goals, then the concluded
// ones, each in id order, so one listing is one order whatever the filesystem
// answers in.
//
// A goal file that cannot be read is not a goal of this listing and is not a
// refusal either: the ledger has its own validator, and a project check that
// failed over the ledger's hygiene would be refusing something it does not own.
func (p *Project) readGoals() {
	p.Goals = append(p.Goals, p.goalsIn(liveGoalsDir, GoalLive)...)
	p.Goals = append(p.Goals, p.goalsIn(concludedGoalsDir, GoalConcluded)...)
}

// goalsIn reads one of the two directories, from inside it and nowhere else:
// entries are listed and opened through the directory's own root, so a symlink
// whose target lies outside does not open.
func (p *Project) goalsIn(relative, where string) []Goal {
	directory := filepath.Join(p.Roots.StateRoot, filepath.FromSlash(relative))
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil
	}
	defer func() { _ = root.Close() }()
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil
	}
	goals := make([]Goal, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") || name == rootRecordName {
			continue
		}
		read, ok := readGoalFile(root, name)
		if !ok {
			continue
		}
		id := strings.TrimSuffix(name, ".md")
		title := read.Id
		if title == "" {
			title = id
		}
		one := Goal{
			ID:     id,
			Title:  title,
			State:  read.State,
			Intent: normalizeSpace(read.Intent),
			Where:  where,
			Path:   relative + "/" + name,
		}
		if sliced := read.Sliced; sliced != nil {
			one.Sliced = &Slicing{At: sliced.At, Machine: sliced.Machine, Lineage: sliced.Lineage}
		}
		goals = append(goals, one)
	}
	sort.SliceStable(goals, func(i, j int) bool { return goals[i].ID < goals[j].ID })
	return goals
}

// readGoalFile opens one goal file inside its own directory and parses it with
// the engine's parser. The parser's problems are the ledger's to answer for;
// what is taken here is only what the file declares about itself.
func readGoalFile(root *os.Root, name string) (*goal.GoalFile, bool) {
	file, err := root.Open(name)
	if err != nil {
		return nil, false
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, false
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, false
	}
	parsed, _ := goal.ParseFile(data)
	if parsed == nil {
		return nil, false
	}
	return parsed, true
}

// HasGoal reports whether the ledger carries this goal, live or concluded. It
// is what a Goals line is judged against, and the one place that judgement is
// made.
func (p *Project) HasGoal(id string) bool {
	for index := range p.Goals {
		if p.Goals[index].ID == id {
			return true
		}
	}
	return false
}
