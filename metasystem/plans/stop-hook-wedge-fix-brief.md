Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal stop-hook-wedge-on-enrollment-drift)
Date: 2026-09-04

# Correction round on chain shw-build1 (your reviewed tree 8b8cbf02)

The Fable review (shw-build1-cc1) found nothing material in the
sandbox, but the orchestrator ran the process-owning fixture suite
outside the sandbox, as the orchestration contract requires: on your
tree `scripts/agents/supervision-hook-fixtures.sh` is RED at its first
leg ("template attended-human authorization did not end quietly": the
hook answered a block whose reason carries the open-work sentence),
while the same suite on main is GREEN on the same machine. The
review's SHW-02 names the mechanism: the deadline parent now runs
engine work before launching the worker (two JSON reads, a slug, a git
toplevel query) and again after an overrun (record read, lock, atomic
write), all inside the provider's five-second cap; on a loaded machine
the worker's budget shrinks, the hook overruns, and the overrun is
recorded as a first occurrence and blocks. Also SHW-05: the record
lock is non-blocking, so an overlapping writer turns a first
occurrence into a surface.

# The change

1. The deadline parent launches the worker FIRST, exactly as main did
   (stdin staged to the payload file is fine, but nothing else runs
   before the launch); it resolves the session, repository and record
   path while the worker runs, using only cheap shell (the payload is
   JSON; read session_id and cwd with the engine only if that finishes
   in the wait loop, otherwise fall back to a shell parse); the overrun
   path must complete well under one second: one `report stop-block`
   call with the record, no other engine calls. Measure: the fixture's
   deadline leg must pass under five seconds on this machine (the
   suite runs with METASYSTEM_FIXTURE_CAP_SCALE unset).
2. The record lock waits briefly (a bounded blocking flock, at most one
   second) instead of failing immediately, so an overlapping writer
   cannot turn a first occurrence into a surface (SHW-05).
3. Quote the canonical engine path everywhere it is expanded (SHW-04).
4. SHW-01 and SHW-03 are recorded for the design owner; do not change
   them in this round.

# Gate

`bash scripts/agents/supervision-hook-fixtures.sh` green on this
machine outside any sandbox (the orchestrator will rerun it; if your
sandbox cannot run it, say so and run every leg it can); `bash -n` on
the hook; `go test ./internal/report/ ./cmd/metasystem/ -run 'Stop|Report' -count=1`
green. Declare the boundary as every file that differs from main.

# Constraints

Wall-clock budget: 40 minutes; return before it ends even if something
is red, naming it. Gap rule: stop and report a gap with your proposed
contract written out.
