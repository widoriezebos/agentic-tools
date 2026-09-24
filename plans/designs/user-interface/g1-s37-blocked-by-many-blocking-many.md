# g1-s37 Blocked by many, blocking many

- Kind: design
- Id: 01M39FGA0X84X669Q6WREHDEGS
- Status: draft
- Cites: 01M34HS374KF1RSS3EWKBGD2WE

Revision 1, 2026-09-24, Claude on Fable, on Wido's finding: "a goal could unblock several others, right? And it could also be the other way around. A goal could be blocked by several others ... this feels to me like an oversight." Ruled: "design for this and get it implemented. And indeed, you need a critique from Astra." Draft until Astra has read it. Touches the goal package, a tier-1 floor, on Wido's word.

## What is true today

The ledger already holds the relation both ways: a goal's record carries `BlockedBy` as a list, the board shows every open blocker, and a blocked goal returns to the queue only when every goal in its list is done (`returnBlockerParks`). What is missing is the verbs that write the edges. `goal open --blocks` takes one id, so a new goal can block one goal and no more; nothing lets a new goal open already blocked by others; and no verb adds or removes a blocker on a goal that exists, so today that is a hand edit of the ledger. `ParkRecord.Blocker` names the one goal whose open parked this one; the lift rule reads the whole list. A seat may open a goal only as the blocker of its own claimed goal (`SeatOpenNeedsBlocker`); a human opens freely.

## Step 1

**Engine, `internal/goal`.**

1. `goal open --blocks A,B` (repeatable or comma-separated): every named live goal gains the edge and, if not already parked, parks in the same publish with the new goal as its `Blocker`; an already-parked goal gains the edge only. The seat rule is unchanged: a seat's open must name its claimed goal among them.
2. `goal open --blocked-by A,B`: the new goal opens with those edges and parks at once; `Blocker` names the first, `Because` names them all; the lift rule is the existing one, every listed goal done.
3. `goal block --id X --by G`: adds the edge and parks X if it is live and unparked (any actor, as parking is today). `goal unblock --id X --by G`: removes the edge; when X's remaining blockers are all done or gone, X returns; removing an edge whose blocker is not done lifts a block early, which the ledger reserves to a human, so that case needs a human proof as `resume` does.
4. Refusals: a goal never blocks itself; an edge that would close a cycle is refused naming the cycle; an unknown goal is refused as today; a done or abandoned goal cannot be blocked.
5. History: the two new verbs record with the existing keys (actor, targets, reason); no new History key. The board's projection and the goal page read `BlockedBy` in both directions: the goals X waits for, and the goals that wait for X.

**Interface.**

6. The New goal sheet's "More" holds two pickers, "Blocks" and "Blocked by", each taking several goals as chips (the picker of g1-s36 in multi mode); the request carries two lists; hints say the consequence in each direction: "The chosen goals wait for this one" and "This goal waits for the chosen ones and parks until they are done".
7. The goal page shows "Waits for" and "Holds" with the goals as links and their states, and the acts to add or remove an edge through two routes, `POST /api/backlog/goals/{id}/block` and `/unblock`, under the same human-act rule as the board's other acts. A Waiting card's line reads "waiting for A and B to be done".

## Later, when it hurts

- A picture of the dependency graph; bulk edits; blockers across arcs shown on the board as lines.
- Seats adding blockers to goals they do not hold.
