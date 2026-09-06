Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, follow-up round two of chain rgr-build1 under goal race-gate-red-on-main, tier 3, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

Fold critic round one of chain rgr-build1. The critic's return is
metasystem/artifacts/agents/rgr-critic1/rounds/1/return.json; its
findings bind as stated there. Fold each finding by id. Nothing else
changes: the Store copies, the hook capture, the register exclusion and
the sixty-minute ceiling stand as reviewed.

# The folds, by id

- F-1 (material): the new comment above the race run in
  metasystem/scripts/agents/go-gate.sh says no individual test exceeds
  12 seconds under the race detector. That figure came from the goal
  package alone; the mission-runner package's slowest tests take 19
  seconds standalone under the race detector. Correct the number to 19
  seconds, or state the bound as "under twenty seconds". Keep the
  argument the sentence supports: the cost is a serial sum, not one
  giant test.
- F-2 (not material, folded because the same comment is being edited):
  the sentence "Sixty minutes is above every measured package runtime
  under that contention and still ends a hung package" rests on
  measurements the reader cannot see. Replace it with what is known and
  in the file's own voice: under the gate's contention the slowest
  packages take about ten minutes on a large machine and have passed
  thirty on a small one; sixty minutes leaves room for a small machine
  and still ends a hung package. No machine names, dates, goal names or
  round references.
- F-3 (not material, folded as a direct consequence of this change):
  metasystem/scripts/agents/coverage-delta.sh lines 197 to 199 say its
  30-minute ceiling is "the same 30m ceiling the go gate carries". After
  this change that is false. That script runs one package at a time
  without the race detector, and its slowest package takes under ten
  minutes standalone, so its own thirty minutes stays right. Rewrite
  only that comment so it states its own reason: a hang bound for one
  package without the race detector, above the slowest package by a
  wide margin, and a timed-out probe would read as a package failure
  with a phantom partial percentage. Do not change the timeout value or
  any other line of that file.

# Workspace

Your existing worktree for chain rgr-build1, on top of your round-one
work.
May touch: metasystem/scripts/agents/go-gate.sh
May touch: metasystem/scripts/agents/coverage-delta.sh
Must not touch: anything else. The Go files from round one stay exactly
as they are; every test file stays byte-identical to main; nothing under
plans.

# Inputs

- metasystem/artifacts/agents/rgr-critic1/rounds/1/return.json: the
  findings, verbatim.
- metasystem/scripts/agents/go-gate.sh lines 523 to 531 in your
  worktree: the comment and the race run line from round one.
- metasystem/scripts/agents/coverage-delta.sh lines 195 to 201: the
  comment to rewrite and the line that must not change.

# Constraints

- Comments only. No timeout value changes, no code changes, no test
  changes.
- One round, at most 30 minutes of wall clock.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `bash -n ./scripts/agents/go-gate.sh` (expected: exit 0)
- `bash -n ./scripts/agents/coverage-delta.sh` (expected: exit 0)
- `grep -c 'timeout 60m' ./scripts/agents/go-gate.sh` (expected: 1)
- `grep -c 'timeout 30m' ./scripts/agents/coverage-delta.sh` (expected: 1)
- `grep -n '12 seconds' ./scripts/agents/go-gate.sh` (expected: no output)
- `grep -n 'same 30m ceiling' ./scripts/agents/coverage-delta.sh` (expected: no output)
- `git diff --stat main` (expected: the five round-one files plus
  coverage-delta.sh; the Go files' hunks unchanged from round one)

diffBoundary lists every path touched across the chain so far, each
starting with `metasystem/`.

# Acceptance Criteria

1. go-gate.sh's race-run comment states a per-test bound of 19 seconds
   or "under twenty seconds", and its ceiling sentence reads as F-2
   specifies; the race run line itself is unchanged from round one.
2. coverage-delta.sh's comment no longer claims to mirror the gate's
   ceiling and states its own reason; its timeout stays 30m.
3. No Go file differs from its round-one state; no test file differs
   from main.

# Gap Rule

stop and report a gap; never fill it silently.
