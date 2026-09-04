# human-carried-landing — design: the machine never refuses a verified human (revision 1)

Goal: plans/goals/human-carried-landing.md. Ruling: R-75-m3 (Wido,
verbatim: "I do not ever want to be in a HAL2000 situation where the
computer refuses under a situation judged as critical for the human";
and on the fast track: "I'm considering a 'Just Fucking Do It' risk
tier for hot-patch scenarios that need to be fast-tracked under close
human scrutiny ... And it must also be fully authorized by the human
explicitly."). Parent designs: plans/severity-tiered-rigor-design.md
(the tiers, the tier-1 receipt of STR2-TIER1-EVIDENCE-13, the five-member
box) and plans/severity-tiered-rigor-p2-design.md (the four risk answers,
the review obligations, `goal accept-risk`, the exception counter).
Every cite below is re-read at the reviewed base.

The shape in one paragraph. Urgency changes who carries the rigor, never
the risk: the tier stays derived from the four answers, and a hot patch
to a severity-3 problem is still severity 3. What the human may do is
carry the rigor in person: speak one explicit word for one candidate
tree, and the machine lands that tree at once, with its review deferred
into an obligation on the goal, the use counted, the warnings printed.
The machine keeps exactly one gate, identity: it must know the word is
the human's. Past that gate it verifies, records, counts and warns; it
never blocks.

### HCL-PRINCIPLE-01: refusals bind agents; a verified human is never refused

A refusal in this tree is a law for agents. For the human, every
refusal has one of three shapes and nothing else: (a) an identity
check, the machine establishing that the word is the human's (the
enrolled terminal of internal/humanauthority, the authenticated channel
word, the temporary word with a review date); (b) a warning, printed
and recorded, after which the machine does what was asked; (c) a
question back to the human, when the word is stale (an expired carry,
a tree that no longer exists) and the machine asks for a fresh one.
A fourth shape, the machine refusing a verified human on its own
judgement of the situation, is the HAL2000 refusal and is a defect
wherever it exists, whether or not anyone has hit it yet. Fixture:
HCL-01-THREE-SHAPES, the refusal register of 02 classifies every code
as a, b or c or names the human verb that overrides it; a code with no
classification fails the test.

### HCL-AUDIT-02: the refusal register

One exported table, `internal/refusal/register.go`: every refusal code
the tree prints (the `UPPER_SNAKE` and `kebab-refused` constants of
dispatch, goal, landing, steward, channel and humanauthority), with
its shape under 01 and, for shape (b) and for agent-only refusals, the
human verb that carries past it (`goal resume` for a breach stop, the
human override of the p2 design's 16 for a derived tier, `goal
accept-risk` for an exhausted review budget, `landing carry` of 03 for
a landing gate). The table is the audit Wido asked for, and it is kept
by a test, not by discipline: HCL-02-EVERY-CODE-ROWED walks the source
of the named packages for refusal-shaped constants and fails on one
absent from the table; HCL-02-EVERY-ROW-REAL fails on a row naming a
verb the binary does not have. The first build slice is this table
alone, landed with the codes it finds today; a code it classifies as
the fourth shape is listed under `defects` in the table and each is a
backlog item opened by the slice, never fixed silently.

### HCL-WORD-03: one word, one tree, hours not days

`metasystem landing carry --tree <sha> --goal <id> --why "<text>"
[--expires <duration>]` from the enrolled terminal (proof as
`SetBudgetApproved`, the strictest proof the tree has), or through the
authenticated channel as the message `carry <tree7> <goal> <why>` under
the TOTP word (the channel's `--approved-ref` path of stop and resume,
extended to this verb). It writes
`artifacts/agents/landing/carries/<tree>.json`: tree, goal, the human's
name, why, the proof record, issuedAt, expiresAt (default two hours,
ceiling four; a longer request is shape (c), asked back), consumed
(empty until 04 lands it). One word carries one tree once: a second
landing of the same tree needs a second word; a different tree needs
its own. The word names the goal so the obligation of 05 and the
counter of 06 have an owner; a goal that is not claimed by this seat's
machine is not a refusal: `carry` claims it in the same transaction
under the human's name (the p2 design's raise shows a one-transaction
re-bind). Fixtures: HCL-03-WORD-WRITTEN (terminal proof, record
complete); HCL-03-CHANNEL-WORD (the TOTP message form); HCL-03-ONE-USE
(a consumed word does not carry a second landing); HCL-03-EXPIRY-ASKS
(an expired word prints the ask for a fresh one and nothing else);
HCL-03-AGENT-CANNOT (an agent invoking `carry` gets the identity refusal,
shape a).

### HCL-LANDING-04: the carried landing lands in one command

`land.sh --carried <tree>` (and `landing observe --carried`): the
classification `human-carried`, beside `chain`, `register-carriage`,
`exact-revert` and `tier-1`. Requirements, all of them identity or
record checks and none of them judgement: an unconsumed, unexpired
carry record for exactly the candidate tree and the landing's goal.
There is no chain, no critic, no closed register, no tier bar. The
gate width's battery still runs (tier-1 receipt form of 13; the full
battery under width full) and its result is recorded in the landing's
trailer, `battery=green` or `battery=red exit=<n>`; a red battery is
shape (b): printed in full, recorded, and the landing proceeds, because
the human standing at the keyboard has said this tree lands and the
machine's judgement of a red test is advice at that moment. The commit
trailer carries `carried-by=<human> carry=<tree7> obligation=<id>`. The
steps are the ordinary land.sh steps with the observe in
`human-carried` mode; nothing is added to the path between the word and
the push. Fixtures: HCL-04-LANDS-WITHOUT-CHAIN (a tier-3 goal's tree
lands with no job record at all); HCL-04-FOREIGN-TREE-ASKS (the carry
names another tree: shape c); HCL-04-RED-BATTERY-LANDS (recorded, not
refused); HCL-04-TRAILER-FORM.

### HCL-OBLIGATION-05: review deferred, never deleted

The landing appends a review obligation to the goal (the p2 design's
obligation records, `ReviewObligation{Finding, Chain, Artifact, Test,
State}`): Finding `carried:<tree7>`, Chain `human-carried`, Artifact
the landing commit, Test empty until discharged, State open. `goal
conclude` refuses an agent while it is open (existing law). It is
discharged one of two ways: a critic chain dispatched with `--reviews
<landing commit>` (the critic subject reader of 2b gains the commit
form beside the job form; its diff is `git diff <parent>..<commit>`)
that returns no material finding, then `goal discharge-review-obligation
--test <the critic's record>`; or the human's `goal accept-risk` on the
finding id. A red battery of 04 is written into the obligation's
Finding as `carried:<tree7> battery=red` so the discharge cannot forget
it. Fixtures: HCL-05-OBLIGATION-WRITTEN; HCL-05-CONCLUDE-REFUSED-OPEN;
HCL-05-DISCHARGE-BY-CRITIC; HCL-05-DISCHARGE-BY-ACCEPTED-RISK;
HCL-05-RED-BATTERY-IN-FINDING.

### HCL-COUNTER-06: every use counts

Each carried landing increments the goal's `BudgetExceptions` (the p2
design's 19) and writes the history line `Carried: tree=<tree7>
by=<human> at=<time>`. The appetite line carries `carried=<k>` per
claimed goal and the health summary a fleet count for the day; at two
on one goal the existing `repeated exception: defect signal` fires. No
threshold refuses anything. The counter is the answer to "how often do
we hot-patch": a team that carries every week has a defect, and the
line says so. Fixtures: HCL-06-COUNTED; HCL-06-FLEET-LINE.

### HCL-VOICE-07: what the machine says, then does

At the word and again at the landing the machine prints, in this
order: the goal's tier and its four answers; what is being skipped (the
critic rounds the tuple holds, the chain); the battery result; the
obligation id it will write; the exception count after this one. Then
it acts. The lines are advisory by construction: nothing in them is a
condition. Fixture: HCL-07-LINES-THEN-ACT asserts the five lines and
the landing in one run.

### HCL-SPEED-08: ceremony is refusal by other means

From the word to the push is two commands (`landing carry`, `land.sh
--carried`) or one channel message plus one command; nothing asks a
second question once the word is proven. The audit of 02 records, for
each human verb it names, the number of commands between the human's
intent and the effect; a verb that needs more than two is listed under
`slow` in the table and is a backlog candidate. Fixture:
HCL-08-TWO-COMMANDS.

### Fixtures of this revision (Go tests and goal-cli-fixtures.sh)

The seventeen named above. Each is one test that exists and passes;
the code critique of the build treats an unbuilt or failing name as
material (R-60-m1).

### Build list

- Slice 1, the audit: `internal/refusal/register.go` and its two tests;
  the `defects` and `slow` lists as backlog items. Tier 2 on its own
  answers (severity 1, novelty 2, exposure 2, accumulation 1).
- Slice 2, the word, the landing, the obligation, the counter, the
  voice (03 to 07): cmd/metasystem/landing_verbs.go, internal/landing
  (observe.go, a carry record reader), internal/goal (obligation,
  history line, counter), internal/steward/health.go, scripts/agents/land.sh,
  internal/dispatch/finding_register.go (the commit form of the reviewed
  subject). Tier 3 (severity 3: this is the path that lands unreviewed
  code; exposure 3).
- Slice 3, the channel word and the docs: internal/channel, the
  orchestration doc and AGENTS.md naming the carried landing beside the
  tiers.

### Budget, the box Wido approved

The goal's box, approved on his word 2026-09-04 ("approved" on the
channel, thread 24/27): elapsed one working day, ten attempts, 360
reserved job minutes, one active job, three review rounds. The seat's
plan inside it: design critique one round (R-60-m1 stop rule) 30;
slice 1 build 45 and review 30; slice 2 build 45, review 30, one
correction 45, re-review 30; slice 3 build 45 and review 30: 330
minutes in nine attempts, one attempt and 30 minutes in reserve. The
tiering law that this design cites landed on main 2026-09-04 as
b4ae9395 (the risk basis) and its docs as d46e18ee; every cite above is
re-read at that base. The goal carries no Risk record of its own (it was
opened before the law); its answers on the seat's reading are severity
3, novelty 2, exposure 3, accumulation 1, tier 3 as approved.
