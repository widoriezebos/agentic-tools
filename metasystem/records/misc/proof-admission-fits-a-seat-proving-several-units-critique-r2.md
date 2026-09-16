# Design critique: proof-admission-fits-a-seat-proving-several-units, round 2

Six material findings remain. Five cross the proof boundary; three also cross the authority boundary. The design is not ready to build.

## Protocol record

- Protocol: design-critique v3
- Goal: `proof-admission-fits-a-seat-proving-several-units`
- Round: 2 of 3
- Mode: design critique
- Runtime: Codex
- Reviewed design: `metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md`
- Reviewed repository state: `origin/main` at `faf9d057cd7cdb8a462173fe796704dc15e83db4`
- Claimed session/model: not exposed by this runtime
- Evidence obtained: design, brief, goal, prior critique, binding rulings, role and skill protocols, and the cited source and test files were read; `HEAD` and `origin/main` were compared; the design's older source commit and current `origin/main` are byte-identical over the cited implementation paths.
- Evidence deliberately not run: tests, fixture beds, and all process-managing commands, per the seat's instruction.
- Evidence gap: the permitted `bin/metasystem goal show --id proof-admission-fits-a-seat-proving-several-units` could not be run because this worktree has no `bin/metasystem`; the goal file was read directly.
- Materiality test: Would an implementer working from this design build something DIFFERENT, or WRONG, because of this finding?

## Prior material findings

| Prior finding | Status in revision 2 |
|---|---|
| PA-001, charged goal does not select its plan | **Closed.** D2, R1/W1, and U3a make `--goal X` select X's plan while authority selection is separate (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:96-110,216-221,360-361`). |
| PA-002, an unclaimed charged goal is erased at claim time | **Not closed.** D3 recognizes the reset problem, but chooses `Approved.Revision` as the episode key and relies on an incorrect description of automatic budget mutation (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:112-124`). See PA-002-R2. |
| PA-003, one projection cannot serve authority and candidate accounting | **Not closed.** D4 introduces two named lenses, but specifies the candidate lens as though the budget had one attempt store and leaves earned budget extension without an authorized candidate transition (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:126-146`). See PA-003-R2. |
| PA-004, no linearization boundary for two-goal admission | **Not closed.** D5 adds two locks and a reread, but ordinary candidate mutations do not participate in that lock protocol, and the proposed order is cyclic for reciprocal charges (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:148-162`). See PA-004-R2. |
| PA-005, epoch authority is asserted rather than attested | **Closed for the live command path.** D7 and R12/W14 require the command layer to attest the classified holder and require mutation helpers to reject bad authority (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:177-196,296-305`). Its recovery claim creates a separate defect; see PA-013. |
| PA-006, the legacy tree fallback preserves the defect | **Closed.** D6, R7-R10, U1, and U4 derive and persist candidate tree identity while keeping historical fallback explicit (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:164-175,252-278,357-366`). |
| PA-007, the reader inventory omitted decision-bearing consumers | **Closed.** Section 3.1 inventories those readers, and U5a/U5b allocate their migration (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:334-349,367-370`). |
| PA-008, `Resumed` had no schema owner or compatibility rule | **Closed.** D8 and R14/W15 assign the field to `HistoryLine`, keep omission backward compatible, and allocate its schema/parser work (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:198-209,318-321,373-374`). |
| PA-009, named decisions lacked focused witnesses | **Closed.** W3, W4, W10c, and W16b-c now cover the distinct-artifact rule, independent-accounting proof, legacy reuse exclusion, and filesystem/history observability (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:230-250,272-278,307-316`). |

The prior non-material PA-010 provenance observation remains non-material: the page names `dacadefe0`, while current `origin/main` is `faf9d057c`; a read-only diff over the design's cited command, internal, script, and documentation paths is empty.

## Code-fact conformance audit

The design's implementation premises were checked against current `origin/main`, not accepted from its older provenance line.

| Design dependency | Current-tree result |
|---|---|
| Goal resolution, planning, and launch | Confirmed: top-level proof resolves `--goal` or the unique live claim, then resolves a claimed binding (`metasystem/cmd/metasystem/proof_run.go:404-435`); planning resolves the supplied goal, uses claimed risk, and passes the same goal to the trusted engine (`metasystem/cmd/metasystem/test.go:213-218,310-315,431-435,1338-1382`). Admission then projects and decides on that binding (`metasystem/cmd/metasystem/proof_run.go:505-537`). |
| Existing attempt attribution and reuse | Confirmed: candidate tree currently survives only in testing identity/result fields (`metasystem/cmd/metasystem/test.go:811-815`; `metasystem/internal/proofrun/test_result.go:171-182`), while component selection filters the old goal/accounting tuple and can report ambiguity across failures (`metasystem/internal/proofrun/attempt.go:636-720`). Exact-result reuse also uses the old goal/accounting tuple (`metasystem/internal/proofrun/test_result.go:352-374`). |
| Claim/re-claim accounting | Partly contradicted: fresh binding writes a claim accounting revision (`metasystem/internal/goal/verbs.go:323-346`), but same-pair release/re-claim records and restores the prior episode (`metasystem/internal/goal/verbs.go:349-429,926-1005`). The blanket section 1.1 statement that "a re-claim starts a fresh count" is false; the resulting episode-owner defect is PA-002-R2. |
| Fence and epoch divergence | Confirmed: set-budget refuses a fence (`metasystem/internal/goal/verbs.go:1206-1208`); resume checks tuple continuity and preserves the recorded epoch (`metasystem/internal/dispatch/stop.go:433-481`); the command edge obtains/falls back the claim epoch (`metasystem/cmd/metasystem/goalsync_mutations.go:525-533`; `metasystem/internal/lease/verbs.go:371-376`); and the set-budget tail can replace the recorded value (`metasystem/internal/goal/verbs.go:1214-1249`). Proof admission enforces holder epoch equality (`metasystem/cmd/metasystem/proof_run.go:495-503`). |
| Attorney set-budget | Confirmed: the attorney path constructs the request without a human actor (`metasystem/cmd/metasystem/goalsync_mutations.go:1745-1788`), while claim rebinding requires a positive authenticated epoch (`metasystem/internal/goal/verbs.go:323-330`). |
| Attempt schema and persistence | Confirmed: schemas 1 and 2 are the only accepted versions; decoding is strict; validation knows only the old identity; and reservation writes schema 2 (`metasystem/internal/proofrun/attempt.go:33-34,285-309,395-405,530-597`). This makes D1's unstated schema choice material; see PA-012. |
| Budget projection | Contradicted where the design assumes one store: projection reads checkout-local proof attempts plus job and governed-run records (`metasystem/internal/dispatch/budget.go:332-446,513-704`). The incomplete candidate predicate is PA-003-R2. |
| Approval revision and automatic raises | Contradicted: consumption extension does not rewrite approval (`metasystem/internal/goal/verbs.go:1032-1115`); the cited rewrite is a risk raise, whose new approval-event revision is required by validation (`metasystem/internal/goal/verbs.go:2653-2675`; `metasystem/internal/goal/file.go:745-815`). See PA-002-R2. |
| Locking assumptions | The lock API can acquire a goal/revision path (`metasystem/internal/goalrevision/lock.go:123-145`), but current candidate mutations do not share D5's protocol: set-budget lacks the lock while resume takes it (`metasystem/cmd/metasystem/goalsync_mutations.go:1692-1742,1960-1975`). See PA-004-R2. |
| Recovery authority | Contradicted: recovery explicitly treats journal data as evidence rather than authority and refuses set-budget replay (`metasystem/internal/goal/recover.go:34-44,138-151,340-377`). See PA-013. |
| Refusal/history extension points | Confirmed: refusals have a centralized row table (`metasystem/internal/refusal/register.go:35-98`) with a registry witness (`metasystem/internal/refusal/register_test.go:20-48`); goal history is typed, parsed, and rendered (`metasystem/internal/goal/file.go:386-410`; `metasystem/internal/goal/root.go:278-292,598-603`). The missing arc-mate row is PA-011. |

## Binding-ruling conformance

- R-114-m1e: conformant. No unit depends on tmux, and no rule introduces a stop-at-budget behavior.
- R-115-m1e rule 6: not yet provable. D4's incomplete consumption-store migration and D1's silent fallback can lower an accounting/proof floor; PA-003-R2 and PA-012 are the blocking defects.
- R-116-m1e and R-118-m1e: conformant in scope. The page changes neither successor deny mode nor hook settings; it proposes no activation beyond observe mode.
- R-117-m1e items 2 and 5, as preserved by R-119-m1e: the unit table explicitly assigns every unit the gated landing lane (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:351-377`). This round's material defects are not all local wording folds, so item 2 requires rework rather than design closure.
- Wido 2026-09-12: structurally conformant where specified. Time enters projection through an injected `now`; W7 uses a deterministic barrier and explicitly forbids sleeps; witnesses are added rather than retried or weakened (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:126-162,211-213,252-257`). PA-003-R2 and PA-012 must be corrected to retain the existing floors.
- Wido 2026-09-14: R1-R16 each name a failing witness (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:211-332`), but the omitted arc-mate rule necessarily has no witness; PA-011 is the nonconformance.
- Unit size: the twelve allocations are each at or below 300 changed lines, with a witness-boundary split rule (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:351-377`). The corrections to PA-003-R2, PA-004-R2, PA-011, PA-012, and PA-013 require the estimates and splits to be recalculated before build.

## New and surviving material findings

### PA-011 — Critical — The seat-ruled arc-mate authority refusal is absent

The seat ruled that `--authority` may not name an arc-mate claim in this revision and required the refusal to be named in the register. D2 instead accepts any live, unfenced claim on the same machine, while R15/W17 only reject unclaimed, foreign-machine, stale, or governed/native uses (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:96-110,323-326`). Section 7 repeats the seat's desired answer, but adds neither a rule nor an implementation or witness allocation (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:427-431`). W18's six-code register list also omits an arc-mate refusal (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:328-332`).

This is implementably different, not editorial. Arc membership is persisted on the goal (`metasystem/internal/goal/file.go:35-39`), and another mutation already treats arc mates specially (`metasystem/internal/dispatch/stop.go:453-467`). An implementer following D2 would accept exactly the authority relationship the seat refused.

Required correction: make an arc-mate relationship between candidate and explicit authority a pre-admission refusal; name a stable refusal such as `PROOF_AUTHORITY_ARC_MATE_REFUSED`; add it to R15/W17 and the refusal register W18; and allocate the check and registry work to the relevant sub-300-line units.

Evidence: read.

Material: yes

Rigor: severe — {"local":true,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":true,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false} — Reopen when the design specifies the arc relation, stable refusal, register row, implementation owner, and focused witness.

### PA-002-R2 — Critical — `Approved.Revision` cannot be the budget-episode key

D3 says automatic, consumption-earned budget growth rewrites `Approved` at `verbs.go:2670` and instructs that path to preserve the prior approval revision (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:112-120`). That code fact is false. The consumption-earned `ExtendBudget` path is `metasystem/internal/goal/verbs.go:1032-1115` and does not rewrite `Approved`. Line 2670 is a risk-derived tier raise; that path must bind approval to its new event. Approval validation requires the recorded approval revision to resolve to the approving event and its raise digest (`metasystem/internal/goal/file.go:745-815`). The current risk witness expressly expects `Approved.Revision` to advance while `Claimed.AccountingRevision` stays fixed (`metasystem/internal/goal/risk_test.go:112-126`). Preserving the prior revision there would make the goal invalid; not preserving it would silently start a new accounting episode under D3.

The design therefore cannot simultaneously implement its episode rule, valid approval history, and risk raising. Its claim that release/re-claim does not reset attempts is directionally conformant with the seat ruling, and current same-pair reclaim already restores its recorded accounting revision (`metasystem/internal/goal/verbs.go:349-429,926-1005`; `metasystem/internal/goal/verbs_test.go:601-634`). The chosen owner for the stronger, set-budget-scoped episode is nevertheless wrong.

Required correction: designate an independent durable consumption-episode coordinate owned by human approval/set-budget; an existing accounting/episode field may be reused only if its lifecycle is changed accordingly. Approve or set-budget begins it; risk-derived raises, consumption-earned extensions, release, and re-claim preserve it. Specify legacy initialization and add focused witnesses for risk raise, earned extension, release/re-claim, cross-owner re-claim, and set-budget reset.

Evidence: read.

Material: yes

Rigor: severe — {"local":true,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false} — Reopen when an episode key independent of approval-event identity has an owner, migration rule, transition table, and witnesses.

### PA-003-R2 — Critical — The candidate lens omits budget stores and an earned-extension transition

D4 says both lenses operate over "one attempt store" with the same candidate predicate and must never disagree (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:126-146`). The current projector does not have one store. It totals job records (`metasystem/internal/dispatch/budget.go:332-446`), proof-attempt records (`metasystem/internal/dispatch/budget.go:513-583`), and governed obligation/run state (`metasystem/internal/dispatch/budget.go:585-704`). Specifying only proof-attempt accessor substitution leaves job and governed consumption either charged under authority fields or omitted. Either result can lower a candidate floor or make the claimed and unclaimed forms of the same candidate disagree.

The lifecycle is also incomplete. Admission can offer consumption-earned extension (`metasystem/internal/dispatch/admission.go:282-296`), but the existing extension mutation requires the extended goal to be claimed and owned by the current pair (`metasystem/internal/goal/verbs.go:1072-1076`). D2 explicitly permits unclaimed X. The design does not say who may mutate X's budget, or what stable refusal/outcome applies if nobody may. Thus the charged goal can pass through projection but cannot complete the admission path the projection enables.

Required correction: specify candidate identity and episode filtering for every contributing store, including job records and governed records, and prove identical output for claimed and unclaimed X. Then specify the authority and outcome for consumption-earned extension of X. If unclaimed extension is refused, the refusal must be stable and the no-floor-lowering consequence witnessed; if it is permitted, the mutation needs an explicit authority contract. Re-slice U2/U3c if the full store migration exceeds 300 changed lines.

Evidence: read.

Material: yes

Rigor: severe — {"local":true,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":true,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false} — Reopen when all consumption stores and the earned-extension transition have explicit identity, authority, and witnesses.

### PA-004-R2 — Critical — Candidate locks do not linearize candidate mutations

D5 locks authority C, then candidate X, rereads both, and tests a seam only immediately before the reread (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:148-162,255-257`). A lock is a serialization boundary only when competing mutations participate. The current `set-budget` command does not acquire a goal-revision lock (`metasystem/cmd/metasystem/goalsync_mutations.go:1692-1742`), while `resume` explicitly does (`metasystem/cmd/metasystem/goalsync_mutations.go:1960-1975`). Release, park, approval, and risk paths likewise are not assigned participation by D5. A candidate mutation may therefore land after the reread and before reservation publication, producing an attempt admitted against obsolete candidate state. The proposed seam cannot drive that interval.

The fixed C-then-X order is also not canonical across operations: concurrent C=A/X=B and C=B/X=A requests acquire opposite paths. The lock implementation is per goal/revision and returns busy after a bounded wait (`metasystem/internal/goalrevision/lock.go:123-145`); the page neither rules out nor specifies the observable outcome for reciprocal cross-charges.

Required correction: choose one complete protocol. Either make every transition capable of invalidating admission participate in the same locks, or publish with a versioned compare that fails when either snapshot changed. Acquire multiple identities in a canonical sorted order. Add deterministic barriers after reread/before publication and a reciprocal cross-charge witness; no sleeps or retries.

Evidence: read.

Material: yes

Rigor: severe — {"local":true,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":true,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":true} — Reopen when all invalidating mutations share an atomicity protocol and both race intervals have deterministic witnesses.

### PA-012 — Critical — Optional schema-2 candidate identity can silently revert to authority identity

D1 adds four `omitempty` fields and makes each accessor independently fall back to an authority-era field (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:85-94`). U1 does not bump the attempt schema or require an atomic candidate identity tuple (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:357-359`). Today only schemas 1 and 2 exist (`metasystem/internal/proofrun/attempt.go:33-34`), the reservation writer emits schema 2 (`metasystem/internal/proofrun/attempt.go:530-597`), and validation accepts either schema with only the old base fields (`metasystem/internal/proofrun/attempt.go:395-405`). The strict decoder also makes schema evolution an intentional contract, not a cosmetic choice (`metasystem/internal/proofrun/attempt.go:285-309`).

Under the proposed shape, any missed writer or partially constructed record validates as an ordinary schema-2 attempt and silently charges the authority goal. Independent fallback also permits a mixed tuple—for example candidate goal with authority accounting revision—which is not a real identity. That recreates the bug while the new proof appears valid.

Required correction: introduce a new attempt schema for candidate-aware records; require the candidate goal, episode, revision, and tree fields as one valid tuple for that schema; reserve fallback for genuinely historical schemas; and add corruption/omission witnesses proving that deletion or partial stamping is refused rather than reattributed.

Evidence: read.

Material: yes

Rigor: severe — {"local":true,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false} — Reopen when the schema discriminates legacy records from candidate-aware records and validates the new identity atomically.

### PA-013 — High — Journal attribution is not recoverable holder authority

D7 says `epochAuthority` is placed in `claimIntentArgs` so recovery can replay the same set-budget rebind (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:194-196`), and A4 treats that replay as required behavior (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:420-421`). The current recovery contract says the journal is not an authority credential (`metasystem/internal/goal/recover.go:34-44`); recovery reads recorded human text but cannot attest the current holder (`metasystem/internal/goal/recover.go:138-151`). It therefore explicitly refuses proof-bearing set-budget replay (`metasystem/internal/goal/recover.go:340-377`). Existing intent `by` fields are attribution, not authorization (`metasystem/internal/goal/verbs.go:617-633`). U6 allocates only lease/goal command and mutation files, not recovery (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:371-372`).

An implementer must either make the new journal field inert, contradicting D7/A4, or treat self-reported journal text as authority, contradicting R-115-m1e and the recovery trust boundary. The live-path holder attestation does not cure this replay path.

Required correction: retain the current recovery refusal unless the design introduces a fresh, independently validated capability suitable for replay. Remove the claim that an intent argument proves epoch authority, allocate the recovery behavior explicitly, and add a witness that interrupted set-budget cannot be replayed from attribution alone.

Evidence: read.

Material: yes

Rigor: severe — {"local":true,"recoverable":true,"proofBoundaryCrossed":false,"authorityBoundaryCrossed":true,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false} — Reopen when recovery either remains an explicit refusal with a witness or obtains an independently attestable replay capability.

## Non-material observation

### PA-010-R2 — Low — Source provenance is stale but the cited implementation is unchanged

The design says its implementation facts were checked at `dacadefe0`, not the current `origin/main` commit `faf9d057c` (`metasystem/artifacts/reports/proof-admission-fits-a-seat-proving-several-units-design-r2.md:14-16`). A read-only diff across the cited command, internal, script, and documentation paths is empty, so this does not presently change what would be built. Refresh the provenance line in the next design revision.

Evidence: read.

Material: no

## Open questions: recommended seat answers

- OQ1: Yes—rewrite the fresh-claim consumption expectation in `metasystem/internal/dispatch/admission_test.go:289-331` and preserve the existing same-pair continuity witnesses in `metasystem/internal/goal/verbs_test.go:601-634` and `metasystem/internal/dispatch/budget_test.go:957-979`; do not erase ownership or elapsed-history assertions.
- OQ2: Confirm D3's policy with a corrected owner: only human approve/set-budget starts a new consumption episode; release/re-claim, risk raise, and earned extension preserve it.
- Seat authority question: No—arc-mate authority is refused in this revision; add the explicit rule, stable refusal code, register entry, implementation allocation, and witness required by PA-011.

## Proposed receipt

Round 2 found six material defects: the mandatory arc-mate refusal is absent; approval-event revision cannot own consumption episodes; candidate accounting omits stores and earned-extension authority; the proposed locks do not linearize candidate mutations; optional schema-2 fields permit silent reattribution; and recovery mistakes journal attribution for holder authority. The revision closes PA-001, PA-005's live path, PA-006, PA-007, PA-008, and PA-009, but PA-002 through PA-004 remain open in revised form.

VERDICT: rework
