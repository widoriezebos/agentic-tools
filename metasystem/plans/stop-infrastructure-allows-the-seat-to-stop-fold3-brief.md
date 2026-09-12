Finding Id: sia-build-fold-3
Disposition: accepted

# Finding Being Corrected

Round 3 folded the six; the orchestrator's proof of its tree is still red on
the same bed leg ("supervision hook chat-line fixture omitted the pending
narrator digest", attempt proof-mtyugmdf-112a637644c90c4a), and the second
independent code read (Opus) explains it and finds five more. Fold all six,
in place, touching only the eleven workspace files; do not commit.

# Disposition Reasoning and Evidence

1. The bed's red leg is the read's F-2: in
   metasystem/internal/report/stopblock.go (the infrastructure branch,
   around lines 150 to 159) the notice concatenates header, cause, remedy,
   the whole detail (the hook's failure_detail, itself up to 4000 runes)
   and the system message into one string bounded once at 4000 runes, so
   the check-in tail (the HEALTH line and the NARRATOR DIGEST) is trimmed
   off, while the hook still advances the digest cursor: the digest is lost.
   Fix: the infrastructure notice keeps the detail in its own bounded
   field as the seat-actionable path does (bound the detail first, then the
   check-in tail separately, never trim the tail away), and the hook's
   compose_failed_stop suffix-strip must still match now that report.go
   appends --arming-result after the system message (lines 1029 to 1038
   of the hook duplicated the tail). The chat-line leg must pass with the
   digest present.
2. F-1: metasystem/internal/goal/turnverdict.go lines 337 to 341: when
   consumeSessionStop returns consumed false with no error (marker expired,
   the attending human gone, lifecycle changed under the lock), the closure
   returns a plain error that is not an infrastructureFailure, so the
   generic fallback at lines 365 to 369 allows the stop and discards the
   verdict s.decide computed. Rule, as in fold 2 item 1: the decided
   verdict is never discarded; an unconsumed authorization is not an
   infrastructure condition, it is "no authorization", and the decided
   verdict stands (block or allow as decided). And every infrastructure
   condition names its real component: a session-stop marker condition is
   `session-stop-marker`, a lock acquisition error is `verdict-state-lock`,
   never a mislabeled `verdict-state`.
3. F-3: the hook's verdict reader (lines 1161 to 1172) now requires
   `class` and `countSpent` to be present and in range, so a verdict from
   an engine built before this landing is unreadable and every real block
   becomes a degraded allowance until the seat rebuilds. Default an absent
   `class` to `seat-actionable` and an absent `countSpent` to true (the
   tolerant read the diff removed), and keep the compat_engine bed leg
   green.
4. F-4: the deadline path's record-failure branch (lines 357 to 359)
   emits an infrastructure allowance with no stop-condition line; write
   the line there too (component `stop-deadline`, cause code
   `stop-deadline-expired`), and `mkdir -p` the log's directory before the
   append in every branch.
5. F-5: TestSessionStopInfrastructurePreservesAuthority's consume-failure
   case runs on a bed with no approved backlog, so its "no idle count
   spent" assertion is vacuous; give it a budgeted queued goal (the bed
   TestSessionStopConsumeErrorAllowsWithoutAuthorization uses) so the
   assertion fails against round 2's tree.
6. F-6: the partial-output bed leg asserts its own cause (the fixed
   diagnostic of that site, slugged per fold 1's rule), not the generic
   "Stop hook output was unreadable" the runtime-list leg asserts.

Also, not material, fix if cheap: `deadline_log_stop_outcome
invalid-worker-output-block` names a block for what is now an allowance;
TestArmingDetailSurvivesStop passes the same string as --remedy and
--arming-result and so cannot fail if --arming-result is dropped: give
them different strings.

Run the focused Go tests, `bash -n` on both scripts and the fast gate; the
hook bed cannot run in your sandbox (say so; the orchestrator replays it).
Same return shape as round 1. Wall clock: 2 hours.

# Unchanged Return Contract

The original role and return schema remain binding without additions, removals, or relaxations.

Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.

Schema: the same scripts/agents/schemas/implementer.schema.json used for the original dispatch
