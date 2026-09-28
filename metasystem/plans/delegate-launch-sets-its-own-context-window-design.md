# delegate-launch-sets-its-own-context-window: build design

Revision: 1 (2026-09-16). Author: Claude Fable 5.1 design delegate, launched headless by seat m1e.
Goal: `delegate-launch-sets-its-own-context-window` (tier 3, priority 1, approved by Wido 2026-09-16 13:17Z).
Round boundary: the goal's DONE conditions (1) to (4), exactly. Section 8 lists what stays out.

## 1. Grounding: how the window reaches a delegate today

The auto-compaction threshold of a `claude` process is min(the `autoCompactWindow` setting, the
model's maximum window). Precedence, highest first: the environment variable
`CLAUDE_CODE_AUTO_COMPACT_WINDOW`, then a `--settings` file on the command line, then the checkout's
`.claude/settings.local.json` (seat facts, binary 2.1.273). Every seat writes 200000 there
(`.claude/settings.local.json:3`).

The engine already owns a per-delegate settings file. `BuildClaudeSettings`
(`metasystem/internal/adapter/claude.go:21-134`) derives tool lists and the sandbox from the job
record and writes the settings map at `claude.go:113-131`; that map carries no window. The shell
adapter writes it to `<round>/claude-settings.json` (`metasystem/scripts/agents/adapters/claude.sh:90-92`,
`:107`) and `BuildClaudeCommand` passes it as `--settings` (`claude.go:400-402`). The launch subshell
(`claude.sh:147-162`) changes into the workspace, exports the scratch and cache variables, and
`exec`s the argv with the packet on stdin and the stream on stdout. It sets no `CLAUDE_CODE_*`
variable, so the process inherits whatever the launching seat's environment holds: this very session
carries `CLAUDE_CODE_AUTO_COMPACT_WINDOW=1000000` from its launcher, and a `metasystem delegate` run
from it would hand that value to its child, above every settings file.

Which checkout cap a delegate inherits depends on where it runs. A read-only round (critic, warden,
verifier: `writeRoots` empty, `claude.go:365-384`) runs in the seat checkout and inherits 200000. A
worktree round runs under `.claude/worktrees/<name>`; none of the worktrees present on 2026-09-16
carries a `.claude/settings.local.json`, so a worktree implementer runs at the model maximum today.
The model maximum on this box is 1000000: every Claude result document reports it
(`artifacts/agents/code-critic-abb457065f1f63f2205a664a/rounds/1/claude-result.json`,
`modelUsage.claude-opus-5.contextWindow`), and the seat launcher writes the same figure
(`/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/5afd308c-3804-42b1-87c4-4547e1460fa9/scratchpad/fable-design-launch.sh:13-15`).

Roles are files plus rows: dispatch refuses a role without `scripts/agents/roles/<role>.md` and
`<role>.requirements.json` (`metasystem/scripts/agents/dispatch.sh:1490`); the roster is
`role.<role>.runtime` and `role.<role>.model.<runtime>` (`metasystem/metasystem.conf:76-93`,
`internal/dispatch/roster.go:152-235`); the permission preset is `dispatch.permissions.<role>`
(`metasystem.conf:95-99`, `dispatch.sh:1615-1619`); the return contract is
`scripts/agents/schemas/<role>.schema.json` (`internal/validate/returncomplete.go:171`) behind the
role tables at `returncomplete.go:26-36`. The critic roles are special-cased by name at
`internal/adapter/claude.go:313-316`, `internal/dispatch/build.go:608-633`, `admission.go:260-272`,
`budget.go:392-402` and `:431-441`, `critique.go:207-245`, `close.go:47` and `:225`,
`read_admission.go:266`, `hazard.go:284-296` and `:338-350`. There is no designer role. The design
authoring lane today is the implementer role in design mode (`metasystem.conf:86-87`,
`metasystem.conf.local:1-2`, `docs/orchestration.md:174`), and since 2026-09-16 the seat script above.

Evidence of context and compaction exists in two places. The stream artifact
`<round>/claude-stream.jsonl` carries one `assistant` line per call with `message.usage`
(`input_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens`; 151 such lines in the
round above) and `system` lines whose compaction subtype the binary spells `compact_boundary` with
`compact_metadata` and `pre_tokens` (binary 2.1.273 strings). The transcript
`<home>/.claude/projects/<slug>/<session>.jsonl` carries the same calls and the marker spelled
`compactMetadata.preTokens`; `internal/usage/calls_claude.go:47-113` already parses it, and the slug
rule is `calls_claude.go:37-46`. The result document's `usage` is a run total (`claude.go:176-194`),
no peak and no markers. The record's `usage` field is the typed run usage patched at completion
(`internal/adapter/patch.go:43-60`, `scripts/agents/adapters/runtime-common.sh:212-214`);
`ChainUsage` sums exactly four token names inside it (`internal/dispatch/usage.go:18-27`).

## 2. Q1: the window policy, its home and its values

Decision. The policy is a two-class table in the Claude adapter, next to `isClaudeCriticRole`:
`claudeContextWindowClass(role)` returns `maximum` for `designer`, `design-critic`, `code-critic`
and `warden`, and `norm` for every other role. The two numbers are configuration with built-in
defaults, resolved through `config.Get` exactly as the cap chain resolves its keys
(`internal/dispatch/cap.go:26-33`, `internal/config/resolve.go:50-57`):

| Key | Default | Meaning |
| --- | --- | --- |
| `dispatch.context-window.maximum` | 1000000 | written for the maximum class |
| `dispatch.context-window.norm` | 200000 | written for the norm class; the floor no delegate goes below |

`BuildClaudeSettings` gains the conf path (`adapter claude-settings --conf`, `cmd/metasystem/adapter_runtime_verbs.go:205-226`;
`claude.sh:91` passes `$root/metasystem.conf`) and writes `"autoCompactWindow": <value>` into the
settings map at `claude.go:113`. Both keys must be positive integers and norm must not exceed
maximum; a violation is a settings-build error, and `claude.sh:119` gains the same
`fail_pending runtime_error handshake` guard its neighbour at `:133-137` has.

Why 1000000 is safe. Under the min() rule an over-estimate never caps: the process compacts at the
model maximum whenever the setting is at or above it. 1000000 equals the maximum every model on this
box reports, so it is the model maximum for all of them; a future model with a larger window would be
clamped at 1000000 until the key is raised, which the evidence of section 4 would show as an auto
compaction. Rejected: a very large literal (unverified against the binary's validation) and reading
`modelUsage.contextWindow` (only known after the run).

The floor. Wido's rule is that no floor is lowered. The floor here is the seat norm, 200000: the
invariant is that an engine-launched delegate is never written a window below
`dispatch.context-window.norm`, and the maximum class only adds. The DONE text puts implementer
roles at the norm, and this design follows it, with one behaviour change named in the open: a
worktree implementer runs uncapped today (section 1) and will compact at the norm after unit 1. The
reason to accept it: every call re-reads the whole context, so an uncapped implementer at 300 calls
is the costliest shape the engine can launch, and an implementer's state is in its worktree and its
return, so a compaction there is recoverable, unlike a design's or a read's verbatim evidence. The
context evidence of section 4 makes every such compaction visible, and the norm key is one line to
raise. This is open question 1 for the seat (section 9).

Delegate-side settings. None exists and none is added: no `--context-window` flag on `delegate`
(no consumer; a seat that wants another value edits `metasystem.conf.local`). A local settings file
in the workspace is outranked by `--settings`; the settings file lives in the round directory, which
the sandbox denies to the delegate; and the environment is stripped: `CLAUDE_CODE_AUTO_COMPACT_WINDOW`
joins the blocked map of `delegateCommandEnvironment` (`cmd/metasystem/delegate.go:120-127`), the one
place every launch passes through (`delegate.go:28-30`, `:75-76`). Rejected: an `unset` in the launch
subshell at `claude.sh:147` (no shell test harness exists for the Claude adapter, while the Go
environment builder has one at `cmd/metasystem/delegate_test.go:149`); a policy in the job record
(the record carries the decision as evidence, section 4, not the rule); per-role conf keys (a
registry without a consumer, and a misspelt role would silently land in the wrong class); reusing
`context.*` keys (`internal/config/context.go:29-33` refuses unknown keys under that prefix, and that
owner is the seat's context law, not the delegate's).

## 3. Q2: the designer role

`metasystem delegate --role designer --brief <file> --goal <id> --destructive-reach DESIGN-BEARING --page metasystem/plans/<name>.md`

Where it runs. Always in a job worktree: the preset `scripts/agents/permissions/design.json` requests
writes, so `dispatch.sh:1628-1631` selects a worktree. Reads: the repository (`readRoots ["."]`).
Writes: exactly `"<worktree>/metasystem/plans"`. `ExpandPermissions` expands `<worktree>` only as a
whole value (`internal/dispatch/envelope.go:44-53`) and a relative root resolves against the
repository and is refused as an escape (`:71-78`), so the expander gains one case: a value with the
`<worktree>/` prefix joins the rest to the workspace. The Claude sandbox then denies every sibling of
the plans directory up to the worktree root (`claude.go:80-108`); the delegate gets the full tool
list and `acceptEdits` because its write roots are non-empty (`claude.go:43-56`, `:365-384`).
Bash: yes, for reading evidence (`bin/metasystem goal show`, `git log`, `grep`); the sandbox makes
it write-safe, the same argument the critic carries at `claude.go:370-373`. Network: allow, like the
`none` and `workspace` presets and unlike the critic. A critique must attack the page from the
repository alone; a design author's evidence may be a runtime's documented behaviour (this page
needed one such fact). Rejected: a read-only envelope with the page written to scratch and collected
through a named-file channel (a new output channel for one role, and the design critic reads its
subject as a committed blob anyway, `build.go:394-412`).

Brief and page. `--brief` is the file the seat fills from `scripts/agents/templates/design-brief.md`
(`docs/orchestration.md:24-25`). `--page` is repository-relative, must match
`^metasystem/plans/[A-Za-z0-9._-]+\.md$`, and is recorded on the job as `designPage`, an immutable
field (`internal/dispatch/record.go:67-86`, next to `design` at `:84-85`). Dispatch composes one
paragraph into the packet, headed `## Design page`, after the goal section and before the return
path form (next to the critic's declared-outputs paragraph, `dispatch.sh:1770-1780`): write exactly
one file, `<workspace>/<page>`; the return's `page` names it; write nothing else. Flags thread as
`--outputs` and `--design` do today: `delegate.go:226-400` (validity: `--page` only with the designer
role, and required by it), `dispatch.sh:1440-1451` and `:1536-1539`, `job build-record --page`
(`cmd/metasystem/dispatch_verbs.go:932-934`, `build.go:289-290`).

Completion. The process exits; the last `result` line of the stream becomes the result document
(`claude.go:425-456`); `normalize_return` writes `return.json` (`runtime-common.sh:364-369`);
`validate return-complete --role designer` validates it against `schemas/designer.schema.json`; the
record goes to `completed` with the section 4 evidence. `--wait` blocks the seat until then. The
designer commits nothing. The seat lifts the page with one copy from `<workspace>/<page>` into its
checkout, commits it with the `type=design` receipt it already owes (`orchestration.md:25`), and
dispatches the design critic against the committed blob as today. A revision is a fresh dispatch
with a new brief (`orchestration.md:25`, template section "Fresh session"); the follow-up path
refuses the designer role by name next to `dispatch.sh:2475-2487`.

Return contract, version 2 (the implementer's versioning, `returncomplete.go:32-36`,
`returnschema.VersionTwo`): exactly `schemaVersion` 2, `jobId`, `round`, `runtime`, `sessionId`,
`model`, `evidence`, `gaps`, `mode`, `page`, `words`, `status` (`ready` or `blocked`),
`blockedReason` (string or null), `openQuestions` (integer), plus `claimed`. The validator adds one
role check next to `checkDiffBoundary` (`returncomplete.go:239`): `page` equals the record's
`designPage`, and in job mode the worktree's changed paths (`git status --porcelain
--untracked-files=all`) are exactly that page. `designer` joins `returnAllowedRoles` and
`returnVersionedRoles`, not `VersionThreeRoles` (`internal/returnschema/returnschema.go:23-27`).

Roster and files. `roles/designer.md` (the packet: design, do not build; one page; the brief's
tool-call budget and word ceiling bind; return shape above; never a product byte; never a test run,
the seat's proof is the seat's) and `roles/designer.requirements.json` (the implementer's file
without the resume fallback). Committed conf: `role.designer.runtime=<runtime>`,
`role.designer.model.<runtime>=<model>` placeholders in the style of `metasystem.conf:90-91`,
`dispatch.permissions.designer=design`, and the two `mode.design.role.implementer.*` rows at
`metasystem.conf:86-87` deleted: the designer role is the design-author lane, one owner. The seat
sets `role.designer.runtime=claude` and `role.designer.model.claude=claude-fable-5-1` in
`metasystem.conf.local` and removes its two `mode.design` rows there (seat actions, not diff).
`docs/orchestration.md:174` is rewritten to name the role, and the modes table row at `:270` reads:
Design | the page, by a `designer` delegate; critique rounds (`design-critic`) | dispositions, the
obligation matrix, acceptance.

How the engine's critic special cases treat a designer: not a critic anywhere. `build.go:608` writes
no finding register and no `reviewChainCounted`, so `budget.go:392-402` never sees it and
`admission.go:260-272` adds no critique breach; it consumes one attempt and its cap minutes like any
job (`budget.go:443-448`). It is not a chain root of any critic kind (`close.go:225-227`,
`read_admission.go:266`); its chain has no close ceremony, as a verifier's has none
(`dispatch.sh:3018-3034` patches `chainClosed` for critic roles only; `close.go:99` skips
non-implementer members). `hazard.go:288` adds `designer` to the roles excluded from a chain's final
work state: a page is not product bytes, and nothing lands a designer chain, so the DESIGN-BEARING
obligations recorded on it (`hazard.go:43-47`) are discharged by the design-critic chain on the
committed page, as today. DESIGN-BEARING requires a maximal-effort proof for the runtime
(`hazard.go:119-131`); `runtime.claude.maximal-models` lists `claude-fable-5-1`
(`metasystem.conf.local:8`). A tier-1 goal refuses the class (`admission.go:233-235`), which is
right: tier 1 has no design round. `critique.go:244` is unreachable for a designer because the
exhaustion advance runs only for implementer and critic follow-ups (`dispatch.sh:2678-2686`).

## 4. Q3: context peak and compaction count on the record

Decision. Source: the round's own stream artifact. A new adapter function
`ClaudeContextEvidence(streamPath, settingsPath, outputPath)` behind `adapter claude-context`, run by
`claude.sh` right after `claude-derive-result` (`:184-188`), writes `<round>/context.json`:

```text
window            the autoCompactWindow read back from <round>/claude-settings.json
peakTokens        max over assistant lines of input_tokens + cache_creation_input_tokens + cache_read_input_tokens
samples           number of assistant lines counted
compactions       [{trigger, preTokens}] from every system line with subtype compact_boundary
autoCompactions   count of those with trigger auto
capped            autoCompactions >= 1
status            observed, or absent when the stream has no assistant usage line
```

Lines that belong to an in-process subagent are excluded, as the transcript reader excludes
`isSidechain` (`calls_claude.go:73-75`); on the stream the marker is a non-null `parent_tool_use_id`,
which the builder confirms on one stream that ran an Agent tool before relying on it. The reader
accepts both marker spellings (`compact_metadata.pre_tokens` on the stream, `compactMetadata.preTokens`
in a transcript) so the same function can read a transcript in the live witness.

The record. `WriteResultPatch` (`patch.go:43-60`) gains a context path and writes the object as the
record's `context` field; `adapter result-patch --context` (`cmd/metasystem/adapter_verbs.go:180-197`),
`write_patch` (`runtime-common.sh:212-214`) and the recollect path (`dispatch.sh:1356`) pass it. The
field is neither immutable nor dedicated (`record.go:67-86`, `:91-102`), so the ordinary result patch
carries it, and `ChainUsage` ignores it (`usage.go:23-27` sums only inside `usage`). The capped rule:
a run is capped when `autoCompactions` is at least one. A manual compaction is counted but does not
cap. An absent stream leaves `context` null with the failure the round already records. Rejected:
the transcript as the primary source (needs the home directory and a slug derivation, may be pruned,
and its cursor machinery serves the seat's live budget); the result document's usage (totals, no
peak, no markers); folding the numbers into `usage` (a peak must never be summed by `ChainUsage`).

## 5. Q4: witnesses

Go tests that fail without the fix:

| Test | Package and file | Assertion |
| --- | --- | --- |
| `TestBuildClaudeSettingsWritesContextWindowByRole` | `internal/adapter`, `runtime_test.go` | with an empty conf, the settings file of a designer, design-critic, code-critic and warden record carries `autoCompactWindow` 1000000 and an implementer or verifier record 200000; with `dispatch.context-window.norm=150000` the implementer gets 150000; the key is never absent |
| `TestBuildClaudeSettingsDesignerWritesOnlyThePlansRoot` | `internal/adapter`, `runtime_test.go` | `allowWrite` is exactly the plans root and scratch; `denyWrite` names the worktree's other entries |
| `TestDelegateCommandEnvironmentDropsInheritedContextWindow` | `cmd/metasystem`, `delegate_test.go` | `CLAUDE_CODE_AUTO_COMPACT_WINDOW=1` in the base environment is absent from the produced one |
| `TestClaudeContextEvidenceFromStream` | `internal/adapter`, new `claudecontext_test.go` | three usage lines and one subagent line: peak is the largest non-subagent sum; one auto and one manual marker: compactions 2, autoCompactions 1, capped true; an empty stream: status absent, capped false |
| `TestExpandPermissionsWorktreePrefixedRoot` | `internal/dispatch`, `envelope_worktree_test.go` | `<worktree>/metasystem/plans` lands inside the workspace; bare `metasystem/plans` is still refused |
| `TestDelegateDesignerRequiresPage` | `cmd/metasystem`, `delegate_test.go` | designer without `--page` refused; `--page` with another role refused; a page outside `metasystem/plans/` refused |
| `TestReturnCompleteDesignerPageMatchesRecord` | `internal/validate`, `returncomplete_test.go` | a return whose `page` differs from `designPage` is a violation; equal passes |

Mutations the builder proves, one per rule (each named test must go red under its mutation):
swap the two classes in the table; delete the `autoCompactWindow` write; drop the environment key
from the blocked map; sum instead of max for the peak; count manual markers as auto; treat the
`<worktree>/` prefix as a bare relative path; accept a designer without `--page`; skip the page
equality check; write a second file in the designer fixture (the changed-path check must refuse).

Live witness, the first real designer run (unit 3b's acceptance and unit 4's gate):

```text
job=<designer job id>; r=metasystem/artifacts/agents/$job/rounds/1
python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["autoCompactWindow"])' $r/claude-settings.json   # 1000000
grep -c '"subtype":"compact_boundary"' $r/claude-stream.jsonl                                                       # 0
sid=$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["session_id"])' $r/claude-result.json)
ws=$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["workspaceRoot"])' metasystem/artifacts/agents/jobs/$job.json)
grep -c '"subtype":"compact_boundary"' ~/.claude/projects/$(printf %s "$ws" | sed 's/[^A-Za-z0-9]/-/g')/$sid.jsonl   # 0
python3 -c 'import json,sys;c=json.load(open(sys.argv[1]))["context"];print(c["window"],c["peakTokens"],c["autoCompactions"],c["capped"])' metasystem/artifacts/agents/jobs/$job.json
```

The last line must print `1000000 <peak> 0 False`. The witness is conclusive when the peak is at or
above 200000; below it the run is consistent with the rule but does not exercise it, and the first
design that crosses 200000 completes the proof. Both counts must agree.

## 6. Q5: units in landing order

Each unit is one Codex build in a worktree with the tests above, then an Opus read.

| Unit | Content | Changed-line allocation | Witness |
| --- | --- | --- | --- |
| U1 window policy | `claude.go` class table, two keys with defaults, the `autoCompactWindow` write; `--conf` on `adapter claude-settings`; `claude.sh:91` and the `:113` guard; the blocked environment key in `delegate.go`; conf comment rows | 180 | the first three tests of section 5 |
| U2 context evidence | `ClaudeContextEvidence` and `adapter claude-context`; `context.json` in `claude.sh`; `--context` through `WriteResultPatch`, `adapter result-patch`, `write_patch`, the recollect call | 300 | `TestClaudeContextEvidenceFromStream`; a completed critic round shows `context` |
| U3a designer dispatch surface | `envelope.go` prefix case; `permissions/design.json`; `--page` in `delegate.go`, `dispatch.sh`, `job build-record`, `designPage` immutable; the packet paragraph; the follow-up refusal | 260 | expand, delegate and packet tests |
| U3b designer role files and contract | `roles/designer.md`, `designer.requirements.json`, `schemas/designer.schema.json`; the role tables and page check in `returncomplete.go`; `hazard.go:288`; conf rows and the `mode.design` deletion; `orchestration.md:174` and `:270` | 260 | return-complete test; the live witness |
| U4 retirement | one sentence in `AGENTS.md` (section 8); the seat deletes the scratch launcher and updates its memory note; conf.local rows moved | 20 | the live witness passed |

U1 lands first because it fixes every critic and read launched from a capped seat at once. U2
lands before U3 so the first designer run is measured. U4 retires the seat script: it lives in the
seat scratchpad, not the repository, so the retirement is the seat's deletion plus the doctrine
sentence, gated on the live witness.

## 7. Invariants and their owners

| Invariant | Owner |
| --- | --- |
| every Claude delegate settings file carries a positive `autoCompactWindow` at or above the norm | `BuildClaudeSettings` |
| the value depends only on the role class and the two keys; no checkout file, environment variable or delegate write changes it | `BuildClaudeSettings`, `delegateCommandEnvironment`, the sandbox |
| every Claude round that produced a stream records `context`; capped is exactly one or more auto compactions | `ClaudeContextEvidence`, `WriteResultPatch` |
| a designer round changes exactly its declared page under `metasystem/plans/` | the `design` preset, `ExpandPermissions`, the return check |
| a designer job is never a critic root, never review-chain counted, and consumes one attempt | `build.go:608`, `budget.go:392-448` unchanged |

## Moved effects

| Effect | From | To | Code |
| --- | --- | --- | --- |
| setting a delegate's compaction window | the checkout's `.claude/settings.local.json`, inherited | the per-delegate settings file written from the role policy | `metasystem/internal/adapter/claude.go:113-134` |
| launching a design author as its own process | the seat scratch script `fable-design-launch.sh:13-15` | `metasystem delegate --role designer` through `dispatch.sh` and `adapters/claude.sh:147-162` | `metasystem/scripts/agents/dispatch.sh:1936-1947` |
| writing the design page | the seat checkout, by the headless process running from the repository root | the job worktree's `metasystem/plans`, then the seat's copy and commit | `metasystem/internal/dispatch/envelope.go:44-53`, the `design` preset |
| recording a run's compaction | nothing (found in a transcript by hand) | the record's `context` field at completion | `metasystem/internal/adapter/patch.go:43-60` |

## 8. Q6: out of scope

- The seat cap value and the seat's own compaction; in-process Agent subagents of a seat keep the
  seat's window.
- The hact drivers, tmux, and the host launcher (`scripts/agents/hosts/claude.sh`, host mode of
  `BuildClaudeCommand` with an empty record).
- Doctrine beyond one sentence: the design lane runs engine-launched. It belongs in the bullet at
  `metasystem/AGENTS.md:25` ("Dispatch rostered roles through `metasystem delegate`"), as a clause
  naming the designer role; the sentence is unit 4's and is not written here.
- Codex and Devin windows, the reaper, cap continuation for designers, worktree removal, the health
  and stop-status reports, and any change to the context law keys under `context.*`.
- Mission fence accounting of the new evidence, and `chainUsage` aggregation of peaks.

## 9. Open questions for the seat

1. Implementer window: this design follows the DONE text (the norm, 200000) and names the behaviour
   change for worktree implementers, which run uncapped today. If Wido reads his floor rule as
   keeping that, `dispatch.context-window.norm` stays and the implementer class moves to `maximum`
   in the table, one line and one test row; the rest of the design is unchanged.
