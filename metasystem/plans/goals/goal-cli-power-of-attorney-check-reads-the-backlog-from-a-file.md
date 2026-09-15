# goal-cli-power-of-attorney-check-reads-the-backlog-from-a-file

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a false or unexplained red in one fixture leg; novelty 1: the file-first pattern is already in the same bed; exposure 2: every run of the goal-cli deep section; accumulation 1: one site"
- Tier: 1
- Intent: section/goal-cli-fixtures fails its power-of-attorney scenario on unmodified trunk with 'the root record does not carry the tiers 1,2 entry'. The seat reproduced it on 2026-09-15 by running the scenario directly as a harness child three times on trunk 2c8c5d0a and three times on a candidate: six of six failed. m1e saw it green on 6f9e58e7 and fad3ace8 earlier that night. The failing check, scripts/agents/goal-cli-fixtures.sh lines 2104 to 2105, pipes git cat-file -p of the fixture's backlog.md into grep -q under set -euo pipefail, and prints nothing else. The earlier grant check at lines 2066 to 2067 writes the file out first. Either grep -q exits early and cat-file takes SIGPIPE (class: pipelines-never-lose-a-truncated-producer), or the tiers 1,2 entry is really missing. DONE: the check reads the backlog from a file with each status checked and prints the file when the entry is absent; the cause is named with that output; the scenario passes on trunk under the harness.
- Origin: human
- Next step: Build, tier 1: write the backlog to a file first, as lines 2066 to 2067 do, and print it on failure. Run the scenario to see whether the entry is present. If it is, the pipe was the cause and the fix is complete. If not, name the missing entry's cause and fix that too. Free for the next seat by sequence.
- OpenedAt: 2026-09-15T01:12:48Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-15T01:12:48Z 3YCCZA54QX1AFCPMT8W1VTDXK7-m1e-c6925449 open actor=human:Wido targets=goal-cli-power-of-attorney-check-reads-the-backlog-from-a-file
Integrity: sha256=e461487d191e520d80d4c27ffc65bd0b94120da62e6c7af1e25ebe59690d0716
