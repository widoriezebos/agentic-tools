# Sol's read of the g1-s60 build

Produced 2026-09-27 by Codex on `gpt-6-sol`, read-only, against `git diff ui-development...ui/g1-s60` at `107805b16` (ten commits, cut from `4acaa5e7c`) and the design's revision 5, under R-124: one fix round, then landing. Verbatim; the fix round's dispositions are in the design's Built section.

---

## Conformance and proof

| Design | Read result |
| --- | --- |
| D1 | `Service.Unsettled` reads the human's transcript and returns offered waiting, applying, refused and unresolved entries. It excludes applied, dismissed and never-offered entries. |
| D2 and payload | `decisions.proposals` composes the proposal need, including its coordinate, words, goal row, state and version. `handler.decisions` uses `partnerHuman`, matching the Partner routes. Schema 5 adds the proposal member and leaves counts' shape unchanged. |
| D3 | The group is first; the open row and bulk sheet show the action and budget. **Partial:** the row offers an unsafe retry after a known applied act (S60-C-01). |
| D4 | The inbox and card use `runProposals` through separate ports. **Partial:** the inbox has no continuation control after a stopped bulk run (S60-C-02). |
| D5 | The route publishes admitted writes and refuses stale versions. **Partial:** an older beat or snapshot can replace a newer card entry (S60-C-03). |
| D6 | Proposals remain on the conversation record; the diff adds no separate proposal store or Overview group. |

The named Go reader, composition, handler and route tests hold their stated local assertions. The frontend group, row and sheet tests establish rendered content; the inbox port tests establish routing and version arguments. The shared runner tests establish stop and retry rules, but their Continue control is exercised on the card, and their known-applied retry check does not render the inbox. The conversation tests fold beats in order; they do not test an older version arriving late. The four unchanged source guards cover their stated static rules. I inspected the bulk sheet, refused row and drawer screenshots; the builder's network log was reported, not independently reproduced.

I attempted focused Go and frontend tests. Both were blocked by this read-only sandbox when their tools tried to create temporary files (`go-build` and Vite's `.vite-temp`). The worktree remains clean.

## Findings

1. **S60-C-01 — High — A known applied act can be sent again.** Files/functions: src/partner/proposing.ts:1006, src/decisions/proposals.ts:170, src/decisions/InboxRow.tsx:367. Evidence, read: the runner retains an `unrecorded: applied` mark when the act succeeds but its outcome write fails. The open row says it applied, yet `ProposalPresses` always offers a button and `applyLabel` calls an `applying` entry "Try again"; unlike the card, it never consults `offersTryAgain`. Failure: an approval lands, the transcript write fails before persistence, and the entry remains `applying` at version 2. The inbox reread keeps it. Pressing the offered Try again admits `applying → applying` at version 2 and can publish a second approval. Smallest fix: use the runner's known-outcome retry rule in the inbox and suppress sending when the retained answer says the act applied. Changes what is built: yes. Works and stays safe without it: no.

2. **S60-C-02 — Medium — A stopped bulk run leaves its remaining actions without Continue.** Files/functions: src/decisions/DecisionsPane.tsx:483, src/decisions/ProposalBlock.tsx:107, src/decisions/proposals.ts:151. Evidence, read: `runProposals` marks later lines `notRun`, but the pane clears the selection as soon as sheet Apply is pressed. Neither the group nor an open row offers Continue, and collapsed lines omit the `notRun` mark. The named runner test proves that a supplied remainder can run; it supplies that remainder itself. Failure: select three waiting actions; the second returns unresolved. The third is unsent, appears as an ordinary waiting row, and the promised "Continue with the rest" is absent. Smallest fix: retain the unsent remainder and offer Continue from the group, showing its `not run` status on the row. Changes what is built: yes. Works and stays safe without it: no.

3. **S60-C-03 — Medium — A late update can move a card back to an older state.** File/functions: src/partner/conversation.ts:143, `received` and `proposalMoved` at line 321. Evidence, read: `proposalMoved` replaces an entry without comparing versions; `loaded` replaces messages wholesale. Outcome beats have no sequence. The tests send versions 2 then 3 only in order. Failure: a card has folded `applied` version 3; a snapshot captured earlier arrives afterward with `waiting` version 1, or an older beat arrives after a newer one. The card again presents the stale state and receives no further update. The route's version check prevents a duplicate act, but the card no longer shows what the shared record says until another read or press. Smallest fix: retain the highest version for each matching proposal when applying beats and snapshots. Changes what is built: yes. Works and stays safe without it: no.

## Departures adjudicated

1. Accepted: `partnerHuman` matches the Partner route's unnamed-seat fallback; `state(r).Human` is empty in that case.
2. Accepted: a never-offered action has no applicable act and remains explained on its card.
3. Accepted: passing the line to `reconcile` identifies which answer an inbox entry belongs to.
4. Accepted: the swapped commit order changes no delivered behavior.
5. Accepted: the extra commits make the open-row budget read stable and keep bulk Try again out of this slice.
6. Accepted as mapping and representation notes: `why` maps to `explanation`, the version is required, and the collapsed line describes persisted state. The `at` consequence is deferred below.

## Deferred and non-material

- Each admitted write restamps `at`, so an answered row can move to the end of its group and read "today." This changes ordering and age wording, but does not lose the row, its outcome or an act in the slice's first use.
- Transcript trimming can eventually remove an old unanswered proposal once retention cuts its turn. That longer-lived retention contract needs a separate design decision under the stated first-use materiality rule.
- The static budget tests do not themselves prove the asynchronous "one read on open" lifecycle; the source and inspected walkthrough support it.

VERDICT: 3 material findings (fail the works-and-safe test): S60-C-01, S60-C-02, S60-C-03
