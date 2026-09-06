Working Mode: implement
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-hook-health-cost)
Date: 2026-09-06

# Goal

Round 2 of goal stop-hook-health-cost (chain shhc-build1-20260906). The
code review (job shhc-cc1-20260906, return at
metasystem/artifacts/agents/shhc-cc1-20260906/rounds/1/return.json)
found six material defects, all accepted; the seat's own replay of the
health fixture suite outside the sandbox found a seventh. The
dispositions are in
metasystem/plans/dispositions/stop-hook-health-cost-code-critique-r1.md.
Fix the seven in place, change nothing else. Your round-one brief
(metasystem/plans/stop-hook-health-cost-build-brief.md) still governs
everything it says; where this brief differs, this brief wins.

# Workspace

Your existing worktree, uncommitted as you left it. Do not stage or
commit; the seat lands the chain.

# The seven fixes

## SHC-01 — a cache write failure never shrinks the number

In metasystem/internal/spend/transcript.go and
metasystem/internal/spend/measure.go: when a cursor file or the job
cache cannot be written (or the write is not provably durable), the
measurement continues with what it computed in memory and the ledger
is exactly what a full parse would give; the failure is recorded as a
non-fatal note (a counter or a detail string in the seat summary, not
an unreadable entry, not an error). Fault-injection tests: an
unwritable cache directory yields the same ledger as a writable one.

## SHC-02 — only settled outcomes enter the job cache

In measure.go: a terminal record enters the cache only when its
measurement provenance is reported or derived; a pending outcome (the
process group still alive or unprovable) is never cached and is re-read
on the next measurement. Test: a record whose first read is pending is
read again and counted once it settles.

## SHC-03 — the warm measurement is a real Stop

In metasystem/internal/steward/health_cost_test.go: between the cold
and warm calls append one assistant line to at least one member
transcript, so the warm path is cursor read, merge and cache write; the
synthetic job records are about ten kilobytes each, the size of a real
one. The warm bound stays under one second.

## SHC-04 — cache writes are cheap

In metasystem/internal/spend/cache.go use the volatile writer (no
directory-chain sync) for cursor and job cache files, since a torn or
missing cache degrades to a full parse by SHC-01; and on the
foreign-grown branch write nothing at all.

## SHC-05 — the cursor directory is pruned

In transcript.go: after a measurement, cursor files not visited in that
measurement are removed, the way the job cache prunes unseen entries.
Test: a cursor for a deleted transcript disappears on the next
measurement.

## SHC-06 — the partial trailing line is certified

In metasystem/internal/spend/measure_test.go the byte-identity test
adds a transcript that ends without a newline and is then grown across
the line boundary; the incremental ledger equals the full parse.

## SHC-09 — a health read never blocks on a writer's lock

In metasystem/internal/steward/component_evidence.go
`loadComponentEvidenceForHealth` takes the shared lock non-blocking and
retries with short sleeps for at most about 200 milliseconds in total;
when it still cannot take the lock it returns a typed busy error, and
every role that reads component evidence reports unknown with the
reason "component evidence for <component> is busy (a writer holds its
lock)" and its usual remedy. Nothing else about the lock protocol
changes: writers keep the exclusive lock as today. Test: with the
exclusive lock held by the test, the health read returns the busy
error within one second and the role is unknown; with the lock free,
behaviour is unchanged. Say in the return whether the same bounded wait
belongs on the health observation lock in health.go (do not change it
in this round; state the case).

# Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l .` (empty);
`go test ./internal/spend/ ./internal/steward/ ./internal/mission/ -count=1` green
with the coverage floors respected;
`bash scripts/agents/health-fixtures.sh` as far as the sandbox allows
(the seat replays it outside).

# Constraints

Wall-clock budget: 60 minutes; return before it ends even if something
is red, naming it. Declare the boundary as every file that differs from
main. Gap rule: stop and report a gap with your proposed contract
written out.

# Expected Return

Version-2 implementer JSON as before, with the cold and warm times.

# Gap Rule

stop and report a gap; never fill it silently.
