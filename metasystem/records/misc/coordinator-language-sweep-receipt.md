# Coordinator-language sweep receipt — `plans/` rows are applied separately by the seat under custody

This implementer section covers only `docs/`, `AGENTS.md`, `wow.md`, and
`memory/rulings.md`. It applies the in-scope AUTHORITY verdicts from
`plans/coordinator-language-inventory.md` while preserving every in-scope
CALLSIGN and HISTORICAL row verbatim.

## Counts

| File | AUTHORITY replacements applied | CALLSIGN rows preserved | HISTORICAL rows preserved | Already gone |
| --- | ---: | ---: | ---: | ---: |
| `AGENTS.md` | 0 | 0 | 0 | 0 |
| `wow.md` | 0 | 0 | 0 | 0 |
| `docs/backlog-mechanism.md` | 5 | 0 | 0 | 0 |
| `docs/collaboration.md` | 1 | 3 | 0 | 0 |
| `docs/glossary.md` | 1 | 0 | 0 | 0 |
| `docs/journey.md` | 3 | 3 | 7 | 0 |
| `docs/orchestration.md` | 2 | 0 | 0 | 0 |
| `docs/working-modes.md` | 0 | 0 | 0 | 0 |
| `memory/rulings.md` | 0 | 0 | 23 | 0 |
| **In-scope total** | **12** | **6** | **30** | **0** |

These counts reconcile exactly with the inventory's rows for this
implementer's surfaces: all 12 AUTHORITY rows were applied, all 6 CALLSIGN
rows and all 30 HISTORICAL rows were preserved, and none of the 48 in-scope
rows was already gone. The remaining `plans/` partition contains 84 AUTHORITY,
13 CALLSIGN, and 180 HISTORICAL rows, or 277 rows total. This 48-row section
plus that 277-row partition equals the inventory's 325 rows; the seat will
append its applied `plans/` section and perform the final whole-inventory
reconciliation before landing.

## Active-record divergences

The active seat governance record in `plans/seat-governance-record.md`,
activated by Ruling R-30-m1, wins in the following cases.

### `docs/backlog-mechanism.md`: roster separation

- Inventory replacement text: “the dispatch delegate, who briefs, verifies,
  and lands,” split as dispatch delegate for briefing and custodial mechanics
  for verification and landing.
- Applied record-consistent text: “neither is the dispatch delegate, which
  briefs, nor the custodian, which runs gates and lands.”
- Reason: the custodian may run gates and perform a gate-bound acceptance act,
  but the record prohibits the dispatch hand from examining or accepting its
  own work. “Runs gates” avoids assigning independent examination to custody.

### `docs/backlog-mechanism.md`: backlog responsibility

- Inventory replacement text: “Dispatch delegate ownership” and “Backlog
  order — priorities, item shape, intake — is the dispatch delegate's standing
  responsibility on every machine.”
- Applied record-consistent text: “Dispatch delegate sequencing” and “Backlog
  sequencing within recorded priorities, mechanical item shaping, and
  checklist-governed intake are the dispatch delegate's responsibilities
  during a claimed change.”
- Reason: the record reserves worth, scope, and priority to Wido. The dispatch
  delegate may sequence within recorded priorities, perform mechanical intake,
  and decompose without changing scope; its permission is temporary per change.

### `docs/collaboration.md`: continuous session

- Inventory replacement text: “A continuous dispatch delegate session
  dispatches an agent for every build, critique, and fix round.”
- Applied record-consistent text: “A continuous seat session claims the
  dispatch-delegate role for each build, critique, and fix-round dispatch.”
- Reason: the record says the role begins with a recorded claim and ends when
  that change's chain closes or its budget fence does; the seat, not the role,
  persists across changes.

### `docs/journey.md`: backlog order

- Inventory replacement text: “the dispatch delegate owns its order.”
- Applied record-consistent text: “the dispatch delegate sequences it within
  recorded priorities.”
- Reason: backlog priority is human authority; the dispatch delegate only
  sequences work within priorities already recorded.

### `docs/journey.md`: builder and custody separation

- Inventory replacement text: “The rule keeps an unsupervised builder or
  custodial mechanics from quietly doing whatever it likes and calling it
  reviewed.”
- Applied record-consistent text: “The rule keeps the builder and custodial
  mechanics in separate hands instead of letting one actor quietly do whatever
  it likes and call it reviewed.”
- Reason: the governance record assigns implementation to the builder and
  gate-running and landing to custody, while recording that their separation
  at landing is conduct-only. The prohibition is on combining those hands, not
  on the builder performing implementation.

### `docs/orchestration.md`: standing backlog work

- Inventory replacement texts: “While the backlog holds claimable work, the
  dispatch delegate works it” and “A parked STREAM never means a parked
  DISPATCH DELEGATE.”
- Applied record-consistent texts: “While the backlog holds claimable work,
  the seat claims a dispatch-delegate role to work it” and “A parked STREAM
  never prevents the seat from claiming a dispatch-delegate role for another
  item.”
- Reason: the standing no-idle duty belongs to the persistent seat, while the
  dispatch-delegate authority is temporary and claim-bound for each change.

## Already-gone rows

None. Every in-scope AUTHORITY quote was present and replaced.

## Seat-applied surfaces: plans/ (73 AUTHORITY rows)

Applied by the acting seat (m2) under custody, because the implementer
role's binding instruction ("never touch plans/") conflicted with the
inventory scope - the delegate's gap-stop was accepted and the scope
split. Policy: minimal word-swap to the verdict's role noun, sentence
structure preserved; where the seat governance record supplies the
noun ("the seat"), it is used; every non-literal application is listed
below.

Counts (reconciled against the inventory's plans/ AUTHORITY rows):
- Design docs (plans/*.md): 59 rows -> 58 applied, 1 divergence.
- Goal projections (plans/goals/*.md): 14 rows across 12 goals ->
  3 applied via `goal edit` (never-idle-enforcement Intent,
  small-change-lane NextStep, stop-message-truth NextStep - the files
  are ledger projections and regenerate, so direct file edits do not
  hold), 11 divergences (see below).

Divergences:
1. CODE-IDENTIFIER (1): actionable-metrics-design.md:53 - the
   `coordinator | delegate | mixed` enum is the landed `built_by=`
   receipt-grammar value (internal/metrics/obligations_test.go:554).
   Doc text unchanged; renaming the doc without migrating the landed
   grammar would break correspondence. The rename belongs to the
   actionable-metrics goal when it builds.
2. REQUIREMENT-TEXT (11 rows, 9 goals): app-doctrine,
   app-guardrail-program, coordinator-charter (2 rows),
   goal-scope-bounds, incident-proposal-drafting,
   machine-concurrency-governor (2 rows), near-miss-register,
   precedent-index, reconciliation-guards - all HUMAN-ORIGIN goals
   whose quoted text is the human's recorded language. The seat does
   not rewrite the human's words (requirement supremacy outranks the
   inventory's verdict). These rows await Wido: a one-line ratification
   ("apply the sweep verdicts to my goal texts") lets any session apply
   them via goal edit in minutes.
3. WORDING (non-literal applications, both texts recoverable from the
   inventory row + this list): battery-postmortem 140/289 ("the seat"
   for the denied actor; prohibitions preserved verbatim), 495 (split
   per compound verdict: narrator supplies records, builder implements),
   546 ("record-writer seat"); counselor-design 145 (seat = dispatch
   delegate + custodial mechanics, per the seat record), 156/157 (the
   chair is the human's); disk-hygiene 9 rows (the design-only lock
   component renamed "operation custodian" - custodial-mechanics
   component naming; no landed identifier conflicts, internal/janitor
   carries no such name); fleet-pull 100/183 ("fleet-pull role", its
   own name, rather than "dispatch-delegate-pull"); operator-surface
   135 (remedy owner "custodial mechanics").

## Counting reconciliation (both units stated)

The implementer's totals are occurrence-based (325 = 96 AUTHORITY + 19
CALLSIGN + 210 HISTORICAL occurrences of the word). The seat's totals
are table-row-based (284 rows; a row whose quote contains the word
more than once counts once). Both partitions were applied and verified
independently; the authoritative completeness check is the residual
scan, not the arithmetic: after the sweep, every remaining
"coordinator" occurrence in a swept surface belongs to a CALLSIGN or
HISTORICAL row preserved verbatim (verified by grep over all seven
edited design docs and the five edited docs files), and the eleven
REQUIREMENT-TEXT divergences listed above are the only AUTHORITY rows
left unapplied anywhere, awaiting Wido's ratification.
