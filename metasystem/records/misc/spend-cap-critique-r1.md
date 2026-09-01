# Spend-cap retirement design critique — round 1 (Sol)

Chain: design implementer-11b58a81842f112b8ff19c03 (Fable lane) ->
critic design-critic-20a8f1bdf17c3df866738e14 (codex gpt-5.6-sol,
xhigh, fresh context), 2026-09-01. Verdict: the $50-backstop direction
stands ("directionally safer than the clean kill") with four material
corrections. Revision 2 folds each by id.

## SCR-R1-LEVEL-001 — high, material=True

CLAIM: The 50-dollar level is calibrated from reservation allowances instead of actual running time, so the design has not established that it is never hit by legitimate work. The implementer needs a level derived from elapsed-time evidence and the allowed wall-clock horizon, or a different explicit contract for when legitimate work may be stopped; that decision changes the default literal and its test.

EVIDENCE: metasystem/plans/spend-cap-retirement-design.md:96-99 divides 5.07 to 10.01 dollars by approximately 40 reserved minutes to claim 0.13 to 0.25 dollars per minute. metasystem/plans/goals/dispatch-cap-necessity.md:4 records that these reservations can be 120 minutes for a 10-minute run. The seven available raw results instead measure 0.528 to 0.974 dollars per elapsed minute, including legitimate completions at 0.729, 0.738, and 0.974. At the observed high legitimate rate, 50 dollars is reached after about 51 minutes, well inside the shipped 120-minute cap, and 120 minutes extrapolates to about 117 dollars rather than the design's 30-to-60-dollar range. The design's own authoring round, metasystem/artifacts/agents/implementer-11b58a81842f112b8ff19c03/rounds/1/claude-result.json, completed legitimately at 4.499743 dollars in 6.096 minutes, or 0.738 dollars per minute.

## SCR-R1-PROTECTION-002 — high, material=True

CLAIM: The native flag is a measured-cost threshold with post-call overshoot, not a hard 50-dollar bound, and the evidence does not show that it owns the provider-pricing anomaly used to justify retaining it. The design must define the native counter's accounting basis and maximum overshoot, or name another authoritative billed-dollar owner; otherwise its central statement that a ten-times pricing anomaly is capped at 50 dollars is false and the backstop does not satisfy the stated escape clause.

EVIDENCE: metasystem/plans/spend-cap-retirement-design.md:161-162 says the flag “caps a 10× pricing anomaly at $50 per job.” Four raw results crossed the existing 5-dollar threshold and terminated at 5.047039, 5.150365, 5.21833, and 5.342624 dollars, proving that the threshold is checked only around indivisible paid work and can overshoot. Those same results label their accounting basis as costBasis "list", not authoritative provider billing. A sufficiently expensive anomalous API call can therefore cross 50 dollars before the CLI can stop another call, while a provider billing anomaly not reflected in list cost may not move the counter at all.

## SCR-R1-OWNER-003 — medium, material=True

CLAIM: The wall-clock inventory assigns the wrong owner and omits accepted liveness residuals. Retaining a native dollar backstop is directionally safer than the clean kill, because it still bounds the primary Claude process when the time enforcers are unavailable; however, the design must name that purpose and cannot claim that pricing anomaly is the only refutation.

EVIDENCE: metasystem/plans/spend-cap-retirement-design.md:57-63 attributes the hard kill to the dispatch reap ladder. Shipped metasystem/scripts/agents/adapters/runtime-common.sh:206-305 has a second, inner owner: the detached adapter supervisor checks capDeadline and kills its own group. metasystem/internal/supervise/reaper.go:126-156 explicitly refuses a live over-cap custodian. Most decisively, metasystem/records/misc/f4-orphan-window-design.md:108-114 states that a live-but-wedged custodian has “NO hard bound”; when its outer dispatcher is also unavailable, only the native Claude limit remains. For host turns, metasystem/internal/missionrunner/host.go:421-450 places turn-cap enforcement in the mission runner, not the host process. These are real liveness-failure scenarios omitted by the claim at design lines 19-24 and by the proposed comment at lines 153-156.

## SCR-R1-ENVELOPE-004 — high, material=True

CLAIM: The tool envelope does not justify the categorical claim that a write-capable delegate cannot fan out paid native work. Bash can launch another Claude session, including the CLI's ordinary background mode; a detached or provider-managed child neither inherits the parent's max-budget argument nor necessarily remains in its process group. Census visibility is observation, not a spend bound. The conditional mandate therefore needs a verified refusal/containment rule or a real spend owner for this path before implementation.

EVIDENCE: metasystem/internal/adapter/claude.go:217 includes Bash in the full write envelope. metasystem/plans/spend-cap-retirement-design.md:90-92 nevertheless says the delegate “cannot fan out native subagents at all.” Its scenario table at line 105 admits a deliberately detached child but assigns the row the same wall-clock bound and calls census exclusion sufficient, even while acknowledging that the child never inherits --max-budget-usd. Locally, Claude Code 2.1.252 advertises `--bg` as starting a background session and returning immediately, and `claude agents` as its management surface. Goal admission sees metasystem dispatch records, not a direct Bash-launched provider session. Thus the proposed 50-dollar parent flag does not own this exposure.

## Critic-declared gaps

- The raw 8.19-dollar and 10.01-dollar ledger-attention results are not present in this worktree; only their durable goal record is available. I did not silently treat their unknown elapsed times as 40 minutes.
- The repository contains no authoritative provider billing source or proof that Claude Code's list-cost counter tracks a provider pricing anomaly. Local CLI help and result artifacts establish only the native threshold and reported list-cost behavior.
- I did not execute `claude --bg` from inside another paid Claude session because doing so would cross an external spend boundary. Whether nested-launch protection refuses that exact command, and whether a permitted background session remains in the recorded process group, remains unproved and must not be assumed.
