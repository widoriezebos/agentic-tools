Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 9: three fixture bugs stand between this slice and green (chain stopverb-build1)

The orchestrator ran the supervision bed on the round-8 tree outside the
sandbox. seat-survives and status-is-live now PASS, which means the
durable stop is proven where it matters most: after a stop, the seat's
own session survives and its Stop hook returns allow with the checkout
reported stopped, creating nothing. That is the exact resurrection
another seat reproduced live this morning, where a session's Stop hook
re-armed supervision 49 seconds after a shutdown, and it is now blocked.

Three scenarios still fail. All three are bugs in the fixtures, not in
the product, and each has one cause.

# Facts, from the orchestrator's run

- stop-everything: the delegate dispatch it now uses is refused with
  `declared outputs line 1: path must begin metasystem/`. Its declared
  outputs manifest names a bare path.
- stop-fence: it waits for `supervise component --component watcher` to
  COMPLETE, and times out at the scaled 96 seconds. That command runs the
  watcher's standing loop; it does not exit. The repair the scenario
  wants to prove refused lives inside that loop.
- arm-again: the stop and the arm both succeeded, with the arm reporting
  `armed (runner pid ...) ... up outcome=recovery-started ... armed
  <repo> generation 2`. The final join then failed with
  `component=session-identity outcome=failed detail="explicit session pid
  is not the caller or one of its ancestors"`: the join passes a session
  pid that is not in its own ancestry, unlike the scenario's first
  arming, which verified.

# Decisions (the orchestrator's; decided, not open)

D34. stop-everything's declared outputs manifest names repository-relative
paths beginning `metasystem/`, the spelling the dispatcher requires.

D35. stop-fence never waits for a standing component to exit. Prove the
fenced repair by its effect, not by the command's completion: run the
watcher component bounded in the background, assert that no runner was
created and no tagged process appeared while it ran, then end it; and
keep the package-level assertion that the repair path reports FENCED
under a closed fence. The scenario's other creation paths stay as they
are.

D36. arm-again's join runs `up` with a session identity that is the
caller or one of its ancestors, the way the scenario's own first arming
does.

D37. Nothing else changes. No product change unless a failing assertion
names one, and if one does, say so explicitly in the return rather than
folding it in silently.

# Verification

Reported at evidence level ran, with the private caches earlier rounds
used: `scripts/agents/go-gate.sh --fast`; `go test -count=1` over every
package in the boundary; and the five scenarios named with the exact
supervision-bed command, which the orchestrator runs outside your
sandbox. Your sandbox cannot run the beds: their scratch installations
are not recognised as git repositories there. Say what you expect and do
not weaken a scenario to make it pass.

# Constraints

Wall-clock budget: 120 minutes. The goal's budget was raised, so a
further round is available if something genuine blocks you: stop and
name it rather than inventing. Return per the implementer schema with
the cumulative diff boundary listed. Gap rule: stop and report a gap;
never fill it silently.
