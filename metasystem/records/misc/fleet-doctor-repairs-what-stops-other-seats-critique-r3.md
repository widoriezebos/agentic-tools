# Design critique: fleet-doctor-repairs-what-stops-other-seats, revision 3, round 3 of 3

Reviewed commit: `7df4c50878dffe35f3f8e33826fb7909e5148efb` (`origin/main`). The worktree `HEAD` remained at `faf9d057cd7cdb8a462173fe796704dc15e83db4`; the one intervening commit changes only `metasystem/plans/goals/registered-wait-matches-the-runtime-session.md`, so every cited code and target-goal byte is identical at the reviewed commit.

Evidence level: revision 3, its brief, the goal, the round-2 critique, the binding rulings, and every code surface on which the findings below depend were checked by reading at this commit. The design's code anchor `29259bf51` is still current for those surfaces: the later commits through the reviewed commit change only goal files. No tests, fixture beds, builds, or process-control commands were run. `bin/metasystem` is absent, so the permitted `goal show` read could not run and the canonical goal file was read directly. Neither `S5/breakglass-design-r3.md` nor revision 2 is present in this worktree; the break-glass interface was therefore judged as the instructed revision-2 assumption.

The binding materiality test is: would an implementer working from this design build something different, or wrong, because of the finding? Only findings that meet that test and leave a unit unbuildable or unprovable are reported.

## Round-2 material findings

| Prior finding | Closed by revision 3? | Evidence |
|---|---|---|
| DRC-R2-001, the engine predicate proves a claim rather than ownership | **No.** Section 4 adds the kernel executable, digest, launcher role, creation-claim, and birth-grace legs, but its runner-record reader collapses unreadable ownership evidence into absence. The signal-only safety claim therefore remains unprovable; see DRC-R3-001. | `metasystem/artifacts/reports/doctor-design-r3.md:101`, `metasystem/artifacts/reports/doctor-design-r3.md:156-157`, `metasystem/artifacts/reports/doctor-design-r3.md:171` |
| DRC-R2-002, the repair publisher drops transaction invariants | **No.** Section 5 now retains acceptance, sync-mode, archive, candidate validation, journal, compare-and-swap, confirmation, and the deadline. The complete repair transaction still cannot be built or proved because its journal intent names a nonexistent field, its receipt call is rejected by the receipt owner, and its unknown-push outcome has no exit mapping; see DRC-R3-002, DRC-R3-004, and DRC-R3-005. | `metasystem/artifacts/reports/doctor-design-r3.md:103-113` |
| DRC-R2-003, diagnosis reads the remote-tracking ref | **Yes.** Section 5 step 1 uses `CaptureTip` and `CleanupRefs`, and DR-9c-prime advances the origin while leaving the remote-tracking ref stale. | `metasystem/artifacts/reports/doctor-design-r3.md:107`, `metasystem/artifacts/reports/doctor-design-r3.md:143`, `metasystem/artifacts/reports/doctor-design-r3.md:151` |
| DRC-R2-004, the authority matrix conflicts with the DONE and A3 | **Yes, under the binding seat ruling and the named pre-build goal edit.** A1 restricts agent repair to the holding MAIN, Q1 skips checkout-local human repair beside a live or uninspectable holder, and the ledger is the named global exception. | `metasystem/artifacts/reports/doctor-design-r3.md:117-125`, `metasystem/artifacts/reports/doctor-design-r3.md:214` |
| DRC-R2-005, U8 closes with budget unrepaired | **No.** Revision 3 adds a resume followed by a clean rerun, but the fault it constructs is the pre-stop `ADMISSION_CLOSED_ELAPSED` state and the named `goal resume` entry refuses that state. The allocated composite test also cannot call the unexported command seam from its package. See DRC-R3-003 and DRC-R3-007. | `metasystem/artifacts/reports/doctor-design-r3.md:74`, `metasystem/artifacts/reports/doctor-design-r3.md:161`, `metasystem/artifacts/reports/doctor-design-r3.md:173`, `metasystem/artifacts/reports/doctor-design-r3.md:205` |
| DRC-R2-006, `finding-register.lock` is idle flock state | **Yes.** The path is explicitly not a finding, and the healthy bed carries it. | `metasystem/artifacts/reports/doctor-design-r3.md:98`, `metasystem/artifacts/reports/doctor-design-r3.md:155`, `metasystem/artifacts/reports/doctor-design-r3.md:174` |
| DRC-R2-007, injected clock and prober cannot drive owner operations | **No.** The health and record-reader seams are specified, but the deadline transaction, base lock wait, and TERM-to-KILL grace still have wall-time owners outside the proposed seams. See DRC-R3-006. | `metasystem/artifacts/reports/doctor-design-r3.md:38`, `metasystem/artifacts/reports/doctor-design-r3.md:151`, `metasystem/artifacts/reports/doctor-design-r3.md:158`, `metasystem/artifacts/reports/doctor-design-r3.md:171` |
| DRC-R2-008, allocations omit the tests they promise | **No.** Revision 3 lists test files and keeps every stated allocation at or below 300 changed lines, but U8 places its required entry-point witness in a package that cannot call that entry point. The map is therefore not implementable as allocated; see DRC-R3-007. | `metasystem/artifacts/reports/doctor-design-r3.md:187-208` |
| DRC-R2-009, the rule-to-witness list is not one-to-one | **No.** Most prior gaps now have named witnesses, but malformed runner ownership, the unknown-push exit, and wall-time fallback remain without a witness that fails when the unsafe rule is removed. See DRC-R3-001, DRC-R3-004, and DRC-R3-006. | `metasystem/artifacts/reports/doctor-design-r3.md:139-167` |

## DRC-R3-001 — Unreadable runner ownership is treated as no ownership before TERM and KILL

Severity: critical

Material: yes

The P3 “unrecorded” predicate permits signalling when `steward.RunnerRecordRef` does not return an identity matching the process, and U4a defines that new reader as `(RunnerRecord, identity.Ref, bool)` with no error result (`metasystem/artifacts/reports/doctor-design-r3.md:101`, `metasystem/artifacts/reports/doctor-design-r3.md:171`, `metasystem/artifacts/reports/doctor-design-r3.md:198-200`). The current owner reader returns `false` for every `runner.json` read or decode error, not only for an absent record (`metasystem/internal/steward/runner.go:1007-1013`). The proposed signature cannot preserve the distinction. A torn, permission-denied, or schema-invalid runner record therefore becomes “no record,” P3 passes, and a genuine recorded runner can reach TERM and KILL.

The other ownership sources show why this distinction is required: `ReadArmingOwner` reports malformed or incomplete owner data as an error (`metasystem/internal/supervise/arming.go:199-213`), and `stopfence.Claims` fails when any claim cannot be reduced safely (`metasystem/internal/stopfence/fence.go:331-340`). The general UNAVAILABLE state cannot save the runner leg after U4a has already collapsed the error to `bool=false`. DR-15 proves only a readable matching runner record; no witness corrupts or makes that record unreadable (`metasystem/artifacts/reports/doctor-design-r3.md:156`).

U4a/U4b/U4c need a fail-closed owner view that distinguishes absent, valid, and unreadable evidence, and the signal re-proofs need the same distinction beside both signals. A malformed-runner-record witness must prove no signal occurs. This changes the owner-reader interface, P3, the signal helper, the witness join, and the unit allocations.

Rigor: severe

Facts: local=true; recoverable=false; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true.

Reopening trigger: a typed runner ownership read whose unreadable state makes the engine class UNAVAILABLE or SKIPPED, used again immediately before TERM and KILL, plus a malformed/unreadable `runner.json` witness that records no signal.

## DRC-R3-002 — The ledger repair request uses a journal field that does not exist

Severity: high

Material: yes

Section 5 constructs `Intent{Verb: "doctor-repair-ledger", By: name}` and relies on that stored human name to make dead-owner recovery close the entry as rejected (`metasystem/artifacts/reports/doctor-design-r3.md:110`). The shipped `goal.Intent` has exactly `Verb`, `Targets`, `Args`, and `Deltas`; there is no `By` field (`metasystem/internal/goal/journal.go:57-66`). The request therefore does not compile as written. More importantly, recovery recognizes a human act only through `Intent.Args["by"]` (`metasystem/internal/goal/recover.go:138-151`). Adding a new `By` field to make the literal compile would silently miss the recovery boundary the design claims.

U2c must encode the human in the existing normalized intent arguments, and DR-9f must assert the stored `args.by` and the human-specific rejected recovery outcome. That is an implementation and proof change, not a spelling correction.

Rigor: severe

Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true.

Reopening trigger: section 5 and U2c use the existing `Args["by"]` journal contract, and the death-before-push witness reads the entry and proves recovery rejected it because journal text cannot replay human authority.

## DRC-R3-003 — The four-fault budget state cannot be reopened by the command U8 invokes

Severity: high

Material: yes

The budget class detects `ADMISSION_CLOSED_ELAPSED`, prints `goal resume`, and U8 invokes that command before expecting a clean exit-zero rerun (`metasystem/artifacts/reports/doctor-design-r3.md:74`, `metasystem/artifacts/reports/doctor-design-r3.md:86`, `metasystem/artifacts/reports/doctor-design-r3.md:161`, `metasystem/artifacts/reports/doctor-design-r3.md:173`). In the current health owner, `ADMISSION_CLOSED_ELAPSED` is evaluated only after the `StopFence` branch has been skipped; any goal with a stop fence is reported through the breach-stop branch and `continue`s before that state (`metasystem/internal/steward/health.go:1104-1131`, `metasystem/internal/steward/health.go:1158-1167`). The `goal resume` owner, however, requires the goal to have both a stop capability and a stop fence (`metasystem/internal/goal/stop.go:421-429`), and its command entry refuses immediately when the resolved binding has no fence (`metasystem/cmd/metasystem/goalsync_mutations.go:1959-1967`).

The D4 fixture as specified is therefore pre-stop and `goal resume` cannot clear it, so DR-20b and U8 cannot reach the promised final exit 0. The design must either map pre-stop admission closure to the proof-bearing budget owner that actually reopens it, or construct and diagnose a completed breach-stop state if `goal resume` is the intended command. The detector, printed command, authority proof, fixture, and final witness must all describe the same state.

Rigor: severe

Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true.

Reopening trigger: one code-grounded state-to-verb mapping in which the printed command accepts the exact D4 fixture state, followed by a witness that runs that entry and observes the clean doctor rerun.

## DRC-R3-004 — An unknown ledger push has no state or exit code

Severity: high

Material: yes

Section 5 introduces `UNKNOWN` when the push outcome cannot be classified, keeps the journal entry pushed, and directs the operator to `goal recover`; DR-9h requires that output (`metasystem/artifacts/reports/doctor-design-r3.md:110`, `metasystem/artifacts/reports/doctor-design-r3.md:151`). But the public outcome table contains only CLEAR, FOUND, NAMED, UNAVAILABLE, SKIPPED, and STILL, and the exit rule assigns codes only to those states and refusals (`metasystem/artifacts/reports/doctor-design-r3.md:65-74`). As written, an implementer can let UNKNOWN fall through to exit 0, map it to exit 1, or treat it as STILL and exit 3. Those are observably different contracts, and exit 0 is especially wrong because a pushed-unknown journal entry blocks later mutations (`metasystem/internal/goal/txn.go:521-534`, `metasystem/internal/goal/txn.go:772-777`).

U2c/U7a need one declared summary state and exit for this outcome, and DR-9h must assert both rather than only the text and record. Until then the ledger state machine and command contract are unprovable.

Rigor: severe

Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true.

Reopening trigger: UNKNOWN is mapped to one existing or newly declared public state with a nonzero exit and recovery command, and DR-9h fails when that state or exit mapping is removed.

## DRC-R3-005 — The specified receipt call is rejected before it can build the in-commit receipt

Severity: high

Material: yes

The ledger mutation says it obtains the receipt line by running `receipt.Add(Options{Now: clock, Type: "other", Outcome: "shipped", Note: ...})` against a temporary copy, and the record contract requires exactly one receipt per repair run (`metasystem/artifacts/reports/doctor-design-r3.md:110`, `metasystem/artifacts/reports/doctor-design-r3.md:130-132`, `metasystem/artifacts/reports/doctor-design-r3.md:160`). `receipt.Add` rejects an empty `Verify`, `StopLoss`, or nonnumeric `Corrections` before writing, and also needs the target `File` in order to append (`metasystem/internal/receipt/receipt.go:152-195`, `metasystem/internal/receipt/receipt.go:217-241`). Every omitted field in the design's literal is empty, so the call returns code 2 and no receipt bytes.

U1b/U2c must either consume the assumed break-glass receipt-content builder or specify a complete valid `receipt.Options` call against the temporary file and check its result. DR-19c must exercise the real validation path and compare the resulting committed line, not assume `Add` produced one.

Rigor: severe

Facts: local=false; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true.

Reopening trigger: a complete receipt-builder contract naming every required field and file, plus a witness that fails on a nonzero `receipt.Add` result and proves the exact line is present once in the published commit.

## DRC-R3-006 — The artificial-clock contract still stops at the owner boundaries

Severity: high

Material: yes

The page promises that every time observation uses the injected clock and names time-bound witnesses for the publish deadline, lock acquisition, and TERM-to-KILL grace (`metasystem/artifacts/reports/doctor-design-r3.md:38`, `metasystem/artifacts/reports/doctor-design-r3.md:151`, `metasystem/artifacts/reports/doctor-design-r3.md:158`, `metasystem/artifacts/reports/doctor-design-r3.md:171`). Those witnesses cannot drive the decisions as allocated:

- `goal.runTransaction` computes and tests its retry deadline with `time.Now`, while DR-9g merely sets a one-nanosecond duration; no clock reaches the decision (`metasystem/internal/goal/txn.go:609-615`, `metasystem/internal/goal/txn.go:757-769`).
- `goalrevision.Acquire` delegates its bounded wait to `internal/lock`, whose deadline, ownerless age, timeout checks, and poll sleep are all wall-time operations (`metasystem/internal/lock/lock.go:157-158`, `metasystem/internal/lock/lock.go:187-203`, `metasystem/internal/lock/lock.go:215-221`). U3a allocates only `internal/goalrevision`, so its advertised `Now`, wait, and poll seam cannot control the owner of that wait (`metasystem/artifacts/reports/doctor-design-r3.md:195`).
- The doctor seam has `Now func() time.Time` but no timer or wait control, while U4c must withhold KILL for a ten-second grace. A fake `Now` that jumps forward proves only two reads, not that the implementation waited without wall time (`metasystem/artifacts/reports/doctor-design-r3.md:101`, `metasystem/artifacts/reports/doctor-design-r3.md:171`, `metasystem/artifacts/reports/doctor-design-r3.md:200`).

This violates the binding artificial-clock ruling, which requires every time-bound test to drive an injected clock and treats a load-shaped failure as a test defect, never a reason to retry or widen a floor (`metasystem/memory/rulings.md:163`). The design must carry a clock/timer contract into the actual deadline and wait owners, allocate the added owner files and tests, and name witnesses that fail if a wall-time fallback returns.

Rigor: severe

Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false.

Reopening trigger: injected clock/timer seams decide the transaction deadline, base-lock wait, and signal grace in both production wiring and focused tests, with unit files and allocations updated for every changed owner.

## DRC-R3-007 — U8 cannot call the command entry from its allocated test package

Severity: medium

Material: yes

DR-20b says the closure composite calls `runGoalResumeWithAuthority` with a test prover, while U8 allocates only `internal/doctor/composite_test.go` (`metasystem/artifacts/reports/doctor-design-r3.md:161`, `metasystem/artifacts/reports/doctor-design-r3.md:205`). That function is unexported in the separate `cmd/metasystem` package `main`, and its own comment says it is a direct seam for that package's tests (`metasystem/cmd/metasystem/goalsync_mutations.go:1`, `metasystem/cmd/metasystem/goalsync_mutations.go:1899-1906`). An `internal/doctor` test cannot import package `main` and cannot name an unexported function across a package boundary.

The final entry-point witness must move to an allocated `cmd/metasystem` test or use a different exported owner-level seam while retaining a command-level proof that the printed invocation works. Either choice changes U8's files, dependency shape, and changed-line allocation.

Rigor: severe

Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false.

Reopening trigger: U8 allocates the closure witness in a package that can lawfully invoke the real resume entry, and that witness executes the exact printed arguments before requiring the clean doctor rerun.

## Recommended answers to revision 3's open items

1. Goal contract: apply the exact section-13 DONE edit before implementation so the holding MAIN exception is canonical rather than present only in the design.
2. Break-glass dependency: accept the stated revision-2 interface as the provisional contract now; at U2 build, bind to revision 3 when present and adapt U2 rather than duplicating its planner or content builders.
3. Janitor dependency: require the bed-owner record to carry the canonical checkout root; keep fixtures UNAVAILABLE until that field and the reap primitive land.

Material findings: 7.

VERDICT: rework
