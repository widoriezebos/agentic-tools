# Dispositions: supervision lifecycle, round 4 — the first convergence round

Round 4 (gpt-5.6-sol, job supervision-lifecycle-r4, 14 material, 9
critical, verdict NOT-CONVERGED) reviewed the 2026-08-09 rewrite and
its new registry contract. All fourteen accepted; nine of them broke
the registry/custody surface introduced by the rewrite itself. The
folds land in `records/supervision/supervision-lifecycle.md` (same-day revision) and
`records/supervision/supervision-registry.md`. The structural moves this round
forced: currency comes from the LOCK, not from state content; ONE lock
covers every registry mutation including the arming gate's
reduce-count-reserve; reduction is an explicit fold with absorbing
terminals; custody is scoped to same-lifetime provisioners and cohorts
get BUILT teardown plus driver-entry recovery; the ceiling is enforced
continuously, not only at launch.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SLC-R4-001 | accepted | A superseded owner's in-flight `launch_set` can republish state after the successor, evicting the healthy successor under the false-death case E2 exists for. | D-1: `owner.json` is the sole CURRENCY authority; state content never decides supersession; publication re-checks the lock immediately before the atomic rename. Proof: false-death supersession. |
| SLC-R4-002 | accepted | The cap check and the reservation were separate steps under per-checkout locks; two armers race to K+1. | REG-4/D-4: the arming gate runs reduce-count-reserve under the single registry lock. Proof: cap atomicity. |
| SLC-R4-003 | accepted | Compaction read-reduce-replace under a janitor-only lock discards a concurrent lock-free append. | REG-4: every mutation holds the one registry lock — appends for one write, compaction across the rewrite. Proof: compaction safety. |
| SLC-R4-004 | accepted | Latest-event-wins cannot represent a multi-event claim; `relaunched` would erase liveness, a late `armed` would resurrect a closed claim. | REG-3: reduction is a fold; terminals absorb; the reduced claim merges armed identity with latest relaunch/launch records. Proof: reduction. |
| SLC-R4-005 | accepted | An owner that never completes first publication idles forever (check disarmed, UNKNOWN blocks everything), and an `arming`-only claim has no kill identity. | D-1: establishment bounded at 5 observations, exiting as `establishment-failed`. REG-3/REG-6: stale-`arming` rules; tag-verified single-observation kill. REG-5: a failed armer stops the owner it launched. |
| SLC-R4-006 | accepted | Dead `armed` claims on live checkouts (SIGKILL, lost `exited`) accumulate to K and refuse arming forever; TERM had no exit reason. | D-4/REG-3: the cap counts LIVE-VERIFIED claims; dead-owner claims are closed as `reaped`/`owner-dead`. D-5/REG-2: `--shutdown` appends `exited` reason `shutdown`. Proof: poisoned cap. |
| SLC-R4-007 | accepted | Path-keyed custody: a late release of custody A hides custody B, and a stale dead custody reaps a later healthy human arm of the same path. | REG-2/REG-3: `custodyId` on every custody event; releases name their id; mandatory `custody-bound {custodyId, ownerTag}`; the dead-custodian rule fires only against the bound claim. Proof: custody binding. |
| SLC-R4-008 | accepted | The cohort lifecycle is intentionally multi-invocation; every candidate custodian process is dead while the target healthily awaits approval — process custody false-reaps it. | D-3: custody scoped to SAME-LIFETIME provisioners (fixtures). Cohorts are not custodied at all; their teardown is the driver's (below). Proof: cohort wait survives. |
| SLC-R4-009 | accepted | `run-cohort.sh` contains no shutdown invocation; the design's "the driver still tears down" was a false premise, leaving completed-cohort owners alive — the incident class. | D-3: driver teardown is NEW behavior this design mandates — at repetition/cohort completion and as recovery at EVERY driver entry. Implementation order names it explicitly. |
| SLC-R4-010 | accepted | A launch-time gate cannot bound post-launch forking; the watcher forks census and duration helpers after launch, so 12 can become 13+ unchecked. | D-2: the ceiling is also checked at every breaker observation; overshoot is an incrementing observation that stops the set. Honest bound: never above ceiling at two consecutive observations. Proof: ceiling under forking. |
| SLC-R4-011 | accepted | Tag-only records cannot find an untagged helper after its tagged leader dies; the sweep's promise failed for exactly the incident's population. | REG-2: post-launch `launched` records with component pids; REG-6/D-4: pgid-membership kills (leader pgid = leader pid; non-empty groups are not recyclable), tag sweep as backstop. Proof: leader-dead helper. |
| SLC-R4-012 | accepted | Append-exited-then-teardown closes the claim before the teardown can fail, hiding survivors from a janitor that reads only live claims. | D-1/D-2: teardown precedes the terminal record, which carries `teardownComplete`; REG-3/D-4: incomplete-teardown terminals stay sweepable; `custody-released` only after verified teardown. Proof: breaker ordering, sweepable terminals. |
| SLC-R4-013 | accepted | "Report malformed records" chose neither fail-open nor fail-closed for readers whose mistakes kill healthy supervision or blind the cap. | REG-5: torn final line tolerated; any other corruption fails BOTH critical readers closed — no arming, no kills, loud report, human repair. Proof: corrupt registry. |
| SLC-R4-014 | accepted | `checkoutPath` is a string join key with no canonical form; a symlinked provisioner path splits one checkout into two states. | REG-1: every writer records the physical path of the git top level, resolved before writing. Proof: canonical paths. |
