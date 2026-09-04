# channel-fake-server-synthetic-timestamps

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="The fake channel server used only by fixtures stamps replies in 1970; the send-time rule refuses them; only the fixture suite is affected."
- Tier: 1
- Intent: internal/channel/fake/fake.go stamps every Slack reply with a synthetic counter timestamp (for example 1000002.000000, in 1970) and ignores a ts the fixture supplies; since the code send-time landing (4b919708) the Slack adapter verifies a reply's code at its send time and refuses any reply older than the poll interval plus one step, so the channel fixture's coded answer is answered 'not recorded: code too old: sent 1787543923s before the poll' and the suite is red. Seen seat-side on m2 2026-09-04 17:46Z through the fake journal. DONE means the fake server stamps replies with the current time (and preserves a ts the caller supplies), the same for the fake Telegram update's date, and channel-fixtures.sh is green.
- Origin: main
- Next step: One fake-server change and a test: build, go test ./internal/channel/fake/..., run channel-fixtures.sh seat-side, land through a chain. Approved under R-76-m2.
- OpenedAt: 2026-09-04T17:49:14Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-04T17:49:14Z FJKE4MY2NK3B98A0AZHTDJ9H3X-m2-5fcf08ab open actor=human:Wido targets=channel-fake-server-synthetic-timestamps
Integrity: sha256=8a72f8f10bae6ce8f8650fe5a770b2a02cca3413b2790a12be8011fdbd2a04b0
