package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// The loop from the room (g1-s66), as this fixture serves it: a design with an
// Outcome, a canned critique round of two material findings, the three routes
// answered in memory in the engine's own words, and a Partner that drafts the
// section a Fold asks for.
//
// What the fixture does not have is the engine: no critic runs and no register
// is folded. Send to critique starts round 1 reading, and the next read of the
// page finds it back. Answer the round does what the verb does with the
// design's digest: a design nothing changed since the round was sent closes,
// with the Dispositions section appended to the file; a design a fold changed
// gets round 2, the failsafe round. The rows are judged by the rules the
// engine's decisions file is held to, so a refusal on the page is one the
// engine would give.

const (
	// loopRecord is the design the loop is walked on.
	loopRecord = "plans/designs/loop.md"
	loopChain  = "fixture-rev1"
	// loopGoal is the approved goal the canned tree carries, so the Send to
	// critique sheet preselects it as the design's one approved goal.
	loopGoal = "g1-s14"
	// loopFold is the phrase of Fold's request the drafted section is narrowed
	// to, and loopSection the heading it drafts.
	loopFold    = "Fold finding L1"
	loopSection = "4. Decisions"
)

const loopText = `# The loop from the room

- Kind: design
- Id: design-loop
- Status: draft
- Goals: g1-s14

## 1. What exists

A design sitting ends in a record, and what happens to the design afterwards
happens at a terminal.

## 3. The room

On the design page, beside the status, stands Send to critique.

## 4. Decisions

- D1. The design verbs as browser acts, under the human's sign-in.
- D2. Findings as cards, from the round's return.

## 5. Step 1, the smallest thing that works

D1 and D2, each with its tests.

## Outcome

A design goes to critique with one press, comes back as cards, is folded and answered from the page, and becomes a goal with one press.

The rest is later.
`

// loopDrafted is the section the fake Partner drafts for Fold.
const loopDrafted = `## 4. Decisions

- D1. The design verbs as browser acts, under the human's sign-in, with the funding goal and the reader budget the engine requires.
- D2. Findings as cards, from the round's return.`

var loopFindings = []httpd.DesignFinding{
	{ID: "L1", Severity: "high", Material: true,
		Claim:    "4. Decisions: the act as written cannot start a review, because the engine refuses a review without a reader budget",
		Evidence: "metasystem/cmd/metasystem/intent_delivery.go:771, plans/designs/loop.md:17",
		Change:   "carry the funding goal and the reader budget in D1"},
	{ID: "L2", Severity: "medium", Material: true,
		Claim:    "3. The room: the page computes the exit line itself",
		Evidence: "plans/designs/loop.md:13"},
}

// loopFixture is the critique's state, in memory beside the fixture checkout.
type loopFixture struct {
	mu       sync.Mutex
	checkout string
	// round is 0 before Send, and the examination's number after it.
	round   int64
	reading bool
	// reads is how many times the page has read the chain since a round was
	// sent: the round is back on the second.
	reads  int
	closed bool
	// sent is the design's digest when the newest round was sent.
	sent string
	rows map[int64][]httpd.DesignRow
}

func newLoopFixture(checkout string) *loopFixture {
	return &loopFixture{checkout: checkout, rows: map[int64][]httpd.DesignRow{}}
}

func (f *loopFixture) digest() string {
	data, _ := os.ReadFile(filepath.Join(f.checkout, filepath.FromSlash(loopRecord)))
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func (f *loopFixture) read(design string) (httpd.DesignLoop, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loop(design, true)
}

func (f *loopFixture) loop(design string, reading bool) (httpd.DesignLoop, error) {
	loop := httpd.DesignLoop{Design: design, ToolCalls: 30, Rounds: []httpd.DesignRound{}, State: "none", Limit: 2}
	if design != loopRecord || f.round == 0 {
		return loop, nil
	}
	if reading && f.reading {
		f.reads++
		if f.reads > 1 {
			f.reading = false
		}
	}
	loop.Chain, loop.Goal, loop.Round, loop.Critic = loopChain, loopGoal, f.round, "gpt-6-astra"
	for round := int64(1); round <= f.round; round++ {
		if round == f.round && f.reading {
			break
		}
		findings := loopFindings
		if round == 2 {
			findings = []httpd.DesignFinding{}
		}
		rows := append([]httpd.DesignRow{}, f.rows[round]...)
		loop.Rounds = append(loop.Rounds, httpd.DesignRound{Round: round, Findings: findings, Decisions: rows,
			Answerable: round == f.round && len(rows) == len(findings)})
	}
	switch {
	case f.closed:
		loop.State, loop.Status = "closed", "completed"
	case f.reading:
		loop.State, loop.Status = "reading", "running"
	case loop.Rounds[len(loop.Rounds)-1].Answerable:
		loop.State, loop.Status = "answered", "completed"
	default:
		loop.State, loop.Status = "deciding", "completed"
	}
	return loop, nil
}

// review is Send to critique and Answer the round, in the verb's words.
func (f *loopFixture) review(design string, asked httpd.DesignAsked) (httpd.DesignAnswer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if design != loopRecord {
		return httpd.DesignAnswer{Outcome: "refused", Summary: design + " is not a design this fixture critiques"}, nil
	}
	if asked.ToolCalls < 1 {
		return httpd.DesignAnswer{Outcome: "refused", Summary: "a review brief states the reader's tool-call budget, and none is configured",
			Decision: "name it with --tool-calls N"}, nil
	}
	if asked.After == 0 {
		if f.round > 0 {
			return httpd.DesignAnswer{Outcome: "in-progress", Chain: loopChain,
				Summary: fmt.Sprintf("examination %d of design design-loop is rejoined", f.round)}, nil
		}
		if asked.Goal != loopGoal {
			return httpd.DesignAnswer{Outcome: "refused", Summary: "goal " + asked.Goal + " has no approved review-round budget for this review",
				Decision: "approve goal " + asked.Goal + " with a budget first: metasystem goal approve " + asked.Goal}, nil
		}
		f.round, f.reading, f.reads, f.sent = 1, true, 0, f.digest()
		return httpd.DesignAnswer{Outcome: "in-progress", Chain: loopChain, Summary: "review job " + loopChain + " is running"}, nil
	}
	if f.digest() != f.sent && f.round < 2 {
		f.round, f.reading, f.reads, f.sent = 2, true, 0, f.digest()
		return httpd.DesignAnswer{Outcome: "in-progress", Chain: loopChain,
			Summary: "round 2 of critique " + loopChain + " requested, the final round; review job " + loopChain + "-r2 is running"}, nil
	}
	f.closed = true
	if err := f.appendDispositions(); err != nil {
		return httpd.DesignAnswer{Outcome: "failed", Summary: err.Error()}, nil
	}
	return httpd.DesignAnswer{Outcome: "confirmed", Chain: loopChain,
		Summary: "design design-loop's critique is complete: every finding is decided and chain " + loopChain + " is closed"}, nil
}

// decide writes one row, refused by the rules the engine's decisions file keeps.
func (f *loopFixture) decide(design string, round int64, row httpd.DesignRow) (httpd.DesignLoop, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	refuse := func(code, message string) (httpd.DesignLoop, error) {
		return httpd.DesignLoop{}, &httpd.DesignRefusal{Code: code, Message: message}
	}
	var finding *httpd.DesignFinding
	for at := range loopFindings {
		if loopFindings[at].ID == row.Finding {
			finding = &loopFindings[at]
		}
	}
	switch {
	case design != loopRecord || round != f.round || f.reading || f.closed:
		return refuse("round", fmt.Sprintf("round %d is not the critique's newest finished examination; only that round is decided", round))
	case finding == nil:
		return refuse("finding", fmt.Sprintf("round %d carries no finding %s", round, row.Finding))
	case finding.Material && row.Disposition == "noted":
		return refuse("noted", fmt.Sprintf("material finding %s cannot be noted; defer it as out-of-scope with the evidence that it is outside the brief", row.Finding))
	case row.Disposition == "refuted" && row.Reasoning == "":
		return refuse("reason", fmt.Sprintf("refuting %s needs your reason: the check you made and what it showed", row.Finding))
	case row.Disposition == "out-of-scope" && !strings.Contains(strings.ToLower(row.Reasoning), "scope") && !strings.Contains(strings.ToLower(row.Reasoning), "brief"):
		return refuse("join", fmt.Sprintf("finding id '%s' is out-of-scope without citing the brief's declared scope or threat model; cite it or the finding stands", row.Finding))
	}
	for _, held := range f.rows[round] {
		if held.Finding == row.Finding {
			return refuse("decided", fmt.Sprintf("finding %s of round %d is already decided: %s", row.Finding, round, held.Disposition))
		}
	}
	f.rows[round] = append(f.rows[round], row)
	return f.loop(design, false)
}

// appendDispositions is the close's last act, as the verb writes it.
func (f *loopFixture) appendDispositions() error {
	path := filepath.Join(f.checkout, filepath.FromSlash(loopRecord))
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var section strings.Builder
	section.WriteString("## Dispositions (critique " + loopChain + ")\n\n")
	section.WriteString("Written by metasystem design review when critique " + loopChain + " closed: every answered round's decisions, as the author made them.\n\n")
	section.WriteString("| Round | Finding id | Finding | Disposition | Reasoning and evidence | Amendment |\n| --- | --- | --- | --- | --- | --- |\n")
	for round := int64(1); round <= f.round; round++ {
		for _, row := range f.rows[round] {
			claim := ""
			for _, finding := range loopFindings {
				if finding.ID == row.Finding {
					claim = strings.ReplaceAll(finding.Claim, "|", `\|`)
				}
			}
			section.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s | %s |\n", round, row.Finding, claim, row.Disposition, row.Reasoning, row.Amendment))
		}
	}
	return os.WriteFile(path, []byte(strings.TrimRight(string(data), "\n")+"\n\n"+section.String()), 0o644)
}

// loopReads is the section the fake Partner drafts for Fold, in the tool's own
// section frame.
var loopReads = []fakeacp.Read{{
	When:  loopFold,
	Name:  "mcp__" + uitools.ServerName + "__" + uitools.OpSuggest,
	Title: "suggest(" + loopRecord + " · " + loopSection + ")",
	Result: uitools.PreparedSectionLine + "\n" + uitools.SectionSuggestionHeader + loopRecord + uitools.SuggestionJoin +
		loopSection + "\n" + uitools.SuggestionSeparator + "\n" + loopDrafted + "\n",
}}

var loopAnswers = []fakeacp.Answer{{When: loopFold, Chunks: []string{
	"I have drafted §4 anew with the funding goal and the reader budget in D1, which is what L1 asks for. ",
	"Old and new stand side by side on the design's page; Use writes exactly that section.\n",
}}}
