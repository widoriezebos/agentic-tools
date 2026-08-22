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

## Coordinator ownership

Backlog order — priorities, appetites, item shape, intake — is the
coordinator's standing responsibility on every machine. Disorder is
fixed by discussing it with Wido: never reordered by fiat, never
silently tolerated.
