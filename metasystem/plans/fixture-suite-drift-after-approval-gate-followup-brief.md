Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal fixture-suite-drift-after-approval-gate)
Date: 2026-09-04

# Follow-up: the dispatch scenario now fails one leg further

Your seven-file change is right as far as it goes: the channel suite is green seat-side, and the dispatch scenario now gets past the model-alias roster assertion. It then fails at the serving-goal leg (`scripts/agents/dispatch-fixtures.sh` around line 1647): `bin/metasystem goal open --root "$agent_repo" ... --tier 3` answers `flag provided but not defined: -tier`. After the suite ran, the checkout's `bin/metasystem` (rebuilt by the suite at 13:32 local) does not know `--tier`, while the engine built from the same checkout with `go build ./cmd/metasystem` does. So some build step in the suite (look at `build_fixture_engine`, the cap-engine and fixture-install legs, and anything that writes `bin/metasystem`) builds from an older or vendored source tree and overwrites the checkout's engine. Find that step, make it build into its own path or from the checkout's source, and make the serving-goal leg call the engine the rest of the scenario uses (`$engine`), not a cwd-relative `bin/metasystem`. This leg was latent behind the alias red, so it is part of "each script runs green on main".

Keep every path in your return relative to the repository root (starting with `metasystem/`). The orchestrator reruns the suite seat-side.
