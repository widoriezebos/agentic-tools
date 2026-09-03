# human-approval-for-execution

- State: queued
- Intent: Wido's rule 2026-09-03 (verbatim): 'everybody can get anything on the backlog, but only the human (I) can approve it for execution. So the state ready for impl can only be set by me'. TODAY IT IS NOT SO: the ledger knows four states (queued, claimed, parked, done); a queued goal becomes claimable the moment it carries a budget, and under R-44-m0b and R-45-m0b a seat sets the standing budget itself (small items 4h/10/240m/1, big 8h/10/240m/1), so any seat can open, budget and claim an item and start spending without a per-item human word; only an over-norm budget needs a ruling token. Every seat has used that path this week. WHAT THIS FEATURE IS: a fifth state between queued and claimable, set only by the human: approved for execution (ready for implementation). Opening stays free for everyone, machines included; a claim is refused on a goal that is not approved, naming the missing approval; the budget attaches at approval (the standing tuples of R-44/R-45 become the default a human approval carries, or the human states another); approval is a human-only act by process ancestry at a terminal, or by the relayed-word path already used for resume and set-obligation (R-32-m1), so Wido can approve from his phone and, once the fleet conversation channel lands, from a Slack thread; a human may also revoke approval, which parks the claim at the next safe point. DONE means: the state exists in the goal file and the projection; goal approve and goal unapprove are human-only verbs with the relayed-word form; claim refuses an unapproved goal with a typed message; the steward's idle verdict counts only approved goals as claimable work (so machines never nag about unapproved items); goal next shows approved-and-claimable separately from queued-awaiting-approval; existing queued goals with a budget are grandfathered only by an explicit human sweep (a listing he approves in one act), never silently; fixtures prove each refusal and the relayed-word approval. CONDUCT UNTIL IT LANDS: a seat claims only a goal Wido approved by word, recorded on the goal.
- Origin: main
- Next step: LANDED c285d5a0 (2026-09-03): the approved state, the human-only goal approve verb, APPROVAL_REQUIRED on every execution path, over-norm token still required, edit-invalidates-by-digest, the grandfather sweep, and the relayed-word form are all on main. Tier-3 ladder held (design, one review, one fold, one closing review, build, one code review + fixes). Residual: decision D7 (retired-seat-pool raise vs R-58-m1) flagged in the design for Wido; coverage floor met at 80.3%. m0 releases to take first-headless-run per R-66-m1.
- OpenedAt: 2026-09-03T09:07:28Z
- Revision: 11
- Labels: authority, backlog
- Budget: elapsedLimit=2d attemptLimit=30 reservedJobMinutesLimit=3000 activeJobLimit=1
- NormApproval: approvedRef=R-63-m1 minutes=3000 goalRevision=5
- Sliced: machine=m0 lineage=main-1788178136-1684505-4ffe42 revision=6 at=2026-09-03T09:39:42Z

History:
- 2026-09-03T09:07:28Z JHK32Y2RJ8M4K8Y6JTFG7BN97P-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=human-approval-for-execution
- 2026-09-03T09:15:44Z 2DFZ4E1RCA96Z24NSHXZCR4P31-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=human-approval-for-execution
- 2026-09-03T09:17:54Z 33P4CKTKH9C0TS8QBE7SJ51BAM-m1-7bb1546e set-budget actor=m1+main-1788333680-2840-7f79f4 targets=human-approval-for-execution
- 2026-09-03T09:21:19Z F9ZY5R5S6YV8C8JMKZDBCK6JTF-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=human-approval-for-execution
- 2026-09-03T09:22:31Z EN41A0GJPX17WS1SDQ7D00RZGP-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=human-approval-for-execution
- 2026-09-03T09:37:59Z DKQDHYR74RQCFZ5BJSDS22N13C-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=human-approval-for-execution
- 2026-09-03T09:39:42Z TPFEXX16ZGB198JEWJ877BDJ3F-m0-c5dbf036 slice-start actor=m0+main-1788178136-1684505-4ffe42 targets=human-approval-for-execution
- 2026-09-03T09:58:43Z XQF7D1AQAM5ZSDM13SV01C161C-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=human-approval-for-execution
- 2026-09-03T10:43:26Z QDT124ZPJFQ0PHZ7YGNWT7XBE3-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=human-approval-for-execution
- 2026-09-03T13:54:01Z CFR2S5CFEYRD5S5NX5NTAXWQM8-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=human-approval-for-execution
- 2026-09-03T13:54:05Z V8KENZXFC3HQC3HVRRV5TXWVFZ-m0-c5dbf036 release actor=m0+main-1788178136-1684505-4ffe42 targets=human-approval-for-execution
Integrity: sha256=0bcc5b404f228a8506a9b00923b460c3c2cfcf6d70f3fa8e1f1493f1d89631fa
