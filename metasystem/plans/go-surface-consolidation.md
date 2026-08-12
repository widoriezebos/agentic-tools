# Go surface consolidation: unimport the shell's architecture

Working Mode: design
Mission Stream: kill-shell

Status: DRAFT under critique; the TARGET TREE below additionally needs
the human's sign-off before any implementation (his standing ruling of
2026-08-12 created this program and he has corrected course twice —
the shape is his call). This program replaces kill-shell Phases B–F.

## Problem

The Go port succeeded at the engine layer and failed at the surface.
The binary exposes 29 families and ~180 verbs; most verbs exist
because one shell line once needed one decision extracted, not because
the design wanted them. Consequences, concretely:

- The mission domain is split across EIGHT families (mission-state,
  -fence, -contract, -prompt, -runner, -turn, -jobs, -ledger).
- dispatch.sh sequences ~24 micro-verbs; the job lifecycle's
  invariants live in bash call ordering, in neither language.
- adapter (34 verbs) and host (8) carry near-duplicates
  (devin-config, devin-usage, fake-return appear in both).
- Scripts were demoted to shims carrying no logic, while their usage
  texts and argument conventions were embedded INTO Go verbs to keep
  byte-identical behavior — shell contracts preserved in Go aspic.
- Bookkeeping grew around the migration itself: a 55-entry disposition
  registry with shapes, verdicts, debts, and export conditions.

The ruling this program executes: core decisions belong in Go,
plumbing belongs in scripts. Scripts are not debt; script-shaped Go
is.

## Target tree (needs human sign-off)

Ten families. Verb counts are ceilings, not quotas; the deletion
census (step 0) sets the real numbers.

| target | absorbs | notes |
| --- | --- | --- |
| `job` | dispatch (24), capability (1), authority (1), schema (1) | The delegate-job domain. Record lifecycle verbs coarsen where bash ordering currently carries an invariant: record-create+record-setup become one `job reserve` with the husk rule inside; the reap verdict LADDER becomes one `job reap-verdict` (decisions only — wind-down signaling stays in dispatch.sh, which executes the verdict). owner-lock, chain queries, cap-resolution keep their grain: they are single decisions. |
| `mission` | mission-state, -fence, -contract, -prompt, -runner, -turn, -jobs, -ledger, evidence (9 families, 33 verbs) | Pure regrouping under one family (`mission seal`, `mission fence-refuse`, `mission ledger-append`, `mission gc`). No behavior change. |
| `runtime` | adapter (34), host (8) | The fragments the adapter/host scripts call at the custody boundary. Deduplicate the doubled verbs, delete unused ones; the scripts KEEP custody (launch, wait, signal) per the ruling — no driver interface in Go. |
| `proc` | identity (4), census (7), supervise (10) | Process identity, census, supervision — one domain: who is running, provably. |
| `validate` | unchanged (12) | Whole-artifact validators. |
| `audit` | unchanged (2) | metasystem + coverage-ratchet. |
| `config` | unchanged (7) | |
| `lease` | unchanged (10) | Worktree session custody. |
| `receipt` | unchanged (5) | |
| `util` | util (5), json (3), event (1), gate (3), hooks (1), report (3) | Grab-bag survivors; regrouping these renames hundreds of call sites for cosmetics, so they merge under `util` ONLY where a call-site sweep is already touching the file — otherwise they keep working under their old names via the alias table until organically gone. |

CLI compatibility during migration: the family router gains a
one-table alias layer (old family/verb → new) so scripts migrate per
commit, not big-bang; the alias table is deleted at the end.

## What gets deleted

1. Step 0 is a caller census: every verb greps against scripts/,
   internal/, cmd/, skills/, hooks. A verb with zero callers dies in
   the same commit that records the census. Known suspects: the
   adapter/host duplicates, exhaustion-patches, selftest fragments.
2. The disposition registry shrinks to a plain ship-list (what
   adoption exports). Shapes, verdicts, debts, and verified dates were
   bookkeeping for the retired shim program; the reviewed-verdict
   discipline lives in review, not in JSON.
3. The usage texts embedded in Go verbs (design-obligations,
   stop-loss, conformance) move back to their wrapper scripts, which
   become ordinary scripts again — parsing their own arguments,
   printing their own help, calling clean Go verbs with Go-native
   flags. Byte-identical stderr stops being a goal; on-disk formats
   and exit codes remain contracts.
4. RETIRED: the Phase F fixture-python elimination. Fixture sequencers
   are plumbing; their python3 heredocs are fine. (Kill-shell's
   "python dies at Phase F" goal is void; production python is already
   zero and stays zero by review, not by fence.)

## Migration order

Each step lands with the suite green from a pristine worktree and all
call sites updated in the same commit.

1. Caller census + dead-verb deletion + adapter/host dedupe. Output:
   the real verb count and the alias table.
2. mission-* merge (mechanical; largest coherence gain per hour).
3. job-family formation: dispatch verbs regrouped, reserve and
   reap-verdict coarsened, dispatch.sh updated to consume them —
   the lifecycle invariants move from bash ordering into two Go
   functions with unit tests. dispatch.sh keeps launch, wind-down,
   and polling custody.
4. runtime/proc regrouping; scripts restored to ordinary scripts
   (usage texts return; shims that stay are one-liners by choice, not
   doctrine).
5. Registry shrink to ship-list; alias table deletion.

Estimated two working sessions. Steps are independently valuable;
stopping after any of them leaves the system better than before it.

## Non-goals

No engine rewrites (missionrunner, lease, conformance, patience stay
as they are). No new fences or meta-machinery. No byte-identical
output conformance for renamed verbs — scripts calling them are
updated in the same commit; external contracts (file formats, exit
codes, hook entry points, adopted-repo script names) do not change.
