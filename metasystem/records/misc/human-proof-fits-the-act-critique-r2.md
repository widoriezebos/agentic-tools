# human-proof-fits-the-act, design critique round two (register)

Critic human-proof-crit2-20260909 (design-critic, codex gpt-5.6-sol, xhigh, read-only) on revision 2 of plans/human-proof-fits-the-act-design.md (commit 7b6ac5e7e), revised on the Fable lane the same hour. Seven material findings, one critical; built by the orchestrator from the critic's returned JSON, verbatim in claim and evidence, because the critic's runtime was read-only.

## HPA-10 (critical, material)

Claim: HPA-10 — Critical, material — Sections “What remains impossible,” 7 and 8 in metasystem/plans/human-proof-fits-the-act-design.md preserve and expand a command-line fixture authority that an agent can mint by editing metasystem.conf to set metasystem.runtimes=fake. That worktree value alone lets --fixture-human-authority produce a valid enrolled-grade proof; no fake identity table or separately protected capability is required. The proposed expansion registers the flag on steal, resume, set-pin, reconcile, migrate and repair, so an agent can cross the exact authority boundary the design claims is impossible. Amend the design to replace the production command-line fixture grant with a test-only injected prover or another capability an agent cannot create by editing repository bytes, and add a negative canary for a real ledger whose worktree configuration was changed to fake.

Evidence: metasystem/internal/fixtureauth/fixtureauth.go:63 and 291-292 derive fixture mode solely from the worktree configuration; lines 268-273 make that enough for GoalHumanAuthorityProbe.Allows. metasystem/cmd/metasystem/goalsync_mutations.go:583-604 turns the probe into a proof, and metasystem/internal/humanauthority/authority.go:99-105 and 126-145 accepts that observed fixture proof as valid. Revision 2 section 7, lines 800-806, expands the exposed flag to additional authority-widening verbs.

## HPA-11 (high, material)

Claim: HPA-11 — High, material — The recovery contract in “The two proofs,” the matrix and section 7 of metasystem/plans/human-proof-fits-the-act-design.md upgrades every pre-change journal entry with by but no authorityGrade to terminal-grade authority. That is unsafe because the design itself proves that the untouched builder allowed an agent with an exported lineage to create exactly such human-attributed intents without any proof. A stranded pre-upgrade foreign release, park, unpark or similar entry can therefore be completed by post-upgrade goal recover without either walk. Amend the design so a missing stamp grants no grade and terminalizes toward a fresh human invocation; only an explicitly human-ratified migration record may grandfather an old entry.

Evidence: Revision 2 lines 173-184 say missing authorityGrade is read as terminal. metasystem/internal/goal/verbs.go:409-427 currently stores by without proof metadata. metasystem/internal/goal/recover.go:137-207 takes over a dead owner's journal and runs its stored intent without a fresh authority check. metasystem/cmd/metasystem/goalsync_mutations.go:71-117 currently skips Prove when METASYSTEM_OWNER_LINEAGE or --lineage is present, so an agent can be the source of an unstamped by entry.

## HPA-12 (high, material)

Claim: HPA-12 — High, material — Section 1 of metasystem/plans/human-proof-fits-the-act-design.md misgrades mission resolve-taint --restore as terminal because it discards disputed work. The code shows that restore, like adopt-disputed-tree, starts a new sequence segment and changes the mission from parked to running when it resolves the last taint. Under the design's own reopening tie-break, restore requires the enrolled check after the classifier. Amend the matrix, refusal row and TestArmAndAdoptNeedTheEnrolledTerminal obligation to cover both resolution variants.

Evidence: metasystem/internal/missionrunner/resolve.go:3-11 states that both typed resolutions reopen the workspace and begin a new segment. Lines 294-305 advance the segment and decide whether the mission unparks; lines 445-452 set status to running and clear the wall-violation park for either variant when no unresolved taint remains. The design's matrix at lines 305-306 assigns terminal only to restore and enrolled only to adoption.

## HPA-13 (high, material)

Claim: HPA-13 — High, material — Sections 1, 4 and 6 of metasystem/plans/human-proof-fits-the-act-design.md neither inventory nor encode the shipped channel-authority contract consistently. Set-obligation currently accepts an authenticated-channel word through --approved-ref, resume accepts that same proof class, while approve and norm exceptions use the distinct verified-channel-answer class. The design omits set-obligation's channel alternative, calls resume's alternative verified, and gives recovery only enrolled or terminal authorityGrade values. Implementers could delete a supported channel path, accept the wrong outcome, or coerce channel authority into an unrelated grade. Amend the matrix with every existing channel consumer and specify whether channel-authorized journals preserve their exact authority class or remain deliberately non-replayable; do not default them to either terminal or enrolled.

Evidence: metasystem/cmd/metasystem/goalsync_mutations.go:1285 and 1311-1319 implement set-obligation --approved-ref through AuthenticatedChannelProof; lines 1041-1048 do the same for resume. metasystem/internal/humanauthority/authority.go:152-175 accepts the authenticated-channel outcome for set-obligation and resume. metasystem/internal/goal/approval.go:386-390 and internal/goal/norm.go:112-116 separately recognize OutcomeVerifiedChannel. Revision 2 lines 268-269, 626-630 and 843-846 claim unchanged channel behavior without representing these distinctions.

## HPA-14 (medium, material)

Claim: HPA-14 — Medium, material — “The CLI builder proves on every --by” and section 3 of metasystem/plans/human-proof-fits-the-act-design.md require the command layer to retain the failed enrolled proof so a later GradeRefused can render the exact no-enrollment or wrong-terminal recovery row, but the proposed interfaces provide nowhere to retain it. VerbRequest carries only the successful terminal proof, while GradeRefused carries only verb, row, needed grade and received grade. Implementers must guess between adding delivery-layer state to the request, re-running Prove, using global state, or degrading the refusal. Amend the design with one explicit ephemeral carrier; preferably have the CLI builder return the engine request plus the structured enrolled attempt, and pass that attempt to the refusal renderer without placing rendering data in the goal engine.

Evidence: Revision 2 lines 137-153 define VerbRequest.Authority and GradeRefused without an enrolled-attempt field. Lines 155-164 say the builder keeps the failed full-walk refusal after replacing it with a terminal proof, and lines 443-448 require the later renderer to use that refusal. Current callers retain only the VerbRequest and pass only the engine error to printSyncResult, as shown throughout metasystem/cmd/metasystem/goalsync_mutations.go.

## HPA-15 (medium, material)

Claim: HPA-15 — Medium, material — Sections 2 and 3 of metasystem/plans/human-proof-fits-the-act-design.md promise that goal enroll-terminal replaces an unreadable or incomplete enrollment, but the current write path refuses that case and the revision does not decide what generation the replacement receives. Resetting to generation 1 can recreate an old terminal-derived lineage, while preserving monotonicity is impossible from unreadable bytes without another source. Amend the design either to introduce an independently durable monotonic enrollment identifier or, more conservatively, make this refusal say that no command can safely complete it until the corrupt record is repaired; do not print an enroll command that may fail or silently reuse an authority identity.

Evidence: metasystem/internal/humanauthority/authority.go:514-531 returns errors for malformed and incomplete records. Enroll at lines 586-590 starts at generation 1 only for a missing record, increments a readable record and refuses every other read error. Revision 2 lines 109-111 and 453-454 classify unreadable records as not enrolled and promise that re-enrolling replaces them, without a generation rule.

## HPA-16 (medium, material)

Claim: HPA-16 — Medium, material — Section 1's claimed complete inventory in metasystem/plans/human-proof-fits-the-act-design.md omits goal reopen's human-only all-parked-arc branch. Because the implementation instructions replace only the Actor.Human tests named by the inventory, an implementer can leave this branch accepting a human name without a terminal proof, including during recovery. Add a terminal-grade reopen row and include it in the Actor.Human-without-Authority engine fixture.

Evidence: metasystem/internal/goal/verbs.go:1330-1400 shows that ordinary reopen returns queued and needs no human, but reopening an archived member into an all-parked arc checks only r.Actor.Human at line 1392 and copies the standing park. metasystem/internal/goal/recover.go:344-345 rebuilds reopen through that same closure. No reopen row appears in the revision 2 matrix.

## HPA-17 (low, note)

Claim: HPA-17 — Low, not material — Section 1's positive inventory in metasystem/plans/human-proof-fits-the-act-design.md also omits the goal set-next command as a separate command entry point. It can reach Edit's foreign-claim or parked-goal human branches, but the shared request builder and the same Edit engine helper already cover it, so an implementer would build the same behavior. No artifact change is required under the binding materiality test.

Evidence: metasystem/cmd/metasystem/goalsync_mutations.go:504-509 implements set-next by calling goal.Edit, whose conditional human checks are at metasystem/internal/goal/verbs.go:1545-1549. The matrix's goal edit rows already determine those branches' terminal grade.

## Gaps the critic named

- No test, bed, scratch-ledger mutation or live human verb was run, as required by the brief; all behavioral conclusions are from code reading.
- The requested metasystem/records/misc/human-proof-fits-the-act-critique-r2.md file was not created because the binding critic role forbids edits and this runtime has read-only filesystem authority. This returned JSON is the register for the orchestrator to persist.
- The runtime exposed no independent session identifier, so sessionId is recorded as unobserved and no different model or session is claimed.
