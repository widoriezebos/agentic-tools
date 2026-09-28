# Dispositions: kill-shell plan, round 30

Job: design-critic-20260812t035650z-e1d0 (codex gpt-5.6-sol, xhigh).
3 findings, 3 material, all accepted.

| id | disposition |
| --- | --- |
| KS-R30-001 | accepted — the digest covers what the COMPILER sees: every .go file under cmd/ and internal/ in the working tree, tracked or not, plus go.mod and go.sum. Tracked-only was the wrong universe. |
| KS-R30-002 | accepted — truthful stamping by construction: compute the digest, build, recompute; publish only when the two digests are equal, else discard and retry bounded. A stamp can then never name a tree the build did not read, because inequality never publishes. |
| KS-R30-003 | accepted — the stamp interface is the embedded build stamp the binary already carries (the ldflags pattern of supervise.BuildStamp): readable via a version verb, atomic with the binary by embedding; no sidecar file to race. |
