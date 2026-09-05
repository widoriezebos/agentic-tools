# Supervision custody per checkout: code review, second round (chain scc-build2-cc2)

Reviewed tree 87af671b0059bd64c77b7f604d35dd2812f407c8 (chain scc-build2, round 2). Critic: Claude Fable 5.1. Two material findings; a third round follows.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SCC-01 | accepted | Both callers of the guard pass the state root twice, so the path comparisons are tautologies; the only veto is a prefix test on the owner tag against a slug of the git scope, which a sibling checkout with a hyphenated suffix or a nested worktree satisfies. | Compare the requested canonical path with the checkout path recorded in the owner's own registry row, exactly; the tag only vetoes; the dead pre-registry branch goes. |
| SCC-02 | accepted | The reduction now fails the whole registry closed when a tag or custody id appears under two checkout paths, a new corruption class under REG-5, which the design names a contract change; nothing names it. | Keep REG-5 as it is: a record that conflicts with an earlier owner's identity is dropped as sequence-illegal per the reduction's own rule, and the drop is logged; no new fail-closed class. |
| SCC-03 | noted | The dead pre-registry branch is the mechanism behind SCC-01; removed with it. | with SCC-01 |
| SCC-04 | noted | The self-check accepts a scenario's bed-child shell pid as the main identity; defensible, scenario-scoped, and the pid cannot select a victim. | none |
| SCC-05 | noted | Shutdown now depends on a clean registry reduction; consistent with fail-closed, recorded. Resolved in effect by the SCC-02 amendment. | none |
