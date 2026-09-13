Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal wait-verb-returns-on-recorded-events)
Date: 2026-09-13

# Goal

Round 2 of the bed fix. Round 1 added the shared helper as a new package,
internal/testutil (file wait_binary.go), used only by other packages'
tests. The full Go engine gate's coverage ratchet refuses any package in
the module that produced no coverage and has neither a floor nor a
recorded exemption: `CheckCoverage` in metasystem/internal/audit/coverage.go
says "package ... is in the module but produced no coverage (no test
files?); add tests and a floor, or record an exemption". The delivery
proof runs that gate, so the candidate would be refused.

Make the candidate pass the ratchet without weakening it for any other
package. Either record an exemption for internal/testutil, with a one-line
rationale in the same style as the existing entries, in both
metasystem/scripts/agents/coverage-ratchet.json and
metasystem/scripts/agents/coverage-ratchet-linux.json; or give the
package a test and register a floor at its measured value on darwin and
on linux. Choose one and say why in riskiestPart. Keep round 1's helper
and test changes as they are. Do not commit.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json):
jobId, round, runtime, sessionId, model, evidence, gaps, mode, riskiestPart,
diffBoundary, whatWasDone. evidence carries `{command, observed, level}`
items replayable verbatim from the worktree's repository root: (1)
`git -C metasystem status --short`; (2) a run of the ratchet audit over
this package set proving internal/testutil raises no violation (for
example `go test -cover` of the packages round 1 touched plus
internal/testutil into a log, then `bin/metasystem audit coverage-ratchet
--baseline scripts/agents/coverage-ratchet.json --input <log> --packages
<list>` from metasystem/, with an engine built from the worktree), with
its output; (3) `( cd metasystem && scripts/agents/go-gate.sh --fast )`
with its last line.

Every path in your return (diffBoundary, files) is relative to the
repository root, so it starts with `metasystem/`.

# Constraints

Wall clock: 30 minutes.

# Gap Rule

stop and report a gap; never fill it silently.
