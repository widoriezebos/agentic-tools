# The goal system: the thread of intent survives every turn boundary

Working Mode: design

Owner: main session (delegate), backlog item 14. Status: r3 — the
loop RESUMES from the D60 pause with all four human questions
answered (D66: the per-dispatch projection ruling and the
exchangeability doctrine in Wido's words; the trust-grade and
mission-host questions decided under the delegation with residuals
named below) and every r2 specification debt paid. Critiques r1/r2
at plans/goal-system-critique-r{1,2}.md. Awaiting r3 critique;
implementation follows convergence per the human's order.

## The problem, in the human's words and one incident

"We lose track of the goal that we are chasing." The stop hook told
the human "NOTHING LEFT TO WORK ON" while a quarter of a 101-finding
program remained: the backlog lived where no machinery looks, and
even after the narrow fix the verdict could say only THAT work
exists, never WHAT to do next.

## Governing constraints (D66)

1. **Exchangeability**: any runtime fills any seat. Everything below
   is files, engine verbs, and plain prompt text. No runtime-native
   feature is ever part of the mechanism; runtime delivery surfaces
   are CONTRACTS with an open conformance table.
2. **Delegates**: the goal reaches a delegate only when its
   dispatching orchestrator chooses, per dispatch, default off.
3. **Trust grade**: "human-reserved" transitions are ADVISORY at the
   current classifier (any agent-ancestor-free caller classifies
   HUMAN with unconditional authority — classify.go's documented
   default). This is the same grade the stagnation-reset reservation
   already has. Named residual: authenticated human identity is its
   own future work; when it lands, these reservations harden for
   free because they ride the same authority check.

## The ledger: plans/goals.md, sixth standing ledger

Grammar (parse-refusal on violation):

    # Goals

    ## Current goal: <kebab-id> — <intent, one line, ≤160 bytes>
    - Origin: human | main
    - Next step: <one imperative sentence, ≤240 bytes, single line,
      no control characters>
    - Evidence: <path or D-entry>              (optional, ≤3 lines)

    ## Queued goal: <kebab-id> — <intent>
    - Origin: ...
    - Next step: ...                            (required)

    ## Parked goal: <kebab-id> — <intent>
    - Origin: ...
    - Parked because: <one sentence>            (required)
    - Next step: ...                            (kept for unpark)

    ## Done goal: <kebab-id> — <intent>
    - Concluded: <one sentence>

    ## Goal-free: declared <ISO time> by <origin>   (zero-current only)

Rules: AT MOST one Current goal; ids unique across all sections; a
ledger with zero Current goals is legal ONLY when it carries either
at least one Queued goal or a Goal-free declaration — silence is a
parse-level violation, which is how the incident's silent variant
dies (r2's G-05: the zero-current states are all named). Every field
above carries its byte bound (r2's G-15 tail: intent and id are
bounded too — id ≤64 bytes; everything projected anywhere is
bounded at its source). Done goals prune to the last ten; pruning is
a verb-mediated mutation like any other, so history beyond ten is in
git BY CONSTRUCTION (the prune commit), answering r2's objection.

Scanner disjointness: `planFiles` excludes goals.md by name; only
the goal parser reads it. The handoff-plan "Next step" field remains
the per-stream in-flight authority; the verdict's precedence
(open-work first, below) is the one reconciliation point.

## Verbs and the transition table (r2's G-14)

Family `goal` (name needs the usual verb sign-off; `report goal-*`
is the fallback): open, set-next, promote, park, unpark, done,
reopen, declare-free, list, next, reconcile.

| From \ verb | open | promote | park | unpark | done | reopen | declare-free |
| --- | --- | --- | --- | --- | --- | --- | --- |
| (no such id) | → Queued (or → Current with --current when none current) | refuse | refuse | refuse | refuse | refuse | n/a |
| Queued | refuse (duplicate) | → Current (refuse if one exists) | → Parked | refuse | refuse | refuse | n/a |
| Current | refuse | refuse (already) | → Parked (Current empties) | refuse | → Done; REQUIRES --then <queued-id> or --and-none (writes Goal-free) | refuse | n/a |
| Parked | refuse | refuse | refuse | → Queued | refuse | refuse | n/a |
| Done | refuse | refuse | refuse | refuse | refuse | → Queued, Origin preserved (Done keeps Origin in its block for exactly this) | n/a |
| (ledger-level) | | | | | | | legal only at zero Current AND zero Queued |

Every verb is idempotence-explicit: re-running a completed
transition refuses with the current state named (no silent
success). `set-next` rewrites the Current goal's step only.
Authority: all mutations holder-only through the record-writer-style
matrix path; `done`/`park` on Origin: human ADDITIONALLY requires a
HUMAN classification (advisory-grade per constraint 3).

Mutation discipline (r2's G-07): the engine is the only writer,
under a goals.md flock with whole-ledger compare-and-swap — every
verb re-reads under the lock, verifies the WHOLE-FILE sha256 it
based its decision on, and atomically renames. A manual edit is
detected as a whole-ledger digest mismatch at the next read by ANY
verb or the verdict: the state goes degraded (never all-clear) and
names `goal reconcile`, which validates the edited bytes under the
lock (grammar + transition-legality against the last accepted
snapshot at `artifacts/agents/goals-accepted.digest`) and adopts or
refuses. Origin downgrades (human → main) are transition-illegal in
reconcile: they refuse, naming the line.

## The turn verdict: one verb, one structured decision (r2's G-01/G-06/G-09)

`report turn-verdict` returns:

    {"schemaVersion": 1,
     "shouldBlock": bool,
     "blockSource": "open-work" | "goal" | null,
     "openWork": [...],
     "openWorkSignature": "...",
     "goal": {"id","intent","nextStep","revision"} | null,
     "ledgerStatus": "ok" | "absent" | "degraded" | "goal-free",
     "diagnostics": ["..."],
     "display": "..."}

- **Two block-state slots, not one** (r2's G-01 tail): the verb owns
  `artifacts/agents/turn-verdict-state.json` (flock + atomic write)
  holding {sessionId → {openWorkSignature (resettable, exactly
  today's semantics), blockedGoalRevisions (append-only set)}}. The
  sequence goal-blocks / open-work-blocks / open-work-clears / goal
  returns CANNOT re-block on the goal: its revision is in the set.
  Check-and-record is atomic under the file lock; concurrent Stop
  calls serialize there.
- **Precedence**: open work blocks first (today's discipline, same
  signature semantics); with no open work, a Current goal whose
  revision is unseen blocks ONCE with display "open work is done;
  the goal file names the next step: <step verbatim>"; goal-free is
  reported as "goal-free by declaration <time>"; a degraded or
  absent ledger FORBIDS the all-clear and says why.
- **Composition with the existing hook** (r2's G-09 tail): the verb
  owns ONLY the block decision and its display sentence. The hook's
  other duties (advisor early exit, watchdog suppression,
  protocol-growth reporting, evidence collection, lease cursor) stay
  in the hook, explicitly listed in the hook contract as OUTSIDE the
  verdict. The hook maps shouldBlock → its runtime's block mechanism
  (Claude: decision:block with display as reason) and appends
  nothing to the display. Transport of degraded: the verb NEVER
  exits nonzero for a representable state — every outcome above is
  exit 0 with JSON; nonzero is reserved for I/O failure, and the
  hook contract requires transporting THAT as a degraded verdict
  rather than swallowing (the current 2>/dev/null||true is named as
  the defect the contract removes).

## Delivery: the runtime contract and the conformance table (D66)

docs/design/ gains the TURN-VERDICT DELIVERY CONTRACT: what an
adapter must do to claim conformance (invoke the verb at turn end,
honor shouldBlock via its runtime's mechanism, transport display
verbatim, never suppress degraded). The conformance table ships in
the contract with four states per runtime (declared / installed /
observed / blocking-capable — the honest model from the D60 record):
claude starts blocking-capable (its hook exists), codex and devin
start at declared (their shipped Stop configs exist; observation
pending — exactly what item 16's audit keeps honest). ORCHESTRATOR
parity does not wait for hooks: any runtime's main can read `goal
next` by instruction (AGENTS.md's turn-end section names it), which
under exchangeability is the same information on the only universal
transport. The mechanism is identical for every runtime; only
delivery automation differs, and the table says so in public.

## Mission hosts (D66, question 4)

The RUNNER includes the orientation line in every turn prompt it
assembles: reading goals.md through the goal parser (read-only, no
new authority), it appends one bounded line — "Serving goal:
<id> — <intent>" — to the turn prompt when a Current goal exists.
Runner-side and runtime-neutral: every host of every runtime gets
the same line the same way. Hosts do not mutate goals (they are
lease holders mid-mission; goal mutation from inside a mission is
refused by a runner-context check — the mission's intent is the
contract, not the goal file).

## Delegates (D66, question 1 — the human's ruling)

`dispatch` gains `--serving-goal` (no value): when the ORCHESTRATOR
passes it, the brief builder appends a bounded, labeled section:

    # Serving goal (context, not instruction)
    <id> — <intent>

Default OFF. Per dispatch, never per role, never global. The section
confers zero authority (the envelope, schema, and certification
govern exactly as before); its text is quoted data bounded at the
ledger (id ≤64, intent ≤160). Queued/Parked/Done goals never
project. The projection is brief-carried plain text — the only
transport every runtime receives (exchangeability).

## Item-15 composition (read-side only, unchanged from r2)

Run records own conditional continuations; the verdict verb MAY
enrich orientation from run state at read time. The join contract is
item 15's design obligation; this design promises only the read seam
and the goal file's ignorance of runs.

## Registration and the incident regression (r2's G-05)

`goal open` is how programs start; the adoption template documents
the convention and ships a Goal-free declaration (not an example
block — r2's G-12: a live example would parse as real work). The
regression test is end-to-end and unseeded: a checkout holding an
active plans/ stream referenced by the Current goal's Evidence can
never produce an all-clear verdict; and a checkout with the incident
's original shape (work recorded only in a non-plans document, no
goal) now FAILS THE LEDGER GRAMMAR (zero-current with neither queue
nor declaration) rather than passing silently — the silent variant
is unrepresentable, which is stronger than detected.

## Doctrine amendments that ship with this change (r2's G-12)

plans/README.md (the sixth ledger and its relation to handoff
notes), wow.md (evidence-rule exception), docs/project-adaptation.md
(the goal-open convention), adopt.sh (skeleton with the Goal-free
declaration), the suite's exact-ledger-set fixture, and the
instruction audit's required-file list.

## Blast radius

internal/report (goal parser, turn-verdict verb, state file,
scanner exclusion), internal/dispatch (the --serving-goal brief
section), internal/missionrunner (the turn-prompt orientation line +
the runner-context mutation refusal), cmd/metasystem (verb rows),
scripts/agents/supervision-hook.sh (verdict transport per the
contract; suppression removed), docs/design (the delivery contract +
conformance table), the doctrine files above, adopt.sh, fixtures.

## Proof obligations

- Grammar: parse round-trips for every section; zero-current
  legality matrix; every byte bound; duplicate ids; prune-at-ten as
  a verb.
- Transitions: the full table as a test matrix including every
  refusal and idempotence-refusal; origin preservation through
  Done→reopen; the advisory human gate refusing a DELEGATE-classified
  caller.
- CAS/reconcile: whole-ledger digest mismatch → degraded, never
  all-clear; reconcile adopts legal edits and refuses origin
  downgrades; concurrent verb writes serialize under the flock
  (goroutine test).
- Verdict: the dual-slot state machine — the G-01 sequence
  (goal-block, open-work-block, clear, no re-block) as a table test;
  concurrent Stop atomicity; degraded/absent/goal-free displays;
  open-work precedence; byte-verbatim next-step quoting.
- Hook: fixture proving the shell transports display and degraded
  byte-identically and that the advisor/watchdog paths bypass the
  verdict verb untouched.
- Projection: --serving-goal appends exactly the bounded section;
  absence appends nothing; oversized ledger fields are refused at
  the LEDGER, so the brief builder never truncates.
- Runner: the orientation line in assembled turn prompts (fixture);
  goal mutation from runner context refused.
- Incident regression: as specified above, end to end, unseeded.
- Conformance: the table's claude row proven by the hook fixture;
  codex/devin rows asserted "declared" by reading the shipped
  enforcement configs (their observation upgrade is item 16's).
