# claude-critic-shell-network-deny

- State: claimed
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="severity 2: an outbound side effect from a reviewer is a real breach but one a credential-holding human can undo; novelty 2: the sandbox's network deny shape is not yet proven on this CLI; exposure 2: every claude critic round; accumulation 1: nothing compounds"
- Tier: 2
- Intent: Once a claude critic has a shell (goal code-critic-runtime-has-no-shell), the record's network rule of allow gives it outbound side effects the read-only tool list never offered: a git push with the user's stored credentials, a curl upload, a package install. The round-one critique of that goal (ccs-critic1, F-3) named it; the brief kept the network rule unchanged because the sandbox's network shape was unproven. DONE means a critic-set delegate (code-critic, design-critic, warden) with no write roots requests and gets network deny in its sandbox settings (the shape the claude sandbox honours, proven by a live probe: an outbound curl from the critic's shell is refused), the record's requested network field says so, the dispatch fixtures pin it, and an implementer's network rule is unchanged.
- Origin: main
- Next step: STATE 2026-09-06 23:40Z (m1c): three chains landed (ccn-build1 d471864f preset+default+fixtures+packets; ccn-fix1 7c93ce31 the conf keys; ccn-fix2 f00d3e3b the happy-leg assertion), the dispatch fixture bed green on the last candidate, engine re-armed. The second proof critic ccn-proof2, dispatched by the landed engine, is recorded with preset critic, requested and effective network deny, sandbox allowedDomains the non-resolving sentinel: that record is the goal's proof. Waiting for its return, then goal done.
- OpenedAt: 2026-09-06T19:24:48Z
- Revision: 7
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T21:18:46Z revision=2 opid=10S4262XEJBA9XC00VBMTQ7WE6-m1-7cd0bd60 authority=proven digest=b08f019671386f8a8883cb4dbcff15adab87926c80432f2961d00d9902b425f9
- Sliced: machine=m1c lineage=main-1788680061-17829-64951c revision=3 at=2026-09-06T22:30:06Z
- Claimed: machine=m1c lineage=main-1788680061-17829-64951c at=2026-09-06T22:29:53Z revision=3 accountingRevision=3
- StopCapability: generation=3 revision=3 machine=m1c claimEpoch=1 fenceEpoch=0

History:
- 2026-09-06T19:24:48Z W4E4JQ4APGSJ20W2WAP5GRQC2J-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=claude-critic-shell-network-deny
- 2026-09-06T21:18:46Z 10S4262XEJBA9XC00VBMTQ7WE6-m1-7cd0bd60 approve actor=human:Wido targets=adopt-bed-authority-probe-passes-tier,adopt-bed-gate-unreachable-under-load,claude-critic-sandbox-allows-loopback,claude-critic-shell-network-deny,claude-delegate-scratch-cleanup,claude-implementer-read-roots-writable
- 2026-09-06T22:29:53Z 3DFR259CW4D07PN7HBEZG3PKQZ-m1c-7cd0bd60 claim actor=m1c+main-1788680061-17829-64951c targets=claude-critic-shell-network-deny
- 2026-09-06T22:30:06Z M72WCXCJDWGDFZMYYM1T9ARTBE-m1c-7cd0bd60 slice-start actor=m1c+main-1788680061-17829-64951c targets=claude-critic-shell-network-deny
- 2026-09-06T22:30:33Z FAT5M09HDAVTBHT2PF19ECC4PE-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=claude-critic-shell-network-deny
- 2026-09-06T23:02:05Z X1WBRV3956B9569XJ9P71PG2B4-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=claude-critic-shell-network-deny
- 2026-09-06T23:35:29Z 5B454B1QDDSQE2HFBSX99QHN2M-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=claude-critic-shell-network-deny
Integrity: sha256=0990fd34596007f51f035819eb9ee04de74b0b715d9a2686229adbd9d40ca8bc
