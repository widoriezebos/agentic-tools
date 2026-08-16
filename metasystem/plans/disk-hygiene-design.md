# Disk hygiene: every byte written gets a lifecycle (backlog item 19)

- Status: DRAFT r3 — critiques r1 and r2 folded (r1: 12 findings;
  r2: 15, of which 14 structural — six r1 folds were shown
  incomplete and are re-folded for real below). Final round of the
  first budget.
- Goal: disk-hygiene
- Next step: Fold the critique verdict when run dh-critique-r3 concludes; implement only after convergence.
- In flight right now: run dh-critique-r3 (codex xhigh critique; watch it with: bin/metasystem run watch --id dh-critique-r3 --root .)

The human's ruling, verbatim intent (2026-08-15): "Whenever we
write to logs or temp files we need to factor in that these need
to be managed either by deleting them or distilling import[ant]
information from them and then deleting them. Or archiving them."
The incident: ENOSPC at 149MB free, a 29GB Go build cache, a
read-only Lima guest, a dead suite wearing a code failure's
clothes.

## The class schema

One Go-owned registry (internal/janitor). A class declares:

- **base + pattern** — inside one of the TWO SWEEP DOMAINS (r2
  F8): the owned temporary namespace, or a repo-rooted artifacts
  base. Both are swept; each has containment rules; nothing else
  is ever swept.
- **lifecycle** — five shapes: `delete-after-use`,
  `distill-then-delete`, `retain-with-cap`,
  `archive-verified-then-delete`, `capped-cache` (r1 F6).
- **quiescence proof** — a named proof; AGE IS NEVER AN ORPHAN
  PROOF, with ONE explicitly justified exception class (r2 F7):
  zero-content coordination artifacts (lock temps) whose protocol
  bounds holder lifetime — for those, the protocol's bound IS the
  proof, and the class is marked as carrying that justification.
  This names the evidence GC's existing lock-temp behavior
  instead of silently contradicting it.
- **outcome transition** (r2 F6) — a class may declare a
  FAILURE-PROMOTION target: on success the writer deletes; on
  failure the object durably ENROLLS into the retained target
  class. Cross-device promotion publishes destination-local and
  verifies BEFORE source deletion; incomplete promotion retains
  the source in place.
- **ceiling, age, floor** — floors are schema fields pressure
  cannot cross.
- **enforcement owner** — janitor, or a DELEGATED owner. r2 F7's
  correction: delegation is NOT a lifecycle. A delegated surface
  must still publish EXECUTABLE CLASS RECORDS — the evidence tree
  splits into owner-supplied SUBCLASSES (verified archives:
  archive-verified-then-delete; residue prunes: their own rule;
  lock temps: the justified exception above), each with lifecycle
  and proof, so the registry describes reality instead of
  pointing at a black box. The janitor still never walks
  delegated trees; owners enforce and return structured outcomes.

Registry validation: duplicate patterns, overlapping bases,
missing distillers, missing proofs, destructive classes without
ceilings, distillers without a declared RESIDUE CLASS (r2 F14),
and journals without lifecycles all fail Validate. The sweep
never deletes what no class claims; unknown accumulation is
reported; the report is diagnostic, the INVENTORY is compliance.

## The owned namespace and the staged migration (r1 F2; r2 F8)

The namespace: `${TMPDIR}/metasystem/<checkout-key>/<run-or-job-id>/…`,
ownership record per root (class, creator pid + start, created
at). Sweeping rejects symlinks, mount crossings, malformed
records, escapes. Generic `/tmp/tmp.*` litter is report-only
forever.

The migration is STAGED, because a suite may be running while
the upgrade commits (r2 F8): (1) ship the versioned allocator and
a report-only reader; (2) migrate every writer — the suite's
mktemp roots, gate temp files, supervision-hook payload/error
temps, dispatch recovery/setup files, benchmark provisioning
scratch, receipt evidence; (3) enable destructive recognition
LAST, only for roots carrying the new ownership records. New
scripts feature-detect old engines; legacy roots and in-flight
records stay report-only until their original owners finish. A
mixed-version running-suite fixture proves the staging.

## The operation journal and the distill transaction (r2 F1, F14)

ONE durable operation journal precedes EVERY destructive action —
not only distills. Reports are COMPILED FROM the journal
(including recovered prior-sweep actions) and published
immutably before journal retirement, closing the
crash-between-delete-and-report hole. Journals and reports are
themselves declared classes with bounded compaction.

The distill transaction, write-ahead and crash-durable:

1. Synced `prepared` journal entry — BEFORE any filesystem
   change.
2. Unique, no-replace CLAIM: rename into the class's quarantine;
   then SYNC the source and quarantine directories (the
   internal/atomicfile publisher's discipline — the GC's
   temp+rename alone does not sync the directory).
3. Durable `claimed` entry.
4. Residue via the durable two-outcome writer
   (internal/atomicfile), enrolled into its DECLARED residue
   class; residue durability positively verified.
5. Delete the quarantined source; durable `completed` entry.

Recovery defines every journal-state × filesystem-state pair.
Failure or source mutation retains. **Quarantine placement is
per-class and ON THE SOURCE DEVICE** (r2 F2): each destructive
class declares its quarantine + journal location on the device
it claims; device equality is verified immediately before the
claim; no safe local quarantine → retain and report.

## Quiescence, strengthened to the real topologies (r2 F3, F4)

- **Suite roots**: creator death is NOT enough — the suite
  launches detached supervision writers that outlive shutdown.
  Each suite root carries a durable WRITER SET (every detached
  writer enrolls at spawn); reclamation requires the creator dead
  AND every member dead or explicitly released.
- **Run-registered logs**: terminality is NOT enough — wind-down
  can expire with the group alive. A run log becomes reclaimable
  only in a new `log-sealed` state: terminal record AND group
  empty AND logger EOF (or an enforced no-descriptor-escape
  contract). Bare terminality leaves the destructive proof
  taxonomy entirely.
- **Run log ownership** (r2 F4): the destructive default applies
  ONLY to logs created by `run launch` inside the owned
  namespace. Adopted or caller-path logs (today's registration
  proves location, not ownership) require explicit ownership
  transfer plus ALIAS CHECKS — records demonstrably share one
  leash log today, so lifecycle completion waits until NO other
  record references the path. Existing arbitrary-path records
  are report-only during migration. The gate and leash logs in
  /tmp/mon-gate are NOT a hard-coded janitor row (r2 F2): future
  runs launch them inside the namespace via `run launch`; legacy
  ones are report-only.

## The sweep, the report, and the one coordinator (r2 F9, F10)

`janitor sweep` enforces janitor-owned classes with the journal
discipline and collects delegated owners' outcomes. One
immutable, bounded report per sweep, compiled from the journal;
new reports record old-report evictions.

**One coordinator lock** (r2 F9): both public entrypoints — the
janitor sweep and the standalone evidence-GC wrapper the Stop
hook invokes — acquire the SAME operation coordinator exactly
once, in one fixed order relative to the checkout lease (lease
first, matching the shipped wrapper). A combined pass calls an
UNLOCKED in-process GC primitive so it cannot self-deadlock.
Lock location, holder identity, death-only takeover, bounded
wait, and a structured busy outcome are specified. The GC's
current tolerate-concurrent-collectors behavior is superseded by
the coordinator for both entrypoints.

**Active logs are writer-owned and unreapable** (r2 F10):
hooks.log's concurrent appenders make rename-rotation unsafe (a
renamed inode still receives writes). The supervision hook moves
to ONE IMMUTABLE, ATOMICALLY PUBLISHED FILE PER INVOCATION in a
retain-with-cap directory; the janitor touches only files with
durable sealed markers. The same rule covers the whole
supervision and flight-recorder family (r2 F15): owner.ndjson,
arming.log, and events.jsonl get writer-owned bounded rotation;
only SEALED segments enter retained or verified-archive
lifecycles; active files are never sweep candidates.

## Pressure mode (r1 F5; r2 F13)

Deficit-driven, per-filesystem, device-scoped — reclaim only on
the short device; floors uncrossable; a preallocated emergency
reserve guarantees the pressure pass can record itself; outcome
`recovered` | `still-below-floor`, the latter refusing startup
with byte accounting. Two r2 additions:

- Pressure passes (device, deficit) to DELEGATED owners, which
  may delete only already-verified, minimum-retention-satisfied,
  RELEASED sources on that device — and never create a new
  archive on the short device as a side effect.
- The benchmark provision path gets a NON-ENGINE bootstrap
  headroom check (plain df arithmetic before any compilation),
  because provisioning can run a large fallback Go build before
  it ever resolves the target — the guard must precede the first
  byte, not the first engine call.

## The owned Go cache (r1 F1; r2 F11)

The user's shared cache is never touched (diagnosed only). The
owned cache binds at ONE shared Go-command boundary used by the
gate, go-build.sh, adoption, and provisioning — not per-script
exports that adoption would miss. Its key is the STABLE LOGICAL
SERVING-CHECKOUT identity, propagated into witness snapshots by
environment — never path-derived, because witness snapshots get
fresh paths every suite and a path key would run every gate
cold. A whole-gate/build LEASE blocks sweep and pressure eviction
until the gate completes — the gate prewarms race binaries
because cold compilation starves timing fixtures, and an
eviction mid-gate would reintroduce exactly that flake class.
Validation measures cold and warm full-gate times and proves
reuse across suites.

## The inventory (r1 F11; r2 F6, F15 — every known writer)

| Surface | Class / owner |
|---|---|
| owned Go cache | capped-cache, janitor, gate-lease protected |
| suite temp roots (migrated) | delete-after-use, janitor, proof: creator dead + writer set empty |
| gate temp files (migrated) | delete-after-use, janitor; failure-promotion → gate-failures |
| gate-failure preservations | retain-with-cap, janitor |
| suite-failure preservations | retain-with-cap, janitor |
| receipt-probe evidence | failure-promotion into the suite-failure class (r2 F6) — it exists to keep intermittent-failure diagnostics, so it is NOT delete-after-use |
| supervision hook records (was hooks.log) | one immutable file per invocation, retain-with-cap, janitor |
| owner.ndjson / arming.log / events.jsonl | writer-owned bounded rotation; sealed segments retained or archive-verified; active files unreapable (r2 F15) |
| run-launch logs (namespace) | distill-then-delete at log-sealed, DELEGATED to internal/run |
| adopted/caller-path run logs | report-only until ownership transfer + alias clearance (r2 F4) |
| per-job round evidence | DELEGATED to evidence-gc, published as subclasses (r2 F7) |
| dispatch-owned worktrees | delete-after-use, janitor, proof: job terminal + group quiescent |
| dispatch recovery/setup temps (migrated) | delete-after-use, janitor |
| external/session scratchpads | report-only unless registered via adapter capability |
| benchmark resources | kit-declared via the envelope below; `.evidence` is a PROTECTED evidence-GC destination, never a disposable source (r2 F5) |

## The kit surface: generic envelope + kit state machine (r1 F7; r2 F5, F12)

The engine offers a versioned GENERIC envelope of repeatable
resource rows: {resourceId, canonicalPath, containmentAnchor,
custodyToken, lifecycleRef, opaque kit metadata}. The engine
validates and persists generic invariants (containment,
registration-before-first-write, custody); the KIT enforces
policy and returns outcomes — no kit layout is compiled into the
engine, and engine evolution decouples from kit evolution.

EACH creating command registers its own resources before its
first write — run-cohort registers cohort state and the durable
results store (different lifecycles: operational state versus
durable results), provision registers targets and origin
siblings. Deletion authority comes from the KIT'S DURABLE STATE
MACHINE — provisioning, awaiting-approval, mission-running,
draining, grading, archived, released — never from provisioner
death or lease retirement, because the trial deliberately sits
unattended between provisioning and the human's signing (r2 F5).
Only `released` (plus verification and minimum retention)
authorizes deletion; `.evidence` removal requires an explicit
ownership handoff from the evidence GC.

## Boundaries

Agnosticism: no runtime names in the core; provider scratch
roots arrive via the adapter capability seam. The narrator owns
prose; distillers produce residue files. Host-global resources
beyond the owned cache: out of scope.

## Validation

Everything from r2 plus: journal-state × filesystem-state
recovery table exercised at every crash boundary; quarantine
device-mismatch refusal; suite-root writer-set enrollment and
the detached-writer survival case; log-sealed gating including
the shared-leash-log alias case; per-invocation hook records
under concurrent Stop hooks; coordinator lock ordering across
both entrypoints with the lease, including the busy outcome and
death takeover; GOCACHE warm/cold gate timing and the gate-lease
eviction block; failure-promotion atomicity across devices;
bootstrap headroom refusal before provisioning's first build;
kit envelope containment (unregistered writes refused);
delegated pressure reclaim honoring released-only; the
mixed-version running-suite migration fixture.

## Blast radius

internal/janitor (registry, sweep, journal, transaction, report,
pressure, coordinator), cmd/metasystem (janitor verbs, headroom),
internal/run (log-sealed state, launch-owned logs, ownership
transfer, alias checks, prune refusal), internal/evidence
(subclass publication, unlocked primitive, coordinator
integration, reservedDirs), internal/events (rotation sealing),
cmd/metasystem/supervise_owner.go (rotation),
scripts/agents/supervision-hook.sh (per-invocation records),
scripts/agents/arm-supervision.sh (rotation),
scripts/agents/go-gate.sh + go-build.sh (single GOCACHE
boundary, gate lease, temp migration),
scripts/validate-metasystem.sh (namespace migration, headroom,
teardown sweep, writer-set enrollment),
scripts/agents/dispatch.sh (worktree class, recovery-temp
migration), the benchmark kit (bootstrap headroom, envelope
registration calls, the kit state machine — kit-owned), docs,
fixtures. NOT touched: the user's shared Go cache, the GC's
archive-verification logic (it gains subclass publication and
the coordinator, not new policy — except the lock-temp exception
is now NAMED rather than silent), the kit's grading logic.

## Loop discipline

Codex at xhigh; two-budget allowance; stop on zero unrefuted
material findings or the ratified exits. History: r1 12
findings, r2 15 (six r1 folds shown incomplete — the re-folds
above are structural, not editorial). This closes the first
budget; the trajectory is not falling, so if r3 still carries
structural findings the second budget opens with the stop
criterion checked against what remains. The r3 critique should
attack: whether the journal-first discipline is actually total
(name a destructive action that escapes it); whether the
writer-set enrollment for suite roots can be implemented without
the suite knowing every future detached writer's identity at
root creation; whether log-sealed is achievable for the leash
pattern (a poller writing until its own exit); whether the
coordinator supersession of the GC's two-collector tolerance
breaks the Stop hook under real concurrency; whether the
single-GOCACHE boundary genuinely covers every go invocation in
the repo (name one outside it); whether the kit envelope's
generic rows survive the kit's actual command topology; and
whether any inventory row still hides an unmanaged byte.
