# fleet-channel-gateway — dispositions, build step 3a code review, first root, round 2 (job fcg-build3a-cc-r2)

reviewedTree 516c8e5c, on the round-3 tree; the register carry. Zero material findings: F-4 withdrawn, one note. The narrative is in plans/fleet-channel-gateway-build3a-code-review-dispositions.md.

| Finding id | Disposition | Reasoning and evidence | Amendment |
|---|---|---|---|
| F-4 | noted | withdrawn by the critic: the budget row is exercised end to end through ChannelInboundRequest in both branches; the first assertion fails if the rowName choice is removed (the classifier refuses and Publish returns rejected) and the row-probe assertion fails in whichever branch expects the other row if the choice is hard-coded; every contract field is asserted on the committed tree | none; the register is clear |
| N-5 | noted | the test swaps two ChannelMatrix entries per subtest and restores them by defer; safe while the goal package's tests run sequentially (no t.Parallel in the package) | no change; the same note as the second root's N-2 |
