# Dispositions: supervision lifecycle, round 13 — the cap, and the close

Round 13 (gpt-5.6-sol, job supervision-lifecycle-r14, 4 material,
verdict NOT-CONVERGED) was the LAST round under the human's close
rule. All four findings verified and folded; the chain is CLOSED at
the cap, honestly NOT as converged — these folds carry no critique
pass, and that is the close's named risk. The Proof list is the
arbiter from here; implementation-exposed design defects get one
defect-driven sol round each.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SLC-R13-001 | accepted | The round-12 shutdown set predicate covered recorded identities only; a watcher forked after `relaunched` but before its `launched` append exists as a TAG alone — and unlike the accepted sub-second helper residue, a watcher is long-lived. Escalated shutdown could succeed over it. | D-1: the set includes its write-ahead shadow — shutdown success also requires NO live process matching the claim's component invocation signatures with the claim's tags (the janitor's own REG-6 shapes); a signature-matched survivor fails the shutdown loudly. |
| SLC-R13-002 | accepted | The round-12 recovery rewrite dropped round 10's prerequisite that a reusable scorecard carry a teardown-ledger trace; full identity alone also matches pre-ledger deployments and foreign copies, grading unverified state. | D-3: reuse requires full repetition identity AND a `teardown-due` record for this repetition; full identity without any trace parks the repetition loudly (re-provision per the KI-30 precedent). |
| SLC-R13-003 | accepted | `armed` and the owner.json replacement had no defined order: an armer dying after the lock replacement but before `armed` leaves a live joinable owner that reduces as an unarmed reservation — later reaped as an establishment orphan under a recordless joiner. | REG-2/D-1: THE REGISTRY SPEAKS BEFORE THE LOCK — `armed` precedes the owner.json replacement; the crash between them converges kill-free (claim armed; next arm replaces the dead armer's lock; the owner self-supersedes). The establishment-orphan reap also inherits the SLC-R5-006 live-announced-session guard. |
| SLC-R13-004 | accepted | custodyId minting had no uniqueness law while ownerTag did; reduction closes custody absorbingly, so a reused id reads as already released and one release can hide another. | REG-2: custodyId = sanitized target path + custodian pid + start + 4 random hex, and a `custody` append whose id is SEEN AT ALL in the reduction is refused under the lock; the provisioner regenerates and retries — the armer's own law, mirrored. |
