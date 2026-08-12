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

## What "script-shaped" means, and the round-4 severance

Script-shaped means: a bash file owns a CALL SEQUENCE whose ordering
carries a correctness invariant that no Go function states. Rounds 1-4
tried to build first a census methodology and then a unified reap
verdict owner on top of that definition, and every round demanded more
machinery: authority classes, must-defer action lists, a complete
decision table, a durable deferred-action handoff, a two-phase
post-action feedback contract, revision-aware compare-and-swap
(r1/GSC-R1-004, r2/GSC-R1-008/009, r3/GSC-R1-017, r4/GSC-R1-019
through 022). That escalation is the generating-cause signal: the
three reap ladders differ because their CONTEXTS differ — lease-epoch
visibility, kill authority, post-action facts like the group-death
timestamp, side-effect ownership — and unifying them does not remove
that complexity, it relocates it into a contract. Under the human's
core-versus-plumbing ruling this program therefore SEVERS the
unification (r4): the three reap paths stay where they are, each
already consulting Go decision fragments; what remains from the
analysis is one real DEFECT and two DOCUMENTED invariants.

- KNOWN DEFECTS, recorded and handed to the human rather than fixed
  here (r5 severance): round 2 found the supervise reaper applies
  verdicts by whole-record overwrite without the record lock, so a
  completion landing after its read can be clobbered (GSC-R1-009);
  round 5 found the deeper sibling — the same no-kill reaper stamps
  timeout AND a group-death timestamp on a LIVE over-budget process
  it neither stopped nor proved dead, and its unit test asserts
  exactly that (GSC-R1-027). Rounds 4-5 proved a repair is not small:
  routing it through the record CAS owner changes endedAt semantics
  downstream (GSC-R1-031), crosses an import boundary
  (GSC-R1-030), and the real question — does the standing reaper get
  kill authority, or must it stop stamping death it cannot prove — is
  a supervision-lifecycle design decision that belonged to the human.
  HUMAN RULING (Wido, 2026-08-12): the standing reaper never gets kill
  authority — it stops stamping death it cannot prove. It may
  terminalize only provably-dead custodians, through the locked
  compare-and-swap owner with an expected status; a live over-budget
  job keeps its running status until the kill-capable dispatch path
  winds it down. The fix is its own program outside this one (one
  reviewable intent), with regression tests for the
  completion-after-read race, the live-over-budget-not-terminalized
  rule, and the dead-over-budget timeout. This consolidation program
  itself still makes ZERO behavior changes.
- The invariants (documented in place, not coarsened): the
  reservation pair's two-phase ordering (record-create before
  shell-owned setup; observable pending-setup feeds the cleanup trap)
  and the reap ladder's budget-before-liveness priority. Each gets a
  comment at its call site naming the invariant, nothing more. The
  formal sequence-census methodology is WITHDRAWN with the
  unification it existed to feed (r4/GSC-R1-023: it could not be made
  executable over helper indirection without growing into a
  call-graph analyzer — more machinery about the system instead of
  the system).
- The pre-commit guard's authority ordering stays in the guard with
  its invariant documented (r4/GSC-R1-024 withdrew the step-4
  coarsening: three lines of shell carrying a documented rule beat an
  eleventh validate verb).

Consequence, stated plainly so the program cannot fail its own
definition (r5/GSC-R1-028): after the severances, the surviving
shell-owned orderings are ACCEPTED plumbing under the human's ruling —
custody interleaves them, so shell is where they belong. This
program's goals are deletion and coherence, and it claims nothing
else. "Unimporting the shell's architecture" means the CLI stops
mirroring historical shell decomposition; it does not mean shell owns
no ordering.

## Target tree (human-approved 2026-08-12; amended by rounds 1-2)

Nineteen families (r7/GSC-R1-043): three real merges (job, mission,
proc), the engine
families unchanged, and the small domains left alone — rounds 1 and 2
withdrew every merge whose only product was renaming. The adapter+host
merge is WITHDRAWN by r1/GSC-R1-003: the doubled names (devin-config,
devin-usage, fake-return) are behaviorally DIFFERENT operations — the
adapter side consumes job records, the host side consumes mission
roots — and both sides have live callers, so a merge would rename for
cosmetics exactly where confusion costs most. They stay two families.
The surface shrinks by deletion; the regroupings are coherence work
(the reap ownership inversion did not survive round 4).

| target | absorbs | notes |
| --- | --- | --- |
| `job` | dispatch (24), capability (1), authority (1) | The delegate-job domain, regrouped per the appendix — a purely mechanical rename step (r5/GSC-R1-033: one reviewable intent per commit; no behavior change rides along). The reservation protocol stays TWO-PHASE (r1/GSC-R1-002). |
| `mission` | mission-state, -fence, -contract, -prompt, -runner, -turn, -ledger (7 families, 28 verbs) | Regrouping with the EXHAUSTIVE collision-resolving verb map in the appendix (r1/GSC-R1-003, r2/GSC-R1-011) — all 28, no illustrative subsets. evidence stays its own family (r2/GSC-R1-012: the collector is repository-wide custody that merely protects mission state; `mission gc` would misname its scope). |
| `adapter` | unchanged (34) | Custody-boundary fragments per runtime; scripts keep launch/wait/signal custody. The census found no dead adapter verbs (r6/GSC-R1-036). |
| `host` | unchanged (8) | Same boundary, mission-host side. |
| `proc` | identity (4), census (4) | Process identity and census are one domain — who is running, provably. supervise STAYS its own family (r2/GSC-R1-013's logic applied honestly: folding it in would just prefix every verb with its old family name, a cosmetic merge). Map in the appendix. |
| `supervise` | unchanged (10) + census fingerprint | Supervision lifecycle. census fingerprint hashes supervision code, signatures, and configuration to detect stale supervision — it is a supervision-staleness detector, not a process probe (r4/GSC-R1-026), so it becomes `supervise fingerprint`. |
| `validate` | unchanged (10) | Whole-artifact validators. |
| `audit` | unchanged (2) | metasystem + coverage-ratchet. |
| `config` | unchanged (7) | |
| `lease` | unchanged (9) | Worktree session custody. |
| `receipt` | unchanged (5) | |
| `evidence` | unchanged (1) | Repository-wide durable-evidence custody (r2/GSC-R1-012). |
| `schema` | unchanged (1) | Role-return schema materialization: takes a role and version, never a job — moving it under job would misname its boundary exactly as mission gc would have (r6/GSC-R1-037). |
| small domains | gate (3), report (3), hooks (1), event (1), json (3), util (5) | The util merge is WITHDRAWN (r2/GSC-R1-013): these are distinct responsibilities with distinct failure contracts, and the repository's own design standard names a catch-all util as the anti-pattern. They stay as they are; the earlier gate-check/hooks-check collision worry evaporates with the merge. |

Compatibility (r6/GSC-R1-034: the alias layer is WITHDRAWN — more
machinery guarding a boundary that does not exist). In this
repository every caller moves in the same commit as its rename, so
nothing here ever needs an alias. For adopted repositories the
contract is stated instead of shimmed: binary verb names are INTERNAL
and may change between versions — the shipped scripts are the stable
interface — and the upgrade documentation carries the old-to-new
table from the appendix for any project that called verbs directly
anyway. This also dissolves the alias activation-ordering questions
(r6/GSC-R1-035): census fingerprint simply becomes supervise
fingerprint in step 4's commit, which updates arm-supervision.sh and
the suite in the same change.

## What gets deleted

1. Step 0 is a caller census with EXECUTABLE rules (r1/GSC-R1-007)
   and a FINITE scope (r6/GSC-R1-036): the census ran once and its
   slice landed (c72f662, minus 344 lines); the program's final
   surface is exactly the currently registered set minus that slice,
   renamed per the appendix. Further deletions happen only if a
   regroup step or the analyzer pass surfaces a new zero-caller verb,
   each recorded in that step's commit message under the same rules —
   no open-ended hunting. The analyzer is exactly
   `go run golang.org/x/tools/cmd/deadcode@latest ./cmd/metasystem`
   (r7/GSC-R1-042), and its verdicts apply ONLY to unregistered Go
   functions: a router-registered handler is always reachable to it,
   so verb-deletion authority comes from the caller census alone. The rules:
   the corpus is every tracked file EXCLUDING cmd/metasystem's router
   registration table (a verb's own registration is not a caller); an
   invocation is the literal "family verb" pair OR, in Go files, the
   family and verb appearing as separate adjacent string arguments to
   a command execution (r7/GSC-R1-039: the mission engine launches
   `mission-runner run-loop` and `mission-state anchor` through
   exec.Command with split arguments, invisible to the pair grep —
   every rename commit greps BOTH shapes); a family invoked anywhere
   with a variable verb (family "$var") keeps all its verbs pending
   manual proof; a
   zero-caller candidate additionally needs a loose per-verb grep
   (wrapper functions like mission_fence hide the family word), an
   implementation trace (does any Go path call the backing function),
   and a docs check for human entry points. Go unit tests never keep a verb
   alive, but FIXTURE SEQUENCERS DO (r5/GSC-R1-032): the *-fixtures.sh
   drivers and validate-metasystem.sh are the production validation
   suite, so census run and signature-check — called only from
   fixtures — are live and map into proc like everything else. Tests
   of genuinely deleted code are adapted to the surviving internal
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
3. WITHDRAWN in round 4 (GSC-R1-025): the wrapper scripts keep
   forwarding to the Go verbs that parse their historical arguments.
   Restoring parsing to the wrappers would create a new
   interface-boundary specification problem for purely cosmetic gain;
   the current split works and is tested.
4. RETIRED: the Phase F fixture-python elimination. Fixture sequencers
   are plumbing; their python3 heredocs are fine. (Kill-shell's
   "python dies at Phase F" goal is void; production python is already
   zero and stays zero by review, not by fence.)

## Migration order

Each step lands with the suite green from a pristine worktree and all
call sites updated in the same commit.

1. Already landed: the census slice (c72f662). The upgrade-notes
   table in docs/metasystem-reconciliation.md grows WITH the renames
   (r7/GSC-R1-041): each of steps 2-4 appends its own rows in the
   same commit, so the published table never names a command that
   does not exist yet.
2. mission-* merge per the appendix map, all in-repo callers updated
   in the same commit (mechanical; largest coherence gain per hour).
3. job-family regrouping per the appendix — mechanical rename only
   (r5/GSC-R1-033). No reap unification (severed, round 4); the
   recorded defect pair stays with the human (r5/GSC-R1-027).
4. proc regrouping and supervise fingerprint per the appendix; the
   two kept invariants get their call-site documentation.
5. Registry deletion; a final sweep proving zero old-name callers
   remain in-repo — INCLUDING cmd/metasystem itself outside the
   registration table (r7/GSC-R1-040): flag-set labels, usage
   strings, and error prefixes rename with their verbs, so the
   surface never tells a user to invoke a command that no longer
   exists. Each rename step already carries its own share of these;
   step 5 is the proof pass.

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
name. The upgrade-notes rows are generated from these tables and
nothing else (the alias layer did not survive round 6).

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

### proc (8 verbs from 2 families; fingerprint went to supervise, r4/GSC-R1-026)

| today | target |
| --- | --- |
| identity started-at | proc started-at |
| identity probe | proc probe |
| identity exists | proc exists |
| identity group-exists | proc group-exists |
| census fingerprint | supervise fingerprint |
| census run | proc census |
| census alive | proc alive |
| census signature-check | proc signature-check |
| census find-ancestor | proc find-ancestor |

`census run` becomes `proc census` — "run" alone said nothing.

### job (26 rows from 3 families: 24 dispatch verbs plus capability and authority; schema stays its own family, r6/GSC-R1-037)

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
| dispatch reap-facts | job reap-facts |
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

No new verbs anywhere in this map: every row is a rename
(r5/GSC-R1-029; the reap-verdict remnant rows died with the round-4
severance).
