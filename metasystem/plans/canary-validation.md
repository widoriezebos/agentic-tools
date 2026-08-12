# Canary validation: a cheap first verdict before the full suite

Requested by Wido 2026-08-12 ("implement a mechanism which is the canary
mechanism that does a cheap first test before running full-on tests…so we
need to have this mechanism implemented so that you know when to apply it
and how to apply it"). Not yet designed; this file is the requirement and
the evidence, so the design starts from fact.

## The problem, evidenced

The full suite costs ~13 minutes per run. On 2026-08-12 an intermittent
receipt-fixture investigation consumed roughly eight full runs where a
targeted fixture invocation (seconds) answered the same question each
iteration — the investigator had to hand-extract fixture blocks into
standalone scripts to get cheap probes. Every validation loop pays the same
tax: the suite is the only sanctioned verdict, so the cheapest question
costs the full price.

## The requirement

1. **A cheap first verdict**: an invokable subset of validation that runs
   in seconds-to-a-couple-of-minutes and catches the likely failures of a
   given change class before the full suite runs.
2. **Known WHEN**: the discipline that the canary runs first on every
   change, and the full suite runs as confirmation once the canary is
   green — plus the named exceptions (acceptance runs, close-outs, and
   anything the canary cannot represent).
3. **Known HOW**: the mechanism must be discoverable and mechanical — a
   flag or verb, not folklore. Candidate shape: validate-metasystem.sh
   grows `--section <name>` (its fixture sections become individually
   invokable) plus a curated `--canary <change-class>` mapping; the
   go-gate already serves as the code-level canary.

## Design constraints (from this repo's rulings)

- The canary must never be mistakable for the full verdict: its output
  names itself a canary, and the commit discipline still requires the full
  gate; suite-priced validation stays the acceptance bar.
- Section extraction must not fork fixture logic: sections run in place
  from the one suite file, or the mechanism refactors the suite into a
  section registry — that choice is the design's first decision.
- The design goes through docs/design/design-obligation-gate.md before
  implementation (it moves validation behavior).

## Design decision and implementation (2026-08-12, same session)

The first decision resolved by inventory instead of refactor: all fourteen
fixture drivers already exist as standalone-invokable scripts, so the
canary COMPOSES existing checks and forks no fixture logic.
`scripts/canary.sh <change-class>` maps classes (go, supervision,
dispatch, mission, lease, records, shell, docs) to the go gate plus the
relevant drivers; unknown classes are refused so a typo cannot pass
vacuously; the output names itself CANARY and non-authoritative in both
directions (red: "fix before spending the full suite"; green: "not a
verdict").

Measured: the mission-class canary runs in ~23s hot (gate cache warm) and
a few minutes cold, against ~13 minutes for the full suite.

### Default completion check

1. Contract met: a cheap first verdict exists, mechanical (`scripts/canary.sh`),
   with WHEN (before any full-suite spend; the discipline line in the
   header) and HOW (the class map) explicit in the tool itself.
2. Owner: `scripts/canary.sh` (plumbing, shell — composition only; every
   decision it invokes lives in the Go gate and the drivers it calls).
3. Verification run: refusal path exits 2; docs and shell classes green
   with real audit output; mission class green end to end at 23s.
4. On failure or bad input: unknown classes refuse with exit 2; any red
   check aborts immediately with the failing label named.
5. Unverified and stated: per-class fixture selection is judgment, not
   proof — a class map miss shows up as a full-suite failure the canary
   missed, and the map is edited where it happened.
