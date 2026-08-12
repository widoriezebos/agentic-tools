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
