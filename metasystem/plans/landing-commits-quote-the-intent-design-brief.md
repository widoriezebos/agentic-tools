# Design brief: landing-commits-quote-the-intent

Working Mode: Design
Proposed goal, opened 2026-10-09 by Wido via m1e. Source: Wido's conversation of 2026-10-08 with a fellow engineer about reviewing agent work, read against the metasystem by m1e on 2026-10-09. The transcript is not in the repository; this page carries everything the design needs.

## Intent, in one sentence

Every commit the machinery lands on main carries the goal's intent in the person's own words and names the design page it was built from, so git log reads what was meant before what was done; a generated summary never stands in for the intent line.

## Why

The engineer in the conversation wants generated commit messages banned on his team: a commit message written by the agent tells him what the diff does, which he can see, and not what the human was trying to accomplish, which is the only thing he can review against. The metasystem keeps the human's intent upstream of the commit, on the goal and on the design page, and its commit subjects are machine phrases such as `goal G units U1,U2` with `Goal-Unit` trailers (`internal/goal/branch/commit.go:236-262`). A person reading `git log` on main sees a goal id and has to open the ledger to learn what was meant. Recent landings read "goal done review-chain-stops-and-records" and "goal G: merge main (records only) before landing".

The fix is small and in the metasystem's own grain: the intent already exists in the person's words, so the landing commit quotes it.

## What is true today (read 2026-10-09; the design verifies every site)

1. Goal-branch commits are composed by `commitMessage` with a fixed subject per kind (unit, plan, read) and a trailer (`internal/goal/branch/commit.go:236`); the landing's merge and done commits have their own fixed phrases.
2. The goal record holds `Intent` (one sentence in the person's words), `Origin` (human or main), and the designs that govern it (`goal show`).
3. A goal's commits must be tied to a goal and a steward check for commits without one is approved work (`commit-goal-binding`); that goal owns the trailer, not the words.
4. Commit messages "state intent and observable effect" (`docs/collaboration.md:32`); the collaboration doc owns the authorship convention, and landed commits carry human attribution when a human granted the authority (Wido, 2026-09-17).

## What done looks like

- The landing commit (merge or squash, whatever the lane produces) has the goal's intent as its body's first paragraph, quoted verbatim and marked as the goal's intent, then the design page path and revision, then the units landed, then the trailers. The subject stays short and names the goal.
- Unit, plan and read commits on the goal branch keep their machine subjects and gain one body line naming the intent's first sentence, so a reviewer on the branch reads it too.
- A goal whose intent changed between claim and landing (goal `intent-is-revisited-on-evidence`) quotes the intent as it stood at landing and names that it was revised.
- No agent prose is added: the body is composed from recorded fields only.

## Measure

Every landing after the unit lands shows the intent in `git log --format=%B -1` on main. The goal is proven by one real landing and one test per commit kind.

## Constraints and freedoms

- Composed from the record; never drafted by a model.
- The trailers `commit-goal-binding` relies on stay byte-identical.
- One unit, under 250 production lines; the full `cmd/metasystem` package runs once at integration as every landing does.
- Free: the exact body layout and the marker word for the intent paragraph.

## Not in this goal

Rewriting history; commit messages of hand commits by a person; the steward's trailer check.

## Neighbours to read first

`commit-goal-binding` (the trailer and the steward check), `landing-takes-an-hour` (the lane's commits), `intent-is-revisited-on-evidence` (a revised intent is quoted as revised).
