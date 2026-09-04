Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal dispatch-engine-script-skew-silent-exit)
Date: 2026-09-04

# Build brief: the dispatcher must refuse loudly when the engine is older than its scripts

Goal `dispatch-engine-script-skew-silent-exit` (tier 1, approved by Wido 2026-09-04, box 1 hour / 3 attempts / 1 active job / no review round). No critic in this chain.

## The defect

`scripts/agents/dispatch.sh` runs under `set -e`. Its helper `json_value` (line 150) calls `metasystem json get --value <json> --field <field>`; when the field is absent the engine exits non-zero without a message, the script dies bare, and the delegate wrapper reports only `exit status 1`. This happened on m2 on 3 September: after a pull that brought the model-alias landing, the script read the roster field `aliasedFrom` (lines 1293 and 1791) while `bin/metasystem` was still the build from before that landing. Three dispatches were refused with no reason until a bash trace found the line.

## What to build

1. **A skew preflight in the dispatcher.** The engine is linked with a commit stamp (`go-build.sh` sets `internal/supervise.BuildStamp` to the commit it built from; the value already surfaces as `engineBuild` in the enrollment ledger). Add an engine verb, or extend an existing informational verb, that prints the build stamp, and a dispatch preflight that compares that stamp with the checkout: when the stamp is a commit that is an ancestor of HEAD and any file under `internal/`, `cmd/`, or `scripts/agents/` changed between the stamp and HEAD, refuse with a message naming both commits and the remedy `scripts/agents/go-build.sh`, then `steward arm`. A `dev` stamp (unstamped local build) is reported once and allowed. The check must cost one git command and never fail dispatch on its own errors (an unknown stamp commit, a shallow clone) beyond a one-line warning.
2. **A named refusal from `json_value`.** When the field is absent, `json_value` prints `dispatch: roster field <field> is missing from the engine's output (engine older than the scripts? run scripts/agents/go-build.sh)` to stderr and exits with a distinct status; call sites that lawfully accept a missing field (`aliasedFrom` may be null) keep working: distinguish "present and null" from "absent".

## Verification

Add a leg to `scripts/agents/dispatch-fixtures.sh` that builds an engine, commits a change under `scripts/agents/`, and shows the dispatcher refusing with the skew message; and a unit test for the json verb's absent-field status. Your sandbox cannot run the fixture suites (KI-15); run `go test ./cmd/... ./internal/...` for the verb, `bash -n` on the script, and show the refusal message by invoking the preflight function directly with a stale stamp. The orchestrator runs the fixture suite seat-side.

## Bounds

Touch `scripts/agents/dispatch.sh`, `scripts/agents/dispatch-fixtures.sh`, the json verb in `cmd/metasystem/json.go` and its test, and one place that exposes the stamp. No docs beyond one line in `docs/orchestration.md` if the new verb is user-facing. Return within the box.
