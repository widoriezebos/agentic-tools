Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal supervision-custody-per-checkout)
Date: 2026-09-05

# Follow-up: the critic's register, two material findings accepted

The code critic's register for your round is in chain scc-build2-cc1 (findings SCC-01 and SCC-02; read its return for the full text). Both are accepted:

- SCC-01 (critical): in `EnsureArmed` the guard `requireOwnerCheckout` receives `options.Scope`, the git top-level, as the requested checkout, while the registry stores the state root (`options.Root`, the value the owner is launched with and `RegistryLedger.CheckoutPath` records). On any checkout whose git scope differs from its state root, which includes this repository (the git top is the outer repository, the state root is its `metasystem` directory), ordinary re-arming is refused. Compare the same path on both sides: the canonical state root the record carries, resolved the way the record writer resolves it, and make the invariant test cover a checkout whose git scope and state root differ, so the test would have caught this.
- SCC-02 (medium): the suite's self-check (`assert_fixture_supervision_isolation`) watches an environment variable, an audit file only `arm-supervision.sh` writes, and announcement files under roots registered through `make_repo`, but the stop-hook-monitor scenario arms by calling `metasystem up` inside the scenario, which none of those see. Make the self-check cover that path too: every supervision bring-up a scenario performs, by whatever verb, must be under the scenario's own registry home and main identity, and the check must fail the suite when it is not.

Keep everything else. Run `gofmt -l`, `go vet`, staticcheck on `./internal/registry/... ./internal/supervise/...`, `go test ./internal/registry/...` and the invariant tests in `internal/supervise` by name, `bash -n` on both scripts. Return within one hour. Every path in your return is relative to the repository root (starting with `metasystem/`). The orchestrator runs the supervision suite seat-side and re-arms this checkout as the proof for SCC-01.
