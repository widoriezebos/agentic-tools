# Independent read: coordinator-context slice 3, unit B1

Reader: Opus 5, independent of the builder. Worktree
`/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccb-s3/metasystem`,
base `origin/main` at 449ec649 (units A1 and A2 landed).

Both halves were read: the tracked diff (`internal/steward/intervene.go`,
`stage.go`, `stage_test.go`, 254 additions and 17 deletions) and the three
untracked files a diff does not print (`internal/steward/handoff.go` 35 lines,
`handoff_state.go` 446 lines, `handoff_state_test.go` 325 lines). That matches
the builder's own line count of 1,060 changed lines.

## What was run

No fixture bed and no repository file was edited. All probes ran through
`go test -overlay`, which compiles substitute files from the scratchpad without
writing into the worktree.

- `go vet ./internal/steward/`: clean.
- `go test -count=1 ./internal/steward/` (full package): passes, 115 s,
  coverage 79.5 percent of statements. The builder's recorded full-package
  failure (nested fake-runner module downloads) does not reproduce here.
  Floors are 74.0 darwin (`scripts/agents/coverage-ratchet.json:73`) and 78.5
  linux (`scripts/agents/coverage-ratchet-linux.json:73`).
- Focused run of the B1 tests: all pass.
- Nine single-rule mutations of `handoff_state.go`, each run against the whole
  B1 test set, with one control mutation to prove the overlay takes effect.
- Four behaviour probes (state portability across nonces, cross-nonce
  reference, manifest byte bound, nextStep reference count).

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| F-1 | high | yes | CCB-3-07's named proof never reaches the state digest. Deleting the digest comparison outright passes the entire B1 test set. The only "state" tamper case appends a byte that also breaks strict JSON decoding, and the assertion matches the wrapper sentence, which every verification failure produces. | Rule at `internal/steward/handoff_state.go:425`. Tamper at `internal/steward/stage_test.go:129-142` appends `"x"`; assertion at `:203` matches only `handoff state drifted since the authorization was minted`. Mutation M5 (digest comparison deleted) ran the whole B1 suite green. Control mutation (removing `DisallowUnknownFields`) correctly turned two subtests red, so the overlay is effective. A probe confirms the shipped code does catch a valid-JSON edit: rewriting `nextStep.text` and leaving the digest stale is refused with `handoff state digest mismatch expected=7504b69... found=3118882...`, so the implementation is right and only its proof is missing. |
| F-2 | high | yes | The state-to-binding equality block, which is the whole answer to "what binds this file to this predecessor" and "is this file inside this installation", is unproven. Six further single-line deletions each pass the full B1 test set. | Each mutation run separately against `TestHandoffStateVerifierIsStrictAndBounded`, `TestHandoffStagesTheIntentWithThePredecessor` and `TestVerifyStagedDigests*`; all green. Rules with no failing test: `handoff_state.go:308` writtenAt equals RecordedAt (the anti-stale rule); `:314` normalizedSession is the normalization of the recorded original session; `:315` `state.Seat.Identity == binding.Predecessor` (the predecessor bind itself); `:322` `state.HeldGoal.ID == goalID`; `:340` the fixed disposable declaration; `:377` `binding.StatePath == expectedPath`. The last one matters most: with only the `filepath.Clean` half left, the existing `noncanonical binding path` subtest (`handoff_state_test.go:292-299`) still passes, because its fixture path is non-clean. The equality with the canonical owned path is the only rule keeping the bound state read inside this installation's handoff directory, and nothing exercises it. |
| F-3 | medium | no | `state.json` is bounded at 32 KiB before it is read, but the manifest it authorizes has no byte bound and no field-length bound, so the verifier certifies an arbitrarily large orientation record. The manifest's declared `Bytes` is known before the read, yet the read is unbounded. | `handoff_state.go:416` size-checks the state before `os.ReadFile`; `validateManifestReference` (`:234-249`) checks shape and then hands off to `dispatch.ReadVerifiedReference`, which does `os.ReadFile` with no size gate (`internal/dispatch/references.go:33-41`). Probe: a manifest carrying one receipt whose `note` is 4 MB verifies, with `state.json` at 1,964 bytes and `manifest.json` at 4,195,539 bytes. The brief tells B1 to reuse that reference owner and sets no manifest bound, so this conforms; but this verifier is the only read-side bound in the slice, and the brief's own "do not inline briefs, logs or large records" has no enforcer anywhere yet. Worth a bound when unit C is reviewed. |
| F-4 | low | no | The 50/20/50 inline caps apply only to the state file, so moving lists into the manifest removes the cap entirely, and a test locks that in as intended. D3-2 reads as capping the lists themselves, not just the inline half. | Caps at `handoff_state.go:352-360` test `state.OpenJobs`, `state.Scratch`, `state.MessagesOwed` only; the manifest's copies go through `validateHandoffLists` (`:349`) with no count check. Only `lastLandings` is checked combined (`:361-363`). `handoff_state_test.go:277-290` asserts that 51 messages in the manifest must verify. The v2 brief says "overflow goes to the owned manifest", which supports the build; D3-2 (`plans/coordinator-context-stays-under-budget-design.md`, section 1) says "Lists are capped (openJobs 50, scratch 20, messagesOwed 50)" and describes the manifest as the destination for a byte overflow. Reading is ambiguous; flagging so the seat picks one. |
| F-5 | low | no | The state file carries no handoff identity of its own. A byte-identical state verifies under a second nonce. Two handoffs are kept apart only by `writtenAt` and by reference open paths, and a state with no references and no manifest has neither. | Probe: a state stripped of `manifest` and `scratch` verifies under nonce `0000000000000aaa` and, copied byte for byte with the same digest, again under `0000000000000bbb`. A reference into another nonce's directory is correctly refused (`reference open path ... is outside the immutable handoff directory`, `handoff_state.go:225-227`), so the hole closes as soon as the state names anything. Not independently exploitable, since the binding lives in the trusted intent store. D3-2's field list has no nonce member, so adding one would be a design change, not a build fix. |
| F-6 | low | no | `HandoffDir` is exported and joins an unvalidated nonce into a path, and it silently swallows `filepath.Abs` and `EvalSymlinks` errors. Every B1 call site validates the nonce first, but units C and D are the units that build paths from operator input. | `internal/steward/handoff.go:27-35`. The nonce grammar is checked in `validateHandoffBinding` (`handoff_state.go:369`) before `HandoffDir` is used in B1 (`:331`, `:376`, `:434`), and `StageHandoffIntent` verifies the binding before touching `BriefPath` (`stage.go:130`), so B1 itself cannot escape the root. The brief assigns nonce validation to the verbs; validating inside the helper would make the trap impossible. |
| F-7 | low | no | `StageIntent`'s failure-path side effect changed. The installation-identity check now runs before the brief is written instead of after, so an unarmed installation no longer leaves an orphan brief. An improvement, but the brief said to keep `StageIntent` behaviour. | Old order at 449ec649: role, requirements, schema, permissions digests, then brief write, then `filepath.Abs` and `VerifyIdentity`. New order folds `Abs` and `VerifyIdentity` into `stagedDigests` (`stage.go:73-84`), which `StageIntent` calls first (`stage.go:91`). Success-path behaviour, digests and the returned intent are unchanged. |
| F-8 | low | no | `StageHandoffIntent` leaves its brief on disk when `digestFile` fails after the exclusive write, and a retry on the same nonce then fails with a bare "file exists". | `stage.go:165-171`. `writeExclusiveBrief` (`:36-51`) cleans up its own write and close failures but not a later digest failure. Not reachable while unit C mints a fresh nonce per attempt, and `context prune` does not delete briefs, so the residue is inert. |
| F-9 | low | no | Every handoff verification failure out of `VerifyStagedDigests` is reported as drift, including a missing, unreadable or malformed state file. The brief reserves that sentence for drift. | `stage.go:211-213` wraps any `verifyBoundHandoffState` error in `handoff state drifted since the authorization was minted: %w`. The wrapped cause is carried, so the seat can still tell what happened; only the headline is wrong. |
| F-10 | low | no | The `combined list limit` subtest name promises an inline-plus-manifest rule; its assertion is inline-only. | `handoff_state_test.go:262-275` puts 51 messages inline and expects `inline messagesOwed exceeds 50`. Only `lastLandings` is genuinely combined (`handoff_state.go:361-363`). Naming only. |

## The seat's seven questions, answered

**1. Schema and binding.** The handoff is identified by its nonce, which is
carried only by the directory path; the state file itself names no nonce
(F-5). The binding pins, and the verifier cross-checks, runtime, normalized
session, main id, the exact `identity.Ref`, tag and job of the predecessor
(`handoff_state.go:314-318`), plus `state.HeldGoal.ID` against the intent's
goal (`:322`). `RecordedAt` must equal `writtenAt` (`:308`), which is the rule
that stops a stale state from being read as a fresh one: two handoffs from one
seat cannot share a state file unless they were recorded at the same instant,
and two seats cannot, because each has its own nonce directory and each state
names its own predecessor. The predecessor must have `Pid > 0` and a valid
comparison mode (`:393`), so an empty or unusable identity refuses before
staging. The design is sound. The problem is that six of these rules, plus the
digest, have no failing test (F-1, F-2).

**2. Staged verification.** The same private verifier runs twice: once inside
`StageHandoffIntent` before any brief is written (`stage.go:130`) and again in
`VerifyStagedDigests` (`stage.go:211`), which the dispatcher's gate calls
immediately before launch (`cmd/metasystem/steward_verbs.go:402`). A file that
changed in between is refused there. A `seatHandoff` intent with no binding, and
a binding on any other reason, are both refused (`stage.go:204-210`). I could
not construct a path in B1 by which a successor acts on unverified content: the
brief written for the successor names the nonce, the digest, the relative state
path and the `context verify` command, and the brief itself is digest-pinned by
the existing check (`stage.go:189`). The residual gaps are the untested
containment rule in F-2 and the wrong headline in F-9.

**3. Writes outside the state root.** B1 writes exactly one file,
`BriefPath(repoRoot, nonce)`, and only after the nonce grammar has been
validated. Reads are confined three ways: the state must equal the canonical
`HandoffDir(canonical(root), nonce)/state.json` after full symlink resolution
(`handoff_state.go:376-386`), every reference open path must resolve inside the
nonce directory (`:217-227`), and `dispatch.ReadVerifiedReference` re-checks
containment in the root. A symlinked ancestor is refused, and that rule does
have a passing test (`handoff_state_test.go:301-315`; mutation M9 turns it
red). Nothing enumerates or removes directories, so there is no way to reach
the private diagnostic root of 8c.11 (`internal/steward/context.go:135-146`),
including when TMPDIR sits inside the installation. B1 does assume
`repoRoot == state root` for the handoff tree; that holds today because
`goal.ResolveStateRoot` never moves state above the installation and because
the existing role and permissions digest lookups already require `repoRoot` to
be the installation. Unit C must build its state path through `HandoffDir` and
not by hand, or a symlinked root will mismatch.

**4. Half-recorded handoffs.** B1 cannot half-record one: it mints nothing and
publishes no state. Its only residue is an orphan brief on a late failure
(F-8), which carries no authority because no intent references it. Every
failure returns an error to the caller, so the seat is told. The genuine
half-record risk (nonce directory written, intent not minted) lives in unit C.

**5. Expiry.** Clean. B1 reads no clock: there is no `time.Now` in
`handoff.go`, `handoff_state.go` or either test file, no sleep, and no maximum
age anywhere. `RecordedAt` is persisted and required non-zero, independent of
mtimes, and is pinned to `writtenAt`, so a later expiry rule computed from
either lands on the same instant. Nothing assumes an answer in either
direction, and the no-limit assertion is correctly absent from B1, since the
brief reserves it for `TestHandoffDefaultHasNoExpiry` in unit C's file.

**6. Pre-change behaviour and assertion quality.** The named tests cannot fail
on the pre-change tree for a behavioural reason: they call `StageHandoffIntent`,
`seatHandoffReason`, `writeStagedHandoffFixture` and the whole state schema,
none of which exist at 449ec649, so the package simply does not compile. That
is normal for new-API tests, which is why I substituted mutation testing. The
result is the two material findings: nine mutations, seven survive. No
assertion depends on wall-clock timing; every fixture time is a fixed
`time.Date(2026, 9, 14, ...)`, there are no sleeps and no `t.Parallel`. One
assertion is near-tautological but defensible: `live source changes are outside
the immutable capture` (`stage_test.go:186-192`) mutates a file the verifier
never opens, but it does distinguish the shipped design from an implementation
that re-verified the copy against its live source, so it earns its place.

**7. Scope.** Clean. `git status --porcelain --ignored=matching` shows exactly
the three modified tracked files and the three new untracked Go files; the only
ignored trees are `artifacts/` and `bin/`, and `artifacts/agents/context` does
not exist, so no stray handoff state was left behind. No `testing.json`, no
coverage floor, no role or docs text, no fixture script, no receipt row, and
nothing from B2, C or D: `verdict.go`, `revive.go` and `turnverdict.go` are
untouched, there is no `ActHold`, no `decideForHandoff`, no `handoffExpired`,
no `Handoff`, no `PruneContext` and no CLI verb. The public `VerifyHandoffState`
is deferred to C, which the brief's C section supports; what B1 ships is a
complete private verifier, not a stub, and it has no dependency on an unlanded
unit. Every new exported symbol is currently unused outside `handoff*.go` and
`stage*.go`, which is the expected shape of a first-landed split unit.

## Verdict

**Not fit to land as it stands. Two material findings, both about proof rather
than behaviour.** The implementation is, as far as I could break it, correct:
every rule I removed by mutation is a rule the shipped code enforces, and the
probes confirm the digest catches a valid-JSON edit, references cannot point
outside their own nonce directory, and a symlinked ancestor cannot redirect the
state read. What is missing is the evidence.

To land, the following must change:

1. **F-1.** Add a drift case to `TestVerifyStagedDigestsRefusesADriftedStateFile`
   that edits a field of `state.json` and leaves it valid JSON (rewriting
   `nextStep.text` is enough), and assert the refusal names `digest mismatch`,
   not only the wrapper sentence. Without it, CCB-3-07's obligation is not
   proved by its named test.
2. **F-2.** Add negative subtests to `TestHandoffStateVerifierIsStrictAndBounded`
   for the binding equality rules: a `writtenAt` that differs from
   `RecordedAt`; a `seat.identity` that differs from `binding.Predecessor`; a
   `heldGoal.id` that differs from the intent's goal; a `normalizedSession`
   that is not the normalization of the recorded session; a changed
   `disposable` sentence; and a binding whose `statePath` is a clean, canonical
   absolute path that is not this nonce's owned state file. Each must refuse.
   The last one is the containment rule and should not ship untested.

Both are additions to `internal/steward/handoff_state_test.go` and
`stage_test.go`; no production file needs to change for either.

Landing conditions that are not findings against the code, but which the seat
still owns: the fast gate has never run on this change (the builder's run died
on a staticcheck download), and the linux coverage floor of 78.5 sits one point
below the 79.5 percent I measured on darwin, so the ratchet should be confirmed
on the seat before landing. F-3 and F-4 are worth carrying into unit C's review
as bounds the manifest does not yet have.
