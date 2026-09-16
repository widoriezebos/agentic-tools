# waits-run-outside-model-contexts

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="severity 2: waste of tokens and budget, no record is corrupted; novelty 2: extends the existing metasystem wait verb with event kinds the seats already wait on by hand; exposure 3: every seat, lane and reader waits; accumulation 1: one verb, one contract rule, one template change"
- Tier: 2
- Intent: Token audit 2026-09-16 (memory token-usage-audit-2026-09-16, S19/usage-today.sh): Agent-tool subagents get a 5-minute prompt-cache TTL (the main session has 1 hour). A landing lane or reader that waits 10 minutes in one tool call comes back cold and rewrites its whole 85-160K context at 1.25x; m1e lanes and readers did this 11 to 22 times each, about 12 percent of the day (313M weighted units by 18Z). Wido 2026-09-16: high priority under the efficiency program. DONE: (1) `metasystem wait` blocks on the events seats wait on by hand today: a proof attempt verdict, a landing on a goal (exists), the testrun lock release, and the appearance of a delegate result file (design run, critique run, Codex return), each with a bound and one result line, no model polling. (2) The seat contract (AGENTS.md and the wow routes) states the rule: a subagent never waits longer than 240 s in one tool call; any longer wait runs as `metasystem wait` in a harness background task of the main session, which is woken on completion; the landing-lane and reader prompt templates in the repository carry the rule. (3) Go tests with an injected clock and fake event sources prove each event kind and the bound; a mutation that sleeps or polls on wall time turns red. (4) Evidence after one day: the spend report (goal spend-fence-reports-tokens-per-model-and-cause) shows subagent cold-cache rewrites under 2 percent of the weighted day.
- Origin: human
- Next step: Opened in the name of Wido by seat m1e from the enrolled pane on his word 2026-09-16 20:25 CEST. Seat brief, one Codex critique of the brief, Codex gpt-5.6-sol builds in units of at most 300 lines, Opus reads, the landing lane lands. Until it lands, the seat rule applies by prompt: no subagent wait call over 240 s (lane-5 prompt template updated 18:20Z).
- OpenedAt: 2026-09-16T18:20:23Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-16T18:20:23Z KKVQ4CNFW8ZTHWJH3CAWR6RYD3-m1e-c6925449 open actor=human:Wido targets=waits-run-outside-model-contexts
Integrity: sha256=c39a59b16dd5a7882e7928855cc89780c0567708bf4037682f70899a78a04464
