# Review work regardless of who wrote it

- Kind: design
- Id: 01M3EJ5W1JX95BA2NHD4G6Z1N7
- Status: draft
- Goals: verbs-match-intent

Author: root Codex, 26 September 2026. This completes a demonstrated capability
gap in the user's full intent redesign; accepted parent and planning contracts
remain required. Investigation: plans/intent-manual-review-investigation.md.

## Contract and first usable slice

A person or agent can request independent feedback on a patch or current changes,
then submit manually written work for the same real examination and delivery as
model-written work. No paid builder or knowledge of internal commit trailers is
required. No new top-level verb, authority policy, certificate or workflow engine.

Step 1 is public diagnostic review of current changes and a supplied diff,
including retained feedback, repeat, stop and failure. Complete manual goal
submission and landing is the next slice and remains REQUIRED before this user
request is complete. Deferred: arbitrary cross-repository imports, automatic
conflict resolution, automatic finding judgment, and release based on diagnostic
feedback. Existing review, proof and landing authority stays decisive.

## Grounded owners

Current public goal review selects UnitRunner results; job review requires a
completed implementer; commit review requires a Goal-Unit in its goal's range
(intent_selection.go, intent_delivery.go, internal/goal/branch/read.go:250).
Normal project-rules.md still sends a bare-diff read to internal launch.

launch.Manager.Start Kind read owns independent roster, immutable read.diff,
process custody and retained outputs. ChooseReadMode and UnitRunner's read-step
planning own whole/package/wide selection and bounded compaction reruns. The
latter orchestration is private and needs a focused extraction for this consumer.

Manual delivery already exists: branch.CommitStaged writes Goal-Unit trailers
without a builder. Its scratch-worktree buildCommitOnto applies a binary patch
with git apply --index --3way before validated installation (commit.go:382).
CheckCommitCheckout/CheckCommitAccess and the CLI's commit token retain authority.
branch.Push, RunBranchRead, whole close, collection/publication and landGoal then
provide actual certification and delivery. There is no exported frozen-patch
submission operation today; do not pretend there is one.

gittree.Workspace.Snapshot/Diff/Apply use isolated indexes and preserve source
files/index. MaterializePaths does not support gitlinks or all file/directory
transitions, so do not use it as a supposedly complete importer. Reuse the commit
owner's scratch apply for manual delivery.

## Public subjects

```
metasystem review changes --brief FILE [--goal G]
metasystem review diff PATCH --brief FILE [--goal G]
metasystem review G --changes --brief FILE [--work NAME]
metasystem review G --patch PATCH --brief FILE [--work NAME]
metasystem show review REF
metasystem wait review REF [--timeout DURATION]
metasystem stop review REF
```

The first two request feedback only and clearly say so in help and results.
Optional goal associates the feedback, as the existing read launch does; it does
not mint execution approval or a landing attestation. The latter two submit real
work to the goal's certified-review path, which already commits/publishes review
subjects. Help states that effect. Ordinary review G, its --dispositions/--retry,
status G, revise G and land G remain continuations after submission. A manual
correction can be submitted again with --changes/--patch and --after N; it amends
that work through the existing commit owner and invalidates the old read. It need
not hire a builder through revise. A new real correction is explicit, not a retry.

For changes, the capture is the invoking Git checkout's complete tracked and
untracked, unignored changes against its HEAD. It preserves the caller's actual
index; the brief and generated reports live outside the captured tree. Show the
selected file list and exact base/diff identity in the result. Empty changes are
an unchanged result, not a paid read. Supplied PATCH is frozen exactly; no claim
that it matches the invoking checkout is inferred. Paths in a patch are handled
only by existing Git owners in a private destination, never applied to the source
by diagnostic review. Unsupported/malformed input explains the actual limitation.

## Diagnostic review through the read owner

Add a focused retained read request in internal/launch using its existing store
and request/lock primitives. Freeze base/context, patch and brief bytes plus
resolved read runtime/model before spawning. A per-request entry retains the
read sequence's child IDs and public attempt; it is not a second job registry.
Extract the current read partition/sequence behavior so builds and standalone
reads use one policy. Larger inputs never require package/wide engine flags.
Existing caps, output custody and non-counting compaction behavior remain.

A private disposable reader checkout captures the context tree; it contains no
local secret config or ignored files. The reader writes only retained reports in
that private area, not the caller's work. For a supplied patch, record its context
base separately; if it cannot apply there, the feedback reports that limitation
rather than claiming the workspace was verified. Diagnostic snapshot/archive
preparation belongs to the launch owner using existing gittree/Git primitives.
No general import engine or new repository cache.

An identical request rejoins before new admission, even after a failed or finished
read. Source changes cannot mutate its frozen bytes. Explicit --retry N on the
same diagnostic command requests one attempt after displayed failed attempt N,
only once old processes are proved stopped; replay rejoins that retry too. Reuse
the accepted launch startup recovery, never relaunch an uncertain child. The
public REF from every result drives show/wait/stop; no private file discovery.

A result lists each retained report and completion state; missing, failed or
non-counting reads remain incomplete. The existing textual verdict 'land' is
feedback wording and cannot authorize goal delivery. No Goal-Read, register close,
claim takeover, goal conclusion or commit is generated by these two subjects.
Test that the source HEAD, index and file bytes remain unchanged even when a fake
reader tries to write its current working directory.

## Submit manual work through goal-branch owners

Add a focused manual-submission request operation to internal/goal/branch. Its
retained request lives beside existing branch operation/read state and freezes
source/base/patch/brief, goal/work name, request identity, admitted destination and
commit/publication result. It owns replay; the CLI must not scan private files or
fabricate a UnitRunner result. Goal/work names share the public namespace: an
existing model-built item is not silently replaced by a manual item. Default
work is main when the goal has no work; ambiguity requires --work as in the parent.
Owner reads let status/review see manually submitted work alongside built work.

Use existing lawful claim and prepareGoalWorktree; no source branch manipulation
is required from the caller. Add CommitPatch as a narrow entry to the SAME commit
core used by CommitStaged, taking the frozen binary patch instead of reading it
from the index. Retain current claim, checkout lease/commit token, path-class,
range, branch-divergence and install checks. Record the computed subject and
commit operation before publication so a lost commit or push response resolves
the authentic existing result, never duplicates it. Preserve the original source
checkout when it is distinct from the goal checkout; when invoked inside that
goal checkout, installation has the same documented commit effects as review G,
with existing unstaged-path protections. Never discard unrelated work or force
through an active writer. Conflicts preserve both inputs and give the public
work/status choice; no automatic merge resolution.

Push the authentic Goal-Unit through branch.Push. The existing committed review
owner runs its fast gate and actual independent critic against the installed
subject, then the parent's bound decisions/full-close/collect/publish flow applies.
Its exact brief binds the manual work; no synthetic preliminary read, fake build
exit or hand-written attestation is admissible. land G uses its normal exact
candidate proof and publication. Status describes the actual stage. Repeating
submission after completion must locate its retained request before interpreting
an already installed/empty workspace as different work. Changed bytes require an
explicit new submission after N; its amendment preserves other work and removes
only this work's obsolete attestation, as the existing amendment owner does.

## Proof and critique

| Obligation id | Severity | Required behavior | Owner | Test proof | Runtime proof | Status |
| --- | --- | --- | --- | --- | --- | --- |
| IM-1 | HIGH | Manual diff/current-change feedback with no builder; source untouched | Launch retained read/partition owners | TestIntentManualDiffReview; TestStandaloneReadPartition | Public commands, actual capture/retention with fake reader | MISSING |
| IM-2 | CRITICAL | Replays, failed retry and live/compacted outcomes honest | Existing launch custody and request retention | TestStandaloneReadReplayAndRecovery | One launch per request; show/wait/stop use public REF | MISSING |
| IM-3 | CRITICAL | Manual submission has authentic commit/review/landing authority | Goal branch commit/read owners | TestIntentManualWorkDelivery | Manual edit -> public submission -> bound review -> land, no builder | MISSING |
| IM-4 | CRITICAL | Lost response, amendment and unrelated/source work preservation | Branch request/commit/amend owners | TestManualSubmissionReplayAndAmend | Lost commit/push result, repeat and correction with another work item | MISSING |

Fable: maximum two rounds on this new concrete capability, failsafe round 2;
criterion is whether step 1 works/safely meets its contract, plus whether the
required manual-delivery extension is honestly implementable. Challenge the
premise and delete needless machinery. Root adjudicates. Named mechanical
findings become fixtures, and independent Sol review of the complete combined
implementation remains mandatory. No partial completion claim for this user goal.
