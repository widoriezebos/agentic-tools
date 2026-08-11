# Dispositions — patience-orphan-usage, critique round 2

Critic: design-critic-20260811t125750z-b6ad (codex gpt-5.6-sol, xhigh).
Verdict: 5 material. All 5 ACCEPTED. Per the human's ruling after this
round, the amendment is FACT-GROUNDED before it is written: a
code-grounded fact pass (Codex) is answering the five mechanism
questions these findings expose — chain-closure boundaries, real
aggregation call sites and locks, live event-stream concurrency, the
usage artifact schema and its consumers, and the prompt records
grammar — and the design will be amended from verified anchors only.
The skill gains the standing rule (Ground the Facts Before the First
Round).

| # | Sev | Claim (short) | Status |
|---|-----|---------------|--------|
| 0 | critical | Chain closure is the wrong retention boundary (parks strand, long missions accumulate) | ACCEPT — boundary re-decided from Q1 facts. |
| 1 | high | The conclude-time aggregation call site does not exist | ACCEPT — call sites re-specified from Q2 facts. |
| 2 | high | Live event-stream reads race the writer under no lock | ACCEPT — read discipline re-specified from Q3 facts. |
| 3 | medium | Per-round provenance has no schema home | ACCEPT — placed from Q4 facts. |
| 4 | medium | The failure row has no grammar under strict validation | ACCEPT — expressed from Q5 facts. |
