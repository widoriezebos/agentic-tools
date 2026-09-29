package backlog

import (
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// Gate is the landing gate's reading of one goal waiting to land (g1-s70 §6),
// computed from the goal's history and the layered settings, never from the
// interface's private store; the card and the inbox say it, and nothing here
// runs a clock: AutoLandsAt is when the goal becomes eligible, and the holder
// lands it on its next turn after that.
type Gate struct {
	Tier          uint8  `json:"tier"`
	HumanFromTier uint8  `json:"humanFromTier"`
	AutoAfter     string `json:"autoAfter"`
	WaitsForHuman bool   `json:"waitsForHuman"`
	// ClockFrom and AutoLandsAt are the grace time's start and end below the
	// tier; empty at or above it and under a hold.
	ClockFrom   string `json:"clockFrom,omitempty"`
	AutoLandsAt string `json:"autoLandsAt,omitempty"`
	Eligible    bool   `json:"eligible"`
	// HeldBy is every standing review sitting, whose it is and on which record.
	HeldBy []GateHold `json:"heldBy,omitempty"`
	// Reviewed is the newest human word on the landing: a verdict or a
	// decision to land without a sitting.
	Reviewed *GateWord `json:"reviewed,omitempty"`
	// Landed says the holder recorded the landing's confirmed publication.
	Landed bool `json:"landed"`
}

// GateHold is one standing sitting.
type GateHold struct {
	By     string `json:"by"`
	Record string `json:"record"`
	Since  string `json:"since"`
}

// GateWord is the newest word: its kind (clear-to-land, send-back or
// land-without-sitting), whose it is and the tip it was given at.
type GateWord struct {
	Kind string `json:"kind"`
	By   string `json:"by"`
	Tip  string `json:"tip"`
	// Moved says the goal branch at origin is no longer at Tip, so a word to
	// land no longer lets the goal land: a moved tip needs the word again
	// (G5); BranchTip is where the branch is now. Both are empty where the
	// branch's tip was not read.
	Moved     bool   `json:"moved,omitempty"`
	BranchTip string `json:"branchTip,omitempty"`
}

// GateOf is one goal's reading at now.
func GateOf(f *goal.GoalFile, s goal.GateSettings, now time.Time) *Gate {
	read := goal.ReadGate(f, s, now)
	gate := &Gate{Tier: read.Tier, HumanFromTier: s.HumanFromTier, AutoAfter: s.AutoAfterText,
		WaitsForHuman: read.WaitsForHuman, Eligible: read.Eligible, Landed: read.Landed}
	if !read.ClockFrom.IsZero() {
		gate.ClockFrom = read.ClockFrom.UTC().Format(time.RFC3339)
		gate.AutoLandsAt = read.AutoLandsAt.UTC().Format(time.RFC3339)
	}
	for _, hold := range read.HeldBy {
		gate.HeldBy = append(gate.HeldBy, GateHold{By: hold.By, Record: hold.Record, Since: hold.At})
	}
	if read.Word != "" {
		gate.Reviewed = &GateWord{Kind: read.Word, By: read.WordBy, Tip: read.WordTip}
	}
	return gate
}

// JoinGates fills the gate's reading on every row in the Review lane, from the
// tree the rows were projected from; branchTip reads a goal branch's tip at
// origin, against which a word to land is read, and may be nil.
func JoinGates(rows []Row, tree *goal.TreeGoals, s goal.GateSettings, now time.Time, branchTip func(goalID string) (string, error)) {
	if tree == nil {
		return
	}
	for index := range rows {
		if rows[index].Lane != LaneReview {
			continue
		}
		if f := tree.Live[rows[index].ID]; f != nil {
			rows[index].Gate = GateOf(f, s, now)
			readAgainstTheBranch(rows[index].Gate.Reviewed, rows[index].ID, branchTip)
		}
	}
}

// readAgainstTheBranch says whether a word to land still stands at the goal
// branch's tip at origin: the engine refuses a landing whose word was given at
// another tip, so the card and the inbox ask for the word again (G5). A send-
// back lets nothing land and is not read; a tip that cannot be read says
// nothing either way.
func readAgainstTheBranch(word *GateWord, goalID string, branchTip func(string) (string, error)) {
	if word == nil || word.Kind == goal.VerdictSendBack || branchTip == nil {
		return
	}
	if tip, err := branchTip(goalID); err == nil && tip != "" && tip != word.Tip {
		word.Moved, word.BranchTip = true, tip
	}
}
