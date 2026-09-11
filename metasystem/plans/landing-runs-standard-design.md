# Per-landing verification is standard; deep runs at cadence

Owner: the testing contract (`testing.json`) and its policy package
(`internal/testpolicy`). Ruling: R-3. Decided by Wido on 2026-09-10 as option
B of slice 1 in `plans/suite-speed-plan.md`, with option C's move.

## The law

Per-landing depth is a property of the change, never of the goal.

- A landing runs deep when the change is a protected policy change, touches
  a path no surface owns, affects a surface whose declared risk raise lifts a
  dimension to 2 or worse (or declares non-revert reversibility, delayed
  detection or unbounded recovery), or when the caller asks for `--mode deep`.
- Every other landing runs the canary set plus the affected surfaces'
  standard lists and the providers of their critical obligations.
- The goal's four risk answers never choose depth. They scale the landing's
  behavior-surface weight: `gate weight-add --goal <id>` multiplies the
  measured weight by the goal's highest answer (1 to 3), so riskier goals
  bring the weight-triggered deep cadence run sooner. A goal whose risk
  cannot be read weighs unscaled; weight bookkeeping never refuses a landing.
- The two whole-package coverage groups and the three big process sections
  (`section/dispatcher-adapter-and-mission-runner-fixtures`,
  `section/adoption-fixtures`, `section/supervision-and-census-fixtures`) sit
  in no surface's deep list. They run at cadence, and where a surface names
  one as the provider of a critical obligation it still runs in that
  surface's standard set.

## What it changes in code

- `internal/testpolicy/select.go`: the depth assessment is computed from the
  project's baseline and the affected surfaces alone; the goal's answers are
  named in the plan's risk reasons so a reader sees where they went.
- `internal/testpolicy/risk.go`: `requiresDeep` documents the law.
- `testing.json`: nine surfaces lose the five cadence-only groups from their
  deep lists; the `cadence` list is unchanged, no group is deleted.
- `internal/gaterun/weight.go`: `WeightAddScaled`; `cmd/metasystem/gate_weight.go`
  reads the goal's risk; `scripts/agents/commit.sh` passes the landing's goal.

## What stays

- A changed contract runs its callers (R-18): a landing that touches
  `testing.json` or `internal/testpolicy` still plans deep and runs every
  `testing-policy-*` provider.
- Runs continue and collect (R-16); cadence selects the whole battery.
- The contract's `projectRisk` remains a declared baseline a project may
  raise to make every landing deep.

## Proof

- `test plan --root . --goal <ordinary goal> --mode auto --purpose delivery`
  reports `requiredMode: standard` with about eleven groups.
- The same probe with `--purpose cadence` lists every cadence group.
- A diff touching `internal/testpolicy` still plans deep.
- `TestDepthIsDecidedByTheChangeNotTheGoal`, the updated
  `TestModesDoNotLowerRequiredRisk`, the updated contract pins, and
  `TestWeightAddScalesByGoalRisk`.
