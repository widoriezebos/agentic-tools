# Human Approval for Execution — closing review (tier-3 last review)

Critic: codex gpt-5.6-sol (hae-closing-review) over design revision 2
(d69478bb). 4 material findings. Per R-54/R-60-m1 the review cycle ENDS
here - these fold into the BUILD as required corrections (all agreed,
artifact-named), verified by the one code review; no further design
review round.

## HAE-CLOSE-01-SWEEP-INTENT-BINDING [critical]

The grandfather sweep can approve an intent the human never saw. Section 9 must change the sweep listing and confirmation-hash contract implemented in metasystem/internal/goal/verbs.go, and TestSweepRefusesOnDigestDrift must cover intent drift between preview and confirmation.

Evidence: Metasystem/plans/human-approval-for-execution-design.md:24-25 says approval is bound to the exact intent the human approved, and lines 42-43 say the sweep is bound to the listing the human saw. However, section 9 at metasystem/plans/human-approval-for-execution-design.md:628-635 neither displays intent nor includes it in the digest; the digest contains only identifier, state, budget, and authority. A queued goal has no Approved record, so the preserved edit behavior at metasystem/internal/goal/verbs.go:1113-1115 can change its intent after the preview without changing that digest. Confirmation then writes an Approved digest for the new intent. An implementer must therefore change the sweep payload and its named drift test, making this material.

DISPOSITION: BUILD-INPUT (fold in the implementation; the code review verifies).

## HAE-CLOSE-02-STEAL-EXPIRED-APPROVAL [critical]

The claimed-revision gate still allows steal to create a fresh execution revision from an expired relayed approval. Metasystem/internal/goal/verbs.go must either apply approval expiry to steal or require fresh proof that renews approval; metasystem/cmd/metasystem/goalsync_mutations.go and the TestNoClaimWithoutApproval assertion change with that choice.

Evidence: The summary at metasystem/plans/human-approval-for-execution-design.md:31-33 says every claimed revision requires an unexpired Approved record or a proof-bearing act. The steal row at lines 302 and 341 expressly ignores expiry, section 5 at lines 499-501 calls this continuation, and the folded test obligation at line 906 requires success on an expired record. Live steal at metasystem/internal/goal/verbs.go:1184-1188 requires only a --by name, which metasystem/cmd/metasystem/goalsync_mutations.go:53-58 copies without proof; metasystem/internal/goal/verbs.go:1253-1258 then binds a fresh claim revision. Metasystem/internal/dispatch/admission.go:78-103 admits new reservations from that fresh claimed revision. This is new execution, not merely completion of already-running jobs, so the original critical claimed-writer hole was renamed rather than closed.

DISPOSITION: BUILD-INPUT (fold in the implementation; the code review verifies).

## HAE-CLOSE-03-FLEET-ENROLLMENT-CUTOFF [high]

The first-enrollment cutoff is implemented as a machine-local fact, so enrollment on one fleet machine leaves relayed approvals usable on the others. The build needs a fleet-visible cutoff artifact—such as a synced field owned by metasystem/internal/goal/root.go—and must wire it into metasystem/internal/humanauthority/authority.go and the projection and command callers; TestRelayRefusedAfterEnrollment must exercise two machines.

Evidence: Ruling R-29-m1 at metasystem/memory/rulings.md:58 and R-32-m1 at line 78 terminate the relay at the first enrolled session. The design instead repeatedly says this machine's enrollment at metasystem/plans/human-approval-for-execution-design.md:34-35,487-498 and 679-683. Its chosen source, metasystem/internal/humanauthority/authority.go:375-399, is artifacts/agents/authority/human-terminal.json, an unsynced machine-local file. In the four-machine fleet, Wido's first enrollment therefore does not terminate grants or existing relayed approvals elsewhere. The original high-severity enrollment-expiry finding remains open at fleet scope.

DISPOSITION: BUILD-INPUT (fold in the implementation; the code review verifies).

## HAE-CLOSE-04-ALL-PARKED-ARC-TRANSITION [high]

The supposedly complete transition table omits the new approved-to-all-parked-arc outcome. Metasystem/internal/goal/verbs.go and metasystem/internal/goal/reconcilepub.go still require a decision on whether an approved goal joining an all-parked destination becomes parked, stays approved, or refuses, and TestTransitionTableIsClosed lacks the corresponding normal and reconcile cases.

Evidence: Section 2 at metasystem/plans/human-approval-for-execution-design.md:277-283 and 316-320 claims exhaustive transition closure. Its approved set-arc rows cover caller-owned claimed destinations at line 292 and mixed or empty destinations at line 296, but no all-parked destination. The existing normal matrix has a distinct all-parked branch that writes StateParked at metasystem/internal/goal/verbs.go:2026-2032, and reconcile duplicates it at metasystem/internal/goal/reconcilepub.go:484-487. Adding StateApproved to these source switches without a specified row leaves a genuine implementation choice about state, Parked record, Approved record, and budget preservation. This is the same transition-closure class as the original high-severity finding, not a prose-only omission.

DISPOSITION: BUILD-INPUT (fold in the implementation; the code review verifies).
