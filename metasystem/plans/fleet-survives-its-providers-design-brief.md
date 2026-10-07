# Design brief: fleet-survives-its-providers (plan goal 3)

Working Mode: Design
Opened 2026-10-07 in Wido's word. Read first: `plans/machinery-open-findings-plan-2026-10-06.md` findings 4 (provider limits), 7 (the unit boundary), 8 (restart bound) and 16 (host load), the mechanisms page (host state, the boundary event), and the "Learned 2026-10-07" section. SIZE: each unit at most 250 production lines, the goal at most 5 units; split before the first critique round if it does not fit.

## Intent

A provider limit is an environment condition with a reset time, recorded once fleet-wide, pausing every dependent budget clock; a session ends at the unit boundary with a handoff, never mid-step; restarts are bounded; and the number of goals building at once follows the measured host load.

## Must deliver (from the plan and from 10-07)

1. Host state as one record (load, cores, suite minutes against the unloaded baseline, provider marks), read fresh by every launcher.
2. One provider-limit mark per provider, in the lane's state; clocks that depend on it pause; the mark clears at the reset time or on the first success.
3. The minimal unit-boundary event the driver (goal 4) consumes; a session ends there with its handoff.
4. The restart bound: how many restarts in which window before the seat holds and asks.
5. **Load-aware work in progress (10-07):** the admission of a new build or suite follows measured host load, not a fixed count (today: three goals in parallel took the 18-core host to load 7 to 8 and the suite from about 8 to 22 to 33 minutes); full-suite runs serialize per host; the policy has the three values (`auto`: derive from load; a cap N; `person`). Measure: suite minutes per unit and units per hour, read from machinery-measures-its-own-process.

## Not in this goal

The driver itself (goal 4). The flake register (machinery-housekeeping). Budget derivation (goal-budget-follows-its-plan).

## Shape

As the accepted designs: header, Wido's binding words, what it delivers, Decisions with code sites read on the current tree, Units table with production lines and areas, estimates, one public-verb test per unit failing under its mutation, readers; the five questions for every new function, record and act; the four defect classes.
