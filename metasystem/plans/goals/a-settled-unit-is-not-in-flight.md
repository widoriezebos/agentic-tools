# a-settled-unit-is-not-in-flight

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="One gate in the steward's claim-quota ask, about 15 production lines, covered by the existing table test; the worst outcome is one extra ask to a holder"
- Tier: 1
- Intent: The claim-quota ask's in-flight gate names the stages that are in flight (starting, running, built or committed ready for review, under examination, collected but not yet published, drop pending, stopped needing a revision) instead of allow-listing three settled ones, so a holder whose unit is dropped or whose commit is ready to land without a read is asked once to release the machine's one claim (the-steward-acts U4c2 read 4 F-1, accepted as a risk at its landing; finding 522)
- Origin: human
- Next step: One unit, about 15 production lines in internal/steward/claim_quota.go (claimQuotaInFlight) plus the settled stages as expect-an-ask cases of TestNoReleaseIsProposedForWorkInFlight; brief cut from D13 of the-steward-sees-and-acts and finding 522; one Astra round; built beside the-claim-quota-is-a-host-setting on m1e
- OpenedAt: 2026-11-05T13:38:39Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-11-05T13:38:39Z Y8E6KYQP71XS778WQHX1WR82HV-m1e-c921266c open actor=human:Wido targets=a-settled-unit-is-not-in-flight
Integrity: sha256=0a754e03d0a559b82db996f6734d12c555090d36b559cc7721259b849a92d04c
