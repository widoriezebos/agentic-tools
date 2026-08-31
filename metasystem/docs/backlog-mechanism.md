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
   the claimed goal revision.
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

Health judges claimed goals only. A claimed goal without the tuple is
dead under `claimed-goal-appetite` and names this remedy:
`metasystem goal set-budget --root . --id <id> --elapsed-limit ...
--attempt-limit ... --reserved-job-minutes-limit ...
--active-job-limit ...`. Text beginning with `Appetite:` in a queued
goal's next step is ordinary human prose. No parser or enforcement path
reads it.

**Slicing is the way (Wido's ruling, 2026-08-22).** Slicing is the
delivery law: large pieces of
work are NEVER built in one go. They are split into iterative,
independently DEPLOYABLE pieces — each slice lands whole, works on
its own, and leaves the system better — and the backlog carries the
next slice plus a note naming the remainder, sliced when its turn
comes. The structured budget limits one claimed revision; slicing
governs how anything large gets delivered. The two laws compose.

**Reviews carry round budgets and threat models.** A review brief
declares both up front; a TRUE finding outside that threat model
closes as `out-of-scope` in the dispositions (citing the scope as
evidence) — accepted as fact, rejected as work. The closure validator
enforces both the citation and the evidence-carrying refutation rule.

## Drafts and promotion

Items are shaped in `plans/goals-drafts/` — free-form files, no
grammar, no budget required. "Draft" is the status name. The
backlog itself holds only ready items: promotion (`goal open`) is
the dispatch delegate's explicit intake act, performed after the checklist
below passes. Delete the draft file in the same change that
promotes it.

## The intake checklist

Before promoting any draft:

- [ ] The intent says what DONE looks like, in one line.
- [ ] It is one DEPLOYABLE piece (the delivery law): it lands whole,
      works on its own, and leaves the system better. Anything
      larger is sliced first — the item carries its next slice, a
      note names the remainder.
- [ ] The item is small enough to claim as one deployable slice; its
      structured budget will be supplied as one complete tuple at claim.
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
A machine's own frontier (goal next) skips goals pinned elsewhere —
they are not claimable there, and reporting them ready would hide
genuinely claimable work.

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
