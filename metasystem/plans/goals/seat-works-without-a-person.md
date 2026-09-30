# seat-works-without-a-person

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="engine starts agent sessions unattended on this computer with the human own runtime credentials; bounded by the existing continuation role and permissions preset; local to one seat"
- Tier: 2
- Intent: What: when a seat has approved goals ready and no agent running, the steward starts an agent on that seat by itself, and a goal that becomes ready while the seat is idle is picked up on the next check. The code for this is built and on main (batch 3); what is left is one real run on a live seat. Why: until now a person had to notice waiting work and start an agent by hand, so ready work sat untouched overnight and whenever nobody was watching. Pros: machines keep delivering approved work without a person starting each session; idle seats cost nothing. Cons: an agent started without a person spends tokens and makes commits nobody watched live, so it stays opt-in per seat and must be easy to stop.
- Origin: main
- Next step: Next: run the first live proof on the ui seat: rebuild the engine, return the helm, switch on the seat launch setting, and let the steward start a seat against one approved goal; then mark the design page done. Done when: a steward-started seat claims an approved goal, commits on its branch and exits at idle.
- OpenedAt: 2026-09-30T11:55:56Z
- Revision: 4
- BudgetExceptions: 0

History:
- 2026-09-30T11:55:56Z CCNKJTVQC8G2MASVSB426PXV0H-ui-966d857e open actor=human:Wido targets=seat-works-without-a-person
- 2026-09-30T14:44:04Z T57Q3TFPCJ8A5NP2166VK73663-ui-bc2fda53 approve actor=human:Wido targets=seat-works-without-a-person
- 2026-09-30T18:57:18Z TXBHCEZ001FTF9EB2B1DMH5KRX-ui-bc2fda53 unapprove actor=human:Wido targets=seat-works-without-a-person reason=the backlog clean-up of 2026-09-30 rewrites the intent in plain English; approved again with the same box
- 2026-09-30T18:57:23Z HHNSD6CMJDF1GF5G8WAMKKCZE8-ui-bc2fda53 edit actor=human:Wido targets=seat-works-without-a-person
Integrity: sha256=097d005a06bea58d6172c98e44fe4be02cc6f5e2f9d5fe27a355af8b140bbfc8
