package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/act"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
)

// The landing gate in the walkthrough (g1-s70 §6): the two settings resolved
// by the engine's own layered reader over a fixture metasystem.conf and its
// .local beside it, so the Settings page's sources are real; three Review
// goals below the tier — one on its clock, one past its grace time, one held
// by a sitting — beside the two at tier 3 that wait for a person; and the
// room's hold and the Decide sheet's decision written onto the canned goals'
// history by the engine's own grammar.

// plantLandingGate writes the fixture's two configuration files: the grace
// time committed, the threshold overridden in .local, which is the case the
// layered read exists for.
func plantLandingGate(checkout string) (string, error) {
	conf := filepath.Join(checkout, "metasystem.conf")
	if err := os.WriteFile(conf, []byte(config.LandingAutoAfterKey+" = 4h\n"), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(conf+".local", []byte(config.LandingHumanFromTierKey+" = 2\n"), 0o644); err != nil {
		return "", err
	}
	return conf, nil
}

// landingGoals adds the three below-tier goals to the canned tree.
func landingGoals(add func(*goal.GoalFile) *goal.GoalFile) {
	waiting := func(id, intent, machine string, landed time.Duration) *goal.GoalFile {
		file := add(ranked(walkthroughGoal(id, goal.StateClaimed, intent), 2, 16))
		file.Tier = 1
		file.Claimed = &goal.ClaimRecord{Machine: machine, Lineage: "coordinator", At: stampedAgo(landed + time.Hour)}
		opid := "op-land-" + id
		file.Landing = &goal.LandingRecord{At: stampedAgo(landed), Opid: opid}
		file.History = append(file.History, goal.HistoryLine{At: stampedAgo(landed), Opid: opid,
			Verb: "land-ready", Actor: machine + "+coordinator", Targets: []string{id}, Keep: -1})
		return file
	}
	waiting("g1-s91", "The card counts the grace time down", "m1e", 48*time.Minute)
	waiting("g1-s92", "A goal past its grace time waits for its holder", "m2a", 5*time.Hour)
	held := waiting("g1-s93", "A standing sitting holds a small goal too", "m1e", 2*time.Hour)
	held.History = append(held.History, goal.HistoryLine{At: stampedAgo(30 * time.Minute), Opid: "op-hold-g1-s93",
		Verb: "review", Actor: "human:Wido", Targets: []string{"g1-s93"}, Keep: -1,
		Reason: goal.SittingReason(true, "plans/reviews/review-of-g1-s93.md", "Wido")})
}

// gateActs writes the gate's two human acts onto the canned goals, as the
// engine's own lines, so the board reads them back exactly as it reads a real
// ledger's.
type gateActs struct {
	mu     sync.Mutex
	l      *ledger
	serial int
}

func (g *gateActs) line(id, verb, reason string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	f := g.l.tree.Live[id]
	if f == nil || f.State != goal.StateClaimed {
		return &act.Refusal{Kind: act.KindEngine, Code: "rejected", Message: "goal " + id + " is not claimed; the landing gate's acts are on a goal whose work waits to land"}
	}
	g.serial++
	f.History = append(f.History, goal.HistoryLine{At: time.Now().UTC().Format(time.RFC3339),
		Opid: fmt.Sprintf("01M3MPGATE%016d-mac-ui-1a2b3c4d", g.serial), Verb: verb, Actor: "human:Wido",
		Targets: []string{id}, Keep: -1, Reason: reason})
	f.Revision++
	return nil
}

func (g *gateActs) sitting(_ *session.Session, id, record string, open bool) error {
	path, err := goal.ReviewRecordPath(g.l.roots.StateRoot.Path(), filepath.Join(g.l.roots.Checkout, filepath.FromSlash(record)))
	if err != nil {
		return &act.Refusal{Kind: act.KindRequest, Code: "record", Message: err.Error()}
	}
	holds := goal.HoldsOf(g.l.tree.Live[id])
	for _, hold := range holds {
		if hold.By == "Wido" && hold.Record == path && open {
			return nil
		}
	}
	if !open && len(holds) == 0 {
		return nil
	}
	return g.line(id, "review", goal.SittingReason(open, path, "Wido"))
}

func (g *gateActs) landWithoutSitting(_ *session.Session, id, tip, reason string) error {
	decided, err := goal.WithoutSittingLine(tip, "Wido", reason)
	if err != nil {
		return &act.Refusal{Kind: act.KindRequest, Code: "reason", Message: err.Error()}
	}
	return g.line(id, goal.LandWithoutSittingVerb, "landed-without-sitting tip="+decided.Tip+" by=Wido because="+decided.Reason)
}
