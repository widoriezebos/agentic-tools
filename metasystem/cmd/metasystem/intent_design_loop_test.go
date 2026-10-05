package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

// The design loop's close (g1-s66 D4): the author's decisions reach the
// register before the close, the final round exits by the engine's own
// classification, and the close appends the design's Dispositions section
// from every answered round.

// designLoopBed is a design review bed whose close owner runs the real
// register close on the chain's root: the refusal it returns is the close
// owner's stderr, and a close it admits marks the chain closed as the whole
// close owner does.
type designLoopBed struct {
	*designReviewBed
	closes int
	// deferred, when set, stands in for the goal ledger the deferral publishes
	// to, which this bed does not carry: the close records the deferral on the
	// register as the real close does once the goal accepted it.
	deferred bool
}

func newDesignLoopBed(t *testing.T) *designLoopBed {
	b := &designLoopBed{designReviewBed: newDesignReviewBed(t)}
	b.owners.rebind = func(string, string) (map[string]string, error) { return map[string]string{}, nil }
	dispatch := b.handler
	b.handler = func(process intentProcess) intentProcessResult {
		if len(process.argv) < 2 || process.argv[1] != "close" {
			return dispatch(process)
		}
		b.closes++
		root := flagValue(process.argv, "--job")
		if b.deferred {
			record := b.job(root)
			register := record["findingRegister"].([]any)
			for _, raw := range register {
				entry := raw.(map[string]any)
				if entry["status"] == "open" {
					entry["status"], entry["resolution"], entry["decisionOpid"] = "deferred", "deferred", "fixture-opid"
				}
			}
			record["chainClosed"] = true
			b.writeJob(record)
			return intentProcessResult{stdout: []byte("deferred\n")}
		}
		outcome, err := dispatchcore.CritiqueRegisterClose(b.install, root)
		if err != nil {
			code := 1
			var refused *dispatchcore.OpError
			if errors.As(err, &refused) {
				code = refused.Code
			}
			return intentProcessResult{stderr: []byte(err.Error() + "\n"), code: code}
		}
		record := b.job(root)
		record["chainClosed"] = true
		b.writeJob(record)
		return intentProcessResult{stdout: []byte(outcome + "\n")}
	}
	return b
}

func (b *designLoopBed) review(extra ...string) intentResult {
	b.t.Helper()
	_, result := b.do(append([]string{"design", "review", b.design, "--tool-calls", "30"}, extra...)...)
	return result
}

// register writes the chain root's finding register as the dispatch owner
// folds it: one open entry per id, and the round the fold reached.
func (b *designLoopBed) register(round int, limit int, material []int64, entries ...map[string]any) {
	record := b.job("rev1")
	register := []any{}
	for _, entry := range entries {
		full := map[string]any{"critic": "rev1", "rigorClass": "bounded", "grain": "invariant", "fixture": "",
			"factsDigest": strings.Repeat("a", 64), "facts": nil, "artifact": "plans/designs/reader.md", "title": entry["findingId"],
			"status": "open", "resolution": "", "decisionOpid": "", "evidence": "e", "evidenceDigest": strings.Repeat("b", 64), "multiplicity": 1}
		for key, value := range entry {
			full[key] = value
		}
		register = append(register, full)
	}
	history := []any{}
	for index, count := range material {
		history = append(history, map[string]any{"round": index + 1, "material": count})
	}
	record["findingRegister"], record["findingRegisterRound"], record["reviewRoundLimit"] = register, round, limit
	record["criticRoundsConsumed"], record["materialByRound"] = round, history
	record["goalId"], record["machineId"], record["mainId"], record["claimEpoch"] = "standing-validation", "m", "l", 1
	b.writeJob(record)
}

// decide fills the round's own decisions template, one row per finding.
func (b *designLoopBed) decide(result intentResult, rows map[string]string) string {
	b.t.Helper()
	template, _ := result.Data.(map[string]any)["template"].(string)
	if template == "" {
		b.t.Fatalf("no decisions template: %+v", result)
	}
	body := string(mustRead(b.t, template))
	for id, row := range rows {
		body = strings.Replace(body, "| "+id+" | DECIDE | | |", "| "+id+" | "+row+" |", 1)
	}
	b.writeFile(template, body)
	return template
}

func finding(id string, material bool, claim string) map[string]any {
	return map[string]any{"id": id, "severity": "high", "material": material, "claim": claim, "evidence": "reader.md:3"}
}

// TestDesignLoopRefutedFindingClosesUnchangedDesign (S66-06): a refuted
// material finding on the unchanged design closes the chain through the real
// register close, and the close appends the design's Dispositions section.
func TestDesignLoopRefutedFindingClosesUnchangedDesign(t *testing.T) {
	t.Parallel()
	b := newDesignLoopBed(t)
	b.review()
	b.finish("rev1", 1, "completed", finding("F1", true, "the reader forgets the page"), finding("N1", false, "a word is loose"))
	b.register(1, 2, []int64{1}, map[string]any{"findingId": "F1"})
	template := b.decide(b.review(), map[string]string{
		"F1": "refuted | searched reader.md for page and found line 3 | ",
		"N1": "noted | a wording matter | ",
	})
	result := b.review("--dispositions", template, "--after", "1")
	if result.Outcome != intentConfirmed || b.closes != 1 || !strings.Contains(result.Summary, "closed") {
		t.Fatalf("the refuted finding did not close the unchanged design: %+v closes=%d", result, b.closes)
	}
	design := string(mustRead(t, b.design))
	for _, want := range []string{"## Dispositions (critique rev1)", "| 1 | F1 | the reader forgets the page | refuted | searched reader.md for page and found line 3 |", "| 1 | N1 | a word is loose | noted |"} {
		if !strings.Contains(design, want) {
			t.Fatalf("the Dispositions section lacks %q:\n%s", want, design)
		}
	}
	// The same answer again is the close it already made, and appends nothing
	// twice.
	again := b.review("--dispositions", template, "--after", "1")
	if again.Outcome != intentUnchanged || b.closes != 1 || strings.Count(string(mustRead(t, b.design)), "## Dispositions (critique rev1)") != 1 {
		t.Fatalf("the repeated close: %+v closes=%d", again, b.closes)
	}
}

// finalRound runs a chain to its answered final round: round 1's F1 is
// accepted and folded, the follow-up re-raises F1 and adds the round's own
// findings, and the author folds again. What the round-2 answer does is the
// engine's close. The chain's root froze a two-round limit, as roots
// dispatched before the cap rose to five did; the final round is the frozen
// limit, whatever it is.
func finalRound(t *testing.T, b *designLoopBed, register func(), roundTwo []map[string]any, decisions map[string]string) intentResult {
	t.Helper()
	b.review()
	critical := finding("F1", true, "the reader forgets the page")
	critical["severity"] = "critical"
	b.finish("rev1", 1, "completed", critical)
	root := b.job("rev1")
	root["reviewRoundLimit"] = 2
	b.writeJob(root)
	first := b.decide(b.review(), map[string]string{"F1": "accepted | a real gap | section 2 names the page"})
	b.writeFile(b.design, strings.Replace(string(mustRead(t, b.design)), "First version.", "Second version.", 1))
	if requested := b.review("--dispositions", first, "--after", "1"); len(b.followUps) != 1 || !strings.Contains(requested.Summary, "rev1-r2") ||
		!strings.Contains(requested.Summary, "round 2 of critique rev1 requested, the final round") {
		t.Fatalf("round 2 was not requested for the changed design: %+v", requested)
	}
	b.finish("rev1-r2", 2, "completed", roundTwo...)
	second := b.decide(b.review(), decisions)
	b.writeFile(b.design, strings.Replace(string(mustRead(t, b.design)), "Second version.", "Third version.", 1))
	register()
	return b.review("--dispositions", second, "--after", "2")
}

// A critical finding permits one final examination within the budget.
func TestDesignLoopRoundTwoOfFiveIsFinal(t *testing.T) {
	t.Parallel()
	b := newDesignLoopBed(t)
	b.review()
	critical := finding("F1", true, "the reader forgets the page")
	critical["severity"] = "critical"
	b.finish("rev1", 1, "completed", critical)
	first := b.decide(b.review(), map[string]string{"F1": "accepted | a real gap | section 2 names the page"})
	b.writeFile(b.design, strings.Replace(string(mustRead(t, b.design)), "First version.", "Second version.", 1))
	requested := b.review("--dispositions", first, "--after", "1")
	if len(b.followUps) != 1 || !strings.Contains(requested.Summary, "round 2 of critique rev1 requested, the final round") {
		t.Fatalf("round 2 of a five-round design chain: %+v", requested)
	}
}

// TestDesignLoopFinalRoundExitsByTheEngine (S66-07, S66-08): the final
// round's answer on a design a fold changed is the close, never a third
// round, and it exits each of the engine's three ways.
func TestDesignLoopFinalRoundExitsByTheEngine(t *testing.T) {
	t.Parallel()
	t.Run("clean", func(t *testing.T) {
		b := newDesignLoopBed(t)
		result := finalRound(t, b, func() {
			b.register(2, 2, []int64{1, 1}, map[string]any{"findingId": "F1"}, map[string]any{"findingId": "R2"})
		}, []map[string]any{finding("F1", true, "the page is still forgotten"), finding("R2", true, "the round two claim")},
			map[string]string{"F1": "refuted | section 2 line 7 names it | ", "R2": "refuted | the brief scopes it | "})
		if result.Outcome != intentConfirmed || len(b.followUps) != 1 || b.closes != 1 || !strings.Contains(result.Summary, "closed") {
			t.Fatalf("the clean final round: %+v followUps=%d closes=%d", result, len(b.followUps), b.closes)
		}
		design := string(mustRead(t, b.design))
		for _, want := range []string{"| 1 | F1 | the reader forgets the page | accepted | a real gap | section 2 names the page |",
			"| 2 | F1 | the page is still forgotten | refuted | section 2 line 7 names it |", "| 2 | R2 |"} {
			if !strings.Contains(design, want) {
				t.Fatalf("the Dispositions section lacks %q:\n%s", want, design)
			}
		}
	})
	t.Run("fixture obligations", func(t *testing.T) {
		b := newDesignLoopBed(t)
		b.deferred = true
		result := finalRound(t, b, func() {
			b.register(2, 2, []int64{2, 1}, map[string]any{"findingId": "F1", "status": "resolved", "resolution": "withdrawn"},
				map[string]any{"findingId": "M1", "grain": "mechanical", "fixture": "go test ./reader -run TestPage"})
		}, []map[string]any{finding("M1", true, "the page count is off by one")},
			map[string]string{"M1": "accepted | a real slip | section 3 counts from one"})
		if result.Outcome != intentConfirmed || b.closes != 1 || !strings.Contains(result.Summary, "closed at round 2 on 1 fixture obligation") ||
			!strings.Contains(fmt.Sprint(result.Data.(map[string]any)["obligations"]), "M1: go test ./reader -run TestPage") {
			t.Fatalf("the fixture close: %+v", result)
		}
	})
	t.Run("human required", func(t *testing.T) {
		b := newDesignLoopBed(t)
		result := finalRound(t, b, func() {
			b.register(2, 2, []int64{1, 1}, map[string]any{"findingId": "F1"}, map[string]any{"findingId": "I1"})
		}, []map[string]any{finding("F1", true, "the page is still forgotten"), finding("I1", true, "an invariant")},
			map[string]string{"F1": "refuted | section 2 line 7 names it | ", "I1": "accepted | a real gap | section 4 holds it"})
		said := fmt.Sprint(result.Data.(map[string]any)["ownerMessage"])
		if result.Outcome != intentRefused || b.closes != 1 || strings.Contains(said, dispatchcore.CritiqueCapExhaustedReason) ||
			fmt.Sprint(result.Data.(map[string]any)["exitCode"]) != fmt.Sprint(dispatchcore.CritiqueCapExhaustedExitCode) ||
			!strings.Contains(said, "findings I1 for a person to decide") {
			t.Fatalf("the human-required close: %+v", result)
		}
		if closed, _ := b.job("rev1")["chainClosed"].(bool); closed || strings.Contains(string(mustRead(t, b.design)), "## Dispositions") {
			t.Fatalf("a human-required close closed the chain or appended the section")
		}
	})
}

// TestDesignLoopAcceptedMaterialOnAnEarlierRoundStillRefuses: before the
// final round an accepted material finding on the unchanged design is still
// the author's to fold, and nothing is closed.
func TestDesignLoopAcceptedMaterialOnAnEarlierRoundStillRefuses(t *testing.T) {
	t.Parallel()
	b := newDesignLoopBed(t)
	b.review()
	critical := finding("F1", true, "the reader forgets the page")
	critical["severity"] = "critical"
	b.finish("rev1", 1, "completed", critical)
	b.register(1, 2, []int64{1}, map[string]any{"findingId": "F1"})
	template := b.decide(b.review(), map[string]string{"F1": "accepted | a real gap | section 2"})
	if result := b.review("--dispositions", template, "--after", "1"); result.Outcome != intentRefused || b.closes != 0 {
		t.Fatalf("an accepted material finding on the unchanged round-1 design: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(b.install, "artifacts", "agents", "rev1", "rounds", "1", "decisions.md")); err != nil {
		t.Fatal(fmt.Errorf("the round's own decisions file: %w", err))
	}
}

// TestDesignLoopUnfrozenRootTakesTheCeiling: a design chain whose root froze
// no limit ends at metasystem.budget.review-round-max, not at a fixed five
// (Wido 2026-10-02): under a ceiling of 2, round 2 is the final round.
func TestDesignLoopUnfrozenRootTakesTheCeiling(t *testing.T) {
	t.Parallel()
	b := newDesignLoopBed(t)
	conf := filepath.Join(b.root(), "metasystem.conf")
	existing, _ := os.ReadFile(conf)
	b.writeFile(conf, string(existing)+"metasystem.budget.review-round-max=2\n")
	b.review()
	critical := finding("F1", true, "the reader forgets the page")
	critical["severity"] = "critical"
	b.finish("rev1", 1, "completed", critical)
	first := b.decide(b.review(), map[string]string{"F1": "accepted | a real gap | section 2 names the page"})
	b.writeFile(b.design, strings.Replace(string(mustRead(t, b.design)), "First version.", "Second version.", 1))
	requested := b.review("--dispositions", first, "--after", "1")
	if len(b.followUps) != 1 || !strings.Contains(requested.Summary, "round 2 of critique rev1 requested, the final round") {
		t.Fatalf("round 2 under a ceiling of 2: %+v", requested)
	}
}
