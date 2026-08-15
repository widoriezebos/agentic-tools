# Agnosticism audit: the ruling set

- Status: DRAFT r2 — critique r1 folded (10 findings, all material); awaiting critique r2
- Goal: agnosticism-audit (backlog item 16, D-series: pending)
- Next step: Fold the critique verdict when run agno-critique-r2 concludes; implement only after convergence.
- In flight right now: run agno-critique-r2 (codex xhigh critique of this revision; watch it with: bin/metasystem run watch --id agno-critique-r2 --root .)

The human's rule, verbatim intent (2026-08-15): "the meta system must be
agent agnostic (it should work with Codex and Devin and any other future
agent too)." The end state: every runtime-integration surface is DECLARED
by its runtime's seam entry, the core consumes declarations, and adding a
runtime touches only its own seam files plus one registry declaration.

Revision history: r1 swept only cmd/ + internal/ and missed the shipped
script control plane, the `fake` runtime's production branches, and the
role-requirements waiver matrix; its usage and host-resume moves crossed
seam boundaries the wrong way. Critique r1
(plans/agnosticism-critique-r1.md) found all of this; r2 is the fold.

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
- name; delegate/host capability flags (has adapter, has host launcher)
- skills/agents directories (today's validate.go:344 table)
- instruction-file name (CLAUDE.md for claude; AGENTS.md otherwise)
- tailoring priority — an explicit UNIQUE integer pinning today's
  precedence codex > devin > claude > fake (critique r1-2: map order
  never substitutes for policy), plus an optional synthesized-model
  value (fake declares "fake-model"; real runtimes declare none)
- OPTIONAL hook-enforcement capability: {enforcement-config filename,
  live-settings path, vendored-entry marker} — declared only by
  runtimes that ship hook enforcement (claude today). Enforcement
  config and self-check are INDEPENDENT optional capabilities
  (critique r1-10): a hookless runtime is not an audit failure
- permission-waiver matrix: the exact (role, field) sites where this
  runtime's non-enforcement is waived (critique r1-7) — see Class 9

Conformance tests pin: registered real runtimes have their declared seam
files/launchers; the tailoring precedence and fake-model synthesis are
byte-identical to today; a NEWLY declared instruction filename enters
both the audit allowlist and the conformance no-waiver set.

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
from `bin/metasystem runtime ...` verbs. The per-runtime ARMS (what to
install for claude, which env var carries the project dir) are seam
knowledge: each runtime's session-env names move into its declaration
(claude: CLAUDE_PROJECT_DIR; devin: its own), and the scripts branch on
declared values, not names. Where a script's arm is irreducibly
runtime-specific plumbing (adopt.sh's install steps), the arm moves into
that runtime's adapter script and adopt.sh dispatches to it.

### Class 4 — the hooks self-check: GENERALIZE

`internal/hooks/hooks.go:62` hardcodes `$CLAUDE_PROJECT_DIR/metasystem`.
RULING: `CheckOwnHooks` takes the vendored-entry marker as a parameter;
`metasystem hooks check` gains a REQUIRED `--runtime` flag (critique
r1-10: a claude default would reintroduce the core name) that resolves
the marker and paths from the registry's hook-enforcement capability; a
runtime without that capability gets a LOUD "no hook enforcement
declared for <runtime>". The one suite call site passes the runtime.

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

RULING: per-runtime parsing/recovery moves into the adapter/host seams
(per-runtime files there); `internal/usage` retains only runtime-neutral
typed-usage arithmetic and serialization. Recovery is declared per
runtime over a runtime-NEUTRAL recovery context {repo, round directory,
predecessor artifacts} and returns TYPED usage fed through the same
generic aggregator as reported usage (critique r1-6: the token-only sink
undercounts cost/provider-unit recoveries; unifying is an intentional,
tested improvement, not silent). Preservation tests pin today's claude
and codex recoveries field-for-field. Devin dead-round recovery is
DECLARED UNSUPPORTED — its usage math needs transcript metrics and
predecessor cumulative state that a dead round's event stream does not
carry; today's behavior (unavailable) is preserved, now stated. The two
Devin cmd verbs reroute through the seam entry points.

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
  r1-8). RULING: registry-declared instruction filenames feed BOTH the
  audit allowlist and the conformance protected set; the test declares
  a new filename and proves it lands in both.
- The audit loop at `internal/audit/metasystem.go:332` requires every
  runtime to have a contract row and enforcement config; that fails
  hookless runtimes (critique r1-10). RULING: the loop iterates
  runtimes DECLARING hook enforcement; the handwritten conformance row
  in docs/design/turn-verdict-delivery-contract.md is admitted in the
  doctrine as the one explicit exception to "one declaration" — it is
  evidence prose, and the audit checks consistency between it and the
  declarations rather than deriving it.

### Class 9 — the permission-waiver matrix: GENERALIZE, EXACTLY

`scripts/agents/roles/*.requirements.json` name devin under
readRoots/writeRoots, and `internal/capability/select.go:129,300` treat
those names as waiver gates — live security behavior, not fixture data
(critique r1-7). RULING: the exact current (runtime, role, field) waiver
matrix moves into the registry's per-runtime waiver declaration,
consumed generically by capability selection. The move is
byte-preserving: a golden test regenerates today's decisions for every
role file. Explicitly REFUSED: waiving any runtime that reports
notEnforced — that would broaden privilege.

### Class 10 — the `fake` runtime's production branches: NAMED EXCEPTION, PER SITE

The r1 sweep excluded `fake` while treating it as part of the universe
(critique r1-9). Its production branches are trusted-fixture
capabilities: host identity/wind-down (`internal/missionrunner/host.go:213`,
`launch.go:134`), the synthetic census ancestor
(`internal/census/ancestor_production.go:55`), the trusted
process-identity override (`internal/supervise/reservedcap.go:39`).
RULING: `fake` remains a NAMED test-harness exception; each fake-gated
capability is listed individually in the doctrine and keeps its explicit
`== "fake"` guard. Explicitly REFUSED: a generic IsFixture bypass —
future fixtures must NOT inherit security-sensitive bypasses by
declaration.

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
