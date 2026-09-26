# Review work regardless of who wrote it

- Kind: design
- Id: 01M3EJ5W1JX95BA2NHD4G6Z1N7
- Status: accepted
- Goals: verbs-match-intent

Author: root Codex, 26 September 2026. This completes a demonstrated capability
gap in the user's full intent redesign; accepted parent and planning contracts
remain required. Critique closed at round 2 on two named fixture obligations
(material trajectory 3 -> 2), plus the required concurrency/replay fixtures;
independent Sol implementation critique is mandatory. Investigation: plans/intent-manual-review-investigation.md.

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
metasystem review goal G [--work NAME]
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
correction can be submitted again with --changes/--patch and --after COMMIT; the
full current commit is supplied by public status/review (an unambiguous prefix is
accepted). This authorizes replacing exactly that version and amends
that work through the existing commit owner and invalidates the old read. It need
not hire a builder through revise. A new real correction is explicit, not a retry.

The subject words design, job, unit, commit, changes, diff and goal are reserved
in this command. `review goal G` is the unambiguous spelling for EVERY goal id,
including those words; the explicit goal form accepts the same goal-review flags.
A goal named changes can never change the meaning of `review changes`. Public
continuations use the explicit goal form when the id would collide.

For changes, the capture is the invoking Git checkout's complete tracked and
untracked, unignored changes against its HEAD. Resolve Git's checkout top level
before constructing the snapshot workspace; captures, displayed paths and replay
comparisons are top-level relative even from nested metasystem/ or application
directories. TestManualCaptureFromNestedCheckout proves outside-prefix changes
are included and the source index is unchanged. It preserves the caller's actual
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
ignored source files or conf.local; linked Git repository configuration is shared
and is not claimed to be a sandbox. The reader writes only retained reports in
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

No new manual-submission registry, attempt counter or CommitPatch entry point.
The authoritative work is the existing goal branch range, addressed by work name
and immutable Goal-Unit commit. Read it through branch.InspectStatus, combining
that real work view with NamedWork for model builds without fabricating a build
result. Both producers share names; an existing item is never silently replaced.
Default work is main when there is no work; ambiguity names executable --work
choices. Manual work versions are commits, not invented build-attempt numbers.

Freeze the patch and brief before effects, using existing read input custody.
Use existing lawful claim and prepareGoalWorktree. Establish the holder before locking; use the existing bounded checkout-mutation
lock across stage plus commit, then recheck the holder and mint the existing
commit token inside that critical section. The token is identity for the hook,
NOT exclusion. Apply the same lock in the shared goal-commit wrapper so a raw
commit cannot race public staging. Do not reacquire it in nested commit calls
or hold it across push, paid review or waits. Under that lock, inspect the branch and selected work, check replay, stage the frozen
change and call CommitStaged unchanged. Staged paths, binary patch, index tree,
installation preflight and amendment must all describe the SAME candidate; do not
inject only the Patch function. A distinct destination must be clean or already
hold exactly this captured candidate, otherwise refuse without overwriting it.
Stage the frozen patch with `git apply --index --binary`, WITHOUT --3way in
the persistent destination, so both index and destination files agree. Three-way
application stays in disposable scratch owned by CommitStaged.
TestManualStageRefusalPreservesCheckout uses a patch that needs a three-way merge
and asserts the destination index and files remain byte-identical on refusal. A supplied patch that does not apply is refused with
its inputs preserved; no generic importer or automatic conflict resolver.

When source equals destination, the captured edits are already present: stage
only the captured paths after checking HEAD and captured bytes still match, rather
than applying them twice. The real commit owner installs the goal tip; help states
these effects. Preserve unrelated paths, refuse another writer, retain ordinary
claim/path-class/range/remote-divergence checks and the owner token. Before commit,
a failed staging operation restores only its own index/files under the same token;
never reset or discard other work. An interrupted staging operation recognizes
only an exact staged candidate on repeat, otherwise refuses with public guidance.

Replay uses branch identity, not a new operation registry. For initial submission,
look up the work name before staging: if frozen patch applied to that unit's actual
parent produces its exact tree, rejoin that authentic commit. A different tree
requires explicit --after COMMIT. With --after, compare against that named version:
if it remains current, the existing amendment owner may apply the correction; if
already replaced, compare the requested result on the named old unit with the
current unit before rejoining. Never compare the whole current branch tip, which
may include other work and read records. A stale/unavailable base or mismatch
refuses; it cannot silently authorize another amendment. Replay comparison uses the commit owner's actual scratch three-way application
semantics, not plain prefix-scoped gittree.Apply: an adopted branch can install
a patch through three-way application. Reuse/expose that bounded scratch-tree
comparison, without committing or changing the source. An adopt-then-resubmit
case in TestManualSubmissionReplayAndAmend must rejoin the actual unit.
No-change in the source
checkout after successful installation rejoins the selected existing work before
attempting to create an empty commit. Changed brief is checked by the existing
read owner's frozen-brief binding, never treated as a new clean certification.

These comparisons and staging are serialized by the bounded checkout lock;
the identity token alone does not serialize. Existing branch update-ref
compare-and-swap and range checks remain the final publication protection.
TestManualSubmissionReplayAndAmend interleaves repeated and differing submissions,
proves no overwritten input/token or duplicate unit, and proves the losing caller
can repeat safely. Release the lock before any other workflow stage. A crash before installation may
leave only a dangling scratch commit; a repeat commits once to the branch. A lost
commit response is resolved from the actual range and candidate comparison; a
lost push response uses branch.Push's existing journal reconciliation. Do not
promise CommitRequest.OpID alone provides commit idempotence: it does not.

Push that authentic Goal-Unit through branch.Push. The existing committed-review
owner runs its fast gate and actual independent critic; reuse reviewCommit's
bound decisions/full-close/collect/publish composition for manual work. This is
one review lifecycle regardless of author. Manual amendments preserve later work
and remove only this work's obsolete read through the existing amendment owner.
Status and ordinary review G see it through the range; land G already consumes
the branch's attestations. No fake preliminary read/build exit, hand-written
attestation or additional delivery protocol. The complete public journey remains
mandatory before this user's goal is done.

## Proof and critique

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| IM-1 | HIGH | Diagnostic public subjects and complete snapshot | Manual diff/current-change feedback with no builder; source untouched | Launch retained read/partition owners | internal/launch/standalone_read.go and read_sequence.go; public review adapter | TestIntentManualDiffReview; TestStandaloneReadPartition; TestManualCaptureFromNestedCheckout | Public commands, actual capture/retention with fake reader | MISSING | Complete owner and public adapter, drive exact candidate |
| IM-2 | CRITICAL | Diagnostic request lifecycle | Replays, failed retry and live/compacted outcomes honest | Existing launch custody and request retention | Standalone read request/sequence using Manager custody | TestStandaloneReadReplayAndRecovery | One launch per request; show/wait/stop use public REF | MISSING | Complete and drive lifecycle |
| IM-3 | CRITICAL | Manual submission and committed review | Manual submission has authentic commit/review/landing authority | Goal branch commit/read owners | goal/branch CommitStaged and RunBranchRead; shared commit wrapper and public manual adapter | TestIntentManualWorkDelivery; TestManualStageRefusalPreservesCheckout | Manual edit -> public submission -> bound review -> land, no builder | MISSING | Implement after diagnostic slice and drive |
| IM-4 | CRITICAL | Manual replay, locking and amendment | Lost response, amendment and unrelated/source work preservation | Existing branch range/commit/amend owners | goal/branch commit/push/range owner and checkout mutation lock | TestManualSubmissionReplayAndAmend | Lost commit/push result, repeat and correction with another work item | MISSING | Implement exact comparison/locking and drive |


Round 1 folded IM-C1..3: deleted the proposed manual request registry and
CommitPatch, retained exact branch-owner comparison and existing indexed commit,
and made subject-word collisions unambiguous. Dispositions are recorded in
plans/intent-manual-review-dispositions.md. Round 2 IM-C5 and IM-C6 became the named nested-capture and staging-refusal
fixtures above. Root also corrected IM-C4's mistaken token-as-lock premise and
IM-C7's replay semantics; all seven findings have dispositions. Implementation
is authorized, but no runtime or delivery certification is claimed.

Fable: maximum two rounds on this new concrete capability, failsafe round 2;
criterion is whether step 1 works/safely meets its contract, plus whether the
required manual-delivery extension is honestly implementable. Challenge the
premise and delete needless machinery. Root adjudicates. Named mechanical
findings become fixtures, and independent Sol review of the complete combined
implementation remains mandatory. No partial completion claim for this user goal.


## Concrete capture boundary correction before full submission

Root source audit found the live source has ordinary generated receipt/narrator
appends. branch.PathClass already assigns them and goal projections ClassExcluded;
CommitStaged refuses those paths in a Unit. Do not weaken that owner or ask the
caller to clear system-written files. For implicit manual `--changes`, preserve
ClassExcluded paths in the source and omit them from the submitted unit using the
existing path-class/exclusion owner, never a maintained second list. Other
non-unit paths retain existing refusal with the applicable public design/goal act.
An explicit `--patch` is not silently rewritten to remove disallowed changes.
Source=destination may not discard unrelated pre-existing staging; normal unstaged
coordination appends must survive. Test real source preservation and actual owner
acceptance, not only the filtered path list.

The caller's explicitly named brief is control input, not an accidental candidate
file. When implicit changes are captured, omit that exact brief path if it lies
inside the checkout, while retaining its bytes/digest as the brief. This makes the
existing promise that brief/reports are outside capture true for the caller's
original too. Do not require users to know an ignored internal directory. Other
ordinary files remain captured; explicit patch contents are authoritative and are
not filtered. Use literal Git path selection/private-index operations, not a text
regex that rewrites binary diffs. Diagnostic reads otherwise keep their complete
tracked/untracked whole-checkout capture, including any records the user wants
feedback on. Add nested in-repo brief and normal background-record fixtures.
