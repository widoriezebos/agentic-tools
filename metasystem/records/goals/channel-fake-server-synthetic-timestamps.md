# channel-fake-server-synthetic-timestamps

- State: done
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="The fake channel server used only by fixtures stamps replies in 1970; the send-time rule refuses them; only the fixture suite is affected."
- Tier: 1
- Intent: internal/channel/fake/fake.go stamps every Slack reply with a synthetic counter timestamp (for example 1000002.000000, in 1970) and ignores a ts the fixture supplies; since the code send-time landing (4b919708) the Slack adapter verifies a reply's code at its send time and refuses any reply older than the poll interval plus one step, so the channel fixture's coded answer is answered 'not recorded: code too old: sent 1787543923s before the poll' and the suite is red. Seen seat-side on m2 2026-09-04 17:46Z through the fake journal. DONE means the fake server stamps replies with the current time (and preserves a ts the caller supplies), the same for the fake Telegram update's date, and channel-fixtures.sh is green.
- Origin: main
- Next step: One fake-server change and a test: build, go test ./internal/channel/fake/..., run channel-fixtures.sh seat-side, land through a chain. Approved under R-76-m2.
- Concluded: Landed b5782c4d: the fake server stamps replies with real time, the fixture answers with the token, and channel-fixtures.sh is green seat-side (41 seconds).
- OpenedAt: 2026-09-04T17:49:14Z
- Revision: 5
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-04T17:49:20Z revision=2 opid=GF8C4RE0R9C38MGFNWM6SK9YFE-m2-5fcf08ab authority=relayed digest=3fa0fade8bd1c228420ceb9bd58a9e3bdc4d8162bb43df9b55f40b13f05129be reviewBy=2026-09-06
- Sliced: machine=m2 lineage=main-1788441779-14484-82d6ed revision=3 at=2026-09-04T17:51:10Z

History:
- 2026-09-04T17:49:14Z FJKE4MY2NK3B98A0AZHTDJ9H3X-m2-5fcf08ab open actor=human:Wido targets=channel-fake-server-synthetic-timestamps
- 2026-09-04T17:49:20Z GF8C4RE0R9C38MGFNWM6SK9YFE-m2-5fcf08ab approve actor=human:Wido targets=channel-fake-server-synthetic-timestamps authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="You may pick and execute the tier-1 items opened on 2026-09-04 in my name."
- 2026-09-04T17:50:25Z 39ECE9Y544Y0VN2QDZ03BCZ85F-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=channel-fake-server-synthetic-timestamps
- 2026-09-04T17:51:10Z 66RB8MDVZB7JKAWM1FX75DTX6M-m2-5fcf08ab slice-start actor=m2+main-1788441779-14484-82d6ed targets=channel-fake-server-synthetic-timestamps
- 2026-09-04T18:10:37Z T93GX7DDESA19KDMM8PCWPE28S-m2-5fcf08ab done actor=m2+main-1788441779-14484-82d6ed targets=channel-fake-server-synthetic-timestamps
Integrity: sha256=82bf9cd0dbd87a58af552b31cb170080e5aebc3ff4319812828fd5c0af008d13
