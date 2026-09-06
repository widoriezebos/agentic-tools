# claude-critic-shell-network-deny

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="severity 2: an outbound side effect from a reviewer is a real breach but one a credential-holding human can undo; novelty 2: the sandbox's network deny shape is not yet proven on this CLI; exposure 2: every claude critic round; accumulation 1: nothing compounds"
- Tier: 2
- Intent: Once a claude critic has a shell (goal code-critic-runtime-has-no-shell), the record's network rule of allow gives it outbound side effects the read-only tool list never offered: a git push with the user's stored credentials, a curl upload, a package install. The round-one critique of that goal (ccs-critic1, F-3) named it; the brief kept the network rule unchanged because the sandbox's network shape was unproven. DONE means a critic-set delegate (code-critic, design-critic, warden) with no write roots requests and gets network deny in its sandbox settings (the shape the claude sandbox honours, proven by a live probe: an outbound curl from the critic's shell is refused), the record's requested network field says so, the dispatch fixtures pin it, and an implementer's network rule is unchanged.
- Origin: main
- Next step: After code-critic-runtime-has-no-shell lands: probe the claude sandbox network keys with the same live-probe recipe (scratchpad ccs-probe2.sh pattern), then brief, build, critique, land.
- OpenedAt: 2026-09-06T19:24:48Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T19:24:48Z W4E4JQ4APGSJ20W2WAP5GRQC2J-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=claude-critic-shell-network-deny
Integrity: sha256=1997b77ed8abb0dcfbf32443661000b7735eacc25f220d3a7d9816245471eccc
