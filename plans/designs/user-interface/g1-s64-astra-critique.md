# Astra's critique of g1-s64 revision 1

Produced 2026-09-27 by Codex on `gpt-6-astra`, read-only, against revision 1 of [g1-s64-abandon-from-the-browser.md](g1-s64-abandon-from-the-browser.md) at `657621cd4`, under R-121, R-124 and R-125. Verbatim; the dispositions are the design's revision 2.

---

1. **S64-01 — High — A stale card can abandon a goal whose work has changed.**

   **Evidence, read:** D4 adds abandon without extending the stored reading or comparison. Those cover only approve and edit in proposal admission (metasystem/internal/ui/partner/proposals.go:253) and the runner (src/partner/proposing.ts:628). Abandon's revision check compares against its own fresh projection (metasystem/internal/goal/abandon.go:51), not what the card showed.

   **Concrete failure:** The Partner proposes abandoning queued goal G. In another tab, the human changes G's intent and next step. The original card retains its old title; pressing Apply archives the revised goal. The proposal-entry version does not detect changes to the goal.

   **Change:** Extend the existing proposal reading and pre-send comparison to abandon, refusing a changed or unreadable goal before sending.

   **Test 1:** Yes—changes admission, runner behavior and stale-card tests. **Test 2:** **Fails safety**—an ordinary two-tab sequence can abandon work the confirmation never showed.

2. **S64-02 — High — The confirmation omits abandon's dependent-repointing consequence.**

   **Evidence, read:** D2 says live dependents are refused, while D4 specifies the card's word without defining this consequence. In fact, a successor bypasses that refusal (abandon.go:233), and the mutation rewrites dependents (abandon.go:315). The existing argument renderer (src/partner/proposing.ts:261) requires explicit handling; a catalogue row and word-map entry do not supply it.

   **Concrete failure:** An erroneous Partner proposal names successor S for G. Applying it also redirects every live dependent from G to S, despite the design's stated refusal. A card showing only abandonment—or even "successor S" without explaining that effect—does not disclose the additional changes being authorized.

   **Change:** Require the card to display the actual reason and successor, explicitly state that live dependents will be redirected to that successor, and make D2's refusal statement conditional on no successor.

   **Test 1:** Yes—changes the confirmation content, D2's contract and its verification. **Test 2:** **Fails safety**—the first supported successor proposal can change dependencies without an informed press.

**Deferred and non-material**

3. **S64-03 — Low — The dependent refusal names an internal flag.** abandon.go:240 recommends `--carried`; the public descriptor exposes `--successor` (cmd/metasystem/intent_planning.go:220). Following that particular remedy under `metasystem goal abandon` uses the wrong public spelling. The refusal still names the dependent and gives valid `--waive` and `--also` alternatives. Change: express the remedy using the public command and `--successor`. Test 1: yes. Test 2: passes.

4. **S64-04 — Low — The ruling needs capture, not another human decision.** The register's ownership rule (metasystem/memory/rulings.md:20) forbids owning documents from hosting the only copy; the design's self-grade leaves the row to Wido. The durable register would retain R-125's narrower wording without its authorized extension. Change: have the coordinator capture the quoted extension in the register with its context. Test 1: no product change. Test 2: passes—the brief explicitly establishes Wido's extension, so no fresh permission or register-first build blocker is justified.

D1 otherwise holds: `SignedIn` supplies the human name, `request()` carries it and the proof, and abandon's appended history event can receive `recordSessionAuthority`. D3's route policy and public `successor` mapping fit the existing owners.

Reviewed design commit `657621cd4`. Evidence is source inspection; no tests ran and no files changed.

VERDICT: 2 material findings (fail test 2): S64-01, S64-02
