# fence-ask-flake

- State: done
- Intent: The dispatch fence job-cap-min leg flaked its third time in thirty days (the batched ask intermittently unwritten) — promoted to a fix goal by the flake registry's own three-strike law
- Origin: main
- Next step: Appetite: 2h. Sightings: two carried 2026-08-20, third 2026-08-26 during the batch battery (evidence artifacts/agents/suite-failures/20260826T035542Z-dispatch-36491); solo rerun green same hour. The fence's cap-refusal ask lands asynchronously to the driver's exit (the registry's sibling row for batched-ask got a bounded wait 2026-08-23 — this leg likely needs the same shape: a bounded wait on the ask file with the same failure message). Diagnose from the preserved evidence first; if it is the known async-landing class, the fix is small and its fixture gets the deliberate-load proof.
- Concluded: Fixed at the root: three reapers (shell dispatch reap, Go standing reaper, mission runner drain reap) raced one CAS with only one carrying the fence-ask hook — the losing-side skip WAS the flake, load-correlated because battery load decided the race. All three now raise the batched ask through one shared mechanism, applied-only, same reason mapping, loud on failure per fence.go's own law; the shell's silent swallow is gone. Certified 2/0 with the critic finding the third reaper; proofs green at honest package durations; the registry row records cause and fix. The production stake: a mission job timing out under any reaper now always leaves its recovery ask instead of parking indistinguishable-from-waiting-on-a-human.
- OpenedAt: 2026-08-26T04:08:04Z
- Revision: 3

History:
- 2026-08-26T04:08:04Z V91DJHP9G84QEFTA0F0BEWNVBY-m2-bc1be9cb open actor=m2+mac-coordinator targets=fence-ask-flake
- 2026-08-26T10:02:42Z 0Q1JR53QPJMF37RTM1BDPR1W77-m2-bc1be9cb claim actor=m2+mac-coordinator targets=fence-ask-flake
- 2026-08-26T11:37:49Z SS22RPPBC1GQMVSWK2PDZ726AC-m2-bc1be9cb done actor=m2+mac-coordinator targets=fence-ask-flake
Integrity: sha256=cf631dabf10e04f7e26ab02a4e36870af52c2ca1497a7eb82f004db1ec103dbe
