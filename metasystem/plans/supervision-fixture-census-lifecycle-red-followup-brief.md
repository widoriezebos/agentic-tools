Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal supervision-fixture-census-lifecycle-red)
Date: 2026-09-04

# Follow-up: the failing lines

Seat-side, from `scripts/agents/supervision-fixtures.sh` on m2 at 20:00Z:

census-lifecycle, right after "enumerate-filter-resolve census fixture passed":
`{"outcome":"REFUSED-REQUEST","headline":"refused","detail":"design-critic dispatch requires --outputs <file> and --design <file>"}`
The scenario dispatches a design critic; since the design-gate-at-dispatch landing every design-critic dispatch must carry `--outputs <file>` and `--design <file>`. Give the scenario's design-critic dispatch those two files (a declared-outputs file and a design file that fit the leg; the dispatch fixture's `fixture_declared_outputs` and its design brief show the shape) and read on for any later leg in the scenario that would fail for the same reason.

idle-hook, the lines before its failure:
supervision fixture scenario passed: slow-census
turn-end hook emitted no surfaced message
{"decision":"block","reason":"Work named in a plan is unblocked and nothing is in flight. Do it now, or record in the plan why it is blocked or waiting on the human. This refusal does not repeat for the same work.\n\nMetasystem Stop deadline expired before a safe turn verdict; stopping is refused."}
supervision fixture scenario failed: idle-hook (rc=1); continuing
Replace the scenario's wall-clock waits with patience counted in observed events (census passes or hook invocations), keeping a silence-only failsafe, as the build brief says.

`bash -n` on the script. Every path in your return is relative to the repository root (starting with `metasystem/`). The orchestrator reruns the suite seat-side.
