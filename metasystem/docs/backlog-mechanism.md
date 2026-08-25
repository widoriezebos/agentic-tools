# The backlog mechanism

How work enters, moves through, and leaves the backlog. Born from the
2026-08-22 ruling set after a script-sized task absorbed a full night
inside a review loop nobody was measuring.

## Appetite

Every backlog item carries an appetite: how much the work is WORTH,
agreed between Wido and the coordinator before the item is ready.
It is recorded as a plain `Appetite:` line at the start of the
goal's next step — a convention, not grammar; no engine change.

The appetite acts at every stage:

1. **Intake** — no item is promoted without one.
2. **Design input** — the design is scoped TO the appetite. If no
   honest design fits, that goes to Wido; the appetite never
   silently stretches.
3. **Post-design check** — does the designed shape still fit?
4. **Implementation cadence** — effort spent is compared against the
   appetite at regular intervals, not only when someone notices.
5. **Breach** — a blown appetite STOPS the item: paused on the
   backlog, raised with Wido. Other work continues if possible;
   otherwise block and wait.

The standing ceiling: nothing sized over eight hours starts without
a discussion with Wido first.

**Slicing is the way (Wido's ruling, 2026-08-22).** Slicing is not
an appetite mechanism — it is the delivery law: large pieces of
work are NEVER built in one go. They are split into iterative,
independently DEPLOYABLE pieces — each slice lands whole, works on
its own, and leaves the system better — and the backlog carries the
next slice plus a note naming the remainder, sliced when its turn
comes. The appetite is a different instrument: it sizes the WORTH
of a feature. A well-sliced piece still carries its own appetite;
a blown appetite still pauses and raises. The two laws compose,
they do not substitute.

**The appetite is machine-enforced, as covenant.** Its recorded form
opens the goal's next step with a duration token — `Appetite: 4h`,
`Appetite: 1d` (a day is eight working hours), `Appetite: 30m` —
prose welcome after the token. Every read of the backlog (goal next,
goal list) computes claim-age against the appetite and BANNERS a
breach on every machine; the steward's tick queues the same breach
to Wido's notification channel. A steward instruction — breach or
otherwise — is binding on the agent that receives it: covenant, not
advice.

**Reviews carry appetites and threat models.** A review brief
declares its round budget and its threat model up front; a TRUE
finding outside that threat model closes as `out-of-scope` in the
dispositions (citing the scope as evidence) — accepted as fact,
rejected as work. The closure validator enforces both the citation
and the evidence-carrying refutation rule.

## Drafts and promotion

Items are shaped in `plans/goals-drafts/` — free-form files, no
grammar, no appetite required. "Draft" is the status name. The
backlog itself holds only ready items: promotion (`goal open`) is
the coordinator's explicit intake act, performed after the checklist
below passes. Delete the draft file in the same change that
promotes it.

## The intake checklist

Before promoting any draft:

- [ ] The intent says what DONE looks like, in one line.
- [ ] It is one DEPLOYABLE piece (the delivery law): it lands whole,
      works on its own, and leaves the system better. Anything
      larger is sliced first — the item carries its next slice, a
      note names the remainder.
- [ ] The appetite is agreed with Wido and recorded.
- [ ] The item fits the appetite as understood today — anything that
      smells bigger goes back to shaping or to Wido.
- [ ] Origin is honest (`human` for Wido's asks — they carry his
      authority gates; `main` otherwise).
- [ ] It does not duplicate or belong inside an existing item.
- [ ] The next step states INTENT, CONSTRAINTS, and FREEDOMS — never
      a script of the how (IL-31, the mission-command discipline). A
      goal written as steps binds its executor to the author's
      context and goes stale the moment reality shifts; intent
      survives both. The test: a different machine claims it and
      executes without consulting the author. The coordinator
      rewrites script-shaped next-steps at intake.

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

## Coordinator ownership

Backlog order — priorities, appetites, item shape, intake — is the
coordinator's standing responsibility on every machine. Disorder is
fixed by discussing it with Wido: never reordered by fiat, never
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
