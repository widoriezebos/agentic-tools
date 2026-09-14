# Follow-up design critique, round 2

Reviewed commit: `28c701543406cd1093dce608fee95f331d28a652`.

Evidence level: checked by reading revision 2, revision 1, round 1, the parent design, and the cited source. I ran a read-only check of register `Site` lines. I ran no code or runtime test.

## Material findings

### CCB-FU-01, reopened

Severity: high  
Material: yes

Claim: Revision 2 still does not apply the `session stop` human gate. It records holder coordinates, but does not bind the cancellation target or the mutation to the current holder and lease epoch. It also omits two enforced parts of the existing gate.

The design says, "An admitted proof is bound to the checkout: `HolderMainId == \"\"` or `ClaimEpoch < 1` is a plain error" and later says, "the current holder and lease epoch [are] bound and recorded with the human's process identity" (`metasystem/artifacts/reports/ccb-followup-design-r2.md:82`, `metasystem/artifacts/reports/ccb-followup-design-r2.md:119`). Its command path reads the holder before calling the steward and passes the resulting strings and number as values (`metasystem/artifacts/reports/ccb-followup-design-r2.md:103`).

Evidence, checked by reading:

- `session stop` does not accept copied coordinates merely because they are nonempty. `WriteSessionStop` rereads the lease at the persistence boundary and requires its holder main id and epoch to equal the marker before it writes (`metasystem/internal/goal/sessionstop.go:263-270`). It then binds the current announcement lifecycle (`metasystem/internal/goal/sessionstop.go:271-275`).
- The proposed human branch has no equivalent currentness check. `lease.CurrentHolder` is expressly a diagnostic view that reads the recorded holder without classifying a caller (`metasystem/internal/lease/verbs.go:255-268`). The proposed verb reads that view before `CancelHandoff` takes steward arbitration. The proposed steward check tests only nonempty main id and a positive epoch. A lease change after the read is not detected.
- A cancellation names an arbitrary nonce, unlike `session stop`, whose target session is taken from the current holder. The live handoff already carries its recorder's main id and session (`metasystem/internal/steward/handoff.go:25-35`; populated at `metasystem/internal/steward/handoff_capture.go:834-837`). The proposed human branch never compares either field with `HolderMainId` or `HolderSession`. Therefore a human can cancel a live handoff that does not belong to the current holder. `ClaimEpoch` cannot repair that omission because the handoff binding carries no epoch.
- The design accepts only a nonblank `--by` at the command boundary (`metasystem/artifacts/reports/ccb-followup-design-r2.md:101`). The existing gate trims the name, rejects more than 200 bytes, and rejects control characters before persistence (`metasystem/internal/goal/sessionstop.go:102-110`, `metasystem/internal/goal/sessionstop.go:128-132`). Revision 2 would accept and record names that `session stop` refuses.
- The design says the human process identity is recorded, but its required success witness expects `human-pid=0`, `human-started=0`, and zero tick and boot values (`metasystem/artifacts/reports/ccb-followup-design-r2.md:160`). `TerminalValidFor` permits those zeros for a fixture-only proof (`metasystem/internal/humanauthority/authority.go:109-117`, `metasystem/internal/humanauthority/authority.go:203-208`). `session stop` instead refuses a marker whose human pid or start time is missing or whose identity mode is invalid (`metasystem/internal/goal/sessionstop.go:141-142`). The named test freezes a weaker steward contract.

The announced-session exception is not the defect. The Stop hook does run `up --retire` on `end` (`metasystem/scripts/agents/supervision-hook.sh:1494-1509`), `up.Retire` calls `lease.Retire` (`metasystem/internal/up/up.go:817-828`), and retirement removes the matching announcement (`metasystem/internal/lease/verbs.go:214-233`). `CurrentHolder` retains the lease main id and epoch while deriving the session only from a remaining announcement (`metasystem/internal/lease/verbs.go:268-289`). Omitting the announced-session precondition is therefore a faithful adaptation for a dead recorder. It does not justify omitting a target-to-holder match and a mutation-time lease check.

An implementer following this design would build an attributed cancellation, not a holder-bound cancellation. The fold needs an owner that proves the supplied holder and epoch are still current when cancellation commits, a rule that ties the nonce to that holder, the full human-name and process-identity checks, and failing tests for a different holder, a moved epoch, an invalid name, and a missing process identity.

Rigor: severe. Facts: `local=true`, `recoverable=true`, `proofBoundaryCrossed=false`, `authorityBoundaryCrossed=true`, `secretsBoundaryCrossed=false`, `irreversibleDataBoundaryCrossed=false`, `externalSideEffectBoundaryCrossed=false`. Reopening trigger: a design and named tests that prove cancellation commits only for a nonce belonging to the still-current holder and epoch, with the same name and human-process requirements as `session stop` except for the justified absent announcement.

### CCB-FU-04, reopened

Severity: high  
Material: yes

Claim: The line estimates are credible, but the two-unit sequence does not meet the ruling that each unit land on its own. Unit F1 removes the only human recovery path before unit F2 supplies the authenticated replacement.

The design says, "Two units, F1 then F2, each landing on its own," then states, "Between the two landings the verb cancels only by the recorder's identity: a held handoff whose recorder is dead waits for F2" (`metasystem/artifacts/reports/ccb-followup-design-r2.md:208`, `metasystem/artifacts/reports/ccb-followup-design-r2.md:214`). It repeats that exposure in its final note (`metasystem/artifacts/reports/ccb-followup-design-r2.md:251`).

Evidence, checked by reading:

- Today the cancel command calls `CancelHandoff` without a caller, and a live handoff is cancelled after only liveness checks (`metasystem/cmd/metasystem/context_verbs.go:162-167`; `metasystem/internal/steward/handoff_capture.go:1029-1054`). A person can therefore recover a dead recorder's held handoff today.
- Under F1, that person classifies `HUMAN`, then the unchanged admission switch returns `HANDOFF_NOT_HOLDER` for every class other than `MAIN` and `DELEGATE` (`metasystem/internal/steward/handoff_capture.go:302-329`). F2's `--by` leg is the first path that restores human cancellation.
- The gap is not bounded by expiry. The parent design explicitly leaves held-handoff expiry for Wido (`metasystem/plans/coordinator-context-stays-under-budget-design.md:313-315`), and the shipped expiry owner always returns false (`metasystem/internal/steward/handoff.go:18-21`, `metasystem/internal/steward/handoff.go:48-52`). A crash or interruption after F1 can leave the command unable to cancel a hold without another code landing.

The estimates of about 275 and 315 changed lines include both register resyncs and the new test file boundary, so they are plausible. The stop-and-split rule above 400 is also explicit. Those facts do not make F1 a standalone behavioral unit. The split must preserve today's recovery behavior until the complete authenticated cutover lands, or make the cutover atomic.

Rigor: severe. Facts: `local=true`, `recoverable=true`, `proofBoundaryCrossed=false`, `authorityBoundaryCrossed=true`, `secretsBoundaryCrossed=false`, `irreversibleDataBoundaryCrossed=false`, `externalSideEffectBoundaryCrossed=false`. Reopening trigger: a landing order in which every intermediate commit still permits a human to cancel a dead recorder's held handoff, with a named test run against each intermediate unit.

### CCB-FU-05

Severity: medium  
Material: yes

Claim: The two-line change to `proveSessionStopHuman` still lacks a one-rule, one-failing-test witness for its second error return.

The design requires both error paths to retain the populated proof, naming `return proof, err` and `return proof, fmt.Errorf(...)` (`metasystem/artifacts/reports/ccb-followup-design-r2.md:106`). Its witness drives an unknown pid so `stableRead` fails before the signature set is read (`metasystem/artifacts/reports/ccb-followup-design-r2.md:159`).

Evidence, checked by reading:

- The current helper has two distinct proof-discarding returns. One handles an error from `ProveTerminal`. The other handles a nil-error proof that fails `TerminalValidFor` (`metasystem/cmd/metasystem/session_stop.go:22-30`). The proposed test reaches only the first return. Reverting only the second proposed return to `humanauthority.Proof{}` leaves that test green.
- The change does not alter `session stop` command behavior. `runSessionStop` exits on the error and never uses the returned proof (`metasystem/cmd/metasystem/session_stop.go:68-72`). `TestSessionStopAttendedHumanEndsQuietly` and `TestSessionStopAgentClassifiedCallerCannotReachTheWriter` stub the helper and remain unchanged (`metasystem/cmd/metasystem/session_stop_test.go:193-259`, with stubs at `:211-213` and `:245-248`). Only the helper's new caller observes the populated error result.

An implementer can either leave the second return unchanged because the new cancellation path does not need it, or add a deterministic witness for that branch. Revision 2 currently commands the change while its mutation table cannot prove it.

Rigor: severe. Facts: `local=true`, `recoverable=true`, `proofBoundaryCrossed=true`, `authorityBoundaryCrossed=false`, `secretsBoundaryCrossed=false`, `irreversibleDataBoundaryCrossed=false`, `externalSideEffectBoundaryCrossed=false`. Reopening trigger: a named test that fails when only the post-`TerminalValidFor` return discards the proof, or removal of that unnecessary return change from the design.

### CCB-FU-06

Severity: medium  
Material: yes

Claim: The exact `HANDOFF_OTHER_SESSION` register row freezes an override that is not a valid invocation.

The design requires `Override: "context handoff --cancel --by <human> typed at an enrolled or agent-free terminal"` and makes `TestHCL03HandoffCancelRows` compare it exactly (`metasystem/artifacts/reports/ccb-followup-design-r2.md:127`, `metasystem/artifacts/reports/ccb-followup-design-r2.md:148`).

Evidence, checked by reading:

- The design's own usage contract requires `--root ROOT --cancel NONCE [--by HUMAN]` (`metasystem/artifacts/reports/ccb-followup-design-r2.md:101`). The current command likewise requires a root and a nonce value after `--cancel` (`metasystem/cmd/metasystem/context_verbs.go:139-156`). In the proposed override, `--by` occupies the missing value position for `--cancel`, so the text does not describe the one command that carries past the refusal.
- The register defines `Override` as "The human verb that carries past it" and counts commands from intent to effect (`metasystem/internal/refusal/register.go:13-20`). The proposed exact-row test would preserve the wrong verb rather than catch it.

Rigor: bounded. Facts: `local=true`, `recoverable=true`, `proofBoundaryCrossed=false`, `authorityBoundaryCrossed=false`, `secretsBoundaryCrossed=false`, `irreversibleDataBoundaryCrossed=false`, `externalSideEffectBoundaryCrossed=false`. Reopening trigger: the row and its exact-field test name the complete one-command override, including the required root and nonce positions.

## Non-material findings and closure checks

### CCB-FU-02

Severity: low  
Material: no

Claim: Revision 2 genuinely closes the refusal-mapping finding.

The design now says every error from `admitHandoffCaller` passes through unchanged and reserves `HANDOFF_OTHER_SESSION` for a caller that passed admission but failed the binding comparison (`metasystem/artifacts/reports/ccb-followup-design-r2.md:56-75`). Current admission emits `HANDOFF_NOT_HOLDER` on incomplete holder or delegate identity and `HANDOFF_UNOBSERVABLE` for an unobservable runtime (`metasystem/internal/steward/handoff_capture.go:262-329`). F1-R3 and F1-R4 give those two stages separate exact-output tests (`metasystem/artifacts/reports/ccb-followup-design-r2.md:144-145`). This is a real control-flow correction, not a restatement.

### CCB-FU-03

Severity: low  
Material: no

Claim: Revision 2 genuinely closes the root-scoped refusal-token finding.

The design specifies the exact new line `HANDOFF_HUMAN_UNPROVEN nonce=<nonce> by=<by> caller=<class> human=<token>` and distinguishes `unattempted`, a refused proof's `<Outcome>`, `other-root`, and `unobserved`; `none` remains only on `HANDOFF_OTHER_SESSION` when no human leg was offered (`metasystem/artifacts/reports/ccb-followup-design-r2.md:89-97`). `ProveTerminal` does return a populated outcome on ancestry failures (`metasystem/internal/humanauthority/authority.go:641-647`, `metasystem/internal/humanauthority/authority.go:664-671`), while root validity is a separate check (`metasystem/internal/humanauthority/authority.go:193-208`). F1-R3 checks `none`; F2-R8, F2-R9, F2-R10, and F2-R11 independently check `unattempted`, `<Outcome>`, `other-root`, and `unobserved` (`metasystem/artifacts/reports/ccb-followup-design-r2.md:144`, `metasystem/artifacts/reports/ccb-followup-design-r2.md:162-165`). Removing any one token branch changes that row's exact output.

### CCB-FU-N05

Severity: low  
Material: no

Claim: The proposed register `Site` test is narrow enough to pass on today's register. It does not activate the repository's stale global lines.

The design expressly scopes the test to rows whose owner is `internal/steward` and whose site starts with `handoff_capture.go:` (`metasystem/artifacts/reports/ccb-followup-design-r2.md:149`). A read-only line check found all eleven current rows in that scope point to lines containing their code (`metasystem/internal/refusal/register.go:47-57`). The known stale `BRIEF_AUTHORITY_REFUSED` row is owned by `internal/dispatch`, says `brief.go:34`, and emits at line 37 (`metasystem/internal/refusal/register.go:80`; `metasystem/internal/dispatch/brief.go:36-38`). The wider set has 28 wrong lines among the 54 rows in the previously measured owner set, but none enters this test. The gate will not break on landing for that reason.

### CCB-FU-N06

Severity: low  
Material: no

Claim: Revision 2 does not decide amendment 8c.7, the held-handoff expiry policy.

The design says it "decides nothing about it" and leaves a future expiry to `handoffExpiryRule` with a separately named notice owner (`metasystem/artifacts/reports/ccb-followup-design-r2.md:244-250`). The parent page still asks Wido whether a held handoff should expire and takes no expiry until he answers (`metasystem/plans/coordinator-context-stays-under-budget-design.md:313-315`). The shipped rule always returns false (`metasystem/internal/steward/handoff.go:48-52`). This is an explicit non-goal, so it does not change this follow-up's build. The sentence "No open question for Wido" at revision 2 line 248 means no new question, not that 8c.7 was settled.

Verdict: fold these findings first.
