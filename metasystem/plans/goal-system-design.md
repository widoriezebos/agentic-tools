# The goal system: the thread of intent survives every turn boundary

Working Mode: design

Owner: main session (delegate), backlog item 14, under Wido's
2026-08-14 night ruling 4 (design pass tonight, DESIGNS ONLY —
implementation is a separate, later decision). Status: r1, awaiting
first critique.

## The problem, in the human's words and one incident

"We lose track of the goal that we are chasing." The motivating
incident: the stop hook told the human "NOTHING LEFT TO WORK ON in
this checkout" while roughly a quarter of a 101-finding program
remained — the backlog lived in docs/reviews/, invisible to the
open-work scanner. The narrow fix (a plans/ note) made the scanner
see THAT work exists; the verdict still cannot say WHAT to do next,
so an idling orchestrator stays idle and a human reads "nothing left"
while the program's thread of intent sits in a file no machinery
consumes.

Tonight added a second shape of the same loss, one level down: two
benchmark hosts spent three cycles critiquing and never dispatched an
implementer, because nothing in front of them named the next step
that mattered (D58's control case: the same host shape with runway
used it). The mission path has its own prompt machinery and is NOT
this design's surface, but the incident sharpens the requirement:
naming the next step verbatim changes behavior; knowing work exists
does not.

## What exists today

- The open-work scanner (internal/report) reads plans/ streams and
  answers THAT open work exists; the stop-block refuses to end a turn
  once per open-work set. It has no concept of which step comes next.
- plans/ is the sanctioned home for task-local state and standing
  ledgers (wow.md's routing rule), read by humans and by the scanner.
- The mission ledger demonstrates the house pattern this design
  reuses: a markdown file humans read and edit, with a strict line
  grammar that machinery parses, mutated only through engine verbs
  under a lock.
- Only Claude sessions have a turn-end hook today (backlog item 16's
  finding); codex and devin agents get no turn-end machinery at all.

## Design

### 1. One file, ledger-grade grammar: plans/goals.md

A markdown document, human-readable and human-editable, with a
machine grammar (the mission-ledger pattern, not a new document
kind):

    # Goals

    ## Goal: <kebab-case-id> — <one-line intent>
    - Status: active | parked | done
    - Next step: <one imperative sentence, verbatim-quotable>
    - Why it matters: <one sentence>            (optional)
    - Evidence: <path or D-entry>               (optional)
    - Parked because: <reason>                  (required iff parked)

Rules the grammar enforces: at most ONE `Next step` line per goal;
an `active` goal REQUIRES a `Next step`; `done` goals keep their
block (history stays); ids are unique. The file is the single
checkout-level intent surface — programs, backlogs, and "after X do
Y" hand-offs all become goals with next steps, instead of prose in
files machinery cannot see.

### 2. Engine verbs own every mutation

A `goal` verb family (or `report goal-*` if the family budget is
tight — naming needs the sign-off that all new verbs need):

- `goal list` — parse and print goals with status, machine-readable.
- `goal next` — print the active goals' next steps, verbatim, in
  file order; exit 3 when none.
- `goal set --id X --next "<step>"` / `goal done --id X` /
  `goal park --id X --because "<reason>"` — mutations, atomic
  rewrite under the same discipline as every other owned document,
  authority-checked like other control-plane writes (holder-only:
  goals are the checkout's intent, and the lease holder is the one
  agent entitled to redefine it).

Humans edit the file directly when they prefer — the grammar is
lenient markdown, and `goal list` doubles as the validator the
suite can call.

### 3. The turn-end verdict names the next step verbatim

The stop-block/open-work verdict (internal/report) gains one clause,
sourced from `goal next`: when the turn's own tracked work is
complete but active goals exist, the verdict is exactly the human's
requested shape — "the task may be done, but proceed with:
<next step verbatim>". When open work AND goals both exist, the
open-work refusal keeps priority (finish what is in flight first)
and the goal clause rides along as orientation. When neither exists,
today's "nothing left" stays truthful — and now it is TRUE, because
intent that used to hide in prose has a home the verdict reads.

### 4. Runtime-neutral by construction (items 16/17 doctrine)

The verdict is computed by ONE engine verb; how it reaches an agent
is each runtime's adapter-declared surface. Claude's existing stop
hook calls the same verb it calls today and appends the goal clause.
Codex and devin have no turn-end hook yet — this design does NOT
invent one per runtime; it defines the verb so that WHEN item 16's
audit gives each adapter a declared hook surface, the goal clause is
already there. Until then, codex/devin mains reach the same
information through `goal next` in their instructions (AGENTS.md's
turn-end section names it). No runtime name appears anywhere in the
goal machinery.

### 5. One mechanism or two? Two, one verdict surface.

Backlog item 1 (a turn may not end with unwatched in-flight work)
guards RUNNING work; goals guard INTENT between turns. They stay two
mechanisms with two grammars — conflating them would make a goal
block a turn the way in-flight work does, and the human's incident
shows the opposite need: goals must NOT block, they must direct.
They share the one verdict surface (the stop-block message), which
is where item 15's monitor facility also lands: a watched RUN's
continuation ("on green, do X") is exactly a goal with a next step,
so item 15 writes goals instead of inventing its own continuation
record. That is the composition: 15 answers WHEN, this answers WHAT
NEXT, item 1 answers WHETHER ENDING IS ALLOWED.

### 6. Delegates are out of scope

Delegates receive briefs and return against schemas; they do not
read checkout intent, and giving them the goal file would leak
mission-level context past the permission envelope. Hosts and main
sessions are the audience.

## What this deliberately does not do

- No priorities, deadlines, dependencies, or assignment — one
  ordered file of goals with one next step each. The failure mode
  was losing the thread, not scheduling.
- No mission integration — the mission prompt machinery already
  carries per-cycle intent; bridging the two is future work once the
  serialization question (D58) is settled on its own terms.
- No new document kind — the ledger pattern, reused.

## Proof obligations (for the later implementation decision)

- Grammar: parse/round-trip tests; active-requires-next-step;
  duplicate-id refusal; the suite validates the shipped template's
  example block.
- Verdict: table tests — goals-only, open-work-only, both, neither;
  the clause quotes the next step byte-verbatim.
- Authority: non-holder mutation refused; human edit + `goal list`
  reconciliation.
- Incident regression: a goals.md naming a program's next step
  yields a verdict that names it — the "NOTHING LEFT" incident
  becomes a failing test before the fix and a passing one after.
