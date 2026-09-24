# Astra's critique of g1-s37 revision 1

Produced 2026-09-24 by Codex on `gpt-6-astra`, read-only, against revision 1 of [g1-s37-blocked-by-many-blocking-many.md](g1-s37-blocked-by-many-blocking-many.md) at `4623b11dc`, under R-121. Verbatim; the dispositions are the design's revision 2.

---

The draft passes the “Step 1 Before Anything” gate: it names a usable slice and a deferred list. I read revision 1 at `4623b11dc` on `ui-development`. Findings below apply only where step 1 would be built differently or incorrectly. Failure scenarios are inferred from code; no tests or agents were run, no files changed, and the prohibited configuration file was not opened.

1. **S37-01 — High: the seat authority expands beyond the stated deferral.**  
   [Draft:18](g1-s37-blocked-by-many-blocking-many.md:18) requires only that a seat’s claimed goal appear *among* the targets; line 20 allows `block` by “any actor.” Today, [verbs.go:898](../../../metasystem/internal/goal/verbs.go:898) requires the affected goal to be that seat’s own claim. Ordinary parking also protects human-origin goals and foreign claims at [verbs.go:2369](../../../metasystem/internal/goal/verbs.go:2369).

   **Failure:** seat m1 holds A, m2 holds B. m1 opens G with `--blocks A,B`; the draft permits clearing m2’s claim despite expressly deferring seats blocking goals they do not hold.  
   **Required change:** define the actor/state matrix for both verbs and multi-target open. Preserve the restriction for **every affected target**, or make the new mutations human-only in step 1. Including one owned target must not authorize the others.

2. **S37-02 — High: removing an edge must not lift an independent human pause.**  
   [Draft:20](g1-s37-blocked-by-many-blocking-many.md:20) returns X whenever its remaining blockers are satisfied. Existing automatic return deliberately requires a nonempty `Parked.Blocker` at [verbs.go:953](../../../metasystem/internal/goal/verbs.go:953); lifting a human’s ordinary park requires authority at [verbs.go:2494](../../../metasystem/internal/goal/verbs.go:2494).

   **Failure:** Wido parks X pending a decision, then adds blocker G. G finishes; X correctly remains parked for the decision. A seat removes the completed G edge. Under the draft, no early-lift proof is needed and X returns, silently cancelling Wido’s pause.  
   **Required change:** distinguish removing a dependency from lifting a dependency-created park. An ordinary park must survive both completion and edge removal. Its separate `unpark` authority remains applicable.

3. **S37-03 — High: “first” works as a marker only if removal repairs it.**  
   [Draft:19](g1-s37-blocked-by-many-blocking-many.md:19) initializes `Parked.Blocker` to the first dependency, but specifies no maintenance after `unblock`. [file.go:811](../../../metasystem/internal/goal/file.go:811) requires that marker to remain in `BlockedBy`.

   **Failure:** X has blockers A and B, with marker A. A finishes and is removed while B remains live. Keeping marker A makes the transaction invalid; clearing it prevents `returnBlockerParks` from ever returning X when B finishes.  
   **Required change:** define the scalar as the marker of a dependency-created park, with all dependencies governing release. Rebind it when its edge disappears; clear the park only when its dependency condition is satisfied. Existing precedents already do this: [abandon.go:525](../../../metasystem/internal/goal/abandon.go:525) repairs rewritten blockers, and [split.go:287](../../../metasystem/internal/goal/split.go:287) selects the first replacement member. A second blocker list is unnecessary.

4. **S37-04 — High: “human proof as resume does” does not support the proposed browser action.**  
   [Draft:20](g1-s37-blocked-by-many-blocking-many.md:20) uses resume as its proof model, while line 27 exposes early unblock through the browser. Today [Resume at stop.go:439](../../../metasystem/internal/goal/stop.go:439) calls `AuthorizesResume`; [authority.go:237](../../../metasystem/internal/humanauthority/authority.go:237) excludes signed-in session proofs. Approval explicitly accepts those separately at [approval.go:395](../../../metasystem/internal/goal/approval.go:395).

   **Failure:** a signed-in Wido removes an unfinished blocker through `/unblock`; copying resume’s check rejects his valid browser authority. Checking only the human actor string would instead discard the proof boundary.  
   **Required change:** name session-aware admission for these operations and carry the verified principal through the existing act owner. Record its authority provenance using existing History fields, as [approval.go:437](../../../metasystem/internal/goal/approval.go:437) does. This follows the human standing required by [interface design:230](../../../plans/designs/user-interface-design.md:230).

5. **S37-05 — High: unconditional parking leaves the protected-claim outcome undefined.**  
   [Draft:18–20](g1-s37-blocked-by-many-blocking-many.md:18) promises to park live, unparked targets. Existing blocker parking clears the claim, records displacement, and preserves or drops its accounting episode at [verbs.go:922](../../../metasystem/internal/goal/verbs.go:922). Crucially, [clearClaimBinding at verbs.go:532](../../../metasystem/internal/goal/verbs.go:532) refuses a breach-stopped claim.

   **Failure:** X is claimed and breach-stopped; a human blocks X behind G. Following the draft’s unconditional outcome would clear the fence through parking, bypassing resume’s completed-stop-batch requirement. Reusing the existing helper instead refuses—an outcome the draft does not specify.  
   **Required change:** explicitly inherit the claim teardown and fence refusal. State that approval survives a dependency park, ownership does not automatically return, and existing running work follows the current parking behavior. `unblock` removes edges, `unpark` leaves edges intact, and `resume` handles a fenced claim; none substitutes for the others. The existing claim and conclusion checks already examine every blocker at [verbs.go:1134](../../../metasystem/internal/goal/verbs.go:1134) and [verbs.go:2234](../../../metasystem/internal/goal/verbs.go:2234).

6. **S37-06 — Medium: a blocker completed before submission can create a permanent unnecessary park.**  
   [Draft:19](g1-s37-blocked-by-many-blocking-many.md:19) parks immediately and relies on the existing completion-triggered return. That return runs when `done` executes at [verbs.go:2252](../../../metasystem/internal/goal/verbs.go:2252).

   **Failure:** the user selects live A, another seat completes A, then `open --blocked-by A` publishes. The new goal parks after the relevant completion event has already happened. It remains parked until another operation happens to lift it.  
   **Required change:** evaluate satisfaction in the mutation itself. Specify whether already-completed dependencies are retained without parking or refused. Also replace “done or gone” with an exact rule: remaining referenced goals must be done; deleting an edge differs from a missing goal, which is not satisfied.

7. **S37-07 — Medium: `--by G` collides with the human-identity flag.**  
   [Draft:20](g1-s37-blocked-by-many-blocking-many.md:20) uses `--by` for the blocker. The shared CLI already assigns it to the directing human at [goalsync_mutations.go:1006](../../../metasystem/cmd/metasystem/goalsync_mutations.go:1006).

   **Failure:** the human cannot express both blocker G and their identity for an early unblock; reusing the parser interprets G as the human.  
   **Required change:** give the dependency a distinct flag, such as `--blocker G`, preserving `--by` for authority attribution.

8. **S37-08 — Medium: “two lists” needs a compatible request contract.**  
   [Draft:26](g1-s37-blocked-by-many-blocking-many.md:26) changes the open request without defining compatibility. Today `blocks` is a string in [acts.go:101](../../../metasystem/internal/ui/httpd/acts.go:101), [act.go:287](../../../metasystem/internal/ui/act/act.go:287), and [opening.ts:104](../../../metasystem/internal/ui/web/_app/src/backlog/opening.ts:104). JSON decoding rejects unknown fields and type mismatches at [write.go:326](../../../metasystem/internal/ui/httpd/write.go:326).

   **Failure:** an existing browser tab sends `"blocks": ""` or `"blocks": "A"` to a server changed to accept only arrays; ordinary goal creation now returns 400.  
   **Required change:** specify the two fields, normalization of existing scalar requests, and omitted/empty behavior. Define the edge body for `/block` and `/unblock` too: the path ID should identify the dependent, so an action from X’s “Holds” list targets that dependent’s route. Both directions must reach one checked mutation, with authority supplied by `mayAct`, never the body.

9. **S37-09 — Medium: recovery must preserve both directions and the new authority conditions.**  
   The draft describes History but omits durable operation intent. Existing open journals its blocker at [verbs.go:816](../../../metasystem/internal/goal/verbs.go:816), and recovery reconstructs the mutation from that argument at [recover.go:404](../../../metasystem/internal/goal/recover.go:404).

   **Failure:** a seat opens N to block its claimed X, with N itself blocked by G, then dies before publication. A replay retaining only the old `blocks` argument creates N without its dependency on G.  
   **Required change:** include both lists in journal intent and reconstruct the same mutation. Specify replay or explicit fresh-invocation refusal for the new verbs. Early-unblock authority cannot be reconstructed from a stored human name; that existing boundary is explicit at [recover.go:145](../../../metasystem/internal/goal/recover.go:145).

**Outside step 1**

- **Cycles and unknown goals need no new graph mechanism.** Existing validation rejects missing references, abandoned blockers, and cycles at [validate.go:317](../../../metasystem/internal/goal/validate.go:317). The unknown-blocker display at [backlog/project.go:348](../../../metasystem/internal/backlog/project.go:348) reports incomplete evidence; it does not authorize dangling edges. Remote transactions fetch and rebuild after competing publication ([txn.go:659](../../../metasystem/internal/goal/txn.go:659), [txn.go:797](../../../metasystem/internal/goal/txn.go:797)). A goal another seat has published can therefore be resolved at mutation time; one not yet published must wait. Forward-reference support is deferred.
- **A new History verb is not a new key.** The parser reads the verb position directly at [file.go:1969](../../../metasystem/internal/goal/file.go:1969); unknown *keys* fail at [file.go:2097](../../../metasystem/internal/goal/file.go:2097). `block` and `unblock` alone do not justify a format migration or reader rollout.
- **Keep graph pictures, bulk editing, cross-arc lines, and seats changing unheld goals deferred**, as the draft already proposes. Their homes are the existing dependency projection and actor policy.
- **Do not add a general pause-cause framework or new cancellation system.** The current marker, claim teardown, and explicit fence refusal can support this slice.
- Proposed receipt, not written: `Read-only design critique of g1-s37 revision 1 at 4623b11dc; nine material step-1 findings; no tests or edits.`

**Verdict: build after the nine listed changes.**


