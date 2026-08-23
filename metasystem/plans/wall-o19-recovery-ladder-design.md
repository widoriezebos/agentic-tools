# Review brief: wall-o19 recovery ladder

Round budget: 3 focused rounds — agreed before round one; exhaustion
follows the critique skills' budget rules, never a silent round 4.

Threat model: one trusted human operator; no external adversaries.
IN SCOPE: accidents and crashes around violation recovery (a crash
mid-restore, a re-verification that lies, a repeat pattern hidden by
silent recovery); a mission host gaming the ladder — engineering a
violation whose automatic recovery would launder bytes or whose
repetition stays invisible; forged or stale safe-tree/ledger-blob
records. OUT OF SCOPE: the unbuilt isolation tier (D100 ruling 2),
runtime compromise, hostile third parties, and the sealed-dirty
composition (wall-o14 — stopped and raised separately; this design
must not depend on its outcome).

Appetite: 6h for this design; findings whose fixes exceed it pause
and go to the human.

Scope: the tier structure below; ResolveTaint's split into an engine
core; the runner's tier-2 recovery pass inside the park flow; the
repeat-offense derivation; event registration and emission; the
fixture set. OUT: any new state schema fields; any change to the
human resolve-taint verb's CLI contract; recovery of anything but
workspace and ledger domains.

Return format: numbered findings, most severe first, each with
file, rule, and the concrete failure it causes; or AGREE with
observations that do not gate.

---

# Design: the wall recovery ladder (revision 1, from the D117 draft)

Wido's ruling, verbatim intent: wall violations must not freeze the
mission for the human by default. The machinery first figures out
how to recover; the human is asked only on ambiguity or big
implications.

## Tier 1 — record (unchanged from the landed slice-6 doctrine)

Every violation books its taint entry (the mint returns the id and
the taint-set event carries it), writes wall.json with unaccounted
paths, and anchors the disputed tree. The audit trail is identical
whether recovery is automatic or human.

## Tier 2 — automatic recovery (the new runner authority)

Runs in the SAME runner pass as the park, after the park's state
proposal lands and before any ask is raised.

1. INSPECTION FIRST, AS ITS OWN RECORDED STEP. The runner evaluates
   mechanical recoverability and emits the already-registered
   recovery-inspected event with verdict restorable | unrestorable
   | adoption-question, before attempting anything. An unrestorable
   or adoption verdict falls through to tier 3 with the verdict on
   the ask.
2. WORKSPACE RESTORE: the recorded safe tree (the violated turn's
   pre-tree / current expected point) exists as an authenticated
   anchor and git can materialize it. After materialization the
   runner re-verifies byte-exact equality through the SAME
   recordedSafeTree + observed==tree path the human verb uses —
   tier 2 calls ResolveTaint's engine core with
   resolvedBy="runner:auto-restore", never a parallel code path.
   The engine core is the existing function body behind the verb;
   the human/CLI entry keeps its authority classification, the
   in-process runner entry records runner identity instead. One
   body, two authenticated entries.
3. LEDGER RESTORE: the anchored blob exists and authenticates under
   the landed slice-6 predicate; bytes rewrite from the blob through
   the same core.
4. ON SUCCESS: the resolution records variant=restore with the
   runner identity and a reason naming the violation; the segment
   advances; the mission CONTINUES; no ask exists. A registered
   wall-auto-recovered event carries the taint id and the restored
   domain(s). Restoring discards the violating bytes by
   construction — an engineered violation gains nothing from its
   own restoration, which is why tier 2 can be automatic at all.
5. ON ANY FAILURE — materialization error, re-verification
   mismatch, crash — the taint stays unresolved and tier 3 raises;
   a crash between restore and resolution record leaves the taint
   unresolved and the run-mode taint STOP (already landed) refuses
   every run mode at resume, so a half-recovery cannot be ridden.

## Tier 3 — human escalation (narrowed, not removed)

An ask is raised and the mission stays parked exactly when:
1. adoption is the question — waiving attribution is inherently
   human;
2. no mechanically verifiable restore exists (no authenticated
   safe tree or blob; materialization fails; re-verification
   fails);
3. REPEAT OFFENSE: the same mission suffers another violation
   within K=1 turns of an auto-recovery — the very next turn. A
   pattern is a judgment call, not an accident. The ask names the
   prior auto-recovery's taint id so the human sees the pattern.

The repeat-offense predicate derives from the hash-chained taint
entries alone: an unresolved-now entry whose predecessor entry
carries resolution.resolvedBy with the "runner:" prefix and a
turnId within the lookback window. No new state fields.

The human resolve-taint verb is byte-identical in contract for
every tier-3 case.

## Events

- recovery-inspected (registered, emission lands here): verdict +
  turnId, one per violation, before any attempt.
- wall-auto-recovered (registered in this change): taintId +
  restored domains + turnId. Records remain the authority.

## Doctrine edits in the same landing

- hiw-critique-r2 §4's "human-reserved" amends to the ladder,
  recorded as Wido's ruling with its date.
- The HIW-O6 row gains the tier-2 fixtures.

## Fixture obligations (the arbiter)

- F1: solo-build offense → park → same-pass inspection
  (recovery-inspected restorable) → auto-restore → byte-exact
  re-verification → mission continues → NO ask exists.
- F2: ledger-domain auto-restore from the authenticated blob.
- F3: adoption question escalates: ask raised, human verb resolves,
  ladder never touches it.
- F4: no-safe-tree shape escalates with verdict unrestorable.
- F5: repeat offense: offense → auto-restore → next-turn offense →
  ask raised naming the prior auto-recovery's taint id; the human
  verb works unchanged.
- F6: crash between restore and resolution record: taint stays
  unresolved, resume refuses through the landed taint STOP, no
  double-restore on the next pass.
- F7: the runner's tier-2 entry cannot be reached from the CLI:
  the classification gate still refuses non-human callers of the
  verb (regression of the landed authority matrix).
- F8: re-verification mismatch (tampered anchor content) refuses
  the auto-restore and escalates — the restore trusts the
  verification, never the anchor's name.

---

# ROUND 1 STATE: nine material findings — STOPPED AND RAISED

Codex round one (archived under artifacts/agents/critiques/wall-o19/)
returned nine material findings, four CRITICAL, all transaction- or
invariant-level: no crash-safe park-recover-ask ordering exists (the
landed park writes asks first); the landed taint STOP contradicts the
promised crash escalation; the park can anchor a poisoned ledger that
tier 2 would then faithfully restore; "inside the park flow" is four
different detection phases needing a phase table and a typed outcome;
the destructive restore target is ambiguous exactly where choosing
wrongly loses accepted work; the adoption verdict has no derivable
predicate; the K=1 repeat window cannot be derived from taint entries
as they exist; the runner: prefix is presentation text, not
provenance; and events are best-effort while the design leaned on one
as evidence.

The stop is not only the appetite. Two findings embed decisions that
are HUMAN-RESERVED:

1. THE ADOPTION POLICY (O19-R3-06): for a restorable violation that
   might contain valuable work, does the inspector always discard
   (restore, never ask) or always ask? This decides how often the
   human is interrupted — the exact quantity D117 rules on — and no
   recorded fact can derive it.
2. RUNNER PROVENANCE (O19-R3-08): distinguishing automatic from
   human resolutions authentically either reserves a prefix in the
   human CLI's --by (a contract change) or adds an authenticated
   actor field (a schema change); both cross lines the scope
   declared closed.

Recommendation for re-scoping when Wido rules: fold the nine
findings under a fresh brief with the phase table and the ledger
transaction as the design's spine, appetite a full day (the o14
lesson: wall design rows are systematically bigger than their
original tokens). The chain stands at one round spent; any
successor enumerates all nine findings.

---

# Review brief: wall-o19 successor chain (fresh three-round budget)

Round budget: 3 focused rounds. This is the SECOND chain; per the
budget rules a second exhaustion stops with the work waiting on the
human, regardless of finding count.

Threat model: unchanged from the first brief — one trusted operator;
accidents, crashes, laundering-through-recovery, and repeat patterns
in scope; the isolation tier, runtime compromise, and the sealed-dirty
composition out.

Appetite: 1d (Wido's re-scope token, with his two rulings below).

Open findings this chain MUST resolve, enumerated per the budget
rules: O19-R3-01 (no crash-safe park-recover-ask transaction),
O19-R3-02 (the taint STOP blocks the promised crash escalation),
O19-R3-03 (the park can anchor a poisoned ledger tier 2 would
restore), O19-R3-04 (four detection phases need a phase table and a
typed recovery outcome), O19-R3-05 (the destructive restore target is
ambiguous where choosing wrongly loses accepted work), O19-R3-06 (the
adoption verdict has no derivable predicate), O19-R3-07 (the repeat
window is not derivable from taint entries as they exist), O19-R3-08
(runner-prefix provenance is presentation, not authentication),
O19-R3-09 (best-effort events cannot be the inspection's evidence).

Wido's rulings, now inputs rather than questions: restorable
violations NEVER ask — auto-restore silently, a repeat offense still
escalates (this DERIVES the adoption predicate O19-R3-06 demanded:
the inspector never labels adoption; adoption exists only as the
human's own resolve-taint choice). Provenance gets an authenticated
ACTOR FIELD in the resolution record — a deliberate schema change,
which also settles O19-R3-08 and gives O19-R3-07 its recorded
dimension (the actor field plus the acceptance entry's turn identity
make the repeat window derivable).

Return format: numbered findings, most severe first, each with file,
rule, and the concrete failure; or AGREE with observations that do
not gate.

---

# Design: the wall recovery ladder (revision 4 — one anchored commit point)

Round two's verdict named the disease behind every symptom: recovery
had no authenticated, reachable commit point. Revision 4 builds
exactly one primitive and derives everything else from it.

## The primitive: the ANCHORED RECOVERY INTENT

At park time, in the same pass that books the taint, the runner
writes ONE recovery-intent record covering EVERY violated domain
(S2-R2-01) — per-domain verdicts and per-domain targets, the
detection phase, the inspection evidence digests, and the typed
plan — then COMMITS it to a git anchor:
refs/metasystem/missions/<m>/recovery/<taintId>, a commit whose
tree holds the record and whose creation is the RECOVERY COMMIT
POINT. The state's taint entry stores only {taintId, status,
anchorCommit}. Authentication after any crash is the anchor: a
recovery admitted anywhere verifies the record against its anchored
bytes, and a record without its anchor is no record (S2-R2-03 —
state hashes cannot vouch for bodies, so the anchor does).
The anchor commit's own position supplies the durable sequence
point the repeat predicate reads (S2-R2-07): repeat = a new
recovery anchor whose commit is reachable from the prior
runner-resolved anchor with no accepted turn commit between them,
judged per phase exactly as revision 3 ruled.

## The record's shape

{ taintId, phase, domains: [{domain, verdict:
mechanical|escalate, target, materialization}], status:
pending|restored|escalated, actor, createdAt } — status transitions
live in STATE (custody: the runner-recovery writer capability,
unchanged from revision 3); everything else is immutable under the
anchor. A record listing ANY escalate domain escalates whole: partial
automatic recovery of a multi-domain violation is exactly the
masking round two convicted (S2-R2-01).

## Materialization is path-scoped (S2-R2-09)

Each workspace domain's materialization names the exact paths the
wall found unaccounted — restore THOSE paths from the target tree,
touch no carrier, delete nothing outside the list, preserve every
filtered runner file and untracked non-violating path. The clean-
carrier narrowing from revision 3 stands: any carrier divergence is
an escalate domain by classification, so path-scoped restore is
the ONLY mechanical materialization that exists. Re-verification
re-runs the wall's own capture over the named paths.

## The ledger transaction, repaired (S2-R2-02)

The composed post-recovery ledger — prior blob PLUS the park
block — is built in memory at intent time and recorded as the
ledger domain's target (bytes digest in the record, bytes in the
anchor tree). The seven steps collapse to: anchor the intent (the
commit point), restore the composed bytes, verify byte-exact
against the record, advance status. The pending-aware anchor lane
anchors the RECORDED composed target; the cycle-count precondition
holds because the composed bytes already carry the park block.
Disputed bytes ride the anchor tree as evidence, replacing
revision 3's ad-hoc turn-directory file.

## Escalation and admission (S2-R2-04)

Resume's recovery-only mode admits a parked mission whose taint
entry carries a recovery anchor in ANY non-terminal shape: pending
(complete the plan), escalated-without-ask (create the ask,
idempotent by taintId), restored-unconcluded (finish bookkeeping).
The three gates key on the anchor's existence and validity, not on
one status value — every crash window converges.

## The typed outcome, restored to the arbiter (S2-R2-05)

recoveryOutcome = restored | escalated | deferred, and the caller
mapping is part of the design: pre-acceptance restored → the pass
re-runs its gate over the restored tree and proceeds to a lawful
acceptance or a fresh verdict, never replaying consumed work;
post-verification restored → the pass appends the clean
re-verification entry so the accepted completion stands;
reservation-continuity → deferred (below); escalated anywhere →
the pass ends parked-with-ask. G6 pins every mapping.

## Deferred has an owner (S2-R2-06)

The next-tick actor is the STEWARD — the machinery whose whole
charter is that open delegated work is never silently idle. A
parked mission with a live recovery anchor is OPEN WORK to the
steward's census, and its revival path invokes recovery-only
resume exactly as it revives continuations today. No new
scheduler; the fleet's existing watcher is the trigger, and the
recovery design composes with the landed steward rather than
inventing a second one.

## The legacy variant (S2-R2-08)

Old states migrate their unresolved taints to a LEGACY recovery
record: {kind: legacy, status: escalated} with no phase, domains,
or target — its ONLY lawful continuation is the human's
resolve-taint, and the upgrade creates its ask. Nothing is
invented, nothing trusts mutable evidence, nothing old ever
auto-restores; historical resolutions keep revision 3's
conservative human-actor mapping.

## Unchanged from revision 3

The phase table and phase-owned targets; state-first-ask-second
with idempotent repair (now anchored, S2-R2-03); the writer
capabilities and per-entry target selection; the clean-carrier
narrowing; the resume-time versioned upgrade; both events in the
registry and the arbiter.

## Fixture obligations (the arbiter, revision 4)

- H1: the anchor IS the commit point — a taint entry whose anchor
  is missing or mismatched refuses every recovery admission.
- H2: multi-domain — a simultaneous ledger and workspace violation
  produces one record with both domains; any escalate domain
  escalates whole; no partial restore exists.
- H3: path-scoped materialization — only the named paths change;
  carriers, filtered files, and untracked innocents survive; a
  moved-HEAD carrier escalates named (revision 3's G9).
- H4: the composed-ledger target — anchor, restore, byte-exact
  verify, cycle count holds; disputed bytes readable from the
  anchor tree.
- H5: every crash window — after intent, after restore, after
  escalated-before-ask — converges through recovery-only resume;
  working modes refused throughout.
- H6: the typed outcome's four caller mappings, including
  post-verification's clean re-verification append and
  pre-acceptance's no-replay rule.
- H7: the steward revives a parked-with-anchor mission through
  recovery-only resume — the deferred trigger proven end to end.
- H8: the repeat predicate from anchor positions, per phase, both
  window sides.
- H9: writer capabilities both directions (revision 3's G8).
- H10: the upgrade — legacy records escalate with their ask; the
  chain advances; nothing old auto-restores.
- H11: both events, payload and ordering.

## Design-obligation matrix

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| RL-O1 | CRITICAL | o19 r4 primitive | The anchored recovery intent is the one authenticated commit point | mission anchor + state | intent record + anchor ref | H1, H5 | forced-kill across the park in the suite | MISSING | implement |
| RL-O2 | CRITICAL | o19 r4 domains | Multi-domain records; escalate-whole on any non-mechanical domain | missionrunner wall | combined inspection | H2 | dual-violation fixture | MISSING | implement |
| RL-O3 | CRITICAL | o19 r4 materialization | Path-scoped restore; carriers and innocents untouched | missionrunner recovery + gittree | path materializer | H3 | moved-HEAD escalation | MISSING | implement |
| RL-O4 | CRITICAL | o19 r4 ledger | The composed-target ledger transaction through the pending-aware lane | mission anchor | composed target + lane | H4 | ledger-tamper end to end | MISSING | implement |
| RL-O5 | CRITICAL | o19 r4 admission | Recovery-only resume admits every non-terminal anchored shape | missionrunner launch/loop | gate keying on the anchor | H5 | crash-window suite legs | MISSING | implement |
| RL-O6 | HIGH | o19 r4 outcome | The typed outcome's caller mappings | missionrunner callers | outcome type + mappings | H6 | resume and post-verification legs | MISSING | implement |
| RL-O7 | HIGH | o19 r4 steward | The steward revives parked-with-anchor missions | steward + missionrunner | census + revival wiring | H7 | steward tick reviving in the suite | MISSING | implement |
| RL-O8 | HIGH | o19 r4 repeat | Repeat from anchor positions, phase-correct | missionrunner recovery | anchor adjacency | H8 | per-phase repeat legs | MISSING | implement |
| RL-O9 | HIGH | o19 r4 actor | Writer capabilities, both directions | mission state + entries | capability plumbing | H9 | runner actor in history | MISSING | implement |
| RL-O10 | MEDIUM | o19 r4 legacy | The legacy record variant with its ask | mission state upgrade | legacy kind + upgrade | H10 | old-state upgrade leg | MISSING | implement |
| RL-O11 | MEDIUM | o19 r4 events | Both events registered, emitted, ordered | events + missionrunner | registry + emissions | H11 | recorder stream leg | MISSING | implement |

---

# SECOND EXHAUSTION — THE DESIGN WAITS ON WIDO (by rule)

Two chains, six rounds, trajectory 9-11-9-8: never converging. What
six rounds proved is itself the finding: AUTOMATIC RECOVERY FROM A
WALL VIOLATION INTERSECTS EVERY HARD INVARIANT THE SYSTEM OWNS —
the state/anchor publication order (S2-R3-01), the wall's own
namespace law (the recovery ref would be judged a violation,
S2-R3-04), the authorization-composed expected tree (restoring the
pre-tree can discard reviewed work, S2-R3-02 — a correction that
stands regardless of any future path), whole-posture re-verification
(S2-R3-03), mixed-domain human closure (S2-R3-05), provenance
ownership across immutable and mutable records (S2-R3-06), a git
graph that can actually answer ordering (S2-R3-07), and steward
authority that does not yet exist (S2-R3-08).

What the chains produced of lasting value: the phase table with
phase-owned targets; the anchored-intent concept; the clean-carrier
narrowing of "mechanical"; the composed-ledger target; the
never-ask ruling dissolving the adoption predicate; and eight sharp
constraints any future design must honor, each grounded in landed
code.

## The options for Wido

1. THE MINIMAL LADDER (recommended): descope to the dominant case
   only — pre-acceptance phase, workspace domain only, all carriers
   clean, no simultaneous domains, and ANY doubt (crash, mixed
   domain, dirty carrier, unlisted late mutation) escalates rather
   than resumes recovery. No new refs, no schema cutover beyond the
   recovery field, no steward wiring — recovery either completes in
   its own pass or becomes a human ask. Perhaps two days honestly;
   kills the most common interruption while every hatch opens
   toward the human.
2. DEFER D117 WHOLE: wall violations stay human-resolved until
   mission volume proves the interruption cost; the six rounds'
   constraints wait in this file.
3. THE FULL PRIMITIVE AS ITS OWN PROGRAM: the authenticated
   recovery transaction (state+anchor publication protocol) built
   as a mission-state feature first, the ladder second — a
   multi-day program with its own design chains.

Reopen from this record; any future chain enumerates
O19-S2-R3-01..08.

## SLICE A LANDED (2026-08-23, the minimal ladder's first day)

What landed, per Wido's minimal-ladder ruling and the slice-A token:

- gittree.MaterializePaths: path-precise worktree restore from a named
  tree — blobs, modes, symlinks; refuses gitlinks and symlinked
  ancestors before any byte moves; never touches index, HEAD, or refs.
- wallGate grew the mechanical rung (attemptWallRecovery), engaged ONLY
  at the main conclusion gate (concludeCycle) — never at resume
  re-drives, where the crash itself is the doubt that belongs to the
  human. Eligibility: no declaration/ledger violation, the violation is
  the undeclared-change branch (UndeclaredOnly), stable capture,
  judgeScope + judgeCaptureIntegrity clean over that capture (clean
  carriers), no prior recovery in the mission (repeat -> human), all
  restore paths materializable. Restore set: Unaccounted minus declared
  artifacts. Target: the COMPOSED expected tree (S2-R3-02 honored — a
  fixture proves reviewed bytes B survive, never pre-tree A).
- Whole-posture re-verification: the restore is re-verified by a full
  fresh inspection round (S2-R3-03 honored — never by rechecking only
  the named paths). Any residual violation parks with current truth.
- The anchored recovery record: a `recovered` block (violation,
  restoredPaths, restoredAt) on the acceptance entry's wall payload —
  anchored by the existing state chain. NO new refs (S2-R3-04 avoided),
  NO taint resolution written by the runner (refuseResolutionTransition
  stays untouched — resolutions remain human-only), NO schema cutover
  (one optional exact-shaped key; old states validate unchanged).
- Crash story, both tails, as corrected by the critic's round (codex
  NOT AGREE, four findings, all fixed): sticky wall.json violation ->
  re-park verbatim -> existing human path. A PUBLISHED recovery pass
  whose anchored record never landed (crash before acceptance) also
  PARKS verbatim at the next gate: the recovery record reaches a pass
  ONLY through the live in-pass context (the acceptance stability loop
  hands its block down); rewritable evidence is never promoted into the
  chain, so no window loses both the taint and the record, and a forged
  `recovered` block in evidence can never enter the chain at all. The
  capture authority check (acceptancePayloadMismatch) refuses payloads
  whose record disagrees with the gate that ran.
- MaterializePaths, as corrected across the critic's three rounds:
  paths resolve against the WORKSPACE root (toplevel + prefix — the
  nested checkout would otherwise restore into the wrong directory);
  every mutation runs through an os.Root descent that Lstats each
  directory component no-follow and refuses symlinks before opening
  (per-path handle scope, so a large restore set never accumulates
  descriptors); modes are set on the open file handle, never by
  pathname. The confinement claim, stated honestly per round 3: a
  pre-planted symlink anywhere in a chain REFUSES, and no operation
  can escape the opened root — but a hostile process racing the
  descent can still redirect a write to a wrong-but-confined in-root
  location, and a rename can relocate an already-opened directory.
  Both degrade to wrong bytes somewhere the restore was allowed to
  write, never a silent success: the whole-posture re-verification is
  the safety floor that turns every such outcome into a refused
  recovery and a parked taint. The one path trusted by name is the
  workspace root argument itself — exactly as far as every git -C
  invocation trusts it (recorded shared assumption).
- Two more rung refusals from round 2: mixed domains refuse BEFORE any
  byte moves (the inspection stops at the first undeclared path, so the
  rung scans the whole unexplained delta for a declared artifact
  disputing reviewed bytes and refuses pre-mutation), and the carriers
  are judged at RESTORE time on a fresh capture that must equal the
  judged one whole — a mission-namespace carrier planted between the
  verdict and the rung, or any byte motion since the verdict, refuses
  the restore.
- recovery-inspected events (already registered): refused / failed /
  recovered.

One deliberate delta recorded for review: runWallInspection now sets
stable=true on the violating-branch confirmation (the original left
the flag unread there; the rung is its first consumer).

Constraint dispositions: S2-R3-01 avoided (no cross-store publication;
in-pass only), S2-R3-02/03 honored by construction and fixture,
S2-R3-04 avoided (no refs), S2-R3-05 avoided (single-domain only;
mixed domains refuse the rung), S2-R3-06 narrowed (the record lives in
ONE owner, the acceptance chain; no actor field until slice B),
S2-R3-07 narrowed (repeat = any prior recovered acceptance in this
mission's chain — locally answerable, fail-toward-human), S2-R3-08 out
of scope (no steward wiring; recovery completes in its own pass or
parks).

SLICE B (second 1d, tokened when this lands): escalation-ask
automation, events beyond the three verdicts, crash-window hardening
within this shape.

## SLICE B LANDED (2026-08-24, the ladder's second day)

What landed: the escalation carries its context. The wall-violation
ask now arrives with a recoveryNote — the rung's refusal reason ("a
repeat offense in this mission belongs to the human", "the violation
is in the declaration or ledger domain"), a failed re-verification's
residual violation, or the published-recovery crash-tail explanation —
and the violating wall.json carries the same story as a `recovery`
field whenever the rung was consulted, so the turn dir answers the
whole question without a walk through the event stream. Sticky crash
tails never rewrite evidence; their context rides the ask alone.

Two token candidates REJECTED, with reasoning:

- The composed expected tree as a recordedSafeTree for the human
  RESTORE: after a recovery crash tail the violated turn never
  accepted, so its authorizations were never consumed — they survive
  as records and re-drive in the next turn after an ordinary pre-tree
  restore. The widening would re-enter the authorization-staleness
  swamp (rounds r4-r5) to save one turn's re-run. The ask note tells
  the human exactly this.
- Events beyond the three verdicts: the registered recovery-inspected
  event already speaks refused/failed/recovered, and the ask +
  evidence notes carry the prose. Records are the authority; events
  stay observability; a richer event would duplicate both.

The critic's slice-B round (codex NOT AGREE, five findings, all
fixed): the evidence enrichment is best-effort by contract — a failed
context write must never convert "park and taint" into a runner error;
the sticky crash tail replays a persisted note onto the ask (decoration
only — the taint reason stays the verbatim violation); the crash-tail
note now states the re-drive fact itself (unconsumed authorizations
survive and re-drive after a pre-tree restore); the shipped
wall-evidence schema mirrors the Go shape again (scope/posture on both
verdicts, recovered on a pass, recovery on a consulted violation); and
the failed-re-verification shape is pinned through a named test seam
(postRestoreHook): a late mutation between restore and re-verification
becomes a fresh violation with the failed note on ask AND evidence,
never a laundered pass.

With slices A and B landed, the MINIMAL LADDER per Wido's ruling is
complete. The upper rungs (multi-domain recovery, the authenticated
recovery transaction, steward-invoked resume) remain in this file's
constraint record for reopening under the drop rule.
