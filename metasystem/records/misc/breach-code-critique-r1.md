# Fable code critique of the breach-clock build — round 1 (2026-09-02)

Job breach-crit-code-2 (code-critic, claude Fable, cap 20; breach-crit-code-1
died on the Claude session limit that reset at 22:50 and reviewed nothing),
reviewed build breach-build-2 at worktree commit 0d8e47ef (branch
agent/breach-build-2) against design revision 6a, brief
plans/breach-clock-code-critique-brief.md. Four findings, two material
defects, one material gate gap the orchestrator has since closed, one
non-material. Full return: artifacts/agents/breach-crit-code-2/rounds/1/return.json.

## F-1 (medium, material) — done over a fenced foreign claim loses its displacement

internal/goal/verbs.go: the Done verb's fence block (lines 802-813) sets
`f.Claimed = nil` before the displaced pair is computed from `f.Claimed`
(lines 825-828), so a human `goal done` over a claimed-and-fenced goal owned
by another pair writes no Displaced marker and the displaced machine is never
acknowledged. Release (703-709), park (909-919) and the reconcile done row
(reconcilepub.go 316-328) all compute the pair BEFORE clearing. The design's
Done rule places the clear before the existing `clearClaimBinding` call, that
is, after the displacement computation; the build put it earlier.
TestHumanDoneClearsBreach uses the own pair and cannot see it.

Decision (m0b): fix in the build. Compute `displaced` before the fence block,
mirroring release; add a test with a foreign pair asserting the marker and
the acknowledgment.

## F-2 (medium, material) — the INHERIT rule has no writer-level test

The revision-5 rule (a raise with no live obligation carries the prior
binding's third episode key forward) is asserted only by tests that write the
key by hand through `moveEpisodeClaim` and compare against their own value
(internal/dispatch/budget_test.go 323-325, 344-346, 357-360). The one
writer-level test, TestSetBudgetPinsLegacyAnchor, starts from a zero key.
`rebindClaimKeepEpisode` reads right (verbs.go), but nothing proves it.

Decision (m0b): add a writer-level test in internal/goal/verbs_test.go: claim,
set-obligation, discharge (key non-zero, no live obligation), SetBudget raise,
assert the key is unchanged and non-zero. No change to the writer unless the
test fails.

## F-3 (high, material) — the gate did not run in the sandbox

Resolved by the orchestrator's host gate (2026-09-02, recorded in the goal's
next step): go test ./... green in every changed package (internal/goal,
internal/dispatch, internal/goalbudget, internal/steward, cmd/metasystem);
the only failures, internal/missionrunner winddown tests, reproduce
identically on a clean export of origin/main. scripts/agents/goal-cli-fixtures.sh
PASSED all five scenarios, which executes inventory rows converted there and
the new day-refusal row. scripts/agents/dispatch-fixtures.sh: mission-runner,
adapter-selftest and steward-continuation pass; scenario `dispatch` fails at
GOAL_NORM_REFUSED on its 10000-minute claim identically on origin/main
(pre-existing since 84f847aa, filed as goal
dispatch-fixture-refused-by-goal-norm). Honest limit: the converted `8h`
token on that claim line is parsed before the norm refusal but the rest of
that scenario is not executed by anyone until the fixture is repaired.

## F-4 (low, not material) — soft no-write guard in the day-refusal fixture row

goal-cli-fixtures.sh 463-466 reads the origin tip through `if git cat-file
... | grep -q`; a cat-file failure passes silently. Folded into the fix build:
the cat-file must succeed or the row fails.

## Next

Sol fix build breach-build-3 (brief plans/breach-clock-fix-build-brief.md)
starts by cherry-picking 0d8e47ef and folds F-1, F-2, F-4; Fable code
critique round 2 over the fix hunks only; then land with --chain.
