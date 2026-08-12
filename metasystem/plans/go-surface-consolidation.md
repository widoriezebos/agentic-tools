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
caller census: ANY fixed ordering of two or more verb invocations
(r3/GSC-R1-014 — the reservation pair is itself a two-verb invariant
sequence, and three was an arbitrary floor), same-family or
cross-family (r2/GSC-R1-010), INCLUDING orderings hidden behind shell
re-entry — a script invoking itself with internal __verbs is part of
the corpus, and its router branches count as the call sites. The
pre-commit guard's lease classify → wrapper-token ordering (human
commits sovereign, identified agents need wrapper proof) is the
cross-family type specimen.

Each census hit records its named invariant and a decision made by
RULE, not taste (r3/GSC-R1-015): COARSEN into one Go verb when the
ordering is purely a decision ordering — no process launch, signal,
wait, or file custody interleaves the calls, and no observable
intermediate state another process consumes would disappear.
KEEP-AND-DOCUMENT when custody interleaves or intermediate state is
load-bearing. Worked through today's three known hits: the
reservation pair KEEPS (observable pending-setup, custody between the
phases); the reap ladder COARSENS (pure decisions between lock and
CAS — the wind-down action is delegated by contract, below); the
pre-commit guard's authority pair COARSENS (decision-only) into one
validate verb in step 4. Sequences with no invariant are legitimate
plumbing and stay.
Regrouping (mission, proc) is coherence work and is claimed as such,
not as de-shell-ification.

## Target tree (human-approved 2026-08-12; amended by rounds 1-2)

Eighteen families: three real merges (job, mission, proc), the engine
families unchanged, and the small domains left alone — rounds 1 and 2
withdrew every merge whose only product was renaming. The adapter+host
merge is WITHDRAWN by r1/GSC-R1-003: the doubled names (devin-config,
devin-usage, fake-return) are behaviorally DIFFERENT operations — the
adapter side consumes job records, the host side consumes mission
roots — and both sides have live callers, so a merge would rename for
cosmetics exactly where confusion costs most. They stay two families.
The surface shrinks by deletion (census slices) and by ownership
inversion (the reap verdict), not by relabeling.

| target | absorbs | notes |
| --- | --- | --- |
| `job` | dispatch (24), capability (1), authority (1), schema (1) | The delegate-job domain. The reservation protocol stays TWO-PHASE (r1/GSC-R1-002: record-create must precede shell-owned setup so a second dispatcher cannot prepare the same id, and the cleanup trap depends on observable pending-setup); no `job reserve` merge. Coarsening candidates come only from the step-0 sequence census, under the rule that no observable intermediate state another process relies on may disappear. The reap verdict ladder becomes ONE decision owner with THREE consumers (r1/GSC-R1-004, r2/GSC-R1-008): an internal function called by the supervise component's reaper AND by the mission runner's reap path (whose reapReservedRecords/applyReapVerdict mapping is today a third independent ladder), and a `job reap-verdict` verb dispatch.sh consults. The consumers legitimately DIFFER in action authority (r3/GSC-R1-017: the supervise reaper times out over-budget records without kill authority, the mission runner acts only on a proven-dead custodian, dispatch winds down live groups and performs fence/usage/mirror/event side effects), so the owner's contract is: inputs are the record, the reap facts, and the caller's declared authority class (kill-capable, no-kill, record-only); output is the verdict PLUS the required-action list — wind-down-first, CAS target and patch, fence refusal, usage aggregation, mirror, events — split into actions this caller MUST perform and actions it must DEFER to a kill-capable consumer. The status-and-facts to verdict mapping (including budget-judged-before-liveness) is stated exactly once; no consumer can silently weaken enforcement because deferred actions are named in its output, and the fixtures assert each consumer class end-to-end. Verdict APPLICATION centralizes with the decision (r2/GSC-R1-009): every consumer lands its verdict through the locked compare-and-swap record owner with an expected status — the supervise reaper's current whole-record overwrite, which can clobber a completion that lands after its read, is a defect this step fixes with a regression test, recorded as the program's one deliberate behavior change. |
| `mission` | mission-state, -fence, -contract, -prompt, -runner, -turn, -ledger (7 families, 28 verbs) | Regrouping with the EXHAUSTIVE collision-resolving verb map in the appendix (r1/GSC-R1-003, r2/GSC-R1-011) — all 28, no illustrative subsets. evidence stays its own family (r2/GSC-R1-012: the collector is repository-wide custody that merely protects mission state; `mission gc` would misname its scope). |
| `adapter` | unchanged (34 minus census deletions) | Custody-boundary fragments per runtime; scripts keep launch/wait/signal custody. |
| `host` | unchanged (8) | Same boundary, mission-host side. |
| `proc` | identity (4), census (5) | Process identity and census are one domain — who is running, provably. supervise STAYS its own family (r2/GSC-R1-013's logic applied honestly: folding it in would just prefix every verb with its old family name, a cosmetic merge). Map in the appendix. |
| `supervise` | unchanged (10) | Supervision lifecycle. |
| `validate` | unchanged (10) | Whole-artifact validators. |
| `audit` | unchanged (2) | metasystem + coverage-ratchet. |
| `config` | unchanged (7) | |
| `lease` | unchanged (9) | Worktree session custody. |
| `receipt` | unchanged (5) | |
| `evidence` | unchanged (1) | Repository-wide durable-evidence custody (r2/GSC-R1-012). |
| small domains | gate (3), report (3), hooks (1), event (1), json (3), util (5) | The util merge is WITHDRAWN (r2/GSC-R1-013): these are distinct responsibilities with distinct failure contracts, and the repository's own design standard names a catch-all util as the anti-pattern. They stay as they are; the earlier gate-check/hooks-check collision worry evaporates with the merge. |

CLI compatibility during migration: the family router gains a
one-table alias layer (old family/verb → new) so scripts migrate per
commit, not big-bang. Aliases activate WITH their targets
(r3/GSC-R1-016): step 1 lands the mechanism plus the full table
generated from the appendix, with every entry inert until its target
family registers — an alias whose target is unregistered is a router
error if exercised, and a step-1 test asserts the table matches the
appendix row-for-row. Step 2 activates the mission entries in the
commit that registers the mission family, step 3 the job entries,
step 4 the proc entries. reap-facts aliases to job reap-facts when
the job family lands and retires only when reap-verdict replaces its
callers within step 3. Alias deletion has a mechanical completion
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

1. Caller census (first slice landed: c72f662) + sequence census over
   both shapes naming every ordering-carried invariant. Output: the
   deletion record, the coarsening-candidate decisions, and the alias
   table BUILT FROM THE APPENDIX MAPS (r2/GSC-R1-011: the complete
   old-name to new-name mapping is this document's appendix, not a
   later step's homework — step 1 needs it and sign-off covers it).
2. mission-* merge per the appendix map (mechanical; largest
   coherence gain per hour).
3. job-family formation: dispatch verbs regrouped per the appendix;
   the reap verdict ladder becomes one decision owner with ALL THREE
   consumers wired in the SAME commit — the supervise component's
   reaper and the mission runner's reap path call the function,
   dispatch.sh consults the verb — so no other ladder survives
   (r1/GSC-R1-004, r2/GSC-R1-008), and every consumer applies its
   verdict through the locked compare-and-swap owner with expected
   status (r2/GSC-R1-009; the supervise reaper's stale-overwrite
   defect dies here, with a regression test for a completion landing
   after the reaper's read). Reservation stays two-phase
   (r1/GSC-R1-002). Further coarsening only from the step-1 census
   decisions. dispatch.sh keeps launch, wind-down, and polling
   custody.
4. proc regrouping per the appendix; scripts restored to ordinary
   scripts (usage texts return; shims that stay are one-liners by
   choice, not doctrine).
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

## Appendix: the exhaustive verb maps (r2/GSC-R1-011)

Every renamed verb, old to new. Anything not listed here keeps its
name. The alias table in step 1 is generated from these tables and
nothing else.

### mission (28 verbs from 7 families)

| today | target |
| --- | --- |
| mission-state init | mission state-init |
| mission-state write | mission state-write |
| mission-state verify | mission state-verify |
| mission-state anchor | mission state-anchor |
| mission-state reconcile | mission state-reconcile |
| mission-fence check-job | mission fence-check-job |
| mission-fence reserve-job | mission fence-reserve-job |
| mission-fence reserve-cycle | mission fence-reserve-cycle |
| mission-fence authorize-cap | mission fence-authorize-cap |
| mission-fence aggregate-usage | mission fence-aggregate-usage |
| mission-fence refuse | mission fence-refuse |
| mission-contract validate | mission contract-validate |
| mission-contract seal | mission contract-seal |
| mission-contract preflight | mission contract-preflight |
| mission-contract measure | mission contract-measure |
| mission-contract envelope-allows | mission contract-envelope-allows |
| mission-prompt assemble | mission prompt-assemble |
| mission-runner start | mission start |
| mission-runner resume | mission resume |
| mission-runner status | mission status |
| mission-runner answer | mission answer |
| mission-runner run-loop | mission run-loop |
| mission-turn adjudicate | mission turn-adjudicate |
| mission-turn conclude | mission turn-conclude |
| mission-turn record-failure | mission turn-record-failure |
| mission-turn park | mission turn-park |
| mission-ledger init | mission ledger-init |
| mission-ledger append | mission ledger-append |

The runner verbs take the bare names: they are the family's front
door for humans. state-init and ledger-init are the collision the
prefixes resolve.

### proc (9 verbs from 2 families)

| today | target |
| --- | --- |
| identity started-at | proc started-at |
| identity probe | proc probe |
| identity exists | proc exists |
| identity group-exists | proc group-exists |
| census fingerprint | proc fingerprint |
| census run | proc census |
| census alive | proc alive |
| census signature-check | proc signature-check |
| census find-ancestor | proc find-ancestor |

`census run` becomes `proc census` — "run" alone said nothing.

### job (27 rows from 4 families: 24 dispatch verbs plus three single-verb families; count corrected by r3/GSC-R1-018)

| today | target |
| --- | --- |
| dispatch record-create | job record-create |
| dispatch record-setup | job record-setup |
| dispatch record-cas | job record-cas |
| dispatch record-protocol-error | job record-protocol-error |
| dispatch build-setup | job build-setup |
| dispatch build-record | job build-record |
| dispatch build-follow-record | job build-follow-record |
| dispatch latest-chain-record | job latest-chain-record |
| dispatch chain-members | job chain-members |
| dispatch chain-usage | job chain-usage |
| dispatch custody-add | job custody-add |
| dispatch handshake-eval | job handshake-eval |
| dispatch reap-facts | job reap-facts (retires into job reap-verdict within step 3) |
| dispatch census-fresh | job census-fresh |
| dispatch watcher-ceiling | job watcher-ceiling |
| dispatch expand-permissions | job expand-permissions |
| dispatch validate-mission | job validate-mission |
| dispatch mirror | job mirror |
| dispatch close-check | job close-check |
| dispatch critique-exhaustion | job critique-exhaustion |
| dispatch exhaustion-patches | job exhaustion-patches |
| dispatch cap-resolution | job cap-resolution |
| dispatch brief-mode | job brief-mode |
| dispatch owner-lock | job owner-lock |
| capability select | job snapshot-select |
| authority check | job authority-check |
| schema materialize | job schema-materialize |

`job reap-verdict` is new (step 3); reap-facts keeps an executable
alias until reap-verdict replaces its callers inside step 3.
