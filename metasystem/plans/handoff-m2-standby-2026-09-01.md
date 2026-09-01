# m2 hands over — 2026-09-01

m2's Fable capacity is exhausted; Wido moves this seat to standby and
names m0/m0b the successor for the program seat. This file is the
complete state transfer. Everything referenced is landed on
origin/main unless marked otherwise.

## THE TWO NEW LAWS — read before touching anything

- **R-38-m2, THE BACKLOG LAW** (Wido verbatim: "THIS IS A LAW FROM NOW
  ON. WE HAVE A BACKLOG FOR A REASON. NOTHING GETS BUILD WITHOUT THE
  USING A PROPER BACKLOG ITEM"): every build travels the full ladder —
  backlog item, design, design critique, build, code critique, tests.
  No urgency exception. The specimen was m2 dispatching a build
  straight from diagnosis under fix-immediately pressure; Wido caught
  it, it was cancelled mid-run, the ladder restarted.
- **R-39-m2, UNBOUND CHANGES NEED HIS WORD** (verbatim: "Whenever you
  add/change code without a backlog item, you will need explicit
  approval from me"): includes seat-authored fixes and gate top-ups
  that previously rode on disclosure alone.

Both are conduct now and machinery soon: `commit-goal-binding`
(commit gate verifies a Goal-Item trailer against the ledger; steward
sweeps canonical commits for violations; the human-waiver design is
R-39's only lawful unbound path), `design-gate-at-dispatch` (the
delegate door refuses design-bearing builds without a certified
design AND retires `--goal none-explicit`), `critique-always` (m3's:
every chain gets cross-family critique, mechanical included).

## THE FIX WAVE — highest priority, Wido's order: nothing else resumes
   until these are fixed and proven with tests

1. `breach-clock-and-budget-honesty` — RELEASED FOR YOU, three
   goal-seam defects proven the night of 2026-08-31:
   - the elapsed breach clock restarts on every budget raise
     (SetBudget re-binds the claim record; dispatch/budget.go anchors
     on the current revision) — a raised budget outruns the breaker
     forever;
   - budget durations parse through a working-hours grammar (d = 8h)
     and normalize inputs, so a human's "9d" is enforced at 72 clock
     hours — one third of intent, silently, on live budgets;
   - a breach-stopped claim refuses release and still counts against
     the one-claim quota, freezing a whole machine (m3 was the
     wedged specimen).
   STATE: design authored (plans/breach-clock-and-budget-honesty-design.md,
   unlanded/untracked on m2 — recover from
   artifacts/agents/worktrees/breach-design/ or author fresh); Sol's
   design-critique returned EIGHT MATERIAL findings, folded into the
   register (chain breach-design-crit). BLOCKED on budget: reserved
   job-minutes 360/360 spent; Wido's raise (proposed 840) unblocks the
   revision.
2. `burn-without-delivery-tripwire` — CLAIMED BY m0: the steward
   watches landing cadence per claimed goal, escalates past 4h,
   breach-stops past 6h; watches landings not limits, so raises can
   never quiet it.
3. `failed-job-attention` — m3's, waiting on the wedge fix.
4. `proof-harness-process-custody` — m2's leak: twelve CPU-hog loops
   orphaned for 12+ hours, polluting the night's own load diagnoses.
   Killed; the class needs deterministic process-group custody.

## What m2 landed under the takeover (all on origin)

The counselor carriage and its accepted-risk register; the complete
coordinator-language sweep; the set-obligation temporary-word leg; the
65-goal backlog triage; the seat-communication doctrine
(docs/seat-communication.md — READ IT, it governs every message to
Wido); goal-scope-bounds slice 2 (the norm and the split verb, with
five boundary mechanisms designed in-line under his lane exception);
ledger-attention (the machines now notice the ledger moving); four
stale fixtures repaired; the leak test that caught a real custody hole.

## Open items for the human

- The fix-wave budget raise (above) — the one blocker.
- `winddown-census-handoff-leak` (HIGH): the sharpened leak test
  caught a REAL wind-down-to-census custody hole in governed run
  20260901-a; unfixed, blocks the weight discharge.
- `standing-validation`: the first governed discharge remains
  UNDISCHARGED after seven attempts (each red was a real defect, all
  but the last now fixed); weight keeps accumulating.
- The 2026-09-06 terminal pass: re-ratify m1, m2, and m3's temporary
  steward arms and every act under departure R-29 (including m2's
  obligation arms and the R-37 re-arm).
- `sweep-ratification-rows` residue: 2 already-gone rows, 1 goal-id
  rename deferred to its own goal.

## Standing arrangements the successor inherits

R-25 lanes (Sol builds and critiques designs; Fable designs and
critiques implementations) with R-34 pinning Sol absolutely — no model
substitution without his explicit word; R-35/R-32 load leniency (no
machine reservations, a load-shaped red is coordination debt, caps
that convert slowness into failure are defects); R-36 policy decisions
get critique then his word, never seat adoption; slices ≤4h and every
slice lands.
