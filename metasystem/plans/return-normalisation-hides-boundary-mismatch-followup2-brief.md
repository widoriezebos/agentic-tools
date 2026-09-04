Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal return-normalisation-hides-boundary-mismatch)
Date: 2026-09-05

# Follow-up: the four legs after the corrected one

The second critic (chain rnb-build1-cc2) resolved RNB-01 and RNB-02 and raised RNB-21, accepted: in `scripts/agents/dispatch-fixtures.sh`, the untracked-plan, committed-plan, uncommitted-plan and control-plane-change legs that follow the corrected `diff-boundary-mismatch` leg re-run `validate conformance --stage review` over the round after its first successful review has persisted `review.json`, so under the round-immutability rule (2026-09-01) they meet the immutability refusal instead of the refusals they assert ("trusted plans/ state changed", "agent control plane contains delegate-created files"). Move each of those legs so its review runs on a round with no persisted success (before the first success, in the order the legs need, or on a fresh round each), keeping every asserted refusal text and every declared-then-success half, and read on to the end of the scenario for any further leg with the same shape. RNB-22 is resolved (the validator package ran green seat-side on your round-2 tree). `bash -n` on the script. Every path in your return is relative to the repository root (starting with `metasystem/`). The orchestrator reruns the suite seat-side.
