# Critique findings need proof, and fixes must not grow the system

- Kind: design
- Id: 01M3V29GSZPW1MEP1C0RXXSJ4V
- Status: accepted
- Goals: critique-findings-need-proof

Revision 2: Astra round 1 folded. F1: R2 also admits a traced causal path, and a passing test refutes only when it exercises the claimed conditions. F2: R4 is made exact, with precedence over fixtures-as-arbiter. F3/F5: existing budgets stay binding, and duplicates of rules the design skill already has are named and limited to the code skill. F4 noted. Round 2 is the failsafe round.

Revision 1. Author: seat m1e (Opus 5.5). Cites were read at `819099a09`.

Wido, 2026-10-01, verbatim:

> Time and time again, you derail completely and go down a fucking rabbit hole. How do I stop you from doing that? The smallest thing that works.

> I'm not quite sure just capping at one code review is the way to go. If there are real findings, they should be fixed.

## 1. The problem, with evidence

The landing-lane redesign (goal `landing-lane-runtime-redesign`, 2026-09-30 to 2026-10-01) had:
- 10 design revisions;
- about 15 code fix rounds, with a re-review after each;
- seven build units, and the old owner deleted (about 32K lines);
- and the lane still never ran end to end once.

There were three causes, and each one is a gap in today's skills rather than a skipped rule:

1. **No threat model was declared, so critics assumed an adversary.** Four rounds of K-b fixes closed "loopholes" that only an agent deliberately cheating the 40-line cap could use: `patch-id --stable` ignoring whitespace, `-diff` attributes, refs/replace, unordered counting. Each fix added code, and the next round attacked that code. `review-brief.md` has a threat-model field, but nothing says what applies when it is left empty, and the lane briefs left it empty.
2. **"Evidence-backed" admitted reasoning as evidence.** `skills/code-critique/SKILL.md` asks for evidence-backed findings. A plausible paragraph passes that bar, so findings no test could reproduce drove fix rounds.
3. **Fixes added mechanisms, and nothing flagged that as a signal.** The design skill bans adding a mechanism only to answer a finding that passes the works-without-it test (R-124). For findings that fail it, the fix was always "add a guard": tuple tokens, a pre-push hook, deny-table rows, custody. Fix code became the next round's attack surface, and the trajectory never fell.

A fourth cause is **not** a skill gap. I started critics by hand (codex-companion, in-process code-critique agents) instead of through `metasystem design review` / `work review`, so no round limit was counted. The template already forbids this ("An independent read starts through its intent, never as an in-process agent"). That one is a conduct fix, recorded in seat memory. It needs no text change.

## 2. Step 1: what changes (text only, three files)

Every real finding is still fixed. A round cap is not the lever: the rules below change what counts as a finding and how it is fixed.

**R1. A default threat model.** In `internal/protocol/templates/review-brief.md`, the threat-model field (which exists, but has no default) gets a default that applies when the brief leaves it blank: "our own agents and operators make mistakes and crash; nobody is attacking us; hostile inputs and deliberate circumvention are out of scope." A brief that wants an adversary must say so and name it. Both critique skills cite the default.

**R2. A code finding needs concrete evidence to be material.** In `skills/code-critique/SKILL.md`, a Layer 2 (adversarial) finding is material only when it lies inside the declared threat model (R1) and gives one of:
- a reproduction: a failing test, or a command and its observed output; or
- a traced causal path: the trigger, and each step from it to the wrong outcome, cited file:line in the reviewed tree. This is for defects a test can't pin reliably, such as a race or a platform the reviewer lacks.

"An agent could, if it tried, ..." with neither of these is recorded as `noted` with the reason "no reproduction or traced path". It never blocks. Layer 1 (conformance) findings are exempt, because the diff against the brief is their proof. A passing test refutes a finding only when it exercises the conditions the finding names; otherwise the finding stays open. Existing round budgets, exhaustion handling and R-124 stay binding: R2 narrows what is admitted, and grants no extra rounds.

**R3. Fix by subtraction (code critique).** `skills/design-critique/SKILL.md` already says "Every mechanism added to appease a finding is itself new attack surface" (Fix the Generating Cause Once). `skills/code-critique/SKILL.md` has no such rule; it gets a short one. When the fix for an accepted finding would add a new mechanism (a new type, file, verb, hook, store or check), the author first tries to remove or narrow the behaviour that has the defect, and the disposition says in one line why subtraction would not do. A second mechanism-adding fix in the same chain is a scope signal: the author asks the human (the existing scope authority) before folding it.

**R4. Divergence stop (code critique).** The design skill already stops a loop where "roughly half of a round's material findings were introduced by the previous round's own edits" (Expect the Returns to Diminish). The code skill gets the same rule, made exact. From round 2 on, counting only adjudicated material findings (accepted, or still open; not noted or out-of-scope):
- if the round has none, the chain closes as today;
- if the count is at least the previous round's count, or at least half of them (rounded up) cite lines the previous fold added or changed in its diff, the chain stops before another fold. The author runs `take-a-step-back` and reports the trajectory to the human. The findings stay open: they are not dropped.

Fixtures-as-arbiter needs a falling trajectory, and this stop needs a non-falling one, except for the half-in-the-fold case. When both apply, this stop wins, because a fold critiquing itself is the condition that exit was never meant to cover.

**R5. Run the real thing before a second code round.** In `skills/code-critique/SKILL.md`: when the change has a runnable surface, the `verify` skill run happens after the first fold and before round 2. Failures from that run outrank the open review findings. Round 2 reads the fold plus whatever the run touched.

Out of step 1, in the deferred list: mechanical enforcement, e.g. a `reproduction` field in the critic `return.json` that `work review` refuses without, or automatic trajectory counting in `work review`. These come later, when the text rules are seen not to hold.

## 3. Pros and cons

Pro:
- Real defects are still fixed: a finding with a reproduction or a traced causal path is material, within the existing round budgets.
- Hypotheticals stop driving work. R1 and R2 together would have demoted every K-b cap loophole.
- Fix code stops feeding the next round (R3, R4).
- Reality ranks findings (R5).

Con:
- Critics spend more effort per finding, writing a test or tracing a path. Partly offset by fewer rounds.
- A real adversary case needs the brief to say so. That is the point: it is a choice, not a default.

Closed at round 2 (failsafe): Astra agreed, 0 material; its two prose notes are folded here. Accepted on Wido's order of 2026-10-01 ("get this immediately implemented").

## 4. Tests

Text-only change; no code. Proof:
- `go run ./cmd/devgate static` and the audit tests that read skills (`-run 'TestAudit'` in cmd/metasystem) stay green.
- The next critique this seat runs (the first after landing) states the threat model, cites R2 on each Layer 2 finding, and records the trajectory. Checked by hand at that review.

## 5. Use of the wait

While Astra reads, the seat watches the stage-1 switch-on (the ui seat's `ask-what-happened-follow-ups`).
