# Kill-shell fact pass

## Q1 — script caller graph

**F Q1.1.** The suite invokes the gate and process-owning fixture scripts in this order: `go-gate.sh`, `supervision-go-fixtures.sh`, then—outside delegate scope—`supervision-fixtures.sh` and `fingerprint-harness.sh`, followed by `mission-fixtures.sh` in both scopes.

> `scripts/validate-metasystem.sh:72:  bash scripts/agents/go-gate.sh`  
> `scripts/validate-metasystem.sh:78:  bash scripts/agents/supervision-go-fixtures.sh`  
> `scripts/validate-metasystem.sh:243:    scripts/agents/supervision-fixtures.sh`  
> `scripts/validate-metasystem.sh:246:    scripts/agents/fingerprint-harness.sh --iterations 2`  
> `scripts/validate-metasystem.sh:248:  scripts/agents/mission-fixtures.sh`

**F Q1.2.** The suite spawns, rather than sources, the remaining standalone fixture scripts: `conformance-fixtures.sh`, `telemetry-census-fixtures.sh`, `return-schema-fixtures.sh`, `config-identity-fixtures.sh`, `authority-regression-fixtures.sh`, `pre-commit-guard-fixtures.sh`, `record-protocol-fixtures.sh`, `evidence-segment-fixtures.sh`, `second-session-fixtures.sh`, `lease-succession-fixtures.sh`, `flight-recorder-fixtures.sh`, and `delegate-caps-fixtures.sh`.

> `scripts/validate-metasystem.sh:282:bash scripts/agents/conformance-fixtures.sh`  
> `scripts/validate-metasystem.sh:283:bash scripts/agents/telemetry-census-fixtures.sh`  
> `scripts/validate-metasystem.sh:284:bash scripts/agents/return-schema-fixtures.sh`  
> `scripts/validate-metasystem.sh:285:bash scripts/agents/config-identity-fixtures.sh`  
> `scripts/validate-metasystem.sh:292:bash scripts/agents/authority-regression-fixtures.sh`  
> `scripts/validate-metasystem.sh:293:bash scripts/agents/pre-commit-guard-fixtures.sh`  
> `scripts/validate-metasystem.sh:294:bash scripts/agents/record-protocol-fixtures.sh`  
> `scripts/validate-metasystem.sh:295:bash scripts/agents/evidence-segment-fixtures.sh`  
> `scripts/validate-metasystem.sh:296:bash scripts/agents/second-session-fixtures.sh`  
> `scripts/validate-metasystem.sh:297:bash scripts/agents/lease-succession-fixtures.sh`  
> `scripts/validate-metasystem.sh:298:bash scripts/agents/flight-recorder-fixtures.sh`  
> `scripts/validate-metasystem.sh:299:bash scripts/agents/delegate-caps-fixtures.sh`

**F Q1.3.** The suite invokes the real adapters through dynamic help checks, directly invokes the fake adapter’s probe, and checks host wrappers. `dispatch.sh` dynamically selects the runtime adapter.

> `scripts/validate-metasystem.sh:317:for runtime in claude codex devin; do`  
> `scripts/validate-metasystem.sh:318:  adapter="scripts/agents/adapters/$runtime.sh"`  
> `scripts/validate-metasystem.sh:322:  adapter_usage=$($adapter --help 2>&1)`  
> `scripts/validate-metasystem.sh:475:fake_adapter="$fake_probe_root/scripts/agents/adapters/fake.sh"`  
> `scripts/validate-metasystem.sh:481:fake_probe_result=$($fake_adapter probe)`  
> `scripts/validate-metasystem.sh:349:for runtime in claude codex fake; do`  
> `scripts/validate-metasystem.sh:350:  host="scripts/agents/hosts/$runtime.sh"`  
> `scripts/agents/dispatch.sh:558:adapter="$root/scripts/agents/adapters/$runtime.sh"`  
> `scripts/agents/dispatch.sh:586:  "$adapter" "$verb" --job "$job" --start-gate "$start_gate" \`

**F Q1.4.** Go code invokes adapters for census signature work and invokes runtime hosts for mission turns.

> `internal/census/fingerprint.go:47:		out, err := exec.Command(adapter, "signature").CombinedOutput()`  
> `internal/census/run.go:521:	cmd := exec.Command(adapter, "signature")`  
> `internal/missionrunner/host.go:169:	hostPath := filepath.Join(e.Root, "scripts", "agents", "hosts", runtime+".sh")`  
> `internal/missionrunner/host.go:192:	cmd := exec.Command(hostPath, args...)`

**F Q1.5.** `runtime-common.sh` is sourced by the Claude, Codex, and Devin adapters. The Codex host sources the Codex adapter; telemetry fixtures source the Claude adapter; fake selftest and process fixtures source `fixture-budget.sh`.

> `scripts/agents/adapters/claude.sh:21:source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/runtime-common.sh"`  
> `scripts/agents/adapters/codex.sh:21:source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/runtime-common.sh"`  
> `scripts/agents/adapters/devin.sh:21:source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/runtime-common.sh"`  
> `scripts/agents/hosts/codex.sh:9:source "$root/scripts/agents/adapters/codex.sh"`  
> `scripts/agents/telemetry-census-fixtures.sh:10:source "$root/scripts/agents/adapters/claude.sh"`  
> `scripts/agents/adapters/fake.sh:247:  . "$root/scripts/agents/fixture-budget.sh"`  
> `scripts/validate-metasystem.sh:103:source scripts/agents/fixture-budget.sh`

**F Q1.6.** `arm-supervision.sh` is called by `supervision-hook.sh`, `second-session.sh`, mission-runner Go code, and supervision fixtures. Go mission preflight calls its `fingerprint` verb.

> `scripts/agents/supervision-hook.sh:25:arm=$script_dir/arm-supervision.sh`  
> `scripts/agents/second-session.sh:40:"$root/scripts/agents/arm-supervision.sh" --repo "$root" >/dev/null`  
> `internal/missionrunner/launch.go:287:		filepath.Join(e.Root, "scripts", "agents", "arm-supervision.sh"), args...)`  
> `internal/mission/contract.go:1374:	script := filepath.Join(projectRoot, "scripts", "agents", "arm-supervision.sh")`  
> `internal/mission/contract.go:1375:	command := exec.Command(script, "fingerprint", "--repo", projectRoot)`  
> `scripts/agents/supervision-fixtures.sh:263:arm="$repo/scripts/agents/arm-supervision.sh"`

**F Q1.7.** `dispatch.sh` is prescribed by the root role contract, called by Go mission-runner loop/drain code and fixture scripts, and retained as a callback path by `runtime-common.sh` and `fake.sh`.

> `AGENTS.md:23:- When the runtime provides subagents, delegate independent exploration and verifiable subtasks. Keep the main context for decisions. Dispatch rostered roles through \`scripts/agents/dispatch.sh\`; if exact-session resume is unavailable, use the documented fresh-dispatch embed fallback (\`docs/orchestration.md\`).`  
> `internal/missionrunner/loop.go:744:	dispatch := filepath.Join(e.Root, "scripts", "agents", "dispatch.sh")`  
> `internal/missionrunner/drain.go:56:	dispatchScript := filepath.Join(e.Root, "scripts", "agents", "dispatch.sh")`  
> `scripts/agents/adapters/runtime-common.sh:10:  dispatch="$root/scripts/agents/dispatch.sh"`  
> `scripts/agents/adapters/fake.sh:32:dispatch="$root/scripts/agents/dispatch.sh"`

**F Q1.8.** `emit-event.sh` is sourced by `dispatch.sh` and `arm-supervision.sh`; `evidence-gc.sh` is run by `supervision-hook.sh`.

> `scripts/agents/dispatch.sh:44:  source "$flight_lib"`  
> `scripts/agents/arm-supervision.sh:145:  source "$flight_lib"`  
> `scripts/agents/supervision-hook.sh:166:  "$script_dir/evidence-gc.sh" >>"$supervision_dir/hooks.log" 2>&1 || true`

**F Q1.9.** `assert-conformance.sh` is called by the code-critique skill and conformance fixtures. `assert-return-complete.sh` is called by runtime glue, the fake adapter, and Go mission code. `assert-turn-prompt.sh` and `assert-mission.sh` are called by Go mission-runner code.

> `skills/code-critique/SKILL.md:29:For a dispatched implementation, run \`scripts/agents/assert-conformance.sh --stage review --job <job-id>\`.`  
> `scripts/agents/conformance-fixtures.sh:310:"$controller/scripts/agents/assert-conformance.sh" --stage merge --job impl >/dev/null`  
> `scripts/agents/adapters/runtime-common.sh:198:  "$root/scripts/assert-return-complete.sh" --job "$job" >"$violation" 2>&1`  
> `scripts/agents/adapters/fake.sh:116:  if "$root/scripts/assert-return-complete.sh" --job "$job" >"$violation" 2>&1; then`  
> `internal/mission/landed.go:147:	cmd := exec.Command(filepath.Join(repo, "scripts", "assert-return-complete.sh"), "--job", jobID)`  
> `internal/missionrunner/loop.go:958:		filepath.Join(e.Root, "scripts", "assert-turn-prompt.sh"),`  
> `internal/missionrunner/answer.go:84:			filepath.Join(e.Root, "scripts", "assert-mission.sh"),`

**F Q1.10.** `commit.sh` is executed by mission anchoring; `second-session.sh` is directed by lease classification and the supervision hook; `pre-commit-guard.sh` is executed by the repository Git hook and by the hook installed into adopted targets.

> `internal/mission/anchor.go:209:	wrapper := filepath.Join(repo, "scripts", "agents", "commit.sh")`  
> `internal/mission/anchor.go:217:	cmd := exec.Command(wrapper, args...)`  
> `internal/lease/verbs.go:302:		return fmt.Errorf("OWNED-ELSEWHERE: this checkout is held by %s; use scripts/agents/second-session.sh for an isolated writer", lease.HolderMainId)`  
> `scripts/agents/supervision-hook.sh:124:    advisor_message="OWNED-ELSEWHERE: this main is a read-only advisor in this checkout. To write independently, run scripts/agents/second-session.sh."`  
> `/Users/wido/LocalStorage/GitHub/agentic-tools/.git/hooks/pre-commit:2:guard="$(git rev-parse --show-toplevel)/metasystem/scripts/agents/pre-commit-guard.sh"`  
> `/Users/wido/LocalStorage/GitHub/agentic-tools/.git/hooks/pre-commit:3:[[ -x "$guard" ]] && exec "$guard"`  
> `scripts/adopt.sh:325:    printf '#!/usr/bin/env bash\nguard="$(git rev-parse --show-toplevel)/scripts/agents/pre-commit-guard.sh"\n[[ -x "$guard" ]] && exec "$guard"\nexit 0\n' >"$hook_dir/pre-commit"`

**F Q1.11.** Each enforcement configuration invokes `supervision-hook.sh` for SessionStart, Stop, and SessionEnd.

> `scripts/enforcement/claude-code-hooks.json:10:            "command": "\"$(git rev-parse --show-toplevel)/scripts/agents/supervision-hook.sh\" claude session-start"`  
> `scripts/enforcement/claude-code-hooks.json:21:            "command": "\"$(git rev-parse --show-toplevel)/scripts/agents/supervision-hook.sh\" claude stop"`  
> `scripts/enforcement/claude-code-hooks.json:36:            "command": "\"$(git rev-parse --show-toplevel)/scripts/agents/supervision-hook.sh\" claude session-end"`  
> `scripts/enforcement/codex-hooks.json:10:            "command": "\"$(git rev-parse --show-toplevel)/scripts/agents/supervision-hook.sh\" codex session-start"`  
> `scripts/enforcement/codex-hooks.json:21:            "command": "\"$(git rev-parse --show-toplevel)/scripts/agents/supervision-hook.sh\" codex stop"`  
> `scripts/enforcement/codex-hooks.json:32:            "command": "\"$(git rev-parse --show-toplevel)/scripts/agents/supervision-hook.sh\" codex session-end"`  
> `scripts/enforcement/devin-hooks.json:8:          "command": "\"$(git rev-parse --show-toplevel)/scripts/agents/supervision-hook.sh\" devin session-start"`  
> `scripts/enforcement/devin-hooks.json:19:          "command": "\"$(git rev-parse --show-toplevel)/scripts/agents/supervision-hook.sh\" devin stop"`  
> `scripts/enforcement/devin-hooks.json:30:          "command": "\"$(git rev-parse --show-toplevel)/scripts/agents/supervision-hook.sh\" devin session-end"`

**F Q1.12.** Policy-facing callers are: `assert-critique-closed.sh` from critique guidance; `assert-design-obligation-gate.sh` from the completion gate; `assert-stop-loss.sh` from take-a-step-back/improve; `frontier.sh` from improve; `refactor-baseline.sh` from refactor; `receipt.sh` from the orchestrator and retro contracts; `audit-metasystem.sh` and `validate-skill.sh` from the suite.

> `skills/code-critique/SKILL.md:58:Use \`accepted\` or \`refuted\` for material findings. Use \`noted\` only for non-material findings. Close the round by running \`scripts/assert-critique-closed.sh --findings <return.json> --dispositions <file>\`; a count or prose claim is not closure.`  
> `docs/design/design-obligation-gate.md:50:Run \`scripts/assert-design-obligation-gate.sh --file <matrix>\`.`  
> `skills/take-a-step-back/SKILL.md:66:Record every cycle and its classification in the ledger, and run \`scripts/assert-stop-loss.sh --file <ledger>\` before contracting a new cycle.`  
> `skills/improve/SKILL.md:28:- The frontier is the best-known state: exact SHA, score, evaluation command, and run artifact. Manage it with \`scripts/frontier.sh\` (record, challenge, status).`  
> `skills/refactor/SKILL.md:14:- Before every new edit batch, run \`scripts/refactor-baseline.sh check\`.`  
> `scripts/validate-metasystem.sh:120:scripts/audit-metasystem.sh .`  
> `scripts/validate-metasystem.sh:133:    scripts/validate-skill.sh "$(dirname "$skill_md")"`

**F Q1.13.** `assert-plan-consistency.sh` and `check-preamble-quotes.sh` have suite callers only. `mission-runner.sh` has fixture and documented scheduler callers. `watch-background-jobs.sh` has arm-supervision, fixture, and inline-suite callers.

> `scripts/validate-metasystem.sh:1272:scripts/assert-plan-consistency.sh --root .`  
> `scripts/validate-metasystem.sh:792:if scripts/agents/check-preamble-quotes.sh --root . >"$quote_out" 2>"$quote_err"; then`  
> `scripts/agents/mission-fixtures.sh:556:"$runner" start --contract plans/mission-runner-success.contract.md --mission runner-success --foreground >/dev/null`  
> `docs/examples/mission-cron.example:7:*/5 * * * * cd /path/to/repository && scripts/agents/mission-runner.sh resume --mission nightly-improvement >>artifacts/agents/missions/nightly-improvement/cron.log 2>&1`  
> `scripts/agents/arm-supervision.sh:36:watcher="$root/scripts/watch-background-jobs.sh"`  
> `scripts/validate-metasystem.sh:4256:scripts/watch-background-jobs.sh --dir "$wbj/live-log/jobs" --state "$wbj/s3b" --stale-min 5 --once >"$wbj/o4b" 2>&1`

**F Q1.14.** `adopt.sh` is called from documented adoption procedures and from its template-only selftest. `metasystem-config.sh` is called by `dispatch.sh`, adapters, supervision fixtures, `adopt.sh`, and the suite.

> `README.md:214:1. From the template checkout, run \`scripts/adopt.sh <target> [--runtimes claude,devin,codex] [--enable debug-java]\`.`  
> `scripts/validate-metasystem.sh:3888:  adopt="$root/scripts/adopt.sh"`  
> `scripts/adopt.sh:197:  "$staging/bin/metasystem" config tailor --file "$staging/metasystem.conf" --runtimes "$runtime_csv"`  
> `scripts/agents/dispatch.sh:364:  "$root/scripts/metasystem-config.sh" get --key "$1" --default "${2:-}"`  
> `scripts/agents/adapters/codex.sh:226:    selftest_model=$("$root/scripts/metasystem-config.sh" get \`  
> `scripts/agents/supervision-fixtures.sh:108:  cp "$source_root/scripts/metasystem-config.sh" "$repo/scripts/"`  
> `scripts/validate-metasystem.sh:149:  scripts/metasystem-config.sh validate`

**F Q1.15.** The optional Java `preflight.sh` is called by its skill and by the suite when present.

> `optional-skills/debug-java/SKILL.md:26:Use \`scripts/preflight.sh\` for local process/artifact checks when configured.`  
> `scripts/validate-metasystem.sh:3393:for preflight in optional-skills/debug-java/scripts/preflight.sh skills/debug-java/scripts/preflight.sh; do`  
> `scripts/validate-metasystem.sh:3396:    "$preflight" --source "$tmp/source" --artifact "$tmp/artifact" >/dev/null`

**F Q1.16.** No tracked `*.sh` has “NO caller found.” Fixture-only scripts are called by F Q1.1–Q1.2; adapters/hosts/runtime glue by F Q1.3–Q1.5; supervision/control scripts by F Q1.6–Q1.11; and policy/suite scripts by F Q1.12–Q1.15.

> `scripts/validate-metasystem.sh:185:for link in \`  
> `scripts/validate-metasystem.sh:195:  scripts/agents/dispatch.sh \`  
> `scripts/validate-metasystem.sh:215:  scripts/agents/mission-runner.sh \`  
> `scripts/validate-metasystem.sh:226:  scripts/agents/assert-conformance.sh \`  
> `scripts/validate-metasystem.sh:232:  scripts/agents/check-preamble-quotes.sh; do`

**F Q1.17.** The premise that `adopt.sh` has a per-script shipped-file list is false. It allowlists the top-level `scripts` directory and retains every tracked path below it. `plans/` is separately reduced to `plans/README.md`. Thus every tracked shell below `scripts/` is part of the ordinary adopted on-disk contract.

> `scripts/adopt.sh:153:payload_allow=(.gitattributes .gitignore AGENTS.md CLAUDE.md docs metasystem.conf optional-skills plans scripts skills wow.md)`  
> `scripts/adopt.sh:154:for top in "${payload_allow[@]}"; do`  
> `scripts/adopt.sh:156:    "$top"|"$top"/*) retained+=("$file"); keep=1; break ;;`  
> `scripts/adopt.sh:162:# Projects own plans. Ship only the directory contract, never template workstreams.`  
> `scripts/adopt.sh:168:  [[ "$file" == plans/README.md ]] || unset 'keep_map[$file]'`  
> `scripts/adopt.sh:214:for file in "${staged_files[@]}"; do`  
> `scripts/adopt.sh:219:  mv "$staging/$file" "$target/$file"`

**F Q1.18.** The optional `debug-java` shell is not in a default adopted target. Enabling that optional skill moves the complete staged directory from `optional-skills/debug-java` to `skills/debug-java`, yielding `skills/debug-java/scripts/preflight.sh`.

> `scripts/adopt.sh:185:for skill in "${!enable_set[@]}"; do`  
> `scripts/adopt.sh:187:  mkdir -p "$staging/skills"`  
> `scripts/adopt.sh:188:  mv "$staging/optional-skills/$skill" "$staging/skills/$skill"`

**F Q1.19.** `validate-metasystem.sh` is invoked by the shipped GitHub Actions workflow and by adoption/reconciliation instructions; `adopt.sh` prints it as the target’s required next step.

> `scripts/enforcement/github-actions-metasystem.yml:15:        run: scripts/validate-metasystem.sh`  
> `README.md:216:3. Run \`scripts/validate-metasystem.sh\` in the target; it must pass with zero placeholders.`  
> `scripts/adopt.sh:333:echo "  3. Run scripts/validate-metasystem.sh in the target; it must pass with zero placeholders."`

## Q2 — `dispatch.sh` section map

**F Q2.1.** Lines 1–72 are usage, root/global setup, and optional flight-recorder loading. Bash decides whether event emission is available.

> `scripts/agents/dispatch.sh:43:if [[ -f "$flight_lib" ]]; then`  
> `scripts/agents/dispatch.sh:44:  source "$flight_lib"`  
> `scripts/agents/dispatch.sh:62:runtime=''`  
> `scripts/agents/dispatch.sh:72:cap_resolution_file=''`

**F Q2.2.** Lines 73–117 contain ID/time helpers, open-work and census-freshness consultation, and JSON access. They call `util token-hex`, `report open-work`, `dispatch census-fresh`, `arm-supervision.sh fingerprint`, and `json get`; Bash selects fallbacks and passes the current fingerprint.

> `scripts/agents/dispatch.sh:76:  "$ms" util token-hex --bytes 4`  
> `scripts/agents/dispatch.sh:94:  "$ms" report open-work --repo "$root"`  
> `scripts/agents/dispatch.sh:105:  "$ms" dispatch census-fresh --last "$last" --state "$state" --fingerprint "$current"`  
> `scripts/agents/dispatch.sh:115:  "$ms" json get "$@"`

**F Q2.3.** Lines 118–158 are record CAS/create/setup choreography. Go verbs are `dispatch record-cas`, `record-create`, and `record-setup`; Bash interprets exit codes, re-execs under a record lock, emits events, removes patches, and prepares failure patches.

> `scripts/agents/dispatch.sh:121:  "$ms" dispatch record-cas --job "$job_file" --expect "$expected" --status "$status" --patch "$patch"`  
> `scripts/agents/dispatch.sh:134:  "$ms" dispatch record-create --job "$job_file" --setup "$setup_file"`  
> `scripts/agents/dispatch.sh:143:  if "$ms" dispatch record-setup --job "$job_file" --record "$complete_file"; then`

**F Q2.4.** Lines 160–191 are lease and authority checks. Go verbs are `lease require-holder`, `run-held`, `classify`, `json get`, and `authority check`; Bash captures holder identity/epoch and selects the wrapped command.

> `scripts/agents/dispatch.sh:162:  "$ms" lease require-holder --repo "$root" --operation "$operation"`  
> `scripts/agents/dispatch.sh:174:  "$ms" lease run-held --repo "$root" --operation "$operation" -- "$@"`  
> `scripts/agents/dispatch.sh:182:  caller_json=$("$ms" lease classify --repo "$root") || return $?`  
> `scripts/agents/dispatch.sh:187:  "$ms" authority check --operation "$operation" --mode "$mode" --caller-json "$caller_json" \`

**F Q2.5.** Lines 193–277 are exact-process/group liveness and wind-down. Go supplies `identity exists` and `identity group-exists`; Bash also uses `kill -0`, `ps`/`awk` PGID ownership, fake proof/heartbeat files, TERM/KILL, polling, and `wait`.

> `scripts/agents/dispatch.sh:196:  "$ms" identity exists --pid "$pid"`  
> `scripts/agents/dispatch.sh:226:  "$ms" identity group-exists --pgid "$pgid"`  
> `scripts/agents/dispatch.sh:244:  ps -axo pid=,pgid= | awk -v target="$pgid" -v self="$$" '$2 == target && $1 != self { found=1 } END { exit(found ? 0 : 1) }'`  
> `scripts/agents/dispatch.sh:258:  kill -TERM -- "-$pgid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true`  
> `scripts/agents/dispatch.sh:269:    kill -KILL -- "-$pgid" 2>/dev/null || kill -KILL "$pid" 2>/dev/null || true`

**F Q2.6.** Lines 279–313 are the owner primitive and chain lock. They call `dispatch owner-lock`; Bash creates/removes lock directories, writes owner state, retries, and reports contention.

> `scripts/agents/dispatch.sh:283:  "$ms" dispatch owner-lock --mode "$mode" --lock "$lock_dir" --owner "$$" --output "$owner_file"`  
> `scripts/agents/dispatch.sh:297:  if mkdir "$lock_dir" 2>/dev/null; then`  
> `scripts/agents/dispatch.sh:311:  rm -rf "$lock_dir"`

**F Q2.7.** Lines 315–337 are the lifecycle lock using `dispatch owner-lock` plus Bash timeout/poll/release. Lines 339–360 are the cap-authority lock entirely in Bash; no Go verb is called there.

> `scripts/agents/dispatch.sh:321:    if owner_lock_primitive claim "$lifecycle_lock_dir" "$lifecycle_owner_file"; then`  
> `scripts/agents/dispatch.sh:326:    (( SECONDS < deadline )) || { echo "timed out acquiring lifecycle lock" >&2; return 1; }`  
> `scripts/agents/dispatch.sh:344:    if mkdir "$cap_authority_lock_dir" 2>/dev/null; then`  
> `scripts/agents/dispatch.sh:357:  rmdir "$cap_authority_lock_dir" 2>/dev/null || true`

**F Q2.8.** Lines 362–429 resolve configuration and caps. They call the config shim, `config canonical-model`, `config conf-value`, `dispatch cap-resolution`, and `dispatch watcher-ceiling`. Bash applies precedence, validates positive integers, records origin, and enforces the ceiling.

> `scripts/agents/dispatch.sh:364:  "$root/scripts/metasystem-config.sh" get --key "$1" --default "${2:-}"`  
> `scripts/agents/dispatch.sh:379:  canonical_model=$("$ms" config canonical-model --model "$model")`  
> `scripts/agents/dispatch.sh:382:  if value=$("$ms" config conf-value --file "$root/metasystem.conf" --key "cap.role.$role.min"); then`  
> `scripts/agents/dispatch.sh:415:  "$ms" dispatch cap-resolution --output "$cap_resolution_file" --role "$role" --model "$canonical_model" \`  
> `scripts/agents/dispatch.sh:424:  ceiling=$("$ms" dispatch watcher-ceiling --root "$root" --jobs-dir "$jobs_dir") || return $?`

**F Q2.9.** Lines 431–540 validate runtime, mode, tier/escalation, and mission context. Go supplies `config get/keys`, `dispatch brief-mode`, `mission-contract envelope-allows`, and `dispatch validate-mission`; Bash validates runtime membership and tier contiguity, selects escalation, prompts for approval, and rejects ambiguous mission environment.

> `scripts/agents/dispatch.sh:438:  runtimes=$(config_get metasystem.runtimes '')`  
> `scripts/agents/dispatch.sh:465:  "$ms" dispatch brief-mode --brief "$brief" --modes "$modes"`  
> `scripts/agents/dispatch.sh:492:    "$ms" mission-contract envelope-allows --file "$mission_contract" --kind pair --value "$candidate_runtime:$candidate_model" >/dev/null 2>&1`  
> `scripts/agents/dispatch.sh:518:  read -r -p "Approve escalation for $role to $runtime/$model? [y/N] " answer`  
> `scripts/agents/dispatch.sh:539:  "$ms" dispatch validate-mission --root "$root" --mission "$mission" --contract "$mission_contract"`

**F Q2.10.** Lines 542–570 resolve permissions, capability snapshot, adapter identity, and root job. Go verbs are `dispatch expand-permissions`, `capability select`, adapter `config-identity`, and adapter `root-job`; Bash selects presets, applies the network floor, and supplies snapshot age.

> `scripts/agents/dispatch.sh:545:  "$ms" dispatch expand-permissions --preset "$preset" --workspace "$workspace" --output "$output"`  
> `scripts/agents/dispatch.sh:554:  "$ms" capability select --root "$root" --runtime "$runtime" --version "$version" --config-hash "$hash" --max-age-days "$max_age"`  
> `scripts/agents/dispatch.sh:565:read -r runtime_version config_hash < <("$adapter" config-identity)`

**F Q2.11.** Lines 572–654 assemble the prompt, launch the runtime, and perform handshake CAS. They call `dispatch latest-chain-record`, `supervise launch-detached`, `identity started-at`, `dispatch record-cas`, and `json get`; Bash assembles prompt text, polls identity, writes the ownership patch, touches the gate, and times out the handshake.

> `scripts/agents/dispatch.sh:580:  latest=$("$ms" dispatch latest-chain-record --jobs-dir "$jobs_dir" --root-job "$root_job") || return 1`  
> `scripts/agents/dispatch.sh:598:  launch_json=$("$ms" supervise launch-detached --log "$log_file" -- "$adapter" "$verb" --job "$job" --start-gate "$start_gate") || return 1`  
> `scripts/agents/dispatch.sh:607:  pid_started_at=$("$ms" identity started-at --pid "$pid") || return 1`  
> `scripts/agents/dispatch.sh:640:  : >"$start_gate"`

**F Q2.12.** Lines 656–739 wait, aggregate usage, and mirror evidence. Go verbs are `dispatch chain-usage`, `mirror`, `chain-members`, `mission-fence aggregate-usage`, and adapter `root-job`; Bash polls statuses, invokes reap, decides terminality/path checks, creates temporary patches, and retries CAS.

> `scripts/agents/dispatch.sh:662:  "$ms" dispatch chain-usage --jobs-dir "$jobs_dir" --root-job "$root_job" --output "$output"`  
> `scripts/agents/dispatch.sh:679:  "$ms" dispatch mirror --root "$root" --root-job "$root_job" --output "$output"`  
> `scripts/agents/dispatch.sh:691:  "$ms" mission-fence aggregate-usage --root "$root" --mission "$mission" --output "$mission_usage"`  
> `scripts/agents/dispatch.sh:712:    members=$("$ms" dispatch chain-members --jobs-dir "$jobs_dir" --root-job "$root_job") || return 1`

**F Q2.13.** Lines 741–872 implement reap. They call `dispatch reap-facts`, identity/group liveness, `mission-fence refuse`, `mission-fence aggregate-usage`, `dispatch mirror`, and chain usage. Bash orders stale-epoch, abandonment, handshake, budget, and liveness cases; winds down processes; builds patches; retries CAS; and emits events.

> `scripts/agents/dispatch.sh:746:  facts=$("$ms" dispatch reap-facts --job "$job_file" --now "$now") || return $?`  
> `scripts/agents/dispatch.sh:810:      wind_down_process "$pid" "$pgid" "$tag"`  
> `scripts/agents/dispatch.sh:836:      "$ms" mission-fence refuse --root "$root" --mission "$mission" --reason "$reason" >/dev/null 2>&1 || true`  
> `scripts/agents/dispatch.sh:857:    if record_cas "$job_file" "$status" "$new_status" "$patch"; then`

**F Q2.14.** Public `dispatch` spans lines 874–1066: flags 880–898; validation 899–917; roster/model 918–938; mission/escalation 939–972; reservation/census/locks 974–993; cap 995–1013; worktree/permissions 1015–1031; prompt 1033–1042; record setup 1043–1057; launch/wait 1058–1065. Go also supplies `build-setup`, `build-record`, `mission-fence reserve-job`, and `authorize-cap`; Bash owns flag checks, Git worktree creation, sequencing, and error handling.

> `scripts/agents/dispatch.sh:880:  while [[ $# -gt 0 ]]; do`  
> `scripts/agents/dispatch.sh:976:    "$ms" mission-fence reserve-job --root "$root" --mission "$mission" --job "$job" --role "$role"`  
> `scripts/agents/dispatch.sh:1020:    git -C "$root" worktree add -q -b "$branch" "$worktree" HEAD`  
> `scripts/agents/dispatch.sh:1046:  "$ms" dispatch build-setup --job "$job" --role "$role" --runtime "$runtime" --model "$model" \`  
> `scripts/agents/dispatch.sh:1051:  "$ms" dispatch build-record --setup "$setup_patch" --brief "$brief" --workspace "$workspace" \`  
> `scripts/agents/dispatch.sh:1060:  launch_and_handshake dispatch "$job" "$job_file" "$start_gate"`

**F Q2.15.** Lines 1068–1082 apply critique-exhaustion patches through `dispatch critique-exhaustion`, `dispatch exhaustion-patches`, and CAS. Lines 1084–1239 implement follow-up: flags, census/chain/worktree freshness, critic Git synchronization, exhaustion, cap, permissions, resume embedding, follow-record setup, launch, and wait. Bash retains Git and orchestration decisions.

> `scripts/agents/dispatch.sh:1072:  action=$("$ms" dispatch critique-exhaustion "$@") || return $?`  
> `scripts/agents/dispatch.sh:1076:  done < <("$ms" dispatch exhaustion-patches --manifest "$manifest")`  
> `scripts/agents/dispatch.sh:1087:  while [[ $# -gt 0 ]]; do`  
> `scripts/agents/dispatch.sh:1144:    git -C "$worktree" reset --hard "$reviewed_tree" >/dev/null`  
> `scripts/agents/dispatch.sh:1223:  "$ms" dispatch build-follow-record --parent "$latest" --job "$job" --brief "$brief" \`

**F Q2.16.** Lines 1241–1272 implement status and cancel. Status uses `json get`; cancel uses lease/authority and dynamically calls adapter `cancel`. Bash validates status values and formats census age.

> `scripts/agents/dispatch.sh:1248:  status=$(json_field "$job_file" status)`  
> `scripts/agents/dispatch.sh:1262:  printf '%s\n' "$status"`  
> `scripts/agents/dispatch.sh:1271:  "$root/scripts/agents/adapters/$runtime.sh" cancel --job "$job"`

**F Q2.17.** Lines 1274–1300 close a root chain using adapter `root-job`, `dispatch close-check`, and CAS. Lines 1302–1354 expose reap using `supervise heartbeat`, authority/lease, and `reap_job`. Bash retains flag parsing, root enforcement, lock/gate behavior, and polling.

> `scripts/agents/dispatch.sh:1282:  root_job=$("$ms" adapter root-job --jobs-dir "$jobs_dir" --job "$job") || return $?`  
> `scripts/agents/dispatch.sh:1285:  "$ms" dispatch close-check --jobs-dir "$jobs_dir" --root-job "$root_job"`  
> `scripts/agents/dispatch.sh:1314:  "$ms" supervise heartbeat --file "$heartbeat_file" --component reaper --tag "$instance_tag" --pid $$ >/dev/null`  
> `scripts/agents/dispatch.sh:1345:      reap_job "$record" || rc=$?`

**F Q2.18.** Lines 1356–1516 are custody, handshake, cancel, critique, reap-held, launch, and handshake-timeout callbacks. Lines 1518–1574 re-exec under locks and route every public/internal command in Bash.

> `scripts/agents/dispatch.sh:1367:  started=$("$ms" identity started-at --pid "$pid") || return 1`  
> `scripts/agents/dispatch.sh:1368:  "$ms" dispatch custody-add --job "$job_file" --pid "$pid" --started-at "$started" --tag "$tag"`  
> `scripts/agents/dispatch.sh:1381:  "$ms" dispatch handshake-eval --job "$job_file" --runtime "$runtime" --session "$session" --patch "$patch"`  
> `scripts/agents/dispatch.sh:1534:case ${1:-} in`  
> `scripts/agents/dispatch.sh:1573:  *) usage; exit 2 ;;`

## Q3 — adapter and host call graph

**F Q3.1.** `runtime-common.sh` defines 32 functions. Callers and classifications are:

- `adapter_common_init` (line 6), called by Claude/Codex/Devin at line 22: setup glue.
- `field` (15), called throughout common and real adapters: `json get` glue.
- `parse_supervisor_args` (19), called by `prepare_supervision`: Bash flag decisions.
- `root_job_id` (32), called by `prepare_supervision`: adapter `root-job` glue.
- `adapter_milliseconds_to_sleep` (36), called at 49/76/162: numeric-validation glue.
- `prepare_supervision` (43), called by Claude 100, Codex 134, Devin 262: effective-init glue plus gate/path decisions.
- `register_cli_custody` (74), called by Claude 156, Codex 161, Devin 341: callback/retry decisions.
- `record_actual_workspace_write_scope` (89), called by each real supervise path: effective-workspace glue.
- `fail_if_effective_wider_before_launch` (93), called by each real supervise path: permission-check plus failure decision.
- `record_handshake` (102), called by each real adapter: handshake callback plus collision decisions.
- `record_result_effective_model` (117), called by Claude/Devin and telemetry: model-patch/CAS glue.
- `write_patch` (125), called by terminal helpers: result-patch glue.
- `fail_pending` (129), `finish_running` (140), and `finish_protocol_error` (151), called by completion paths: terminal-state/CAS decisions.
- `wait_for_cli` (160) and `terminate_cli_child` (175), called by real adapters: Bash process control.
- `normalize_return` (184), called by `validate_candidate` and fixtures: adapter `normalize-return` glue.
- `validate_candidate` (191), called by `complete_from_cli`: normalization plus assert-return decision.
- `attempt_return_repair` (212), called by `complete_from_cli`: repair eligibility/prompt decisions.
- `complete_from_cli` (236), called by all real adapters: terminal/repair state machine.
- `record_return_repairs` (288), called by `complete_from_cli`: repairs-patch/CAS glue.
- `configuration_identity` (294), called by adapter identity functions: config-identity glue.
- `configuration_identity_field` (304), called by identity/probe functions: JSON-get glue.
- `write_capability_snapshot` (308), called by probes: capability-snapshot glue.
- `make_selftest_brief` (324), called by the full selftest: Bash template construction.
- `selftest_turn_cap` (366), `wait_for_selftest_job` (370), `selftest_usage_check` (385), `selftest_envelope_declaration` (389), and `selftest_attempt_matches_declaration` (393), called by `run_full_contract_selftest`: adapter selftest verbs plus polling/declaration decisions.
- `run_full_contract_selftest` (431), called by Claude 225, Codex 229, Devin 453: Bash selftest orchestration.

> `scripts/agents/adapters/runtime-common.sh:6:adapter_common_init() { # runtime`  
> `scripts/agents/adapters/runtime-common.sh:43:prepare_supervision() { # dispatch|follow-up and supervisor args`  
> `scripts/agents/adapters/runtime-common.sh:74:register_cli_custody() { # child pid`  
> `scripts/agents/adapters/runtime-common.sh:236:complete_from_cli() { # cli status, usage file, candidate file, optional transcript`  
> `scripts/agents/adapters/runtime-common.sh:431:run_full_contract_selftest() { # native|unavailable, optional devin flag`  
> `scripts/agents/adapters/claude.sh:100:  prepare_supervision "$@"`  
> `scripts/agents/adapters/codex.sh:134:  prepare_supervision "$@"`  
> `scripts/agents/adapters/devin.sh:262:  prepare_supervision "$@"`

**F Q3.2.** The adjacent Go adapter surface includes root-job, effective-init/workspace, permission checking, model/repairs/result patches, capability snapshots, normalize-return, runtime-specific parsers/builders, and selftest helpers.

> `cmd/metasystem/main.go:121:				{"root-job", "print a job's root ancestor by walking parentJob", runAdapterRootJob},`  
> `cmd/metasystem/main.go:122:				{"effective-init", "materialize the effective permissions from a job record", runAdapterEffectiveInit},`  
> `cmd/metasystem/main.go:124:				{"permission-check", "report which effective permission fields are wider than requested", runAdapterPermissionCheck},`  
> `cmd/metasystem/main.go:150:				{"normalize-return", "normalize the runtime reply into return.json/return.md", runAdapterNormalizeReturn},`  
> `cmd/metasystem/main.go:151:				{"selftest-usage", "assert a selftest job's typed usage", runAdapterSelftestUsage},`

**F Q3.3.** `claude.sh` defines `usage` (4), `claude_version` (24), `claude_config_identity` (29), `claude_identity` (47), `probe` (55), `build_claude_settings` (83), `claude_usage` (89), `claude_result_field` (93), and `supervise` (97). Main dispatch calls usage/identity/probe/supervise; identity/probe call config identity; config identity calls version and common configuration identity; supervise calls settings/usage/result helpers. Version/config/identity/settings/usage/result are exec glue over adapter verbs; probe and supervise retain CLI availability/authentication, process launch, handshake, and completion decisions.

> `scripts/agents/adapters/claude.sh:24:claude_version() {`  
> `scripts/agents/adapters/claude.sh:55:probe() {`  
> `scripts/agents/adapters/claude.sh:83:build_claude_settings() { # output settings`  
> `scripts/agents/adapters/claude.sh:97:supervise() { # dispatch|follow-up and supervisor args`  
> `scripts/agents/adapters/claude.sh:212:    config-identity) claude_config_identity ;;`  
> `scripts/agents/adapters/claude.sh:216:    probe) probe ;;`  
> `scripts/agents/adapters/claude.sh:218:    dispatch|follow-up) supervise "$@" ;;`  
> `cmd/metasystem/main.go:133:				{"claude-settings", "build the Claude job settings from a record", runAdapterClaudeSettings},`  
> `cmd/metasystem/main.go:135:				{"claude-result-field", "read a Claude result field with modelUsage collapse", runAdapterClaudeResultField},`

**F Q3.4.** `codex.sh` defines `usage` (4), `codex_version` (24), `codex_config_identity` (29), `codex_identity` (43), `probe` (51), `codex_event_field` (82), `codex_usage` (86), `codex_permission_settings` (90), `build_codex_command` (107), and `supervise` (131). Main dispatch calls identity/probe/supervise; supervise calls event/usage/permission/command helpers. Version/config/event/usage/command are exec glue over adapter verbs; permission settings maps requested permissions in Bash; probe and supervise retain CLI/authentication, launch, event-loop, handshake, and completion decisions.

> `scripts/agents/adapters/codex.sh:82:codex_event_field() { # events JSONL, session|turn`  
> `scripts/agents/adapters/codex.sh:90:codex_permission_settings() { # permissions JSON, optional dotted prefix`  
> `scripts/agents/adapters/codex.sh:107:build_codex_command() { # dispatch|follow-up, model, workspace, schema, output, sandbox, network, session`  
> `scripts/agents/adapters/codex.sh:131:supervise() { # dispatch|follow-up and supervisor args`  
> `cmd/metasystem/main.go:130:				{"codex-event", "read the session or turn field from a Codex event stream", runAdapterCodexEvent},`  
> `cmd/metasystem/main.go:132:				{"codex-command", "build the Codex delegate argv (NUL-terminated)", runAdapterCodexCommand},`

**F Q3.5.** `devin.sh` defines `usage` (4), `devin_version` (34), `devin_config_identity` (39), `devin_identity` (55), `probe` (63), `build_devin_config` (92), `list_devin_sessions` (103), `new_devin_session` (118), `devin_usage` (123), `previous_round_artifact` (145), `devin_record_effective_model` (153), `devin_settle_session_identity` (174), `runtime_usage_after_repair` (207), `runtime_settle_after_repair` (224), `runtime_repair_turn` (238), and `supervise` (259). Main dispatch calls identity/probe/supervise; supervise calls config/list/new-session/usage/model/session helpers; common completion calls the three `runtime_*` repair hooks. Config/new-session/usage are adapter-verb glue; listing sessions, artifact selection, result settling, repair launch, and supervise remain Bash/external-CLI decisions.

> `scripts/agents/adapters/devin.sh:92:build_devin_config() { # config output, provenance output`  
> `scripts/agents/adapters/devin.sh:103:list_devin_sessions() { # output file`  
> `scripts/agents/adapters/devin.sh:153:devin_record_effective_model() { # transcript`  
> `scripts/agents/adapters/devin.sh:238:runtime_repair_turn() { # prompt file, output file`  
> `scripts/agents/adapters/devin.sh:259:supervise() { # dispatch|follow-up and supervisor args`  
> `cmd/metasystem/main.go:139:				{"devin-config", "build the Devin job config from the user config", runAdapterDevinConfig},`  
> `cmd/metasystem/main.go:141:				{"devin-usage", "compute the Devin per-round usage delta", runAdapterDevinUsage},`

**F Q3.6.** `fake.sh` defines `usage` (4), `field` (36), `parse_supervisor_args` (40), `behavior_present` (53), `fake_guarded_write` (55), `fake_guarded_network_call` (59), `probe_fake_envelope_mechanism` (63), `fixture_milliseconds_to_sleep` (88), `cas_terminal` (95), `write_valid_return` (107), `complete_valid` (113), `supervise` (125), and `probe` (231). Main dispatch calls probe/supervise; supervise calls parsing, behavior, timing, CAS, return, and completion helpers; probe calls guarded operations and snapshot helpers. Field access, guarded operations, fixed usage/return, snapshots, and result patches are Go glue; marker interpretation, timing, signals, terminal transitions, and probe pass/fail remain Bash decisions.

> `scripts/agents/adapters/fake.sh:53:behavior_present() { grep -Fqi "FAKE:$1" "$prompt"; }`  
> `scripts/agents/adapters/fake.sh:63:probe_fake_envelope_mechanism() {`  
> `scripts/agents/adapters/fake.sh:95:cas_terminal() { # target, error, phase`  
> `scripts/agents/adapters/fake.sh:125:supervise() { # verb and remaining args`  
> `scripts/agents/adapters/fake.sh:231:probe() {`  
> `cmd/metasystem/main.go:143:				{"fake-return", "write the fake runtime's canned role return", runAdapterFakeReturn},`  
> `cmd/metasystem/main.go:146:				{"fake-guarded-write", "attempt a permission-guarded write (77 = refused)", runAdapterFakeGuardedWrite},`

**F Q3.7.** `hosts/claude.sh` defines `usage` (4), `atomic_result` (15), and `wait_for_start_gate` (20). `hosts/codex.sh` defines `usage` (11), `atomic_result` (19), and `wait_for_start_gate` (24). Each main body calls the gate helper and calls `atomic_result` on terminal paths. Atomic result is host `result-write` glue; gate polling and runtime CLI/result branching are Bash decisions. The Codex host also reuses the sourced Codex adapter helpers.

> `scripts/agents/hosts/claude.sh:15:atomic_result() { # result path, session, outcome, usage JSON path, raw, return or empty`  
> `scripts/agents/hosts/claude.sh:20:wait_for_start_gate() {`  
> `scripts/agents/hosts/codex.sh:9:source "$root/scripts/agents/adapters/codex.sh"`  
> `scripts/agents/hosts/codex.sh:19:atomic_result() { # result path, session, outcome, usage JSON path, raw, return or empty`  
> `scripts/agents/hosts/codex.sh:24:wait_for_start_gate() {`  
> `cmd/metasystem/main.go:161:				{"result-write", "write a host turn's result envelope", runHostResultWrite},`  
> `cmd/metasystem/main.go:163:				{"claude-result", "extract the Claude return and usage", runHostClaudeResult},`

**F Q3.8.** `hosts/devin.sh` defines `usage` (4), `atomic_result` (16), and `wait_for_start_gate` (21); its main body calls the latter two. Go host verbs build Devin config, extract return and usage, and write results. Bash retains polling, external session lifecycle, cumulative-usage storage, and outcome branching.

> `scripts/agents/hosts/devin.sh:16:atomic_result() { # result path, session, outcome, usage JSON path, raw, return or empty`  
> `scripts/agents/hosts/devin.sh:21:wait_for_start_gate() {`  
> `cmd/metasystem/main.go:164:				{"devin-config", "assemble the Devin job config", runHostDevinConfig},`  
> `cmd/metasystem/main.go:165:				{"devin-return", "extract the Devin return", runHostDevinReturn},`  
> `cmd/metasystem/main.go:166:				{"devin-usage", "compute the Devin per-round usage delta", runHostDevinUsage},`

**F Q3.9.** `hosts/fake.sh` defines `usage` (4) and `wait_for_start_gate` (19); its main body calls the gate helper. Go host `fake-return` and `fake-result` perform output construction, while Bash retains marker-based fake behavior, timing, gate polling, and terminal choice.

> `scripts/agents/hosts/fake.sh:19:wait_for_start_gate() {`  
> `cmd/metasystem/main.go:167:				{"fake-return", "write the fake-runtime return and terminal record", runHostFakeReturn},`  
> `cmd/metasystem/main.go:168:				{"fake-result", "write the fake-runtime result envelope", runHostFakeResult},`

## Q4 — Python 3 call sites

**F Q4.1.** There are 191 tracked-shell text hits for `python3`. Two are not invocations: `flight-recorder-fixtures.sh:24` is a comment and `validate-metasystem.sh:3367` performs command discovery. The generated shebang at `delegate-caps-fixtures.sh:180` is an interpreter invocation when the generated `ps` stub is executed. The executable set is therefore 189 sites.

> `scripts/agents/flight-recorder-fixtures.sh:24:#    and a PATH without python3 each leave a set -e caller alive.`  
> `scripts/validate-metasystem.sh:3367:if command -v python3 >/dev/null; then`  
> `scripts/agents/delegate-caps-fixtures.sh:180:#!/usr/bin/env python3`

**F Q4.2.** `assert-conformance.sh` has five invocations: line 39 parses/validates the job record and emits shell facts—no single existing generic verb; 108 enforces path/install-prefix boundaries—no; 192 writes `review.json`—no; 223 reads `critiqueWaived`—yes, `json get`; 238 performs the final boundary/numstat/identity/critic-independence join—no.

> `scripts/agents/assert-conformance.sh:39:facts=$(python3 - "$root" "$record" <<'PY'`  
> `scripts/agents/assert-conformance.sh:108:  python3 - "$workspace" "$root" "$root_job" "$round" "$paths_file" "$install_prefix" <<'PY'`  
> `scripts/agents/assert-conformance.sh:192:  python3 - "$review_file" "$job" "$reviewed_tree" "$(basename "$diff_file")" <<'PY'`  
> `scripts/agents/assert-conformance.sh:223:has_waiver=$(python3 - "$record" <<'PY'`  
> `scripts/agents/assert-conformance.sh:238:python3 - "$root" "$record" "$paths_file" "$numstat_file" "$install_prefix" \`

**F Q4.3.** `config-identity-fixtures.sh` has six invocations: 47 asserts bookkeeping does not alter identity—no existing generic verb; 60 asserts semantic config does alter it—no; 78 compares canonical identities—no; 101 checks malformed/out-of-range full-hash fallback—no; 117 constructs a capability-snapshot fixture—adjacent to adapter capability-snapshot but not answered by it; 152 validates the three adapter filter manifests—`config validate` does not validate those manifests.

> `scripts/agents/config-identity-fixtures.sh:47:python3 - "$identity_before" "$identity_bookkeeping" <<'PY'`  
> `scripts/agents/config-identity-fixtures.sh:60:python3 - "$identity_before" "$identity_changed" <<'PY'`  
> `scripts/agents/config-identity-fixtures.sh:78:python3 - "$canonical_a" "$canonical_b" <<'PY'`  
> `scripts/agents/config-identity-fixtures.sh:101:python3 - "$malformed" "$out_of_range" "$full" <<'PY'`  
> `scripts/agents/config-identity-fixtures.sh:117:python3 - "$fixture_root" "$identity_before" <<'PY'`  
> `scripts/agents/config-identity-fixtures.sh:152:python3 - "$root/scripts/agents/adapters" <<'PY'`

**F Q4.4.** `conformance-fixtures.sh` has ten invocations: 53 writes implementer records/returns—no; 108 writes follow-up returns—no; 136 writes critic records/returns—no; 198 checks the instruction-bearing path inventory—no; 273 and 362 read `reviewedTree`—yes, `json get`; 441 seeds exhaustion records—no; 474 validates exhaustion manifests/patches—no existing validate verb; 493 mutates a critic successor—no; 509 checks exhaustion-chain relations—no.

> `scripts/agents/conformance-fixtures.sh:53:  python3 - "$controller" "$worktree" "$base_sha" "$waiver" "$@" <<'PY'`  
> `scripts/agents/conformance-fixtures.sh:108:  python3 - "$controller" "$fixture_round" "$@" <<'PY'`  
> `scripts/agents/conformance-fixtures.sh:136:  python3 - "$controller" "$tree" "$material_id" "$exhaustion" "$model" <<'PY'`  
> `scripts/agents/conformance-fixtures.sh:198:python3 - "$source_root" <<'PY'`  
> `scripts/agents/conformance-fixtures.sh:273:reviewed_tree=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["reviewedTree"])' \`  
> `scripts/agents/conformance-fixtures.sh:362:first_tree=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["reviewedTree"])' \`  
> `scripts/agents/conformance-fixtures.sh:441:python3 - "$exhaustion_root" <<'PY'`  
> `scripts/agents/conformance-fixtures.sh:474:python3 - "$exhaustion_root" "$exhaustion_manifest" <<'PY'`  
> `scripts/agents/conformance-fixtures.sh:493:python3 - "$exhaustion_root/artifacts/agents/jobs/critic-r3.json" <<'PY'`  
> `scripts/agents/conformance-fixtures.sh:509:python3 - "$exhaustion_root" <<'PY'`

**F Q4.5.** `delegate-caps-fixtures.sh` has eleven direct calls plus the generated shebang in F Q4.1: 66/78 assert selected and narrowed cap documents—no; 205 seeds process identity—no; 217 synchronizes identity from supervision state—no; 250/260 mutate state and heartbeat fixtures—no; 300 seeds a blocking job—no; 312 inspects state cap decisions—no; 329 polls state/census consistency—no; 411 proves an incomplete assertion registry fails—no; 421 proves the executed registry is exact—no.

> `scripts/agents/delegate-caps-fixtures.sh:66:python3 - "$selected" <<'PY'`  
> `scripts/agents/delegate-caps-fixtures.sh:78:python3 - "$narrowed" <<'PY'`  
> `scripts/agents/delegate-caps-fixtures.sh:205:python3 - "$identity_fixture" "$$" "$process_start" <<'PY'`  
> `scripts/agents/delegate-caps-fixtures.sh:217:    python3 - "$identity_fixture" "$harness/artifacts/agents/supervision/state.json" <<'PY' || true`  
> `scripts/agents/delegate-caps-fixtures.sh:250:python3 - "$state" "$heartbeat" <<'PY'`  
> `scripts/agents/delegate-caps-fixtures.sh:260:python3 - "$state" "$heartbeat" <<'PY'`  
> `scripts/agents/delegate-caps-fixtures.sh:300:python3 - "$harness/artifacts/agents/jobs/blocking-job.json" <<'PY'`  
> `scripts/agents/delegate-caps-fixtures.sh:312:python3 - "$state" <<'PY'`  
> `scripts/agents/delegate-caps-fixtures.sh:329:  until python3 - "$state" "$census" <<'PY'`  
> `scripts/agents/delegate-caps-fixtures.sh:411:if python3 - AUTH-R2-001 AUTH-R2-002 AUTH-R2-003 AUTH-R2-005 AUTH-R2-006 <<'PY'`  
> `scripts/agents/delegate-caps-fixtures.sh:421:python3 - "${passed[@]}" <<'PY'`

**F Q4.6.** `evidence-segment-fixtures.sh` has two invocations: 40 verifies mirrored segments/manifests and archive collection—no single existing report/validate verb; 77 builds and ages a legacy evidence fixture—no.

> `scripts/agents/evidence-segment-fixtures.sh:40:python3 - "$evidence" <<'PY'`  
> `scripts/agents/evidence-segment-fixtures.sh:77:python3 - "$legacy_root/artifacts/agents/legacy-chain/brief.md" \`

**F Q4.7.** `flight-recorder-fixtures.sh` has seven actual calls: 55 validates one emitted event; 84 checks concurrent JSONL integrity; 99 checks cap/truncation; 111 validates registry/schema fields; 142 reads lease `claimEpoch`—yes, `json get`; 166 validates recovery after malformed input; 183 checks payload limits. The event assertions have no existing validate/report verb.

> `scripts/agents/flight-recorder-fixtures.sh:55:python3 - "$stream" <<'PY' || exit 1`  
> `scripts/agents/flight-recorder-fixtures.sh:84:python3 - "$stream" <<'PY' || exit 1`  
> `scripts/agents/flight-recorder-fixtures.sh:99:python3 - "$stream" <<'PY' || exit 1`  
> `scripts/agents/flight-recorder-fixtures.sh:111:python3 - "$stream" "$root/scripts/agents/event-registry.json" <<'PY' || exit 1`  
> `scripts/agents/flight-recorder-fixtures.sh:142:python3 -c "`  
> `scripts/agents/flight-recorder-fixtures.sh:166:python3 - "$stream" <<'PY' || exit 1`  
> `scripts/agents/flight-recorder-fixtures.sh:183:python3 - "$stream" <<'PY' || exit 1`

**F Q4.8.** `lease-succession-fixtures.sh` has two invocations: 41 validates announced main/lease succession records; 96 validates second-session manifest/worktree isolation. No existing JSON-get, validate, schema, or report verb answers either multi-document assertion.

> `scripts/agents/lease-succession-fixtures.sh:41:python3 - "$checkout" <<'PY'`  
> `scripts/agents/lease-succession-fixtures.sh:96:python3 - "$turns" <<'PY'`

**F Q4.9.** `mission-fixtures.sh` has twenty invocations: 152 creates missing/malformed contract cases—no; 238 stales a sealed contract—no; 272 writes supervision identity fixtures—no; 319/346 read `integrity.hash` and 387 reads the anchor hash—yes, `json get`; 321/339/351/388 mutate state proposals—no; 330/364/381 assert state/reconcile/ledger fields—no single verb; 417 writes a fence fixture—no; 481 writes supervision fixture state—no; 595/601/623 assert mission/chain close state—no; 666/683 assert host result envelopes—no single validate verb.

> `scripts/agents/mission-fixtures.sh:152:  python3 - "$base" "$missing" "$malformed" "$key" <<'PY'`  
> `scripts/agents/mission-fixtures.sh:238:python3 - "$stale" <<'PY'`  
> `scripts/agents/mission-fixtures.sh:272:python3 - "$supervision" "$watcher_pid" "$reaper_pid" "$watcher_start" "$reaper_start" <<'PY'`  
> `scripts/agents/mission-fixtures.sh:319:state_hash=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["integrity"]["hash"])' "$state")`  
> `scripts/agents/mission-fixtures.sh:321:python3 - "$state" "$proposal" <<'PY'`  
> `scripts/agents/mission-fixtures.sh:330:python3 - "$state" <<'PY'`  
> `scripts/agents/mission-fixtures.sh:339:python3 - "$state" "$stoploss_proposal" <<'PY'`  
> `scripts/agents/mission-fixtures.sh:346:state_hash=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["integrity"]["hash"])' "$state")`  
> `scripts/agents/mission-fixtures.sh:351:python3 - "$forked" <<'PY'`  
> `scripts/agents/mission-fixtures.sh:364:python3 - "$forked" <<'PY'`  
> `scripts/agents/mission-fixtures.sh:381:python3 - "$anchor_state" <<'PY'`  
> `scripts/agents/mission-fixtures.sh:387:anchor_hash=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["integrity"]["hash"])' "$anchor_state")`  
> `scripts/agents/mission-fixtures.sh:388:python3 - "$anchor_state" "$proposal" <<'PY'`  
> `scripts/agents/mission-fixtures.sh:417:python3 - "$race_contract" "$race_fences" <<'PY'`  
> `scripts/agents/mission-fixtures.sh:481:python3 - "$supervision" <<'PY'`  
> `scripts/agents/mission-fixtures.sh:595:python3 - "$repo/artifacts/agents/missions/gate-and-close/state.json" <<'PY'`  
> `scripts/agents/mission-fixtures.sh:601:python3 - "$repo/artifacts/agents/missions/gate-and-close" <<'PY'`  
> `scripts/agents/mission-fixtures.sh:623:python3 - "$repo" <<'PY'`  
> `scripts/agents/mission-fixtures.sh:666:python3 - "$host_turn/result.json" <<'PY'`  
> `scripts/agents/mission-fixtures.sh:683:python3 - "$host_turn/result-missing.json" <<'PY'`

**F Q4.10.** `record-protocol-fixtures.sh` has four invocations: 28 asserts the pending-setup record; 53 backdates it; 71 continuously checks atomic JSON readability; 99 checks CAS/chain record relations. Only the JSON-validity portion of 71 is covered by `util json-validate`; the continuous race and other assertions are not.

> `scripts/agents/record-protocol-fixtures.sh:28:python3 - "$fixture_root/artifacts/agents/jobs/setup-job.json" <<'PY'`  
> `scripts/agents/record-protocol-fixtures.sh:53:python3 - "$fixture_root/artifacts/agents/jobs/stale-setup.json" <<'PY'`  
> `scripts/agents/record-protocol-fixtures.sh:71:python3 - "$fixture_root/artifacts/agents/jobs/chain-r2.json" "$tmp/reader.stop" "$tmp/reader.bad" <<'PY' &`  
> `scripts/agents/record-protocol-fixtures.sh:99:python3 - "$fixture_root/artifacts/agents/jobs/chain.json" \`

**F Q4.11.** `return-schema-fixtures.sh` has five invocations: 10 constructs return candidates; 43 recursively asserts materialized structured-output schemas; 93 checks normalized identity/model claims; 108 constructs a one-claim candidate; 118 checks its normalized `claimed` object. `schema materialize` supplies inputs at line 40 but does not perform these constructions or assertions.

> `scripts/agents/return-schema-fixtures.sh:10:python3 - "$fixture" <<'PY'`  
> `scripts/agents/return-schema-fixtures.sh:40:  "$ms" schema materialize --root "$root" --role "$role" --version 2 \`  
> `scripts/agents/return-schema-fixtures.sh:43:python3 - "$fixture" <<'PY'`  
> `scripts/agents/return-schema-fixtures.sh:93:python3 - "$fixture/return.json" <<'PY'`  
> `scripts/agents/return-schema-fixtures.sh:108:python3 - "$fixture/candidate.json" "$fixture/one-claim.json" <<'PY'`  
> `scripts/agents/return-schema-fixtures.sh:118:python3 - "$fixture/return.json" <<'PY'`

**F Q4.12.** `second-session-fixtures.sh` has two invocations: 22 writes the worktree manifest fixture; 101 checks the bootstrap arming log/destination. No existing JSON-get, validate, schema, or report verb answers those write/cross-file assertions.

> `scripts/agents/second-session-fixtures.sh:22:python3 - "$manifest" <<'PY'`  
> `scripts/agents/second-session-fixtures.sh:101:python3 - "$tmp/bootstrap-arm.log" "$bootstrap_destination" <<'PY'`

**F Q4.13.** `supervision-go-fixtures.sh` has five invocations: 59 waits for published component state; 79 validates purpose-gone terminal teardown; 98 validates supersession; 111 validates cycle-trace decision basis; 129 validates crash-loop giving-up teardown. No current report verb exposes these registry/trace assertions.

> `scripts/agents/supervision-go-fixtures.sh:59:  python3 - "$1" <<PY 2>/dev/null || exit 1`  
> `scripts/agents/supervision-go-fixtures.sh:79:python3 - "$registry" <<'PY' || fail "no purpose-gone terminal with complete teardown"`  
> `scripts/agents/supervision-go-fixtures.sh:98:python3 - "$registry" <<'PY' || fail "no superseded terminal"`  
> `scripts/agents/supervision-go-fixtures.sh:111:python3 - "$trace" <<'PY' || fail "cycle trace does not narrate the decision basis"`  
> `scripts/agents/supervision-go-fixtures.sh:129:python3 - "$registry" <<'PY' || fail "no giving-up terminal with complete teardown"`

**F Q4.14.** `telemetry-census-fixtures.sh` has four invocations: 14 implements fixture CAS; 36 constructs job/result telemetry fixtures; 80 asserts normalized model/usage; 114 asserts failed-census shape. No existing JSON-get, validate, schema, or report verb performs those mutations or multi-field assertions.

> `scripts/agents/telemetry-census-fixtures.sh:14:  python3 - "$record" "$9" "$5" "$7" <<'PY'`  
> `scripts/agents/telemetry-census-fixtures.sh:36:  python3 - "$record" "$case_dir/result.json" "$job" "$model_usage" <<'PY'`  
> `scripts/agents/telemetry-census-fixtures.sh:80:  python3 - "$record" "$round_dir/return.json" "$expected" <<'PY'`  
> `scripts/agents/telemetry-census-fixtures.sh:114:python3 - "$tmp/failure-census.json" <<'PY'`

**F Q4.15.** `supervision-fixtures.sh` has 35 invocations. Lines 123, 580, 592, 598, and 628 are simple JSON field reads answerable by `json get`; 525 is an OS `getpgid` query not covered by the named Go families. Lines 135/145/300/318/401 are compound census assertions; 447/470/519/527/541/575/583/603/668/702/737/750/755/760 create or mutate fixtures; 557/564/596/607/616/623 inspect compound census conditions; 814 creates a dead-child fixture; 921/994/1025 assert gate markers, arming logs, and foreign owner identity. No existing generic verb performs those compound, mutation, or OS tasks.

> `scripts/agents/supervision-fixtures.sh:123:  python3 - "$1" "$2" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:135:  python3 - "$1" "$2" "$3" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:145:  python3 - "$output" "$@" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:300:python3 - "$repo" "$tmp/enumerate-filter-resolve-procs.json" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:318:python3 - "$tmp/enumerate-filter-resolve.json" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:401:python3 - "$last" "$state" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:447:python3 - "$warning_supervision/state.json" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:470:python3 - "$warning_supervision/last-census.json" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:519:python3 - "$repo/artifacts/agents/mains/announced-$announced_pid.json" "$announced_pid" "$announced_start" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:525:supervisor_pgid=$(python3 -c 'import os; print(os.getpgid(int(__import__("sys").argv[1])))' "$$")`  
> `scripts/agents/supervision-fixtures.sh:527:python3 - "$repo/artifacts/agents/jobs/owned.json" "$repo/artifacts/agents/hb/owned" "$repo" \`  
> `scripts/agents/supervision-fixtures.sh:541:python3 - "$identity_fixture" "$$" "$supervisor_start" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:557:if python3 - "$last" "$peer_pid" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:564:python3 - "$last" "$unresolved_pid" <<'PY' \`  
> `scripts/agents/supervision-fixtures.sh:575:python3 - "$repo/artifacts/agents/jobs/owned.json" "$custody_start" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:580:wait_until "S4-2 wrong-tag census pass" bash -c '[[ $(python3 -c '\''import json,sys; print(json.load(open(sys.argv[1]))["completedAtEpoch"])'\'' "$1") -gt "$2" ]]' _ "$last" "$previous_epoch"`  
> `scripts/agents/supervision-fixtures.sh:583:python3 - "$repo/artifacts/agents/jobs/owned.json" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:592:wait_until "S4-6 enumeration failure" bash -c '[[ $(python3 -c '\''import json,sys; print(json.load(open(sys.argv[1]))["verdict"])'\'' "$1" 2>/dev/null) == CENSUS-FAILED ]]' _ "$last"`  
> `scripts/agents/supervision-fixtures.sh:596:  '[[ -n $(python3 -c '\''import json,sys; print("".join(e for e in json.load(open(sys.argv[1])).get("errors",[]) if "enumeration" in e))'\'' "$1" 2>/dev/null) ]]' _ "$last"`  
> `scripts/agents/supervision-fixtures.sh:598:wait_until "census recovery" bash -c '[[ $(python3 -c '\''import json,sys; print(json.load(open(sys.argv[1]))["verdict"])'\'' "$1") == SUCCESS ]]' _ "$last"`  
> `scripts/agents/supervision-fixtures.sh:603:python3 - "$process_fixture" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:607:wait_until "S4-6 raced-exit census" bash -c 'python3 - "$1" "$2" <<'\''PY'\'''`  
> `scripts/agents/supervision-fixtures.sh:616:wait_until "S4-6 unreadable argv" bash -c 'python3 - "$1" <<'\''PY'\'''`  
> `scripts/agents/supervision-fixtures.sh:623:wait_until "S4-6 unreadable start time" bash -c 'python3 - "$1" <<'\''PY'\'''`  
> `scripts/agents/supervision-fixtures.sh:628:wait_until "S4-6 partial-failure recovery" bash -c '[[ $(python3 -c '\''import json,sys; print(json.load(open(sys.argv[1]))["verdict"])'\'' "$1") == SUCCESS ]]' _ "$last"`  
> `scripts/agents/supervision-fixtures.sh:668:  python3 - "$gate_repo/artifacts/agents/supervision/last-census.json" \`  
> `scripts/agents/supervision-fixtures.sh:702:python3 - "$gate_repo/artifacts/agents/supervision/state.json" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:737:python3 - "$gate_repo/artifacts/agents/supervision/state.json" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:750:python3 - "$gate_repo/artifacts/agents/supervision/last-census.json" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:755:python3 - "$gate_repo/artifacts/agents/supervision/last-census.json" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:760:python3 - "$gate_repo/artifacts/agents/supervision/state.json" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:814:dead_pid=$(python3 - <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:921:  python3 - "$gate_probe_markers" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:994:python3 - "$repo/artifacts/agents/supervision/arming.log" <<'PY'`  
> `scripts/agents/supervision-fixtures.sh:1025:python3 - "$foreign/repo/metasystem/artifacts/agents/supervision/lock.d/owner.json" \`

**F Q4.16.** `validate-metasystem.sh` lines 308–1405 contain eleven invocations: 308 validates lifecycle-hook JSON shape—`hooks check` covers live settings, not these source configs; 482 compares fake snapshot/probe facts—no; 562 writes a filled fixture config—no; 613 checks host-template placeholders—no; 629 checks schemas/permissions/requirements shapes—no single verb; 724 checks orchestrator quote-marker inventory—no; 818 builds return fixtures—no; 1088 builds invalid turn prompts—no; 1161 builds critique fixtures—no; 1393 creates a round-two return—no; 1405 creates bad-identity returns—no.

> `scripts/validate-metasystem.sh:308:python3 - scripts/enforcement/claude-code-hooks.json scripts/enforcement/codex-hooks.json scripts/enforcement/devin-hooks.json <<'PY'`  
> `scripts/validate-metasystem.sh:482:python3 - "$fake_snapshot" "$fake_probe_result" <<'PY'`  
> `scripts/validate-metasystem.sh:562:  python3 - "$1" "$2" <<'PY'`  
> `scripts/validate-metasystem.sh:613:python3 - scripts/agents/templates/host-turn-instruction.md <<'PY'`  
> `scripts/validate-metasystem.sh:629:python3 - "$root" <<'PY'`  
> `scripts/validate-metasystem.sh:724:python3 - scripts/agents/roles/orchestrator.md <<'PY'`  
> `scripts/validate-metasystem.sh:818:python3 - "$return_fixtures" <<'PY'`  
> `scripts/validate-metasystem.sh:1088:python3 - "$good_turn_prompt" "$turn_fixture" <<'PY'`  
> `scripts/validate-metasystem.sh:1161:python3 - "$return_fixtures/design-critic-positive.json" "$critique_fixtures" <<'PY'`  
> `scripts/validate-metasystem.sh:1393:python3 - "$return_fixtures/implementer-positive.json" \`  
> `scripts/validate-metasystem.sh:1405:python3 - "$return_fixtures/implementer-positive.json" "$return_fixtures" <<'PY'`

**F Q4.17.** `validate-metasystem.sh` lines 1498–1831 contain five invocations: 1498 sets a stale Git-lock mtime—no; 1533 polls compound census conditions—no; 1589 reads status—yes, `json get`; 1724 is a bounded PTY driver—no; 1831 removes model tiers from a fixture config—no.

> `scripts/validate-metasystem.sh:1498:      python3 - "$runner_repo/.git/index.lock" "$runner_git_stale_sec" <<'LOCK'`  
> `scripts/validate-metasystem.sh:1533:      if [[ -n "$expected" ]] && python3 - \`  
> `scripts/validate-metasystem.sh:1589:      status=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("status", "malformed"))' \`  
> `scripts/validate-metasystem.sh:1724:    python3 - "$agent_fixture/$name.out" "$typed" "$expected_exit" "$agent_fixture_cap_sec" \`  
> `scripts/validate-metasystem.sh:1831:  python3 - "$good_agent_conf" "$no_tier_conf" <<'PY'`

**F Q4.18.** `validate-metasystem.sh` lines 1900–2446 contain 26 invocations. Simple field reads at 1971, 2037, 2119, 2121, 2126, 2132, 2258, 2261, 2263, 2342, and 2369 are answerable by `json get`. Line 2088 is answerable by `util json-validate`. Lines 1900/1959 assert record/override behavior; 2012 constructs launch-window state; 2039/2046 mutate/assert process-loss state; 2178 backdates cap fields; 2266 checks mirror hashes; 2296 checks follow-up chain/usage/mirror; 2314 nulls session; 2373/2381 mutate `diffBoundary`; 2414 mutates an old snapshot; 2424 checks fresh follow-up context; 2446 adds a waiver. Those compound/mutation tasks have no current generic verb.

> `scripts/validate-metasystem.sh:1900:  python3 - "$agent_repo" <<'PY'`  
> `scripts/validate-metasystem.sh:1959:  python3 - "$agent_repo/artifacts/agents/jobs/flag-runtime.json" <<'PY'`  
> `scripts/validate-metasystem.sh:1971:  [[ "$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["status"])' "$agent_repo/artifacts/agents/jobs/no-signal.json")" == failed ]] \`  
> `scripts/validate-metasystem.sh:2012:  python3 - "$agent_repo/artifacts/agents/jobs/happy.json" "$launch_window_source" "$launch_window_pending" <<'PY'`  
> `scripts/validate-metasystem.sh:2037:  [[ "$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["status"])' "$agent_repo/artifacts/agents/jobs/launch-window.json")" == pending ]] \`  
> `scripts/validate-metasystem.sh:2039:  python3 - "$agent_repo/artifacts/agents/jobs/launch-window.json" <<'PY'`  
> `scripts/validate-metasystem.sh:2046:  python3 - "$agent_repo/artifacts/agents/jobs/launch-window.json" <<'PY'`  
> `scripts/validate-metasystem.sh:2088:  python3 -m json.tool "$agent_repo/artifacts/agents/jobs/interrupted.json" >/dev/null`  
> `scripts/validate-metasystem.sh:2119:  [[ "$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["network"])' scripts/agents/permissions/workspace.json)" == allow ]] \`  
> `scripts/validate-metasystem.sh:2121:  [[ "$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["network"])' scripts/agents/permissions/none.json)" == allow ]] \`  
> `scripts/validate-metasystem.sh:2126:  [[ "$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["permissions"]["requested"]["network"])' "$agent_repo/artifacts/agents/jobs/net-default.json")" == allow ]] \`  
> `scripts/validate-metasystem.sh:2132:  [[ "$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["permissions"]["requested"]["network"])' "$agent_repo/artifacts/agents/jobs/net-floor.json")" == deny ]] \`  
> `scripts/validate-metasystem.sh:2178:  python3 - "$agent_repo/artifacts/agents/jobs/timed.json" <<'PY'`  
> `scripts/validate-metasystem.sh:2258:  if [[ "$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["mirror"])' "$agent_repo/artifacts/agents/jobs/mirror-retry.json")" == None ]]; then`  
> `scripts/validate-metasystem.sh:2261:  mirror_hash_before=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["mirror"]["manifest"])' "$agent_repo/artifacts/agents/jobs/mirror-retry.json")`  
> `scripts/validate-metasystem.sh:2263:  mirror_hash_after=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["mirror"]["manifest"])' "$agent_repo/artifacts/agents/jobs/mirror-retry.json")`  
> `scripts/validate-metasystem.sh:2266:  python3 - "$agent_repo/artifacts/agents/jobs/mirror-retry.json" <<'PY'`  
> `scripts/validate-metasystem.sh:2296:  python3 - "$agent_repo" <<'PY'`  
> `scripts/validate-metasystem.sh:2314:  python3 - "$agent_repo/artifacts/agents/jobs/default-role.json" <<'PY'`  
> `scripts/validate-metasystem.sh:2342:  [[ "$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["runnerClosed"])' "$agent_repo/artifacts/agents/jobs/happy.json")" == False ]] \`  
> `scripts/validate-metasystem.sh:2369:  conformance_workspace=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["workspaceRoot"])' "$agent_repo/artifacts/agents/jobs/conformance.json")`  
> `scripts/validate-metasystem.sh:2373:  python3 - "$agent_repo/artifacts/agents/conformance/rounds/1/return.json" source.txt <<'PY'`  
> `scripts/validate-metasystem.sh:2381:  python3 - "$agent_repo/artifacts/agents/conformance/rounds/1/return.json" source.txt plans/delegate.md <<'PY'`  
> `scripts/validate-metasystem.sh:2414:  python3 - "$agent_repo/artifacts/agents/jobs/old-capabilities.json" <<'PY'`  
> `scripts/validate-metasystem.sh:2424:  python3 - "$agent_repo" <<'PY'`  
> `scripts/validate-metasystem.sh:2446:  python3 - "$requirements" <<'PY'`

**F Q4.19.** `validate-metasystem.sh` lines 2531–2835 contain twelve invocations: 2531 validates escalation approval record/output; 2583 stamps mission-fence fixture hashes; 2613 and 2669 write mission lease/identity fixtures; 2644 validates mission usage/prompts; 2740 backdates timeout fields; 2754 prints diagnostics—its fields are individually answerable by `json get`; 2761 seeds another-provider usage; 2767 validates typed mission usage; 2778 reads mission—yes, `json get`; 2824 reads `completedAtEpoch`—yes, `json get`; 2835 writes process identity from supervision state. The remaining compound writes/assertions have no named generic verb.

> `scripts/validate-metasystem.sh:2531:  python3 - "$agent_repo/artifacts/agents/jobs/escalation-approved.json" "$agent_fixture/escalation-approved.out" <<'PY'`  
> `scripts/validate-metasystem.sh:2583:    python3 - "$agent_repo/plans/mission-$1.contract.md" "$agent_repo/artifacts/agents/missions/$1/fences.json" "$1" <<'PY_STAMP'`  
> `scripts/validate-metasystem.sh:2613:  python3 - "$agent_repo/artifacts/agents/missions/mission-alpha/lease.json" "$mission_identity" "$mission_pid" "$mission_pgid" <<'PY'`  
> `scripts/validate-metasystem.sh:2644:  python3 - "$agent_repo" <<'PY'`  
> `scripts/validate-metasystem.sh:2669:    python3 - "$mission_dir/lease.json" "$mission" "$mission_pid" "$mission_pgid" <<'PY'`  
> `scripts/validate-metasystem.sh:2740:  python3 - "$agent_repo/artifacts/agents/jobs/mission-timeout-job.json" <<'PY'`  
> `scripts/validate-metasystem.sh:2754:    python3 -c 'import json,sys;v=json.load(open(sys.argv[1]));print("status:",v.get("status"),"error:",v.get("error"),"phase:",v.get("phase"))' "$agent_repo/artifacts/agents/jobs/mission-timeout-job.json" >&2 2>/dev/null || true`  
> `scripts/validate-metasystem.sh:2761:  python3 - "$agent_repo/artifacts/agents/jobs/other-provider.json" <<'PY'`  
> `scripts/validate-metasystem.sh:2767:  python3 - "$agent_repo/artifacts/agents/missions/mission-alpha/usage.json" <<'PY'`  
> `scripts/validate-metasystem.sh:2778:  [[ "$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["mission"])' "$agent_repo/artifacts/agents/jobs/happy.json")" == None ]] \`  
> `scripts/validate-metasystem.sh:2824:  completed=$(python3 - "$fixture_root/artifacts/agents/supervision/last-census.json" <<'PY' 2>/dev/null || true`  
> `scripts/validate-metasystem.sh:2835:  python3 - "$fixture_root/artifacts/agents/supervision/state.json" \`

**F Q4.20.** `validate-metasystem.sh` lines 2989–3334 contain ten invocations: 2989 is an atomic-result watcher; 3027 validates runner prompt header/preamble; 3096 is the body of a generated fake Codex host; 3147 extracts host PID/tag—answerable as two `json get` calls; 3163 locates cycle two; 3180 checks host argv/session/usage; 3248 checks ghost rejection/ask; 3266 checks fence parking; 3279 extracts an unverified ask; 3334 validates fake selftest records. No current validate/report verb performs the compound runner assertions.

> `scripts/validate-metasystem.sh:2989:    python3 - "$result_path" "$agent_fixture_cap_sec" "$METASYSTEM_FIXTURE_POLL_INTERVAL_MS" \`  
> `scripts/validate-metasystem.sh:3027:  python3 - "$runner_repo" "$cycle_turn/prompt.md" <<'PY'`  
> `scripts/validate-metasystem.sh:3096:python3 - "$prompt" "$output" "$sequence" <<'PY'`  
> `scripts/validate-metasystem.sh:3147:  read -r codex_host_pid codex_host_tag < <(python3 - "$codex_turn_one/turn.json" <<'PY'`  
> `scripts/validate-metasystem.sh:3163:  codex_turn_two=$(python3 - "$runner_repo/artifacts/agents/missions/runner-codex/turns" <<'PY'`  
> `scripts/validate-metasystem.sh:3180:  python3 - "$runner_repo" "$codex_host_fixture" "$codex_turn_one" "$codex_turn_two" <<'PY'`  
> `scripts/validate-metasystem.sh:3248:  python3 - "$runner_repo/artifacts/agents/missions/runner-ghost" <<'PY'`  
> `scripts/validate-metasystem.sh:3266:  python3 - "$runner_repo/artifacts/agents/missions/runner-fence" <<'PY'`  
> `scripts/validate-metasystem.sh:3279:  unverified_ask=$(python3 - "$runner_repo/artifacts/agents/missions/runner-unverified" <<'PY'`  
> `scripts/validate-metasystem.sh:3334:  python3 - "$agent_selftest_repo/artifacts/agents/selftests" <<'PY'`

**F Q4.21.** `validate-metasystem.sh` lines 3368–4260 contain six actual invocations: 3368 extracts the Stop-hook command—`json get` can answer; 3724 seeds receipt-relation records—no; 3969 sets `chainClosed`—`json set` is adjacent, but it is not a get/validate/schema/report operation; 4232, 4251, and 4260 backdate watcher fixture files—no.

> `scripts/validate-metasystem.sh:3368:  hook_cmd=$(python3 -c "import json; print(json.load(open('$hooks_json'))['hooks']['Stop'][0]['hooks'][0]['command'])")`  
> `scripts/validate-metasystem.sh:3724:python3 - "$receipt_relation/artifacts/agents/jobs" <<'PY'`  
> `scripts/validate-metasystem.sh:3969:  python3 - "$chain_root/artifacts/agents/jobs/implementer-20260101t000000z-cccc.json" <<'PYEOF'`  
> `scripts/validate-metasystem.sh:4232:python3 - "$wbj/jobs/live.json" <<'AGE'`  
> `scripts/validate-metasystem.sh:4251:python3 - "$wbj/live-log/jobs/busy.json" <<'AGE'`  
> `scripts/validate-metasystem.sh:4260:python3 - "$wbj/live-log/jobs/busy.log" <<'AGE'`

## Q5 — Go family surfaces

**F Q5.1.** The binary currently advertises 27 families. Identity through host have this complete verb-to-handler-file surface:

- `identity`: `started-at`, `probe` → `cmd/metasystem/identity.go`; `exists`, `group-exists` → `identity_probes.go`.
- `census`: `classify`, `fingerprint`, `run`, `alive`, `authentication-identity`, `signature-check`, `find-ancestor` → `census.go`.
- `capability`: `select` → `capability.go`.
- `config`: `canonical-model`, `identity` → `config.go`; `get`, `validate`, `keys`, `conf-value` → `config_verbs.go`; `tailor` → `validate_verbs.go`.
- `validate`: `turn-prompt`, `plan-consistency`, `critique-closed`, `preamble-quotes`, `code-critique-claim`, `waiver-facts`, `wrapper-token`, `session-isolation`, `return-complete` → `validate_verbs.go`.
- `dispatch`: all 24 verbs from `record-create` through `owner-lock` → `dispatch_verbs.go`.
- `adapter`: `root-job` through `capability-snapshot` → `adapter_verbs.go`; runtime-specific verbs from `version-parse` through `usage-unavailable` → `adapter_runtime_verbs.go`; `normalize-return` and selftest verbs → `adapter_selftest_verbs.go`.
- `host`: `result-write`, `json-compact`, `claude-result`, `devin-config`, `devin-return`, `devin-usage`, `fake-return`, `fake-result` → `host_verbs.go`.

> `cmd/metasystem/main.go:30:			name:    "identity",`  
> `cmd/metasystem/main.go:40:			name:    "census",`  
> `cmd/metasystem/main.go:53:			name:    "capability",`  
> `cmd/metasystem/main.go:60:			name:    "config",`  
> `cmd/metasystem/main.go:73:			name:    "validate",`  
> `cmd/metasystem/main.go:88:			name:    "dispatch",`  
> `cmd/metasystem/main.go:118:			name:    "adapter",`  
> `cmd/metasystem/main.go:158:			name:    "host",`  
> `cmd/metasystem/dispatch_verbs.go:36:func runDispatchRecordCreate(args []string) int {`  
> `cmd/metasystem/adapter_runtime_verbs.go:20:func runAdapterVersionParse(args []string) int {`  
> `cmd/metasystem/host_verbs.go:19:func runHostResultWrite(args []string) int {`

**F Q5.2.** The exact `dispatch` verb surface is `record-create`, `record-setup`, `record-cas`, `record-protocol-error`, `build-setup`, `build-record`, `build-follow-record`, `latest-chain-record`, `chain-members`, `chain-usage`, `custody-add`, `handshake-eval`, `reap-facts`, `census-fresh`, `watcher-ceiling`, `expand-permissions`, `validate-mission`, `mirror`, `close-check`, `critique-exhaustion`, `exhaustion-patches`, `cap-resolution`, `brief-mode`, and `owner-lock`.

> `cmd/metasystem/main.go:91:				{"record-create", "reserve a job by writing its pending-setup record", runDispatchRecordCreate},`  
> `cmd/metasystem/main.go:101:				{"custody-add", "append a custody process to a job record under its lock", runDispatchCustodyAdd},`  
> `cmd/metasystem/main.go:109:				{"close-check", "validate a chain is closable", runDispatchCloseCheck},`  
> `cmd/metasystem/main.go:114:				{"owner-lock", "claim or release the dispatch owner lock (0 done, 3 busy, 4 not-owner)", runDispatchOwnerLock},`

**F Q5.3.** The exact `adapter` verb surface is `root-job`, `effective-init`, `effective-workspace`, `permission-check`, `model-patch`, `repairs-patch`, `result-patch`, `capability-snapshot`, `version-parse`, `codex-event`, `codex-usage`, `codex-command`, `claude-settings`, `claude-usage`, `claude-result-field`, `claude-read-roots`, `claude-append-result`, `claude-session-signal`, `devin-config`, `devin-session`, `devin-usage`, `usage-unavailable`, `fake-return`, `fake-usage`, `fake-effective-network`, `fake-guarded-write`, `fake-guarded-network`, `fake-capability-snapshot`, `fake-selftest-record`, `normalize-return`, `selftest-usage`, `selftest-envelope`, `selftest-record`, and `selftest-listener`.

> `cmd/metasystem/main.go:121:				{"root-job", "print a job's root ancestor by walking parentJob", runAdapterRootJob},`  
> `cmd/metasystem/main.go:129:				{"version-parse", "extract the semver from CLI version output on stdin", runAdapterVersionParse},`  
> `cmd/metasystem/main.go:143:				{"fake-return", "write the fake runtime's canned role return", runAdapterFakeReturn},`  
> `cmd/metasystem/main.go:154:				{"selftest-listener", "one-shot loopback listener for the denied-fetch probe", runAdapterSelftestListener},`

**F Q5.4.** Gate through lease have this complete surface:

- `gate`: `register`, `check`, `fence` → `gate.go`.
- `authority`: `check` → `authority.go`.
- `report`: `stop-block`, `open-work` → `report.go`.
- `schema`: `materialize` → `schema.go`.
- `hooks`: `check` → `hooks.go`.
- `util`: `token-hex`, `slug`, `json-validate`, `now-ns` → `util.go`; `hold` → `util_hold.go`.
- `event`: `emit` → `event.go`.
- `json`: `get`, `object`, `set` → `json.go`.
- `lease`: `announce`, `retire`, `classify`, `require-holder`, `renew`, `run-held`, `protocol-growth`, `protocol-advance`, `commit-token`, `reclaim` → `lease.go`.

> `cmd/metasystem/main.go:172:			name:    "gate",`  
> `cmd/metasystem/main.go:181:			name:    "authority",`  
> `cmd/metasystem/main.go:188:			name:    "report",`  
> `cmd/metasystem/main.go:196:			name:    "schema",`  
> `cmd/metasystem/main.go:203:			name:    "hooks",`  
> `cmd/metasystem/main.go:210:			name:    "util",`  
> `cmd/metasystem/main.go:221:			name:    "event",`  
> `cmd/metasystem/main.go:228:			name:    "json",`  
> `cmd/metasystem/main.go:237:			name:    "lease",`

**F Q5.5.** Mission and evidence families have this complete surface:

- `mission-state`: `init`, `write`, `verify`, `anchor`, `reconcile` → `mission.go`.
- `mission-fence`: `check-job`, `reserve-job`, `reserve-cycle`, `authorize-cap`, `aggregate-usage`, `refuse` → `mission.go`.
- `mission-contract`: `validate`, `seal`, `preflight` → `mission_contract.go`; `measure`, `envelope-allows` → `mission_contract_verbs.go`.
- `mission-prompt`: `assemble` → `mission_prompt.go`.
- `mission-runner`: `start`, `resume`, `status`, `answer`, `run-loop` → `missionrunner_verbs.go`.
- `evidence`: `gc` → `evidence_verbs.go`.
- `mission-turn`: `adjudicate`, `conclude`, `record-failure`, `park` → `missionrunner_verbs.go`.
- `mission-jobs`: `drain`, `close-chains` → `missionrunner_verbs.go`.
- `mission-ledger`: `init`, `append`, `verify`, `count` → `mission.go`.

> `cmd/metasystem/main.go:253:			name:    "mission-state",`  
> `cmd/metasystem/main.go:264:			name:    "mission-fence",`  
> `cmd/metasystem/main.go:276:			name:    "mission-contract",`  
> `cmd/metasystem/main.go:287:			name:    "mission-prompt",`  
> `cmd/metasystem/main.go:294:			name:    "mission-runner",`  
> `cmd/metasystem/main.go:305:			name:    "evidence",`  
> `cmd/metasystem/main.go:312:			name:    "mission-turn",`  
> `cmd/metasystem/main.go:322:			name:    "mission-jobs",`  
> `cmd/metasystem/main.go:330:			name:    "mission-ledger",`

**F Q5.6.** `supervise` is the final family. Its complete surface is `owner` → `supervise_owner.go`; `component` → `supervise_component.go`; `status` → `supervise.go`; `blocking-reserved-cap`, `write-owner-identity`, `component-identity`, `launch-detached`, `watchdog-report`, `heartbeat` → `supervise_arming.go`; `watcher-pass` → `supervise_watcherpass.go`.

> `cmd/metasystem/main.go:340:			name:    "supervise",`  
> `cmd/metasystem/main.go:343:				{"owner", "run the owner loop for a checkout (internal; launched by arm)", runSuperviseOwnerLoop},`  
> `cmd/metasystem/main.go:350:				{"watchdog-report", "report a stale census, untracked processes, and dead components", runSuperviseWatchdogReport},`  
> `cmd/metasystem/main.go:352:				{"watcher-pass", "run one census pass as a standalone writer under the census lock", runSuperviseWatcherPass},`  
> `cmd/metasystem/supervise_arming.go:276:func runSuperviseWatchdogReport(args []string) int {`

**F Q5.7.** Internal package backing maps as follows: identity→`internal/identity`; census→`internal/census`; capability→`internal/capability`; config→`internal/config`; validate→`internal/validate`; dispatch→`internal/dispatch`; adapter→`internal/adapter`; host→`internal/host`; gate→`internal/gaterun`; authority→`internal/authority`; report→`internal/report`; schema→`internal/returnschema`; hooks→`internal/hooks`; event→`internal/events`; lease→`internal/lease`; mission state/fence/contract/prompt/ledger→`internal/mission`; mission runner/turn/jobs→`internal/missionrunner`; evidence→`internal/evidence`; supervise→`internal/supervise` plus census/config/identity/lock/registry. JSON and util use command-package/standard-library code rather than an internal family package.

> `cmd/metasystem/census.go:12:	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"`  
> `cmd/metasystem/dispatch_verbs.go:11:	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"`  
> `cmd/metasystem/validate_verbs.go:9:	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"`  
> `cmd/metasystem/missionrunner_verbs.go:10:	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner"`  
> `cmd/metasystem/supervise_owner.go:14:	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"`  
> `cmd/metasystem/supervise_owner.go:15:	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"`

**F Q5.8.** Current ownership for the requested prospective concepts is: conformance assertion has no `conformance` family/verb, while whole-artifact assertions belong to `validate`/`internal/validate`; receipt has no `receipt` family, but `receipt.sh` already calls two validate verbs; frontier has no family/verb/internal package; audit has no family/verb/internal package; adoption has no `adopt` family, while `config tailor` already performs adoption-time config rewriting; watchdog classification already has `supervise watchdog-report` in `internal/supervise`; fixture assertions have no fixture/assert family and currently live in shell fixtures and Go package tests.

> `cmd/metasystem/main.go:74:			summary: "whole-artifact validators the assert scripts exec into",`  
> `cmd/metasystem/main.go:80:				{"code-critique-claim", "verify a receipt's code-critique delegate claim", runValidateCodeCritiqueClaim},`  
> `cmd/metasystem/main.go:81:				{"waiver-facts", "print an implementer delegate's critique-waiver class and mission stream", runValidateWaiverFacts},`  
> `scripts/receipt.sh:100:    "$ms" validate code-critique-claim --job "$job" --result "$result" --runtime "$runtime"`  
> `scripts/receipt.sh:107:    waiver_facts=$("$ms" validate waiver-facts --job "$job")`  
> `cmd/metasystem/main.go:69:				{"tailor", "rewrite metasystem.conf in place for a selected runtime set", runConfigTailor},`  
> `scripts/adopt.sh:197:  "$staging/bin/metasystem" config tailor --file "$staging/metasystem.conf" --runtimes "$runtime_csv"`  
> `cmd/metasystem/main.go:350:				{"watchdog-report", "report a stale census, untracked processes, and dead components", runSuperviseWatchdogReport},`

**F Q5.9.** The main router states the binary/shell ownership contract: families group decisions invoked by shell wrappers, while wrappers retain historical names and exec into verbs.

> `cmd/metasystem/main.go:1:// Command metasystem is the metasystem's one binary: each family`  
> `cmd/metasystem/main.go:2:// groups the decisions the shell wrappers invoke, exposed as git-style`  
> `cmd/metasystem/main.go:3:// verbs. Wrappers keep their historical names and exec into these`  
> `cmd/metasystem/main.go:4:// verbs.`

## Q6 — suite structure

**F Q6.1.** `validate-metasystem.sh` executes these logical sections in order:

1. Argument/delegate-scope setup, lines 1–29.
2. Root and gate registration, 30–59.
3. Full-scope Go gate/supervision/fence, 61–101.
4. Fixture-budget initialization, audit, and skill validation, 103–135.
5. Mode/routed/protocol asset checks, 137–234.
6. Process-owning fixture group, 236–249.
7. Shell syntax and spawned fixture suites, 251–299.
8. Configuration/adapter/host/profile static checks, 300–465.
9. Main temp directory, fake probe, and cleanup trap, 467–550.
10. Audit fallback/template/protocol/preamble checks, 552–813.
11. Return-schema and turn-prompt fixtures, 814–1155.
12. Critique closure and plan consistency, 1156–1354.
13. Job-mode return identity, 1355–1439.
14. Dispatcher/adapter/mission-runner process fixtures, 1440–3355.
15. Hook and optional-preflight checks, 3357–3404.
16. Obligation/frontier/stop-loss/refactor/receipt fixtures, 3406–3820.
17. Adopted-mode and adopt selftests, 3821–4214.
18. Watcher fixtures and final scope accounting, 4216–4329.

> `scripts/validate-metasystem.sh:61:# The Go engine gate runs first (plans/go-migration.md): gofmt, vet, the`  
> `scripts/validate-metasystem.sh:122:# Validate every skill present, including project-added and moved optional`  
> `scripts/validate-metasystem.sh:236:# Section 3.11 and retained watch-list round S4 have one bounded fixture suite.`  
> `scripts/validate-metasystem.sh:251:# Real runtime selftests spend model calls and remain manual acceptance steps.`  
> `scripts/validate-metasystem.sh:470:# The fake runtime is the only sandbox this suite owns. Its probe drives the`  
> `scripts/validate-metasystem.sh:552:# IL-3: prove the audit's fallback with a PATH that contains its ordinary POSIX`  
> `scripts/validate-metasystem.sh:814:# Build canonical positive returns and one role-specific negative per role.`  
> `scripts/validate-metasystem.sh:1156:# Critique closure joins the canonical return JSON against the one Markdown`  
> `scripts/validate-metasystem.sh:1355:# Job mode derives the schema and return path from the record and then checks`  
> `scripts/validate-metasystem.sh:1440:# Dispatcher and fake-adapter fixtures run in a minimal adopted-mode Git`  
> `scripts/validate-metasystem.sh:3357:# The shipped Stop hook must stay rooted and surface via JSON output: hooks`  
> `scripts/validate-metasystem.sh:3821:# Adopted-mode contract: a copy without the template marker validates with a`  
> `scripts/validate-metasystem.sh:4216:# watch-background-jobs: all four reportable states plus baseline suppression.`

**F Q6.2.** `validate-metasystem.sh` sources only `fixture-budget.sh`; it does not source a `*-fixtures.sh` file. Fixture suites are child processes. `bash -n` at lines 253–281 is an additional syntax invocation, not sourcing.

> `scripts/validate-metasystem.sh:103:source scripts/agents/fixture-budget.sh`  
> `scripts/validate-metasystem.sh:253:bash -n scripts/agents/arm-supervision.sh`  
> `scripts/validate-metasystem.sh:281:bash -n scripts/agents/adapters/runtime-common.sh`  
> `scripts/validate-metasystem.sh:282:bash scripts/agents/conformance-fixtures.sh`

**F Q6.3.** The suite’s own EXIT trap is installed at line 550 after temporary-directory creation and fake probing. Cleanup removes the gate marker, shuts down every tracked armed repository, waits boundedly for processes referencing the suite temp directory, preserves the temp tree under `artifacts/agents/suite-failures` on failure, and deletes it on success.

> `scripts/validate-metasystem.sh:467:tmp=$(mktemp -d)`  
> `scripts/validate-metasystem.sh:504:armed_supervision_repos=()`  
> `scripts/validate-metasystem.sh:513:validation_cleanup() {`  
> `scripts/validate-metasystem.sh:516:  for repo in ${armed_supervision_repos[@]+"${armed_supervision_repos[@]}"}; do`  
> `scripts/validate-metasystem.sh:540:  if [[ "${validation_exit_status:-1}" != 0 && -d "$tmp" ]]; then`  
> `scripts/validate-metasystem.sh:548:  rm -rf "$tmp" 2>/dev/null || { sleep 1; rm -rf "$tmp" 2>/dev/null || true; }`  
> `scripts/validate-metasystem.sh:550:trap 'validation_exit_status=$?; validation_cleanup' EXIT`

**F Q6.4.** Every temporary/process fixture except `authority-regression-fixtures.sh` has an EXIT trap. Simple removal traps are conformance 6, telemetry 6, return-schema 8, config-identity 10, pre-commit 6, record 6, evidence 6, second-session 8, lease 27, and flight 11. Process-aware cleanup functions are supervision 96, fingerprint 59, mission 43, delegate-caps 23, and supervision-go 22. `authority-regression-fixtures.sh` has no temp directory or trap; it reads checkout sources and invokes the authority binary.

> `scripts/agents/conformance-fixtures.sh:6:trap 'rm -rf "$fixture_root"' EXIT`  
> `scripts/agents/telemetry-census-fixtures.sh:6:trap 'rm -rf -- "$tmp"' EXIT`  
> `scripts/agents/return-schema-fixtures.sh:8:trap 'rm -rf "$fixture"' EXIT`  
> `scripts/agents/config-identity-fixtures.sh:10:trap 'rm -rf "$tmp"' EXIT`  
> `scripts/agents/pre-commit-guard-fixtures.sh:6:trap 'rm -rf "$tmp"' EXIT`  
> `scripts/agents/record-protocol-fixtures.sh:6:trap 'rm -rf "$tmp"' EXIT`  
> `scripts/agents/evidence-segment-fixtures.sh:6:trap 'rm -rf "$tmp"' EXIT`  
> `scripts/agents/second-session-fixtures.sh:8:trap 'rm -rf "$tmp"' EXIT`  
> `scripts/agents/lease-succession-fixtures.sh:27:trap 'rm -rf "$tmp"' EXIT`  
> `scripts/agents/flight-recorder-fixtures.sh:11:trap 'rm -rf "$tmp"' EXIT`  
> `scripts/agents/supervision-fixtures.sh:96:trap cleanup EXIT`  
> `scripts/agents/fingerprint-harness.sh:59:trap cleanup EXIT`  
> `scripts/agents/mission-fixtures.sh:43:trap cleanup EXIT`  
> `scripts/agents/delegate-caps-fixtures.sh:23:trap cleanup EXIT`  
> `scripts/agents/supervision-go-fixtures.sh:22:trap cleanup EXIT`  
> `scripts/agents/authority-regression-fixtures.sh:16:root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)`

**F Q6.5.** Spawned fixture scripts do not share a temporary directory or fixture repository: each creates its own `mktemp` root, and process-owning groups are explicitly serial with separate repositories. State is shared only within an individual fixture process, such as `mission-fixtures.sh`’s repo and remote below one `fixture_root`. `authority-regression-fixtures.sh` uses the source root read-only.

> `scripts/validate-metasystem.sh:237:# Process-owning groups run serially and use separate temporary repositories,`  
> `scripts/validate-metasystem.sh:238:# so their supervisors and dispatch jobs cannot share lifecycle state. They`  
> `scripts/agents/conformance-fixtures.sh:5:fixture_root=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-conformance-fixtures.XXXXXX")`  
> `scripts/agents/supervision-fixtures.sh:12:tmp=$(mktemp -d)`  
> `scripts/agents/fingerprint-harness.sh:45:repo=$tmp/repo`  
> `scripts/agents/mission-fixtures.sh:10:fixture_root=$(mktemp -d)`  
> `scripts/agents/mission-fixtures.sh:11:repo=$fixture_root/repo`  
> `scripts/agents/mission-fixtures.sh:12:remote=$fixture_root/origin.git`

**F Q6.6.** Inline dispatcher/adapter/mission-runner fixtures share the suite temp directory. They create `agent_fixture`, `agent_repo`, and `agent_evidence` below it, later create runner state below the same temp directory, and register armed repositories in the shared `armed_supervision_repos` array.

> `scripts/validate-metasystem.sh:1448:  agent_fixture="$tmp/agent-fixture"`  
> `scripts/validate-metasystem.sh:1449:  agent_repo="$agent_fixture/repo"`  
> `scripts/validate-metasystem.sh:1450:  agent_evidence="$agent_fixture/evidence"`  
> `scripts/validate-metasystem.sh:504:armed_supervision_repos=()`  
> `scripts/validate-metasystem.sh:511:  armed_supervision_repos+=("$repo")`  
> `scripts/validate-metasystem.sh:518:    if [[ "$repo" == "${runner_repo:-}" ]] && declare -p runner_process_env >/dev/null 2>&1; then`

**F Q6.7.** Fixture budgets are shared through exported environment. The suite sources `fixture-budget.sh` and calls `harness_fixture_budget_init`. Delegate scope pre-sets scale 3 so initialization does not run a census. Initialization exports scale plus poll, census, watcher, heartbeat, and handshake intervals. Some child process fixtures source and initialize the same policy again.

> `scripts/validate-metasystem.sh:103:source scripts/agents/fixture-budget.sh`  
> `scripts/validate-metasystem.sh:104:if (( delegate_scope )); then`  
> `scripts/validate-metasystem.sh:108:  : "${METASYSTEM_FIXTURE_CAP_SCALE:=3}"`  
> `scripts/validate-metasystem.sh:111:harness_fixture_budget_init "$root"`  
> `scripts/agents/fixture-budget.sh:105:  read -r METASYSTEM_FIXTURE_CAP_SCALE METASYSTEM_FIXTURE_CAP_SCALE_MILLI <<EOF`  
> `scripts/agents/fixture-budget.sh:108:  export METASYSTEM_FIXTURE_CAP_SCALE METASYSTEM_FIXTURE_CAP_SCALE_MILLI`  
> `scripts/agents/fixture-budget.sh:128:  export METASYSTEM_FIXTURE_POLL_INTERVAL_MS METASYSTEM_FIXTURE_POLL_INTERVAL_SEC \`  
> `scripts/agents/mission-fixtures.sh:5:source "$root/scripts/agents/fixture-budget.sh"`  
> `scripts/agents/mission-fixtures.sh:6:harness_fixture_budget_init "$root"`

**F Q6.8.** Delegate scope skips exactly three named process-visibility sections through `delegate_process_section`: supervision/census fixtures, fingerprint heal harness, and the combined dispatcher/adapter-selftest/mission-runner block. The Go engine section is also excluded by a separate direct `! delegate_scope` guard. `mission-fixtures.sh` and the static/spawned fixtures at 251–299 still run. The final delegate block verifies that the skipped list equals the owed list and prints it.

> `scripts/validate-metasystem.sh:16:delegate_owed_sections=(`  
> `scripts/validate-metasystem.sh:17:  "supervision and census fixtures"`  
> `scripts/validate-metasystem.sh:18:  "supervisor fingerprint heal harness"`  
> `scripts/validate-metasystem.sh:19:  "dispatcher, adapter selftest, and mission-runner process fixtures"`  
> `scripts/validate-metasystem.sh:71:if (( ! delegate_scope )) && [[ -f go.mod ]]; then`  
> `scripts/validate-metasystem.sh:241:if [[ -z "${METASYSTEM_SKIP_AGENT_FIXTURES:-}" || $delegate_scope -eq 1 ]]; then`  
> `scripts/validate-metasystem.sh:248:  scripts/agents/mission-fixtures.sh`  
> `scripts/validate-metasystem.sh:1446:if delegate_process_section "dispatcher, adapter selftest, and mission-runner process fixtures" \`  
> `scripts/validate-metasystem.sh:4317:if (( delegate_scope )); then`  
> `scripts/validate-metasystem.sh:4318:  [[ ${#delegate_skipped_sections[@]} -eq ${#delegate_owed_sections[@]} ]] \`  
> `scripts/validate-metasystem.sh:4325:  echo "orchestrator still owes these process-visibility sections:"`
