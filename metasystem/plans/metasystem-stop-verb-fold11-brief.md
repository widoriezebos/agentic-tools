Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 12: one missing field, and the slice is green (chain stopverb-build1)

The orchestrator ran the supervision bed on the round-11 tree outside
the sandbox. FOUR of the five acceptance scenarios pass: seat-survives,
status-is-live, stop-fence and arm-again. census-lifecycle is green
again, so your localisation of the design artifact was right and the
regression is closed. Every other pre-existing scenario passes except
rearm-launch-fails, which is red on main and owned elsewhere.

One scenario remains, and it fails on a single missing field in the
product.

# Fact, from the orchestrator's run

stop-everything's status listing differs from the grammar by one token:

    expected: job stop-fixture-job running pid 33550 pgid 33550 role design-critic: running
    actual:   job stop-fixture-job running pid 33550 pgid 33550 role design-critic

Every other family's status line carries the state in the verdict
position after the colon (`run ...: running`, `steward-runner ...:
running`, `supervision-owner ...: running`). The job family omits it.
Section 6's same-list contract requires it: status prints one line per
thing "with the state in the verdict position".

# Decisions (the orchestrator's; decided, not open)

D46. The status renderer prints the verdict for job lines as it does for
every other family, so the job line ends `role <role>: <state>`. The
apparent redundancy with the status field earlier in the line is
deliberate and stays: that field is the record's own status, while the
verdict is what this listing says about the thing now, and for a live
job the two coincide. Do not remove either. Cover it with a package test
beside the existing line-grammar tests.

D47. Nothing else changes.

# Verification

Reported at evidence level ran, with the private caches earlier rounds
used: `scripts/agents/go-gate.sh --fast`; `go test -count=1` over every
package in the boundary; and the exact supervision-bed command, which
the orchestrator runs outside your sandbox. If your sandbox still cannot
run the bed, say what you expect and change no assertion to suit it.

# Constraints

Wall-clock budget: 60 minutes; this is one field and its test. Return
per the implementer schema with the cumulative diff boundary listed. Gap
rule: stop and report a gap; never fill it silently.
