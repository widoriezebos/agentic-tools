# Go surface consolidation — round 5 dispositions

Critic: design-critic-20260812t080947z-518b (codex, gpt-5.6-sol).
7 findings, 7 material. The round-4 severance did not cut deep
enough: the surviving "small standalone defect fix" was itself the
remaining generating cause, and this round severs it too. The program
now makes zero behavior changes.

| Finding | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| GSC-R1-027 | accepted | The no-kill supervise reaper stamps timeout plus groupDeathProvenAt on live processes it never stopped — its own unit test asserts it. A worse defect than the overwrite, and its fix needs a kill-authority design decision. | Recorded with GSC-R1-009 as a KNOWN DEFECT PAIR handed to the human; the fix leaves this program. |
| GSC-R1-028 | accepted | After the severances the design failed its own definition by leaving invariant orderings in shell while claiming to remove script-shaped architecture. | The definition section now states plainly: surviving orderings are accepted plumbing; the program claims deletion and coherence, nothing else. |
| GSC-R1-029 | accepted | Stale appendix rows still built reap-verdict after the severance withdrew it. | reap-facts is a plain rename; the map contains no new verbs. |
| GSC-R1-030 | resolved by severance | The CAS repair's ownership boundary (jobs-dir-only reaper vs repo-rooted CAS owner, plus an import cycle) is real architecture the fix would have to invent. | Defect fix severed; the boundary question goes to the human with the defect pair. |
| GSC-R1-031 | resolved by severance | Routing through the CAS owner silently adds endedAt to reaper transitions, changing downstream ordering. | Same severance. |
| GSC-R1-032 | accepted | census run and signature-check are called only by fixture sequencers, and the census rule ambiguously said tests keep nothing alive. | Rule clarified: fixture sequencers are the production validation suite and are live callers; Go unit tests are not. Both verbs live and map into proc. |
| GSC-R1-033 | accepted | Step 3 mixed a mechanical rename with a behavior fix in one commit, against the collaboration rules. | With the fix severed, step 3 is a pure rename; the rule is cited in the step. |
