Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal return-normalisation-hides-boundary-mismatch)
Date: 2026-09-05

# Follow-up: the critic's register, and the real cause of the fixture leg

The code critic's register for your round is in chain rnb-build1-cc1 (findings RNB-01 to RNB-07). Three are material and accepted:

- RNB-01: the dispatch fixture's `diff-boundary-mismatch` leg fails because of the conformance-round-immutability rule landed on 2026-09-01, not because of the normalisation: the fixture persists a successful review for round 1 and then re-runs review over the same round after adding an undeclared file, and the immutability rule refuses that before the boundary check. The brief's diagnosis was wrong. Fix the leg in `scripts/agents/dispatch-fixtures.sh` so it tests the mismatch on a round that has no successful review yet (a fresh round, or the mismatch before the first success), keeping the refusal text it asserts and the declared-then-success half; the boundary widens to that script for this one leg.
- RNB-02: make the unit test mirror the fixture's real order on a fresh round.
- RNB-03 is resolved: the orchestrator ran the validator package green.

Keep the narrowing of the normalisation (only entries that resolve under the actual metasystem installation). Findings RNB-04 to RNB-07 are noted, no change. Seat-side the dispatch scenario also failed at the recollection leg under load ("did not conclude … after 2 completed recollection passes"); that is a separate item, not yours. `bash -n` on the script; `go test ./internal/validate/...`; `gofmt -l`, `go vet`, staticcheck. Every path in your return is relative to the repository root (starting with `metasystem/`).
