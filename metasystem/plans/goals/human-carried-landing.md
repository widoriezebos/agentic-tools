# human-carried-landing

- State: claimed
- Tier: 3
- Intent: A verified human may fast-track a landing under close scrutiny and the machine never refuses that human. Wido, verbatim (2026-09-04): 'I'm considering a Just Fucking Do It risk tier for hot-patch scenarios that need to be fast-tracked under close human scrutiny' and 'it must also be fully authorized by the human explicitly', and the governing line: 'I do not ever want to be in a HAL2000 situation where the computer refuses under a situation judged as critical for the human'. Design shape agreed with the seat: an authority mode, not a tier (the tier stays derived from the four risk answers; urgency changes who carries the rigor, never the risk); one explicit human word per candidate tree, digest-bound, expiring in hours, through the enrolled terminal or the TOTP word; the closing review it skips becomes an open review obligation on the goal so conclude refuses until it is discharged; every use increments BudgetExceptions and the appetite line counts it; the gate width and the battery still run; the machine warns, records and counts and never blocks a verified human. DONE means the verb exists, the audit below is landed, and a tier-3 chain lands in one command on the human word with the obligation and the counter written.
- Origin: human
- Next step: SLICE 1 LANDED ef296c39 (the refusal register, internal/refusal, 171 tokens, three tests, closing review hcl-audit-cc-2 nothing material). Review notes for slice 2: (1) internal/landing/tierone.go:38 builds tier1-goal-tier-<n>-refused with %d, which no grammar collects; it needs a row by hand and a design amendment to HCL-AUDIT-03; (2) the eleven landing codes outside knownRefusalCode are rows without Defect entries because their sites already return Mode refuse, so the promotion reader never needs them; ruling on that reading belongs to slice 2; (3) land.sh:116 and :120 (the tier-1 declaration refusals) are not in ShellRows. THE CARRY (points 02, 04-09) WAITS ON WIDO'S RULING, question XXG9ZYJJ7KHE1K0WVB28DJPC21: sibling goal with its own chain (recommended), park, or spend the last round on the frozen chain hcl-design-cc1. Budget after slice 1: four attempts and 105 reserved minutes spent (two setup refusals consumed ops: hcl-audit-build-1 and hcl-audit-cc, both BRIEF_AUTHORITY_REFUSED on new-file paths), two of three design review rounds consumed.
- OpenedAt: 2026-09-04T10:56:02Z
- Revision: 8
- Labels: governance
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-04T11:10:45Z revision=4 opid=1EY407NT6NWYHYWWB7EP376V8C-m3-a5da21ff authority=relayed digest=a040b44ca36176f3e5484fbd34b9741dcbeeb6cdff82460428b7250650e0628c reviewBy=2026-09-06
- Sliced: machine=m3 lineage=mac-m3 revision=5 at=2026-09-04T14:47:24Z
- Claimed: machine=m3 lineage=mac-m3 at=2026-09-04T14:44:56Z revision=5 accountingRevision=5
- StopCapability: generation=5 revision=5 machine=m3 claimEpoch=1 fenceEpoch=0

History:
- 2026-09-04T10:56:02Z 295K6XSWMRHBC0V1H94Y9C6XQ9-m3-a5da21ff open actor=human:Wido targets=human-carried-landing
- 2026-09-04T11:03:19Z 1FM8FM8VANKPK8H7FP5SWAP71Z-m3-a5da21ff ask actor=m3+mac-m3 targets=human-carried-landing
- 2026-09-04T11:09:39Z 1NFRANX98GMQRVDZ8E7671Y9H6-m3-a5da21ff answer actor=human:wido targets=human-carried-landing authorityOutcome=AUTHENTICATED_CHANNEL_WORD channelProvider=telegram channelUser=1365582 channelRef=24/27 channelStep=59617339 reason=approved
- 2026-09-04T11:10:45Z 1EY407NT6NWYHYWWB7EP376V8C-m3-a5da21ff approve actor=human:Wido targets=human-carried-landing authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="ask me on telegram and i will approve"
- 2026-09-04T14:44:56Z X47078W5EG395PG2N6HSP4H1HM-m3-a5da21ff claim actor=m3+mac-m3 targets=human-carried-landing
- 2026-09-04T14:47:24Z FRTJKG2C1D91FDNTFB7ZYPZNYS-m3-a5da21ff slice-start actor=m3+mac-m3 targets=human-carried-landing
- 2026-09-04T15:32:48Z SJZE941HTX7XA8YG8VA5Y3N3CM-m3-a5da21ff ask actor=m3+mac-m3 targets=human-carried-landing
- 2026-09-04T15:53:20Z SZPNYSR54WKGY9ZX15GA2HDNC9-m3-a5da21ff edit actor=m3+mac-m3 targets=human-carried-landing
Integrity: sha256=7ca4e360f35bd377c8166ff4cef00e6d0f7a33de5dd107b617d423fd4cbadcdd
