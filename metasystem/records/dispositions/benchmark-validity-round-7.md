# Dispositions: benchmark-validity closure, round 7 — the exhaustion round

All seven findings accepted and CARRIED OPEN to records/misc/mission-completion-protocol.md per the exhaustion contract; none are resolved by this design. The successor document enumerates every finding id verbatim.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| BV-7-1 | accepted | No branch for gatePassed=false. | Carried to the protocol stream. |
| BV-7-2 | accepted | First-transition crash can repeat measurement. | Carried. |
| BV-7-3 | accepted | Runner append and driver copy are unordered. | Carried. |
| BV-7-4 | accepted | Identity incompatible with the established tuple. | Carried. |
| BV-7-5 | accepted | recover and finalize race on one repetition. | Carried. |
| BV-7-6 | accepted | Cohort arrows are not crash-recoverable transitions. | Carried. |
| BV-7-7 | accepted | Finalize depends on an abandonment path that does not exist. | Carried; abandonment becomes a designed, owned feature of the protocol stream. |
