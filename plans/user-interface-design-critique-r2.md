# User interface design: second independent review

Reviewer: Opus 5, fresh context, not the author, no part in round one. Date: 2026-09-21. Subject: [user-interface-design.md](user-interface-design.md) at commit `cb08ac9b5`, scoped to the amendments made after [round one](user-interface-design-critique.md) and its [dispositions](user-interface-design-critique-dispositions.md), because delivery gates 2 and 3 are built on them. The reviewer read source only and executed nothing.

This file records the findings and **proposes** dispositions. It does not adjudicate: the master is the owner's document, and the rulings are the human's. Claude, who recorded this and planned the implementation, did not write the master and has not edited it for these findings.

**Rulings, 2026-09-21.** The human accepted S3 to S10 as proposed ("for the rest I follow your recommendations"). S1 and S2 went to the [Astra design review](user-interface-design-critique-astra.md), which agreed with the planner's proposals with changes and corrected S10. The human then instructed the planner to apply everything it agrees with, to list what it does not, and to give the reason each time; that is the [last section](#dispositions-after-the-astra-review). The master was amended in one pass on the same day.

Verdict: 10 material findings. None blocks gate 1. All ten bear on gates 2 to 7, and S1, S2, S3, S5, and S10 must be ruled before gate 2 or gate 3 can be designed.

## Standing of the round-one findings

| Finding | Status | What is missing |
| --- | --- | --- |
| UID-R1-01 | Partly resolved | What a browser-derived human principal is to the authority owner (S2) |
| UID-R1-02 | Partly resolved | The proof shape and grade a signed-in session produces (S2) |
| UID-R1-03 | Partly resolved | The clone collides with the declared brain checkout (S1); the retry rule is wrong against the journal (S3); "all writes" is only the goal ledger (S4) |
| UID-R1-04 | Partly resolved | No owner or transaction publishes `records/fleet-control/` (S4) |
| UID-R1-05 | Resolved | |
| UID-R1-06 | Partly resolved | "Reversible" is undefined (S5); the guided flow's basis is undecided (S6) |
| UID-R1-07 | Resolved | |
| UID-R1-08 | Partly resolved | Whether state a compound flow changes itself belongs to its basis (S6) |
| UID-R1-09 | Resolved | |
| UID-R1-10 | Partly resolved | Which checkout is the brain's contradicts the clone amendment (S1) |
| UID-R1-11 | Partly resolved | Its own proof obligation contradicts the storage split (S8) |
| UID-R1-12 | Resolved | |
| UID-R1-13 | Partly resolved | Gate 3 depends on a gate 5 deliverable (S7) |
| UID-R1-14 | Resolved | |
| UID-R1-15 | Partly resolved | `propose`'s persistence has no owner or transaction (S4, S7) |

## Findings

Code references are relative to `metasystem/`. "Checked" means Claude read the cited source after the review and found it as stated. After a finding on the first slice design was accepted on half a chain and had to be reversed, nothing here is marked checked on the reviewer's word alone.

### S1. The server's clone and the declared brain checkout are two checkouts, and the design requires both

"The brain" says the interactive session occupies the existing declared brain checkout. "Clone, publication, and freshness" says to use a dedicated, server-owned, quiescent brain checkout. `brain.Declare` allows one brain per host per ledger and refuses a second (`internal/brain/brain.go:283`, checked). So the brain is either the human's existing checkout or the server's new clone, never both, and the design does not say which, who declares the clone, or that the declaration must move. `brain declare` is a human-only terminal verb, which would make a terminal command a precondition for the browser brain. One implementer makes the clone the brain and silently breaks the terminal brain; another points the server at the existing checkout and breaks "quiescent" and "not a delivery worktree". Whether "terminal and browser occupants exclude each other" is enforceable at all depends on the answer. Blocks gates 2 and 3.

Resolved, superseding the first proposal. The human's premise: every fleet has a brain whether or not the interface runs, and the interface is optional, so the brain cannot live inside it. One brain seat per fleet at one canonical brain home; the brain's live session is hosted by its own process there, which holds the lease, the agent session, the application tools, the sitting store, and the budgets; the interface is a window that owns browser sign-in and connects its dock to the brain. There is no server-owned clone: the interface serves the checkout it is started in. Amended by the Astra review: fleet-wide uniqueness is a contract to add, because the code enforces only one brain per host (A5); hosted startup must verify it really holds the lease; the tool endpoint lives in the brain process; sign-in and the human's session stay in the interface, so not all of gate 3 moves; and the second process makes gate 2's publication boundary cross-process (A4). Open for the human: whether a brain must also act unattended.

### S2. A signed-in session has no decided proof shape or grade, and the engine's human gates branch on both

The design decides that sign-in grants human authority, but not what the authority owner receives. The engine has two grades and three predicates: `ValidFor` requires the enrolled grade, `TerminalValidFor` accepts either, `EnrolledTerminalFor` also excludes the fixture equivalent (`internal/humanauthority/authority.go:196` to `:215`, checked), and a valid proof requires an observed ancestry chain and a terminal, which a browser session lacks. Verbs consume the distinction directly: approve, abandon, order, and split require `ValidFor`; session stop accepts the terminal grade; the norm path requires `EnrolledTerminalFor` (`internal/goal/abandon.go:54`, `order.go:63`, `split.go:393`, `sessionstop.go:258`, `norm.go:122`, `verbs.go:313` to `:319`, checked). Two implementers produce different outcomes for the same operation. Blocks gate 3 and every human verb after it.

Resolved, superseding the first proposal, which the human rejected: certain acts need a human, and enrollment is one mechanism that establishes that, not the requirement. A server-verified session has full human standing as one more source of a verified human word. No new grade, no per-verb table. Amended by the Astra review: the planner's claim that the change is confined to `internal/humanauthority` was wrong. Admission can stay as it is, but carry, session stop, and human handoff cancellation consume terminal facts downstream and must be extended to session evidence (A2), and recording the session in History needs a compatible extension and a reader rollout (A3).

### S3. "A retry uses the same identity" is refused by the journal after every non-confirmed terminal outcome

`Publish` returns idempotent success for an operation id it has already journaled only when that entry is terminal and confirmed or confirmed-late. For any other existing entry it refuses: "the recovery rule owns it, not a second publish" (`internal/goal/txn.go:563` to `:590`, checked). After `lost`, for which the design's own table offers recovery, resubmitting the same request needs a new operation id, and the design's two cases, same request same id and changed request new id, leave no room for that. An adapter either reuses the id and is refused forever, or mints a new id for a confirmed operation and duplicates it. Blocks gates 2 and 4.

Proposed: accept. The master already separates request identity from operation identity in its results table; use it. The request identity is stable across attempts. The operation id belongs to one attempt. A fresh attempt after a terminal non-confirmed outcome takes a new operation id linked to the original request.

### S4. "Blocks all further writes through this clone" is true only of the goal ledger, and the amendments add two more writers

The block is `PushedBlocking`, checked inside `goal.Publish`; it gates goal-ledger publications only. The amendments add `propose`, which persists working material into `plans/`, and `records/fleet-control/` requests. No fleet package exists under `internal/` (checked against the package list), and the goal journal is the only transactional publisher. Undecided: plain commits, a second journalled owner, or worktree-only. Each answer changes what the block covers, whether working material replicates, and whether the clone stays quiescent. Blocks gates 3, 5, and 6.

Proposed: accept, with S7. In gate 3 `propose` writes to the server-local sitting store only. Gate 5 delivers the shared working-material owner, and gate 6 a journalled owner for fleet control. The interface states that the pushed-unknown block covers ledger writes.

### S5. The direct-performance restriction rests on a "reversible" predicate that no owner can compute

Direct performance is allowed for an "owner-authorized, reversible" edit that "the same actor must be permitted to reverse". The effect clauses can be computed from a preview; reversibility cannot. The goal owner has no inverse operation, and the inverse's permission would need an allowed-actions query against a future state. One implementer hard-codes an allowlist and another builds an evaluator, giving the brain different unattended powers. That is a trust boundary. It also silently includes draft edits, whose owner does not exist before gate 5. Blocks gates 3 and 4.

Proposed: accept. Replace "reversible" with an enumerated allowlist the owner can evaluate: a queued goal that is not approved, claimed, or parked, and only the fields `Intent`, `NextStep`, and `Labels`. `Blocked` changes other goals' readiness, and `Tier` and `Risk` enter the approval digest, so they are excluded. The human rules on the list.

### S6. The guided revision of an approved definition destroys, in step two, the precondition that step three's basis must check

The flow prepares once, then runs withdraw, edit, approve under separate operation ids. The preparation rule requires the basis, permissions, and preconditions to be rechecked inside each transaction. The intent edit depends on exactly the state the withdrawal removes: it is refused while the goal is approved (`internal/goal/verbs.go:2876` to `:2880`, as the round-one dispositions also cite), and withdrawal changes the revision and clears the approval digest. Either the approval state is in the edit's basis, and the flow's own second step makes the third stale, or it is not, and a concurrent re-approval by someone else goes unnoticed. The design does not choose. Blocks gate 5 and touches gate 4.

Proposed: accept. A compound flow's basis is recomputed per step against the state its earlier steps produced, and the flow is pinned to the proposal, not to the snapshot taken before it began.

### S7. Gate 3 requires the working-material owner that gate 5 delivers

Gate 3's tools include `propose`, which persists working material, and its sitting contract requires working records and a fresh remote brain resuming from that material alone. The owner arrives at gate 5, and the design itself says no writer exists. Gate 3 would ship a `propose` that writes nowhere, or improvise a second store that gate 5 must migrate. Blocks gate 3.

Proposed: accept, with S4. The simple case first: gate 3 keeps working material in the server-local store and drops resume on another machine; gate 5 adds the shared owner and the migration. UID-R1-11's cross-machine clause moves to gate 5.

### S8. The UID-R1-11 proof obligation forbids what the storage amendment requires

The amendment puts proposals and active notes in `plans/` to carry continuity to another machine and says shared notes do not automatically become examiner input. The obligation demands that an examiner cannot read exploratory notes through any granted root or tool. An examiner's checkout contains `plans/`, which also holds the adopted designs it must read. The test cannot pass, or examination is starved. Blocks gates 3 and 5.

Proposed: accept. Restrict the obligation to the private transcript store. Shared notes in `plans/` are permitted input that is never attached to an examiner's brief.

### S9. The brain fence the design relies on is inert on an undeclared checkout

The design says the server's checkout cannot dispatch, claim, cancel, reap, close, or land by bypassing the brain fence. `brain.Fence` returns no refusal when the checkout is undeclared (`internal/brain/brain.go:429` to `:434`, checked). The design lets the server run undeclared, and gates 1 and 2 precede any declaration. In that state the clone is indistinguishable from a delivery checkout. Blocks gates 6 and 7.

Proposed: accept. The server adapter refuses those operation families unconditionally, whatever the declaration state.

### S10. The promised recovery action can do nothing, and the states that make it so are missing from the outcome table

The table lists terminal outcomes and pushed-but-unknown. `ClassifyRecovery` also returns `leave-to-owner`, `keep-retrying`, `abandon-own`, `expire-own`, and `complete` (`internal/goal/journal.go:539` to `:547`, checked). While the owning process is alive and is not the caller, a second server or a terminal command on the same clone, the human's recovery changes nothing. The first version of this finding said the clone stays blocked "until that owner's deadline"; that was wrong (A7). `ClassifyRecovery` returns `leave-to-owner` for another live owner's entry before it looks at the deadline, and only the owner itself reaches `expire-own`, so the entry stays blocked until evidence resolves it or the owner finishes or ends. A Recover button would report success under a banner that does not clear. Blocks gate 2.

Proposed: accept. Add the non-terminal dispositions to the table, each with what the human can and cannot do.

## Non-material

- `records/fleet-control/` is said to extend a record owner that does not exist; gate 6 already names it as new work.
- The board's lane table defines lanes from a recorded phase that gate 6 says is not recorded; gate 1 already states the gap.
- The brain principal is bound to a session announcement "and brain role", but announcements carry no role; the role comes from the declaration. Settled by S1.
- Adding `DefinitionRefs` to split members changes the ratification digest; the design already names serialization as part of the extension.

## Not checked

Provider-side ACP capabilities, which the design defers to a named investigation. Nothing was exercised at run time. Of the ten findings, the code behind S1, S2, S3, S9, and S10 was re-read by Claude; S4 to S8 turn on the design's own text, which matches Claude's reading of it.

The master's status line still says the amendments have had no second review. It should be updated when these findings are ruled.

## Dispositions after the Astra review

The human's instruction, 2026-09-21: apply everything the planner agrees with, list everything it does not, and always state the reason. Every code claim below was re-read in source by the planner along its whole chain, from first cause to claimed effect, before the finding was accepted.

| Finding | Disposition | Reason | Applied in |
| --- | --- | --- | --- |
| Part 1, `g1-s1`: implement as written | Agree | Nothing to change; it matches three critique rounds | No edit. Building still waits for the human's explicit go |
| A1. The health read the plan named changes engine behaviour | Agree | `ObserveHealth` takes a write lock, advances the observation, and saves it (`internal/steward/health.go:285` to `:329`); a dead role increments a failure count (`:693` to `:699`) that `AutoHealingEnded` reads (`:1738`) and that makes runner repair return without repairing (`runner.go:554`). Polling it from a browser would advance the engine's repair breaker. The planner's sweep had listed it as a plain read without opening its body. The existing `PreviewHealth` does not advance the breaker but re-probes every role on each call; the retained verdict with its own observation time is cheaper and says honestly when it was observed, so Astra's direction is taken | Plan: gate 1 facts and `g1-s6`. Master: freshness paragraph |
| A2. Admission is only half the authority contract | Agree; the planner's claim was wrong | `Carry` admits through `ValidFor` and records the terminal generation (`internal/goal/verbs.go:4512`, `:4626`); landing refuses a carry word whose generation is not positive (`internal/landing/carried.go:273`). Session stop copies `proof.InvokerRef` and later requires that process alive (`sessionstop.go:264`, `:506`); handoff cancellation does the same (`internal/steward/handoff_capture.go:1236`). A session proof with invented process facts would equate the interface server's lifetime with a human's presence | Master: human acts. Plan: gates 2 and 3 |
| A3. Session provenance in History needs more than the proof package | Agree | The History parser returns an error on an unknown key (`internal/goal/file.go:2089`), approval authority values are enumerated (`:855`), and `FetchAdvance` validates the whole tree before it moves the accepted ref (`fetchadvance.go:64`). A new key published early strands every older reader. Added by the planner: on a single-machine fleet the rollout is the one engine build, which keeps the first case simple | Master: human acts, gate 2. Plan: `g2-s8` |
| A4. Two processes need one publication admission boundary | Agree; the planner's claim that gate 2 is unaffected was wrong | `Publish` checks `PushedBlocking` only at admission (`internal/goal/txn.go:553`), no lock spans admission to outcome, and `CreateEntry` and `MarkPushed` lock one journal update each (`journal.go:231`, `:370`). Sharpened by the planner: the lock belongs inside the publication owner itself, not in an adapter-level owner, because only that choke point also covers the CLI and the steward | Master: clone and publication, gate 2. Plan: `g2-s3` |
| A5. "One brain seat per fleet" is wider than what the code enforces | Agree | The registry home is the local user's home (`internal/brain/brain.go:228`) and `Declare` enforces one brain per host per fleet (`:283`); two hosts can each declare | Master: the brain. Plan: gate 3 |
| A6. A tip change is not enough to invalidate the read models | Agree | Questions are local files (`internal/channel/question.go:80`), and `Project` computes an approval horizon from the time it is given (`internal/goal/project.go:92`), which `ApprovalExpired` uses (`file.go:290`). A projection cached by tip can show yesterday's authority state, and tip-only events leave Decisions and Fleet unchanged | Master: freshness. Plan: `g1-s2`, `g1-s3`, `g1-s5` to `g1-s7` |
| A7. S10's deadline wording was wrong | Agree | `ClassifyRecovery` returns `leave-to-owner` before it considers the deadline (`internal/goal/journal.go:573` to `:583`) | This file, S10. Master: outcome table |
| A8. Two coverage assignments could certify unbuilt behaviour | Agree | UID-R1-05 requires a withdrawal preview, and withdrawal arrives at gate 5; scenario 1 includes approving through controls, which also arrives at gate 5. No code claim involved | Plan: coverage, with a rule separating foundation evidence from final discharge |
| S1 alternative: the brain process is the single MAIN occupant with provider sessions beneath it; the tool endpoint lives there; both processes call the shared owners; sign-in stays with the interface; gate 3 delivers the brain service first, then the browser connection | Agree; the planner's "gate 3's content moves into the brain process" was too sweeping | The claim path verified: a live competing holder keeps the lease while the claim returns without error (`internal/lease/claim.go:117` to `:122`), so startup must check that it holds the lease. Routing human acts through the brain process instead would put human credentials next to the agent, so two callers of shared owners is right | Master: the brain, components, gate 3. Plan: gate 3 |
| S2 alternative: standing separate from evidence source; extend the few owners that consume terminal evidence; terminal enrollment stays a terminal act | Agree | Follows from A2 and A3, and it keeps the human's principle: a human can carry or stop from the browser, recorded with honest evidence, without manufactured process facts | Master: human acts |

**Not agreed with: nothing.** That is not a courtesy. Each finding was checked in source before it was accepted, after an earlier round on the slice design had been accepted on half a chain and reversed. Three points go beyond Astra's text and are the planner's own: the choice of the retained verdict over `PreviewHealth` (A1), the placement of the operation lock inside the publication owner (A4), and the single-machine simplification of the reader rollout (A3).

**Still open for the human.** Whether "every fleet must have a brain" also means a brain that is alive and acts on fleet events with nobody present. Astra and the planner both assumed an explicitly started occupant. Unattended turns would need a wake-up source and a budget model of their own.

