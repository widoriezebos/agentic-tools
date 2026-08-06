# Metasystem inventory: every path, classified

Requested by the human 2026-08-06 after three leaks in two days proved that
"probably fine" is not a classification. Every entry in `metasystem/` carries
exactly one of three verdicts, and each verdict has a deciding rule:

- **SHIPS** — equipment. In the adoption allowlist, installed into every
  project. Deciding rule: would a project that never heard of this repository
  need it? The pollution sweep (grep for our names, models, machines, repo
  paths over every shipped file) ran clean on 2026-08-06.
- **PROJECT-STATE** — tracked here, never ships. This repository's own living
  state as an adopted-shaped repo whose product is the metasystem source.
  Deciding rule: is it about *this* project's ongoing work?
- **RUNTIME** — gitignored, machine-local, owned by the machinery. Deciding
  rule: could it be deleted and regenerated (or is it mirrored elsewhere)?
  History never lingers here: `evidence-gc.sh` collects what the mirror
  already holds.

Anything that fits none of the three belongs beside the metasystem
(`../development/`, `../benchmark/`) or outside the repository entirely.

## Top level (tracked)

| Path | Verdict | Purpose |
| --- | --- | --- |
| `AGENTS.md` | SHIPS | the always-loaded agent contract |
| `CLAUDE.md` | SHIPS | Claude-runtime pointer into the contract |
| `wow.md` | SHIPS | the way-of-working entry document |
| `metasystem.conf` | SHIPS | roster/config template, placeholders only |
| `.gitattributes`, `.gitignore` | SHIPS | line/merge hygiene; artifacts/ ignore |
| `README.md` | tracked, not shipped | describes the template to a repo browser; allowlist excludes it so a project's own README is never touched |
| `metasystem.conf.local` | RUNTIME (gitignored) | THIS machine's development roster and evidence root |
| `plans/` | mixed, see below | |
| `docs/`, `skills/`, `optional-skills/`, `scripts/` | SHIPS, see below | |
| `artifacts/` | RUNTIME, see below | |

## docs/ — all SHIPS

`collaboration.md`, `orchestration.md` (the delegation contract),
`working-modes.md`, `working-with-agents.md`, `project-adaptation.md`,
`metasystem-reconciliation.md` (install/upgrade procedure),
`project-rules.md` (placeholder template; this repo's filled local rules live
in `../development/project-rules-local.md`), `design/` (two design-mode
docs), `examples/` (mission contract, obligation matrix, step-back ledger,
cron example). The Devin runbook was evicted 2026-08-06: machine logistics,
not equipment.

## skills/ and optional-skills/ — all SHIPS

Seven core skills (code-critique, design-critique, improve, refactor, retro,
take-a-step-back, verify), each `SKILL.md` plus per-runtime profiles;
optional-skills carry the same shape for opt-in installs. No file here may
mention this repository.

## plans/ — PROJECT-STATE except one shipped seed

| File | Verdict | Note |
| --- | --- | --- |
| `README.md` | SHIPS | explains what a plans/ directory is for |
| `agent-orchestration-design.md` | PROJECT-STATE | the living master design with the obligation matrix |
| `metasystem-benchmark-design.md` | PROJECT-STATE | benchmark design + trial records |
| `benchmark-spec-bm1-design.md` | PROJECT-STATE | BM-1 case design history |
| `instruction-ledger.md`, `known-issues.md`, `receipts.log` | PROJECT-STATE | ours; adoption ships FRESH empty ones |
| `llm-wiki-pattern.md` | not ours | a peer session's file, deliberately untouched (IL-7); its disposition belongs to that session or the human |

Finished reports were evicted to `../development/` 2026-08-06
(cross-runtime report, triaged watchlist, stale remediation plan).

## scripts/ — all SHIPS

Top level: `adopt.sh` (allowlist payload installer), `audit-metasystem.sh`,
`validate-metasystem.sh` (the gate of record), `validate-skill.sh`,
`metasystem-config.sh` (single config resolution order),
`receipt.sh` (receipts + retro cadence), `frontier.sh` (improve mode),
`refactor-baseline.sh` (refactor mode), `watch-background-jobs.sh`, and the
assertion family: `assert-critique-closed.sh` (close-by-join),
`assert-design-obligation-gate.sh`, `assert-mission.sh`
(validate/seal/preflight), `assert-plan-consistency.sh` (retired-term drift),
`assert-return-complete.sh` (role return schemas), `assert-stop-loss.sh`,
`assert-turn-prompt.sh`.

`scripts/enforcement/` — hook configs per runtime plus the CI workflow
adopt.sh installs.

`scripts/agents/` — the orchestration machinery: `dispatch.sh` (job
lifecycle), `adapters/` (claude, codex, devin, fake + `runtime-common.sh` +
the Claude session signal helper), `hosts/` (mission host adapters: claude,
codex, fake), the mission family (`mission-runner.sh`, `mission-state.py`,
`mission-contract.py`, `mission-ledger.py`, `mission-fence.py`,
`mission-prompt.py`, `mission-fixtures.sh`), supervision
(`arm-supervision.sh`, `process-census.py`, `supervision-hook.sh`,
`supervision-fixtures.sh`, `open-work.py`, `stop-block.py`,
`check-own-hooks.py`), hygiene (`evidence-gc.sh`, `pre-commit-guard.sh`,
`fixture-budget.sh`, `assert-conformance.sh`, `check-preamble-quotes.sh`),
and the role system (`roles/*.md` + `*.requirements.json`, `schemas/*.json`,
`templates/` brief, follow-up and host-turn instruction, `permissions/`
presets).

## artifacts/ — all RUNTIME, live state only

| Dir | Holds | Written by | Cleaned by |
| --- | --- | --- | --- |
| `agents/jobs/` | one JSON record per job round — the registry — plus in-flight logs | dispatch | records kept on purpose (small, they ARE the history join); mirrored logs collected by gc |
| `agents/capabilities/` | one probe snapshot per runtime+version+config fingerprint; only the newest per runtime+version is ever matched | adapter `probe` | gc removes superseded snapshots |
| `agents/hb/` | heartbeat/start/waiting markers for live jobs | adapters | gc removes markers of terminal jobs |
| `agents/record-locks/` | transient locks, lifecycle dirs, mktemp workspace | dispatch | dispatch age-sweeps on entry; gc removes terminal jobs' locks. 142k orphans (557MB) found here 2026-08-06 |
| `agents/locks/`, `agents/worktrees/` | dispatch serialization; job worktrees while live | dispatch | worktrees removed at merge time; empty when idle |
| `agents/mains/` | live session announcements | session hooks | pruned by supervision on session death |
| `agents/supervision/` | owner/watcher/reaper state, census log (rotated), hook log, stop-block state | supervision | live while armed; `--shutdown` |
| `agents/<chain-id>/` | round payloads of a chain IN FLIGHT | dispatch | gc, once terminal + hash-verified against the mirror manifest |

Steady state when idle: a few tens of MB, nothing historical. History lives
at the evidence root outside the repository (browsable via the repo-root
`evidence/` symlink) and is named in `development/evidence-index.md`.
