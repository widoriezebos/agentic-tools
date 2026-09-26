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

---

# Astra's round 2, the failsafe round, on revision 2

Produced 2026-09-26 by Codex on `gpt-6-astra`, read-only, on the same critic chain, against revision 2 at `c3e099af4`. Verbatim; the dispositions are the design's revision 3 and g1-s58's revision 10.

---

1. **S60-04 — High — A stale Apply can silently become a retry of an unresolved act**

   **Evidence (read):** g1-s58 D6 (revision 9) permits `applying` from `waiting`, `refused` or `unresolved`, but the request body carries only `{state, words}`. It does not identify the state the human saw. g1-s60 D5 updates another tab's transcript without refreshing its inbox.

   **Concrete failure:** Tab A's approval lands, its response is lost, and its runner records `unresolved`. Tab B still shows the proposal as waiting. Its ordinary Apply successfully transitions the persisted `unresolved` entry to `applying` and submits another approval, without first showing the uncertainty warning or asking for Try again. The unchanged goal definition and budget pass the freshness guard.

   **Change:** Include the caller's reviewed source state in the transition request and compare it atomically, returning the current entry without dispatch when it differs.

   **Test 1:** Yes—changes the transition payload, admission check and both callers. **Test 2:** Fails **SAFE**—a stale confirmation can repeat an act that already landed without informed retry consent.

2. **S60-05 — Medium — A conflict skips an action whose outcome is still unknown**

   **Evidence (read):** g1-s58 D6 (revision 9) directs the runner to reconcile every state conflict and "go on." A conflict can return `applying`, which is not settled. This contradicts the parent's stop-at-uncertainty rule, inherited by g1-s60 D4.

   **Concrete failure:** Tab A starts only `open X`. Tab B confirms `[open X, block Y by X]` while that first act remains in flight. Its first transition conflicts with `applying`; the specified reconciliation skips ahead and submits the block before X exists. The block is refused, then X lands, leaving the requested sequence incomplete. No explicit Continue authorized proceeding past the unknown result.

   **Change:** Reconcile conflicts according to the returned state, stopping on an in-flight or unresolved entry and requiring explicit Continue before sending subsequent lines.

   **Test 1:** Yes—changes conflict handling in the shared runner. **Test 2:** Fails **WORK**—an ordinary two-tab interaction violates the promised ordering and uncertainty stop.

3. **S60-06 — Medium — Try again on an `applying` row cannot succeed**

   **Evidence (read):** g1-s60 D3 explicitly offers Try again for persisted `applying` entries. The shared runner must first record `applying`, but g1-s58 D6 (revision 9) rejects `applying → applying`; neither design specifies an intervening recovery operation.

   **Concrete failure:** The browser closes after recording `applying` but before submitting the act. On return, the human checks the goal and presses the offered Try again. Every press receives a state conflict and sends nothing. The row remains permanently `applying`, and Dismiss is also inadmissible from that state.

   **Change:** Specify a supported recovery path for abandoned `applying` entries and align the offered controls with it while retaining the rejection of an ordinary duplicate Apply.

   **Test 1:** Yes—changes recovery controls and their transition contract. **Test 2:** Fails **WORK**—the advertised recovery fails on the first interrupted application.

**Deferred and non-material**

- **S60-02's confirmation fold is sufficient:** open rows and bulk sheets capture and display approval budgets before submission. **Test 1:** No further change required. **Test 2:** Passes.
- **S60-03's retention fold is sufficient apart from S60-06:** failed and in-flight entries survive rereads with their words. Counts describe those outstanding entries; "n new" remains based on proposal time. **Test 1:** No additional counting mechanism required. **Test 2:** Passes.
- The stale g1-s59 landing reference does not override the explicit removal of that dependency. **Test 1:** No substantive implementation difference. **Test 2:** Passes.
- Reviewed `c3e099af4`. Findings are inferred from the written contracts and code read; no files changed or runtime tests run.

VERDICT: 3 material findings (fail test 2): S60-04, S60-05, S60-06

---

# Astra's scoped confirmation read, on revision 3 (and g1-s58 revision 10)

Produced 2026-09-26 by Codex on `gpt-6-astra`, read-only, on the same critic chain, against `fe4bac5df`; the checklist was the three round-2 folds and nothing else. Verbatim; the disposition is g1-s60's revision 4 and g1-s58's revision 11.

---

Checked both designs at `fe4bac5df`; read-only.

1. **S60-04 — confirmed as folded.** g1-s58 D6 requires `{from, state, words}`, with `from` captured from the line shown at the press and compared inside the writer. g1-s60 D5 inherits that contract for the inbox. A stale `from: waiting` cannot move a persisted `unresolved` or `applying` entry; reconciliation sends no act. **Test 1:** Yes—the fold changes the request and admission contract. **Test 2:** Passes for the identified stale-waiting failure.

2. **S60-05 — confirmed as folded.** g1-s58 D6 continues past settled conflicts and stops on `applying` or `unresolved`, offering Continue; g1-s60 D5 states the same rule. In the two-tab `[open X, block Y by X]` case, conflicting with X's `applying` state now stops before the block. **Test 1:** Yes—the fold changes conflict control flow. **Test 2:** Passes for the identified premature continuation.

3. **S60-06 — not confirmed: retrying `applying` does not exclude another retry.** **Evidence:** g1-s58 D6 allows `applying → applying`; g1-s60 D3 offers that retry. The parent nevertheless promises that two tabs cannot publish the act twice. **Concrete failure:** Both tabs display the same abandoned `applying` approval. After inspecting the goal, the human presses Try again in each tab before either finishes. Both send `{from: applying, state: applying}`. The first accepted write leaves the compared state unchanged, so the second also succeeds—even with serialized writes—and both runners submit approval. The specified "only one admitted" fixture fails for this pair. Recovery and protection against stale `from: waiting` are present, but exclusion between retries is not. **Test 1:** Yes—the recovery admission mechanism must distinguish competing attempts. **Test 2:** Fails **SAFE**—an in-scope two-tab recovery can publish the same approval twice.

Every nonterminal state now has an allowed outgoing transition; no such line is permanently immovable. The remaining defect is duplicate retry admission.

VERDICT: 1 folds not confirmed: S60-06
