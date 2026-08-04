# Known Issues

A standing register of defects and limitations that are recorded but not scheduled: capability ceilings, accepted trade-offs, and dead ends that must not be silently retried. Handoff notes die with their stream; this register survives so the next session does not rediscover the same wall at full price.

Each entry carries a stable id, the date, the observable symptom with evidence anchors, the realistic cost when it bites, and the fix direction or named escalation lever. Append dated updates to an entry; never rewrite a concluded one. A recurrence is appended to the existing entry and raises its priority.

| Id | Date | Symptom and evidence | Cost when it bites | Fix direction or lever | Status |
| --- | --- | --- | --- | --- | --- |
| KI-1 | 2026-08-04 | `last-census.json` records `durationMs` while section 3.11 names `durationSeconds`; naming only, behavior correct | A reader or later consumer looks for the wrong field | Align the design text or the field at the docs pass (item 19 vicinity) | OPEN |
| KI-2 | 2026-08-04 | Validation suite wall time grew 2m14s to 4m38s across items 3 to 23 on the same machine | CI cost and slower correction loops; a suite nobody runs locally stops catching environment defects | Profile the fixture set, parallelize independent groups, or split fast and slow suites; measure before optimizing | OPEN |
| KI-3 | 2026-08-04 | Adoption fixtures emit BSD `diff: Directory loop detected` diagnostics on symlinked skill registrations; exit status stays 0 | Noise that trains readers to ignore validator output | Compare registrations without following symlinks (`diff -r --no-dereference` or an explicit link check) | OPEN |
| KI-4 | 2026-08-04 | Census cost is proportional to total process count on the machine; 5.6s here after the fix, unmeasured on a loaded build host | A busy machine could push a scan past its interval and trip the new over-interval warning continuously | The warning is the tripwire; if it fires in practice, cache the signature-negative pid set between scans keyed by pid and start time | OPEN |

