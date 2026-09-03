# m2 to m3: the part-one state and the names part two depends on (2026-09-03 22:2x local)

Read records/misc/severity-tiered-rigor-split-2026-09-03.md. Agreed on the split and the ceremony.

## State of part one (chain str-build1)

Round 4 ended on the 120-minute cap with the work in the worktree (51 files, +1390/-399; `go build ./...` and `go vet` green seat-side) but no return. The chain law needs an implementer return before conformance can compute the reviewed diff, so a RETURN-ONLY round 5 runs now (no source change: gate, boundary, return); then the Fable code review, at most one correction, land with --chain.

ETA for part one on main: the review takes about 30 minutes after the return round (which is 20 to 30 minutes because the goal package tests are slow on this Mac); one correction adds about an hour. Expect part one on main between 00:30 and 01:30 local on 2026-09-04, barring a second correction, which goes to Wido.

## The names, as they stand in the worktree

- `GoalFile.Tier uint8` (internal/goal/file.go), rendered `- Tier: <n>`; zero tolerated until the classification sweep; `ApprovalDigest(intent, tier, budget)` hashes `tier=<n>` between intent and tuple.
- The token quadruple in internal/goal/norm.go: `goal=<id> minutes=<n> reviewRounds=<n> goalRevision=<r>`; `GOAL_NORM_REFUSED` names the tier box on excess.
- `goalTier` on chain roots: `BuildSetup(..., goalID, goalRevision, goalTier uint8, ...)` in internal/dispatch/build.go, `nullableGoalTier`, and the claim launch path (internal/dispatch/claim.go) per the gap-3 answer.
- Config keys in metasystem.conf: `metasystem.budget.review-round-max=3` (0 removes the ceiling; decision 06), `metasystem.budget.tier-1=1h/3/360m/1/0`, `tier-2=4h/6/720m/1/2`, `tier-3=8h/10/1200m/1/3` (five members, the last is review rounds), `dispatch.cap-max=120` (decision 14).
- `goal classify-sweep --draft <file> --preview | --confirm <digest> --by <human>` per plans/severity-tiered-rigor-build1-gap2-brief.md; revision 4's risk answers change the draft row to four scores and a basis: that is part two's amendment of the sweep, not part one's.

## Touching the close or the transition table

Nothing in the part-one tree touches internal/dispatch/close.go, finding_register.go or critique.go's transition logic; part one stops at the tier, the tuple, the sweep, the tombstone and goalTier on roots.
