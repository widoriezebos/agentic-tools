# Sol's read of the g1-s58 step-1 build

Produced 2026-09-27 by Codex on `gpt-6-sol`, read-only, against `git diff ui-development...ui/g1-s58` at `b340940d4` (eleven commits, cut from `2ef9b3d64`) and the design's revision 12 at `9837d966d`, under R-124: one fix round, then landing. Verbatim; the fix round's dispositions are in the design's Built section.

---

## Conformance

I checked the implementation against revision 12 on `ui-development`, then traced the failure paths below.

| Design | Result | Code path |
|---|---|---|
| D1 | Partial: the nine route ids and frame are present; an empty label list and a foreign field are mishandled. | `uitools/propose.go`: `propose`, `framed`; `uitools/mcp.go`: `schema` |
| D2 | Partial: the host preserves prepared actions and the service checks subjects and scalar blockers. Admission does not check `open`'s `blockedBy` and `blocks` ids. | `partner/host.go`: `proposedIn`; `partner/proposals.go`: `admitProposal` |
| D3 | Substantially met: cards wait for the terminal beat and approvals receive a displayed tuple. The single-action foot still says "1 selected." | `partner/Proposal.tsx`: `Foot`; `partner/proposing.ts`: `busyAnswering`, `displayedFor` |
| D4 | Met: lines show the verb, subject, arguments and reviewed basis; dispatch uses the displayed approval tuple. | `partner/proposing.ts`: `argumentsOf`, `budgetWords`, `dispatchOf` |
| D5 | Partial: fetch-first guarding and outcome ordering are implemented, but later lines reuse the pre-run snapshot after an earlier line changes the same goal. | `partner/proposing.ts`: `guardFor`, `runProposals` |
| D6 | Partial: the outcome route uses a version check under the transcript writer, with the specified transition table. Failed outcome recording and stranded `applying` recovery have defects below. | `partner/proposals.go`: `RecordProposal`; `partner/proposing.ts`: `runProposals`; `partner/Proposal.tsx`: `Foot` |
| D7 | Fails on continuation: sign-in ends the run, but success resumes with a stale proposal version. | `partner/proposing.ts`: `runProposals`; `partner/store.tsx`: `runLines` |
| D8 | Met: cards, older-card folding, drawer growth and the cross-answer bar count are implemented. | `partner/proposing.ts`: `cardsIn`, `barLine`; `shell/Drawer.tsx` |
| D9 | Met: the skill directs the Partner to prepare actions, and the next-question context reports recorded outcomes. | `partner/project-partner.skill.md`; `partner/proposals.go`: `proposalsBlock` |

Section 6's catalogue, join, host, stream/snapshot, context, describe, and outcome-route tests exercise their named paths. The route tests cover every allowed transition and stale versions; their race test covers a representative transition rather than every pair. The fixture tests for S58-03, -04, -05, -07 and -08 cover the stated isolated rules. S58-06 misses the stale-version continuation because its record stub does not compare versions. S58-09 supplies a fabricated publish result to `settle`, so it does not prove the requested fixture publish path. S58-10 and -11 read source to guard offered refresh functions; they do not mount a form through a refresh. S58-12 tests mapping and continuation separately. These proof limits are distinct from the code defects below. I did not rerun tests in this read-only review.

## Material findings

1. **S58-C-01 — High — A later approval can authorize an edit the card did not show.** File/function: proposing.ts:906, `runProposals` and `guardFor`. Evidence: `look()` runs once; every line compares against `looked.rows`, even after an earlier act succeeds. Failure: A card contains `edit-goal G` changing intent A to B, then `approve-goal G` displaying A. Both are ticked. The edit lands; the approval still passes its comparison against the pre-run A and sends an approval for the now-current B. Smallest fix: Refresh the affected goal after a successful edit and refuse or re-present later approve/edit lines whose displayed basis has changed. Tests: changes what is built, yes; works and safe without it, no.

2. **S58-C-02 — High — A failed outcome write invites a second send of a landed act.** File/function: proposing.ts:936, `runProposals`, `lineState`, `offersTryAgain`. Evidence: After `send` answers `applied`, a failed outcome write sets `refusedUnsent`; that mark takes precedence over the known answer and enables Try again. The persisted line can remain `applying`, which the versioned transition table permits to become `applying` again. Failure: An approval lands, but the transcript's `applied` write fails. The page says it could not record the outcome and offers Try again; pressing it can send the approval again without the in-flight "check the goal" warning. Smallest fix: Retain the known act answer and withhold resending until a fresh reconciliation establishes that the act did not land. Tests: yes; no.

3. **S58-C-03 — Medium — Sign-in success skips the action that required sign-in.** File/function: proposing.ts:948, `runProposals`; store.tsx:1670, `runLines`. Evidence: The first run records `applying` at version 2 and the sign-in refusal at version 3, then hands `lines.slice(at)` to the callback with its original version 1. On success, the new run writes version 1, receives a conflict with settled `refused` version 3, and skips that line. Failure: The human signs in after a mid-run refusal; the action prompting sign-in is never applied, while later lines may proceed. Smallest fix: Resume from the latest returned proposal/version, rebuilding the remaining lines from current state. Tests: yes; no.

4. **S58-C-04 — Medium — Clearing all labels is lost in the tool frame.** File/function: propose.go:365, `propose`, `valueOf`, `framed`. Evidence: `labels: []` is accepted as the edit's change, but its joined value is empty and `framed` emits only nonempty values. The host therefore sees no labels field; `editOf` sends no label change. The existing Go test explicitly expects no `Labels:` line. Failure: The Partner prepares an edit that clears a goal's labels; Apply sends an empty edit body, which the act layer refuses as `no-change`, leaving the labels intact. Smallest fix: Emit `Labels:` when the list was supplied, including when it is empty, and test the tool-to-route round trip. Tests: yes; no.

5. **S58-C-05 — Medium — Foreign tool fields are silently discarded.** File/function: propose.go:280, `propose`; mcp.go:312, `schema`. Evidence: The schema permits additional properties, and `propose` checks named forbidden fields and known fields from other acts but never checks all supplied keys. Section 6's "foreign field refused in words" test is absent. Failure: An error-prone Partner supplies a valid `open-goal` with `blocked_by: ["G2"]` instead of `blockedBy`. The tool reports it prepared the action; the card omits the dependency, and Apply can create an unblocked goal. Smallest fix: Refuse every supplied key outside the common fields and the selected route's admitted fields. Tests: yes; no.

6. **S58-C-06 — Medium — A stranded `applying` line cannot be dismissed.** File/function: Proposal.tsx:277, `Foot`; store.tsx:1730, `dismissProposals`. Evidence: `dismissable` includes `applying`, and the route admits `applying → dismissed`; the foot renders Dismiss only when a line is `waiting`, and the handler writes dismissal only for waiting lines. Failure: A page closes after recording `applying`. On reload, the human checks the goal and sees the act landed; the card offers Try again but no way to settle the stranded line safely. Smallest fix: Offer Dismiss for eligible `applying` lines and persist their dismissal when no run owns them. Tests: yes; no.

## Departures adjudicated

1. Join test in `internal/ui/httpd`: accepted. It joins the two current owners, `Acts()` and the tool catalogue.
2. Edit act-table row in commit 1: accepted. Commit order does not change the delivered behavior.
3. Open frame has `Id:` without `Goal:`: accepted. The id is the route body's subject; the host reads it.
4. "Waits for" and "No longer waits for": accepted. Both state the relation in the goal page's vocabulary.
5. Proposal context absent from Seeing: accepted. It enters the next-question opening through `ComposeOpening`.
6. Three rules moved to `proposing.ts`: accepted. The provider uses those rules and the move permits direct tests.
7. Drawer growth counts proposals: accepted. The original deposit-only trigger would not grow for a proposal card.
8. Bar count visible at phone width: accepted. It preserves the waiting-action signal.
9. S58-10/11 source-reading guard: accepted as an implementation departure. It provides narrower proof than the specified mounted-form fixtures.

## Deferred and non-material

- `admitProposal` does not check ids in an open action's `blockedBy` and `blocks` lists; the act can refuse the invalid relation before writing.
- A later approve or edit of a goal opened in the same answer is admitted but cannot use a pre-run read of that new goal. The approve line explicitly says it needs its budget and excludes itself; this combination needs a separate design choice.
- The single-action "1 selected" text conflicts with D3's simpler single-action presentation but changes no act.
- The version-race, publish-error and mounted-refresh proof gaps noted above warrant stronger tests in a later pass.
- The builder's whole `cmd/metasystem` run timed out and had a base-reproduced assertion failure. This diff changes no file under `metasystem/cmd/`, so I found no path by which it changes that package's command code.

VERDICT: 6 material findings (fail the works-and-safe test): S58-C-01, S58-C-02, S58-C-03, S58-C-04, S58-C-05, S58-C-06
