# Dispositions: shhc-cc1-20260906, round 1

Chain under review: shhc-build1-20260906 (reviewed tree 0efc61ffab1735e9dd9939ed0a05e1f33ff6289e, round 1).
Critic: shhc-cc1-20260906, six material findings, two noted.
Orchestrator: m1d. One more finding, SHC-09, is the orchestrator's own,
from replaying the health fixture suite outside the sandbox.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SHC-01 | accepted | Read transcript.go and measure.go at the cited lines: a cursor write failure marks the transcript unreadable and drops its spend; a job-cache write failure fails the whole measurement. Both shrink or blank the number the spend fence reads, the one outcome the brief named as worst. | Round 2 per metasystem/plans/stop-hook-health-cost-fold2-brief.md: a cache write failure keeps the in-memory measurement; fault-injection tests. |
| SHC-02 | accepted | Read measure.go line 179 against fence.go 884-921: a terminal record whose usage is still pending (process group alive) is cached under a key that never changes, so its spend is never counted later. | Round 2: only reported or derived provenance enters the job cache; a pending record is re-read; test. |
| SHC-03 | accepted | The warm call measures the unchanged-files path, which a real Stop hook never takes: the seat's own transcript always grew. | Round 2: the test appends to a member transcript between cold and warm and uses records of realistic size. |
| SHC-04 | accepted | Every cursor write pays the full durability chain (six directory syncs on macOS) once per Stop, and once more per foreign transcript that grew, for a cache that is disposable by design. | Round 2: the volatile writer for cache files; no write on the foreign-grown branch. |
| SHC-05 | accepted | The cursor directory only grows; the job cache prunes and the cursor cache does not. | Round 2: prune cursor files not visited in the current measurement. |
| SHC-06 | accepted | The partial trailing line is the most intricate cursor path and no test ends a file mid-line. | Round 2: the byte-identity test adds a mid-line file grown across the boundary. |
| SHC-07 | noted | True: the files count on the line drops on hosts with many checkouts because foreign files are skipped instead of counted with zero requests; the format is unchanged and the new skipped count is in the summary. No artifact changes. | none |
| SHC-08 | noted | The grown-file rule trusts earlier bytes, as the brief specified; transcripts are append-only. Recorded. | none |
| SHC-09 | accepted (orchestrator) | Replaying health-fixtures.sh outside the sandbox on this tree, the narrator-recovery scenario hung: the fixture stops the steward runner with SIGSTOP inside its tick, and every later health call blocked without bound on the tick's component evidence lock (loadComponentEvidenceForHealth takes a shared lock; the stopped tick held the exclusive one from completeComponentAttempt). Identical on main, so pre-existing, but it is a cost the Stop hook pays whenever the tick is slow inside a durable write under load, which is this goal's symptom. | Round 2: the health read takes the shared lock without blocking, retries briefly (bounded, about 200 ms total), and on failure reports the role unknown with the reason "component evidence is busy"; a test with a held exclusive lock proves the bound. |
