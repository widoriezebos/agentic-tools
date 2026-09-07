# effort-by-complexity-per-model, critique round one: dispositions

Chain ebc-build1 after two build rounds (round one stopped on the gap
rule for two files outside its boundary; round two widened it and
finished), critic ebc-critic1 (claude, claude-fable-5-1, with a
shell), reviewed tree c9826186b6a2d5855ec50564c84f40b8fcc195ee. Seat
proof: the dispatch, adapter, config and command packages, vet, gofmt
and bash -n green. The dispatch fixture bed could not certify the new
legs tonight: its approve-relay leg refuses because the temporary
authority horizon (a code constant, 2026-09-06) expired at midnight
UTC, which is recorded for Wido on goal fixture-review-by-date-rolls-over;
the bed is owed once that is decided.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F-1 | accepted | Real: the follow-up path demands a recorded effort source and every job record from before this change lacks one, so no open chain could continue; the builder resolved the brief's silence with a refusal instead of reporting the gap. | Folded in round three: an absent source on the parent record is read as the parent's recorded effort with source "record:pre-effort-keys" (no refusal), pinned by a test. |
| F-2 | accepted | reasoningEffortSource is not in the record's immutable field list while reasoningEffort is. | Folded in round three: added to the immutable list. |
| F-3 | accepted | The claude adapter forwards a literal "null" effort where the codex adapter clears it. | Folded in round three: the claude adapter clears null like codex. |
| F-4 | noted | The "critic below effort floor" closure test also swaps obligations, so its refusal is not the floor's; the floor is pinned in the roster test; the pattern predates the change. | none; recorded here. |
| F-5 | noted | With Fable at high the maximal-models proof is never consulted for claude and the tier word "maximal" is no longer backed by a runtime proof; the brief asked for this narrowing. | none; recorded here for the day a tier word must carry proof again. |
| F-6 | noted | An unlisted runtime now refuses every dispatch for lacking a vocabulary; none exists; the agent-agnostic doctrine would want the vocabulary in the adapter seam. | none; recorded here; the adapter-seam move is a later goal if a runtime is added. |
| F-7 | accepted | The channel-fixtures dates in the worktree were the orchestrator's own probe leftover (the bed run with the expired literal patched for the run), not the implementer's; restored from git before round three. | none in the chain; the probe recipe now restores through git. |
