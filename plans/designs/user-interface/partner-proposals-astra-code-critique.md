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


---

# The second confirmation read (the second fix pass)

Produced 2026-09-28 by Codex on `gpt-6-astra`, read-only, against ui-development at `4e7433f14` (both second-pass branches merged; the three rulings excluded as the third pass's). Verbatim.

| Finding | Confirmation | Holding test and evidence |
|---|---|---|
| A-01 | **Closed** | `TestASecondPressOnAnActAlreadyRunningIsRefusedBeforeItReadsAnything`, `TestTheHoldIsOneGoalsOneActAndEndsWhenItAnswers`, `TestAnIdenticalSignedInSessionEditIsANoOp`, and the four repeated-act tests hold exclusion and no duplicate edit publication. The page’s new concurrency failures are E-01 and E-02 below. |
| A-03 | **Not closed** | `TestTheSeatsSittingMovesWithItsMessages` and `TestASignInDuringAnAnswerLandsItInTheHumansConversation` cover an established turn. **Read:** [submit](metasystem/internal/ui/partner/service.go:526) captures the seat conversation, then waits in `host.Ready` before installing `s.current` at line 572. Sign-in during that startup lets [Adopt](metasystem/internal/ui/partner/service.go:297) finish without rebinding the pending question. Startup then binds the turn to the emptied seat; its question, answer and proposals remain there. The header permits this first-use sequence. **WORK fails.** Reserve an adoptable turn before startup, releasing it on startup failure; hold the regression with a channel-gated startup test. |
| C-04 | **Closed** | `says nothing of an answer older than the entry the record now holds` covers the real inbox hook; the shared-clearing guards cover both callbacks. The in-memory probe displayed `applied`, with no retry, over the obsolete refusal. Premature expiration is separately recorded as E-03. |
| D-01 | **Closed** | `TestARecoveryCannotTakeTheEntryOfAPublicationStillRunning` and `TestARecoveryClassifiesUnderTheLockAPublicationTakes` cover both sides of the shared lock and preservation of the authority proof. Publication and recovery use the same root owner. Ordinary snapshot reads do not acquire this lock. |
| D-02 | **Closed** | `settles nothing from a read that failed, and sends nothing` and its already-unresolved counterpart hold the correction. The production-runner probe wrote only `unresolved` and sent nothing when the cached row was parked but fetching failed. |
| D-03 | **Closed** | `sends the approval where the one the goal carries has expired` and the expired case in `reads an approval as carried only with the tuple the card displayed` hold the correction. The production-runner probe sent the replacement approval. |

Reviewed clean HEAD `4e7433f14bd2db7c9910749c66fadfa5446ee7c4` and the specified diff, with the brief’s exclusions. No files were edited. Named Go and Vitest tests were **read**; both focused runners stopped before executing tests because the sandbox denied temporary directories. TypeScript probes **ran in memory against production functions with substituted I/O**; the concurrency probe used the transition table extracted from the Go source. They did not exercise HTTP or a mounted browser.

1. **E-01 — High — An in-flight edit lets the following approval use an obsolete reading.**

   **File:** [proposing.ts:717](metasystem/internal/ui/web/_app/src/partner/proposing.ts:717), with invalidation at [line 1393](metasystem/internal/ui/web/_app/src/partner/proposing.ts:1393).

   **Evidence, ran:** The inbox retries an applying edit alongside a waiting approval of the same goal. Its canonical read completes before the first tab begins publishing. The edit request receives `in-flight`; the runner records `unresolved` and continues without invalidating its reading. The first tab’s edit then lands.

   The probe observed **one read**, followed by an approval carrying reviewed intent `"old intent"` while the goal’s actual intent was `"new intent"`. The publication lock serializes the acts but does not rerun the page’s comparison.

   **Smallest fix:** Stop the batch on `in-flight`, preserving the remaining lines for Continue. Alternatively, require reconciliation and a fresh comparison before proceeding.

   **Material: yes — SAFE fails.**

2. **E-02 — High — A delayed refusal can defeat a newer successful attempt.**

   **File:** [proposing.ts:1366](metasystem/internal/ui/web/_app/src/partner/proposing.ts:1366).

   **Evidence, ran:** Tab A’s act finishes refused, releasing its server hold, but its outcome write is delayed. Tab B retries the applying entry and starts an act that will succeed. A rebases its older refusal onto B’s newer applying version:

   `A:applying@1 → B:applying@2 → A:refused@2(conflict) → A:refused@3 → B:applied@3(conflict)`

   The probe ended with **the act applied but the entry `refused@4`**. B reconciles to that settled refusal and discards its successful answer. A newer unsettled version does not establish that its writer holds no independent act.

   **Smallest fix:** Do not rebase an older refusal over a newer attempt merely because its entry is unsettled. Preserve the newer attempt’s ownership; retry settlement only when the newer entry belongs to the same executing attempt.

   **Material: yes — WORK fails.**

3. **E-03 — Low — An unsettled version expires a known result too early.**

   **File:** [proposing.ts:528](metasystem/internal/ui/web/_app/src/partner/proposing.ts:528).

   **Evidence, ran:** An unrecorded `applied` result at version 2 was hidden by `unresolved@4` carrying another tab’s in-flight sentence. The probe displayed unresolved and offered Try again, including when the held result explained that publication succeeded but authority-proof recording failed.

   **Concrete failure:** Another tab’s bookkeeping replaces the only known outcome and hides its explanation.

   **Smallest fix:** Retire a held result when a later definitive outcome supersedes it, rather than on every version increment.

   **Material: no — record.** The demonstrated result can still be reconciled by read; duplicate publication was not demonstrated under the new engine protections.

4. **E-04 — Low — A panic during request assembly strands the publication lock.**

   **File:** [act.go:688](metasystem/internal/ui/act/act.go:688).

   **Evidence, read:** `request` registers the act and acquires `publications`, then calls `assemble` without deferred cleanup. The caller installs `defer done()` only after `request` returns. A panic inside an assembly reader therefore leaves both the registration and lock held.

   **Concrete failure:** If that panic occurs, HTTP recovery cannot release the owner; subsequent acts and explicit reconciliation remain blocked until restart.

   **Smallest fix:** Defer cleanup within `request` until ownership has successfully transferred to its caller.

   **Material: no — record.** No production panic trigger was established; ordinary error returns release correctly.

5. **E-05 — Low — Equivalent goal spellings bypass immediate in-flight refusal.**

   **Files:** [owner.go:77](metasystem/internal/ui/act/owner.go:77), [verbs.go:2903](metasystem/internal/goal/verbs.go:2903).

   **Evidence, read:** The registry keys the raw ID, while Block and Unblock trim it before publication. Requests naming `g` and ` g ` obtain different registrations but address the same ledger goal.

   **Concrete failure:** The second equivalent request queues and executes instead of immediately answering `in-flight`.

   **Smallest fix:** Normalize IDs consistently before registration and dispatch.

   **Material: no — record.** Normal browser proposals use canonical IDs; serialization and engine no-ops still protect the demonstrated repeat.

The builders’ left items are adjudicated individually:

- **Sign-in `trouble` field and rendering — record:** completed at HEAD; the header renders the server’s words, with a rendering test.
- **`s.spokeLast` retaining the seat — record:** causes a fresh runtime session with retained history; no demonstrated loss.
- **ACP session ID not moving — record:** currently advisory state, rewritten on the next turn.
- **No route test for sign-in during an answer — record:** existing service evidence covers established turns; **fix now** for A-03’s uncovered startup interval.
- **Unpruned owner map — record:** one root in the production server; no demonstrated operational growth.
- **Repeated park, unpark and abandon — record:** reserved for the separate R-129-ui review, excluded here.
- **Unbounded proposal count — record:** reserved for R-130-ui, excluded here.
- **Previously unreadable transcripts — record:** reserved for R-131-ui, excluded here.
- **Walkthrough transitive cycle — record:** production validation remains intact.
- **Tool prose about the card — record:** descriptive wording does not exclude Decisions.
- **Frontend halves delegated elsewhere — fix now:** E-01 and E-02 remain; C-04’s reported failure is closed.
- **Go transition-table half of result retry — record:** both new pairs are present and covered by the allowed-pair and racing-writer tests.
- **Approval authority absent from `carriesAlready` — record:** the unexpired approval remains usable; replacing its authority is beyond the demonstrated expiry failure.
- **Shared-clearing tests using source guards — record:** weaker than mounted interaction tests, but the callbacks were inspected.
- **Duplicated TypeScript transition table — record:** it is already stale; Go tests cover the added pairs, and this review’s probe used the actual Go table.

VERDICT: 1 not closed: A-03; 2 new material: E-01, E-02; 3 new non-material: E-03, E-04, E-05

