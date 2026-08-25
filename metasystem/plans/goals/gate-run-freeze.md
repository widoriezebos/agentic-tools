# gate-run-freeze

- State: queued
- Intent: Goal verbs and commits refuse mechanically while a registered gate run is live on the checkout, outside the run's own chain — the ledger freeze during batteries becomes machinery (IL-29, retro-2026-08-25)
- Origin: main
- Next step: Appetite: 4h. Intent: no state mutation can kill a running battery again; the gate-registration marker already exists (gate register/fence) and the commit wrapper already consumes a fence — the goal family does not. Constraints: the running suite's OWN chain stays exempt (a battery must not deadlock itself); refusals name the live run and its age; a dead run's marker stops blocking (the existing staleness law). Freedoms: where the check lives inside internal/goal, whether CAS publishes are also fenced or only local writes, and the exact refusal wording. Evidence: two batteries killed in one day by the coordinator's own mid-run ledger writes (suite-failures 20260825T071434Z-adopt-33526). Queue: next after the retro closes — it protects every battery that follows.
- OpenedAt: 2026-08-25T12:31:23Z
- Revision: 1

History:
- 2026-08-25T12:31:23Z 1JP1BD1SMAH7M1DE9A2EBSH2XQ-m1-bf243850 open actor=m1+coordinator targets=gate-run-freeze
Integrity: sha256=147735b325eb3f6eb520b08d19d2a23e33c11a402f68b0d71abf80695a81138e
