# alert-escalation-channel

- State: claimed
- Intent: Ruling L escalations reach Wido IMMEDIATELY over a dedicated external channel - distinct from the narrator's channel - so he is notified the moment machinery lawfully needs his judgment, decision, or answer to unblock something; explicitly back of the backlog behind the planned program (Wido 2026-08-29) MERGED (backlog triage 2026-08-31, Wido's order): this goal now ALSO carries narrator-delivery-channel's scope - the narrator's digests and account delivery ride the same external channel as a second, lower-urgency message class (alerts immediate and unmissable; digests batched), one channel design, one credential shape, one delivery-receipt law.
- Origin: human
- Next step: Appetite: 3h — design then build. PROMOTED by Wido 2026-08-31; design round done (landed 578eba43), Sol critique in flight, fold next. Wido's design words, verbatim, all binding on the fold: (1) "it needs to have an abstraction/adapter. I want to be able to have email, slack, telegram, whatsapp etc underneath by simple configuration"; (2) Telegram confirmed as the first example implementation; (3) "We can use the same mechanism for the session bridge too, so there is a bit of reuse there" — the adapter contract bears a second consumer, runtime-agnostic seat-to-seat messaging (goal:seat-mutual-awareness); (4) "Another one would be slack, which has threaded messages. that also needs to fit the design of the alert channel and session bridge" — the contract must carry conversation/thread identity such that a threaded adapter (Slack) threads naturally (an episode's alert, updates, and acknowledgment as one thread; a seat-to-seat exchange as one thread) while flat adapters degrade honestly (reply-chains or flattening), never per-adapter leakage into call sites. DRIVING SPECIMEN: records/misc/idle-loss-2026-08-31.md. Alert classes: Ruling L escalations plus every blocked-on-human state; episode store is sole truth; delivery never blocks machinery; credentials outside the repository
- OpenedAt: 2026-08-29T06:19:06Z
- Revision: 10
- Budget: elapsedLimit=4h attemptLimit=3 reservedJobMinutesLimit=90 activeJobLimit=1
- Claimed: machine=m3 lineage=mac-m3 at=2026-08-31T21:12:46Z revision=10
- StopCapability: generation=10 revision=10 machine=m3 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-29T06:19:06Z 6RYCTDER5WR9E3X1VDDVPWJDGF-m1-bf243850 open actor=human:wido targets=alert-escalation-channel
- 2026-08-31T10:23:07Z DP2WFHHE2CDBNST3WANBJR2V26-m2-bc1be9cb edit actor=m2+mac-coordinator targets=alert-escalation-channel
- 2026-08-31T17:26:08Z W13H8YFCYK7Z23HXAG80856220-m3-a5da21ff edit actor=m3+mac-m3 targets=alert-escalation-channel
- 2026-08-31T19:08:09Z ZXPBZFBVW351F1BX81JMQS6SZV-m3-a5da21ff edit actor=m3+mac-m3 targets=alert-escalation-channel
- 2026-08-31T19:59:51Z EVPCQ9T098S05E41CY8AHSE5X2-m3-a5da21ff claim actor=m3+mac-m3 targets=alert-escalation-channel
- 2026-08-31T20:08:45Z 1TAC4KFHAK6MX01JDB9BN4VKHM-m3-a5da21ff edit actor=m3+mac-m3 targets=alert-escalation-channel
- 2026-08-31T20:30:12Z 9ZA3JV8P3MWRN8AC4KJZK4TXCW-m3-a5da21ff release actor=m3+mac-m3 targets=alert-escalation-channel
- 2026-08-31T20:30:36Z VJ68PSYHWX15F90A7VA98ZWXK1-m3-a5da21ff claim actor=m3+mac-m3 targets=alert-escalation-channel
- 2026-08-31T21:12:24Z MCRJK5TBPG107M4XBKJ10HFGV6-m3-a5da21ff release actor=m3+mac-m3 targets=alert-escalation-channel
- 2026-08-31T21:12:46Z AN8E31JJHQ9GAH7R68CZPFNQEB-m3-a5da21ff claim actor=m3+mac-m3 targets=alert-escalation-channel
Integrity: sha256=ac7090ed15d9f5d309967872b6767818f7c57660ce4f3fc2035a982e4783e3c6
