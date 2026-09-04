Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-04

# Goal

Build PART FOUR of the tiering machinery, the docs, moved to m2 by
R-74-m3. The machinery on main: part one (6c86953a, the tier at
intake: Tier on every goal bound into approval and frozen on chain
roots, the five-member budget whose fifth member is the review rounds,
the tier boxes metasystem.budget.tier-1|2|3 and review-round-max in
metasystem.conf, dispatch.cap-max, goal classify-sweep, the token
quadruple, goal open refusing without --tier, a hazard above a tier-1
goal refused) and part three (efaa5cf4, the tier-1 landing class: a
test receipt bound to the candidate tree, the changed-lines bound, the
manifest's floor rows, --root-job, gateWidth read from the root,
landing.receipt-bound-min). Part two (another seat, goal
severity-tiered-rigor-p2) is not yet on main: the four risk answers
that derive the tier (design revision 4 in
metasystem/plans/severity-tiered-rigor-p2-design.md: --risk and
--basis on goal open and edit, --why for an override, gateWidth:
area|full written on the root) and the material stop and close
(design revision 3 part two: the artifact member of a material
finding, the demotion rule, job critique-close with its two exits,
goal discharge-review-obligation, goal accept-risk, the review
obligations on the goal record). Cite those as PENDING from the
design, in one clause each, so the other seat re-touches the sentence
when it lands.

The rulings that bind the words: R-54-m1 (classify at intake, then
budget by tier; the three tiers), R-60-m1 (review depth is a
risk-based budget; a finding is material only if it changes what gets
built and names the artifact; no separate cap machinery), R-42-m0
(three review rounds is the ceiling), R-58-m1 (the minute pool is a
runaway guard), R-73-m3 (risk is four questions, never the shape of
the change), all in metasystem/memory/rulings.md.

# The change

1. metasystem/docs/orchestration.md: the critique-budget passage
   (around lines 30 to 55) says the review budget is the tier's review
   rounds on the goal (0, 2, 3 by tier box), that the stop criterion is
   the material rule, that exhaustion opens no fresh budget (the token
   raise of the tuple is the only raise, under R-42-m0's ceiling), and
   that a tier-1 change lands as a receipted direct fix; name the tier
   boxes and the config keys once. Keep every other paragraph.
2. metasystem/skills/design-critique/SKILL.md, section "Round Budget
   and Exhaustion", and metasystem/skills/code-critique/SKILL.md,
   section "Round Budget and Exit": the shipped budget is no longer a
   fixed three; it is the goal's tier box; the material rule and the
   artifact-naming test are the stop; exhaustion goes to the close
   (pending part two) or to the human, never to a silent fourth round;
   the specimen paragraphs stay as history. Keep both skills' other
   sections.
3. metasystem/AGENTS.md: the intake sentence of the Work Contract (the
   backlog paragraph, or a new one-sentence bullet beside "The Goal
   Thread") says every backlog item is classified into a tier at
   intake and its budget follows the tier (R-54-m1), with the four risk
   answers pending from part two.
4. The obligation matrix: write a NEW record file, in the records
   directory under misc, named severity-tiered-rigor-obligation-matrix.md,
   in the template of metasystem/docs/design/design-obligation-gate.md
   (its table header) with one row per mechanism point of design
   revisions 2 to 4 (points 1 to 7 as amended by STR2-01 to -14,
   STR3-01 to -08, STR4-15 to -19): severity, design source, required
   behavior, owner file, code proof, test proof, runtime proof, status
   (done for parts one and three with the landed test names; pending
   for part two), next action. Do NOT edit files under plans/ (the
   conformance stage refuses delegate changes there); the orchestrator
   adds the one-line pointer to the design.
5. Nothing else. No code. Prose in the repository's voice: plain
   English, identifiers only where the reader must go there.

# Gate

`bash scripts/audit-metasystem.sh .` (the static audit that reads
these documents) green; `git diff --check` clean; the metasystem's
`metasystem validate preamble-quotes` green if any quoted block is
touched. Declare the boundary as every file that differs from main.

# Constraints

Wall-clock budget: 60 minutes; return before it ends even if something
is red, naming it. DESIGN-BEARING reach (the goal's chain law). Gap
rule: stop and report a gap with your proposed wording written out.
