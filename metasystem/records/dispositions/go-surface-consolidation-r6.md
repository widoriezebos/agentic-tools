# Go surface consolidation — round 6 dispositions

Critic: design-critic-20260812t082500z-0d10 (codex, gpt-5.6-sol).
5 findings, 4 material. The alias layer follows the pattern of every
guard this loop has examined: machinery protecting a boundary that
does not exist.

| Finding | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| GSC-R1-034 | accepted | In-repo callers move atomically with their rename; adopted repositories' stable interface is the shipped scripts, and no shipped contract makes verb names public. | Alias layer withdrawn. Verb names are declared internal; the upgrade documentation carries the appendix's old-to-new table. |
| GSC-R1-035 | resolved by withdrawal | The activation-ordering questions belonged to the alias mechanism. | census fingerprint becomes supervise fingerprint in step 4's atomic commit. |
| GSC-R1-036 | accepted | "Deletion slices as found" had no stop condition; two implementers could ship different surfaces. | Finite scope: the final surface is the current registered set minus the landed slice, renamed per the appendix; later deletions only when a step or the analyzer surfaces one, recorded in that step's commit. |
| GSC-R1-037 | accepted | schema materialize takes a role and version, never a job; its caller is an adapter. | schema stays its own single-verb family; job is 26 rows. |
| GSC-R1-038 | noted (non-material) | Stale ownership-inversion sentence. | Removed. |
