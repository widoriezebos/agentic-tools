Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 7: make the four written scenarios green, and one boundary correction (chain stopverb-build1)

The orchestrator ran the supervision bed on the round-6 tree outside the
sandbox. The good news first: in every scenario that produced output,
the stop report matched the section 6 grammar line for line, the
narrator line included, the owner tag, both `already gone (by the
owner)` lines and the start-again line. The verb works. All four
scenarios nevertheless failed, for two small reasons, and this round
fixes them.

# Facts, from the orchestrator's run

- seat-survives, status-is-live and arm-again fail on one difference
  only: the expected file says `/var/folders/...` while the engine
  prints `/private/var/folders/...`. On this platform `/var` is a
  symlink to `/private/var`, and the engine resolves the checkout, so
  the fixture's expectation must resolve it too. Every other line
  matched exactly.
- stop-everything never reached its assertions. Its held fake job is
  dispatched through metasystem/scripts/agents/dispatch.sh, which
  answered `REFUSED-REQUEST: the legacy dispatch authority grammar was
  removed; use metasystem delegate`. This is not the internal-guard
  variable; that grammar is gone.
- The steward repair the stop-fence scenario needs is NOT in
  metasystem/cmd/metasystem/supervise_watcherpass.go, which only writes
  the heartbeat and censuses. `steward.RepairEnrolledRunner` is called
  from metasystem/cmd/metasystem/supervise_component.go at line 202 (the
  watcher component's pass) and from metasystem/internal/up/up.go at
  line 701 (EnsureRunner). Both are inside this chain's boundary, so
  round 6's boundary gap does not stand.

# Decisions (the orchestrator's; decided, not open)

D26. Every scenario resolves the checkout path it expects with the
bed's own idiom, `cd "$repo" && pwd -P`, before composing its expected
output, so a platform symlink cannot defeat a grammar comparison.

D27. stop-everything dispatches its held fake job through the supported
path, `bin/metasystem delegate`, the way the dispatch bed's fixtures
do; the direct legacy script invocation goes away.

D28. stop-fence asserts the fenced steward where the repair actually
runs: the watcher component's pass in
metasystem/cmd/metasystem/supervise_component.go and EnsureRunner in
metasystem/internal/up/up.go, both in boundary. Drive those, not
`supervise watcher-pass`, which owns no repair; assert that nothing is
launched under a closed fence and that the fenced status is reported.
Leave metasystem/cmd/metasystem/supervise_watcherpass.go untouched.

D29. Then the whole verification below green. Nothing else changes: no
new scenario beyond the five of the cut, no product change that a
failing scenario did not name.

# Verification

Reported at evidence level ran, with the private caches earlier rounds
used: `scripts/agents/go-gate.sh --fast`; `go test -count=1` over every
package in the boundary; `scripts/agents/dispatch-fixtures.sh` and
`scripts/agents/goal-cli-fixtures.sh` fully green; and the five
scenarios named with the exact supervision-bed command, which the
orchestrator runs outside your sandbox. If your sandbox cannot execute
that bed, say so and name what you expect; do not weaken a scenario to
make it pass.

# Constraints

Wall-clock budget: 120 minutes. This is the last round of this chain:
what is not green at the end is named precisely in your return. Return
per the implementer schema with the cumulative diff boundary listed.
Gap rule: stop and report a gap; never fill it silently.
