Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal dispatch-fixture-critic-close-register-fold)
Date: 2026-09-04

# Follow-up: the channel fixture's budget question must carry its proposed box

Seat-side after your second round, `scripts/agents/channel-fixtures.sh` fails at once with: "a budget-above-norm question requires a complete valid proposed budget tuple: elapsedLimit "" is not a positive duration (for example 4h or 1d2h)" (exit 2). The budget-answer landing (3615da7a) made `channel ask --kind budget-above-norm` require the five limits (`--elapsed-limit`, `--attempt-limit`, `--reserved-job-minutes-limit`, `--active-job-limit`, `--review-round-limit`). Find every `channel ask` in the channel fixture (and any in the dispatch fixture) that opens a budget-above-norm question and give it a complete box that fits the leg (for example 2h, 5 attempts, 600 minutes, 1 active job, 0 review rounds), and keep any assertion on the rendered question text consistent with the "Proposed box" line the verb now renders. Keep your earlier changes. `bash -n` on the touched scripts. Every path in your return is relative to the repository root (starting with `metasystem/`). The orchestrator reruns the suites seat-side; this is the box's last round.
