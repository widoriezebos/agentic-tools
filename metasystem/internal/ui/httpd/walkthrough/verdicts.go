package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/act"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/review"
)

// The verdict that does something, and the candidate (g1-s69), as this fixture
// serves them.
//
// The verdict is judged by the engine's own rules — the record resolved in its
// home, its Goals line, its Outcome's verdict and Reviewed at, create-only
// publication — through goal.ReviewAct, and its line is written onto the canned
// goal's history, so the board reads it exactly as it reads the ledger's. What
// the fixture does not have is a ledger to publish to: the publication is kept
// in memory beside the tree.
//
// The candidate is a run of this fixture's own: Run starts it at the goal
// branch's tip as the branches file names it at that moment, so a push while
// the room reviews its recorded tip is the moved case, and Stop stops it.
// -launch-contract=absent is a project with no launch contract to run by.

// holderAnswers is what the fixture's holding seat answers a send-back with, if
// anything: its step taken at once, so the lane card's two later readings can
// be stood in front of.
type holderAnswers string

const (
	holderNone      holderAnswers = ""
	holderAttempt   holderAnswers = "attempt"
	holderNeedsWork holderAnswers = "needs-work"
)

// verdicts is the fixture's publication beside the tree, and how its holder
// answers.
type verdicts struct {
	mu        sync.Mutex
	published map[string][]byte
	holder    holderAnswers
	serial    int
}

// review records one verdict on a canned goal waiting to land.
func (l *ledger) review(held *verdicts, id string, asked act.Reviewed) (act.Recorded, error) {
	held.mu.Lock()
	defer held.mu.Unlock()
	path, content, err := goal.ResolveReviewRecord(l.roots.StateRoot, filepath.Join(l.roots.Checkout, filepath.FromSlash(asked.Record)))
	if err != nil {
		return act.Recorded{}, &act.Refusal{Kind: act.KindRequest, Code: "record", Message: err.Error()}
	}
	f := l.tree.Live[id]
	if f == nil || f.State != goal.StateClaimed || f.Landing == nil {
		return act.Recorded{}, &act.Refusal{Kind: act.KindEngine, Code: "rejected", Message: "goal " + id + " is not waiting to land; a verdict is recorded on a goal in the Review lane"}
	}
	reviewing := goal.ReviewAct{Record: path, Content: content, Verdict: asked.Verdict, Brief: []byte(asked.Brief), Work: asked.Work}
	if asked.Verdict == goal.VerdictSendBack && strings.TrimSpace(asked.Brief) == "" && asked.Work != "" {
		reviewing.Brief = held.published[goal.BriefPathFor(path)]
	}
	line, err := reviewing.Line(id, "Wido")
	if err != nil {
		return act.Recorded{}, &act.Refusal{Kind: act.KindRequest, Code: "record", Message: err.Error()}
	}
	recorded := act.Recorded{Verdict: line.Verdict, Tip: line.Tip, Record: line.Record, By: line.By, Brief: line.Brief, Work: line.Work, Line: line.Reason()}
	for _, standing := range goal.ReviewLinesOf(f) {
		if standing.Reason() == line.Reason() {
			return recorded, nil
		}
	}
	publishing := map[string][]byte{path: content}
	if line.Brief != "" {
		publishing[line.Brief] = reviewing.Brief
	}
	for at, words := range publishing {
		if kept, present := held.published[at]; present && !bytes.Equal(kept, words) {
			return act.Recorded{}, &act.Refusal{Kind: act.KindEngine, Code: "rejected",
				Message: at + " is already published with other words, another review's; recorded words are never overwritten, so start a new review of " + id}
		}
	}
	for at, words := range publishing {
		held.published[at] = append([]byte(nil), words...)
	}
	held.serial++
	opid := fmt.Sprintf("01M3MPVERD1CT%013d-mac-ui-1a2b3c4d", held.serial)
	f.History = append(f.History, goal.HistoryLine{
		At: time.Now().UTC().Format(time.RFC3339), Opid: opid, Verb: "review", Actor: "human:Wido",
		Targets: []string{id}, Reason: line.Reason(), Keep: -1,
	})
	f.Revision++
	if line.Verdict == goal.VerdictSendBack {
		held.answer(f, opid, line.Work)
	}
	return recorded, nil
}

// answer is the fixture holder's step on a send-back, where it takes one.
func (held *verdicts) answer(f *goal.GoalFile, review, work string) {
	answer := ""
	switch {
	case held.holder == holderNeedsWork && work == "":
		answer = "send-back needs-work candidates=discovery,writer review=" + review
	case held.holder == holderAttempt, held.holder == holderNeedsWork:
		answer = "send-back attempt=3 review=" + review
		if work != "" {
			answer += " work=" + work
		}
	default:
		return
	}
	held.serial++
	f.History = append(f.History, goal.HistoryLine{
		At: time.Now().UTC().Format(time.RFC3339), Opid: fmt.Sprintf("01M3MPANSWR%015d-%s-1a2b3c4d", held.serial, f.Claimed.Machine),
		Verb: "send-back", Actor: f.Claimed.Machine + "+" + f.Claimed.Lineage, Targets: []string{f.Id}, Reason: answer, Keep: -1,
	})
	f.Revision++
}

// candidates are the fixture's candidate runs, by goal.
type candidates struct {
	mu         sync.Mutex
	runs       map[string]httpd.Candidate
	git        fixtureGit
	noContract bool
}

func (c *candidates) act(goalID, action string) (httpd.Candidate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.noContract {
		return httpd.Candidate{}, &httpd.CandidateRefusal{Code: httpd.CodeNoContract,
			Message: "this goal's candidate cannot run from here: this project has no launch contract: write launch.json and name it with launch.contract=launch.json in metasystem.conf"}
	}
	stopped := httpd.Candidate{Goal: goalID, State: "stopped", Readiness: "no-probe", Said: "no application run is recorded for goal-" + goalID}
	switch action {
	case "status":
		if run, running := c.runs[goalID]; running {
			return run, nil
		}
		return stopped, nil
	case "start":
		if run, running := c.runs[goalID]; running {
			return run, nil
		}
		tip, err := c.git.ResolveCommit("origin/" + review.Branch(goalID))
		if err != nil {
			return httpd.Candidate{}, &httpd.CandidateRefusal{Code: "refused", Message: "goal " + goalID + " has no branch to run: " + err.Error()}
		}
		run := httpd.Candidate{Goal: goalID, State: "running", Readiness: "answering", Address: "127.0.0.1:7981", Commit: tip,
			Since: time.Now().UTC().Format(time.RFC3339), Said: "the application is running and answering"}
		c.runs[goalID] = run
		return run, nil
	case "stop":
		delete(c.runs, goalID)
		return stopped, nil
	}
	return httpd.Candidate{}, &httpd.CandidateRefusal{Code: "action", Message: action + " is not read, start or stop"}
}
