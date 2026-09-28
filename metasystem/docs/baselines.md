# Baselines

Dated measurements that still ground a decision. Each entry names the decision
it grounds. Remove an entry when its decision is retired or a newer measurement
replaces it; stale numbers are not kept here. Distilled on 2026-09-28 from the
finished records removed that day (tag `records-archive-2026-09-28`).

## Token spend

Weighted units: Claude input + 1.25 x cache-write + 0.1 x cache-read + 5 x
output; Codex uncached input + 0.1 x cached input + 5 x output.

### Weighted tokens per landed line (2026-09-17)

Lines are additions on main and goal branches, ledgers excluded.

| Window (CEST)      | Total M | Per line | Per Go line |
|--------------------|--------:|---------:|------------:|
| 16 Sep 02:00-14:00 | 240     | 18.2K    | 43.7K       |
| 17 Sep 09:00-15:00 | 84      | 6.8K     | 7.4K        |

- The drop came from ending model waits (32% of spend before) and the 200K
  seat window. Codex was 59% of spend in the last window.
- Codex effort `high` instead of `xhigh` saved no tokens: spend is calls
  times context, and 99% of Codex input is cache hits, which the weekly limit
  still counts. Effort changes wall-clock time, not tokens.
- A 2,200-line Codex build compacted three times inside its 258K window. This
  grounds splitting units above about 1,500 lines.
- Target this measures against: under 2,000 weighted tokens per landed line
  (`docs/paper/18-outlook.md`).

### Cold cache rewrites (2026-09-19, 00:00-05:52Z, every Claude transcript on m1e's host)

- 1,496 calls, 33.7M weighted units. Agent-tool subagent cold-cache
  rewrites: 0.2M = 0.49 %, and all four were a subagent's first call.
- All 34 seat cold rewrites after a transcript's first call followed a
  compaction; none followed a wait of 300 s or more.
- Codex jobs in the same window: 280.9M input tokens, of which 274.9M were
  cached reads, and 1.4M output; Codex logs report no cache writes.
- Grounds: waits no longer cost cache, and compaction is the cause of cold
  rewrites (goal waits-run-outside-model-contexts). The counting method is in
  `plans/spend-fence-reports-tokens-per-model-and-cause-design.md`.

## Proof reuse

### Proof reuse before goal-landing-efficiency (2026-09-17)

- The 12 diagnostic deep proofs of `units-land-in-batches-under-one-proof`
  (54-57 groups each, 3.12 attempt-hours, no overlap) reused 13.6 % of group
  observations. The four largest fixture groups ran 12 times with no reuse.
- The judge key changed in 6 of 11 adjacent attempts, and each change dropped
  every execution identity. In two of those transitions, 34 of 55 input
  digests stayed equal.
- Grounds: keep the broad judge identity until a measured executor/evaluator
  split (`plans/application-testing-contract-design.md`, Decisions). A split
  must beat this reuse figure on a matched cohort.
- Source: the goal-landing-efficiency analysis of 2026-09-20, from retained
  `proof-runs/attempts/` records.
