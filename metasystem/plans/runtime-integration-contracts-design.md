# Runtime integration contracts (agnosticism phase B)

- Status: DRAFT r1 — extracted from the agnosticism ruling set's contested sections (D74 split); awaiting critique r1 of THIS loop (fresh budget)
- Goal: runtime-integration-contracts
- Next step: Fold the critique verdict when run ric-critique-r1 concludes; implement only after convergence.
- In flight right now: run ric-critique-r1 (codex xhigh critique; watch it with: bin/metasystem run watch --id ric-critique-r1 --root .)

Scope (D74): the three subdomains the six-round agnosticism loop left
structurally contested, moved here WHOLE with their critique history
(plans/agnosticism-critique-r1..r6; the r6 findings 1-10 are this
loop's opening worklist). Phase A shipped the registry, the capability
tables, and every converged consumer (D75, 91ff675); this design
completes the story so that adopt.sh, supervision-hook.sh, and
validate-metasystem.sh lose their open-coded runtime arms.

## Contract 1 — adoption/registration/installation

The canonical row is a TAGGED UNION carried by the registry and served
through `runtime registration <name>`:

- Every row: {id (stable artifact role: enforcement-config,
  skill-tree, skill-profiles, config-filter, ...), requiredness (see
  below), destination}.
- Operations and payloads:
  - tree {source, mode: user-selectable link|copy} — bound to the
    EXISTING per-skill grain fixtures: adoption expands skills/* and
    links or copies each child separately with exact symlink targets
    (adopt.sh:293,302; adopt-fixtures.sh:237,280 arbitrate; r6-5)
  - copy-file {source}
  - optional-profile-copy {source}
  - json-strip-key {source, key} — claude's enforcement install
  - skill-profiles {source pattern, delivery copy|in-place,
    destination pattern} — claude <skill>.md, devin <skill>/AGENT.md,
    codex in-place agents/openai.yaml
- Requiredness is ONE contextual enum (r6-2): required |
  template-required (mandatory in the template tree,
  validate-metasystem.sh:525) | adopted-optional (required only when
  the source exists, :580) | optional. The flat required|optional pair
  and the skill-profiles-local variant are gone.
- (runtime, artifactRole) is unique; a row reference by role (e.g.
  shippedEnforcementConfig) must resolve to a row of the right
  operation or the declaration test fails; the filename form of
  shippedEnforcementConfig is REMOVED once rows land (r6-2).
- Installation plans by EXPANDED CONCRETE DESTINATION (r6-3):
  preserve every (runtime, id) alias, merge requiredness to the
  STRICTER state on compatible overlaps, execute each compatible
  output once, refuse incompatible overlaps. Codex+devin's shared
  .agents/skills is tested together in both link and copy modes.
- Collision roots live in the registry as per-runtime CONTRIBUTED
  roots, deduplicated and scanned as the FULL population regardless
  of selection (r6-4). Current declarations reproduce exactly
  {.claude, .devin, .agents}; codex deliberately contributes no
  .codex — adding it is a human-adjudicated security change.
- The installer escape is a TYPED INSTALLER TABLE owned by a new
  `internal/install` seam package (r6-6: named owner — not the probes
  table, not adapter scripts): typed request {rowId, source,
  destination, mode, phase} and result {installed, note}; duplicate
  (runtime, operation) rejection; read-only lookup/list views joined
  by the conformance test; ONE generic shell invocation
  (`metasystem install row --runtime R --row ID ...`). New handlers
  are registry-plus-seam edits; the doctrine never enumerates uses.
- Wire framing (r6-10): ONE versioned encoding. Line 1 is exactly
  `registration/v1`; each row is one line of tab-separated fields in
  pinned order [id, operation, requiredness, destination,
  payload fields in the operation's declared order, "-" for unused];
  zero rows = header only; trailing newline; the declaration grammar
  forbids tabs/newlines/CR in every field.
- Adoption recognition order (r6-10): syntax and runtime-list
  validity refuse FIRST (adopt.sh:57,72); then healthy same-SHA
  exits 0 as a no-op without comparing runtime/copy/skill options
  (adopt.sh:117); incomplete same-SHA refuses (:125); optional-skill
  validation stays after recognition (:213). adopt.sh builds the
  source-fresh binary before any pre-mutation registry query.
- Derived views: `runtime dirs`, config directory-presence
  validation, installed-enforcement paths, and byte-drift validation
  all walk the rows. Drift validation is PER OPERATION (r6-9): exact
  bytes for copied skills/profiles and exact symlink targets; the
  live hook config keeps its structural subset check (hooks.go:36) —
  exact live-config equality is NOT ratified.

## Contract 2 — fixture authorization

- The predicate is exactly today's reserved-cap rule: the checkout's
  conf declares `metasystem.runtimes=fake` (reservedcap.go:38), read
  from the ROOT.
- Transport (r6-1): `internal/identity` stays config-free. A new leaf
  `internal/fixtureauth` owns the root-checked construction
  (`FixtureAuthorization{root, allowed}` from the conf) and every
  fixture-capable entry point takes it explicitly: census verbs,
  lease classification, custodian verdicts, census.Alive consumers
  (verifyarmed.go:22, watchdog.go:108, contract.go:1370), and the
  missionrunner proc helpers. CLI verbs that can read fixtures
  RECONSTRUCT the authorization from their canonical --root; a verb
  with no root (census.go:81 proc alive) gains --root, and
  arm-supervision.sh passes it from line 125 on — the arming check
  happens BEFORE announcement, lease authorization, or any mutation.
- The central reader (fixture.go:25) REFUSES fixture identity without
  a valid authorization value — defense in depth with the entry-point
  checks, not the sole gate.
- Outcomes pinned by tests: fake-checkout bootstrap positive; census
  authentication, census supervision liveness, lease classification,
  and custodian verdicts each refuse fixture identity in a non-fake
  checkout; unreadable conf = no authorization (fail closed).

## Contract 3 — enforcement-map transport

- `runtime enforcement-map <name>` emits the registry's
  expectedEnvelopeEnforcement as canonical JSON (sorted keys, one
  line). Each adapter gains a side-effect-free declaration verb
  (reused by probe production) emitting the SAME shape; the suite
  compares the two generically for every adapter-declaring runtime
  (r6-8) — replacing validate-metasystem.sh:488's source greps.

## Also carried from the split

- The conf-template tailoring materialization
  (role.default.model.<runtime> rows generated for selected
  non-synthesized runtimes) — lands with Contract 1 since adoption
  and tailoring interlock.
- The mechanical no-exhaustive-universe docs assertion — lands as an
  audit check once `runtime registration` exists to derive claims
  from.

## Blast radius

internal/runtimes (rows + collision roots + enforcement-map),
internal/install (NEW), internal/fixtureauth (NEW), internal/identity
(reader refusal), internal/census, internal/lease, internal/contract,
internal/supervise (authorization threading), cmd/metasystem (runtime
registration/enforcement-map verbs, install verb, --root on proc
alive), scripts/adopt.sh (generic row loop), supervision-hook.sh,
validate-metasystem.sh (row-driven validation, generic enforcement
compare), arm-supervision.sh, adopt-fixtures.sh (codex+devin legs,
recognition-order legs), conftailor (materialization).

## Loop discipline

Critique rounds with codex at xhigh; fresh two-budget allowance; the
stop is zero unrefuted material findings, the fixtures-as-arbiter exit
(falling trajectory + all-mechanical), or the budget boundary with its
recorded options. Implementation only after convergence, then the
MANDATORY post-gate code critique as in phases before it.
