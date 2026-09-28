# Dispositions: benchmark-validity closure, round 5

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| BV-5-1 | accepted | Layered amendment blocks left incompatible requirements operative. | The Changes section rewritten as one consolidated specification; amendment blocks deleted. |
| BV-5-2 | accepted | Resume could not know if the preserved result passed the gate. | measuredOutcome persists gatePassed explicitly. |
| BV-5-3 | accepted | Mission-state fields changed without schema evolution. | State schema v2 with a versioned reader, same pattern as the census. |
| BV-5-4 | accepted | Same machine but different measuring code could span one run. | Per-attempt entries carry the measuring commit; validity requires agreement on both. |
| BV-5-5 | accepted | The ungradeable park had no durable cohort state or terminal transition. | ungradeable-pending-recovery, re-extraction on recovery, finalize converts pending to permanent. |
