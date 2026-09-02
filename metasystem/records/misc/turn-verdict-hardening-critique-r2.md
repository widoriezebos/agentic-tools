# Turn-verdict hardening design — Sol critique round 2 (tvh-crit-3b)

Reviewed: plans/turn-verdict-hardening-design.md revision 2 at commit 164f00b104c28430c16c7f11cf95a273f693532c. Runtime codex (Sol lane), design-critic, read-only. Material findings: 5 of 11.

## Findings

### TVH-R1-CLAIM-ADMISSION-OMITS-AUTHORITY-AND-REPLAY — low

Closure verdict — real. Revision 2 makes ClaimAdmission carry the authenticated claim epoch, models the machine-quota rule, and keeps operation replay before admission. An implementer following it will no longer classify an advisor's zero-epoch claim as READY or turn a successful replay into a new refusal.

Evidence: metasystem/plans/turn-verdict-hardening-design.md section 1.2.0 specifies replay before ClaimAdmission and passes seat.ClaimEpoch. This agrees with metasystem/internal/goal/verbs.go:440-477, where opidLanded precedes admission and bindClaim rejects an epoch below one, and with metasystem/internal/goal/validate.go:250-281, where machine quota is enforced.

### TVH-R1-R3-NAMES-ILLEGAL-EXIT — critical, MATERIAL

Closure verdict — partial. Revision 2 correctly excludes fenced claims, but R3 can still block repeatedly while advertising an impossible recovery. Its promised byte-verbatim Move strings omit the mandatory --id flags. More deeply, it chooses one exhausted claim H, yet one machine may lawfully hold several claims in the same non-empty arc. If H1 and H2 are held in arc A and queued goal g is in arc B, parking or releasing only H1 leaves H2, so claiming g still fails machine quota. The next Stop repeats the same unusable advice.

Evidence: metasystem/plans/turn-verdict-hardening-design.md section 1.2.1 says the move is rendered from accepted verbs as `goal park <H> --because ...`, `goal release <H>`, then `goal claim <g>`, and section 1.3 says that Move is placed byte-verbatim in the reason. metasystem/cmd/metasystem/goalsync_mutations.go:104-159 and :232-241 require --id and do not consume positional goal identifiers. metasystem/internal/goal/validate.go:250-281 explicitly permits multiple same-machine claims when every claim shares one non-empty arc. The fenced-claim exclusion itself agrees with metasystem/internal/goal/verbs.go:128-136, :653-668, and :832-873.

### TVH-R1-SLICE1-IGNORES-GOAL-BOUND-GOVERNED-RUNS — low

Closure verdict — real in the completed four-slice design. Revision 2 brings the governed-run join into slice 1 and requires goal revision, owner lineage, claim epoch, main identity, state, and live process or waiter evidence, so active goal-bound runs are no longer knowingly ignored.

Evidence: metasystem/plans/turn-verdict-hardening-design.md sections 2.2 and 10 put Relevant over jobs and runs in slice 1. metasystem/internal/run/run.go:111-134 and :160-201 contain the cited governed attempt and record fields, while metasystem/internal/report/scan.go:195-239 already reads run records and waiter evidence. The design also discloses the remaining Darwin direct-process field work in slice 2.

### TVH-R1-JOB-LIVENESS-DOWNGRADES-EXACT-IDENTITY — low

Closure verdict — real. Revision 2 requires native exact process identity for direct job liveness, retains exact waiter identity, and treats legacy seconds-only job records as not relevant. That preserves the process-identifier reuse protection instead of downgrading it.

Evidence: metasystem/plans/turn-verdict-hardening-design.md sections 2.1 and 10 require dispatch.IdentityRefOf and include explicit legacy-seconds and reused-process tests. This agrees with the exact fields written in metasystem/internal/dispatch/ownership.go:55-96, the comparison rules in metasystem/internal/identity/identity.go:192-217, and exact waiter reconstruction in metasystem/internal/run/waiter.go:192-209.

### TVH-R1-FRESH-CURSOR-IS-NOT-A-CURRENTNESS-WITNESS — low

Closure verdict — real. Revision 2 withdraws the ten-minute cursor as a currentness witness and requires each verdict to perform its own bounded fetch before projection, or prove local-sync mode. A previous process's recent fetch can no longer authorize an allow.

Evidence: metasystem/plans/turn-verdict-hardening-design.md section 4 explicitly rejects the cursor rule, orders fetch before projection, and names TestFreshnessNoTimeWindowExists. This matches the limited instant-of-fetch guarantee visible in metasystem/internal/goal/fetchadvance.go and the offline accepted-ref read in metasystem/internal/goal/project.go.

### TVH-R1-HUMANSTOP-CANNOT-RESCUE-DECISION-OWNER-FAILURES — low

Closure verdict — real. Revision 2 separates class-A failures, where the verdict can compare and consume HUMANSTOP, from class-B hook failures, where the marker cannot be consulted and repair is the only recovery. It no longer promises HUMANSTOP as a universal escape.

Evidence: metasystem/plans/turn-verdict-hardening-design.md sections 3.1 and 5 distinguish class A from class B: missing engine, unidentified world, and unusable verdict are class B, while state-lock failure is converted inside the verb after the marker step. The outcome rows no longer apply the earlier universal `unless HUMANSTOP` precedence.

### TVH-R1-FAIL-CLOSED-TABLE-OMITS-PREVERDICT-SHELL-EXITS — critical, MATERIAL

Closure verdict — partial. The EXIT trap covers the previously omitted shell exits, but the prescribed explicit failure outputs violate the trap's own one-response invariant. The design says only emit_stop_payload may set emitted=1, while P2 and failure rows F1, F2, and F5 prescribe literal direct printf JSON followed by exit 0. EXIT will then see emitted=0 and print a second block object. An implementer following the stated shape will produce two JSON responses or runtime-dependent parsing instead of one fail-closed response.

Evidence: metasystem/plans/turn-verdict-hardening-design.md section 3.0 rule 4 says `emit_stop_payload sets emitted=1 ... and nowhere else`. Its on_exit body prints whenever emitted is not one. The same section says P2 uses `explicit block JSON via command printf`, and section 3.1 gives F1, F2, and F5 literal or direct block-JSON paths. The current single-response protocol is visible in metasystem/scripts/agents/supervision-hook.sh:273-321.

### TVH-R1-STOP-DEADLINE-DOES-NOT-BOUND-EMISSION — critical, MATERIAL

Closure verdict — partial. The cap-table addition is arithmetically correct: 11,000 milliseconds of ceremonies plus a 3,500-millisecond verdict plus a 1,500-millisecond emission reserve equals 16,000 milliseconds, or 80 percent of 20 seconds. The claimed construction is nevertheless false because only the named ceremonies and final verdict are bounded. Root mapping, runtime lookup, payload handling, hook-attempt recording, ancestor discovery, lease classification, JSON field extraction, and response construction remain outside run-bounded; any one can hang before the next remaining-budget check. The clock also starts after engine testing and therefore cannot charge earlier world-mapping work. Finally, the marker phase may wait five seconds for its own lock even though the worst-case verdict allocation is only 3.5 seconds, contradicting the promise that the marker always runs under deadline failure.

Evidence: metasystem/plans/turn-verdict-hardening-design.md section 3.2(c) claims `Every step bounded` but lists run-bounded caps only for up, health, digest, watchdog, evidence cleanup, and report turn-verdict. Section 3.0 P1-P11 lists additional engine commands and shell operations without caps. Section 3.2(b) starts timing after the engine test, while the wrong-root mapping precedes usable-engine resolution. Section 5.3 assigns the marker lock the five-second withWaiterLock bound, exceeding section 3.2(c)'s computed 3,500-millisecond worst-case verdict allowance. metasystem/internal/up/up.go:412-500 also shows that up mutates its components sequentially, so `idempotent` does not mean an in-progress invocation is atomic.

### TVH-R1-RUNTIME-HOOK-CHECK-OMITS-TWO-SUPPORTED-RUNTIMES — low

Closure verdict — real. Revision 2 owns Codex, Devin, and Claude separately, changes all three Stop timeouts, and uses a budget-only check for Codex and Devin rather than requiring the Claude-only SelfCheck contract.

Evidence: metasystem/plans/turn-verdict-hardening-design.md sections 3.2, 6, and 10 name all three shipped files and the two check modes. This agrees with metasystem/internal/runtimes/runtimes.go:161-242, which declares Codex, Devin, and Claude and gives SelfCheck only to Claude, and with metasystem/cmd/metasystem/hooks.go:33-36, which rejects a missing SelfCheck.

### TVH-R2-SLICE1-HIDDEN-WRONG-ROOT-DEPENDENCY — critical, MATERIAL

Slice 1 has a hidden dependency on supervision-hook-wrong-root. At the verdict-function boundary its three named specimen fixtures would refuse correctly: specimen one satisfies R1 and specimens two and three satisfy R2. On the affected fleet seats, however, the shipped hook resolves the delegate worktree, cannot find the enrolled engine or governing artifacts, and exits before slice 1's caller-pid and F10 changes can invoke that verdict. Deferring the wrong-root dependency to slice 2 therefore makes the statement `Slice 1 alone refuses all three specimens` false at the deployed Stop boundary.

Evidence: metasystem/plans/turn-verdict-hardening-design.md section 10 makes only slice 2 depend on supervision-hook-wrong-root; slice 1 changes the hook only to pass caller-pid and make F10 block. metasystem/records/misc/seat-stop-analysis.md:39-42 says wrong-root resolution occurs on every fleet seat. metasystem/plans/goals/turn-verdict-hardening.md says the root fix must land first or be carried in slice 1. The current missing-engine branch in metasystem/scripts/agents/supervision-hook.sh:26-30 exits zero before verdict processing.

### TVH-R2-HUMANSTOP-SEAT-AUTHORITY-UNSPECIFIED — high, MATERIAL

The new HUMANSTOP relay contract does not specify a HUMANSTOP-scoped authorization check or an authenticated derivation of the seat written into the marker. ProveOrTemporaryGoalAuthority only constructs a proof; the shipped proof exposes authorization methods scoped to set-obligation and resume. The proposed command has no caller-pid or target-seat contract, yet it must write `machine`, `lineage`, and `relayedBy` for the invoking seat. An implementer must guess whether to reuse a differently scoped authorizer, trust goal-command environment identity, or classify process ancestry. That can let caller-controlled lineage mint a marker for another seat and falsify relay provenance. R-47-m0b permits relaying the human word; it does not authorize an arbitrary target seat or forged relayer identity.

Evidence: metasystem/plans/turn-verdict-hardening-design.md section 5.2 says authority is ProveOrTemporaryGoalAuthority and records `relayedBy` as the invoking seat, but its command signature supplies neither caller pid nor seat. metasystem/internal/humanauthority/authority.go:113-137 exposes AuthorizesSetObligation and AuthorizesResume only; :195-237 states that a temporary proof cannot verify who supplied the words. metasystem/cmd/metasystem/goalsync_mutations.go:40-69 builds goal.Actor from machine and lineage inputs before separately attempting lease classification. The section 10 tests check stored provenance and world/pair/session binding but include no forged-lineage or cross-seat minting case.

## Gaps declared by the critic

- The exact comparison between killing supervision arming through run-bounded and today's runtime timeout cannot be proved from repository source. The repository establishes a configured five-second timeout and shows that metasystem up is sequential and rerunnable, but the external runtime's timeout implementation is not present, so it is unknown whether today's timeout kills the hook only, its child, or the entire process group.
- The 85-builder-minute slice-1 estimate is not supported by the cited precedent. The large two-minute diff was recovered prewritten work, and the completed 120-minute-cap job was much smaller. Static evidence cannot establish the slice's actual duration, so no replacement duration was invented.
- No Go command, test, live Stop event, or executable specimen replay was run, as required by the read-only brief. Specimen outcomes and runtime delivery behavior were assessed from design text, code, and recorded incident states only.
- The launcher classifies this broad-read job as advisory and does not prove context isolation or an independently observed provider tool catalog.
