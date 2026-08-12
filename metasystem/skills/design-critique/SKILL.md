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

## Round Budget and Exhaustion

The shipped round budget is three focused rounds; record it in the brief. If
material findings remain after round three, the next focused follow-up must
enumerate every open finding identifier; dispatch records that successor in
the chain's `critiqueExhaustions` array and opens one fresh three-round budget
on the same critic chain. If material findings exhaust that second budget,
stop outright with the design waiting on the human. A human decision recorded
in the stream plan is the only remedy.

The budget lives on the critic CHAIN, so rounds must run as follow-ups on
one chain. Dispatching a fresh critic job per round silently evades the
budget — no exhaustion can ever fire — which is how a seven-round loop ran
unbudgeted on this repository's own consolidation design (2026-08-12)
before the human called it.

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

## Recorded Precedent: the Loop Protecting Its Own Redesign

2026-08-11, the stop-loss redesign (`plans/stop-loss-last-defense.md`).
Three rounds at xhigh produced 13, 14, and 14 material findings — 41
accepted, zero refuted — including a hidden second fuse the author had
missed, an oscillation hole in the author's own fix, and a whole half of
the design specified against a runner sequence that did not exist. The
loop did not converge, and the author did not force it: exhaustion was
recorded in the plan, the design was parked, and the human ruled a split
into a small core and satellites. This is the reference case for three
rules above at once — exhausting rounds is not agreement; when findings
keep landing in one region, the generating cause (here: a mis-founded
half) is rewritten or severed rather than patched; and the loop's verdict
outranks the author's confidence, including when the author is the main
agent and the design is about the loop's own governing mechanism.

## Satellites: What a Split Produces

A SATELLITE is a design unit severed from a critique-exhausted parent, and
the word carries obligations. A satellite is born from evidence: it exists
only because accepted findings route to it — when a loop's findings keep
clustering in a separable region, that region is severed (the generating
cause rule), never invented from ambition. A satellite inherits and does
not re-litigate: the parent's human ruling travels unchanged, together
with an explicit routing of the parent findings it must resolve. A
satellite converges alone: its own design note, critique loop,
dispositions, implementation, and tests — sized so the loop can close,
which is exactly what the parent could not do. And satellites are ordered
by dependency on truth: the ones that make a signal honest precede the
mechanisms that consume the signal, all standing on whatever shared
ground-truth mapping the split named as a precondition. Reference case:
the stop-loss split (`plans/stop-loss-satellites.md`), where the parent's
41 accepted findings route to one shipped core and four satellites.

## Ground the Facts Before the First Round

Most rounds burned in this repository's loops were spent fact-checking
the author, not judging the design: drafts carried claims about shipped
mechanisms — call sites, grammars, lifecycles — that the code
contradicted, and the critic did the correcting at critique prices
(human ruling, 2026-08-11: find a way not to produce the flaggable
claims at all). The discipline: before a design's first round, every
claim about shipped behavior is verified with a file:line anchor —
gathered by a code-grounded fact pass (the main's own investigation or
a harness-side evidence agent; within a mission the investigator role
is main-assigned and evidence gathering is never a design delegation).
The design then cites the fact sheet, and a mechanism claim without an
anchor is a defect in review. The critique loop exists to attack
judgment — tradeoffs, invariants, failure behavior — and every fact it
must correct is a round it cannot spend on judgment.
