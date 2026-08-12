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

## What "script-shaped" means operationally (r1/GSC-R1-001)

The metric is NOT the verb count, and this program does not promise a
small number for its own sake — fragments at a custody boundary are
correct under the ruling. Script-shaped means: a bash file owns a CALL
SEQUENCE whose ordering carries a correctness invariant that no Go
function states. Step 0 therefore runs a SEQUENCE CENSUS beside the
caller census: for each script that calls three or more verbs of one
family in a fixed order, name the invariant the ordering protects (or
record that there is none). Sequences that carry an invariant are the
coarsening list; everything else is legitimate plumbing and stays.
Regrouping (mission, proc) is coherence work and is claimed as such,
not as de-shell-ification.

## Target tree (human-approved 2026-08-12; amended by round 1)

Eleven families. The adapter+host merge from the approved draft is
WITHDRAWN by r1/GSC-R1-003: the doubled names (devin-config,
devin-usage, fake-return) are behaviorally DIFFERENT operations — the
adapter side consumes job records, the host side consumes mission
roots — and both sides have live callers, so a merge would rename for
cosmetics exactly where confusion costs most. They stay two families.

| target | absorbs | notes |
| --- | --- | --- |
| `job` | dispatch (23), capability (1), authority (1), schema (1) | The delegate-job domain. The reservation protocol stays TWO-PHASE (r1/GSC-R1-002: record-create must precede shell-owned setup so a second dispatcher cannot prepare the same id, and the cleanup trap depends on observable pending-setup); no `job reserve` merge. Coarsening candidates come only from the step-0 sequence census, under the rule that no observable intermediate state another process relies on may disappear. The reap verdict ladder becomes ONE decision owner with TWO consumers (r1/GSC-R1-004): an internal function the supervise component's reaper calls directly, and a `job reap-verdict` verb dispatch.sh consults; wind-down signaling stays in dispatch.sh. |
| `mission` | mission-state, -fence, -contract, -prompt, -runner, -turn, -ledger, evidence (8 families, 29 verbs) | Regrouping with an EXHAUSTIVE collision-resolving verb map as the step-2 deliverable (r1/GSC-R1-003): two init verbs and two verify verbs exist today, so the map prefixes by sub-domain (state-init, ledger-init, ledger-verify, fence-refuse, gc) and covers all 29, no illustrative subsets. |
| `adapter` | unchanged (34 minus census deletions) | Custody-boundary fragments per runtime; scripts keep launch/wait/signal custody. |
| `host` | unchanged (8) | Same boundary, mission-host side. |
| `proc` | identity (4), census (5), supervise (10) | Process identity, census, supervision — one domain: who is running, provably. Same exhaustive-map rule as mission. |
| `validate` | unchanged (10) | Whole-artifact validators. |
| `audit` | unchanged (2) | metasystem + coverage-ratchet. |
| `config` | unchanged (7) | |
| `lease` | unchanged (9) | Worktree session custody. |
| `receipt` | unchanged (5) | |
| `util` | util (5), json (3), event (1), gate (3), hooks (1), report (3) | Merged with an exhaustive map like mission (gate check and hooks check collide today, r1/GSC-R1-003 — they become gate-check and hooks-check). |

CLI compatibility during migration: the family router gains a
one-table alias layer (old family/verb → new) so scripts migrate per
commit, not big-bang. Alias deletion has a mechanical completion
condition (r1/GSC-R1-005): step 5 sweeps the tree for remaining
old-name invocations, updates them (bounded, this repository only —
adopted repositories call scripts by path, never verbs, so no
external contract names a verb), proves zero old-name callers by the
same census rules, and only then deletes the table. "Organically" is
retired.

## What gets deleted

1. Step 0 is a caller census with EXECUTABLE rules (r1/GSC-R1-007),
   already run once (first slice landed as c72f662, minus 344 lines):
   the corpus is every tracked file EXCLUDING cmd/metasystem (a verb's
   own registration is not a caller); an invocation is the literal
   "family verb" pair; a family invoked anywhere with a variable verb
   (family "$var") keeps all its verbs pending manual proof; a
   zero-caller candidate additionally needs a loose per-verb grep
   (wrapper functions like mission_fence hide the family word), an
   implementation trace (does any Go path call the backing function),
   and a docs check for human entry points. Tests never keep a verb
   alive; tests of deleted code are adapted to the surviving internal
   path or deleted with it. The deadcode analyzer runs after each
   deletion slice and its verdicts get the same manual verification.
   First-slice results: census classify and authentication-identity,
   validate code-critique-claim and waiver-facts, the mission-jobs
   family, mission-ledger verify and count, lease reclaim, and three
   orphaned functions.
2. The disposition registry is DELETED, not shrunk (r1/GSC-R1-006):
   nothing executable reads it — adoption's own allowlist in adopt.sh
   is the export contract and stays the single one. A ship-list that
   no code consumes would be decorative bookkeeping, and wiring one
   into adopt.sh would be new machinery this program exists to avoid.
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

1. Caller census (first slice landed: c72f662) + sequence census
   naming every ordering-carried invariant. Output: the deletion
   record, the coarsening list, and the alias table.
2. mission-* merge with its exhaustive verb map (mechanical; largest
   coherence gain per hour).
3. job-family formation: dispatch verbs regrouped; the reap verdict
   ladder becomes one decision owner with both consumers wired in the
   SAME commit — the supervise component's reaper calls the function,
   dispatch.sh consults the verb — so no second ladder survives
   (r1/GSC-R1-004). Reservation stays two-phase (r1/GSC-R1-002).
   Further coarsening only from the step-1 sequence census. dispatch.sh
   keeps launch, wind-down, and polling custody.
4. proc regrouping; scripts restored to ordinary scripts (usage texts
   return; shims that stay are one-liners by choice, not doctrine).
5. Registry deletion; old-name sweep to zero by the census rules;
   alias table deletion last.

Estimated two working sessions. Steps are independently valuable;
stopping after any of them leaves the system better than before it.

## Non-goals

No engine rewrites (missionrunner, lease, conformance, patience stay
as they are). No new fences or meta-machinery. No byte-identical
output conformance for renamed verbs — scripts calling them are
updated in the same commit; external contracts (file formats, exit
codes, hook entry points, adopted-repo script names) do not change.
