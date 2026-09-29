# Devin runs on every launch lane

- Kind: design
- Id: 01M3PCDEF9MC6APAJB3VJ17MBG
- Status: accepted

Requested directly by Wido on 2026-09-29: agents must be interchangeable everywhere, so the gap below is fixed now rather than scheduled.

**The decision:** agents are interchangeable on every lane. The launch lanes behind `work build`, `work review` and `design write` (build, read, design and critique) accept `devin` as their runtime, next to `claude` and `codex`. A lane on `devin` starts the Devin CLI in print mode through a new launch adapter, `devin-print`. Its model, effort, resume, measurement, verdict and stray handling match the other two adapters. Only the lane's runtime setting changes, and the rest of the launch machinery stays as it is.

## 1. The gap

`adapterForLane` (`internal/launch/launch.go`) maps runtime `claude` to `claude-headless` and `codex` to `codex-exec`, and names no adapter for anything else. So `launch.<lane>.runtime=devin` makes `Start` refuse with `launch kind "<lane>" is not available`. The delegation roster (`role.*`) already runs Devin through `internal/adapter/supervisor`, so the gap exists only on the launch lanes. A test even records the refusal as intended: `TestDesignAdapterFollowsTheResolvedModel` has the case "an agent this engine cannot launch is refused" with runtime `devin`.

## 2. Observed Devin print-mode contract (CLI 3000.11.3)

Observed by running the CLI on this machine:

- `devin -p --prompt-file FILE --model M --permission-mode dangerous --respect-workspace-trust false --export PATH [-r SESSION]` runs one turn without a person present, prints the final message on stdout, and exits 0.
- An unknown model is refused before any model call: `Error: Unknown model: '<m>'` followed by the available families. The CLI still exits 0 in that case, so the exit code alone cannot prove a turn ran.
- Print mode refuses an untrusted directory unless `--respect-workspace-trust false` is passed. Launch worktrees are never trusted interactively.
- `--export PATH` writes an ATIF JSON object: `session_id`, `agent.model_name` (a display name), and `steps[]`. Every agent step carries `model_name` (the model id, for example `claude-opus-5-5-xhigh`), `tool_calls[]`, and `metrics` with `prompt_tokens` (the whole context, cached part included), `completion_tokens`, `cached_tokens` and `extra.cache_creation_input_tokens`. `final_metrics` is a session total whose cached count was seen going stale, so the adapter adds up the per-step metrics instead.
- `-r SESSION` resumes the session, and the export then holds the whole session. That matches the Claude adapter, which measures the whole session file too.
- Devin puts the effort level in the model id (`claude-opus-5-5-xhigh`, `gpt-6-astra-xhigh`), and the CLI has no separate effort flag.
- Devin has no command-line or environment control for the context window.

## 3. Behaviour

**Lane selection.** `adapterForLane` maps runtime `devin` to `devin-print`. An unknown runtime is still refused, so a runtime nobody wired up can never be guessed.

**Model and effort.** The lane's effort is appended to the model as `<model>-<effort>` unless the model already ends in `-<effort>`. This lets one pair of settings mean the same thing on every runtime: `launch.build.model=claude-opus-5-5` with `launch.build.effort=xhigh` runs `claude-opus-5-5 --effort xhigh` on Claude and `claude-opus-5-5-xhigh` on Devin. An empty effort passes the model through unchanged. A Devin lane with no model is refused when its command is built: Devin has no lane default, and a silent fallback would run a model nobody chose.

**Context window.** A positive `launch.<lane>.window.tokens` on a Devin lane is refused when the command is built, and the refusal names the key to set to 0. A cap the runtime cannot enforce must not be recorded as enforced. The shipped default is 0, meaning no cap, so this refusal only fires for an explicit override.

**Command.** The adapter writes the brief, with the read packet appended exactly as the other adapters do, to `<state>/prompt.md`. Then it runs `devin -p --prompt-file <state>/prompt.md --respect-workspace-trust false --model <model> --permission-mode dangerous --export <state>/transcript.json` in the record's working directory, adding `-r <session>` when the record carries `resumeSession`. Stdout goes to `<state>/result.txt` and stderr to `<state>/stderr.log`. The `dangerous` permission mode matches Claude's `--dangerously-skip-permissions` and the Devin delegation path, because a launch child has no person to approve its tool calls.

**Measurement.** The adapter reads `<state>/transcript.json` and refuses a file over 256 MiB. Transcripts come from the launch's own child, and the read has a ceiling so a runaway file fails loudly and is never truncated. Each agent step that carries metrics counts as one call, and:

- Input is `prompt_tokens` minus cached and cache-creation tokens (never below 0). Cached input, cache creation and output come from their own fields.
- Peak context is the largest `prompt_tokens`, and calls above 200K compare `prompt_tokens`.
- Tool calls are the sum of `tool_calls` over agent steps, and turns are the number of agent steps.
- A compaction is a step whose telemetry source or operation names compaction or summarisation. See section 5.

The patch records `sessionID` and `observedModel` (the last agent step's `model_name`). The result text is stdout. Page, verdict and declared outputs are measured by the same shared helpers the Claude adapter uses.

**Outcome.** A missing or unparseable export, an export with no `session_id`, or an export with no agent step is `result-unreadable`, and the record fails. That covers the unknown-model case, where the CLI exits 0 without exporting a turn. Otherwise a nonzero exit is `exit-N`, and anything else completes.

**Strays.** A process is a stray Devin launch when its argv0 base is `devin`, it has `-p`, and its `--export` path sits directly in a launch state directory under the launch store root, with no running `devin-print` record owning it. It is reported as `stray-devin-print pid=N id=<launch id>`. Like the Claude adapter, strays are reported and never signalled.

## 4. Wiring

`newLaunchManager` (`cmd/metasystem/launch_verbs.go`) registers `devin-print` beside the other two adapters, with the binary `devin`, the launch store root, and the shared process scanner. No setting, schema or record field changes. The lanes already store their runtime, and the adapter name is already free text on the record.

## 5. Unverified, and the risk that follows

- **How a compaction appears in a Devin export was not observed.** Print mode ignored `/compact`, and no session on this machine had compacted. The telemetry match in section 3 is the best available signal. If Devin marks compaction differently, a compacted read would count its verdict. Reads use one-million-token models, so compaction is rare, but this should be confirmed on the first long read and the match corrected if it misses.
- Hooks: a Devin child in a checkout loads the project's `.devin/config.json` hooks, just as a Claude child loads `.claude` settings. The end-to-end run below shows whether a print-mode child finishes normally under them.

## 6. Proof

- Unit tests in `internal/launch`: the Devin lane routes to `devin-print` and an unknown runtime is still refused; the command's argv, prompt file, effort composition, resume and window refusal; measurement from a fixture export (tokens, peak, tool calls, session, observed model, read verdict, compaction); the outcome table, including an export with no agent step; and stray detection.
- End to end: a real `devin-print` launch through the engine on a cheap model, with the record read back as completed and measured.
