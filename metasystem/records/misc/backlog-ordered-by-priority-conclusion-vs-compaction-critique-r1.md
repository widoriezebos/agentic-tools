# conclusion-vs-compaction design critique — round 1 (revision 1)

Chain: revision 1 (landed 8ca43596, sha256 4613d232c47fc70878b19432841fb57d9df13ce80305b9a7c722047888ac35c5) -> critic bcc-crit1 (design-critic, codex gpt-5.6-sol, xhigh, read-only; the harness observed gpt-5.6-sol and the return claimed nothing different). Reviewed at worktree commit 4fb0cf5c. 3 findings, 1 material. The coordinator carried the return here verbatim because the critic's sandbox is read-only and it wrote no register file.

## BCC-01 — medium, material=True

CLAIM: BCC-01: The State-only discriminator does not settle conclusion versus compaction. A live goal can first survive a neighbour’s conclusion and receive that operation’s done event, then later be archived by goal split. Its final State is done, but its last done verb still belongs to the neighbour. Both the proposed concludedAt helper and the separately guarded concludingEpoch scan would therefore report the neighbour’s timestamp as this split parent’s conclusion. The design needs a discriminator and canary covering this reachable terminal path before implementation.

EVIDENCE: metasystem/internal/goal/verbs.go lines 1162-1175 append done events to shifted live survivors. metasystem/internal/goal/split.go lines 288-299 later set a live parent to StateDone, append split rather than done, and move it to the archive. The planned State check in metasystem/plans/backlog-ordered-by-priority-conclusion-vs-compaction-design.md section 1.4 cannot distinguish that history.

## BCC-02 — low, material=False

CLAIM: BCC-02: The refusal-cause census overstates APPROVAL_REQUIRED as a production frontier category. An approved goal lacking a budget, or carrying an invalid approval record, is rejected while parsing the accepted tree before Next can call the admission gate. With file.go explicitly unchanged, only synthetic in-package projections can reach that branch. This is not material because the typed refusal branch and unchanged parser leave the implementation unambiguous, but the statement must not be used as a production guarantee or fixture premise.

EVIDENCE: metasystem/internal/goal/file.go lines 495-504 validate approval records during parsing, and ValidateApprovalRecord at lines 618-630 refuses a missing budget or mismatched digest. metasystem/internal/goal/verbs.go lines 235-244 makes loadTree fail on those parse problems before metasystem/internal/goal/project.go can compute the frontier.

## BCC-03 — low, material=False

CLAIM: BCC-03: The page does not literally route all four metrics readers through one helper. goalBounds, selectedGoals, and ConcludedInWindow use concludedAt, but concludingEpoch retains its own backward done scan with a duplicated State check. This does not alter the specified behavior and therefore is not material, but the claimed single-reader ownership is inaccurate.

EVIDENCE: Section 1.4 step 2 explicitly preserves concludingEpoch’s scan after adding an early State check, while steps 3 and 4 route the other consumers through concludedAt.

## Gaps the critic named

- The requested register file metasystem/records/misc/backlog-ordered-by-priority-conclusion-vs-compaction-critique-r1.md was not created because this critic’s filesystem is read-only and the role forbids repository edits. This JSON is the complete register content for the orchestrator to persist.
- No fixture bed was run, as required by the brief. All behavior and fail-before conclusions above are source-derived and are not runtime proof.

## What the critic verified that the page rests on

- git rev-parse HEAD && git status --short: The synchronized worktree is at commit 4fb0cf5c9f2588e408a7b1468ac052f3f1394a03, with no reported worktree changes.
- shasum -a 256 metasystem/plans/backlog-ordered-by-priority-conclusion-vs-compaction-design.md: The reviewed page’s SHA-256 digest is 4613d232c47fc70878b19432841fb57d9df13ce80305b9a7c722047888ac35c5, matching the brief.
- Read metasystem/internal/goal/verbs.go lines 1103-1178 and metasystem/internal/goal/split.go lines 224-346: A ranked survivor receives a done-verb event when a neighbour concludes. That survivor can later become a split parent; split sets its State to done, appends a split event, and archives it without appending a done event for its own conclusion.
- Read metasystem/internal/goal/reconcilepub.go lines 91-156 and 327-358, plus metasystem/internal/goal/order.go lines 188: Reconcile compaction operates on the final live map. Each reconcile-departed record first receives its own done event, while shifted survivors receive or merge the compaction event. No reconcile path was found that adds a neighbour’s done event directly to an already archived record.
- rg -n --glob '*.go' for done-verb comparisons and historyTime calls outside test files, then read metasystem/internal/me: The non-test census found the four metrics uses identified by the page: goalBounds, selectedGoals, concludingEpoch, and ConcludedInWindow. No other non-test package interprets a done history verb as a per-goal conclusion.
- Read metasystem/internal/counselor/sources.go lines 501-593 and metasystem/internal/counselor/compute.go lines 113-137: The counselor groups history by operation identifier. When another carrier has the same timestamp and verb, it skips the duplicate; differing facts mark the operation conflicted and exclude it. Done compaction carrier lines therefore do not double-count the operation.
- Read metasystem/internal/goal/project.go lines 493-581, metasystem/internal/goal/approval.go lines 261-364, and metasyst: The typed goal-admission wrapper makes the frontier partition stable: GOAL_NORM_REFUSED is retained, configuration and tier-box errors fail the entire frontier, and APPROVAL_EXPIRED is classified as Awaiting before the gate. However, production parsing rejects missing, budgetless, or invalid approval records before Next, so APPROVAL_REQUIRED cannot normally populate Refused.
- Read metasystem/cmd/metasystem/goal.go lines 462-522, metasystem/internal/channel/report.go lines 137-185, and metasyste: The proposed command, channel, and turn-verdict strings are producible at the named sites. Refused work is absent from Claimable; with no other claimable work, idle enforcement resets its refusal count and returns without blocking. The unchanged goal-list path performs no admission traversal.
- Read metasystem/internal/metrics/compute.go lines 511-600 and the proposed timestamps in metasystem/plans/backlog-ordere: Before repair, claim at T2, landing twelve hours later, and the neighbour’s done event at T3 form a twenty-four-hour lifecycle. The formatter therefore produces exactly c building_hours=12.000 proving_hours=12.000 waiting_share=0.500 epochs=1.
- Read metasystem/internal/goal/order_test.go lines 133-157 and 445-479, metasystem/cmd/metasystem/goal_priority_test.go l: The over-norm, command, and channel fixture builders can produce the specified records after their approval digests are recomputed. Before the surface repair, the command emits no matching eligible work and the channel omits the skipped clause, so those two canaries have independent fail-before behavior.

## Coordinator disposition (m1b, 2026-09-09)

All three are accepted; BCC-01 is the only one that changes what gets built,
and it does, so the design loop's stop criterion is not met by revision 1.
This is fold-read cycle 1.

- BCC-01 (split parent): confirmed in the code by the coordinator before
  accepting. split.go:288-299 sets the parent to State done, appends the verb
  `split`, and archives it with no `done` line; the same transaction runs
  compactDepartedPriorities over t.Live and merges the compaction onto
  survivors with the verb `split` (split.go:301-316). So compaction lines
  carry the departing act's verb, `done` or `split`, and archive acts carry
  `done` (verbs.go:1160 via doneRequest, reconcilepub.go:352) or `split`
  (split.go:297). The page's rule "under State done the last done verb is the
  goal's own conclusion" is therefore false for a ranked survivor that later
  becomes a split parent, exactly as the critic says, and the page's canary
  cannot see it. The State-first test survives; what the page must revise is
  which line is the conclusion instant, and it must prove the set of archive
  writers by enumeration rather than assume it. The reader-side repair is
  preferred and the section 3 writer boundary stands; if the design lane finds
  the reader cannot be made sound without a writer change, that is a scope
  widening to raise, not take. Goes to the design lane as a fold.
- BCC-02 (APPROVAL_REQUIRED cannot reach the frontier in production because
  file.go:495-504 and ValidateApprovalRecord reject the record at parse):
  accepted as a wording and fixture-premise correction, folded with BCC-01.
  The Refused category's production cause set is GOAL_NORM_REFUSED; the page
  may keep the typed branch but must not premise a fixture on the synthetic
  path or state the category as if it were reachable in production.
- BCC-03 (concludingEpoch keeps its own scan): accepted as wording; the fold
  either routes it through the helper or says honestly that it does not.

Coordinator's own error carried forward: the brief for revision 1 named a
wrong discriminator, which the page refuted; nothing in this round changes
that record.

Next: fold brief to the design lane (Fable, R-89-m1b) for revision 2 in
place, with a split-parent canary that must fail on the untouched tree; then
a fresh read whose reviews is revision 2.
