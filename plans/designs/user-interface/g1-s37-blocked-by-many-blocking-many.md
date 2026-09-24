# g1-s37 Blocked by many, blocking many

- Kind: design
- Id: 01M39FGA0X84X669Q6WREHDEGS
- Status: accepted
- Cites: 01M34HS374KF1RSS3EWKBGD2WE

Revision 2, 2026-09-24, Claude on Fable, after Astra's critique of revision 1 ([g1-s37-astra-critique.md](g1-s37-astra-critique.md): nine material findings, "build after the nine listed changes", all nine taken below). Revision 1 was on Wido's finding: "a goal could unblock several others, right? And it could also be the other way around. A goal could be blocked by several others ... this feels to me like an oversight." Ruled: "design for this and get it implemented. And indeed, you need a critique from Astra." Touches the goal package, a tier-1 floor, on Wido's word.

## What is true today

The ledger already holds the relation both ways: a goal's record carries `BlockedBy` as a list, the board shows every open blocker, and a blocked goal returns to the queue only when every goal in its list is done (`returnBlockerParks`). What is missing is the verbs that write the edges. `goal open --blocks` takes one id, so a new goal can block one goal and no more; nothing lets a new goal open already blocked by others; and no verb adds or removes a blocker on a goal that exists, so today that is a hand edit of the ledger. `ParkRecord.Blocker` names the one goal whose open parked this one; the lift rule reads the whole list. A seat may open a goal only as the blocker of its own claimed goal (`SeatOpenNeedsBlocker`); a human opens freely.

## Step 1

**Engine, `internal/goal`.** The relation is the existing `BlockedBy` list; `Parked.Blocker` is the *marker of a dependency-created park*, and release is governed by the whole list. Nothing new is stored.

1. **Who may write an edge** (the actor matrix). A **human** may block or unblock any live goal, open a goal that blocks several, and open a goal blocked by several. A **seat** keeps exactly today's power and no more: its open may name its own claimed goal as the one it blocks (`SeatOpenNeedsBlocker`), and `block` and `unblock` from a seat are admitted only for that same goal; naming one goal it holds never authorises the others. A seat never runs an early unblock, so the only edge it removes is a satisfied one on the goal it holds.
2. **`goal open --blocks A,B`** (repeatable or comma-separated) and **`goal open --blocked-by A,B`**. Every target that is live and not parked parks through the existing `parkBehindBlocker` path, which clears its claim, records the displacement, and *refuses* a breach-stopped claim (`clearClaimBinding`): that refusal is this verb's answer too, naming the fenced goal, and nothing publishes. A target already parked gains the edge only. A goal opened blocked-by parks at once with the marker on the first named blocker and `Because` naming them all. Satisfaction is evaluated in the mutation: a blocker already done is kept as a satisfied edge and causes no park; a blocker unknown to the ledger is refused as today; a done or abandoned target cannot be blocked; a self-edge and an edge that closes a cycle are refused by the existing validation, which already rejects missing, abandoned and cyclic blockers.
3. **`goal block --id X --blocker G`** adds the edge and parks X under the same path and refusals as above. **`goal unblock --id X --blocker G`** removes the edge and nothing else, then applies one rule: a park returns only when it is a dependency-created park (its marker is set) *and* every remaining goal in the list is done; a missing goal is never satisfied. When the removed edge was the marker and live edges remain, the marker is rebound to the first remaining blocker, as `abandon` and `split` already repair it. An ordinary park, one a human set with no marker, survives both a blocker's completion and an edge's removal; lifting it stays `unpark`, a fenced claim stays `resume`; none of the three substitutes for another. Approval survives a dependency park; ownership does not return by itself.
4. **Authority.** Removing an edge whose blocker is not done is an early lift and a human act; its admission is the approval gate's (`approvalProofClass`), which admits the enrolled terminal, the verified channel and the signed-in session, and its provenance is recorded with the existing History fields as approve does. `--by` stays the human's name everywhere; the blocker flag is `--blocker`.
5. **Journal and recovery.** An open's journal intent carries both lists and recovery rebuilds the same mutation; `block` and `unblock` journal their edge and are replayed the same way, except an early unblock, whose human authority cannot be rebuilt from a stored name: a recovered one is refused and the human redoes it.
6. **History**: `block` and `unblock` are verb names in the existing position, not keys; readers are unaffected.

**Interface.**

7. The open request keeps `blocks` compatible: a string (one id, or comma-separated) or an array, normalised on the server, absent or empty meaning none; it gains `blockedBy` as an array with the same rule. The sheet's "More" holds two pickers, "Blocks" and "Blocked by", each taking several goals as chips, with the hints "The chosen goals wait for this one" and "This goal waits for the chosen ones and parks until they are done"; a fenced target's refusal is shown under the picker with the goal named.
8. Two routes, `POST /api/backlog/goals/{dependent}/block {blocker}` and `/unblock {blocker}`, where the path id is always the goal that waits; a "Holds" row on G's page therefore acts on X's route. Authority comes from the act owner, never the body; an early unblock needs the signed-in human like every act. The goal page shows "Waits for" and "Holds" with states and links, the Waiting card's line reads "waiting for A and B to be done", and the board's unknown-blocker gap stays as it is.

## Later, when it hurts

- A picture of the dependency graph; bulk edits; blockers across arcs shown on the board as lines.
- Seats adding blockers to goals they do not hold.
