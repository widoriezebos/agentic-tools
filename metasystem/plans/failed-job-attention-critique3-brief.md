Working Mode: design
Orchestrator Identity: <dispatching seat>+<its session main> (dispatch delegate under goal failed-job-attention)
Date: 2026-09-02

# Goal

Round-3 (closing) critique of metasystem/plans/failed-job-attention-design.md
(revision 3, landed, in your worktree). Your round-2 register is
metasystem/records/misc/failed-job-attention-critique-r2.md: four material
findings. Revision 3 REMOVES two revision-2 mechanisms rather than patching
them (the write-ahead transition journal; the birth-token fallback chain),
puts queued-notification delivery under the tick's arbitration lock per
item, and gives the channel-migration window an owner and an end. It
declares that the build is blocked on goal job-record-birth-token, because
an executed spike proved no shipped record field identifies an
incarnation; the coordinator has to add that edge after release, which is
a decision, not a finding. Section 11 is the fold record.

# Your mandate

1. CLOSURE CHECK, one verdict per finding, against the tree:
   - FJA-R2-BIRTH-ABA-REMAINS: is the digest rule (minted birth generation
     or empty forever for pre-contract records) collision-free on every
     lawful identifier reuse, and does the pre-contract rule really
     guarantee no record's digest ever changes (read
     metasystem/internal/dispatch/record.go for the immutable fields and
     the create path)? Was the spike's refutation of every shipped field
     (createdAt, startedAt, operationId defaulting to the job id) correct?
   - FJA-R2-TRANSITION-PHANTOM: with the journal gone, can any tick narrate
     a raise or clear that never committed; does the marker-survives-until-
     drain rule leak markers on a crash between digest append and drain;
     does widened candidacy (every terminal failed or timed-out
     goal-bearing record) re-raise resolved episodes or create duplicates
     (read metasystem/internal/steward/tick.go, narrate.go, and the
     episode helpers the design cites)?
   - FJA-R2-PENDING-SNAPSHOT-RACE: is the per-item critical section
     actually under the same lock the tick holds
     (metasystem/internal/steward/tick.go around 110-114, runner.go around
     131, metasystem/cmd/metasystem/steward_verbs.go around 270,
     notify.go around 64-93); does the stated bound hold with two
     overlapping passes; what does holding the lock across a 15-second
     external send do to concurrent ticks and worker enrollment
     (metasystem/internal/steward/arbitration.go), and is that acceptable?
   - FJA-R2-CHANNEL-MIGRATION-UNOWNED: is the named owner and end condition
     consistent with metasystem/plans/alert-channel-design.md, and does the
     goal-facing residual say what the channel design must add?
2. ATTACK THE BLOCKER DECISION: is blocking the build on
   job-record-birth-token (metasystem/plans/goals/job-record-birth-token.md)
   the only honest closure, or is there a per-incarnation key the design
   overlooked that ships without the token? Wido ordered this fix done
   first; a wrong block delays it, a wrong key ships a suppression.
3. ATTACK THE FIXTURES (section 6 and the rewritten fixture 12 and new
   4b): deterministic, no sleeps, and each actually replays the failure
   its finding describes.
4. NEW FINDINGS only if material and grounded. Zero material findings is
   an acceptable, closing answer if the reading supports it; this is the
   closing round the goal's resume recipe names.

Findings quote the disagreeing text or code. Your sandbox is read-only:
verify by reading, do not run go. Declared gaps are residuals, not
findings, unless one hides a false claim.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
