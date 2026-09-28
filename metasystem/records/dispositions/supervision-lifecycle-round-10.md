# Dispositions: supervision lifecycle, round 10 — the folds' own defects, and shutdown's snapshot blindness

Round 10 (gpt-5.6-sol, job supervision-lifecycle-r11 — the r10 job id
was burned by a dispatch payload collision; this is CRITIQUE ROUND
10 — 5 material, verdict NOT-CONVERGED) verified the round-9 folds.
The count fell 6 → 5. Two findings are round-9 folds' own defects,
two are recovery/spec gaps the folds exposed, and one — shutdown's
single-snapshot blindness to a concurrent takeover — is the round's
one genuinely new interleaving, with incident-class consequences (a
"successful" shutdown leaving a live owner is how cleanup paths leak
supervisors). All five accepted and folded.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SLC-R10-001 | accepted | Round 9's SLC-R9-006 fold retained a bound custody's claim `arming`/`armed` but dropped its TERMINAL, so re-reduction of the compacted file reopened a PHANTOM claim — consuming a slot, eligible for re-reaping — out of a cleanly closed one. | REG-3: compaction retains the bound claim's FULL REDUCED SKELETON — `arming`, `armed`, terminal, and `swept` if landed. Proof sharpened: re-reduction shows no phantom open claim. |
| SLC-R10-002 | accepted | `--shutdown` reads one owner snapshot and signals once. Two uncovered interleavings: the amended lock can name a live ARMER (no rule at all); and a false-death successor can replace the snapshotted owner mid-shutdown, clean the predecessor's intent, and survive a "successful" shutdown. | D-1: SHUTDOWN IS CHECKOUT-WIDE — wait out an armer transition (establishment deadline); after stopping the snapshotted owner, RE-READ the lock and repeat with a fresh intent against any new live identity; at most 3 iterations (D-6) then loud failure; success only on "lock absent, or named identity proven dead and none appeared within one settle window". |
| SLC-R10-003 | accepted | TERM-first stop lets a graceful owner append `exited terminated` between the janitor's TERM and its `reaped`; terminals are absorbing, so the janitor's causal reason cannot land and REG-5's append-or-fail rule reads it as a janitor failure. KILL-first would fix the race by destroying graceful teardown. | D-4: THE OWNER'S OWN TERMINAL WINS — the post-kill re-reduction decides: still-open claims get `reaped`; self-closed claims get at most a verified `swept`, and a clean self-close is the sweep outcome achieved, reported as success. TERM-then-KILL stays. |
| SLC-R10-004 | accepted | The teardown ledger repairs teardown but not COMPLETION: after teardown-due + scorecard + crash, the phase stays `grading` and the committed driver REFUSES the existing scorecard — recovery as written still wedges the cohort. | D-3: RECOVERY CONTINUES COMPLETION — a driver entering `grading` with an existing scorecard VALIDATES it (schema + own reviewed commit) and REUSES it, advancing the phase; an invalid one is archived aside loudly and re-extracted; the refusal remains only for a scorecard with no ledger trace of completion beginning. |
| SLC-R10-005 | accepted | The reap-intent fence (SLC-R8-004) had no grace or marker-validity numbers while D-6 claims every number is fixed — different choices change how long a crashed janitor blocks legitimate joins. | D-6: reap-intent grace before the re-check = 10 seconds scaled; a marker is stale past 10 minutes (the registry's standing grace number). Join-fence Proof carries the numbers. |
