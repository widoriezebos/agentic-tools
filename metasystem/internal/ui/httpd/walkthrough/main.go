// Command walkthrough serves the committed interface bundle over a canned
// backlog so the board can be driven in a real browser without a ledger, a
// terminal enrolment, or the checkout's own state.
//
// It exists for the walkthrough and nothing else: it is not wired into the
// engine, it publishes nothing, and its two acts are recorded rather than
// performed. `-proven` chooses which server the board is talking to — one
// that can act as the human, or one an agent started.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/web"
)

const agentReason = "the interface was started by an agent process (claude-code); start it from your own terminal with bin/metasystem ui restart to act as yourself"

func main() {
	listen := flag.String("listen", "127.0.0.1:7979", "loopback address")
	proven := flag.Bool("proven", false, "serve as a server that proved its human at boot")
	flag.Parse()

	manifest, err := web.ReadManifest()
	if err != nil {
		log.Fatalf("this executable carries no bundle: %v", err)
	}
	state := newLedger()
	authority := httpd.AuthorityInfo{Reason: agentReason}
	if *proven {
		authority = httpd.AuthorityInfo{Proven: true, Human: "Wido"}
	}
	info := httpd.Info{
		Checkout: "/walkthrough", StartedAt: time.Now().UTC().Format(time.RFC3339),
		EngineBuild: "walkthrough", BundleDigest: manifest.SourceDigest,
		Observe:   state.observe,
		Authority: authority,
		Approve: func(id string, budget goalbudget.Budget) error {
			return state.approve(id, budget)
		},
		Withdraw: func(id, reason string) error { return state.withdraw(id, reason) },
		BudgetDefaults: func() (map[string]goalbudget.Budget, error) {
			return map[string]goalbudget.Budget{"3": {
				ElapsedLimit: "8h", AttemptLimit: 10, ReservedJobMinutesLimit: 1200, ActiveJobLimit: 1, ReviewRoundLimit: 3,
			}}, nil
		},
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("ready http://" + listener.Addr().String())
	server := &http.Server{Handler: httpd.New(info, listener.Addr(), web.Dist()), ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(server.Serve(listener))
}

// ledger is the canned tree the board reads, and the two acts change it the
// way the engine would: an approval moves a goal to approved, a withdrawal
// returns it to queued. Everything else refuses.
type ledger struct{ tree *goal.TreeGoals }

func newLedger() *ledger {
	tree := &goal.TreeGoals{
		Root:      &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "2", SyncMode: goal.SyncLocal, Revision: 1},
		Live:      map[string]*goal.GoalFile{},
		Done:      map[string]*goal.GoalFile{},
		Abandoned: map[string]*goal.GoalFile{},
	}
	tree.Live["g1-s12"] = walkthroughGoal("g1-s12", goal.StateQueued, "The board, and approve and withdraw by drag")
	tree.Live["g1-s13"] = walkthroughGoal("g1-s13", goal.StateQueued, "The goal page reads the whole record")
	ready := walkthroughGoal("g1-s14", goal.StateApproved, "The Fleet section reads the seats")
	ready.Approved = &goal.ApprovalRecord{By: "human:Wido", At: "2026-09-20T09:00:00Z", Authority: goal.ApprovalAuthorityProven, Revision: 3}
	ready.Budget = &goalbudget.Budget{ElapsedLimit: "4h", AttemptLimit: 6, ReservedJobMinutesLimit: 720, ActiveJobLimit: 1, ReviewRoundLimit: 2}
	tree.Live["g1-s14"] = ready
	claimed := walkthroughGoal("g1-s15", goal.StateClaimed, "The Decisions section answers a question")
	claimed.Claimed = &goal.ClaimRecord{Machine: "m1e", Lineage: "coordinator", At: "2026-09-21T08:00:00Z"}
	tree.Live["g1-s15"] = claimed
	return &ledger{tree: tree}
}

func walkthroughGoal(id, state, intent string) *goal.GoalFile {
	return &goal.GoalFile{
		Id: id, State: state, Intent: intent, Origin: "main", Tier: 3, Priority: 2, Sequence: 1,
		NextStep: "Start " + id + ".", OpenedAt: "2026-09-01T00:00:00Z", Revision: 3,
		History: []goal.HistoryLine{{At: "2026-09-01T00:00:00Z", Opid: "op-open", Verb: "open", Actor: "m1e+coordinator"}},
	}
}

func (l *ledger) observe() snapshot.Observation {
	now := time.Now().UTC()
	horizon := goal.NewApprovalHorizon(l.tree, now)
	return snapshot.Observation{
		ObservedAt: now, StateRoot: "/walkthrough", State: snapshot.StateRead,
		Tip: "c5d517f427e35e19c2944ab9aefee4d9c9cd9e6c", CommittedAt: now.Add(-time.Minute),
		Tree: l.tree, Horizon: horizon, SyncMode: goal.SyncLocal,
		// The claim gate cannot be asked about a tree with no repository
		// behind it, so the walkthrough answers it directly: every approved
		// goal is one a seat could claim.
		Admission: l.admission(),
		Fetch: snapshot.FetchState{
			Outcome: snapshot.OutcomeCurrent, StartedAt: now.Add(-2 * time.Second), FinishedAt: now.Add(-time.Second),
			Tip: "c5d517f427e35e19c2944ab9aefee4d9c9cd9e6c", Detail: "already at the canonical tip",
			Cadence: snapshot.CadenceConnected, NextAt: now.Add(5 * time.Second),
		},
	}
}

func (l *ledger) admission() backlog.Admission {
	admission := backlog.Admission{
		Answered: true,
		Ready:    map[string]bool{}, Blocked: map[string]bool{}, Awaiting: map[string]bool{},
		Refused: map[string]string{},
	}
	for id, file := range l.tree.Live {
		if file.State == goal.StateApproved {
			admission.Ready[id] = true
		}
	}
	return admission
}

func (l *ledger) approve(id string, budget goalbudget.Budget) error {
	file := l.tree.Live[id]
	if file == nil {
		return fmt.Errorf("goal %s is not live", id)
	}
	if file.State != goal.StateQueued {
		return fmt.Errorf("goal %s is %s; approve admits queued work", id, file.State)
	}
	file.State = goal.StateApproved
	file.Budget = &budget
	file.Approved = &goal.ApprovalRecord{
		By: "human:Wido", At: time.Now().UTC().Format(time.RFC3339),
		Authority: goal.ApprovalAuthorityProven, Revision: file.Revision,
	}
	return nil
}

func (l *ledger) withdraw(id, reason string) error {
	file := l.tree.Live[id]
	if file == nil || file.Approved == nil {
		return fmt.Errorf("goal %s carries no approval to withdraw", id)
	}
	file.State = goal.StateQueued
	file.Approved, file.Budget = nil, nil
	_ = reason
	return nil
}
