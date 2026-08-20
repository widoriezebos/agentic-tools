# Migration manifest for this repository (backlog-git-sync)

Applied by `goal migrate --manifest` in the same transaction as the
generic conversion. Reviewed with plans/backlog-git-sync-design.md;
the design's expected map is this file's acceptance oracle.

SCHEMA, closed and deterministic (R5-09): entries are `### add-goal:
<id>` or `### amend-goal: <id>` headings with `- key: value` lines.
add-goal REQUIRED keys: Intent, Origin, Next; OPTIONAL: BlockedBy
(comma-separated existing ids), Arc. amend-goal keys, each OPTIONAL
but at least one required: state (closed enum: queued | parked —
claimed and done are not manifest-assignable), parked-by and
parked-at (REQUIRED together whenever state: parked; parked-at may be
the literal EPOCH), parked-because, next, blockedBy (full
replacement), arc. UNKNOWN keys refuse; a duplicate key within an
entry refuses; a duplicate entry id refuses; add-goal of an existing
id refuses; amend-goal of a missing id refuses. Values are single
paragraphs, LF-joined, no escaping (a literal `- ` at a continuation
line start is forbidden and refuses). BlockedBy, when present, names
only ids that exist after all add-goal entries apply; an empty
BlockedBy is expressed by OMITTING the key, never by a sentinel.
MIGRATION_EPOCH and REVIEWED_SOURCE_SHA256 are REQUIRED header
fields, each appearing exactly once before the first entry; a
duplicate or missing header field refuses. In amend-goal entries,
OMITTING blockedBy means "no amendment"; the explicit clearing form
is `blockedBy: -` (the one place `-` is lawful, meaning "now empty").
A `state: parked` amendment without parked-because refuses — a park
always carries its reason. Positions are ONE-BASED in
textual order of add-goal entries; OpenedAt for add-goals = EPOCH +
(1000 + position) minutes. No entry is a no-op: an amend that changes
nothing against the converted output refuses at build time.

MIGRATION_EPOCH: 2026-08-20T00:00:00Z
REVIEWED_SOURCE_SHA256: 266f3dc6a7c3c2cbb884349e54fca0c1f0f33db9b188a6d39ddd245f35e11a94

## add-goal entries (the wall's open obligation rows, D115-D117)

### add-goal: wall-o13-acceptance-write
- Intent: The acceptance write is the single commit point joining wall verdict, trees, turn log, and consumed digests; a crash on either side leaves a consistent state (HIW-O13, CRITICAL)
- Origin: main
- Next: Design the single-append commit point per the wall design's O13 row; crash-before and crash-after fixtures; cold derived-index rebuild.

### add-goal: wall-o14-sealed-dirty-composition
- Intent: A mission admitted on a human-sealed dirty baseline composes with delegate authorization (HIW-O14)
- Origin: main
- Next: Design the composition (derived committed point with path-disjointness, or expected-tree worktrees) plus the provenance-backed diagnosis; the interim generic refusal is not the end state.

### add-goal: wall-o15-head-accounting
- Intent: HEAD movement during a turn is accounted: only certified integrations advance committed HEAD; staged bytes cannot ship unseen (HIW-O15, CRITICAL)
- Origin: main
- Next: Design the HEAD-accounting obligation; predates slice 7 (recorded at its round-15 review).

### add-goal: wall-o16-host-repo-fence
- Intent: The host is fenced at the whole repository in nested checkouts exactly as delegates are (HIW-O16)
- Origin: main
- Next: Design the host-side repository fence together with wall-o15 (one snapshot-scope design owns both).
- BlockedBy: wall-o15-head-accounting

### add-goal: wall-o19-recovery-ladder
- Intent: Wall violations recover on a ladder: the runner auto-restores mechanical cases; a human is asked only for adoption, no verifiable restore, or repeat offense (D117, Wido's ruling)
- Origin: main
- Next: Design per D117 from plans/recovery-ladder-design-draft.md; the resolution engine gains a runner-identity tier-2 path.

### add-goal: wall-o8-verbatim-interim
- Intent: The wall's verbatim interim rule lands (HIW-O8)
- Origin: main
- Next: Implement per the wall design's O8 row through the normal loop.

### add-goal: wall-o9-extractor-floor
- Intent: The extractor delegation floor lands (HIW-O9); its kit dependency landed at 2af908b
- Origin: main
- Next: Implement per the wall design's O9 row; the kit authority handoff it needed is live.

### add-goal: wall-o10-evidence-durability
- Intent: Wall evidence durability incl. DropAnchors semantics (HIW-O10)
- Origin: main
- Next: Implement per the wall design's O10 row; align with the go-production-grade durability migration.

### add-goal: wall-o11-legacy-state-refusal
- Intent: Pre-wall mission state refuses resume with the named error; no migration path (HIW-O11)
- Origin: main
- Next: Implement per the wall design's O11 row (an independent refusal, not a verification of other rows).

## amend-goal entries (reviewed semantic changes to legacy goals)

### amend-goal: host-implementer-wall
- state: parked
- parked-by: operator
- parked-at: EPOCH
- parked-because: Umbrella over the wall's open rows; the operator unparks and concludes when the wall-o* goals finish (parked goals never auto-conclude).
- next: Unpark and conclude when every wall-o* goal is done; rows O12/O17/O18 are READY_FOR_RUNTIME awaiting VM seals.
- blockedBy: wall-o13-acceptance-write, wall-o14-sealed-dirty-composition, wall-o15-head-accounting, wall-o16-host-repo-fence, wall-o19-recovery-ladder, wall-o8-verbatim-interim, wall-o9-extractor-floor, wall-o10-evidence-durability, wall-o11-legacy-state-refusal

### amend-goal: genesis-authority-design
- state: parked
- parked-by: operator
- parked-at: EPOCH
- parked-because: Operator assignment 2026-08-19 — hand-assigned to the second machine (in flight there); parked as the mechanical anti-duplication guard until that machine can claim it; unpark-and-claim is its first act on gaining the verbs.
- arc: genesis-authority

### amend-goal: provision-genesis-authority
- state: parked
- parked-by: operator
- parked-at: EPOCH
- parked-because: Same operator assignment as genesis-authority-design; one arc, one claimant, when the second machine claims.
- arc: genesis-authority

### amend-goal: critique-stop-rule
- arc: covenant-patience
- next: Design the critique-loop stop mechanism adopting plans/patience-attempts.md's two tiers (no-gain rounds + absolute failsafe), per D114's addenda; one patience concept across fixtures, missions, and review loops. The failsafe round number is DECLARED AT LOOP START as standing policy and enforced by the harness, never set mid-loop by an agent's judgment (Wido's instruction 2026-08-19, from the backlog-git-sync loop where the declaration only happened at round 10 because an agent remembered the rulings — D119 records the lesson).

### amend-goal: executable-covenant
- arc: covenant-patience
- next: Build battery.sh (one entrypoint, verdict file) and the critique-round driver carrying the arc's stop mechanism; designed together with critique-stop-rule (D114). The driver REFUSES to start a critique loop without a declared failsafe round and stops the loop itself when a tier fires — the mechanism runs without any particular agent (Wido's instruction 2026-08-19).

### amend-goal: runtime-install-execution
- next: Implementation-first behind fixtures per D81 (the ruling that unblocked it): run the throwaway wire probe, then build against stubs with fixtures pinning each contract; seeds in plans/ric-critique-r1..r6.

### amend-goal: lease-acquire-atomicity
- next: KI-38: one flock over lease classification-and-removal and marker-and-record publication, plus a two-process witness; its wait (the wall landing) is satisfied.

### amend-goal: goal-ledger-ergonomics
- state: parked
- parked-by: operator
- parked-at: EPOCH
- parked-because: Superseded-as-park — the backlog-git-sync format delivers its core (edit verb, direct conclusion, prose caps removed); the residue (cap policy audit) folds into agent-ease-assessment.

### add-goal: idle-watchdog
- Intent: A machine with open delegated work is never silently idle: an OS-scheduled steward detects open-work-with-no-live-worker and revives the configured agent runtime, receipting and notifying the operator every time (D121)
- Origin: main
- Next: Design per D121's charter: the open-work predicate reads the goal ledger and transaction journal; worker liveness uses the shipped process identity (ticks+bootId); revival launches through the adapter seam so it is agent-agnostic (Claude, codex, Devin alike); every revival writes a receipt and notifies the operator; the interim session-level cron guard from D121 retires when this lands. Wido's ruling 2026-08-20: a ten-hour silent stall is inexcusable — this must be machinery, never agent discipline.

### add-goal: backlog-local-promotion
- Intent: A local-mode (remote-less) goal ledger can join a fleet: the promotion protocol with its full case table (absent, equal, remote-ancestor, local-ancestor, divergent), crash ordering, and fixtures
- Origin: main
- Next: Design the promotion protocol descoped from backlog-git-sync round 5; until it lands, local mode is terminal and its banner names this goal.
