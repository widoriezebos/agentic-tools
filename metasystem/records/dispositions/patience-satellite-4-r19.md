# Dispositions: patience-satellite-4, round 19

Job: design-critic-20260811t214348z-f214 (codex gpt-5.6-sol, xhigh).
4 findings, 4 material, all accepted — all four are verification
cases the round-18 section regeneration dropped from earlier
versions. No mechanism moved; the section gains the four cases back.

| id | disposition |
| --- | --- |
| P4-087 | accepted — verification gains the branch case: a root with two unwitnessed started terminal siblings counts two, proving set aggregation over single-lineage walks (r3/P4-029). |
| P4-088 | accepted — verification gains the duplicate-round case: two started jobs both numbered round two count as two (r2/P4-023). |
| P4-089 | accepted — verification gains the live case: a running job with an effective model never counts while running and counts after a terminal transition without certification (r6/P4-043). |
| P4-090 | accepted — verification regains the positive cancellation case: a job that reached running (model or usage proof) and is then cancelled counts (r10/P4-066). |
