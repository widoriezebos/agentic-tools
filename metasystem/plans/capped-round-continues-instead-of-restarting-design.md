# A round cut off at its cap is continued, not restarted (goal capped-round-continues-instead-of-restarting)

- Status: revision 3 after two design reads (section 6); the second read's three material findings are values and bounded choices folded one-to-one into section 4, so under R-97-m1e the build started behind section 4 with no third round; built 2026-09-12 (the dispatch bed passes with the four new scenarios)
- Goal: capped-round-continues-instead-of-restarting (goal 21 of plans/delivery-efficiency-plan.md)
- Next step: build, one code read, land; Wido's word on section 3's two dispositions

## What is true today (read in the tree)

A round is killed by its record alone: the reaper judges `budgetExpired`
(`capDeadline`, else `startedAt` plus `capMin`) before it looks at process
liveness, winds the owned process group down and writes
`status=timeout error=budget-cap phase=supervision groupDeathProvenAt=…`
(scripts/agents/dispatch.sh, the reap verdict; internal/dispatch/reapfacts.go).
A round whose process vanished writes `status=failed error=process-lost`,
whether it had handshaken or not; a refused return is `failed` with
`protocol_error`; a returned round is `completed`; a person's stop is
`cancelled`. A reader and a script already tell a cap hit from the others
by `status` and `error`, so nothing new is recorded on the capped round.

The job worktree survives the kill. Chain rounds open one worktree
(`artifacts/agents/worktrees/<root job>`) and never commit, so the dead
round's changes sit in the working tree exactly as it left them; nothing
removes delegate worktrees. `job follow-up-rebase-plan` reads the chain's
changed paths from the worktree's dirty set and skips a round that has no
`return.json` (internal/dispatch/followup_rebase.go). The conformance
review computes the whole base-to-working-tree diff and refuses a changed
path outside the chain's cumulative declared boundary; it does not refuse
a declared path that is unchanged (internal/validate/conformance.go). No
byte a dead round left behind can reach a landing unreviewed.

The follow-up verb refuses that worktree. `follow_up()` admits a newest
chain record that is `completed`, or `failed` with `protocol_error`, and
otherwise dies with "use a fresh dispatch after pending, running, timeout,
or process-lost". A fresh dispatch opens a worktree of its own at its own
job's path and strands the old one (`--workspace <old worktree>` is taken
as a shared-checkout launch, which prove-round then refuses), so the only
lawful continuation starts clean and the dead round's work is carried by
hand. Three worktree build
chains paid for that in one week (clbm-build1 on 2026-09-08, gawr-build2
on 2026-09-10, hcl-build1 on 2026-09-11): each time the round had finished
or nearly finished the work, and each time the coordinator moved a patch
into a fresh chain by hand, with a marker-file handshake in one case.

A follow-up's context is engine-composed. `job compose-role-packet`
accepts three fixed continuation slots, `prior-brief`, `prior-return` and
`critique-register` (internal/dispatch/composition.go, mirrored by the
composition validation in internal/dispatch/build.go). The dispatcher
passes continuation slots only on its fresh-context branch
(`resume_cap` false): `prior-return=…/rounds/<parent>/return.json`, a
file a capped round never wrote. When the snapshot says the recorded
session resumes (claude, codex and the fake runtime's default profile all
say so), the adapter's `follow-up` verb resumes it blind: `--resume
<session>` or `codex exec resume <session>`, with no check of how the
session ended. The composed packet says nothing about the round's cap:
the cap is resolved after composition (`authorize_job_cap` runs after the
packet's input hash is taken), so a delegate learns its wall-clock budget
only from the brief a coordinator wrote.

## 1. Research: what the refusal protects

The next step asked three questions before any design.

**(1) What the timeout exclusion protects.** The line is as old as the
dispatcher: it arrived with "Ship the dispatcher core" (d88a5f4d2) and no
commit, design record or document since argues it. Reading the follow-up
path, these are the things it needs from the parent round, and which of
them a dead round lacks:

- a `return.json` under the root's `rounds/<n>`: absent; the fresh-context
  packet reads it and would fail on the missing file;
- a session id on the parent record: stamped at the handshake, so present
  for a round that ran and absent for one lost while pending;
- `usage.json`: absent (adapters write it after the CLI exits); nothing on
  the follow-up path reads it;
- the critic register: a critic parent is folded before composition and
  the fold accepts only completed, failed or cancelled rounds, so a
  `timeout` critic round cannot be followed up at all;
- the chain build cache: removed once every chain member is terminal,
  which a kill makes true, so the successor starts cold, as it does today
  between completed rounds;
- the mission incarnation and the goal binding: on the record from launch.

No incident in the records shows a follow-up after a dead round doing
harm; the incidents show the refusal costing a coordinator's hour and a
job's minutes each time. The exclusion protects a composition assumption
and a register rule, not a safety property of the worktree.

**(2) Whether a half-written file is a real risk.** The wind-down is TERM,
a grace period, then KILL to the process group, so a delegate can die in
the middle of writing a file, and the successor inherits that file. It is
not a landing risk: an implementer's round carries the shared testing
requirement (focused tests and the fast gate before it returns), the
orchestrator proves the worktree snapshot with `job prove-round`, and the
conformance review diffs the whole worktree. A torn file surfaces as a
build or test failure the successor fixes, like any other defect it finds
in the tree. What the successor needs is to be told that the tree is its
predecessor's unfinished work and not a clean base.

**(3) The smaller change.** Admitting the capped round as a follow-up
parent is one admission rule in the verb that already inherits the
worktree, plus one continuation slot. Letting a fresh dispatch adopt a
stranded worktree needs a new flag, a chain re-link (the fresh job is a
new root, so the old chain's records, register and cache lose their
successor) and a worktree ownership hand-over. The follow-up is the
smaller change and the record stays one chain.

## 2. The design

**The successor is a follow-up round of an implementer chain.**
`follow_up()` admits a newest chain record that is `timeout` with
`budget-cap` when the chain's role is `implementer` and its record says
`launchMode=worktree` (the recorded mode admits, never a path shape) and
the worktree directory still exists. The
implementer is the one role whose return carries a `diffBoundary` and
whose rounds the conformance review diffs; the critic roles keep today's
refusal because their register cannot fold a capped round (a capped
critique is re-run as a fresh round under the critique cap as today), and
the verifier, investigator and steward-continuation roles keep it because
their return schemas close their property sets and no review diffs their
worktree. Pending-setup, pending and running still refuse (a live round),
unless the live round is the standing follow-up itself, which is the
repeated-operation path as today; a shared-checkout chain still refuses
(its dirty set is the coordinator's, and prove-round, conformance and the
rebase planning all refuse or skip it); a missing worktree still refuses.
Each refusal names its remedy, the fresh dispatch where that is the
remedy. Nothing else is admitted: `process-lost` stays refused in this
goal (section 3), and so do `cancelled` and `protocol_error`, which keep
their own paths. All three incidents were implementer worktree chains.
A continuation is a round like any other: it charges the goal's box (an
attempt, its reserved minutes) through the same admission every follow-up
passes.

**A continuation always composes fresh context.** A killed session is a
broken transcript; the adapters resume blind and a failed resume would
land `failed` with `runtime_error` or `handshake_timeout`, which nothing
admits, stranding the worktree a second time. So a continuation after a
cap takes the fresh-context branch regardless of the snapshot's `resume`
capability: adapter verb `dispatch`, `resumeMode=fresh-context`, the
packet carrying `prior-brief` and the new `prior-worktree` slot, never
`prior-return`: a return written in the seconds before the kill is not the
round's word, because the reaper judges the cap before recollection, and
the paragraph says the round wrote none. A repeated wrapper of a standing
continuation reads `continuation` from the standing record and composes
the same bytes from the paragraph the first wrapper kept in the round
directory. The follow-up's identity still keys on
the parent's session id, as every fresh-context follow-up does today; a
capped round has one, stamped at its handshake, because the reaper's
timeout verdict needs a running round. The adapters are untouched, and
the verb a continuation uses is the one every runtime runs on every
fresh dispatch. Resuming the killed session as an accelerator is a later
question, not this goal's.

**The successor is told, by the engine.** A new engine continuation slot
`prior-worktree` joins the three fixed slots; the composition validation
accepts it as `engine:prior-worktree`. `metasystem job cap-continuation
--root . --parent <record> --worktree <path> --output <file>` writes the
paragraph the packet carries under that slot. It states the fact and what
the tree holds, and one rule the review needs, nothing more: round <n> of
this chain was cut off at its <N>-minute cap at <groupDeathProvenAt> and
wrote no return; the worktree already holds its work, <K> paths changed
against the worktree's HEAD at composition (the trunk commit the
follow-up rebase moved it to, `rebasedTo`, when it moved), the set the
rebase planner already reads (tracked changes plus untracked files),
listed one per line with a path that carries whitespace or a control
character quoted (a filename cannot carry a line into the engine's slot),
capped at 200 entries and eight kilobytes with the total named; because the review diffs
the whole worktree and the predecessor declared no boundary, your
`diffBoundary` must list every changed path of the chain, the
predecessor's included. The record's `baseSha` is not the base: it never
moves, while the follow-up rebase moves HEAD before composition, and a
listing against it would name every trunk change since the branch point
as the predecessor's work. The shared testing requirement travels as it
does for every implementer round; the slot does not repeat it. The verb
refuses a parent that is not an implementer round in `timeout` with
`budget-cap`, and a worktree that is missing.

**The successor's record.** `build-follow-record` writes
`continuation=after-cap` beside `parentJob` (the field is absent on an
ordinary follow-up and immutable once written), so a script tells a
continuation from a correction without reading the parent; the record
owner refuses the field unless the parent is an implementer round in
`timeout` with `budget-cap`, the mode is fresh context, the launch mode is
worktree, and the packet carries `prior-worktree` and no prior return. The
follow-up's `--message` stays mandatory and is the coordinator's word on
what remains (one line is enough); on a headless node the same verb is
what the steward continuation would call, and launching it unattended is
out of scope here (section 3).

**The return-by line (the absorbed clause of rounds-return-before-the-cap).**
The engine's generated runtime notice gains one sentence when the cap is
known at composition: "Your round is capped at <N> minutes from its
reservation; write your return by minute <N minus M>, naming what is
left." It is a function of `capMin` and M alone: no clock time enters the
bytes, because a repeated operation mints a fresh `capDeadline` at every
authorization and the fingerprint binds the input hash, so a deadline in
the packet would refuse every retry as an identity mismatch. M is
`dispatch.return-margin-min` (default 10, validated with the other knobs;
zero asks for the return by the cap itself); when M is not below N the
sentence is omitted, so the one-minute fixture caps say nothing; when a
mission reservation's cap was truncated by the wall clock
(`capResolution.truncatedBy` set), the minutes are not the true budget
and the sentence is omitted too, and a repeated operation reads the
standing record's truncation so its bytes match. To know N the dispatcher authorizes the
cap before it composes: `acquire_cap_authority_lock` and
`authorize_job_cap` move ahead of `job compose-role-packet` in both
`dispatch()` and `follow_up()`, and the input hash covers the line.
Identities do not move: the claim fingerprint already binds `capMin` and
`inputHash` together, a repeated operation reads the reservation's
`capMin` and composes the same bytes, and non-mission resolution is
config-determined; a repeated follow-up preflights `PREFLIGHT-MATCHED`
as today (section 4).
Two side effects are recorded: the cap-authority lock is held across
composition (milliseconds), and a composition refusal (a context source,
an oversized input) now happens while a mission-fence authorization
exists, which the verb's exit trap releases as it releases every other
unpublished authorization. Lock order is unchanged. The margin is a
sentence, not a fence: a round that runs on is still reaped at its cap and
continued by this design.

**Out of scope, and why.** Raising caps (needs supervision re-armed with a
higher ceiling); reconstructing what the killed round was thinking (the
worktree carries the work; the transcript is not resumed); the mission
fence's refusal when a mission job hits its cap (a mission rule,
unchanged); adopting a stranded worktree into a fresh dispatch (not needed
once the follow-up admits); launching the continuation unattended (the
steward continuation's call, a separate goal); cancelled rounds (a
person's stop is not continued by a rule).

## 3. Alternatives not taken, and the absorbed clauses

- Recording the cap hit as a new terminal status: `timeout` with
  `budget-cap` is already distinct; a new status touches the lawful
  transition graph, the watcher, the reports and the wait exit codes for
  a fact the record carries.
- Admitting `process-lost` beside `budget-cap` (the absorbed clause of
  timed-out-round-resumes-as-a-follow-up named both): refused here, for
  two reasons found in the tree. A round lost while pending has no session
  id and no follow-up can be built for it; and a follow-up round that
  wrote its return and then lost its process is recorded `process-lost`,
  not `completed`, because both recollection paths look under the reaped
  job's own id while rounds two and up write under the root's payload, so
  the rule would call a returned round "no return". That recollection
  defect is a proposal in memory/backlog-notes.md; once it reads the
  root's rounds, admitting a process-lost round that ran and returned
  nothing is the same one-line rule as this one. Wido's ask and all three
  incidents are cap hits.
- Recording the capped round's terminal tree and diffing the successor
  against it (the second absorbed clause): refuted for this goal. Every
  round's review already diffs the whole worktree against the chain base
  and prove-round proves the whole snapshot; a per-round tree would add
  attribution the review does not use. The successor's boundary rule
  (section 2) is what the review needs. Put to Wido at the landing beside
  the process-lost disposition.
- Resuming the killed session: refused above (a blind resume of a broken
  transcript lands a failure nothing admits).
- A coordinator-written marker file or "continue, do not restart" line:
  the live incidents did exactly that by hand; the packet slot does it
  the same way every time.

## 4. Fixtures that prove it

- scripts/agents/dispatch-fixtures.sh: an implementer chain in a job
  worktree whose fake round writes a file into its workspace
  (`FAKE:worktree-file=<relative path>`, a new fake behaviour that writes
  through the guarded write in round 1 only) and then holds
  (`FAKE:cap-hold-round=1`, a hold in that round only, so the continuation
  that inherits the brief completes); the fixture backdates the
  record and reaps it to `timeout` with `budget-cap`; `follow-up --job`
  then succeeds where it refused before: round 2's prompt carries
  `# Prior Worktree`, "cut off at its" and the file's path; round 2's
  composition record lists `engine:prior-worktree` and no
  `prior-return`; round 2's record carries `continuation=after-cap`,
  `resumeMode=fresh-context` and the parent job; the file is still in the
  worktree when round 2 completes; the chain's newest record is
  `completed`. The same case runs under the fake's `old` profile (no
  resume capability) with the same assertions, so the fresh-context floor
  is proven where the default profile would have resumed. A second case
  removes the worktree first and the follow-up refuses naming the fresh
  dispatch. A third case caps a design-critic round and the follow-up
  still refuses, naming the rule (implementer worktree chains). A fourth
  case, on the `rebase-wt` pattern, caps a round in a worktree that is
  behind trunk on unrelated files: the continuation's paragraph lists the
  chain's paths only, none of trunk's. The happy implementer dispatch
  (conf cap 120, margin 10) has "write your return by" and "minute 110"
  in its prompt; the one-minute timed round has no return-by line; the
  repeated follow-up (`repeat-rebase-wt`) still binds to its standing
  operation with the sentence in its bytes; an implementer's first round
  at the conf cap (`rebase-wt`) carries "minute 110"; a repeated wrapper
  of a standing continuation binds to it, and a continuation cut off at
  its own cap is continued again (`capped-repeat`, rounds 2 and 3). The
  truncated-mission replay (the standing record's truncation read on a
  repeat) has no bed of its own: mission chains run only in the mission
  runner scenario, which does not repeat a follow-up.
- internal/dispatch: `ComposeRolePacket` accepts `prior-worktree` and
  refuses any other new slot; the composition validation accepts it; the
  cap-continuation text names the round, the cap, the death time, the
  worktree head, the path count and the paths, and caps the list; the
  follow record carries `continuation` (a `--continuation` flag on
  `build-follow-record`, set at build like `resumeMode`); the runtime
  notice carries the return-by sentence for a known cap and omits it when
  the margin is not below the cap or the cap was truncated.
- cmd/metasystem: `job cap-continuation` refuses a parent that is
  completed, running, cancelled, `process-lost` or `protocol_error`, a
  parent whose role is not implementer, and a missing worktree.
- Runtimes (plan principle 0): the adapters are untouched; the
  continuation uses the `dispatch` verb every runtime already runs, and
  the fact rides the engine's packet; the fake bed proves the shape end to
  end on both snapshot profiles. A live continuation on claude or codex
  after a one-minute-cap kill is run at the landing when the runtime is
  dispatchable from this Mac, and the design page records the result;
  it is evidence, not the proof, because no runtime path changes.
- docs/orchestration.md's corrections paragraph names the continuation
  after a cap and the return-by line; the dispatch.sh usage and refusal
  text match.

## 5. What the second read attacked

Whether scoping to worktree build chains and `budget-cap` alone leaves a
real incident uncovered (no: all three were implementer worktree chains);
whether always composing fresh context loses anything a resumed session
would have carried (no: the fresh-context branch needs the prior brief,
the permissions, the reach class, the tier and the session id, all on the
capped record); whether the continuation paragraph is the fact and the
one earned rule only (yes, once the base is the worktree head and the
rule is spoken to implementers alone); whether moving the cap ahead of
composition changes any identity (only if a clock time entered the
bytes, which it now does not); and whether the fixture list proves each
sentence of section 2.

## 6. Critique record

Round 1, design-critique subagent, 2026-09-12: seven material findings,
all folded. M1: recollection never concludes a follow-up round that
returned (both paths look under the job's own id), so the process-lost
premise was false for rounds two and up. M2: a round lost while pending
has no session id. M1 and M2 together: process-lost is refused in this
goal and the recollection defect goes to the proposals. M3: a critic
parent cannot be folded from `timeout`; the rule is scoped to non-critic
roles. M4: the slot was placed on no branch; a continuation now always
composes fresh context and the fixture runs on both snapshot profiles.
M5: a blind resume of a killed session strands the worktree again; fresh
context always. M6: a shared checkout is not a floor the slot can
describe; worktree chains only. M7: the terminal-tree clause was dropped
without a disposition; refuted in section 3 and put to Wido. Eight minor
findings folded: the consumer list, the paragraph trimmed to the fact and
the earned rule, dead defined exactly, the return-by line quotes the
deadline and names its fixture numbers, the two side effects of the
reordering, the mandatory message, cancelled rounds out of scope, and the
goal record's truncated clause (its text lives in the ledger's history).

Round 2, design-critique subagent, 2026-09-12: three material findings,
all folded; under R-97-m1e no third round. M-1: the return-by sentence
quoted an absolute deadline that a repeated operation cannot reproduce
(the fingerprint binds the input hash) and a wall-clock-truncated mission
cap overstated the budget; the sentence is now a function of the cap and
the margin alone, omitted when the cap was truncated. M-2: the listing's
base was the record's immutable `baseSha` while the follow-up rebase
moves HEAD before composition; the base is now the worktree's HEAD at
composition. M-3: the role scope spoke the boundary rule to roles whose
return schemas refuse a `diffBoundary`; the admission is implementer
only. Seven minor findings folded: the false index clause in the
terminal-tree refutation removed, the fresh-dispatch mechanism reworded,
the standing follow-up named as the repeated-operation path, the
session-id fact recorded, the `--continuation` flag named, the box charge
named; no fixture pins the notice bytes.

Code read, Codex (gpt-5.6-sol, codex-rescue, read-only), 2026-09-12:
eleven material and five minor findings, all folded and tested except
where named. Material: the untracked engine files ride the tree-to-tree
landing diff (not a code change); a repeated wrapper of a standing
continuation lost its continuation and could not bind; a repeated
wall-clock-truncated mission cap composed different bytes; a return
written just before the kill rode the packet; the record owner did not
hold the after-cap invariant and the field was patchable; the launch mode
was inferred from a path; the paragraph rendered raw pathnames, had no
byte bound and carried a second imperative; a zero margin removed the
sentence; the verb lacked `--root` and accepted positional arguments;
five named proofs were weaker than stated; round two could recreate the
predecessor's marker. Minor: the paragraph temp leaked on a refusal; the
fake mutated directories before the guarded write; the knob was not
validated; ordinary follow-up records carried `continuation: null`; the
docs overstated the line's coverage and the old-engine behaviour. Two
landing-time notes: the truncated-mission replay fix has no bed; the
compatibility wording now says an older engine cannot create or repeat a
continuation round.
