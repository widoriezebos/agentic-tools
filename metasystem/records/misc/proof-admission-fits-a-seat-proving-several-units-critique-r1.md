# Design critique: proof admission fits a seat proving several units, round 1

Nine material findings require rework before implementation. The design separates
authority from accounting in the attempt record, but it does not carry that
separation through test-plan selection, budget episodes, projection dimensions,
all readers, or the claim-epoch authority proof.

## Protocol record

- Schema version: 3
- Job id: not exposed by this direct critic task
- Round: 1
- Runtime: Codex
- Session id: not exposed
- Model: not exposed; no different model is claimed
- Mode: design critique
- Reviewed checkout commit: `e70a832e27b659074118d73d350837621ce40cc1`
- Verified current `origin/main`: `912ea6f11ca1c59a09642b984725ce203b947e6c`
- Claimed session id: null
- Claimed model: null
- Material finding count: 9

## Findings

### PA-001 — Critical — The charged goal does not select the proof floor

Material: yes

Evidence: read. Decision D2 makes `--goal` the claimed authority goal C and
`--charge` the candidate goal X
(`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r1.md:73`).
Today `prepareTesting` resolves `request.GoalID` before admission and uses that
goal's risk to select the plan (`metasystem/cmd/metasystem/test.go:213`,
`metasystem/cmd/metasystem/test.go:310`). `testingGoalRisk` reads risk and
accounting revision from that same claimed goal
(`metasystem/cmd/metasystem/test.go:1338`). The trusted policy engine is also
given only `request.GoalID` (`metasystem/cmd/metasystem/test.go:431`). Nothing
in U2a or witness W1 changes or tests this selection.

A candidate for a high-risk X can therefore be planned under a lower-risk C.
That is a proof-floor regression forbidden by R-115-m1e, not merely inaccurate
accounting. Specify that the candidate/accounting goal supplies risk, required
mode, accounting revision, and the trusted-engine `--goal`; the authority goal
supplies only claim and launch authority. Add a witness with different risks on
C and X that fails if C's risk reaches `testpolicy.Select`.

Rigor: severe. Facts: `{"local":true,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}`. Reopening trigger: any plan, trusted-policy-engine call, or required-mode check still receives the authority goal when a distinct charged goal is named.

### PA-002 — Critical — Charging an unclaimed goal is erased when that goal is claimed

Material: yes

Evidence: read. D2 assigns an unclaimed X the revision where its budget was
approved or set
(`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r1.md:76`).
On a later claim, `newClaimRecord` sets `AccountingRevision` to the new claim
revision (`metasystem/internal/goal/verbs.go:323`), and `Claim` always calls that
bind after touching the goal (`metasystem/internal/goal/verbs.go:991`). The
budget projection then skips every attempt whose accounting revision is older
than the claim's accounting revision (`metasystem/internal/dispatch/budget.go:519`,
`metasystem/internal/dispatch/budget.go:525`). No accounting unit changes the
claim transition.

Thus proofs charged to approved, unclaimed X disappear from X's spend as soon
as X is claimed, reopening its attempt and minute budget. Define one durable
accounting episode that starts before the first unclaimed charge and survives
the later claim/reclaim transition unless a real human-approved budget reset
starts a new episode. Add a charge-X, then claim-X witness that observes the
same attempts and reserved minutes before and after the claim.

Rigor: severe. Facts: `{"local":true,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}`. Reopening trigger: claiming or reclaiming a charged goal can still advance the accounting boundary past pre-claim charged attempts without a budget change.

### PA-003 — Critical — One projection key cannot enforce the proposed split gates

Material: yes

Evidence: read. D3 assigns elapsed and active-job gates to authority C, but
attempt and reserved-minute gates to accounting goal X
(`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r1.md:84`).
The current projection first filters a proof attempt once by `GoalID`,
`AccountingRevision`, and `BudgetEpoch`
(`metasystem/internal/dispatch/budget.go:517`,
`metasystem/internal/dispatch/budget.go:525`,
`metasystem/internal/dispatch/budget.go:528`), then increments attempts,
reserved minutes, and active jobs together
(`metasystem/internal/dispatch/budget.go:571`). The reservation currently gets
the authority projection's `BudgetEpoch`
(`metasystem/cmd/metasystem/proof_run.go:509`,
`metasystem/cmd/metasystem/proof_run.go:521`). D1 adds an accounted goal and
revision, but no accounted budget epoch and no per-dimension projection rule.

Changing the existing filter to X loses C's live active job; leaving it on C
does not charge X. Retargeting the single `BudgetEpoch` to X also becomes
ambiguous when C and X have different weight epochs. Define separate authority
and consumption lenses, including which budget epoch each consumes, and how a
single live attempt contributes to both projections. The witness must give C
and X distinct revisions/weight epochs, keep the attempt live, and prove C's
active-job refusal and X's attempt/minute refusal independently.

Rigor: severe. Facts: `{"local":true,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}`. Reopening trigger: one pre-loop filter still controls both authority concurrency and accounting consumption, or a charged attempt carries no unambiguous accounting weight epoch.

### PA-004 — High — The two-goal admission has no chosen linearization rule

Material: yes

Evidence: read. The existing launch resolves authority C, acquires C's
goal-revision lock, then acquires the proof-attempt mutation lock
(`metasystem/cmd/metasystem/proof_run.go:430`,
`metasystem/cmd/metasystem/proof_run.go:453`). It rechecks only C before it
builds the reservation (`metasystem/cmd/metasystem/proof_run.go:505`). D2 adds
eligibility and revision decisions for X, but D2/D3 do not say whether those
facts are an admission snapshot, must be rechecked, or are protected by a
second lock. No concurrent state-change witness exists.

An implementer must guess what happens if X is parked, fenced, claimed
elsewhere, or re-budgeted between its eligibility read and publication of the
attempt. The choices produce different accepted attempts and accounting
revisions. Name the linearization point and deterministic lock/recheck order
for C, X, and the attempt store. Prove it with an injected barrier, never a
sleep: a concurrent X transition must have one specified before-or-after
outcome.

Rigor: severe. Facts: `{"local":true,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":true,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":true}`. Reopening trigger: a charged attempt can still publish after X changes state or accounting episode without a specified linearizable before-or-after result.

### PA-005 — High — The set-budget layer cannot tell a holder epoch from an arbitrary positive epoch

Material: yes

Evidence: read. D6 says a live claim is restamped only when
`classification.Holder` is true
(`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r1.md:115`).
The command edge copies `classification.ClaimEpoch` into the request but drops
the `Holder` fact (`metasystem/cmd/metasystem/goalsync_mutations.go:525`,
`metasystem/cmd/metasystem/goalsync_mutations.go:529`). `VerbRequest` carries a
number and caller class, not a holder attestation
(`metasystem/internal/goal/verbs.go:197`). The set-budget tail therefore sees
only `r.ClaimEpoch` and whether a human name is present
(`metasystem/internal/goal/verbs.go:1241`). W10's proposed direct requests with
epochs 1 and 6 do not prove which number was authenticated as the holder's.

Carry the classified holder fact across the command/domain boundary as a
trusted field or capability, and have the goal owner reject contradictory
combinations. Then test a positive non-holder epoch, an authenticated holder
epoch, a human outside the lease, and the attorney path. Inferring holder status
from the number or from caller class would rebuild the current authority flaw.

Rigor: severe. Facts: `{"local":true,"recoverable":true,"proofBoundaryCrossed":false,"authorityBoundaryCrossed":true,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}`. Reopening trigger: internal goal mutation still decides epoch replacement from `ClaimEpoch > 0`, human name, or caller class without an authenticated holder fact.

### PA-006 — High — The legacy candidate-tree rule deliberately preserves the reported defect

Material: yes

Evidence: read. D4 treats every retained attempt without the new field as if it
belonged to the request's tree
(`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r1.md:91`).
Current top-level testing attempts already retain the candidate tree as the
first proof identity input (`metasystem/cmd/metasystem/test.go:811`), and the
identity inputs are durable attempt data
(`metasystem/internal/proofrun/attempt.go:54`,
`metasystem/internal/proofrun/attempt.go:143`). Terminal testing attempts also
retain `TestResult.CandidateTree`
(`metasystem/internal/proofrun/test_result.go:171`). The design neither derives
legacy trees from those sources nor validates that its new duplicate field
agrees with them; current proof-identity validation checks only shape and digest
(`metasystem/internal/proofrun/attempt.go:499`).

After rollout, retained T1 failures can still make a T2 request ambiguous, in
direct conflict with the goal's “another candidate's failures never” DONE
(`metasystem/plans/goals/proof-admission-fits-a-seat-proving-several-units.md:8`).
Choose one authoritative candidate-tree accessor: validate the new field
against existing evidence, derive legacy terminal/direct attempts where the
tree is already recorded, and define a fail-closed treatment only for genuinely
unidentifiable legacy attempts. W4 must assert derivation rather than pretending
the old attempt came from the current tree.

Rigor: unproven. Facts: `{"local":true,"recoverable":true,"proofBoundaryCrossed":false,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}`. Reopening trigger: any identifiable legacy T1 failure is still treated as a T2 failure, or two retained fields can disagree about the candidate tree.

### PA-007 — High — The implementation map omits readers whose meaning must change

Material: yes

Evidence: read. D1 says retry and reuse history key on the accounting pair, but
U1 names only `attempt.go` and `budget.go`
(`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r1.md:220`).
Exact result reuse still filters the old goal/accounting fields in
`metasystem/internal/proofrun/test_result.go:357`. Moved-input diagnostics do
the same in `metasystem/cmd/metasystem/test.go:1145` and
`metasystem/cmd/metasystem/test.go:1198`. `job prove-round` passes the job goal
as `--goal` and discovers the resulting attempt by `Attempt.GoalID`
(`metasystem/cmd/metasystem/prove_round.go:99`,
`metasystem/cmd/metasystem/prove_round.go:124`,
`metasystem/cmd/metasystem/prove_round.go:214`). Consumption-earned budget
extension likewise recognizes advancement only by `Attempt.GoalID`
(`metasystem/internal/dispatch/budget_extension.go:270`,
`metasystem/internal/dispatch/budget_extension.go:284`). None of
`test_result.go`, `prove_round.go`, or `budget_extension.go` appears in a unit.

Inventory every attempt-field reader and classify it as authority,
candidate/accounting, or both. Assign every changed reader and focused witness
to a unit under 300 changed lines. In particular, prove exact reuse, moved-input
diagnostics, round proof discovery, extension evidence, receipts, stop batches,
terminal locking, and reconciliation against their intended side of the split.
Otherwise the implementation can compile while retaining mutually
incompatible meanings of `GoalID`.

Rigor: severe. Facts: `{"local":true,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}`. Reopening trigger: any attempt-field consumer remains unclassified, or a unit's file boundary omits a consumer that must follow the accounting pair.

### PA-008 — Medium — The promised `resumed=` history fact has no schema owner

Material: yes

Evidence: read. D5 promises one `set-budget` history operation carrying
`resumed=<stopId>`
(`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r1.md:102`),
but U4 omits `internal/goal/file.go`. `HistoryLine` has no resumed field
(`metasystem/internal/goal/file.go:378`); its parser recognizes `stopId=` but
not `resumed=` (`metasystem/internal/goal/file.go:1769`,
`metasystem/internal/goal/file.go:1824`); and validation refuses a `stopId=` on
`set-budget` (`metasystem/internal/goal/file.go:854`).

An implementer can smuggle the stop id into free prose, misuse the invalid
`stopId` field, or invent a structured grammar change. Specify the structured
field, parser, renderer, validation, replay/recovery behavior, and exact file
test, then re-slice U4 if those changes exceed its 280-line allocation.

Rigor: unproven. Facts: `{"local":true,"recoverable":true,"proofBoundaryCrossed":false,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}`. Reopening trigger: a one-step rebudget can land without a strictly parsed history field binding it to the cleared stop id.

### PA-009 — Medium — Several rules do not have witnesses for all behavior they require

Material: yes

Evidence: read. The binding instruction says every rule names the witness that
fails without it. R3 requires six eligibility properties, but W3 covers only
parked and fenced goals
(`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r1.md:153`).
R5 says cross-tree live duplicates remain unchanged, but W5/W5b cover only
failure and success (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r1.md:165`).
R8 requires both stop-batch completion and the no-other-live-claim rule, but W8
tests only the batch (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r1.md:188`).
D3 also retains the authority active-job gate while its named test covers only
elapsed (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r1.md:224`).

Add a failing witness for every listed clause: queued/unapproved/unbudgeted,
done or absent, and claimed-on-another-machine charged goals; cross-tree live
duplicate behavior; the other-live-claim rebudget refusal; and authority
active-job admission. These are required proof, not optional test polish.

Rigor: severe. Facts: `{"local":true,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}`. Reopening trigger: any rule still has a branch or invariant that can be removed without making its named witness fail.

### PA-010 — Low — The design names an old `origin/main`, but the reviewed code is unchanged

Material: no

Evidence: ran. The design calls `e70a832e2` current `origin/main`, while this
review observed checkout HEAD `e70a832e2` and current `origin/main` `912ea6f11`.
`git diff --quiet HEAD..origin/main -- metasystem/cmd metasystem/internal metasystem/scripts`
reported no differences, so every cited code blob is identical and an
implementer would build the same code. Correct the provenance when revising the
page, but this does not keep the loop open.

## Recommended answers to the page's open questions

- Q1: No; `Claim` checks only the target goal and does not scan other goals (`metasystem/internal/goal/verbs.go:943`), so keep the multiple-claim branch and explicitly refuse a charged goal claimed on another machine.
- Q2: No; `ProjectBudget` returns unknown when `Claimed` is absent (`metasystem/internal/dispatch/budget.go:261`), so add a consumption-only projection rooted in a durable approved/kept episode and preserve it through claim as required by PA-002.
- Q3: Land the corrected epoch-preservation owner before one-step set-budget and before the neighbouring restamp/health goal consumes it; do not expose the helper until PA-005's authenticated-holder contract is fixed.
- Q4: `--under` gets the epoch only when its process is the classified holder; otherwise it carries zero (`metasystem/cmd/metasystem/goalsync_mutations.go:529`), so preserve the recorded epoch for the non-holder and carry an explicit holder attestation for replacement.
- Q5: Yes; `job prove-round` directly calls `test run` and its record names the job goal (`metasystem/cmd/metasystem/prove_round.go:99`), so make that goal the candidate/accounting goal, resolve authority separately, and match the new attempt by its accounting side.
- Q6: Treat the m1c transcript only as corroborating provenance; the mutually refusing code paths are already sufficient design evidence, and a transcript discrepancy must not change the rules or witnesses.
- Q7: Yes; `TestHCL03EveryCodeRowed` scans production Go refusal tokens (`metasystem/internal/refusal/register_test.go:20`), and the new codes should also get exact row/site assertions rather than relying only on the broad collector.
- Q8: Keep elapsed on the authority claim and reserved minutes on X; that bounds this feature without inventing an elapsed clock for an unclaimed goal, and any second elapsed contract needs Wido's ruling.

## Evidence and gaps

- Ran: `git rev-parse HEAD`, `git rev-parse origin/main`, the scoped quiet diff,
  and SHA-256 reads of the design and brief. The design digest reviewed was
  `4086111e70ea87b0ba40f2cbdc5d88624286c7fd0602720c95d6ac96c05fba5e`.
- Ran: the permitted live `goal show`; it reported state approved, Tier 2,
  revision 38, a two-round stored review limit, and synchronized tip
  `912ea6f11ca1c59a09642b984725ce203b947e6c`.
- Read: the full design, brief, goal, binding rulings, critic role, critique
  skill, design principles, obligation gate, and every code path cited above.
- Not run: Go tests, metasystem tests, fixture beds, or any command that starts,
  signals, or kills a workload process, as instructed.
- Gap: the m1c transcript named by Q6 was not present in the supplied context;
  the recommendation is inferred from the source paths and the goal's retained
  incident statement.

Proposed receipt: `RECEIPT|type=design-critique|goal=proof-admission-fits-a-seat-proving-several-units|outcome=material-findings|verify=read-only-origin-main-source-trace|corrections=0|skills=design-critique|delegate=none`

VERDICT: rework
