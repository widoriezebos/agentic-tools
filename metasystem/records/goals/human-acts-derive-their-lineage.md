# human-acts-derive-their-lineage

- State: done
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="It costs the human a refusal and a paste on every terminal act; no data is at risk; every machine's terminal has it."
- Tier: 2
- Intent: Every goal mutation demands a coordinator lineage (--lineage or METASYSTEM_OWNER_LINEAGE) or refuses with 'mutations carry their coordinator's identity', including human-only acts run at the enrolled terminal with --by. Wido hit it on 2026-09-06 concluding race-gate-red-on-main from his terminal: the refusal names nothing he can derive, and the value he must paste is a seat's session-main id he has no reason to know. A human act at an enrolled terminal already proves who acts (the terminal enrollment and --by); its lineage should follow from that, not from a seat's environment. DONE means a human-only verb (approve, unapprove, set-budget, done, edit --by, park, resume, accept-risk, set-obligation) run at the enrolled terminal without --lineage derives a lineage from the enrolled terminal identity and records it, the refusal text for agent callers stays, and a goal-cli fixture pins the terminal path without --lineage.
- Origin: main
- Next step: STATE 2026-09-06 21:40Z (m1c): hal-build1b built the brief (tree 32d55266): syncReq derives terminal-<sanitised id>-<generation> from the enrollment for a --by act with no lineage, refuses naming enroll-terminal without one, keeps the agent text; TestSyncReqLineage (three cases, enrollment via humanauthority.Enroll) and a goal-cli leg human-lineage. Seat proofs green: cmd/metasystem package, vet, gofmt, bash -n, and the full goal-cli fixture bed PASSED with the new leg. Critique r1 (hal-critic1) running with a shell. Then dispositions, close, land.sh --chain (area), rebuild, up, done; the live proof is Wido's next --by act without a lineage.
- Concluded: Landed at 6ce7d5b2 (chain hal-build1b, two rounds, two Fable critiques with a shell, r2 zero material): a --by act with no --lineage and no METASYSTEM_OWNER_LINEAGE derives lineage terminal-<terminal id>-<generation> from the terminal enrollment, but only after the invoking process proves the enrolled terminal (humanauthority.Prove; the six verbs that already prove reuse their proof); a shell that does not descend from the terminal is refused naming the terminal, a checkout with no enrollment is refused naming enroll-terminal with the reader's error, and an agent call without --by keeps the old refusal byte for byte; enroll-terminal enrolls first and records the lineage of the enrollment it published; discharge-review-obligation passes its name. TestSyncReqLineage pins four cases with a deterministic process reader; the goal-cli bed's human-lineage leg approves with --by and no lineage against an on-disk enrollment and checks the opid's lineage hash; the bed is green on the landed tree. Dispositions in records/misc/human-acts-derive-their-lineage-critique-r1-dispositions.md and -r2-. Live proof owed to Wido's next --by act at his terminal without a lineage.
- OpenedAt: 2026-09-06T10:57:41Z
- Revision: 7
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T11:02:38Z revision=2 opid=SB01VF9Q2A59X40GM66RJJP2JV-m1-a4f8999f authority=proven digest=294c3e20b1f457b5ae16b73899f61d641b9e7a7e7830e7f01fca008860b30298
- Sliced: machine=m1c lineage=main-1788680061-17829-64951c revision=3 at=2026-09-06T21:02:18Z

History:
- 2026-09-06T10:57:41Z YSC37S3KG47CEV62V250QDKY2B-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=human-acts-derive-their-lineage
- 2026-09-06T11:02:38Z SB01VF9Q2A59X40GM66RJJP2JV-m1-a4f8999f approve actor=human:Wido targets=code-critic-runtime-has-no-shell,human-acts-derive-their-lineage,land-sh-omits-the-full-width-chain-receipt,landing-receipt-races-the-narrator-digest
- 2026-09-06T21:00:46Z PJFBV6VXGEBMZ1Y8DNTWY4DV0A-m1c-7cd0bd60 claim actor=m1c+main-1788680061-17829-64951c targets=human-acts-derive-their-lineage
- 2026-09-06T21:02:18Z 51NNCHW65JCAFVJ6DPY1GTE7XD-m1c-7cd0bd60 slice-start actor=m1c+main-1788680061-17829-64951c targets=human-acts-derive-their-lineage
- 2026-09-06T21:02:59Z DZ8BYQ6F13D9ZT2QDJ3ND7FKGN-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=human-acts-derive-their-lineage
- 2026-09-06T21:22:45Z NZ6780CCRR02STBNSAS46J273C-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=human-acts-derive-their-lineage
- 2026-09-06T21:52:22Z 5P5BH7DFGAWS8DWB9JJPVV1FQ4-m1c-7cd0bd60 done actor=m1c+main-1788680061-17829-64951c targets=human-acts-derive-their-lineage
Integrity: sha256=b68819b673af5db0a0724b20db73de52d7e2076398e1f0fab65b2e626e171650
