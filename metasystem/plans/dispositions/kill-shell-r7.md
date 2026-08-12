# Dispositions: kill-shell plan, round 7

Job: design-critic-20260812t000753z-a997 (codex gpt-5.6-sol, xhigh).
3 findings, 3 material, all accepted. Convergence: 10 → 7 → 7 → 3.

| id | disposition |
| --- | --- |
| KS-R7-001 | accepted — the projection is defined by making the registry the EXPORT MANIFEST: adoption ships exactly the registry's live scripts, adopted repos validate exactly the registered files, and anything an adopted project adds is unregistered by construction. "Payload globs" leaves the text; there is one list and the registry is it. |
| KS-R7-002 | accepted — fold debt: the round-6 rewrite dropped the enforcement sentence. Restored verbatim in force: the named plan must exist, the package must exist, and an unreachable package without an entry fails. |
| KS-R7-003 | accepted — the bootstrap is NON-PUBLISHING: it compiles to a temporary path, consults the gate fence through that binary, and only a fence-clear run may replace bin/metasystem — the foreign-gate safety rule holds before any Go verb exists to enforce it. |
