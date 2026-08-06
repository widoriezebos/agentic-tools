# The Collaboration Loop: how orchestrator and delegate actually work together

- Goal and current status: the loop that inspired this metasystem — orchestrator designs, delegate critiques to agreement, delegate implements, orchestrator critiques to agreement — stated once as canon, enforced where it can be, and honest where it cannot. Status: CLOSED 2026-08-06 — round 3 returned zero material findings after 17 were folded across rounds 1-2. Chain design-critic-20260806t202601z-4f2f.
- Next step: implement, through the loop this design itself prescribes: delegate implements, a code-critic on a different model reviews to a zero-material round over the merged tree.
- In flight right now: both gates running over the closed design (quiet machine); implementation brief written at the scratchpad, dispatches the moment the gates return — sequenced behind them because a sol delegate at xhigh and the full suite on one laptop is the exact load contention that flaked arming twice today
- Waiting on the human: nothing — implementation authorized under the standing instruction to fix all findings

## Why this exists

The human states the intent: *the coordinator designs plans and orchestrates;
the delegate critiques the design in a loop until both agree; then
implementation happens; then the orchestrator critiques the implementation
until both agree it is finished.* That loop is the reason this metasystem
exists at all. Benchmark trial 002 then produced excellent software with zero
delegation and was scored invalid, which exposed that the intent lived in
practice and in nobody's instructions.

Fixing the coordinator instruction was necessary and insufficient. A critical
look at how the loop actually runs here found three more gaps, and one of
them is a lie in our own records.

## What is true today

**The design half is real and enforced.** The orchestrator writes a design; a
`design-critic` delegate returns findings with stable ids and materiality;
the orchestrator dispositions every finding; `assert-critique-closed.sh`
joins findings against dispositions mechanically; the loop closes only at
zero material findings. Two chains this session ran eight and twelve rounds
and closed properly. Nothing here needs changing.

**The implementation half is not.** Three findings:

- **F-1: no closure gate.** The design half closes by a mechanical join. The
  implementation half closes when the orchestrator decides it looks right and
  the suite is green. A green suite proves no regression; it does not prove
  the change is correct, complete, or free of the class of defect a reviewer
  would catch. There is no artifact that says "both parties agree this
  implementation is finished".
- **F-2: the second pair of eyes never arrives.** A `code-critic` role, a
  code-critique skill with a materiality criterion and a two-layer method,
  and a return schema all ship. In this repository's entire history, zero
  code-critic chains have been dispatched. Every implementation this session
  was reviewed by the same agent that wrote its brief and merged its result.
- **F-3: receipts claim a critique that did not happen.** Eight receipts
  carry `skills=code-critique` while no code-critic ran. The orchestrator
  reading a diff is review; it is not the two-party critique the field names.
  A record that overstates its own rigour is worse than one that admits the
  gap, because retros read these.

## Roles are configured, never named in the rules

Nothing in this loop names a vendor. The parties are ROLES — orchestrator,
design-critic, implementer, code-critic — and a project's own configuration
decides which agent fills each. One project may run the orchestrator on one
model and every critic on another; a second may invert it; a third may use a
single agent in different roles and accept the weaker independence. The rules
below therefore say "a delegate in the design-critic role", never a product
name, and any rule that would only make sense for one vendor belongs in that
vendor's adapter, not here.

The single constraint the loop does impose is independence — and it is
stated as what the evidence can actually verify, because an unenforceable
must-not is worse than an honest boundary (CL-1-6). The checkable rule: the
code-critic job's `effectiveModel` must differ from the implementer job's,
read from the two job records at merge time. Model is the property that
matters — the same model behind a different transport still grades its own
blind spots, so runtime difference counts for nothing (CL-2-3). Every
delegate dispatch is already a fresh session with no shared context. On
failure the behaviour is defined, not ambiguous: the merge is refused, the
refusal names both models and both job ids, and it states the two remedies —
dispatch a critic on a different model, or declare
`independence=session-only` in configuration, which the gate then accepts
and records in evidence. A project that declares it gets honesty in its
record, not a pretense of independence.

## The loop, stated once as canon

For each substantial piece of work:

1. **Design.** The orchestrator writes it. Small, obvious changes skip to 3
   and say so.
2. **Design critique.** A delegate critiques; the orchestrator dispositions
   every finding; rounds continue until zero material findings, joined
   mechanically. This is agreement, not exhaustion.
3. **Implementation.** A delegate implements against the closed design. The
   orchestrator does not write the product itself; a piece too small to
   delegate is named as such with a reason.
4. **Implementation critique.** A delegate in the code-critic role — never
   the same agent instance that implemented it, per the independence property
   above — critiques the result. The orchestrator dispositions every finding.
   Rounds continue until zero material findings, joined by the same script.
5. **Gate and merge.** The orchestrator runs the gate of record and merges.
   The gate is a floor, not the agreement.

Steps 2 and 4 are the same mechanism applied to different artifacts, and both
close the same way.

## Changes

**C-1: implementation critique becomes a required step with a real gate,
bound to the bytes it reviewed.** After a delegate's implementation returns
and before the orchestrator merges, a `code-critic` delegate reviews the diff
against the brief and the design. The review object is exact and canonically
hashed: `reviewedTree` is the hash `git write-tree` produces over a temporary
index holding the worktree's full contents at review time — defined for
uncommitted work, independent of commit metadata. The merge gate verifies
three things mechanically: the chain names the implementer job being merged;
the tree of the implementer branch's final commit equals `reviewedTree` (the
comparison is against the branch being merged, never against the post-merge
result, which legitimately differs when the target has moved); and the final
round reports zero material findings. When the merge target has moved since
review, the branch is rebased and the critic confirms over the new tree —
the existing WORKTREE-BEHIND warning already names this case (CL-2-1). A join that balances
findings against dispositions is bookkeeping; the zero-material final round
over the exact merged bytes is the agreement (CL-1-1, CL-1-2).

**C-1a: both critique loops are bounded, and exhaustion has an executable
successor, symmetrically.** Design critique and implementation critique
follow the shipped round budgets of their skills — the same rule for both,
because the design loop's own exit was still unbounded after round 1
(CL-2-5). When a budget runs out with material findings open, the chain
records `critiqueExhausted` with the open finding ids; the merge (or design
closure) stays refused while that list is nonempty. The successor is a
contract, not a hope (CL-2-4): the next implementer (or design) follow-up
round's brief must enumerate every open finding id — conformance checks the
enumeration — and folding them reopens critique on the same chain with a
fresh follow-up budget. A second exhaustion on the same chain stops the
machinery: the item moves to "waiting on the human" in the stream's plan,
and only a human decision recorded there unblocks it. Nothing merges on an
exhausted chain (CL-1-3).

**C-1b: the loop has reverse edges, stated.** An implementer gap-stop reopens
the design (step 1) with the gap as input. A critic finding that indicts the
design rather than the code reopens design critique (step 2). A failed gate
at step 5 returns to implementation (step 3) and the next critique round
reviews the new tree. The five steps are the spine, not the only legal moves
(CL-1-4).

**C-1c: the handoff mechanics are part of the contract.** The critic reviews
the implementer's worktree diff against its recorded base — passed as the
diff artifact plus the tree hash, never as prose. Fixes fold in the same
implementer chain's worktree via follow-up rounds; each critic follow-up
names the new tree hash it reviewed. Critic follow-ups go to the same critic
chain so the round history stays in one place (CL-1-8). The design loop gets
the same rigor: a design-critic follow-up round re-syncs its worktree to the
current commit before reading anything, and its return records the commit it
reviewed, so a critique of a stale plan is detectable exactly like a critique
of a stale tree (CL-2-8).

**C-2: mechanical enforcement where it is honest.** `assert-conformance.sh`
already runs before merge; it gains the three-part check from C-1. The
`code-critic` role also becomes real configuration: the shipped template
gains the role with a placeholder, this repository's local configuration pins
it, and the conformance error when the role is unconfigured says exactly
which key to set — without this, the gate would deadlock every merge in any
repository that adopted the rule before the roster (CL-1-7).

**C-2a: the waiver is bounded by consequence, not syntax.** A diff is
waivable only when it touches nothing that steers behaviour. The metasystem
names its instruction-bearing paths in one list — the agent contract, the
routing index, skills, roles, templates, schemas, project rules, and
everything under `scripts/` — and no diff touching any of them is ever
waivable, because a "documentation-only" change to an instruction file IS a
behaviour change (CL-2-2). What remains waivable: fixture-expectation-only
diffs, and prose outside the instruction list under a named line threshold.
The waiver states its claimed class; conformance checks the claim against
the diff and refuses a mismatch. Waivers are counted per stream and surface
in the retro (CL-1-5).

**C-3: receipts stop overstating, mechanically, and the claim must be
related, not merely real.** `receipt.sh add` refuses `skills=code-critique`
unless the line names both the code-critic chain id and the implementer job
id it reviewed, and the chain's own record names that same implementer job —
an unrelated chain cannot substantiate the claim (CL-2-6). History is
corrected append-only: `receipt.sh correct` writes a CORRECTION line whose
reference is unique — the original entry's epoch plus the hash of the
original line, so colliding timestamps cannot make a correction ambiguous
(CL-2-6). The eight overstating lines each receive one; none are edited,
because the record of the mistake is the valuable part (CL-1-9).

**C-4: the loop is written where every party reads it.** `docs/orchestration.md`
gains the five-step loop as its own section; the orchestrator role and the
host-turn instruction point at it rather than restating it, so there is one
canonical statement and no drift between copies.

## What is deliberately not changed

The design half. It works, it is enforced, and two long chains this session
proved it under pressure.

Delegate-to-delegate critique is not introduced. The orchestrator remains the
single point that dispositions findings and owns the merge, because splitting
adjudication across agents removes the one place a human can look to see what
was decided and why.

## Proof

- Merge with no code-critique chain and no waiver: refused, naming what is
  missing and, if the role is unconfigured, the exact key to set.
- A chain whose final round reviewed a different tree than the one being
  merged: refused (the stale-review case, CL-1-1).
- A chain whose final round still carries a material finding: refused, even
  though every finding has a disposition (the letter-vs-purpose case, CL-1-2).
- A chain that exhausted its round budget: refused, `critiqueExhausted`
  recorded, and the open findings named (CL-1-3).
- A waiver claiming documentation-only over a diff that touches a script:
  refused; the same waiver over a genuinely doc-only diff: allowed and
  counted (CL-1-5).
- Critic and implementer jobs identical in both runtime and model, without a
  declared `independence=session-only`: refused (CL-1-6).
- `receipt.sh add` with `skills=code-critique` and no chain id: refused at
  write time; `receipt.sh correct` writes an append-only CORRECTION line
  (CL-1-9).
- The five-step loop appears once in `docs/orchestration.md`; the role and
  instruction reference it rather than duplicating it.

## Completion

Complete when the proofs pass, the historical receipts carry their
correction, and one real change has gone through all five steps end to end —
its code-critique chain ending in a zero-material final round over the exact
tree that merged, per C-1, not merely a balanced join (CL-2-7).
