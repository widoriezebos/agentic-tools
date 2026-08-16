# Disk hygiene: every byte written gets a lifecycle (backlog item 19)

- Status: DRAFT r2 — critique r1 folded (plans/dh-critique-r1.md:
  12 findings, 10 structural; all folded, none refuted)
- Goal: disk-hygiene
- Next step: Fold the critique verdict when run dh-critique-r2 concludes; implement only after convergence.
- In flight right now: run dh-critique-r2 (codex xhigh critique; watch it with: bin/metasystem run watch --id dh-critique-r2 --root .)

The human's ruling, verbatim intent (2026-08-15): "Whenever we
write to logs or temp files we need to factor in that these need
to be managed either by deleting them or distilling import[ant]
information from them and then deleting them. Or archiving them."
The motivating incident: ENOSPC at 149MB free — a 29GB Go build
cache — cascading into a read-only Lima guest that killed a
running suite in a way that looked like a code failure.

## The class schema (r1 F3, F5, F6)

One Go-owned registry (internal/janitor) of ARTIFACT CLASSES. A
class declares:

- **base + pattern** — inside the owned namespace (below) or a
  repo-rooted artifacts path. No class may claim paths the
  metasystem cannot prove it owns.
- **lifecycle** — the algebra now has FIVE shapes, because three
  could not represent the shipped contracts (r1 F6):
  - `delete-after-use` — the writer deletes; the sweep takes
    orphans only with quiescence proof.
  - `distill-then-delete` — transactional (below); residue
    survives, source dies.
  - `retain-with-cap` — local capped retention, oldest reaped
    first (was "archive-with-cap"; renamed because it archives
    nothing).
  - `archive-verified-then-delete` — the evidence collector's
    real contract: durable copy to a NAMED destination, manifest
    or digest verification, minimum retention, delete only after
    verification.
  - `capped-cache` — rebuildable bytes (build caches): reap to
    cap freely, no residue, no quiescence beyond not-open.
- **quiescence proof** (r1 F3) — every destructive class names
  HOW the sweep proves the writer is finished: exact pid + start
  time dead; process group empty; a terminal run/job lifecycle
  state; released benchmark custody; or a named owning state
  machine. AGE IS NEVER AN ORPHAN PROOF — age only schedules the
  attempt. Unknown or unreadable liveness reports and retains.
  Manual and pressure sweeps use the same proof.
- **ceiling, age, and floor** — the cap, the sweep-eligibility
  age, and (for evidence-bearing classes) the FLOOR that pressure
  mode cannot cross — a schema field, not prose (r1 F5).
- **enforcement owner** (r1 F6, F8, F9) — lifecycle POLICY is
  separate from enforcement OWNERSHIP. A class either belongs to
  the janitor sweep or names its DELEGATED OWNER (evidence-gc,
  run-facility, kit). The janitor NEVER walks a delegated class's
  tree; delegation means the owner enforces and returns a
  structured outcome for the combined report.

The registry is data, validated like the runtimes registry:
duplicate patterns, overlapping bases, missing distillers,
missing quiescence proofs, and destructive classes without
ceilings fail Validate. **The sweep never deletes what no class
claims** — unknown accumulation is reported loudly, and the
report is a diagnostic, not compliance: every KNOWN writer must
appear in the inventory below (r1 F11).

## The owned namespace (r1 F2)

`/tmp/tmp.*` can never be claimed — the pattern matches other
tools' files and the suite's own unqualified `mktemp` calls are
indistinguishable from strangers. So the writers MIGRATE: suite
temp roots, gate temp files, and every future scratch root move
under one dedicated physical namespace keyed by checkout and run
identity (`${TMPDIR}/metasystem/<checkout-key>/<run-or-job-id>/…`).
Each ROOT carries an ownership record: class, creator pid + start
time, creation time. The sweep operates ONLY inside the
namespace and rejects symlinks, mount-boundary crossings,
malformed ownership records, and any resolved path that escapes
the canonical base. Pre-existing generic `tmp.*` litter is
REPORT-ONLY forever; the incident's leaked roots get cleaned once
by hand, not by code that could eat a neighbor's files.

## The distill transaction (r1 F4)

`distill-then-delete` is a transaction, not a sequence:

1. Prove quiescence (the class's declared proof).
2. Atomically CLAIM the exact object: same-filesystem rename into
   the sweep's quarantine directory (rename fails → someone else
   owns it → retain, report).
3. Durably record intent (the journal entry precedes the work).
4. Write the residue via temp-file + fsync + rename (the GC's
   shipped publication pattern), then VERIFY it.
5. Delete the quarantined source; record completion.

Every step is idempotently recoverable: a crash between any two
steps leaves either the source in quarantine with intent recorded
(resume) or the residue published (finish the delete). Failure or
detected source mutation retains the source. Nothing is ever
deleted before its residue is durable and verified.

## The sweep and its report (r1 F12)

`janitor sweep` walks the janitor-owned classes, enforces
lifecycles with the above proofs, and asks each delegated owner
for its structured outcome. The report is ONE IMMUTABLE,
atomically published, size- and entry-bounded file PER SWEEP —
never an append-only growing file. Report files are themselves a
`retain-with-cap` class; a new report records the eviction of old
reports (no recursive write into the file being removed). Every
reap line carries: path, class, lifecycle verb, why (age |
ceiling | distilled | cache-cap), bytes, and proof used. Nothing
vanishes silently.

## Pressure mode (r1 F5)

Pressure is deficit-driven and filesystem-aware, not "tighter
ceilings":

- Floors and required bytes are declared PER PHYSICAL FILESYSTEM
  (the repo, TMPDIR, the benchmark target root, the evidence
  sibling, and the owned Go cache can all be different devices).
- The guard computes the DEFICIT on the specific filesystem that
  is short, then reclaims only eligible artifacts ON THAT DEVICE:
  capped-cache first, then expired delete-after-use orphans (with
  proofs), then retain-with-cap down toward floors. Evidence
  floors are schema fields pressure cannot cross.
- A bounded emergency reporting reserve (a small preallocated
  journal) guarantees the pressure pass can still record itself
  on a full disk.
- The outcome is an enum: `recovered` | `still-below-floor`. The
  latter REFUSES startup and reports exactly which owned,
  unknown, and floor-protected bytes stand between here and the
  floor — the operator decides, with names in hand.

## The Go build cache (r1 F1)

Automatic cleaning of the USER'S shared cache is off the table —
it would evict unrelated checkouts' work and still race new
writers. Instead: suite and benchmark Go commands set a
METASYSTEM-OWNED, checkout-keyed GOCACHE (a declared
`capped-cache` class inside the namespace, with its own ceiling).
Automatic recovery cleans only that owned cache. An oversized
legacy shared cache is DIAGNOSED in the headroom report and left
for explicit operator action. The 29GB incident becomes
impossible not because we clean harder but because the
metasystem's builds stop billing their bytes to a cache nobody
owns.

## The headroom guard

Free-space floors checked at suite startup (joining the existing
gate fence) and at benchmark provision, per filesystem the run
will touch. Below floor: refuse, run the pressure sweep, re-check
once, and only then either proceed or fail LOUDLY NAMED
(`still-below-floor`, with the byte accounting) — so a disk
problem never again wears a test failure's clothes.

## The inventory (r1 F11 — every known writer, assigned)

| Surface | Class / owner |
|---|---|
| owned Go build cache (new) | capped-cache, janitor |
| suite temp roots (migrated into the namespace) | delete-after-use, janitor, proof: creator pid+start dead |
| gate temp files (go-gate.sh, migrated) | delete-after-use, janitor, same proof |
| gate-failure preservations (artifacts/agents/gate-failures — unbounded today) | retain-with-cap, janitor |
| suite-failure preservations (797MB today) | retain-with-cap, janitor |
| receipt-probe evidence (${TMPDIR}/receipt-evidence — persistent today) | migrated into namespace; delete-after-use, janitor |
| supervision hooks.log (append-only today) | rotated: bounded segments, retain-with-cap on rotated segments, janitor |
| VM suite logs + gate logs (/tmp/mon-gate, leash logs) | distill-then-delete (residue: verdict + failure excerpt), janitor, proof: run record terminal |
| run-registered logs | DELEGATED to internal/run (r1 F9): the run facility seals and claims the log after terminality + group quiescence, records residue path and completion, and refuses record prune until the lifecycle finishes; generic sweeping never discovers run logs |
| per-job round evidence | DELEGATED to evidence-gc (r1 F8): sole policy and deletion owner; janitor never walks or pressure-ages it; its structured outcome joins the report; any new infra dir under artifacts/agents is added to reservedDirs |
| dispatch-owned worktrees (artifacts/agents/worktrees) | delete-after-use, janitor, proof: job terminal + group quiescent (r1 F10) |
| external/session scratchpads | REPORT-ONLY unless registered; a provider-created scratch root arrives via an adapter CAPABILITY recorded in the job lifecycle — the core never encodes a provider pathname (r1 F10) |
| benchmark targets, .evidence, .origin.git, cohort dirs | DELEGATED to the kit via the registration surface below (r1 F7); lifecycle archive-verified-then-delete per the kit's own doctrine |

## The kit registration surface (r1 F7)

A versioned kit→engine registration: PROVISION REGISTERS BEFORE
WRITING — canonical target path, origin sibling, evidence root,
cohort directory, identity, custody, and lifecycle. The engine
validates containment (registered paths must be inside the
declared trials root or the explicitly registered absolute
target) and enforces ONLY against registered instances. Kit paths
are never compiled into the core registry; the kit owns its
declarations and caps (benchmark specifics stay in the kit).

## Boundaries

- No runtime names in the core (agnosticism): provider scratch
  roots come through the adapter capability seam.
- The narrator (item 20) owns storytelling; distillers produce
  residue files, not prose.
- Host-global resources beyond the owned cache are out of scope;
  the shared user cache is diagnosed, never touched.

## Validation

Fixtures: age-out WITH quiescence proofs (a live writer refuses,
a dead writer's orphan reaps); ceiling enforcement oldest-first;
the full distill transaction including crash-at-every-boundary
recovery and source-mutation retention; namespace safety
(symlink, mount crossing, malformed ownership, escape attempt —
all refused); unknown-accumulation reporting without deletion;
report immutability, rotation, and eviction recording; pressure
mode's per-filesystem deficit math, floor protection, emergency
reserve on a full filesystem, and both outcome values; the
headroom guard's refuse → pressure → recheck → named-failure
path; delegated-owner outcomes joining the report; kit
registration containment (an unregistered target is refused);
concurrent sweep serialization (shared operation lock with the
GC when a combined pass runs — r1 F8).

## Blast radius

internal/janitor (class registry, sweep, transaction, report,
pressure), cmd/metasystem (janitor sweep + headroom verbs),
internal/run (registered-log lifecycle, prune refusal — r1 F9),
internal/evidence (orchestration boundary + reservedDirs
addition only), scripts/agents/go-gate.sh (owned GOCACHE, temp
migration, gate-failure cap), scripts/validate-metasystem.sh
(namespace migration of mktemp roots, startup headroom check,
teardown sweep, receipt-evidence migration),
scripts/agents/supervision-hook.sh (hooks.log rotation),
scripts/agents/dispatch.sh (worktree class declaration), the
benchmark kit's provision.sh (registration calls; the kit's own
class declarations), docs (operations), fixtures. NOT touched:
the evidence GC's policy and deletion logic, the kit's archive
doctrine (it gains a registration call, not a new owner), the
user's shared Go cache.

## Loop discipline

Critique rounds with codex at xhigh; two-budget allowance; stop
on zero unrefuted material findings or the ratified exits. r1
found 12 (10 structural); all folded, none refuted. The r2
critique should attack: whether the five-shape lifecycle algebra
is now closed over every writer in the inventory (name a writer
whose contract still has no shape); whether the namespace
migration list is complete and the migration itself is safe
mid-flight (a suite running during the upgrade); whether the
quiescence-proof taxonomy covers the actual writer topologies
(supervision processes that outlive the suite, run descendants
after leader death); whether the distill transaction's quarantine
rename is sound across the filesystems in play; whether pressure
mode's device-scoped reclaim can actually fix the incident's
shape (the shortage on one device, the reclaimable bytes on
another — what then); the janitor/GC lock interaction under the
stop hook's concurrent invocations; and whether the kit
registration surface is minimal enough to survive kit evolution
without engine changes.
