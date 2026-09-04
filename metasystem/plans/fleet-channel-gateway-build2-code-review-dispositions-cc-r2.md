# fleet-channel-gateway — dispositions, build step 2 code review, first root, round 2 (job fcg-build2-cc-r2)

The register carry on the round-5 tree (reviewedTree 98f0b886): F-1 withdrawn as fixed and proven; one new note. The narrative is in plans/fleet-channel-gateway-build2-code-review-dispositions.md.

| Finding id | Disposition | Reasoning and evidence | Amendment |
|---|---|---|---|
| F-1 | noted | withdrawn by the critic: the fake reloads the controls under the lock on every tick before the delivery filter, pause and conflict stay arrival-time, a malformed file mid-poll ends the request with the 500; the critic named the assertion that fails without the reload | none; the round-1 disposition (accepted, round 4) stands |
| F-9 | noted | round 2 of this root (fcg-build2-cc-r2, on the round-5 tree, F-1 withdrawn as fixed): writeControl and any fixture writing control.json by truncate-then-write can be read empty between the two by the fake's per-tick reload, ending an in-flight poll with a 500; a flake hazard for the step-3 fixtures, not a defect in the fake, whose loud-on-malformed rule is the specified one; the same fact as the second root's F-2 | carried to the cut-over step's fixture brief as a rule: every control.json writer writes through a temporary name and rename, as the fake does for its base-url file |
