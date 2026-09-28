# Dispositions: supervision lifecycle, round 3 plus the independent review

Round 3 (gpt-5.6-sol, 13 material, 8 critical) was adjudicated by the
independent review of 2026-08-09 (Claude Fable,
`records/supervision/supervision-lifecycle-critique.md`), which re-verified every
claim against the working tree and added three findings of its own
(SLC-F-001..003). All sixteen accepted; all folded into
`records/supervision/supervision-lifecycle.md` (revision of 2026-08-09) and
`records/supervision/supervision-registry.md`. The eight criticals reduce to five
distinct defects: derived identity (R3-001/002), undefined breaker
semantics (R3-004), untagged blast radius (R3-006), the unspecified
registry (R3-008..010), and no machine-wide bound or durable teardown
(R3-011/012).

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SLC-R3-001 | accepted (verified against code) | `fingerprint()` hashes supervision scripts and config; the repo contributes only its path string (process-census.py:908-943). A code edit would kill healthy supervision. | D-1 rebuilt on assigned identity: the owner's own `state.json` stanza. Fingerprint keeps only its re-arm job (D-7). Proof adds the code-edit-survival regression. |
| SLC-R3-002 | accepted, understated | A replacement repo at the same path hashes IDENTICAL — the fingerprint contributes nothing to adoption detection; the inode carried the whole burden. | Derived identity dropped entirely; a stranger repo cannot contain a `state.json` naming this owner's exact identity. |
| SLC-R3-003 | accepted (verified against code) | The takeover gate requires exact pid+start proven DEAD (arm-supervision.sh:619-628); a hung owner is alive under it, so the stated E2 scenario cannot arise legally. | E2 folded into D-1's ANOTHER'S branch, with its real production path named: a false death observation under load. |
| SLC-R3-004 | accepted (verified against code) | "Stale" includes state-read failure (arm-supervision.sh:426); five unreadable cycles would trip the breaker, contradicting the chmod-000 proof. | D-2: three-way observation. DEAD/STALE increment; UNKNOWN neither increments nor resets, and blocks relaunch. |
| SLC-R3-005 | accepted | Counting per relaunch vs per observation spreads time-to-give-up ~5x (≈25 min vs ≈5 min). | D-2 ONE TIMELINE: the counter counts base-interval observations; backoff gates relaunches only. |
| SLC-R3-006 | accepted (verified against code) | The incident population was untagged: census helpers (watch-background-jobs.sh:175-215) and `__lock-owner` helpers (dispatch.sh:2602-2609). | D-2: ceiling counts PROCESS-GROUP members of recorded components — the boundary `stop_identity` already kills by. |
| SLC-R3-007 | accepted | "more than 12" admits a thirteenth. | D-2/D-6: refused AT the ceiling; never exceeded. |
| SLC-R3-008 | accepted, understated | Schema lacked teardown data and a reason field; worse, `launch_set` mints new tags per generation (arm-supervision.sh:384-388), so a single armed record goes stale on the first self-heal. | REG-2: `relaunched` events per generation (write-ahead), `reason` on terminals; D-4: owner-tag-prefix sweep as backstop. |
| SLC-R3-009 | accepted | Append-only with no reduction rule leaves the janitor unable to tell current from terminal. | REG-3: reduce by ownerTag, latest event wins; joins append nothing; janitor-only compaction under lock. |
| SLC-R3-010 | accepted, narrowed | Framing was inheritable from the flight recorder; the genuinely missing piece was the append-failure rule. | REG-5: arming and provisioning FAIL on append failure; exit appends best-effort; janitor reaping idempotent. REG-1: torn-line tolerance. |
| SLC-R3-011 | accepted | Every bound was per-checkout; the incident was machine-wide, and the Proof promised a number no mechanism computed. | D-4 arming gate: refuse at K live claims machine-wide (K=8, D-6). Bound = K × ceiling, assertable. |
| SLC-R3-012 | accepted | D-5's report lived in the driver's own cleanup path — the path that does not run when the driver is killed. | D-3 write-ahead custody: `custody` records before arming; janitor reaps provably dead custodians. No renewal requirement, so the dead-man's-switch hazard does not return. |
| SLC-R3-013 | accepted (verified against code) | Components launch (arm-supervision.sh:389,395) before the state publish (399-413); an owner dying in the window leaves an unrecorded component. | D-2 write-ahead launch: `relaunched` appended before the first `launch_detached`; sweep covers the remainder. |
| SLC-F-001 | accepted (verified against code) | The shipped EXIT trap kills whatever the CURRENT state file names (arm-supervision.sh:358-366, 317-324) — a superseded owner's late TERM kills the successor's set. | D-1: trap replacement with held-identity teardown is a NAMED implementation item; Proof extends the superseded case to the TERM trap. |
| SLC-F-002 | accepted | The Proof referenced the intent lease D-3 had dropped. | Stale line removed from the Proof. |
| SLC-F-003 | accepted | Three writer classes, no concurrency rule stated. | REG-4: single O_APPEND framed line per write; janitor-only compaction under lock. |
