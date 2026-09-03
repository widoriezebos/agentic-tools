# severity-tiered-rigor-p2 — design: the risk basis (revision 4 of the tiering design)

Parent design: plans/severity-tiered-rigor-design.md (revisions 1 to 3,
owned by goal severity-tiered-rigor on m2). This file is revision 4 of
that design, carried under goal severity-tiered-rigor-p2 on m3 because
the two goals land in parallel; section numbers continue the parent's.

Wido, 21:45 local, verbatim: "I want the second part to include the
risk scoring 'Risk is four separate questions, never the shape of the
change' because without it is is a stupid deterministic rule at best."
Revisions 2 and 3 pick the tier from the shape of the change (R-54-m1's
three lines). This revision makes the tier DERIVED from four recorded
answers (docs/paper/06-proof-over-trust.md line 51 and
11-economy.md line 23, verbatim questions) and keeps everything else:
the tier stays the enforcement key that part one builds; the ladder,
the boxes and the refusals read it unchanged. Built as the first slice
of part two, on part one's landed tree, under goal
severity-tiered-rigor-p2. Code facts read at HEAD 99b44caa.

### STR4-RISK-RECORD-15: four scores and a basis on the goal

Amends 01. `GoalFile` gains `Risk *RiskRecord` with `Severity`,
`Novelty`, `Exposure`, `Accumulation` (each 1, 2 or 3) and `Basis`
(one non-empty line), rendered as one line `- Risk: severity=<n>
novelty=<n> exposure=<n> accumulation=<n> basis="<text>"` above the
`- Tier:` line. The four questions, and what each score means:
- severity, how severe could the harm be if the change is wrong: 1
  visible and reversible on one machine; 2 recoverable but it crosses
  a proof, authority, secrets, data or external-side-effect boundary
  (the bounded/severe line of round 1); 3 irreversible, or it moves
  authority, secrets or a landing bar.
- novelty, how unfamiliar is the approach to the system and its
  independent examiners: 1 an existing owner whose existing checks
  cover the change; 2 new logic inside an existing owner; 3 a new law,
  verb, schema, seam or role.
- exposure, how many users or systems can it affect: 1 one machine or
  one fixture; 2 every seat of the fleet; 3 every dispatch or every
  landing (shared law: internal/goal, dispatch, landing, validate,
  config, the two wrappers, metasystem.conf).
- accumulation, how much change has accumulated since the last broad
  examination of the touched area: 1 broadly examined since its last
  change; 2 several landings since; 3 the area's last broad
  examination predates the goal's own base.
`ApprovalDigest` (file.go:101) hashes `risk=<s>,<n>,<e>,<a>` between
`tier=` and the tuple, so the approving human sees the scores.

### STR4-TIER-DERIVATION-16: one fixed table, an override with a record

Amends 01 and 02. `goal open` and `goal edit` take `--risk
severity=<n>,novelty=<n>,exposure=<n>,accumulation=<n> --basis
"<text>"` and derive the tier: the highest of severity, novelty and
exposure; accumulation 2 or 3 raises a derived tier 1 to 2 (a broad
look is one code review, never a design round) and selects the FULL
battery (go test ./..., the fixture scripts) as the gate of every
build under the goal in place of the area's tests; the gate choice is
rendered on the root by build.go as `gateWidth: area|full` beside
`goalTier`, and the tier-1 landing class of 13 requires a receipt whose
command is the full battery when the width is full. `--tier` without
`--risk` is refused: "answer the four questions: --risk ... --basis".
`--tier` with `--risk` and a value different from the derivation is an
override: `--why` required; a tier ABOVE the derivation any actor may
set; a tier BELOW it is a human act (`--by <human>`, proof as
`SetBudgetApproved`); either writes `TierOverride: derived=<n> set=<m>`
on the history line. The classify-sweep draft of 02 carries the four
scores and the basis per goal, never a bare tier; the confirm derives.

### STR4-MISCLASSIFICATION-17: a wrong answer is a defect with a record

Amends 01's edit rule. After approval or claim, `goal edit --risk` that
RAISES the derived tier is admitted to the owning pair and the human
with `--evidence <ref>` (a chain root, a finding id or a refusal code)
and appends `Misclassified: from=<n> to=<m> evidence=<ref>` to the
history line and re-binds the digest as 02's confirm does; it does not
displace the claim. A change that LOWERS it keeps 01's rule: unapprove,
edit, approve, a human act. `counselor.AppendMisclassification` (the
writer of 10, one more kind `misclassification`, id `mc-<goal>-<opid>`,
facts = the two derivations and the evidence) makes both directions
countable in records/counselor/accepted-risk-register.jsonl's sibling
file misclassification-register.jsonl, same strict schema.

### STR4-MARKING-MODE-18: the risk gate is born marking

Amends 02's marker. The refusal of a tiered goal without a Risk record
is governed by one tracked key, `metasystem.budget.risk-gate=mark|enforce`
(config/budget.go beside `TierBox`; validate.go refuses any other
value), landed as `mark`. In mark mode `delegate` proceeds and prints
`RISK_UNANSWERED goal=<id> tier=<n> next: goal edit --risk`, and the
steward's claimed-goal-appetite line (internal/steward/health.go:55)
carries `riskUnanswered=<count>`; in enforce mode `delegate` refuses
with the same code and next step. Owner: the coordinator seat that
lands part two; review date 2026-09-06 (Wido's word review); known-bad
case: goal fable-model-alias, run as tier 3 where the four answers score
2,2,2,1 (tier 2); appeal route: `goal edit --risk`. Activation is one
conf line on Wido's word; goals opened after the TierLaw marker of 02
still need a tier, so mark mode never reopens the tierless case.

### STR4-EXCEPTION-COUNTER-19: repeated exceptions are a signal, not a gate

Amends 03. `GoalFile` gains `BudgetExceptions uint16`, rendered
`- BudgetExceptions: <k>`, incremented by every `SetBudgetApproved`
(verbs.go:585) whose tuple exceeds the tier box in minutes, attempts or
rounds; `goal show` prints it and the appetite line carries
`exceptions=<k>`; at k >= 2 the line ends with `repeated exception:
defect signal`. No refusal anywhere (11-economy.md: a forecast never
authorizes, a repeated exception is evidence for a retrospective).

### Fixtures of this revision (goal-cli-fixtures.sh and Go tests)

Open with `--risk` derives 1, 2, 3 and the accumulation floor; `--tier`
alone refused; override above by the pair recorded; override below by
the pair refused and by the human recorded; digest differs by risk; raise
after claim by the pair with evidence writes Misclassified and one
register line; lower after claim refused; mark mode passes with the
RISK_UNANSWERED line, enforce refuses; an unknown gate value refused at
load; two over-box set-budgets print the defect signal; the sweep draft
carries scores; `gateWidth: full` on a root under accumulation 2.

### Build list, revision 4: part two is two slices under goal severity-tiered-rigor-p2

- Slice 2a, the risk basis (15 to 19). Files: internal/goal/file.go,
  approval.go, verbs.go; cmd/metasystem/goalsync_mutations.go;
  internal/config/budget.go and validate.go; internal/dispatch/build.go,
  servinggoal.go, admission.go; internal/steward/health.go (appetite
  line); internal/counselor/register.go; scripts/agents/dispatch.sh,
  goal-cli-fixtures.sh.
- Slice 2b, the material stop and the close: revision 3's part two,
  unchanged. Parts one and three stay under severity-tiered-rigor (m2);
  part four's docs land with 2b and name the four questions.
