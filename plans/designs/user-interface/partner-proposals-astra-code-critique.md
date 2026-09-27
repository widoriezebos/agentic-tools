# Astra's code critique of the Partner-proposes body of work

Produced 2026-09-28 by Codex on `gpt-6-astra`, read-only, in three parallel reads by area against ui-development at `7e0a486d1` (equal to main), on Wido's word "have astra critique the code you created and fix any issues". The briefs are the session's `astra-code-brief-A|B|C.md`; the six designs were the spec; Sol's per-slice reads were given as read. Verbatim; the dispositions and the fixes are recorded at the foot.

---

# Area A: the engine's abandon, the act layer, the routes

Reviewed HEAD `7e0a486d12a422eae394191242e7265353af1f9b`. No files changed. Focused Go tests could not start because the sandbox denied the temporary build directory. An in-memory probe of the production TypeScript runner reproduced A-01’s duplicate dispatch; it did not exercise the Go HTTP server or ledger.

1. **A-01 — High — A second tab can apply the same proposal while its first act remains in flight.**

   **Evidence:** [proposalTransitions and RecordProposal](metasystem/internal/ui/partner/proposals.go:82) admit `applying → applying`, increment the version at line 590, and publish it to other tabs at line 634. [offersTryAgain](metasystem/internal/ui/web/_app/src/partner/proposing.ts:552) permits retry whenever that tab’s local `inFlight` is false. `runProposals` sends another act after the successful write at line 1107. [Authority.request](metasystem/internal/ui/act/act.go:649) generates a fresh operation ID for each request.

   **Failure:** Tab A starts approval, recording `applying@2`. Tab B receives version 2 and presses the offered Try again. Its `applying@2 → applying@3` succeeds, authorizing another send. The probe produced both sends before either response completed. Both approvals can land if A’s publication finishes while its HTTP response remains delayed: [Approve](metasystem/internal/goal/approval.go:619) suppresses identical proven/attorney approvals, but excludes session approvals. With an unchanged existing budget, the freshness comparison also passes. A’s eventual outcome write merely conflicts after publication.

   **Smallest fix:** Preserve the existing attempt’s operation identity and reconcile it before permitting another publication. Learning a newer `applying` version must not itself authorize a fresh act.

   **Marks:** Material: **yes**. **SAFE fails**: one proposal can publish twice.

2. **A-02 — High — Refresh cannot recover an unknown push, leaving subsequent browser acts blocked.**

   **Evidence:** [Authority.unsettled](metasystem/internal/ui/act/act.go:605) returns `pushed-unknown` and directs the human to the next read. [runTransaction](metasystem/internal/goal/txn.go:815) leaves the journal at `pushed` when confirmation fails; `Publish` at line 609 refuses subsequent operations while that entry remains. The browser’s [advance callback](metasystem/cmd/metasystem/ui.go:246) only calls `FetchAdvanceBounded`; its [implementation](metasystem/internal/goal/project.go:199) advances the accepted ref without classifying journal entries. No UI path invokes journal recovery.

   **Failure:** Approval lands remotely, but its confirming fetch fails. Connectivity returns and Refresh successfully shows the approved goal. Nevertheless, trying an unrelated act still returns “this clone mutates nothing until it is classified.” Dismissing the proposal only changes the transcript; retrying creates another operation ID and meets the same blocker. The page cannot restore normal operation.

   **Smallest fix:** Connect browser recovery to the existing [RecoverWithPolicy classification](metasystem/internal/goal/recover.go:51), returning the original operation’s disposition before allowing a resend. A successful ledger read must not stand in for journal recovery.

   **Marks:** Material: **yes**. **WORK fails**: an explicitly supported unknown outcome cannot be recovered from the page.

3. **A-03 — Medium — First sign-in changes conversations and strands the proposal being applied.**

   **Evidence:** [partnerHuman](metasystem/internal/ui/httpd/partner.go:169) selects `"seat"` before an unnamed human signs in, then selects the session’s human. `partnerProposal` at line 524 resolves that identity again for every outcome write. [signIn](metasystem/internal/ui/httpd/session.go:120) installs the session without transferring the conversation. [Service.conversation](metasystem/internal/ui/partner/service.go:231) opens a separate transcript per name. Meanwhile, [Foot.apply](metasystem/internal/ui/web/_app/src/partner/Proposal.tsx:279) resumes the existing card after sign-in.

   **Failure:** Start an unproven seat with no configured or remembered human, obtain a proposal, and press “Sign in to apply.” Signing in as Wido switches the outcome route to Wido’s empty transcript. Recording `applying` returns “this conversation has no answer for turn …,” so no act is sent. Reloading reads Wido’s transcript and removes the original proposal from view. The unnamed-seat configuration is explicitly covered by [the session tests](metasystem/internal/ui/httpd/session_test.go:392).

   **Smallest fix:** Preserve the conversation across the first identity binding, transferring the unnamed seat’s transcript before the sign-in continuation runs.

   **Marks:** Material: **yes**. **WORK fails**: the advertised first-use “Sign in to apply” flow cannot apply its proposal.

**Deferred and non-material**

4. **A-04 — Low — The shared decoder accepts oversized bodies and malformed trailing delimiters.**

   **Evidence and failure:** [decode](metasystem/internal/ui/httpd/write.go:393) limits reading to `maxWriteBody+1` but never checks the byte count. A complete valid object of 65,537 bytes therefore passes the declared 65,536-byte bound. Its `reader.More()` check also accepts `{"version":1,"state":"applying"}]`, allowing the proposal write despite malformed trailing JSON; `More` tests for another array/object element, not end-of-document.

   **Smallest fix:** Count bounded bytes as `decodeDocument` does, then require a second decode to return `io.EOF`.

   **Marks:** Material: **no**. No demonstrated first-use **WORK/SAFE failure** under the supplied threat model; current browser serialization does not produce these bodies.

5. **A-05 — Low — Walkthrough abandon accepts inputs the real engine refuses.**

   **Evidence and failure:** [ledger.abandon](metasystem/internal/ui/httpd/walkthrough/main.go:1571) checks only that the reason is nonblank. It accepts an embedded newline, whereas [goal.Abandon](metasystem/internal/goal/abandon.go:50) refuses it. Also, with live D blocked by G, abandoning G with successor D rewrites D to block itself and returns success. The real abandon validates the resulting commit at line 376, whose [graph validation](metasystem/internal/goal/validate.go:348) rejects that cycle.

   **Smallest fix:** Enforce the one-line reason and reject successor repointing that introduces a dependency cycle before mutating the fixture.

   **Marks:** Material: **no**. Production **WORK/SAFE remain intact**; these are walkthrough fidelity defects outside its normal canned path.

VERDICT: 3 material findings: A-01, A-02, A-03; 2 non-material: A-04, A-05


---

# Area B: the Partner's service, the tools, the decisions composition

1. **B-01 — High — One oversized proposal can make the saved conversation unreadable.**

   **Evidence, read:** `Readers.propose` checks that the goal is nonempty but imposes no length limit ([propose.go:510](metasystem/internal/ui/uitools/propose.go:510)). `admitProposal` retains that goal even when refusing it ([proposals.go:227](metasystem/internal/ui/partner/proposals.go:227)). `Conversation.Append` bounds only `Message.Text`, then serializes the complete message; `Conversation.load` rejects lines exceeding approximately 257 KiB ([conversation.go:670](metasystem/internal/ui/partner/conversation.go:670), [conversation.go:630](metasystem/internal/ui/partner/conversation.go:630)).

   **Failure:** Call `propose` with `verb:"resume"`, a goal containing 300,000 `a` characters, and `explanation:"x"`. Admission refuses the nonexistent goal but persists the oversized proposal. After a service restart, opening that transcript fails. Decisions also fails because its handler propagates the transcript-read error ([decisions.go:110](metasystem/internal/ui/httpd/decisions.go:110)). No Apply press is needed.

   **Smallest fix:** Make transcript decoding accommodate complete messages the writer stores, including proposal metadata, and enforce the ledger’s existing bounds on goal and label tokens before framing them.

   **Marks:** changes what is built: yes; material: **yes — WORK fails**.

2. **B-02 — Medium — Malformed list arguments silently remove requested dependencies.**

   **Evidence, read:** `wire.call` invokes the tool without validating its advertised schema ([mcp.go:426](metasystem/internal/ui/uitools/mcp.go:426)). `Args.List` silently drops non-string array entries, and `valueOf` accepts the resulting empty list ([mcp.go:94](metasystem/internal/ui/uitools/mcp.go:94), [propose.go:690](metasystem/internal/ui/uitools/propose.go:690)). The browser sends that empty list as `blockedBy` ([proposing.ts:907](metasystem/internal/ui/web/_app/src/partner/proposing.ts:907)).

   **Failure:** With `bank-sandbox` live, submit an otherwise valid Open proposal containing `"blocked-by":[{"id":"bank-sandbox"}]`. Instead of refusing the malformed argument, the tool reports preparation success and offers an Open without that dependency. Apply creates a queued goal without its requested blocker; the engine only records and parks behind blockers actually supplied ([verbs.go:904](metasystem/internal/goal/verbs.go:904)).

   **Smallest fix:** Reject unsupported list types and non-string members by field name before preparing a proposal. Preserve the explicitly supported string shorthand without silently discarding entries.

   **Marks:** changes what is built: yes; material: **yes — WORK fails**.

**Deferred and non-material**

3. **B-03 — Low — Proposal outcome context disappears after ten ordinary exchanges.**

   **Evidence, read:** `Service.submit` passes only the last twenty messages to `proposalsBlock`, although that function already selects the last two proposal-bearing answers ([service.go:511](metasystem/internal/ui/partner/service.go:511), [proposals.go:474](metasystem/internal/ui/partner/proposals.go:474)).

   **Failure:** Propose an action, exchange ten further question/answer pairs without proposals, then apply the old action from Decisions. The next prompt omits its recorded outcome despite it remaining among the latest proposal-bearing answers.

   **Smallest fix:** Select the two proposal-bearing answers from the retained transcript independently of the recovery-history window.

   **Marks:** changes what is built: yes; material: **no — the immediate proposal/apply flow remains working and safe**.

4. **B-04 — Low — Both Partner instructions incorrectly say only the conversation card can apply proposals.**

   **Evidence, read:** The “Propose an act” instruction makes that exclusive claim in both [the embedded skill:32](metasystem/internal/ui/partner/project-partner.skill.md:32) and [the canonical skill:32](metasystem/skills/project-partner/SKILL.md:32). Decisions also invokes the shared runner ([proposals.ts:437](metasystem/internal/ui/web/_app/src/decisions/proposals.ts:437)).

   **Failure:** When explaining where an unanswered proposal can be applied, the Partner is instructed to give an incomplete account of the available interface.

   **Smallest fix:** Name both the conversation card and Decisions in both copies, and update the assertion preserving the old sentence.

   **Marks:** changes what is built: yes; material: **no — both application surfaces still work safely**.

Reviewed HEAD `7e0a486d12a422eae394191242e7265353af1f9b`. Findings are source-read evidence. Focused Go tests could not start because the sandbox denied the temporary build directory. No files were edited; the worktree remains clean.

VERDICT: 2 material findings: B-01, B-02; 2 non-material: B-03, B-04


---

# Area C: the frontend

Reviewed clean HEAD `7e0a486d12a422eae394191242e7265353af1f9b`. No files changed. In-memory Node probes reproduced C-01 through C-04 using the source functions and stubbed ports. Vitest could not run because its executable is absent; C-05 was verified by tracing the component lifecycle, without a mounted-browser test.

1. **C-01 — High — An applied version conflict lets later actions use a stale goal read.**

   **File/function:** [proposing.ts:1091](metasystem/internal/ui/web/_app/src/partner/proposing.ts:1091), `runProposals`.

   **Evidence and failure:** A normal successful act sets `stale = true` at line 1123. An `applying` write that instead returns a conflict containing an already-applied entry reconciles it and continues without invalidating the read.

   Start with an edit of G from intent A to B followed by an approval displaying A. This tab reads A; another tab applies the edit; this tab receives that applied conflict. It skips the edit, then checks the approval against its retained A and sends it against current intent B. The probe observed one backlog read and an approval sent with reviewed intent A while current intent was B.

   **Smallest fix:** When reconciling an applied conflict, invalidate the backlog reading just as a successful local act does, so the next guarded line fetches again.

   **Marks:** Changes implementation: yes. **Material: yes — SAFE fails.**

2. **C-02 — High — The inbox can resend an act the drawer knows already applied.**

   **Files/functions:** [store.tsx:1642](metasystem/internal/ui/web/_app/src/partner/store.tsx:1642), `runLines`; [decisions/proposals.ts:405](metasystem/internal/ui/web/_app/src/decisions/proposals.ts:405), `useProposals`; [decisions/proposals.ts:123](metasystem/internal/ui/web/_app/src/decisions/proposals.ts:123), `lineOf`.

   **Evidence and failure:** The drawer and inbox maintain separate outcome marks. When an act lands but its outcome write fails, the drawer retains `unrecorded: applied` and suppresses retry. The inbox constructs the same proposal using only its own marks.

   Approve G in the drawer using G’s existing budget, then fail the outcome write before persistence. The transcript remains `applying`, version 2. The ensuing inbox reread offers **Try again**, although the adjacent drawer says applied. Pressing it records `applying` at version 3 and sends a second approval. The probe observed two sends. The transition is allowed by [proposals.go:82](metasystem/internal/ui/partner/proposals.go:82); an already-approved goal is admissible, and session approvals do not receive the no-op treatment shown in [approval.go:619](metasystem/internal/goal/approval.go:619).

   **Smallest fix:** Have both surfaces consult the known outcome held in the existing Partner store when displaying and admitting retries.

   **Marks:** Changes implementation: yes. **Material: yes — SAFE fails.**

3. **C-03 — Medium — An automatically folded older card cannot reopen.**

   **Files/functions:** [proposing.ts:156](metasystem/internal/ui/web/_app/src/partner/proposing.ts:156), `cardsIn`; [store.tsx:1545](metasystem/internal/ui/web/_app/src/partner/store.tsx:1545), `reopenProposals`; [Proposal.tsx:94](metasystem/internal/ui/web/_app/src/partner/Proposal.tsx:94), `ProposalCard`.

   **Evidence and failure:** `cardsIn` folds every older card with waiting lines independently of `dismissedCards`. The folded button calls `reopenProposals`, which only removes the card from `dismissedCards`.

   With two answers containing waiting proposals, press the older card’s “Show what the Partner proposed” button. It was never explicitly dismissed, so removing its ID changes nothing; the automatic condition still folds it. The probe returned `folded: true` before and after the press. Its arguments and controls remain inaccessible through that card, including after following its goal chip.

   **Smallest fix:** Record explicit expansion and let it override automatic folding for the selected card.

   **Marks:** Changes implementation: yes. **Material: yes — WORK fails.**

4. **C-04 — Medium — A recovered proposal keeps its obsolete failure and retry button.**

   **Files/functions:** [proposing.ts:1060](metasystem/internal/ui/web/_app/src/partner/proposing.ts:1060), `runProposals`; [proposing.ts:470](metasystem/internal/ui/web/_app/src/partner/proposing.ts:470), `lineState`; [proposing.ts:559](metasystem/internal/ui/web/_app/src/partner/proposing.ts:559), `offersTryAgain`.

   **Evidence and failure:** A failed outcome write installs `mark.unrecorded`. A later run clears `notRun` and `refusedUnsent`, but neither successful outcome recording nor reconciliation clears the obsolete `unrecorded` mark. Both displayed status and retry eligibility prefer it over the persisted entry.

   First refuse an act and fail its outcome write. Resolve the refusal’s cause, then retry successfully and persist `applied`. The probe ended with persisted state `applied`, while the line still displayed “refused: claimed; the conversation could not record this” and offered **Try again**. Further presses meet the settled entry but leave the same misleading recovery state.

   **Smallest fix:** Clear an obsolete unrecorded outcome when a later definitive outcome is successfully recorded or reconciled; preserve it when no newer outcome establishes what happened.

   **Marks:** Changes implementation: yes. **Material: yes — WORK fails.**

5. **C-05 — Medium — A failed offered refresh destroys the inline Fleet draft.**

   **Files/functions:** [FleetPane.tsx:104](metasystem/internal/ui/web/_app/src/fleet/FleetPane.tsx:104), `FleetPane` read effect; [FleetPane.tsx:147](metasystem/internal/ui/web/_app/src/fleet/FleetPane.tsx:147), offered refresh and rendering; [LaunchCard.tsx:136](metasystem/internal/ui/web/_app/src/fleet/LaunchCard.tsx:136), `Retry`.

   **Evidence and failure:** Fleet offers `again`, which preserves the page while requesting data. However, a rejected request sets the whole pane to `failed`; the conditional rendering removes `Blocks`, including its `LaunchCard`. The inline authorization text and review date live in `Retry`’s local state.

   Type those fields, apply an unrelated proposal in the drawer, and let its requested Fleet reread fail. The error view unmounts the form and discards the draft. A successful subsequent read creates a new form with initial values. The source guard at [refresh.test.ts:120](metasystem/internal/ui/web/_app/src/shell/refresh.test.ts:120) only checks the offered callback for `state: "loading"` and misses this failure path.

   **Smallest fix:** Retain the last successful Fleet payload and mounted children when an offered refresh fails, displaying the refresh error alongside them.

   **Marks:** Changes implementation: yes. **Material: yes — WORK fails.**

**Deferred and non-material**

6. **C-06 — Low — Drawer dismissal silently loses failed writes and conflict reconciliation.**

   **Files/functions:** [store.tsx:1586](metasystem/internal/ui/web/_app/src/partner/store.tsx:1586), `writeState`; [store.tsx:1709](metasystem/internal/ui/web/_app/src/partner/store.tsx:1709), `dismissProposals`.

   **Evidence and failure:** Dismiss folds the card immediately and launches writes whose handler processes only `written`. If persistence fails, the proposal remains waiting without a failure explanation; a returned conflict entry is likewise ignored by this path. Reloading can restore the supposedly dismissed proposal.

   **Smallest fix:** Surface failed dismissal writes and reconcile conflict entries, following the existing inbox dismissal behavior.

   **Marks:** Changes implementation: yes. **Material: no** — no ledger act occurs, and the proposal remains available for recovery.

7. **C-07 — Low — Successful drawer dismissal leaves the mounted inbox stale.**

   **Files/functions:** [store.tsx:1709](metasystem/internal/ui/web/_app/src/partner/store.tsx:1709), `dismissProposals`; [DecisionsPane.tsx:149](metasystem/internal/ui/web/_app/src/decisions/DecisionsPane.tsx:149), payload-reading effect.

   **Evidence and failure:** Drawer dismissal updates the conversation but never requests the offered reread. Decisions reads on `attempt` changes and does not fold the Partner’s proposal updates into its payload. With Decisions open behind the drawer, successfully dismissing a proposal leaves its inbox row and count visible until another refresh.

   **Smallest fix:** Request the existing deferred reread after the dismissal writes finish.

   **Marks:** Changes implementation: yes. **Material: no** — refresh restores consistency, and the version check prevents the stale row from applying the dismissed proposal.

VERDICT: 5 material findings: C-01, C-02, C-03, C-04, C-05; 2 non-material: C-06, C-07


---

# The confirmation read of the first fix round

Produced 2026-09-28 by Codex on `gpt-6-astra`, read-only, against ui-development at `9d5c8bc47` (the two fix branches merged, plus the document decoder). Verbatim.

| Finding | Confirmation | Holding test and evidence |
|---|---|---|
| A-01 | **Not closed** | `TestIdenticalSignedInSessionApprovalIsANoOp` and `TestASecondIdenticalApproveFromASessionAnswersApplied` cover duplicate approvals. Concurrent edits still dispatch twice: the production-runner probe observed `applying@2`, `applying@3`, then two sends of the same edit. [runProposals](metasystem/internal/ui/web/_app/src/partner/proposing.ts:1181) permits this when neither initial read carries the effect; the [Go edit](metasystem/internal/goal/verbs.go:3521) records both operations without an identical-fields no-op. **SAFE fails.** |
| A-02 | **Closed** | `TestTheNextActClassifiesAPushNobodyCouldConfirm`, `TestAPushedEntryARecoveryMayNotTouchLeavesTheActUnresolved`, and `TestTheRefreshAdvanceClassifiesTheJournalFirst` hold the reported stranded-push recovery. The new concurrency regression is D-01 below. |
| A-03 | **Not closed** | `TestTheFirstSignInKeepsTheSeatsConversation` and `TestTheSeatsMessagesMoveToTheHumanAndLeaveTheSeatEmpty` hold completed-turn transfer. [Adopt](metasystem/internal/ui/partner/service.go:268) transfers only saved messages. Signing in during an answer leaves `running.human` and the running turn’s conversation unchanged; its [answer is appended to the emptied seat](metasystem/internal/ui/partner/service.go:1046). The named conversation loses that answer and its proposals. Sitting state also remains behind. **WORK fails.** |
| A-04 | **Closed** | `TestAWriteBodyOneBytePastTheBoundIsRefusedForItsSize`, `TestAWriteBodyWithJSONAfterTheObjectIsRefused`, and the at-bound/whitespace controls hold the correction. The corresponding document-decoder tests cover the extra commit. |
| A-05 | **Closed** | `TestTheFixtureAbandonRefusesAReasonThatIsNotOnOneLine` and `TestTheFixtureAbandonRefusesASuccessorThatWaitsForTheAbandonedGoal` hold both reported examples. Transitive cycles remain a residual below. |
| B-01 | **Closed** | `TestTheIdsAndLabelsAFrameCarriesAreTheLedgersOwn`, `TestAFramePastTheLedgersBoundsIsRefusedWithoutKeepingIt`, and `TestAMessageAtTheBoundReadsBackWhateverJSONSpendsOnIt` hold the oversized-field failure and worst-case text escaping. Aggregate proposal size remains a residual. |
| B-02 | **Closed** | `TestAListThisToolCannotReadWholeIsRefusedByItsName` checks malformed members, unsupported values, and preservation of comma-string shorthand. |
| B-03 | **Closed** | `TestWhatHappenedToAProposalOutlastsTenExchangesWithoutOne` checks the next prompt after the outcome leaves the ordinary history window. Selection now uses the retained transcript. |
| B-04 | **Closed** | `TestTheSkillTellsThePartnerToPropose` names both surfaces; `TestTheEmbeddedSkillIsTheKitsOwnFile` holds copy equality. |
| C-01 | **Closed** | `reads again after an applied conflict and refuses the next line against the fresh reading` holds the reported edit-then-approve sequence. Both existing conflict sites invalidate the read. |
| C-02 | **Closed** | `what the drawer already knows about a line` exercises the real hook through server rendering and checks the displayed outcome, absent retry, and zero sendable lines. Its interaction with C-04 remains defective below. |
| C-03 | **Closed** | `unfolds an older card the human expanded, and folds it again when it is dismissed` holds the folding rule. The store’s expansion wiring was read. |
| C-04 | **Not closed** | The two new runner tests hold local clearing. Across surfaces, [knownWith](metasystem/internal/ui/web/_app/src/decisions/proposals.ts:145) imports the drawer’s old `unrecorded` whenever the inbox’s own mark is null—including after successful clearing. The probe produced persisted `applied@4` with “refused: claimed; the conversation could not record this” and `Try again: true`. Clearing must reach the shared mark, or its version must prevent an older mark overriding a settled entry. **WORK fails.** |
| C-05 | **Closed** | `keeps what it already read when the read fails, in every pane that offers one` guards all six failure paths. Source inspection confirms the successful payload and mounted child branch remain. This is source evidence, not a mounted-form test. |
| C-06 | **Closed** | The conflict and failed-dismissal guards in `store.test.ts` cover the changed callbacks structurally. Reading confirms conflict entries are folded in and failed writes expose their words and undo dismissal. |
| C-07 | **Closed** | `asks for the offered re-read once, after the writes have answered` guards the callback. Reading confirms `Promise.all` finishes before the single deferred reread request. |

Reviewed clean HEAD `9d5c8bc472bbcb141d3015e1450d51f9dba4d622` and the complete specified diff, excluding the generated bundle. No files were edited. All named tests were **read**: Go could not create its temporary build directory, and Vitest was absent. The TypeScript probes **ran in memory against production functions**, with substituted I/O; they did not exercise the HTTP server or React lifecycle.

1. **D-01 — High — Recovery can terminalize a browser act that is still executing.**

   **File:** [act/recover.go:40](metasystem/internal/ui/act/recover.go:40).

   **Evidence, read:** Refresh and another act now invoke whole-journal recovery concurrently with publication. After an act’s push lands but before its confirming fetch finishes, recovery finds its operation in canonical history and [marks it terminal](metasystem/internal/goal/recover.go:87). The original request subsequently calls [MarkTerminal](metasystem/internal/goal/txn.go:833), which rejects an already-terminal entry. [settle/unsettled](metasystem/internal/ui/act/act.go:553) then report an engine refusal and skip the separate authority-proof write.

   **Concrete failure:** Press Refresh in another tab while an approval is confirming. The approval lands, but its originating request can answer “refused” and omit its authority-proof record. The live-owner test covers another process, not another request owned by this server process.

   **Smallest fix:** Coordinate recovery with active browser publications and settlement through one shared owner, so recovery cannot take an entry away from an executing request. Add a controlled concurrent Refresh/publication test.

   **Material: yes — WORK and SAFE fail.**

2. **D-02 — High — Retry records success from a backlog whose canonical fetch failed.**

   **File:** [proposing.ts:1186](metasystem/internal/ui/web/_app/src/partner/proposing.ts:1186).

   **Evidence, ran:** With an `applying` park, `look.outcome: "failed"`, and a cached parked row, HEAD wrote only `applied` and sent nothing. The base runner attempted the act. `guardFor` checks fetch success only for approve, edit, and abandon; the new reconciliation branch uses rows for the other verbs without checking it.

   **Concrete failure:** The accepted cache says parked, but the canonical goal has since resumed. A failed fetch followed by Try again permanently settles the park proposal as applied while leaving the goal running.

   **Smallest fix:** Require a successful canonical read before inferring an effect and recording `ALREADY_CARRIED`; otherwise preserve an unresolved outcome.

   **Material: yes — WORK and SAFE fail.**

3. **D-03 — Medium — An expired approval suppresses the fresh approval needed to replace it.**

   **File:** [proposing.ts:842](metasystem/internal/ui/web/_app/src/partner/proposing.ts:842).

   **Evidence, ran:** An `applying` approval with unchanged displayed fields and budget, but a **relayed, expired** approval on the row, passes the guard. HEAD writes `applied` with zero sends; the base sends the approval. `carriesAlready` checks existence and tuple equality but ignores `approved.expired`. The Go no-op explicitly [requires an unexpired approval](metasystem/internal/goal/approval.go:629).

   **Concrete failure:** An interrupted attempt to replace an expired approval is retried. The proposal becomes applied while the goal retains the expired approval and remains inadmissible for work.

   **Smallest fix:** Exclude expired approvals from the shortcut and test retry against an expired relayed approval.

   **Material: yes — WORK fails.**

The departures are adjudicated as follows:

- **Go departure 1:** The session-specific no-op wording and omission of a second proof are appropriate for a genuine duplicate approval; they do not close A-01’s concurrent-edit case.
- **Go departure 2:** The pushed-entry gate and extracted `uiAdvance` are appropriate; invoking recovery without coordinating active publications is not, per D-01. Refusing unsupported breach-stop replay preserves the authority boundary.
- **Go departure 3:** The shared seat constant, transfer semantics, fixture extraction, and existing-human test are appropriate. Moving messages alone is insufficient, and silently discarding adoption failure leaves A-03 unresolved; preserve sign-in while making transfer failure recoverable and visible.
- **Go departure 4:** Shared bounds, ledger label validation, bounded refusal IDs, and increased decoding capacity are appropriate. Eight times the text bound is not a guaranteed bound on a complete encoded message.
- **Go departure 5:** Combining B-01 and B-02 is justified by their shared argument-decoding changes.
- **Go departure 6:** Adding the required test main and correcting exemption prose is appropriate; the exemption itself was not widened.
- **Frontend C-05:** Applying the same preservation rule across six panes is justified; the backlog prop and source-helper extraction fit that correction.
- **Frontend C-02:** Reading the existing Partner store through the hook is appropriate, but shared outcome ownership must include clearing, as C-04 demonstrates.
- **Frontend A-01:** Omitting open and withdraw from effect inference avoids inventing success; their repeat refusals are acceptable. Effect inspection alone does not provide exclusive ownership of an in-flight act.

The left items are adjudicated individually:

- **Document decoder:** Already fixed at HEAD; the new trailing-JSON and whitespace tests cover it.
- **Unbounded proposal count:** **Record residual**; aggregate overflow remains possible, but no ordinary first-use-sized answer was shown to reach it. Do not describe the reader ceiling as the writer’s guaranteed bound.
- **Previously oversized transcripts:** **Record residual**; old files beyond the new ceiling still need recovery, but no affected user transcript was established here.
- **Unnamed-seat sitting:** **Fix now under A-03**; the sitting can be opened before sign-in, and moving its messages without its state breaks continued sitting work.
- **Turn in flight during sign-in:** **Fix now under A-03**; the header permits this ordinary sequence, and the answer is subsequently saved to the wrong conversation.
- **Walkthrough transitive cycle:** **Record residual**; production validation remains intact, and this extends fixture fidelity beyond the demonstrated canned-path defect.
- **Tool prose about the card under the answer:** **Record residual/no change needed**; the sentence is descriptive, not exclusive.
- **Frontend halves delegated elsewhere:** Assessed in the table; A-01 and C-04 remain open.
- **C-06/C-07 source-reading guards:** **Record residual**; they are weaker than driven provider tests, but the inspected callbacks implement the requested behavior.
- **C-03 store wiring without a behavioral test:** **Record residual**; the folding behavior is tested and the small wiring is directly inspectable.

VERDICT: 3 not closed: A-01, A-03, C-04; 3 new material: D-01, D-02, D-03; 0 new non-material: none

