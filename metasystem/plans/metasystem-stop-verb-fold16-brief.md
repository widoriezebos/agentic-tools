Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 18: one unresolved path, and the slice is done (chain stopverb-build1)

Round 17's product fix is right and proven. On the round-17 tree,
outside the sandbox, the supervision bed is fully green (all eight
slice-1 scenarios, every pre-existing one but `rearm-launch-fails`,
which is red on main), and in mission-stop the ownerless sweep now
works: the orchestrator ran that scenario's child directly and the verb
printed

    repo-watcher pid 74301: stopped (TERM)
    job-reaper pid 74311: stopped (TERM)
    stopped <checkout>; start again: metasystem arm --repo <checkout>

with exit status 0. So decision D63's sweep and D64's footer both hold.

The scenario nevertheless fails, on one thing only.

# Fact, from the orchestrator's run

`run_complete_stop` builds its expected closing line from the scenario's
own `$repo` variable, which is the unresolved `/var/folders/...` form,
while the engine prints the physical `/private/var/folders/...` path it
resolved. The status was 0 and no `NOT STOPPED` line was printed, so the
comparison of the last line is the only failing condition. This is the
same platform symlink that broke three scenarios in the supervision bed
and one in the goal bed.

# Decisions (the orchestrator's; decided, not open)

D67. Every path this scenario compares against engine output is resolved
once, at the top, with the bed's own idiom (`cd … && pwd -P`), and the
resolved value is what the expectations use. Check the whole scenario for
other comparisons built from unresolved paths rather than fixing only the
line that failed: this is the third bed to trip on it.

D68. Nothing else changes. If mission-stop then passes, the slice is
done and its next stop is the closing critique.

# Verification

Reported at evidence level ran: `scripts/agents/go-gate.sh --fast` and
`go test -count=1` over the boundary. Name what you expect from the
mission bed; the orchestrator runs all four beds outside your sandbox
and reports.

# Constraints

Wall-clock budget: 45 minutes; this is one resolution and its
consequences. This is the goal's twentieth and last attempt under the
standing budget, so nothing else may be attempted in it. Return per the
implementer schema with the cumulative diff boundary listed. Gap rule:
stop and report a gap; never fill it silently.
