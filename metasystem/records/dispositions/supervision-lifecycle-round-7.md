# Dispositions: supervision lifecycle, round 7 — the recovery machinery's own crashes

Round 7 (gpt-5.6-sol, job supervision-lifecycle-r7, 9 material + 1
non-material, verdict NOT-CONVERGED) verified the round-6 folds. The
count fell 10 → 9 and the kind narrowed again: within-claim kill
order, one more guarded append, crash recovery FOR the crash-recovery
machinery (the registry lock, the one-byte framing edge, the ledger's
own torn writes), the owner's teardown surviving the deleted checkout
it must tear down after, and three attribution/pairing rules. All
nine accepted and folded; the non-material summary fix applied too.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SLC-R7-001 | accepted | No within-claim kill order: components first lets a live orphan owner append `relaunched` and respawn mid-sweep, then close clean over survivors — the incident's defining behavior. | D-4: the OWNER DIES FIRST (the incident's own remediation, now a rule), then components, then a POST-KILL re-reduction — never a pre-kill snapshot — before `reaped`; residue sets `sweepPending`. |
| SLC-R7-002 | accepted | A provisioner pausing past grace loses its unbound custody to compaction, then arms with a dangling custodyId — the ungoverned owner returns. | REG-2: a custodied `arming` is GUARDED — refused unless the referenced custody still reduces open; provisioning fails instead. |
| SLC-R7-003 | accepted | The registry lock had no owner record, takeover rule, or malformed-lock outcome; a holder killed mid-append wedges arming and reaping forever. | REG-4: lock owner.json written atomically with (pid, pidStartedAt); takeover only after proven holder death; ownerless lock past the publication window equally takeable; uninspectable is alive. |
| SLC-R7-004 | accepted (critic executed the parser check) | A crash after the closing brace but before the newline leaves valid JSON, so the not-valid-JSON repair never fires and the next append concatenates two objects into fatal corruption. | REG-1: two-part repair — no trailing newline + valid JSON final line means append the newline alone; not-valid-JSON means terminate, torn marker, payload. |
| SLC-R7-005 | accepted | The committed `stop_identity` verifies identities via process-census.py INSIDE the checkout; after deletion the helper is gone, every component reads dead, and no signal is sent — teardown silently no-ops. | D-1: teardown from MEMORY and SYSTEM BINARIES only (held identities, direct ps, pgroup signals); terminal appends self-contained; replacing the helper dependence is a named implementation item beside the trap. |
| SLC-R7-006 | accepted | The teardown ledger was called durable and authoritative with no framing, torn-write, or recovery rule — a driver killed mid-append is undefined between rerun, early teardown, and wedge. | D-3: the ledger inherits REG-1's framing, torn-run tolerance, and repair verbatim; a partial `teardown-due` is a torn fragment and NOT due, so completion re-runs. |
| SLC-R7-007 | accepted | `launched` retries can land after a newer generation's records; "latest" was undefined between append order and generation, so reduction could pair old pids with new tags and miss the current group. | REG-3: pairing is BY GENERATION — current set = highest generation's tags with that generation's identities; every recorded generation stays available to sweeps; stale retries can neither mispair nor hide a group. |
| SLC-R7-008 | accepted | The collision test covered open claims only; a minted tag equal to a closed, uncompacted claim's tag can neither open (absorbing terminal) nor be refused — deadlock or resurrection. | REG-2/D-6: a reservation is refused if its tag is SEEN AT ALL, open or closed; the armer regenerates its suffix and retries. |
| SLC-R7-009 | accepted | `--shutdown` just sends TERM; the owner cannot know the reason, so the required `shutdown` record needs an invented protocol. | D-1: shutdown-intent file written beside owner.json before signalling; intent present → `shutdown`, absent → new reason `terminated` (REG-2 enum extended). |
| SLC-R7-N001 | accepted (non-material) | D-6's numeric summary omitted closed sweepable claims that D-4/REG-3 count. | D-6 summary now lists all four slot classes. |
