Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-09

# Follow-up 2 to slice 1: the fourth seam, answered; finish the slice

Round 2 built the slice (40 files, Go proofs green, fast gate green) and
stopped at one seam: `metasystem/scripts/agents/go-build.sh` stamps
`dev-<short commit>-dirty` (line 47 uses `rev-parse --short`), while
section 10 of the design and your new `enginebuild.StampCommit` require a
40-character commit. Right call to stop. The decision:

## D5: the build script stamps the full commit

Change `go-build.sh` line 47 to `git rev-parse HEAD` so both the clean
stamp and the `dev-<commit>-dirty` stamp carry the 40-character id. The
script is an agent script under this goal's reach and the change is part
of this slice's diff; the parser's closed grammar stands. Then check every
consumer of the stamp with the longer value: the displays in
`metasystem/internal/up/up.go` (around lines 550 to 630) and
`metasystem/internal/steward/runner.go` (around 719, 741, 835) print it as
is, which is acceptable; the dispatch preflight that refuses "engine commit
X is older than checkout commit Y" resolves the stamp through git, which
accepts either length, but verify it by reading that code path and say
where it is; any fixture that asserted the short form is updated. The
`supervise status` output keeps reporting `BuildStamp` unchanged.

## Finish

- Rebuild with the changed script and run the real-binary shell scenario
  `abandoned-with-a-reason` in `metasystem/scripts/agents/goal-cli-fixtures.sh`
  to its end.
- Run the gate the slice-1 brief names, with private caches under `/tmp`
  as you did for the fast gate. If a bed is denied its temp directory or
  process visibility, say exactly which and why; the orchestrator runs it
  outside the sandbox before the landing.
- Your return lists every file touched (including the script), every
  fixture with its untouched-tree failure point and its pass now, the
  census result, and the four seams with how each was resolved.
