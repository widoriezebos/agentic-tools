# Agnosticism audit: the ruling set

- Status: DRAFT — awaiting one codex critique before any move is implemented
- Goal: agnosticism-audit (backlog item 16, D-series: pending)
- Next step: Critique this ruling set with codex at xhigh; fold; then implement the moves.
- In flight right now: none

The human's rule, verbatim intent (2026-08-15): "the meta system must be
agent agnostic (it should work with Codex and Devin and any other future
agent too)." The end state: every runtime-integration surface is DECLARED
by its runtime's seam entry, the core consumes declarations, and adding a
runtime touches only its own seam files.

## The sanctioned seams

1. `internal/adapter/` — delegate-side integration, per-runtime files
   ({claude,codex,devin,fake}.go and their named companions).
2. `internal/host/` — host-side integration, per-runtime files.
3. `cmd/metasystem` verb wiring for the adapter/host families — thin
   routing whose verb NAMES are the seam's CLI face.
4. `scripts/agents/adapters/*.sh` and `scripts/enforcement/<runtime>-*.json`
   — per-runtime plumbing and shipped configuration.

Everything else is core, and core consumes declarations only.

## The sweep

`rg -i 'claude|codex|devin' cmd/ internal/` minus the two seam packages,
2026-08-16, tree 1255a95: 19 production files, 66 files total with tests.

## Rulings by class

### Class 1 — provenance prose: STAYS (9 files)

`internal/dispatch/{mirror,ownerlock,record}.go`,
`internal/evidence/gc.go`, `internal/supervise/reservedcap.go`,
`internal/missionrunner/patience.go`, `internal/config/model.go`: every
hit is a comment naming the critic that produced a review finding
("review codex-1") or a motivating incident (Devin's tokenless accounts,
a model-name normalization example). Comments are provenance, not
integration; erasing them would erase the audit trail. Doctrine will say
so explicitly so future sweeps don't relitigate them.

### Class 2 — the runtime universe open-coded four times: GENERALIZE

The same fact — "the supported runtimes are claude, codex, devin, fake"
— is open-coded at:

- `internal/config/validate.go:118` (supportedRuntimes map) and `:344`
  (per-runtime skills/agents directory table)
- `internal/validate/conftailor.go:21,47` (two more copies)
- `cmd/metasystem/config_verbs.go:41-50` (usage string + switch)
- `internal/audit/metasystem.go:332` (conformance-row loop) and `:348`
  (`enforcementConfigFor`: claude → claude-code-hooks.json)

Adding a runtime today touches five core files — the exact failure the
human named. RULING: one declaration. New leaf package
`internal/runtimes` (pure data, no behavior, importable by config,
validate, audit, and cmd without cycles) declaring per runtime: name,
skill/agent directories, enforcement-config filename, instruction-file
name (CLAUDE.md for claude, AGENTS.md otherwise), and whether it is a
real delegate runtime or a fixture ("fake"). The five sites consume the
registry. `internal/adapter` and `internal/host` gain a conformance test
each: every registered real runtime has its seam files, so the registry
cannot drift from the seams.

### Class 3 — the hooks self-check defines "runs under itself" as Claude-only: GENERALIZE

`internal/hooks/hooks.go:62` hardcodes `$CLAUDE_PROJECT_DIR/metasystem`
in a CORE package, and the whole check is silently meaningless for a
codex or devin session. RULING: the vendored-entry marker becomes a
parameter of `CheckOwnHooks`, declared per runtime in the registry
(claude declares `$CLAUDE_PROJECT_DIR/metasystem`; runtimes with no hook
dialect declare nothing). `cmd/metasystem hooks check` gains
`--runtime` (default claude — today's only hook dialect — so the one
call site in validate-metasystem.sh keeps working), looks the marker up,
and a runtime with no declared self-check is a LOUD "no self-check
declared for <runtime>" instead of silence. The suite call site is
updated to pass the runtime explicitly.

### Class 4 — the devin resume branch in core: MOVE BEHIND THE HOST SEAM

`internal/missionrunner/turnio.go:63`: `turn.Runtime == "devin"` gates
`resumeDevinDelivery`, and turnio.go names host.HostDevinCollect. The
D64 delivery ladder is real behavior; the ownership is wrong. RULING:
`internal/host` exposes a per-runtime delivery-resume capability
(`host.DeliveryResumer(runtime)` returning nil for runtimes without
one); turnio consumes the capability and never the name.
resumeDevinDelivery's body moves into internal/host beside the collect
code it already calls.

### Class 5 — round-usage recovery assumes the codex stream shape: GENERALIZE

`internal/mission/fence.go:786` calls `usage.CodexUsageValue` for EVERY
provider whose round events exist. Today a claude stream just yields "no
usage block" and normalizes to unavailability — quietly wrong provenance
for a claude round that DOES carry usage in its result object. RULING:
per-runtime usage recovery is declared: `internal/usage` splits into
per-runtime files (usage/devin.go, usage/codex.go — this is item 17's
placement rule applied early where we are already touching the package)
with a dispatch map `usage.RoundValue(runtime, eventsPath)`; fence asks
by provider. Runtimes without a declared recoverer stay honestly
unavailable — same behavior, now stated instead of accidental.

### Class 6 — CLAUDE.md by name in the instruction-asset audit: GENERALIZE

`internal/audit/metasystem.go:30,109` lists CLAUDE.md beside AGENTS.md,
wow.md, SKILL.md, AGENT.md. RULING: the generic names stay literal (they
are runtime-neutral conventions); each runtime's instruction-file name
comes from the Class-2 registry. The audit builds its allowlist as
generic ∪ registry-declared.

### Class 7 — cmd verb tables naming runtimes: STAYS

`cmd/metasystem/main.go:127-139`, `adapter_runtime_verbs.go`,
`adapter_selftest_verbs.go`, `host_verbs.go`: `adapter claude-usage`,
`adapter codex-event`, host collect verbs — thin wiring to seam
functions. The verb names ARE the seam's CLI face; scripts under
scripts/agents/adapters/ call them by these names. RULING: stays, with
one obligation — the verbs' bodies remain one-call-thin routing. Any
decision logic found in them moves into the seam package (none found in
this sweep beyond flag parsing).

### Class 8 — tests naming runtimes: STAYS

47 test files use runtime names as fixture data (roster entries, record
fields, conformance rows). Tests exercise seam-declared behavior; the
names are opaque strings there. RULING: stays. The mechanical fence that
would keep future core code clean (a sweep-check with a comment/test
allowlist) is item 17's scope — it owns placement enforcement — and is
NOT built here.

## Doctrine

docs/architecture.md gains the standing rule: the core never names an
agent runtime in behavior; runtime knowledge lives in the four seams
above as declarations the core consumes; provenance comments and seam
CLI verb names are the two sanctioned appearances of runtime names
outside seam files; adding a runtime touches only its own seam entries
plus one registry declaration.

## Order of moves

1. `internal/runtimes` registry + the five Class-2 consumers + seam
   conformance tests.
2. Class 6 (audit allowlist from the registry) — same commit, same file.
3. Class 3 (hooks marker parameter + suite call site).
4. Class 4 (host delivery-resume capability).
5. Class 5 (usage split + fence dispatch).
6. Doctrine in docs/architecture.md.
7. Full pre-verify, both host gates, goal done.

Each move is behavior-preserving for today's runtimes; the acceptance
question for the critique is whether any ruling misclassifies a site or
leaves a core runtime-conditional standing.
