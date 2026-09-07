Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 10: the last three, and one product ordering defect (chain stopverb-build1)

The orchestrator ran the supervision bed on the round-9 tree outside the
sandbox. seat-survives and status-is-live pass again. Each of round 9's
three fixes worked and uncovered the next layer, and one of those layers
is a real product defect rather than a fixture bug.

# Facts, from the orchestrator's run

- stop-everything: the outputs manifest now passes, and the dispatch is
  refused one argument later with `design path must begin metasystem/`.
- stop-fence: it now reaches the delegate refusal assertion and gets
  `dispatch refused: last census verdict is CENSUS-FAILED` where it
  expects `REFUSED-STOPPED`. That is the product's fault, not the
  fixture's: `require_fresh_census` in
  metasystem/scripts/agents/dispatch.sh runs before the Go claim path
  that reads the fence, and a stopped checkout has no live watcher, so
  its census verdict fails by construction. After a real stop every
  dispatch would blame the census instead of saying the checkout is
  stopped.
- arm-again: the join now runs from the announced live driver and reports
  `component=session-identity outcome=verified`, the owner, watcher and
  reaper verified at generation 2, and `up outcome=armed authority=writer`.
  The scenario then fails because it expects the word `verified` for the
  join itself. The design line it followed was wrong: `arm` takes no
  lease and makes no announcement, so the surviving main legitimately
  becomes the writer. The design is corrected in the same landing as
  this brief.

# Decisions (the orchestrator's; decided, not open)

D38. Every repository-relative argument the stop-everything dispatch
passes begins `metasystem/`, the design argument included. Check the
whole invocation rather than the one field the last refusal named.

D39. PRODUCT: the stopped refusal takes precedence over every other
gate. In metasystem/scripts/agents/dispatch.sh the dispatch entry reads
the fence BEFORE `require_fresh_census` and refuses `REFUSED-STOPPED`
with section 9's two lines; the Go preflight keeps its own read as
defence in depth. Section 9 of the design now states this precedence, and
the reasoning is in the fact above: a gate that fails only because the
checkout is stopped must never be the reason a caller is told. Add the
assertion to stop-fence and a package test at the seam you change.

D40. arm-again asserts what the design now says: the surviving main's
join reports the owner, watcher and reaper verified at the new
generation and becomes the writer.

D41. Nothing else changes. If another layer appears behind one of these,
name it in the return rather than chasing it silently.

# Verification

Reported at evidence level ran, with the private caches earlier rounds
used: `scripts/agents/go-gate.sh --fast`; `go test -count=1` over every
package in the boundary; and the five scenarios named with the exact
supervision-bed command, which the orchestrator runs outside your
sandbox, where the beds work.

# Constraints

Wall-clock budget: 120 minutes. Return per the implementer schema with
the cumulative diff boundary listed. Gap rule: stop and report a gap;
never fill it silently.
