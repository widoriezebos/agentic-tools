# Sol's read of the g1-s63 build

Produced 2026-09-27 by Codex on `gpt-6-sol`, read-only, against `git diff ui-development...ui/g1-s63` at `9c4f3d361` (seven commits, cut from `657621cd4`) and the design's six items, under R-124: one fix round, then landing. Verbatim; there was nothing to fix, so the slice landed as built (the design's Built section).

---

## Conformance and findings

**No material findings.** Read-only review of `git diff ui-development...ui/g1-s63`, the changed source and named tests, the design and earlier reads, the builder's report, and the screenshot found no source change outside the six items and generated bundle.

| Item | Code and proof read |
| --- | --- |
| 1 | `clearShowing` in `metasystem/internal/ui/web/_app/src/partner/store.tsx` clears the drawer target; `setDrawer` in `metasystem/internal/ui/web/_app/src/shell/Shell.tsx` calls it on close. The named test checks that wiring by reading source; the existing `opensAt` test checks the resulting empty target. |
| 2 | `admitProposal` and `edgesNamedBy` in `metasystem/internal/ui/partner/proposals.go` check each edge ID against the accepted observation or an earlier admitted open. The named test covers existing goals in different states, an earlier open, and missing IDs in both lists. |
| 3 | `RecordProposal` preserves `at` and stamps `updatedAt` on each admitted write. `Unsettled`, `decisions.proposals`, the TypeScript types, and `lineOf` carry both; row age, ordering, and "new" use the original `at`. The named Go and frontend tests cover those paths. The screenshot shows the refused row retaining its position. |
| 4 | The named route test derives all eleven allowed transition pairs, walks each starting state through the route, then races eight writers at one version and checks one admission, seven conflicts, and one version increment. |
| 5 | The three named settle tests (`metasystem/internal/ui/act/settle_test.go`) drive `Approve` through the injected repository seam. They assert `pushed-unknown`, `journal-unreadable`, and `refused`, including the real journal reader and resulting ledger state; they invoke no real Git. |
| 6 | `portsFor` in `metasystem/internal/ui/web/_app/src/decisions/proposals.ts` folds successful writes into the inbox's held entries. The named test shows an `unresolved` version 3 winning over an in-flight payload at version 2. The inbox and Partner store use version ordering when later payloads or beats arrive. |

I did not rerun tests in this read-only review; the passing commands are the builder's reported results.

## Departures adjudicated

- **Accepted:** The card omits a display of both timestamps; the design says it *may* show both, while the inbox age and order are corrected.
- **Accepted:** Decisions schema 6 identifies the added `updatedAt` payload field.
- **Accepted:** The refused compare-and-set test fails the subsequent rebuild capture, allowing the transaction to finish with a real terminal journal entry and `refused` answer.
- **Accepted:** The all-pairs race test replaces the two-pair test it covers.
- **Accepted:** The test bed's repository field uses the existing interface so the injected wrapper can run the publication path.

## Deferred and non-material

- A proposal written before this slice can lack `updatedAt`; the Go reader and optional TypeScript fields tolerate it. If its old `at` was already restamped, the original admission instant cannot be recovered from that field.
- The screenshot establishes the refused row's position, but its "today" age does not distinguish admission time from write time. The dated composition and frontend tests make that distinction.

VERDICT: 0 material findings; land as built
