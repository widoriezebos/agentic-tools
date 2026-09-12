Finding Id: sia-build-fold-2
Disposition: accepted

# Finding Being Corrected

Round 2 built the mechanism; the orchestrator's proof of its tree went red
in one bed leg, and the independent code read (Opus) returned five material
findings. Fold all six, in place, in your worktree; touch only the eleven
files of the workspace list; do not commit.

# Disposition Reasoning and Evidence

1. A real work refusal must never become an allowance inside the engine.
   In metasystem/internal/goal/turnverdict.go, the verdict-state write
   failure (around lines 295 to 310) preserves the block only when the
   verdict is an idle refusal; a seat-actionable block (owned work with no
   waiter, an unsurfaced plan line, a changed queue, a stale goal-free
   declaration) with a failed state write falls through to the
   infrastructure allowance. The rule: any verdict with ShouldBlock true
   keeps blocking when its state write fails; the write failure is added to
   its diagnostics and, for the idle refusal, CountSpent is false. Same for
   the status write. Add the assertion to TestInfrastructureVerdictNeverBlocks'
   sibling: a seat-actionable block survives a failed state write.
2. A session-stop consume failure is infrastructure: the stop is allowed
   without consuming and without inventing an authorization; it does not
   re-run the idle evaluation and it never spends an idle count (lines 312
   to 331 call enforceIdleBacklog and save the state again: remove that).
   The idle counter and the three-refusals handoff are the build brief's
   non-goal. TestSessionStopInfrastructurePreservesAuthority asserts no
   count is spent.
3. The hook log gets one stop-condition line per collected condition in
   every outcome: the append loop moves above the readable/unreadable
   verdict split in metasystem/scripts/agents/supervision-hook.sh (the
   if/elif chain around lines 1211 to 1221 and the unreadable-verdict
   branch near 1251), so an infrastructure verdict line, the uncounted idle
   refusal's line and every record_stop_failure condition are all written.
4. TestArmingDetailSurvivesStop in metasystem/cmd/metasystem/up_test.go
   must prove behavior, not grep the hook's source: drive the stop notice
   composition with an ENROLLMENT_DRIFT up result (a component line with
   outcome, detail and remedy, then the aggregate) and assert those four
   appear in the emitted notice byte for byte. If the composition lives in
   the shell only, move it behind an engine verb (report stop-notice or an
   argument of report stop-block) so a Go test can drive it, and have the
   hook call that. And TestInfrastructureVerdictNeverBlocks must exercise
   the producers through the verdict entry point (state root, fence, verdict
   state, status write), not call infrastructureVerdict directly.
5. No hook-bed leg is deleted. Restore the runtime-list early-abort leg in
   metasystem/scripts/agents/supervision-hook-fixtures.sh (its injection
   step still stands near line 632; the assertion was dropped near lines
   702 to 705) and rewrite its assertion to the new contract: the early
   abort is an infrastructure allowance with the degraded notice, never a
   block. The partial-output leg asserts its cause, not the generic
   "stopping is allowed" fragment.
6. The bed's red leg: "supervision hook chat-line fixture omitted the
   pending narrator digest" (the supervision-and-census section of the
   orchestrator's proof, attempt proof-mtytmwcy-a486476aece21995). The
   allow path must still carry the pending narrator digest in the chat
   line exactly as before; the new notice is added beside it, never in its
   place. Find the leg in supervision-hook-fixtures.sh and make it pass
   without weakening it.

Also from the read, not material, fix if cheap: do not wrap an already
rendered JSON response inside a systemMessage on a log-append failure
(line 352); pass only the failed component lines and the aggregate into
the notice and the record's remedy (lines 861 and 966), not the whole up
output; drop the dead FailClosed branch in the hook (line 1173) if
Verdict.FailClosed is never set now; the lost-counter idle block should
still go through renderTurnVerdict so its reason keeps its envelope and
the turn-verdict artifact is written.

Run the focused Go tests, `bash -n` on both scripts, and the fast gate; the
hook bed cannot run in your sandbox (process inspection is denied), so say
so and the orchestrator replays it. Same return shape as round 1. Wall
clock: 2 hours.

# Unchanged Return Contract

The original role and return schema remain binding without additions, removals, or relaxations.

Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.

Schema: the same scripts/agents/schemas/implementer.schema.json used for the original dispatch
