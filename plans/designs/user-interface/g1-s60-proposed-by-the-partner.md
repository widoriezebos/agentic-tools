# g1-s60: proposed by the Partner, in the inbox

- Kind: design
- Id: 01M3FQWTG1JZJW29CT5Y70WJE9
- Status: draft
- Goals: browser-interface

Step 2 of g1-s58 (its section 4), on Wido's UX ruling of 2026-09-26:
the conversation is where a proposal is made and answered, and the
wrong place to keep one that waits; "make sure the design reflects this
insight". Author Fable. Every cite read at `35b67dc0a`; builds on
g1-s58 step 1 as built, and lands after it and after g1-s59.

## What exists and binds

1. **Waiting proposals are persisted, per human, on the Partner's
   messages.** g1-s58 D6: `Message.proposals[]` with `state: waiting |
   applying | applied | refused | unresolved | dismissed`, `verb`,
   `goal`, `title`, `fields`, `read`, `explanation`, `at`, and one
   outcome route that rewrites a message in place and answers the
   snapshot; the conversation is one per human per checkout, under the
   account's home (internal/ui/partner/conversation.go:59-68), opened by
   the service under the human's name (service.go:115-122). The runner
   that applies a line, records `applying` and the outcome, guards the
   press, passes refusals and stops at an unresolved answer is g1-s58
   D5, built as pure rules in a sibling module of the card.
2. **The inbox is groups over one composed payload.** `decisions.Compose(in,
   now)` turns `Inputs` (the project pane, the rows, the journal, the
   asks, the register, the human's standing, the visit window) into
   `Page{NeedsYou []Need, Decided, Counts, Visit}` (internal/ui/decisions/decisions.go:400-455);
   a `Need` is a kind, an id, a title, what is asked, by whom, since
   when, the silence line, a `Where`, an `Act`, an optional backlog row
   and `New` by the visit window (:156-200). The page groups the needs
   by kind in a fixed order, cheapest and freshest first, showing only
   groups with rows, one open at a time, each line with its count, the
   newest row's age and "n new" (src/decisions/groups.ts:41-105); an
   open row shows the substance of its kind and its acts (InboxRow.tsx:106-175),
   with "Sign in to act" in their place when unproven; the queue group
   has checkboxes and a selection bar (QueueBlock.tsx:143-178). The
   handler composes on every read, on mount, Refresh and after an act,
   from `h.state(r)`'s human (internal/ui/httpd/decisions.go:42-75), and
   the page re-reads whole after any act (g1-s48).
3. **The Partner's beats reach every open transcript.** The service
   publishes events to its watchers and the page's one stream carries
   them (service.go:29-68, 977-1000); the store folds a `proposal` beat
   into the message it belongs to (g1-s58 D2, section 5).

## How it works, from the chair

You come back the next morning. Decisions says "12 asked of you", and
the first group line reads "Proposed by the Partner · 5 · 2 hours ago ·
5 new". You open it. Five rows, each one line: "Pause · Fleet presence
is read from the census, not polled", "Unapprove · Refunds land within a
day", and beside each the age. You open the first: the line's words
whole, the reason the Partner gave, its explanation in its own voice,
"asked by the Partner, 2 hours ago; if you do nothing: it stays
proposed, nothing is applied", and the acts: Apply, Dismiss, Open the
goal, Ask the Partner. You tick four, the bar at the group's foot says
"4 selected · Apply · Dismiss · Clear", you press Apply, and the rows
leave as each act lands, exactly as they would from the card. The fifth
you Dismiss. In the drawer, the card from last night now reads "4
applied · 1 dismissed", because the row and the card are one record.

## Decisions

- D1. **One reader, the conversation's.** The Partner service gains
  `Waiting(human) ([]Waiting, error)`: every `proposals[]` entry in
  state `waiting` across the human's transcript, each with its turn, its
  index, the verb, the goal and its title, the fields, `read`, the
  explanation and `at`, newest first. It reads the transcript as the
  snapshot does and holds nothing.
- D2. **One more input, one more kind.** `decisions.Inputs` gains
  `Proposals []Waiting`; `Compose` turns each into a `Need` of kind
  `proposal`: id `<turn>/<index>`, title the goal's title, `asked` the
  line's words as g1-s58 D4 renders them (the verb word, the subject,
  the arguments), `by` "the Partner", `since` the proposal's `at`,
  silence "it stays proposed; nothing is applied", `where` the goal,
  `act` `apply`, the row where the backlog has it, and a `proposal`
  member carrying verb, fields, `read`, explanation, turn and index.
  `New` follows `since` as every kind does. The handler wires
  `Info.Proposals` from the service under the same human the standing
  names; a seat with no Partner configured supplies none.
- D3. **The group, first.** `groups.ts` gains `{ id: "proposed", kind:
  "proposal", title: "Proposed by the Partner" }` at the head of the
  order: it is the cheapest and freshest kind, one press each, and it is
  what the human asked the Partner for. The line, the count, the age and
  "n new" are the groups' own. An open row's substance is the line
  component g1-s58 built for the card, with the explanation under it in
  the Partner's voice; its acts are Apply, Dismiss, Open the goal and
  Ask the Partner, the last opening the drawer with the composer filled
  as the card's does. The group has checkboxes and the selection bar of
  the queue, with Apply and Dismiss over the ticked rows on screen.
- D4. **One runner, two callers.** The card's runner is a module the
  inbox calls too, with its dependencies handed in: the act clients, the
  outcome route, `askToSignIn`, and what re-reads after a confirmed act.
  From the inbox that is the page's own `reload`, in place as g1-s58 D5
  made every offered re-read; the run's rules, the fetch-first read, the
  freshness guard, the outcome mapping and the two writes per line are
  the same code. A row in flight shows the line's state as the card
  does; Dismiss records `dismissed` through the route.
- D5. **The card and the row read one state.** The outcome route, on
  recording a state, publishes a `proposal` beat with the updated entry
  to the service's watchers, so a transcript open in another tab, or
  the drawer beside the inbox, folds the change into its card without a
  re-read; the inbox re-reads its payload after the run as it does after
  every act. A dismissed line leaves the inbox and folds on the card.
- D6. **Nothing else moves.** No new persistence: the proposals live
  where g1-s58 put them. No server-side runner. No chip on the goal's
  row (step 3). Overview's own "needs you" count is untouched.

## Payload

`GET /api/decisions` schema 4: `needsYou[]` gains kind `proposal` with
`proposal: { turn, index, verb, fields, read, explanation }` on that
kind and `null` on the others; `counts` unchanged in shape. The Partner
service's `Waiting`; the outcome route's beat. Nothing else changes.

## Not here, later

The chip on the subject's row (step 3). Bulk Try again. Proposals in
Overview's group. A proposal older than its goal's current revision
marked as such beyond the freshness guard.

## Verification and box

Go: `Waiting` over a transcript with waiting, applied, refused and
dismissed lines across two answers, newest first; `Compose` with
proposals: the kind, the id, the words, `since` and `New` against the
window, the row joined where the backlog has the goal and nil where it
does not, the member carried, none when the input is empty; the handler
wiring under the standing's human and none without a Partner; the route
publishing the beat. Frontend: the group first with its line and count;
the open row's substance and four acts; Sign in to act when unproven;
Apply through the shared runner with the page's reload as the re-read,
a refused line passed and an unresolved line stopping; the selection
bar over rows on screen only; Dismiss leaving the row; the store folding
a `proposal` beat into the right message; the guards stay green with
the rows `cuts.test.ts` needs. Walkthrough: the fake Partner's canned
proposal left waiting, the inbox opened on a new visit showing the
group with "n new", one row applied and one dismissed, the card in the
drawer showing both; screenshots at 1280 and 400. Budgets as always.
Box: one build lane (Claude on Opus), one code read (Codex on Sol) with
one fix round under R-124, after Astra's read; two attempts, 120 to 180
job-minutes; lands after g1-s58 and g1-s59.

## Self-grade

High on D1 to D3: a reader over what exists, one input, one kind, one
group in a page built to take one. High on D4: the runner was written
as a module for this reason. Medium on D5: the beat is one line in the
route, but it is the first time a human's act, not the Partner's turn,
publishes on the Partner's stream; it is the same event type the store
already folds. Weakest: the id `<turn>/<index>` is a conversation
coordinate in an inbox of records; it is stable for the proposal's life
and is shown small, as every id is.
