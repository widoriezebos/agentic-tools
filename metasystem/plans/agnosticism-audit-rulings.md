# Agnosticism audit: the ruling set

- Status: DRAFT r3 — critique r2 folded (7 findings); awaiting critique r3
- Goal: agnosticism-audit (backlog item 16, D-series: pending)
- Next step: Fold the critique verdict when run agno-critique-r3 concludes; implement only after convergence.
- In flight right now: run agno-critique-r3 (codex xhigh critique of this revision; watch it with: bin/metasystem run watch --id agno-critique-r3 --root .)

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
critiques are preserved beside this plan; r3 is the r2 fold.

## The sanctioned seams

1. `internal/adapter/` — delegate-side integration, per-runtime files.
2. `internal/host/` — host-side Go integration, per-runtime files.
3. `scripts/agents/hosts/<runtime>.sh` — host-side shell launchers (a
   runtime's host implementation MAY be shell that reuses its adapter —
   codex is exactly this; host conformance checks declared capabilities
   and an executable launcher, never "a Go file per runtime").
4. `scripts/agents/adapters/*.sh` and `scripts/enforcement/<runtime>-*.json`
   — per-runtime plumbing and shipped configuration.
5. `cmd/metasystem` verb wiring for the adapter/host families — thin
   routing whose verb NAMES are the seam's CLI face, and whose bodies
   call seam entry points only (obligation: the two Devin verbs that
   today call `usage.DevinUsage` directly reroute through the seam).

Everything else is core, and core consumes declarations only.

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
runtime` verbs (list, dirs, enforcement-config, instruction-file), per
the core-vs-plumbing boundary: the declaration lives in Go, scripts ask
the binary.

Each runtime declares:
- name — validated against a shell-safe grammar (kebab, `^[a-z][a-z0-9-]{0,31}$`);
  every emitted relative path is validated clean-relative (no `..`, no
  absolute, no whitespace) at declaration-test time (critique r2-3)
- delegate/host capability flags (has adapter, has host launcher)
- `adoptable` (claude, codex, devin — never fake) and ONE
  `adoptionDefault` (claude), pinning adopt.sh:53's default and
  adopt-fixtures.sh:245's expectation (critique r2-3); config
  validation keeps consuming the FULL list including fake
- registration contract — the structured shape adopt.sh:311 and
  validate-metasystem.sh:591 currently open-code: entries of
  {source pattern, destination, kind, required|optional, copy|link}
  so installation and byte-drift validation both consume declarations
  (critique r2-6)
- session-environment names (claude: CLAUDE_PROJECT_DIR; devin: its
  project-dir variable) for supervision-hook.sh's dispatch
- skills/agents directories (today's validate.go:344 table)
- instruction-file name (CLAUDE.md for claude; AGENTS.md otherwise)
- tailoring priority — an explicit UNIQUE integer pinning today's
  precedence codex > devin > claude > fake (critique r1-2), plus an
  optional synthesized-model value (fake declares "fake-model")
- TWO independent optional hook fields (critique r2-2 — bundling them
  dropped real coverage: codex-hooks.json and devin-hooks.json ship
  today): `shippedEnforcementConfig` (filename; declared by claude,
  codex, AND devin) and `liveSelfCheck` {settingsPath, vendoredMarker}
  (declared by claude only). The audit iterates the first; `hooks
  check` requires the second
- permission-residual identifiers — see Class 9 (the WAIVERS stay
  role-owned; the registry only NAMES the residuals a role may waive)

The `metasystem runtime` verbs are purpose-filtered, not one flat list
(critique r2-3): `list` (all), `list --adoptable`, `adoption-default`,
`dirs`, `enforcement-config`, `self-check`, `instruction-file`,
`session-env`, `registration` (the structured contract, NUL- or
tab-delimited rows for shell). Each verb has a pinned output encoding
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

RULING: the runtime LISTS and per-runtime lookups in these scripts come
from the purpose-filtered `bin/metasystem runtime ...` verbs (adopt.sh
uses `list --adoptable` and `adoption-default`; config surfaces use the
full list — the two populations differ and MUST not merge, critique
r2-3). Installation and registration-layout validation consume the
declared registration contract: adopt.sh's install arms become a generic
loop over `runtime registration <name>` rows ({source, destination,
kind, required|optional, copy|link}), and validate-metasystem.sh's
byte-drift checks walk the same rows, so neither weakens (critique
r2-6). supervision-hook.sh resolves the project-dir variable via
`runtime session-env`. Where an arm is irreducibly runtime-specific
plumbing beyond the contract's vocabulary, it moves into that runtime's
adapter script and adopt.sh dispatches to it — the contract rows are
preferred; the dispatch escape is the exception and each use is listed
in the doctrine.

### Class 4 — the hooks self-check: GENERALIZE

`internal/hooks/hooks.go:62` hardcodes `$CLAUDE_PROJECT_DIR/metasystem`.
RULING: `CheckOwnHooks` takes the vendored-entry marker as a parameter;
`metasystem hooks check` gains a REQUIRED `--runtime` flag (critique
r1-10) that resolves settingsPath and vendoredMarker from the registry's
`liveSelfCheck` field; a runtime without liveSelfCheck gets a LOUD "no
live self-check declared for <runtime>". This is INDEPENDENT of
shippedEnforcementConfig (critique r2-2): codex and devin keep their
shipped-config audit rows while declaring no live self-check. The one
suite call site passes the runtime.

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
  as reported usage (critique r1-6)
- exact source paths and detail for provenance, matching today's
  per-round source/detail records (fence.go:794)
- malformed provider evidence NORMALIZES to unavailable or is skipped
  per today's parser semantics (usage.go:236 skips bad JSONL lines and
  still recovers an earlier valid block) — a truncated final line in a
  killed stream must NEVER become an aggregate-wide error
  (usageprojection.go:13's standing-aggregate hazard)
Group-death and custodian-death gating stays in `internal/mission`
(fence.go:746). Preservation tests pin today's claude and codex
recoveries field-for-field, including the malformed-tail case. Devin
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
RULING: `fake` remains a NAMED test-harness exception; every branch
above is listed in the doctrine and keeps its explicit local
`== "fake"` guard at its security boundary. The implementation adds a
pinning test that THIS list matches the tree (the sweep re-run), so a
new fake branch cannot land unlisted. Explicitly REFUSED: a generic
IsFixture bypass — future fixtures must NOT inherit security-sensitive
bypasses by declaration.

### Class 11 — cmd verb tables naming runtimes: STAYS (with the Class-6 reroute)

Verb names are the seam's CLI face. Bodies stay one-call-thin into seam
entry points; the two Devin usage verbs are the known reroutes.

### Class 12 — tests naming runtimes: STAYS

Fixture data exercising seam-declared behavior. The mechanical fence for
future core code is item 17's scope, not built here.

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
