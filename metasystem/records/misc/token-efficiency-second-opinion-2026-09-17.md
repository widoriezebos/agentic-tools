# Token efficiency since the 16 September changes: a second opinion (Fable, 17 September 2026, 15:30 local)

Sources: every Claude transcript under ~/.claude/projects (S19/usage-rows-0917-1500.jsonl, weight = input + 1.25 cache-write + 0.1 cache-read + 5 output) and every Codex rollout under ~/.codex/sessions (S19/codex-rows-0917.json, weight = uncached input + 0.1 cached + 5 output). Lines = additions in commits on origin/main and the goal branches, ledgers excluded; "code" = Go source and tests.

## Spend per landed line, both providers

| Window (local)        | Claude M | Codex M | Total M | Lines all / code | Per line | Per code line |
|-----------------------|---------:|--------:|--------:|-----------------:|---------:|--------------:|
| 16 Sep 02:00-14:00    | 163      | 76      | 240     | 13,180 / 5,488   | 18.2K    | 43.7K         |
| 16 Sep 14:00-02:00    | 148      | 105     | 253     | 13,144 / 7,148   | 19.2K    | 35.4K         |
| 17 Sep 02:00-09:00    | 35       | 53      | 88      | 6,618 / 4,596    | 13.3K    | 19.2K         |
| 17 Sep 09:00-15:00    | 35       | 49      | 84      | 12,327 / 11,411  | 6.8K     | 7.4K          |

The Claude-only figure reported earlier today (12.4K to 3.5K per line) is right but incomplete: Codex is now the larger spender (41% Claude, 59% Codex in the last window). Combined, the fall is 2.7 times per landed line and about 5 times per code line. The target of 2,000 per line is not reached.

Claude calls over 200K context: 100-250 per hour on 16 September, none since 19:00 local that day. Subagent share of Claude spend: about half on 16 September, 24% today, and today's subagents are reads, not waits.

## Codex weekly limit

The Codex account's 7-day window read 52% at 10:00 local on 16 September and 97% at 14:00 local today: 45 points in 28 hours, roughly 2-4 points per build. It resets Monday 21 September 20:58 local. No credits are attached. Builds in flight when it reaches 100% fail mid-job (the worktree keeps the partial diff).

Effort did not change this: blb Build B at xhigh cost 7.2M weighted for 2,219 lines (374 calls, 3 compactions); blb Build C+D at high has cost 8.8M for about 1,060 lines so far (472 calls, 3 compactions); m1c's glb fix at high cost 7.2M (411 calls). Codex spend is calls times context, and 99% of its input is cache hits that the weekly counter still counts. Effort is a wall-clock knob, not a token knob.

## Compactions

Since the 200K window took effect (16 September, about 19:00 local): each seat's main session compacted 34-46 times (m1e 45, m1c 46, m1b 34). Of 58 delegates, none exceeded 200K; 15 ran at 160-200K; 4 compacted, and two of those were today's reads of 2,200-line builds (m1e's Build B read twice, m1b's R2 read twice). Every Codex build of 2,200 lines compacted 3 times inside Codex's own 258K window.

## Which measures touch quality

| Measure | Evidence | Quality effect | Keep? |
|---|---|---|---|
| No model waits; lane and branch landing as scripts | 32% of spend was waits; now 0 | None on the code; the scripts themselves failed 7 times today and each failure cost a rerun | Keep the rule; replace the scripts with the Go verbs (blb, glb, ldg) |
| 200K window on the seats | 45 compactions per seat-day; seat spend halved | Coordination slips after compactions (re-orientation, one shell slip), none reached main; the handoff note carries state | Keep, with the note as the designed record |
| 200K window on delegates | 2 of today's big reads compacted twice | A reader that compacts holds summaries of the first files it read; a real risk on reads over about 1,500 lines | Change: reads of more than 1,200 diff lines run per package or headless with a 400K window (mechanism landed: CLAUDE_CODE_AUTO_COMPACT_WINDOW in headless-design-launch.sh) |
| Codex effort high | No token saving, no wall-clock saving measured; glb B at high returned a red gate, 6 material and 1 breaking item | Unproven either way; the C+D read (high) is the next data point | Return builds to xhigh unless the C+D read is clean |
| Units of 600-1,500 lines built as 2,200-line jobs | Builder compacted 3 times per job; Build B's 13 material items were page-conformance drift | Consistent with the builder losing the page mid-job | Keep the unit; split a job above 1,500 lines into two serial jobs on the branch |
| One critique round | glb: after one round, the build surfaced two page tensions and one breaking design gap, each costing a Codex fix build (about 7M) | Design gaps moved from critique (cheap) to build+read (dear); the second round the rule allows after a rule-changing fold was skipped by the seat, not by the rule | Keep the rule; apply its second-round clause |
| One read per build, non-breaking items fixed after the merge | B's 10 minors and C+D's non-breaking items are queued behind the merge | Only a risk if the queue is not tracked | Keep, with the rule that a goal cannot close while read items are open on its ledger |
| Cheap gate on the branch, one proof at the merge | glb branch carries a red inherited from its base; a new failure in the same test would hide behind it | None on main (the merge proof runs the union); later discovery on the branch | Keep; the merge lane proves the union, and a branch rebases onto main when main fixes a red the branch shares |

## What a machine set up from scratch gets today

Present in the repository: the process rules (orchestration, design principles, brief template, paper chapter 18, seat-communication), the context-budget verb and the session hooks in the tracked .claude/settings.json.

Absent: the 200K window (settings.local.json, ignored by git), the Codex model and effort (~/.codex/config.toml), the lane, branch-landing, launcher, watcher and testing.json merge scripts (a session scratchpad, copied to hact-20260912/m1e-tools-0917), the 240-second wait cap (memory only), the usage measurement (scratchpad), the hact landing drivers.
