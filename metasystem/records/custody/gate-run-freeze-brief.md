# Design brief: gate-run-freeze (the unkillable milestone battery)

INTENT. The milestone battery — run only when the weight accumulator
says feature gravity has made it worth its cost — executes against a
snapshot/worktree of COMMITTED bytes, so nothing the live checkout
does during the run can kill it, invalidate it, or be blocked by it.
The behavior-surface scope becomes one declared list read by every
law that reasons about "the bytes": the adoption payload-equivalence
digest and the exact-final-bytes rule both consult it, so
coordination state (goal ledger, receipts, artifacts) can neither
summon the battery nor break its claims. Whatever narrow interference
window remains after isolation gets the verb-level freeze, and only
that window.

CONSTRAINTS.
- The battery's subject is a COMMIT (git-archive snapshot or
  worktree of it), never the live tree; the D33 witness snapshot is
  the proven pattern to extend.
- Machine-global fixture surfaces must stay lawful in the isolated
  home: supervision fixtures spawn real processes, gate markers and
  leases are per-checkout, the census walks real pids. Every such
  surface either works unchanged in the worktree or names its
  explicit accommodation.
- Failure evidence copies OUT to the durable suite-failures home
  before the isolated home is cleaned; evidence beats disk, always.
- Shared GOCACHE keeps the isolated run's cost flat vs today.
- A green isolated battery runs weight-reset against the REAL
  checkout (the accumulator lives with the coordination state, not
  the snapshot).
- The behavior-surface scope is a DECLARED machine-readable list
  (suggested: scripts/agents/behavior-surface.txt of excluded
  coordination paths), consumed by the adopt-fixtures equivalence
  probe and documented as the landing law's definition of "bytes";
  exemptions never live in prose judgment.
- The battery entry point stays one command an operator or scheduler
  can run; its report names the commit it judged.

FREEDOMS. The isolation primitive (git worktree vs archive-extract),
the wrapper script's name and home, the scope file's exact format,
how the freeze remainder is expressed (a gate-fence consultation in
the goal family, or nothing if isolation leaves no window), and
whether weight-reset rides the battery wrapper or validate itself.

EVIDENCE THIS ANSWERS. 2026-08-25: two batteries killed by the
coordinator's own ledger writes mid-run (suite-failures
20260825T071434Z-adopt-33526), one green battery invalidated by a
ledger-only rebase, ~5 battery-hours spent on a 20-line script.

TESTING (no battery until the battery set lands — Wido's standing
order). Unit tests for the scope reader; a fixture proving the
isolated run survives a concurrent goal write and a concurrent
commit in the live checkout; a fixture proving evidence copy-out on a
forced red; statics; the touched fixture legs. The FIRST full
milestone battery run after the set lands is its own acceptance.

## AMENDMENTS (round 1, session 01a03a22 — all six folded; failsafe
## round declared: ROUND 3; the first round whose material findings
## are all fixture-expressible closes the loop and building begins)

A1. WEIGHT IS TRANSACTIONAL ACROSS THE RUN: the isolated battery
    checkpoints the accumulator at start (subject commit + value);
    reset applies only the checkpointed portion, preserving every
    landing weighed DURING the run — concurrent landings lose nothing.
A2. THE SUBJECT IS A DETACHED-COMMIT WORKTREE: reuse D33's witness
    PRINCIPLES (digest, run id, fences) but not its archive-only
    execution and never its live-binary copy-back — the isolated run
    builds and consumes its own binary; the live checkout's bin/ is
    untouched.
A3. MACHINE-GLOBAL STATE IS ISOLATED BY POLICY: the run points the
    supervision registry (armed-checkouts) at a run-scoped home,
    copying of live coordination artifacts into the worktree is
    prohibited, and PID/census ownership rules are pinned by focused
    fixtures — this boundary is the TIMEBOX STOP LINE: if it cannot
    fit the day, the split lands here, never as omitted
    accommodations.
A4. ONE BEHAVIOR-SURFACE OWNER: a versioned policy in the ENGINE
    (`metasystem behavior-surface` producing/comparing a
    surfaceDigest and classifying paths), consumed by payload
    equivalence, landing equivalence, AND weight classification —
    the three hard-coded closures (go-gate's positive list, the
    wrapper's inverted closure exceptions, weight.go's exclusions)
    converge on it; toolchain identity stays a separate witness
    field, never mixed into the byte digest.
A5. THE EVIDENCE ENVELOPE COPIES ON BOTH OUTCOMES: run-id-scoped
    logs, exit codes, failure artifacts, subject SHA, surface digest,
    start/end, validation exit, copy result, reset result — BEFORE
    teardown; a failed copy-out RETAINS the worktree, reports its
    path, and classifies the run evidence-incomplete, never green;
    reset is forbidden after a failed copy.
A6. DELIVERY SEQUENCE DECLARED: this critique chain (failsafe round
    3) → CODEX BUILDS from the converged design → a FRESH codex
    session critiques the build to AGREE → coordinator verifies and
    lands; the landing also updates docs/collaboration.md's battery
    section from the stale per-landing doctrine to the weight-based
    milestone cadence.

## ROUND-3 CONTRACTS (resolving r2 findings 2-7; this is the failsafe
## round — findings after it are fixture-expressible obligations or
## shape defects, and building begins)

C1 (r2-F2, the weight transaction). The accumulator's every mutation
   (add, checkpoint, reset) runs under an flock on the state file —
   the outage mark's proven pattern — and the state gains a
   GENERATION counter bumped on every mutation. Checkpoint at battery
   start records {generation, accumulated, landings, subject}; only
   ONE outstanding checkpoint may exist (a second battery refuses
   while one is open — batteries are rare by design, serialization
   over cleverness). Reset consumes its checkpoint under the lock:
   accumulated -= checkpointed.accumulated, landings -=
   checkpointed.landings, SinceUTC restarts only when the remainder
   is zero, LastCommit = the battery's subject; a checkpoint whose
   generation base was already consumed refuses as stale; malformed
   state refuses loudly, never silently zeroes. Deterministic
   interleaving fixtures: add-during-battery preserved; two
   batteries; stale checkpoint; malformed state; reset failure.

C2 (r2-F3, worktree visibility). The isolated home is an INDEPENDENT
   LOCAL CLONE (plain git clone of the checkout, detached at the
   subject commit), never a linked worktree — it enters no worktree
   census, so the mission wall's private-carrier enumeration is
   structurally untouched. The run directory is mktemp-fresh, outside
   every checkout, symlink-free by the component walk. Fixtures:
   clone-create and teardown while a recorded mission wall
   enumeration runs in the live checkout, asserting the enumeration
   is byte-stable.

C3 (r2-F4, the isolation oracle). One explicit seam: the supervision
   registry path becomes resolvable via a single declared override
   consumed exactly where os.UserHomeDir derives it today (default
   unchanged); the battery sets it to a run-scoped home — process
   HOME never changes, Go and adapter homes untouched. Configuration:
   the clone receives a SANITIZED conf.local materialized from a
   committed battery template (generic-safe values only; the live
   conf.local is never copied); the equivalence digest excludes
   conf.local exactly as today. PID/census: the isolated run arms its
   own supervision inside its registry seam; bilateral expectation
   pinned by fixture — the live census never claims the battery's
   pids, the battery's census never claims the live checkout's; the
   battery's teardown kills by ITS recorded exact pids only.

C4 (r2-F5, the surface policy). One versioned policy, THREE NAMED
   PROJECTIONS — no digest pretends universality: ENGINE (D33's
   closure, consumer: the witness), LANDING (repository-wide minus
   the coordination class, consumers: the exact-final-bytes law and
   weight classification — weight weighs precisely the landing
   projection's members), PAYLOAD (adoption's allowlist minus the
   TAILORED class, consumer: the payload-equivalence probe). The
   tailored class (configuration, registrations, ignore files, fresh
   ledgers) is EXCLUDED from byte-equality BY NAME and owned by
   adoption's existing assertions; every comparison names its
   projection and both endpoints. Path grammar: toplevel-relative,
   NUL-safe, rename-across-class = one removal plus one addition,
   symlinks judged by the component walk, modes outside the digest.

C5 (r2-F6, provenance and toolchain). Each consumer judges with the
   engine built FROM the bytes it judges: the landing wrapper's
   weight call moves onto the proof engine the fast gate already
   builds (the stale live binary never classifies a prospective
   landing); the battery's classifier is the clone's own build.
   Toolchain identity is a SEPARATE witness field whose EQUALITY is
   independently required wherever a digest is accepted — separated,
   never dropped. Fixtures: policy self-change (the landing that
   edits the policy is judged by its own prospective policy), stale
   binary, nested prefixes.

C6 (r2-F7, the evidence lifecycle without cycles). Staged, each stage
   atomic by rename: (1) the evidence bundle (logs, exit codes,
   failure artifacts, subject SHA, surface digest, timings,
   per-file copy digests) publishes to the durable home BEFORE any
   reset; (2) the transactional reset (C1) runs only after stage-1
   verification; (3) reset.json publishes as a separate atomic
   appendix — the envelope never waits on the reset it must record,
   so the cycle dissolves. Outcome classes over validation × copy ×
   reset × teardown: copy-fail retains the worktree, forbids reset,
   classifies evidence-incomplete (never green); reset-fail after
   green evidence classifies green/reset-unrecorded with the open
   checkpoint resolving truth; teardown-fail is a retained-worktree
   note on an otherwise-final verdict. The full matrix is the
   fixture table.

C7 (r2-F8, the bounded threat model). The universal "nothing the
   live checkout does" claim is WITHDRAWN, replaced by the named
   interference inventory — goal verb writes, commits, rebases and
   checkouts, pushes, conf.local edits, supervision arm/disarm, a
   second battery attempt — each a fixture, plus the by-construction
   argument (independent clone, registry seam) for everything the
   inventory cannot enumerate. The performance claim, the first-run
   acceptance, the stop line, and the delivery workflow are process
   evidence, tracked on the goal, not fixtures.

## ROUND-4 SHAPE REPAIRS (the five r3 shape-defects, nothing else)

S1 (r3-1). The lock is a STABLE SIBLING (battery-weight.flock beside
   the state file), held across read-modify-rename — the outage
   mark's actual pattern, verbatim; the state file's inode is never
   the lock.
S2 (r3-2). A checkpoint has three terminal states: CONSUMED by the
   green-path reset; ABANDONED explicitly when its run ends non-green
   (red validation, copy failure, operator abort) — abandonment
   publishes an abandoned.json appendix, the weight STAYS accumulated
   (a battery that proved nothing resets nothing), and the next
   battery may open; SUPERSEDED when a new battery finds the open
   checkpoint's recorded runner pid provably dead by the identity
   liveness rules — recorded in the superseding run's envelope. One
   command always works: a non-green run can never block batteries
   forever.
S3 (r3-3). Reset NEVER touches LastCommit — it tracks the newest
   LANDING and belongs to the add path alone; the battery's subject
   lives in the checkpoint and the envelope. The state gains
   PostCheckpointSinceUTC, set by the FIRST add (weighted or
   zero-weight) while a checkpoint is open; reset with a nonzero
   remainder adopts it as the new SinceUTC (falling back to the reset
   time when no add occurred), so the window always names the true
   start of what remains.
S4 (r3-4). Payload equality plus independent toolchain equality
   authorize EXACTLY the existing delivery-contract skip families
   (the witness-check-only engine gate and the delivery_contract_skip
   sections adoption's own assertions already cover for tailored
   paths) — enumerated by name in the policy file, and NO new skip
   family may cite payload equality without amending the policy
   version.
S5 (r3-5). The matrix completes: reset-succeeded-but-appendix-
   publish-failed leaves the checkpoint consumed and is REPAIRED on
   the next weight read (a consumed checkpoint missing its appendix
   re-publishes from the state under the lock — read-side repair, an
   explicit behavior); teardown results publish as a teardown.json
   appendix into the envelope directory that stage 1 already created
   (so a post-teardown failure always has a durable home); runner
   death at any stage leaves whatever appendices published, and S2's
   liveness rule unblocks the next run.
