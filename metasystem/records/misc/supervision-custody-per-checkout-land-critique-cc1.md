# Supervision custody landing member: code review (chain scp-build1-cc1)

Reviewed tree 84ccc7fc5aa06441e710321b6bc55aed0bb81687 (chain scp-build1, round 1). Critic: Claude Fable 5.1. Three material findings; a correction round follows.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SCC-41 | accepted | The guard reads an owner's checkout from an open reduced claim, which only an arming-reservation row opens; production owners write only relaunched, launched and exited rows, which the reduction discards without a reservation. Every live owner reads as unregistered: the lockout in a new form. | Read the owner's checkout path from the rows production actually writes (launched and relaunched carry checkoutPath). |
| SCC-42 | accepted | The invariant test seeds arming and armed rows the previous binary never wrote, so it passes against a registry shape that does not exist and could not catch SCC-41. | The test seeds exactly the rows the previous binary wrote. |
| SCC-43 | accepted | ShutdownAt guards before the liveness check; a dead owner without a row or with a stale path makes shutdown refuse where it used to proceed. | Shutdown guards live owners only, as arming now does. |
| SCC-44 | noted | An audit row for the inferred driver hard-codes the in-bed flag. | with the round |
| SCC-45 | noted | The negative probe uses a no-op engine; acceptable and safer for the seat. | none |
