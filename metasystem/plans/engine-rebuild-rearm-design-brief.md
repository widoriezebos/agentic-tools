Working Mode: design
Orchestrator Identity: m1 (lineage main-1788594343-3833-fb64b9, dispatch delegate under goal engine-rebuild-rearms-itself, tier 3 DESIGN-BEARING)
Date: 2026-09-06

# Goal

Author the design document engine-rebuild-rearm-design.md, a NEW file you
create in the metasystem plans directory, revision 1, for this goal: a rebuild of
the enrolled engine at its own enrolled path re-arms itself, and every
other drift cause still reaches the human.

# The defect, against the code

metasystem/internal/steward/identity.go pins one engine digest in the
enrollment record; OpenEnrolledBinary refuses ENROLLMENT_DRIFT whenever the
bytes at the enrolled path differ from it, and metasystem/internal/up/up.go
turns that into a failed arming whose only remedy is a human at an
agent-free terminal running steward arm. Rebuilding the engine is ordinary
work in this repository, so every rebuild wedged every seat on the machine
until a human typed the command: m2 lost a night to re-prompts on
2026-09-04 and m1 lost most of 2026-09-05.

identity.go's own header states what the pin is for: "accident-proofing at
the repository's trust level: a stray cron job does not match a pinned
record; a same-user adversary is out of scope repo-wide." The design's
first job is to decide, against that stated threat model, whether a
rebuild at the enrolled path, invoked by that engine, in that repository,
owned by that user, is a condition the pin exists to catch — and to say so
either way with the reasoning.

# Prior art the design must take as input, not as a landing

An implementation exists and was critiqued; it is NOT the design. On
2026-09-05 m1 built: a typed ErrEngineRebuilt wrapped alongside
ErrEnrollmentDrift at the digest-mismatch site; a steward.ReArmRebuiltEngine
that re-reads the enrollment, refuses unless the caller's binary is the
enrolled path, and calls the existing arm() with replace=true carrying the
temporary human word and review date forward; and an up.openInvokingEnrollment
that routes only that typed cause to the re-arm, retries the open once, and
reports accepted-engine outcome=re-armed. It armed this machine three times
across real rebuilds. A Codex critique (recorded on this goal's ledger
record, plans/goals/engine-rebuild-rearms-itself.md, in its next-step
history) returned three findings the design must dispose of by id:

1. MAJOR, stale state across the arm lock: the re-arm read the enrollment
   BEFORE arm() takes the arm flock (runner.go's arm lock), so an automatic
   caller that read a permanent enrollment can overwrite a concurrent
   legitimate ArmTemporary's word and review date with stale empty
   strings — a permanent-looking record produced by concurrency, the exact
   laundering the change claimed to prevent. Eligibility and carry-forward
   fields must be re-read inside the lock.
2. MINOR, the successful hook path discards the re-armed notice:
   metasystem/scripts/agents/supervision-hook.sh exits 0 on a successful up
   and reads up's output only on failure, so a rebuild's re-arm is silent
   in the hook.
3. MINOR, recovery can report recovery-not-needed after a re-arm already
   replaced the runner; and recovery passes replace=true inside a
   --if-down path, exceeding "start only missing repository rings".

Wido's ruling on the carry-forward (2026-09-05, verbatim options): carry
the temporary word and review date forward AND stamp which generations
were machine-minted rather than human-witnessed, so a reader can tell them
apart. The design binds that.

# What the design must decide

- Decision 1: the threat-model reading above, and the exact set of drift
  causes that re-arm versus refuse (repository identity, path, owner, mode,
  generation, path changed while pinning, invoking engine not the enrolled
  path — each dispositioned).
- Decision 2: the re-arm as one act under the arm lock: what is re-read
  inside it, what the machine-minted stamp is on InstallIdentity, how a
  human re-arm clears it, and what health shows for a machine-minted
  generation.
- Decision 3: visibility — the outcome line, the hook on success, and
  recovery's aggregate; whether unattended recovery may re-arm at all.
- Decision 4: failure shape — mint-before-launch in arm() leaves a bumped
  generation on a failed launch; say whether the automatic path may
  inherit that or must not.
- Decision 5: fixtures — a real rebuild at the enrolled path re-arms; a
  stranger binary refuses without touching the record; two concurrent
  re-arms mint once; a re-arm racing ArmTemporary preserves the human's
  word; the hook surfaces the re-arm; each with a named session id.

Self-grade and reject condition, as the hook-root design carries them.

# Constraints

Wall-clock budget: 25 minutes. Read metasystem/internal/steward/identity.go,
metasystem/internal/steward/runner.go, metasystem/internal/up/up.go and
metasystem/scripts/agents/supervision-hook.sh before writing. Version-2
implementer JSON; diffBoundary lists exactly the one new design file you
created in the metasystem plans directory.

# Gap Rule

Stop and report a gap; never fill it silently.
