# The host-implementer wall (goal host-implementer-wall, D99/D100)

- Status: IMPLEMENTATION-FIRST (D81 pivot, D101) — five critique
  rounds folded (plans/hiw-critique-r1..r5.md; trajectory 9, 8,
  4+matrix, 5, 1). r5 left exactly one structural finding
  (occurrence identity, folded above as the sequence-point
  binding) and the critic itself invoked the ratified pivot:
  budget one spent, this document is the SPEC, the 13 obligation
  rows and their fixture lists are the arbiter, and the remaining
  mechanical-grain shape choices (exact JSON schemas, event
  registry types) are owned by golden fixtures during the build.
  D100's rulings stand: NO self-work exception, DETECTOR tier.
- Goal: host-implementer-wall (Current)
- In flight right now: implementation, starting with HIW-O4's
  foundation (the shared git-tree primitive).
- Waiting on the human: nothing.
- Next step: implement per the obligation matrix, mandatory code
  critique per slice, both-host gates before any ship.

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
so its contribution is empty. Machine-owned paths must be IGNORED,
not merely untracked (r3 HIW-R3-03, sharpened by r4 HIW-R4-03: the
shared snapshot uses `git add -A`, which sweeps untracked
non-ignored files INTO the shippable projection — only gitignore
coverage keeps machine output out of the equation). Mission
preflight validates, READ-ONLY and BEFORE arming supervision or
any other target write (the current launch order arms first and
changes): the exact machine-output inventory from the named
contracts below has zero tracked entries AND ignore coverage for
every creatable descendant; a tracked-and-ignored entry is still
tracked and refuses. Refusal provably leaves the target tree
unchanged. Adoption already gitignores artifacts/ by construction,
so the precondition is the existing norm made explicit. Anything the equation cannot account for is a
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
consumable in turn j if and only if B IS A NAMED E-SEQUENCE POINT:
the authorization binds, beside the base tree id, the SEQUENCE-
POINT IDENTITY of its base — the state-chain sequence number plus
segment ordinal at which that expected tree was accepted (r5
HIW-R5-01: tree ids legitimately repeat — a no-change turn yields
E(i+1)==E(i), and RESTORE deliberately returns to an older tree —
so a bare tree id can match several occurrences, and which
occurrence decides the intervening-change set). Conformance reads
the current sequence point at issuance; consumption verifies the
named point exists and is prior, AND either k == j, or every accepted change
between E(k) and E(j) is DISJOINT from A's changed paths
(changed-path intersection empty, rename endpoints counting on
both sides) AND P applies to E(j) byte-exactly with no fuzz, AND
after application every entry A changed carries the SAME object id
and git mode as in R. That last equality is what makes "exactly
the reviewed bytes" checkable rather than assumed — a hunk that
exact-applies while the file elsewhere drifted fails the object-id
comparison and refuses. Anything else — unknown base, overlap,
drift, apply failure — means fresh conformance or park. Delayed certification of landed returns
stays legitimate under this predicate; a consumed or superseded
authorization is rejected always. THE COMMIT POINT IS ONE WRITE (r3
HIW-R3-02; representation fixed by r4 HIW-R4-04): turn acceptance
appends a single hash-chained mission state entry whose PAYLOAD
carries, together, the wall verdict, the pre/expected/post tree
ids, the accepted turn-log reference, and the consumed
authorization digests — payload-bearing acceptance entries, not
hash-only history (the current chain's history rows hold only
hashes and cannot rebuild consumption). The consumption index is
DERIVED by replaying acceptance-entry payloads on every resume; a
duplicate digest across entries or a malformed digest in one
refuses the state as corrupt. RECONCILIATION with the existing
mission ledger (which stays the narrative truth and is written
first, per current doctrine): a LEDGER-AHEAD state — ledger block
written, no acceptance entry — is an UNACCEPTED turn; recovery
inspects the tree and re-decides, and nothing was consumed.
State-ahead cannot occur (single writer, acceptance appended after
the ledger block). The acceptance writer uses the durable
two-outcome writer and treats committed-but-durability-unknown as
unaccepted-until-verified (re-read before proceeding). The
return's `certified` entries carry the authorizationDigest
explicitly (a schema field), ending job-ID guessing. Adjudication verifies every returned certification
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
verifies the host-artifact delta touches only declared files, AND
that those files are PAIRWISE DISJOINT from every consumed
authorization's changed paths in the same turn (r4 HIW-R4-05: a
declared artifact must never overwrite reviewed bytes while the
authorization keeps its consumption credit — overlap refuses).

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

THE RESOLUTION'S E-TRANSITION (r4 HIW-R4-02): either resolution
starts a NEW SEQUENCE SEGMENT — E(next) is the restored or adopted
tree, recorded as a resolution entry in the same hash-chained
state. Prior consumptions remain consumed. Every path that differs
between the pre-resolution expected tree and the resolution tree
counts as an INTERVENING CHANGE for the staleness predicate, so a
delayed authorization overlapping the resolution refuses and needs
fresh conformance. Resolutions never rebase or replay anything.

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
- SUPERSESSION is immutable (r4): a content-addressed record
  never mutates its own status; a NEW authorization names its
  predecessors in an immutable `supersedes[]` list and eligibility
  is DERIVED (an authorization is superseded iff a later valid one
  names it). Two un-superseded heads for one chain is a FORK and
  refuses consumption of either until a successor resolves it.
- MISSION STATE VERSION: the wall ships state schemaVersion 2; a
  valid version-1 state receives the exact legacy refusal BEFORE
  strict shape validation.
- HOST-ARTIFACT DECLARATION: mission contract key `hostArtifacts`
  (a list of canonical repository-relative files). PROTECTED-PATH
  TABLE (denied always): `plans/mission-*.contract.md`,
  `plans/goals.md`, `plans/goals-accepted.json`,
  `plans/instruction-ledger.md`, `plans/known-issues.md`.
- FIELD SCHEMAS: the authorization record, acceptance-entry
  payload, openTurn, taint/resolution entries, and wall.json each
  get a JSON schema beside the code (internal/missionrunner and
  internal/validate own their halves); digests are sha256 over the
  canonical wiredoc encoding — the repository's one canonical
  encoder, no ad-hoc serialization. The DIGEST DOMAIN (r5): the
  embedded `authorizationDigest` field is OMITTED from the bytes
  being digested (digest-then-embed); the filename carries the
  same digest. Event identifier fields follow the registry's
  existing conventions (`missionId`, ids under the registry's ids
  slot) — the exact registry types land with the implementation's
  event registration, per the r5 ruling that golden fixtures own
  the remaining shape choices.
- EVENTS (emitter → required payload): conformance validation →
  authorization-issued {authorizationDigest, jobId, mission};
  missionrunner adjudication → authorization-consumed
  {authorizationDigest, turnId} / authorization-refused
  {authorizationDigest, reason}; missionrunner wall →
  wall-passed {turnId, postTree} / wall-violated {turnId,
  unaccountedPaths}; missionrunner recovery → recovery-inspected
  {turnId, verdict}; mission state → taint-set {taintId} /
  taint-resolved {taintId, variant}. Registered in the event
  registry; observability only.
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
| HIW-O3 | CRITICAL | hiw-critique-r3 §1-2 | Provenance verified (mission incarnation, stream, role, job digest); consumption is one-time via the acceptance write; the staleness predicate admits disjoint delayed returns and refuses the rest | missionrunner adjudication | adjudicate.go verification + acceptance append | cross-mission, replay, superseded, later-turn-disjoint, later-turn-overlap, unknown-base, same-file-drift (object-id mismatch), rename-endpoint, multi-authorization, resolution-induced-staleness fixtures; POSITIVE fixtures: authorization after a no-change turn, and fresh post-resolution work consuming against the new segment | mission fixture with delayed certification across turns | MISSING | implement |
| HIW-O4 | CRITICAL | hiw-critique-r2 §2 | Exact ordered patch composition over the shared isolated-index primitive equals the accepted tree | shared git-tree primitive (one owner) | new internal/gittree package used by validator + runner | deletion, mode, symlink, binary, gitlink, order, overlap, non-application, crash-after-order-recorded-before-integration fixtures | conformance + wall over one real chain | MISSING | implement |
| HIW-O5 | CRITICAL | hiw-critique-r1 §5 | Every host-exit path checks the wall; violation taints and parks before measurement or completion, outranking gate success | missionrunner cycle/lifecycle | cycle.go fault path rework | green-gate-after-mutation must park; mutation on capped, nonzero, malformed-return, and recovery exits each checked | scripted-host mission fixture writing product bytes | MISSING | implement |
| HIW-O6 | CRITICAL | hiw-critique-r2 §4 | No new baseline while tainted; only typed RESTORE (exact equality) or ADOPT_DISPUTED_TREE clears; adoption earns no floor credit | mission resolve-taint verb + state chain | mission.go reserved verb | generic-answer refusal, restore mismatch, exact adoption, crash-recovery fixtures | tainted mission resolved both ways in a fixture | MISSING | implement |
| HIW-O7 | CRITICAL | hiw-critique-r2 §3 | The tree partition is equation-complete and default-deny; protected paths refuse even inside declared locations; machine-path tracking refused at mission start | mission contract parser + wall + preflight | contract.go declaration grammar | protected-path, path-escape, symlink-ancestry, glob-grant, tracked-metadata-refusal, untracked-non-ignored refusal, tracked-and-ignored refusal, preflight-before-any-write (refusal leaves tree unchanged), authorization-vs-host-artifact overlap fixtures | preflight refusal on a tracking repo fixture | MISSING | implement |
| HIW-O8 | HIGH | hiw-critique-r2 §5 | The interim rule text lands VERBATIM in both live authorities (host-turn-instruction.md, orchestrator.md), the doctrine narrows, and all four bm-2 completionGate.command fields carry the discipline sentence; interactive direct work is unaffected | prompt assembler + role/template + manifests | template/role bytes + manifest diffs | assembled-prompt byte test asserting the verbatim rule; interactive boundary test | a claude-host mission fixture turn showing the new prompt | MISSING | implement |
| HIW-O9 | HIGH | hiw-critique-r3 fold of r2 §6 | The delegation floor counts only a validated implementer job with a nonempty patch whose unsuperseded authorization was consumed into the accepted post-tree | benchmark extractor + evidence schema | extractor.py floor rule | empty, sham, replayed, unapplied, adopted-tree fixtures | re-extraction of the D99 cohort must stay invalid | MISSING | implement |
| HIW-O10 | HIGH | hiw-critique-r2 §7 | Authorization, wall, recovery, and taint evidence is durable, mirrored, and observable via registered events | dispatch mirror/close + event registry | mirror + events registration | restart/readback, missing-record close-failure, event-payload, tree-ref-cleanup-at-mission-close fixtures | mission fixture restart with evidence intact | MISSING | implement |
| HIW-O11 | HIGH | hiw-critique-r3 §4 | Pre-wall mission state refuses resume with the named error; no migration path exists | mission state loader | loader version check | preserved version-1 state receives the exact legacy error before shape validation | resume attempt on a preserved pre-wall state | MISSING | implement |
| HIW-O12 | HIGH | hiw-critique-r3 matrix gaps | core.fileMode is pinned in wall-touched repositories and the initial baseline is clean or human-sealed; violations refuse before the first turn | mission preflight | preflight checks | fileMode-off fixture; dirty-baseline refusal fixture; sealed-baseline acceptance fixture | preflight on a deliberately dirty target | MISSING | implement |
| HIW-O13 | CRITICAL | hiw-critique-r3 §2 | The acceptance write is the single commit point joining wall verdict, trees, turn log, and consumed digests; crash on either side leaves a consistent state | missionrunner acceptance append | single-append implementation | crash-before and crash-after fixtures proving no burn and no replay; cold derived-index rebuild; duplicate and malformed digest refusal; ledger-ahead recovery; durability-doubt re-verification | forced-kill mission fixture across the append | MISSING | implement |

## Loop discipline

Codex xhigh. The r3 critique verifies the r2 fold (all eight
findings) and attacks any remaining contract an implementer would
have to invent. No code before convergence or the ratified
mechanical-grain exit.
