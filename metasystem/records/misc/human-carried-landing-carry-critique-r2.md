# Critique register: human-carried-landing-carry design, read 2

Job hcl-crit2-20260911 (codex, gpt-5.6-sol, design-critic) reviewed revision 4 at local scaffold 4d4dfa54a (sha256 5e4ad956cf52bad3274f69b478ff5fb850a73612a00ee4353edd566cdbbf2f87). Material findings: 9 of 9. Projected verbatim from artifacts/agents/hcl-crit2-20260911/rounds/1/return.json.

## HCL-C-33 (critical)

**Claim.** HCL-C-33 (HCL-COUNTER-08 and HCL-TRANSACTION-06): two seats can stack carried landings during the gap between the first code push and publication of its review obligation. Seat A pushes its carried code commit; before A runs goal carried, seat B fetches that commit, sees no open human-carried obligation, passes the debt check, and pushes a second carried commit. The local carrying journal is invisible to B. What changes if this stands: the protocol needs a fleet-visible in-flight debt or reservation before the code push, or another serialization rule covering this interval, plus a two-seat interleaving fixture.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:753-795 puts the local carrying entry and code push before goal carried publishes the obligation. metasystem/plans/human-carried-landing-carry-design.md:1158-1173 defines debt solely as an already-open human-carried obligation. Nothing visible to another seat represents A's carry during that interval.

## HCL-C-21 (critical)

**Claim.** HCL-C-21 (HCL-LANDING-05): the revised match rule still lands two defects under one code word because it treats an empty set M as proof that testing has no defect. A verification error explicitly produces battery unverified and empty M; delivery may also be insufficient only because of uncovered obligations or discrepancies, which are omitted from M. An ordinary refusal code then matches and lands alongside that testing defect. What changes if this stands: code matches must require a sufficient green testing result, group matches must exclude every other insufficiency dimension, and fixtures must cover verification failure, uncovered obligations, and discrepancies.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:535-548 makes M only missing plus failing groups, sets M empty on a verify error, and permits a code word whenever M is empty. metasystem/internal/proofrun/test_result.go:201-241 shows that uncovered obligations and discrepancies independently make delivery insufficient without necessarily populating missing or failing groups.

## HCL-C-30 (high)

**Claim.** HCL-C-30 (HCL-WORD-04 and HCL-TRANSACTION-06): supersede can consume a word after its code commit has already landed but before the carried ledger row exists. Its mutation checks only for an existing carried row, while the design's full consumption rule also includes an origin commit with the exact Carry trailer. A subsequent recovery then encounters a superseded row for an already-landed commit and cannot write the landed obligation. What changes if this stands: supersede must recheck the complete, freshly anchored consumption predicate inside its mutation and fixture the push-before-row race.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:330-347 defines supersede's unconsumed precondition only as absence of a carried row. metasystem/plans/human-carried-landing-carry-design.md:797-813 defines consumption as either that row or an origin Carry trailer, and lines 835-843 explicitly recognize the crash interval where the trailer exists before the row.

## HCL-C-26 (high)

**Claim.** HCL-C-26 (HCL-TRANSACTION-06): a crash after the goal transaction becomes visible but before the counselor append still has no specified executable repair. Recovery sees the transaction trailer and takes its confirm path, which at HEAD invokes only the split-specific confirmed effect rather than reconstructing the carried request and its AfterConfirmed hook. The design's ledger branch says to run goal carried --entry with a fresh operation identifier, but --entry requires an existing created entry and no such fresh-entry command is defined. What changes if this stands: confirmed carried recovery must explicitly replay the counselor hook, and the ledger branch must name a valid command and payload source; add the exact crash fixture between ledger publication and AfterConfirmed.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:776-790 puts the counselor append in AfterConfirmed. Lines 844-846 prescribe the undefined fresh-opid --entry replay. metasystem/internal/goal/recover.go:59-100 sends an already-present transaction to recoverSplitConfirmedEffect, while metasystem/internal/goal/recover.go:416-423 makes that effect a no-op for every verb except split.

## HCL-C-25 (high)

**Claim.** HCL-C-25 (HCL-TRANSACTION-06): replay is not a full-payload comparison. The durable intent includes missing groups, failing groups, ledger tip, and richer judge evidence, but the comparison omits those values; the carried row also does not preserve missing and failing groups. A replay can therefore be accepted and use conflicting evidence to repair the counselor line. What changes if this stands: every immutable input needed by the row, obligation, and counselor line must be stored or deterministically re-derived and compared, with a negative fixture for each field.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:753-766 lists missing, failing, judge, and ledger in the intent. Lines 856-866 compare commit, workspace, project tree, past, battery, judge mode, outcome, and by only. Lines 1086-1105 show ledger on the row and missing and failing on the counselor line, proving that the omitted values affect the recorded payload.

## HCL-C-03 (high)

**Claim.** HCL-C-03 (HCL-LANDING-05): the base judge still can approve a candidate that changes policy the base cannot see because the blindness fence is an incomplete exact-file list. It omits receipt.go, which owns the TestReceipt wire schema at HEAD, and omits the newly designed carried.go policy owner and metasystem.conf budget law. What changes if this stands: use a conservative owner or directory fence, or enumerate every policy and wire owner including new files and configuration, with a table fixture that changes each protected owner.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:513-531 lists exact protected files but includes only testing.go and registers.go for receipts. metasystem/internal/landing/receipt.go:33-50 defines the actual TestReceipt schema. The design itself names carried.go as the new classification owner at metasystem/plans/human-carried-landing-carry-design.md:465-471 and the committed cap policy in metasystem.conf at lines 1128-1140; neither is fenced.

## HCL-C-34 (high)

**Claim.** HCL-C-34 (contract ownership and fixtures): the testing contract additions own the newly changed paths but still do not select many declared future Go tests. The proposed authority group runs all tests only in four internal packages; existing goal, landing, and command groups run fixed old test lists, while revision 4 assigns new fixtures to those packages. A green selected plan can therefore omit the new unit fixtures. What changes if this stands: extend the owning groups to execute the named new tests or all tests in those packages, and make the selection fixture assert execution of every new fixture group rather than path ownership alone.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:1384-1389 limits authority-standard to four internal packages, while lines 1428-1436 assign new fixtures to internal/goal, internal/landing, and cmd/metasystem too. metasystem/testing.json:27,31-32,38 shows that the selected command, landing, and goal groups execute explicit existing test names rather than all tests. HCL-31-CONTRACT-OWNS-FIVE-PACKAGES at design lines 1411-1417 asserts ownership and group inclusion, not execution of the new tests.

## HCL-C-07 (medium)

**Claim.** HCL-C-07 (HCL-OBLIGATION-07): accepted-risk replay treats a changed human rationale as idempotent. The acceptance reason is a durable schema field sourced from --why, but the replay rule compares only --by, so a second command with a different reason exits successfully while the append-only record retains the old reason. What changes if this stands: compare the acceptance reason and every other immutable accepted-risk input on replay, refusing and naming any mismatch; extend the replay fixture with a changed-why case.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:1026-1037 makes acceptanceReason a stored field sourced verbatim from --why. Lines 1039-1045 define replay success solely by the same --by. The existing behavior being retained has the same gap at metasystem/internal/goal/verbs.go:1061-1071, where the prior record is accepted before the history reason is compared.

## HCL-C-32 (medium)

**Claim.** HCL-C-32 (Fixtures of 05): the rebase-conflict oracle contradicts the specified algorithm. The algorithm says rebase --abort restores HEAD to the temporary wip commit; the fixture requires HEAD at that commit's parent with the candidate staged. Those are different repository states and lead to different retry behavior. What changes if this stands: choose one exact recoverable checkout posture, specify any required soft reset, and make the algorithm and fixture assert the same HEAD, index, and working-tree state.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:447-453 says abort restores HEAD to the wip commit. HCL-05-REBASE-ASKS at lines 679-681 says HEAD is the wip commit's parent and the index still holds the candidate.

## Gaps the critic named

- No test, fixture bed, build, or live proof was run because the review brief explicitly prohibited beds and requested read-only design critique.
- The runtime is read-only, so metasystem/records/misc/human-carried-landing-carry-critique-r2.md was not created; this return is the register for coordinator projection.
- The provider tool catalog remains unobserved, so this review is advisory about runtime isolation.
- The carried implementation does not exist at HEAD; conclusions about its new control flow are design-level inferences grounded in the existing transaction, recovery, receipt, and testing-contract code.

## Coordinator dispositions (m1d, 2026-09-11, binding on fold 2 = revision 5)

All nine accepted; none refuted. Narrowings:
- HCL-C-33 (critical): a fleet-visible reservation before the code push: the `carrying` intent is published to the ledger (a `carrying` history row on the goal, with the word's opid, the workspace tree and the seat) BEFORE the push, and the debt check counts an open `carrying` row as debt; the two-seat interleaving fixture.
- HCL-C-21 (critical): a code word hits only when the testing result is sufficient (`Delivery.Sufficient`, not merely M empty) and O.Code equals the word; a group word hits only when the ONLY insufficiency is that group (missing/failing = {G}, no uncovered obligation, no discrepancy); `unverified` never matches a code word; fixtures for the verify error, an uncovered obligation and a discrepancy.
- HCL-C-30: supersede rechecks the complete consumption predicate (row, trailer on origin since the anchor, superseding row) inside the mutation; the push-before-row race fixture.
- HCL-C-26: confirmed-carried recovery replays the counselor hook explicitly (a named path in recover.go, not the split-only effect); the ledger branch names a valid command with its payload source; the crash fixture between ledger publication and the hook.
- HCL-C-25: replay compares every field the row, the obligation and the counselor line carry (missing, failing, ledger tip, judge evidence included) with a negative fixture per field; the row stores missing and failing.
- HCL-C-03: the blindness fence is by owner, not by exact file: every file under internal/landing, internal/goal, internal/proofrun, internal/testpolicy, internal/behaviorsurface, internal/config, plus metasystem.conf and testing.json; the table fixture changes each.
- HCL-C-34: the owning groups run every test of the packages the fixtures land in (or the named new tests), and the selection fixture asserts execution, not ownership.
- HCL-C-07: accepted-risk replay compares `--why` and every immutable input; the changed-why fixture.
- HCL-C-32: one recoverable posture after a rebase conflict (HEAD at the wip commit's parent with the candidate staged, reached by `rebase --abort` then `reset --soft`), stated once and asserted by the fixture.
