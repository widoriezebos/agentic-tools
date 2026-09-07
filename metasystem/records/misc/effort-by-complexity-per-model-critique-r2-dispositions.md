# effort-by-complexity-per-model, critique round two: dispositions

Chain ebc-build1 after three build rounds, critic ebc-critic2 (claude,
claude-fable-5-1, with a shell), reviewed tree
6d5883ccc7ab7ae8e1418d7d28f2ed1b19212de6, the whole chain diff. One
material finding, five notes. Seat proof on round three: the dispatch,
adapter, config and command packages, vet, gofmt and bash -n green;
the dispatch fixture bed is owed (goal fixture-review-by-date-rolls-over).

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F-1 | accepted, NOT folded: the goal is parked | Real and high: the closure gate (validateIndependentCritiqueReference in internal/dispatch/hazard.go) compares the critic record's recorded builderReasoningEffort by string equality to the new floor word high; critic records written before this lands recorded xhigh, so after the landing every governed chain with a pre-change critic record (other seats' open chains included) refuses to close. The fix is a fourth build round comparing the recorded word by its position in the runtime vocabulary (recorded at or above the floor passes) plus a third critique, and the goal's tier-2 box allows two review rounds. | Parked for Wido: either a review-round exception (one more critique) or his word that the closure-gate fold may land as a MECHANICAL chain without a critique. |
| F-2 | noted | A follow-up on a pre-change chain on a no-switch runtime refuses loudly (the parent's old floor word is not in its vocabulary); no live devin chains exist. | Fold with F-1 when the goal resumes: treat a pre-change parent word outside the vocabulary as the no-op. |
| F-3 | noted | Follow-ups on pre-change claude chains will send --effort xhigh (the parent's recorded value); consistent with the brief; Fable's high applies to new chains. | none. |
| F-4 | noted | Effort keys mirror the model keys' file precedence but not the environment-override precedence; out of the threat model. | none. |
| F-5 | noted | Recorded-equals-received is proven on the record side and in the Go argv test; the claude shell hop has no stub-CLI bed. | none; a coverage note. |
| F-6 | noted | Two Go test failures in the critic's sandbox are the doubled-slash temp path family (goal claude-delegate-scratch-cleanup, built, waiting to land). | none. |
