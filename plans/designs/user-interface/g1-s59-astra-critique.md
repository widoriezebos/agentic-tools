# Astra's critique of g1-s59 revision 1

Produced 2026-09-26 by Codex on `gpt-6-astra`, read-only, against revision 1 of [g1-s59-one-word-for-one-act.md](g1-s59-one-word-for-one-act.md) at `4234b75c9`, under R-121 and R-124. Verbatim; the dispositions are in the design.

---

1. **S59-01 — Low, non-material — The blanket grep includes a different act.**
   **Evidence (read):** the design requires no old words outside specified exceptions, but project/Sheet.tsx:541 renders "Withdraw" to withdraw a question.
   **Concrete failure:** A literal zero-match check rejects a correct goal-label rename; changing this question button to "Unapprove" would misdescribe its act.
   **Change:** Scope the absence check to the four goal acts, allowing the question's "Withdraw."
   **Test 1:** Yes—this changes the verification criterion.
   **Test 2:** Yes, the slice works and is safe without an amendment: its stated four-goal-act scope already excludes question withdrawal.

2. **S59-02 — Low, non-material — The table omits secondary label locations.**
   **Evidence (read):** the design names the rank sheet's title but omits its submit button (src/backlog/RankSheet.tsx:95). The bulk-sheet row omits BulkSheet.tsx:140's "To Do → Not now" eyebrow and line 142's "Not now for selected" sheet name, which supplies Partner context.
   **Concrete failure:** Following only the table leaves these old names behind.
   **Change:** Include the rank submit button, bulk eyebrow, and bulk sheet name in the table's locations.
   **Test 1:** No—the complete design already requires the rename everywhere and an old-word search.
   **Test 2:** Yes, the slice works and is safe without expanding the table; its existing instructions cover these occurrences.

**Deferred and non-material**

Both findings are non-blocking clarification; neither warrants added machinery.

- **Revision and verbs:** Confirmed design commit `4234b75c9`, matching the working-tree file. Ran the executable's `help all`: all four names match. Their descriptors are at cmd/metasystem/intent.go:177 for pause/resume and intent_planning.go:189, line 247, for prioritize/unapprove.
- **Old-word coverage:** Searched all requested strings under `src/`. The table omissions are those in S59-02; the separate question withdrawal is S59-01. The Decided subtab's name and count are covered.
- **Generated vocabulary:** Regeneration is explicit in the design. The existing terms.test.ts:142 compares the complete help projection with `partner/vocabulary.json`. Read this test; did not run it.
- **"Paused" and migration wording:** "Paused" accurately describes the human-made parks selected by internal/ui/decisions/decisions.go:1093, and fits beside "Answered" and "Approved." "Until the verbs landed" is implementation-history wording, but does not misstate the act's effect.
- **Landing order:** No behavioral dependency on the old labels found. g1-s58 persists and dispatches route IDs; D4 explicitly anticipates removing the temporary wording bridge. The committed bundle explains serialization; removing that bridge is the named follow-up cleanup.

VERDICT: 0 material findings; build as written
