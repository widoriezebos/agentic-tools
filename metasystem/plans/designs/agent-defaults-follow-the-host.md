# The default agent is whichever one this host has

- Kind: design
- Id: 01M3PETBKECKC9ZV13QQ53GGGT
- Status: accepted

Requested by Wido on 2026-09-29. His rulings: an agent counts as available when its program is on the PATH; the preference order and the models come from configuration and can be overridden locally; proof records the choice that was actually made.

**The decision:** every setting that picks an agent (`role.<role>.runtime`, `mode.<mode>.role.<role>.runtime`, `launch.<lane>.runtime`) accepts the value `auto`, and `auto` becomes the shipped default. `auto` means the first runtime in `metasystem.runtimes`, in the order listed, whose program is on the PATH. The shipped order is `claude,codex,devin`. A setting that names a runtime explicitly always wins, so a seat that wants Devin keeps saying so. Each runtime gets its own default model for every role and lane, in configuration, so switching agents never carries one agent's model to another.

## 1. What is hard-coded today

- Every role and every launch lane defaults to `claude`, whatever is installed (`internal/config/defaults.go`, the `role.*.runtime` and `launch.*.runtime` rows).
- The launch lanes have one model per lane (`launch.build.model=claude-opus-5-5` and so on), which names a Claude model even when the lane runs on another runtime.
- The roles have per-runtime model keys (`role.<role>.model.<runtime>`), but compiled defaults exist only for Claude.
- Adoption rewrites the default runtime to a fixed priority: Codex, then Devin, then Claude (`TailoringPriority` in `internal/runtimes/runtimes.go`, used by `TailorConf`). That order is the reverse of the one Wido wants, and it is fixed once at adoption instead of following the host.
- Two adapter-level fallbacks pick a model or an agent by themselves. The Claude adapter picks a model per kind when the record has none. A lane with no runtime is routed by the model's name prefix (`claude-` means Claude, anything else means Codex).

## 2. Behaviour

**Detection.** A runtime is available when its program (`claude`, `codex`, `devin`) is an executable file found in a directory of `PATH`. The program name comes from the runtime's declaration in `internal/runtimes`, as a new `Executable` field. The fake fixture runtime declares none, so it is never detected and fixtures keep naming it explicitly.

**Preference order.** It is `metasystem.runtimes` in the order listed; the compiled default is `claude,codex,devin`. There is no second list, so the set of runtimes an installation supports and the order it prefers them in can never disagree. It can be overridden like any key: in `metasystem.conf`, in `metasystem.conf.local`, or through `METASYSTEM_METASYSTEM_RUNTIMES`.

**Resolution owner.** `config.Get` resolves `auto`. When the value it resolves for a runtime-selection key is exactly `auto`, it returns the first available runtime in the preference order. It searches the `PATH` of the lookup environment it was given, so a caller with its own environment (a proof, a carried request) gets the answer for that environment. Every current reader already goes through `Get`: launch settings, the delegation roster, conformance's critic check, the critic environment carried into goal worktrees, and the proof configuration digest. So each of them sees a concrete runtime, and none needs its own detection. Within one process the answer is computed once for each distinct PATH and runtime list, so one run cannot see two answers.

**Nothing on the PATH.** Then `auto` resolves to the first runtime listed. That is exactly today's behaviour on a host without agents, keeps the proof digest and `settings show` working there, and the launch still fails loudly at start when the program is missing. I rejected refusing inside `Get`: that would break every settings reader, including the proof digest, on build hosts that have no agent installed.

**Models.** Each runtime has its own default model for every role and lane:

| | Claude | Codex | Devin |
|---|---|---|---|
| Build (`launch.build`, role `implementer`, role `default`) | `claude-opus-5-5` | `gpt-6-sol` | `claude-opus-5-5` (xhigh composed) |
| Design writing (`launch.design`, `mode.design.role.implementer`) | `claude-fable-5-1` | `gpt-6-astra` | `claude-opus-5-5` |
| Code reading (`launch.read`, role `code-critic`) | `claude-fable-5-1` | `gpt-6-astra` | `gpt-6-astra` |
| Design critique (`launch.critique`, role `design-critic`) | `claude-opus-5-5` | `gpt-6-sol` | `gpt-6-astra` |

The author and the reviewer are different models within every runtime. On Devin the delegation roster passes the model id straight to the CLI, so its role defaults carry the effort (`claude-opus-5-5-xhigh`, `gpt-6-astra-xhigh`). Its launch lanes compose the effort themselves, as they do today. `runtime.devin.maximal-models` gets a compiled default naming the two Devin xhigh ids, so design-heavy work is admitted on Devin without a local setting.

- **Launch lanes** gain `launch.<lane>.model.<runtime>`. A lane's model is `launch.<lane>.model` when any layer sets it; that key remains the explicit, runtime-independent override, so today's local settings keep working. Otherwise the lane's model is `launch.<lane>.model.<resolved runtime>`. A lane with neither is refused and the refusal names both keys.
- **Roles** keep their existing keys (`role.<role>.model.<runtime>`, then `role.default.model.<runtime>`) and gain the Codex and Devin rows above.
- **Where the defaults live.** They live in the compiled configuration table (`internal/config/defaults.go`), the one home the 2026-09-28 ruling gives every default. `settings show` names them as `default`, and they can be overridden at every layer. They move out of adapter code: the Claude adapter's per-kind fallback model and the model-prefix routing in `adapterForLane` are removed. A lane with no runtime or no model is refused instead of guessed.

**Adoption.** `TailorConf` stops choosing a default runtime for a real selection. The adopted `metasystem.runtimes` is the selection in the canonical order Claude, Codex, Devin, whatever order the adopter typed. A repository changes its preference by overriding `metasystem.runtimes`. The canonical order is the runtime registry's `TailoringPriority`, which becomes Claude 1, Codex 2, Devin 3 (fake 4); today it is Codex 1, Devin 2, Claude 3. `role.default.runtime` stays `auto`. Any role or lane naming an unselected runtime is rebound to `auto`, and its runtime-independent `launch.<lane>.model` is dropped, so the per-runtime default applies. Model rows of unselected runtimes are dropped, as today. A selection of only the fake runtime keeps today's explicit `fake` binding, because fake is never detected. `TailoringPriority` then serves only that fixture case.

**Validation.** `auto` is a valid runtime value. For a key resolving to `auto`, `settings check` requires a model for every runtime in `metasystem.runtimes`. The check does not depend on the host, so a committed configuration is valid on every machine and not just this one. An explicit runtime keeps today's check.

**Display.** `settings show` prints the resolved runtime and how it was chosen, for example `launch.build.runtime=claude (default; auto: first of claude,codex,devin on PATH)` or `... (default; auto: none on PATH, first listed)`.

## 3. Proof and records

The proof configuration digest already resolves every proof-input key through `Get` with the proof's own environment (`internal/proofrun/attempt.go`, `effectiveProofConfigurationDigest`). It therefore binds the runtime actually chosen: two hosts that pick different agents get different digests, and two hosts that pick the same agent share one. Each run records the concrete choice as today: a launch record's adapter and model, a unit run's request digest (which holds the resolved build and read runtimes), and a dispatch job's runtime and model pair. An explicit setting wins, because `auto` applies only when the resolved value is literally `auto`.

## 4. What changes for this seat

This seat's `metasystem.conf.local` names `devin` explicitly for every role and lane, so nothing changes here. On this Mac, `auto` would pick Claude, because `claude` is on the PATH and listed first. Deleting the local runtime settings would therefore move this seat to Claude, while keeping them keeps Devin.

## 5. Proof

- `internal/config`: `auto` resolves through a lookup environment's PATH, in list order, to the first executable found. With nothing found it falls back to the first listed. An explicit value wins. The answer is memoized per PATH and runtime list. Validation requires a model for every listed runtime under `auto`.
- `internal/launch`: the lane model comes from the per-runtime key, and the runtime-independent key still overrides it. A lane with no model is refused. There is no routing by model prefix.
- `internal/dispatch`: the roster resolves `auto` to a concrete runtime and its per-runtime model.
- `internal/validate`: the tailoring cases above, with updated expectations.
- End to end on this Mac: `settings show` with the local runtime keys removed through an environment override shows `claude (auto)`. With `PATH` narrowed so only `devin` is found, it shows `devin (auto)`, and a real launch runs on `devin-print`.

## 6. Wido's rulings on this page (2026-09-29)

1. **Default models** live in the compiled configuration table, overridable at every layer.
2. **Adoption** is included, and it writes the preference order Claude, Codex, Devin unless configuration overrides it.
3. **Codex defaults:** design writing on Astra, design critique on Sol, build on Sol, code critique on Astra.
