# Human Approval for Execution — design review (the one review), with dispositions

Critic: codex gpt-5.6-sol (hae-review) over design 071446e1. 8 material
findings, each naming its artifact (R-60-m1). This is the tier-3 single
review; ALL fold into the one revision, then the closing review.

## HAE-R1-CLAIM-WRITERS [critical]

The design's claim-refusal boundary is not exhaustive. Design sections 2, 3, and 12 say every claimed revision passes the approved-state check and enumerate claim, arc claim, retired open-claim, reopen, steal, resume, and replay. They omit normal set-arc, reconciled set-arc, and set-budget claim rebinding. An agent can currently join a queued member to its own claimed arc through metasystem/internal/goal/verbs.go:2033-2064; the hand-edit path repeats the write at metasystem/internal/goal/reconcilepub.go:488-516; and set-budget creates a fresh claimed revision at metasystem/internal/goal/verbs.go:535-540. Their recovery paths are live at metasystem/internal/goal/recover.go:259-264 and 325-328. The proposed at-rest check in design section 3 cannot repair this as written: design section 2 simultaneously tolerates every pre-gate claim without an Approved record, section 10 rejects a format marker, and the current tree contains four such claims. There is no specified fact by which metasystem/internal/goal/validate.go can distinguish a tolerated old claim from a new set-arc bypass before the sweep. The fold must change the claim writers and their tests in metasystem/internal/goal/verbs.go, metasystem/internal/goal/reconcilepub.go, metasystem/internal/goal/recover.go, metasystem/internal/goal/validate.go, and metasystem/internal/goal/verbs_test.go, and must define a persistent cutover discriminator if the at-rest defense remains.

Evidence: Read: metasystem/records/human-approval/hae-design.md:149-199, 499-501, and 419-448. Ran the claimed-state writer search and read the concrete writes at metasystem/internal/goal/verbs.go:2033-2064, metasystem/internal/goal/reconcilepub.go:488-516, metasystem/internal/goal/verbs.go:535-540, and their replay routing at metasystem/internal/goal/recover.go:259-264 and 325-328.

DISPOSITION: FOLD into revision 2.

## HAE-R1-APPROVED-PAYLOAD-MUTATION [critical]

Approval does not remain bound to the human-approved intent and budget. Design section 4 says only an agent's intent edit on an approved goal refuses, permits a human set-budget operation on approved work, and deliberately preserves agent set-budget on the claimant's own claimed goal. Those rules have no strong proof parameter. The shipped request builder copies arbitrary --by text into Actor.Human at metasystem/cmd/metasystem/goalsync_mutations.go:53-69; edit trusts that string at metasystem/internal/goal/verbs.go:1089-1115; set-budget trusts it and also lets the owning agent rebind its budget at metasystem/internal/goal/verbs.go:511-540; and reconcile requires only a nonempty name at metasystem/internal/goal/reconcilepub.go:27-32. Following the design would let an agent pass --by Wido to alter an approved intent or within-norm budget, or claim first and then enlarge its own budget, while the Approved record remains. The fold must choose and enforce a strong-authority or unconditional-unapprove route for these approval-bearing mutations in metasystem/internal/goal/verbs.go, metasystem/internal/goal/reconcilemap.go, metasystem/internal/goal/reconcilepub.go, metasystem/cmd/metasystem/goalsync_mutations.go, and their focused tests.

Evidence: Read: metasystem/records/human-approval/hae-design.md:224-241 and 266-272. Read the weak name propagation at metasystem/cmd/metasystem/goalsync_mutations.go:53-69 and the mutation gates at metasystem/internal/goal/verbs.go:511-540, 1089-1115, and metasystem/internal/goal/reconcilepub.go:27-32.

DISPOSITION: FOLD into revision 2.

## HAE-R1-RELAY-ENROLLMENT-EXPIRY [high]

The relayed-word expiry omits the binding first-enrollment cutoff. Design section 5 enforces only the review date and says the relay lasts until 2026-09-06 unless the ruling is renewed. Rulings R-29-m1 and R-32-m1 instead end the authority at 2026-09-06 or the first enrolled session, whichever occurs first. The reused helper at metasystem/internal/humanauthority/authority.go:159-192 validates dates but does not read enrollment, and metasystem/internal/humanauthority/authority.go:228-236 falls back to a relay after any ancestry failure. Consequently an agent-chain caller could still relay an approval after Wido enrolls. The fold must add the enrollment cutoff to metasystem/internal/humanauthority/authority.go and cover both approve and unapprove in metasystem/internal/humanauthority/authority_test.go.

Evidence: Read: metasystem/records/human-approval/hae-design.md:287-329 and metasystem/memory/rulings.md:58,78. Read the current temporary validation and fallback at metasystem/internal/humanauthority/authority.go:159-237.

DISPOSITION: FOLD into revision 2.

## HAE-R1-TRANSITION-CLOSURE [high]

The transition table is not complete enough to implement the Approved-record invariant. Design section 2 requires queued goals to have no Approved record while allowing that record on claimed, parked, and done goals, but says unlisted rows keep today's rules. Today's release, arc release, detach, and set-arc fallback paths move claimed work to queued at metasystem/internal/goal/verbs.go:663, 1651, 1850, and 2067 without clearing a future Approved record. The design does not decide whether these operations preserve approval by landing in approved or revoke approval and clear the record and budget. Reconcile duplicates these transitions at metasystem/internal/goal/reconcilepub.go:410-525, its hand-state grammar lacks approved at metasystem/internal/goal/reconcilemap.go:250-270, and an approved split parent currently reaches the rejecting default at metasystem/internal/goal/split.go:313-329. The fold must specify every outcome and add transition tests in those four artifacts; otherwise ordinary release or arc maintenance will either publish an invalid queued-plus-Approved file or silently withdraw human approval.

Evidence: Read: metasystem/records/human-approval/hae-design.md:133-169. Read the live transition writes at metasystem/internal/goal/verbs.go:628-672, 1620-1665, 1808-2075; the hand-edit transitions at metasystem/internal/goal/reconcilemap.go:250-270 and metasystem/internal/goal/reconcilepub.go:410-525; and the split state switch at metasystem/internal/goal/split.go:313-329.

DISPOSITION: FOLD into revision 2.

## HAE-R1-REAPPROVAL-TRANSITION [medium]

Expired relayed approvals have no specified reapproval transition. Design section 5 leaves the goal in StateApproved when its review date passes, refuses its claim, and says reapproval is a fresh approve; section 9 likewise says a relayed sweep's goals can be reapproved at the terminal. The complete table in section 2 permits only queued to approved, not approved to approved, and the proof plan tests expiry but not renewal. Because state predicates are explicit in the engine, as shown by metasystem/internal/goal/verbs.go:443-453, the implementer must guess whether approve replaces an expired Approved record in place, requires unapprove first, or refuses. The fold must decide the behavior in the new approve request in metasystem/internal/goal/verbs.go and add the renewal case to metasystem/internal/goal/verbs_test.go.

Evidence: Read: metasystem/records/human-approval/hae-design.md:149-162, 306-316, 411-415, and 486-489. Read the existing explicit state-gate pattern at metasystem/internal/goal/verbs.go:443-453.

DISPOSITION: FOLD into revision 2.

## HAE-R1-APPROVAL-RECORD-BINDING [high]

The Approved record is not specified as evidence bound to an actual approval event. Design section 2 calls its revision and timestamp the approval operation's coordinates, and section 4 relies on their history order to show what was approved, but the listed file invariants check only record presence, budget presence, authority-dependent reviewBy shape, and state. They do not require a positive revision at or below the file revision, a matching approve history event, matching timestamp and actor, a human-prefixed by value, or a closed authority value. The generic parser at metasystem/internal/goal/file.go:598-628 closes keys but not values, while metasystem/internal/goal/file.go:374-394 shows the explicit validation needed for another revision-bearing record. Because design section 3's at-rest defense trusts Approved presence, a malformed record could satisfy the gate. The fold must define these invariants and refusal fixtures in metasystem/internal/goal/file.go, metasystem/internal/goal/validate.go, metasystem/internal/goal/file_test.go, and metasystem/internal/goal/validate_test.go.

Evidence: Read: metasystem/records/human-approval/hae-design.md:118-140, 194-199, and 266-272. Read the closed-key parser and current revision-record validation pattern at metasystem/internal/goal/file.go:374-394 and 598-628.

DISPOSITION: FOLD into revision 2.

## HAE-R1-FRONTIER-CLOCK [medium]

The frontier has no specified owner for approval-expiry time. Design sections 5 and 8 require claim and Next to classify a relayed approval by a UTC review date, but current Projection stores no observation instant at metasystem/internal/goal/project.go:26-31 and Next accepts no clock at metasystem/internal/goal/project.go:489-523. The existing callers pass time to Project and then lose it, including metasystem/internal/goal/project.go:297-304 and metasystem/cmd/metasystem/goal.go:459-469. An implementer must choose between adding time to Projection, changing Next's interface, or reading the wall clock inside Next; those choices produce different midnight behavior and test determinism. The fold must choose the clock owner and whether reviewBy remains valid for its entire UTC date, then name the assertions in metasystem/internal/goal/project_test.go and the affected caller tests.

Evidence: Read: metasystem/records/human-approval/hae-design.md:306-310 and 359-378. Read Projection and Next at metasystem/internal/goal/project.go:26-31 and 481-523, plus the command caller at metasystem/cmd/metasystem/goal.go:459-469.

DISPOSITION: FOLD into revision 2.

## HAE-R1-STATE-CONSUMERS [medium]

The state-consumer inventory and build bound omit code that will drop approved work from operational views. Steward ledger attention builds its queue by accepting only StateQueued at metasystem/internal/steward/ledgerattention.go:147-173; therefore a blocked or expired approved goal will be neither ready nor queued unless that package consumes the proposed Awaiting bucket. Debt metrics likewise age only queued or parked goals at metasystem/internal/metrics/compute.go:624-636, so approved backlog disappears from debt-age values. Design section 13 excludes both packages from the implementer surface even though section 8 promises changed steward behavior. The fold must add metasystem/internal/steward/ledgerattention.go and metasystem/internal/metrics/compute.go to the build surface and decide the approved and expired-approved classifications in their focused tests.

Evidence: Read: metasystem/records/human-approval/hae-design.md:91-97, 359-378, and 509-518. Read the omitted state consumers at metasystem/internal/steward/ledgerattention.go:141-173 and metasystem/internal/metrics/compute.go:603-650.

DISPOSITION: FOLD into revision 2.
