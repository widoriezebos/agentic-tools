# Astra's critique of g1-s60 revision 1

Produced 2026-09-26 by Codex on `gpt-6-astra`, read-only, against revision 1 of [g1-s60-proposed-by-the-partner.md](g1-s60-proposed-by-the-partner.md) at `77170636a`, under R-121 and R-124, treating g1-s58 as designed. Verbatim; the dispositions are the design's revision 2, and S60-01 is also g1-s58's revision 9.

---

1. **S60-01 — High — A stale second tab can apply the same proposal again**

   **Evidence (read):** D4 shares the runner and updates transcripts through beats, but refreshes the inbox after its own run. The inherited outcome contract (g1-s58 D6) specifies state writes without a conditional transition; its Apply guard is local to the caller. Each act gets a fresh operation identifier (internal/ui/act/act.go:544), and browser approvals can be repeated (internal/goal/approval.go:619).

   **Concrete failure:** Apply an approval from the card in tab A. Tab B's transcript receives the applied state, but its inbox still offers the waiting row. Pressing that stale Apply can record `applying` again and submit another approval: neither the local ref nor the goal-definition freshness comparison rejects it. Both cards ultimately show one applied proposal despite two publications. Simultaneous presses have the same problem.

   **Change:** Make the existing conversation writer atomically admit `applying` only from an eligible persisted state, reject dismissal of a non-waiting line, and return the current entry on conflict so the caller reconciles without dispatching.

   **Test 1:** Yes—changes the outcome route's admission contract and caller handling. **Test 2:** Fails **SAFE**—ordinary stale-tab use can repeat an already completed decision.

2. **S60-02 — High — Bulk Apply bypasses the selected lines' confirmation content**

   **Evidence (read):** D3 puts the complete line inside the open row while offering direct Apply over ticked rows. The borrowed queue control actually opens a bulk sheet (src/decisions/DecisionsPane.tsx:340), which displays every selected budget (src/decisions/BulkSheet.tsx:193). The parent instead captures budgets at the answer's terminal beat (g1-s58 D3) and requires the runner to send the displayed tuple.

   **Concrete failure:** On a cold inbox visit, tick two collapsed approval proposals and press Apply. Neither approval's budget has been presented for confirmation; the inbox also has no specified replacement for the card's terminal-beat budget capture. Computing budgets during dispatch authorizes unseen tuples, while requiring the missing captured tuples leaves the advertised bulk action unusable. Other verbs likewise hide their complete arguments inside unopened rows.

   **Change:** Before bulk confirmation, display the selected lines' complete arguments and approval tuples obtained from a pre-Apply backlog read, retain those tuples, and pass those reviewed lines to the shared runner.

   **Test 1:** Yes—changes bulk confirmation and the runner's prepared inputs. **Test 2:** Fails **SAFE**, or **WORK** if uncaptured approvals are refused—the direct bulk path lacks the parent's confirmation basis.

3. **S60-03 — Medium — The required reload removes failure results and recovery controls**

   **Evidence (read):** D1 returns only `waiting` entries; D5 reloads that payload after the run. `groupsOf` (src/decisions/groups.ts:79) removes groups without rows. The inherited runner contract (g1-s58 D5) requires visible refusal or uncertainty and recovery controls.

   **Concrete failure:** Apply the group's only proposal and receive an unresolved answer. Recording `unresolved` succeeds, then the required reload removes the row and its entire group. The inbox loses the explanation and recovery path precisely when the human needs them. Reloading during `applying` similarly omits the parent's "was being applied" warning. Keeping the page mounted does not preserve a row excluded from its payload.

   **Change:** Keep applying, refused and unresolved entries available to inbox result and recovery rendering across rereads, removing applied and dismissed entries as specified.

   **Test 1:** Yes—changes the reader's state selection and inbox result lifecycle. **Test 2:** Fails **WORK**—the first unsuccessful application loses its promised explanation and recovery surface.

**Deferred and non-material**

- Identity selection resolves to the same transcript, including the unnamed-seat fallback; absence of a Partner is explicitly handled. No material finding.
- Schema 4 already exists on the branch. Older group tables skip the new kind; supporting an old frontend's proposal UI is not a declared requirement. **Test 1:** No additional compatibility mechanism required. **Test 2:** Passes with the matching bundled frontend.
- Goal chips, Overview counts, bulk retry and additional stale markers can remain deferred. **Test 1:** They would change later behavior. **Test 2:** Passes without them.
- Reviewed design commit `77170636a`, treating g1-s58 as designed. Findings are inferred from the contracts and code read; no runtime tests were run or files changed.

VERDICT: 3 material findings (fail test 2): S60-01, S60-02, S60-03
