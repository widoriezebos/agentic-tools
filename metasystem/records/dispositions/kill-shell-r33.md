# Dispositions: kill-shell plan, round 33

Job: design-critic-20260812t042426z-6ba9 (codex gpt-5.6-sol, xhigh).
2 findings, 2 material, both accepted — the last leaning text
aligned to always-rebuild.

| id | disposition |
| --- | --- |
| KS-R33-001 | accepted — "build or locate" dies: the adoption bootstrap rebuilds unconditionally like every template bootstrap; a stale gitignored binary can never be located into service. |
| KS-R33-002 | accepted — the losing validator's criterion is EXISTENCE: after the bounded wait, an executable binary is used (the winner published it this window; every future invocation rebuilds anyway) and no binary means the r24 re-entry. "Fresh publish" leaves the text with the oracles. |
