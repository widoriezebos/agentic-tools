# Agnosticism audit: the ruling set

- Status: PHASE A SHIPPED (D75) — both hosts green at 91ff675 after the mandatory code critique's 14 findings were folded; phase B lives in goal runtime-integration-contracts
- Goal: agnosticism-audit CLOSED (backlog item 16; D74 split, D75 ship)
- Next step: none
- In flight right now: none

Phase-A implementation notes (what shipped vs the text below): the
registry, runtime verbs, all Class-2/4/8/9 consumers, the three typed
capability tables with the both-ways conformance test, the usage split
with the fence's declared-recovery dispatch, the probe lifecycle, the
role files rewritten to residual identifiers (fake declares its
network residual for the fixture harness; the unregistered-runtime
fixture now proves fail-closed), dispatch.sh's help line, the four
docs' universe claims, and the architecture doctrine. Deferred WITH
phase B (they interlock with the adoption/registration contract): the
conf-template tailoring materialization, the mechanical
no-exhaustive-universe docs assertion, and every adopt.sh/
supervision-hook.sh/validate-metasystem.sh open-coded arm except the
hooks-check call site.

The human's rule, verbatim intent (2026-08-15): "the meta system must be
agent agnostic (it should work with Codex and Devin and any other future
agent too)." The end state: every runtime-integration surface is DECLARED
by its runtime's seam entry, the core consumes declarations, and adding a
runtime touches only its own seam files plus one registry declaration.

Revision history: r1 swept only cmd/ + internal/ and missed the shipped
script control plane, the `fake` runtime's production branches, and the
role-requirements waiver matrix; its usage and host-resume moves crossed
seam boundaries the wrong way (critique r1, 10 findings). r2's fold left
seven material defects: the waiver move inverted security ownership, the
hook capability bundle dropped codex/devin audit coverage, the registry
had no adoption shape, the fake list was incomplete, instruction
filenames missed four consumers, the shell verb schema could not express
registration, and recovery had no outcome contract (critique r2). The
critiques are preserved beside this plan. r3's fold left nine: the
residual identifiers lacked field binding, the pure-data registry could
not hold behavioral capabilities, the registration rows could not
express installation reality (json-transform, user-selectable
link/copy, directory collisions, build ordering), the `none` sentinel
vanished, hook verification kept hardcoded consumers, recovered could
be unmeasured, the fake list missed the identity-fixture authority
path, more validate-metasystem rows were open-coded, and --devin-checks
sat in shared CLI code (critique r3). r4's fold left seven: deriving
enforcement expectations from waiver residuals was unsound against
codex's real snapshot, the fixture gate was not actually a gate, the
registration contract kept parallel owners and an untyped transform,
the capability table needed per-owner typing, config-identity filters
and skill profiles sat outside every declaration, the probe replaced
only the marker, and shipped docs still enumerate the universe
(critique r4). r5's fold left nine (6 structural / 3 mechanical): the
fixture gate's "armed fixture mode" wording collided with supervision
arming and named no real predicate, the flat row schema could not
carry operation payloads or the three profile projections, duplicate-
destination rejection broke codex+devin's shared tree, collision
roots were unpinned, the installer escape pointed at the probes
table, the config-identity filter had no live-path verb, checkout
configuration itself had no sanctioned class, the enforcement enum
admitted a value the live schema rejects, and framing/idempotency
stayed contradictory (critique r5). r6 was the r5 fold and the FINAL
budgeted round; critique r6 returned REVISE with structural findings
remaining (trajectory 10/7/9/7/9/11, rising at budget exhaustion), so
neither convergence nor the fixtures-as-arbiter exit was available.
Per D74 the goal SPLIT: everything the critic's own fold tables mark
resolved across r2-r6 is phase A and implements under this goal;
the still-contested subdomains are phase B, moved WHOLE to goal
runtime-integration-contracts with critiques r1-r6 as seed material.

## The split (D74)

PHASE A — implement now (converged across rounds; the critic's fold
tables mark each resolved): the `internal/runtimes` pure-data registry
core (names + grammars, tailoring priority, fake-model synthesis,
adoptable/adoptionDefault with the `none` sentinel, session-env names,
instruction files reaching every consumer, the exact
expectedEnvelopeEnforcement enum, permissionResiduals with role-owned
live waivers, shippedEnforcementConfig/liveSelfCheck split); the
purpose-filtered `runtime` verbs EXCEPT `registration` (list,
list --adoptable, adoption-default, dirs*, enforcement-config,
self-check, instruction-file, session-env, config-identity-filter —
*dirs reads the validate.go:344 table until phase B's rows replace its
source); the per-owner typed capability tables (host recollection,
usage recovery with RecoveryOutcome, adapter probes with the typed
lifecycle); Class 5 (delivery recollection), Classes 6+7 (usage seam
split, fence dispatch, preservation tests), Class 4 (hooks --runtime
with explicit paths), Class 8 (audit + conformance consumers), Class 9
(residual identifiers, waivers stay role-owned), Classes 1/11/12/13
(stays + the probe lifecycle), Class 14 (docs claims), Class 15 (conf
seam — including r6-7's refinement: SCHEMA-DEFINED runtime positions,
key segments as well as values, validated against the registry; conf
joins the seam list, sweep, and doctrine), and r6-11's site: Class 3
gains dispatch.sh:8's help line, whose runtime choices derive from
`runtime list`.

PHASE B — goal runtime-integration-contracts (design loop, seeded by
plans/agnosticism-critique-r1..r6; the r6 findings are the opening
worklist): the ENTIRE adoption/registration/installation contract
(tagged-union rows, artifact roles and requiredness semantics,
union/dedup with alias preservation, collision-root declarations,
per-skill tree grain, the typed installer owner, wire framing bytes,
per-operation drift validation, same-SHA recognition order — r6
findings 2,3,4,5,6,9,10); fixture authorization transport and full
consumer coverage (r6-1); the enforcement-map adapter transport verb
(r6-8). Until phase B lands, adopt.sh, supervision-hook.sh, and
validate-metasystem.sh keep their current open-coded arms — phase A
does NOT touch them except where a phase-A declaration already has a
consumer test (instruction files, shipped-hook iteration).

## The sanctioned seams

1. `internal/adapter/` — delegate-side integration, per-runtime files.
2. `internal/host/` — host-side Go integration, per-runtime files.
3. `scripts/agents/hosts/<runtime>.sh` — host-side shell launchers (a
   runtime's host implementation MAY be shell that reuses its adapter —
   codex is exactly this; host conformance checks declared capabilities
   and an executable launcher, never "a Go file per runtime").
4. `scripts/agents/adapters/*.sh`, the runtime-owned JSON assets those
   scripts and declarations address (config-identity filters,
   `scripts/enforcement/<runtime>-*.json`), and per-skill runtime
   profile files under `skills/` and `optional-skills/` (critique
   r4-5: adoption copies them, adopt.sh:314, and the suite validates
   them, validate-metasystem.sh:525 — they are declared assets, in
   the sweep and the seam).
5. `cmd/metasystem` verb wiring for the adapter/host families — thin
   routing whose verb NAMES are the seam's CLI face, and whose bodies
   call seam entry points only (obligation: the two Devin verbs that
   today call `usage.DevinUsage` directly reroute through the seam).
6. `internal/usage`'s per-runtime files (usage/claude.go,
   usage/codex.go, usage/devin.go) — the declared single owner of
   shared usage parsing (critique r3-2 resolved the r3 contradiction:
   these ARE seam entries; usage.go itself stays runtime-neutral).

Everything else is core, and core consumes declarations only. Cross-
package consumption of per-runtime behavior goes through the
capability table below — never a direct named call from core (Go
imports packages, not files, so the boundary is enforced by the sweep
check over call sites, not by the import graph).

## The sweep (r2 scope)

`rg -i 'claude|codex|devin|fake'` over cmd/, internal/, scripts/
(shipped shell + machine-readable data: roles/*.requirements.json,
instruction-bearing-paths.txt, event/enforcement configs), minus the
seams above and minus tests. The r1 sweep's three blind spots — scripts/,
the `fake` token, name fragments reached via helpers (`CLAUDE_`,
`.claude`, `claude-code-hooks.json`, `CLAUDE.md`) — are in scope by
construction now.

## The registry (the one declaration)

New leaf package `internal/runtimes`: pure data, importable by config,
validate, audit, missionrunner, and cmd without cycles. Shell never
parses it directly — plumbing consumes it via new thin `metasystem
runtime` verbs, per the core-vs-plumbing boundary: the declaration
lives in Go, scripts ask the binary.

Behavior does NOT live in the leaf (critique r3-2), and there is no
single untyped table (critique r4-4: delivery, recovery, and probes
share no contract — one table forces type erasure or import
inversions). Each behavioral OWNER keeps its own typed table in its
own package: delivery recollectors in `internal/host`, usage
recoverers in `internal/usage`, self-test probes in
`internal/adapter`. Each per-runtime seam file registers into its
owner's table from its own package init — never a central wire-up
list, so adding a runtime edits no shared file. Registration rejects
duplicate (runtime, capability) keys; each table exposes read-only
lookup and list views. The pure-data registry declares which
capabilities each runtime is EXPECTED to provide; a conformance test
joins declaration against each owner's list view both ways (expected
but unregistered fails; registered but undeclared fails). Core
consumes lookups and neutral results only.

Each runtime declares:
- name — validated against a shell-safe grammar (kebab, `^[a-z][a-z0-9-]{0,31}$`);
  every emitted relative path is validated clean-relative (no `..`, no
  absolute, no whitespace) at declaration-test time (critique r2-3)
- delegate/host capability flags (has adapter, has host launcher)
- OPTIONAL `configIdentityFilter` path (critique r4-5: runtime-common.sh:413
  constructs it per runtime and validate-metasystem.sh:373 hardcodes
  the three existing filter JSONs; `has adapter` cannot derive it —
  the fake adapter has none). Filter existence and validation derive
  from this declaration, AND live identity consumes it (critique
  r5-6): a `runtime config-identity-filter <runtime>` verb with
  pinned absent semantics (exit 1 + empty output when undeclared)
  replaces runtime-common.sh's path construction, so the bytes the
  suite validates are provably the bytes live identity hashes for
  snapshot selection (dispatch.sh:485) — one test asserts that
  equality
- `adoptable` (claude, codex, devin — never fake) and ONE
  `adoptionDefault` (claude), pinning adopt.sh:53's default and
  adopt-fixtures.sh:245's expectation (critique r2-3); config
  validation keeps consuming the FULL list including fake. `none` is
  NOT a runtime and never enters the registry: adoption and `config
  tailor` keep it as a named non-runtime sentinel handled BEFORE
  registry validation, preserving its exclusivity and empty-roster
  semantics (adopt.sh:72, conftailor.go:29, adopt-fixtures.sh:387;
  critique r3-4)
- registration contract — the ONE canonical row schema (critique
  r4-3: no parallel path declarations anywhere; the skills/agents
  `dirs` view below is a DERIVED view of these rows, not a second
  declaration). A row is a TAGGED UNION (critique r5-2: a flat
  5-tuple cannot carry payloads): every row has {id (stable artifact
  role, e.g. enforcement-config, skill-tree, skill-profiles),
  required|optional, destination}; per-operation payloads are
  - tree {source, mode: user-selectable link|copy (adopt.sh:293)}
  - copy-file {source}
  - json-strip-key {source, key} (claude's enforcement install
    strips _comment, adopt.sh:317)
  - skill-profiles {source pattern, delivery: copy|in-place,
    destination pattern, template-required/adopted-optional} — the
    typed profile contract carrying claude's <skill>.md projection,
    devin's <skill>/AGENT.md projection, and codex's in-place
    agents/openai.yaml consumption (adopt.sh:308;
    validate-metasystem.sh:525 template-required vs :580
    adopted-optional)
  `shippedEnforcementConfig` becomes a ROW REFERENCE by artifact
  role, never a repeated path (critique r5-2). Installation computes
  the UNION of the selected runtimes' rows before mutation,
  deduplicates compatible overlaps (identical operation, source,
  payload, mode — codex+devin's shared .agents/skills, adopt.sh:323,
  334), and rejects only INCOMPATIBLE overlaps (critique r5-3).
  Collision roots are PINNED to today's exact values — .claude,
  .devin, .agents, scanned as the FULL population regardless of
  selected runtimes (adopt.sh:129; selection-scoped scanning would
  weaken foreign-instruction detection); adding .codex is a
  human-adjudicated security change NOT taken here (critique r5-4).
  Re-running adoption: healthy same-SHA installation exits
  successfully as a no-op regardless of newly requested options
  (adopt.sh:117); incomplete same-SHA refuses (adopt.sh:125) — both
  pinned (critique r5-9). Shell framing is PINNED as ONE versioned
  tab encoding: schema-version header line, one row per line, fixed
  field order with the operation tag and payload fields flattened,
  zero rows = header only, trailing newline; the declaration grammar
  forbids tabs and newlines in every field (critique r5-9). Config
  directory-presence validation (validate.go:339), installed-
  enforcement paths, and byte-drift validation are DERIVED from
  these rows. A future operation the vocabulary cannot express
  registers in a SEPARATE typed INSTALLER capability table owned by
  the installation layer (critique r5-5: not the probes table),
  with duplicate rejection, lookup/list conformance views, and one
  generic shell invocation — a registry-plus-seam edit; the doctrine
  never enumerates uses. adopt.sh builds the source-fresh binary
  BEFORE any pre-mutation registry query (today it rebuilds only
  after mutation, adopt.sh:50,225,268)
- session-environment names (claude: CLAUDE_PROJECT_DIR; devin: its
  project-dir variable) for supervision-hook.sh's dispatch
- skills/agents directories — a DERIVED view over the registration
  rows (critique r4-3), surfaced as `runtime dirs` for consumers; not
  a separate declaration
- instruction-file name (CLAUDE.md for claude; AGENTS.md otherwise)
- tailoring priority — an explicit UNIQUE integer pinning today's
  precedence codex > devin > claude > fake (critique r1-2), plus an
  optional synthesized-model value (fake declares "fake-model")
- TWO independent optional hook fields (critique r2-2 — bundling them
  dropped real coverage: codex-hooks.json and devin-hooks.json ship
  today): `shippedEnforcementConfig` (filename; declared by claude,
  codex, AND devin) and `liveSelfCheck` {vendoredMarker} (declared by
  claude only). EVERY shipped-config consumer iterates the first —
  the audit loop AND the suite's required-asset and JSON-shape rows
  (validate-metasystem.sh:315,464; critique r3-5). `hooks check`
  KEEPS its explicit live/shipped path arguments (the suite resolves
  live settings in the parent repository for the nested-template case,
  validate-metasystem.sh:1382, and a clean-relative path cannot
  express that); `--runtime` selects only the capability and marker.
  Both the nested-template and adopted-root cases are pinned
- session-environment names are validated against `^[A-Z][A-Z0-9_]*$`
  and expanded indirectly (bash `${!name}`), never via eval
  (critique r3-5)
- `permissionResiduals`: an exact map of permission FIELD (readRoots,
  writeRoots, network, ...) to a GLOBALLY UNIQUE residual identifier
  (critique r3-1) — the registry names the enforcement gaps; role
  files waive an identifier UNDER ITS FIELD; selection fails closed
  when an unverified restrictive field has no declared residual, when
  an identifier is claimed for another field or runtime, and when
  identifiers collide (uniqueness is a declaration test). SCOPE
  (critique r4-1): residuals govern ONLY live selection — the case
  where a snapshot lists a field in permissions.unverified
- `expectedEnvelopeEnforcement[field]`: a SEPARATE complete map over
  EXACTLY {writeRoots, readRoots, network} with values
  mapped | notEnforced (critique r5-8: snapshot.go:101 and
  select.go:260 accept exactly these two; a conformance fixture pins
  the registry and snapshot enums identical) that static adapter
  validation asserts against each adapter's declared snapshot shape
  (critique r4-1: codex declares readRoots notEnforced while
  reporting NO unverified fields — the two facts are independent, and
  deriving one from the other either changes live security decisions
  or falsifies validation). Explicitly REFUSED: the notEnforced ⇒
  unverified derivation — that is a security-policy change needing
  human adjudication of codex and the role waivers, not a preserving
  move

The `metasystem runtime` verbs are purpose-filtered, not one flat list
(critique r2-3): `list` (all), `list --adoptable`, `adoption-default`,
`dirs`, `enforcement-config`, `self-check`, `instruction-file`,
`session-env`, `config-identity-filter`, `registration` (the
structured contract in the ONE versioned tab encoding pinned above —
critique r5-9 removed this section's stale NUL-or-tab alternative). Each verb has a pinned output encoding
and exit codes (0 ok, 1 unknown runtime, 2 usage); adoption's
pre-mutation checks run against these verbs before any file lands.

Conformance tests pin: registered real runtimes have their declared seam
files/launchers; the tailoring precedence, fake-model synthesis,
adoption default, and registration layout validation are byte-identical
to today; a NEWLY declared instruction filename reaches EVERY consumer
in Class 8; name/path grammars reject a hostile declaration.

## Rulings by class

### Class 1 — provenance prose: STAYS

`internal/dispatch/{mirror,ownerlock,record}.go`,
`internal/evidence/gc.go`, `internal/supervise/reservedcap.go` (the
comment hits), `internal/missionrunner/patience.go`,
`internal/config/model.go`: comments naming the critic that produced a
finding or a motivating incident. Provenance, not integration. Doctrine
says so explicitly.

### Class 2 — the runtime universe open-coded in Go: GENERALIZE via the registry

- `internal/config/validate.go:118` (supportedRuntimes), `:344`
  (per-runtime directory table)
- `internal/validate/conftailor.go:21,47` — PLUS its two policies the
  r1 ruling missed: the fixed default precedence at `:43` and the
  fake-only fake-model synthesis at `:64,127` (critique r1-2). Both
  become registry declarations with pinning tests.
- `cmd/metasystem/config_verbs.go:41-50`
- `internal/audit/metasystem.go:332` conformance loop — reworked per
  Class 8, not merely re-pointed at the registry.

### Class 3 — the shipped script control plane: GENERALIZE via runtime verbs

The r1 sweep missed these entirely (critique r1-1):
- `scripts/adopt.sh:77,308` — open-coded runtime rejection and
  per-runtime installation arms
- `scripts/agents/supervision-hook.sh:6,22` — runtime rejection plus
  Claude/Devin project-directory variables
- `scripts/validate-metasystem.sh:473,505,591` — fixed runtime lists
  and registration layouts
- `scripts/validate-metasystem.sh:315,368,427,464,488` (critique
  r3-5, r3-8) — hardcoded shipped-hook assets and their JSON-shape
  validation, host and adapter/config-filter asset lists, per-host
  syntax checks, and the expected envelope-enforcement declarations

RULING: the runtime LISTS and per-runtime lookups in these scripts come
from the purpose-filtered `bin/metasystem runtime ...` verbs (adopt.sh
uses `list --adoptable` and `adoption-default`; config surfaces use the
full list — the two populations differ and MUST not merge, critique
r2-3). Installation and registration-layout validation consume the
declared registration contract: adopt.sh's install arms become a generic
loop over `runtime registration <name>` rows (the canonical schema
above — critique r4-3 removed this section's stale copy of it), and
validate-metasystem.sh's byte-drift checks walk the same rows, so
neither weakens (critique r2-6). supervision-hook.sh resolves the project-dir variable via
`runtime session-env`. The suite's per-runtime validation rows derive
from the registry (critique r3-8): required seam assets and per-host
syntax checks walk the capability flags and registration rows;
shipped-hook presence and JSON shape iterate shippedEnforcementConfig;
the expected envelope-enforcement rows consume the DECLARED
expectedEnvelopeEnforcement map (critique r4-1 killed the
residual-derivation: codex's notEnforced-yet-never-unverified
readRoots disproves the equivalence; Devin's unenforced boundaries
stay protected by their own declared rows). Where an
arm is irreducibly runtime-specific plumbing beyond the contract's
vocabulary, it moves into that runtime's adapter script and adopt.sh
dispatches to it — the contract rows are preferred, and anything
beyond them registers in the typed installer capability table
(critique r5-5: no doctrine enumeration of uses; registry-plus-seam
edits only).

### Class 4 — the hooks self-check: GENERALIZE

`internal/hooks/hooks.go:62` hardcodes `$CLAUDE_PROJECT_DIR/metasystem`.
RULING: `CheckOwnHooks` takes the vendored-entry marker as a parameter;
`metasystem hooks check` gains a REQUIRED `--runtime` flag (critique
r1-10) that resolves the vendoredMarker from the registry's
`liveSelfCheck` field while KEEPING its explicit live/shipped path
arguments (critique r3-5: the suite's nested-template case resolves
live settings in the parent repository); a runtime without
liveSelfCheck gets a LOUD "no live self-check declared for <runtime>".
This is INDEPENDENT of shippedEnforcementConfig (critique r2-2): codex
and devin keep their shipped-config audit rows while declaring no live
self-check. The one suite call site passes the runtime.

### Class 5 — the devin resume branch: MOVE THE RECOLLECTION ONLY

`internal/missionrunner/turnio.go:63`. The r1 ruling moved the whole
body and would have created an import cycle (missionrunner already
imports host) and displaced runner-owned validation (critique r1-3).
RULING: `internal/host` exposes a per-runtime delivery-recollection
capability returning a recollected candidate (or nothing); turnio keeps
the session-fault gate, the one-resume limit, validation, the
ReturnValidation construction, and the original-error fallback, and
consumes the capability by lookup, never by name.

### Class 6 — usage parsing: MOVE BEHIND THE SEAMS, KEEP NEUTRAL ARITHMETIC

`internal/usage/usage.go` holds Devin metric semantics and Codex stream
semantics; `cmd` calls `usage.DevinUsage` directly from two verbs
(critique r1-4). The r1 ruling ALSO misread the baseline: Claude appends
its result with top-level usage to events.jsonl, and today's parser
recognizes input_tokens/output_tokens/reasoning_tokens — a claude round
IS partially recoverable today (critique r1-5).

RULING: `internal/usage` is DECLARED the single seam owner of shared
usage parsing (critique r2-7: adapter AND host both consume Devin's
math — the package's documented dedup purpose stands), reorganized into
per-runtime files (usage/claude.go, usage/codex.go, usage/devin.go —
item 17's placement rule) that are sanctioned seam entries, with
usage.go retaining only runtime-neutral typed-usage arithmetic and
serialization. Core packages other than the seams and the fence's
declared recovery dispatch may not import the per-runtime files. The
two Devin cmd verbs reroute through adapter/host entry points that call
the seam owner (critique r1-4). Recovery is declared per runtime over a
runtime-NEUTRAL recovery context {repo, round directory, predecessor
artifacts} and returns a `RecoveryOutcome` (critique r2-7), not a bare
value or error:
- state: recovered | unavailable | unsupported
- typed usage (on recovered) fed through the SAME generic aggregator
  as reported usage (critique r1-6); `recovered` is VALID ONLY when
  the aggregator reports measured=true — a typed object with every
  value null normalizes to unavailable with today's source/detail
  (critique r3-6: fence.go:786 counts fields today; that check is the
  contract, not an implementation detail). `unsupported` maps outward
  exactly as today's unavailable rows do (provenance, source, detail),
  plus the declared reason
- exact source paths and detail for provenance, matching today's
  per-round source/detail records (fence.go:794)
- malformed provider evidence NORMALIZES to unavailable or is skipped
  per today's parser semantics (usage.go:236 skips bad JSONL lines and
  still recovers an earlier valid block) — a truncated final line in a
  killed stream must NEVER become an aggregate-wide error
  (usageprojection.go:13's standing-aggregate hazard)
Group-death and custodian-death gating stays in `internal/mission`
(fence.go:746). Preservation tests pin today's claude and codex
recoveries field-for-field, including the malformed-tail case, PLUS
the devin (unsupported) and no-recoverer rows' outward provenance
(critique r3-6). Devin
dead-round recovery is DECLARED UNSUPPORTED — its usage math needs
transcript metrics and predecessor cumulative state a dead round's
event stream does not carry; today's behavior (unavailable) is
preserved, now stated.

### Class 7 — round-usage recovery call in the fence: GENERALIZE

`internal/mission/fence.go:786` calls `usage.CodexUsageValue` for every
provider. RULING: the fence asks the Class-6 per-runtime recovery
declaration by provider and aggregates the returned typed usage
generically. No declared recoverer → honestly unavailable, as today.

### Class 8 — the audit/conformance boundary: GENERALIZE BOTH CONSUMERS

Two defects beyond r1's audit-allowlist fix:
- The conformance no-waiver set hardcodes CLAUDE.md via
  `scripts/agents/instruction-bearing-paths.txt:5` and
  `internal/validate/conformance.go:385,448`; a future runtime's
  instruction file could be merged under a prose waiver (critique
  r1-8). And the filename has more consumers the r2 ruling missed
  (critique r2-5): the audit's outside-reference scan roots
  (audit/metasystem.go:29), its instruction inventory (:107),
  adoption's collision detection (adopt.sh:129), and the adoption
  payload allowlist (adopt.sh:166). RULING: the registry-declared
  instruction-file set feeds ALL of: audit inventory, outside-reference
  scan roots, conformance no-waiver protection, adoption collision
  detection, and adoption payload inclusion (the last two via the
  `runtime` verbs). Declared instruction files are themselves
  sanctioned seam entries. ONE test declares a new filename and proves
  it lands in every consumer.
- The audit loop at `internal/audit/metasystem.go:332` requires every
  runtime to have a contract row and enforcement config; that fails
  hookless runtimes (critique r1-10). RULING: the loop iterates
  runtimes DECLARING hook enforcement; the handwritten conformance row
  in docs/design/turn-verdict-delivery-contract.md is admitted in the
  doctrine as an explicit exception to "one declaration" — it is
  evidence prose, and the audit checks consistency between it and the
  declarations rather than deriving it. (The other declared exception
  is Class 9's role-file waiver edit.)

### Class 9 — the permission-waiver matrix: GENERALIZE, EXACTLY

`scripts/agents/roles/*.requirements.json` name devin under
readRoots/writeRoots, and `internal/capability/select.go:129,300` treat
those names as waiver gates — live security behavior, not fixture data
(critique r1-7). The r2 ruling moved the matrix into the compiled
registry; critique r2-1 showed that INVERTS security ownership: role
files are the live, checkout-local control (select.go:81 reads them on
every dispatch; docs/orchestration.md:58 assigns capability policy to
them), and a compiled default would keep authorizing after a role file
revoked. RULING: waivers STAY role-owned and live in the role
requirements files. The registry declares only each runtime's
PERMISSION-RESIDUAL IDENTIFIERS (unique names for the enforcement gaps
a role could waive, e.g. devin's unenforced read/write roots);
capability selection matches a role file's explicit waiver of an
identifier generically instead of matching the runtime NAME. A role
file that does not name the identifier fails closed — including for
every future runtime, whose new identifiers no existing role file
waives. This is a declared exception to "one registry edit": granting
a new runtime's waiver is a HUMAN policy edit to role files, by
design. A golden test regenerates today's decisions for every role
file. Explicitly REFUSED: waiving any runtime that reports notEnforced
— that would broaden privilege.

### Class 10 — the `fake` runtime's production branches: NAMED EXCEPTION, PER SITE

The r1 sweep excluded `fake` while treating it as part of the universe
(critique r1-9), and r2's list was still incomplete (critique r2-4).
The FULL enumeration of fake-gated production branches, each a
trusted-fixture capability with authority consequences:
- host identity/wind-down: `internal/missionrunner/host.go:213`,
  `internal/missionrunner/launch.go:134`
- the synthetic census ancestor:
  `internal/census/ancestor_production.go:55`
- fake-only census process enumeration: `internal/census/run.go:355`
- the trusted process-identity override:
  `internal/supervise/reservedcap.go:39`
- mission lease identity substitution:
  `internal/dispatch/mission.go:100`
- delegate-writable mirror fault injection:
  `internal/dispatch/mirror.go:44`
- fake supervisor and process-group ownership fallbacks:
  `scripts/agents/dispatch.sh:238,265`
- the SHARED identity-fixture authority path (critique r3-7): the
  fixture reader `internal/identity/fixture.go:25` and its authority
  consumers — census authentication identity
  (`internal/census/verbs.go:28`), lease identity
  (`internal/lease/identity.go:36`), custodian liveness
  (`internal/identity/custodian.go:24`)
- mission-runner fixture helpers accepting fixture commands, group
  ownership, and published fixture identity:
  `internal/missionrunner/proc.go:32,76,102`
- the DIRECT census-liveness consumer `internal/census/run.go:453`
  (critique r4-2 caught its omission)
RULING: `fake` remains a NAMED test-harness exception; every branch
above is listed in the doctrine and keeps its explicit local guard at
its security boundary. The identity-fixture path gets a REAL,
PRE-ARMING, ROOT-BOUND authorization (critique r4-2, r5-1 — the r5
wording "armed fixture mode" collided with supervision arming, which
happens AFTER the fixture-capable reads the bootstrap needs). The
predicate is EXACTLY today's reserved-cap rule: the checkout's conf
declares `metasystem.runtimes=fake` (reservedcap.go:38) — read from
the ROOT, not the environment. Because `internal/identity` is
foundational and must not read configuration (docs/architecture.md
layering), the checked capability is a small FixtureAuthorization
VALUE constructed at the entry points that HAVE a root — census
verbs, lease classification, custodian callers, the reserved-cap
scan — and passed down to the fixture reader; the reader refuses
fixture identity without it. arm-supervision.sh constructs the
authorization from its root BEFORE the announcement, lease
authorization, and any lock or mutation (its first fixture-capable
read at :359-361 is already covered). Tests: positive fake-checkout
bootstrap, plus census authentication, census supervision liveness,
lease classification, and custodian verdicts each REFUSING fixture
identity in a non-fake checkout. The pinning test for this list is enumeration-based (the
sweep plus the named helper paths), because a textual fake-string
sweep cannot find helper-hidden authority (critique r3-7). Explicitly REFUSED: a generic
IsFixture bypass — future fixtures must NOT inherit security-sensitive
bypasses by declaration.

### Class 11 — cmd verb tables naming runtimes: STAYS (with the Class-6 reroute)

Verb names are the seam's CLI face. Bodies stay one-call-thin into seam
entry points; the two Devin usage verbs are the known reroutes.

### Class 12 — tests naming runtimes: STAYS

Fixture data exercising seam-declared behavior. The mechanical fence for
future core code is item 17's scope, not built here.

### Class 13 — the self-test's --devin-checks switch: GENERALIZE

`cmd/metasystem/adapter_selftest_verbs.go:91,146` expose
`--devin-checks`, steering shared self-test code
(`internal/adapter/selftestrun.go:270,343`) by runtime name (critique
r3-9). RULING: the switch becomes a TYPED probe registered in the adapter's
capability table (critique r4-6: the old flag steers four stages, not
one marker). The probe lifecycle contract: prepare scratch state
(selftestrun.go:193), contribute prompt text (:270), verify returned
evidence (:343), and return the exact behavior labels earned for the
pass record (:354; selftest.go:123's documented-exit-status-observation
and symlinked-skill-discovery both preserved). The runner selects the
RUNTIME'S DECLARED probes from the registry expectation and REFUSES a
cross-runtime or undeclared probe name; the generic verbs carry
`--probe` only as a filter over declared probes. A preservation test
pins today's devin pass-record fields byte-for-byte. (The critic
marks this mechanical-grain; the lifecycle contract above is the
fixture-arbitrable shape.)

### Class 14 — shipped operational documentation: GENERALIZE THE CLAIMS

Shipped docs enumerate the runtime universe and its installation
contract as core prose (critique r4-7): docs/project-adaptation.md:5,10
(runtime lists and manual installation layouts), docs/orchestration.md:226
(the runtime mechanics table), docs/glossary.md:182 (the universe),
README.md:169 (three profile formats claimed as the supported set).
RULING: shipped operational documentation joins the sweep scope.
Exhaustive runtime lists and installation instructions are replaced by
pointers to the registry's views (`runtime list`, `runtime
registration`) and ONE generic manual-repair procedure; runtime-
specific operational detail moves to seam-owned help or declared
assets. Non-exhaustive EXAMPLES stay legal where they neither claim
the supported universe nor prescribe the installation contract — the
doctrine states that boundary, and the docs audit (the same one that
checks instruction assets) asserts no doc claims an exhaustive
universe outside registry-derived text.

### Class 15 — checkout configuration: A NARROW SANCTIONED SEAM

`metasystem.conf` carries live runtime selections (metasystem.conf:5)
and one default-model row per real runtime (:30), consumed by dispatch
(roster.go:117), tailoring (conftailor.go:125), and validation
(validate.go:275) — and the shipped template scaffold enumerates the
universe, so a new runtime edits core (critique r5-7). RULING:
runtime-name VALUES in checkout configuration are a sanctioned narrow
seam: they express the OPERATOR'S selection, are validated against
the registry, and never define the supported universe. The per-runtime
model boilerplate in the shipped scaffold is replaced by generic
tailoring that materializes `role.default.model.<runtime>=<model>`
for every selected non-synthesized runtime (the synthesized fake-model
rule already lives in the registry declaration). Sweep scope includes
the shipped conf template.

## Doctrine

docs/architecture.md gains the standing rule: the core never names an
agent runtime in behavior; runtime knowledge lives in the sanctioned
seams as declarations the core consumes; the sanctioned appearances of
runtime names outside seam files are (a) provenance comments, (b) seam
CLI verb names, (c) the named `fake` test-harness exceptions listed
there, and (d) the handwritten conformance-evidence rows in
docs/design/turn-verdict-delivery-contract.md; adding a runtime touches
its seam entries plus one registry declaration.

## Order of moves

1. `internal/runtimes` registry + `metasystem runtime` verbs + Class-2
   consumers + pinning tests (precedence, fake-model, seam conformance).
2. Class 8 both consumers (audit loop + conformance protected set) with
   the new-filename test.
3. Class 4 (hooks marker via required --runtime + suite call site).
4. Class 3 (scripts consume runtime verbs; runtime-specific arms move
   into adapter scripts).
5. Class 5 (host recollection capability).
6. Classes 6+7 (usage seam split, fence dispatch, preservation tests,
   Devin verbs reroute).
7. Class 9 (waiver matrix with the golden test).
8. Doctrine in docs/architecture.md (including the Class-10 list).
9. Full pre-verify, both host gates, goal done.

Acceptance question for critique r2: does any ruling still misclassify a
site, break behavior it claims to preserve, or leave "adding a runtime"
touching core?
