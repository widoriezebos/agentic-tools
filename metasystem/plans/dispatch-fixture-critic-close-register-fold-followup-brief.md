Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal dispatch-fixture-critic-close-register-fold)
Date: 2026-09-04

# Follow-up: the fixture's goal open must answer the four risk questions

Seat-side the dispatch scenario now fails earlier, at its serving-goal leg: the fixture's `goal open` (the `fixture-serving` goal in `scripts/agents/dispatch-fixtures.sh`) answers "answer the four questions: --risk severity=,novelty=,exposure=,accumulation= --basis" (exit 2). The risk-basis landing (b4ae9395, this afternoon) made every `goal open` carry `--risk severity=<n>,novelty=<n>,exposure=<n>,accumulation=<n>` (each 1, 2 or 3) and `--basis <sentence>`; the tier follows from the answers, so drop the leg's `--tier 3` unless a `--why` override is intended. Find every `goal open` in the dispatch fixture (and in `scripts/agents/channel-fixtures.sh`, whose goals are opened the same way) and give each the four answers and a one-sentence basis that fit what the leg tests. Keep your critic-close changes. `bash -n` on the touched scripts. Every path in your return is relative to the repository root (starting with `metasystem/`). The orchestrator reruns the suites seat-side.
