Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-04

# Goal

Second correction of slice 2b (chain str-p2-build-2c, your round-2
tree aea3bbef). The re-review (dispositions in
metasystem/records/misc/severity-tiered-rigor-p2-2b-critique-cc2.md)
confirmed all nine round-1 fixes and found three material defects and
three notes. Fix them in your working tree, gate, return. Contracts
unchanged. This is the last correction this chain gets.

# Fixes, in order

1. F-1 (critical). `designBlobSource` in metasystem/internal/dispatch/build.go
   accepts a blob id only if it matches `incarnationRe` (64 hex, the
   content-digest pattern). Git object ids are 40 hex in a SHA-1
   repository, which this repository and the fixture bed are, so every
   design-critic dispatch is refused "is not a blob at reviewed
   commit" after landing. Fix: a separate object-id pattern, 40 or 64
   hex, used for the blob id only; content digests keep `incarnationRe`.
   Test: TestBuildRecordDesignCriticCarriesDeclaredOutputs initialises
   a DEFAULT git repository (no `--object-format`) and asserts the
   source is `<design path>@<40 or 64 hex>`; keep one SHA-256 case
   only if it is cheap.
2. F-2 (high). The cap-driver and cap-warden block of
   metasystem/scripts/agents/dispatch-fixtures.sh (the block that
   dispatches the flag-runtime code-critic root reviewing review-target
   and the warden chain, roughly lines 2124 to 2202 in your tree) still
   encodes the removed first-exhaustion regime. Rewrite it to the
   regime your diff built: run conformance on the reviewed implementer
   round before the critic follow-up so its diff.patch exists; give
   every rigor row a canonical `artifact` inside the subject set;
   expect the refusal texts the engine now emits ("review-round limit
   is exhausted" and the next-step texts of fix 9 in the previous
   brief) instead of "implementer follow-up" and "warden critique
   budget exhausted"; drop the successor-reopened retry of round four
   if the new regime has no such path, or assert the path it has.
   Because the bed adopts at the git toplevel and the artifact grammar
   requires the nested project name (F-3 below), the reviewed
   workspace of these two scenarios must be nested: the bed's
   review-target implementer works in a `metasystem/` directory of its
   repository (as the design file you copied in round 2 already does),
   so its diff.patch carries `metasystem/...` paths. Nesting the bed
   is a mechanical choice; record it. Do not change the artifact
   grammar. Run the two scenarios; paste their final lines.
3. F-3 (out of scope). The empty install prefix at a toplevel adoption
   contradicts the artifact grammar; that is the design owner's and is
   recorded as backlog. Do not change `validArtifactPath` or
   `projectInstallPrefix`.
4. F-6 (note). Remove the dead `firstSevereExhaustion` branches in
   CritiqueExhaustionAdvance (metasystem/internal/dispatch/critique.go)
   and the constant if nothing else reads it; staticcheck must stay
   silent.
5. F-5 (note). Add one assertion each on the legacy (counters absent)
   path: a cancelled round is not counted; the exhaustion advance
   works on a root missing both counters.
6. F-4 (note), only if it costs under five minutes: parse a plain
   `diff --git` header by its ` b/` separator so a path with a space
   is kept; otherwise leave it and say so.

# Gate

`scripts/agents/go-gate.sh --fast` silent; `go test -count=1 ./internal/dispatch/
./cmd/metasystem/ ./internal/validate/` green; for internal/goal only
`go test -count=1 -run 'STR|Accept|Obligation|Discharge' ./internal/goal/`;
return-schema-fixtures.sh; the two rewritten dispatch-fixtures
scenarios in your sandbox if it can run them, else the exact refusal
(the seat reruns the full gate and all three fixture scripts on your
tree after the return). Stage nothing, no commit wrapper, no plans or
records. diffBoundary: every file that differs from main. Remove the
stray `internal/dispatch/build.go.orig`.

# Constraints

Wall-clock budget: 45 minutes; return by minute 40 whatever the
state, listing what is fixed and what is not. Version-2 implementer
JSON with the test names and the gate lines.

# Gap Rule

stop and report a gap; never fill it silently. The grain: mechanical
choices are recorded under `decisions` and built; only a law-changing
contract stops you.
