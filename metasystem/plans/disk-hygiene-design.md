- Status: IMPLEMENTATION-FIRST (D81/D85) — r1/r2 folded, r3 (14 findings) is the fixture list; building highest-value slices behind fixtures rather than a 4th prose round. Slice 1: the headroom guard (the ENOSPC fix) — SHIPPED, both hosts green; hardened 2026-08-17 after its retroactive code review (plans/opus-window-review-dh.md part A, D95): fd-pinned measurement, ENOENT-only ascent (every other establish failure refuses), checked arithmetic and floor validation, the suite distinguishing measure-failure (refuse) from below-floor (advisory), a df bootstrap check on clean checkouts, and the documented rule that entries are per-path advisories (APFS volumes share a container pool across distinct device ids — never sum entries). Slice 2 attempt: a worktree observer (`janitor worktrees`) classifying the dispatch-owned-worktrees row by "job terminal + custody dead" — BUILT then REVERTED (D89). Its mandatory code critique (plans/wt-code-critique-r1.md, 5 structural) proved the verdict UNSOUND for any future reclaimer to trust: it classified as reclaimable three implementer worktrees still holding UNMERGED work (a modified dispatch.sh in caps-census-gate-order), because terminality is NOT a data-release proof — conformance review and merge read the worktree AFTER the job terminates. The corrected worktree-reclaim proof is captured below; the accumulation it surfaced (118 dirs / ~500MB) is recorded as KI-35, REPORT-ONLY: the earlier "safe manual cleanup" advice was WITHDRAWN (the dh review F13 — ignored data, committed-but-unmerged branches, and a repository-global prune made it unsafe); no manual bulk cleanup until the journaled reclaim exists.
  Critiques r1 and r2 are folded into this text (12 and 15
  findings). The r3 verdict landed after the park: REVISE, 14
  findings (13 structural), preserved UNFOLDED at
  plans/dh-critique-r3.md — trajectory 12, 15, 14, budget one
  exhausted without convergence.
- Goal: disk-hygiene (parked; yielded to acp-transport under D81)
- Next step (on resume): fold plans/dh-critique-r3.md first. The
  trajectory shows the same execution-heavy signature D81 rules
  on; on resume, weigh entering budget two against going straight
  to implementation-first with the r3 findings as the first
  fixture list.

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
| dispatch-owned worktrees | delete-after-use, janitor, proof: DATA-RELEASED (see "The worktree-reclaim proof" — terminal + group quiescent is NOT sufficient) |
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

## The worktree-reclaim proof (corrected by plans/wt-code-critique-r1.md)

The slice-2 attempt proved that "job terminal + custody dead" is
NOT a sound proof that a dispatch worktree is disposable. A sound
reclaim of `artifacts/agents/worktrees/<job>` must hold ALL of:

1. **Data released, not just terminal (critique F1; corrected by
   the opus-window dh review F8 — git is NOT a release proof).**
   Terminality is a record-state, not a merge/review state:
   conformance review and the authoritative-diff computation READ
   the worktree after the implementer terminates, and CloseCheck
   admits a completed, unreviewed implementer with no computed
   diff. Reclaim needs an explicit DURABLE release decision
   (reviewed AND merged-or-discarded by decision AND no downstream
   consumer), recorded BEFORE removal begins, with recovery
   semantics for partial failure. `git worktree remove` without
   --force is a useful LAST refusal, never the proof: it passes a
   clean branch holding committed-but-unmerged work, deletes
   IGNORED data without protest (nine currently-clean delegate
   worktrees hold ignored artifacts/config/caches), refuses
   populated submodules (making them unreclaimable rather than
   handled), leaves detached-HEAD reachability unaddressed, and is
   not a side-effect-free predicate (its check and recursive
   deletion are one operation that can continue past a failure).
2. **Group death, not custody death (F2).** The custody list holds
   the direct CLI child only; grandchildren survive reparenting and
   a failed handshake writes `failed` before a best-effort wind-down
   that may not complete. Proof needs process-GROUP death (pgid,
   no-escape enrollment), not a dead custody entry, and an empty
   custody list is NOT quiescence.
3. **Alias resolution under the chain lock (F3).** Follow-up rounds
   (`<job>-rN`) inherit the root's workspace, so a running `-r2`
   uses `worktrees/<root>` while `<root>.json` reads terminal.
   Reclaim must resolve EVERY record referencing the canonical
   workspace (workspaceRoot, which is inconsistently populated —
   sometimes null, sometimes the repo root) and revalidate while
   holding the same lock that excludes follow-up creation (TOCTOU).
4. **Ownership + containment (F4).** Prove dir-name == jobId ==
   the record's workspaceRoot, the dir is a registered git worktree,
   and the path is canonical with no symlinked ancestor before any
   RemoveAll-style operation.
5. **Same-user + platform-capability trust for Dead (F5, refined
   by the dh review F11).** ENOENT-as-Dead is only sound under
   same-user scope and non-restrictive process visibility, expressed
   per platform capability (macOS identity inspection is sysctl,
   not procfs); a reclaimer must enforce/prove that, as supervision
   does.

Additions the opus-window dh review (plans/opus-window-review-dh.md
F9–F12) proved necessary beyond the original five:

6. **Records must outlive the proof (F9).** Evidence GC prunes
   terminal job records after a 5400s grace, yet the proof needs
   them for lineage, aliases, ownership, and release — already 12
   of 118 worktrees have no same-named record and four more lack a
   workspaceRoot. Reclaim needs a durable per-worktree
   ownership/release record retained UNTIL reclaim, with record
   pruning serialized against reclamation; missing or malformed
   records fail closed (report-only).
7. **A canonical-workspace lease, not just the chain lock (F10).**
   Fresh dispatch can accept an arbitrary workspace under a
   DIFFERENT job's lock, and conformance readers take no chain
   lock — one lineage lock cannot fence all aliases and readers.
   The resource needs a canonical-workspace lease shared by fresh
   dispatch, follow-ups, conformance, and the reclaimer.
8. **A closed consumer set, not group-death (F11).** An empty
   original process group excludes neither a setsid descendant,
   another dispatched job, conformance, nor any same-user process
   with a cwd or open descriptor inside the tree. The proof needs
   an enforceable closed consumer set or a reclaim-time same-user
   kernel census tied to the canonical path.
9. **A child-first descendant inventory (F12).** The candidate
   being a registered worktree says nothing about registered
   descendant worktrees, nested repositories, or independently
   owned resources inside it — a same-device descendant bypasses
   the mount-crossing check. Unsupported nested cases stay
   report-only; refusal never falls back to forced removal.

This is the full journaled destructive slice, not a report. Until
it exists there is NO sound worktree reclaim, automatic or manual:
KI-35's earlier "safe manual cleanup" advice was corrected to
report-only (the dh review F13 — ignored data, unmerged branches,
and a repository-global prune made it unsafe as written).

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
