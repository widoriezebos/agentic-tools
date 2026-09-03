# Human Approval for Execution — design (goal human-approval-for-execution, revision 2)

Author m0 (Fable lane, job hae-design-r1c), 2026-09-03; revision 2 by the
fold job hae-fold the same day, folding all eight material findings of
the one review (records/human-approval/hae-review.md, HAE-R1-*). Tier 3
under R-54-m1: this design, one review, one fold, one closing review,
then build and one code review; R-60-m1's material stop criterion
governs the reviews, and disputed-but-non-material points become named
test obligations (§17), never a new round. Wido's rule (verbatim on the
goal record): "everybody can get anything on the backlog, but only the
human (I) can approve it for execution. So the state ready for impl can
only be set by me". The goal's DONE line is the acceptance contract (§13).

## 0. Shape summary (one paragraph)

One new state, `approved`, sits between `queued` and `claimed` in the
synced per-goal ledger; `queued` keeps meaning "on the backlog, awaiting
the human's word" and a draft stays a file outside the ledger. Only the
human moves a goal into `approved`, through `goal approve`, whose gate is
the enrolled-terminal proof (or the recorded relayed word, with its
expiry and its conceded limit) that `goal resume` and `goal set-obligation`
already require. The approval is EVIDENCE, not a flag: the `Approved`
record names who, when, at which revision and operation, under which
proof class, and carries a digest of the exact intent and budget the
human approved; a file whose intent or budget no longer matches that
digest does not parse. The approval carries the complete budget tuple as
one act (a named box `--budget small|big` or the four limits), runs the
over-norm strict-form gate once, and is the ONLY way a budget reaches
unclaimed work; changing an approved goal's budget or intent afterwards
is itself a human proof-bearing act (re-approve, resume, set-budget) or
is refused. Every path that creates a claimed revision (nine, enumerated
in §3) either requires a standing, unexpired `Approved` record or is a
human proof-bearing act that writes one. A relayed approval expires at
the earliest of its review date, the R-32-m1 horizon, and this
machine's first enrolled terminal; expiry is a read-time predicate
against one named observation (the transaction's or projection's
instant plus the enrollment record), never a state, and re-approval is
`goal approve` again. The frontier, the idle verdict, the steward's
backlog judgment, `goal next` and the debt metric count only approved
goals as claimable. `goal unapprove` withdraws approval (parking a
running claim through the existing human park). The existing backlog is
grandfathered by one recorded human act, the sweep, bound to the listing
the human saw; a proven sweep may later ratify relayed records; the four
machines rebuild first, the sweep lands last.

## 1. What exists and is reused (traced)

1. The live ledger is the synced per-goal world, not the legacy file.
   This checkout has no `plans/goals.md` and carries 110 goal files under
   `plans/goals/` plus the root record; `converted()`
   (cmd/metasystem/goal.go:323-329) routes every verb to the sync engine.
   The four states are `queued|claimed|parked|done`, closed at
   internal/goal/file.go:189-202 and refused by name at :263-264; the
   file grammar is `ParseFile` (:206-372) and `RenderFile` (:693-807);
   the root record is `RootRecord` (root.go:18-30), parsed by the closed
   field switch at :219-248 and rendered at :252-270. The brief's seam
   citations for the state machine and the claim gate name the legacy
   ledger (goal.go:230-233, :275-281, :286-292; goalverbs.go:405-412):
   that world has no claim verb and this design leaves it byte-identical
   (§16 records the mapping as a brief gap).
2. The claim path is `Claim`/`claimRequest` (verbs.go:404-482):
   `StateQueued` is the only admitted source (:452-453), the budget comes
   from the flags or the stored tuple (:463, `budgetForClaim` :163-174),
   and the over-norm gate runs at every claim (:467, `goalNormApproval`
   norm.go:98-132). `bindClaim` (:113-126) is the one constructor of a
   claimed revision: it writes `Claimed`, `StopCapability`, clears the
   fence and the obligation, and needs a positive claim epoch.
3. **Every writer of a claimed revision (the enumeration the fold owns,
   HAE-R1-CLAIM-WRITERS).** Grepping the non-test tree for `bindClaim(`,
   `State = StateClaimed` and `State: StateClaimed` yields exactly nine
   sites: (a) `claimRequest` verbs.go:471-475; (b) `setBudgetRequest`
   :535-541, which on a claimed goal rebinds the claim to the fresh
   revision (a fresh claimed revision without a state change);
   (c) the reopen into-claimed-arc branch :1011-1016; (d) `stealRequest`
   :1256; (e) `openClaimRequest` :1318-1323; (f) `claimArcRequest`
   :1591-1595; (g) `setArcRequest`'s caller-owned-claimed-destination arm
   :2057-2063, which auto-claims a queued or parked member joining the
   caller's claimed arc; (h) the reconcile set-arc row's twin of (g),
   reconcilepub.go:509-515; (i) `resumeRequest` stop.go:405. Recovery
   replays (a), (b), (c), (e), (f), (g) through the same constructors
   (recover.go:239-264, :296-297, :327-328); (h) and (i) re-run from
   their entry points (:346). Revision 1 named (a), (c), (d), (e), (f),
   (i) and replay; it missed (b), (g) and (h). §3 gates all nine.
4. **Every writer that returns a goal to `queued`** (the rows the
   transition table must close, HAE-R1-TRANSITION-CLOSURE): release
   verbs.go:663, unpark :911, reopen :981, arc release :1651, arc unpark
   :1785, detach :1850, set-arc source-detach :2005, set-arc empty
   destination :2023, set-arc mixed destination :2067; the reconcile
   twins unpark reconcilepub.go:292, detach :427, set-arc :467, :481,
   :519. Park writers (verbs.go:857, :1722, reconcilepub.go:269) and done
   writers (verbs.go:768, reconcilepub.go:317, split.go:288) leave every
   other record in place except the claim binding. The hand-edit state
   grammar is reconcilemap.go:252-274 (park from queued or claimed,
   unpark to queued, done; every other change refuses at :269-270); the
   split parent switch is split.go:313-330.
5. Human authority today has two grades. The weak one is a name:
   `r.Actor.Human != ""` from `--by`, which `syncReq` copies from the
   flag unverified (goalsync_mutations.go:53-58; set-pin verbs.go:1885-1888,
   steal :1185, park/done/edit/set-budget rows, reconcile reconcilepub.go:30-32).
   The strong one is process ancestry: `humanauthority.Prove`
   (authority.go:478-559) walks from the command's parent to the enrolled
   terminal and returns `AGENT_IN_AUTHORITY_CHAIN` on any adapter-signed
   ancestor (:533-538); `set-obligation` (verbs.go:553-557) and `resume`
   (stop.go:355-358) refuse without a proof that
   `AuthorizesSetObligation`/`AuthorizesResume` (authority.go:117-137).
   `lease.Classify` (internal/lease/classify.go:308-392) answers HUMAN for
   a terminal-holding caller with no recognised ancestor (:368-370);
   `syncReq` uses it only to derive the claim epoch (goalsync_mutations.go:59-69).
6. **The mutations that can change what the human approved
   (HAE-R1-APPROVED-PAYLOAD-MUTATION).** Intent is written by `editRequest`
   verbs.go:1113-1115 (gated by the weak name at :1093-1098) and by the
   reconcile edit row reconcilepub.go:381-383 (the mapping at
   reconcilemap.go:288-292). Budget is written by `claimRequest` :472,
   `setBudgetRequest` :532 (agent on its own claim, or any `--by` name,
   :511-519), `claimArcRequest` :1592, `openClaimRequest` :1320,
   `resumeRequest` stop.go:399; a hand-edited budget refuses at
   reconcilemap.go:279-282. Recovery replays `edit` and `set-budget` as an
   AGENT: `actorFromEntry` (recover.go:359-361) drops the stored `by`
   argument, so replayed `steal` and `set-pin` already refuse for want of
   a human (:320-324, :329-332).
7. The relayed form is `ProveOrTemporaryGoalAuthority` (authority.go:228-237):
   enrolled ancestry wins; on ANY ancestry failure `--temporary-human-word`
   plus `--review-by` mint a `TEMPORARY_HUMAN_WORD` proof (:199-222)
   validated for three words minimum, a non-past date, and the R-32-m1
   horizon (:159-180; governance/types.go:96-97, ruling R-32-m1, horizon
   2026-09-06). Nothing in that path reads the terminal enrollment.
   The enrollment record is `Enrollment` (authority.go:48-56, with
   `EnrolledAt` and `Generation`), written by `Enroll` (:434-473) to
   `artifacts/agents/authority/human-terminal.json` (:375-377) and read
   by `ReadEnrollment` (:381-400), which refuses an incomplete record.
   The engine records the relay tuple on the history line
   (`recordTemporaryRelay` file.go:144-149), allows one relayed act per
   goal per ruling (`repeatedRelayedActError` :174-181), keeps relayed
   lines of pruned goals on the root record (verbs.go:1409-1417 through
   `recordedRelayedAct` file.go:183-186, today resume and set-obligation
   only), and concedes the limit in code: "the relay records the supplied
   words but cannot verify who supplied them" (authority.go:114-116).
   Proofs are stored per operation by `recordProof` (:584-607).
8. The over-norm strict form: `goalNormApproval` (norm.go:98-132) needs
   `--approved-ref` naming a rulings row or a `human:` history operation
   whose reason carries `goal=<id> minutes=<n> goalRevision=<r>`
   (`RecordedNormApproval` :53-91), at the goal's pre-touch revision
   (:122-124); it publishes `NormApproval` (file.go:59-64), and
   `ValidateTree` proves minutes cover the budget (validate.go:206-209).
   Because the budget act bumps the revision, every claim after a
   set-budget needed a fresh token (R-59-m1, R-62-m1, R-63-m1 are the
   specimens). The norm is `goal.norm.job-minutes` (1440 per the brief).
9. **Clocks (HAE-R1-FRONTIER-CLOCK).** Commands stamp `r.Now` from
   `goalCommandNow` (cmd/metasystem/goal.go:21-32: the wall clock unless
   the root authorizes a fixture clock). `Project(e, fetchFirst, now)`
   (project.go:49-97) uses `now` only for the staleness banner and stores
   no instant on `Projection` (:26-31); `ProjectAt(root, tip)`
   (attention.go:48-54) takes none; `Next` (project.go:490-524) reads no
   clock. Callers: `ReadClaimableBudgetedWork(root, now)` :272-331 (the
   turn verdict passes `s.now()`, turnverdict.go:181 via goalverbs.go:65-70;
   the steward passes `time.Now()`, openwork.go:44, :96), `nextSynced`
   goal.go:454-469, `listSynced` :339 (`time.Now()`), the turn verdict's
   own projections turnverdict.go:546, :610, :660 (`s.now()`), and the
   steward's attention stage, which holds a tick instant `at`
   (ledgerattention.go:220-267) but projects through `ProjectAt` without it.
10. **Consumers of "claimable" and of the state set (HAE-R1-STATE-CONSUMERS).**
    The non-test switches on goal state outside the engine's own verbs:
    `Next` project.go:495-521; `ReadClaimableBudgetedWork` :315-325
    (Queued = every queued goal, Claimable = Ready with a valid budget);
    the idle block turnverdict.go:246-265; the queued-frontier digest
    :604-648 (literal `"queued"` at :621); the steward's backlog judgment
    openwork.go:51-72; the steward's ledger-attention snapshot
    ledgerattention.go:147-174 (Ready from Next, Queue and Pinned from
    `StateQueued` only) and its narration narrate.go:162-168, :279-291;
    the debt metric compute.go:603-654 (ages parked and queued only);
    `goal list` sections and JSON goal.go:384-393; `goal next` :469-481;
    the goal-free exclusivity validate.go:285-294 and `declare-free`
    verbs.go:1157-1162; the hand-edit grammar reconcilemap.go:95-97,
    :252-274; split members and parent split.go:262-271, :313-330; the
    manifest's assignable states manifest.go:97-101 (queued or parked
    only); the legacy migration migrate.go:348, :388, :426-427. Claimed-only
    readers that need no change but are named so the build drops nothing:
    dispatch admission.go:59, :78, servinggoal.go:35, budget.go:242,
    stop.go:54, :290; steward health.go:707, :835, delivery.go:63;
    evidence/gc.go:504, :519. `internal/report` has no goal-state switch
    (the one `"queued"` at report/scanjobs.go:229 is a job status).
11. Hand edits and replay: `mapOneChange` (reconcilemap.go:205-338)
    refuses generated fields and unknown state changes; hand-created
    goals may not carry a budget or a pin (:98-103); `requestForEntry`
    (recover.go:209-347) replays journal entries through the verb
    constructors, and verbs needing a live proof re-run from their own
    entry point (:346).
12. Drafts: `plans/goals-drafts/` free-form files, promotion is `goal open`
    (docs/backlog-mechanism.md:58-65, R-2). Standing budget boxes:
    R-45-m0b, small 4h/10/240m/1 and big 8h/10/240m/1. The law text that
    says the budget arrives at claim (docs/backlog-mechanism.md:7-13) is
    rewritten by this design (§13).
13. The backlog today (this worktree's tree): 99 queued (95 with a budget,
    4 without; 30 human-origin), 3 claimed (dispatch-cap-necessity on m1b,
    fleet-slack-channel on m0b, path-class-manifest on m1; all budgeted),
    7 parked (2 budgeted). The fleet is m0, m0b, m1, m1b (R-61-m1).

## 2. The state, the record, the invariants, the complete transition table

**States, closed, five:** `queued | approved | claimed | parked | done`.
`StateApproved = "approved"` joins file.go:189-202; the parse message at
:264 names all five. No other state is added; "expired" and "awaiting"
are read-time classifications (§5, §8), not states (R-11).

**The `Approved` record (HAE-R1-APPROVAL-RECORD-BINDING).** One new
record line on the goal file, rendered after `NormApproval:` and before
`Sliced:` (RenderFile order) and parsed by the closed-key grammar
(`parseKVRecord`, file.go:598-629):

```
- Approved: by=human:<name> at=<RFC3339> revision=<n> opid=<opid> authority=proven|relayed digest=<sha256-hex> [reviewBy=<YYYY-MM-DD>]
```

Each key is bound to the approval EVENT, and `ParseFile` refuses the
file by name when any binding fails (the pattern of
`ValidateClaimRevision`, file.go:378-394):

- `by` starts with `human:` and names a non-empty human; `at` is RFC3339.
- `revision` is a positive integer at or below the file's `Revision`
  and at or below `len(History)`; the history event `History[revision-1]`
  has `At == at`, `Opid == opid`, `Actor == by`, and a verb in the closed
  set of APPROVAL-BEARING VERBS `{approve, resume, set-budget}` (§4).
  The proof file `artifacts/agents/authority/proofs/<opid>.json` with the
  matching action is the off-ledger half of the same event.
- `authority` is exactly `proven` or `relayed`. `relayed` ⇒ `reviewBy`
  is present, parses as a date, and the bound history event carries
  `authorityOutcome=TEMPORARY_HUMAN_WORD` with `authorityReviewBy` equal
  to `reviewBy`; `proven` ⇒ no `reviewBy` and the bound event carries no
  `authorityOutcome`.
- `digest` is 64 hex characters equal to `ApprovalDigest(f.Intent, *f.Budget)`,
  defined as sha256 over the bytes `"intent=" + Intent + "\n" + "budget=" +
  <the exact Budget line RenderFile writes at file.go:722-723, without
  the "- Budget: " prefix> + "\n"`; `Budget` must therefore be present
  and valid. This is the binding of HAE-R1-APPROVED-PAYLOAD-MUTATION: a
  file whose intent or budget differs from what was approved does not
  parse, whatever path wrote it.
- `State: approved` ⇒ `Approved` present; `State: queued` ⇒ no `Approved`.
  `Approved` may stand on `claimed`, `parked` and `done` files (the audit
  line of who admitted the work). A `claimed` file WITHOUT it is
  tolerated by `ParseFile` exactly as a revisionless claim is tolerated
  (file.go:81-83); the tree-level rule below bounds that tolerance.

`Approved` is a generated field: a hand edit that changes it refuses
(reconcilemap.go:206-248 gains the row, same shape as `NormApproval`);
a hand-created goal carrying it refuses (:104-106 gains it).

**The root-record cutover mark (the discriminator HAE-R1-CLAIM-WRITERS
asks for).** One new root-record line, rendered after `Goal-free:`
(root.go:268-270) and parsed at :239-247:

```
- ApprovalGate: since=<RFC3339> opid=<opid>
```

It is written ONCE, by the first approval-bearing act of any kind that
lands on a tree without it (`approve`, the sweep, `resume`, `set-budget`
under proof), in that act's own transaction, with that act's stamp and
opid; it is never changed afterwards (the engine refuses to rewrite it;
the root record has no hand-edit grammar, reconcilemap.go:76-77). It is
the persistent fact that separates a tolerated pre-gate claim from a
bypass.

**Tree invariants** (`ValidateTree`, validate.go): (1) the at-rest claim
rule: for every live `claimed` goal without an `Approved` record, when
the root carries `ApprovalGate`, `Claimed.At` must be EARLIER than
`ApprovalGate.since`; otherwise the problem reads "claim at <at> landed
after the approval gate armed at <since> without an Approved record".
Before the gate arms the at-rest rule is inactive by construction and
the verb-level gate (§3) is the whole defense; that is the §10 window,
now stated as such. (2) Goal-free exclusivity (validate.go:285-294) and
`declare-free` (verbs.go:1157-1162) treat `approved` as they treat
`queued`. (3) The NormApproval-covers-budget rule (:206-209) is unchanged.
The root-record and placement rules are unchanged. Zero-current legality
is a legacy-ledger rule (goal.go:286-292) and does not apply to the
synced world; its synced twin is exactly the exclusivity rule.

**Transitions, the complete table (HAE-R1-TRANSITION-CLOSURE).** Every
verb that can change state or the approval-bearing fields is listed;
"record" means the `Approved` record, "tuple" means `Budget` plus
`NormApproval`. The helper `restingState(f)` is the ONE rule for a goal
that stops being claimed or parked: `approved` when the record stands,
else `queued`; it replaces every `State = StateQueued` write listed in
§1.4.

| from → to | verb | who | record and tuple |
| --- | --- | --- | --- |
| (none) → queued | open, reconcile open, split member, migrate | anyone | none |
| queued → approved | approve | human (proof, §4, §5) | writes tuple and record |
| approved → approved | approve (re-approval, §5) | human (proof) | replaces tuple and record; NothingToDo when nothing changes and the standing record is proven and unexpired |
| approved → queued | unapprove | human (proof) | clears record and tuple |
| approved → claimed | claim, claim --arc | agent | record stands and is unexpired; tuple taken from the record, never from flags |
| approved → claimed | set-arc / reconcile set-arc into the caller's own claimed arc | the arc's claimant, or a human by hand edit | same gate as claim; a `queued` member joining lands `queued` (no auto-claim) |
| approved → parked | park, park --arc, hand park | as queued today (human-origin gate stands) | record and tuple stand |
| approved → done | done, hand done | as queued today | record and tuple stand on the archive file |
| approved → done | split (parent) | as queued today (split.go:313-330 gains the `approved` arm, same rule as queued) | record stands on the archive; members open `queued` |
| approved → approved | edit next-step/labels/blockers, set-pin, set-arc (mixed or empty destination), detach | as queued today | unchanged (the digest does not cover these fields) |
| any with a record → (refused) | edit --intent, hand-edited Intent | every actor | refused: "the human approved this intent; unapprove, edit, approve" |
| queued or approved → (refused) | set-budget | every actor | refused: "budgets on unclaimed work are the human's approval act (goal approve)" |
| claimed → claimed (fresh revision) | set-budget | human (proof, §4) | replaces tuple, rewrites the record with a fresh digest |
| claimed → claimed (fresh revision) | resume | human (proof) | replaces tuple, rewrites the record |
| claimed → claimed | approve (re-ratification only, §5) | human (proof) | record rewritten `authority=proven` with the same tuple; claim binding untouched; budget flags refuse |
| claimed → claimed (new pair) | steal | human `--by` as today | each moved member must carry a record; record and tuple unchanged |
| claimed → claimed | set-obligation, slice-start, breach-stop | as today | unchanged |
| claimed → restingState | release, release --arc, detach, set-arc source-detach, hand detach, hand set-arc | as today | unchanged: an agent act never withdraws or manufactures approval |
| claimed → parked | park, park --arc, hand park | as today | record and tuple stand |
| claimed → parked | unapprove | human (proof) | parks with `because=approval revoked: <text>`, displaced pair recorded; clears record and tuple |
| claimed → done | done, hand done, split (own claimant or human) | as today | record stands on the archive |
| parked → restingState | unpark, unpark --arc, hand unpark (target state `queued` or `approved` both map to unpark; the engine decides by the record), set-arc of a parked member into an empty or mixed arc | as today (human-park gate stands) | unchanged |
| parked → parked | unapprove | human (proof) | clears record and tuple; the pause stands |
| parked → done | done, split | human | record stands on the archive |
| done → queued | reopen | as today | clears record and tuple: a reopened goal needs a fresh word |
| done → done | prune (root) | as today | relayed `approve`/`unapprove`/`set-budget` lines of pruned goals are retained on the root record (`recordedRelayedAct` file.go:183-186 gains the three verbs) |
| queued → claimed | (none) | — | retired: claim on queued refuses `APPROVAL_REQUIRED`; `open --claim` (verbs.go:1267-1340) and the reopen into-claimed-arc branch (:995-1019) are removed and their replays refuse by name (recover.go:239-244, :296-297: "retired; close this entry by hand"); the set-arc auto-claim of a `queued` member (verbs.go:2033-2065, reconcilepub.go:488-517) is removed |
| root | declare-free | as today | refuses while any goal is queued, approved or claimed |

Rows are exhaustive over the verbs in verbs.go, stop.go, split.go,
reconcilepub.go and the new verbs; the manifest and the migration write
`queued`, `parked`, `claimed`, `done` only and refuse `approved` by their
existing closed switches (manifest.go:97-101; migrate.go imports the
legacy ledger, which has no approval).

## 3. The claim gate at every claim-creating path (HAE-R1-CLAIM-WRITERS)

The ONE gate is `requireApprovedForClaim(t, f, horizon)` in the engine
(`horizon` is §8's `ApprovalHorizon`), returning one of:

```
APPROVAL_REQUIRED: goal <id> is <state> and not approved for execution; only the human approves it (goal approve --id <id> --by <name> --budget small|big …) — this <verb> is refused
APPROVAL_EXPIRED: goal <id> was approved by a relayed word (review by <date>, approved <at>); that approval no longer admits new work here because <the review date has passed | this machine's terminal was enrolled at <enrolledAt> (R-29-m1: relayed words end at the first enrolled session)>; a fresh approval is required (goal approve at the enrolled terminal)
```

and, when it passes, the tuple the claim takes: `*f.Budget` exactly
(the digest guarantees it is the approved one). It runs at these sites,
one per writer in §1.3:

| writer | rule in revision 2 |
| --- | --- |
| (a) `claimRequest` verbs.go:452-453 | source state must be `approved` (not `queued`); then the gate; budget flags on `goal claim` refuse ("the budget was bound by the human's approval; claim carries no tuple"), `--approved-ref` refuses as not applicable; `budgetForClaim` (:163-174) is no longer called by claim |
| (f) `claimArcRequest` :1572-1574 | per member: `approved` passes the gate and is claimed; `parked` and own-claimed members ride along as today; a `queued` member refuses the WHOLE cascade naming the member (fail closed, not a silent skip) |
| (g) `setArcRequest` ownClaimed arm :2033-2065 and (h) its reconcile twin reconcilepub.go:488-517 | the arm fires only for a member whose state is `approved` and passes the gate; a `queued` member lands `queued` by the default arm; a `parked` member (human) lands `restingState`; the budget checks at :2050-2055 and :502-507 are replaced by the gate (the approved tuple is within the norm or carries its `NormApproval`) |
| (d) `stealRequest` :1226-1243 | every member the steal moves must carry an `Approved` record (a pre-gate claim after the sweep has one; before the sweep the steal refuses `APPROVAL_REQUIRED` and waits for it); expiry is NOT evaluated: steal continues work already started |
| (b) `setBudgetRequest` :535-541 and (i) `resumeRequest` stop.go:405 | human proof-bearing acts that WRITE the record (§4); they need no standing record (they may ratify a pre-gate claim) and are the only writers that both create a claimed revision and mint approval |
| (c) reopen arc-join :1011-1016 and (e) `openClaimRequest` :1318-1323 | removed; replays refuse by name |
| recovery replay recover.go:245-258 (`claim`), :259-264 (`set-budget`), :327-328 (`set-arc`) | `claim` and `set-arc` replay through the constructors and hit the same gate; `set-budget` needs a live proof and moves to the :346 arm ("re-runs from its entry point"); pre-cutover journal entries of `set-budget`, `open-claim` and reopen-into-arc refuse by name and close by hand (§10 step 5) |

**Boundary:** the gate lives in the engine's mutation callbacks, so it
binds the CLI, the arc cascade, the reconcile rows, recovery replay and
any future caller alike; the at-rest rule (§2) catches a forged path
after the gate arms; before it arms, the cutover order (§10) carries the
window.

**Over-norm at claim (no duplication).** Claim, arc claim, the set-arc
auto-claim and steal call `normApprovalForApproved(f)`: within the norm
→ nothing; over the norm → the stored `NormApproval` must exist and
cover `ReservedJobMinutesLimit` (the rule `ValidateTree` already
enforces, validate.go:206-209), else `GOAL_NORM_REFUSED` naming
re-approval by the human. No fresh token at claim: the approval act is
the human word the token expressed, and the revision churn of R-59/R-62/
R-63 ends.

## 4. Approval binds intent and budget; the human-only verbs (HAE-R1-APPROVED-PAYLOAD-MUTATION)

```
metasystem goal approve --id <id> [--id <id> …] --by <name>
    [--budget small|big | --elapsed-limit … --attempt-limit … --reserved-job-minutes-limit … --active-job-limit …]
    [--approved-ref <ref>] [--temporary-human-word "<words>" --review-by <date>] [--lineage …]
metasystem goal unapprove --id <id> --by <name> --because "<text>" [relay flags]
metasystem goal approve --sweep [--confirm <listing-sha256>] --by <name> [relay flags]
metasystem goal set-budget --id <id> --by <name> --elapsed-limit … --attempt-limit … --reserved-job-minutes-limit … --active-job-limit … [--approved-ref <ref>] [relay flags]
```

**The binding, mechanically.** The human approves an intent and a
tuple; the record's `digest` (§2) covers exactly those two. Consequences,
each one rule:

1. **Intent edits refuse on any goal carrying a record**, for every
   actor and every path: `editRequest` (verbs.go:1113) refuses when
   `f.Approved != nil && fields.Intent != nil` with "the human approved
   this intent; unapprove, edit, then approve"; `mapOneChange`
   (reconcilemap.go:288-292) refuses a changed `Intent` when the base
   carries a record, at mapping time, before any replay. Next-step,
   labels, blockers, arc and pin edits keep their rows (the digest does
   not cover them). The human path to a changed intent is `unapprove`
   → `edit` → `approve` (on a claimed goal: unapprove parks it; unpark
   lands `queued`; approve; claim again).
2. **`set-budget` on `queued` or `approved` refuses for everyone**:
   "budgets on unclaimed work are the human's approval act: goal approve
   --id <id> --budget …". This retires the seat-budgets-queued-goals
   practice R-44/R-45 licensed (the sweep ratifies what that practice
   produced, §9) and the budget-then-claim shape of R-2.
3. **`set-budget` on `claimed` is a human proof-bearing act** with the
   shape of `resume`: `SetBudget(r, id, budget, proof)` refuses unless
   `r.Actor.Human != "" && proof != nil && proof.AuthorizesGoalApproval(root)`;
   it admits `claimed` only (a parked goal's tuple changes through
   unpark and approve); runs `goalNormApproval` at the pre-touch
   revision as today; writes the tuple; rebinds the claim to the fresh
   revision (:535-541, unchanged); rewrites the `Approved` record at that
   revision with `authority=<the proof's class>` and the fresh digest;
   records the relay tuple on the history line when relayed. An
   identical tuple with an identical `NormApproval` is `NothingToDo`
   whatever the standing record's authority. An agent `set-budget` on
   its own claim refuses: "the budget was bound by the human's approval;
   a raise is the human's act (goal set-budget --by … from the enrolled
   terminal, or with the relayed word)". This is the strong-authority
   route the finding names; §14 D7 records what it supersedes.
4. **`resume`** (stop.go:365-412) keeps its shape and, in the same
   transaction that installs the fresh tuple, rewrites the `Approved`
   record at the fresh revision under its own proof class (a resume IS a
   human budget act with proof).
5. **`approve` itself**: on `queued` the budget (box or four limits) is
   REQUIRED and the verb writes tuple and record; on `approved` the
   budget is OPTIONAL (absent = re-ratify with the standing tuple,
   present = re-bind with the new one); on `claimed` budget flags REFUSE
   ("a claimed goal's tuple changes through set-budget") and the verb
   only re-ratifies a relayed record to `proven` with the claim binding
   untouched. Repeated `--id` applies one budget to every target and
   refuses if any target is `claimed`. `--budget small` binds R-45-m0b's
   4h/10/240m/1, `--budget big` 8h/10/240m/1, the four limits bind exactly
   what the human typed (`budgetTuple(true)`, goalsync_mutations.go:166-181,
   reused); box and limits together refuse. A budget stored earlier on a
   queued goal (a pre-cutover shape) is replaced, never adopted. Each
   target runs `goalNormApproval` at its own pre-touch revision; over
   the norm needs `--approved-ref` naming the strict token; the
   resulting `NormApproval` is published beside the budget. That is the
   ONE strict-form check.
6. **The proof gate**, the exact shape of `SetObligation`
   (verbs.go:553-557): `Approve`, `Unapprove`, the sweep and `SetBudget`
   refuse unless `r.Actor.Human != "" && proof != nil && proof.AuthorizesGoalApproval(root)`,
   where `AuthorizesGoalApproval` is `ValidFor || temporaryValidFor`
   (authority.go:117-119 pattern) and `TemporaryGoalAuthorityFor` reports
   the relayed class; `RecordGoalApprovalProof(root, opid, action, proof)`
   stores the proof with action `goal approve`, `goal unapprove`,
   `goal approve --sweep` or `goal set-budget` (:584-607 pattern). The
   command layer mirrors `runGoalResumeWithAuthority`
   (goalsync_mutations.go:354-418): proof first (`ProveOrTemporaryGoalAuthority`,
   real wall clock), then `syncReq`, then publish, then record;
   `parseSyncFlags` (:125-128) admits the relay flags for `approve`,
   `unapprove` and `set-budget` as it does for `resume`. Consequences: a
   caller whose ancestry crosses any adapter-signed process gets
   `AGENT_IN_AUTHORITY_CHAIN` and refuses; a caller from an unenrolled
   terminal without relay flags gets "no readable terminal enrollment"
   and refuses; `--by` alone authorizes nothing on these four verbs.
   `lease.Classify`'s HUMAN class is not the gate. Recovery never
   replays them (recover.go:346 arm).
7. **Unapprove.** On `approved`: → `queued`, clears `Approved`, `Budget`,
   `NormApproval`; history `unapprove … reason=<because>`. On `claimed`:
   → `parked` with `Parked: by=human:<name> … displaced=<pair>@<at>
   because=approval revoked: <text>`, claim binding cleared through
   `clearClaimBinding` (verbs.go:128-137, so a breach-stopped goal refuses
   and names `resume`), record and tuple cleared. "At the next safe
   point" is what a human park already means: dispatch admission needs a
   claimed revision, so no new reservation lands, running jobs run out,
   and the displaced pair hears it through the acknowledgment path
   (:210-286). On `parked` with a record: clears record and tuple; the
   pause stands.

## 5. The relayed form: expiry and re-approval (HAE-R1-RELAY-ENROLLMENT-EXPIRY, HAE-R1-REAPPROVAL-TRANSITION)

Same flags, same validator, same horizon as resume and set-obligation
(`ValidateTemporaryWordPair`, `validateTemporaryGoalAuthority`,
authority.go:143-180; horizon `governance.TemporaryGoalAuthorityHorizon`,
2026-09-06). Enrolled ancestry wins whenever provable.

**Grant-time cutoff (one owner: `humanauthority`).** `temporaryGoalProofAt`
(authority.go:206-222) gains one check before minting: when
`ReadEnrollment(root)` succeeds, the relay refuses with
`RELAY_AFTER_ENROLLMENT: this machine has an enrolled agent-free terminal
(enrolled <EnrolledAt>, generation <n>); relayed words end at the first
enrolled session (R-29-m1); run the verb from that terminal`. A missing
file is the only "no enrollment"; an unreadable or incomplete record
refuses by its own message. Because `ProveOrTemporaryGoalAuthority`
(:228-237) falls into the relay after ANY ancestry failure, this closes
the hole the finding names for every relayed verb — approve, unapprove,
the sweep, set-budget, resume, set-obligation — which is R-29-m1's own
scope ("everything done under it"), not a widening.

**The durable record**, three places, existing grammar: (1) the goal's
history line carries `authorityOutcome=TEMPORARY_HUMAN_WORD
authorityReviewBy=<date> authorityRuling=R-32-m1 temporaryHumanWord="<words>"`
(`recordTemporaryRelay`, file.go:144-149; `recordedRelayedAct` :183-186
gains `approve`, `unapprove`, `set-budget`); (2) the `Approved` record
carries `authority=relayed reviewBy=<date>`; (3) the proof file under
`artifacts/agents/authority/proofs/<opid>.json`. The command announces
the temporary state on stdout as set-obligation does
(goalsync_mutations.go:646-648).

**Read-time expiry, the predicate.** `(f *GoalFile).ApprovalExpired(h ApprovalHorizon) (expired bool, why string)`
in `internal/goal`, where `ApprovalHorizon{Now time.Time; EnrolledAt time.Time}`
is §8's observation: a `proven` record never expires; a `relayed` record
is expired when EITHER `h.Now`'s UTC calendar date is after `reviewBy`
(the record is valid through 23:59:59Z of its review date; the
comparison is date-to-date, never instant-to-midnight), OR `h.EnrolledAt`
is non-zero (this machine has an enrolled terminal, whatever its order
relative to the approval: R-29-m1 ends the departure at the first
enrolled session and re-ratifies everything done under it), OR `h.Now`'s
date is after the R-32-m1 horizon (redundant with the grant-time check,
stated so the horizon constant has one more reader that fails closed).
The earliest of the three ends the approval, which is the wording of
R-29-m1 and R-32-m1. Work already claimed is not stopped: expiry gates
the START of work (claim, arc claim, the set-arc auto-claim), which is
the gate Wido asked for; steal continues started work (§3).

**Expired is not a state (R-11).** The goal stays `approved`; the
frontier lists it under `Awaiting` with the predicate's `why` (§8); claim
refuses `APPROVAL_EXPIRED` (§3); `goal list --pretty` marks it
`(relayed, EXPIRED: <why>)`.

**Re-approval, the specified path.** `approve` on an `approved` goal is
the row "approved → approved" (§2): it ALWAYS replaces the record in
place (fresh `at`, `revision`, `opid`, `authority`, `digest`), whether the
standing record is expired or not; it never requires `unapprove` first.
`NothingToDo` only when the standing record is `proven`, unexpired, and
the tuple and `NormApproval` are unchanged. A relayed re-approval of a
goal that already used relayed `approve` under the same ruling refuses
through `repeatedRelayedActError` (file.go:174-181), as resume and
set-obligation do — so an expired RELAYED approval is renewed only at
the enrolled terminal, which is what the enrollment cutoff means. On a
`claimed` goal `approve` re-ratifies only (§4.5). Unapprove works on an
expired record exactly as on a live one.

**The honest limit, conceded.** An agent's relay cannot prove the human's
word (authority.go:114-116; enrollment law R-29-m1). The design records
the words, the date and the ruling, announces the temporary state,
refuses after the date and after enrollment; it cannot authenticate who
typed `--temporary-human-word`. Closing that forge belongs to
ledger-authentication (plans/goals/ledger-authentication.md). The fleet
channel's authenticated reply (plans/fleet-slack-channel-design.md §5,
`AUTHENTICATED_CHANNEL_WORD`) is the intended third proof class for
approve from his phone; that design's consumer list must name
`goal approve --approved-ref <opid>` when it folds, and
`AuthorizesGoalApproval` gains that branch there, not here.

## 6. Scope note on R-32-m1

R-32-m1 scoped the relayed form to exactly resume and set-obligation.
This design widens that set to `approve`, `unapprove`, the sweep and the
proof-bearing `set-budget` on the authority of the goal's DONE line
("human-only verbs with the relayed-word form"), approved for execution
by R-61-m1, and of this brief's Input 5. The closing review should
confirm the widening is Wido's word; §16 lists it.

## 7. The draft question, settled: one new state

A draft is a file in `plans/goals-drafts/` (R-2; docs/backlog-mechanism.md
:58-65): free-form, no grammar, no budget, outside the ledger. It stays
that. Anyone, machines included, may open a goal from it or directly
("everybody can get anything on the backlog"); `goal open` is unchanged
and yields `queued`. R-2's "only his word opens the goal" for big tickets
is superseded by Wido's 2026-09-03 rule: the gate moved from opening to
execution. So the ledger gains ONE state, `approved`; `queued` IS
"awaiting approval"; there is no `draft`, `pending-approval` or `expired`
ledger state (R-11): two new states would double every consumer switch
in §1.10 for a distinction the drafts directory and the expiry predicate
already carry. Split members open `queued` and are approved by the human
with one repeated `--id` act under one budget; a human-ratified split
does not carry the parent's approval to the members because members
need their own within-norm tuples, which is exactly the judgment
approval exists to record.

## 8. Consumers: the clock owner and the complete inventory (HAE-R1-FRONTIER-CLOCK, HAE-R1-STATE-CONSUMERS)

**The clock owner.** `Projection` (project.go:26-31) gains one field,
`Horizon ApprovalHorizon` (§5). `Project(e, fetchFirst, now)` fills it
from the `now` it already receives plus `humanauthority.ReadEnrollment(e.Root)`
(missing file → zero `EnrolledAt`; any other error → `Project` refuses
by that error, fail closed). `ProjectAt(root, tip)` (attention.go:48-54)
gains a `now time.Time` parameter and fills the horizon the same way;
its three callers (ledgerattention.go:225, :242, :478) pass the tick
instant they hold (`at` at :220; the recovery path at :478 passes
`time.Now().UTC()`). `Next(p, machine, labels…)` reads `p.Horizon` and
NOTHING else; it never touches the wall clock, so a test builds
`Projection{Tree, Horizon}` and asserts the buckets at a fixed instant.
Every command path keeps its existing clock: `goalCommandNow` for verbs,
`goal next` and `goal show`; `s.now()` for the turn verdict; `time.Now()`
for the steward's judgment and `goal list`. Verbs evaluate the same
predicate against `ApprovalHorizon{Now: r.Now, EnrolledAt: <read in the
mutation callback from r.Endpoint.Root>}`, so a claim and the frontier
agree on the same instant when given the same instant. `reviewBy` is
valid for its entire UTC calendar date (§5).

**`Next` (project.go:490-524).** Buckets: `Claimed` (this machine's
claims, as today); `Ready` = `approved`, unexpired, labels and pin
honoured, every blocker done; `Blocked` = `approved`, unexpired, an open
blocker; new `Awaiting` = every `queued` goal plus every `approved` goal
whose record is expired, sorted like the others, whatever its blockers.
`NextVerdict` gains `Awaiting []string`; the expiry reason is read from
`f.ApprovalExpired(p.Horizon)` by any printer that wants it.
`ReadClaimableBudgetedWork` (:315-325): `Claimable` = `Ready` (the valid
budget check stays as a belt), `Queued` counts `Awaiting` (field name
kept; its comment changes).

**The inventory, one line per consumer, what changes:**

| consumer | change |
| --- | --- |
| project.go:495-521 `Next` | the buckets above |
| project.go:315-325 `ReadClaimableBudgetedWork` | Queued = len(Awaiting) |
| turnverdict.go:246-265 idle block | code unchanged; sees only approved goals as claimable (a test proves it) |
| turnverdict.go:604-648 queued-frontier digest | covers `queued` AND `approved` (literal at :621 becomes the two states) so the once-per-change block keys on both |
| steward/openwork.go:51-72 | the `Queued > 0` line reads "N goals await the human's approval (queued or expired approval); none is claimable" |
| steward/ledgerattention.go:147-174 `snapshotLedger` | Ready from Next (approved-ready); Queue = `Awaiting` in OpenedAt order; Pinned = queued-or-approved goals pinned here |
| steward/narrate.go:162-168, :279-291 | "claimable" wording stands (it now means approved-ready); the queue line reads "the awaiting queue reordered" |
| metrics/compute.go:603-654 `computeDebt` | `approved` ages like queued: anchor `OpenedAt`, kind `approved opened-at anchor`; an expired approval is the same class (expiry is not a debt kind) |
| cmd/metasystem/goal.go:384-393 `goal list` | `--pretty` gains the section `approved` between `claimed` and `queued`, each line marked `(relayed, review by <date>)` or `(relayed, EXPIRED: <why>)`; the JSON gains the key `approved` |
| cmd/metasystem/goal.go:469-481 `goal next` | in order: `continue your claimed goal: X`; `next ready goal: X`; `all approved goals are blocked; the first is X`; `no claimable goal; N await the human's approval (first: X)`; label and empty cases as today — the awaiting line never suggests a claim |
| validate.go:285-294, verbs.go:1157-1162 | approved counts as live intent |
| reconcilemap.go:95-97, :252-274 | hand-created `approved` refuses; hand park from approved maps to park; hand unpark to `queued` or `approved` maps to unpark; every other approved transition refuses at :269-270 by name |
| split.go:313-330 | `approved` parent splits as queued does |
| manifest.go:97-101, migrate.go | unchanged; a test asserts `approved` is refused as manifest-assignable and never produced by migration |
| dispatch admission.go:59, :78, servinggoal.go:35, budget.go:242, stop.go:54, :290; steward health.go:707, :835, delivery.go:63; evidence/gc.go:504, :519 | unchanged (claimed-only readers); named so the build's caller enumeration (R-18) is complete |
| internal/report | no goal-state switch; nothing to change |
| docs/backlog-mechanism.md:7-13, :58-65; docs/glossary.md | the law: the budget arrives at approval, not at claim; the backlog holds queued and approved items; `approved`, `Approved`, `ApprovalGate`, "awaiting" defined |

## 9. The grandfather sweep: one rule, one recorded act

**The rule:** every live goal that carries a valid `Budget` at sweep time
is approved by the sweep; a goal without a budget is not. Rationale the
record states: a budget on a queued goal is the seat's act under the
standing tuple delegation R-44-m0b/R-45-m0b, which Wido pre-approved;
the sweep ratifies exactly that set once and ends the delegation (§4.2).
Claimed goals gain the `Approved` record with state, claim, budget,
revision binding and stop capability untouched (`Claimed.revision` keeps
pointing at the claim event, the sweep's history line is a later
revision, `Approved.revision` points at that line, and
`ValidateClaimRevision` file.go:378-394 still holds). Parked goals with a
budget gain the record and stay parked (unpark → approved).

**The act:** `goal approve --sweep` (no `--confirm`) is a dry run: it
prints the listing — one line per goal: id, state, origin, budget,
NormApproval if any, standing `Approved` authority if any — plus
`listing-sha256=<digest over the sorted "id state budget authority"
lines>` and changes nothing. `goal approve --sweep --confirm <digest>
--by <name> [relay flags]` performs it in ONE transaction (one opid, one
commit, the multi-file shape `claim --arc` already publishes): it refuses
if the tree's listing digest differs from `--confirm`; it writes per
file the history line `approve actor=human:<name> targets=<id>
reason=sweep` and the `Approved` record for every listed goal whose
record is ABSENT or `relayed`; it skips every goal whose record is
`proven` (the sweep never replaces a terminal word); it writes one
root-record line `approve actor=human:<name> reason=sweep
listing=<digest> approved=<n> ratified=<n> skipped=<ids without budget>`
(root history is the existing declare-free/prune/ack channel,
root.go:17-30) and, if absent, the `ApprovalGate` mark (§2). A RELAYED
sweep additionally refuses when any `Approved` record exists on the tree
or when the root record already carries an `approve` line with
`authorityOutcome=TEMPORARY_HUMAN_WORD` under the same ruling (one
relayed sweep, ever); a PROVEN sweep is idempotent (a second one finds
nothing to ratify and returns `NothingToDo`). On today's tree the
listing is 95 queued + 3 claimed + 2 parked = 100 approved, 9 skipped
(4 queued, 5 parked without budget). It is not a per-goal question: Wido
reads one listing and gives one word. A relayed sweep stamps every
record `authority=relayed reviewBy=<date>`, so all 100 expire together
on that date or at his first enrolled terminal — stated on the dry run's
last line — and the proven sweep at that terminal is the one-act
re-ratification R-29-m1 asks for.

## 10. Migration for the four-machine fleet

Old binaries cannot parse the new state, the `Approved` field or the
`ApprovalGate` line (unknown field/state refuse by name, file.go:263-264,
:589-590, root.go:247), so an approved tree is unreadable to a stale
engine: every verb, `goal next`, the steward's open-work judgment
(WorkDegraded) and the turn verdict fail closed there. That is safe and
loud, and it fixes the order:

1. Land the build on main (engine, CLI, docs/backlog-mechanism.md law,
   fixtures). The tree is unchanged by the landing; old and new binaries
   read it.
2. Every machine (m0, m0b, m1, m1b) rebuilds and re-arms under the
   standing order R-37-m3, each landing message naming the commit. A
   rebuilt machine refuses every new claim (`APPROVAL_REQUIRED`) and
   every agent set-budget until step 4; its claimed goals continue
   (dispatch admission reads `claimed`), and `goal next` says the queue
   awaits approval. A machine not yet rebuilt keeps claiming under the
   old law during this window; the window closes at step 4 and the
   at-rest rule (§2) arms with the first approval act; the seats'
   conduct rule on the goal record ("a seat claims only a goal Wido
   approved by word") covers the window.
3. Wido enrolls his terminal where he types (`goal enroll-terminal`, per
   machine; none is enrolled as of R-29-m1) or uses the relay. Enrolling
   ends the relay on that machine (§5).
4. Wido runs the sweep (§9) on one machine; the ledger branch carries it
   to the others through the existing sync.
5. Pending journal entries of pre-cutover `claim`, `open-claim`,
   reopen-into-arc and `set-budget` replays refuse under the new rule
   and close by hand; the recovery report names them.

No format-version bump (rejected, §11); no data rewrite except the
sweep; rollback is `git revert` of the engine, and a rollback after the
sweep needs a forward fix because an old engine refuses the approved
tree — the closing review must weigh it (§15).

## 11. Rejected alternatives

1. **Two new states (`draft`/`pending-approval` plus `approved`), or an
   `expired` state.** Rejected: `queued` already means awaiting, the
   drafts directory already holds drafts, expiry is a function of time
   and enrollment that a stored state would misreport the moment the
   clock or the enrollment moved; every consumer switch in §1.10 would
   double.
2. **No new state: approval as a record on a queued goal.** Rejected:
   claimability would again hide in a field, which is today's defect.
3. **Approval as a strict-form history token consumed at claim.**
   Rejected: duplicates the norm machinery, keeps the claim
   agent-initiated with a copied token, inherits the per-revision churn.
4. **Gate on `--by` only.** Rejected: a name is not a boundary
   (goalsync_mutations.go:53-58 copies it unverified).
5. **A pre-sweep tolerance mode in the engine.** Rejected: any tolerance
   an agent can trigger is a bypass; the cutover order carries the
   window and the `ApprovalGate` mark bounds it.
6. **Bumping the root `FormatVersion`.** Rejected: stale engines already
   refuse the new field, state and root line by name.
7. **Revoke through breach-stop machinery.** Rejected: a human park
   already closes admission and lets running jobs finish.
8. **Unconditional unapprove on any budget or intent change (the
   finding's other route).** Rejected for claimed goals: dropping a
   running claim to `queued` mid-flight because its pool was raised is
   the disruption R-58-m1 complained of, and a claimed goal without a
   record after the gate arms is exactly the at-rest violation; the
   strong-authority route (proof on set-budget and resume, refusal on
   intent) keeps every record-bearing file consistent by construction.
9. **Deriving the cutover instant from the earliest `Approved.at` in the
   tree instead of a root line.** Rejected: prune deletes done files and
   would move the instant later, weakening the check silently; one
   explicit line written by the first human act is the honest fact.
10. **Ending relayed approvals at enrollment only at grant time, leaving
    existing relayed records valid until their date.** Rejected: R-29-m1
    says the first enrolled session re-ratifies everything done under
    the departure item by item; keeping relayed records live past it
    would make the terminal session optional, which is the boundary the
    departure spent.
11. **Reading the wall clock inside `Next`.** Rejected: the frontier
    would be untestable at a fixed instant and could disagree with the
    claim that runs a moment later on the same tree.

## 12. Proof plan (tests by name; fixture)

internal/goal, the record: `TestApprovedStateRoundTrips` (render/parse,
both authority forms); `TestApprovedRecordBindsToItsEvent` (refuses:
revision above file revision, event verb not approval-bearing, at/opid/
actor mismatch, `by` without `human:`, relayed without matching history
tuple, proven with reviewBy, digest mismatch after an intent or budget
change, queued-with-record, approved-without-record, approved-without-budget);
`TestApprovalGateRootLineWrittenOnceByFirstApproval`;
`TestValidateTreeRefusesPostGateClaimWithoutRecord` (pre-gate claim
tolerated, post-gate claim refused, inactive before the mark).

internal/goal, the claim gate: `TestNoClaimWithoutApproval` — the nine
writers of §1.3 enumerated by name, each either passes only through
`approved` or refuses: claim, claim --arc (queued member fails the whole
cascade), set-arc auto-claim (approved joins claimed; queued lands
queued; parked lands restingState), reconcile set-arc twin, steal
(record required, expiry ignored), set-budget (proof-bearing, claimed
only), resume (writes the record), open --claim and reopen arc-join
(retired, replay refuses by name); `TestClaimRefusesUnapprovedByName`
(`APPROVAL_REQUIRED` text); `TestClaimRefusesExpiredByName`
(`APPROVAL_EXPIRED` with both reasons); `TestClaimTakesOnlyTheBoundBudget`
(flags and `--approved-ref` refused); `TestClaimOverNormUsesStoredNormApproval`;
`TestRecoveryReplayHitsTheSameGate` (claim, set-arc) and
`TestRecoveryRefusesRetiredAndProofBearingEntries` (open-claim, reopen
arc-join, set-budget).

internal/goal, the binding: `TestIntentEditRefusesOnRecordBearingGoal`
(verb and hand edit, every actor, every state with a record);
`TestSetBudgetRefusesOnUnclaimedWork` (queued and approved, agent and
`--by`); `TestSetBudgetOnClaimedRequiresProofAndRebindsApproval`
(agent own-claim refused; `--by` without proof refused; proven and
relayed rewrite the record with the fresh digest; identical tuple is
NothingToDo); `TestResumeRebindsApproval`; `TestApproveBindsBudgetBoxesAndTuple`;
`TestApproveRunsNormGateOnce`; `TestApproveRefusesWithoutProof`
(agent-chain proof, name-only `--by`); `TestApproveReplacesStandingRecord`
(approved → approved with and without a new tuple; NothingToDo only when
proven, unexpired, unchanged; relayed repeat refused);
`TestApproveReratifiesClaimedRelayedRecord` (budget flags refuse; claim
binding untouched); `TestUnapproveOnApprovedQueuedAndClaimedParked`
(clears record and tuple; displacement; breach-stopped refuses; expired
record accepted).

internal/goal, the table: `TestTransitionTableIsClosed` — one case per
row of §2, including every restingState writer (release, arc release,
detach, set-arc source and destination arms, unpark, arc unpark, the
reconcile twins) landing `approved` with a record and `queued` without;
park and done keep the record; reopen clears it; split parent from
approved; prune retains the three relayed verbs on the root;
`TestHandEditCannotApprove` (reconcile refuses queued→approved, a
changed `Approved`, a hand-created approved goal; hand park from
approved and hand unpark to either target work).

internal/humanauthority: `TestRelayRefusedAfterEnrollment` (grant refused
for approve, unapprove, sweep, set-budget, resume, set-obligation when
the enrollment file exists; unreadable enrollment refuses by its own
message; missing file grants). internal/goal:
`TestApprovalExpiredPredicate` (proven never; relayed by date at the
last second of reviewBy versus the first second after; relayed by
enrollment regardless of order; horizon).

internal/goal, consumers: `TestNextSeparatesReadyBlockedAndAwaiting`
(fixed horizon; expired relayed lands in Awaiting; queued lands in
Awaiting whatever its blockers; labels and pin on approved);
`TestProjectionCarriesTheHorizon` (Project and ProjectAt fill it from
the given instant and the enrollment file; Next reads no clock);
`TestClaimableWorkCountsOnlyApproved` (turn verdict idle block and
steward judgment inputs); `TestQueuedFrontierDigestCoversApproved`.
internal/steward: `TestLedgerAttentionSnapshotUsesApprovedReadyAndAwaitingQueue`;
`TestOpenWorkWordingForAwaiting`. internal/metrics:
`TestDebtAgesApprovedLikeQueued`. internal/goal:
`TestManifestAndMigrationNeverProduceApproved`.

internal/goal, the sweep: `TestSweepDryRunListsAndDigests`;
`TestSweepConfirmsDigestApprovesBudgetedOnly` (100/9 shape on a fixture
tree; claimed goal's claim untouched; root line; ApprovalGate written);
`TestSweepRefusesOnDigestDrift`; `TestRelayedSweepRunsOnce`;
`TestProvenSweepRatifiesRelayedAndSkipsProven` (idempotent second run).

cmd/metasystem: `TestGoalApproveCommandProofOrder` (approve, unapprove,
sweep, set-budget: proof before syncReq, record after confirm, relay
announcement); `TestGoalNextWordingForAwaiting`; `TestGoalListApprovedSection`
(relayed and expired markers). Fixture `scripts/agents/goal-cli-fixtures.sh`
gains one scenario: open, claim refused, approve by relay (announced),
claim succeeds with the bound budget, agent set-budget refused, intent
edit refused, unapprove parks, sweep dry-run then confirm, second sweep
NothingToDo. The validation suite runs outside the delegate sandbox
(KI-15).

## 13. Build bound (one slice, tier 3)

The build is one implementer job over: internal/goal (file.go, root.go,
verbs.go, stop.go, split.go, norm.go, project.go, attention.go,
validate.go, reconcilemap.go, reconcilepub.go, recover.go, turnverdict.go),
internal/humanauthority (authority.go: the enrollment cutoff, two
methods, one record function), internal/steward (ledgerattention.go,
narrate.go, openwork.go), internal/metrics (compute.go), cmd/metasystem
(goalsync_mutations.go, goal.go, main.go), docs/backlog-mechanism.md and
docs/glossary.md, the tests of §12 and the fixture scenario. Out of
scope, named: the fleet channel's proof class (its design),
forge-proofing (ledger-authentication), tier-derived budget defaults
(severity-tiered-rigor), the legacy ledger, the token spend fence.
Acceptance is the goal's DONE line: state in file and projection;
approve/unapprove human-only with the relayed form; typed claim refusal;
idle verdict counts approved only; `goal next` separates the two; sweep
explicit and recorded; fixtures prove each refusal and the relayed
approval.

## 14. Decisions Wido may still change (recorded, not blocking)

D1 the state name `approved` (alternative `ready`). D2 `--budget
small|big` as the named boxes; a tier-named box waits for the tiering
machinery. D3 relayed approvals expire at the earliest of `reviewBy`,
the horizon and this machine's enrollment, and gate only the start of
work. D4 the sweep rule "budgeted on the day" with a digest-bound
listing, re-runnable proven to ratify relayed records. D5 R-2's opening
restriction superseded by the execution gate. D6 intent edits refuse on
any record-bearing goal (the human's path is unapprove, edit, approve).
D7 agent `set-budget` is retired entirely, INCLUDING the seat raising the
pool of its own claimed goal: this supersedes R-58-m1 item (2) ("a seat
sets the reserved-minute pool of a claimed goal high enough that a
lawful ladder never stalls") — under this design a pool raise is his
word, typed at the terminal or relayed from his phone with
`set-budget --temporary-human-word`, until the token fence lands. If Wido
keeps the seat raise, the alternative is to exclude
`reservedJobMinutesLimit` from the digest and let the agent raise only
that field on its own claim; the design does not choose it because the
finding names exactly that raise as the hole.

## 15. Self-grade (R-24)

Confidence: high on the record binding, the claim gate over the nine
enumerated writers, the transition closure and the sweep (every rule
reuses a traced mechanism and every writer is cited by line); medium on
the enrollment cutoff's operational shape (a machine Wido enrolls stops
honouring every relayed approval until he re-ratifies there, which is
R-29-m1's letter but may surprise on the day) and on the fleet cutover
window (§10 step 2 depends on four rebuilds landing in one day). Reject
this design if any of these holds: a writer of a claimed revision exists
outside §1.3's nine (the grep for `bindClaim(`, `State = StateClaimed`,
`State: StateClaimed` over the non-test tree yields fifteen lines at
exactly those nine sites, the constructor's own definition excluded; a
hit at any other site refutes it); the relayed form's widening beyond R-32-m1's two
verbs is not Wido's word (§6); Wido reverses D7 and wants the seat to
raise its own pool, in which case §4.3 and the digest change together;
or the fleet cannot rebuild all four engines before the sweep, in which
case the sweep waits and the design needs the tolerance it rejected
(§11.5). The weakest part is D7's supersession of R-58-m1 item (2): it
is the consequence of binding approval to the budget, and it moves one
routine seat act to the human until the token fence lands.

## 16. Brief gaps recorded (not filled silently)

1. Inputs 1–3 of the design brief cite the legacy ledger (goal.go:275-292,
   goalverbs.go Claim and humanGate :405-412); the live claim path is
   verbs.go:404-482 and the states are file.go:189-202; goalverbs.go has
   no Claim. The design targets the synced world and leaves the legacy
   ledger byte-identical; the zero-current rule's synced twin is the
   Goal-free exclusivity rule (§2). Confirm the mapping.
2. R-32-m1's "no other human-only act is opened" versus the goal's DONE
   line and Input 5 (relayed form for approve): resolved here by the
   later word (R-61-m1 approving the goal as written); confirm (§6).
3. R-2's "only his word opens the goal" versus Wido's 2026-09-03 rule:
   resolved by the later rule (§7); a rulings-register note should record
   the supersession, and so should D7's supersession of R-58-m1 item (2).

## 17. Fold record: findings to sections, and the named test obligations (R-60-m1)

| finding | folded in | disputed-but-non-material point → test obligation |
| --- | --- | --- |
| HAE-R1-CLAIM-WRITERS | §1.3 (nine writers enumerated by grep), §3 (the gate at each), §2 (the `ApprovalGate` mark and the at-rest rule) | whether steal should also evaluate expiry (design: no, steal continues started work) → `TestNoClaimWithoutApproval` asserts steal succeeds on an expired record and fails on a missing one |
| HAE-R1-APPROVED-PAYLOAD-MUTATION | §2 (the digest), §4.1-4.6 (intent refusal, set-budget retired off unclaimed work and proof-bearing on claimed, resume rebinds) | whether the digest should cover blockers/labels/pin (design: no) → `TestApprovedRecordBindsToItsEvent` asserts those edits keep the record valid |
| HAE-R1-APPROVAL-RECORD-BINDING | §2 (every key bound to the event; refusal fixtures) | none |
| HAE-R1-TRANSITION-CLOSURE | §1.4, §2 (the complete table, `restingState`), §8 (reconcile grammar, split) | whether release should preserve approval (design: yes; an agent act never withdraws it) → `TestTransitionTableIsClosed` asserts release lands `approved` with the record intact |
| HAE-R1-REAPPROVAL-TRANSITION | §5 (approve replaces the standing record in place; relayed repeat refused; claimed re-ratification) | none |
| HAE-R1-RELAY-ENROLLMENT-EXPIRY | §5 (grant-time cutoff in `humanauthority`, read-time predicate with the three reasons) | whether an enrollment should expire relayed records made before it (design: yes, regardless of order) → `TestApprovalExpiredPredicate` asserts both orders |
| HAE-R1-FRONTIER-CLOCK | §8 (`Projection.Horizon`, `ProjectAt` gains `now`, `Next` reads no clock, date-valid-through-its-day) | midnight semantics → `TestApprovalExpiredPredicate` asserts the last second of `reviewBy` admits and the first second after refuses |
| HAE-R1-STATE-CONSUMERS | §1.10 (the grep inventory), §8 (one line per consumer), §13 (steward and metrics in the build bound) | debt classification of expired approvals (design: same as approved) → `TestDebtAgesApprovedLikeQueued` |
