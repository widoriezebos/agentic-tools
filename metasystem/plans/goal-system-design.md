# The goal system: the thread of intent survives every turn boundary

Working Mode: design

Owner: main session (delegate), backlog item 14, under Wido's
2026-08-14 night ruling 4 (design pass tonight, DESIGNS ONLY).
Status: r2 — every r1 finding (G-01..G-16, critique preserved at
plans/goal-system-critique-r1.md) is dispositioned inline below;
awaiting r2 critique.

## The problem, in the human's words and one incident

"We lose track of the goal that we are chasing." The motivating
incident: the stop hook told the human "NOTHING LEFT TO WORK ON in
this checkout" while a quarter of a 101-finding program remained —
the backlog lived in docs/reviews/, invisible to the open-work
scanner, and even the narrow fix (a plans/ note) yields a verdict
that says THAT work exists, never WHAT to do next.

## Design r2

### 1. One CURRENT goal, a bounded stack behind it (G-13)

`plans/goals.md` holds exactly one `## Current goal` block and any
number of `## Queued goal` / `## Done goal` blocks. The grammar:

    # Goals

    ## Current goal: <kebab-id> — <one-line intent>
    - Origin: human | main
    - Next step: <one imperative sentence, ≤240 bytes, no control
      characters, single line>
    - Evidence: <path or D-entry>          (optional)

    ## Queued goal: <kebab-id> — <intent>
    - Origin: ...
    - Next step: ...                        (required)

    ## Done goal: <kebab-id> — <intent>
    - Concluded: <one sentence>

At most one Current goal (parse refusal otherwise); "next" is always
the Current goal's step — deterministic, no file-order policy to
game. Parking the current goal promotes nothing automatically; the
holder (or human) promotes a queued goal explicitly.

### 2. This IS a sixth standing ledger — owned as one (G-12)

goals.md joins the named standing ledgers. The doctrine amendments
ship with the change: plans/README.md names it and its relation to
handoff notes, wow.md's evidence rule gains the exception, the
adoption payload ships a template with one example block, and the
instruction audit checks its presence like the other ledgers. Done
goals are PRUNED to the last ten (history beyond that is git),
capping growth.

### 3. Causal continuation: goals ride the block-once path (G-01, G-09)

One engine verb — `report turn-verdict` — replaces the current
split where the engine reports open work and shell assembles the
message. It returns one structured decision:

    {"shouldBlock": bool, "signature": "...", "openWork": [...],
     "goal": {"id": ..., "nextStep": ..., "revision": "<sha of the
     goal block bytes>"}, "degraded": bool, "display": "..."}

Blocking rules, in order: (a) open work blocks exactly as today
(same signature discipline); (b) with NO open work and a Current
goal whose revision the session has not yet been blocked on, the
verdict blocks ONCE with display "open work is done; the goal file
names the next step: <step verbatim>" — the causal continuation the
incident needed; (c) the same revision never blocks the same session
twice (no loop); (d) with neither, today's all-clear stands and is
now true. The hook's shell shrinks to transporting the decision —
signature state, block-once bookkeeping, and display text all come
from the verb (G-09's single owner).

The goal clause never rides along with an open-work block (r1's
dual display is DROPPED): in-flight work first, orientation after —
one message, one meaning (G-02's collision source, G-11's conflict
surface).

### 4. The scanner and goals.md are disjoint by construction (G-02)

`planFiles` excludes goals.md by name (it is a ledger, not a plan —
exactly how the scanner should treat the other ledgers), and the
goal reader is its own grammar-aware parser. A done goal's retained
text can never produce OPEN-WORK because the scanner never reads the
file; the verdict verb reads goals only through the goal parser.

### 5. Handoff plans keep in-flight next steps; goals reference them (G-11)

A handoff plan's "Next step" remains the per-stream, in-flight
authority the scanner consumes. The Current goal's Evidence field
references the plan(s) serving it. One direction, enforced by the
verdict verb's precedence (open work first) and checked by the
suite: an active plan stream whose file the goal references cannot
coexist with a "nothing left" verdict — which is the incident's
regression test, end to end, no pre-seeded record (G-05's test
objection).

### 6. Losing the thread becomes an explicit act (G-05)

`goal done` on the CURRENT goal refuses unless the caller either
promotes a named queued goal (`--then <id>`) or declares the
checkout intentionally goal-free (`--and-none`, which the verdict
verb then reports as "goal-free by declaration at <time>" instead
of silence). Programs start by creating a goal (`goal open`), and
the adoption template documents that convention. Registration stays
an act — but silent absence and declared absence are now different
verdicts, and the incident's silent variant cannot recur while any
referenced plan stream is open (section 5's cross-check).

### 7. Authority: the engine is the only writer; origin gates transitions (G-07, G-08)

The file is mutated only through the verbs, under its own flock +
atomic rewrite (the ledger discipline verbatim). A manual edit is
detected by revision mismatch at the next read: the verdict goes
degraded (never all-clear, G-06) and names `goal reconcile`, which
validates the edited bytes under the lock and adopts or refuses
them. Verbs: `goal open|set-next|promote|park|unpark|done|reopen`,
with a legal-transition table in the contract (G-14). Authority:
mutations are holder-only (checkout custody), AND goals carry
Origin — `done`/`park` on a human-origin goal is HUMAN-reserved
(the stagnation-reset doctrine applied to intent): the holder may
progress the step, only the human may declare the human's goal
finished. Lease takeover changes nothing about goals — the file is
checkout state, not holder state; a new holder inherits intent
(G-08's custody/authority split, stated).

### 8. Failure is loud (G-06)

Missing file → a distinct verdict field ("no goal ledger"), exit 3
from `goal next`. Malformed file → exit 1, degraded verdict,
all-clear FORBIDDEN. The hook contract (below) requires transporting
degraded state, not swallowing it; the current `2>/dev/null || true`
suppression is named as a defect this change removes.

### 9. Runtime equivalence, honestly staged (G-04)

The decision is one verb; delivery is per-runtime and TODAY only
Claude has a turn-end hook. The design makes the gap explicit
instead of wishing it away: the deliverable includes a HOOK CONTRACT
(docs/design/): what a runtime adapter must declare to deliver
turn-verdicts (invoke the verb, transport display, honor
shouldBlock, never suppress degraded). Claude's hook implements it
at ship; codex and devin CANNOT claim it until item 16's audit gives
them a declared surface, and the contract document carries a
conformance table (claude: yes; codex: no turn-end surface; devin:
no turn-end surface) that the audit keeps honest. Item 14 ships as
"the mechanism plus one conforming runtime and a contract the others
must meet" — stated in those words for Wido's sign-off, not implied
(G-04's ruling made explicit).

### 10. Delegates get a projection, not the file (G-03)

Item 14 says "host and delegate alike"; r1's exclusion overreached.
r2: mission hosts are mains and read goals natively. DELEGATES get a
projection: the dispatch brief builder includes the Current goal's
one-line intent (id + intent, never the file, never queued/done
blocks) as a "serving goal" line, envelope-safe by size and content
bounds. Whether every brief carries it by default or roles opt in is
FLAGGED FOR WIDO — both are cheap; the difference is context-budget
policy, and that call is his (G-03 asked for exactly this ruling
rather than a design assertion).

### 11. Goal text is quoted data, never instruction (G-15)

The grammar bounds the step (single line, ≤240 bytes, no control
characters); the verdict frames it as quotation from a named file;
and the contract states: a goal confers ZERO authority — an agent
following it still meets every envelope, authority check, and
human-reserved boundary. Injection through goals.md is bounded to
what a plans-file note could already do, and the framing plus bounds
shrink it further.

### 12. Item-15 composition, corrected (G-10)

A watched run's three continuations (green/red/hang) live on the RUN
record — conditional intent is run state, not goal state. The goal
file never learns about runs directly and supervision never writes
goals (the authority matrix already refuses it). Composition is
read-side only: the verdict verb MAY read run records to enrich
orientation ("the goal's step is waiting on run X, still in
flight"), which needs no new write authority anywhere. Item 15's
design owns the run record; this design only promises the read seam.

## What this deliberately does not do

- No priorities, deadlines, or dependency graphs — one current goal,
  a queue, done history.
- No mission-prompt integration — the serialization question (D58)
  stays with the mission machinery; goals are checkout-level.
- No per-runtime hooks invented here — the hook contract defines
  them; item 16 delivers the surfaces.

## Proof obligations (rewritten to the seams, G-16)

- Scanner/goal disjointness: goals.md content can never produce
  OPEN-WORK; plan streams still can; the incident regression runs
  the verdict end to end from a backlog-plus-goal checkout.
- Block-once across both sources: unchanged open work does not
  re-block after a goal edit; a new goal revision blocks once; the
  same revision never re-blocks; goal-then-openwork transitions.
- Failure: malformed goals.md forbids all-clear; missing file is
  the distinct no-ledger verdict; the hook transports degraded.
- Authority: non-holder mutation refused; human-origin done/park
  refused for the holder; manual edit → degraded → reconcile
  round-trip; lease takeover leaves goals intact.
- Projection: brief carries id+intent only, bounded; no queued/done
  leakage.
- Verdict verb owns display: the shell transports byte-identical
  text; signature/bookkeeping parity with today's block-once proven
  by fixture.
- Runtime conformance table matches reality (audit-checked).
