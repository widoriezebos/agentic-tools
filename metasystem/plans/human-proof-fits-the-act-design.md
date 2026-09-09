# The proof a human act demands fits what the act enables (goal human-proof-fits-the-act, slice 1)

- Status: design, revision 1, awaiting one critique on the design-critic lane
- Goal: human-proof-fits-the-act (tier 3; the DONE line of the goal record is
  the specification; this page settles its shape and adds the evidence of
  2026-09-09 on m1)
- Next step: one critique, then slice 2 builds it behind the canaries of
  section 7

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
generation), a fact no refusal states.

The enrollment record binds one terminal identity, one terminal process and one
session leader (`Enrollment`, line 52). On m1 the live record reads
`terminalId darwin-tdev:268435461`, pid 8458, enrolled 2026-09-06T06:37Z: the
zsh inside tmux session `human`. A device number is what the refusal has to
show a human today, and `TERMINAL_NOT_REACHED` shows not even that: `Prove`
returns the bare code (line 647) and the CLI wraps it as "goal approve could
not prove enrolled human ancestry: TERMINAL_NOT_REACHED"
(`cmd/metasystem/goalsync_mutations.go:613`).

Which verbs demand which proof today, traced to the gate:

| Act | Gate today | Where |
|---|---|---|
| approve, approve --sweep, unapprove, set-budget, accept-risk | the full walk, checked in the engine through `approvalProofClass` | `internal/goal/approval.go:373`, `verbs.go:683`, `verbs.go:1041` |
| set-obligation, resume | the full walk or the retired temporary word, in the engine | `verbs.go:781`, `stop.go:356` |
| set-priority | the full walk, in the engine | `internal/goal/order.go:63` |
| session stop | the lease classifier says HUMAN, then the full walk; the store re-checks `ValidFor` | `cmd/metasystem/session_stop.go:62,68`; `internal/goal/sessionstop.go:244` |
| metasystem stop, metasystem arm, steward arm, steward restart | the lease classifier says HUMAN; no enrollment | `cmd/metasystem/process_verbs.go:336`; decided in `plans/metasystem-stop-verb-design.md` section 7 |
| brain declare, brain withdraw | the lease classifier says HUMAN; no enrollment | `cmd/metasystem/brain.go:18` |
| enroll-terminal | its own walk to the session leader; no prior enrollment | `authority.go:567` |
| steal, a foreign release, classify-sweep confirm, set-pin, reconcile with --by, and every other "is a human act" row in `verbs.go` | `--by` present; NO proof when a lineage is supplied | see below |
| migrate, repair --accept-remote | `--by` present; the brain fence only | `goalsync_verbs.go:334`, `goalsync_verbs.go:442` |

The last two rows are a gap the goal's intent names as the reason the walk
exists ("an agent with a shell in the checkout would otherwise approve its own
goals and raise its own budgets by typing --by"). The shared request builder
`syncReqClassified` (`goalsync_mutations.go:59`) proves human ancestry only
when it must DERIVE the lineage from the enrollment (lines 75 to 107). When
`--lineage` is given or `METASYSTEM_OWNER_LINEAGE` is exported, which every
agent seat does, the walk is skipped and `Actor.Human` is set from `--by`
(line 117). The engine verbs in that row test only `r.Actor.Human == ""`
(`Steal` at `verbs.go:1692`, foreign `Release` at 887, `InstallTierLaw` and
`ClassifyTier` at `approval.go:147,189`, `SetPin` at 2344, `Reconcile` at
`reconcilepub.go:31`). `CallerClass` is carried on the request but gates
nothing in the goal package. Only a brain-declared checkout refuses
(`brainHumanWordClassification`, line 128). So on m1 today an agent seat can
run `goal steal --by Wido --id <goal>` and it lands. The same is true of
`goal classify-sweep --confirm`. This page closes that as part of grading.

The lease classifier (`internal/lease/classify.go:376`) is the other proof in
the tree: no announced main, delegate, steward, supervision or adapter
ancestor, and the caller has a controlling terminal. It reads each process
once and records no audit proof. It is what the process verbs and the brain
use, and what `session stop` checks before the walk.

`goal resume` demands the five-member tuple on the command line
(`f.budgetTuple(true)`, `goalsync_mutations.go:1026`) and then refuses unless
it equals the standing approved budget byte for byte (`stop.go:391`); the
refusal names the flags, never the values. The temporary relayed word is
retired in fact: `TemporaryGoalAuthorityHorizon` is 2026-09-06
(`internal/governance/types.go:99`) and the not-in-the-past rule
(`authority.go:246`) and the horizon rule (line 250) cannot both hold from
2026-09-07 on (ruling R-82-m1b). `metasystem arm --temporary-human-word`
still mints a steward identity with provenance `temporary-word`
(`internal/steward/runner.go:240`); it validates only the pair's shape.

The claim quota is one claim per machine, enforced by the tree validator on
every commit (`internal/goal/validate.go:298` to 331), counting every goal in
state claimed, breach-stopped or not. `clearClaimBinding` refuses while a stop
fence stands (`verbs.go:277`), so release and steal of a stopped goal are
impossible and only resume clears it. Goal breach-stop-wedges-seat owns that
interaction and is claimed and in build on m1d as this page is written.

## The two proofs

**The full proof is unchanged.** `Prove` stays as it is, line for line. Its
result is a `Proof` whose new field `grade` reads `enrolled`.

**The weak proof is Enroll's own walk without the write.** A new function,
`ProveTerminal(root, invokerPID, reader, now)`, performs exactly what `Enroll`
performs before it writes: the invoker is read stably and must have a
controlling terminal; the operating system's session leader is read; the walk
from the invoker to that session leader reads every node twice, refuses any
adapter signature at any node, and refuses a node whose terminal differs from
the invoker's. It reads no enrollment. It records the same node list, with
`TerminalRef` set to the session leader it reached, `TerminalGeneration` zero,
`Outcome` `HUMAN_AUTHORITY_PROVEN`, and `grade` `terminal`. `Enroll` becomes
`ProveTerminal` plus the write, so the two can never drift. No new outcome
constant is added: the grade is a field of the proof, not an outcome, and the
refusal register's exclusion list is untouched.

Two predicates. `ValidFor(root)` keeps its meaning: the full grade only, so
every caller that checks it today keeps the full walk until this page changes
it on purpose. `TerminalValidFor(root)` accepts either grade. The fixture proof
(`FixtureGoalProof`, line 126) carries the enrolled grade and satisfies both,
as it does today. Proof records written before this change carry no grade and
are read as enrolled.

**Where the grade is enforced: in the engine, not the CLI.** The proof travels
on the request: `VerbRequest` gains `Authority *humanauthority.Proof`, set by
the CLI at request-build time (it already builds the proof there,
`syncReqWithProof`). One helper in the goal package,
`(VerbRequest).requireHuman(grade)`, replaces every `r.Actor.Human == ""`
test: it refuses when `--by` is absent, when no proof travels, or when the
proof's grade is below the verb's. The verbs that take an explicit `proof`
parameter today (approve, unapprove, set-budget, accept-risk, set-obligation,
set-priority, resume, split) read it from the request instead; the parameter
goes. `Publish` is not touched.

The CLI's shared request builder changes in one way: whenever `--by` is
present it proves at least the terminal grade, whether or not a lineage was
supplied, and derives the lineage from the proof when none was supplied. A
full-grade act derives `terminal-<id>-<generation>` from the enrollment as
today; a terminal-grade act at an unenrolled or differently enrolled terminal
derives `terminal-<id>-0`, where 0 says "not an enrolled generation". The
lineage feeds only the operation id suffix; the history actor stays
`human:<name>`.

**The classifier gate stays where it is.** `metasystem stop`, `metasystem
arm`, `steward arm`, `steward restart`, `brain declare` and `brain withdraw`
keep the lease classifier as their gate and their landed refusal grammar. This
is a deliberate deviation from one-owner-per-proof: the stop design settled
that gate (its section 7), the brain and supervision beds pin its texts, and
those verbs write no goal-ledger line naming a human. The walk and the
classifier agree on what matters, that an agent's own process tree never
passes.

### What remains impossible

- From an agent's own process tree, neither grade passes: every node is
  matched against every installed adapter signature, so the agent's process
  is `AGENT_IN_AUTHORITY_CHAIN` whatever it types, `--by` included.
- A `--by` that reaches the ledger without a proof of the verb's grade: the
  engine helper refuses it, however the lineage arrived. This closes the
  steal, foreign release, classify-sweep, set-pin and reconcile gap above.
- A headless caller (no controlling terminal) passes neither grade:
  `TERMINAL_NOT_REACHED` at the invoker.
- A proof file presented later as authority: parsed proof JSON has no
  authority (`Valid` requires the in-process `observed` flag, line 107), for
  either grade.
- An agent stopping or releasing work it does not own under its own name:
  those rows still require `--by`, and `--by` now always requires a proof.

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

## 1. The grading table

The rule from the goal's DONE line: acts that only stop, park or release work
take the terminal grade; acts that grant or widen authority keep the enrolled
grade. For acts the two classes do not name, the tie-break used here: an act
takes the enrolled grade when it changes what a seat is permitted to spend
(approval, budget, obligation, risk acceptance, tier) or who holds authority
over a goal or the ledger; it takes the terminal grade when it only halts,
parks, releases, or reorders work that is already approved.

| Act | Grade | Why it sits there |
|---|---|---|
| goal approve (--id and --sweep) | enrolled | grants execution authority and binds a budget; the DONE line names it |
| goal unapprove | enrolled | named by the DONE line; it rewrites the approval record, and a wrong withdrawal parks a claim mid-work, so the re-approval must come from the same place |
| goal set-budget | enrolled | widens the box |
| goal accept-risk | enrolled | lets a severe or unproven finding land |
| goal set-obligation | enrolled | binds a governed recurrence: standing spend |
| goal resume | enrolled | reopens admission; see the paragraph below |
| goal classify-sweep --confirm | enrolled | sets tiers, and a tier sets a box |
| goal steal | enrolled | creates a claim on another machine, which is admission to spend there; a release plus the seat's own claim is the terminal-grade way to move work |
| goal reconcile --by | enrolled | republishes hand edits, which can write any field of any goal, budgets and approvals included |
| goal repair --accept-remote | enrolled | replaces the accepted tip; a rewind can erase a stop fence or revive an approval |
| goal enroll-terminal | its own walk: the terminal grade on the terminal being enrolled, then the write | no prior enrollment, as today; the DONE line's "unchanged" is this walk |
| goal release of another lineage's claim | terminal | only halts |
| goal set-priority | terminal | reorders work that is already approved; grants nothing; another set-priority corrects it |
| goal set-pin | terminal | the same: it directs which machine claims approved work |
| goal migrate | terminal | the cutover precedes every enrollment by construction (`enroll-terminal` requires the synced backlog, `goalsync_mutations.go:1227`); today it has no proof at all |
| the other `--by` rows in `verbs.go` (a foreign park, unpark of a human's park, done of a human-origin goal, edit of a parked or foreign goal, detach, set-arc) | terminal | each moves or concludes work already approved; the helper is one function, so the sweep is mechanical |
| session stop | terminal | only halts; the wedge of the goal's first defect |
| metasystem stop | terminal (classifier) | only halts; unchanged |
| metasystem arm, steward arm, steward restart | terminal (classifier) | starts nothing that approval did not admit: the steward dispatches only approved goals under their boxes; unchanged |
| brain declare | terminal (classifier) | narrows what a checkout may do; unchanged |
| brain withdraw | terminal (classifier) | restores the ordinary state and grants nothing beyond it; unchanged |

**Resume is widening, not continuing.** A breach stop is the machine's own
finding that a budget was breached; it closes admission. Resume creates a
new execution revision under the standing approval (`stop.go:412`), and every
job reserved after it is spend the stop had refused. The elapsed clock and the
attempt count start again on the new revision. That is the shape of a grant,
and the DONE line places resume with the grants. The wedge of the brief's
fifth defect is not solved by weakening resume; it is solved by sections 4
and 5, which leave nothing for a human to type at the enrolled terminal
merely to free a seat.

**Set-priority and set-pin** are the one placement the DONE line's two classes
do not decide. They are placed on the terminal grade because they permit
nothing new: every goal they order is already approved, and a wrong order is
one more command to undo. If the critique reads "directs spend" as "widens
authority", they move to the enrolled grade with no other change.

## 2. The enrollment shape: one terminal, and every refusal names it

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
terminal.** At refusal time the engine resolves the enrolled terminal live
first: is the enrolled terminal process alive (the same kernel probe the walk
uses, by pid and start identity), and, if so, what tty and tmux session does
it have now (the same two resolvers); when the process is dead, the recorded
`tty` and `tmuxSession` are used and the refusal says the terminal is no
longer alive. It names the current shell's tty the same way. The name a human
recognises is the tmux session name when there is one, the tty otherwise, and
the enrollment date. The one command it prints is the enroll command, because
that is the command that makes this terminal the enrolled one; the act
itself is run again afterwards, and the second line says so in words. The
texts are in section 3.

The resolvers are injectable (a function value beside the `Reader`), so the
package tests name terminals without a tmux server.

## 3. The refusal texts for the authority layer's codes

The grammar is the one goal human-goal-verbs-forgiving settled for the goal
verbs and the stop design settled for the process verbs: two lines on stderr,
the first one plain sentence, the second either `run: <one command>`, with an
optional prefix in words saying where to run it and an optional `; then
<words>`, or `no command completes this: <words>`. That goal's design names
the function, `refuseHumanVerb` in `cmd/metasystem/goal_refusal.go`; this page
adds the rows for the authority layer's codes to it. Whichever chain lands
first creates the file; the other adds rows. `<name>` is the `--by` value the
verb saw, or, once that design's default lands, the enrolled human's recorded
name; until then the literal `<your name>`. `<root>` is printed only when it
is not the working directory. Every other refusal of every goal verb belongs
to that goal and is not redesigned here.

The engine keeps returning its codes as it does (the register's sites stay
valid); the CLI renders the two lines by code and grade. The lineage
derivation's own three refusals (`goalsync_mutations.go:81,101,104`) are the
same three situations as the rows below and are rendered by them, once.

| Code and situation | Line 1 | Line 2 |
|---|---|---|
| `AGENT_IN_AUTHORITY_CHAIN`, either grade | `goal approve: this command runs under an agent (claude, pid 18533); an agent's shell cannot carry a human's word, with or without --by.` | enrolled-grade act: `at the enrolled terminal, run: metasystem goal approve --root <root> --id <id> --by <name>`; terminal-grade act: `at an agent-free terminal, run: <the same command>` |
| `TERMINAL_NOT_REACHED`, enrolled grade, no enrollment exists | `goal approve: no terminal is enrolled in this checkout yet; enrolling needs no prior enrollment.` | `run: metasystem goal enroll-terminal --root <root>; then run this command again` |
| `TERMINAL_NOT_REACHED`, enrolled grade, the enrolled terminal is alive and this is another terminal | `goal approve: this is not the enrolled terminal. The enrolled terminal is tmux session "human" (/dev/ttys010), enrolled 2026-09-06 06:37Z, and it is alive; this shell is /dev/ttys014.` | `run it there, or make this terminal the enrolled one first: metasystem goal enroll-terminal --root <root>` |
| `TERMINAL_NOT_REACHED`, enrolled grade, the enrolled terminal is dead | `goal approve: this is not the enrolled terminal. The enrolled terminal was tmux session "human" (/dev/ttys010), enrolled 2026-09-06 06:37Z, and it is no longer alive; this shell is /dev/ttys014.` | `run: metasystem goal enroll-terminal --root <root>; then run this command again` |
| `TERMINAL_NOT_REACHED`, enrolled grade, the enrolled terminal is alive but its session leader changed (a new login on the same tty) | the same as the dead row, with "and its login has been replaced" in place of "and it is no longer alive" | the dead row's second line |
| `TERMINAL_NOT_REACHED`, terminal grade (no controlling terminal on this shell, or the walk left the terminal before the session leader) | `goal release: this shell has no controlling terminal, so no human can be at it.` | `at a terminal of this host, run: metasystem goal release --root <root> --id <id> --by <name>` |
| `TERMINAL_NOT_REACHED` from `goal enroll-terminal` (no terminal) | `goal enroll-terminal: this shell has no controlling terminal, so it cannot be enrolled.` | `at a terminal of this host, run: metasystem goal enroll-terminal --root <root>` |
| `ANCESTRY_UNREADABLE`, `ARGV_UNREADABLE`, either grade | the sentence `processReadRefusal.Error` builds today (pid, executable, owner, the reason), verb-prefixed | `no command completes this from this shell: run it from a shell whose ancestry up to its session leader is owned by you, for example a shell inside a tmux session you started from Terminal` (the existing workaround sentence, `authority.go:369`) |
| `ANCESTRY_CHANGED`, `PROCESS_REUSED`, either grade | `goal approve: a process in this shell's ancestry was replaced or changed its arguments between two reads.` | `run: <the same command>; then, if it repeats, a process above this shell is being restarted under you` |
| `ANCESTRY_CYCLE`, either grade | `goal approve: the process table reports a cycle in this shell's ancestry, which no kernel produces, so it cannot be trusted.` | `no command completes this: report it with the output of ps -axo pid,ppid,command` |
| `APPROVAL_REQUIRED` at a claim, dispatch or steal (`approval.go:266`) | the engine's sentence as today: `APPROVAL_REQUIRED: goal <id> is queued and not approved for execution; only the human approves it with goal approve -- this claim is refused` | `at the enrolled terminal, run: metasystem goal approve --root <root> --id <id> --by <name>` |
| `APPROVAL_REQUIRED` at resume, a typed tuple that differs from the ledger (`stop.go:392`, reshaped by section 4) | `goal resume: resume keeps the approved budget 1d/10/1200m/1/3 as it stands; the tuple typed was 2d/10/1200m/1/3.` | `run: metasystem goal resume --root <root> --id <id> --by <name>; then, for a different box, goal set-budget` |
| `APPROVAL_EXPIRED` (`approval.go:360`), a relayed approval from before the cutoff | the engine's sentence as today | `at the enrolled terminal, run: metasystem goal approve --root <root> --id <id> --by <name>` |
| `RELAY_AFTER_ENROLLMENT` (`approval.go:411`) | retired with the relay class (section 6); the row leaves the register | |
| `session stop`, either code | the same sentences with the `session stop:` prefix, and `metasystem session stop --by <name>` as the printed command; the classifier's own refusal (`session_stop.go:63`) keeps its sentence and gains the `at an agent-free terminal, run:` line |

The compact budget form in the resume row is the one goal
human-goal-verbs-forgiving defines (`FormatBox`); until it lands, the five
values print as `elapsedLimit=1d attemptLimit=10 ...`, the ledger's own
spelling, which is what the human copies from today.

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
publishes are unchanged (the tuple, now the ledger's), so the history line and
the recovery replay read as before.

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
surface goes.** Ruling R-82-m1b keeps the horizon at 2026-09-06 and forbids
depending on the path again without a new row. A flag that can never succeed
breaks this page's own rule (every refusal prints a command that would
succeed), so slice 2 removes `--temporary-human-word` and `--review-by` from
the goal verbs and from `metasystem arm`, and deletes the class with them: the
temporary outcome and its validators (`authority.go:36`, `213` to `311`), the
temporary branches of `AuthorizesResume` and `AuthorizesSetObligation` (which
then equal `ValidFor` and go), `ProveOrTemporaryGoalAuthority`, the
`temporary` legs of approve, unapprove, set-budget, resume and set-obligation,
`repeatedRelayedActError`, `RELAY_AFTER_ENROLLMENT`, `steward.ArmTemporary`,
and the tests that pin them. What stays: reading an `authority=relayed`
approval already in a ledger, its `APPROVAL_EXPIRED` refusal, and the steward
identity's `temporary-word` provenance on read, because records exist with
them. The `brain-human-word-refuses` scenario of `goal-cli-fixtures.sh` drops
its three relayed lines (521, 524, 534); the brain fence's sentence is
unchanged. Goal fixture-review-by-date-rolls-over's residual question is
answered by this section.

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
`proveSessionStopHuman`, `proveSyncReqHumanAuthority`, `runGoalEnrollTerminalWith`).
The headless beds cannot drive the walk's refusals: the walk reads the real
process tree through `KernelReader`, which ignores the fixture identity table
on purpose (`identity/terminal.go:16`), so a bed's verdict would depend on
who runs it. The bed's part is the fixture-authority success path, below.

The tree reader needs three shapes it does not have. A second terminal:
pids 60 (`terminal-session-2`, parent 1, `tty-2`), 61 (`interactive-shell`,
parent 60, `tty-2`), 62 (`command-wrapper`, parent 61, `tty-2`), which needs
`SessionLeader` to answer per pid (a `sessions map[int64]int64` beside the
single `session` field). A headless shape: pid 70 (`command-wrapper`, parent
71) and 71 (`scheduler`, parent 1), both with an empty terminal id. A dead
enrolled terminal: `enrolledReader` enrolled on `tty-1`, then pids 10 and 20
deleted from the snapshot map. The terminal-name resolvers are injected as
function values returning fixed names.

The four canaries, each one Go test with a ceiling:

1. **An agent shell is refused for an approve and for a stop.**
   `TestAgentShellRefusedForBothGrades` in `internal/humanauthority`: pid 50
   (`shell-wrapper`) under 40 (`codex-agent`) is `AGENT_IN_AUTHORITY_CHAIN`
   from both `Prove` and `ProveTerminal`; the CLI twin
   `TestApproveAndSessionStopRefuseAnAgentShell` in `cmd/metasystem` asserts
   the two lines of section 3 for `goal approve` and `session stop`.
   `go test ./internal/humanauthority ./cmd/metasystem -run
   'RefusedForBothGrades|RefuseAnAgentShell' -count=1 -timeout 120s`.
2. **A human at an unenrolled terminal may stop and may not approve, and the
   enroll command is printed.** `TestUnenrolledTerminalStopsButCannotApprove`
   in `cmd/metasystem`: no enrollment file; `session stop` through the seam
   with `ProveTerminal` writes its marker; `goal approve` refuses with the
   no-enrollment row, line 2 `run: metasystem goal enroll-terminal --root
   <root>; then run this command again`. Same command shape, `-timeout 120s`.
3. **The printed command runs and the approve then succeeds.**
   `TestPrintedEnrollCommandThenApproveSucceeds` in `cmd/metasystem`: the
   test takes the command from line 2 of canary 2, runs it through
   `runGoalEnrollTerminalWith` with the same reader, then runs the approve
   with the same reader; the Approved line reads `authority=proven`, the
   proof record's grade is `enrolled`, and the fleet enrollment is recorded
   once. `-timeout 120s`.
4. **A dead enrolled terminal is recovered by one enroll from a fresh
   terminal.** `TestDeadEnrolledTerminalRecoversByOneEnroll` in
   `internal/humanauthority` and its CLI twin: enroll on `tty-1` (pid 20);
   delete pids 10 and 20; `Prove` from pid 62 is `TERMINAL_NOT_REACHED` and
   the CLI prints the dead row naming the recorded tty; `Enroll` from pid 61
   yields generation 2; `Prove` from 62 is proven. `-timeout 120s`.

Their twins, same instruments and ceilings:

- `TestOtherLiveTerminalRefusalNamesTheEnrolledOne`: both terminals alive,
  the injected resolver returns tmux name `human`; the alive row prints
  `tmux session "human" (/dev/ttys010)` and the enroll command.
- `TestSessionLeaderReplacedRefusalSaysSo`: `sessions[20]` changed to 11; the
  replaced-login row.
- `TestHeadlessCallerFailsBothGrades`: pid 70; `TERMINAL_NOT_REACHED` from
  both, and the terminal-grade row for `goal release`.
- `TestByWithoutProofIsRefusedWhateverTheLineage` in `internal/goal`: steal,
  foreign release, `InstallTierLaw`, `ClassifyTier`, `SetPin` and `Reconcile`
  with `Actor.Human` set and no `Authority` refuse; with a terminal-grade
  proof, release, set-pin and set-priority land and steal, classify-sweep and
  reconcile still refuse; with an enrolled-grade proof all land. The CLI twin
  exports `METASYSTEM_OWNER_LINEAGE` and passes `--by` from the agent shape.
- `TestResumeTakesTheLedgerBudget` in `cmd/metasystem`: resume with `--id`
  and `--by` alone confirms; with an equal tuple confirms; with a different
  tuple prints the standing values and the command without the tuple.
- `TestStoppedClaimDoesNotHoldTheSlot` is goal breach-stop-wedges-seat's own
  fixture; this page adds `TestResumeRefusesASecondLiveClaim` in
  `internal/goal` beside it.
- `TestTemporaryWordFlagsAreGone` in `cmd/metasystem`: `goal approve
  --temporary-human-word x --review-by 2026-09-30` fails to parse and the
  forgiving rule's flag-parse row prints the command without them.
- `TestProofGradeRoundTrips` in `internal/humanauthority`: a recorded
  terminal-grade proof re-reads as terminal, a record without the field reads
  as enrolled, and neither parsed record is `Valid`.

The bed: `scripts/agents/goal-cli-fixtures.sh` gains one scenario,
`proof-grades`, in its list: under `--fixture-human-authority` (the flag is
registered on release, steal, resume, set-pin, set-priority, reconcile and
classify-sweep, which the bed cannot drive today) a foreign release, a steal, a
resume without a tuple and a set-priority confirm and their history lines read
`human:Wido`; `metasystem stop` and `session stop` keep their existing
scenarios (`wrong-terminal` and the supervision bed's seat-refused). Ceiling:
`bash scripts/agents/goal-cli-fixtures.sh` runs its whole list serially, about
100 seconds today; 300 seconds is the hang bound. The landing receipt is the
one the goal record names: the go gate plus the human-authority package and
the goal CLI fixtures; no full battery.

## 8. Scope of slice 2

Changes: `internal/humanauthority/authority.go` (`ProveTerminal`, the grade
field and predicates, `Enroll` as the walk plus the write, the two record
fields, the resolvers, the relay class removed); `internal/goal` (the
`Authority` field and `requireHuman` helper on the request, the sweep over
`r.Actor.Human == ""` sites, the resume budget, the quota predicate's grade
row, the resume quota check, the session stop store predicate, the relay legs
removed); `cmd/metasystem/goalsync_mutations.go` and `goal_refusal.go` (prove
on every `--by`, the lineage rule, the refusal rows, the flags removed, the
fixture flag registered where the bed needs it); `cmd/metasystem/session_stop.go`
(`ProveTerminal`); `internal/steward/runner.go` (`ArmTemporary` removed);
`internal/refusal/register.go` (the `RELAY_AFTER_ENROLLMENT` row removed;
`TERMINAL_NOT_REACHED` and the others keep their sites); the fixture beds and
package tests named above; the help lines of the graded verbs, which say
"human-only, at any terminal" or "human-only, at the enrolled terminal".

Unchanged: `Prove` itself; every record line format of the ledger and the
intent arguments each verb publishes; the fixture authority path and its root
binding; the verified channel word and where it binds; the process verbs'
gate and refusal grammar; the brain's gate; the breach-stop batch, the fence
and the stop's own budget law; what approval means; the claim quota's purpose.

The one part of this slice the orchestrator may cut into its own slice without
loss is the relay-class deletion (section 6): the grading and the refusals do
not depend on it, only the flag removal does, and the flags can go first.
