# Unit U1 of lane-proves-at-the-batch-risk: the `impact` proof scope with its own provenance, static first

Working Mode: Implement. Design: `plans/designs/lane-proves-at-the-batch-risk.md`, decisions D1a and D1b (read them first, with the critique notes they answer). Size: at most 250 production lines. Smallest change.

## What exists

- `internal/landing/plain/prove.go` and `scope.go`: scopes `full`, `gate`, `scoped`. `scoped` inherits a batch's earlier full proof (scope.go ~98: `d.base.FullAt` must parse and be under an hour old; `describe` ~404 carries `FullTree/FullAt` into the result); prove.go ~1166 treats an empty environment fingerprint on a `scoped` run as grounds to run full again.
- `cmd/metasystem/test_impact.go`: the cheap runner; ~86-96 skips contract groups whose inputs cover `cmd/**` (so `fast-static-build` never runs here); ~145 `runNamedTestGroups` prints no `landing environment` line.
- `internal/repoproof/full.go` ~149-160: the full reporter runs its static phase only when `full` is true; it prints `landing environment ...` as its first line (the fingerprint source).
- `Result` (prove.go ~107) has `Scope`, `ScopeReason`, `Base`, `Ran`, `Environment`, `FullTree`, `FullAt`.

## Build

1. A new scope value `impact`, chosen by `landing prove --impact` (flag; the policy that chooses it automatically is U2). Its provenance is its own: `Base` = the batch base commit and tree (the fetched main the batch's first merge started from, as the batch record holds it), the plan hash (sha256 of `metasystem test impact --plan --base BASE` output), and the environment fingerprint. It never reads or writes `FullTree/FullAt` and needs no earlier full green. `describe` leaves `FullTree/FullAt` empty for `impact`.
2. The impact runner prints the same `landing environment ...` header line as the full reporter (factor the line's producer so both use one function), and the proof reads it into `Result.Environment`; an empty fingerprint on an `impact` run is a proof error named in `ScopeReason`, never a silent re-run at full.
3. At `impact` scope the proof first runs the contract's `fast-static-build` group (build, vet, static) through the same group runner the full uses, then the impact selection; a red in that group is the proof's red with the group named.
4. `landing status` and the result record show `scope impact`, the base, the plan hash prefix and the fingerprint presence.

## Tests (new or changed only; mutations named)

- A first low-tier-style landing fixture with NO prior full proof of its tree proves at `impact` scope green and the push is admitted (mutation: make the scope inherit `FullAt` -> the test fails because no full exists).
- Impact tests pass while a `fast-static-build` stub fails -> the proof is red naming the group (mutation: skip the group -> green, test fails).
- The runner prints the environment line and the result carries the fingerprint; an empty fingerprint -> proof error, not a full re-run (mutation: drop the line -> error asserted).
- `scoped` and `gate` behaviour unchanged: run the existing scope tests.

Then `go run ./cmd/devgate static`, `go test -count=1 -timeout 30m ./internal/landing/plain ./internal/repoproof`, every existing test using a seam you changed (grep, run by name), and the cmd tests by name (`TestLandingProve|TestTestImpact|TestLandingStatus|TestReadStatus|TestScope`). Never edit testing.json; never open metasystem.conf.local. Report: diff --stat, each exit, the status text.
