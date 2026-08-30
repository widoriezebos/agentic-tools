# The engine: what the Go binary is and how it is laid out

Status: MAP, not design. This records what exists, assembled from the
package docs (which remain the per-package authority). If this file and
a package doc disagree, the package doc is closer to the code — fix
whichever is wrong.

## What the binary is

The metasystem's decisions live in one Go binary. `go-build.sh` builds
it to `bin/metasystem` (an untracked build artifact, never committed);
every shell entry point resolves it as
`${METASYSTEM_BIN:-<root>/bin/metasystem}`. The CLI is
`metasystem <family> <verb> [flags]`: families group verbs by domain,
`cmd/metasystem/main.go` only routes, and each verb parses its own
flags. Adoption ships the engine as source — `cmd/`, `internal/`,
`go.mod`, `go.sum` ride the payload, `metasystem.engine-delivery=source`
is a required conf key, and the adopted repository builds its own
binary under its own go gate.

## The boundary: core versus plumbing

Standing doctrine (human ruling, Wido, 2026-08-12, recorded in
`records/kill-shell/kill-shell.md`'s header): core functionality belongs in Go;
plumbing — process launching, polling, signaling, environment glue,
fixture drivers — remains in scripts, because that is what scripts are
for. A Go programmer must never read the engine and find a shell script
wearing Go syntax; a script must never make a decision the engine
owns. In practice: `scripts/agents/*.sh` launch, wait, and wire
environments, and call back into engine verbs at every decision point
(`dispatch.sh` is the largest example — the delegate-job choreography
stays shell, every verdict inside it is a verb).

## Runtime agnosticism

The core never names an agent runtime in behavior (human ruling
2026-08-15; agnosticism audit, D74). Runtime knowledge lives in the
sanctioned seams as declarations the core consumes:

- `internal/runtimes` — the ONE pure-data registry: names, priorities,
  adoption shape, session environments, instruction files, hook
  capabilities, enforcement expectations, permission residuals,
  expected behavioral capabilities. Shell consumes it through the
  `metasystem runtime` verbs, never by parsing.
- `internal/adapter`, `internal/host`, and `internal/usage`'s
  per-runtime files — behavioral seams. Behavioral capabilities
  (delivery recollection, usage recovery, self-test probes) register
  seam-locally into their owner package's typed table; the registry
  only declares what is EXPECTED, and a conformance test joins the two
  both ways.
- `scripts/agents/adapters/*.sh` (with their runtime-owned JSON
  assets), `scripts/agents/hosts/*.sh`, per-skill runtime profiles,
  and `scripts/enforcement/<runtime>-*.json` — the shell seams.

Sanctioned appearances of runtime names outside seam files: (a)
provenance comments naming the critic or incident behind a decision,
(b) the adapter/host families' CLI verb names, (c) schema-defined runtime
positions in checkout configuration — operator-selected values AND the
runtime-bearing key segments the configuration schema defines
(model-key suffixes, capability-floor segments, tier prefixes) — each
validated against the registry,
(d) the named `fake` test-harness exceptions (each fake-gated branch
keeps its explicit local guard; no generic fixture bypass exists), and
(e) the handwritten conformance-evidence rows in
docs/design/turn-verdict-delivery-contract.md.

Runtime-native capabilities are sanctioned ACCELERATORS (human
ruling 2026-08-16): an operating agent may use whatever its own
harness offers — mid-session wakeups, background-task notifications,
push channels — to act sooner, and should, rather than letting
available functionality lie unused. The boundary is that correctness
never depends on them: every outcome an accelerator surfaces must
also land in metasystem records (runs, jobs, the turn verdict), so an
agent on any runtime, with no accelerator at all, continues from the
records alone.

Adding a runtime touches its seam entries plus one registry
declaration — with two declared exceptions: granting a new runtime's
permission-residual waiver is a HUMAN edit to the role requirements
files (the live, checkout-local security control; a runtime with an
undeclared residual fails closed), and the delivery-contract evidence
row is handwritten prose the audit cross-checks. The
adoption/registration/installation contract is being generalized under
goal runtime-integration-contracts (records/agnosticism/agnosticism-audit-rulings.md
carries the split).

## Layering

Three tiers, imports point strictly downward:

1. **Foundations** — packages with no metasystem imports:
   `atomicfile`, `boundedexec`, `identity`, `wiredoc`, `jsonedit`,
   `lock`.
2. **Domain packages** — the decision owners under `internal/`,
   importing foundations and each other sparingly. `gaterun` is in this tier:
   weight transactions compose `atomicfile`, `behaviorsurface`, and
   `identity`.
3. **`cmd/metasystem`** — flag parsing and routing only; one file per
   verb family; no logic worth testing lives here.

Shell sits above all three and below none: scripts call verbs, verbs
never call scripts (the two exceptions are deliberate: `boundedexec`
runs adapter/gate commands the caller names, and the mission runner
launches host adapters — both run programs handed to them, neither
encodes shell knowledge).

## Package map

One line per package; the package doc is the full story.

| Package | Owns |
| --- | --- |
| `adapter` | shared runtime-adapter lifecycle plumbing plus the per-runtime decision helpers (claude/codex/devin command construction, event reads, result derivation) |
| `atomicfile` | atomic, durable file replacement |
| `audit` | the shipped instruction-asset audit and the development-time coverage ratchet |
| `authority` | the control-plane authority matrix: may this classified caller write in this mode |
| `boundedexec` | running external commands under a time bound |
| `behaviorsurface` | the versioned ENGINE, LANDING, and PAYLOAD byte projections plus delivery-contract skip set |
| `capability` | selecting and validating the capability snapshot for one dispatch identity |
| `census` | classifying the machine's processes: announced, custody, or untracked |
| `config` | conf reading at three depths: hot-path ConfValue, layered Get, domain Validate |
| `contract` | mission-contract grammar, sealing, and launch preflight |
| `dispatch` | the delegate-job control plane: record CAS spine, attestation, envelopes, mission proof, mirroring, close, critique policy, owner lock, briefs, usage |
| `events` | the flight-recorder emitter |
| `evidence` | the durable-evidence collector: mirrored chains, residue pruning, archive aging |
| `gaterun` | gate-run markers plus the direct-validation weight accumulator and authorized discharge boundary |
| `hooks` | self-check that the repo runs under its own metasystem |
| `host` | per-turn host work around one CLI invocation: envelopes, usage, return extraction |
| `identity` | provable process identity: pid plus start time, never claims |
| `janitor` | the machine-wide sweep that closes dead claims |
| `jsonedit` | shell-facing JSON verb decisions |
| `lease` | checkout write-authority: announce, classify, hold, renew, sweep |
| `lock` | the supervision registry's acquisition discipline |
| `mission` | mission lifecycle decisions: ledger, fences, state, ask/answer |
| `missionrunner` | the engine that launches and drives mission cycles |
| `receipt` | the task-receipt ledger and retro cadence |
| `registry` | the machine-wide supervision registry contract |
| `report` | turn-end report decisions plus the improvement-mode frontier ledger |
| `returnschema` | versioned role-return schema materialization |
| `stateroot` | mode-aware directories for application state, derived from the installed engine and repository |
| `supervise` | the supervision owner lifecycle: watcher, reaper, breaker, wind-down |
| `turn` | the vocabulary of a mission turn |
| `up` | the session-start arming transaction and its typed aggregate outcome |
| `usage` | typed usage extraction, the single owner |
| `validate` | whole-artifact validators and rewrites (incl. conf tailoring) |
| `wiredoc` | the mechanism of typed on-disk documents |

## Family-to-package table

The `metasystem` usage text is the authority for what each family says
it does; this table adds where the decisions live.

| Family | Backing packages |
| --- | --- |
| `up` | `up`, coordinating `lease`, `supervise`, and `steward` |
| `proc` | `identity`, `census` |
| `config` | `config`, `validate` (tailor) |
| `validate` | `validate` |
| `job` | `dispatch`, `authority`, `capability`, `census` |
| `adapter` | `adapter`, `usage`, `config` |
| `host` | `host`, `usage` |
| `audit` | `audit` |
| `gate` | `gaterun` |
| `behavior-surface` | `behaviorsurface` |
| `report` | `report` |
| `receipt` | `receipt` |
| `schema` | `returnschema` |
| `hooks` | `hooks` |
| `util` | small helpers (`atomicfile`, token/time utilities) |
| `event` | `events` |
| `json` | `jsonedit` |
| `lease` | `lease` |
| `mission` | `mission`, `missionrunner`, `contract`, `config` |
| `evidence` | `evidence` |
| `supervise` | `supervise`, `registry`, `lock`, `identity`, `census`, `dispatch` |

## Where the sequences are documented

Choreography that stays shell has ground-truth sequence maps in
`docs/design/`: `mission-cycle-sequence.md` for the mission path and
`dispatch-sequence.md` for the delegate-job path. Standing behavioral
contracts (wire documents, supervision registry and lifecycle, flight
recorder, stop-loss core) also live in `docs/design/` — `plans/` holds
task-local designs and history, never policy.
