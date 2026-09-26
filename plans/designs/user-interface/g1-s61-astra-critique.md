# Astra's critique of g1-s61 revision 1

Produced 2026-09-26 by Codex on `gpt-6-astra`, read-only, against revision 1 of [g1-s61-a-chip-on-the-goals-row.md](g1-s61-a-chip-on-the-goals-row.md) at `d3e497263`, with g1-s58 revision 12 and g1-s60 revision 5 as designed, under R-121 and R-124. Verbatim; the dispositions are in the design.

---

1. **S61-01 — Low — The board citation points to the list renderer.**

   **Evidence (read):** The design cites `GoalRow.tsx` for board cards. The board actually renders its own `Card`, with chips at src/backlog/Board.tsx:754.

   **Concrete failure:** Editing only the cited component would leave board cards without the chip.

   **Change:** Correct the board citation to `Board.tsx`'s `Card`.

   **Test 1:** Yes—the citation directs implementation to a different component. **Test 2:** Passes—the explicit board requirement and board walkthrough already require the correct surface; correcting this reference adds no behavior or mechanism.

2. **S61-02 — Low — "Whole conversation" overstates the loaded history.**

   **Evidence (read):** The design claims the whole conversation is held, but internal/ui/httpd/partner.go:58 limits snapshots to 100 messages.

   **Concrete failure:** After sufficient subsequent conversation, reloading can remove an older waiting proposal's chip because its message falls outside the snapshot.

   **Change:** Describe the loaded-history boundary accurately and defer coverage beyond it.

   **Test 1:** No change to this slice's specified hook, which explicitly reads loaded proposals. **Test 2:** Passes for first use; broader history coverage concerns later scale, and g1-s60 provides the persisted proposals' inbox.

**Deferred and non-material**

Both findings pass test 2. No additional mechanism is warranted.

The provider covers all three target surfaces and `/brain`; outside it, the default store is empty. Settled-state removal and card navigation are supported by the prerequisite designs. The queue's proposal button must sit beside its existing row-toggle button—ordinary implementation of D3's independent control. No additional dependency, timer, request, or colour literal is required.

Reviewed `d3e497263`, with g1-s58 revision 12 and g1-s60 revision 5 as designed. Conclusions are from source and contract reads; no runtime tests were run or files changed.

VERDICT: 0 material findings; build as written
