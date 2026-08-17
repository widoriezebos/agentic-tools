# The host-implementer wall (goal host-implementer-wall, D99/D100)

- Status: DRAFT r4 — r1–r3 folded (plans/hiw-critique-r1..r3.md).
  D100's rulings stand: NO self-work exception, DETECTOR tier. r4
  closes r3's four structural decisions: the delayed-authorization
  staleness rule (disjoint-path transport with exact apply), the
  atomic acceptance/consumption commit (one hash-chained write),
  tracked-runner-metadata repositories (refused at mission start),
  and legacy state (explicit refusal, no migration) — and rewrites
  the obligation matrix in the gate's canonical ten-column format
  with the missing rows added.
- Goal: host-implementer-wall (Current)
- In flight right now: the r4 design critique (codex xhigh); not a
  dispatch job record, so the open-work scanner cannot see it.
- Waiting on the human: nothing.
- Next step: none.

## What happened (unchanged evidence)

bm-2d rep 1: the Devin host built the whole solution solo — eleven
product files in one turn, zero dispatch attempts in 66 commands —
and the runner accepted a return whose `dispatched` list was empty
beside that diff. Only the kit's grading-time delegation floor
noticed. Wido ruled it a total failure of the metasystem (D99).

## The invariant

A MISSION HOST TURN NEVER SHIPS IMPLEMENTER WORK, and the runner
PROVES it at turn acceptance with a byte-level tree equation over
the SHIPPABLE tree (the isolated-index projection; ignored files
and git administrative state are outside it by construction):

    post-tree = pre-tree
              + the consumed integration authorizations' EXACT
                patches, in their durably recorded order
              + the exact declared host-artifact delta
                (mission-declared, file-precise — see below)

There is NO self-work term (D100) and NO machine-metadata term:
r2 proved the metadata term incoherent — in this repository
artifacts/ is ignored and git state is not in the shippable tree,
so its contribution is empty. A repository that TRACKS
machine-owned paths is REFUSED at mission start (r3 HIW-R3-03):
mission preflight validates that every runner/machine path is
untracked or ignored in the target and refuses with a named error
otherwise — no third equation category, no misclassification as
host artifacts; adoption already gitignores artifacts/ by
construction, so the precondition is the existing norm made
explicit. Anything the equation cannot account for is a
protocol violation: return refused, evidence persisted, workspace
tainted, MISSION parked — before and outranking any
completion-gate success.

## The integration authorization (HIW-R1-01, HIW-R2-01)

Issued ONLY by conformance validation, at the moment all
implementer-return, critic-chain, and merge checks pass, proving
atomically at issue time:

    apply(exact bound patch, exact bound base) = reviewed tree

with the patch digest and changed paths derived from those same
bytes — never from any party's claim. The record is durable and
content-addressed, and binds ALL of:

- job id AND the immutable job-record digest;
- mission id AND the mission incarnation (the signed
  contract/genesis digest — a mission identifier alone can be
  reused);
- the dispatch turn and the authorization-issuance turn;
- the unambiguous input/base tree and output/reviewed tree;
- the schema version and the canonical authorization digest;
- supersession status against earlier rounds of the same chain
  (only the final authorized round of a chain is eligible; a
  superseding authorization retires its predecessors).

CONSUMPTION is one-time, and the STALENESS RULE is explicit
(r3 HIW-R3-01). Define the expected-tree sequence: E0 is the
mission's anchored initial baseline; E(i+1) is turn i's accepted
post-tree (E(i) + turn i's consumed patches + its declared
host-artifact delta). An authorization A — apply(P, B) = R — is
consumable in turn j if and only if EITHER B == E(j), OR every
accepted change between B and E(j) is DISJOINT from A's changed
paths (changed-path intersection empty) AND P applies to E(j)
byte-exactly with no fuzz. Disjointness plus exact apply means the
consumed bytes on A's files are exactly the reviewed bytes;
anything else — overlap, contextual drift, apply failure — means
fresh conformance or park. Delayed certification of landed returns
stays legitimate under this predicate; a consumed or superseded
authorization is rejected always. THE COMMIT POINT IS ONE WRITE (r3
HIW-R3-02): turn acceptance appends a single hash-chained mission
state entry carrying, together, the wall verdict, the
pre/expected/post tree ids, the accepted turn-log reference, and
the consumed authorization digests. The consumption ledger is a
DERIVED INDEX over accepted entries, rebuilt on recovery — never a
separate first writer, so there is no ledger-first burn on a
tainted turn and no acceptance-first replay window: a crash before
the append means nothing was consumed and the turn is unaccepted
(recovery inspects and re-decides); after it, both facts are
durable together. The return's `certified` entries carry the
authorizationDigest explicitly (a schema field), ending job-ID
guessing. Adjudication verifies every returned certification
against the authorization record AND the dispatch job record —
role, stream, mission incarnation — never trusting the return's
own fields; only adjudicated facts enter the turn log. Dispatch gains the structured immutable
provenance this requires (mission, incarnation, turn, stream, role
in the job record's immutable set — today stream is unstructured
and mission/turn are not immutable).

## The tree composition contract (HIW-R2-02)

- The SNAPSHOT primitive is the conformance validator's isolated
  index (read-tree, add -A, write-tree), shared as ONE owner with
  the runner — it captures deletions, git file modes, symlink
  target blobs, binary content, and superproject gitlink ids. Its
  stated boundaries: arbitrary POSIX metadata, ignored untracked
  files, and dirty/untracked content INSIDE a submodule are
  outside the projection — stated and fixture-tested, with
  core.fileMode pinned so "mode" means git mode.
- The authoritative PATCH ORDER is durably recorded BEFORE
  integration begins; recovery replays the recorded order, never
  guesses, never trusts a later host claim.
- OVERLAP is changed-path intersection, and it REFUSES unless a
  new combined authorization is issued — the conservative rule;
  hunk-level cleverness is not attempted.
- APPLY is exact: no three-way merge, no rejects, no fuzz, no
  host-authored conflict resolution. A patch that does not apply
  cleanly to the expected tree means fresh conformance or mission
  park.
- The INITIAL BASELINE requires a clean shippable tree or a
  human-sealed initial pre-tree — pre-existing dirty product bytes
  are never silently grandfathered.
- Snapshot trees are kept REACHABLE: a runner-owned ref (or
  equivalent durable object anchor) protects pre/expected/post
  trees from git garbage collection until the mission closes.

## The host-artifact delta (HIW-R2-03)

Legitimate host-authored, shippable artifacts (a design note the
mission assigns to the host, dispositions, receipts) are DECLARED:
canonical repository-relative FILES — default deny, no directory
or glob grants, no symlink traversal — in the mission contract.
Protected paths (the signed contract, instruction ledgers) stay
denied even inside otherwise host-declared locations. The wall
verifies the host-artifact delta touches only declared files.

## Taint: a state machine, not an outcome (HIW-R2-04)

The hash-chained mission state carries a runner-owned `openTurn`
(with the anchored preTree) and a monotonic `workspaceTaint`. The
lease-holding missionrunner is the only ordinary writer; Wido's
resolution is a separate human-reserved operation. Honestly per
the detector tier: this channel is runner-owned and TAMPER-EVIDENT
under the cooperative posture, not unforgeable (the current public
mission state-write accepts any proposed state with the current
hash — tightening it to runner custody is part of this build;
unforgeability would need the rejected isolation tier).

The binding order:

1. Snapshot and durably anchor `openTurn.preTree` BEFORE the host
   launches.
2. After EVERY host exit — normal, capped, nonzero, malformed
   return, or crash recovery — inspect the tree BEFORE any
   measurement or conclusion.
3. On mismatch: persist evidence, set taint, park the mission —
   before any gate-success path (today ConcludeFaultedTurn
   completes a faulted mission when the gate passes, and recovery
   heals a reserved cycle from HEAD without inspecting the
   unfinished workspace — both change).
4. On resume: verify state and anchor; if tainted, STOP. Otherwise
   inspect any unfinished open turn before reserved-cycle healing
   or a new baseline.

Typed resolution, two variants only:

- `RESTORE`: names a recorded safe tree; the runner verifies exact
  equality before clearing.
- `ADOPT_DISPUTED_TREE`: binds the taint identifier and the
  observed tree, records Wido's identity, reason, and the exact
  attribution claims being waived. Adoption clears the operational
  taint but never fabricates authorization, never erases the
  violation record, and never earns delegation-floor credit.

A generic free-text answer NEVER clears taint.

## The prompt and doctrine change (HIW-R2-05)

No legitimate mission product activity depends on the prose
allowance. It is removed from BOTH live authorities —
scripts/agents/templates/host-turn-instruction.md and
scripts/agents/roles/orchestrator.md (both assembled into every
mission prompt; the role bytes are validated exactly) — and the
canonical docs/orchestration.md allowance and generic
"When Not to Delegate" language are narrowed. The interim rule,
verbatim:

> Inside a runner-created mission, the host never authors product
> bytes, regardless of size or urgency. A mechanically small
> change may omit a separate design artifact only when the
> existing contract permits it; implementation still requires an
> implementer job, critic closure, conformance-issued integration
> authorization, and exact authorized-patch integration. Until
> small-change-lane ships, use that ordinary path. A fence refusal
> parks through the runner; it never authorizes host
> implementation. Interactive work outside missionrunner is
> unaffected.

The orchestrator role's broad "Repository work … is yours" opening
narrows to the enumerated host duties (design/briefs,
adjudication, decisive verification, exact integration, receipts,
certification). All four bm-2 manifests get the discipline
sentence in `completionGate.command` — the only field provisioning
copies into the signed contract; anywhere else is dead text.

## The hardened delegation floor (HIW-R2-06)

A qualifying stream requires at least one validated
implementer-role job with a NONEMPTY conformant patch whose
UNSUPERSEDED integration authorization was ACTUALLY CONSUMED into
the accepted post-tree. Rejected, empty, replayed, superseded,
unapplied, and human-adopted evidence never counts. The extractor
consumes the runner-adjudicated records; the benchmark's mirrored
evidence schema updates in the same change.

## The boundary (settled, r1/r2)

The wall binds exactly at the missionrunner's transition after a
runner-created mission host exits and before the turn is accepted.
Interactive development never traverses missionrunner acceptance
and keeps KI-27's direct-implementation model; a boundary test
proves an interactive direct commit is unaffected.

## What the wall is NOT (the tier line, ruled)

Detector tier (D100 ruling 2): every accidental and naive shape is
caught, including D99 exactly. A host that can mutate delegate
worktrees can still forge apparent authorship — recorded in
r1's laundering table, closed only by the unbuilt isolation tier.

## Named contracts (r3 HIW-R2-07 closed — nothing left to invent)

- INTEGRATION AUTHORIZATION record:
  `artifacts/agents/missions/<mission>/authorizations/<authorizationDigest>.json`,
  schemaVersion 1, written by conformance validation via the
  durable two-outcome writer, mirrored by the dispatch mirror at
  chain close, retained until mission close + the evidence GC's
  verified-archive rule.
- CONSUMPTION: no standalone file — the consumed digests ride the
  accepted turn entry in the hash-chained mission state
  (state.json chain); the ledger is an in-memory index rebuilt
  from the chain on every resume.
- RETURN SCHEMA: `certified[]` gains `authorizationDigest`
  (required); the benchmark's mirrored evidence schema updates in
  the same commit.
- WALL EVIDENCE per turn, in the turn directory:
  `wall.json` (preTree, expectedTree, postTree, orderedDigests,
  verdict, unaccounted paths on violation) beside the existing
  turn records.
- TREE ANCHORS: `refs/metasystem/missions/<mission>/<treeId>`,
  created at snapshot, deleted at mission close by the runner.
- TYPED RESOLUTION: a human-reserved verb, `mission resolve-taint
  --mission M (--restore <treeId> | --adopt --taint <id> --reason
  R)`, classifying HUMAN-only through the same authority matrix as
  every reserved operation.
- EVENTS (observability only; records are the authority):
  authorization-issued, authorization-consumed,
  authorization-refused, wall-passed, wall-violated,
  recovery-inspected, taint-set, taint-resolved — registered in
  the event registry with their payload fields.
- LEGACY STATE (r3 HIW-R3-04, decided): a mission state predating
  the wall schema REFUSES to resume, with the named error
  `mission resume refused: state predates the host-implementer
  wall; re-provision the mission`. No migration machinery —
  missions are short-lived cohort artifacts; a deterministic
  refusal beats a migration path nobody exercises.

## Design obligations

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| HIW-O1 | CRITICAL | hiw-critique-r2 §3 | Pre-tree/open-turn state is anchored reachable before host launch and inspected before any recovery healing or new baseline | missionrunner loop + mission state chain | loop.go snapshot/anchor path | crash fixture at every snapshot boundary + object-GC fixture | mission fixture run with forced kill between snapshot and launch | MISSING | implement |
| HIW-O2 | CRITICAL | hiw-critique-r1 §1 | Authorization issues only on the exact patch/base/reviewed-tree join with critic closure | validate conformance | conformance.go issuance join | mutated/stale patch, wrong base, empty patch, critic-mismatch fixtures | conformance run over a real implementer chain | MISSING | implement |
| HIW-O3 | CRITICAL | hiw-critique-r3 §1-2 | Provenance verified (mission incarnation, stream, role, job digest); consumption is one-time via the acceptance write; the staleness predicate admits disjoint delayed returns and refuses the rest | missionrunner adjudication | adjudicate.go verification + acceptance append | cross-mission, replay, superseded, later-turn-disjoint, later-turn-overlap fixtures | mission fixture with delayed certification across turns | MISSING | implement |
| HIW-O4 | CRITICAL | hiw-critique-r2 §2 | Exact ordered patch composition over the shared isolated-index primitive equals the accepted tree | shared git-tree primitive (one owner) | new internal/gittree package used by validator + runner | deletion, mode, symlink, binary, gitlink, order, overlap, non-application fixtures | conformance + wall over one real chain | MISSING | implement |
| HIW-O5 | CRITICAL | hiw-critique-r1 §5 | Every host-exit path checks the wall; violation taints and parks before measurement or completion, outranking gate success | missionrunner cycle/lifecycle | cycle.go fault path rework | green-gate-after-mutation fixture must park | scripted-host mission fixture writing product bytes | MISSING | implement |
| HIW-O6 | CRITICAL | hiw-critique-r2 §4 | No new baseline while tainted; only typed RESTORE (exact equality) or ADOPT_DISPUTED_TREE clears; adoption earns no floor credit | mission resolve-taint verb + state chain | mission.go reserved verb | generic-answer refusal, restore mismatch, exact adoption, crash-recovery fixtures | tainted mission resolved both ways in a fixture | MISSING | implement |
| HIW-O7 | CRITICAL | hiw-critique-r2 §3 | The tree partition is equation-complete and default-deny; protected paths refuse even inside declared locations; machine-path tracking refused at mission start | mission contract parser + wall + preflight | contract.go declaration grammar | protected-path, path-escape, symlink-ancestry, glob-grant, tracked-metadata-refusal fixtures | preflight refusal on a tracking repo fixture | MISSING | implement |
| HIW-O8 | HIGH | hiw-critique-r2 §5 | The interim rule text lands VERBATIM in both live authorities (host-turn-instruction.md, orchestrator.md), the doctrine narrows, and all four bm-2 completionGate.command fields carry the discipline sentence; interactive direct work is unaffected | prompt assembler + role/template + manifests | template/role bytes + manifest diffs | assembled-prompt byte test asserting the verbatim rule; interactive boundary test | a claude-host mission fixture turn showing the new prompt | MISSING | implement |
| HIW-O9 | HIGH | hiw-critique-r3 fold of r2 §6 | The delegation floor counts only a validated implementer job with a nonempty patch whose unsuperseded authorization was consumed into the accepted post-tree | benchmark extractor + evidence schema | extractor.py floor rule | empty, sham, replayed, unapplied, adopted-tree fixtures | re-extraction of the D99 cohort must stay invalid | MISSING | implement |
| HIW-O10 | HIGH | hiw-critique-r2 §7 | Authorization, wall, recovery, and taint evidence is durable, mirrored, and observable via registered events | dispatch mirror/close + event registry | mirror + events registration | restart/readback, missing-record close-failure, event-payload fixtures | mission fixture restart with evidence intact | MISSING | implement |
| HIW-O11 | HIGH | hiw-critique-r3 §4 | Pre-wall mission state refuses resume with the named error; no migration path exists | mission state loader | loader version check | old-schema resume fixture asserting the exact error | resume attempt on a preserved pre-wall state | MISSING | implement |
| HIW-O12 | HIGH | hiw-critique-r3 matrix gaps | core.fileMode is pinned in wall-touched repositories and the initial baseline is clean or human-sealed; violations refuse before the first turn | mission preflight | preflight checks | fileMode-off fixture; dirty-baseline refusal fixture; sealed-baseline acceptance fixture | preflight on a deliberately dirty target | MISSING | implement |
| HIW-O13 | CRITICAL | hiw-critique-r3 §2 | The acceptance write is the single commit point joining wall verdict, trees, turn log, and consumed digests; crash on either side leaves a consistent state | missionrunner acceptance append | single-append implementation | crash-before and crash-after fixtures proving no burn and no replay | forced-kill mission fixture across the append | MISSING | implement |

## Loop discipline

Codex xhigh. The r3 critique verifies the r2 fold (all eight
findings) and attacks any remaining contract an implementer would
have to invent. No code before convergence or the ratified
mechanical-grain exit.
