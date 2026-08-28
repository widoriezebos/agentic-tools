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
- In flight right now: INTERPOSED AGAIN at Wido's direction —
  GitHub issues #4/#5/#6 (vm-smoke-4: the agents scored FULL
  MARKS on the held-out grader and the metasystem refused to
  bank it). Priority order #6 (sealed host caps), #5 (worktree
  delegate commits), #4 (stop-loss candidate visibility +
  ledgerSemantics 3). Wall slice 5 resumes after; its state and
  build map follow unchanged:
- Slice-5 state: ALL FOUR BUILD-MAP ITEMS BUILT (2026-08-18);
  round-1 critique returned SEVEN material findings, ALL FOLDED
  same day: (1) the open write anchors, and resume inspects an
  unfinished open turn BEFORE healing — violated parks with
  taint, clean closes the unaccepted marker; (2) the full
  sequence-point/staleness predicate: acceptance payloads carry
  their occurrence ({sequence, segment}, transition-pinned to the
  landing write), openTurn records the CURRENT sequence point,
  issuance refuses a base that is no named E-point, and
  consumption verifies the named point, intervening-change
  disjointness, byte-exact apply, and reviewed object-id/mode
  equality; (3) patch bytes authenticate against the issued
  patchDigest; (4) conclusions fail CLOSED — no wall.json, no
  entry, and the state transition refuses payload-less appends;
  (5) the wall runs BEFORE the drain can park for a lesser
  reason; (6) the design's full protected-path table; (7) pre,
  expected, post, base, and reviewed trees are all anchored.
  One NEW decision surfaced by the fold: the state anchor
  force-tracks the mission's own ledger into history, so exactly
  that one path (artifacts/agents/missions/<m>/ledger.md) is
  machine-owned metadata inside the projection and exempt from
  the equation — nothing else under artifacts/ is, so tracked
  smuggling stays caught. ROUND 2 overturned that exemption and
  found five more material defects; all six folded 2026-08-18:
  (1) authorization records AUTHENTICATE at consumption
  (validate.AuthorizationRecordDigest recomputed by the index and
  the wall — a coordinated record+patch rewrite dies at the first
  read; fixtures now self-digest, proving an attacker who
  self-digests forged content is caught by the SEMANTIC checks);
  (2) the ledger exemption is REMOVED — initializeState's anchor
  tracks the ledger before any turn opens, the wall runs before
  every append, so no exemption is needed and a host editing the
  ledger mid-turn (stop-loss reset injection) violates; (3) the
  resume violation ramp tolerates a ledger-ahead crash (already
  booked cycle skips the double append; taint and park still
  land); (4) staleness is a TURN-BY-TURN union of accepted
  changes, never an endpoint diff — changed-then-reverted paths
  refuse; issuance's current-point labeling is defended as sound
  by FULL-TREE equality (identical review context); (5) E0 joins
  the named-point index (first payload's preTree at {0,0}); (6)
  declared artifact paths refuse symlinked ancestry in the post
  tree. ROUNDS 3-5 then chased one legacy doctrine to its root:
  the state anchor committed the force-tracked ledger ONTO THE
  MISSION BRANCH, which poisoned crash recovery, delayed
  authorization identity, the segment fence, conformance
  inheritance, and the guard baseline. Resolution (2026-08-18):
  STATE ANCHORS LIVE ON A RUNNER-OWNED REF
  (refs/metasystem/missions/<m>/state-anchors), plumbing-built
  (ledger blob in the anchor commit's own tree, CAS ref update,
  pinned identity, bounded calls, --no-filters), branch and index
  untouched; the wall's snapshots filter the ledger path (legacy
  branches) and the in-turn guard compares live ledger bytes to
  mission.AnchoredLedgerTruth — the authenticated anchor-ref
  machinery — so a host committing its own alteration can never
  become its baseline; reconciliation tolerates EXACTLY one
  ledger-ahead block and re-anchors its heal (retry-safe);
  ErrNoAnchor means only "ref absent"; the live taint segment
  fences old-segment authorizations immediately; E0 is a named
  point; canonical docs rewritten (orchestration.md,
  mission-cycle-sequence.md S15f). Existing missions with
  on-branch anchors re-provision — consistent with the schema-3
  barrier. Awaiting round-6 verification. What landed beyond the map's text:
  the openTurn marker transition rule (immutable in flight — a
  write may open, conclude, or leave it, never swap it); the
  taint STOP in internalRun (an unresolved taint refuses every
  run mode before any turn machinery); the wall-violation park
  reason + runner ask class + registered event; the fake host's
  solo-build behavior and TestInternalRunSoloBuildParksWallViolation
  (the D99 shape end to end: park, taint naming solo.go, wall.json
  evidence, no turn-log conclusion, resume refused); every fixture
  repo now carries the deployment's projection boundary
  (.gitignore artifacts/ bin/ metasystem.conf — the wall made the
  divergence visible); the unmeasurable-fixture mechanism swapped
  from deleting a tracked file (now a wall violation by design) to
  deleting the gate's pinned ref. Known loose ends for the
  critique: gittree.DropAnchors has no caller yet (anchor refs
  accumulate past mission close); the wall payload's exactKeys
  shape means wall.json carries `violation` beside the five keys
  and builders strip it; certification adjudication trusts the
  content-addressed record file (no digest recompute — the
  filename==embedded-digest check only); resolution verbs
  (RESTORE / ADOPT_DISPUTED_TREE) are the next slice, so a tainted
  mission today is stopped but unresolvable by machinery.
  Original build map, for the record:
  (1) DONE: openTurn written at
  reservation (sequence point + anchored preTree, before the host
  launches), cleared by BOTH conclusion paths;
  (2) adjudication verification — certified entries pass
  UNVERIFIED today (turnio.go:124 copies returnDoc["certified"]
  straight into the turn entry; nothing in Adjudicate touches
  them — the D99 hole itself). Build: export
  validate.JobIdentityDigest(record) (wraps jobIdentityKeys +
  canonicalDigest); new missionrunner verification in Adjudicate:
  for each certified entry with verdict "accepted" — digest
  present+64hex; authorizations/<digest>.json exists;
  record.jobId==entry.jobId; record.mission==mission;
  record.missionIncarnation==live fences approvedContractSha256;
  record.jobRecordDigest==JobIdentityDigest(current job record);
  digest is a supersession HEAD (no record's supersedes[] names
  it; two heads for one rootJob = fork = reject both);
  digest not in mission.ConsumedAuthorizations(state).
  Rejected entries -> verdict.Rejected + the auto-ask machinery;
  verified entries ride the verdict (new field) and ConcludeFiles
  stops copying the raw return's certified — only adjudicated
  facts enter the turn log. Entries with verdict "rejected" need
  no authorization (null digest lawful).; (3) the tree equation at cycleConclude:
  postTree snapshot, post == pre + ordered patches (gittree.Apply
  chain from authorizations/<digest>.patch) + declared
  hostArtifacts (contract key; protected paths refuse), wall.json
  in the turn dir, refusal -> taint entry + park BEFORE any
  completion-gate success; (4) ConcludeTurn acceptance entry
  gains wall + consumedAuthorizations payloads (state v2 shapes
  landed in a29f303). Fixtures per HIW-O1/O3; critique per slice.
- Landed: slice 3 (authorization issuance, HIW-O2) — two critique
  rounds (3 findings then 0, AGREE). Round 1's sharpest catch:
  the prose-under-30 waiver path issued authorizations WITHOUT
  critic closure; mission chains now refuse any waiver by name
  (D100). Also folded: the authorization-issued event was being
  silently dropped (now registered), and the full-merge-path
  fixture set (waiver/critic-less/tamper/empty-diff). The
  return-schema field stays deferred to the adjudication slice —
  deferral verified sound by the critic.
- Landed: slices 1+2 — internal/gittree (the HIW-O4 primitive,
  one projection owner, reviewStage refactored onto it) and
  dispatch provenance (mission/incarnation/turn/stream immutable,
  complete-or-refused; incarnation read from the mission's own
  fences; follow-ups re-verify the live incarnation BEFORE any
  side effect; turnId/envelopeTurn split). Verified through FIVE
  codex critique rounds under the both-must-agree covenant,
  finding trajectory 5 → 3 → 3 → 2 → 0, "AGREE — slices land"
  (scratchpad wall-slice*-output.md; scanner gap recorded in
  KI-34).
- Waiting on the human: the session itself is mid-build on
  slice 5 right now (no human input needed); the build map above
  is the lossless handoff when the context window compacts.
- Next step: execute the slice-5 build map above in order, then
  its critique loop; then the remaining rows (doctrine/prompts,
  delegation floor, events, detector tier) — both-host gates
  before any ship.

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
  --mission M --taint <id> --by <name> --reason R (--restore <treeId>
  | --adopt --waives <claim> [--waives <claim> ...])`, classifying
  HUMAN-only through
  the same authority matrix as every reserved operation. (AMENDED at
  slice-6 landing, D109: the original sketch — restore without a
  taint id, adoption without identity or waived claims — contradicted
  this design's own resolution-record contract, which requires the
  taint id, the resolver's identity, the reason, and at least one
  named waived claim on adoption. The record contract is the
  authority; the sketch now matches it.)
- EVENTS (observability only; records are the authority):
  authorization-issued, authorization-consumed,
  authorization-refused, wall-passed, wall-violation,
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
  `memory/instruction-ledger.md`, `memory/known-issues.md`.
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
  wall-passed {turnId, postTree} / wall-violation {turnId,
  unaccountedPaths} (the registered live name); missionrunner recovery → recovery-inspected
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
| HIW-O6 | CRITICAL | hiw-critique-r2 §4 | No new baseline while tainted; only typed RESTORE (exact equality) or ADOPT_DISPUTED_TREE clears; adoption earns no floor credit | internal/missionrunner/resolve.go + internal/mission/state.go | resolve.go verb; state.go custody + transition rules; anchor.go pinned anchors | wall_test.go (TestResolveTaint* incl. TestResolveTaintThroughWrapper both ways), state_test.go custody/transition, missionrunner_verbs_test.go grammar | tainted mission resolved both ways through the wrapper in wall_test.go TestResolveTaintThroughWrapper; live VM run at seal | READY_FOR_RUNTIME | VM seal validation |
| HIW-O7 | CRITICAL | hiw-critique-r2 §3 | The tree partition is equation-complete and default-deny; protected paths refuse even inside declared locations; machine-path tracking refused at mission start | mission contract parser + wall + preflight | contract.go declaration grammar | protected-path, path-escape, symlink-ancestry, glob-grant, tracked-metadata-refusal, untracked-non-ignored refusal, tracked-and-ignored refusal, preflight-before-any-write (refusal leaves tree unchanged), authorization-vs-host-artifact overlap fixtures | preflight refusal on a tracking repo fixture | MISSING | implement |
| HIW-O8 | HIGH | hiw-critique-r2 §5 | The interim rule text lands VERBATIM in both live authorities (host-turn-instruction.md, orchestrator.md), the doctrine narrows, and all four bm-2 completionGate.command fields carry the discipline sentence; interactive direct work is unaffected | prompt assembler + role/template + manifests | template/role bytes + manifest diffs | assembled-prompt byte test asserting the verbatim rule; interactive boundary test | a claude-host mission fixture turn showing the new prompt | MISSING | implement |
| HIW-O9 | HIGH | hiw-critique-r3 fold of r2 §6 | The delegation floor counts only a validated implementer job with a nonempty patch whose unsuperseded authorization was consumed into the accepted post-tree | benchmark extractor + evidence schema | extractor.py floor rule | empty, sham, replayed, unapplied, adopted-tree fixtures | re-extraction of the D99 cohort must stay invalid | MISSING | implement |
| HIW-O10 | HIGH | hiw-critique-r2 §7 | Authorization, wall, recovery, and taint evidence is durable, mirrored, and observable via registered events | dispatch mirror/close + event registry | mirror + events registration | restart/readback, missing-record close-failure, event-payload, tree-ref-cleanup-at-mission-close fixtures | mission fixture restart with evidence intact | MISSING | implement |
| HIW-O11 | HIGH | hiw-critique-r3 §4 | Pre-wall mission state refuses resume with the named error; no migration path exists | mission state loader | `internal/mission/state.go` validate: the version barrier (v1 ErrLegacyState, v2/3 ErrPreSnapshotScopeState) runs before shape validation, and the missing-baseline named refusal outranks the shape diagnostic; sentinels pass through Reconcile unclassified | preserved version-1 state receives the exact legacy error before shape validation | `internal/missionrunner/legacy_state_test.go` TestPreWallStateRefusesResumeByName: public resume on preserved v1/v2/v3/baseline-less bodies (shape-invalid on purpose) gets the verbatim named refusal, byte-identical state after, no corrupt-state or recovery artifacts; plus the validate-level and Reconcile-level legs in `internal/mission/state_test.go` | DONE 2026-08-22 | — |
| HIW-O12 | HIGH | hiw-critique-r3 matrix gaps | core.fileMode is pinned in wall-touched repositories and the initial baseline is clean or human-sealed; violations refuse before the first turn | internal/missionrunner/wall.go + internal/contract/contract.go | wall.go wallPreflight (launch-gated); contract.go wall.sealed-baseline key | preflight_fixture_test.go TestWallPreflightPreconditions (filemode legs, dirty-baseline, sealed-baseline) + TestSealedBaselineBirthsAndRuns (the JOINED proof: seal, re-pin, child re-admission, recorded E0, a booked cycle with the sealed dirt in place) | preflight on a deliberately dirty VM target at seal time | READY_FOR_RUNTIME | VM seal validation |
| HIW-O13 | CRITICAL | hiw-critique-r3 §2 | The acceptance write is the single commit point joining wall verdict, trees, turn log, and consumed digests; crash on either side leaves a consistent state | missionrunner acceptance append | single-append implementation | crash-before and crash-after fixtures proving no burn and no replay; cold derived-index rebuild; duplicate and malformed digest refusal; ledger-ahead recovery; durability-doubt re-verification | forced-kill mission fixture across the append | MISSING | implement |
| HIW-O14 | MEDIUM | slice-7 review (D109 successor) | A mission admitted on a human-sealed dirty baseline composes with delegate authorization; a sharper-than-generic refusal additionally needs authenticated admission provenance (a baseline differing from committed HEAD also arises from a lawful mid-turn merge) | `internal/validate/authorization.go` + dispatch worktree creation | missionBaseSequencePoint generic named-point refusal (interim) | nested_conformance_test.go TestUnnamedBaseKeepsTheGenericRefusal (interim) | sealed-dirty mission dispatching an implementer on a VM target | MISSING | design the composition and the provenance-backed diagnosis |
| HIW-O15 | CRITICAL | slice-7 round-15 review | HEAD movement during a turn is accounted BY CONTENT: every commit advancing HEAD carries only reviewed-or-declared bytes (certified integrations, declared host artifacts, named expected-tree points — an empty or accounted-tree commit moves no unreviewed byte and is lawful), and staged-only bytes cannot ship through a commit the snapshot never sees | wall turn lifecycle + `internal/gittree` | none yet | none yet | host commits staged bytes mid-turn on a VM target | MISSING | designed and CONVERGED in records/wall/wall-snapshot-scope-design.md (WSS-1..13, D123 posture); implement |
| HIW-O16 | HIGH | slice-7 round-15 review | The HOST is fenced at the whole repository in nested checkouts exactly as delegates are: a sibling-path change on the host path surfaces as a wall violation or refusal, never invisibly | wall snapshot scope + admission | none yet (delegates: conformance.go refuseOutsideProjectChanges) | none yet (delegates: nested_conformance_test.go sibling witnesses) | host edits a sibling path on a nested VM target | MISSING | designed and CONVERGED with the O15 work in records/wall/wall-snapshot-scope-design.md (D123 posture); implement |
| HIW-O17 | HIGH | slice-7 round-15 review | Birth is provable from a durable record: a missing state file alone never authorizes stillborn cleanup or fence-clock reuse against a mission that may have lived | `internal/missionrunner` birth record | launch.go born.json stamped before publication, unstamped only on a same-pass proven failure, self-healed at resume; startAmbiguityRefusal freezes every start entry on the record, booked cycles, or surviving anchors | preflight_fixture_test.go TestLostStateFreezesTheBornMission (record, cycles-belt, anchors-belt legs) + TestBirthRecordSelfHealsAtResume | delete a live mission's state file, restart on a VM target | READY_FOR_RUNTIME | VM seal validation |
| HIW-O18 | HIGH | slice-7 round-18 review | Mission starts are serialized per mission id: one launcher holds an exclusive launch lock from its birth-evidence check through its pin writes, and the child's birth holds the same lock, so a concurrent start can never overwrite a newborn mission's pin, fences, or clock | mission launch ladder | launch.go acquireLaunchLock held across armAndPreflight; loop.go holds it across the child's checks, stamp, and publication | preflight_fixture_test.go TestLaunchLockSerializesStartDecisions (a launcher blocked across a birth re-checks and refuses; the newborn's clock survives) | two simultaneous starts on one mission id on a VM target | READY_FOR_RUNTIME | VM seal validation |
| HIW-O19 | HIGH | D117 (Wido's recovery-ladder ruling) | Wall violations recover on a ladder: the runner auto-restores mechanical cases (byte-exact safe-tree restore, authenticated ledger-blob restore) through the resolution engine under a runner identity; a human is asked only for adoption, no verifiable restore, or a repeat offense | resolution engine + runner recovery pass | none yet (design draft: recovery-ladder-design-draft) | none yet | offense, auto-restore, continue, no ask — on a VM target | MISSING | design per D117; claimable under the parallel backlog |

## Loop discipline

Codex xhigh. The r3 critique verifies the r2 fold (all eight
findings) and attacks any remaining contract an implementer would
have to invent. No code before convergence or the ratified
mechanical-grain exit.
