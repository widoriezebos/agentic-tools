# Devin, from written-down to proven

- Goal and current status: make Devin a full citizen of the metasystem —
  delegate adapter proven against the real CLI, a host adapter so Devin can run
  mission turns, and a benchmark run with Devin delegates. The delegate adapter
  was written from documentation and had never executed a single verb; that
  changed today, and the first live runs contradict several of its assumptions.
- Next step: critique this design, then implement it
- In flight right now: nothing
- Waiting on the human: nothing blocking

## What the live CLI actually does

Every claim below was observed on this machine against `devin 3000.3.27`,
model `swe-1-7`, in a scratch git repository. They are facts, not readings of
the documentation, and three of them contradict what the adapter assumes.

O-1. A turn runs as `devin -p --prompt-file F --respect-workspace-trust false
--model M --permission-mode MODE --export T`. Exit status 0, the reply on
stdout, and the conversation exported to `T`.

O-2. `--sandbox` FAILS on this account: `session/set_mode failed: Invalid
params: "Mode 'autonomous' is restricted by your organization's policy"`. It
fails alone, with or without `--config`. The adapter passes `--sandbox` on
every dispatch, so no Devin dispatch could ever have run here.

O-3. `--permission-mode autonomous`, which the adapter passes, is not a mode
the CLI offers. The modes are `auto`, `accept-edits`, `smart`, `dangerous`.

O-4. `--config FILE` REPLACES the user configuration rather than layering on
it. A generated file therefore loses the organisation id and the onboarding
marker, and the CLI prints a "Welcome to Devin CLI!" banner into stdout on
first use of each new config file. The real user config is small:
`version`, `devin.org_id`, `shell.setup_complete`, `theme_mode`, `agent.model`.

O-5. The exported transcript is the authoritative record of a turn. It carries
`session_id`, `agent.model_name` (the model that actually answered, e.g.
"SWE-1.7"), `agent.version`, and `final_metrics` with `total_prompt_tokens`,
`total_completion_tokens`, `total_cached_tokens`, `total_steps`.

O-6. Resume works and preserves identity: `-r <session-id>` continues the
session and the next export carries the same `session_id`.

O-7. `devin list --format json` reports sessions for the current directory as
objects with `id`, `short_id`, `working_directory`, `last_activity_at`,
`title`. It is usable while a turn is still running, which the export is not.

O-8. Writes happen under `--permission-mode accept-edits` when the permission
config allows `edit`, `exec`, and `Write(<root>/**)`; with those denied, the
turn ends with exit 0 and empty stdout rather than an error.

## D-1. Permission mode is mapped, and the sandbox is not used

A role with no write roots runs `--permission-mode auto`; a role with write
roots runs `--permission-mode accept-edits`. `dangerous` is never used.

`--sandbox` is not passed. It is the one flag that would enforce the roots at
the OS level, and it is unavailable under this organisation's policy (O-2).
Depending on it would make Devin unusable here, and quietly weakening the
declaration while still passing the flag would be worse. The capability
snapshot therefore declares what is true: roots are mapped into the permission
layer, and that layer — not the operating system — enforces them.

## D-2. The per-turn config layers on the user's, it does not replace it

The generated config starts as a copy of the user's `config.json` and adds the
`permissions` block. This keeps the organisation id and the onboarding marker,
so no banner is printed into a turn's stdout (O-4), and it leaves the user's
own settings intact. The config-identity hash already covers the user file, so
layering keeps identity meaningful rather than hashing a file the run replaced.

## D-3. Session identity comes from the transcript, correlated live by list

During the run the adapter still correlates by `devin list --format json`,
because the export does not exist until the turn ends (O-7). When the turn
ends, the exported `session_id` is authoritative and replaces the correlated
value in the record. A disagreement between the two is a protocol error, not a
preference: the adapter refuses rather than guessing which is the session.

## D-4. Usage is native, and cost is absent rather than estimated

`final_metrics` gives input, output, and cached token counts (O-5), so usage
availability is `native`, not `unavailable`. No per-session cost is reported
and cloud ACUs are not local usage, so `cost` stays null and nothing is
estimated from token counts. `providerUnits` records `total_steps`, which is
the one provider-shaped number the transcript actually reports.

## D-5. The effective model is observed, never assumed

`agent.model_name` names the model that answered (O-5). The adapter records it
as the effective model instead of echoing the requested one, which is the same
rule the other runtimes follow and the reason model telemetry exists.

## D-6. A host adapter, so Devin can run mission turns

`scripts/agents/hosts/devin.sh` implements the same contract as its two
siblings: `start-turn --mission --turn-id --prompt --result [--resume-session]
--instance-tag`, honouring the start gate, and writing one atomic result with
`sessionId`, `outcome`, `usage`, `rawPath`, and `returnPath`.

Devin has no native structured output, so the return arrives the way the
mission prompt already asks for it: the host copies the turn's stdout to the
return path when it parses as the orchestrator return, and otherwise leaves the
return absent so the runner's own validation reports it. Outcomes follow the
existing three-way rule: `failed` on nonzero exit, `unresumable` when the
session is missing or differs from the one being resumed, `completed`
otherwise.

## D-7. The selftest runs here now

`development/devin-selftest.md` was written for "the other laptop" because no
machine here had Devin. One does now. The page keeps its steps but states that
the acceptance run happened, names the CLI version it happened against, and
keeps the capability snapshot as the artifact that travels.

## Proof

- The adapter's own selftest passes end to end against the real CLI: dispatch,
  follow-up on the same session, cancel, permission mapping, and usage.
- A dispatch with no write roots cannot write; one with write roots can.
- The recorded effective model is the transcript's, and a job record carries
  native token counts with a null cost.
- Session identity in the record equals the transcript's `session_id`, and a
  follow-up round records the same one.
- A mission turn hosted by Devin produces a valid return and a resumable
  session; a second turn resumes it.
- The full suite and the kit gate stay green, and a benchmark cohort runs with
  Devin delegates on `swe-1-7`.
