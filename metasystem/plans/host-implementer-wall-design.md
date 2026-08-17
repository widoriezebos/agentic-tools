# The host-implementer wall (goal host-implementer-wall, D99/D100)

- Status: DRAFT r3 — r1 and r2 folded (plans/hiw-critique-r1.md: 9
  findings; plans/hiw-critique-r2.md: 8 findings + the obligation
  matrix). D100's rulings stand: NO self-work exception, DETECTOR
  tier. r3 completes the contracts r2 found missing: the
  replay-safe authorization, the total tree-composition rule, the
  taint state machine, the interim prompt text, and the hardened
  floor outcome rule.
- Goal: host-implementer-wall (Current)
- In flight right now: the r3 design critique (codex xhigh); not a
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
so its contribution is empty; an adopting repository that TRACKS
runner metadata gets those bytes represented as exact deltas like
everything else, never exempted because the path looks
machine-owned. Anything the equation cannot account for is a
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

CONSUMPTION is one-time and ledgered: the runner keeps a durable
`authorizationDigest → consumedByTurnId` ledger. Delayed
certification of landed returns stays legitimate (an unconsumed,
current, unsuperseded authorization is usable in a later turn);
a consumed or superseded one is rejected. Adjudication verifies
every returned certification against the authorization record AND
the dispatch job record — role, stream, mission incarnation —
never trusting the return's own fields; only adjudicated facts
enter the turn log. Dispatch gains the structured immutable
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

## Design obligations (the gate's matrix)

| ID | Severity | Required behavior | Owner / code target | Focused proof target |
|---|---|---|---|---|
| HIW-O1 | CRITICAL | Persist and anchor reachable pre-tree/open-turn state before host launch; inspect before recovery healing or rebasing | missionrunner + mission state | crash and object-GC fixtures at every snapshot boundary |
| HIW-O2 | CRITICAL | Issue authorization only after the exact patch/base/reviewed-tree and critic-closure join | conformance + dispatch evidence | mutated/stale patch, wrong base, empty patch, critic-mismatch fixtures |
| HIW-O3 | CRITICAL | Verify immutable mission/incarnation/stream/role/job provenance; consume authorization once; allow legitimate delayed landed returns | dispatch + adjudication | cross-mission, cross-stream, duplicate, replay, superseded, later-turn fixtures |
| HIW-O4 | CRITICAL | Deterministically compose exact patches and compare the full git tree | shared git-tree primitive + missionrunner | deletion, mode, symlink, binary, gitlink, order, overlap, non-application fixtures |
| HIW-O5 | CRITICAL | Every host-exit path checks the wall; violation taints and parks before measurement or completion | missionrunner lifecycle/state | green completion gate after host mutation must still park |
| HIW-O6 | CRITICAL | No new baseline while tainted; only typed restore/adopt clears, never manufacturing floor credit | mission resolution path + state | generic-answer refusal, restore mismatch, exact adoption, crash-recovery fixtures |
| HIW-O7 | CRITICAL | Tree partition is equation-complete and default-deny | mission contract parser + wall | protected path, path escape, symlink ancestry, directory/glob grant, tracked-metadata fixtures |
| HIW-O8 | HIGH | Mission prompts carry no self-work license; interactive direct work unaffected | prompt assembler, role/template, orchestration doctrine | assembled-prompt byte test + interactive boundary test |
| HIW-O9 | HIGH | Delegation floor counts only nonempty, actually consumed adjudicated authorization per stream | benchmark extractor / evidence schema | empty, sham, replayed, unapplied, adopted-tree fixtures |
| HIW-O10 | HIGH | Authorization, wall, recovery, and taint evidence is durable, mirrored, observable | dispatch mirror/close + event registry | restart/readback, missing-record close failure, event-payload fixtures |
| HIW-O11 | HIGH | Legacy mission state migrates safely or refuses explicitly | mission state loader | old-schema resume fixture |

Events (authorization issued/consumed/refused, wall
passed/violated, recovery inspected, taint resolved) are
observability; the state and authorization records are the
authority.

## Loop discipline

Codex xhigh. The r3 critique verifies the r2 fold (all eight
findings) and attacks any remaining contract an implementer would
have to invent. No code before convergence or the ratified
mechanical-grain exit.
