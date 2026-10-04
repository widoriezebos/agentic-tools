# Design brief: goal-worktree-dispatch-uses-the-primary

## Revision

Revision: revision 4 of `plans/designs/goal-worktree-dispatch-uses-the-primary.md`

Reason: round 3 of the design critique (Codex on Astra, design-critic-27f014429cd290f06ce7fc74) returned one material finding, on the **Messages** bullet of section 6; the seat accepted it. Change that bullet, the matching clause of unit 6's witness and mutation, and nothing else.

## Context pack

Read this pack first. Open another file only to check one of the cited lines
below; do not widen the read beyond that check.

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

Diagnosis or prior revision:

The complete prior revision is the page itself, at the path below; read it and edit it in place.

Critique findings being answered:

1. RULING-goal-worktree-dispatch-uses-the-primary (material, high). Claim: "The message selection still does not guarantee the goal's required 'last three messages between the seat and the coordinator.' It now takes the newest three across all qualifying seat conversations. Three newer messages from other seats therefore displace all coordinator messages." Evidence: `internal/board/mailbox.go:982` reads every mailbox, and its sender and recipient fields distinguish participants. Disposition: accepted. Seat's finding while deciding: no code names a coordinator seat on this host; `metasystem settings coordinator` is a per-checkout declaration and the coordinator seat's checkout has none, so a rule that looks the coordinator up would find nobody today. Amendment: group by counterpart instead of naming the coordinator. The prompt keeps the newest three messages of each seat that exchanged messages with this seat or about its goal (either direction, seat- or goal-addressed, replies included), counterparts ordered by their newest message, within the prompt's bound; a cut names the counterparts and messages left out and `metasystem agent inbox`. The witness has three coordinator messages followed by three newer messages from another seat, and all six are kept. Mutation: one limit of three across every counterpart.

Cited code excerpts:

none

Example page:

The page itself.

## Recurring findings

none recorded

## Tool-call budget

Maximum delegate tool calls: 20

Stop when this number is reached. List anything the budget did not allow you
to check.

## Page-size ceiling

Maximum page size: 2300 words

Cut a draft that exceeds this ceiling. If cutting would make the page
incomplete, stop and propose a split instead.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique
only through the context pack above. Never resume a delegate from an earlier
revision.

## Page artifact and return shape

Write the page to: /Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/goal-worktree-dispatch-uses-the-primary.md

Keep its head (title, Kind, Id, Status: draft, Goals) exactly as it is. Do not edit any other file, run test suites, build engines, claim or release goals, or commit.

Return only these two lines, N being the page's word count:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/goal-worktree-dispatch-uses-the-primary.md
DESIGN: ready (N words)
```

or:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/goal-worktree-dispatch-uses-the-primary.md
DESIGN: blocked (the reason in one line)
```

When blocked, stop and return `BLOCKED:` with the refusal's first line, what you tried and the decision needed; your seat asks.
