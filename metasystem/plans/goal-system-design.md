# The goal system: the thread of intent survives every turn boundary

Working Mode: design

Owner: main session (delegate), backlog item 14. Status: r5 — folds
the thirteen r4 findings (critiques at
plans/goal-system-critique-r{1..4}.md). The obligation matrix passes
the gate's STRUCTURAL check; its rows stay PARTIAL until
implementation, so the gate exits 1 by design until completion (r4's
"satisfies" wording was wrong and is retracted). Human rulings (D66)
remain fixed input. Awaiting r5 critique.

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
blocks once, a further world change blocks once more. An ABSENT ledger on an
existing installation is ADVISORY, never degraded (r3 finding 11's
upgrade cliff): the verdict reports "no goal ledger; `goal open`
starts one" without blocking and without forbidding the all-clear —
only a PRESENT-but-malformed ledger is degraded. Fresh adoption
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

Family `goal` (name needs the usual verb sign-off; `report goal-*`
is the fallback): open, set-next, promote, park, unpark, done,
reopen, declare-free, prune, list, next, reconcile. Byte bounds are
COMPLETE (r4 finding 12): intent ≤160, next step ≤240, id ≤64,
Evidence ≤3 lines of ≤200 bytes, Parked-because ≤240, Concluded
≤240; the parser refuses any excess at write and flags it degraded
at read.

| From \ verb | open | promote | park | unpark | done | reopen | declare-free |
| --- | --- | --- | --- | --- | --- | --- | --- |
| (no such id) | → Queued (or → Current with --current when none current) | refuse | refuse | refuse | refuse | refuse | n/a |
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
INITIALIZATION (r4 finding 2): adoption seeds goals.md AND
goals-accepted.json together (the adopter's plans-set rebuild and
its exact-files fixture both gain the pair); on an EXISTING
installation, the first `goal` verb ever run initializes the
baseline from the ledger it accepts — and when a ledger exists with
NO baseline and no verb has run, the verdict treats it as degraded
with the display naming `goal reconcile` as the bootstrap, which
adopts the ledger as the first accepted state after full grammar and
zero-history transition checks (every block must be reachable from
empty — reconcile's replay handles genesis). RECONCILE
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
     "ledgerStatus": "ok" | "absent" | "degraded" | "goal-free",
     "diagnostics": ["..."],
     "display": "..."}

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
  on any write; the caps bound Stop latency and storage by
  construction. Flock with a 2-SECOND DEADLINE (the installed Stop
  hook runs a 5-second budget; a wedged lock degrades, never
  hangs); atomic write; check-and-record atomic under the lock;
  concurrent Stop calls serialize there.
- **The scanner grows a STRUCTURED result first** (r4 finding 5 —
  a named prerequisite, in this change's scope): openwork gains
  `ScanResult{Open []Item, WaitingOnHuman []Item, StalePlans []Item,
  Busy bool, Unreadable []string}` (Busy = live delegate jobs or
  gate runs, the fact the hook reads separately today); the legacy
  []string surface remains as a formatter over it for existing
  callers. The verdict verb consumes ScanResult — precedence is then
  DECIDABLE: Busy → no goal clause (an active checkout needs no
  prodding); Open non-empty → open-work block, today's semantics;
  WaitingOnHuman non-empty (and Open empty) → the wait reported,
  goal clause suppressed (no contradictory imperative); StalePlans
  stay warning-only and NEVER block (r4's accidental-blockable
  catch); all empty → the goal blocks once with "open work is done;
  the goal file names the next step: <step verbatim>". Unreadable
  inputs populate diagnostics (never silently dropped). Goal-free
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
the same line the same way. Hosts do not mutate goals (they are
lease holders mid-mission; goal mutation from inside a mission is
refused by a runner-context check — the mission's intent is the
contract, not the goal file).

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
enrichment: the goal PARSER is an exported function with exactly TWO
consumers in this change — the verdict verb and
mission.AssemblePrompt (r4 finding 11: the mission projection reads
through the same parser, no duplication, and the design says so
rather than leaving it to be discovered); item 15's design owns run
enrichment with its own tests when it exists.

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
  byte-identically; the ADVISOR path alone bypasses the verdict
  verb; the WATCHDOG path calls it like every other (r4 finding 8:
  the r4 proof line contradicted the normative prose — the prose was
  right).
- Projection: --serving-goal appends exactly the bounded section;
  absence appends nothing; oversized ledger fields are refused at
  the LEDGER, so the brief builder never truncates.
- Runner: the orientation line in assembled turn prompts (fixture);
  goal mutation from runner context refused.
- Incident regression: as specified above, end to end, unseeded.
- Conformance: the table's claude row proven by the hook fixture;
  codex/devin rows asserted "declared" by reading the shipped
  enforcement configs (their observation upgrade is item 16's).

## Design-obligation matrix (the readiness gate, r3 finding 13)

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GOAL-01 | CRITICAL | The ledger: staleness rule | A Goal-free declaration whose scan digest no longer matches blocks once and never reads as all-clear | internal/report goal parser + verdict | internal/report/goal.go | TestGoalFreeStaleness | fixture leg: declare, add a stream, verdict blocks | PARTIAL | implement |
| GOAL-02 | CRITICAL | Mutation discipline | Reconcile replays FULL transition authority over the accepted-bytes delta; a manual edit never reaches a state the verbs would refuse | internal/report goal verbs | internal/report/goalverbs.go | TestReconcileReplaysAuthority (human-origin done via edit refused for MAIN) | fixture: edit + reconcile round-trip | PARTIAL | implement |
| GOAL-03 | CRITICAL | Mutation discipline | Ledger-then-accepted write order; crash between the two degrades and reconcile repairs; accepted state lives at plans/goals-accepted.json | internal/report goal verbs | internal/report/goalverbs.go | TestAcceptedStateCrashWindow | fixture: kill between writes | PARTIAL | implement |
| GOAL-04 | CRITICAL | The turn verdict | Dual-slot block-once: the G-01 sequence cannot re-block; state flock bounded at 2s; concurrent Stop calls serialize | internal/report verdict | internal/report/turnverdict.go | TestVerdictDualSlotSequence + goroutine race test | hook fixture | PARTIAL | implement |
| GOAL-05 | CRITICAL | Hook envelope | Block reason is the display byte-verbatim; watchdog/protocol-growth stay in the non-blocking channel; verb I/O failure yields the hook's fixed degraded message, never silence | supervision-hook.sh + hook contract doc | scripts/agents/supervision-hook.sh | supervision fixture leg asserting byte-identity and the degraded path | claude hook fixture | PARTIAL | implement |
| GOAL-06 | HIGH | Transition table | park/done on the only Current require --then/--and-none; reopen requires --next; open/promote/unpark drop a standing Goal-free atomically | internal/report goal verbs | internal/report/goalverbs.go | TestTransitionTableMatrix (every cell incl. refusals) | — | PARTIAL | implement |
| GOAL-07 | HIGH | Precedence | Human-waiting streams suppress the goal clause; goal blocks only when the scanner reports nothing | internal/report verdict | internal/report/turnverdict.go | TestPrecedenceWaitingOnHuman | fixture leg | PARTIAL | implement |
| GOAL-08 | HIGH | Delegates | --serving-goal section is inside the stored brief bytes and input hash; survives fresh-context fallback | scripts/agents/dispatch.sh + brief builder | dispatch.sh brief path | dispatch fixture: hash equality + fallback rebuild carries the section | — | PARTIAL | implement |
| GOAL-09 | HIGH | Mission hosts | AssemblePrompt emits the optional section and the turn-prompt validator accepts it; goal absence produces no line and never blocks assembly | internal/mission + internal/validate | prompt.go + turnprompt.go | TestPromptGoalSection both ways | runner fixture leg | PARTIAL | implement |
| GOAL-10 | HIGH | Upgrade rule | Absent ledger is advisory (no block, no all-clear veto); malformed is degraded | internal/report verdict | internal/report/turnverdict.go | TestAbsentVsMalformedLedger | — | PARTIAL | implement |
| GOAL-11 | HIGH | Delivery contract | Conformance table rows carry only evidenced states; AGENTS.md turn-end amendment ships | docs/design + AGENTS.md | contract doc | audit leg: table matches shipped configs | live blocking observation upgrades claude's row by date | PARTIAL | implement |
| GOAL-12 | MEDIUM | Item-15 seam | The verdict verb's goal-facts read interface is exported and UNCONSUMED in item 14 | internal/report | turnverdict.go | compile-level + a no-enrichment assertion | — | PARTIAL | implement |
| GOAL-13 | MEDIUM | Ledger honesty | Prune reports dropped blocks on stdout; docs state the ledger is not an audit log | internal/report goal verbs | goalverbs.go | TestPruneReportsDrops | — | PARTIAL | implement |
| GOAL-14 | CRITICAL | Mutation discipline: initialization | Adoption seeds goals.md + goals-accepted.json together; ledger-without-baseline degrades and reconcile bootstraps via genesis replay | scripts/adopt.sh + internal/report | adopt.sh + goalverbs.go | adopt fixture pair-assertion + TestReconcileGenesis | adopt fixture | PARTIAL | implement |
| GOAL-15 | HIGH | Goal-free staleness | declare-free renews (the named idempotence exception); stale digests block once via blockedFreeDigests | internal/report | goalverbs.go + turnverdict.go | TestGoalFreeRenewAndBlockOnce | fixture leg | PARTIAL | implement |
| GOAL-16 | HIGH | Scanner facts | openwork exposes ScanResult (Open/WaitingOnHuman/StalePlans/Busy/Unreadable); legacy strings become a formatter; stale plans never block | internal/report | openwork.go | TestScanResultClassification | supervision fixture leg | PARTIAL | implement |
| GOAL-17 | HIGH | Diagnostics | Unreadable plan/record inputs are reported in diagnostics, never silently dropped | internal/report | openwork.go | TestUnreadableInputsSurface | — | PARTIAL | implement |
| GOAL-18 | MEDIUM | Delivery contract audit | The instruction audit checks the conformance table's rows against shipped enforcement configs | internal/audit | metasystem.go | TestConformanceTableAudit | — | PARTIAL | implement |
