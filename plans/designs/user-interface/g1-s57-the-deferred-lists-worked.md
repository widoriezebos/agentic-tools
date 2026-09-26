# g1-s57: the deferred lists, worked

- Kind: design
- Id: 01M3F0YA88RXTV1A1FSEHVVJ9Z
- Status: accepted
- Goals: browser-interface

Wido, 2026-09-26: "I want all the open ones designed and implemented
now." Author Fable. Every item below was named deferred by an Astra or
Sol read under R-124 in the Built section of the design it belongs to;
each was judged then to leave the slice working and safe, and each is
built now exactly as that read's smallest fix said. No critique loop of
its own: the reads are the critique; Sol reads the build.

## The items

1. **g1-s44** Decisions. The alert inbox reads pages of the journal until
   the seven-day window's start, not one page of two hundred. A
   renewal's `since` is the effective expiry instant the engine
   computes where the row exposes it, else the review date as today.
   Mentions link record ids as well as goal ids where a ruling names
   one.
2. **g1-s45** Fleet. This seat's jobs in the opened block grouped by goal;
   minute counts of two hours and more shown as hours and minutes.
3. **g1-s46** Decisions queue. Label chips drawn from the rows shown after
   Find and the other chips, as the design said. The walkthrough's
   canned unpark admits an early return of a human-directed blocker park
   as the engine does. The act layer's fixture takes its git through an
   injected seam rather than running real git in a temporary directory,
   as the project rules ask. The load flake in `internal/goal/branch`'s
   read-gate test: find why its lock release does not settle before the
   next refusal is asserted, and make the test wait on the release it
   already observes rather than on time.
4. **g1-s47** Edit. The walkthrough's canned edit refuses in the engine's
   order, claim and park before approval. The route's test file gets a
   surface path of its own in testing.json.
5. **g1-s48** Decisions inbox. The visit is recorded after every reader
   that can fail, so a failed read never advances the marker; the
   Partner's capture names the group resolved for the page.
6. **g1-s49** Application. Known problems stand above What concluded,
   since the open problems are what a human wants first and the weeks
   are long; the four open weeks stay.
7. **g1-s54** The private store. The size measurement ignores only a
   vanished file and reports any other metadata error.
8. **g1-s53** The sitting. A deposit card in the drawer gets the room the
   focused view gives it: the drawer's default height grows to the
   larger of its current default and what one card plus the composer
   need, once, on the first card, and the human's drag still rules.

Not here: the full gate from this seat (it needs a goal-bound proof
launch, which is the engine's act, not the interface's).

## Verification and box

Each item with the test its read named or the smallest one that shows
it; the guards stay green; screenshots only where a page changed (the
Application order, the Fleet block grouped, a card in the drawer).
Budgets as always. Box: one build lane (Claude on Opus), one code read
(Codex on Sol) with one fix round under R-124; two attempts, 150 to 240
job-minutes.
