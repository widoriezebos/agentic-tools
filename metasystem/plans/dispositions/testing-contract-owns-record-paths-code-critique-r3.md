# Dispositions: testing-contract-owns-record-paths code critique round 3, chain tcorp-cc3-20260910

The read of the carried engine chain (Opus, 2026-09-10 21:24 to 21:30Z) certified reviewed tree e9072e844d16a9e4e51e88f2da62f8c3ce80355d (job implementer-2aae86f16a7801089540f7dc, the certified fallback engine carried onto main at ce71e66d with a union merge of the command package's test file) with zero material findings: every test main had is present unchanged, the handover test is present and fails when the handover lines are removed, and the fallback engine's behaviour is the certified one. The chain closes on this read.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| TCC-01 | noted | When the protected base already declares a fallback and a candidate tries to move it to a new pathless surface, the command layer refuses (rightly: the base's fallback is protected) but with the generic invalid-surface message rather than one naming the protected fallback. Certified behaviour, unchanged by the carry; a wording improvement. | Recorded in the goal's conclusion as a small follow-up; no change in this chain. |

Gaps recorded by the critic: it did not run the conformance review stage (read-only policy; it recomputed the diff and found it byte-identical), nor the fast gate, full command tests and contract check (the orchestrator ran those green on the carry tree), and it repeated the design tension with the design page's rule on unowned paths, carried to the chain that declares the residual surface.
