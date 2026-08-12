# Dispositions: kill-shell plan, round 29

Job: design-critic-20260812t034509z-125b (codex gpt-5.6-sol, xhigh).
2 findings, 2 material, both accepted.

| id | disposition |
| --- | --- |
| KS-R29-001 | accepted — SOURCE DIGEST is defined: the SHA-256 over the sorted (path, content-hash) pairs of the tracked Go source set plus go.mod, computed from the WORKING TREE — never a commit id, which cannot see a dirty tree. The stamp carries the digest; freshness means recompute-equals-stamp. |
| KS-R29-002 | accepted — the moved-source claim softens to what is true: sources cannot take the lock, so a change racing the final rename window can publish a just-staled binary — but the stamp still names exactly what was built, the NEXT consult's freshness check detects it, and the rebuild path repairs it. Publication guarantees truthful stamping and eventual freshness, not an impossible atomicity over the filesystem. The honesty pattern of the patience crash contract, applied here. |
