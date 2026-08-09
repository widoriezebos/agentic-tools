# Dispositions: supervision lifecycle, round 6 — narrowing fold failures

Round 6 (gpt-5.6-sol, job supervision-lifecycle-r6, 10 material,
verdict NOT-CONVERGED) verified the round-5 folds. The count fell
from 18 to 10 and the kind narrowed again: decision-table priority,
slot accounting, one missing invocation shape, gating semantics, one
ordering rule, crash-recoverable repair, and terminal-path
canonicalization. No reframes. All ten accepted and folded into
`plans/supervision-lifecycle.md` and `plans/supervision-registry.md`.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SLC-R6-001 | accepted | Deleting the checkout removes owner.json too, so the written table classified it SUPERSEDED while the Proof demanded purpose-gone. | D-1: PURPOSE GONE is checked first and owns the deleted-checkout case; SUPERSEDED requires the checkout root to persist. |
| SLC-R6-002 | accepted | Threshold compaction between `custody` and `arming` drops a fresh unbound custody (retention listed only bound custody), recreating the ungoverned owner. | REG-3: compaction also retains unbound custody inside its grace window. Proof: custody vs compaction. |
| SLC-R6-003 | accepted | A reservation stopped consuming a slot at grace expiry even with its detached owner alive, and nothing forced cleanup before the next admission. | REG-3/D-4: a reservation holds its slot until CLOSED; grace makes it ACTIONABLE, and the GATE resolves actionable claims itself before granting slots. |
| SLC-R6-004 | accepted | The kill proof listed only component shapes; the establishment orphan's live owner runs `arm-supervision.sh __owner --tag <tag>` and was legally unkillable — and it respawns components, the incident shape. | REG-6/D-4: the OWNER invocation shape joins the known signatures. |
| SLC-R6-005 | accepted | `teardown-due` before "completion state" did not name the scorecard; the committed driver creates it during grading and refuses it on re-entry — the exact wedge survives. | D-3: the scorecard IS completion state; `teardown-due` precedes scorecard creation. Grading does not need the target's supervision, so recovery teardown in that interval is safe. |
| SLC-R6-006 | accepted | REG-5 called `relaunched`/`launched` best-effort while D-2 called them write-ahead; skipping them silently produces processes outside custody. | REG-5/D-2: custody-creating appends are GATING (no relaunch without a landed `relaunched`; failed `launched` retried, persistent failure trips the breaker); only terminal appends stay best-effort. Proof: unrecordable set. |
| SLC-R6-007 | accepted | A closed sweepable claim consumed no slot, so unprovable surviving component leaders accumulate across checkouts while the cap reads free. | REG-3/D-4: sweepable closed claims consume a slot until `swept`. Proof: hidden survivors extended. |
| SLC-R6-008 | accepted | Only owner-dead closures had a sweepable rule; checkout-gone, custodian-dead, and establishment-orphan reaps can also finish unproven, with no schema field to say so. | REG-2: `reaped` carries `sweepPending`, set on ANY unproven residue; REG-3: sweepable = exited.teardownComplete false OR reaped.sweepPending true. |
| SLC-R6-009 | accepted | The torn repair could itself crash between newline and marker, leaving a fragment no later repair can legalize — a permanent fail-closed wedge. | REG-1: tolerance restated over runs — a non-JSON line is tolerated iff every later valid record is separated from it by a `torn` marker; trailing garbage always tolerated; repair idempotent at any crash byte. |
| SLC-R6-010 | accepted | REG-1 required canonicalizing before every write, but terminal writers speak for checkouts that no longer exist to resolve. | REG-1: canonicalization happens at record creation; `exited`/`reaped`/`swept` reuse the claim's recorded path verbatim. Proof: terminal path. |
