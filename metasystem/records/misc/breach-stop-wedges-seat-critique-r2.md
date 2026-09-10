# Critique r2 of chain bsws-build1b-20260909 (goal breach-stop-wedges-seat)

Critic bsws-crit3-20260910 (code-critic, claude claude-opus-5, xhigh,
read-only) on work round bsws-build1b-20260909-r5 (the rebase onto trunk
d1a47c354), reviewed tree 554a254b62f34268142cde646c8a52a832082216,
2026-09-10 07:04 to 07:15Z. A first attempt at this read, bsws-crit2-20260909
on round 4, died on an HTTP 429 rate limit and returned nothing. Four
findings, two material. Dispositions are the orchestrator's (m1d).

| id | severity | finding, in one line | disposition |
|---|---|---|---|
| BSW-08 | medium | The round-4 resume pre-check (BSW-02) refuses on any other live claim on the machine and never compares arcs, while the quota rule it stands in for lets members of one arc count once; a human resuming a stopped arc member beside a live sibling is now refused although the resulting tree is lawful, where before the chain it succeeded | accept; fold in round 6: skip the other claim when it shares the resumed goal's non-empty arc; arc fixture |
| BSW-09 | medium | A fourth reader of "claimed on this machine" that feeds a decision was missed: uniqueActiveProofGoal in cmd/metasystem/proof_run.go (and resolveTestingGoal in cmd/metasystem/test.go), which pick the goal proof and test accounting charge when no --goal is given. With one stopped and one live claim, the normal state this chain creates, they refuse as ambiguous, which breaks the default delivery check in commit.sh and land.sh and the main seat's proof launch; with only a stopped claim they pick a goal admission never lets a proof run against | accept; fold in round 6: skip fenced claims there as ServingProjection does; fixtures for both states |
| BSW-10 | low, not material | The channel report's two-slot cut (counts live claims only, from BSW-03) has no fixture; the one report fixture holds no live claim, so reverting the counter leaves it green | fold the fixture in round 6 while the round is open: two live arc members plus a fenced claim |
| BSW-11 | low, not material | Two notes that predate the chain: the report's FENCED lines live in the Next up block, which is dropped when nothing was delivered in the window; and the brain seat's declaration obstacle and boot line tell the human to "goal release" a claim that, if stopped, only resume can clear | not this chain; the brain advice is worth its own small goal (the orchestrator will put it to Wido) |

The critic's gaps: it ran the eight goal, steward and channel chain fixtures
on a scratch copy but not the command-package fixture, the full packages or
the goal-cli bed (the orchestrator ran all of those green outside the
sandbox on the round-5 tree); it inferred, not byte-compared, that a
refused resume leaves the goal file unchanged; it noted trunk had moved
eighteen commits past the round-5 base, none touching cmd/ or internal/,
and that the landing must re-check against the tree it lands on.
