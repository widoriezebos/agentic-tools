# The stop-hook wedge fix: re-review after the correction (chain shw-build1-cc2)

Reviewed tree 193ef69195b93d53baf5363edae47cc4084189ff (chain shw-build1, round 3). Critic: Claude Fable 5.1. Brief: plans/stop-hook-wedge-code-critique2-brief.md. Zero material findings; the orchestrator's run of the process-owning fixture suite outside the sandbox is green on this tree. The chain closes and the fix lands.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SHW2-01 | noted | The record lock waits 100 milliseconds by polling, not the full second allowed; the test margin is 60 milliseconds. Backlog if it flakes. | none |
| SHW2-02 | noted | The deadline parent slugs the raw session id while the worker hashes unsafe ids; unsafe ids could use two record files. Session ids in practice are safe-shaped; for the design owner. | none |
| SHW2-03 | noted | The deadline path's record-failure fallback is a fixed message. Cosmetic. | none |
| SHW2-04 | noted | One git rev-parse runs synchronously after the worker launch; delays overrun handling only. | none |
