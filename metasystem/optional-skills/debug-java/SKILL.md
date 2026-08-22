---
name: debug-java
description: Diagnose live Java runtime behavior through JDWP and debugger tools. Use for runtime bugs, wrong branches or state, concurrency/deadlocks, variable or thread inspection, breakpoints and stepping, timeout attribution, or proving an end-to-end causal chain. Do not use when static code/tests already prove the cause or when the task only asks to implement a known fix.
---

# Debug Java

Use the repository's configured JDWP tools, including Descartes when available. Debugging is an evidence cycle: combine this skill with `take-a-step-back` when the investigation is repeated, expensive, or unclear.

## Ownership and Safety

- Read source before setting breakpoints.
- Treat proxy and existing debugger-session lifecycle as user/shared state. Start or stop only what the task authorizes.
- Inventory active breakpoints, watches, and suspended threads; remove only what you created.
- Never leave threads suspended. Stop a session only if you started it.
- Launch agent-controlled targets through a non-TTY managed launcher; never use a foreground PTY target.

## Preflight

1. Freeze symptom, reproduction, exact SHA/local changes, configuration, timeout budget, and hypothesis.
2. Verify launched classes/JAR are at least as new as relevant source. An unrelated `resolve_line` result is stale-artifact evidence: rebuild and relaunch.
3. Check target/process, JDWP endpoint, proxy, and debugger session status.
4. Detect embedded vs proxy mode. Embedded can trigger asynchronously; proxy mode needs an external HTTP/manual trigger.
5. Fetch breakpoint events and store `latest_sequence` before waiting for new hits.

Use `scripts/preflight.sh` for local process/artifact checks when configured.

## Hypothesis and Breakpoints

Write two to four falsifiable hypotheses with class/method, decision variable, confirming value, and refuting value. Prefer the first owning decision where state can diverge.

Resolve uncertain lines before setting breakpoints. Start with at most five high-signal breakpoints. Conditional expressions may be metadata rather than server-side filters; confirm conditions at hit time. Disable a breakpoint after two low-signal hits.

## Async-Safe Loop

1. Trigger asynchronously. Never use synchronous JShell for code that can hit a breakpoint; it can deadlock the MCP channel.
2. Wait for `debugger.breakpoint_hit` with `since_sequence=<latest_sequence>` and carry the returned cursor forward. Do not routinely clear the debugger event queue.
3. Capture filtered stack trace, then frame-zero locals, then only targeted child values or expressions.
4. Record a decision snapshot: request/thread identity, class/method/line, branch inputs, budgets/counters, selected outcome, and hypothesis result.
5. Step only when one targeted transition remains unclear.
6. Resume all threads promptly; repeat only if the next hit can establish a novel causal fact.

## Prove Causality

Do not stop at the first anomaly. Prove:

1. first divergent state;
2. downstream owner/gate consumes it;
3. gate outcome produces the reported symptom.

For timeouts, distinguish debugger-influenced delay from product behavior with a low-pause confirmation and actual budget values.

## Recovery and Completion

On attach/reconnect/dirty-state failures, follow [references/recovery.md](references/recovery.md); do not loop blindly. Clean up shared state and report exact runtime facts, causal chain, source location, regression-test target, and proposed owning fix. Diagnose only unless the user also asked for implementation.
