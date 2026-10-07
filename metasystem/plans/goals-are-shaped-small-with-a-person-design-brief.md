# Design brief: goals-are-shaped-small-with-a-person

Working Mode: Design
Goal goals-are-shaped-small-with-a-person. Tier 3. Opened 2026-10-07 by m1e for Wido.

## Wido's words (binding)

> "so the small design is an important one; it is about splitting large goals into smaller parts (the smaller designs). I don't think we have this properly covered yet and it requires a human in the loop during goal construction / splitting. This means we also have a UX aspect to cover here. This is a larger design / goal on it's own." (2026-10-07)

And the rule it serves (2026-10-07 11:10 CEST): designs are small from the start: each unit at most 250 production lines, each goal at most 5 units; a larger design splits into goals before its first critique round.

## Intent

Goal construction becomes a shaped, person-in-the-loop act. When a goal is opened, or when its design (draft or under critique) exceeds the size caps, the machinery proposes how to split it: the child goals, each with its intent, its share of the design's sections, its unit estimates, its areas, and the order between them (which builds first, which depends on which). A person sees the proposal, edits it (merge, move a section, rename, reorder, drop), and accepts it; acceptance opens the child goals, records the parent as split with its lineage, and hands each child its slice of the design as its brief. Nothing builds from an oversized goal. The same act serves the design stop rule's automatic split (a split with buildable remainder) so there is one split owner, whether a person or the stop rule asks.

## What must be designed (the design decides the shapes; these are the obligations)

1. The split proposal: what produces it (the design author, a split command, the convergence exit), its record (parent, children, section mapping, estimates, order, areas), and its freshness against the design it splits.
2. The person's acts and their UX: object-action verbs (for example `goal split propose|show|edit|accept|withdraw`, named by m1e's naming rule: intuitive without context), every UI capability backed by a verb (UI parity), summary output by default and --verbose for detail, every message line 1 the situation and line 2 the next act; a UI view in the review room or the app that shows the proposal as a tree with sizes, lets the person edit and accept, and shows the lineage after.
3. The automatic path: when no person is at the helm and a split is needed by the design stop rule, the machinery applies its own proposal (never asks a person, per the stop rule), records it, and a person can still edit or reverse it after (No HAL 9000: a person's later decision takes effect). Policy value per mechanism 1: goal.split = auto | person.
4. Lineage and accounting: the parent goal's state after a split (split, not done), the children's links to the parent and each other, budget boxes derived per child (plan finding 10), what `goal show` and the board show, how integration whole-goal rules apply per child.
5. The caps as committed declarations (design.unit-lines-max=250, design.goal-units-max=5) read through the settings owner, adoptable.
6. Interplay: goal-intake-detects-overlapping-work (overlap at intake), agent-works-as-project-partner (the partner session shapes goals with the person), review-drops-and-design-convergence (its convergence exit's split uses this owner; its admission cap refuses and names `goal split propose`). Say which goal owns what so nothing is built twice.

## The five questions

Answer for every new function, record and act: who calls it in production; how fresh is every state a decision reads; person or agent (a person's split decision always takes effect; an agent proposes and, under goal.split=auto, applies only what the stop rule requires); can every refusal's remedy succeed when followed; does every unreadable input fail safe for the agent and never refuse a person.

## Size (this goal follows its own rule)

Units at most 250 production lines, at most 5 units. If the design needs more, it splits itself into goals (for example: the split record and verbs; the UX view; the automatic path) before its first critique round, using the shapes this design defines.

## Order in the plan

After review-drops-and-design-convergence (whose admission cap and convergence split consume this goal's owner) and before switch-on.
