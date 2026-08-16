# Disk hygiene: every byte written gets a lifecycle (backlog item 19)

- Status: DRAFT r1 — awaiting critique r1
- Goal: disk-hygiene
- Next step: Fold the critique verdict when run dh-critique-r1 concludes; implement only after convergence.
- In flight right now: run dh-critique-r1 (codex xhigh critique; watch it with: bin/metasystem run watch --id dh-critique-r1 --root .)

The human's ruling, verbatim intent (2026-08-15): "Whenever we
write to logs or temp files we need to factor in that these need
to be managed either by deleting them or distilling import[ant]
information from them and then deleting them. Or archiving them."
The motivating incident, same day: ENOSPC at 149MB free — a 29GB
Go build cache after a night of snapshot gates — cascading into a
read-only Lima guest that killed a running suite in a way that
looked like a code failure.

## The contract: declared artifact classes

One Go-owned registry (internal/janitor, the family the mandate
names as host) of ARTIFACT CLASSES. A class declares:

- **base + pattern**: where the class's files live (repo-rooted
  for artifacts, absolute for the temp roots the suite already
  uses).
- **lifecycle**: exactly one of `delete-after-use` (the writer
  deletes; the janitor sweeps orphans by age),
  `distill-then-delete` (a named distiller reduces the file to
  its durable residue, then deletes), or `archive-with-cap`
  (retained under a size/count ceiling, oldest reaped first).
- **ceiling and age**: per-class caps; absent means the class
  must justify itself at declaration (no undeclared unbounded
  class).

The registry is data, validated like the runtimes registry
(duplicate patterns, overlapping bases, missing distillers fail
Validate). Writers do not annotate writes; they write into
class-shaped locations. **The sweep never deletes what no class
claims** — unknown accumulation under a swept base is REPORTED
loudly, not reaped. Fail-safe over tidy.

## Enforcement: the janitor sweep

A `janitor sweep` verb walks the declared classes and enforces
lifecycles: age out delete-after-use orphans, run distillers,
enforce ceilings oldest-first. Every reap appends to a janitor
REPORT (itself a declared archive-with-cap class): what was
deleted, why (age | ceiling | distilled), how many bytes — so
evidence discipline survives deletion; nothing vanishes silently.
The sweep takes the existing GC's discipline (single writer,
deterministic ages, refusal without its lock). Two modes:
`routine` (declared ceilings) and `pressure` (invoked by the
headroom guard: tighter ceilings, but evidence-bearing classes
never reap below their declared floor — pressure trims comfort,
not proof).

Trigger points: the suite's teardown (routine), the headroom
guard (pressure), and by hand.

## The known surfaces, each assigned (the incident inventory)

| Surface | Class / policy |
|---|---|
| Go build cache (29GB that day) | not a repo artifact — owned by the HEADROOM GUARD: below the floor, `go clean -cache` runs as the guard's declared recovery step, loudly, before the suite is allowed to start |
| suite temp roots (/tmp/tmp.*, /tmp/metasystem-*) | delete-after-use; orphan sweep by age (they leak on kills today) |
| suite-failure preservations (797MB, unbounded) | archive-with-cap: count + size ceiling, oldest-first |
| VM suite logs in /tmp (one per boundary, never reaped) | distill-then-delete: verdict line + failure excerpt survive as the residue |
| gate logs (/tmp/mon-gate/*) | distill-then-delete, same distiller shape |
| benchmark cohort targets + .evidence siblings | the ENGINE provides the mechanism only; the kit declares its own classes and caps (benchmark specifics stay in the kit) |
| session scratchpad suite-pin worktrees (full build trees) | delete-after-use; age-swept |
| per-job round evidence (double-snapshotted transcripts) | already owned by the evidence GC — this design does NOT create a second owner; the class registry POINTS at the GC as that surface's enforcement, and the janitor report includes the GC's pass so one report covers the disk |

## The headroom guard

A free-space floor checked at the two places that today assume
space: suite startup (the gate guard that already fences
concurrent runs gains the check) and the benchmark provision
path. Below the floor: refuse to start, run the pressure sweep
plus the build-cache recovery, re-check, and only then proceed —
with the refusal and recovery in the log. The ENOSPC cascade
(read-only Lima guest masquerading as a code failure) is the
fixture story: the guard exists so a disk problem NAMES itself
instead of wearing a test failure's clothes.

## Boundaries

- No runtime names anywhere (agnosticism): classes describe
  paths and lifecycles, never which agent wrote them.
- The narrator (item 20) owns storytelling; distillers here
  produce residue files (verdict + failures), not prose.
- Run records already know their logs (item 15); the VM-log
  distiller reads the run record's log path, it does not invent
  discovery.
- Host-global resources beyond the build cache (system /tmp
  writ large, other tools' caches) are out of scope: the
  metasystem manages what the metasystem writes, plus the one
  cache the incident proved it inflates.

## Validation

Fixtures: a sweep over a synthetic tree proves age-out, ceiling
enforcement oldest-first, distiller residue, unknown-accumulation
reporting without deletion, the report's own cap, and pressure
mode's evidence floor. The headroom guard fixture fakes a
below-floor filesystem and proves refusal + recovery + loud
naming. The suite gains one end-of-run routine sweep invocation.

## Blast radius

internal/janitor (class registry + sweep), cmd/metasystem
(janitor sweep + headroom verbs), internal/evidence (report
integration point only), scripts/validate-metasystem.sh (startup
headroom check, teardown sweep), the benchmark provision script
(headroom check), scripts/agents/dispatch.sh (scratchpad class
base declaration), docs (operations section), fixtures. NOT
touched: the evidence GC's ownership of job evidence, the kit's
internal archive doctrine.

## Loop discipline

Critique rounds with codex at xhigh; two-budget allowance; stop
on zero unrefuted material findings or the ratified exits. The
critique should especially attack: whether the class registry can
actually claim the real paths (suite temp roots are created by
mktemp under /tmp — can a pattern claim them safely without
claiming other tools' files?); whether never-delete-unknown plus
pressure mode leaves the incident's actual failure (a full disk)
unfixable when the bytes sit in unclaimed paths; whether
`go clean -cache` as a guard recovery step is sound on a machine
where OTHER checkouts share that cache; the distiller contract
(who guarantees the residue was durably written before the
source dies); and whether one janitor report satisfies the
evidence-discipline bar the mandate sets for deletions.
