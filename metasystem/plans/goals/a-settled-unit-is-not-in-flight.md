# a-settled-unit-is-not-in-flight

- State: approved
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="One gate in the steward's claim-quota ask, about 15 production lines, covered by the existing table test; the worst outcome is one extra ask to a holder"
- Tier: 1
- Intent: The claim-quota ask's in-flight gate names the stages that are in flight (starting, running, built or committed ready for review, under examination, collected but not yet published, drop pending, stopped needing a revision) instead of allow-listing three settled ones, so a holder whose unit is dropped or whose commit is ready to land without a read is asked once to release the machine's one claim (the-steward-acts U4c2 read 4 F-1, accepted as a risk at its landing; finding 522)
- Origin: human
- Next step: One unit, about 15 production lines in internal/steward/claim_quota.go (claimQuotaInFlight) plus the settled stages as expect-an-ask cases of TestNoReleaseIsProposedForWorkInFlight; brief cut from D13 of the-steward-sees-and-acts and finding 522; one Astra round; built beside the-claim-quota-is-a-host-setting on m1e
- OpenedAt: 2026-11-05T13:38:39Z
- Revision: 3
- Pinned: m1e
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=600 activeJobLimit=1 reviewRoundLimit=6
- BudgetExceptions: 0
- NormApproval: approvedRef=97AV3AE4NWERSAVYEHKPJ89RB4-m1e-c921266c minutes=600 reviewRounds=6 goalRevision=1
- Approved: by=human:Wido at=2026-11-05T13:38:58Z revision=2 opid=97AV3AE4NWERSAVYEHKPJ89RB4-m1e-c921266c authority=proven digest=514c98463753961c4a7515dc56531209d88d85aea56de108b207e68ff75aa842 episode=2

History:
- 2026-11-05T13:38:39Z Y8E6KYQP71XS778WQHX1WR82HV-m1e-c921266c open actor=human:Wido targets=a-settled-unit-is-not-in-flight
- 2026-11-05T13:38:58Z 97AV3AE4NWERSAVYEHKPJ89RB4-m1e-c921266c approve actor=human:Wido targets=a-settled-unit-is-not-in-flight
- 2026-11-05T13:39:16Z ZZT7WVM2P4JVS2T96XMS85WC00-m1e-c921266c set-pin actor=human:Wido targets=a-settled-unit-is-not-in-flight
Integrity: sha256=842701bfc61f8d4e6f81611cf9d98dec9f6d7abb769683b5e6d1de620f4ea6ab
