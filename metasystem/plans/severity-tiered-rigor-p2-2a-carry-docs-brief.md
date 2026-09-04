Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2-2a-carry)
Date: 2026-09-04

# Carry slice 2a, the docs re-touch: one round applying the seat's prose patch

The risk basis landed on main as b4ae9395 ("Risk is four separate
questions, never the shape of the change"). Its docs re-touch, three
prose files the seat wrote (docs/orchestration.md,
skills/design-critique/SKILL.md, skills/code-critique/SKILL.md), was
refused as landing carriage: docs and skills are behavior-class paths
and travel only on a reviewed chain. This round is that chain's build.

Step one: from the repository toplevel (the parent of `metasystem/`)
save the unified diff fenced at the end of this brief, byte for byte,
to a file and apply it: `git apply <file>` (blank context lines in the
fence carry no leading space; git apply accepts them; three files, 112 lines
against main at b4ae9395; it applies cleanly). Do not rewrite,
reformat, or "improve" the prose.

Step two, the fact check the seat owes: every mechanism the new prose
names must exist in the tree by that name. Verify with a search under
metasystem/cmd and metasystem/internal: `job critique-budget-rebind`,
`job critique-register-close`, `goal discharge-review-obligation`,
`goal accept-risk`, the config keys `metasystem.budget.risk-gate` and
`metasystem.budget.review-round-max`, the refusal `RISK_UNANSWERED`,
`gateWidth: full`, the `--risk severity=<n>,novelty=<n>,exposure=<n>,accumulation=<n>`
and `--basis` flags of `goal open` and `goal edit`, the `--evidence`
grammar (`root:<jobId>`, `finding:<jobId>/<id>`, `refusal:<code>`), and
`BudgetExceptions` with the `repeated exception: defect signal` line.
A name the tree does not carry is corrected to the name it does carry,
in that sentence only, and the correction is listed under `decisions`
with the file:line that grounds it. A claim the tree contradicts in
substance (the mechanism does not do what the sentence says) is a gap:
stop and report it with the anchor; do not rewrite the sentence.

Nothing else changes: no Go, no tests, no plans, no records. Run
nothing but the searches. Do not stage or commit; the seat lands the
chain.

# Constraints

Wall-clock budget: 15 minutes; return by minute 12 whatever the state.
Return under the implementer schema with `decisions` listed (an empty
list when every name checked) and the three files named under evidence.

# Gap Rule

Stop and report a gap only for a substantive contradiction between the
prose and the tree (the mechanism named does not exist at all, or does
the opposite of what the sentence says); a name spelled differently
from the tree is corrected, recorded under `decisions`, and applied. A
choice recorded in the return is not silent.

# The patch

```diff
diff --git a/metasystem/docs/orchestration.md b/metasystem/docs/orchestration.md
index 4a25645d..3467fed1 100644
--- a/metasystem/docs/orchestration.md
+++ b/metasystem/docs/orchestration.md
@@ -51,23 +51,43 @@ critique draw on a review-round member that Part One stores on the goal: the
 `metasystem.budget.tier-1`, `metasystem.budget.tier-2`, and
 `metasystem.budget.tier-3` keys in `metasystem.conf` provide zero, two, and
 three rounds, while `metasystem.budget.review-round-max` keeps three as the
-ceiling. Mechanical accounting is PENDING in Part Two under design point
-STR2-ROUND-ACCOUNTING-05: dispatch will freeze that boundary on the chain
-root, count rounds spent against it, and make exhaustion open no fresh
-budget. The tier is classified at intake; PENDING Part Two from design
-revision 4, the four `--risk` answers and `--basis` derive it, `--why`
-records an override, and `gateWidth: area|full` is written on the chain root.
+ceiling. Part Two accounts mechanically (design point
+STR2-ROUND-ACCOUNTING-05): dispatch freezes the goal's round member on the
+critic chain root at dispatch (a goal-free root reads the configured ceiling
+alone), counts each follow-up round against it, and refuses the round past it;
+exhaustion opens no fresh budget. Only an approved token raising the goal's
+five-member tuple raises the stored member, never above the ceiling, and `job
+critique-budget-rebind` copies the raised member onto an open root.
+
+Risk is four separate questions, never the shape of the change. A goal's
+tier derives from its Risk record: severity, novelty, exposure and
+accumulation, each scored 1 to 3 with a basis sentence, given to `goal open`
+and `goal edit` as `--risk severity=<n>,novelty=<n>,exposure=<n>,accumulation=<n>
+--basis <text>` (the classification sweep takes the same four scores as
+`<s>,<n>,<e>,<a>` and renders the tier itself); `--tier` without the four
+answers is refused. An override above the derivation is recorded
+with `--why`; an override below it, or a lowering after claim, is the human's
+act alone. A raise after claim is one transaction that re-binds the claim's
+revision and clears no fence or obligation; a chain dispatched before the
+raise keeps the tier it was dispatched under. A misclassification is raised
+with `--evidence` in a fixed grammar (`root:<jobId>`, `finding:<jobId>/<id>`,
+or `refusal:<code>` with a code from the admission list), and the risk gate runs in the mode
+`metasystem.budget.risk-gate` names: `mark` prints `RISK_UNANSWERED` and
+admits, `enforce` refuses with that code. Accumulation 2 or higher writes
+`gateWidth: full` on the chain root, and a full-width chain lands only with
+the full battery receipt. Every over-box budget member increments the goal's
+`BudgetExceptions`; a second exception marks the appetite line `repeated
+exception: defect signal`.

 Under R-60-m1, the reviewer stops critique at the first round with no material
-finding. A material finding must change what gets built and, PENDING Part Two from design
-revision 3, name that artifact; the same pending machinery demotes a finding
-that fails the artifact test and gives `job critique-close` its
-bounded-obligation and human-risk exits through `goal
-discharge-review-obligation`, `goal accept-risk`, and the goal's
-review-obligation records. Only an approved token raising the goal's
-five-member tuple can raise its stored round member, and never above the
-three-round ceiling; until the accounting and close land, the reviewer takes
-exhausted work to the human, never to a silent fourth round.
+finding. A material finding must change what gets built and name that
+artifact; a finding that fails the artifact test is demoted at registration.
+When the rounds are spent, `job critique-register-close` defers each bounded
+open finding into a review obligation on the goal (discharged later by `goal
+discharge-review-obligation` against the chain, artifact and test that carry
+it) and closes the register; a severe or unproven finding closes only after a
+human records `goal accept-risk` for it. The reviewer never dispatches a
+silent fourth round.
 Tier 1 has no critique and lands as a receipted direct fix bound to the
 candidate tree.
 The tier boxes' reserved-minute members are the runaway guard;
diff --git a/metasystem/skills/code-critique/SKILL.md b/metasystem/skills/code-critique/SKILL.md
index 2f5055d4..90da90b3 100644
--- a/metasystem/skills/code-critique/SKILL.md
+++ b/metasystem/skills/code-critique/SKILL.md
@@ -62,10 +62,15 @@ Use `accepted` or `refuted` for material findings; a TRUE finding outside the br
 When a design chain exited through fixtures-as-arbiter (see the design-critique skill), this code critique is MANDATORY and the named fixture obligations are part of its findings surface: an unimplemented or failing named fixture is a material finding.

 Part One stores the review-round member in the goal's tier box: zero rounds
-for Tier 1, two for Tier 2, and three for Tier 3. Mechanical accounting is
-PENDING in Part Two under design point STR2-ROUND-ACCOUNTING-05: dispatch will
-freeze that boundary on the critic chain, count rounds spent against it, and
-make exhaustion open no fresh budget. Start every chain from
+for Tier 1, two for Tier 2, and three for Tier 3, and the tier itself derives
+from the goal's four risk answers (severity, novelty, exposure, accumulation),
+never from the shape of the change. Part Two accounts mechanically (design
+point STR2-ROUND-ACCOUNTING-05): dispatch freezes that member on the critic
+chain root (a goal-free root reads `metasystem.budget.review-round-max`
+alone), counts each follow-up round against it, refuses the round past it,
+and exhaustion opens no fresh budget. A chain root under accumulation 2 or
+higher carries `gateWidth: full` and lands only with the full battery
+receipt. Start every chain from
 `scripts/agents/templates/review-brief.md` — round budget, threat model,
 appetite, and scope declared BEFORE round one; a true finding outside the
 declared threat model closes as out-of-scope citing the brief. Record the
@@ -77,12 +82,14 @@ goal's budget in the brief before review:

 As the reviewer's R-60-m1 rule, stop at the first round with zero material findings.

-Only a finding that changes what gets built can keep the chain open and,
-PENDING Part Two from design revision 3, it must name the artifact it would
-change; that pending machinery demotes a finding that fails the artifact test.
-If material findings remain, do not certify the change. Only an approved token
-raising the goal's five-member tuple can raise its stored review-round member,
-never above the three-round ceiling. PENDING Part Two from design revision 3, `job
-critique-close` sends exhausted bounded findings to review obligations and
-closes after accepted risk; until the accounting and close exist, stop with the
-work waiting on the human. Never dispatch a silent fourth round.
+Only a finding that changes what gets built can keep the chain open, and it
+must name the artifact it would change; a finding that fails the artifact
+test is demoted at registration. If material findings remain, do not certify
+the change. Only an approved token raising the goal's five-member tuple can
+raise its stored review-round member, never above the three-round ceiling;
+`job critique-budget-rebind` copies the raised member onto an open root. When
+the rounds are spent, `job critique-register-close` sends exhausted bounded
+findings to review obligations on the goal (discharged later by `goal
+discharge-review-obligation`) and closes after a human records `goal
+accept-risk` for the rest; while a severe or unproven finding stands, stop
+with the work waiting on the human. Never dispatch a silent fourth round.
diff --git a/metasystem/skills/design-critique/SKILL.md b/metasystem/skills/design-critique/SKILL.md
index 947df07e..8a0929a9 100644
--- a/metasystem/skills/design-critique/SKILL.md
+++ b/metasystem/skills/design-critique/SKILL.md
@@ -36,25 +36,30 @@ The loop's stop rule is fixed before round 1, never improvised mid-loop: the bri
 ## Round Budget and Exhaustion

 Part One stores the review-round member in the goal's tier box: zero rounds
-for Tier 1, two for Tier 2, and three for Tier 3. Mechanical accounting is
-PENDING in Part Two under design point STR2-ROUND-ACCOUNTING-05: dispatch will
-freeze that boundary on the critic chain, count rounds spent against it, and
-make exhaustion open no fresh budget. Start every chain from
+for Tier 1, two for Tier 2, and three for Tier 3, and the tier itself derives
+from the goal's four risk answers (severity, novelty, exposure, accumulation),
+never from the shape of the change. Part Two accounts mechanically (design
+point STR2-ROUND-ACCOUNTING-05): dispatch freezes that member on the critic
+chain root (a goal-free root reads `metasystem.budget.review-round-max`
+alone), counts each follow-up round against it, refuses the round past it,
+and exhaustion opens no fresh budget. Start every chain from
 `scripts/agents/templates/review-brief.md` — budget, threat model, appetite,
 and scope declared BEFORE round one; a true finding outside the declared
 threat model closes as out-of-scope citing the brief. Record the goal's budget
 in the brief. As the reviewer's R-60-m1 rule, stop at the first round with no
 material finding. A finding keeps the chain open only when it changes what gets built
-and, PENDING Part Two from design revision 3, names the artifact it would
-change; that pending machinery demotes a finding that fails the artifact test.
+and names the artifact it would change; a finding that fails the artifact
+test is demoted at registration.

 Only an approved token raising the goal's five-member tuple can raise its
-stored review-round member, and the three-round ceiling still applies.
-PENDING Part Two from design revision 3, `job critique-close` defers exhausted
-bounded findings into review obligations or closes after a human records
-accepted risk; until the accounting and close exist, or whenever a severe or
-unproven finding remains, stop with the design waiting on the human. Never
-dispatch a silent fourth round.
+stored review-round member, and the three-round ceiling still applies; `job
+critique-budget-rebind` copies the raised member onto an open root. When the
+rounds are spent, `job critique-register-close` defers exhausted bounded
+findings into review obligations on the goal (`goal
+discharge-review-obligation` discharges them later against the chain,
+artifact and test that carry them) or closes after a human records `goal
+accept-risk`; whenever a severe or unproven finding remains, stop with the
+design waiting on the human. Never dispatch a silent fourth round.

 Rounds must run as follow-ups on one critic chain. Dispatching a fresh critic
 job per round silently evades the budget — no exhaustion can ever fire — which
```
