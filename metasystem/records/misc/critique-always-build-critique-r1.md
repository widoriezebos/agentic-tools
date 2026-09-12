# critique-always — independent code critique (Opus 5, read + focused runs)

Worktree: `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/codex-critique-always/metasystem` (branch `codex/critique-always`).
Diff reviewed: `git diff -- . ':!memory/receipts.log'` — 13 files, 317+/47-.
Ran: `go build ./...` (0), `go vet ./internal/dispatch ./cmd/metasystem` (0), `go test -race -count=1 ./internal/dispatch` (ok, 57.5s), `go test -race -count=1 -run 'Dispatch|Compose|Hazard|Claim' ./cmd/metasystem` (ok, 20.3s), `gofmt -l` (clean), plus a purpose-built binary probe of `job compose-role-packet`. No file in the worktree was modified.

## Verdict

2 material findings.

## Brief conformance (layer 1)

| Decision | Status | Evidence |
| --- | --- | --- |
| D1 EffectiveObligations | **met** | `internal/dispatch/hazard.go:54-73`. Class floor resolved first, only the three critique fields raised, and only when `!configuration.IndependentCritiqueRequired` — so DESIGN-BEARING and DESTRUCTIVE-REACH rows are untouched at every tier, `LiveProofRequired` and both builder fields are never written. Tier 0: `independentCritiqueRequiredByTier[0]` misses, `tier != 0` is false, no error, class floor returned. Tier 4-255: exact message `goal tier must be 1, 2, or 3`. |
| D2 Both surfaces | **met** | `scripts/agents/role-packets.json:29` adds `"independentCritiqueByTier": {"1": false, "2": true, "3": true}`; parses; the three `destructiveReach` rows are byte-identical (the diff adds exactly one line). `internal/dispatch/composition.go:31` adds the field with the json tag; `composition.go:346-348` refuses a table with an empty/absent map; `hazard.go:96-101` refuses any map that is not `reflect.DeepEqual` to the Go rule (catches extra keys, missing keys, wrong values, and non-`"1"/"2"/"3"` spellings) with the exact required message. |
| D3 Admission records effective obligations | **met literally, with one scope widening — see F-1** | `hazard.go:78` takes the tier; sole caller `composition.go:190`. `composition.go:115` `GoalTier uint8`. `cmd/metasystem/dispatch_verbs.go:73` `--goal-tier` uint default 0. All three shell compose calls pass it (`scripts/agents/dispatch.sh:1462, 1758, 2641`); goal-less chains pass 0 (`dispatch.sh:1383`, `:2273` initialise `goal_tier=0`, and `--goal none-explicit` is stripped to no `--goal` at `cmd/metasystem/delegate.go:309`); the follow-up reads the frozen tier from the latest record (`dispatch.sh:2454`). `claim.go:248`, `build.go:479` (fresh), `build.go:837` (follow-up) and `build.go:985` (`readCompositionForJob`/`configurationObligationsMatchObject`) all resolve through `EffectiveObligations`. `MinimumHazardConfiguration` kept as the class floor for `ValidateRuntimeHazardConfiguration` (`hazard.go:120`) and `admission.go:207`. |
| D4 Close enforces the tier | **met — see F-2 for the operational consequence** | `hazard.go:196-212`: root record located by `jobId == root`; absent or `null` `goalTier` means 0; a non-integer or out-of-range value refuses with `REFUSED-R22-M1-RULING-O-HAZARD-RECORD`; a numerically valid but unlawful tier (4-255) refuses at `hazard.go:229-232`, never silently lowers the duty. Critic-only exemption (`hazard.go:249`), live-proof duty and `validateIndependentCritiqueReference` unchanged. `docs/orchestration.md:150-151` adds the sentence. |
| D5 Tests | **met** | All four new closure subtests plus the mirror/invalid-tier/claim/follow-up tests exist and pass under `-race`. No sleep, clock, or wall-clock wait added (`t.TempDir` + fixed `time.Date`). `provenance_test.go:24-36` derives its expectation through `EffectiveObligations(HazardMechanical, 2)` instead of the bare class row — adjusted, not weakened; the record it builds carries tier 2, so the old literal was simply wrong under the new rule. |
| D6 Fixture beds | **met; independently re-audited** | `goalTier` appears in `scripts/agents/land-fixtures.sh` only at `:1138` (tier 1 / DESTRUCTIVE-REACH) and `:1815` (tier 2 / DESIGN-BEARING) — `EffectiveObligations` returns those class rows unchanged. `scripts/agents/dispatch-fixtures.sh` contains no `goalTier` and no `--goal-tier`; its only goal-bound dispatches are `structured-budget` (MECHANICAL, `:1427/1451/1490/1511`) and that chain is never closed (no `close --job structured-budget-*`). The bed's closed chains (`happy`, `conformance`, `close-race`, `flag-runtime`, `cap-warden`) are goal-less and/or exact critic roles. No fixture edit was needed. |
| D7 Scope | **met** | `AGENTS.md` and `wow.md` untouched (and neither mentions the class-keyed critique rule, so no doc contradiction is left behind); `requiredConfigurationByHazard` effort values unchanged; no clock or sleep added. |

Layer-1 exception: the diff contains one change outside what D3 requires — see F-1.

---

## Findings

### F-1 — material, high. `scripts/agents/dispatch.sh`: the goal-binding block was moved ~145 lines earlier to feed a tier the destination never reads

`scripts/agents/dispatch.sh:1449-1456` (new position) / removed from `:1596`.

The move was made so that `--goal-tier "$goal_tier"` at `dispatch.sh:1462` carries a real tier. **That flag is inert on that call.** `cmd/metasystem/dispatch_verbs.go:91-94` bounds-checks `*goalTier`, then `:95-110` takes the `--validate-only` branch, calls `dispatchcore.ValidateRolePacketSources(*root, *role, sources)` (`internal/dispatch/composition.go:296` — no tier parameter) and **returns before** `ComposeRolePacket`, the only consumer of `GoalTier` (`dispatch_verbs.go:130-134`). So D3 is satisfied at `:1462` by passing `0`; the reorder buys nothing and is a scope widening.

What it changes:

1. **Refusal precedence and exit codes.** `job goal-binding` now runs before the source-admission preflight (`:1459-1469`), the checkout execution guard (`:1472`), the `--reviews` / `--outputs` usage checks (`:1481-1508`), `brief_mode` (`:1515`), `lease_entry_check` (`:1528`) and `resolve-roster` (`:1535`). Concretely: `dispatch.sh dispatch --role implementer --goal G --brief b.md --destructive-reach MECHANICAL --source docs/project-rules.md`, where `G` is not a claimed goal with stop capability, previously emitted the `REFUSED-CONTEXT-SOURCE` JSON, recorded it via `record_delegate_outcome_raw`, and returned 9; it now `die 1 "cannot bind delegate operation to accepted goal G stop authority"` with no recorded delegate outcome. Same shape for `--role code-critic --goal G` without `--reviews`: exit 2 (usage) becomes exit 1 (binding). The in-repo beds do not hit this — every `--source` leg uses `--goal none-explicit` (`dispatch-fixtures.sh:2165`), which `delegate.go:309` strips to no `--goal` — but the refusal contract changed for live callers.
2. **A wider goal-revision TOCTOU window.** `goal_revision` is now read ~190 lines before `acquire_goal_revision_lock "$goal" "$goal_revision"` (`:1639`), across the roster subprocess, mission resolution and the lease entry; previously ~40 lines. `require_goal_revision_admission` (`:1795`) re-validates the exact revision, so a concurrent `goal edit` refuses rather than mis-dispatching — but it now refuses after the husk, lock and worktree exist instead of before. The post-lease epoch comparison at `:1599-1600` still catches a claim-epoch roll, as the builder claims; it does not catch a revision bump inside the same epoch.
3. The new callsite assertion `cmd/metasystem/dispatch_callsite_test.go:78-81` pins the ordering, freezing the widened scope as a contract.

Confirmed by reading `dispatch_verbs.go:95-110` and `composition.go:296` (the tier is provably unreachable on that path) plus the before/after positions in the diff. Minimal correction: restore the binding block to `:1596` and pass `--goal-tier 0` (or drop the flag) at `:1462`, and drop the `assertShellOrder` assertion that encodes the move.

### F-2 — material, medium. A MECHANICAL tier-2/3 chain's critic must be dispatched at DESIGN-BEARING or the chain can never close; nothing in the code, the refusal message, or the doc says so

`internal/dispatch/hazard.go:354-359`, `docs/orchestration.md:150-151`, `scripts/agents/dispatch.sh:1852`.

Confirmed by running. I built `cmd/metasystem` to `/tmp/opus-critique-ms` and composed a **code-critic** packet at the class a seat would naturally mirror from the chain under review:

```
job compose-role-packet --role code-critic --destructive-reach MECHANICAL --goal-tier 3 ...
→ configurationObligations = {
    "builderEffortTier": "ordinary", "builderReasoningEffort": "medium",
    "independentCritiqueRequired": true, "independentCritiqueEffortTier": "maximal",
    "independentCritiqueReasoningEffort": "xhigh", "liveProofRequired": false }
```

`validateIndependentCritiqueReference` then refuses that critic, because it reads the critic's own **builder** rows:

```go
// hazard.go:354-359
if !ok || asString(configuration["builderEffortTier"]) != required.IndependentCritiqueEffortTier ||   // "ordinary" != "maximal"
    asString(configuration["builderReasoningEffort"]) != required.IndependentCritiqueReasoningEffort || // "medium" != "xhigh"
    asString(critic["reasoningEffort"]) != required.IndependentCritiqueReasoningEffort {
    return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("independent-critique job %q does not prove the required maximum critic effort", ref))
```

and `dispatch.sh:1852` sets the critic job's `reasoningEffort` from `configurationObligations.builderReasoningEffort`, i.e. `"medium"`, so the third clause fails too.

Failure scenario: goal `G` at tier 3; MECHANICAL implementer chain `R` completes. Seat runs `dispatch --role code-critic --reviews R --goal G --destructive-reach MECHANICAL`; the critic completes and `StampClaimedReviewReference` attaches it. `dispatch.sh close --job R` refuses **permanently** with `REFUSED-R22-M1-RULING-O-INDEPENDENT-CRITIQUE: independent-critique job "..." does not prove the required maximum critic effort` — a message that names effort, not the class the operator must raise. The chain cannot be closed without re-dispatching the critic at `--destructive-reach DESIGN-BEARING`.

D4 anticipated exactly this ("the seat dispatches such a critic at the critique rows … DESIGN-BEARING") and asked for it in `docs/orchestration.md`. The built sentence instead asserts "the critic still runs at maximal effort with xhigh reasoning", which is **false** for the natural dispatch and gives no instruction. No test covers a tier-raised chain with a same-class critic. (The passing subtest `mechanical at tier 3 requires a critique` writes its critic with `requiredConfigurationByHazard[HazardDesignBearing]`, so it silently encodes the workaround without naming it.)

Correction surface: one clause in `docs/orchestration.md:150-151` naming the class to dispatch the critic at, and/or extending the refusal detail at `hazard.go:358` to say which field of which record fell short.

---

## Non-material observations (record, do not block)

- **N-1.** `docs/orchestration.md:150-151` sits inside the R-60-m1 *review-round budget* paragraph, where "critique" means a review round, not the hazard-class independent-critique job. In that paragraph's vocabulary the new sentence reads close to a tautology (tier 2 and 3 already carry 2 and 3 rounds) and does not convey that the rule is a close-time gate.
- **N-2.** No test asserts the D1 invariant that a tier leaves the *builder* rows and `liveProofRequired` alone: `TestClaimLaunchRecordsEffectiveTierObligations` (`claim_test.go:274-279`) and `TestComposeRolePacketCommandCarriesGoalTier` (`dispatch_verbs_test.go:45-51`) check only the three critique fields. The behaviour is correct today (verified by the binary probe above); the regression guard is missing.
- **N-3.** `hazard.go:196-203` returns `nil` when no member's `jobId` equals `root`, before the per-member class validation that previously ran first. Unreachable: `chainMembers` (`chain.go:53-88`) only admits records whose `lineageRoot` resolves to `root`, and a record with an unresolvable ancestor is skipped, so `rootRecord == nil` implies `len(members) == 0`, which `CloseCheck` (`close.go:21-23`) already refuses. Dead branch, no behaviour change.
- **N-4.** The `goalTier` **absent-key** case is handled (`hazard.go:206`, `present &&`) but untested; the tests only cover explicit `null` (`composition_test.go:441-449` writes `record["goalTier"] = tier` with `tier == nil`).
- **N-5.** `composition_test.go:280` assigns with `=` into the outer `err` from inside a `t.Run` closure. Harmless while the subtests are sequential; a latent race if anyone adds `t.Parallel()`. Same file: the `writeRecord` return at `:174` is discarded (a failed write would still be caught by the following assertion). `closeReadyHazardChainAtTier(t, class, tier any)` taking `any` and type-asserting to `uint8` is an awkward substitute for `*uint8`.
- **N-6.** The "missing independent critique tier rule" subtest (`composition_test.go:271-292`) asserts only `err != nil`, not which of the two refusals fired, so it does not distinguish D2's `readRolePacketTable` refusal from the `ResolveHazardConfiguration` mirror refusal.
- **N-7.** `ValidateRolePacketSources` reaches `readRolePacketTable` but not `ResolveHazardConfiguration`, so the `--validate-only` preflight admits a table whose tier map is present but *wrong*. The real compose refuses it; no exploitable gap.
- **N-8.** The CLI bounds check at `dispatch_verbs.go:91-94` runs before the `--validate-only` branch, so `--validate-only --goal-tier 9` exits 2 without validating sources. Trivial.
- **N-9.** A goal-bound record can never carry tier 0: `nullableGoalTier` (`build.go:123-134`) refuses `goalId` without a tier in 1-3 and refuses a tier without a `goalId`. So every goal-bound chain reaches `validateHazardCompletion` with a real tier — the new gate cannot be bypassed by omitting `--goal-tier`. Good property; noted because it is load-bearing and undocumented.
- **N-10.** Raising a goal's tier mid-chain does not govern the open chain: `dispatch.sh:2454` freezes the follow-up's tier from the latest record and `build.go:829-831` enforces inheritance, so close reads the root's original tier. Built as the brief specifies; flagged so the seat knows `goal edit --tier 3` on an open tier-1 chain adds no critique duty.
- **N-11.** A MECHANICAL chain cannot land through `land.sh --chain` at all — `internal/landing/observe.go:369-372` refuses any `destructiveReach` other than DESIGN-BEARING/DESTRUCTIVE-REACH with `chain-not-design-bearing`, and `internal/landing/tierone.go:117` gates the other lane on tier 1. So the new MECHANICAL tier-2/3 critique duty bites only at `dispatch.sh close`; the landing lane for such a chain is a separate, pre-existing question.
- **N-12.** Verification residue: `scripts/agents/go-gate.sh` accumulates static reds rather than exiting early (`go-gate.sh:485-521`), and the builder's tail reports exactly `1 static check(s) red`, so gofmt, `go vet ./...`, `go test ./internal/refusal` and the engine build all passed — only pinned staticcheck v0.8.0 never ran (proxy DNS). Reading the changed Go for the usual staticcheck families found nothing: error strings are lowercase and unpunctuated, `strconv` is used, no unused symbols, no shadowed package-level identifiers beyond the pre-existing `copy :=` at `hazard.go:235`. The register (`internal/refusal/register.go`) keys on SCREAMING_SNAKE codes and this change introduces none. Staticcheck is still the one required command outstanding.
- **N-13.** `internal/dispatch/composition.go:210` digests the whole `role-packets.json`, so `recipeDigest` values change with this diff. Nothing pins that digest (`build.go:989` checks format only), and the full suite is green.
