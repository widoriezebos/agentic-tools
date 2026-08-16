# Runtime integration contracts (agnosticism phase B)

- Status: DRAFT r2 — critique r1 folded (14 findings: 12 structural, 2 mechanical); awaiting critique r2
- Goal: runtime-integration-contracts
- Next step: Fold the critique verdict when run ric-critique-r2 concludes; implement only after convergence.
- In flight right now: run ric-critique-r2 (codex xhigh critique; watch it with: bin/metasystem run watch --id ric-critique-r2 --root .)

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
and proc alive's callers were unenumerated. r2 is the fold.

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
    child separately, emitting the canonical relative target
    ../../skills/<name> (adopt.sh:293,302; critique r1-11); direct
    fixtures pin child-grain and the emitted target
  - copy-file {source}
  - json-strip-key {source, key}
  - skill-profiles {source pattern, delivery copy|in-place} — claude
    <skill>.md, devin <skill>/AGENT.md, codex in-place
    agents/openai.yaml; the row's one destination expression carries
    the pattern
  - installer {handlerId, source, destination} — the PERMANENT escape
    arm (critique r1-2): a stable handler identifier resolved in the
    `internal/install` typed table; new install behaviors add a
    registry row and a seam handler and NEVER a new central tag. If a
    novel behavior needs payload shapes this arm cannot carry, that
    is a declared doctrine exception, not a silent union edit.
- Validation policy per row (critique r1-12), declared or derived
  from the operation, one of: exact-bytes (copied skills/profiles),
  transformed-canonical-bytes (json-strip-key output), non-dangling
  link (tree link mode — exact-target enforcement is a HARDENING NOT
  TAKEN here; today's suite accepts any non-dangling link,
  validate-metasystem.sh:568,573, and adopting exact targets needs
  separate human ratification; critique r1-11), structural-hook-subset
  (the live hook config, hooks.go:40,58), in-place-source (codex
  profiles). Declaration validation rejects unsupported
  operation/policy combinations.
- (runtime, artifactRole) is unique; a row reference by role must
  resolve to a row of the right operation; the filename form of
  shippedEnforcementConfig is REMOVED once rows land.
- Installation plans by EXPANDED CONCRETE DESTINATION: preserve every
  (runtime, id) alias, join requiredness componentwise to the
  stricter state on compatible overlaps (identical operation, source,
  payload, mode), execute each compatible output once, refuse
  incompatible overlaps. Codex+devin's shared .agents/skills is
  tested together in both link and copy modes.
- Collision roots are per-runtime CONTRIBUTED declarations,
  deduplicated, scanned as the FULL population regardless of
  selection; current declarations reproduce exactly {.claude, .devin,
  .agents}; codex contributes no .codex (adding it is a
  human-adjudicated change). Declaration validation PROVES every
  instruction-bearing expanded destination lies beneath a contributed
  collision root, except explicit human-adjudicated exclusions
  (critique r1-8's forgotten-root hole).
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
- The installer invocation is pinned (critique r1-3):
  `metasystem install row --root R --runtime RT --row ID --phase
  pre-mutation|mutate --source S --destination D [--mode link|copy]`.
  Exit codes: 0 installed or no-op (idempotent by content compare),
  1 handler refusal (nothing mutated — handlers stage-and-rename so
  partial mutation cannot survive an error), 2 usage, 3 unknown
  (runtime, row, or handler). Stdout: one line
  `installed|unchanged <destination>`. The `internal/install` table
  rejects duplicate handler ids and exposes lookup/list views joined
  by the conformance test.
- Bootstrap and recognition order (critique r1-10) — pinned as
  today's sequence, with the registry query inserted WITHOUT moving
  the build: (1) argument syntax refusals (adopt.sh:57,72 — the
  `none` rules and duplicate detection are syntax, not registry);
  (2) toolchain/preflight; (3) source provenance (adopt.sh:102);
  (4) runtime-name recognition against the registry via a
  SOURCE-FRESH, NON-OVERWRITING query — the query binary builds to a
  staging path (`go build -o <tmp>`), never replacing bin/metasystem
  and never passing the go-build gate fence, so a healthy same-SHA
  run still exits 0 as a no-op WITHOUT rebuilding the live binary
  and a live gate cannot block a no-op adoption (go-build.sh:16's
  refusal stays a mutation-path property); (5) healthy same-SHA
  no-op / incomplete same-SHA refusal (adopt.sh:117,125);
  (6) install; (7) optional-skill validation (adopt.sh:213).
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
- The COMPLETE fixture-source enumeration (critique r1-4), each
  either unified behind the authorization or eliminated:
  - the identity fixture reader and its census/lease/custodian
    consumers (census verbs, lease classification incl. CLAIM and
    TAKEOVER whose Live check consumes fixture-backed liveness,
    claim.go:73; custodian verdicts; census.Alive in verifyarmed,
    watchdog, contract preflight)
  - dispatch.ValidateMission's direct fixture read
    (dispatch/mission.go:97,115) — unified: it constructs the
    authorization from its root
  - the mission-process identity file
    (METASYSTEM_MISSION_PROCESS_IDENTITY_FILE, contract.go:1375,1387)
    — unified behind the same probe
  - the synthetic ancestor (METASYSTEM_FAKE_AGENT_ANCESTOR_PID,
    ancestor_production.go:54) — unified: honored only under an
    authorization constructed from the census root
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
  semantics (critique r1-6). Each static-map adapter gains a
  side-effect-free declaration verb emitting the same shape (reused
  by probe production); the suite DECODES AND CANONICALIZES both
  sides before comparing, for every static-map runtime — replacing
  validate-metasystem.sh:488's source greps. Fake keeps its
  profile-driven behavioral test untouched.

## Contract 4 — validator populations and the shell control plane

- Purpose-filtered registry views replace every hardcoded shell
  population (critique r1-7): `runtime list --with-adapter`,
  `runtime list --with-host`, plus row-derived asset lists. The
  explicit replacements: required enforcement/filter assets
  (validate-metasystem.sh:306,379), per-host syntax checks (:433),
  adapter contract rows (:479), host contract rows (:511).
- supervision-hook.sh contract (critique r1-9): membership check via
  `runtime list` (unknown runtime refuses, exit 2 as today); the
  known runtime's OPTIONAL session environment via
  `runtime session-env` (absent capability → skip to fallback, the
  verb's exit-1-empty-stdout absent semantics distinguish it in
  context because membership was already proven); cwd resolution
  order pinned: payload cwd, then the declared variable's nonempty
  value via indirect expansion of the VALIDATED name, then PWD. A
  future-runtime fixture proves a new declaration needs no script
  edit.

## Contract 5 — carried classes

- Conf-template tailoring materialization
  (role.default.model.<runtime> for selected non-synthesized
  runtimes) — lands with Contract 1 (adoption and tailoring
  interlock).
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
