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
