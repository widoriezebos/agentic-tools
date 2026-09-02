# governed-exhaustion-reprojection

- State: queued
- Intent: The governed exhaustion check decides on a reserved figure frozen at admission (internal/dispatch/governed.go:149 ReservedBefore; internal/run/conclude.go:316-318 ReservedBefore + observedMinutes >= limit) and never re-projects at conclusion, so open caps that have since settled still count and a failing attempt can be marked exhausted, raise retroactive debt and block later governed work on spend that no longer exists. DONE means exhaustion at conclusion is decided on a fresh projection taken at that instant, excluding the concluding run from both the run records and the durable attempts, retry-safe after a partial commit, with every production store that concludes a governed run carrying the projection seam
- Origin: main
- Next step: Appetite: 4h, full ladder. Split out of dispatch-cap-necessity on 2026-09-02 by m1b (R-4: residue demands a token) so the accounting fix stays in its box; the machinery is already designed and critiqued through three rounds: plans/dispatch-cap-settlement-design.md revisions 3 and 4, section 4.3 (ProjectSpend seam on run.Store, dispatch.NewConcludingRunStore as the one constructor for every concluding site: cmd/metasystem/run.go, cmd/metasystem/supervise_component.go, internal/lease/sweep.go; ErrNoSpendProjection when the seam is absent; exclusion of the concluding run by run id from both stores) and the open findings against it in plans/dispatch-cap-settlement-dispositions-r3.md and the round-4 return of chain cap-settle-crit (DCS-R4-EXCLUSION-HIDES-DUPLICATE-OWNER: the exclusion must still validate duplicate durable owners for the run id; DCS-R4-T13-POST-DEBT-RETRY: the retry fixture must model the window before the run-record write, after retroactive debt). A builder starts from those sections and folds the two findings. Sequenced behind dispatch-cap-necessity, whose settled meaning of ReservedJobMinutes this consumes.
- OpenedAt: 2026-09-02T18:37:29Z
- Revision: 1

History:
- 2026-09-02T18:37:29Z HPTWTQNYQHY9BEMEMGQN383W4Y-m1b-fad3674e open actor=m1b+main-1788333346-60696-6a3256 targets=governed-exhaustion-reprojection
Integrity: sha256=813a584ed52384af3ca665f9fc02d361045aad40fa3e2ad3bd10d62f7d468474
