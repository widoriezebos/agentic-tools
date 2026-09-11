# fixture-repo-copies-exclude-the-artifacts-store

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: a red bed fills the disk and the ENOSPC cascade turns an infrastructure failure into a suite failure and a read-only guest, no wrong decision is admitted; novelty 1: an exclusion at existing copy paths and a bound at the existing capture writer; exposure 2: every fixture bed that copies the checkout plus the capture writer; accumulation 1: one mechanism, no cross-layer state"
- Tier: 2
- Intent: A fixture that copies a checkout never carries the artifacts store, and a suite-failure capture never nests one. On 2026-09-07 the dispatch bed skew-repo copied the live checkout including artifacts/agents/suite-failures (20GB, 829k files) and the capture of that red bed (suite-failures/20260907T005711Z-dispatch-58078) nested the whole store again: one 22.5GB capture, self-compounding on every red. DONE means every fixture copy of a checkout excludes artifacts/ (and any other declared write-only store), a bed scenario proves a copied fixture repo has no artifacts/agents tree, and a capture is bounded: it never contains a nested artifacts/agents/suite-failures.
- Origin: human
- Next step: INTENT: fixture copies and failure captures stay bounded in size and never contain a prior capture. CONSTRAINTS: find every place a bed or the adoption comparison copies the checkout (skew-repo and its siblings, cp or rsync of the root under scripts/agents and the fixture beds) and the capture writer named in records/misc/flight-recorder.md; exclude artifacts/ at the copy and refuse nesting at the capture; prove with a bed scenario that is red on today main and green after. Evidence: the 2026-09-11 disk triage found 2.4GB free with that one capture at 22.5GB, of which 22.2GB was the nested copy (deleted that day, the capture own evidence kept). FREEDOMS: a shared exclusion helper or per-bed excludes; the capture writer prunes or refuses. ROSTER: Sol implements, Fable critiques (R-25). Tier 2.
- OpenedAt: 2026-09-11T10:28:06Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-11T10:28:06Z H49WWF4NWQ1EDM86RCJT7NPK0C-m1-c6925449 open actor=m1+main-1788680071-18713-e76d5d targets=fixture-repo-copies-exclude-the-artifacts-store
Integrity: sha256=1b21b5f4b8f7adcfaa08d1783f602ddcd4572118625cc10f2ed23bb2877d00c2
