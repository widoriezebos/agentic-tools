# DRAFT: backlog-mechanism

Status: PROMOTED 2026-08-22 (goal backlog-mechanism, claimed). The
conventions live in docs/backlog-mechanism.md; what remains in this
draft is the retrospective agenda for the conversation with Wido.

## Why this item exists (Wido, 2026-08-22, verbatim rulings)

The backlog conversion — a one-off on a handful of files, sized as
"a simple script" — absorbed a full night inside an eight-round
adversarial review loop before Wido stopped it: "Do not go down the
fucking rabbit hole. Be practical. Don't waste my tokens." The
retrospective and the mechanism below are one item because the
mechanism is the answer to the retrospective.

## Part 1 — Retrospective (analysis, with Wido)

Why did a script-sized task get handled as far-from-trivial work?
Candidate causes to examine honestly: a backlog item accepted with
no size judgment attached; no role that owns "is this still worth
it"; the coordinator optimizing for review-covenant compliance over
value; no check-in with Wido after ten hours on one task. Output:
what detects "trivial task being handled non-trivially" EARLY, and
what acts on the detection.

## Part 2 — The appetite mechanism (design + implement as metasystem machinery)

- Every backlog item carries an APPETITE (Wido's name): a sizing
  agreed between Wido and the coordinator BEFORE the item is
  accepted as ready.
- The appetite is an INPUT to detailed design: the design is scoped
  TO the appetite where possible; where no design fits, that goes to
  Wido — never silently absorbed.
- Guarding is CONTINUOUS: checked right after design (does the
  designed shape still fit?), then at regular intervals during
  implementation as spent effort accumulates.
- A blown appetite is a hard stop: the item is PAUSED on the backlog
  and raised with Wido; the coordinator continues other work if
  possible, else blocks on his verdict.

## Part 3 — Drafts and promotion (this folder)

- Draft items live here (plans/goals-drafts/), free-form, while they
  are shaped and their appetite agreed. "Draft" is the status name.
- The backlog itself is never polluted: promotion (goal open) is the
  explicit intake act, performed by the coordinator only after the
  intake checks pass and Wido has agreed the appetite.
- The backlog's invariant: everything in it is ready to work.

## Part 4 — Coordinator ownership (metasystem design element)

Backlog order — priorities, appetites, item shape, intake — is the
coordinator's standing responsibility. Disorder is fixed by
discussing it with Wido: never reordered by fiat, never tolerated.

## Priority

HIGH — picked up immediately after the conversion; part of the
backlog mechanism itself. No other machine joins the backlog until
this item is complete (Wido's ruling).
