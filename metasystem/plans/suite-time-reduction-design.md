# Suite-time reduction (REVISION 5, after critique round 4)

Status: r5. Round-1 critique (codex gpt-5.6-sol, xhigh): REVISE, ten
material findings plus forced answers to all five open questions. The
decisive ones (1, 4, 5, 9 — a persistent user-writable cache is
neither a trust boundary nor equivalent to a fresh gate, and races
around a shared key/binary are structural) reshaped Piece 1 from a
persistent cache into a BOUNDARY-SCOPED WITNESS: memoization owned and
handed down by the one outer controller within one validation run.
Finding 7 exposed a pre-existing fail-open that D17 created; fixing it
joins the design.

## Where the time goes (unchanged)

Boundary ≈ 45 wall minutes; D17 added a full go-gate (~2-4 min warm,
plus govulncheck's network round) to each of 3+ nested adopted
validations per suite run, on a Go tree byte-identical to the outer
one. Nested gates and nested fixture families dominate; same-afternoon
repeat gates are the smaller half.

## Piece 1 — the boundary-scoped gate witness

One validation run, one gate. The OUTER suite runs the full gate
exactly as today. On pass, it writes a witness INSIDE ITS OWN RUN
STATE (artifacts/agents/gate-witness/<run>/witness.json — never a
shared or persistent cache) recording: the input manifest digest
(below), go env identity, the ratchet baseline actually selected, the
govulncheck DB timestamp from the run, and the gate summary. It then
hands the witness path to the nested validations IT SPAWNS via one
explicit variable (METASYSTEM_GATE_WITNESS), which the adopt fixtures
set for their nested runs and nothing else sets.

A nested gate that receives a witness: computes the SAME manifest
digest over ITS OWN tree's gate inputs; digests equal AND the witness
belongs to the same outer run → accept, print
"go gate: PASSED (outer witness <key8>, this boundary)" — a wording
that cannot be mistaken for a fresh full gate — and still rebuild its
own binary (compilation and the D17 self-gating proof stay real).
Digest mismatch, absent witness, or any doubt → full gate, exactly as
today. The witness dies with the run (run-scoped directory; no TTL
semantics — findings 1/5, forced answer 4/5: TTL is never soundness).

Input manifest digest: sorted (path, mode, symlink-target,
content-hash) over cmd/**, internal/**, go.mod, go.sum, and ALL of
scripts/agents/** (forced answer 1's safer interim), hashed with git
hash-object; PLUS `go env GOOS GOARCH GOFLAGS GOWORK GOEXPERIMENT
CGO_ENABLED GOTOOLCHAIN`, `go version`, and the gate script's own
hash. Presence of go.work or vendor/ at either end → refuse the
witness (full gate). Untracked files inside the hashed roots are
INCLUDED by the walk (it hashes the filesystem, not the index).

Seed-mode fencing (finding 3): METASYSTEM_COVERAGE_RATCHET_SEED runs
never write and never read a witness. METASYSTEM_GATE_FORCE=1
disables witness acceptance. Concurrency (finding 9): the witness is
written once by the outer run before any nested run starts and is
read-only thereafter — no shared mutable key, no cross-run
single-flight needed; the existing gate fence keeps concurrent full
gates apart as today. Same-afternoon CROSS-boundary reuse is DROPPED:
canaries and per-commit gates keep paying the full gate (finding 1 —
govulncheck data and race schedules are time-varying; a fresh
boundary claims a fresh gate).

Trust statement (finding 5, stated plainly): the witness is a
memo from the outer controller to its own children within one run.
Malicious tested code is out of scope here exactly as it is for the
rest of the suite, which already executes fixtures with user
authority; the run-scoped location removes the standing writable
surface a persistent cache would have added.

## Round-2 findings, folded

1. THE SNAPSHOT ALREADY EXISTS: adopt stages nested payloads from
   `git archive HEAD` — an immutable tree. The witness therefore
   digests HEAD'S TREE OBJECTS for the gate-input roots (git
   ls-tree/hash-object over the committed content), and the witness is
   written ONLY when the worktree is CLEAN for those roots at gate
   start AND end (git status --porcelain over the roots, checked both
   sides — the pre/post manifest, cheap). Gate-on-worktree and
   staged-payload-from-HEAD are then provably the same bytes. The
   closure self-check: the gate runs `go list ./...` and REFUSES the
   witness if any package lives outside cmd/ or internal/ (closes the
   root-package drift); go.work, vendor/, or replace directives
   pointing outside the module refuse the witness as before.
2. DELIVERY MODE IS REQUIRED, NOT INFERRED: metasystem.engine-delivery
   becomes a required key — the template carries `source`, adopt
   stamps `source` into targets, and validation FAILS on a missing key
   anywhere a metasystem.conf exists (no "absent means legacy"; the
   ecosystem has no legacy targets worth a migration path, and a
   deleted key must read as damage, not as a mode).
3. BINARY IDENTITY JOINS THE EQUIVALENCE: the gate builds with
   -buildvcs=false and stamps the ENGINE-INPUT DIGEST (the witness
   key) instead of the enclosing repository's commit; the delivery
   contract's smoke asserts the stamped digest equals the digest the
   outer run validated. Byte-identical source now yields
   identity-identical binaries in template and adopted trees.
4. CONCURRENCY, CLAIMED HONESTLY: the witness removes the shared-cache
   race and nothing else. The standing one-validation-per-checkout
   doctrine (suite startup guard + gate fence, and the pinned-worktree
   pattern for local runs) is the guarantee against gate/artifact
   races, stated as doctrine rather than as atomicity the fence does
   not have.
5. WITNESS HANDOFF (round-2 answer 1): the env var stays; the
   receiving gate sanitizes it — honored only in --delivery-contract
   runs, canonical path beneath the current run's state dir, matching
   the run identifier, regular file, schema-valid — closing accidental
   and stale injection within the stated threat model.

## Round-3 findings, folded

1. THE WITNESS-WRITING GATE RUNS INSIDE THE SNAPSHOT: when the outer
   suite wants a witness, it extracts `git archive HEAD` to a run-
   scoped temp directory and runs the full gate THERE — the exact
   bytes adoption stages, immune to mid-run worktree mutation and to
   ignored/untracked files by construction (the archive carries only
   committed content, which is also exactly what nested targets get).
   The Go build/test caches are content-keyed and shared, so the
   snapshot gate pays seconds of extraction, not a cold build. The
   ordinary standalone gate (developer runs, canaries, per-commit)
   is UNCHANGED and never writes witnesses; witness ⇔ snapshot-gated,
   one implication, no third mode.
2. HANDOFF CHECKS, COMPLETED (round-4 completion): the controller
   creates the run-state directory and every witness parent
   owner-only (0700) and the witness itself 0600; the receiving gate
   lstat-verifies a non-symlink regular file with those permissions
   under that directory, and the expected run identifier and state
   root come from the controller's own environment, never parsed out
   of METASYSTEM_GATE_WITNESS itself.
3. ONE DRIFTED-TARGET LEG KEEPS THE FULL VALIDATOR: the profile-drift
   negative leg continues to invoke the canonical full entry point,
   proving the full suite still detects and rejects drift end to end;
   the other nested variants move to --delivery-contract. (This also
   answers r2 open question 2.)

## Piece 1b — close the D17 fail-open (finding 7)

adopt stamps metasystem.engine-delivery=source into the target's
metasystem.conf and the key is REQUIRED wherever a metasystem.conf
exists (round-2 finding 2): source with go.mod absent FAILS, and a
missing key FAILS — never infer "no engine expected" from missing
evidence. Template checkouts keep the existing sentinel-file damage
check on top.

## Piece 2 — the delivery contract, a separately named command

Instead of an env-scoped variant of the canonical suite (finding 8),
nested adopted validations run
`scripts/validate-metasystem.sh --delivery-contract`, whose verdict
line is "metasystem delivery contract validated" — never the full
suite's "metasystem validation passed". It proves what a nested run
uniquely proves:

- payload completeness against the allowlist, config identity, role
  assets, hooks installation, enforcement workflow presence,
  profile-drift detection;
- the go-gate (witness-eligible per Piece 1, full gate on any doubt),
  plus a post-build behavior smoke of the freshly stamped binary
  (finding 6: version prints, one decision verb answers);
- the session-isolation and second-session legs (forced answer 3:
  they exercise DELIVERED adapter-local configuration; the outer run
  is not a substitute);
- the outer adopt fixture additionally compares the staged payload's
  engine-input digest against the digest its own gate validated
  (finding 8's runtime equivalence, not prose).

Skipped: the engine-behavior fixture families, each skip justified by
the digest equality the outer controller verified. Adopted repos' own
CI keeps the FULL suite (their machine never saw the outer run).

## Piece 3 — the platform claim, stated honestly (finding 10)

- The VM suite is the per-boundary authority for Linux validity.
- Intermediate boundaries make NO Darwin-validity claim; that is
  already today's practice (the dossier's VM-authority ruling) and is
  now stated instead of implied.
- A Darwin full suite is REQUIRED before: any benchmark cohort start,
  any release/acceptance hand-off to Wido, and after any VM red that
  needs local reproduction. Otherwise Mac runs are sampling (every
  third batch edge), and a Mac-only regression risk between samples
  is an accepted, stated residual.

## Expected effect (revised)

- Nested gates: 3+ full gates per suite run → digest checks + warm
  rebuilds (seconds each), saving ~6-10 min per suite run.
- Nested fixture families under the delivery contract: ~6-8 min per
  nested run → ~1-2 min (session legs retained).
- Cross-boundary repeats: unchanged (deliberately — soundness).
- Boundary estimate: VM ~20 → ~10-12 min; the same shape on Darwin
  when it runs.

## Verification

- Witness legs: nested hit on identical staged content (witness line
  in adopt-filled.out); one-byte mutation in each input class in the
  STAGED target → full gate runs (and fails when the mutation
  breaks it); seed-mode never writes/reads; FORCE bypasses; go.work
  or vendor presence refuses the witness.
- Delivery-contract legs: its verdict line differs from the full
  suite's; payload-digest equality asserted by the outer fixture;
  session legs present in its output; the D17 fail-open leg — a
  source-delivery target with go.mod deleted FAILS.
- The measured before/after wall times land in the decisions entry.

## Open questions for round 2

1. Witness handoff shape: env var path vs writing the witness into
   the staged target before its validation — is the env var an
   injection surface worth closing (only the adopt fixture sets it,
   but nothing enforces that)?
2. Does the delivery contract subsume the existing drifted-target
   leg's full-run expectation, or does that leg keep the full suite
   to preserve its original subject (profile drift detection depth)?
3. Is the digest-equality check in the outer fixture (staged payload
   vs validated tree) the right place, or should the nested gate
   itself verify it received the outer run's exact content?
