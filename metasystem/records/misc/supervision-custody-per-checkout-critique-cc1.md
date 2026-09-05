# Supervision custody per checkout: code review (chain scc-build2-cc1)

Reviewed tree 936888fce62ea68ba94c33b26150c9b7687d7f03 (chain scc-build2, round 1, carrying chain scc-build1's work). Critic: Claude Fable 5.1. Two material findings; a correction round follows.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SCC-01 | accepted | The guard in EnsureArmed compares the git scope with the registry's recorded state root; on any checkout whose scope differs from its state root (this repository: the outer git top versus its metasystem directory) a second `metasystem up` is refused as cross-repository before any liveness check. The invariant test collapsed scope, root and state root into one directory and could not see it. | Compare the canonical state root on both sides; the test covers a checkout whose scope and state root differ. |
| SCC-02 | accepted | The suite self-check watches an environment variable, an audit file only the arm script writes, and roots registered through make_repo; the incident scenario arms through `metasystem up` inside the hook and its root is never registered, so a scenario borrowing the seat's registry home would pass. | The self-check covers every bring-up a scenario performs, by whatever verb. |
