# The Collaboration Loop: how orchestrator and delegate actually work together

- Goal and current status: the loop that inspired this metasystem — orchestrator designs, delegate critiques to agreement, delegate implements, orchestrator critiques to agreement — stated once as canon, enforced where it can be, and honest where it cannot. Status: DRAFT, awaiting critique.
- Next step: design critique by whichever agent the roster assigns the design-critic role, until closed by join; then implement.
- In flight right now: nothing
- Waiting on the human: ratification by accepting this design after critique

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

The single constraint the loop does impose is independence, stated as a
property rather than a roster: **the agent that critiques an implementation
must not be the agent that produced it.** How a project satisfies that —
different model, different runtime, or merely a different session with no
shared context — is the project's decision, recorded in its configuration.

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

**C-1: implementation critique becomes a required step with a real gate.**
After a delegate's implementation returns and before the orchestrator merges,
a `code-critic` delegate reviews the diff against the brief and the design.
Its findings are dispositioned and joined by `assert-critique-closed.sh`,
exactly as the design half. The merge is refused until the join passes.

**C-2: mechanical enforcement where it is honest.** `assert-conformance.sh`
already runs before merge; it gains a check that a closed code-critique chain
exists for the job being merged, naming the chain. When a change is small
enough to skip critique, the orchestrator records that decision explicitly in
the job record (`critiqueWaived` with a reason), which the same check
accepts and which shows up in evidence rather than hiding.

**C-3: receipts stop overstating.** `skills=code-critique` may only be
recorded when a code-critic chain ran. Orchestrator review is `skills=none`
with the review named in the note. The eight historical receipts are
corrected in place with a one-line note rather than rewritten, since the
record of a mistake is worth more than a clean history.

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

- A fixture where a merge is attempted with no code-critique chain and no
  waiver: refused, naming what is missing.
- A fixture where the chain exists but has unjoined findings: refused.
- A fixture where `critiqueWaived` carries a reason: allowed, and the reason
  appears in the job record.
- A fixture asserting a receipt with `skills=code-critique` and no
  corresponding chain is rejected by `receipt.sh`.
- The five-step loop appears once in `docs/orchestration.md`, and the role
  and instruction reference it rather than duplicating it.

## Completion

Complete when the five proofs pass, the historical receipts carry their
correction, and one real change has gone through all five steps end to end
with its code-critique chain closed by join.
