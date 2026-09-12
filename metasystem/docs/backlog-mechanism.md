# The backlog mechanism

How work enters, moves through, and leaves the backlog. Born from the
2026-08-22 ruling set after a script-sized task absorbed a full night
inside a review loop nobody was measuring.

## Structured budget

A queued or draft goal carries no machine budget. The complete budget
arrives when a human chooses to claim the goal: `goal claim` and
`goal open --claim` require all four values, and `goal set-budget`
repairs or revises an existing claim. The machinery never supplies a
default because limits are a human value judgment.

The four limits are:

1. **Elapsed time** — a positive working duration such as `4h` or
   `1d`; one working day is eight hours.
2. **Attempts** — the maximum number of admitted job reservations for
   the claimed goal revision; a terminal setup refusal (`phase=setup`,
   `refusalClass=setup`) releases its reservation and does not count.
3. **Reserved job minutes** — the maximum sum of reservation caps for
   that revision.
4. **Active jobs** — the maximum number of concurrent non-terminal
   reservations for that revision.

All four fields form one tuple. Partial tuples and numeric defaults do
not exist. Before a dispatch publishes a job reservation, admission
projects spending from authoritative job records. Equality at any
limit closes admission because a further reservation would exceed the
budget. Unknown evidence and a claimed goal without a budget also
close admission, naming the exact record that needs repair. No refused
dispatch creates a job record.

A verified channel answer is human approval proof only when the configured human user supplied the exact goal-bound token with a code valid at send time; its question or status thread, message reference, user, and code step are recorded with the goal operation, without the review date required by a relayed console word.
An exact verified answer to a `budget-above-norm` question re-approves the goal with the question's complete proposed box, including when an earlier verified answer already raised that goal's box.

**Acts under power of attorney.** A person may delegate tier-1 approve
and set-budget for up to seven days: `goal grant --by <name> --tiers 1
--verbs approve,set-budget --expires <YYYY-MM-DD>` records an entry in the
root record's `PowerOfAttorney:` section under the person's own proof, and
`goal revoke --by <name> --id <entry>` closes it early. A seat acts under it
with `--under <entry>` and nothing else: no `--by`, no proof, no
`--approved-ref`. The act is the seat's own on the ledger (`authority=attorney`,
`authorityOutcome=POWER_OF_ATTORNEY authorityRuling=<entry>` on the history
line) and is bounded by the entry (verb, tier, expiry, revocation), by the
goal's tier box (a set-budget over the box is refused with the norm
refusal), and by any approval a person made (proven, relayed or channel),
which neither verb rewrites. An entry a hand edit widens past the ruling's
bounds (tier, verbs, seven days) stays readable and is never honoured. An
approval given under an entry stands after the entry ends (R-95-m1e).

Health judges claimed goals only. A claimed goal without the tuple is
dead under `claimed-goal-appetite` and names this remedy:
`metasystem goal set-budget --root . --id <id> --elapsed-limit ...
--attempt-limit ... --reserved-job-minutes-limit ...
--active-job-limit ...`. Text beginning with `Appetite:` in a queued
goal's next step is ordinary human prose. No parser or enforcement path
reads it.

**Split before slicing.** A large intent may enter the backlog intact so its
authority and desired outcome are recorded honestly. It may not be claimed
with more than the configured goal norm unless the human records the strict
approval. Ordinarily `goal split` first atomizes that parent into an arc of
small, independently claimable members and concludes the parent with pointers
to them. Dependency edges own member order. Only then does the delivery law
slice each member into iterative, independently deployable changes. The
structured budget limits one claimed revision; the goal norm limits the normal
scope of one member.

**Reviews carry round budgets and threat models.** A review brief
declares both up front; a TRUE finding outside that threat model
closes as `out-of-scope` in the dispositions (citing the scope as
evidence) — accepted as fact, rejected as work. The closure validator
enforces both the citation and the evidence-carrying refutation rule.

## Drafts and promotion

Items are shaped in `plans/goals-drafts/` — free-form files, no
grammar, no budget required. "Draft" is the status name. The
backlog itself holds only ready items: promotion (`goal open`) is
a person's intake act, performed after the checklist below passes; the
person runs it, or a seat runs it with `--origin human` on the person's
recorded word. Delete the draft file in the same change that
promotes it. A seat's own open is confined to the blocker of its claimed
goal (R-93-m1e); see "Blockers a seat opens" below.

## The intake checklist

Before promoting any draft:

- [ ] The intent says what DONE looks like, in one line.
- [ ] It may be large at intake, but its intent and desired outcome are one
      coherent authority envelope. Before slicing or an ordinary over-norm
      claim, `goal split` must turn it into small arc members.
- [ ] Each member is independently deployable and claimable; explicit blocker
      edges record ordering, and its complete structured budget is supplied at
      claim.
- [ ] Origin is honest (`human` for Wido's asks — they carry his
      authority gates; `main` otherwise).
- [ ] It does not duplicate or belong inside an existing item.
- [ ] The next step states INTENT, CONSTRAINTS, and FREEDOMS — never
      a script of the how (IL-31, the mission-command discipline). A
      goal written as steps binds its executor to the author's
      context and goes stale the moment reality shifts; intent
      survives both. The test: a different machine claims it and
      executes without consulting the author. The dispatch delegate
      rewrites script-shaped next-steps at intake.
- [ ] The ROSTER is named before work starts: who implements, who
      critiques — and they are never the same session, and neither is
      the dispatch delegate, which briefs, nor the custodian, which runs
      gates and lands (Wido's ruling
      2026-08-25). The metasystem's delivery roles apply to work ON
      the metasystem exactly as they apply to every app; a rule that
      exists for the system's outputs binds the system's own work
      UNPROMPTED — the human reminding us is the failure, not the
      mechanism.

## Blockers a seat opens

Seats opened 165 goals in five days against 62 concluded (the delivery
audit of 6 to 11 September 2026). Under R-93-m1e a seat's `goal open`
(origin main) opens one thing only: the defect that blocks the goal it
holds. The open names that goal with `--blocks <goal-id>` and is refused
without it, with the ruling and the lawful forms in the refusal. The verb
does the whole move in one publish:

- the blocker opens queued, awaiting a person's approval like any goal;
- the blocked goal parks, its `Parked` record naming the blocker
  (`blocker=<id>`) beside the reason, and its `BlockedBy` gains the edge;
- the seat's claim on the blocked goal clears, which frees the seat's one
  claim for the blocker.

A goal that is already parked takes only the edge. Another seat's claim
still parks only under a person, and a goal that is not live takes no
blocker. The blocked goal returns to its resting state (approved when its
approval stands, queued otherwise) in the publish that concludes its last
blocker, with an `unpark` line naming the blocker, and is claimable again
at once; that publish lifts the park whoever recorded it, through the
done verb or a hand-concluded blocker at reconcile. An agent cannot lift
such a park earlier; a person can, and the edge then keeps the goal out of
the claimable frontier until the blocker is done. Only the verb writes the
blocker token: a hand edit that carries one is refused at reconcile.

A person's open (`--origin human`) is unchanged: it takes `--blocks` or
not, and may name any live goal, a parked one included, which then takes
only the edge. An improvement a seat discovers that blocks nothing goes to
`memory/backlog-notes.md` as a proposal, never to the ledger.

## The drop rule

A backlog item earns its place from current behavior or current
pain; losing that, it concludes with a record cheap to reopen from.
History is preserved in the conclusion note, never in a queue slot.

## Pinning a goal to a machine

A goal may be pinned to one machine's nickname (`goal set-pin --id X
--pin m2`; `--pin -` clears): only that machine may claim it, because
it alone has the setup, network, or resources the work needs. The pin
binds every claim path — an ordinary claim on any other machine
refuses by name, and even a human steal onto a foreign machine refuses
until the pin is moved. Pinning directs machines, so set-pin is a
human act (`--by`), and re-pinning a goal another machine currently
claims refuses: release it first — or clear the pin, steal, and
re-pin — so ownership never silently contradicts the pin. One reserved word: "-" is the clear form, so a
machine enrolled under that literal name can never be a pin target.
A machine's own frontier (`goal next --machine <nick> --fetch`) traverses the
global priority-then-sequence order and skips goals pinned elsewhere. A local
pin makes a goal eligible but never moves it ahead of an earlier unpinned
goal. A free seat claims only the returned goal; if another seat wins that
read-to-claim race, it fetches and selects again. The read is not a
reservation, and records are read to understand work rather than choose it.
A goal the claim gate would refuse is reported by `goal next` and the channel
status with the gate's own cause, is never handed to a seat, and is repaired
by the human act the cause names.

## Ordering the backlog

An open goal may carry a priority from 1 (highest) through 3 and a one-based
sequence within that priority. Both fields absent means unranked, and unranked
goals sort after every ranked goal. Rank is a human act at the enrolled
terminal: `metasystem goal set-priority --by <human> --id <goal> --priority
<n> [--sequence <n>]`. Inserting a position re-sequences the other goals in
that priority so its positions remain consecutive.

## Dispatch delegate sequencing

Backlog sequencing within recorded priorities, mechanical item shaping,
and checklist-governed intake are the dispatch delegate's responsibilities
during a claimed change. Disorder is raised to Wido: never reordered by fiat, never
silently tolerated.

## Concluding a goal

A goal is not done until its story is told. Concluding a goal appends
its paragraph to `docs/journey.md` — what it changed and why it
mattered, in plain English for a reader, not a grep — in the same
landing as the conclusion. The narrator goal owns the file's shape;
every concluder writes in it. Mechanically: append the chapter at
the end of the file — never anchor an edit on existing prose, which
wraps across lines and fails silently.

How a chapter is written (Wido's standard, 2026-08-23): the journey
is for a casual reader who has never seen this repository. Every
chapter must be understandable on its own after one read — a story
a person could retell. Concretely:

- No acronyms, identifiers, decision numbers, or commit hashes in
  the prose. Reference numbers live in commit messages and records,
  never in the story.
- Abstractions are welcome, including ones that borrow familiar
  words (the wall, the kit, the ledger) — but the FIRST use in a
  chapter explains what the thing means inside this system, in one
  plain clause, before the story leans on it.
- Say what actually happened — who did what, what broke, what
  changed — before any principle drawn from it. Concrete first,
  meaning second.
- Prefer everyday words over the system's internal vocabulary:
  "the success signal a command exits with", not "the rc"; "a small
  embedded Python program", not "a heredoc".
- The test is reading a chapter aloud to someone who was not there:
  every sentence must survive that.
