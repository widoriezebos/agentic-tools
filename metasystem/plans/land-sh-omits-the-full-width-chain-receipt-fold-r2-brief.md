Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, follow-up round two of chain lsr-build1 under goal land-sh-omits-the-full-width-chain-receipt, tier 3, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

Fold critic round one of chain lsr-build1. The critic's return is
metasystem/artifacts/agents/lsr-critic1/rounds/1/return.json; it found
nothing material, so the lane stands as built. Three of its notes are
operator-facing refusals that print no reason, which the seat
communication rules forbid, and one is a missing fixture leg; fold
those four by id (F-1, F-2, F-4); F-3, F-5 and F-6 are noted and change
nothing.

# The folds, by id

- F-1: when the file named by --test-receipt is missing, unreadable, or
  not JSON with a `tree` field, the receipt step fails with only the
  step name. Make check_supplied_test_receipt in
  metasystem/scripts/agents/land.sh say why: "land refused: the receipt
  at <path> cannot be read as a landing receipt (<what failed: missing,
  unreadable, or no tree field>)".
- F-2: the three usage refusals (--test-receipt without --chain, with
  --tests, with --direct-fix tier-1) print the usage line only. Each
  prints one "land refused: ..." sentence first that names the
  combination and what to do instead, then the usage line, exit 2, in
  the style of the neighbouring --root-job/--tests refusal.
- F-4: the fixture scenario full-width-chain in
  metasystem/scripts/agents/land-fixtures.sh gains one leg: --tests and
  --test-receipt together is refused with the F-2 sentence before any
  work; add it as the first leg of that scenario and keep the other
  three.

# Workspace

Your existing worktree for chain lsr-build1, on top of round one.
May touch: metasystem/scripts/agents/land.sh
May touch: metasystem/scripts/agents/land-fixtures.sh
Must not touch: anything else.

# Constraints

- Bash 3.2 clean. No other behavior change. At most 30 minutes of wall
  clock. Run the land bed only if your sandbox allows it; the
  orchestrator runs it seat-side.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `bash -n ./scripts/agents/land.sh` and `bash -n ./scripts/agents/land-fixtures.sh` (expected: exit 0)
- `grep -n 'land refused' ./scripts/agents/land.sh` (expected: the three usage sentences and the receipt-unreadable sentence among the hits)
- `grep -c 'test-receipt' ./scripts/agents/land-fixtures.sh` (expected: at least one more than before)
- `git diff --stat HEAD` (expected: the two files)

# Acceptance Criteria

1. Every refusal the new lane adds prints a "land refused:" sentence a
   person can act on before the usage line.
2. The full-width-chain scenario has four legs and the bed's count line
   still reads seven scenarios.

# Gap Rule

stop and report a gap; never fill it silently.
