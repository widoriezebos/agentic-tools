# Runtime integration contracts (agnosticism phase B)

- Status: DRAFT r3 — critique r2 folded (15 findings; 5 r1 folds verified resolved); awaiting critique r3
- Goal: runtime-integration-contracts
- Next step: Fold the critique verdict when run ric-critique-r3 concludes; implement only after convergence.
- In flight right now: run ric-critique-r3 (codex xhigh critique; watch it with: bin/metasystem run watch --id ric-critique-r3 --root .)

Scope (D74): the three subdomains the six-round agnosticism loop left
structurally contested, moved here WHOLE with their critique history
(plans/agnosticism-critique-r1..r6 seed it; plans/ric-critique-r1.md
is this loop's own first round). Phase A shipped the registry, the
capability tables, and every converged consumer (D75, 91ff675); this
design completes the story so adopt.sh, supervision-hook.sh, and
validate-metasystem.sh lose their open-coded runtime arms.

Revision history: r1 extracted the split text; critique r1 found 14
material defects — the requiredness enum could not carry template AND
adopted contexts at once, the installer escape still extended a
closed union, three fixture-authority sources and the lease-takeover
consequence were unenumerated, the fixtureauth direction violated the
layering, the enforcement compare was impossible for fake, the
validator populations had no derivation contract, instruction
collision coverage had two holes, supervision-hook had no contract,
the recognition order contradicted the build requirement, the drift
rules overreached the fixtures, docs carried only the weak assertion,
and proc alive's callers were unenumerated. r2's fold left fifteen:
the fixture matrix missed mission-runner signal authority and purpose
scope, the bootstrap could not keep both the early refusal and the
no-op, the installer arm had no lifecycle contract or seam sanction,
collision roots had no transport or exception schema, --with-adapter
selected fake into checks it cannot pass, overlap compatibility
ignored policy, no policy preserved live-hook behavior, the hook
script's precedence was impossible, adopt.sh's default/help stayed
core-owned, tree hardcoded its depth, and the pattern/invocation/
adapter-verb/build bytes were unpinned (critique r2). r3 is the fold.

## Contract 1 — adoption/registration/installation

The canonical row is a TAGGED UNION carried by the registry and served
through `runtime registration <name>`:

- Every row: {id (stable artifact role: enforcement-config,
  skill-tree, skill-profiles, config-filter, ...), requiredness,
  destination (ONE expression, scalar or patterned — critique r1-1
  removed the second destination field), validation policy (below)}.
- Requiredness is a CONTEXT-INDEXED PRODUCT (critique r1-1):
  {templateSource: required|optional, adoptedDestination:
  required|source-conditioned|optional} — the same profile row is
  template-required AND adopted-source-conditioned at once
  (validate-metasystem.sh:531 vs :594,600,614). Overlap merging joins
  componentwise to the stricter member.
- Operations and payloads:
  - tree {source, mode: user-selectable link|copy} — per-skill grain:
    installation expands the source's children AFTER optional-skill
    materialization (post---enable staged tree) and links/copies each
    child separately. Link targets are COMPUTED as the clean relative
    path from each expanded destination's parent to its expanded
    source child (critique r2-10); the current rows must produce
    exactly `../../skills/<name>` (adopt.sh:293,302) and that literal
    is a current-row FIXTURE, not the algorithm
  - copy-file {source}
  - json-strip-key {source, key}
  - skill-profiles {source pattern, delivery copy|in-place} — claude
    <skill>.md, devin <skill>/AGENT.md, codex in-place
    agents/openai.yaml; the row's one destination expression carries
    the pattern. Pattern grammar (critique r2-11): the single
    placeholder `{skill}`, substituted per staged skill directory
    (post---enable), results sorted lexicographically, every
    substituted path validated clean-relative, zero matches = zero
    rows (valid); `mode` bytes are exactly `link` | `copy` |
    `in-place`
  - installer {handlerId, source, destination} — the PERMANENT escape
    arm (critique r1-2): a stable handler identifier resolved in the
    `internal/install` typed table; new install behaviors add a
    registry row and a seam handler and NEVER a new central tag. The
    table's contract is a LIFECYCLE (critique r2-3): each handler
    implements Prepare (pre-mutation, read-only, containment-checked),
    Apply (mutate, stage-and-rename), and Validate (the row's drift
    judgment), and declares collision metadata (whether its output is
    instruction-bearing, feeding the collision-root proof).
    Per-runtime handler files SELF-REGISTER seam-locally (no central
    wire-up); the registry/table join is both-ways in conformance;
    `internal/install` joins the sanctioned seam list in
    docs/architecture.md. New validation or collision SEMANTICS (a
    policy the lifecycle cannot express) are a human-reserved
    contract amendment. If a novel behavior needs payload shapes this
    arm cannot carry, that too is a declared exception, not a silent
    union edit.
- Validation policy per row (critique r1-12), declared or derived
  from the operation, one of: exact-bytes (copied skills/profiles),
  transformed-canonical-bytes (json-strip-key output), non-dangling
  link (tree link mode — exact-target enforcement is a HARDENING NOT
  TAKEN here; today's suite accepts any non-dangling link,
  validate-metasystem.sh:568,573; critique r1-11),
  structural-hook-subset (TEMPLATE-ONLY, as today:
  validate-metasystem.sh:1387 checks live hooks only in the template
  tree, and codex/devin declare no self-check marker),
  presence-only (adopted enforcement destinations — today's adopted
  validation does NOT drift-check live hook files, critique r2-7;
  any strengthening is a separately ratified hardening), and
  in-place-source (codex profiles). Declaration validation rejects
  unsupported operation/policy combinations.
- (runtime, artifactRole) is unique; a row reference by role must
  resolve to a row of the right operation; the filename form of
  shippedEnforcementConfig is REMOVED once rows land.
- Installation plans by EXPANDED CONCRETE DESTINATION: preserve every
  (runtime, id) alias, join requiredness componentwise to the
  stricter state on compatible overlaps — compatibility requires
  identical operation, source, payload, mode, VALIDATION POLICY, and
  (for installer rows) handler id (critique r2-6) — execute each
  compatible output once, refuse incompatible overlaps. The
  requiredness orders are pinned: required > source-conditioned >
  optional (adoptedDestination) and required > optional
  (templateSource); source-conditioned evaluates against the
  post---enable STAGED source. Codex+devin's shared .agents/skills is
  tested together in both link and copy modes.
- Collision roots are per-runtime CONTRIBUTED declarations,
  deduplicated, scanned as the FULL population regardless of
  selection; current declarations reproduce exactly {.claude, .devin,
  .agents}; codex contributes no .codex (adding it is a
  human-adjudicated change). Shell transport (critique r2-4):
  `runtime collision-roots` emits the sorted deduplicated full
  population, one per line, trailing newline; adopt.sh's scan
  consumes it. Declaration validation PROVES every
  instruction-bearing expanded destination lies beneath a contributed
  collision root; the exception schema is an explicit per-row
  `uncoveredDestinationException` marker whose ONLY current instance
  is `.codex/hooks.json` (today's accepted uncovered write,
  adopt.sh:335); any OTHER uncovered instruction-bearing destination
  refuses at declaration-test time, and adding an exception or a
  root is a human-reserved security-contract change.
- The registry's InstructionFiles view feeds adoption's collision
  detection (adopt.sh:129) AND payload inclusion (adopt.sh:158,166) —
  the two Class-8 consumers phase A could not reach without rows; ONE
  fixture proves a new instruction filename lands in all five
  consumers (audit inventory, scan roots, conformance protection,
  collision detection, payload inclusion).
- Wire framing registration/v1 (critique r1-3): line 1 exactly
  `registration/v1`. One GLOBAL column list, pinned:
  [id, operation, templateSource, adoptedDestination, destination,
  policy, source, mode, key, handlerId] — tab-separated, every row
  carries all ten columns, unused columns hold `-`, per-tag occupancy
  is declared in the registry and validated (tree fills source+mode;
  copy-file fills source; json-strip-key fills source+key;
  skill-profiles fills source+mode; installer fills
  source+handlerId). Zero rows = header only; trailing newline;
  the grammar forbids tabs/newlines/CR in every field.
- The installer invocation is pinned (critique r1-3, r2-12):
  `metasystem install row --root R --staged S --runtime RT --row ID
  --phase prepare|apply [--mode link|copy]`. --root is the canonical
  TARGET root; --staged is the staged-SOURCE root (adoption's staging
  dir, adopt.sh:139); the row's own source/destination expressions
  bind against those two roots with containment checks (source under
  staged, destination under target). Handlers run TWICE: every
  selected row's prepare phase completes (read-only; stdout
  `ready <destination>`) before ANY apply runs — mirroring today's
  collision-then-mutate order (adopt.sh:227,247). apply stdout:
  `installed|unchanged <destination>` (idempotent by content
  compare). Exit codes: 0 ok, 1 handler refusal, 2 usage, 3 unknown
  (runtime, row, or handler). NO ROLLBACK: an apply failure stops the
  remainder loudly and leaves earlier rows installed — today's set -e
  abort behavior, stated. The `internal/install` table rejects
  duplicate handler ids and exposes lookup/list views joined by the
  conformance test.
- Bootstrap and recognition order (critique r1-10, r2-2) — the
  no-compilation no-op is RATIFIED as the priority: (1) argument
  SYNTAX refusals stay shell-owned and first (the `none`
  exclusivity, duplicate detection, and name-shape check,
  adopt.sh:57,72 — a syntactically valid but UNKNOWN runtime name is
  NOT refused here); (2) toolchain/preflight; (3) source provenance
  (adopt.sh:102); (4) healthy same-SHA recognition exits 0 as a
  no-op BEFORE any compilation and therefore WITHOUT registry
  validation of the requested names — the no-op path's semantics are
  "the installation is healthy at this SHA", not "the request is
  valid", and this is the design's explicit, tested choice
  (critique r2-2: both properties cannot coexist with a compiled
  query); incomplete same-SHA refuses (adopt.sh:117,125);
  (5) on the MUTATION path only: registry recognition via a
  SOURCE-FRESH, NON-OVERWRITING staged query binary — unknown
  runtimes refuse here, before any target write; (6) optional-skill
  materialization and validation — after recognition, BEFORE any
  target mutation (adopt.sh:213,247; critique r2-2); (7) install
  (prepare all, then apply). The staged query build's bytes
  (critique r2-14): run from the source root, `CGO_ENABLED=0 go
  build -buildvcs=false -o "$(mktemp)" ./cmd/metasystem`, no stamp
  (query-only), cleanup trap, build failure maps to adoption's
  toolchain refusal; renaming or copying the staged binary over
  bin/metasystem is FORBIDDEN (the macOS in-place SIGKILL class,
  91ff675).
- Derived views: `runtime dirs`, config directory-presence
  validation, installed-enforcement paths, and drift validation all
  walk the rows, applying each row's validation policy.

## Contract 2 — fixture authorization

- The predicate is exactly today's reserved-cap rule: the checkout's
  conf declares `metasystem.runtimes=fake` (reservedcap.go:38), read
  from the ROOT. Unreadable conf = no authorization; fail closed.
- Layering (critique r1-5): `internal/fixtureauth` sits ABOVE
  `internal/identity` and OWNS fixture parsing and root/config
  authorization. `internal/identity` keeps zero metasystem imports;
  its fixture-capable reads accept an injected neutral probe
  (a small interface defined IN identity, implemented only by
  fixtureauth) and REFUSE fixture identity when the probe is absent.
  The probe's only constructor is fixtureauth's root-checked one;
  the raw fixture reader becomes unexported. (In-process forgery by
  hostile Go code is out of scope — the boundary kills ACCIDENTAL
  and environmental bypass, and the doctrine says so.)
- The COMPLETE fixture-source enumeration (critique r1-4, r2-1),
  each unified behind the authorization with its PURPOSE-SPECIFIC
  probe method — root authorization is NECESSARY but never
  SUFFICIENT: every current guard (allowFake flags, runtime-name
  checks, kernel-death-first fallback order) stays layered on top:
  - the identity fixture reader and its census/lease/custodian
    consumers (census verbs, lease classification incl. CLAIM and
    TAKEOVER whose Live check consumes fixture-backed liveness,
    claim.go:73; custodian verdicts; census.Alive in verifyarmed,
    watchdog, contract preflight)
  - dispatch.ValidateMission's direct fixture read
    (dispatch/mission.go:97,115)
  - the mission-process identity file
    (METASYSTEM_MISSION_PROCESS_IDENTITY_FILE, contract.go:1375,1387)
    — consulted only AFTER unreadable kernel argv, as today
    (contract.go:1378); the probe preserves that fallback order
  - the synthetic ancestor (METASYSTEM_FAKE_AGENT_ANCESTOR_PID,
    ancestor_production.go:54) — keeps its additional
    runtime=="fake" guard (:55)
  - mission-runner fixture COMMAND lookup and GROUP-OWNERSHIP proof
    (proc.go:32,76) — the latter authorizes REAL SIGNALS
    (host.go:91); the runner's fixture WRITES (proc.go:102) are
    fixture-mode-only publications under the same authorization
  - fixture-backed custody in drain decisions (drain.go:262) and
    usage-derivation custody proofs (fence.go:746)
  The probe interface exposes purpose-named methods (identity,
  command, groupOwnership, ancestor, missionProcess) so a consumer
  cannot accidentally widen scope; internal/missionrunner and
  internal/mission join the blast radius.
- CLI transport: fixture-capable verbs reconstruct the authorization
  from their canonical --root. `proc alive` gains --root; EVERY
  caller migrates in the same change (critique r1-14):
  arm-supervision.sh:125,359, fingerprint-harness.sh:103,
  supervision-fixtures.sh:23 — each passes its checkout root; the
  interface-change verification runs those harnesses.
- Tests: fake-checkout bootstrap positive; census authentication,
  census supervision liveness, lease classification, custodian
  verdicts, mission validation, mission-process fallback, synthetic
  ancestor, and LEASE TAKEOVER each refuse fixture identity in a
  non-fake checkout; unreadable-conf refusal.

## Contract 3 — enforcement-map transport

- `runtime enforcement-map <name>` emits expectedEnvelopeEnforcement
  as canonical JSON (sorted keys, one line) for runtimes DECLARING a
  static map; a runtime without one (fake — profile-driven,
  runtimes.go:79) exits 1 with empty stdout, the pinned absent
  semantics (critique r1-6). The adapter side is pinned (critique
  r2-13): `<adapter>.sh enforcement-map`, no arguments, exit 0 plus
  ONE canonical-JSON line for static-map adapters, exit 2 for usage;
  fake is not required to implement it. The suite DECODES AND
  CANONICALIZES both sides before comparing, for every static-map
  runtime — replacing validate-metasystem.sh:488's source greps.
  Fake keeps its profile-driven behavioral test untouched.

## Contract 4 — validator populations and the shell control plane

- Purpose-filtered registry views replace every hardcoded shell
  population (critique r1-7): `runtime list --with-adapter`,
  `runtime list --with-host`, `runtime collision-roots`, plus
  row-derived asset lists. The explicit replacements: required
  enforcement/filter assets (validate-metasystem.sh:306,379),
  per-host syntax checks (:433), host contract rows (:511). The
  adapter CONTRACT rows (:479) do NOT use --with-adapter blindly
  (critique r2-5: fake declares an adapter but deliberately shares
  none of the common-initializer source shape, fake.sh:30): the
  source-shape assertions iterate the static-enforcement-map
  population (claude, codex, devin — a principled filter that
  excludes fake), and the remaining universal checks become decoded
  snapshot identity/schema assertions every adapter, fake included,
  can satisfy.
- supervision-hook.sh contract (critique r1-9, r2-8) — the
  UNAVOIDABLE precedence, pinned: (1) event-name syntax refusal
  (exit 2); (2) engine resolution — a MISSING engine stays benign
  exit 0, as today (supervision-hook.sh:26; membership cannot be
  known without the binary, so this is preservation, not a gap);
  (3) with the engine present, registry membership — unknown runtime
  exits 2; (4) the known runtime's OPTIONAL session environment via
  `runtime session-env` (absent capability → fallback; the
  exit-1-empty-stdout semantics are unambiguous here because
  membership was already proven); (5) cwd resolution: payload cwd,
  then the declared variable's nonempty value via indirect expansion
  of the VALIDATED name, then PWD. A future-runtime fixture proves a
  new declaration needs no script edit.

- adopt.sh's own surface (critique r2-9): an OMITTED --runtimes
  resolves via `runtime adoption-default`; an explicit value
  validates each name against `runtime list --adoptable` (after the
  shell-owned syntax step, on the mutation path); the usage text
  reads `--runtimes <comma-separated names>|none` with a registry
  pointer and NO runtime layout prose (the layouts move to the
  Class-14 docs rewrite).

## Contract 5 — carried classes

- Conf-template tailoring materialization
  (role.default.model.<runtime> for selected non-synthesized
  runtimes) — lands with Contract 1 (adoption and tailoring
  interlock).
- The sanctioned seam list in docs/architecture.md adds
  registry-addressed root instruction files (CLAUDE.md, AGENTS.md —
  the split ruling sanctioned them; critique r2-15) and
  `internal/install`.
- Class 14 IN FULL (critique r1-13): docs/project-adaptation.md's
  runtime-specific installation layouts (:10,:14) become `runtime
  registration` pointers plus ONE generic manual-repair procedure;
  the orchestration mechanics table, glossary list, and README
  adoption command are audited as the named violating locations;
  runtime-specific operational detail moves to seam-owned help or
  declared assets; the audit asserts the named locations carry
  registry-derived text, not just the absence of universe-claiming
  phrases.

## Blast radius

internal/runtimes (rows + collision roots + enforcement-map),
internal/install (NEW), internal/fixtureauth (NEW), internal/identity
(probe seam + unexported reader), internal/census, internal/lease,
internal/contract, internal/dispatch (mission validation),
internal/supervise, cmd/metasystem (registration/enforcement-map/
install verbs, --root on proc alive), scripts/adopt.sh (generic row
loop, staged query binary), supervision-hook.sh,
validate-metasystem.sh, arm-supervision.sh, fingerprint-harness.sh,
supervision-fixtures.sh, adopt-fixtures.sh (codex+devin legs,
recognition-order legs, child-grain and target fixtures),
conftailor (materialization), docs (Class 14 rewrite).

## Loop discipline

Critique rounds with codex at xhigh; fresh two-budget allowance; the
stop is zero unrefuted material findings, the fixtures-as-arbiter exit
(falling trajectory + all-mechanical), or the budget boundary with its
recorded options. Implementation only after convergence, then the
MANDATORY post-gate code critique.
