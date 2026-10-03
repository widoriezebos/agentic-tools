# review-findings-read-as-decisions

- State: approved
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="The surface where a person decides what lands; a wrong consequence text makes him record what he did not mean, so severity 2; new fields in the critic's return and new decision acts, novelty 2; every review sitting, exposure 2; nothing accumulates beyond the review records"
- Tier: 2
- Intent: A review sitting in the web UI reads as decisions a person can take: every finding says in plain English what the problem is, why it matters (what is gained or lost), and what the reviewer recommends, with a severity word; the person's choices are named acts with their consequence stated before the press (must fix before landing, fix after landing, not a problem with a reason, accept this risk with a reason), and the sitting ends with one verdict on the goal; the critic's ids, paths and timestamps are evidence behind a fold (Wido 2026-10-03 16:28 CEST: 'the texts are totally useless: they have no meaning at all for a human. All should be in plain easy to understand English, listing the impact (pros and cons) and a recommendation. Right now I have no idea what I would be recording'). Today a finding is a generic Partner deposit: the critic's text as written for an engineer, an anchor path, a consequence sentence, and the buttons Record it and Dismiss, with no summary, no severity, no recommendation and no word on what gets recorded.
- Origin: human
- Next step: UX design first, by a UX designer, from m1e's input at agentic-tools-evidence/review-sitting-ux-20261003/design-input.md (the screenshot, the three questions a person asks, the summary line, the finding card in plain layers, the four decisions with their consequences, the verdict step, phone width, and where the plain text comes from: the critic writes title, why-it-matters and recommendation as return fields under the message audit, the Partner only as fallback). Agree the finding fields with critique-stops-on-convergence. Tier-2 critique, then build in slices: the summary and cards with the four decisions (desktop and phone), the critic's plain fields and their audit, the verdict step. Done when: a person can read a sitting without any id or path, every button says what follows, and a recorded decision is shown back as 'recorded on goal X as ...'; tests cover each decision act.
- OpenedAt: 2026-10-03T14:24:15Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-03T14:24:23Z revision=2 opid=NA4JNMP5XPHSFYYJH8AM5QCKWC-m1e-718ba0eb authority=proven digest=1342c0bb2c1ef2ce9a62f7b53641dfb1b493229d3d6476646087368bd889a15b episode=2

History:
- 2026-10-03T14:24:15Z 5NY52EMF7696VJY0TBPGZ5W14X-m1e-718ba0eb open actor=human:Wido targets=review-findings-read-as-decisions
- 2026-10-03T14:24:23Z NA4JNMP5XPHSFYYJH8AM5QCKWC-m1e-718ba0eb approve actor=human:Wido targets=review-findings-read-as-decisions
Integrity: sha256=05da649ed40c0fb1901f74bbfea7e84165bf7c313a59893b5ae8a01602333c89
