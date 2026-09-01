# Spend-cap retirement design critique — round 2 (Sol)

Chain: revision 2 (implementer-0b9e9e69fdf48f4dd4aa61ed, Fable lane) ->
critic design-critic-0671f76f743c4926c1a39e8f (codex gpt-5.6-sol, xhigh,
fresh context), 2026-09-01. Verdict: converging — two material folds
incomplete (both numeric), two folds sound/non-material. Round-number
note: the harness reports this as round 1 of a new critic job; the
register carries it as the design's semantic round 2.

## SCR-R2-LEVEL-001 — high, material=True

CLAIM: SCR-R1-LEVEL-001, the 150-dollar level-calibration fold, is incomplete. Its own rule requires the threshold to be strictly greater than assumed burn multiplied by the permitted horizon, but it chooses equality: 1.25 dollars per minute multiplied by 120 minutes is exactly 150 dollars. Because Claude Code stops at accumulated cost greater than or equal to the threshold, a legitimate run sustaining that assumed rate can still be stopped at the horizon. The derivation also covers only the 120-minute dispatch default even though the same native default reaches independently capped mission host turns and configured dispatch caps. An implementer must change the literal or define and enforce matching native budgets for every longer horizon.

EVIDENCE: metasystem/plans/spend-cap-retirement-design.md:278-295 states “level > burn × horizon” and then specifies 150.00 from 1.25 × 120. The installed Claude Code predicate implements accumulated cost greater than or equal to the threshold. metasystem/internal/contract/contract.go:72-93 makes host.turn-cap-min required but host.max-budget-usd optional; metasystem/internal/missionrunner/host.go:317-322 exports an override only when present. The sample arithmetic itself is reproducible and its thinness is disclosed; the current eighth result is below the maximum, so the stale count does not change this finding.

## SCR-R2-PROTECTION-002 — medium, material=True

CLAIM: SCR-R1-PROTECTION-002, the measured-threshold and overshoot fold, is incomplete. The accounting basis and threshold-plus-one-call model are sound, but the claimed worst-call size is understated. The installed catalog permits a maximal Claude Fable 5 call costing low tens of list-cost dollars, rather than single dollars. That changes the stated maximum exposure of the proposed backstop and must be priced honestly before implementing the selected threshold.

EVIDENCE: metasystem/plans/spend-cap-retirement-design.md:165-183 says the maximum overshoot is one maximal call and describes it as “order single dollars.” Claude Code 2.1.252 lists a one-million-token window, 64,000 default maximum output tokens, and rates of 10 dollars per million direct input tokens, 50 dollars per million output tokens, and 20 dollars per million one-hour cache-write tokens. A 936,000-token context plus 64,000-token output is about 12.56 dollars with direct input or 21.92 dollars with one-hour cache creation. The observed 0.05-to-0.35-dollar overshoots establish ordinary behavior, not this maximum.

## SCR-R2-OWNER-003 — low, material=False

CLAIM: SCR-R1-OWNER-003, the wall-clock-owner fold, is newly defective in one sentence but otherwise sound. The owner inventory is corrected, while the scenario table describes an impossible combination: a live custodian whose adapter supervisor is gone. Those are the same process. Later sections correctly state the actual residual as a wedged custodian with the dispatcher unavailable, so the implementation remains unambiguous and this wording defect is not material.

EVIDENCE: metasystem/records/misc/f4-orphan-window-design.md:18-27 says the record’s custodian is the adapter supervisor. metasystem/scripts/agents/adapters/runtime-common.sh:206-305 contains that process’s deadline enforcement. metasystem/plans/spend-cap-retirement-design.md:78-96 and 330-334 describe the owners correctly, but lines 31-32 and 154 separate the custodian from its adapter supervisor.

## SCR-R2-ENVELOPE-004 — low, material=False

CLAIM: SCR-R1-ENVELOPE-004, the paid fan-out fold, is sound as an honest open-scenario record. It withdraws the categorical no-fan-out claim, states that a Bash-launched session inherits no native budget, labels detached containment unproved, names no current owner, distinguishes observation from a spend bound, and leaves the shipping-order decision explicit rather than silently resolving it.

EVIDENCE: metasystem/plans/spend-cap-retirement-design.md:191-230 records the scenario as open and unowned, lists candidate owners, and preserves the unproved nested-launch and process-group questions. Claude Code 2.1.252 help confirms that --bg starts a background session and returns immediately. No repository human ruling was found on whether containment must precede the cap change.

## Critic-declared gaps (verbatim)

- The task calls this the semantic second critique round, but the generated runtime notice reports round 1 under a new critic job identifier. This return uses the harness-observed round number and cannot prove continuation on the original critic chain.
- The exact behavior of claude --bg when launched from a paid Claude delegate remains unproved because executing it would cross an external spend boundary.
- No authoritative provider-billing source is present; only Claude Code list-cost accounting is established.
- No human disposition was found on whether the open Bash-launched-session scenario must receive an owner before the cap change ships.
- The raw 8.19-dollar and 10.01-dollar historical results remain absent; revision 2 correctly excludes their unknown durations from its elapsed-rate sample.
