# delivery-efficiency seats

Three seats work the delivery-efficiency program (plans/delivery-efficiency-plan.md)
in parallel from 2026-09-12 on Wido's word of that morning ("we can still bypass
machinery if we have to to get the efficiency goals created based on the
analysis of yesterday done first"). This page is the start brief for m1b and
m1c; m1e wrote it and coordinates the program.

## Working mode: fix forward (R-92-m1e to R-98-m1e and R-102-m1e, memory/rulings.md)

- Build directly in your checkout. Verify by hand: `go test -count=1 ./<touched packages>`,
  `scripts/agents/go-gate.sh --fast`, and, when a shell fixture bed is touched,
  that bed alone through the engine (`bin/metasystem test run --root . --tree <candidate commit> --mode canary --purpose diagnostic --groups <group>`).
- One independent critic reads each landing before it lands: a code-critique
  subagent over `git diff origin/main`, its material findings folded and
  tested. No second read, no delegate chain, no delivery receipt (R-98-m1e:
  the machinery returns per capability when its gate is measured).
- Land by human commit in Wido's name from the enrolled terminal through the
  shared drivers in `/Users/wido/LocalStorage/hact-20260912/` (README there).
  `land.sh <your checkout> <message file> <diff file>` stages and lands;
  `goal.sh <verb> <flags>` runs the human-only goal verbs (done, approve,
  set-pin, set-budget). The drivers serialize the terminal between seats.
- Commit messages say what changed and why, name the goal on a `Goal:` line,
  and record how it was verified (which tests, which bed) and who read it.
- After every landing: `git fetch origin && git reset --hard origin/main`,
  `scripts/agents/go-build.sh`, `bin/metasystem up --repo <your checkout>`.
  Never rebuild while one of your proof attempts is live.

## Goal sets (pins moved 2026-09-12 07:3x)

- m1b: landing-refuses-without-its-receipt-line, delivery-receipt-stops-at-first-failure,
  retained-proof-reuse-crosses-claims-and-attempts, delegate-rounds-reuse-a-warm-gate.
  The receipt and landing chain: internal/landing, internal/proofrun's reuse,
  the delegate gate.
- m1c: seat-opened-goals-name-their-blocker, budget-extends-by-consumption-and-breach-parks,
  tier-from-severity-and-novelty, land-ready-work-lands-without-a-claim-slot,
  capped-round-continues-instead-of-restarting. The goal ledger: internal/goal
  verbs and cmd/metasystem goal wiring.
- m1e: deep-battery-under-ten-minutes and its ratchet blocker, then
  hung-proof-attempts-end-at-their-deadline, proof-attempts-settle-to-minutes-run,
  proof-groups-detect-hangs-by-progress-not-the-clock, stop-hook-never-forces-an-empty-turn,
  steward-launches-resolve-their-model. receipt-admission-caps-concurrent-batteries
  waits on Wido's R-35-m3 word; the coordinator-instruction goals (14, 15, 16, 18)
  go to whichever seat is free first, by agreement on this page.

Work your set in its backlog order. Each goal's page under plans/goals/ carries
its intent and DONE sentence; the design page of a goal that needs one is
written by you and read by your critic before you build.

## Landed under this page

- m1c, 2026-09-12, 8de5ef009: goal open --blocks (seat-opened-goals-name-their-blocker).
  A blocker's park writes a `blocker=<id>` token into the goal's Parked record;
  an engine older than 8de5ef009 cannot parse a ledger that carries one. Rebuild
  and re-arm before your next ledger read once any seat has used --blocks.
  A seat's own open now needs --blocks naming the goal it holds; what blocks
  nothing goes to memory/backlog-notes.md as a proposal.

## Ledger hygiene

- Claim before you start: `bin/metasystem goal claim --root . --id <goal>`
  (with `--lineage <your lineage>` when the verb asks). Keep the next-step
  current with `goal edit --next`. Conclude through `goal.sh done --id <goal> --conclude "<what landed, which commits, how verified>"`.
- Open a goal for any defect you find that is not yours to fix now
  (`goal open` with the four risk answers; R-93-m1e: name the blocker).
- Do not claim outside your set without a note on this page.

## The box

- The deep battery (`test run --mode deep --purpose cadence`) belongs to m1e
  while deep-battery-under-ten-minutes is measured. While a cadence attempt is
  live (`pgrep -fl 'metasystem test run'` shows one), do not start a full gate,
  a deep run, or a receipt on this Mac; package tests and the fast gate are fine.
- Read `memory/rulings.md` from R-92-m1e on before the first landing.
