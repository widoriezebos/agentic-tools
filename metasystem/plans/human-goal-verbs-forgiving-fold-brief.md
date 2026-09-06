Working Mode: build
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal human-goal-verbs-forgiving)
Date: 2026-09-06

# Fold round one: chain hgvf-build1-20260906

The code critic (job hgvf-cc1c-20260906, reviewed tree
f8abd52ce2bf721927dc506fc034956b68acf402) returned three material
findings on your round-one diff. Its return is at
metasystem/artifacts/agents/hgvf-cc1c-20260906/rounds/1/return.json.
The contract is unchanged: metasystem/plans/human-goal-verbs-forgiving-design.md
and the goal record metasystem/plans/goals/human-goal-verbs-forgiving.md.
This is a follow-up on your own job: start from your round-one tree and
change only what the three findings need.

# The change

1. HGV-01, metasystem/internal/goal/approval.go. The approve transaction
   admits a parked goal and nothing else moves (design section 2, the one
   transaction rule that changes). Remove the counter increment you added
   for over-box approvals: on main only set-budget raises
   BudgetExceptions, and that stays so. Restore the idempotency guard for
   parked goals: an approve that lands the box the goal already carries,
   under the same proven authority, answers nothing-to-do for a parked
   goal exactly as for an approved one. Then fix the package test
   TestApproveRecordsABoxWhileLeavingAParkStanding so it approves the
   parked goal with a DIFFERENT box and proves that box lands, the park
   stands, and unpark shows approved with that box. Add the twin: the same
   box again is nothing-to-do and records no line. If the enrolled-terminal
   fold in metasystem/internal/goal/norm.go makes an identical re-run
   compute a different norm claim (the act's own operation id and the
   pre-touch revision), make the idempotency check compare the box and
   the authority, not the claim reference, so an identical over-box
   command run twice records one line. Prove it in the existing package
   test for the terminal fold.

2. HGV-02, metasystem/cmd/metasystem/goalsync_mutations.go and your
   new file goal_refusal.go beside it in cmd/metasystem. Every `run:` line must
   succeed with the values the refusal saw, or the refusal prints the
   words line "no command completes this" instead. Fix each row the
   critic names:
   - approve with `--sweep` and `--id` together: the remedy drops
     `--sweep` and `--confirm` and keeps the ids (the human named goals).
   - an engine refusal about the budget itself (review rounds above
     review-round-max; a rejected `--approved-ref`): never re-print the
     failing command; print the words line naming the limit or the
     reference, and bound rounds in ParseBox
     (metasystem/internal/goal/budget.go) the way the other members are
     bounded so the refusal happens at parse time with a remedy.
   - `keep` on a breach-stopped goal given `--approved-ref`: the printed
     resume command drops the reference (resume refuses any reference).
   - a box with more than five members: the remedy is the first five
     members only when they parse; otherwise the words line.
   - a completion whose result equals the standing box of an approved
     goal: the words line saying the goal already carries that box; never
     a command that exits 1 with nothing-to-do.
   - classify-sweep without `--draft`: the words line; never `--draft ''`.
   - resume without a fence on a claimed goal without a Budget line: the
     design's row says the `goal budget <id> <box>` command with `norm`;
     print that, and change the fixture at the ship-widget case to assert
     the command and run it.

3. HGV-03, metasystem/scripts/agents/goal-cli-fixtures.sh. Design
   section 6: each refusal scenario captures stderr, takes the line after
   `run:`, runs it verbatim, and asserts the history line and Budget line
   equal those the long form produces on a twin goal. Make
   run_forgiving_refusal_remedy do that: it takes the twin goal id and the
   long-form command, runs both, and compares the last history line's verb
   and actor and the Budget line of both records. Every scenario that
   uses it gets a twin. The accept-risk pair scenario runs its printed
   command against a real finding and chain in the bed (or, if the bed
   cannot hold one, asserts the words line and says so in a comment that
   names the bed's limit, never a placeholder that cannot run).

Do not widen: no new verbs, no new flags, no change to authority (every
act stays human-only at the enrolled terminal; every history line stays
as it is).

# Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l . (empty)`;
`go test ./internal/goal/ ./internal/goalbudget/ ./cmd/metasystem/ -count=1`
green; `bash scripts/agents/goal-cli-fixtures.sh` green (say so if the
sandbox cannot run it; the orchestrator replays it outside).

# Constraints

Wall-clock budget: 45 minutes; return before it ends even if something is
red, naming it. MECHANICAL reach (tier 2). Declare the boundary as every
file that differs from main. Gap rule: stop and report a gap with your
proposed contract written out; never fill it silently. Finding ids in the
return: HGV-01, HGV-02, HGV-03, each with what changed and the test that
proves it.
