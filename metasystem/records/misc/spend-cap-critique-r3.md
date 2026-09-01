# Spend-cap retirement design critique — round 3 (Sol)

Chain: revision 3 (recovery-certified) -> critic
design-critic-9613a33a5e5d79b0027f55c3 (codex gpt-5.6-sol, xhigh,
fresh context), 2026-09-01. One material finding; dissolved by Wido's
R-43-m0b clean-kill decision together with the constant it calibrated
(the disposition rides design revision 4). Persisted late — the
fold-4 delegate correctly flagged the register's absence.

## SCR-R3-LEVEL-001 — high, material=True

CLAIM: SCR-R3-LEVEL-001, the long-horizon native-budget coupling, remains incomplete. The new 200-dollar default correctly satisfies the strictly-greater rule for the shipped 150-minute watcher horizon, but the design leaves longer configured or requested dispatches and independently capped mission host turns protected only by prose telling an operator or contract author to supply a matching override. Because the implementation specification explicitly changes only the default, one test assertion, and one comment, an implementer will not enforce that requirement. For example, a legitimate 180-minute turn using the assumed 1.25 dollars per minute reaches the 200-dollar threshold after 160 minutes and can still be killed before its wall-clock cap. The design must either enforce horizon-to-budget coupling for every longer lawful horizon or exclude those horizons from the claimed never-hit contract.

EVIDENCE: metasystem/plans/spend-cap-retirement-design.md lines 292-307 restrict the build to the default, one test assertion, and one comment. Lines 337-341 and 363-369 state matching-budget requirements for longer dispatch and mission horizons without adding validation or automatic sizing. metasystem/internal/contract/contract.go lines 72-93 require host.turn-cap-min but make host.max-budget-usd optional; lines 1619-1629 validate only the shape of a supplied budget. metasystem/internal/missionrunner/host.go lines 320-322 export the override only when present. metasystem/scripts/agents/dispatch.sh lines 1565-1569 compare the time cap only with the watcher ceiling. This recurs from SCR-R2-LEVEL-001, whose recorded reopening condition required one enforced rule across all four cap sources.

## Critic-declared gaps (verbatim)

- The task describes this as the semantic third critique round, but the generated runtime and composition record it as round 1 under a new critic job identifier. This return uses the harness-observed round number and cannot prove continuation on the original critic chain.
- No paid Claude call was executed. The maximal-call catalog parameters were checked against the durable round-2 evidence and the installed Claude Code version; authoritative provider billing remains outside this list-cost design claim.
