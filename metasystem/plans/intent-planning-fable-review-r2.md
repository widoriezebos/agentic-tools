# Fable critique r2 (final): Complete design work through intent

- Kind: critique report, round 2 of 2 (failsafe round, same Fable session)
- Reviewed record: 01M3EFDSFTKWEMSDCP1BB7TDGQ (draft), `metasystem/plans/designs/intent-planning-continuity.md`
- Reviewed design SHA-256: `89d5f0261cdf70f2bae5faf25c869b66d3fe1ccd444f619e2f669045eaa3d47d`
- `project design-of --goal verbs-match-intent` (live m1e executable): draft, accepted parent, superseded predecessor; unchanged registration.
- Worktree HEAD 7be7d119d; product files are changing under Opus and were read only as owners, never as completed work.
- Model: claude-fable-5-1. Tool calls used: 9 of 22. Read-only; no product file, goal, agent, build, commit or conf.local touched.
- MATERIAL count: 2 (IP-C4, IP-C5). Verdict: FOLD TWO SHAPE CORRECTIONS, then implement. Both are contract sentences plus fixture cases, not new mechanisms.

Criterion for every material item: would step 1 be built DIFFERENT or WRONG; does step 1 WORK and is it SAFE without it. Every citation below was read at this worktree in this round.

## Round-1 folds verified

- **Admission without build claims** (design lines 63-73). The stated admission is the `reviewBriefFacts` facts (approved goal, positive review-round budget; `cmd/metasystem/intent_delivery.go`) followed by launch design admission, no claim, no goal worktree, no execution clock. That matches the owner and the r1 correction. One precision: `reviewBriefFacts` also refuses without `--tool-calls`; the design should say `design` reuses only the goal facts, not that reader input.
- **Canonical invoking checkout** (lines 66-67, 99-100, 122-123). Working and publication checkout are the invoking checkout; the lock key is the canonical absolute destination path; staged output under `artifacts/agents/intent-design/<record>-<attempt>/draft.md` follows the `intent-review` precedent. Consistent with `reviewDesign` resolving FILE against `inv.cwd` and `project.HomeFor`.
- **Crash recovery** (lines 101-113). Verified against the owner: `Store.Update` runs under `withLock` (`internal/launch/record.go:181-195, 292`); `Manager.Supervise` claims `record.Supervisor` inside `Store.Update` and refuses `Terminal()` records before any adapter or child (`launch.go:240-256`); `OutputOwnerUnproven` is only set after that claim (`launch.go:284, 300`). The design's atomic recheck of `State=Starting`, absent supervisor/child/process-group, no unproven-owner flag, under `Store.Update`, therefore serialises correctly with the claim: one side wins, the other refuses. It does not treat absent PIDs as death. My r1 age-check-then-`m.fail` wording was indeed insufficient and the fold is right. The fixture `TestDesignRequestCrashBeforeSupervisor` must drive both orders: recovery first then a late `Supervise` refusing, and `Supervise` first then recovery refusing.

## Material findings

### IP-C4: replay after a completed follow-up cannot rejoin through the frozen operation ID as written (MATERIAL)

Design lines 163-167: `review design FILE --dispositions FILE` "calls existing typed follow-up with a frozen operation ID. Retain that request-to-child binding through the dispatch owner BEFORE launch; replaying after completion rejoins the same child rather than recomputing from latest parent."

Owner facts. `scripts/agents/dispatch.sh` `follow_up` admits a chain whose newest record is `completed` by minting `child=$root_id-r$((round+1))` under its own chain lock and dies only on a collision (lines 2440-2446). It then runs `job claim-launch` preflight with `--opid "$child"` and `--operation-id "$operation_id"` and sets `replay_operation=1` only on `PREFLIGHT-MATCHED` (2810-2825). `ClaimLaunchPreflight` (`internal/dispatch/claim.go:71-130`) returns MATCHED only when a record for that same standing opid exists with the same operation id and fingerprint. When the standing opid does not exist it scans for the operation id and, finding it bound to another job, returns `ClaimRefusedOpIDMismatch` with `resolution=operation-id-bound-to-another-job` and `recordedJobId`.

Consequence. After follow-up round rN has completed, the replay computes child rN+1, preflight finds the frozen operation id bound to rN, and the existing owner refuses. It never rejoins. The only MATCHED path is the standing running follow-up (`repeated_follow_up=1`, 2447-2466). So the sentence "calls existing typed follow-up with a frozen operation ID ... replaying after completion rejoins" is not what the owner does. The companion sentence "retain request-to-child binding BEFORE launch" cannot be satisfied either: the child id is minted inside `follow_up` under the chain lock; a caller that pre-computes it races an intervening round and binds the wrong job, and a caller that holds the chain lock deadlocks `acquire_launch_chain_lock`.

Different: yes. An implementer following the text builds a replay that returns an internal OPID-MISMATCH refusal, or a pre-minted binding that can name someone else's round. Works/safe without it: the first decision works; a lost response after completion, the exact case step 1 promises, is WRONG. It is safe from duplicate spend, because the mismatch refusal blocks a relaunch.

Smallest correction. Bind (goal, record path, prior round, disposition digest, new subject digest) to the frozen operation id before launch; retain the child job id after `follow_up` returns. On replay, the dispatch owner consults the retained child first; if absent, it resolves the operation id to its job through the existing owner scan (`findOperationRecord`, `claim.go:132-155`, exposed as a Go read, not a CLI scan) and reports that job whatever its status. `operation-id-bound-to-another-job` with a `recordedJobId` is the rejoin key on this path, never a refusal. Do not pre-mint child ids in the caller. Fixture: `TestDesignCritiqueReplayAndCap` must replay after completion with the binding intact, with the child missing from the binding, and after an intervening foreign round, asserting the same child and no new round each time.

### IP-C5: a capped or failed design examination has no public route and can mint a second root (MATERIAL)

Design line 173: "Failed examination recovery inherits the parent's bounded retry rules." The public grammar (lines 57-61) is `review design FILE [--dispositions FILE] [--after N]`; no `--retry N`.

Owner facts. `follow_up` refuses a critic round cut off at its cap: "start a fresh design-critic round (a critique re-runs under its critique cap)" (`dispatch.sh:2474-2480`). The parent's `review G --retry N` exists for work reviews (`intent_delivery.go:51,66`; `intent_selection.go:296-303`), and the parent design mandates extending follow-up for failed examinations. Read admission for a design subject (`internal/dispatch/read_admission.go:69-150`) refuses only REDUNDANT_READ on an equal clean read; the CONCURRENT_READ guard that blocks a fresh root while a chain has an outstanding round (159-205) is `SubjectLive` only. A fresh `design-critic` dispatch on the same design path after a capped or failed round is therefore admitted as a new root under the aggregate goal allowance.

Consequence. As written, after a capped critic round the implementer either dead-ends (`follow_up` refuses, no public retry spelling) or lets `review design FILE` dispatch fresh and buy a second root for the same path with the finding register left behind, which the design's own line 172 forbids.

Different: yes, a retry spelling and a path-scoped fresh-root refusal are missing. Works/safe without it: the happy path works; the capped path is WRONG (second root or dead end). Aggregate goal admission still caps total spend, so it is budget-safe but continuity-unsafe.

Smallest correction. Add `review design FILE --retry N` to the grammar, bound to the parent's failed-examination follow-up extension and covering `timeout/budget-cap` critic rounds. Extend the read owner's design-path selection so a fresh root is refused while a chain for that canonical path is open, capped, failed or exhausted, mirroring the live-subject guard. Fixture: `TestDesignCritiqueReplayAndCap` adds a capped round: `review design FILE` names `--retry N`, and a fresh root for the path is refused.

## Notes (non-material)

- N8 A supervisor that claimed and then died before spawning leaves a `Starting` record the new recovery deliberately does not touch. The public remedy is `stop design G`, which reaches `Manager.Cancel` and its `provenDead` rule (`launch.go:532-579`). Say so in the status output for that attempt; fixture `TestIntentDesignLifecycle` should cover it.
- N9 `wait proof REF` now has a public source (lines 190-195). Adequate; the fixture rule is stated.

## Unexamined scope

- `cmd/metasystem/launch_verbs.go:115` error path after a refused `Supervise`: assumed to exit without spawning; not read.
- `refDead` on a nil child reference inside `provenDead`: assumed true; not read.
- Whether `follow_up` refreshes a design subject digest into the child record (`subject_temp` locals) was again not traced; IP-2's fixture must prove it.
- Nothing in the uncommitted Opus files was reviewed as completed work.

## Adjudication summary

The three r1 folds are correct, and the crash recovery now rests on the owner's real atomic claim. Two shape defects remain in the review continuation: the frozen-operation-id replay cannot rejoin through the existing follow-up after completion (IP-C4), and a capped or failed design examination has no public route while the design-subject admission lets a fresh root through (IP-C5). Both fold into two contract sentences and fixture cases. No third prose round is warranted.
