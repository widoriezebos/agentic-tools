Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-04

# Goal

Build slice 2a of the tiering machinery's part two, the risk basis,
exactly as designed in metasystem/plans/severity-tiered-rigor-p2-design.md
(revision 4, sections STR4-15 to STR4-19, as amended by revision 4.1
"the one design round folded"; where 4.1 amends a section, 4.1 wins)
and its dispositions record
metasystem/records/misc/severity-tiered-rigor-p2-critique-r1.md. Read
the parent design metasystem/plans/severity-tiered-rigor-design.md
sections STR2-TIER-AUTHORITY-01, STR2-GOAL-SWEEP-02 and
STR2-BUDGET-TUPLE-03 first: part one built them and they are in your
base; this slice amends them. Wido's order, verbatim: "I want the
second part to include the risk scoring 'Risk is four separate
questions, never the shape of the change' because without it is is a
stupid deterministic rule at best." and "I want this done in 16 hours
MAX". The design loop is closed by his stop criterion (R-60-m1): the
nine round-1 findings are fixture obligations below, each a test that
must exist and pass, and a Fable code review follows this build.

# What to build (the design is the spec; this is the index)

1. The risk record (15, amended by 4.1/001 and 005): `Risk *RiskRecord`
   on `GoalFile` in metasystem/internal/goal/file.go, one rendered line
   `- Risk: severity=<n> novelty=<n> exposure=<n> accumulation=<n>
   basis="<text>"` above the `- Tier:` line; the approval digest adds
   the `risk=<s>,<n>,<e>,<a>` segment ONLY when a Risk record exists (a
   nil Risk contributes no bytes: part one's digests stay valid).
2. The derivation (16, amended by 4.1/001, 002 and 004): `goal open`
   and `goal edit` take `--risk severity=,novelty=,exposure=,accumulation=
   --basis "<text>"` (metasystem/internal/goal/verbs.go,
   metasystem/cmd/metasystem/goalsync_mutations.go); tier = highest of
   severity, novelty, exposure; accumulation 2 or 3 raises a derived 1
   to 2 and sets the gate width full; `--tier` without `--risk` refused
   with "answer the four questions: --risk ... --basis"; an override
   above the derivation by any actor with `--why`, below only by the
   human (`--by <human>`, proof as `SetBudgetApproved`), either writing
   `TierOverride: derived=<n> set=<m>`. ANY edit that lowers any score,
   the derived tier, the set tier or the gate width is a human act.
   `gateWidth: area|full` is written on the root by
   metasystem/internal/dispatch/build.go beside `goalTier` and read by
   three owners: the tier-1 receipt check (part three, on the other
   seat; do not build it), the chain observation `observeChain` in
   metasystem/internal/landing/observe.go (a reviewed chain under width full needs a receipt whose
   command is the canonical full battery; if part three's receipt verb
   is absent from your base, write the check against the receipt file
   form named in STR2-TIER1-EVIDENCE-13 and name the seam in the
   return) and the implementer brief composed by
   metasystem/scripts/agents/dispatch.sh, which spells the gate. The
   canonical full battery string, exactly:
   `scripts/agents/go-gate.sh --fast && scripts/agents/dispatch-fixtures.sh && scripts/agents/goal-cli-fixtures.sh`.
   The classify sweep of 02 (metasystem/internal/goal/approval.go)
   selects every goal WITHOUT a Risk record, carries the four scores
   and the basis per goal, and lists a tiered goal whose derivation is
   lower than its current tier as a human decision; the confirm derives
   and never lowers.
3. Misclassification (17, amended by 4.1/003, 004, 007 and 008): after
   approval or claim, `goal edit --risk` that RAISES the derived tier
   is one transaction: Risk and Tier written, `Misclassified: from=<n>
   to=<m> evidence=<ref>` appended, the goal revision bumped, the claim
   re-bound to the new revision with its epoch kept, the approval
   re-bound under the same human name with authority `raise=<opid>`;
   the approval-record validation that part one's 01 binds (named
   `ValidateApprovalRecord` in the design; follow the tree) admits
   `raise=` only when the history holds
   a Misclassified line with that opid whose `to` exceeds its `from`.
   `--evidence` grammar: `root:<jobId>` (a job record under
   artifacts/agents/jobs bound to this goal), `finding:<jobId>/<id>`
   (present in that root's finding register), `refusal:<code>` (one of
   the admission refusal codes of metasystem/internal/dispatch/admission.go);
   checked at edit time, refused otherwise. Roots already dispatched
   keep `goalTier` and `gateWidth`; the next dispatch reads the new
   tier; `ResolveGoalRevision` (metasystem/internal/dispatch/servinggoal.go)
   follows the re-bound claim. The raise calls
   `counselor.AppendMisclassification` (built by slice 2b in
   metasystem/internal/counselor/register.go; if it is absent from your
   base, write the call behind one function in the goal package and
   name the seam in the return).
4. Marking mode (18): tracked key `metasystem.budget.risk-gate=mark|enforce`
   in metasystem/internal/config/budget.go beside `TierBox`, any other
   value refused by metasystem/internal/config/validate.go, landed as
   `mark` in metasystem/metasystem.conf with the comment
   `# Risk gate (R-73-m3): mark until the fleet's open goals carry the four answers; enforce on Wido's word.`
   In mark mode `delegate` proceeds and prints `RISK_UNANSWERED
   goal=<id> tier=<n> next: goal edit --risk`; in enforce mode it
   refuses with the same code and next step
   (metasystem/internal/dispatch/admission.go); the steward's
   claimed-goal-appetite line (metasystem/internal/steward/health.go)
   carries `riskUnanswered=<count>`.
5. The exception counter (19, amended by 4.1/009): `BudgetExceptions
   uint16` on `GoalFile`, rendered `- BudgetExceptions: <k>`,
   incremented by every `SetBudgetApproved` whose tuple exceeds the tier
   box in ANY of its five members; `goal show` prints it, the appetite
   line carries `exceptions=<k>`, and at k >= 2 the line ends with
   `repeated exception: defect signal`. No refusal anywhere.
6. The review-round seam (slice 2b's recorded note STR2P2B-03):
   `goalReviewRoundLimit` in metasystem/internal/dispatch/build.go
   still returns the constant three with the comment that part one's
   tuple member replaces it. Replace the body: the limit is the goal's
   `reviewRoundLimit` budget member (read through the goal record the
   chain is bound to, as the other tuple members are read), and
   `CritiqueBudgetRebind` keeps the tier that `ResolveGoalRevision`
   now returns instead of discarding it. One test: a goal whose tuple
   says nine rounds folds a fourth critic round; the constant is gone.

# Fixture obligations (each is a named test that exists and passes)

Every fixture in the p2 design's "Fixtures of this revision" paragraph,
plus these nine from the round-1 record:

- STR4-R1-SHAPE-FREE-DERIVATION: the same file set scored 1,1,1,1
  derives tier 1 and scored 3,1,1,1 derives tier 3.
- STR4-R1-FULL-WIDTH-CHAIN: a tier-2 chain under width full is refused
  with an area receipt and admitted with a full-battery receipt.
- STR4-R1-RAISE-TRANSACTION: a raise after claim; a root dispatched
  before it keeps its goalTier, the next dispatch reads the new tier,
  and the approval validation admits the re-bound digest only with the
  Misclassified line present (removed line: refused).
- STR4-R1-FOUR-DOWNGRADES-REFUSED: as the pair, lower one score, the
  derived tier, the set tier (override edited back) and the width:
  four refusals.
- STR4-R1-NIL-RISK-DIGEST: a goal approved before this slice (no Risk
  line) validates unchanged in mark and enforce mode.
- STR4-R1-SWEEP-BACKFILL: a tiered goal without a Risk record appears
  in the sweep draft; a draft derivation lower than its tier is listed
  as a human decision and the confirm does not lower it.
- STR4-R1-EVIDENCE-GRAMMAR: each of the three kinds accepted with an
  existing referent and refused with a missing one.
- STR4-R1-MISCLASSIFICATION-KIND: the raise writes one register line of
  kind `misclassification` (or, if the writer is absent from your base,
  the seam test that the call site exists and is invoked once).
- STR4-R1-FIVE-MEMBER-EXCEPTIONS: an over-box elapsed limit and an
  over-box active-job limit each increment the counter.

# Gate

gofmt, go vet, go build; `GOFLAGS=-buildvcs=false go run
honnef.co/go/tools/cmd/staticcheck@2025.1 ./...` silent
(metasystem/scripts/agents/go-gate.sh `--fast` is the same check and
the commit gate refuses a chain on any line); `go test -count=1
-timeout 30m $(go list ./... | grep -v /internal/goal$)` green (main
is green in every package since parts one and three landed); for
internal/goal run the tests you add and touch by name
(`-run`), the whole package takes 17 minutes on this host and the
dispatching seat runs it after your return;
metasystem/scripts/agents/goal-cli-fixtures.sh and
dispatch-fixtures.sh each once if your sandbox can run them (report
the exact refusal if not; the seat reruns them). No benchmarks (R-31), no
sleeps (R-35). Leave the work in your working tree, stage nothing, do
not run the commit wrapper. diffBoundary: metasystem/internal/goal,
metasystem/internal/config, metasystem/internal/dispatch,
metasystem/internal/landing, metasystem/internal/steward,
metasystem/internal/counselor, metasystem/cmd/metasystem,
metasystem/scripts/agents/dispatch.sh,
metasystem/scripts/agents/goal-cli-fixtures.sh,
metasystem/scripts/agents/dispatch-fixtures.sh, metasystem/metasystem.conf.
Nothing under metasystem/plans, metasystem/records or metasystem/memory.
Paste the final gate lines and the list of new test names in your
return.

# Constraints

Wall-clock budget: 45 minutes. Build items 1, 2 and 4 first, then 3,
5 and 6. Return by minute 40 whatever the state: leave the tree
consistent with the fast gate green and list exactly which items and
fixtures are unbuilt; a follow-up finishes them. A round that ends at
the cap without a return is charged and proves nothing; the return is
the deliverable. Version-2 implementer JSON. If a design cite
has drifted against the tree at your base, follow the tree and name
the drift in the return; do not redesign.

# Gap Rule

stop and report a gap; never fill it silently. The grain of the rule:
it stops you for a contract that would change law (a new authority, a
new refusal, a new landing bar, a schema the fleet reads) that neither
the design nor its dispositions answer. It does not stop you for a
mechanical choice (a field mapping, a selector, a flag name, a lock to
reuse, an order of two writes): choose what the tree already does
nearest the seam, record the choice in your return under `decisions`
(one line each: the choice, the alternative, the reason), and build. A
choice recorded in the return is not silent.
