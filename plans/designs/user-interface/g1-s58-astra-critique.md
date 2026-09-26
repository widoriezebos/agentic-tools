# Astra's critique of g1-s58 revision 1

Produced 2026-09-26 by Codex on `gpt-6-astra`, read-only, against revision 1 of [g1-s58-the-partner-proposes-what-to-do.md](g1-s58-the-partner-proposes-what-to-do.md) at `01d9c9d32`, under R-121 and R-124. Verbatim; the dispositions are the design's revision 2.

---

1. **S58-01 — High — A landed act can be reported as refused**

   **Evidence:** D5 relies on `outcomeOf`. The section 1 claim that its refusals mean nothing landed is false: `runTransaction` (metasystem/internal/goal/txn.go:805) can land a push and then return a confirmation error; `settle` (metasystem/internal/ui/act/act.go:515) converts that error to `KindEngine/"refused"`, which becomes HTTP 409, and `outcomeOf` (metasystem/internal/ui/web/_app/src/backlog/editing.ts:252) labels it refused.

   **Scenario:** An approval lands, but its confirming fetch fails. The card records "refused," and the next Partner turn receives that false classification even though the goal was approved.

   **Change:** Preserve publication uncertainty through the act response and classify these errors as unresolved, reserving refused for demonstrated rejection.

   **Test 1:** Yes—changes the outcome contract and its assertions. **Test 2:** Fails **WORK and SAFE**: an explicitly in-scope lost confirmation produces a false outcome.

2. **S58-02 — High — Approval is not bound to the work reviewed on the card**

   **Evidence:** D2–D4 retain a first-sentence title and display an approval budget, while D10 defers the basis check. The approval route (metasystem/internal/ui/httpd/acts.go:272) accepts only the goal identifier and budget. `Approve` (metasystem/internal/goal/approval.go:548) loads the current goal and `bindApproval` (approval.go:455) authorizes that current definition.

   **Scenario:** A proposal offers approval of goal G. Before applying it, the same human edits G through the ordinary UI, materially expanding its intent after the unchanged first sentence. The old card still shows the same title and budget. Apply authorizes the expanded work without showing or checking the difference.

   **Change:** Display and retain the approval's reviewed definition and budget, and have the approval owner reject a materially changed basis before publication.

   **Test 1:** Yes—changes the approval payload, confirmation content and owner check. **Test 2:** Fails **SAFE**: a stale proposal authorizes different work; therefore this part of the deferred basis protection is necessary in step 1.

3. **S58-03 — High — Automatic refresh discards unsaved editor contents**

   **Evidence:** D5 exposes the goal page's existing refresh and calls it after acts. `Briefed.reload` (metasystem/internal/ui/web/_app/src/project/ProjectPane.tsx:202) switches to loading, unmounting `Columns`; its `GoalBlock` (ProjectPane.tsx:825) owns the open edit sheet. The master explicitly requires preserving unsaved buffers during refresh (plans/designs/user-interface-design.md:346).

   **Scenario:** The human has unsaved wording in G's edit sheet and applies a Partner proposal to park H. The successful park triggers the goal page's refresh, unmounting G's editor and losing its draft. No conflicting mutation of G is needed.

   **Change:** Offer a refresh that updates saved data while preserving mounted editors and their drafts.

   **Test 1:** Yes—changes the refresh behavior and its verification. **Test 2:** Fails **SAFE**: an ordinary first-use interaction loses user-entered data.

4. **S58-04 — Medium — Streamed cards become actionable before their outcome record exists**

   **Evidence:** Section 5 streams proposals as admitted, and D3 supplies Apply without a completion gate. D6 records `applying` by rewriting the Partner message. However, `Service.run` (metasystem/internal/ui/partner/service.go:896) appends that message only after `host.Prompt` finishes, at line 950; until then the offers belong to the running turn.

   **Scenario:** The Partner prepares one approval and continues answering. The human presses its visible Apply. The required state route cannot find the proposal in a persisted Partner message and must refuse. Proceeding despite that refusal would instead allow an approval whose applied state disappears when the completed message is subsequently appended or the page reloads.

   **Change:** Enable Apply only after its owning message is durably stored, and require a successful `applying` write before dispatch.

   **Test 1:** Yes—changes action availability and the persistence precondition. **Test 2:** Fails **WORK**: the first visible Apply cannot satisfy its required recording path; proceeding anyway also fails **SAFE**.

5. **S58-05 — Medium — "Try again" is offered when the server says the act already landed**

   **Evidence:** D5 offers Try again for every unresolved outcome. `outcomeOf` (editing.ts:256) puts `proof-not-recorded` in that category, although `settle` (act.go:533) explicitly reports that the act landed and must not be repeated. A new request receives a fresh operation identifier (act.go:566).

   **Scenario:** An approval lands but recording its authority proof fails. The card offers Try again as recovery. Pressing it creates another approval operation; it does not repair the original operation's missing proof. Session approvals do not take the proven-terminal approval's no-op branch in `Approve` (approval.go:619).

   **Change:** Preserve the known-landed distinction and remove resend recovery for `proof-not-recorded`, retaining the warning and inspection link.

   **Test 1:** Yes—changes the outcome-to-recovery mapping. **Test 2:** Fails **WORK**: the offered recovery repeats a completed decision instead of recovering the reported failure.

6. **S58-06 — Medium — Sign-in cancellation cannot settle the proposed runner**

   **Evidence:** D7 promises cancellation leaves the current action not applied and the remainder not run. The existing `askToSignIn` (metasystem/internal/ui/web/_app/src/shell/identity.tsx:106) provides only a success callback; closing the sheet (identity.tsx:132) silently discards it. The existing `EditSheet.submit` (src/backlog/EditSheet.tsx:177) consequently leaves its awaiting promise unsettled on cancellation.

   **Scenario:** The session expires on action two of three. The runner awaits sign-in so it can continue in order. The human closes the sheet. No callback settles that action, so the run remains pending and its in-flight guard prevents recovery.

   **Change:** Give the sign-in owner a cancellation result that settles the waiting action and releases the runner without dispatching anything further.

   **Test 1:** Yes—changes the sign-in interface and runner control flow. **Test 2:** Fails **WORK**: the expressly promised cancellation-and-recovery path cannot complete.

### Deferred and non-material

- D6 enumerates six states while sections 5–7 call them five; the opening mock also mixes six and seven actions. **Test 1:** No substantive implementation difference. **Test 2:** Step 1 works safely without correcting this arithmetic.
- Richer card editing, keyboard shortcuts and ledger-level Partner attribution remain deferred; none is needed to resolve the failures above.

VERDICT: 6 material findings (fail test 2): S58-01, S58-02, S58-03, S58-04, S58-05, S58-06

---

# Astra's round 2, the failsafe round, on revision 2

Produced 2026-09-26 by Codex on `gpt-6-astra`, read-only, on the same critic chain, against revision 2 at `e520a9194`. Verbatim; the dispositions are the design's revision 3.

---

1. **S58-07 — High — The freshness check does not bind the transaction to the reviewed goal.**

   **Evidence:** Design D5; `loadBacklog` (metasystem/internal/ui/web/_app/src/backlog/api.ts:303); `Observe` (metasystem/internal/ui/snapshot/snapshot.go:116); `runTransaction` (metasystem/internal/goal/txn.go:715); `Approve` (metasystem/internal/goal/approval.go:548).

   **Failure:** The card shows queued goal G with intent A. Canonical G changes to B while the browser's accepted tip still contains A. The runner's ordinary backlog read returns A, so its comparison passes; the approval transaction fetches canonical B and approves B. Even forcing a fresh fetch before submission leaves the comparison outside the transaction and its CAS retries.

   **Change:** Carry the reviewed basis into approve/edit and compare it inside each transaction mutation before publishing.

   **Test 1:** Yes—changes the request contract and owner checks. **Test 2:** Fails **SAFE**: stale reads can authorize unreviewed work or overwrite an intervening edit; this part of the deferred owner check is necessary for step 1.

2. **S58-08 — High — Approval can send a different budget from the one displayed.**

   **Evidence:** Design D4, D5, the proposal payload; `prefillFor` (src/backlog/moves.ts:104).

   **Failure:** The card displays budget A. G's budget changes to B before Apply, without changing intent, next step, tier or labels. D5 reloads the backlog and takes B from `prefillFor`; all four freshness comparisons pass. The press that confirmed A therefore submits B. The proposal payload retains neither the displayed tuple nor its source.

   **Change:** Capture the displayed budget and source with the approval line and submit that tuple, requiring renewed confirmation before substituting another.

   **Test 1:** Yes—changes the captured approval data and runner inputs. **Test 2:** Fails **SAFE**: the submitted budget is not necessarily the budget the human confirmed.

3. **S58-09 — High — An unreadable journal still turns a landed act into "refused."**

   **Evidence:** Design D6; `runTransaction` (txn.go:833); `MarkTerminal` (metasystem/internal/goal/journal.go:395); `ReadEntry` (journal.go:217).

   **Failure:** A push lands and its trailer is verified. A journal I/O failure then prevents `MarkTerminal` from reading the entry, so publication returns an error. The same failure prevents D6's subsequent `ReadEntry`. No readable `PhasePushed` entry satisfies its condition, and "every other publish error stays refused." The card reports a definite non-write and offers retry although the act landed. This requires one persistent journal-read fault, not two independent failures.

   **Change:** Treat journal-read failure as unresolved unless separate publication evidence proves that nothing was sent.

   **Test 1:** Yes—changes the new outcome rule and its failure-path test. **Test 2:** Fails **SAFE**: a landed act receives a false refusal and an invitation to repeat it.

4. **S58-10 — High — The modal guard misses Fleet's inline authorization editor.**

   **Evidence:** Design D5; `LaunchCard` Retry (src/fleet/LaunchCard.tsx:136); `FleetPane` reload (src/fleet/FleetPane.tsx:116); refresh offer (FleetPane.tsx:141).

   **Failure:** On Fleet, the human types authorization text and a review date into a failed launch's inline Retry form. No sheet covers the work area. They apply a goal proposal in the Partner drawer. Its automatic reread invokes Fleet's `reload`, which switches to loading and unmounts `Blocks`, `LaunchCard` and `Retry`. Both unsent fields disappear.

   **Change:** Make Fleet's proposal-triggered reread preserve the mounted retry editor, using its existing in-place reread path.

   **Test 1:** Yes—changes which Fleet refresh callback automatic updates invoke. **Test 2:** Fails **SAFE**: ordinary use loses unsaved human text despite the modal guard.

5. **S58-11 — Medium — The goal-page reread leaves the changed goal visibly unchanged.**

   **Evidence:** Design D5; `Briefed` (src/project/ProjectPane.tsx:149); `GoalBlock` (ProjectPane.tsx:792).

   **Failure:** While viewing G, apply a proposal changing its intent. D5 rereads only `ledger`, but the title, state chip and intent come from `briefing.goal`, derived from the separate project payload. They retain the old values after confirmation. The cited callback at line 886 deliberately preserves that payload after an **unconfirmed** save; a confirmed save instead calls `onEdited` to reread the project.

   **Change:** Refresh both payloads in place after a confirmed proposal act while preserving the mounted columns.

   **Test 1:** Yes—corrects the cited premise and the goal-page refresh implementation. **Test 2:** Fails **WORK**: the first successful edit or approval does not update the goal information the human is viewing.

6. **S58-12 — Medium — A definite no-op still stops the refusal-passed run as unresolved.**

   **Evidence:** Design D5; `blockRequest` (metasystem/internal/goal/verbs.go:2930); `terminalFromMutate` (txn.go:901); `settle` (act.go:519); `UNSETTLED` (src/backlog/editing.ts:227).

   **Failure:** Apply `[block Y by X, set priority Z]` when Y already waits for X. Admission permits both. The block returns `NothingToDo`, which becomes `OutcomeAbandoned` with no publish error, then HTTP 409/code `abandoned`. `outcomeOf` calls that unresolved, so Z is not run. D6 changes only publish-error handling and never reaches this definite non-write.

   **Change:** Give proven no-write outcomes a refusal classification that lets the runner continue.

   **Test 1:** Yes—changes outcome mapping and the refusal-passed verification case. **Test 2:** Fails **WORK**: an ordinary redundant proposal stops the run contrary to Wido's ruling.

**Deferred and non-material**

- Step 1 and its deferred list are explicit.
- Terminal-beat gating, failed-`applying` write handling, `proof-not-recorded` treatment and non-awaiting sign-in address their earlier findings.
- The provisional grammar can serve step 1; later renaming must account for persisted proposal names and field parsing, beyond the button-word map.
- No additional unsafe mixed-verb combination was demonstrated beyond the basis and outcome defects above.

VERDICT: 6 material findings (fail test 2): S58-07, S58-08, S58-09, S58-10, S58-11, S58-12
