# Sol's read of the g1-s61 build

Produced 2026-09-27 by Codex on `gpt-6-sol`, read-only, against `git diff ui-development...ui/g1-s61` at `4304e2b6c` (six commits, cut from `4f6ee3f71`) and the design's revision 1, under R-124: one fix round, then landing. Verbatim; the fix round's disposition is in the design's Built section.

---

One material finding. The selection, chip text, and three placements conform to the design, but opening a closed drawer does not reliably bring the proposed card into view.

### Conformance and test read

- **D1:** `proposedFor` in `partner/proposed.ts` and `useProposedFor` in `partner/store.tsx` select the four unsettled states by exact goal id, newest answer first, and exclude never-offered lines. `proposed.test.ts` holds those selection cases and the state change after a settling beat.
- **D2:** `chipWords`, `chipName`, and `ProposedChipView` produce the specified words, count, danger colour, and accessible name. `ProposedChip.test.tsx` holds the words and rendered markup.
- **D3:** `showsAt`, `showProposedFor`, and `ProposalCard` select the right card and scroll to it when the transcript is already mounted. The named navigation test checks source text; it does **not** exercise a mounted closed drawer. Finding S61-C-01 applies.
- **D4–D5:** The chip reads the existing human's Partner store and appears only in `Board.Card`, the approval queue's `InboxRow`, and `ProjectPane.GoalBlock`. Their three static markup tests hold placement and absence, but do not press the controls. The Go diff changes walkthrough fixtures only. The read guards cover call sites, timers, colour literals, tokens, and contrast pairs; the changed source adds none of the forbidden constructs.

**Verification:** Read the tests, source, diff, and supplied screenshots; ran `git diff --check`, which passed. I did not rerun the builder's tests in this read-only session.

### Finding

1. **S61-C-01 — High — Opening the drawer scrolls past the selected proposal.** File and function: src/partner/Proposal.tsx `ProposalCard` (:65), with the competing mount scroll in src/partner/Transcript.tsx `Transcript` (:118). Evidence: `ProposalCard` calls `bringUp` for the selected card, while the newly mounted transcript then sets `scrollTop` to `scrollHeight`. The supplied 400px screenshot shows Apply with the proposal lines above the viewport; the builder also reports that a proposal in an older answer opens at the conversation's end. Concrete failure: A chip for an older answer does not take the human to its card. Even for the newest answer, Apply can be visible before the lines describing its acts. Smallest fix: Make the transcript's opening scroll honor the selected card after mount, and add a mounted closed-drawer test for both an older and the newest answer. Changes what is built? Yes: drawer navigation and its regression test. Works and stays safe without it? No: the chip's promised destination fails in a normal conversation, and the first-use screenshot exposes Apply before the lines to review.

### Departures adjudicated

1. Accepted: The Go additions supply two canned goals to the walkthrough Project pane.
2. Accepted: `need.kind === "approval"` limits the chip to the designed queue rows.
3. Accepted: A pure view plus store wrapper supports the three surfaces and markup tests.
4. Accepted: The applying, mixed-count danger, and unresolved wording choices follow D2's state rules.
5. Finding S61-C-01: `revealed` and the folded-card ref help an already open transcript; they do not preserve the target when a closed drawer mounts.
6. Accepted: Exporting `GoalBlock` permits its real markup test.
7. Accepted: `onMouseDown` uses the board card's existing drag-handle pattern.
8. Accepted: The danger chip uses the asserted danger-on-surface contrast pair in both themes.

### Deferred and non-material

The Overview chip, tooltip, other humans' proposals, and proposals outside the store's last hundred messages remain outside this slice by design. The closed-drawer case is **not** deferred: it affects the chip's central press in the supplied first-use walkthrough.

VERDICT: 1 material findings (fail the works-and-safe test): S61-C-01
