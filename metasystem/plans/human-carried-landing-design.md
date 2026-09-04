# human-carried-landing — design: the machine never refuses a verified human (revision 2)

Goal: plans/goals/human-carried-landing.md. Ruling: R-75-m3 (Wido,
verbatim: "I do not ever want to be in a HAL2000 situation where the
computer refuses under a situation judged as critical for the human";
and on the fast track: "I'm considering a 'Just Fucking Do It' risk
tier for hot-patch scenarios that need to be fast-tracked under close
human scrutiny ... And it must also be fully authorized by the human
explicitly."). Parent designs: plans/severity-tiered-rigor-design.md
(the tiers, the tier-1 receipt of STR2-TIER1-EVIDENCE-13, the five-member
box) and plans/severity-tiered-rigor-p2-design.md (the four risk answers,
the review obligations, `goal accept-risk`, the exception counter), both
law on main since b4ae9395. Every cite below is re-read at main
798935b2.

Revision 2 rewrites revision 1 in one pass after the first critique
(chain hcl-design-cc1, sixteen material findings HCL-C-01 to -16, one
note; dispositions at the end). The generating cause was one: revision
1 invented a bearer file for the human's word and left every write
between the word and the push unordered. Revision 2 puts the word where
every other human word in this tree already lives, the goal ledger, and
orders the writes.

The shape in one paragraph. Urgency changes who carries the rigor, never
the risk: the tier stays derived from the four answers, and a hot patch
to a severity-3 problem is still severity 3. What the human may do is
carry the rigor in person: speak one explicit word for one exact tree,
recorded in the goal ledger under a proof the tree already trusts, and
the machine lands that tree, with its review deferred into an obligation
on the goal, the use counted, every gate's result printed and recorded.
The machine keeps exactly one gate, identity: it must know the word is
the human's. Past that gate it verifies, records, counts and warns; it
never blocks.

### HCL-PRINCIPLE-01: refusals bind agents; a verified human is never refused

A refusal in this tree is a law for agents. For the human, every
refusal has one of three shapes and nothing else: (a) an identity
check, the machine establishing that the word is the human's; (b) a
warning, printed and recorded, after which the machine does what was
asked; (c) a question back to the human, when the word is stale or
names something that no longer exists, and the machine asks for a
fresh one. A fourth shape, the machine refusing a verified human on its
own judgement of the situation, is the HAL2000 refusal and is a defect
wherever it exists, whether or not anyone has hit it yet. Fixture:
HCL-01-THREE-SHAPES, the refusal register of 03 classifies every row as
a, b or c or names the human verb that carries past it; a row with no
classification fails the test.

### HCL-IDENTITY-02: two proofs, and the temporary word is not one of them

The word is the human's under exactly two proofs, both already owned by
this tree: the enrolled terminal (internal/humanauthority `Prove`, an
in-process ancestry observation; `Proof.ValidFor(root)`, the strictest
proof the tree has, which no parsed record can replay: authority.go:93-95,
138-143) and the authenticated channel word (a threaded reply to a
question this seat asked, bound by `channel poll` as a goal `answer`
row with AuthorityOutcome AUTHENTICATED_CHANNEL_WORD and the question's
strict `wants` token in its reason: internal/channel/poll.go:271,
internal/goal/verbs.go:119-124). The temporary recorded word
(`--temporary-human-word`, `temporaryValidFor`) records words but cannot
verify who supplied them (authority.go:145-162) and is NOT a carry proof:
a carry under it is refused with the identity shape. An agent invoking
the human verb of 03 without either proof gets the identity refusal.
Fixtures: HCL-02-TERMINAL-PROOF (the enrolled terminal writes the row),
HCL-02-CHANNEL-PROOF (the answer row with the token writes it),
HCL-02-TEMPORARY-REFUSED, HCL-02-AGENT-CANNOT.

### HCL-AUDIT-03: the refusal register

One exported table, `internal/refusal/register.go`: rows `{Code, Owner,
Shape, Override, Pending}` for every refusal code in the audited set,
with its shape under 01 and, for shape (b) and for agent-only refusals,
the human verb that carries past it (`goal resume` for a breach stop,
the human override of the p2 design's 16 for a derived tier, `goal
accept-risk` for an exhausted review budget, `land.sh --carried` of 05
for a landing gate).

The audited set is defined by a grammar, not by discipline. Test
HCL-03-EVERY-CODE-ROWED walks the Go source files (not tests) of
internal/dispatch, internal/goal, internal/goalbudget,
internal/landing, internal/steward, internal/channel,
internal/humanauthority and cmd/metasystem, and collects every string
literal that matches either `^[A-Z][A-Z0-9]*(_[A-Z0-9]+)+$` (the
UPPER_SNAKE codes, wherever they appear: as constants, as literals in
`Errorf` formats such as norm.go:95-99, or as map keys) or
`^[a-z0-9]+(-[a-z0-9]+)*-(refused|unreadable|malformed|unavailable)$`
plus the landing verdict codes passed as literals in observe.go:75-99
(matched by the second pattern or listed by hand under `landing`); the
test fails on a collected literal with no row. Outcome names that are
not refusals (HUMAN_AUTHORITY_PROVEN, TEMPORARY_HUMAN_WORD,
AUTHENTICATED_CHANNEL_WORD, the WON/REFUSED headline words, log kinds)
live in the table's `Exclusions` list with a one-line reason each, and
the test fails on an exclusion the grammar no longer collects (a dead
exclusion). Shell refusals (land.sh:94-112 and commit.sh print prose,
not codes) are outside the mechanical set: the table lists them by hand
under `Shell` rows with script and line, and the design says plainly
that no test proves that list complete.

HCL-03-EVERY-ROW-REAL fails on a row naming an override verb the binary
does not have, except a row marked `Pending: human-carried-landing`,
which slice 1 uses for the landing gates whose override is
`land.sh --carried` of slice 2; HCL-03-NO-PENDING-AFTER-SLICE-2 asserts
that after slice 2 no row is pending. A code the table classifies as the
fourth shape is listed under `Defects` and each is a backlog item opened
by the slice, never fixed silently. The table also records, per human
verb, the number of commands between the human's intent and the effect
(08).

### HCL-WORD-04: one word, one tree, in the ledger

The word is a human goal-history row, verb `carry`, on the goal that
owns the landing, with the strict token `carry tree=<sha40> goal=<id>`
in its reason (the same token discipline as the budget approvals,
internal/goal/norm.go goalNormApproval), `why=<text>` and
`expires=<RFC3339>` (default two hours after the row's At, ceiling four
hours; a longer request is shape (c), asked back). Two ways to write it:

- Terminal: `goal carry --root . --id <goal> --tree <sha40> --why
  "<text>" [--expires 2h]` from the enrolled terminal; the command
  proves ancestry in-process (`ProveOrTemporaryGoalAuthority` is NOT
  used; `Prove` alone) and writes the row with AuthorityOutcome
  HUMAN_AUTHORITY_PROVEN. It prints the row's opid and the 08 word lines.
- Channel: the seat runs `channel ask --root . --id <goal> --kind carry
  --wants "carry tree=<sha40> goal=<id>"` with the tree, the goal's
  tier and risk answers and the diff stat as facts; the human replies
  in the thread; `channel poll` binds the reply as the `answer` row
  carrying the token (existing grammar, no new envelope). The answer
  row IS the carry word; its opid is the carry's opid.

The tree is a full object id, resolved by the human's terminal or by
the seat composing the question; the ledger never stores a prefix, and
the landing compares full ids (the existing observe validation,
observe.go:32-36). One word carries one tree once: consumption is
defined in 06. A goal the landing machine does not hold is not claimed
by the carry: the ordinary claim law stands (a human moves a claim with
`goal steal`, verbs.go:1641), and the landing prints the `goal steal`
line as shape (c). Fixtures: HCL-04-ROW-FORM (terminal row complete,
token exact); HCL-04-CHANNEL-ROW (question with `--kind carry`, answer
bound with the token); HCL-04-FULL-ID-ONLY (a prefix is refused at the
verb with the ask for the full id); HCL-04-EXPIRY-ASKS (an expired row
prints the ask for a fresh one and lands nothing);
HCL-04-NOT-HELD-ASKS.

### HCL-LANDING-05: the carried landing, and every gate advisory

`land.sh --carried <opid>` runs the ordinary steps with three changes,
all in the wrapper the human already lands through.

Observe. `landing observe --carried <opid>` is the classification
`human-carried` beside `chain`, `register-carriage`, `exact-revert` and
`tier-1`. Its requirements are record checks and nothing else: the
row exists on the landing's goal, is a `carry` or `answer` row with the
exact token, is unexpired, is unconsumed (06), and names exactly the
candidate tree (the index tree commit.sh proves, `proved_tree`). A
different tree is shape (c): the verdict names both ids and the ask.
There is no chain, no critic, no closed register, no tier bar.

Advisory gates. In carried mode commit.sh runs every judgement gate it
runs today and records instead of refusing: the staged-package coverage
delta (commit.sh:226-245), the static re-proof (`go-gate.sh --fast`,
:270-271), the word audit (:383-387), the tier-1 receipt or the full
battery under width full (land.sh's receipt step), and the landing
verdict's promoted refusal bits. Each result is printed in full and
written into one wrapper-owned trailer `Carried-Battery: green` or
`Carried-Battery: red coverage=<exit> reproof=<exit> audit=<exit>
receipt=<exit>` (zero for a gate that passed). The two failures that
are not judgement stay refusals in any mode: a tree that cannot be
proved as one tree (unmerged index, :262-266) and an evaluator that
cannot be built at all (`evaluator-unavailable`) — both are record
failures with the ask printed. The build list names commit.sh, not only
land.sh, for this.

Trailers. The wrapper stamps `Carried-By: <human>`, `Carry: <opid>`,
`Carried-Tree: <sha40>` and `Carried-Battery: ...` in the colon form it
already owns (commit.sh:519-533), refuses a caller-supplied line with
any of these keys the way it scans for Goal-Item (:127-203), and its
postcondition requires exactly one of each after the commit. A commit
without a carry row cannot carry them.

Rebase. The word names a tree. If `fetch origin` shows origin/main
moved past the commit's parent, the rebase produces a tree the human
did not name: the landing rebases, prints the new tree id, does NOT
push, and asks (shape c): for a channel word it posts the `--kind
carry` question for the new tree itself; for a terminal word it prints
the `goal carry` line. The moving-origin retry loop (land.sh:342-359)
follows the same rule. A push happens only with the tree the row names.
Fixtures: HCL-05-LANDS-WITHOUT-CHAIN (a tier-3 goal's tree lands with no
job record at all); HCL-05-FOREIGN-TREE-ASKS; HCL-05-RED-BATTERY-LANDS
(coverage red: recorded in the trailer, the commit lands);
HCL-05-TRAILER-OWNED (a caller-supplied `Carry:` line is refused; the
stamped four are exact); HCL-05-REBASE-ASKS (origin moved: no push, the
ask for the new tree); HCL-05-UNPROVABLE-INDEX-REFUSES (record failure,
not judgement).

### HCL-TRANSACTION-06: the order of writes, and what a crash leaves

Four writes, in this order, each idempotent by the carry opid:

1. The row (04) exists in the ledger. Nothing else is written by the
   word.
2. The commit, with the four trailers, on the local branch.
3. The push.
4. `goal carried --root . --id <goal> --ref <opid> --commit <sha>`, one
   goal operation, agent-run by the landing after the push: it appends
   the `carried` history row with ApprovedRef `<opid>` (the consumer
   pattern of `AuthenticatedChannelApproval`, verbs.go:67-93, which
   gains `carried` beside `resume` and `set-obligation`), the review
   obligation of 07, the BudgetExceptions increment and the history
   line of 08. `opidLanded` makes a repeat a no-op.

Consumed means: a `carried` row with that ApprovedRef exists, OR a
commit reachable from origin/main since the row's At carries `Carry:
<opid>` (the wrapper-owned trailer; `git log --grep` bounded by the
row's time). Observe checks both. A crash after 2 and before 3 leaves
a local commit and an unconsumed row: the next `land.sh --carried
<opid>` finds the commit at HEAD with the trailer and resumes at 3. A
crash after 3 and before 4 leaves a landed commit and no obligation:
observe sees the trailer on origin/main, refuses a second commit
(shape c: "already landed as <sha>; completing the record"), and the
landing runs 4. A second human word for the same tree after 4 writes a
new row and lands a new commit; that is the "second word, second
landing" rule. Fixtures: HCL-06-ORDER (the four writes in order, one
run); HCL-06-CRASH-BEFORE-PUSH-RESUMES; HCL-06-CRASH-AFTER-PUSH-
COMPLETES-RECORD; HCL-06-REPLAY-REFUSED (a consumed opid lands nothing
and prints where it landed); HCL-06-CARRIED-IDEMPOTENT.

### HCL-OBLIGATION-07: review deferred, never deleted

The `carried` operation appends a `ReviewObligation` in the existing
schema without change (file.go:515-518, bareReviewID :997: no
whitespace, every field non-empty): Finding `carried:<sha7>` (or
`carried:<sha7>:battery-red` when the trailer says red, so the
discharge cannot forget it), Chain `human-carried`, Artifact the
landing commit's full id, Test `pending`, State `open`. `goal done`
refuses an agent while it is open (verbs.go:1100-1103, existing law).

Discharge, two ways, both declared changes to the command layer
(cmd/metasystem/goalsync_mutations.go), where the critique-register
lookups already live:

- By critic: a code-critic chain dispatched with `--reviews
  commit:<sha40>` (09) that returns `verdictMaterialCount` 0 and whose
  register is closed; then `goal discharge-review-obligation --finding
  carried:<sha7> --chain human-carried --test <critic root job id>`.
  For chain `human-carried` the command reads the named job record and
  its terminal return before calling the goal package: the record's
  role is code-critic, its reviews subject is `commit:` the
  obligation's Artifact, the return's material count is zero. Any other
  citation is refused with what was expected. The goal package's verb
  (verbs.go:961-990) is unchanged.
- By accepted risk: `goal accept-risk --finding carried:<sha7> --chain
  human-carried` from the human. For chain `human-carried` the command
  skips the critic-register lookup (:218-260; there is no register)
  and the goal package's `AcceptedRiskDecision` (verbs.go:1009-1045)
  additionally marks the matching open obligation discharged with Test
  `accepted-risk:<opid of the accept-risk row>`; that is the one goal
  package change of this point, and it applies only to chain
  `human-carried`.

Fixtures: HCL-07-OBLIGATION-WRITTEN (schema-valid, the exact fields);
HCL-07-CONCLUDE-REFUSED-OPEN; HCL-07-DISCHARGE-BY-CRITIC;
HCL-07-DISCHARGE-REFUSES-ARBITRARY-TEXT (a citation that is not a
zero-material critic of that commit is refused);
HCL-07-DISCHARGE-BY-ACCEPTED-RISK (then `goal done` passes);
HCL-07-RED-BATTERY-IN-FINDING.

### HCL-COUNTER-08: every use counts, and the voice

The `carried` operation increments the goal's `BudgetExceptions` (the
p2 design's 19) and writes the history line `Carried: tree=<sha7>
by=<human> commit=<sha7> at=<time>`. The appetite line carries
`carried=<k>` per claimed goal and the health summary
(internal/steward/health.go) a fleet count for the day; at two on one
goal the existing `repeated exception: defect signal` fires. No
threshold refuses anything.

What the machine says, then does. At the word (`goal carry`, or the
`--kind carry` question's facts): the goal's tier and its four answers,
or `risk: unanswered` when the goal carries no Risk record (file.go:25-28
permits nil); the critic rounds the tuple holds and that they are
skipped; the row's expiry. At the landing, before the push: the battery
result as it will be stamped; the obligation finding id it will write;
the exception count after this one. Then it acts. Nothing in the lines
is a condition. Fixtures: HCL-08-COUNTED; HCL-08-FLEET-LINE;
HCL-08-WORD-LINES; HCL-08-LANDING-LINES-THEN-ACT.

From the word to the push is two commands (`goal carry`, `land.sh
--carried`) or one channel reply plus one command on the seat; nothing
asks a second question once the word is proven. Fixture:
HCL-08-TWO-COMMANDS. The register of 03 lists a human verb that needs
more than two under `Slow`.

### HCL-CRITIC-09: a critic can review a commit

The deferred review of 07 needs a code-critic subject that is a landed
commit, not an implementer job. Declared contract, all three seams:
`dispatch.sh` accepts `--reviews commit:<sha40>` for the code-critic
role and skips the implementer-record check for that form
(dispatch.sh:1302-1306); `internal/dispatch/claim.go:738-743` admits the
form as a reviews subject; `finding_register.go:749-761` resolves it by
writing `git diff <sha>^..<sha>` as the round's diff.patch and
`<sha>^{tree}` as reviewedTree before the round runs, so the critic's
subject reader is unchanged from there on. Two chains on the same
commit subject unite as two chains on the same job subject do.
Fixtures: HCL-09-COMMIT-SUBJECT-DISPATCHES; HCL-09-DIFF-IS-PARENT-TO-
COMMIT; HCL-09-BAD-FORM-REFUSED (a prefix, a tree id, a missing
commit).

### Fixtures of this revision (Go tests and goal-cli-fixtures.sh)

Thirty-eight, named above under 01 to 09. Each is one test that exists
and passes; the code critique of the build treats an unbuilt or failing
name as material (R-60-m1).

### Build list

- Slice 1, the audit: `internal/refusal/register.go`, its grammar walk
  and the three tests of 03 (`Pending` rows for the landing gates); the
  `Defects` and `Slow` lists as backlog items. Lands alone.
- Slice 2, the word, the landing, the transaction, the obligation, the
  counter, the voice, the commit subject (02, 04 to 09): `goal carry`
  and `goal carried` in cmd/metasystem (goalsync_mutations.go) and
  internal/goal (verbs.go, file.go's history rendering);
  `AuthenticatedChannelApproval`'s consumer list; `channel ask --kind
  carry` (internal/channel/question.go kinds); `landing observe
  --carried` (internal/landing/observe.go, a carry-row reader);
  scripts/agents/commit.sh (carried mode, the four trailers, the
  advisory gates) and scripts/agents/land.sh (`--carried`, the rebase
  rule); the discharge and accept-risk changes of 07;
  internal/steward/health.go; the commit subject of 09 in dispatch.sh,
  claim.go, finding_register.go; the 03 rows flipped from pending.
- Slice 3, the docs: docs/orchestration.md and AGENTS.md naming the
  carried landing beside the tiers. Rides the slice-2 chain's closing
  round when the budget allows, its own round otherwise.

### Budget, the box Wido approved

The goal's box, approved on his word 2026-09-04 ("approved" on the
channel, thread 24/27): elapsed one working day, ten attempts, 360
reserved job minutes, one active job, three review rounds. Spent so
far: two attempts and 30 minutes (one dispatch refused at setup for an
unsorted outputs file, charged as an attempt; the first critique
round). The plan inside what remains: design critique round two 30;
slice 1 build 45 and review 30; slice 2 build 60, review 30, one
correction 45, re-review 30: 270 minutes, seven attempts, one attempt
and 60 minutes in reserve for slice 3 or a third design round. The
goal carries no Risk record of its own (it was opened before the law
and the law refuses answering after approval); its answers on the
seat's reading are severity 3, novelty 2, exposure 3, accumulation 1,
tier 3 as approved.

### Revision 2 dispositions of the first critique (chain hcl-design-cc1, round 1)

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| HCL-C-01 | accepted | authority.go:145-162: the temporary relay cannot verify who supplied the words; revision 1 called it identity. | 02 names the two proofs and excludes the temporary word; HCL-02-TEMPORARY-REFUSED. |
| HCL-C-02 | accepted | authority.go:93-95, 138-143, 617-619: a parsed record has no authority; revision 1's JSON file was a bearer token. | 04: the word is a goal-history row under HUMAN_AUTHORITY_PROVEN or AUTHENTICATED_CHANNEL_WORD; no file. |
| HCL-C-03 | accepted | commit.sh:226-245, 270-271, 383-387 refuse before the landing verdict for human and agent alike. | 05 makes each gate advisory in carried mode with the `Carried-Battery` trailer; commit.sh is in the build list; two record failures stay refusals. |
| HCL-C-04 | accepted | land.sh:334-359 commits, then rebases, then pushes; the tree can change after the check. | 05's rebase rule: a moved origin means no push and an ask for the new tree. |
| HCL-C-05 | accepted | No write order in revision 1; land.sh's steps fail independently. | 06 orders the four writes, defines consumed, and names the crash fixtures. |
| HCL-C-06 | accepted | file.go:515-518 and :997 reject an empty Test and whitespace in Finding. | 07 uses Test `pending` and Finding `carried:<sha7>:battery-red`; the schema is unchanged. |
| HCL-C-07 | accepted | goalsync_mutations.go:218-260 requires a register finding; verbs.go:1009-1045 leaves obligations open; :1100-1103 refuses done. | 07 declares the accept-risk change for chain `human-carried` (skip the register lookup, discharge the obligation with Test `accepted-risk:<opid>`). |
| HCL-C-08 | accepted | verbs.go:961-990 accepts any non-empty citation. | 07's command-layer check of the cited critic record; HCL-07-DISCHARGE-REFUSES-ARBITRARY-TEXT. |
| HCL-C-09 | accepted | dispatch.sh:1302-1306, claim.go:738-743, finding_register.go:749-761 all require an implementer job. | 09 declares the `commit:<sha40>` subject at all three seams. |
| HCL-C-10 | accepted | observe.go:75-99 literals, norm.go:95-99 formatted codes, authority.go:27-38 mixed outcomes, land.sh:94-112 prose. | 03's grammar, exclusion list with dead-exclusion check, and hand-listed shell rows with the stated limit. |
| HCL-C-11 | accepted | revision 1 made slice 1 name a verb that slice 2 adds. | 03's `Pending` rows and HCL-03-NO-PENDING-AFTER-SLICE-2. |
| HCL-C-12 | accepted | poll.go:107-159 binds only threaded replies to open questions. | 04's channel path is a `--kind carry` question and its threaded answer: the existing grammar, one human action. |
| HCL-C-13 | accepted | observe.go:32-36 requires a full id; revision 1 spoke seven digits. | 04: full object id only, HCL-04-FULL-ID-ONLY. |
| HCL-C-14 | accepted | verbs.go:559-630 and :1638-1716: two different claim laws; revision 1 invented a third. | 04 drops the automatic claim; the landing prints the `goal steal` line (shape c). |
| HCL-C-15 | accepted | The battery and counter exist only at the landing; Risk may be nil (file.go:25-28). | 08 splits the voice into word lines and landing lines, with `risk: unanswered`. |
| HCL-C-16 | accepted | commit.sh:127-203 scans only Goal-Item; :519-533 stamps colon trailers. | 05's four wrapper-owned trailers, caller lines refused, postcondition exact. |
| HCL-C-17 | noted | Twenty-one fixtures were named, not seventeen. | Revision 2 counts thirty-eight and names each under its point. |
