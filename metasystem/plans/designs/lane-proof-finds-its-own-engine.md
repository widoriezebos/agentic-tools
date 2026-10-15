# Design: the lane's own launches find its engine, and an unrun check is no verdict

- Id: 01M4Z0LANEPATH0000000000A1
- Kind: design
- Status: accepted
- Goals: lane-proof-finds-its-own-engine
- Evidence: lane test finding 325 (2026-10-15 06:51), intervention S162, root-cause read 07:02; findings 326 (a landed batch in the headline) and 327 (an incident outliving its fix) are related UX and go to the follow-up goal.

## Problem

At 06:51 on 2026-10-15 the lane's first due cadence on the new clock could not start its proof. The keeper runs as a pinned engine (`artifacts/agents/steward/engine-pins/generation-N-<hash>`); the proof's first command is bare `metasystem landing status --json` (`internal/repoproof/full.go:70`), resolved against a PATH that `commandenv.EngineEnvironment` prefixes with the pin directory (`internal/landing/plain/prove.go:1245`), which holds no file named `metasystem`. Earlier pinned launches worked only because runners re-armed by the landing agent's session inherited `<install>/bin` on PATH (`internal/launch/landing.go:35-41`); a pinned engine started by `landing deploy` from a person's shell did not. The lane then read the unrun proof (`LANDING-NOT-RUN environment`, `report.go:49`, `replay.go:42-54`) as "main is red", named every open incident as its cause (`shadow.go:220-234`, `status.go:564`), and the clock counted it as a current full check (`wake.go:123-128`), suppressing the next cadence. The same latent pattern sits in the repair and regeneration launches (`resolve.go:587`, `conflict/regeneration.go:73-75`).

## Decisions

D1. Every lane-owned launch whose child may call `metasystem` gets the registered installation's own `bin` first on PATH, from the call context, never from the proof's execution directory; a bare `metasystem` or `env` in a generated argument array is resolved to the installation's binary (or `/usr/bin/env`) before the command is constructed. A missing or non-executable installation binary refuses before any child starts and never falls back to another copy; that refusal classifies as a non-run.

D2. A proof that did not run is no verdict. Every parsed `LANDING-NOT-RUN` is normalised at the shared `runCheck` boundary to a non-run environment failure with `CountedFull=false` regardless of exit status, for trunk and batch proofs alike: no reusable green, no push authorisation, no cadence credit. Status selects the latest applicable trunk result for the current main commit; when it did not run, main reads "unproven" with its reason, names no incident but its own, in text and JSON alike, and says what happens next ("the lane will check main again at its next wake", or the recorded retry time, or the one person action when the environment allowance is spent). No new retry allowance is invented.

D3. The clock counts only a proof that actually checked main: green, or red with `CountedFull`; an unrun proof leaves the cadence due within the existing retry allowance and back-off.

D4. UX: the person reads "main's last check could not run (<plain reason>)" and the one thing to do next; no record paths, ledger terms or seat names.

## Units

- U1 (D1–D4): brief `evidence/briefs/lane-proof-finds-its-own-engine-U1-brief.md` (Astra 7 → 4 → 3 → 3 material, stopped on the non-falling count, the last three folded). ≤160 production lines.
- U2 (truth of a red): a red names the check that failed and nothing else (the 23:30 10-14 mislabel: a flake entry on a test that passed); a confirmed-flake incident closes once its fix goal is done and a full proof is green without it (finding 327). ≤120 lines.

## Not in scope

Generic application launching (`cmd/metasystem/app.go`) and receipt copies; the headline and queue UX (follow-up goal "the lane says what it does").
