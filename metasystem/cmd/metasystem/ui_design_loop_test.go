package main

// The loop from the room's engine half (g1-s66 D1, D2, D4, §8): Send to
// critique runs design review with the funding goal and the reader budget,
// shows the engine's refusal for a missing budget verbatim, and rejoins a
// round already reading; the page reads each round's findings from that
// round's return, a non-material finding and a recurring id each their own
// card; a press writes one row into the engine's decisions file under its
// binding, refusing an unknown finding, a fifth value and a material noted;
// and Answer the round closes an unchanged design whose one material finding
// was refuted, through the real register close.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
)

func (b *designLoopBed) page() (lifecycle.Roots, string, intentOwners) {
	b.t.Helper()
	checkout, _ := filepath.EvalSymlinks(b.root())
	design, _ := filepath.EvalSymlinks(b.design)
	rel, err := filepath.Rel(checkout, design)
	if err != nil || strings.HasPrefix(rel, "..") {
		b.t.Fatalf("the design %s is not beneath the checkout %s", design, checkout)
	}
	owners := b.intentBed.owners()
	owners.delivery = b.owners
	owners.connection = b.connection
	return lifecycle.Roots{Checkout: checkout, Installation: stateroottest.Installation(b.t, b.install), StateRoot: stateroottest.State(b.t, b.install)}, filepath.ToSlash(rel), owners
}

func TestSendToCritiqueRunsTheVerbWithTheGoalAndTheBudget(t *testing.T) {
	t.Parallel()
	b := newDesignLoopBed(t)
	roots, id, owners := b.page()
	missing, err := designReviewRunWith(roots, id, httpd.DesignAsked{Goal: bedGoal}, owners)
	if err != nil || missing.Outcome != intentRefused || b.fresh != 0 ||
		missing.Summary != "the review needs the critic's tool-call budget, and none is configured; nothing was done" || !strings.Contains(missing.Decision, "--tool-calls 30") {
		t.Fatalf("a send without a budget: %+v %v fresh=%d", missing, err, b.fresh)
	}
	sent, err := designReviewRunWith(roots, id, httpd.DesignAsked{Goal: bedGoal, ToolCalls: 17}, owners)
	if err != nil || sent.Outcome != intentInProgress || b.fresh != 1 || sent.Chain != "rev1" {
		t.Fatalf("send to critique: %+v %v fresh=%d", sent, err, b.fresh)
	}
	brief := ""
	for _, call := range b.calls {
		if path := flagValue(call, "--brief"); path != "" {
			brief = string(mustRead(t, path))
		}
	}
	if !strings.Contains(brief, "Maximum reader tool calls: 17") || !strings.Contains(brief, "goal "+bedGoal) {
		t.Fatalf("the brief does not carry the budget and the goal:\n%s", brief)
	}
	// Sending a design already in a round rejoins it (R-129-ui).
	again, err := designReviewRunWith(roots, id, httpd.DesignAsked{Goal: bedGoal, ToolCalls: 17}, owners)
	if err != nil || again.Outcome != intentInProgress || b.fresh != 1 {
		t.Fatalf("the repeated send: %+v %v fresh=%d", again, err, b.fresh)
	}
	// The round's own composition names the model reading it.
	b.writeJSON(filepath.Join(b.install, "artifacts", "agents", "rev1", "rounds", "1", "composition.json"), map[string]any{"model": "gpt-6-astra"})
	loop, err := designLoopRead(roots, id)
	if err != nil || loop.State != "reading" || loop.Round != 1 || loop.Limit != 5 || loop.ToolCalls != 30 || loop.Goal != bedGoal || loop.Critic != "gpt-6-astra" {
		t.Fatalf("the reading chain: %+v %v", loop, err)
	}
}

func TestTheRoundIsReadFromItsReturnAndDecidedRowByRow(t *testing.T) {
	t.Parallel()
	b := newDesignLoopBed(t)
	roots, id, owners := b.page()
	designReviewRunWith(roots, id, httpd.DesignAsked{Goal: bedGoal, ToolCalls: 30}, owners)
	b.finish("rev1", 1, "completed", finding("F1", true, "the reader forgets the page\nChange: name the page in section 2\nTest: a reload lands on the page"),
		finding("N1", false, "a word is loose"))
	b.register(1, 2, []int64{1}, map[string]any{"findingId": "F1"})
	loop, err := designLoopRead(roots, id)
	if err != nil || loop.State != "deciding" || len(loop.Rounds) != 1 || len(loop.Rounds[0].Findings) != 2 || loop.Rounds[0].Answerable {
		t.Fatalf("the round's cards: %+v %v", loop, err)
	}
	card := loop.Rounds[0].Findings[0]
	if card.ID != "F1" || !card.Material || card.Severity != "high" || card.Evidence != "reader.md:3" ||
		card.Change != "name the page in section 2" || card.Tests != "a reload lands on the page" || loop.Rounds[0].Findings[1].Material {
		t.Fatalf("the cards as the return carries them: %+v", loop.Rounds[0].Findings)
	}
	for _, refused := range []struct {
		row  httpd.DesignRow
		code string
	}{
		{httpd.DesignRow{Finding: "F9", Disposition: "refuted", Reasoning: "r"}, "finding"},
		{httpd.DesignRow{Finding: "F1", Disposition: "deferred", Reasoning: "r"}, "disposition"},
		{httpd.DesignRow{Finding: "F1", Disposition: "noted", Reasoning: "r"}, "noted"},
		{httpd.DesignRow{Finding: "F1", Disposition: "refuted"}, "reason"},
		{httpd.DesignRow{Finding: "F1", Disposition: "out-of-scope"}, "reason"},
		{httpd.DesignRow{Finding: "F1", Disposition: "out-of-scope", Reasoning: "it is loose"}, "join"},
		{httpd.DesignRow{Finding: "F1", Disposition: "accepted", Reasoning: "r"}, "amendment"},
	} {
		_, err := designDecide(roots, id, 1, refused.row)
		refusal, ok := err.(*httpd.DesignRefusal)
		if !ok || refusal.Code != refused.code {
			t.Fatalf("%+v: %v, want the %s refusal", refused.row, err, refused.code)
		}
	}
	template := filepath.Join(b.install, "artifacts", "agents", "rev1", "rounds", "1", "decisions.md")
	if body := string(mustRead(t, template)); !strings.Contains(body, "Review binding: goal="+bedGoal+" work=design:01DESIGNREADER attempt=1") ||
		!strings.Contains(body, "| F1 | DECIDE | | |") {
		t.Fatalf("the engine's template, under its binding, before any row: %s", body)
	}
	if _, err := designDecide(roots, id, 1, httpd.DesignRow{Finding: "F1", Disposition: "refuted", Reasoning: "searched reader.md for page | found line 3"}); err != nil {
		t.Fatal(err)
	}
	if _, err := designDecide(roots, id, 1, httpd.DesignRow{Finding: "F1", Disposition: "accepted", Amendment: "a"}); err == nil || err.(*httpd.DesignRefusal).Code != "decided" {
		t.Fatalf("a second row for one card: %v", err)
	}
	loop, err = designDecide(roots, id, 1, httpd.DesignRow{Finding: "N1", Disposition: "noted", Reasoning: "a wording matter"})
	if err != nil || !loop.Rounds[0].Answerable || loop.State != "answered" {
		t.Fatalf("every card decided: %+v %v", loop, err)
	}
	// A reload finds the rows in the file.
	reloaded, _ := designLoopRead(roots, id)
	if len(reloaded.Rounds[0].Decisions) != 2 || reloaded.Rounds[0].Decisions[0] != (httpd.DesignRow{Finding: "F1", Disposition: "refuted",
		Reasoning: `searched reader.md for page \| found line 3`}) {
		t.Fatalf("the rows on reload: %+v", reloaded.Rounds[0].Decisions)
	}
	// Answer the round: the unchanged design closes through the real
	// register close, in the engine's words.
	answered, err := designReviewRunWith(roots, id, httpd.DesignAsked{Goal: bedGoal, ToolCalls: 30, After: 1}, owners)
	if err != nil || answered.Outcome != intentConfirmed || b.closes != 1 || !strings.Contains(answered.Summary, "critique is complete") {
		t.Fatalf("answer the round: %+v %v", answered, err)
	}
	closed, _ := designLoopRead(roots, id)
	if closed.State != "closed" || !strings.Contains(string(mustRead(t, b.design)), "## Dispositions (critique rev1)") {
		t.Fatalf("the closed chain: %+v", closed)
	}
}

func TestARecurringIDIsItsOwnCardInEachRound(t *testing.T) {
	t.Parallel()
	b := newDesignLoopBed(t)
	roots, id, owners := b.page()
	designReviewRunWith(roots, id, httpd.DesignAsked{Goal: bedGoal, ToolCalls: 30}, owners)
	b.finish("rev1", 1, "completed", finding("F1", true, "first"))
	designDecide(roots, id, 1, httpd.DesignRow{Finding: "F1", Disposition: "accepted", Reasoning: "real", Amendment: "section 2"})
	b.writeFile(b.design, strings.Replace(string(mustRead(t, b.design)), "First version.", "Second version.", 1))
	requested, err := designReviewRunWith(roots, id, httpd.DesignAsked{Goal: bedGoal, ToolCalls: 30, After: 1}, owners)
	if err != nil || !strings.Contains(requested.Summary, "round 2 of critique rev1 requested") || strings.Contains(requested.Summary, "the final round") {
		t.Fatalf("round 2 requested: %+v %v", requested, err)
	}
	b.finish("rev1-r2", 2, "completed", finding("F1", true, "second"))
	loop, _ := designLoopRead(roots, id)
	if len(loop.Rounds) != 2 || loop.Rounds[0].Findings[0].Claim != "first" || loop.Rounds[1].Findings[0].Claim != "second" ||
		len(loop.Rounds[0].Decisions) != 1 || len(loop.Rounds[1].Decisions) != 0 || loop.State != "deciding" {
		t.Fatalf("a recurring id across rounds: %+v", loop)
	}
	if _, err := designDecide(roots, id, 1, httpd.DesignRow{Finding: "F1", Disposition: "refuted", Reasoning: "r"}); err == nil {
		t.Fatalf("an answered round was decided again")
	}
	if _, err := os.Stat(filepath.Join(b.install, "artifacts", "agents", "rev1", "rounds", "2", "decisions.md")); err == nil {
		t.Fatalf("a read wrote round 2's decisions file")
	}
}
