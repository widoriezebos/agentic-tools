# One verb gives a goal a box (goal human-goal-verbs-forgiving, slice 1)

- Status: design, awaiting one critique
- Goal: human-goal-verbs-forgiving (member of the arc verbs-match-intent)
- Next step: critique once, then slice 2 builds it behind the fixtures below

## What is true today (read in the tree)

The human's budget act is split by the goal's internal state. `goal approve`
carries a budget only for unclaimed work and refuses a claimed goal with
"its tuple changes through goal set-budget"; `goal set-budget` refuses
unclaimed work with "set through goal approve with --budget"; a parked goal
is refused by both, because the approve transaction admits only queued,
approved and claimed goals. The budget is five long flags, all or nothing,
while every record, message and the config itself write it as
`4h/6/720m/1/2`. `--budget` takes the literal word `box`; that word does
NOT mean the standing box: with it, or with no budget flag at all, approve
replaces the goal's budget with the tier's box from `metasystem.conf`. Every
human verb needs `--by`, though the terminal enrollment record carries no
name at all, so today nothing on disk could supply one. Every refusal names
flags; none prints the command that would have worked.

The ledger underneath stays: approve binds the Approved line, set-budget
replaces the tuple and rebinds the claim to a new revision, resume reopens a
breach-stopped claimed revision under its standing box and refuses any
other, and set-budget refuses a breach-stopped goal. That last pair belongs
to goal breach-clock-and-budget-honesty and is not touched here.

## 1. The verb and its grammar

The verb is `goal budget`. It takes the goal and one box:

```text
metasystem goal budget --id <goal> <box>
```

The box is one positional token, found by shape wherever it sits among the
arguments: the word `norm`, the word `keep`, or a compact form of five
members separated by slashes in the order everyone already writes,
elapsed/attempts/reserved job minutes/active jobs/review rounds, for
example `1d/10/720m/1/3`. The compact form inherits its grammar from what
exists, and redefines nothing:

- elapsed: the working-duration grammar of `ParseWorkingDuration` in
  `internal/goalbudget` (day, hour and minute segments, a day being eight
  working hours; its honesty is the other goal's);
- attempts, active jobs: positive integers, no suffix;
- reserved job minutes: a positive integer with the `m` suffix, exactly as
  the tier keys in `metasystem.conf` write it;
- review rounds: a non-negative integer, no suffix.

An empty member (`2d////`) keeps that limit from the standing box, so a
human can raise one limit without retyping four. Fewer than five members is
a refusal whose printed command fills the missing tail from the standing
box. A goal with no standing box cannot fill anything: the refusal prints
the command with the `norm` box written out in full.

The parser is one function, `ParseBox`, in `internal/goalbudget`, and
`config.TierBox` calls it too, so the config and the human share one
grammar. `FormatBox` renders a tuple the same way for every printed command
and message.

Presets: `norm` is the tier's box from `metasystem.conf` (tier 3 for a goal
without a tier, as every transaction assumes today). `keep` is the standing
box on the goal's Budget line, unchanged; a goal that never had one refuses
`keep` and prints the `norm` command.

The long form stays and means the same: the five flags `--elapsed-limit`,
`--attempt-limit`, `--reserved-job-minutes-limit`, `--active-job-limit`,
`--review-round-limit` on `goal budget`, all five required as today. A
command carrying both a box token and any of the five is refused and the
printed command carries the box token alone.

The old forms keep working for one release and route to the same path with
one hint line on stderr naming the `goal budget` command they became:
`goal approve --budget box` is `goal budget <id> norm`; `goal approve` with
the five limits is `goal budget <id> <those limits>`; `goal set-budget` with
the five limits is the long form of `goal budget`. `goal approve` without a
budget flag stays exactly what it is: the human's approval of the goal as it
stands, which today takes the tier's box. Help lines: approve becomes
"human-only: approve a goal for execution (its box goes through goal
budget)"; set-budget becomes "human-only: long form of goal budget, kept for
one release"; budget reads "human-only: give a live goal a box
(1d/10/720m/1/3, norm or keep); approves, re-approves or rebinds as the goal's
state requires".

## 2. Routing by goal state

The verb reads the goal once, read-only, before it acts, and routes by what
it sees. The human never picks the verb.

| Goal state | What the verb does | Ledger transaction and history verb |
|---|---|---|
| queued | approves with this box | approve (moves it to approved) |
| approved, unclaimed | re-approves with this box | approve |
| parked | records the approval with this box; the park stands, and unpark returns the goal to approved with that box | approve, admitting parked (the one transaction rule that changes) |
| claimed, not stopped | replaces the box and rebinds the claim to a new revision | set-budget |
| claimed, breach-stopped | `keep` (or a box equal to the standing one) resumes; any other box is refused, printing the `keep` command and saying the new box comes after the resume | resume, exactly as today |
| done | refused; no command | none |
| unknown id | refused; no command | none |

Admitting a parked goal into approve is the smallest change that makes the
DONE line true: the ledger has no other transaction that puts a budget on a
parked goal, and unpark already returns a goal to approved when an Approved
line exists. Nothing else in the approve transaction changes: the same
budget, norm, digest and approval binding, the state left as parked. A
human's box on a breach-stopped goal is two acts because the ledger says so
today: resume under the standing box, then the new box. The verb lands one
ledger act per command, so the refusal prints the first act and names the
second in words; both are `goal budget` commands.

`keep` on a claimed goal whose box already reads that way is the engine's
"nothing to do", printed as such with its exit code unchanged.

## 3. Over the tier's norm

Today the norm check admits a verified channel answer as the human's word
without `--approved-ref`, and refuses an enrolled-terminal proof unless the
flag names a recorded word. The one verb folds the terminal in: a box typed
at the enrolled terminal IS the word. `goalNormApproval` gains one branch
beside the channel branch: a proof that is an enrolled-terminal proof
(valid for the root, real ancestry, not fixture-only, not a temporary word,
not a channel proof) records the norm claim with the act's own operation id
as its reference, and the same minutes, rounds and revision the channel
branch records. The NormApproval line format is unchanged; only the
reference value differs by source. `--approved-ref` still works and still
wins when given. The confirmation prints one line naming the tier's box the
new one exceeds and the goal's exception count, so the human sees what was
recorded.

Authority does not move: the temporary human word is not a valid proof and
never enters the branch; the fixture proof is excluded by name, so the
existing over-norm refusal under fixture authority stays a refusal; the
enrolled terminal already could record this word in two steps and now does
it in one.

## 4. The `--by` default

The enrollment record gains the human's name: `goal enroll-terminal` takes
`--by <name>` and refuses without it, because an enrollment without a name is
the defect this goal removes. The name is stored as a new `human` field on
the record. The schema stays 1 and the field is optional on read, so the
enrollments already on disk still parse and still prove; an engine built
before this change refuses the new field as unknown, so a terminal
re-enrolls after the rebuild, not before. A re-enrollment is the existing
act: it bumps the generation and publishes the fleet cutoff as today, and
its fleet history line stays as it is.

Every human-only verb in the file resolves `--by` after proving authority
and before building the request, in one helper: an explicit `--by` wins,
untouched and unchecked against the record; otherwise the name is read from
the same enrollment record the proof walked to, and only when the proof is
the enrolled-terminal proof. A temporary word and a channel proof keep
`--by` required, because the record names who enrolled the terminal, not who
relayed. Under fixture authority the name is read from the enrollment record
in the fake-runtime root, which the fixture bed writes; production never has
a fake root, so this reaches nothing real. Never git config, never a guess.

When no name can be resolved the refusal reads: "the enrolled terminal has
no recorded name" and, since the name is a value the verb did not see, the
second line is words, not a command: re-enroll with `goal enroll-terminal
--by <your name>`, or add `--by <your name>` to this command.

## 5. The refusal rule

One function, `refuseHumanVerb`, in a new file `cmd/metasystem/goal_refusal.go`.
Every refusal of a human-only verb in `goalsync_mutations.go` returns
through it. It prints exactly two lines to stderr and returns the exit code
the site uses today:

```text
goal budget: <one plain sentence>.
run: metasystem goal budget --root <root> --id <goal> 1d/10/720m/1/3
```

or, when no command with the values seen would succeed:

```text
goal budget: <one plain sentence>.
no command completes this: <what the human does instead, in words>
```

A verb hands it a values record assembled once per invocation: root,
lineage, id, the name (seen or defaulted), the fixture flag, the temporary
word pair, the approved reference, the box as typed and as parsed, and the
goal's read-only view (state, standing box, tier box, whether a stop fence
stands). The command renderer prints only flags the verb saw (`--root` when
not the default, `--lineage`, `--fixture-human-authority`, `--by` when
typed), quotes values with spaces, and writes the box in compact form. An
engine refusal (the transaction's error, or an outcome other than
confirmed) passes through the same function: the engine's JSON is printed as
today, then the verb reads the goal again and prints the routing table's
command for the state it now sees, or the words when the table refuses.

Every refusal site, by verb, and what the second line carries:

| Verb | Refusal today | Second line |
|---|---|---|
| shared, flag parse | the flag package's own parse failure; "does not take --label / --unlabel / --tier / --risk, --basis, or --evidence / budget flags / --approved-ref / --members" | the same command with the offending flags dropped |
| shared, request | "mutations carry their coordinator's identity: export METASYSTEM_OWNER_LINEAGE or pass --lineage"; endpoint, machine, guard, clock failures | words: export the lineage the checkout announces; the other four are words naming the failing fact |
| shared, proof | "could not prove enrolled human ancestry"; "could not bind its temporary recorded relay"; "fixture authority does not combine with a temporary human word or review date"; "could not prove fixture human authority" | words: run this at the enrolled terminal; for the fixture pair, the command without the temporary pair |
| shared, after the act | "confirmed but could not record its authority proof"; "cannot record an incomplete human authority proof" | words: the act landed at tip <tip>, the proof did not; nothing to re-run |
| budget | not a synced backlog; needs `--id`; no box and no long form; box and long form together; fewer than five members; a member outside its grammar; empty member with no standing box; `keep` with no standing box; a different box on a breach-stopped goal; done; unknown id; no recorded name | as sections 1, 2 and 4 say: the command with the box completed or the `norm` box written out, the `keep` command, or words |
| approve | "works only with the synced backlog"; "uses either repeatable --id or --sweep"; "--sweep ... takes no budget or --approved-ref"; "needs --by and either repeatable --id or --sweep"; "--budget and the five explicit budget limits are mutually exclusive"; "--budget is box"; "budget flags are all-or-nothing"; a tuple member refused by `NewBudget` | the `goal budget` command with the box the verb saw; the sweep pair prints the sweep command without the extra flags; the by refusal is section 4's |
| set-budget (alias) | "needs a synced backlog plus --id and --by"; "the complete budget tuple is required"; all-or-nothing; `NewBudget` | the `goal budget` command with the box completed from the standing box |
| unapprove | "needs a synced backlog plus --id, --by, and --because" | the command with the missing flag when the value was seen; words for a reason the verb did not see |
| resume | legacy ledger; "needs --id and --by"; tuple required or all-or-nothing; "could not validate --approved-ref"; "is not breach-stopped"; "could not acquire the goal-revision lock"; binding failure | `goal budget <id> keep` for every tuple refusal; words for the rest (an unstopped goal takes `goal budget <id> <box>`, printed) |
| accept-risk | "needs --id, --finding, --chain, --by, and --why"; "--temporary-human-word and --review-by travel together"; the register finding lookup | the command with the seen values and the missing one named in words; the pair refusal prints the command with both or neither |
| set-obligation | the one "requires identity, recurrence, ... every typed review trigger"; not synced; approved-ref validation; proof; clock | words naming each missing flag by name; the flags are values the verb did not see |
| classify-sweep | "needs a synced backlog, --draft, and exactly one of --preview or --confirm; confirmation also needs --by"; unreadable draft; SWEEP_LISTING_CHANGED | the `--preview` command with the draft seen; for the changed listing, the preview command again |
| enroll-terminal | not synced; no `--by`; "terminal enrollment refused: ..."; "enrolled locally but its fleet cutoff did not publish" | words: the ancestry workaround the refusal already names; the local enrollment stands |

The engine's own refusal texts (claimed-goes-through-set-budget,
unclaimed-goes-through-approve, breach-stopped, the norm refusal, approval
required) are not rewritten; the one verb routes so that the first two
never arise, and every one that does gets its second line from the table.
Verbs that a human may run but that are not human-only (park, unpark, split,
edit, discharge-review-obligation, the seat verbs in `trySyncMutation`) keep
their messages; they are the arc's later slices.

## 6. Fixtures for slice 2

All in `scripts/agents/goal-cli-fixtures.sh` under fixture human
authority, one scenario each. Each refusal scenario captures stderr, takes
the line after `run:`, runs it verbatim, and asserts the history line and
Budget line equal those the long form produces on a twin goal.

- Compact form per state: queued (approve line, state approved), approved
  (re-approve), parked (approve line, state still parked, then unpark shows
  approved with that box), claimed (set-budget line, Claimed revision
  advanced), breach-stopped with `keep` (resume line), done (refusal with
  the words line), unknown id (words line).
- Presets: `norm` on a queued goal equals the tier key in the fixture's
  `metasystem.conf`; `keep` on a goal opened with five limits approves
  exactly those; `keep` on a goal without a box refuses and the printed
  `norm` command runs.
- Members: an empty member keeps the standing limit; four members refuse
  and the printed command runs; a member outside its grammar refuses and
  the printed command runs; box plus long form refuses and the printed
  command runs.
- The `--by` default: the bed writes an enrollment record with a name; a
  `goal budget` without `--by` records `human:<that name>` on the history
  line; an explicit `--by` records the explicit name; a bed without a name
  refuses with the words line and no `run:` line.
- Over-norm under fixture authority still refuses (the existing scenario,
  now asserting the words line). The enrolled-terminal fold cannot run
  headless; it gets a package test in `internal/goal` that proves a
  constructed ancestry through the injected process reader and asserts the
  NormApproval reference equals the act's operation id.
- Aliases: `goal approve --budget box`, `goal approve` with five limits and
  `goal set-budget` each print the hint line and land the same history line
  as the `goal budget` form.
- Every remaining row of the refusal table that a headless bed can drive
  (the shared flag-shape refusals, the sweep pair, unapprove, resume without
  a fence, accept-risk's pair, classify-sweep's three).

## 7. Scope of slice 2

Changes: the CLI file that routes the human verbs and the new refusal file
beside it; the verb table and help lines in `cmd/metasystem/main.go`; the
box parser and formatter in `internal/goalbudget` with `config.TierBox`
calling it; the enrollment record's name field and the enroll signature in
`internal/humanauthority`; the approve transaction admitting a parked goal
in `internal/goal/approval.go`; the terminal branch of the norm check in
`internal/goal/norm.go`; the fixture suite and package tests.

Unchanged: every record line format (Budget, Approved, NormApproval,
Claimed, History) and the intent arguments each verb publishes; the history
verbs `approve`, `set-budget`, `resume`, `unapprove`, `enroll-terminal`; the
fixture authority path and its root binding; the temporary word and channel
proof classes and what they may do; the breach-stop and resume rules; the
seat's machine verbs; the `--repo` flag.
