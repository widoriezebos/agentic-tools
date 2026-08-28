# Dispositions — patience-orphan-usage, critique round 2

Critic: design-critic-20260811t125750z-b6ad (codex gpt-5.6-sol, xhigh).
Verdict: 5 material. All 5 ACCEPTED and resolved from the verified fact
sheet (`plans/patience-orphan-usage-facts.md`, cited F Qn.m).

| # | Sev | Claim (short) | Resolution (fact-grounded) |
|---|-----|---------------|----------------------------|
| 0 | critical | Chain closure is the wrong retention boundary | Boundary changed to SUCCESSION: latest unacted round per open chain, dropped when a successor round is dispatched or the chain closes (F Q1.2-4,11-13,21,24). Bounded, no accumulation, no park loss. |
| 1 | high | No conclude-time aggregation call site to reuse | Confirmed none exists (F Q2.9-11); the design adds one `AggregateUsage` call before ProjectFences at each park/conclude, under mission-fence.lock (F Q2.2,12). |
| 2 | high | Live event-stream reads race the writer | Derivation scoped to TERMINAL jobs only — where the aggregator already operates and the writer is dead, so the two-reads race (F Q3.10) cannot occur (F Q3.9, Q4.7). |
| 3 | medium | Per-round provenance has no schema home | Placed in an additive top-level `rounds` collection in mission usage.json (F Q4.19); never in state.fences.usage (F Q4.2-3,20). |
| 4 | medium | The failure row has no grammar under strict validation | The unreadable-chain row is `chain-root  unreadable  none` — three non-empty tab fields, `none` not the `(none)` sentinel (F Q5.20); the section declares three fields (F Q5.10-11). |
