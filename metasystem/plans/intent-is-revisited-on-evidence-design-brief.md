# Design brief: intent-is-revisited-on-evidence

Working Mode: Design
Proposed goal, opened 2026-10-09 by Wido via m1e. Source: Wido's conversation of 2026-10-08 with a fellow engineer about intent and agent work, read against the metasystem by m1e on 2026-10-09. The transcript is not in the repository; this page carries everything the design needs.

## Intent, in one sentence

When evidence contradicts a goal's intent, a person revisits the intent in a sitting, and the record keeps the old wording, the new wording, the evidence and the reason, and names the designs and goals the change invalidates, so a revised intent is a recorded act rather than a silent edit.

## Why

Wido told the engineer that the metasystem requires intent before anything is done. The engineer answered that intent is wrong a lot. Wido agreed: that is why intent is revisited, in the whiteboard and the sittings. The paper carries the scene: months after a release, production evidence shows the implementation followed the written rule and still exposed one person's work to the next, and a sitting convenes to revise the intent and reopen the design (`paper/15-the-sitting.md`, the second scene, repository top level). The machinery has no verb or sitting kind for it. Today an intent changes through `goal edit --intent TEXT`, in place, with no reason field (the `--reason` flag explains tier and risk changes only), no record of the old wording beyond the ledger's history, and nothing that says which accepted designs or sibling goals the new wording invalidates. Wido's rule of 2026-10-03 says re-approval is needed only on intent drift; nothing detects the drift.

## What is true today (read 2026-10-09; the design verifies every site)

1. `goal edit` changes intent, next step, risk or labels in place; raising risk takes evidence, lowering it is a person's act; a reason is asked only for tier or risk (`metasystem goal edit --help`).
2. The ledger's history keeps every act with actor and time, so the old intent is recoverable but not shown as a revision.
3. Design pages name their goals (`Goals:` in the head) and carry a one-word status: draft, accepted, superseded, done. Nothing marks a design as built against an intent that has since changed.
4. The shaping sitting deposits facts, decisions and open questions and ends in recorded intent or a framed design (`paper/15-the-sitting.md:39`); the Partner's sitting purposes are shaping and review (`internal/ui/partner/review.go`).
5. A human-opened goal's conclusion and parking are human-reserved; the intent is the person's.
6. `goal-completion-verifies-observed-intent` (queued) checks at closing that what was asked is observable; it does not revise the ask.

## What done looks like

- One verb, named by Wido (working name `goal revise`), that takes the goal, the new intent, the evidence (a record path, a finding, a landing, or a person's observation in words) and the reason, and is a person's act under the usual proof. It writes one history line that carries old and new wording side by side, and it prints what the change touches: every accepted or draft design naming the goal, every goal blocked by or blocking it, and every unit built against the old wording.
- Each touched design gets a line in its head or its first section saying the intent it was built against changed on this date, with the goal id, until a person marks it still valid or supersedes it. A design so marked is not a landing candidate until the mark is cleared.
- A revision of an approved goal withdraws its approval when the design says the intent drifted (the ledger's existing rule); the person re-approves in the same sitting or later.
- The review room and the shaping room offer "Revise the intent" from the goal page, opening the sitting with the evidence on the desk; the room is the UI, the verb is the act.
- `goal show` lists revisions as a short history: date, by whom, from what to what, why.

## Measure

Every intent change after the unit lands has a reason and evidence attached and names what it touches; a design built against a revised intent cannot land unmarked. Proven by one real revision and tests for the verb, the design mark and the gate.

## Constraints and freedoms

- A person's act; no agent revises an intent, and a grant does not change that.
- Idempotent: revising to the same wording twice records once.
- Human-readable output: line one says what changed and why, the rest lists what it touches.
- Small: at most three units of at most 250 production lines; one gate. Step one is the verb and its history line.
- Free: the verb's name (Wido decides), whether the design mark lives in the head or a section, and how the rooms surface it.

## Not in this goal

Detecting drift automatically from production evidence (the application's observations are outside the metasystem's scope; a person brings them); rewriting accepted designs; the completion check (`goal-completion-verifies-observed-intent`).

## Neighbours to read first

`goal-completion-verifies-observed-intent`, `goals-are-shaped-small-with-a-person` (done: the shaping sitting), `landing-commits-quote-the-intent` (quotes a revised intent as revised), `approval-stays-and-waiting-goals-return-by-themselves` (approval and drift).
