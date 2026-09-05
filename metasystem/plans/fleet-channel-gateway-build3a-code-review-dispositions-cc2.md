# fleet-channel-gateway — dispositions, build step 3a code review, second root (job fcg-build3a-cc2)

reviewedTree 516c8e5c, on the round-3 tree; zero material findings, two notes. The narrative is in plans/fleet-channel-gateway-build3a-code-review-dispositions.md.

| Finding id | Disposition | Reasoning and evidence | Amendment |
|---|---|---|---|
| N-1 | noted | the "answer" and "answer budget" rows share one FROM predicate and the TO predicate is reached only when FROM fails on a present question, which this Mutate decides as late or unmatched first; the row name has no durable effect on a committed byte, so the round-3 probe proves the selection is made, not that a wrong selection would change the ledger; the phase, approval ULID and receipt come from the disposition helper and are asserted on the committed tree | no change; the seat reads the probe as proof of selection only |
| N-2 | noted | the test swaps package-level ChannelMatrix entries and restores them by a deferred assignment per subtest; safe while no goal test runs in parallel, a concurrent map write if one ever does | no change; recorded for the package's maintainers |
