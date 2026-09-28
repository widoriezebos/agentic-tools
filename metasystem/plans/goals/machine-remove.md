# machine-remove

- State: queued
- Risk: severity=3 novelty=2 exposure=2 accumulation=1 basis="Severity 3: it deletes a checkout and can delete evidence; a defect loses unlanded work or another seat's evidence. Novelty 2: composes existing stop, claim, presence, archive and disposal owners. Exposure 2: a person's act on this fleet's machines. Accumulation 1: rare, one-off acts."
- Tier: 3
- Intent: Wido 2026-09-28: verbs that obliterate seats and clean up their evidence. metasystem machine remove NAME [--obliterate] is the inverse of machine start: one previewable plan that stops the seat, releases its claims, presence and enrollment by recorded acts, archives every ref, stash, index and submodule to refs/archive/NAME/* locally and on origin, removes the checkout and its stores; --obliterate also disposes the seat's evidence through evidence dispose (tombstones and receipts). Only a person removes a machine; crash-resumable; idempotent.
- Origin: human
- Next step: Design revision 2 is written: /Users/wido/LocalStorage/agentic-tools-evidence/machine-remove-20260928/machine-remove-design.md (Astra round 1 folded, 17/17). m1e decisions on its open points: a lost host is removed by a recorded person's word --host-lost (H1: never leave claims stuck); system stop publishes a final stopped presence record; the temporary-peer residue on an abandoned host is acceptable. Before build: fold the evidence-bound shape Wido picks for disk Part B, one Astra read, then Wido approves. Designed only; no build until approved.
- OpenedAt: 2026-09-28T20:13:03Z
- Revision: 1
- Labels: disk
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-28T20:13:03Z 8NAS9X9WMSWH604A397YT0XFNM-m1e-c6925449 open actor=human:Wido targets=machine-remove
Integrity: sha256=1fbf7398d31b6d5864447b0d4ab999dc921058e380ff4989989407b79ca540df
