# delegate-rounds-reuse-a-warm-gate: design

Goal 13 of plans/delivery-efficiency-plan.md. One critique read of this page,
then three slices under the one goal, each with its own fixtures and one
critic read; the delegate machinery itself stays off for landings until
R-98-m1e's gates are measured, so the slices land by human commit and the
measurement chain of slice C is the goal's own fixture, not a landing lane.

## 1. Where the hours went

Codex delegate jobs spent 26.5 of 48.6 tool hours verifying (codex-sessions.md
section 4b): focused go test 12.4 hours over 1,829 runs, fixture scripts 9.3
hours, go-gate 4.4 hours over 587 runs with a 0.2 second median, a 12 second
p90 and a 25 minute maximum. The slowest runs (section 4b's table) all show
the delegate exporting its own cache: `GOCACHE=/tmp/metasy...`,
`GOCACHE=/private/tmp/hrd-build1-r9...`, `GOCACHE=/tmp/shhc-build1-go-cache`.
The cause is in the adapters: the Codex sandbox cannot write the user's Go
cache and the codex adapter exports none, so every delegate invents a cache
under /tmp per job or per round and compiles the module from nothing; the
claude adapter exports one but keys it by job AND round
(`${TMPDIR:-/tmp}/metasystem-claude/$job-$round`, adapters/claude.sh:109), so
every follow-up round is cold by construction. Verification is also
ungoverned: the delegate decides what to run and re-runs it after fold
rounds that changed no code, because nothing tells it which groups the
engine already holds proof for.

## 2. Slice A: one build cache per chain

- Every round of a chain runs in the chain root's worktree (follow-up rounds
  rebase it, dispatch.sh `follow_up`), and the envelope grants writes only
  inside that worktree plus its derived git roots (internal/dispatch/
  envelope.go: "permission write root escapes the job worktree"). The cache
  therefore lives in the worktree's private git dir, beside the quarantine
  object store that already lives there for the same reasons:
  `$(git -C <worktree> rev-parse --absolute-git-dir)/metasystem-build-cache/`
  with `go-cache/` and `go-tmp/` beneath. It is outside the shippable
  projection (never swept by the conformance snapshot), writable under both
  sandboxes (the derived git-dir root is what lets a delegate commit), shared
  by every round of the chain, private to the chain, and removed with the
  worktree.
- Both adapters export `GOCACHE`, `GOTMPDIR` and `STATICCHECK_CACHE` to it
  before launching the runtime (staticcheck, which the fast gate runs, exits
  1 when it cannot write its own cache; the deep dive's slow gates all set it
  by hand: design critique M1); the claude adapter keeps its per-round
  `TMPDIR` scratch for everything else. The codex adapter, which exported
  nothing, now does the same. A shared-checkout job (no worktree) exports
  nothing, as today.
- Nothing removes delegate worktrees today (41 of them, 2.0 GB, under
  artifacts/agents/worktrees; design critique M2), and a warmed cache is
  0.5 to 1 GB per chain. `dispatch.sh close` removes the chain's cache when it
  closes the chain, and `dispatch.sh reap` removes the cache of a chain whose
  every member is terminal (a later follow-up starts cold once); the
  worktrees themselves stay as they are, a separate and older gap recorded on this
  page. The litter sweep (memory: disk pressure on suite days) learns the
  path `.git/worktrees/*/metasystem-build-cache`.
- The engine-appended "Required testing contract" section every implementer
  brief with a goal carries (internal/dispatch/build.go) tells the delegate
  that the cache is provided and that it must never set, unset or strip
  `GOCACHE`, `GOTMPDIR` or `STATICCHECK_CACHE` (no `env -u`, no `env -i`
  before a gate: the deep dive's slow runs did both); the brief template
  says the same for seat-written briefs.
- Fixture: a dispatch-fixtures scenario on the fake runtime whose adapter
  records the launched environment; round 1 and round 2 of one chain export
  the same `GOCACHE`, under the worktree's git dir, and a second chain
  exports a different one.
- Measurement (DONE clause 3, first half): matched before-and-after on this
  checkout in a fresh worktree, the same commands with a cold cache and then
  the warmed one: `go test ./internal/testpolicy ./internal/landing` and
  `go-gate.sh --fast`. The numbers go in the landing message and section 5.

## 3. Slice B: the round names its smallest proving run (revised after critique)

- The engine already names it. Every implementer brief with a goal carries
  the engine-appended "Required testing contract" section
  (internal/dispatch/build.go, `TestingRequirement`): run the risk-selected
  `metasystem test` plan on the actual candidate and require `test verify`
  before returning. That plan is the smallest proving run, and with
  8f7becd65 and db58ad931 it reuses every group whose identity the previous
  round already proved, so a fold round that changed no code reruns nothing
  and a round that touched one package reruns that package's groups. A
  hand-written `Proving run:` line would contradict this section for bed
  changes and add a role-specific refusal to a role-agnostic brief-mode
  check (design critique M5): there is no such line. The section instead
  says what it is and asks for the attempt id.
- The return carries it as text, not schema: the implementer return schema
  v1 is frozen with additionalProperties false and every producer (the fake
  runtime, NormalizeReturn, Devin) emits the frozen shape; a required field
  would refuse round 2 of every chain whose round 1 predates the landing
  (design critique M3). The attempt id is reported in the return's facts,
  where the attempt record (artifacts/agents/proof-runs/attempts) already
  says per group whether it ran or was reused and from which attempt. The
  round's record is rounds/<n>/return.json; there is no separate round
  record to extend (design critique M4).
- Built by this landing: the testing-requirement text (slice A carries it,
  one sentence). Nothing else to build.

## 4. Slice C: the three-round chain

- A real chain of three rounds on a real runtime (the roster's implementer
  runtime), with a trivial brief whose proving run is the engine run on the
  chain's tree: round 1 warms the cache and proves; round 2 edits one line
  of docs/orchestration.md (no group's inputs: every group reused, nothing
  compiles), round 3 edits one comment in internal/refusal/refusal.go (its
  group, refusal-register-standard, reruns; the rest reuse, and the warm
  cache carries the recompile). The measure: rounds 2 and 3 verify in under
  three minutes, timed from their attempt records (StartedAt to EndedAt of
  the test run) and the round logs, with the reused and rerun groups named
  per round (design critique M6). Recorded on this page, section 5, with
  the chain id.
- The chain runs while nothing else heavy runs on the Mac and never beside a
  cadence attempt (the seats page box).

## 5. Measurements

- Slice A, cold versus warm, this checkout at db58ad931 in a fresh detached
  worktree, one cache directory used twice (2026-09-12, quiet Mac):
  `go-gate.sh --fast` 15 s cold, 3 s warm; `go test -count=1
  ./internal/testpolicy ./internal/landing` 145 s cold, 137 s warm. The
  cache buys compile time, which is the whole fast gate and the test
  binaries, not test execution: against the audit's 587 gate runs with a
  12 s p90 that is the p90 itself. The 25-minute gate maxima in the deep
  dive were self-set caches under load and full gates; a warm cache cannot
  shorten a test that runs 137 s of its own accord, which is what slice B is
  for.
- Slice C: filled in when the three-round chain runs.

## 6. What does not change

- Which runtime runs a delegate, the envelope's rule that writes stay inside
  the worktree, the quarantine object store, the follow-up rebase.
- The engine's testing contract and its reuse rules; this goal adds no
  second skip mechanism.

## 7. Risks

- A cache inside the git dir grows with the chain; it is removed with the
  worktree, and a chain is bounded by its budget. Left as is.
- A delegate that still sets its own `GOCACHE` loses the win; the testing
  requirement forbids it, and the round's attempt record shows the reruns.
- Two folds from the code critique of slice A: the engine's own test
  environment allowlist (cmd/metasystem/test.go) now carries `GOTMPDIR` and
  `STATICCHECK_CACHE`, or the delegate's proving run would launch its
  groups without the staticcheck cache and the fast gate would fail in the
  sandbox (F-1); and the environment digest that enters every group
  identity (proofrun `digestEnvironment`) ignores cache and scratch
  locations (`GOCACHE`, `GOMODCACHE`, `GOTMPDIR`, `STATICCHECK_CACHE`,
  `TMPDIR`, `TMP`, `TEMP`), because a per-chain cache path or a per-round
  scratch would otherwise key every identity to one chain or round and
  defeat the reuse this goal exists for; this is a judge change and
  invalidates retained identities once. The cache qualifies only for job
  worktrees the dispatcher made under artifacts/agents/worktrees (F-2): a
  seat checkout that is itself a linked worktree has a git dir no envelope
  grants.
