# Debugger Recovery

## Connection Failures

Check, in order: target is alive with JDWP enabled; host/port and bind scope; source/artifact parity; endpoint reachability; another debugger attachment; proxy/client timeout alignment.

Retry a not-ready listener once after a short wait. Repeated attach failures require lifecycle recovery, not another identical attach.

## Dirty or Stale State

- A shutdown connection manager or failed dirty-state reset requires an authorized proxy restart and target relaunch.
- A breakpoint resolving into an unrelated method requires rebuild and relaunch.
- Stale breakpoint events require cursor baselining; clear only when backlog is irrecoverably noisy.
- A target that appears hung may have suspended threads. List them and resume all.

## Expression Failures

Try a primitive/boolean expression, then an explicit cast, then direct locals/targeted child variables. Do not repeat the same failing evaluation more than twice.

## Timeout Layers

Debugger wait timeout, MCP adapter/client deadline, target command timeout, and product timeout are different clocks. Fixed early returns suggest adapter mismatch. Shorten cursor polling or increase the correct client deadline; do not label it a product defect without a low-pause run.
