# Unit U4 of lane-proves-at-the-batch-risk: batches split by depth class; an overdue full proof rides the next batch

Working Mode: Implement. Design: `plans/designs/lane-proves-at-the-batch-risk.md`, decisions D5 and D6 (critique note F4). Builds on U2 (the tier threshold setting). Size: at most 250 production lines.

## What exists

- `internal/landing/plain/batch.go`: `SelectBatch`/`selectBatchLocked` (~296, ~427) build a batch from the waiting lines under the `landing.batch` policy (`1`, `auto`, ...).
- `internal/landing/plain/wake.go` ~78-90: `fullProofDue` raises `full-due`; ~130-138: the clock resets on a green full-scope proof whose tree was pushed.
- `skills/landing-agent/SKILL.md` case 8: the trunk proof runs on `full-due` only when nothing mergeable waits.

## Build

1. Selector (D5): under `landing.batch=auto`, the waiting lines group by depth class: cheap (every member's tier below `landing.full-from-tier`) and full (the rest). The cheap batch is selected first; the full batch follows once it lands. One class present -> one batch as today. Conflict handling and `after` records unchanged. Status names the class (`batch of 2, depth class cheap`).
2. Overdue full proof (D6): when `full-due` is raised, the next batch's depth decision (U2) returns `full` whatever its tiers, with reason `full proof overdue since HH:MM`; its green after the push resets the full-proof clock exactly as a trunk proof's green does (wake.go ~130: a pushed batch tree proven at full scope). With nothing waiting, the trunk proof runs as today (skill case 8 unchanged).
3. Status prints the class and the overdue reason.

## Tests

- Mixed tiers -> two batches, cheap first; one class -> one batch; nothing waiting -> none (mutation: ignore tiers -> one mixed batch).
- `full-due` raised and a low-tier batch waiting -> depth `full` with the overdue reason, and after a green push the clock is reset (mutation: keep impact -> the clock never resets, asserted).
- `after`/conflict records unchanged: run the existing selector tests.

Then `go run ./cmd/devgate static`, `./internal/landing/plain`, cmd tests by name (`TestLandingSelect|TestSelectBatch|TestLandingStatus|TestFullDue|TestWake|TestLandingProve`), every existing test using a changed seam. Never edit testing.json; never open metasystem.conf.local. Report: diff --stat, each exit, the status texts.
