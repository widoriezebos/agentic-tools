# Design brief: understanding-compounds-across-reviews

Working Mode: Design
Proposed goal, opened 2026-10-09 by Wido via m1e. Source: Wido's conversation of 2026-10-08 with a fellow engineer about reviewing agent work, read against the metasystem by m1e on 2026-10-09. The transcript is not in the repository; this page carries everything the design needs.

## Intent, in one sentence

A review builds on what the person already examined: the room opens with what this person saw and decided at design time, lists departures from the accepted design before anything else, and records what the person could explain and what remains unclear, so the next review on the same ground starts from that record instead of from nothing.

## Why

The engineer in the conversation named the hardest problem he sees with agent work: an imperfect understanding compounds. You review a small thing with a partial grasp, the next change is sanded onto it, and you feel you reviewed every step, but at the end you have digested nothing and could not explain the result. He also said a diff gives him no story to review against; a colleague pairing with him walks him through the change as a journey.

The metasystem answers the second half already. The review room walks a person through Asked, Built, Examined, Proven and Behaves, every claim anchored (`internal/ui/partner/review.go:39`, `:54-58`), and the whiteboard keeps remarks, drawings and the builder's evidence on the desk. It does not answer the first half. Each review is complete at its own phase: the design critique closes at design time, the code read closes at build time, the review sitting closes at landing. Nothing carries the person's understanding from one to the next, and the person who nods at landing may never have read the design. The paper prescribes the missing record: a sitting records "what the person could explain and what remains unclear" (`paper/15-the-sitting.md:39`, repository top level), and no room writes that line today.

## What is true today (read 2026-10-09; the design verifies every site)

1. The Asked walk explains "what was asked: the goal's intent, the design and its decisions that govern it, and the rulings that touch it" (`internal/ui/partner/review.go:54`). It does not say what this person decided at design time or which open questions the builder answered.
2. The Built walk explains what was built. The design pages' own Built sections already list departures from the design first ("Departures, each honest and recorded", `plans/designs/user-interface/g1-s71-the-whiteboard.md`, repository top level), but the walk does not.
3. A review asked again on a new version names the earlier findings and recommends the person's earlier decision where one still applies (`internal/ui/partner/review.go:122`). This is the only place the person's earlier judgement is carried, and only within one goal.
4. The review record holds the Outcome, the findings with their decisions and the facts deposited; it has no line for what the person understood or did not.
5. A person's acts at design time (accepting a design, answering a question, a ruling) are history lines on the goal and entries on the design page; they can be read, nobody reads them into the room.
6. The paper's learning chapter retains cases for what they teach and assesses understanding on unfamiliar cases later (`paper/14-how-engineers-learn.md:39`, `:55`).

## What done looks like

- The Asked walk opens with the person's own trail: "You accepted this design on 3 October with two open questions; the builder answered the first as X, the second is still open", derived from the goal's history and the design page, with each claim anchored. Nothing the person did not do is attributed to them.
- The Built walk puts departures first: what the accepted design promised and the build did differently, in the builder's recorded words, before the list of what was built as designed. A build with no recorded departures says so.
- The review record gains one section the person fills in words at the end of the sitting, or leaves empty on purpose: what they can now explain about this change, and what remains unclear. The room asks once; it never refuses a verdict for an empty answer. The Outcome carries it.
- The next review that touches the same ground (the same design, the same goal group, or the same files, the design decides the match) opens with that line: "Last time you noted the handoff between the lane and the gate was unclear to you; this change touches it", so the person's attention goes where their understanding is thinnest.
- Phone width and plain words, as the review room already demands.

## Measure

The share of review sittings whose record carries the understanding line, and the person's own judgement over a month of whether the opening trail saved them from re-reading. The goal is proven when both walks show the new openings on a real goal and one later review opens with a carried line.

## Constraints and freedoms

- The person's trail is read from records, never inferred; an act with no record is not shown.
- The understanding line is the person's words, not the Partner's summary of them.
- No new store: the review record and the goal's history carry everything.
- Small: at most five units of at most 250 production lines; one gate. Step one is the departures-first Built walk, which needs only the builder's recorded departures.
- Free: the wording of the openings, the matching rule for "the same ground", whether the understanding line is one field or two.

## Not in this goal

Learning sittings and assessment on retained cases (the paper's chapter 14; a later goal); a tour of the diff itself beyond what the walks do; changes to the critics' records.

## Neighbours to read first

`review-findings-read-as-decisions` (done: the room's four steps), `browser-interface` sections g1-s65, g1-s67, g1-s71 (the room, the board, the whiteboard; repository top level), `goal-completion-verifies-observed-intent` (what a person should see when a goal is done), `agent-works-as-project-partner` (parked).
