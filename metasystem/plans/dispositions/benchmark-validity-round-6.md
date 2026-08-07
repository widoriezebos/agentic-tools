# Dispositions: benchmark-validity closure, round 6

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| BV-6-1 | accepted | The extractor pins its own mission-state schema; producer-only ownership would make valid v2 state fail extraction. | Mission state follows the census pattern exactly: producer stamp is metasystem, kit schema v2 + version dispatch are kit; stated once for all versioned evidence. |
| BV-6-2 | accepted | measuredOutcome persisted only on failure left crash windows that could re-measure. | Measure, persist, close, publish — persist first; resume with measuredOutcome present is closure-only, never another turn. |
| BV-6-3 | accepted | "Completes the record" named no writer, file, or ordering. | Two files, one writer each; the cohort driver joins by copying from the runner's file; extractor cross-checks both; retry is idempotent rewrite. |
| BV-6-4 | accepted | candidateSha's established meaning could be overwritten; measuring commit source was unnamed. | candidateSha keeps its meaning; measuredCandidateSha is the gate-measured tree; adoptedMetasystemSha reads the adoption stamp, never HEAD; measuringKitSha is the kit's own commit. |
| BV-6-5 | accepted | The recovery contract was not an executable terminal machine and contradicted the no-close-on-park rule. | V-3a: five transitions, one writer, pending never blocks the cohort, finalize abandons (abandonment closes chains, parking never does), every cohort terminates. |
