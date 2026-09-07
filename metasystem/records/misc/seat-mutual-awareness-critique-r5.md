# Design critique register: seat-mutual-awareness, round 5 (job sma-crit5-20260907)

Design under review: metasystem/plans/seat-mutual-awareness-design.md revision 5 at commit 3f782639 (SHA-256 cf709ed9fe29abf981c70cf30c185a5b865d1141fd20501779fc23c8da33b98b). Critic runtime: codex, model {'effective': 'gpt-5.6-sol', 'requested': 'gpt-5.6-sol'}. Material findings: 1 of 1. The critic ran read-only and could not write this record itself; the seat rendered it from the critic's return (artifacts/agents/sma-crit5-20260907/rounds/1/return.json) without editing the words. SMA-C-26 and SMA-C-20 of round 4 are not reopened.

## Gaps the critic declared

- The proposed seat implementation and revision-5 fixture do not exist yet, so their behavior could not be executed; fixture ownership was checked from the design and existing code. Live proof was not required.

- This read-only critic cannot create metasystem/records/misc/seat-mutual-awareness-critique-r5.md; the orchestration seat must render that declared register from this return without altering the finding.

- The launcher exposed neither a critic runtime session identifier nor proof of independent context isolation or the complete tool catalog. The session identifier is therefore reported as unobserved, and this review remains advisory.

## Findings

### SMA-C-25 (material: True, severity: high)

Claim: Seat Mutual Awareness finding SMA-C-25 is reopened: the rollout can still confirm a machine without a running reader from the new generation. Section 8 admits the re-arm component, aggregate re-arm fact, or arming-log line, but all are emitted when the identity is minted, before the new steward runner is successfully launched and verified. The existing rearm-launch-fails fixture proves those exact lines coexist with an overall failed up invocation. The proposed rebuilt-bytes fixture checks only the state before up and the successful path afterward, so it would not catch this failure. Section 8 also treats the presence record’s engine field as confirmation even though that record names no enrolled generation. An implementer following the design would therefore build an unsafe readiness gate and an incomplete fixture in metasystem/plans/seat-mutual-awareness-design.md and metasystem/scripts/agents/supervision-fixtures.sh.

Evidence: Metasystem/plans/seat-mutual-awareness-design.md lines 728-760 admit the mint-stage re-arm lines and the generation-less presence field. Metasystem/internal/up/up.go lines 562-573 create the fact and log at the minted stage, while lines 586-596 can return the re-armed component followed by a steward-runner failure. Metasystem/internal/steward/runner.go lines 614-645 set the minted stage and re-armed status before opening, preparing, and launching the runner. Metasystem/scripts/agents/supervision-fixtures.sh lines 1049-1075 inject the post-mint launch failure and require the re-arm lines to remain visible. The revision-5 fixture at design lines 885-907 covers only no-up and successful-up states.

## Rigor

- SMA-C-25: unproven; reopening trigger: Re-examine when every admitted confirmation proves the live runner generation: a successful up invocation must pair the re-arm fact with a successful steward-runner result for the same generation; mint-only component, aggregate, and log lines must be rejected; presence must either carry and prove that generation or cease to be confirmation; and the focused fixture must reject the existing post-mint launch-failure state.
