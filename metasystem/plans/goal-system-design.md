# The goal system: the thread of intent survives every turn boundary

Working Mode: design

Owner: main session (delegate), backlog item 14. Status: CONVERGED
at r12 (trajectory 16/14/13/13/8/5/4/4/5/4/2; critiques at
plans/goal-system-critique-r{1..11}.md). r11's two residual
findings were stale text from earlier folds — the blast radius
naming internal/report for goal-owned components and the corrected
GOAL-22 schedule surviving uncorrected in prose — fixed here with
zero new decisions; its other two dispositions were confirmed
outright, and every mechanism was confirmed by r10/r11. The stop
criterion is met: findings stopped changing what gets built.
Convergence recorded as D69. IMPLEMENTATION IS GO against the
obligation matrix (GOAL-01..22), statuses PARTIAL→DONE row by row;
the design-obligation gate exits 1 until every critical/high row is
DONE. Human rulings (D66/D67) remain fixed input.

## The problem, in the human's words and one incident

"We lose track of the goal that we are chasing." The stop hook told
the human "NOTHING LEFT TO WORK ON" while a quarter of a 101-finding
program remained: the backlog lived where no machinery looks, and
even after the narrow fix the verdict could say only THAT work
exists, never WHAT to do next.

**Stated non-goal, with the residual named (r4 finding 1, closing
the loop's longest-running objection honestly)**: intent recorded
ONLY where no sensor reads — a backlog in docs/reviews/, a TODO in a
commit message — is undetectable by construction, and this design
does not claim otherwise. What it does: makes declaring intent a
one-command act (`goal open`), makes UNDECLARED absence
unrepresentable in the ledger (zero-current requires a queue or a
declaration), expires declarations when the plans-stream world
moves, and adds the doctrine rule (project-adaptation + AGENTS.md
amendments) that programs START with `goal open`. The residual — a
human or agent who writes a program down somewhere unscanned and
declares nothing — remains possible and is accepted in writing; the
regression tests therefore test the DECLARED lifecycle, and the
incident's full prevention is the doctrine rule plus this
machinery, not the machinery alone.

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
    - Origin: ...                               (kept for reopen)
    - Concluded: <one sentence, ≤240 bytes>

    ## Goal-free: declared <ISO time> by <origin>   (zero-current only)

Rules: AT MOST one Current goal; ids unique across all sections; a
ledger with zero Current goals is legal ONLY when it carries either
at least one Queued goal or a Goal-free declaration. The declaration
CANNOT GO STALE (r3 finding 1): it records the plans-stream scan
digest at declaration time (`## Goal-free: declared <time> by
<origin> over <scan-digest>`), and the verdict recomputes that
digest — a changed stream set invalidates the declaration, and the
verdict then BLOCKS ONCE with "the goal-free declaration predates
new work; declare a goal or renew with `goal declare-free`" — the
world moved, so the declared absence of intent expired with it.
RENEWAL IS DEFINED (r4 finding 3): `declare-free` on a ledger
already carrying a declaration is the renewal transition — it
rewrites the declaration with the fresh time and scan digest (the
one deliberate exception to idempotence-refusal, named in the
table). The block-once state for staleness is the STALE DIGEST
itself, stored in the same per-session state as goal revisions
(blockedFreeDigests, same append/prune rules) — one stale world
blocks once, a further world change blocks once more. Ledger ABSENCE is baseline-aware (r9
finding 4 split it): with NO baseline beside it (goals-accepted.json
also absent — the pre-adoption installation), absence is ADVISORY,
never degraded (r3 finding 11's upgrade cliff): the verdict reports
"no goal ledger; `goal open` starts one" without blocking and
without forbidding the all-clear. With a BASELINE PRESENT, an absent
ledger is POST-ADOPTION DELETION — degraded exactly like a mismatch
(the baseline promised detection of every surviving change, and
deletion is a change): all-clear vetoed, display names `goal
reconcile` as the repair, and RECONCILE'S REPAIR HERE IS
RESTORATION (r10 finding 2: with no candidate bytes there is no
delta to replay): it rewrites goals.md from the baseline's full
accepted bytes and reports "restored from baseline" — this is why
the baseline keeps full bytes, twice over. Wanting the ledger gone
has a legal path (the verbs); rm is not it. A PRESENT-but-malformed ledger stays
degraded as before. Fresh adoption
ships the declaration; existing installations bootstrap when ready. Every field
above carries its byte bound (r2's G-15 tail: intent and id are
bounded too — id ≤64 bytes; everything projected anywhere is
bounded at its source). Done goals prune to the last ten. Stated honestly (r3 finding 11):
the ledger is NOT an audit log — goals mutated and pruned between
commits leave no git history, and the decisions documents remain the
audit surface; the prune verb reports what it dropped on stdout so
the operator can carry anything worth keeping there.

Scanner disjointness: `planFiles` excludes goals.md by name; only
the goal parser reads it. The handoff-plan "Next step" field remains
the per-stream in-flight authority; the verdict's precedence
(open-work first, below) is the one reconciliation point.

## Verbs and the transition table (r2's G-14)

Family `goal` — SIGNED (r5 finding 8, delegated decision D67): a
top-level family in the router, not `report goal-*` sub-verbs; the
doctrine commands humans and agents type (`goal open`, `goal next`)
are the surface, and a mutating ledger is not diagnostics-shaped.
Verbs: open, set-next, promote, park, unpark, done, reopen,
declare-free, prune, list, next, reconcile. Byte bounds are
COMPLETE (r4 finding 12): intent ≤160, next step ≤240, id ≤64,
Evidence ≤3 lines of ≤200 bytes, Parked-because ≤240, Concluded
≤240; the parser refuses any excess at write and flags it degraded
at read.

| From \ verb | open | promote | park | unpark | done | reopen | declare-free |
| --- | --- | --- | --- | --- | --- | --- | --- |
| (no such id) | → Current when no Current exists (the one-command program start — r5 finding 1); → Queued otherwise | refuse | refuse | refuse | refuse | refuse | n/a |
| Queued | refuse (duplicate) | → Current (refuse if one exists) | → Parked | refuse | refuse | refuse | n/a |
| Current | refuse | refuse (already) | → Parked; REQUIRES --then <queued-id> or --and-none exactly like done (r3 finding 4: parking the only Current may not manufacture an illegal ledger) | refuse | → Done; REQUIRES --then <queued-id> or --and-none (writes Goal-free) | refuse | n/a |
| Parked | refuse | refuse | refuse | → Queued | refuse | refuse | n/a |
| Done | refuse | refuse | refuse | refuse | refuse | → Queued, Origin preserved; REQUIRES --next "<step>" (Done blocks carry no step — r3 finding 4) | n/a |
| (ledger-level) | any open/promote/unpark/REOPEN drops an existing Goal-free declaration in the same atomic write — declaring intent supersedes declared absence (r3+r4 finding 4) | | | | | | legal only at zero Current AND zero Queued; --and-none REFUSES while Queued goals exist, naming them — declared absence with a standing queue is a contradiction the caller resolves by promoting or parking first; declare-free on an existing declaration RENEWS (the named idempotence exception) |

Every verb is idempotence-explicit: re-running a completed
transition refuses with the current state named (no silent
success). `set-next` rewrites the Current goal's step only.
Authority: all mutations holder-only through the record-writer-style
matrix path; `done`/`park` on Origin: human ADDITIONALLY requires a
HUMAN classification (advisory-grade per constraint 3).

Mutation discipline (r2's G-07, r3 findings 2+3, stated with the
honesty the threat model deserves): the flock + whole-ledger
compare-and-swap serializes COOPERATING writers — the verbs — and
that is all it claims. A non-cooperating editor racing a verb's
read-check-rename window can lose its edit, exactly as with every
other engine-owned file; the SUPPORTED manual-edit path is
edit-then-`goal reconcile`, and the docs say so. What the mechanism
does guarantee: any manual edit that SURVIVES is detected — the
accepted state is `plans/goals-accepted.json` (beside the ledger it
describes, NOT under disposable artifacts/ — r3's doctrine catch),
holding the last accepted ledger's full bytes and digest, written
AFTER the ledger in verb order; on crash between the two writes the
digest mismatches, the state degrades, and reconcile repairs — the
crash window is a named degraded state, never silence.
INITIALIZATION IS RECONCILE-ONLY (r4 finding 2, sharpened by r5
finding 2): adoption seeds goals.md AND goals-accepted.json together
(the adopter's plans-set rebuild and its exact-files fixture both
gain the pair). On an existing installation, ledger-without-baseline
has exactly ONE authority path: `goal reconcile`, which adopts the
ledger as the first accepted state after full grammar and
genesis-replay transition checks (every block reachable from empty).
Read verbs (list, next) NEVER mutate anything — not the ledger, not
the baseline; a MUTATING verb invoked on an unbaselined ledger
REFUSES with "no accepted baseline; run `goal reconcile` first" (it
must not trust bytes reconciliation never accepted); the verdict
reports the state as degraded naming the same command. One
transition, one owner, no verb-has-run marker to invent. RECONCILE
REPLAYS AUTHORITY (r3 finding 3): it computes the block-level delta
between the accepted bytes (which it has — this is why the baseline
keeps full bytes, not a digest alone) and the edited ledger, maps
the delta to transitions, and applies EVERY rule the direct verbs
apply — including the advisory HUMAN gate on human-origin done/park
and the origin-downgrade refusal. A manual edit can never reach a
state the equivalent verb sequence would have refused.

## The turn verdict: one verb, one structured decision (r2's G-01/G-06/G-09)

`report turn-verdict` returns:

    {"schemaVersion": 1,
     "shouldBlock": bool,
     "blockSource": "open-work" | "goal" | null,
     "openWork": [...],
     "openWorkSignature": "...",
     "goal": {"id","intent","nextStep","revision"} | null,
     "ledgerStatus": "ok" | "absent" | "degraded" | "goal-free" | "queued-only",
     "diagnostics": ["..."],
     "display": "..."}

- **QUEUED-ONLY is a defined verdict state** (r5 finding 1): a
  legal ledger with zero Current and a non-empty queue (reachable
  via reopen, which drops Goal-free) yields goal=null,
  ledgerStatus="queued-only"; when the scanner is all-empty it
  blocks ONCE with "no current goal; the queue holds <first queued
  id>: `goal promote <id>` or park it", keyed on the sha256 of the
  Queued section bytes stored in blockedGoalRevisions like any
  revision — never a silent all-clear, never arbitrary selection,
  never forced promotion.
- **The revision is DEFINED** (r3 finding 5): sha256 of the Current
  block's exact bytes (heading through its last field line). An
  identical reopened goal therefore does NOT re-block — intended: an
  unchanged ask is the same ask, and any wording change to the step
  re-arms it. Unrelated queued/done edits never re-arm (the hash
  scopes to the Current block alone).
- **The block-state file, fully specified** (r4 finding 7):
  `artifacts/agents/turn-verdict-state.json` = {"schemaVersion": 1,
  "sessions": {sessionId: {"lastTouched": ISO, "openWorkSignature":
  str, "blockedGoalRevisions": [≤64, FIFO-evicted],
  "blockedFreeDigests": [≤16, FIFO-evicted]}}}. lastTouched updates
  on every write; sessions with lastTouched older than 30 days drop
  on any write; the sessions MAP ITSELF is capped at 128 entries
  with oldest-lastTouched eviction on overflow (r5 finding 6: expiry
  is not a cardinality bound). SessionId hygiene happens ONCE AT THE
  HOOK BOUNDARY (r6 finding 4): the hook derives safe_session on
  entry — accepted only matching ^[A-Za-z0-9._-]{1,128}$, anything
  else replaced by its sha256 hex — and every downstream use rides
  it. The per-session watchdog-surfaced files are RETIRED INTO THE
  MAP (r7 finding 4: 30-day pruning of loose files is an age rule,
  not a count bound), AS A PROTOCOL, not a bare field (r8 finding
  4): the hook passes --watchdog-surfaced <sha256-of-the-watchdog-
  report> (fixed 64 bytes — the raw text is unbounded and never
  stored); the verdict RESPONSE gains "surfaceWatchdog": bool — the
  verb decides under the flock whether this digest is NEW for the
  session (surface it, store it) or already stored (suppress), so
  concurrent Stop calls surface exactly once; a verdict call WITHOUT
  the flag (no watchdog findings this turn) CLEARS the stored
  digest, restoring today's recover-then-warn-again lifecycle. The
  session entry stores "watchdogSurfaced": str|null (the digest).
  The one capped, pruned, flocked map is the ONLY Stop-state on
  disk. The caps bound Stop latency and storage by construction. Flock with a 2-SECOND DEADLINE (the installed Stop
  hook runs a 5-second budget; a wedged lock degrades, never
  hangs); atomic write; check-and-record atomic under the lock;
  concurrent Stop calls serialize there.
- **The scanner grows a STRUCTURED result first** (r4 finding 5 —
  a named prerequisite, in this change's scope): openwork gains
  `ScanResult{Open []Item, WaitingOnHuman []Item, StalePlans []Item,
  Busy []Item, Unreadable []string}` — Busy is an INVENTORY, not a
  bool (r9 finding 5: the hook's display names which jobs, missions,
  and gates are live; a bool would force a second scanner or lose
  the detail), each Item carrying kind (job|mission|gate), id, and
  a bounded detail string (≤200 bytes — the display line the
  scanner already composes today, e.g. "role jobId [status,
  runtime]"; r10 finding 3: kind+id alone cannot reproduce the
  preserved output, and the hook renders detail verbatim with no
  second scan); emptiness is the idle test. Busy keeps r5's three classes
  but from CHECKOUT-SCOPED FILE FACTS ONLY (r6 finding 2: argv
  matching answers for the whole machine — gaterun.go documents
  exactly that defect): live delegate job records under this root
  (already scoped), gate-run markers via internal/gaterun (already
  checkout-scoped, alive-checked by kernel facts), and the mission
  runner records under this root correlated by status.go's existing
  record-plus-kernel-identity rule (r7 finding 3: r7's earlier
  "heartbeat freshness rule" named a rule that does not exist —
  heartbeats are written but nothing reads their freshness, and
  sidecars survive completion; the status verb's rule is the one
  authority, reused by reference). The verdict path retires argv
  matching; the scanner supplies these facts to every consumer, so
  another checkout's mission can never suppress this checkout's
  goal. The gate inventory becomes LOSSLESS at its edges (r7
  finding 2): Register writes its marker atomically (temp+rename —
  today's plain write races the scanner's malformed-delete) and
  returns NONEMPTY-OR-ERROR — the "unreadable identity, record
  nothing, report success" path is retired (r8 finding 3); the
  suite's registration call stops suppressing errors (a gate that
  cannot record its liveness fails loudly at startup instead of
  running invisibly). REGISTRATION FOLLOWS THE REAL SNAPSHOT
  TOPOLOGY (r9 finding 3: the clean gate runs in a temporary
  snapshot and copies its binary to the serving checkout only after
  it returns — a snapshot-side registration marks the WRONG root):
  the PARENT running in the serving checkout registers the gate run
  against the SERVING root with its own pid for the gate's full
  span, using whichever binary exists first (the previous serving
  binary, else the snapshot's fresh one by path), and unregisters
  on return; the snapshot's own self-registration marks only the
  snapshot and dies with it. The named residual shrinks to the
  truly-clean window before ANY binary exists anywhere, bounded by
  the first build. The standalone go-gate.sh registers itself
  through the binary it gates, closing today's
  unrecorded-supported-gate hole; and the reader classifies per
  marker —
  unreadable or unparsable markers of a LIVE process append to
  Unreadable (today both are silently skipped or deleted; deletion
  remains correct only for dead-process markers). ANY
  inventory-source failure — job-record read error, gate-marker
  enumeration or per-marker read failure, runner-record read error
  — appends to Unreadable (r6 finding 3: enumeration failure must
  not collapse to idle), so the veto below covers unknown activity
  by the same rule; the legacy
  []string surface remains as a formatter over it for existing
  callers. The verdict verb consumes ScanResult — precedence is then
  DECIDABLE: Busy → no goal clause (an active checkout needs no
  prodding); Open non-empty → open-work block, today's semantics;
  WaitingOnHuman non-empty (and Open empty) → the wait reported,
  goal clause suppressed (no contradictory imperative); StalePlans
  stay warning-only and NEVER block (r4's accidental-blockable
  catch); all empty → the goal blocks once with "open work is done;
  the goal file names the next step: <step verbatim>". UNREADABLE
  HAS A DECISION OUTCOME (r5 finding 5): non-empty Unreadable vetoes
  BOTH the all-clear sentence AND any goal block for that turn — the
  scanner cannot assert "nothing left" over inputs it could not
  read, and must not prod with a goal when unread files may hold
  open work; the paths surface in diagnostics AND in the
  non-blocking display ("N inputs unreadable: <paths>"), warning
  every turn until readable, never blocking by themselves. Goal-free
  reports its declaration or staleness block; DEGRADED ledger: the
  verdict sets shouldBlock=false, ledgerStatus=degraded, suppresses
  the all-clear sentence, and the display explains — degradation
  warns loudly and vetoes 'nothing left', but never fabricates a
  block (r4 finding 6's choice, made); ABSENT stays advisory.
- **The final hook envelope, defined** (r3 findings 6+7 close r2's
  G-09): the verdict's display becomes the BLOCK REASON byte-verbatim
  when shouldBlock; the hook's other messages — watchdog,
  protocol-growth, the busy/idle sentence — compose into the
  NON-BLOCKING channel (Claude: systemMessage) exactly as today, and
  never enter the block reason. Correcting r3's own catch of my r3
  error: the watchdog path CALLS the verdict verb like every other
  path — it suppresses only its own relaunch side-effects, never the
  goal decision. The advisor early-exit alone bypasses the verb
  (an advisor turn ends no work). Transport of degraded: every
  REPRESENTABLE state is exit 0 with JSON (degraded included, as
  ledgerStatus); verb nonzero means I/O failure, and the hook
  contract requires the hook to emit its own FIXED degraded
  systemMessage naming the failure ("turn-verdict unavailable:
  <stderr line>") AND suppresses its own all-clear/"NOTHING LEFT"
  sentence for that turn (r4 finding 6: the fixed degraded message
  may never compose with an all-clear it cannot vouch for) —
  hook-side, so it is representable even when the verb cannot
  speak; the current 2>/dev/null||true suppression is the named
  defect the contract removes. The scanner's silently
  dropped unreadable inputs (r3 finding 6 tail) become diagnostics
  entries via a scoped openwork API change: unreadable plan or
  record files are REPORTED, not skipped.

## Delivery: the runtime contract and the conformance table (D66)

docs/design/ gains the TURN-VERDICT DELIVERY CONTRACT: what an
adapter must do to claim conformance (invoke the verb at turn end,
honor shouldBlock via its runtime's mechanism, transport display
verbatim, never suppress degraded). The conformance table describes
THE DISTRIBUTION, not any installation (r4 finding 10's choice,
made): it ships in the contract document unchanged through adoption
— which runtimes THIS checkout installed is already answerable from
metasystem.conf and is not the table's job; the instruction-audit
owner (internal/audit, named in the matrix) checks the table's
claims against the shipped enforcement configs. Four states per
runtime (declared / installed / observed / blocking-capable):
claude starts at installed-with-fixture-proven-EMISSION — the
fixture proves the hook emits decision:block, and the row says
exactly that; blocking-capable is claimed only after a live
observation, which the table records by date when it happens (r3
finding 10: the evidence column never overclaims). codex and devin
start at declared (their shipped Stop configs exist; observation
pending — item 16's audit keeps the rows honest). ORCHESTRATOR
parity does not wait for hooks: any runtime's main can read `goal
next` by instruction — the AGENTS.md turn-end amendment SHIPS WITH
THIS CHANGE and is in the doctrine list and obligations below (r3
finding 10: it was claimed but unlisted) — which
under exchangeability is the same information on the only universal
transport. The mechanism is identical for every runtime; only
delivery automation differs, and the table says so in public.

## Mission hosts (D66, question 4)

The orientation line is owned where the prompt is owned (r3+r4
finding 9), with the EXACT contract: AssemblePrompt emits, between
the mission-intent section and the streams section, the optional
block `## Serving goal\n<id> — <intent>\n` (one heading, one line,
no fields); internal/validate's turn-prompt grammar gains that
optional block at exactly that position. The assembler and validator
change in the same commit or the prompt gate refuses every turn.
Reading goals.md rides the goal parser (read-only, no new
authority); a missing, absent, or degraded ledger produces NO line —
prompt assembly never degrades and never blocks on goal state.
Runner-side and runtime-neutral: every host of every runtime gets
the same line the same way. Hosts do not mutate goals, ENFORCED
AT THE VERB ON A RECORDED FACT (r5 finding 7 → r6 finding 1 → r7
finding 1, the final form): every goal-MUTATION verb refuses while
the checkout has an ACTIVE MISSION — decided by the SAME rule the
mission status verb already applies (the runner record plus kernel
process identity, status.go's authority: a record claiming
"running" counts only while the recorded runner process is actually
alive). TWO DEFECTS IN THAT FACT'S SUBSTRATE ARE IN SCOPE (r8
findings 1+2 — the goal system depends on the fact, so it ships the
fixes): (a) record publication becomes LEASE-SERIALIZED (r9 finding 1
KILLED the r8 self-identity check: a loser that overwrites the
record with its own identity then passes its own check and marks
the live winner failed — checking identity against bytes you
yourself wrote is circular): a runner acquires the mission lease
FIRST and publishes the shared runner record only as the WINNER;
the loser never writes the shared record at all, and finalization
re-verifies lease ownership before every terminal write. The test
contends A and B for the lease and proves the loser NEITHER
publishes NOR finalizes while the winner's record survives
byte-identical (the schedule the fix makes possible — r10 finding 4
corrected the earlier impossible ordering); (b) the
identity package's THREE outcomes propagate uncollapsed: Live means
active, Dead means not active, and UNKNOWN means indeterminate —
Unknown joins ScanResult.Unreadable (vetoing all-clear and goal
block alike) and goal MUTATION during Unknown REFUSES fail-closed
(an unverifiable runner cannot authorize rewriting intent; today
proc.go folds Unknown into Dead). Lease lineage was the r6 answer and is WRONG for attended
missions (live-holder launches deliberately preserve the prior
main's lineage — lease-succession.md); environment markers were the
r5 answer and are strippable. Active-mission is total across
attended and unattended, on-disk, and runtime-agnostic. The refusal
reads "goal mutation while a mission is active is refused; the
mission's intent is the contract — conclude or park the mission
first", which is also the human's path mid-attended-mission. Read
verbs and the verdict remain available everywhere.

## Delegates (D66, question 1 — the human's ruling)

`dispatch` gains `--serving-goal` (no value): when the ORCHESTRATOR
passes it, the section is appended to the brief BEFORE the brief is
hashed and stored (r3 finding 9): it is part of the recorded brief
bytes and the input hash, so it survives the fresh-context fallback
and every follow-up rebuild deterministically — one projection
decision per chain, made at dispatch, never re-evaluated mid-chain:

    # Serving goal (context, not instruction)
    <id> — <intent>

`--serving-goal` with NO usable Current goal (absent ledger, no
Current, degraded) REFUSES the dispatch loudly at setup (exit 3,
"no current goal to project") — the orchestrator asked for a
projection that does not exist, and a silent no-op would record a
brief hash that lies about intent (r4 finding 9). Default OFF. Per dispatch, never per role, never global. The section
confers zero authority (the envelope, schema, and certification
govern exactly as before); its text is quoted data bounded at the
ledger (id ≤64, intent ≤160). Queued/Parked/Done goals never
project. The projection is brief-carried plain text — the only
transport every runtime receives (exchangeability).

## Item-15 composition (read-side only, unchanged from r2)

Run records own conditional continuations. Item 14 ships NO
enrichment: the goal PARSER is an exported function with exactly
THREE consumers in this change — the verdict verb,
mission.AssemblePrompt, and the Go dispatch setup core resolving
--serving-goal (r6 finding 5; r4 finding 11 named the first two).
THE OWNERSHIP GRAPH IS ACYCLIC BY CONSTRUCTION (r9 finding 2:
parking everything in internal/report cannot compile —
report→missionrunner→mission/dispatch→report): a NEW package
internal/goal owns the ledger grammar, parser, verbs, and the turn
verdict; a NEW LEAF internal/missionstate owns the active-mission
rule (the record+kernel-identity authority extracted from
missionrunner's status path, which becomes its first consumer).
Edges: goal→missionstate; mission→goal; dispatch→goal;
missionrunner→{mission, dispatch, missionstate}; report→goal. THE
SCAN BOUNDARY RIDES THAT LAST EDGE (r10 finding 1): ScanResult is
DEFINED IN internal/goal — it is the verdict's input contract — and
internal/report's scanner produces a goal.ScanResult (report
imports goal, the declared direction; the verdict never imports
report). No cycle exists; every GOAL row's owner/code target reads
accordingly (goalverbs.go and turnverdict.go live in
internal/goal). Item 15's design owns run enrichment with its own
tests when it exists.

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

internal/goal (NEW: ledger grammar, parser, verbs, turn-verdict
verb, state file, ScanResult), internal/missionstate (NEW leaf: the
active-mission rule), internal/report (scanner produces
goal.ScanResult; goals.md scanner exclusion), internal/dispatch
(the --serving-goal brief section), internal/missionrunner
(lease-serialized record publication + the turn-prompt orientation
line; mutation refusal lives in internal/goal on the missionstate
fact), internal/gaterun (atomic nonempty-or-error Register),
cmd/metasystem (verb rows),
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
  byte-identically; the ADVISOR path alone bypasses the verdict
  verb; the WATCHDOG path calls it like every other (r4 finding 8:
  the r4 proof line contradicted the normative prose — the prose was
  right).
- Projection: --serving-goal appends exactly the bounded section
  and REFUSES exit-3 when no usable Current goal exists (r6 finding
  5 caught this proof line contradicting the normative prose — the
  prose governs; "absence appends nothing" describes only the
  MISSION projection, which omits its optional block); oversized
  ledger fields are refused at the LEDGER, so the brief builder
  never truncates.
- Runner: the orientation line in assembled turn prompts (fixture);
  goal mutation from runner context refused.
- Incident regression: as specified above, end to end, unseeded.
- Conformance: the table's claude row proven by the hook fixture;
  codex/devin rows asserted "declared" by reading the shipped
  enforcement configs (their observation upgrade is item 16's).

## Design-obligation matrix (the readiness gate, r3 finding 13)

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GOAL-01 | CRITICAL | The ledger: staleness rule | A Goal-free declaration whose scan digest no longer matches blocks once and never reads as all-clear | internal/goal goal parser + verdict | internal/goal/goal.go | TestGoalFreeStaleness | fixture leg: declare, add a stream, verdict blocks | PARTIAL | implement |
| GOAL-02 | CRITICAL | Mutation discipline | Reconcile replays FULL transition authority over the accepted-bytes delta; a manual edit never reaches a state the verbs would refuse | internal/goal | internal/goal/goalverbs.go | TestReconcileReplaysAuthority (human-origin done via edit refused for MAIN) | fixture: edit + reconcile round-trip | PARTIAL | implement |
| GOAL-03 | CRITICAL | Mutation discipline | Ledger-then-accepted write order; crash between the two degrades and reconcile repairs; accepted state lives at plans/goals-accepted.json | internal/goal | internal/goal/goalverbs.go | TestAcceptedStateCrashWindow | fixture: kill between writes | PARTIAL | implement |
| GOAL-04 | CRITICAL | The turn verdict | Dual-slot block-once: the G-01 sequence cannot re-block; state flock bounded at 2s; concurrent Stop calls serialize; sessions map capped at 128 oldest-evicted; sessionId normalized once at the hook boundary (regex-or-sha256); watchdog-surfaced state lives IN the map via --watchdog-surfaced (loose per-session files retired — the map is the only Stop-state) | internal/goal verdict + hook | turnverdict.go + supervision-hook.sh | TestVerdictDualSlotSequence + goroutine race test + TestSessionMapCapAndHygiene + TestWatchdogProtocol (changed-same-clear-same + concurrent exactly-once via surfaceWatchdog) | hook boundary fixture | PARTIAL | implement |
| GOAL-05 | CRITICAL | Hook envelope | Block reason is the display byte-verbatim; watchdog/protocol-growth stay in the non-blocking channel; verb I/O failure yields the hook's fixed degraded message, never silence | supervision-hook.sh + hook contract doc | scripts/agents/supervision-hook.sh | supervision fixture leg asserting byte-identity and the degraded path | claude hook fixture | PARTIAL | implement |
| GOAL-06 | HIGH | Transition table | park/done on the only Current require --then/--and-none; reopen requires --next; open/promote/unpark drop a standing Goal-free atomically | internal/goal | internal/goal/goalverbs.go | TestTransitionTableMatrix (every cell incl. refusals) | — | DONE | — |
| GOAL-07 | HIGH | Precedence | Human-waiting streams suppress the goal clause; goal blocks only when the scanner reports nothing | internal/goal verdict | internal/goal/turnverdict.go | TestPrecedenceWaitingOnHuman | fixture leg | PARTIAL | implement |
| GOAL-08 | HIGH | Delegates | --serving-goal section is inside the stored brief bytes and input hash; survives fresh-context fallback; resolved by the Go dispatch setup core through the exported parser; refuses exit-3 with no usable Current goal | internal/dispatch + scripts/agents/dispatch.sh | dispatch setup core + brief builder | TestServingGoalResolvesAndRefuses | dispatch fixture leg | PARTIAL | implement |
| GOAL-09 | HIGH | Mission hosts | AssemblePrompt emits the optional section and the turn-prompt validator accepts it; goal absence produces no line and never blocks assembly | internal/mission + internal/validate | prompt.go + turnprompt.go | TestPromptGoalSection both ways | runner fixture leg | PARTIAL | implement |
| GOAL-10 | HIGH | Absence semantics | Pre-adoption absence (no baseline) is advisory; post-adoption deletion (baseline present, ledger absent) is degraded with all-clear vetoed and reconcile named | internal/goal | turnverdict.go | TestAbsenceAdvisoryVsDeletionDegraded | — | DONE | — |
| GOAL-11 | HIGH | Delivery contract | Conformance table rows carry only evidenced states; AGENTS.md turn-end amendment ships | docs/design + AGENTS.md | contract doc | audit leg: table matches shipped configs | live blocking observation upgrades claude's row by date | PARTIAL | implement |
| GOAL-12 | MEDIUM | Item-15 seam | The verdict verb's goal-facts read interface is exported and UNCONSUMED in item 14 | internal/goal | turnverdict.go | compile-level + a no-enrichment assertion | — | PARTIAL | implement |
| GOAL-13 | MEDIUM | Ledger honesty | Prune reports dropped blocks on stdout; docs state the ledger is not an audit log | internal/goal | goalverbs.go | TestPruneReportsDrops | — | DONE | — |
| GOAL-14 | CRITICAL | Mutation discipline: initialization | Adoption seeds goals.md + goals-accepted.json together; ledger-without-baseline degrades and reconcile bootstraps via genesis replay | scripts/adopt.sh + internal/goal | adopt.sh + goalverbs.go | adopt fixture pair-assertion + TestReconcileGenesis | adopt fixture | PARTIAL | implement |
| GOAL-15 | HIGH | Goal-free staleness | declare-free renews (the named idempotence exception); stale digests block once via blockedFreeDigests | internal/goal | goalverbs.go + turnverdict.go | TestGoalFreeRenewAndBlockOnce | fixture leg | PARTIAL | implement |
| GOAL-16 | HIGH | Scanner facts | openwork exposes ScanResult; Busy is an inventory of Items (kind job|mission|gate + id) from checkout-scoped FILE facts only, correlated by internal/missionstate's record-plus-kernel-identity rule — argv matching retired; the hook display keeps its detail; stale plans never block | internal/report + internal/missionstate | openwork.go + runningwork.go + gaterun | TestScanResultClassification covering live, completed, crashed, stale-sidecar, identity-mismatch, and identity-UNKNOWN runners (Unknown joins Unreadable, never dead) + TestOtherCheckoutNeverSuppresses | supervision fixture leg | PARTIAL | implement |
| GOAL-17 | HIGH | Unreadable safety | Non-empty Unreadable vetoes both the all-clear and any goal block; inventory-source failures join Unreadable at every edge: gate Register atomic (temp+rename), suite registration failure fatal at gate startup, live-process unreadable/unparsable markers surface (dead-only deletion), job/runner record read errors surface — enumeration failure never collapses to idle | internal/report + internal/gaterun + suite | openwork.go + runningwork.go + gaterun.go + validate-metasystem.sh + turnverdict.go | TestUnreadableVetoesBothOutcomes + TestInventoryFailureVetoes + TestGateMarkerEdges (register-scan race, live-unreadable, dead-delete, nonempty-or-error Register, parent serving-root registration across the snapshot span, go-gate self-registration) | hook transport leg | PARTIAL | implement |
| GOAL-18 | MEDIUM | Delivery contract audit | The instruction audit checks the conformance table's rows against shipped enforcement configs | internal/audit | metasystem.go | TestConformanceTableAudit | — | PARTIAL | implement |
| GOAL-19 | CRITICAL | Program-start doctrine is audited | The instruction audit content-checks the doctrine's program-start rule (programs start with `goal open`) — the sole compensating control for the accepted blind spot | internal/audit | metasystem.go | TestDoctrineProgramStartRule | — | PARTIAL | implement |
| GOAL-20 | CRITICAL | One-command start lands actionable | `goal open` on a no-Current ledger creates the Current goal; queued-only ledgers get the defined block-once verdict, never a silent all-clear | internal/goal | goalverbs.go + turnverdict.go | TestOpenPromotesWhenEmpty + TestQueuedOnlyVerdict | — | DONE | — |
| GOAL-21 | HIGH | Mission seat cannot mutate | Every goal-mutation verb refuses while the checkout has an ACTIVE mission per status.go's record-plus-kernel-identity rule — total across attended and unattended, env-strip immune; reads and the verdict stay available | internal/goal + internal/missionstate | goalverbs.go | TestGoalMutationRefusesActiveMission covering attended (live-holder lineage), unattended, env-stripped, dead-runner (mutation allowed), and identity-Unknown (mutation refused fail-closed) cases | — | DONE | — |
| GOAL-22 | CRITICAL | Mission-fact integrity | Runner-record publication is lease-serialized: only the lease winner writes the shared record; losers never write it; finalization is owner-only before release | internal/missionrunner + internal/missionstate | loop.go internalRun/finishRunner + missionstate.Classify | TestOverlappingResumeKeepsWinnerRecord (loser neither publishes nor finalizes; winner record byte-identical) + TestClassifyThreeWay + TestSurveyClassifiesAndSurfaces | — | DONE | — |
