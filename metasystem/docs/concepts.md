# The metasystem in concepts

Status: LIVING DOCUMENT, started 2026-08-11 at the human's direction —
"describe the philosophy and the actual working of the metasystem in
concepts that are either familiar or that we define." Populated with what
is known today; sections marked TODO are owed. The glossary
(`docs/glossary.md`) defines terms; this document explains how the
concepts compose into a system. When the two disagree, the code is right
and both documents are wrong.

## Why this document

The metasystem is a machine for letting agents build software unattended
without surrendering the properties a careful human team would keep:
provable identity, single ownership of every truth, spending that cannot
run away, evidence that cannot be quietly lost, and human authority at
exactly the boundaries where judgment is sovereign. Each mechanism below
exists to keep one of those properties under adversarial conditions —
crashes, races, model error, and the permanent temptation of an agent to
narrate success instead of proving it.

## The philosophy, compressed

- **Proof over assertion.** Nothing important is true because an agent
  said so. Identity is a kernel fact (pid plus exact start time plus
  argv), progress is a checkable artifact, closure is a mechanical join.
  Where proof is impossible, the system says "unknown" and treats
  unknown as its own state — never as either verdict.
- **Authority is not proof.** Holding a lease authorizes an action; it
  never establishes a fact. The lease says who MAY reap; the kernel says
  what IS dead. The two are never traded for each other.
- **Proof and action fuse.** Where a check and its consequence can be
  separated by a crash or an edit, they are made one operation: a lock
  is born owning (a directory renamed into place with its owner already
  inside), a reclaim proves death inside the same call that deletes, a
  reset IS its ledger line. If the guards can be reordered away from the
  act, someday they will be.
- **Single writer, append-only truth.** Every artifact has exactly one
  writer; contested state lives behind compare-and-swap or a lock that
  names its owner. The ledger is append-only and is the story of record:
  verdicts derive from replaying it rather than from cached state,
  because a cache can lag, tear, or lie, and a pure function of an
  append-only log cannot.
- **Fail toward the fuse, refuse loudly, never skip silently.** When a
  proof errs, the conservative side wins (an unprovable process is not
  dead; an unreadable measurement is not progress). Every refusal names
  itself. Silence is the one failure mode nothing may choose.
- **Nothing paid is silently discarded.** Work that happened is banked —
  a delegate's landed return, a capped turn's committed tree, the tokens
  a killed process already spent. Losing evidence is a defect even when
  losing it is convenient.
- **Bounded everything, with last defenses above the envelope.** Every
  loop has a cap, every wait a deadline, every mission a priced
  exposure. Backstops are sized so that only genuine pathology trips
  them: a fuse that fires during correct operation is not a safety
  mechanism but a design constraint wearing its label.
- **Vocal, once.** State changes a human must know about leave durable,
  attributable records — and an unchanged alarm is surfaced once, not
  every turn. Repetition is noise, and noise trains humans to ignore
  alarms.
- **Human authority at named boundaries.** Sealing and signing a
  contract, approving fences that spend money, resetting a stop-loss,
  authorizing metasystem self-repair during a mission (ruled, pending
  implementation): these are explicit human acts with durable records,
  never ambient states or inferred consents.
- **Equipment, not doctrine bleed.** The metasystem ships its ways of
  working INTO the missions it runs — design loops, critique, receipts
  applied to the product being built — and that is the point, not a
  leak. What must never happen is the mission working ON the metasystem
  itself without a signed grant. The methodology travels; the object of
  work does not.

## Identity and proof

**Kernel identity** is the triple (pid, exact start time, argv), read
from the kernel, never from a pidfile alone. **Three-way liveness**
follows: alive (same pid at the same start), dead (provably gone or
provably recycled), unknown (unreadable) — and unknown never authorizes
anything. **Proven death** is the only ground for takeover: locks,
leases, and reaps all require it, and its dual — a permission error
proves existence — is honored where the kernel hides details. An
**announcement** registers a live main in the checkout's registry;
**custody** ties spawned processes to the job that owns them, so a
reaper can tell a stranger from a charge.

## Authority and writing

The **checkout lease** makes one process the single writer of a
repository checkout; the **holder** is that process, the **epoch** its
ownership generation, the **lineage** the logical writer surviving
process restarts. The **control-plane authority matrix** says which
classified caller may write which artifact class in which mode. The
**commit wrapper** and its **token** put a provable ancestry between an
agent and every commit: a raw commit is refused; a wrapped one carries
the wrapper's kernel identity. Adoption ships the whole apparatus — the
engine included — into any repository the metasystem takes on.

## Supervision

**Arming** establishes supervision over a checkout in a fixed order:
announce, lock, launch, census, verify. The **owner** is the supervisor
process; its **components** — the **watcher** (census each interval) and
the **reaper** (terminal verdicts for over-budget and orphaned jobs) —
carry heartbeats with their kernel identities. The **census** inventories
agent-signature processes against the **fingerprint** — a digest of the
supervision code, signatures, and config that forces a re-arm when the
watched machinery itself changed. The census-writer **lock** is
rename-born; **generations** only rise, so a stale verdict can never
impersonate a fresh one; the **watchdog report** tells a session, once
per change, what supervision knows.

## Missions

A **mission contract** is a human-authored document: intent, non-goals,
streams, and a fenced block declaring the **gate** (the completion
measurement), **guards**, **noise floors**, **fences**, and the
**envelope** of pre-authorizations. **Sealing** freezes the instruments
and prices the **exposure** (a currency-denominated ceiling);
**signing** is a human approval over the exact sealed bytes; the
**preflight** re-proves all of it before launch. **Fences** — wall
clock, cycles, jobs, concurrency, per-job caps — are sealed allowances:
the money guards. The **runner** drives **cycles**: reserve, assemble
the prompt, launch the **host turn** under its cap, collect and
**adjudicate** the return, **drain** delegate jobs, **measure**, append
the **ledger**, conclude the hash-chained, **anchored** mission
**state**. **Streams** are the mission's parallel goals; **asks** are
questions parked for a human, answered through a recorded one-shot
path. **Classification** is what a cycle earned: `contract-improved`,
`unresolved`, `no-progress` — and the fuse below consumes exactly these.

TODO: delegation (roles, briefs, rounds, follow-ups, capability
snapshots, permission envelopes) deserves its own section; today the
glossary and `docs/orchestration.md` carry it.

## Patience, progress, stall

The full treatment is `docs/patience.md`; the concepts in one breath:
**progress** is value produced and proven mechanically, per activity;
**patience** is how much observation without progress is tolerated, a
property of who is working (per role, per model) and always a last
defense, never a pacing target; **stall** is the verdict when patience
exhausts — a vocal park a human can reset through the ledger. **False
stall** is the enemy class: any accounting that books honest work as
stagnation (a punished truthful session, a starved dispatch, an orphaned
return). The **stop-loss** is patience's shipped core: a pure replay of
the ledger — the fuse consults the story of record, not a counter that
could drift from it. **Announced versus observed** identity carries the
same philosophy into session bookkeeping: the announcement is a hint,
the harness's own observation is the witness, and honesty is never
punishable. A **witness** reports; only the runner **judges**.

## The design process

The metasystem is built — and builds — through an adversarial **design
critique loop**: a design note, a critic attacking it at declared
effort, **material findings**, **dispositions** joined mechanically
(closure is a set equality, not a feeling), rounds until a joined round
has zero material findings. **Exhaustion is not agreement**: a loop that
stops converging escalates to the human. When findings cluster in a
severable region, the **generating cause** is severed: the parent splits
into a **core** (small enough to converge and ship) and **satellites** —
evidence-born design units inheriting the parent's ruling and its routed
findings, each converging alone, ordered by dependency on truth (make
the signal honest before building what consumes it). Designs are written
against a **ground-truth map** of the code's actual sequence, because
the reference failure died of designing against an assumed one.
**Receipts** record what shipped; **handoff notes** in `plans/` claim
and transfer streams of work.

## Cross-references

`docs/glossary.md` (terms), `docs/patience.md` (the patience program),
`docs/design/mission-cycle-sequence.md` (the runner's ground truth),
`docs/orchestration.md` (delegation and modes),
`skills/design-critique/SKILL.md` (the loop's rules, the exhaustion
precedent, satellites), `plans/stop-loss-core.md` (the shipped fuse).

TODO: the benchmark kit's concepts (cohorts, repetitions, held-out
graders, seal boundaries, trial fences and their calibration rule).
TODO: the flight recorder and evidence lifecycle (events, mirrors,
manifests, collection) as one story.
TODO: gated metasystem self-repair during missions (ruled 2026-08-11:
a signed contract setting; halt-and-report when blocked without it) —
design pending.
