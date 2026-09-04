# fleet-channel-gateway — dispositions, build step 2 code review, second root (job fcg-build2-cc2)

reviewedTree 2504bd49, on the round-4 tree; one material finding, sent back as round 5 (plans/fleet-channel-gateway-build2-fix4-brief.md), and two notes. The narrative is in plans/fleet-channel-gateway-build2-code-review-dispositions.md.

| Finding id | Disposition | Reasoning and evidence | Amendment |
|---|---|---|---|
| F-1 | accepted | round 4 changed the post-confirm Receive in TestTelegramListenersShareStreamAndConfirmedOffset from the empty cursor to cursor "1", and the design sentence "getUpdates without offset returns only unconfirmed updates" (design line 949) lost its only test; the round-4 brief allowed test changes only where F-2 changes what a lower offset returns | round 5 restores the empty-cursor leg after the confirm alongside the offset-1 leg, fake_test.go only; the third root (fcg-build2-cc3) certified it at reviewedTree 98f0b886 |
| F-2 | noted | reloading control.json on every tick of a blocked long poll multiplies the chance of reading a truncate-then-write rewrite while empty, which ends that poll with a 500 the fixture did not intend; the test helper uses os.WriteFile and channel-fixtures.sh has no control.json writer yet | carried to the cut-over step's fixture brief as a rule: every control.json writer writes by rename |
| F-3 | noted | the delivery-controls test adds three seconds of real time and the malformed-control test binds the Go parser's "invalid character" wording, the same kinds as the first root's F-6 and F-8 | no change |
