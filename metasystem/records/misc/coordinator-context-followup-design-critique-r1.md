# Follow-up design critique, round 1

Reviewed commit: `484633d86607f72a9f53906dcd92e2d2c7d3be73`.

Evidence level: checked by reading the design and the cited source. No code or runtime test was run.

## Material findings

### CCB-FU-01

Severity: high  
Material: yes

Claim: The proposed human cancellation rule is not the same authorization bar as `session stop`. The design uses that false equivalence to choose a weaker, unattributed human operation without asking for the policy decision.

The design says, "This is the bar `session stop` sets for a person ending a session's authorization" and later, "The human bar for cancellation is decided by precedent" (`metasystem/artifacts/reports/ccb-followup-design.md:39`, `metasystem/artifacts/reports/ccb-followup-design.md:195`).

Evidence, checked by reading:

- `session stop` requires a non-empty `--by` value (`metasystem/cmd/metasystem/session_stop.go:40-48`). It classifies the command itself and requires `lease.ClassHuman` before proving terminal ancestry (`metasystem/cmd/metasystem/session_stop.go:56-68`). It also binds the mutation to the current holder, announced session, and lease epoch, and records the named human and the human process identity (`metasystem/cmd/metasystem/session_stop.go:73-100`). The helper at lines 22 to 31 is only one part of that gate.
- The proposed cancellation syntax keeps the current usage with no `--by` (`metasystem/cmd/metasystem/context_verbs.go:139-157`). The proposed human leg checks only `TerminalValidFor`. It does not require the classified caller to be `HUMAN`, the current holder, a lease epoch, or an attributed human name (`metasystem/artifacts/reports/ccb-followup-design.md:39`, `metasystem/artifacts/reports/ccb-followup-design.md:83-109`).
- `TerminalValidFor` accepts either an enrolled proof or an agent-free controlling-terminal proof (`metasystem/internal/humanauthority/authority.go:200-208`). `ProveTerminal` does reject a harness anywhere in the ancestry (`metasystem/internal/humanauthority/authority.go:628-671`, `metasystem/internal/humanauthority/authority.go:719-743`). This proves that agents cannot use the human leg. It does not make the rest of the `session stop` gate redundant.
- The parent design requires a caller that is the recorded session's holder or a human act. It does not choose terminal-grade rather than enrolled-grade authority, nor does it waive attribution (`metasystem/plans/coordinator-context-stays-under-budget-design.md:578-580`). Those choices change the accepted caller set and the command contract.
- The command also classifies the caller before it obtains the human proof. A classification-store error returns exit 1 before a valid human proof can reach the steward (`metasystem/cmd/metasystem/context_verbs.go:191-223`; proposed order at `metasystem/artifacts/reports/ccb-followup-design.md:83-109`). The claimed human-first recovery leg is therefore not first at the command boundary.

The identity outcomes themselves are otherwise sound. A foreign current holder is admitted as a holder but fails the binding comparison. A delegate of another job gets no job identifier because `HookDelegate` does not authenticate it (`metasystem/cmd/metasystem/context_verbs.go:201-210`, `metasystem/internal/lease/hook_delegate.go:54-90`). A dead recorder cannot have a live authenticated process in the caller ancestry (`metasystem/internal/lease/classify.go:398-415`). A later process that reuses the session string still fails the full process identity and tag comparison because those fields are in the binding (`metasystem/internal/steward/handoff.go:25-35`, `metasystem/internal/steward/handoff_state.go:314-316`). The unresolved point is which human gate may override those denials.

Rigor: severe. Facts: `local=true`, `recoverable=true`, `proofBoundaryCrossed=false`, `authorityBoundaryCrossed=true`, `secretsBoundaryCrossed=false`, `irreversibleDataBoundaryCrossed=false`, `externalSideEffectBoundaryCrossed=false`. Reopening trigger: a cited human ruling or owning policy that explicitly authorizes terminal-grade, unattributed cancellation of any live handoff nonce.

### CCB-FU-02

Severity: medium  
Material: yes

Claim: `HANDOFF_OTHER_SESSION` is needed, but only after a caller passes admission and then fails the live binding comparison. Replacing every admission refusal with that code gives the wrong result for a foreign delegate, an unobservable runtime, and the recording session after it loses the lease.

The design says, "a `HandoffRefusal` returned by `admitHandoffCaller` ... is replaced by this refusal" (`metasystem/artifacts/reports/ccb-followup-design.md:64`).

Evidence, checked by reading:

- A delegate of another job fails before any session comparison. `activeDelegateCaller` returns `HANDOFF_NOT_HOLDER` when the caller has no authenticated active job or names a different job (`metasystem/internal/steward/handoff_capture.go:262-269`). Calling that `HANDOFF_OTHER_SESSION` invents a session judgment the code never established.
- A recording main that still has the recorded identity but no longer holds the checkout fails the holder condition (`metasystem/internal/steward/handoff_capture.go:302-311`). The design itself accepts this denial (`metasystem/artifacts/reports/ccb-followup-design.md:54`). Relabelling it `HANDOFF_OTHER_SESSION` is false because it can be the same recorded session.
- An unobservable main or delegate returns `HANDOFF_UNOBSERVABLE` from the shared admission path (`metasystem/internal/steward/handoff_capture.go:291-329`). Replacing it hides the actual failed invariant.
- A new current holder from another session passes `admitHandoffCaller` and then fails the proposed field-for-field binding match. This is the case for which `HANDOFF_OTHER_SESSION` is accurate. `HANDOFF_NOT_HOLDER` would be false there because the caller is the current holder (`metasystem/internal/steward/handoff_capture.go:302-311`, `metasystem/internal/steward/handoff_capture.go:834-837`).

The new code should therefore exist for a post-admission binding mismatch. Existing admission codes should pass through unchanged. With that scope, the proposed exact text and an `Agent` register row are coherent. The register `Site` must be the actual new refusal emission line, not the literal placeholder `handoff_capture.go:<line>` (`metasystem/artifacts/reports/ccb-followup-design.md:66-70`).

Rigor: bounded. Facts: `local=true`, `recoverable=true`, `proofBoundaryCrossed=false`, `authorityBoundaryCrossed=false`, `secretsBoundaryCrossed=false`, `irreversibleDataBoundaryCrossed=false`, `externalSideEffectBoundaryCrossed=false`. Reopening trigger: a new cancellation admission path that deliberately collapses its own typed refusal into a binding-mismatch result.

### CCB-FU-03

Severity: medium  
Material: yes

Claim: The exact `human=<outcome>` refusal field does not report why human authorization failed. The command discards real failure outcomes, while a wrong-root proof reports `HUMAN_AUTHORITY_PROVEN` even though it is invalid for this cancellation.

The design says, "`<outcome>` is `canceller.Human.Outcome` ... for example `TERMINAL_NOT_ENROLLED` or `AGENT_IN_AUTHORITY_CHAIN`" and "A proof error ... yields a zero proof, `human=none`" (`metasystem/artifacts/reports/ccb-followup-design.md:64`, `metasystem/artifacts/reports/ccb-followup-design.md:109`).

Evidence, checked by reading:

- `ProveTerminal` returns a populated proof together with an error when ancestry fails. An agent chain sets `Outcome` to `AGENT_IN_AUTHORITY_CHAIN` (`metasystem/internal/humanauthority/authority.go:641-671`, `metasystem/internal/humanauthority/authority.go:719-743`). The proposed command seam discards that proof on every error, so those named outcomes cannot reach the command's refusal text (`metasystem/artifacts/reports/ccb-followup-design.md:99-106`).
- `FixtureGoalProof` sets `Outcome` to `HUMAN_AUTHORITY_PROVEN` (`metasystem/internal/humanauthority/authority.go:175-190`). `TerminalValidFor` separately rejects it when its observed root differs (`metasystem/internal/humanauthority/authority.go:200-208`). The R5 wrong-root row would therefore print `human=HUMAN_AUTHORITY_PROVEN` for a proof that did not authorize the operation (`metasystem/artifacts/reports/ccb-followup-design.md:125`).

An implementer following the exact-text rule would freeze misleading output and tests. The detail needs to represent authorization for this root, or the design must stop claiming that it reports the proof failure outcome.

Rigor: bounded. Facts: `local=true`, `recoverable=true`, `proofBoundaryCrossed=false`, `authorityBoundaryCrossed=false`, `secretsBoundaryCrossed=false`, `irreversibleDataBoundaryCrossed=false`, `externalSideEffectBoundaryCrossed=false`. Reopening trigger: a proof result type that carries a root-scoped validation outcome without discarding the failed in-process observation.

### CCB-FU-04

Severity: medium  
Material: yes

Claim: The rule-to-witness table does not meet its own one-rule, one-failing-test contract, and the declared Boundary omits the file needed to prove the register row.

The design says, "Each row is one rule the builder adds and the one Go test that fails when that rule alone is removed" and makes that rule verbatim in the builder brief (`metasystem/artifacts/reports/ccb-followup-design.md:115-117`, `metasystem/artifacts/reports/ccb-followup-design.md:174`).

Evidence, checked by reading:

- R9 names `useContextHandoffIdentity`, which is a helper, not a test (`metasystem/artifacts/reports/ccb-followup-design.md:129`; `metasystem/cmd/metasystem/context_verbs_test.go:822-842`). Removing only its proposed prover stub can still pass under an agent-run test process because the real terminal proof fails on the agent ancestry. There is no named test that asserts the stub is installed.
- R1 uses a zero caller (`metasystem/artifacts/reports/ccb-followup-design.md:121`). It proves that a human proof can cancel. It does not fail if an implementation tries the recorder first and falls back to the human proof. It therefore does not prove the stated human-first priority. The existing main caller fixture and fixture human proof make a both-valid witness possible (`metasystem/internal/steward/handoff_capture_test.go:89-92`, `metasystem/internal/humanauthority/authority.go:175-190`).
- R5 does not include the explicitly promised `HANDOFF_UNOBSERVABLE` remapping. That path is distinct in current admission (`metasystem/internal/steward/handoff_capture.go:291-297`). Removing only that remapping would leave all eight named R5 rows green.
- R11 proves only that a code has some row. `TestHCL03EveryCodeRowed` checks membership (`metasystem/internal/refusal/register_test.go:20-49`). `TestHCL03PendingRowsNamed` checks that an `Agent` row has a non-empty override (`metasystem/internal/refusal/register_test.go:71-87`). Neither test asserts the exact `Site`, `Shape`, `Override`, or `Commands` values promised by D15-3. The design itself says no test checks `Site` (`metasystem/artifacts/reports/ccb-followup-design.md:72`).
- The Boundary includes `metasystem/internal/refusal/register.go` but omits `metasystem/internal/refusal/register_test.go` (`metasystem/artifacts/reports/ccb-followup-design.md:171`). That omission prevents a witness for the exact register row without crossing the Boundary.

The single-unit size is plausible but not established. The register estimate is already low under the repository's `git diff --numstat` convention: changing eleven existing rows costs twenty-two changed lines, and adding the new row costs one, not thirteen (`metasystem/internal/refusal/register.go:47-57`; the convention is recorded at `metasystem/records/misc/coordinator-context-slice3-unit-d1a-code-read.md:197-204`). The design's split-at-400 instruction prevents an oversized return if followed. The count must be redone after adding the missing witnesses and Boundary file.

Rigor: severe. Facts: `local=true`, `recoverable=true`, `proofBoundaryCrossed=true`, `authorityBoundaryCrossed=false`, `secretsBoundaryCrossed=false`, `irreversibleDataBoundaryCrossed=false`, `externalSideEffectBoundaryCrossed=false`. Reopening trigger: named mutation results showing each priority, refusal mapping, prover stub, and exact register-field rule fails independently within the declared Boundary.

## Non-material findings

### CCB-FU-N01

Severity: low  
Material: no

Claim: The intent-path projection fix is correct, but the reader inventory is not complete and not every internal reader canonicalizes at the same boundary.

The design says, "Readers of the intent path or the intents directory" and lists the readers it leaves alone (`metasystem/artifacts/reports/ccb-followup-design.md:135-155`).

Evidence, checked by reading:

- `Handoff` canonicalizes its root before creating the state and intent records (`metasystem/internal/steward/handoff_capture.go:896-914`). `StatePath` is made from that root (`metasystem/internal/steward/handoff_capture.go:948-957`). Setting `IntentPath` from `intentsDir(root)` after the exact live read therefore gives both JSON paths one canonical root (`metasystem/internal/steward/handoff_capture.go:995-1004`).
- Supersession runs inside that canonicalized `Handoff` call (`metasystem/internal/steward/handoff_capture.go:931-984`). `PruneContext` and the Stop allowance canonicalize themselves (`metasystem/internal/steward/handoff_retention.go:203-214`, `metasystem/internal/steward/handoff_capture.go:1071-1077`).
- `CompleteRevival` reads `LiveIntents` and consumes the intent from the `repoRoot` it receives (`metasystem/internal/steward/revive.go:93-100`, `metasystem/internal/steward/revive.go:184-200`). The resident runner passes a canonical root (`metasystem/internal/steward/runner.go:114-171`), but the direct `steward revive --repo` command passes its raw flag value (`metasystem/cmd/metasystem/steward_verbs.go:434-447`, `metasystem/cmd/metasystem/steward_verbs.go:493`). Binding validation later canonicalizes the root (`metasystem/internal/steward/handoff_state.go:368-399`).
- The inventory omits other directory readers through `LiveIntents`, including notification authorization and intent update (`metasystem/internal/steward/notify.go:170-183`, `metasystem/internal/steward/notify.go:205-219`) and the tick decision (`metasystem/internal/steward/tick.go:342-375`). None consumes the JSON projection.

These omissions do not change what this unit should build. The only faulty path exposed to a program is the command projection, and D15-6 moves that path to the steward result.

### CCB-FU-N02

Severity: low  
Material: no

Claim: The follow-up does not silently decide amendment 8c.7. Its statement about a future expiry notification is inaccurate.

The design says, "This design does not depend on 8c.7" and that a future expiry "cancels through the tick's notify path" (`metasystem/artifacts/reports/ccb-followup-design.md:191-195`).

Evidence, checked by reading:

- The existing expiry rule always returns false (`metasystem/internal/steward/handoff.go:18-21`, `metasystem/internal/steward/handoff.go:48-52`). It is consulted by launch admission and the Stop allowance (`metasystem/internal/steward/handoff.go:89-91`, `metasystem/internal/steward/handoff_capture.go:1082-1087`). This follow-up changes none of those lines.
- If that rule later returns true, `decideForHandoff` returns `ActNotify` (`metasystem/internal/steward/handoff.go:89-91`). `CompleteRevival` then calls `CancelIntent` at line 143, but it does not queue that decision's notification (`metasystem/internal/steward/revive.go:135-146`). The notification at tick lines 215 to 224 is for the earlier `RunTick` decision, not the later re-arbitrated `CompleteRevival` decision (`metasystem/internal/steward/tick.go:211-224`).

The current build still leaves expiry open. A later expiry design must add or identify the notice owner instead of relying on `revive.go:143`.

### CCB-FU-N03

Severity: low  
Material: no

Claim: The reason given for keeping `HandoffResult.IntentPath` empty on every error is too broad.

The design says, "a failed preparation leaves the record in `cancelledDir`" (`metasystem/artifacts/reports/ccb-followup-design.md:111`).

Evidence, checked by reading:

- Preparation can fail while reading the enrollment fence, before the live intent is minted (`metasystem/internal/steward/revive.go:50-60`).
- If receipt recording fails, cancellation can also fail and leave a live half-prepared intent (`metasystem/internal/steward/revive.go:65-85`).
- The `Handoff` error path merely tries to read a cancelled record if one exists (`metasystem/internal/steward/handoff_capture.go:985-999`).

Assigning `IntentPath` only after the successful exact read remains a clear success-result contract. The inaccurate explanation does not change that build.

### CCB-FU-N04

Severity: low  
Material: no

Claim: Apart from the material contradictions above and the three bounded inaccuracies below, the design's current-code line claims hold on the reviewed commit.

The design says every code claim names a file and line (`metasystem/artifacts/reports/ccb-followup-design.md:3`).

Evidence, checked by reading:

| Design lines | Current-code anchors checked | Result |
| --- | --- | --- |
| 9 | `metasystem/cmd/metasystem/context_verbs.go:152-224`; `metasystem/internal/steward/handoff_capture.go:116-129`, `metasystem/internal/steward/handoff_capture.go:1003-1066`; `metasystem/internal/steward/intervene.go:88-110`; `metasystem/plans/coordinator-context-stays-under-budget-design.md:251` | Holds. Cancellation has no caller, uses the canonical root and arbitration, removes the live record first, writes the stated outcome, and clears the handoff notice. |
| 11 and 17 | `metasystem/internal/lease/classify.go:383-471`; `metasystem/internal/lease/hook_delegate.go:54-90`; `metasystem/internal/lease/verbs.go:268-290`; `metasystem/internal/steward/handoff_capture.go:45-50`, `metasystem/internal/steward/handoff_capture.go:262-330`, `metasystem/internal/steward/handoff_capture.go:834-837`; `metasystem/internal/steward/handoff.go:25-35`; `metasystem/internal/steward/handoff_state.go:314-316` | Holds. This is the current record-path authentication and binding shape. |
| 13 | `metasystem/cmd/metasystem/context_verbs.go:183-184`; `metasystem/internal/goal/project.go:236-259`; `metasystem/internal/steward/handoff_state.go:154-164`; `metasystem/internal/steward/intervene.go:63-66`; `metasystem/internal/steward/handoff_capture.go:995-1004`; `metasystem/internal/steward/stage.go:22-25` | Holds. The command joins the non-canonical intent path while the steward state path comes from a canonical root. |
| 31 | `metasystem/internal/steward/runner.go:236-248`; `metasystem/internal/humanauthority/authority.go:20-27` | Holds. The package import graph already permits steward to use `humanauthority`. |
| 33, 41, 54, and 56 | `metasystem/internal/steward/handoff_capture.go:262-330`, `metasystem/internal/steward/handoff_capture.go:834-837`, `metasystem/internal/steward/handoff_capture.go:1031-1054`; `metasystem/internal/steward/handoff_state.go:304-317`; `metasystem/internal/lease/classify.go:442-449` | The current mechanics hold. The claimed `session stop` equivalence and command-level human-first ordering do not. Those are CCB-FU-01. |
| 58, 64, 66, and 72 | `metasystem/internal/steward/handoff_capture.go:58-68`, `metasystem/internal/steward/handoff_capture.go:106`, `metasystem/internal/steward/handoff_capture.go:262-329`, `metasystem/internal/steward/handoff_capture.go:931-938`; `metasystem/internal/refusal/register.go:35-57`; `metasystem/internal/refusal/register_test.go:20-49`, `metasystem/internal/refusal/register_test.go:71-87`, `metasystem/internal/refusal/register_test.go:115-152` | The cited functions and test behavior hold. The proposed refusal collapse and exact-row proof do not. Those are CCB-FU-02 to CCB-FU-04. |
| 74 to 81 | `metasystem/internal/steward/handoff_capture.go:1031-1066`; `metasystem/internal/steward/revive.go:93-100`, `metasystem/internal/steward/revive.go:184-200`; `metasystem/internal/steward/intervene.go:173-200`; `metasystem/internal/steward/handoff.go:89-124`; `metasystem/internal/steward/tick.go:104-114` | Holds. Liveness precedes identity, consumption wins over later cancellation, and the three transitions share arbitration. |
| 83 to 109 | `metasystem/cmd/metasystem/context_verbs.go:45-52`, `metasystem/cmd/metasystem/context_verbs.go:139-224`, `metasystem/cmd/metasystem/context_verbs.go:313-321` | The cited current branch, seams, usage, caller resolver, and exit mapping hold. The proposed new order has the issue in CCB-FU-01. |
| 111 | `metasystem/internal/steward/handoff_capture.go:70-75`, `metasystem/internal/steward/handoff_capture.go:896-1004`; `metasystem/internal/steward/stage.go:22-25` | The success-path anchors hold. The every-error explanation does not, as CCB-FU-N03 records. |
| 113 | `metasystem/internal/steward/handoff_capture.go:931-984`, `metasystem/internal/steward/handoff_capture.go:1071-1145`; `metasystem/internal/steward/revive.go:74-189`; `metasystem/internal/steward/handoff_retention.go:201-214`; `metasystem/internal/steward/stage.go:127-180` | Holds for the named call sites and non-goals. No role contract or `metasystem/docs/orchestration.md` text mentions `--cancel`. The reader inventory is incomplete as CCB-FU-N01 records. |
| 121 to 131 | `metasystem/internal/steward/handoff_capture_test.go:59`, `metasystem/internal/steward/handoff_capture_test.go:89-92`, `metasystem/internal/steward/handoff_capture_test.go:655-717`, `metasystem/internal/steward/handoff_capture_test.go:899-923`; `metasystem/internal/fixtureauth/fixtureauth.go:257-273`, `metasystem/internal/fixtureauth/fixtureauth.go:286-293`; `metasystem/cmd/metasystem/context_verbs_test.go:647-713`, `metasystem/cmd/metasystem/context_verbs_test.go:789-842`; `metasystem/internal/refusal/register.go:35`; `metasystem/internal/refusal/register_test.go:20-87` | The cited existing fixtures and tests exist and behave as described. The proposed new test names do not exist yet, as expected. Witness sufficiency does not hold for R1, R5, R9, or the exact R11 row, as CCB-FU-04 records. |
| 139 to 142 | `metasystem/cmd/metasystem/context_verbs.go:163`; all eleven `CancelHandoff` test calls at `metasystem/internal/steward/handoff_capture_test.go:891`, `:920`, `:951`, `:1037`, `:1075`, `:1144`, `:1196`, `:1240`, `:1262`, `:1313`, and `:1394`; `handoffMainCaller` at `:89-92`; supersession at `metasystem/internal/steward/handoff_capture.go:973-984` | Holds. The caller inventory is complete. |
| 146 to 155 | `metasystem/cmd/metasystem/context_verbs.go:184`; `metasystem/cmd/metasystem/context_verbs_test.go:672-673`; `metasystem/internal/steward/intervene.go:64-66`; `metasystem/internal/steward/handoff_capture.go:975`, `:1004`, `:1055`; `metasystem/internal/steward/handoff_retention.go:79-93`, `:185-198`; `metasystem/internal/steward/revive.go:100`, `:189`; `metasystem/internal/lease/verbs.go:321`; `metasystem/internal/watch/watch.go:202`, `:704-706`; `metasystem/scripts/agents/supervision-hook-fixtures.sh:895`, `:2067`; `metasystem/scripts/agents/goal-cli-fixtures.sh:1083`; `metasystem/scripts/agents/dispatch-fixtures.sh:5473`; `metasystem/cmd/metasystem/goal.go:710-712` | The individual paths hold except `dispatch-fixtures.sh:5473`, which uses `ls`, not `find`. The broader inventory omissions are CCB-FU-N01. |
| 157 to 162 | `metasystem/internal/steward/handoff_capture_test.go:880-1404`; `metasystem/cmd/metasystem/context_verbs_test.go:647-770`, `metasystem/cmd/metasystem/context_verbs_test.go:789-842`; `metasystem/internal/refusal/register_test.go:20-49` | Holds as a description of current tests and planned edits. It does not prove the new rules listed in CCB-FU-04. |
| 193 and 195 | `metasystem/internal/steward/handoff.go:18-21`, `metasystem/internal/steward/handoff.go:48-52`, `metasystem/internal/steward/handoff.go:89-91`; `metasystem/internal/steward/handoff_capture.go:1082-1087`; `metasystem/internal/steward/revive.go:139-146`; `metasystem/cmd/metasystem/session_stop.go:22-100`; `metasystem/internal/humanauthority/authority.go:193-208` | The no-expiry state and rule locations hold. The future notify-path claim and the claimed full `session stop` precedent do not. See CCB-FU-N02 and CCB-FU-01. |

Verdict: fold these findings first.
