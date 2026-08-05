---
name: design-critique
description: Run an adversarial critique loop over a written design before implementation, with a materiality criterion that decides both what a critic reports and when the loop stops. Use when a design document is complete enough to attack, when adjudicating critique findings, or when a critique loop keeps producing rounds without changing what would be built. Do not use for code review of an implementation (that is conformance review) or for investigations of failing behavior (take-a-step-back).
---

# Design Critique

A design is attacked before it is built, and the loop ends the moment further critique stops changing what an implementer would build. Both halves matter: critique without an exit burns the budget on prose polish, and an exit without adversarial critique ships the design's blind spots.

## Roles

The critic attacks; the designer adjudicates; neither does the other's job. A critic reports findings only; it never rewrites the design, and refuting the design's premise is encouraged, not out of scope. The designer answers every finding with a disposition; silently dropping one is a defect. The critic may be another agent, a fresh-context session, or a human; what matters is that it did not write the design it is attacking.

## The Materiality Criterion

Apply this test to every finding, and give it to the critic verbatim so findings arrive pre-sorted; the criterion binds the critic, not just the adjudicator:

> Would an implementer working from this design build something DIFFERENT, or WRONG, because of this finding?

- **Material: report it, adjudicate it, keep the loop alive.** It changes a contract, schema, or interface; changes control flow or an outcome mapping; changes what a test asserts; changes a named owner; reveals a false premise the design was built on; or leaves the implementer a genuine choice to guess at.
- **Not material: record it, do not action it, never let it block.** The document contradicts itself in prose while the implementation is unambiguous; counts, arithmetic, naming, ordering; restating something stated elsewhere; anything an implementer would resolve identically either way.

Require the critic's verdict line to count only material findings.

## The Loop

1. The critic reads the full design and reports findings, each sorted material or not, with the evidence that supports it.
2. The designer adjudicates every finding: **accept** amends the design; **refute** is recorded with its reasoning. Both are legitimate outcomes; a critique loop where nothing is ever refuted is not being read critically.
3. **Read the findings body, never just the verdict line.** A summary that says "no blocking findings" above a body that lists one is itself a finding; the body governs.
4. Stop the loop the first round that produces no material finding. Do not run another round for prose consistency.

## Close a Round by Join, Not by Count

A round is closed only when every material finding carries a disposition — and that is a claim to be checked, not asserted. Parse the critique into a structured worklist (stable identifier, severity, proposal) and join it against the dispositions; the round closes when the two sets are equal. Working from prose invites the failure this prevents: "N corrections applied" reads like closure while unaddressed findings sit in the body, and the next round spends itself rediscovering them instead of finding anything new. If the critique carries no stable identifier per finding, ask for one — an unjoinable critique can be estimated, not closed.

The mechanical form uses the canonical `findings` array in the critic's `return.json` and a Markdown dispositions table headed `| Finding id | Disposition | Reasoning and evidence | Amendment |`; run `scripts/assert-critique-closed.sh --findings <return.json> --dispositions <file>` to perform the join.

When a round's findings are retained or carried elsewhere (a watch-list, a later round's brief), count the retained findings against the round's own verdict number before calling the round closed. A retention that silently drops findings reads exactly like a complete one, which is the same failure the join above prevents; it has happened twice in this repository's own loops.

Refutations carry the same burden of proof as the findings they answer. Record the evidence literally: the exact string searched and what it returned, not a summary of the conclusion. Both directions fail in practice — a finding can be wrong on the facts, and a refutation can be wrong because it checked the wrong string.

## Fix the Generating Cause Once

When the same class of finding recurs across rounds ("section X contradicts row Y", again and again), stop patching instances. Rewrite the accreted artifact in a single pass and let the next round attack the rewrite. A long design patched in place accumulates contradictions faster than a loop can remove them, and the loop degenerates into finding the previous round's patch seams.

## Expect the Returns to Diminish

Real refutations land early: deleted mechanisms, collapsed abstractions, false premises. After the architecture stops being challenged, watch for the loop becoming self-sustaining: when roughly half of a round's material findings were introduced by the previous round's own edits, the loop is critiquing itself. Stop there, record the judgment, and let implementation be the next source of truth: an ambiguity that survives into code becomes a failing test instead of another paragraph. Retain the final rounds verbatim so the loop can resume if implementation proves the stop premature, and never claim the design is perfect; claim that further critique stopped being worth its cost.

## Record

Keep the adjudication trail (finding, disposition, reasoning) with the design or in a companion ledger under `plans/`, so a later reader can distinguish "considered and refuted" from "never considered". When the design carries an obligation matrix, adjudications that amend the design update the matrix in the same pass (`docs/design/design-obligation-gate.md`).
