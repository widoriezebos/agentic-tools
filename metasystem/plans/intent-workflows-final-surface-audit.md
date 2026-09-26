# Fresh public surface audit before completion

Root notes from real checkpoint4 help and current source, 26 September 2026.
Required by the user's full intent/no-internals contract, not an expanded design.

This is the historical discovery record. Its material gaps are now corrected and
verified; final acceptance is in `intent-workflows-verification.md` and the
completed matrices in the three intent designs. The observations below retain
their original checkpoint context.

- The reduced root has nine starting points and 24 lines. This is meaningful only
  when the complete workflow behind build/review/land, design/review, manual review
  and recovery exists. Keep every capability in intent-workflows-capabilities.md.
- Current review help still instructs a human to provide internal design record
  headers (`- Kind`, `- Id`, `- Goals`). Public design creates those automatically;
  its review help should describe a design and selection/recovery, not require
  knowledge of record schema. Author/maintainer protocol may retain schema details.
- Current commit review help still prescribes close then collect although the
  actual public review --dispositions now does that. Remove stale manual sequencing
  from normal help/docs/skills once clean design closure is also connected.
- Ordinary output should describe work, independent feedback, decisions, readiness,
  delivery and recovery. Internal record/lock/custody names belong in diagnostic
  detail only when they explain a concrete user choice; never as required next steps.
- Check all generated continuations, not just help: missing/failed/ambiguous input,
  author conflict, failed read, question pending, carry partial, manual source same
  as destination, goals matching reserved target names. A continuation must run
  from the caller's location with the displayed public reference and same identity.
- Root must drive a rebuilt immutable CLI and realistic complete fake-provider
  journeys at final candidate. Component green, a stand-in owner or hidden command
  invoked manually by the test is not evidence of a public complete journey.

No extra top-level commands or speculative parser framework are requested.


## Actual author transport needs a visible output path

Read-only recipe preparation found a first-use gap: intent_design.go's generated
contract says "the staged page you were given" but names no path.
design_request.go seeds the draft and sets StartSpec.Page/Outputs; ClaudeHeadless
Command sends the brief and read packet only, so those fields alone do not tell
the author where to write. Confirm against final source and fix through the existing
author prompt/declared-output contract. A fake author that secretly reads store
metadata or receives Page from a test callback would hide the bug. The real-process
fake-author fixture must obtain its output path from the actual prompt, just as
the paid author would. Independent Fable must inspect this boundary.


## Manual submission must work in an ordinary live checkout

Root source audit found +18 receipt and +129 narrator lines in live main, all
ClassExcluded by the existing branch path owner. Full captured changes staged as a
Unit are rejected at commit.go279..294, not filtered by the commit owner. Accepted
manual design now specifies preserving/omitting generated excluded state for
implicit submission and preserving explicit patches as supplied. It also excludes
the explicitly named in-repo brief from implicit candidate capture: a normal
brief.md otherwise becomes code, a plans/ brief blocks the unit. Diagnostic review
otherwise remains complete. These are actual first-use scope corrections, not
permission to weaken CommitStaged or silently rewrite user-supplied patches.

## Remaining source-backed public continuations

The independent source audit is retained at
artifacts/agents/intent-public-continuation-audit.md; root read it fully. These
are existing capability obligations, not new top-level verbs:

- A generated brief currently turns the absence of a worktree into a false
  MISSING DECISION, then build rejects that brief before its automatic preparation.
  Ordinary absence should say build prepares the workspace; genuine unreadable
  state remains an error. No manual Git setup is required for first build.
- Generated work review continuations must use `review goal G`, and goal display
  continuations use `show --goal G`, so valid names such as design/job/changes or
  designs/question cannot be misinterpreted. Keep repository and work selection.
- Posted questions continue through `wait question channel:Q`; failed delivery
  uses exact-question `ask --retry Q`, never a global poll. Preserve the channel
  qualifier across a bounded wait. A posted-but-unrecorded thread still needs its
  specific repair; blindly reposting that branch could duplicate the message.
- Job discovery belongs at `status work`, including active retained work of this
  user and dispatch work in the selected repository; `--all` includes ended work.
  State scope truthfully, show purpose/goal/location, and actionable status/wait/stop
  references. Use the existing owners, no new registry or scheduler. Existing raw
  IDs remain accepted when unique. Root chooses generated opaque references
  `j1:ID` and `j2:ID` for the two existing stores, decoded only at the selector;
  callers copy a job reference without learning those backing stores. Ambiguity
  offers those references with task descriptions and preserves the requested
  action. Owner calls receive their original ID; never pass a qualified reference
  into a legacy owner. Unknown IDs point to public status work --all. No internal
  list/cancel command is a normal continuation.
- Archived-goal resume uses existing public `reopen G --next TEXT`, preserving
  current authority and the caller's repository selection.

Focused fixtures must follow generated commands, not merely assert their strings.
No optional parser framework, migration registry or extra root command is needed.

## Manual refusal cleanup: independently reproduced loss of staging

Root's external Go overlay drives actual stageManual with a fully pre-staged
code.go, then its commit-refusal undo. The staged diff becomes empty (exit1,
TestRootManualUndoPreservesExistingStagingGitAdapter). Artifacts are outside Git
under agentic-tools-evidence/intent-redesign-20260926/manual-staging-diagnostic.
The command must preserve pre-existing staging, undo only its own effects, and
report cleanup failure/uncertain installation honestly. An explicit supplied patch
must also work from a clean target goal checkout; cwd alone never means a patch
is already applied. Complete manual delivery still needs actual endpoint payload,
not only the current fixture's successful batch join with fake landing effects.

## Capability inventory completion, 26 September

Two omitted retained outcomes were found by tracing all48 commands and13 advanced
capabilities to their current public implementation. These correct the accepted
scope; they add no top-level verb or owner.

- Abandonment with a successor currently omits the existing atomic owner's Carried
  input, then attempts succession only after abandon confirms. Live dependents can
  therefore refuse before succession, while interrupted succession emits an internal
  carry command and public replay stops on the archived source. Fresh `abandon G
  --reason TEXT --successor G2` must forward the successor to goal.Abandon; that owner
  already archives and repoints dependents atomically. Already-abandoned recovery
  uses existing CarryAbandoned through that same public intention, preserving the
  original reason and fresh enrolled-human proof. Invalid successors cannot leave
  a newly abandoned source; completed replays cannot append duplicate history.
- Settings supports a known non-launch key but lacks the existing key enumeration
  and complete validation outcomes. `settings --keys [--matching PREFIX]` uses the
  existing config keys owner, and `check settings` uses its validate owner, both
  honoring the selected installation. Preserve existing summary and key reads.
  Synthetic selected-installation fixtures prove discovery and invalid-setting
  refusal; no live private configuration is read during development verification.

These are required capability-preservation fixes for the original Opus builder.
Independent Fable review covers their actual implementation with the whole patch.
