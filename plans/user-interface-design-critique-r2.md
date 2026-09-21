# User interface design: second independent review

Reviewer: Opus 5, fresh context, not the author, no part in round one. Date: 2026-09-21. Subject: [user-interface-design.md](user-interface-design.md) at commit `cb08ac9b5`, scoped to the amendments made after [round one](user-interface-design-critique.md) and its [dispositions](user-interface-design-critique-dispositions.md), because delivery gates 2 and 3 are built on them. The reviewer read source only and executed nothing.

This file records the findings and **proposes** dispositions. It does not adjudicate: the master is the owner's document, and the rulings are the human's. Claude, who recorded this and planned the implementation, did not write the master and has not edited it for these findings.

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

Proposed: accept. The server's dedicated clone is the declared brain checkout. A host that already has a terminal brain for the ledger withdraws it first, as the existing rule requires, and declaration gains a browser path when human authority arrives at gate 3. Until then the server runs undeclared and read-only. This changes how the human works with a terminal brain, so it needs the human's ruling. For gate 1 the simple case applies: the server serves the checkout it is started in, and the dedicated clone waits for this ruling.

### S2. A signed-in session has no decided proof shape or grade, and the engine's human gates branch on both

The design decides that sign-in grants human authority, but not what the authority owner receives. The engine has two grades and three predicates: `ValidFor` requires the enrolled grade, `TerminalValidFor` accepts either, `EnrolledTerminalFor` also excludes the fixture equivalent (`internal/humanauthority/authority.go:196` to `:215`, checked), and a valid proof requires an observed ancestry chain and a terminal, which a browser session lacks. Verbs consume the distinction directly: approve, abandon, order, and split require `ValidFor`; session stop accepts the terminal grade; the norm path requires `EnrolledTerminalFor` (`internal/goal/abandon.go:54`, `order.go:63`, `split.go:393`, `sessionstop.go:258`, `norm.go:122`, `verbs.go:313` to `:319`, checked). Two implementers produce different outcomes for the same operation. Blocks gate 3 and every human verb after it.

Proposed: accept. Name a third, server-validated grade. Following the human's decision that sign-in grants human authority, it satisfies every predicate an enrolled terminal satisfies, is never fixture-only, and is recorded distinctly in history. The human rules on whether any path stays terminal-only.

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

The table lists terminal outcomes and pushed-but-unknown. `ClassifyRecovery` also returns `leave-to-owner`, `keep-retrying`, `abandon-own`, `expire-own`, and `complete` (`internal/goal/journal.go:539` to `:547`, checked). While the owning process is alive and is not the caller, a second server or a terminal command on the same clone, the human's recovery changes nothing and the clone stays blocked until that owner's deadline. A Recover button would report success under a banner that does not clear. Blocks gate 2.

Proposed: accept. Add the non-terminal dispositions to the table, each with what the human can and cannot do.

## Non-material

- `records/fleet-control/` is said to extend a record owner that does not exist; gate 6 already names it as new work.
- The board's lane table defines lanes from a recorded phase that gate 6 says is not recorded; gate 1 already states the gap.
- The brain principal is bound to a session announcement "and brain role", but announcements carry no role; the role comes from the declaration. Settled by S1.
- Adding `DefinitionRefs` to split members changes the ratification digest; the design already names serialization as part of the extension.

## Not checked

Provider-side ACP capabilities, which the design defers to a named investigation. Nothing was exercised at run time. Of the ten findings, the code behind S1, S2, S3, S9, and S10 was re-read by Claude; S4 to S8 turn on the design's own text, which matches Claude's reading of it.

The master's status line still says the amendments have had no second review. It should be updated when these findings are ruled.
