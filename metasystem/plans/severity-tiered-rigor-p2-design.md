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

### Revision 4.1: the one design round folded (2026-09-03 22:20 local)

The design critique (records/misc/severity-tiered-rigor-p2-critique-r1.md)
returned nine material findings. Per Wido's stop criterion (R-60-m1)
this is the only round; each finding is folded here and every fold is a
named fixture obligation of the build brief. Amendments, in the order of
the findings:

- 001, the scales re-imported the shape. Novelty and exposure are
  re-scored by the ANSWER, never by the kind or location of the change.
  Novelty: 1 the approach has a landed precedent that an independent
  examiner passed (the basis names it: a chain root, a landed record or
  a fixture); 2 the parts are known but this combination has never been
  examined; 3 nothing in the tree or the records examined anything like
  it. Exposure: 1 one seat or one fixture can be affected; 2 every seat
  of the fleet; 3 every dispatch, landing or human decision that runs on
  the fleet. Path classes and the owner map are EVIDENCE the basis may
  cite, never an input to the derivation; the fixture proves that the
  same file set scored 1,1,1,1 derives tier 1 and scored 3,1,1,1 derives
  tier 3. 15 and 16 read with this paragraph in place of their scales.
- 002, the full gate has one consumer. `gateWidth: full` is read by three
  owners: the tier-1 receipt (13, unchanged); `RefuseChainMembership` in
  landing (the reviewed chain of a tier-2 or tier-3 goal under width
  full must carry a receipt whose command is the full battery, same
  receipt verb as 13); and the implementer brief composed by dispatch.sh,
  which spells the gate. The full battery is one canonical command:
  `scripts/agents/go-gate.sh --fast && scripts/agents/dispatch-fixtures.sh
  && scripts/agents/goal-cli-fixtures.sh`; the receipt verb matches that
  string exactly.
- 003, the post-claim raise. A raise is one transaction on the goal file:
  Risk and Tier are written, `Misclassified:` appended, the goal revision
  bumps, the claim is re-bound to the new revision with its epoch kept,
  and the approval is re-bound under the SAME human name with authority
  `raise=<opid>`; `ValidateApprovalRecord` admits `raise=` only when the
  history holds a Misclassified line with that opid whose `to` exceeds
  its `from` (a stricter promise never needs a new human word; a looser
  one always does). Roots already dispatched keep their `goalTier` and
  `gateWidth` (a chain runs under the tier it was dispatched with); the
  next dispatch reads the new tier; `ResolveGoalRevision` follows the
  re-bound claim.
- 004, the downgrade paths. Any edit that lowers ANY of the four scores,
  the derived tier, the set tier or the gate width is a human act by
  01's unapprove-edit-approve rule; editing an override back to the
  derivation counts as lowering when the set tier falls. The fixture
  tries all four paths as the pair and sees four refusals.
- 005, the digest of a goal without a Risk record is part one's digest
  unchanged: nil Risk contributes no bytes; a Risk record adds the
  `risk=` segment. Existing approvals stay valid in both modes; adding a
  Risk record to an approved goal is a raise (003) when the derived tier
  exceeds the set one, else a plain edit before approval or a human act.
- 006, the sweep selects every goal WITHOUT a Risk record, tiered or
  not; a tiered goal whose draft derivation is lower than its current
  tier is listed as a human decision in the draft, never lowered by the
  confirm.
- 007, evidence has a grammar: `--evidence root:<jobId>` (a job record
  under artifacts/agents/jobs bound to this goal), `finding:<jobId>/<id>`
  (present in that root's finding register) or `refusal:<code>` (one of
  the admission refusal codes); checked at edit time, refused otherwise.
  Lowering after unapprove needs `--why`, not evidence; both directions
  are countable from the history line.
- 008, no second register in this slice. Slice 2a writes only the
  `Misclassified:` history line; the register writer and the strict
  reader's second kind move to slice 2b, beside 10's accepted-risk
  writer, so the reader contract changes once. 17 reads without its
  last sentence.
- 009, the exception counter counts every member of the box: minutes,
  attempts, rounds, elapsed and active jobs.
- 010, cites re-read at the reviewed base; the alias goal file lives in
  records/goals.

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

### Revision 4.2 (2026-09-03 22:40 local): three part-two gaps answered

The slice-2b build stopped on the three binding gaps part one's brief
had recorded for part two (STR3-03 declared-outputs contract, STR3-04
obligation wire, STR3-05 accepted-risk transition). The answers are in
plans/severity-tiered-rigor-p2-build-2b-gap-brief.md and amend the
parent design's 07, 10 and 09: a byte grammar and parser for the
outputs file bound to the design blob (the equality with the build list
stays a review check, not a proof); Go-quoted obligation values; a
`goal accept-risk` verb under human proof whose accepted entries leave
the unresolved set before the close table is read.

### Revision 4.3 (2026-09-04 12:15 local): four slice-2a gaps answered

The slice-2a build's first round (str-p2-build-2a) stopped on four
contracts and left a clean tree. The answers are in
plans/severity-tiered-rigor-p2-build-2a-gap-brief.md and amend 006, 007,
003 and the build list's item 6: the classification draft's second field
is the four scores `s,n,e,a` and its tail the basis, the tool derives
the tier and marks `HUMAN-DECISION derived=<d>` on a tiered goal whose
derivation is lower; `refusal:<code>` admits exactly the exported list
`AdmissionRefusalCodes` (BUDGET_UNKNOWN, BUDGET_REFUSED, HAZARD_REFUSED,
RISK_UNANSWERED); a raise re-binds the claim's revision only and keeps
the epoch, the launch fence and the governed obligation; a goal-bound
root reads the tuple's review-round member capped at
`metasystem.budget.review-round-max`, a goal-free root reads that
ceiling alone.

### Revision 4.4 (2026-09-04 13:50 local): the raise keeps the spend, and its edges

The closing review of slice 2a (str-p2-build-2a-cc1 on tree 279d0cad)
found the raise of revision 4.3 opens the hole part two exists to close:
the budget projection counts only records at the claim's revision, so a
raise by the pair, which moves that revision, zeroed attempts, reserved
minutes and active jobs without a human word (STR2P2A-01). Amendment to
003 and 005: the claim carries an accounting revision. `Claimed` gains
`accountingRevision`, the goal revision at which the current spend
started; `goal claim` sets it to the claim revision, a human
`set-budget` moves it to the new revision (the reset the tuple approval
already implies), and a raise leaves it where it is. `ProjectBudget`
counts every job record, obligation state and governed run whose
`goalRevision` lies in `[accountingRevision, Claimed.Revision]`; a record
above the claim revision stays BUDGET_UNKNOWN as today. A file whose
claim has no accounting revision reads it as the claim revision. A raise
also lifts the tuple's `reviewRoundLimit` to the new tier's box member
when the stored member is lower (rigor follows the tier; STR2P2A-09) and
touches no spend member, so the lift is not a budget exception.

Edges (STR2P2A-03, -05, -08): `goal edit` refuses a bare `--tier`
exactly as `goal open` does and writes the TierOverride history line
with `--why` for an override above the derivation; a raise combined
with an override writes both the Misclassified and the TierOverride
lines; on a goal whose set tier is above its derivation, a raise that
omits `--tier` keeps the set tier when it is at or above the new
derivation instead of reading it as a lowering. The parser never
indexes history at a zero claim revision: a claim revision of zero with
an Obligation line stays the existing "obligation budgetRevision does
not bind" problem. The elapsed-limit member compares as a duration,
never as a string (STR2P2A-02). Recovery replay of an edit through the
goal package does not re-append the counselor register line; that line
is written by the command layer only, recorded here as known, not
fixed in this slice.
