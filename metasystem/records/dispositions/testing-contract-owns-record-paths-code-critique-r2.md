# Dispositions: testing-contract-owns-record-paths code critique round 2, chain tcorp-cc2-20260910

The fresh read of the final round (Opus, 2026-09-10 12:31 to 12:37Z) certified reviewed tree 5b39b6168ddf34ac4d218cd2d868bc7765ea1839 with zero material findings: a fallback naming a surface with paths or dependencies is refused, the declared-phase pin asserts both, the command-layer handover has a test that fails when the handover lines are removed, and the contract file is unchanged. The chain closes on this read.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| TCF2-01 | noted | The skip of the fallback surface inside the selection loop can no longer fire on a validated contract, because validation now refuses a fallback that declares paths; its test builds a contract validation would refuse. Harmless defence in depth: if validation were ever loosened, the skip still keeps the fallback from matching owned paths. Kept as written. | None. |

Gaps recorded by the critic: it did not run the conformance review stage (read-only policy; it confirmed the six changed blobs equal the reviewed tree), nor the fast gate, the full command run and the contract check (the orchestrator ran those green on the round-3 tree), and it noted that the delivery proof is the orchestrator's (the landing receipt). TCF-04 and the design page's rule on unowned paths stay carried to the chain that declares the residual surface.
