# Runtime integration contracts (agnosticism phase B)

- Status: DRAFT r6 — critique r5 folded (9 findings); awaiting critique r6, the FINAL budgeted round
- Goal: runtime-integration-contracts
- Next step: Fold the critique verdict when run ric-critique-r6 concludes; on non-convergence the recorded boundary options fire (split the adoption-plan mechanics, or escalate).
- In flight right now: run ric-critique-r6 (codex xhigh critique; watch it with: bin/metasystem run watch --id ric-critique-r6 --root .)

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
    path from each expanded destination's parent to the LIVE
    target-root source child `R/source/child` — the payload copy
    already installed under the target root, never the staged read
    source `S/source/child`, which is deleted after adoption
    (critique r3-6; the plan orders the payload copy before link
    creation, matching adopt.sh:293-299). The current rows must
    produce exactly `../../skills/<name>` (adopt.sh:293,302) and
    that literal is a current-row FIXTURE, not the algorithm
  - copy-file {source}
  - json-strip-key {source, key}
  - skill-profiles {source pattern, delivery copy|in-place} — claude
    <skill>.md, devin <skill>/AGENT.md, codex in-place
    agents/openai.yaml; the row's one destination expression carries
    the pattern. Pattern grammar (critique r2-11, r3-10): enumerate the staged
    skill DIRECTORIES first (post---enable), substitute `{skill}`
    exactly once into source and destination per skill, sorted
    lexicographically, every substituted path validated
    clean-relative — THEN apply the row's source requiredness per
    skill (a template-required profile whose source file is missing
    FAILS; zero rows are valid only when there are no staged skills
    or the source is template-optional); `mode` bytes are exactly
    `link` | `copy` | `in-place`. `in-place` expands to a
    ReferenceRequirement, never a ConcreteOutput (critiques r4-3,
    r5-5: codex's agents/openai.yaml ships WITH the payload,
    adopt.sh:247, and codex's arm copies nothing, adopt.sh:334): it
    resolves to exactly one planned core destination, participates
    ONLY in requiredness and validation, and is EXCLUDED from
    output-overlap compatibility and every collision field and
    proof — a reference and its core output are one writer by
    construction, not a refused overlap
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
  presence-only (adopted enforcement destinations — today's adopted
  validation does NOT drift-check live hook files, critique r2-7;
  any strengthening is a separately ratified hardening), and
  in-place-source (codex profiles). Template live-hook verification
  is NOT a row policy (critique r4-4: one scalar policy cannot be
  structural-subset in the template and presence-only adopted, and
  the template check reads the PARENT repo's live settings through
  explicit paths, validate-metasystem.sh:1387): it stays owned by
  LiveSelfCheck + `hooks check`; the enforcement row contributes
  only its source reference and its ADOPTED policy. Declaration
  validation rejects unsupported operation/policy combinations.
- (runtime, artifactRole) is unique; a row reference by role must
  resolve to a row of the right operation; the filename form of
  shippedEnforcementConfig is REMOVED once rows land — and its ONE
  remaining consumer, the template live-hook check that resolves the
  SHIPPED source (validate-metasystem.sh:1387), moves to the
  projection-backed transport `metasystem install row-source
  --runtime RT --row enforcement-config` (one line, the row's
  resolved source path, exits 0/1/2/3 as the other install verbs;
  critique r5-6).
- Installation plans by EXPANDED CONCRETE DESTINATION: preserve every
  (runtime, id) alias, join requiredness componentwise to the
  stricter state on compatible overlaps — compatibility requires
  identical operation, source, payload, mode, VALIDATION POLICY,
  COLLISION CLASS, UNCOVERED-EXCEPTION status, and (for installer
  rows) handler id (critiques r2-6, r4-5 — conflicting security
  metadata refuses) — execute each
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
  consumes it. Every row carries a COLLISION
  CLASS (critique r3-3): instruction-bearing | plain, TOTAL over
  built-in operations (tree and skill-profiles outputs are
  instruction-bearing; copy-file/json-strip-key declare per row;
  installer rows DERIVE it from the handler table's collision
  metadata — one owner via the RESOLVED-REGISTRATION PROJECTION
  (critique r5-4, cycle-safe): `internal/install` imports the raw
  registry rows, joins handler metadata, runs the collision-root
  proof, and encodes registration/v1; `internal/runtimes` never
  imports install, and the CLI's registration/collision verbs
  delegate to the projection).
  Declaration validation PROVES every instruction-bearing expanded
  destination lies beneath a contributed collision root; the
  exception is a BOOLEAN row field `uncoveredException` (wire column,
  `-` reserved strictly for absent optional fields) valid ONLY when
  the expanded destination equals `.codex/hooks.json` (today's
  accepted uncovered write, adopt.sh:335); any other uncovered
  instruction-bearing destination refuses at declaration-test time,
  and adding an exception or a root is a human-reserved
  security-contract change.
- The registry's InstructionFiles view feeds adoption's collision
  detection (adopt.sh:129) AND payload inclusion (adopt.sh:158,166) —
  the two Class-8 consumers phase A could not reach without rows; ONE
  fixture proves a new instruction filename lands in all five
  consumers (audit inventory, scan roots, conformance protection,
  collision detection, payload inclusion).
- Wire framing registration/v1 (critique r1-3): line 1 exactly
  `registration/v1`. One GLOBAL column list, pinned:
  [id, operation, templateSource, adoptedDestination, destination,
  policy, collisionClass, uncoveredException, source, mode, key,
  handlerId] — tab-separated, every row carries all twelve columns,
  unused columns hold `-`, per-tag occupancy
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
  staged, destination under target). The unit of planning is the
  CONCRETE OUTPUT, not the row (critique r3-2), and the PLAN has an
  executable owner (critique r4-2): `internal/install` builds ONE
  immutable plan over the selected runtimes' rows PLUS the explicit
  core outputs — the payload allowlist and the FINAL ENGINE, a
  distinct core output whose bytes the SHELL prepares via a
  refactored `go-build.sh --output PATH --stamp SHA` (critique
  r5-3: the staged archive has no git metadata to derive a stamp
  from, and Go verbs do not invoke scripts) and passes into the
  plan as an immutable prepared source, atomically installed at
  R/bin/metasystem (critique r4-6: the query binary is temporary
  and never ships) — exposed as `metasystem install
  plan|prepare|apply --root R --staged S --runtimes LIST --mode
  link|copy` (the user's skill-tree decision, today's
  --copy-skills, adopt.sh:57; persisted in the completed
  installation state so tree validation judges BY INSTALLED MODE —
  critique r5-9); the per-row invocation is the handler-level tool
  those verbs drive. Handler Validate returns a TYPED
  {clean|drift} result plus a SEPARATE error channel, mapped
  deterministically to exits 0/1/4.
  Before the first target write, install persists an INCOMPLETE
  record at the pinned path `R/.metasystem/adoption-incomplete.json`
  containing {sha, selection, mode, planDigest, the CANONICAL PLAN
  MANIFEST (every planned output), generated values (the goal
  baseline's timestamp, adopt.sh:205 — regenerating would break
  exact-plan identity), per-output status, and PREIMAGES for merge
  outputs (.gitignore/.gitattributes, adopt.sh:256 — user-owned
  content is never blindly deletable)} (critique r5-1). Recognition
  routes an exact planDigest match into RESUME (skip
  installed-and-clean outputs, continue); a mismatch refuses with
  the pinned recovery: restore recorded preimages, remove only the
  outputs the record lists as installed, rerun — never a generic
  deletion. The plan is TOTAL over today's mutations (critique
  r5-2): payload files and MERGE operations, the generated goal
  baseline, the artifacts directory state (adopt.sh:280-291), the
  runtime-neutral CI workflow (adopt.sh:346), the FINAL ENGINE, the
  completion marker, and the git pre-commit hook — whose write can
  land OUTSIDE R in the git common dir (adopt.sh:364): it gets its
  own prepared, collision-checked boundary named in the plan, the
  one sanctioned outside-R output. The COMPLETED adoption marker is
  written atomically LAST — today the SHA marker lands before
  runtime outputs (adopt.sh:291), the exact false-healthy hole this
  closes. prepare runs for EVERY
  planned output before the first write; apply order is pinned
  core-payload-then-runtime-outputs (adopt.sh:247,293). apply
  stdout: `installed|unchanged <destination>` per output (idempotent
  by content compare; each output stage-and-renames individually).
  Exit codes: 0 ok, 1 handler refusal, 2 usage, 3 unknown (runtime,
  row, or handler). NO ROLLBACK, at output grain: a failure stops
  the remainder loudly, leaves earlier OUTPUTS installed, removes
  its own staging residue, and recovery is the RESUME path above. A pinned VALIDATE phase exists too
  (critique r3-11, r4-10): `metasystem install validate --root R
  --runtime RT` — adopted validation resolves BOTH canonical source
  and destination below R (the installed payload IS the source of
  truth; no staged root exists at validation time). One line
  `ok|drift <destination>` per output; exits 0 clean / 1 drift /
  2 usage / 3 unknown declaration-or-handler / 4 operational
  failure (I/O and invariant errors are never drift; on unexpected
  failure the partial `ok` lines stand and the exit code is
  authoritative) — the ONE aggregate validator the suite calls. The `internal/install` table rejects
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
  (critique r2-14, r3-12): `query_dir=$(mktemp -d)`, trap removes
  the DIRECTORY, `CGO_ENABLED=0 go build -buildvcs=false -o
  "$query_dir/metasystem" ./cmd/metasystem` run from the source
  root, the same bound variable execs the queries, no stamp
  (query-only), build failure maps to adoption's toolchain refusal;
  renaming or copying the staged binary over bin/metasystem is
  FORBIDDEN (the macOS in-place SIGKILL class, 91ff675).
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
    (dispatch/mission.go:97,115) — assigned its own
    MissionHolderProbe (command + process-group facts only, the two
    authorities mission-join needs, mission.go:62-124), constructed
    inside ValidateMission from its root (critique r5-8)
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
  The probe is not ONE interface (critique r3-1, r4-1): fixtureauth
  exposes SEPARATE minimal capability values, each constructed only
  by its root-checked factory, each named with its construction and
  recipient call sites —
  - IdentityProbe (identity reads: census verbs, lease classify,
    custodian)
  - CommandProbe (fixture command lookup: proc.go:32, consumed by
    host-start verification, host.go:301)
  - GroupOwnershipGrant (proc.go:76 — the SIGNAL-authorizing
    authority, host.go:91; granted ONLY to the runner's signal path
    and never bundled with command lookup)
  - AncestorProbe (ancestor_production.go:54, keeps the
    runtime=="fake" guard)
  - MissionProcessProbe (contract.go:1375, keeps the
    kernel-argv-first fallback order)
  - PublicationGrant (the runner's fixture WRITES, proc.go:102)
  - ProcessTableProbe (METASYSTEM_CENSUS_PROCESS_FILE selection and
    parsing: census.go:41, supervise_watchercfg.go:45,
    census/run.go:355)
  Tests per authority: fake-positive AND non-fake/unreadable-conf
  refusal for each, including proof an unauthorized group is never
  signalled and an unauthorized fixture is never written.
  internal/missionrunner and internal/mission join the blast radius.
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
  row-derived asset lists. Signature-conformance vectors are
  PROVIDER-OWNED (critique r3-5): each runtime's declaration carries
  its positive process-name vector and a lookalike negative (the
  facts S4-7 hardcodes today, supervision-fixtures.sh:325); the
  fixture iterates `runtime list --with-adapter` consuming declared
  vectors via the pinned transport `runtime signature-vectors
  <runtime>` — one canonical-JSON line {"positive":...,
  "lookalike":...}, trailing newline, exits 0 ok / 1 undeclared /
  2 usage (critique r4-7) — with no shared runtime branch and no
  fake special-case. The explicit replacements: required
  enforcement/filter assets (validate-metasystem.sh:306,379),
  per-host syntax checks (:433), host contract rows (:511). The
  adapter CONTRACT rows (:479) do NOT use --with-adapter blindly
  (critique r2-5), and static-map presence selects ONLY the
  enforcement-map compare (critique r3-4: a standalone adapter can
  declare a static map and a common-lifecycle adapter can be
  profile-driven — the two facts are independent). The registry
  gains an explicit `commonLifecycleAdapter` capability flag
  (claude, codex, devin today; fake deliberately not, fake.sh:30);
  the common-initializer/writer source-shape assertions iterate that
  flag's population via the pinned view `runtime list
  --with-common-lifecycle` (same one-name-per-line framing and
  ordering as the other list filters; fake excluded by declaration;
  filters do not combine — critique r4-8). The remaining universal checks are OFFLINE
  static assertions every adapter can satisfy (critique r3-8: real
  probes need installed providers, validate-metasystem.sh:409):
  each adapter gains a no-auth, no-provider, side-effect-free
  `<adapter>.sh contract` verb whose output is produced by the REAL
  snapshot construction path fed deterministic dummy provider facts
  (critique r4-9 resolved via its second alternative — critique
  r5-7 showed the schema-discriminator route is an unplanned
  live-security cutover invalidating every retained snapshot; NOT
  taken): the suite decodes the constructed snapshot and asserts
  runtime identity and shape. Retained snapshots stay valid; no
  reprobe storm; no schema field lands in production output. The
  decoded-all-adapters assertion rides that verb, not live probes.
- supervision-hook.sh contract (critique r1-9, r2-8) — the
  UNAVOIDABLE precedence, pinned (critique r3-7): (1) shell-owned
  syntax refusals — the event name AND the runtime argument's
  registry-grammar shape `^[a-z][a-z0-9-]{0,31}$` (exit 2; the shape
  check needs no binary); (2) executable resolution — if EITHER the
  engine OR arm-supervision.sh is missing/non-executable, exit 0
  benign, as today (supervision-hook.sh:25-26); (3) with both
  present, registry membership — unknown runtime exits 2; (4) the known runtime's OPTIONAL session environment via
  `runtime session-env` (absent capability → fallback; the
  exit-1-empty-stdout semantics are unambiguous here because
  membership was already proven); (5) cwd resolution: payload cwd,
  then the declared variable's nonempty value via indirect expansion
  of the VALIDATED name, then PWD. A future-runtime fixture proves a
  new declaration needs no script edit.

- adopt.sh's own surface (critique r2-9, r3-9): the shell-owned
  syntax step checks the registry NAME GRAMMAR exactly
  (`^[a-z][a-z0-9-]{0,31}$`), duplicates, and the `none` rules —
  `none` is a shell sentinel that BYPASSES membership and
  adoptability checks entirely (its empty-selection behavior is
  pinned by adopt-fixtures.sh:387). An OMITTED --runtimes defers
  default resolution to the MUTATION path (where the query binary
  exists) via `runtime adoption-default`; explicit non-none names
  validate there against `runtime list --adoptable`. The usage text
  reads `--runtimes <comma-separated names>|none` with a registry
  pointer and NO runtime layout prose.

## Contract 5 — carried classes

- Conf-template tailoring materialization
  (role.default.model.<runtime> for selected non-synthesized
  runtimes) — lands with Contract 1 (adoption and tailoring
  interlock).
- The sanctioned seam list in docs/architecture.md adds
  registry-addressed root instruction files (CLAUDE.md, AGENTS.md —
  the split ruling sanctioned them; critique r2-15) and
  `internal/install`.
- Population TESTS derive from declarations (critique r3-13): the
  runtime-CLI and registry tests that today pin the exact universe
  (runtime_verbs_test.go:32, runtimes_test.go:43-95) are rewritten
  to derive expected populations from runtimes.All() and assert only
  RELATIONAL policies (one adoption default, ascending priorities,
  fake's named guards, claude's self-check) — a valid new
  declaration must not fail shared core tests. This lands WITH the
  rows implementation as its first commit.
- docs/metasystem-reconciliation.md's Phase 0 inventory derives its
  runtime-owned entries from the instruction-file and
  registration/collision views (critique r3-14), keeping fixed
  examples only for foreign, unregistered assets.
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
