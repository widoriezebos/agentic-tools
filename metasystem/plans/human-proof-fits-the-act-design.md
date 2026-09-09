# The proof a human act demands fits what the act enables (goal human-proof-fits-the-act, slice 1)

- Status: design, revision 2, folding critique round one (nine findings,
  HPA-01 to HPA-09, all accepted; register and dispositions under
  `records/misc/human-proof-fits-the-act-critique-r1*.md`)
- Goal: human-proof-fits-the-act (tier 3; the DONE line of the goal record is
  the specification; this page settles its shape and adds the evidence of
  2026-09-09 on m1)
- Next step: one more critique round on this revision, then slice 2 builds it
  behind the canaries of section 7

## What is true today (read in the tree)

Human authority is one proof, `humanauthority.Prove`
(`internal/humanauthority/authority.go:611`). It reads the enrollment record
`artifacts/agents/authority/human-terminal.json` (`enrollmentPath`, line 508),
walks from the command's parent process upward, reads every node twice
(`stableRead`, line 406), refuses any node whose arguments match an installed
adapter signature (line 667), and succeeds only at the exact enrolled terminal
process whose session leader is the enrolled one (lines 673 to 687). There is
no other grade. `Enroll` (line 567) performs the same walk without an
enrollment, from the invoker to the operating system's session leader on the
invoker's own terminal (`walkToPID`, line 535), and writes the record. Enroll
needs no prior enrollment (line 587 reads a prior record only to bump the
generation), a fact no refusal states. With no enrollment on disk `Prove`
returns a zero proof and the plain error "human authority has no readable
terminal enrollment" (line 617): no outcome code, so no renderer can select a
refusal row for it (HPA-07).

The enrollment record binds one terminal identity, one terminal process and one
session leader (`Enrollment`, line 52). On m1 the live record reads
`terminalId darwin-tdev:268435461`, pid 8458, enrolled 2026-09-06T06:37Z: the
zsh inside tmux session `human`. A device number is what the refusal has to
show a human today, and `TERMINAL_NOT_REACHED` shows not even that: `Prove`
returns the bare code (line 647) and the CLI wraps it as "goal approve could
not prove enrolled human ancestry: TERMINAL_NOT_REACHED"
(`cmd/metasystem/goalsync_mutations.go:613`).

**The gap the critique confirmed (HPA-01).** The shared request builder
`syncReqClassified` (`goalsync_mutations.go:59`) proves human ancestry only
when it must DERIVE the lineage from the enrollment (lines 75 to 107). When
`--lineage` is given or `METASYSTEM_OWNER_LINEAGE` is exported, which every
agent seat does, the walk is skipped and `Actor.Human` is set from `--by`
(line 117). Every engine verb outside the approval family then tests only
`r.Actor.Human == ""`: `Steal` (`internal/goal/verbs.go:1692`), the foreign
branch of `Release` (887), `InstallTierLaw` and `ClassifyTier`
(`approval.go:146,188`), `SetPin` (2344), `Reconcile` (`reconcilepub.go:31`),
the foreign and parked branches of park, unpark, done, edit, detach and
set-arc (1243 to 2479), `DischargeReviewObligation` (1007), the arc cascades
(2102, 2167, 2240) and the split of a parked parent (`split.go:365`). Three
more paths never touch the builder at all: `goal migrate` builds its actor
through `goalActor` (`goalsync_verbs.go:298,358`) and passes only the brain
fence (line 334), and `goal.Migrate` (`migrate.go:77`) checks no human;
`goal repair --accept-remote` passes the brain fence (line 442) and
`RepairAcceptRemote` checks only that the name is nonempty
(`accepted.go:23`); `goal enroll-terminal` publishes the fleet cutoff with the
literal name `terminal` (`goalsync_mutations.go:1236`) and
`RecordFleetEnrollment` (`approval.go:765`) records no proof. Recovery
replays a journaled human intent from its stored `by`
(`recover.go:246,388,397`) with no proof at all, because none can exist at
replay. `CallerClass` is carried on the request but gates nothing in the goal
package. So on m1 today an agent seat can run `goal steal --by Wido --id
<goal>` and it lands, and the same is true of `classify-sweep --confirm`,
`set-pin`, `reconcile --by`, `migrate` and `repair`. Section 1 closes every
one of these by inventory, and section 7 makes the bypass the first canary.

The lease classifier (`internal/lease/classify.go:376`) is the other proof in
the tree: no announced main, delegate, steward, supervision or adapter
ancestor, and the caller has a controlling terminal. It reads each process
once and records no audit proof. It is what `metasystem stop`, `metasystem
arm`, `steward arm`, `steward restart` (`process_verbs.go:115,181`,
`steward_verbs.go:576,638`), `brain declare` and `brain withdraw`
(`brain.go:28`), `mission start` and `mission resume` under a closed fence
(`process_verbs.go:417`) and `mission resolve-taint`
(`internal/missionrunner/resolve.go:39`, which classifies its own pid) use,
and what `session stop` checks before the walk (`session_stop.go:57,68`).

`goal resume` demands the five-member tuple on the command line
(`f.budgetTuple(true)`, `goalsync_mutations.go:1026`) and then refuses unless
it equals the standing approved budget byte for byte (`stop.go:391`); the
refusal names the flags, never the values. The temporary relayed word is
retired in fact: `TemporaryGoalAuthorityHorizon` is 2026-09-06
(`internal/governance/types.go:99`) and the not-in-the-past rule
(`authority.go:246`) and the horizon rule (line 250) cannot both hold from
2026-09-07 on (ruling R-82-m1b). `metasystem arm --temporary-human-word` and
`steward arm --temporary-human-word` still mint a steward identity with
provenance `temporary-word` (`process_verbs.go:201`, `steward_verbs.go:603`,
`internal/steward/runner.go:240`); they validate only the pair's shape. A
machine re-arm copies a prior identity's word and review date into the next
generation (`runner.go:337`).

The claim quota is one claim per machine, enforced by the tree validator on
every commit (`internal/goal/validate.go:298` to 331), counting every goal in
state claimed, breach-stopped or not. `clearClaimBinding` refuses while a stop
fence stands (`verbs.go:277`), so release and steal of a stopped goal are
impossible and only resume clears it. Goal breach-stop-wedges-seat owns that
interaction and is claimed and in build on m1d as this page is written.

## The two proofs

**The full proof changes in one place.** `Prove` keeps its walk line for
line from the signature set on (line 619 onward). Its no-enrollment branch
(lines 615 to 618) changes: when `ReadEnrollment` fails, `Prove` runs the
terminal walk below first; a refusal of that walk is returned as it is (an
agent shell, a headless caller and an unreadable ancestry are refused for
the same reason whether or not a record exists), and a walk that reaches the
session leader returns a proof whose outcome is the new constant
`OutcomeNotEnrolled` (`TERMINAL_NOT_ENROLLED`), with the walked nodes
recorded, `TerminalGeneration` zero and `Valid` false. The same outcome
carries a record that exists but is unreadable or incomplete, with the read
error in the error text. That is the one structured no-enrollment outcome
HPA-07 asked for: it is minted in one function, returned to every caller of
`Prove`, and it is what `ProveTerminal` plus "no record" means. A successful
`Prove` yields a `Proof` whose new field `Grade` reads `enrolled`.

**The weak proof is Enroll's own walk without the write.** A new function,
`ProveTerminal(root, invokerPID, reader, now)`, performs exactly what `Enroll`
performs before it writes: the invoker is read stably and must have a
controlling terminal; the operating system's session leader is read; the walk
from the invoker to that session leader reads every node twice, refuses any
adapter signature at any node, and refuses a node whose terminal differs from
the invoker's. It reads no enrollment. It records the same node list, with
`TerminalRef` set to the session leader it reached, `TerminalGeneration` zero,
`Outcome` `HUMAN_AUTHORITY_PROVEN`, and `Grade` `terminal`. `Enroll` becomes
`ProveTerminal` plus the write, so the two can never drift. `Grade` is empty
on every refused proof: it names the grade a proof establishes, never the
grade a caller wanted.

Two predicates. `ValidFor(root)` keeps its meaning: the full grade only, so
every caller that checks it today keeps the full walk until this page changes
it on purpose. `TerminalValidFor(root)` accepts either grade. The fixture proof
(`FixtureGoalProof`, line 126) carries the enrolled grade and satisfies both,
as it does today. Proof records written before this change carry no grade and
are read as enrolled; a parsed record is never `Valid` (line 107), whatever
its grade.

**Where the grade is enforced: in the engine, not the CLI.** The proof travels
on the request: `VerbRequest` (`verbs.go:190`) gains `Authority
*humanauthority.Proof`, set by the CLI at request-build time. One helper in
the goal package, `(VerbRequest).requireHuman(row, grade)`, replaces every
`r.Actor.Human == ""` test named in the inventory below. It refuses when
`--by` is absent (each row keeps the sentence it prints today), when a
`--by` arrived with no proof (an internal invariant, refused with a plain
sentence and no code token, because the builder below never produces such a
request), and when the proof's grade is below the row's. The grade refusal
is a typed error, `GradeRefused{Verb, Row, Needed, Got}`, whose text carries
no code token, so the refusal register is untouched by it; the CLI renders
it through section 3. The verbs that take an explicit `proof` parameter
today (approve, unapprove, set-budget, accept-risk, set-obligation,
set-priority, resume, split, open, edit) read it from the request instead;
the parameter goes. `Publish` is not touched. Rows whose grade depends on
state (unpark, set-arc, the cascades) call the helper inside their `Mutate`
closure, where the goal file is in hand.

**The CLI builder proves on every `--by`, high first, then low.** Whenever
`--by` is present, whether or not a lineage was supplied, the builder calls
`Prove`. On `HUMAN_AUTHORITY_PROVEN` the request carries the enrolled-grade
proof. On `TERMINAL_NOT_REACHED` or `TERMINAL_NOT_ENROLLED` it calls
`ProveTerminal`; if that passes, the request carries the terminal-grade proof
and the builder keeps the full walk's refusal so that a later `GradeRefused`
can be rendered as the enrolled-grade row it is; if that refuses, the
terminal walk's outcome is rendered. Any other outcome of `Prove` (an agent
in the chain, an unreadable or changed ancestry, a cycle) is rendered at
once; the terminal walk would refuse the same node for the same reason. The
lineage is derived from the proof when none was supplied: a full-grade proof
derives `terminal-<id>-<generation>` from the enrollment as today
(`terminalEnrollmentLineage`, `goalsync_mutations.go:40`); a terminal-grade proof derives
`terminal-<id>-0`, where 0 says "not an enrolled generation". The lineage
feeds only the operation id suffix; the history actor stays `human:<name>`.
The fixture flag `--fixture-human-authority` yields the fixture proof in
place of both walks, as it does today for the verbs that register it.

**Recovery replay carries the journaled grade, never a fresh proof.**
`intentArgs` (`verbs.go:414`) stamps `authorityGrade=<enrolled|terminal>`
beside `by` whenever the request carries a proof. Replay
(`actorFromEntry`, `recover.go:426`, and the `by` read at line 246) restores
it as `Replay: true, ReplayGrade: <grade>` on the request, and the helper
accepts a replay whose recorded grade is at or above the row's. An entry
journaled before this page carries no grade and is read as terminal, the
conservative reading; a replay that then falls short refuses with "the
stored <verb> was journaled without an enrolled-grade proof; close it by
hand or run the act again". This is the one change to the intent arguments a
verb publishes, and it exists because the journal is the only proof a replay
can have.

**The classifier gate stays where it is, and the enrolled grade is added
after it where the act reopens execution.** `metasystem stop`, `metasystem
arm`, `steward arm`, `steward restart`, `brain declare`, `brain withdraw`,
`mission start` and `mission resume` under a closed fence, and `mission
resolve-taint` keep the lease classifier as their first gate and their landed
refusal grammar (`requireHumanTerminalAt`, `process_verbs.go:336`;
`brainHumanAct`, `brain.go:18`; `missionFenceBeforeArm`,
`process_verbs.go:402`; `ResolveTaint`, `resolve.go:35`). That classifier is
a walk over process ancestry: it reads each ancestor once, matches
announcements, custody and adapter signatures, and knows nothing of the
enrollment record. `metasystem arm` uses exactly that walk today, and it
keeps it (HPA-04): nothing in its classification changes. What section 1
adds to the reopening acts is a second check after the classifier: a
function `requireEnrolledTerminal(root, verb, retryCommand)` beside
`requireHumanTerminalAt` that calls `Prove` from the parent process and
renders its refusal through section 3 with the verb's own retry command. A
fixture-granted `HUMAN` classification satisfies the second check without a
walk, because the fixture identity table is the bed's explicit authority for
these verbs already (`ArmFixture`, `RestartFixture`, and the comment at
`process_verbs.go:324`). For the terminal-grade process verbs the classifier
alone remains the proof, the deliberate deviation revision 1 recorded: the
stop design settled that gate (its section 7), the brain and supervision beds
pin its texts, and those verbs write no goal-ledger line naming a human.

### What remains impossible

- From an agent's own process tree, neither grade passes: every node is
  matched against every installed adapter signature, so the agent's process
  is `AGENT_IN_AUTHORITY_CHAIN` whatever it types, `--by` included.
- A `--by` that reaches the ledger without a proof of the row's grade: the
  engine helper refuses it, however the lineage arrived and whichever verb
  carried it, including migrate, repair and the fleet enrollment record.
  `Actor.Human` alone is sufficient nowhere in the engine after section 1.
- A headless caller (no controlling terminal) passes neither grade:
  `TERMINAL_NOT_REACHED` at the invoker.
- A proof file presented later as authority: parsed proof JSON has no
  authority (`Valid` requires the in-process `observed` flag, line 107), for
  either grade.
- An agent stopping or releasing work it does not own under its own name:
  those rows still require `--by`, and `--by` now always requires a proof.
- A replay that upgrades itself: the journaled grade is the ceiling.

One boundary this page does not move, and says so: a process that
daemonizes a pseudo-terminal (a tmux server started by an agent, reparented
to launchd) yields a shell whose ancestry carries no agent signature, and
that shell passes the terminal walk today, enrolls, and then passes the full
walk. Ruling R-88-m1b records m1b doing exactly this at Wido's direction.
The weak grade makes that path one command shorter (no enroll) and no
wider: what it admits, the full grade already admitted after one enroll.
Closing it is goal enrollment-proves-a-human-not-a-terminal (a second factor
at enrollment), which is why the acts that grant authority stay on the
enrolled grade: the enrollment is the one record that goal will secure.

## 1. The inventory and the transition matrix

The rule from the goal's DONE line: acts that only stop, park or release work
take the terminal grade; acts that grant or widen authority keep the enrolled
grade. The tie-break for the acts the two classes do not name, sharpened by
HPA-02 to HPA-04: a transition takes the enrolled grade when it changes what
a seat is permitted to spend (approval, budget, obligation, risk acceptance,
tier), who may claim or hold a goal (a claim created, a pin set or cleared,
an approval restored to eligibility, a completion barrier removed), or
whether stopped execution may run again (a fence opened, a runner armed, a
declaration withdrawn, a disputed tree adopted); it takes the terminal grade
when it only halts, parks, releases, concludes, narrows, or reorders work
that is already approved. A precondition that decides between the two is
part of the row.

The table is the positive inventory HPA-01 asked for and the matrix HPA-03
asked for, in one place: every human-only entry point in the tree, the
transition it performs, its precondition, what it grants or removes, its
grade, and where the proof of that grade is checked after this page. Every
row's "checked" cell is the engine helper unless it names something else;
"today" in that cell is the gate the tree has now.

| Human transition | Precondition | What it grants or removes | Grade | Proof checked where (today's gate) |
|---|---|---|---|---|
| goal approve --id | queued goal, or approved goal re-approved after an edit | grants execution authority and binds a budget; the DONE line names it | enrolled | top of `Approve` (today: full walk, `approvalProofClass`, `approval.go:373,466`) |
| goal approve --sweep --confirm | tierless or relayed approvals in the listing | ratifies approvals fleet-wide | enrolled | top of `ApproveSweep` (today: full walk, 684) |
| goal unapprove | approved goal | rewrites the approval record; a wrong withdrawal parks a claim mid-work, so the re-approval must come from the same place | enrolled | top of `Unapprove` (today: full walk, 584) |
| goal set-budget | approved goal, not breach-stopped | widens or narrows the box; either way it changes what may be spent | enrolled | top of `SetBudget` (today: full walk, `verbs.go:680,707`) |
| goal accept-risk | a severe or unproven finding on a closed register | lets the finding land | enrolled | top of `AcceptedRiskDecision` (today: full walk, 1038) |
| goal set-obligation | claimed goal | binds a governed recurrence: standing spend | enrolled | top of `SetObligation` (today: full walk or the retired word, 781) |
| goal resume | breach-stopped claim | reopens admission under the standing approval; a new execution revision, a new clock (section 4) | enrolled; a verified channel answer through `--approved-ref` is the one other form, unchanged | top of `Resume` (today: `stop.go:356`) |
| goal set-priority | any live goal | reorders work that is already approved; grants nothing; another set-priority corrects it | terminal | top of `SetPriority` (today: full walk, `order.go:60`); this row lowers the grade on purpose |
| goal classify-sweep --confirm | tierless goals, or none left and no tier law | sets tiers, and a tier sets a box; installs the tier law | enrolled | top of `ClassifyTier` and `InstallTierLaw` (today: `--by` only, `approval.go:146,188`; bypassed by an exported lineage) |
| goal open --tier below the derived tier | new goal | lowers the rigor a goal is worked under | enrolled | top of `OpenRisked` (today: `--by` plus full walk, `verbs.go:495`) |
| goal edit lowering a risk score, the derived or set tier, or the gate width | live goal | lowers rigor | enrolled | inside `editRequest` (today: `--by` plus full walk, 1531) |
| goal edit of another pair's claimed goal, or of a parked goal, with no lowering | claimed by another pair; parked | changes fields of work that stays where it is; a claimed goal never gains a live blocker (1554) | terminal | inside `editRequest` (today: `--by` only, 1545, 1548) |
| goal split with a human ratification | a human-origin parent, or any parent whose draft the human ratifies | ratifies a decomposition; members are created queued | enrolled, unchanged | `validateSplitRatification` (today: `--by` plus full walk, `split.go:383`) |
| goal split of a parked parent, draft ratified by the machine | parked main-origin parent | creates queued members; nothing is admitted to spend | terminal | `validateSplitParent` (today: `--by` only, `split.go:365`) |
| goal steal | claimed by another machine | creates a claim on another machine, which is admission to spend there; a release plus the seat's own claim is the terminal-grade way to move work | enrolled | top of `Steal` (today: `--by` only, 1692; bypassed) |
| goal release of another pair's claim | claimed by another pair | only halts | terminal | inside `releaseRequest` (today: `--by` only, 887; bypassed) |
| goal park of another pair's claim, or of a human-origin goal | claimed by another pair; human origin | halts and reserves | terminal | inside `parkRequest` (today: 1243, 1248) |
| goal unpark of a human's park, approval standing | parked by `human:`, `f.Approved != nil` | restores Approved: the goal becomes claimable, which is eligibility to spend (`restingState`, 1309) | enrolled | inside `unparkRequest`, grade chosen from `f.Approved` (today: 1306) |
| goal unpark of a human's park, no approval | parked by `human:`, `f.Approved == nil` | restores queued; nothing is claimable until approve | terminal | same site |
| goal done of another pair's claim, of a parked goal, or of a human-origin goal | as named | concludes | terminal | inside `doneRequest` (today: 1136 to 1143) |
| goal detach of a parked member or of another pair's claimed member | as named | releases on the way out (2308) | terminal | inside `detachRequest` (today: 2296, 2301) |
| goal set-arc that moves another pair's claimed member | claimed by another pair | releases on the way out (2456) | terminal | inside `setArcRequest` at that branch (today: 2449) |
| goal set-arc that lifts a parked member into an empty or mixed arc with an approval standing | parked source, `f.Approved != nil`, destination not all-parked | restores Approved (2474, 2498) | enrolled | same closure, grade from `f.Approved` (today: 2461) |
| goal set-arc that creates a claim | approved member joining the caller's own claimed arc (2502 onward) | creates a claim | enrolled | same closure (today: the human row at 2461 or none) |
| goal set-arc into an all-parked arc, or a parked member that stays parked | as named | copies or keeps a park | terminal | same closure (today: 2478) |
| goal set-pin, set or cleared | live goal | clearing lets every machine claim; moving lets the destination claim; both widen who may hold the goal (`pinRefusal`, 2336) | enrolled | top of `SetPin` (today: `--by` only, 2344; bypassed) |
| goal discharge-review-obligation by a non-owner | obligation open on a goal the caller does not hold | removes a completion barrier | enrolled | inside `DischargeReviewObligation` (today: `--by` only, 1007) |
| goal reconcile --by | hand edits in the working tree | republishes hand edits, which can write any field of any goal, budgets and approvals included | enrolled | top of `Reconcile` (today: `--by` only, `reconcilepub.go:31`; bypassed) |
| goal release-arc, park-arc over members the caller does not own | as named | releases or parks | terminal | inside the cascade before the first member changes (today: 2102, 2167, 2170) |
| goal unpark-arc lifting human parks | any member returns Approved | the highest grade any member row needs | enrolled when any member returns Approved, terminal otherwise | same (today: 2240) |
| goal migrate | legacy ledger, reviewed digest | creates the synced ledger and imports approvals, budgets and parks | enrolled (section 2 states the bootstrap order) | top of `Migrate`, which gains the helper (today: brain fence only, `goalsync_verbs.go:334`; no engine check) |
| goal repair --accept-remote | the read-side advance refused a rewind | replaces the accepted tip; a rewind can erase a stop fence or revive an approval | enrolled | top of `RepairAcceptRemote`, which takes a `VerbRequest` instead of a name (today: nonempty name, `accepted.go:23`) |
| goal enroll-terminal | any live terminal, no prior enrollment needed | makes this terminal the enrolled one; retires the previous record; publishes the fleet cutoff once | its own walk (`ProveTerminal`), then the write, then `Prove` at the new record | the fleet record: `RecordFleetEnrollment` requires an enrolled-grade proof whose `TerminalGeneration` equals the generation it records (today: none, `approval.go:765`) |
| goal recover replaying a journaled human intent | a pending journal entry carrying `by` | re-runs the stored transition | the journaled grade | the helper's replay branch (today: the stored name alone, `recover.go:246,388,397`) |
| goal claim, goal claim --arc | any | refuse a `--by` (591, 1978): humans direct agents; they do not claim | none: not a human act | unchanged |
| session stop | classifier says HUMAN | only halts; the wedge of the goal's first defect | terminal: classifier, then `ProveTerminal` | `session_stop.go:68` seam and the store's predicate (today: classifier, then full walk) |
| metasystem stop | classifier says HUMAN | only halts | terminal: classifier only | `requireHumanTerminalAt` (unchanged) |
| metasystem arm | fence closed or runner down | opens the fence and starts the runner: stopped execution runs again | enrolled: classifier, then `Prove` | `requireHumanTerminalAt`, then `requireEnrolledTerminal` (today: classifier only, `process_verbs.go:181`) |
| steward arm, steward restart | no live runner; a live runner to replace | starts or replaces the runner that dispatches work | enrolled: classifier, then `Prove` | same pair (today: classifier, `steward_verbs.go:576,638`) |
| brain declare | undeclared checkout | narrows what a checkout may do | terminal: classifier only | `brainHumanAct` (unchanged) |
| brain withdraw | declared checkout | removes the declaration: the checkout may claim and dispatch again | enrolled: classifier, then `Prove` | `brainHumanAct` with the grade (today: classifier only) |
| mission start, mission resume under a closed stop fence | fence closed | opens the fence: the same act as arm, by the stop design's section 7 | enrolled: classifier, then `Prove` | `missionFenceBeforeArm` (today: classifier, `process_verbs.go:423`); with an open fence there is no human gate and none is added |
| mission resolve-taint --restore | an unresolved taint; the workspace equals a recorded safe tree | discards the disputed work and clears the park: it narrows | terminal: classifier only | `ResolveTaint` (unchanged, `resolve.go:39`) |
| mission resolve-taint --adopt (adopt-disputed-tree) | an unresolved taint | accepts the disputed tree as truth, waives named attribution claims and begins a new sequence segment: it admits | enrolled: classifier, then `Prove` from the parent process | `ResolveTaint` after its classification (today: classifier only) |

**Resume is widening, not continuing.** A breach stop is the machine's own
finding that a budget was breached; it closes admission. Resume creates a
new execution revision under the standing approval (`stop.go:412`), and every
job reserved after it is spend the stop had refused. The elapsed clock and the
attempt count start again on the new revision. That is the shape of a grant,
and the DONE line places resume with the grants. The wedge of the brief's
fifth defect is not solved by weakening resume; it is solved by sections 4
and 5, which leave nothing for a human to type at the enrolled terminal
merely to free a seat.

**Set-priority stays terminal and set-pin does not (HPA-02).** Priority
orders goals that are already approved, and a wrong order is one more
command to undo. A pin decides which machine may claim: `pinRefusal` runs on
every claim-assigning path (2336), so clearing a pin lets every machine
claim and moving it lets the destination claim. Both widen who may hold the
goal, and both take the enrolled grade.

**The reopening acts and their stopping twins (HPA-04).** `metasystem stop`
closes a fence; `metasystem arm` opens it and starts the runner that
dispatches approved goals. `brain declare` narrows a checkout; `brain
withdraw` returns its right to claim. `resolve-taint --restore` throws
disputed work away; `--adopt` keeps it and waives the claims against it. In
each pair the first only halts or narrows and keeps the terminal grade, and
the second lets stopped or disputed execution run again and takes the
enrolled grade. The steward's `arm` and `restart` have no stopping twin in
this table because `metasystem stop` is that twin for both.

## 2. The enrollment shape, and the bootstrap before a ledger exists

One terminal holds the enrollment, as today. Not several, for four reasons.
The record binds one terminal, one session leader and one generation, and the
generation is what the fleet cutoff (`RecordFleetEnrollment`,
`approval.go:765`) and the derived lineage name; several records need a list,
a retirement act, and a rule for which generation those two name. The reading
ruling R-88-m1b prescribes for every human act from m1b ("proves that Wido
directed the act, not that he typed it") depends on there being one
enrollment whose provenance is known; several multiply what a reader must
know. Goal enrollment-proves-a-human-not-a-terminal will put a second factor
on the enrollment act; one record is the thing it secures. And with the
terminal grade, the acts that made the second tab painful (release, stop) no
longer touch the enrollment at all; the acts that still do are the grants,
which a human can reasonably do from one place. Moving the enrollment is one
command from the new tab, and loses nothing: the generation bumps, the old
record is replaced, the fleet cutoff stays where it was first published.

**The record gains two optional fields, written at enrollment**: `tty`, the
terminal's device path (`/dev/ttys013`), and `tmuxSession`, the tmux session
name when the enrolled shell is a tmux pane. Both are observed at `Enroll`
time from the invoker: the device path by matching the terminal device
number against the entries of `/dev` (on Darwin the `darwin-tdev` number is
the device's `rdev`; on Linux the `tty_nr` of `/proc/<pid>/stat` maps to
`/dev/pts/<n>` or `/dev/tty<n>`); the tmux name by asking a running tmux
server for the pane whose `pane_pid` is the invoker (`tmux list-panes -a -F
'#{session_name} #{pane_pid}'`), with no server or no tmux on the path
recorded as no name. Schema stays 1 and both fields are optional on read, the
same rule goal human-goal-verbs-forgiving uses for the `human` name field it
adds to this record; an engine built before this change refuses the new
fields as unknown, so a terminal re-enrolls after the rebuild, as that
design already requires. Records already on disk keep proving.

**Every `TERMINAL_NOT_REACHED` refusal of an enrolled-grade act names the
terminal, and liveness is a probe the tests inject (HPA-09).** The
resolvers are one injectable value beside the `Reader`,
`TerminalResolvers{Alive func(ProcessRef) bool; TTY func(pid int64) string;
TmuxSession func(pid int64) string}`, with a kernel default: `Alive` is the
same probe `KernelReader.Read` uses (`identity.KernelProber`, line 336) plus
the start identity of the record, and the two names come from the two
resolvers above. At refusal time the CLI asks whether the enrolled terminal
process is alive and, if so, what tty and tmux session it has now; when it is
dead, the recorded `tty` and `tmuxSession` are used and the refusal says the
terminal is no longer alive. It names the current shell's tty the same way.
The name a human recognises is the tmux session name when there is one, the
tty otherwise, and the enrollment date. The one command it prints is the
enroll command, because that is the command that makes this terminal the
enrolled one; the act itself is run again afterwards, and the second line
says so in words. The texts are in section 3.

**Bootstrap: enroll locally first, publish the cutoff with the ledger
(HPA-05).** Today `goal enroll-terminal` refuses on a legacy checkout
(`goalsync_mutations.go:1227`) because it publishes the fleet cutoff into the
synced root record, while `migrate` imports approvals, budgets and parks with
no proof at all. Migrate takes the enrolled grade in section 1, so the
enrollment must be able to exist first. The lifecycle, in order:

1. Build the engine and install the adapters; the walk needs their
   signatures (`signatureSet`, line 475).
2. At the human's terminal, `goal enroll-terminal --root <root>`. `Enroll`
   writes the local record (generation 1). On a legacy checkout the fleet
   cutoff is not published; the command prints `enrolled locally; the fleet
   cutoff publishes with goal migrate` and exits 0. On a converted checkout
   it publishes as today, and `NothingToDo` when the fleet already has one
   (lines 1243 to 1248).
3. At the same terminal, `goal migrate --root <root> --source-digest <sha>
   --by <name>`. The builder proves the enrolled grade; `Migrate` requires it
   and, in the same transaction that creates the root record, writes
   `FleetEnrollment{At, Machine, Generation, Opid}` from the local
   enrollment the proof walked to (`TerminalGeneration`). A rerun of migrate
   on an identity that already carries the cutoff changes nothing. Under
   fixture authority the proof's generation is zero and no cutoff is
   published, exactly as the beds run today.
4. Every later machine clones a converted ledger, runs `goal enroll-terminal`
   locally, and finds the cutoff already recorded, as today.
5. A re-enrollment anywhere bumps the local generation and publishes nothing
   new.

`converted()` leaves the enroll-terminal path; the fleet-cutoff publication
is the only thing it guarded, and step 3 now owns it. The relayed approvals
the cutoff expires (`APPROVAL_EXPIRED`, `approval.go:360`) can only exist in
ledgers written before this page, so deferring the cutoff until migrate
widens nothing: a legacy ledger has no relayed approval to expire.

## 3. The refusal texts for the authority layer's codes

The grammar is the one goal human-goal-verbs-forgiving settled for the goal
verbs and the stop design settled for the process verbs: two lines on stderr,
the first one plain sentence, the second either `run: <one command>`, with an
optional prefix in words saying where to run it and an optional `; then
<words>`, or `no command completes this: <words>`. That goal's design names
the function, `refuseHumanVerb` in `cmd/metasystem/goal_refusal.go`; this page
adds the rows for the authority layer's codes to it. Whichever chain lands
first creates the file; the other adds rows. `<root>` is printed only when it
is not the working directory. Every other refusal of every goal verb belongs
to that goal and is not redesigned here.

**The name in a printed command is never a placeholder (HPA-08).** `<name>`
is the `--by` value the verb saw. When the verb saw none (the agent-facing
rows: `APPROVAL_REQUIRED` at a claim, dispatch or steal, and
`APPROVAL_EXPIRED` at a claim), the name is the `human` field of the
enrollment record once goal human-goal-verbs-forgiving has landed it; until
then, or when the record carries no name, the second line takes the words
form and prints no command: `no command completes this from here: a human
approves it at the enrolled terminal with goal approve --id <id>, naming
themselves with --by`. No build order between the two goals follows from
this; the row's test asserts both forms.

The engine keeps returning its codes as it does (the register's sites stay
valid); the CLI renders the two lines by code and grade. The lineage
derivation's own three refusals (`goalsync_mutations.go:81,101,104`) are the
same three situations as the rows below and are rendered by them, once. A
`GradeRefused` from the engine is rendered as the enrolled-grade row of the
full walk's refusal the builder kept (the two proofs, above).

| Code and situation | Line 1 | Line 2 |
|---|---|---|
| `AGENT_IN_AUTHORITY_CHAIN`, either grade | `goal approve: this command runs under an agent (claude, pid 18533); an agent's shell cannot carry a human's word, with or without --by.` | enrolled-grade act: `at the enrolled terminal, run: metasystem goal approve --root <root> --id <id> --by <name>`; terminal-grade act: `at an agent-free terminal, run: <the same command>` |
| `TERMINAL_NOT_ENROLLED`, enrolled grade, no record exists | `goal approve: no terminal is enrolled in this checkout yet; enrolling needs no prior enrollment or authority.` | `run: metasystem goal enroll-terminal --root <root>; then run this command again` |
| `TERMINAL_NOT_ENROLLED`, enrolled grade, a record exists but is unreadable or incomplete | `goal approve: the enrollment record in this checkout cannot be read (<reason>); enrolling here replaces it and needs no prior authority.` | the same second line |
| `TERMINAL_NOT_REACHED`, enrolled grade, the enrolled terminal is alive and this is another terminal | `goal approve: this is not the enrolled terminal. The enrolled terminal is tmux session "human" (/dev/ttys010), enrolled 2026-09-06 06:37Z, and it is alive; this shell is /dev/ttys014.` | `there, or after making this terminal the enrolled one, run: metasystem goal enroll-terminal --root <root>; then run this command again. Enrolling here needs no prior enrollment or authority and retires tmux session "human" as the enrolled terminal` |
| `TERMINAL_NOT_REACHED`, enrolled grade, the enrolled terminal is dead | `goal approve: this is not the enrolled terminal. The enrolled terminal was tmux session "human" (/dev/ttys010), enrolled 2026-09-06 06:37Z, and it is no longer alive; this shell is /dev/ttys014.` | `run: metasystem goal enroll-terminal --root <root>; then run this command again. Enrolling here needs no prior enrollment or authority and retires the dead terminal's record` |
| `TERMINAL_NOT_REACHED`, enrolled grade, the enrolled terminal is alive but its session leader changed (a new login on the same tty) | the same as the dead row, with "and its login has been replaced" in place of "and it is no longer alive" | the dead row's second line |
| `TERMINAL_NOT_REACHED`, terminal grade (no controlling terminal on this shell, or the walk left the terminal before the session leader) | `goal release: this shell has no controlling terminal, so no human can be at it.` | `at a terminal of this host, run: metasystem goal release --root <root> --id <id> --by <name>` |
| `TERMINAL_NOT_REACHED` from `goal enroll-terminal` (no terminal) | `goal enroll-terminal: this shell has no controlling terminal, so it cannot be enrolled.` | `at a terminal of this host, run: metasystem goal enroll-terminal --root <root>` |
| `ANCESTRY_UNREADABLE`, `ARGV_UNREADABLE`, either grade | the sentence `processReadRefusal.Error` builds today (pid, executable, owner, the reason), verb-prefixed | `no command completes this from this shell: run it from a shell whose ancestry up to its session leader is owned by you, for example a shell inside a tmux session you started from Terminal` (the existing workaround sentence, `authority.go:369`) |
| `ANCESTRY_CHANGED`, `PROCESS_REUSED`, either grade | `goal approve: a process in this shell's ancestry was replaced or changed its arguments between two reads.` | `run: <the same command>; then, if it repeats, a process above this shell is being restarted under you` |
| `ANCESTRY_CYCLE`, either grade | `goal approve: the process table reports a cycle in this shell's ancestry, which no kernel produces, so it cannot be trusted.` | `no command completes this: report it with the output of ps -axo pid,ppid,command` |
| `APPROVAL_REQUIRED` at a claim, dispatch or steal (`approval.go:266`) | the engine's sentence as today: `APPROVAL_REQUIRED: goal <id> is queued and not approved for execution; only the human approves it with goal approve -- this claim is refused` | with a name: `at the enrolled terminal, run: metasystem goal approve --root <root> --id <id> --by <name>`; without one: the words form above |
| `APPROVAL_REQUIRED` at resume, a typed tuple that differs from the ledger (`stop.go:392`, reshaped by section 4) | `goal resume: resume keeps the approved budget 1d/10/1200m/1/3 as it stands; the tuple typed was 2d/10/1200m/1/3.` | `run: metasystem goal resume --root <root> --id <id> --by <name>; then, for a different box, goal set-budget` |
| `APPROVAL_EXPIRED` (`approval.go:360`), a relayed approval from before the cutoff | the engine's sentence as today | with a name: `at the enrolled terminal, run: metasystem goal approve --root <root> --id <id> --by <name>`; without one: the words form |
| `RELAY_AFTER_ENROLLMENT` (`approval.go:411`) | retired with the relay class (section 6); the row leaves the register | |
| `session stop`, either code | the same sentences with the `session stop:` prefix; the classifier's own refusal (`session_stop.go:63`) keeps its sentence | the same second lines, with `metasystem session stop --by <name>` as the printed command; the classifier's refusal gains the `at an agent-free terminal, run:` line |
| `metasystem arm`, `steward arm`, `steward restart`, `brain withdraw`, `mission start`, `mission resume`, `mission resolve-taint --adopt`, the enrolled check after the classifier | the same sentences with the verb's prefix, in the process verbs' two-line grammar (`refuseProcessVerb`, `process_verbs.go:310`) | the same second lines, with the verb's own retry command (`processVerbRetryCommand`, or `metasystem mission resolve-taint --mission <id> --taint <n> --adopt --waives <claim> --by <name> --reason <text>`) |

The compact budget form in the resume row is the one goal
human-goal-verbs-forgiving defines (`FormatBox`); until it lands, the five
values print as `elapsedLimit=1d attemptLimit=10 ...`, the ledger's own
spelling, which is what the human copies from today.

The new register row: `{Code: "TERMINAL_NOT_ENROLLED", Owner:
"internal/humanauthority", Site: "authority.go:<the constant's line>", Shape:
Identity}`, beside `TERMINAL_NOT_REACHED`; Identity rows carry no Override
(`register.go:18`), and the enroll command is the renderer's, as it is for
its siblings.

## 4. Resume's budget comes from the ledger

`goal resume` takes `--id` and `--by`, nothing else. Its budget is the
standing approved budget the transaction already reads
(`requireApprovedForClaim`, `stop.go:387`); the CLI reads the same value from
the read-only projection before the transaction, for the channel token
(`ResumeApprovalToken`) and the intent arguments, and the transaction refuses
if the ledger's budget moved in between, which is the existing "LostToCompetitor"
shape. The five explicit flags stay accepted, optional, for scripts that
already pass them: equal to the ledger, they change nothing; different, the
refusal prints the standing values and the command without the tuple
(section 3). The refusal for a missing `--id` or `--by` prints the command
with the value seen and names the missing one in words, under goal
human-goal-verbs-forgiving's rule.

A human who wants a different budget resumes, then runs `goal set-budget`
(or, once it lands, `goal budget <id> <box>`), because set-budget refuses a
breach-stopped goal today and goal breach-clock-and-budget-honesty owns that
pair; the resume refusal says so in words. The intent arguments a resume
publishes are unchanged apart from the `authorityGrade` stamp every proven
act now carries, so the history line and the recovery replay read as before.

## 5. A stopped goal and the machine's claim slot

A breach-stopped claim does not count against the machine's quota. A stopped
goal cannot receive a job (its fence refuses every creation path with
`ADMISSION_CLOSED_ELAPSED` or `ELAPSED_BREACH`), so it is not live work, and
the quota exists to bound live work per machine, not to punish a breach. The
predicate is one line in three places: the validator's `claimsByMachine`
skips a goal whose `StopFence` stands (`validate.go:303`), and `goal next` and
the claim admission use the same predicate.

What still prevents a machine from holding two live claims by stopping one:
the stopped claim cannot be launched (the fence), and `resume` re-checks the
quota inside its transaction, refusing when the machine already holds another
live claim, with the second line naming the goal to release first. A stop
therefore parks the slot's occupant; it never duplicates the slot.

How a machine recovers with the terminal grade alone: for the slot, nothing
is typed; the seat claims the next goal. For the stopped goal itself, the
human act is whichever goal breach-stop-wedges-seat lands: if release becomes
lawful on a stopped claim, that release takes the terminal grade (it only
halts, and the fence on resuming the goal is preserved, as that goal's DONE
requires); if the goal parks without holding the slot, no act is needed. The
resume of the breached goal keeps the enrolled grade either way (section 1).
That goal is claimed and in build on m1d (chain bsws-build1b-20260909); this
page takes its answer for the quota predicate and adds only the grade of the
act. The build order is the orchestrator's: whichever chain lands second
rebases the one predicate.

## 6. The relayed word: recorded, never binding

**No relay from a seat binds.** A human present in an agent session who
speaks an instruction gets it recorded, with its provenance stated and
unverified, and gets the exact command to type; the seat never passes `--by`
for it and never presents it as authority. The record is the goal's next
step, which the seat may already write for a goal it holds or that is
unclaimed and unparked, in a fixed prefix so a reader can tell it from the
seat's own words:

```text
Relayed word, unverified, heard by m1+main-1788940932-18533-7fa6c2 at 2026-09-07T08:12Z: "add a durable-stop requirement to the metasystem-stop-verb intent". To bind it, Wido runs at the enrolled terminal: metasystem goal unapprove --id metasystem-stop-verb --by Wido --because "intent change"; metasystem goal edit --id metasystem-stop-verb --intent "..."; metasystem goal approve --id metasystem-stop-verb --by Wido
```

For a goal the seat may not edit (claimed by another pair, parked by a human),
the record goes to `records/misc/relayed-words.md`, one dated line each, with
the same prefix. Recording a word grants nothing and needs no proof, which is
the whole point: today it is graded as if it were a grant, and so it is not
recorded at all.

**The temporary word does not come back in a binding form, and its dead
surface goes, at every site (HPA-06).** Ruling R-82-m1b keeps the horizon at
2026-09-06 and forbids depending on the path again without a new row. A flag
that can never succeed breaks this page's own rule (every refusal prints a
command that would succeed), so slice 2 removes the surface and the class.
The deletion sites, all read in the tree:

- `cmd/metasystem/goalsync_mutations.go`: the `--temporary-human-word` and
  `--review-by` flags on approve, unapprove, set-budget, accept-risk, resume,
  open and edit (lines 241 to 244) and on set-obligation (1283, 1284); the
  pair checks (317, 601); the word and date parameters of
  `goalAuthorityProver` (581) and the temporary branches of
  `proveGoalHumanAuthority` (610 to 616), resume (1050 to 1058) and
  set-obligation (1319 to 1325); `ProveOrTemporaryGoalAuthority` as the
  shipped prover (308, 1254), replaced by the builder of the two proofs.
- `cmd/metasystem/process_verbs.go`: the two flags on `metasystem arm` (173
  to 177), the pair check (185 to 188) and the `ArmTemporary` call (200,
  201).
- `cmd/metasystem/steward_verbs.go`: the two flags on `steward arm` (554,
  555), the pair check (569 to 572), the TEMPORARY banner (580 to 587) and
  the `ArmTemporary` call (602, 603).
- `cmd/metasystem/main.go:678`: the usage line advertising the flags on
  `metasystem arm`.
- `internal/up/up.go:470`: the drift remedy's clause "or relay the human's
  recorded word with --temporary-human-word and --review-by"; the remedy
  keeps its `steward restart` command and says "at the enrolled terminal",
  which section 1 makes true.
- `internal/steward/runner.go`: `ArmTemporary` (236 to 249); the `Word` and
  `ReviewBy` members of `mintPlan` (281, 282) and of `humanMintDecision`
  (298 to 305); the machine-rebuild carry-forward of `prior.TemporaryHumanWord`
  and `prior.ReviewBy` (337); the temporary exemption from fixture
  enrollment (655); the write of the two fields into the new identity (704);
  the "armed TEMPORARILY" message (749 to 751).
- `internal/humanauthority/authority.go`: `OutcomeTemporary` (36),
  `TemporaryWordRuling` (39), `ValidateTemporaryWordPair` and the temporary
  validators and constructor (213 to 311), the temporary branches of
  `AuthorizesResume`, `AuthorizesSetObligation`, `TemporarySetObligationFor`
  and `TemporaryResumeFor` (148 to 166, 207 onward), which then equal
  `ValidFor` and go; `ProveOrTemporaryGoalAuthority`.
- `internal/goal`: the temporary legs of approve, the sweep, unapprove,
  set-budget, resume and set-obligation (`approval.go:380` to 382, 393 to
  396, 428 to 432, 741, 752 to 754; `verbs.go:834` to 836, 850; `stop.go:370`,
  394 to 401, 409 to 411); `refuseRelayedAfterFleetEnrollment` and
  `RELAY_AFTER_ENROLLMENT` (`approval.go:409`); `repeatedRelayedActError`
  (`file.go:339`); the register row (`register.go:45`).
- `scripts/agents/goal-cli-fixtures.sh`: the three relayed invocations of
  the `brain-human-word-refuses` matrix (the flags at lines 521, 524 and
  534); the brain fence's sentence is unchanged.
- The tests that pin all of the above, in their packages.

What stays, because records exist: reading an `authority=relayed` approval
already in a ledger and its `APPROVAL_EXPIRED` refusal (`file.go:212`,
`approval.go:360`); the history-line and obligation fields
`authorityOutcome`, `authorityReviewBy`, `authorityRuling` and
`temporaryHumanWord` with their parse and render code and the recorded-value
validators (`file.go:1216` onward, `obligation.go:72`,
`governance/types.go`), which read what was written; the "relayed, review
by" detail in `goal show` (`goal.go:398`); the three provenance fields on
`Proof` (83 to 85), which `Valid` keeps refusing when set; and on the steward
identity the `temporary-word` enrollment constant and the two fields
(`identity.go:39,66,67`) with their read-side acceptance (157) and the
status suffix that names them (`runner.go:848`).

**Re-arm never mints a temporary generation.** `ReArmRebuiltEngine`'s
decision (`runner.go:311` to 341) carries a prior identity's `Enrollment`,
witness generation and witness time into the next generation. After this
page it carries no word and no review date (there are none to carry), and a
prior enrollment of `temporary-word` is not eligible for automatic re-arm at
all: the decision returns `ErrEnrollmentDrift` with the remedy "this engine
was armed under a recorded remote word; from the enrolled terminal run
metasystem steward restart --repo <root>", and ordinary `up` refuses as it
does for every other drift. The prior record stays on disk unchanged until
that restart replaces it. Historical fields are provenance a reader may
print; no code path may turn them into a new authority generation.

**The one relay that binds stays exactly where it binds.** A verified channel
answer (the six-digit code on a durable question) is accepted by approve
(`approval.go:387`), by the norm claim (`norm.go:114`) and by resume through
`--approved-ref` (`goalsync_mutations.go:1042`). This page widens it to no
other verb. A remote human path for the acts the channel does not cover
returns, if it returns, with the second factor of goal
enrollment-proves-a-human-not-a-terminal, whose intent reserves that decision
to Wido; not here.

## 7. Canaries first, then their twins

Every proof of this page is a deterministic package test through the tree
reader in `internal/humanauthority/authority_test.go` (`treeReader`,
`authoritySnapshot`) or its CLI twin through the prover seams that already
exist (`runGoalApproveWithAuthority`, `runGoalResumeWithAuthority`,
`proveSessionStopHuman`, `proveSyncReqHumanAuthority`,
`runGoalEnrollTerminalWith`) plus one new seam for the terminal walk beside
`proveSyncReqHumanAuthority`. The headless beds cannot drive the walk's
refusals: the walk reads the real process tree through `KernelReader`, which
ignores the fixture identity table on purpose (`identity/terminal.go:16`), so
a bed's verdict would depend on who runs it. The bed's part is the
fixture-authority success path and the one bypass scenario below, which is
deterministic because it drives only enrolled-grade verbs.

The tree reader needs four shapes it does not have. A second terminal:
pids 60 (`terminal-session-2`, parent 1, `tty-2`), 61 (`interactive-shell`,
parent 60, `tty-2`), 62 (`command-wrapper`, parent 61, `tty-2`), which needs
`SessionLeader` to answer per pid (a `sessions map[int64]int64` beside the
single `session` field). A headless shape: pid 70 (`command-wrapper`, parent
71) and 71 (`scheduler`, parent 1), both with an empty terminal id. A dead
enrolled terminal: `enrolledReader` enrolled on `tty-1`, then pids 10 and 20
deleted from the snapshot map. And the injected `TerminalResolvers`: `Alive`
answers from the snapshot map (present and same start identity means alive),
`TTY` and `TmuxSession` return fixed names per pid. Alive versus dead is
therefore a fact of the fixture, not of the machine (HPA-09).

**Never on the live ledger.** Every test below builds its own ledger in
`t.TempDir()` through `syncedClaimedGoalFixture`
(`goalsync_mutations_test.go:984`) or a legacy ledger of the same shape for
migrate; the bed builds `$tmp/clone`. The bypass must never be reproduced by
hand against a checkout that carries real goals: a `steal --by` that lands
there is a real steal. The orchestrator's hand reproduction, if any, is a
scratch clone.

The canaries, each one Go test with a ceiling, and the smallest run that
proves each:

1. **An exported lineage never carries a human's word (red today).**
   `TestExportedLineageNeverCarriesAHumanWord` in `cmd/metasystem`: a
   scratch synced ledger; `t.Setenv("METASYSTEM_OWNER_LINEAGE", "m1")`; the
   two prover seams return the agent shape (pid 50 under `codex-agent`);
   then `goal steal`, a foreign `goal release`, `goal classify-sweep
   --confirm`, `goal set-pin`, `goal reconcile --by`, `goal repair
   --accept-remote`, and `goal migrate` on a scratch legacy ledger, each
   with `--by Wido`. Every exit is nonzero, the first stderr line is the
   `AGENT_IN_AUTHORITY_CHAIN` row, and no ledger history line reads
   `human:Wido`. Today steal, release, classify-sweep, set-pin, reconcile,
   migrate and repair all land. Bed twin: scenario
   `exported-lineage-refuses` in `scripts/agents/goal-cli-fixtures.sh`, which
   runs steal, set-pin, classify-sweep, reconcile and repair with the
   lineage exported and no fixture flag and asserts every one refused;
   headless it fails the terminal walk, from a human's terminal it passes
   that walk and still lacks the enrolled grade, so the verdict is the same
   either way. Run: `go test ./cmd/metasystem -run
   TestExportedLineageNeverCarriesAHumanWord -count=1 -timeout 120s`; the
   bed runs its whole list (`bash scripts/agents/goal-cli-fixtures.sh`,
   about 100 seconds today, 300 seconds the hang bound), since single
   scenarios are a private child form (`fixture-budget.sh:26`).
2. **A human at an unenrolled terminal may stop and may not approve, and the
   enroll command is printed (red today).**
   `TestUnenrolledTerminalStopsButCannotApprove` in `cmd/metasystem`: no
   enrollment file; `session stop` through the seam with `ProveTerminal`
   writes its marker; `goal approve` refuses with the no-record row, line 2
   `run: metasystem goal enroll-terminal --root <root>; then run this command
   again`. Run: `go test ./cmd/metasystem -run
   TestUnenrolledTerminalStopsButCannotApprove -count=1 -timeout 120s`.
3. **The printed command runs and the approve then succeeds (red today).**
   `TestPrintedEnrollCommandThenApproveSucceeds` in `cmd/metasystem`: the
   test takes line 2 of canary 2, cuts it at `; then`, strips `run: ` and the
   `metasystem` word, and runs the remainder through
   `runGoalEnrollTerminalWith` with the same reader, then runs the approve
   with the same reader; the Approved line reads `authority=proven`, the
   proof record's grade is `enrolled`, and the fleet enrollment is recorded
   once. Same run form, its own name.
4. **A dead enrolled terminal is recovered by the printed command, and the
   old terminal is retired (red today).**
   `TestDeadEnrolledTerminalRecoversByThePrintedCommand` in
   `internal/humanauthority` and its CLI twin: enroll on `tty-1` (pid 20);
   delete pids 10 and 20; `Alive` answers false; `Prove` from pid 62 is
   `TERMINAL_NOT_REACHED` and the CLI prints the dead row naming the recorded
   tty; the test executes the printed enroll command from pid 61 as canary 3
   does; the new record is generation 2 on `tty-2`; `Prove` from 62 is
   proven; then pids 10 and 20 are put back in the snapshot map and `Prove`
   from pid 30 (the old terminal's shell) is `TERMINAL_NOT_REACHED`, which is
   what "retired" means. Run: `go test ./internal/humanauthority
   ./cmd/metasystem -run DeadEnrolledTerminalRecovers -count=1 -timeout 120s`.

The regression guards, green today and named as such so a bed reader does
not mistake them for the change: `TestAgentShellRefusedForBothGrades` in
`internal/humanauthority` (pid 50 under 40 `codex-agent` is
`AGENT_IN_AUTHORITY_CHAIN` from `Prove`, which holds today, and from
`ProveTerminal`, which is new) and `TestApproveAndSessionStopRefuseAnAgentShell`
in `cmd/metasystem` (both refuse today; the test pins the two lines of
section 3 as the new part).

Their twins, same instruments and ceilings:

- `TestProveWithoutEnrollmentIsTheTerminalWalkPlusNotEnrolled` in
  `internal/humanauthority`: no record; the human shape yields
  `TERMINAL_NOT_ENROLLED` with the walked nodes; the agent shape yields
  `AGENT_IN_AUTHORITY_CHAIN`; the headless shape yields
  `TERMINAL_NOT_REACHED`; an unreadable record yields `TERMINAL_NOT_ENROLLED`
  with the read error in the text. Its CLI twin
  `TestNotEnrolledMapsToTheEnrollCommand` asserts the exact line 2 (HPA-07).
- `TestOtherLiveTerminalRefusalNamesTheEnrolledOne`: both terminals alive,
  the injected resolver returns tmux name `human`; the alive row prints
  `tmux session "human" (/dev/ttys010)`, the enroll command, and the words
  "needs no prior enrollment or authority", "retires" and "run this command
  again" (HPA-08).
- `TestSessionLeaderReplacedRefusalSaysSo`: `sessions[20]` changed to 11; the
  replaced-login row.
- `TestHeadlessCallerFailsBothGrades`: pid 70; `TERMINAL_NOT_REACHED` from
  both, and the terminal-grade row for `goal release`.
- `TestApprovalRequiredWithoutANameSaysNoCommand` in `cmd/metasystem`: an
  agent's claim on an unapproved goal with no name derivable prints the
  words form and no `run:` line; with a recorded name, the full command.
- `TestByWithoutProofIsRefusedWhateverTheLineage` in `internal/goal`: every
  row of section 1's table with `Actor.Human` set and no `Authority`
  refuses; with a terminal-grade proof the terminal rows land and the
  enrolled rows refuse with `GradeRefused`; with an enrolled-grade proof all
  land; `RecordFleetEnrollment` refuses a proof whose generation differs from
  the one recorded.
- `TestUnparkGradeFollowsTheStandingApproval` and
  `TestSetArcGradeFollowsWhatItCreates` in `internal/goal`: the
  state-dependent rows, both branches each.
- `TestSetPinNeedsTheEnrolledGrade`: set and clear refuse a terminal-grade
  proof and land an enrolled one (HPA-02).
- `TestMigrateAndRepairNeedTheEnrolledProof` in `cmd/metasystem` and
  `TestEnrollBeforeMigrateThenMigratePublishesTheCutoff`: enroll-terminal on
  a legacy ledger prints the deferral line and publishes nothing; migrate
  with the enrolled proof creates the root record with
  `FleetEnrollment.Generation` equal to the local record's; a later
  enroll-terminal is `NothingToDo` for the cutoff (HPA-05).
- `TestReplayHonoursTheJournaledGrade` in `internal/goal`: a journaled
  enrolled-grade unpark replays; a journaled terminal-grade unpark onto a
  standing approval refuses with the close-by-hand sentence; an entry
  without the stamp reads as terminal.
- `TestArmAndAdoptNeedTheEnrolledTerminal` in `cmd/metasystem` and
  `internal/missionrunner`: with the classifier saying HUMAN and the prover
  seam refusing `TERMINAL_NOT_REACHED`, `metasystem arm`, `steward arm`,
  `steward restart`, `brain withdraw`, `mission start` under a closed fence
  and `resolve-taint --adopt` print the process-verb rows; `metasystem stop`,
  `brain declare` and `resolve-taint --restore` land; under a fixture-granted
  HUMAN all land (HPA-04).
- `TestReArmNeverMintsATemporaryGeneration` in `internal/steward`: a prior
  identity with `enrollment=temporary-word` makes `ReArmRebuiltEngine` return
  `ErrEnrollmentDrift` with the restart remedy, and the record on disk is
  unchanged; a prior `human-terminal` identity re-arms with empty word and
  date fields (HPA-06).
- `TestResumeTakesTheLedgerBudget` in `cmd/metasystem`: resume with `--id`
  and `--by` alone confirms; with an equal tuple confirms; with a different
  tuple prints the standing values and the command without the tuple.
- `TestStoppedClaimDoesNotHoldTheSlot` is goal breach-stop-wedges-seat's own
  fixture; this page adds `TestResumeRefusesASecondLiveClaim` in
  `internal/goal` beside it.
- `TestTemporaryWordFlagsAreGone` in `cmd/metasystem`: `goal approve
  --temporary-human-word x --review-by 2026-09-30`, `metasystem arm
  --temporary-human-word x --review-by 2026-09-30` and `steward arm` with the
  same fail to parse, and the forgiving rule's flag-parse row prints the
  command without them.
- `TestProofGradeRoundTrips` in `internal/humanauthority`: a recorded
  terminal-grade proof re-reads as terminal, a record without the field reads
  as enrolled, and neither parsed record is `Valid`.

The bed: `scripts/agents/goal-cli-fixtures.sh` gains two scenarios in its
list, `proof-grades` and `exported-lineage-refuses`. Under
`--fixture-human-authority` (registered on release, steal, resume, set-pin,
set-priority, reconcile, classify-sweep, migrate and repair, which the bed
cannot drive today) `proof-grades` runs a foreign release, a steal, a resume
without a tuple and a set-priority confirm and reads `human:Wido` on their
history lines; `exported-lineage-refuses` is canary 1's twin. `metasystem
stop` and `session stop` keep their existing scenarios (`wrong-terminal` and
the supervision bed's seat-refused); the `wrong-terminal` arm under fixture
authority keeps landing, since a fixture-granted HUMAN satisfies the enrolled
check. The landing receipt is the one the goal record names: the go gate
plus the human-authority package and the goal CLI fixtures; no full battery.

## 8. Scope of slice 2

Changes: `internal/humanauthority/authority.go` (`ProveTerminal`, the
`Grade` field and predicates, `Enroll` as the walk plus the write, the
no-enrollment branch of `Prove` and `OutcomeNotEnrolled`, the two record
fields, `TerminalResolvers`, the relay class removed); `internal/goal`
(`Authority`, `Replay` and `ReplayGrade` on the request, the `requireHuman`
helper and `GradeRefused`, the sweep over every row of section 1's table
including migrate, repair, the fleet enrollment record and the replay
branch, the `authorityGrade` intent stamp, the resume budget, the quota
predicate's grade row, the resume quota check, the session stop store
predicate, the relay legs removed); `cmd/metasystem/goalsync_mutations.go`
and `goal_refusal.go` (prove on every `--by` high then low, the lineage
rule, the refusal rows, the name rule, the flags removed, the fixture flag
registered where the bed needs it, enroll-terminal's deferral and the proof
it passes to the fleet record); `cmd/metasystem/goalsync_verbs.go` (migrate
and repair through the builder); `cmd/metasystem/session_stop.go`
(`ProveTerminal`); `cmd/metasystem/process_verbs.go`
(`requireEnrolledTerminal`, arm and the mission fence, the flags removed);
`cmd/metasystem/steward_verbs.go` (the enrolled check on arm and restart,
the flags removed); `cmd/metasystem/brain.go` (the enrolled check on
withdraw); `cmd/metasystem/main.go` (the usage line);
`internal/missionrunner/resolve.go` (the enrolled check on adopt);
`internal/up/up.go` (the remedy text); `internal/steward/runner.go`
(`ArmTemporary` removed, the re-arm rule); `internal/refusal/register.go`
(`TERMINAL_NOT_ENROLLED` added, `RELAY_AFTER_ENROLLMENT` removed; the other
rows keep their sites); the fixture beds and package tests named in section
7; the help lines of the graded verbs, which say "human-only, at any
terminal" or "human-only, at the enrolled terminal".

Unchanged: `Prove`'s walk; every record line format of the ledger; the intent
arguments each verb publishes apart from the `authorityGrade` stamp; the
fixture authority path and its root binding; the verified channel word and
where it binds; the process verbs' classifier gate and refusal grammar; the
brain's gate; the breach-stop batch, the fence and the stop's own budget law;
what approval means; the claim quota's purpose; rank, claim and approval
semantics beyond the grades section 1 assigns.

The one part of this slice the orchestrator may cut into its own slice without
loss is the relay-class deletion (section 6): the grading and the refusals do
not depend on it, only the flag removal does, and the flags can go first.

## Revision record

**Revision 2 (2026-09-09, the Fable lane, chain human-proof-design2-20260909)**
folds critique round one. Every finding was accepted by the orchestrator;
what changed, by finding:

- HPA-01: "What is true today" now names every route that took a human name
  without a proof (the exported-lineage builder, migrate, repair, the fleet
  enrollment record, recovery replay). Section 1 is a positive inventory of
  every human-only entry point in the tree, goal verbs, process verbs, brain,
  session stop, the mission runner's fence and taint resolution, with the
  proof each takes and where it is checked. Migrate, repair and the fleet
  record gain explicit proof plumbing; replay carries the journaled grade.
  Section 7 makes the bypass canary 1, in a scratch ledger only.
- HPA-02: set-pin, set and cleared, takes the enrolled grade; set-priority
  stays terminal, with the reason.
- HPA-03: the grading table became a transition matrix with preconditions;
  unpark onto a standing approval, set-arc that creates a claim or lifts a
  park onto an approval, and discharge-review-obligation take the enrolled
  grade; the split, open and edit paths that already carry the full proof
  have explicit grades; the arc cascades take the highest member grade.
- HPA-04: `metasystem arm`, `steward arm`, `steward restart`, `brain
  withdraw` and `resolve-taint --adopt` take the enrolled grade as reopening
  acts, checked after the classifier they use today, which is kept as it is;
  their stopping twins stay terminal. The mission-runner files enter section
  8. This revision also grades `mission start` and `mission resume` under a
  closed fence as enrolled, because the stop design makes opening the fence
  the same act as arm; the reading that arm keeps its ancestry walk and gains
  the enrolled check on top is this revision's, and is the point the next
  round should confirm first if it disagrees.
- HPA-05: section 2 states the bootstrap in order: local enrollment before
  migration, the fleet cutoff published by migrate, migrate on the enrolled
  grade; `converted()` leaves the enroll path.
- HPA-06: section 6 names every deletion site by file and line, including
  the steward-arm caller, the usage line, the up remedy and the runner's
  carry-forward, and decides that re-arm keeps historical fields as inert
  provenance, never mints a temporary generation, and refuses to re-arm a
  temporary-word prior automatically.
- HPA-07: `Prove` changes in its no-enrollment branch only: the terminal walk
  first, then the one structured outcome `TERMINAL_NOT_ENROLLED`, registered
  and mapped to the enroll command by a named test.
- HPA-08: the alive-other-terminal row says no prior enrollment or authority
  is needed, that enrolling retires the other terminal, and that the refused
  command is run again; the printed name is the `--by` seen or the recorded
  name, and otherwise the words form prints no command and no placeholder.
- HPA-09: canary 1 is the bypass and is red; today's agent refusals are named
  regression guards; canary 4 executes the printed command and verifies the
  old terminal is retired; liveness is an injected resolver. Every canary
  names its smallest run.

Not changed by this revision, as the fold brief required: the two-grade shape
and the goal's rule; section 5's deferral to goal breach-stop-wedges-seat;
no second factor (Wido postponed it); no rank, claim or approval semantics
beyond what the matrix grades.
