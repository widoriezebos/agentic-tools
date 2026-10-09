---
name: design-critique
description: Run an adversarial critique loop over a written design before implementation, with a materiality criterion that decides both what a critic reports and when the loop stops. Use when a design document is complete enough to attack, when adjudicating critique findings, or when a critique loop keeps producing rounds without changing what would be built. Do not use for code review of an implementation (that is conformance review) or for investigations of failing behavior (take-a-step-back).
---

# Design Critique

A design is attacked before it is built, and the loop ends the moment further critique stops changing what an implementer would build. Both halves matter: critique without an exit burns the budget on prose polish, and an exit without adversarial critique ships the design's blind spots.

## Roles

The critic attacks; the designer adjudicates; neither does the other's job. A critic reports findings only; it never rewrites the design, and refuting the design's premise is encouraged, not out of scope. The designer answers every finding with a disposition; silently dropping one is a defect. The critic may be another agent, a fresh-context session, or a human; what matters is that it did not write the design it is attacking.

## Find the Record Before the First Round

The record is found before anything is read. The coordinator starts a design critique with one command on the design file:

```bash
metasystem design review <design file>
```

It first resolves the file as the goal's design record, the same resolution `metasystem design list --goal <goal id>` performs, and only then freezes the design, prepares the review inputs and dispatches the configured design-review lane. A critic dispatched by that command never runs it again: it reads the frozen design its brief names and returns findings. A critic working outside the command (a human, or a session handed a design directly) runs that resolver itself as its first act. A design the resolver cannot find is not ready to be attacked: the command refuses, or the outside critic returns that refusal as its single finding and stops, and the designer writes the head and moves the file into the home before the loop starts (`docs/design/design-obligation-gate.md`, "A design is a record"). A design about the project as a whole names no goal and says so in the brief; everything below runs once the record is found.

## Step 1 Before Anything

The critic's first question, answered before any other finding: does the design name **step 1** — what exists and is usable after one slice — and list what it defers? A design that does not is not complete enough to attack; the critique returns that single finding and stops. This is the human's standing law (R-121, 2026-09-22): every design is first the design of the smallest thing that works, whatever the size of the ask, and iteration is a separate decision taken after step 1 has value. The rest of this skill runs inside that anchor.

## Check the Rulings

Before the materiality test, read the standing rulings register, `memory/rulings.md`, and the human decisions on the goal the brief names (`metasystem goal show <goal> --history --json`: its `Intent`, its `NextStep`, and every `History` entry whose `Actor` begins `human:`, with its reason; the plain view truncates). Work that contradicts a ruling or a goal decision is a material finding whatever else is true, because it takes authority a human did not give: its id is `RULING-` followed by the ruling id (`RULING-R-124-m1u`) or the goal id, its claim quotes the ruling's words, its evidence cites the text under review that breaks them, and its rigor row is classified from the boundary facts like any other finding. The return lists what was checked in one `evidence` row: `command` names the register and the goal, `observed` lists the ruling ids read and says `no conflict` or names the findings, and `level` is `read`. A brief that names no goal checks the register alone and says so in that row; a register or goal the critic could not read is a `gaps` entry, never silence.

## The Materiality Criterion

Apply this test to every finding, and give it to the critic verbatim so findings arrive pre-sorted; the criterion binds the critic, not just the adjudicator:

> Would an implementer working from this design build **step 1** DIFFERENT, or WRONG, because of this finding?

- **Material: report it, adjudicate it, keep the loop alive.** It changes a contract, schema, or interface step 1 builds; changes control flow or an outcome mapping in it; changes what one of its tests asserts; changes a named owner; reveals a false premise it was built on; or leaves the implementer a genuine choice to guess at.
- **Outside step 1: record it in the design's deferred list, do not action it, never let it keep the loop alive.** A hole step 1 does not open — a scale the first slice does not reach, a failure no user can yet trigger, a second decision-maker who does not yet exist — is real and is not material; it goes to the deferred list with the field it will build on. A loop without this anchor converges on a power plant: the memory-system design (`plans/memory-system-r6.md` in the host repository, 2026-09-22) took three rounds of 16, 12 and 11 findings and 470 lines before the human cut it to 110 lines and two four-hour slices, and every one of those findings had been material under the unanchored test.
- **Not material: record it, do not action it, never let it block.** The document contradicts itself in prose while the implementation is unambiguous; counts, arithmetic, naming, ordering; restating something stated elsewhere; anything an implementer would resolve identically either way.

**The second test, decisive (R-124, Wido 2026-09-25): does step 1 WORK, and is it SAFE, without this finding?** A finding is material only when step 1 does not work without it (the slice fails at its own first use, or answers wrong) or is not safe without it (authority a human did not give, data lost or overwritten, a silent false answer). A finding that makes step 1 more complete, more robust at a scale or a corner its first use does not reach, more consistent, or more elegant, is real and goes to the deferred list, "later, when it hurts", with the field it will build on. The critic states the answer to this test on every finding. The designer folds only the findings that fail it, and a fold never adds a mechanism to answer a finding that passes it: answering rigor with machinery is the failure mode this rule exists to stop.

Require the critic's verdict line to count only material findings, and to say for each which of the two tests it fails. A finding about circumvention or hostile input is material only when the brief's threat model names that adversary; a blank threat model means the review-brief template's default, which has none.

## The Loop

1. The critic reads the full design and reports findings, each sorted material or not, with the evidence that supports it.
2. The designer adjudicates every finding: **accept** amends the design; **refute** is recorded with its reasoning. Both are legitimate outcomes; a critique loop where nothing is ever refuted is not being read critically.
3. **Read the findings body, never just the verdict line.** A summary that says "no blocking findings" above a body that lists one is itself a finding; the body governs.
4. Stop the loop the first round in which every remaining finding passes the works-without-it test, whatever their number: a round of ten findings that step 1 works and is safe without closes the loop, and those ten go to the deferred list. A round with no finding at all closes it too. Do not run another round for prose consistency, and never for rigor beyond step 1.

## Round Budget and Convergence

Freeze the allowance before the first examination: at most four completed whole-page examinations, bounded by the goal's approved review-round member. Explicit approved zero, including tier 1, admits none. A goal-free review uses the configured positive `metasystem.budget.review-round-max` ceiling capped at four; configured zero means no ceiling and therefore four examinations. Goal-free designs gain no goal authority. Severity and a later budget raise never buy another examination. Paths are aliases of the retained design identity; renaming or unreadable history grants no fresh chain.

The first positive examination continues if allowance remains. Later positive examinations continue only when the whole-page material count strictly falls and no fixed class recurs in the same stable section. A class is fixed when a prior material finding was accepted and its section changed in the bound revision; an optional author mark proves nothing. Equal or rising counts, fixed-class recurrence, or the final allowed examination stop further critique. Zero material publishes automatic acceptance and closes after publication.

At a known stop, retain clean sections and fold every concrete change into its owning unit's acceptance items. Write each folded change into its owning Decision text in one final revision, with named implementation and test evidence; that revision buys no examination. Evaluate buildability after folding. Transfer only unresolved choices and dependent sections into one follow-up queued immediately after the source. If every blocker has a concrete change and splitting leaves no buildable unit, fold and accept. Counts remain truthful: folded findings do not become a zero-material read. Commit the exit and mandatory items before projecting the accepted status and closed-critique head, then close the chain. Acceptance authorizes the design within approved scope; it supplies neither a person's implementation approval nor implementation proof.

Ordinary convergence never asks a person to choose an exit. Read the current `review.stop` policy before every autonomous effect: a numeric value may lower correction allowance, never raise the frozen examination cap; `person` holds the prepared act and asks its holder, including for a goal-free design. Only a successful matching act closes that ask; text, failed acts and unrelated acts do not.

Unknown whole-page evidence is never zero material. Recover the retained return, prose and frozen subject first. `metasystem design review FILE --retry N` reserves one fresh execution of that same candidate after the old process is proven quiescent; it spends no completed-examination, correction or work allowance. Exhaustion retains `stopped <cause>` on both the design round and finding register and prints the bound `design review FILE --ruling TEXT --reason TEXT --by NAME` and `--scope FILE --reason TEXT --by NAME` forms. An enrolled person can use either without repairing advisory evidence; the engine records impact before effect and never invents a clean read or implementation proof. A deadline does not retry.

Start from `internal/protocol/templates/review-brief.md`, with budget, threat model, appetite and scope declared before examination one. A true finding outside the declared threat model closes as out-of-scope citing the brief. A material finding must change what gets built and name the artifact it changes. Continue on the same critic root with bound dispositions; a fresh critic per round evades the frozen allowance.

## Close a Round by Join, Not by Count

A round is closed only when every material finding carries a disposition — and that is a claim to be checked, not asserted. Parse the critique into a structured worklist (stable identifier, severity, proposal) and join it against the dispositions; the round closes when the two sets are equal. Working from prose invites the failure this prevents: "N corrections applied" reads like closure while unaddressed findings sit in the body, and the next round spends itself rediscovering them instead of finding anything new. If the critique carries no stable identifier per finding, ask for one — an unjoinable critique can be estimated, not closed.

The mechanical form uses the canonical `findings` array in the critic's `return.json` and a Markdown dispositions table headed `| Finding id | Disposition | Reasoning and evidence | Amendment |`; `metasystem design review <file> --dispositions <file>`, `metasystem work revise j2:<review> --dispositions <file> --brief <file>` and `metasystem work review <goal> --dispositions <file>` perform that join before acting; `bin/metasystem work review --check-only --findings <return.json> --dispositions <file>` checks it alone.

When a round's findings are retained or carried elsewhere (a watch-list, a later round's brief), count the retained findings against the round's own verdict number before calling the round closed. A retention that silently drops findings reads exactly like a complete one, which is the same failure the join above prevents; it has happened twice in this repository's own loops.

Refutations carry the same burden of proof as the findings they answer. Record the evidence literally: the exact string searched and what it returned, not a summary of the conclusion. Both directions fail in practice — a finding can be wrong on the facts, and a refutation can be wrong because it checked the wrong string.

## Fix the Generating Cause Once

When a fixed class recurs in the same stable section, stop critique and prepare the fold/split exit. Rewrite its concrete changes into the owning Decisions in the single final revision; the rewrite earns no new examination. A long design patched in place accumulates contradictions faster than a loop can remove them, and the loop degenerates into finding the previous round's patch seams.

The same discipline applies to machinery grown under critique pressure: when successive rounds add mechanism to satisfy the critic and the material count still rises, stop answering with machinery and invert the question — name the requirement the smaller design fails. Every mechanism added to appease a finding is itself new attack surface, so the loop can climb its own ladder indefinitely: the backlog-sync design accreted proof envelopes, semantic replay, and anchor chains across three rounds while findings rose from nine to fourteen, and converged three rounds after the scope reduction put the smaller design on the table (D118). The step-1 anchor above is the standing form of that inversion: when the count rises, ask what step 1 needs, not what the critic can imagine.

## Expect the Returns to Diminish

Real refutations remove mechanisms and false premises. Judge later examinations by whole-page counts and mechanically fixed classes in stable sections. When either stops the chain, record the prepared fold/split exit and let implementation test the remaining concrete changes. Retain the final evidence and consumed allowance; an implementation defect supplies no fresh critique budget. Never claim the design is perfect; state the recorded reason critique stopped.

## Record

Keep the adjudication trail (finding, disposition, reasoning) with the design or in a companion ledger under `plans/`, so a later reader can distinguish "considered and refuted" from "never considered". When the design carries an obligation matrix, adjudications that amend the design update the matrix in the same pass (`docs/design/design-obligation-gate.md`).

## Recorded Precedent: the Loop Protecting Its Own Redesign

2026-08-11, the stop-loss redesign (outcome in `docs/patience.md` and
`docs/design/stop-loss-core.md`, Decisions; the design record
`records/stop-loss/stop-loss-last-defense.md` is in the tag `records-archive-2026-09-28`).
Three rounds at xhigh produced 13, 14, and 14 material findings — 41
accepted, zero refuted — including a hidden second fuse the author had
missed, an oscillation hole in the author's own fix, and a whole half of
the design specified against a mission-runner sequence that did not exist. The
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
the stop-loss split (`docs/patience.md`, "The satellite contracts"; the split record
`records/stop-loss/stop-loss-satellites.md` is in the tag `records-archive-2026-09-28`), where the parent's
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
