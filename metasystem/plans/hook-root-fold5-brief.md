Working Mode: design
Orchestrator Identity: m1 (lineage main-1788594343-3833-fb64b9, dispatch delegate under goal supervision-hook-wrong-root)
Date: 2026-09-05

# Goal

Revision 5, four-finding fold of
metasystem/plans/supervision-hook-root-design.md (revision 4 landed at
b2165cf7e). Register: metasystem/records/misc/hook-root-critique-r4.md,
landed. Fold each finding BY ID; the register carries the critic's exact
claims and evidence, and they bind.

# The folds, by id

- SHR-R4-DEADLINE-PARENT-01: the Stop-deadline parent block in
  metasystem/scripts/agents/supervision-hook.sh is a second root and engine
  owner the design does not govern. Fold it into Decision 1 and Decision 4:
  keep the certified worker-first ordering; the parent resolves the same
  script-derived, scrubbed, mapped installation while the worker runs;
  selects its canonical engine from that mapped installation; derives the
  refusal-record root through the state-root verb, never payload cwd. Add
  the fixtures the critic names (normal completion and timeout for an
  engine-less worktree, under git steering and under METASYSTEM_BIN) to
  Decision 3. State plainly that turn-verdict-hardening slice 1b is unsafe
  until this lands.
- SHR-R4-FAIL-CLOSED-REGRESSION-01: the design's literal replacement block
  emits a systemMessage where the shipped hook emits decision:block for a
  missing engine (supervision-hook.sh:4-6,18-21,224-233; fixture
  supervision-hook-fixtures.sh:137-157). Reconcile Decision 2 with the
  shipped fail-closed contract: a missing engine and an old engine lacking
  the verb both BLOCK, and the failure map says so; the existing
  missing-engine fixture is kept and an old-engine fixture is added.
- SHR-R4-UP-GIT-STEERING-01: up's repository-scope query
  (metasystem/cmd/metasystem/up.go, upRepositoryScope) runs git with the
  inherited environment, so a mapped turn is steerable again inside up.
  Withdraw the "out of scope, never selects the state world" claim; the
  scope query runs under the same scrubbed environment as the mapper, or
  the design says exactly why the census scope may differ from the state
  world and what pins it.
- SHR-R4-COPIED-HOOK-OVERRIDE-01: with METASYSTEM_BIN set, the executable
  check never requires an engine at the candidate, so a copied hook inside
  a governed repository adopts that repository's world by pathname — the
  round-two hole reopened. The candidate must carry an engine at
  <candidate>/bin/metasystem for the world to be governed, override or
  not; the override may replace which engine runs but not waive the
  candidate's own engine as the installation's evidence. Re-pin fixture
  case 6 to this.

Consistency pass over Decisions 1-4 and the failure map; self-grade;
reject condition restated. Bump the revision header to 5 with today's date.

# Constraints

Wall-clock budget: 20 minutes. The four folds only; no other decision
moves.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/supervision-hook-root-design.md (that one file).

# Gap Rule

Stop and report a gap; never fill it silently.
