# Design brief: health-is-green-when-the-seat-is-healthy, step 3 (the standing reds)

## Revision

Revision: first draft

Reason: goal `health-is-green-when-the-seat-is-healthy` (tier 2, approved by Wido 2026-10-03, claimed by seat m1l). Steps 1 and 2 (the board follows the lane; the stuck detector) have their own accepted pages and wait in the landing lane. The coordinator m1e, 2026-10-04 21:58 CEST: finish this goal now with its remaining next step, "the design for the reds that still exist today only (one row each: cause, fix, retire or automatic remedy; the no-remedy escalation rule; per-checkout-kind role sets; the stop-hook-duration root cause), retire the reds of 10-03 that are gone with one line each, one Astra round, under 1,000 words, smallest thing that works." Write a new page; do not touch the step 1 and step 2 pages.

## Context pack

Read this pack first. Open another file only to check one of the cited lines below or to find one red's cause; do not widen the read beyond that. Paths are relative to the installation `metasystem/` of `/Users/wido/LocalStorage/GitHub/agentic-tools-m1l` (main at the time of writing: 75933ab51).

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

The goal's intent, verbatim in part: "A seat's health verdict is green when the seat works and red only for something someone can act on (Wido 2026-10-03: 'are our supervisors doing their job well?') [...] Wanted: each standing red is fixed, retired or given a remedy the machinery runs itself; a role with no remedy after N ticks files one defect (goal blocked-seats-route-to-the-machinery-not-a-supervisor) instead of staying red forever; health roles that do not apply to a checkout kind (the lane) are not evaluated there."

Evidence, read 2026-10-04 21:39-21:48 CEST from `artifacts/agents/steward/health.json` of all ten checkouts on this computer (six building seats m1f to m1k, m1l, the coordinator m1e, the browser-interface seat ui, and the landing lane's checkout `agentic-tools-landing`); copies in `/tmp/m1l-evidence/health-NAME.json`. Every checkout reads `unhealthy`. The dead roles, with their recorded reason:

1. retro-debt, all ten: "RETRO DEBT awaits a receipt after arc-goal:verbs-match-intent:602TW9BXPRBJ04Q9NZGSYSHT8K-m1e-c6925449"; remedy a retro receipt by hand; no automatic remedy.
2. trunk-red, all ten: "deep validation cadence is overdue at trunk 7d0fc8692 tree 71e8dd435"; remedy "the landing lane runs the deep validation cadence when it is due". The lane today proves every push with the full suite in a Linux VM; the deep mode does not pass on this Mac (goal intent).
3. capability-snapshots, eight (all but m1e and m1i): "missing or stale capability snapshots" for runtimes such as devin, codex, claude; remedy "the next delegated job for that runtime probes it". Devin is in no roster of this project.
4. ledger-attention, six (landing, m1f, m1h, m1i, m1j, m1k): "the shared ledger moved to X N minutes ago and is unexamined past 30m" (43 to 3955 minutes); no automatic remedy.
5. context-budget, five (m1e, m1f, m1g, m1k, m1l): "482 thousand tokens this call, trigger 105, proof line 150, proof maximum 200, ceiling 250; over the proof maximum"; no automatic remedy. Wido removed the context caps on 2026-09-20 (goal intent); sessions run at 320 to 830 thousand tokens.
6. repo-watcher, four (m1e, m1f, m1l, ui): "lastSuccess belongs to pid 6742, not resident runner pid 22487" (three), "lastSuccess is stale at 12m21s" (ui).
7. session-main, two (landing, m1i): "no session main is announced". The landing checkout never has a session main; m1i was idle.
8. narrator-freshness, two (m1e, m1l): "component generation 220 does not match installation generation 221".
9. hook-freshness, two (m1e, m1l): "turn generation N has an attempt without completion" (both were inside a long turn when read).
10. stop-hook-duration, m1e only: "the last Stop took 29s of the 60s budget; the threshold is 15s".

Reds of 2026-10-03 that are gone today: stop-capability-epoch on m1f and m1j (claim epoch against lease epoch after a budget raise); the ui seat's repo-watcher reason of 10-03 (success owned by a previous runner pid) now reads as staleness.

Two steward defects the seat saw today, for one row each only if this page's remedy covers them (otherwise they stay with goal machinery-blocks-of-2026-10-04): item AY, a working seat read as "idle refusal 3 / idle-backlog-dead" (`internal/steward/verdict.go:17`, `internal/steward/alert_episode.go:301`); item AO, an outage mark whose reset time rolls to the next day.

Code facts (file:line at 75933ab51; verify only what you rely on):

- Roles: `internal/steward/health.go:47-70` (the names) and the order `internal/steward/health.go:72`; evaluated each tick by `evaluateHealthRolesWithLedger` (`internal/steward/health.go:474`). Any dead role makes the aggregate `unhealthy` and counts failures (`internal/steward/health.go:770-780`). `hasLawfulAutomaticRemedy` (`internal/steward/health.go:814`) decides whether a dead role alerts.
- The checks: retro-debt `internal/steward/health.go:598`; hook-freshness `internal/steward/health.go:616-620`; stop-hook-duration `internal/steward/health.go:670`; repo-watcher `internal/steward/health.go:968`; narrator-freshness `internal/steward/health.go:1044`; session-main `internal/steward/health.go:1097`; capability-snapshots `internal/steward/health.go:1524`; context-budget `internal/steward/context.go:180`; trunk-red `internal/steward/trunkred.go:21-31`; ledger-attention `internal/steward/ledgerattention.go:661`.
- Health alerts are one episode per seat keyed by every non-alive role together, cleared only when the whole seat reads healthy (`internal/steward/alert_episode.go:372-492`). Settings live in the `config.Setting` table (`internal/config/defaults.go:15-35`), for example `steward.stop-slow-sec` (`internal/config/defaults.go:149-150`).

## Decisions the seat has taken (build on them; argue only with evidence)

- D1 One row per red of today (the ten above): its cause as the code and the evidence show it, and exactly one of: fix (the check or the component is wrong; name the change), retire (the role does not measure anything someone can act on, or no longer applies; say what replaces it, if anything), or an automatic remedy the steward runs itself (name it and its bound). Name each row's owner. Where the cause is not visible from code and evidence, say so and say the one measurement that finds it; do not guess.
- D2 The no-remedy rule: a dead role with no automatic remedy for N consecutive ticks stops counting toward the seat's aggregate and files one defect where a person or the machinery picks it up (the goal names blocked-seats-route-to-the-machinery-not-a-supervisor). You choose N, where the defect goes (an existing record), and what makes it count again.
- D3 Per-checkout-kind role sets: the landing lane's checkout, the coordinator's checkout and a building seat evaluate only the roles that apply to them. Name the kinds from what the code can already tell (the lane registration names the lane checkout), and the role set of each.
- D4 The stop-hook-duration row finds the root cause of m1e's 29-second Stop (is the health call still the largest share, per the 20 to 42 second regression of 2026-09-16, commit df5c833f0?) and gives a real fix or an automatic remedy, not a standing red.
- D5 Step 1 of this page is the smallest set of rows that turns today's evidence green on a healthy seat; the rest is deferred with the field or home it builds on.
- D6 Behaviour tests stub Git (project rule; `development/project-rules-local.md`).

## What the page must answer

Write the page as a design record (`docs/design/design-obligation-gate.md`, "A design is a record": head with `Kind: design`, a new `Id`, `Status: draft`, `Goals: health-is-green-when-the-seat-is-healthy`) at `plans/designs/health-is-green-standing-reds.md`, at most 1,000 words, scope first.

1. Scope: what step 1 changes and what it leaves alone, in five lines.
2. The rows: one table, a `Red` column first, then cause, decision (fix, retire or automatic remedy), owner, and the test that is red without it (file, test name).
3. The no-remedy rule (D2) and the per-checkout-kind role sets (D3).
4. The stop-hook-duration root cause (D4).
5. Items AY and AO: covered by a row, or left to machinery-blocks-of-2026-10-04, one line each.
6. Retired reds of 2026-10-03, one line each.
7. Moved effects: in the form `| Effect | From | To | Code |` with existing backticked repository paths prefixed `metasystem/`, or the line `No owner moves.`
8. Deferred, each with the field or home it builds on.
9. Open questions for Wido, each with your recommended answer; keep them to real choices.

Estimate the build's changed lines honestly (production and test separately) and propose the units in a table with a `Unit` column first, each under 1,000 changed lines.

## Tool-call budget

Maximum reader tool calls: 80
