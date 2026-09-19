# Spend report for 2026-09-19 (partial day: 00:00Z to 05:52Z)

Goal `waits-run-outside-model-contexts`, DONE 4: "the spend report shows subagent cold-cache rewrites
under 2 percent of the weighted day". The engine has no spend verb yet (goal
`spend-fence-reports-tokens-per-model-and-cause` is approved and not landed), so this record was computed
outside the engine by a scratchpad script that follows that goal's method. This is a record, not code.

## Result

Subagent cold-cache rewrites: **0.2M of 33.7M weighted units = 0.49 percent of the covered window**
(target: under 2 percent). All 4 subagent cold rewrites are the first call of a subagent, where nothing is
cached yet; none followed a wait gap of 5 minutes or more. The figure is for the window 00:00Z to 05:52Z
of 2026-09-19, not the whole day. It is reported as partial and not rounded up to a day; re-running the
script after 2026-09-20 00:00Z gives the whole-day figure.

Caveats for the closing read:
- Only 4 Agent-tool subagents ran as Claude sessions in the window, all from seat m1e. Seats m1b and m1c
  ran their delegates as Codex jobs, which are outside the Claude weighted day (see below).
- Every seat cold rewrite after a transcript's first call follows a compaction (34 of 34): a compaction
  writes the summarized context fresh. The 4 that the per-kind table counts as "after a gap of 300 s or
  more" are compaction rewrites whose compaction also spanned such a gap, not waits with an expired cache.
  No seat or subagent cold rewrite in the window was caused by a wait.

## Window and coverage

- Rows with a timestamp from 2026-09-19T00:00Z up to the run at 05:52Z; 1496 calls.
- Counted: every Claude transcript on this host under `~/.claude/projects` touched since 00:00Z, read-only.
  9 files: the seat sessions of m1b, m1c and m1e (one transcript each), one session in project
  `-Users-wido-LocalStorage` (6 calls at up to 341K context), and 4 Agent-tool subagent transcripts
  (`<project>/<session>/subagents/agent-*.jsonl`, all under the m1e session).
- Session kind is taken from the path: `<project>/<session>.jsonl` is a seat session,
  `<project>/<session>/subagents/agent-*.jsonl` is an Agent-tool subagent, a project path containing
  `--claude-worktrees-` is a worktree session (headless delegate or critique). No worktree-project
  transcript was touched in the window and no assistant row had an entrypoint other than the CLI, so the
  headless-delegate and worktree-critique kinds have zero rows today: that work ran as Codex jobs.
- Not counted, and why:
  - Codex jobs (gpt-5.6-sol), 40 session logs under `~/.codex/sessions/2026/09/19`: 280.9M input tokens of
    which 274.9M were cached reads, 1.4M output tokens (0.5M of them reasoning). They are another provider
    with its own cache pricing, so the Claude weights do not apply and they are not in the weighted day.
    Codex reports no cache writes at all (`cache_write_input_tokens` is 0 in every log), so a cold-rewrite
    signature cannot be read from them. The raw totals are given so the day's Codex cost stays visible.
  - The adapter samples under `metasystem/artifacts/agents/context/samples` cover only this checkout's own
    session, a subset of the transcripts above, so they were not used.
  - Transcripts of other hosts: none on this machine.

## Method

- Script: `spend-day.py` (kept in the m1b scratchpad and its evidence mirror under `tools/`), a Python
  rewrite of m1e's `usage-today.sh` reader with the dimensions of the spend-fence goal added. m1e's reader
  was run on the same day as a cross-check: 1577 rows, 1494 distinct message ids; its `sort -u` keeps a
  streamed reply twice when the two rows differ in output count, which is why its total (34.8M) is above
  this record's 33.7M.
- Assistant rows are deduplicated by `message.id`. A streamed reply is written as several rows with one id
  and a growing output count; the largest value of each usage field counts once.
- Weighted units = input + 1.25 x cache-write + 0.1 x cache-read + 5 x output.
- Cold-cache full rewrite = a call whose cache-write is at or above half its context
  (input + cache-write + cache-read). Sub-signatures: first call of a transcript (unavoidable), call after a
  gap of 300 s or more since the transcript's previous call (the cache expired while something waited),
  call right after a `compact_boundary` row (a compaction writes the summarized context fresh). In the
  by-cause section a compaction takes precedence over a gap; in the per-kind table the two are counted
  independently.
- Calls above 200K context are counted from the same context sum. Compactions are the `compact_boundary`
  system rows.

## Totals

Weighted 33.7M from 1496 calls: input 0.0M, cache-write 5.5M (0.4M at the 5-minute TTL, 5.0M at the
1-hour TTL), cache-read 167.6M, output 2.0M. Models: claude-fable-5-1 1398 calls 31.4M, claude-opus-5 98
calls 2.3M. Compactions: m1c 16, m1b 10, m1e 8.

Per session kind:

| kind | calls | weighted | share | cold rewrites | cold weighted | cold share of window | cold after gap >= 300 s | first-call colds |
|---|---|---|---|---|---|---|---|---|
| Agent-tool subagent | 92 | 1.7M | 5.0% | 4 | 0.2M | 0.49% | 0 | 4 |
| seat session | 1404 | 32.0M | 95.0% | 35 | 2.3M | 6.87% | 4 | 1 |
| worktree session (delegate or critique) | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| headless (non-CLI entrypoint) | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |

## Cold rewrites by cause (seats, not counting each transcript's first call)

- after a compaction: 34
- after a gap of 300 s or more and not after a compaction (the seat waited and its cache expired): none
- other (not first, not after a compaction, gap under 300 s): 0

## Raw script output

```
day 2026-09-19 UTC, 9 transcript files touched, 1496 assistant rows (deduplicated by message id), window 00Z..05Z
weighted total 33.7M (input 0.0M, cache-write 5.5M, cache-read 167.6M, output 2.0M); cache-write 5m-TTL 0.4M, 1h-TTL 5.0M

== per hour UTC: calls, weighted, cold-cache full rewrites (cache-write >= half the context), calls above 200K context
00Z n=183 w=3.5M cold=2 big=0
01Z n=305 w=6.1M cold=7 big=0
02Z n=261 w=5.5M cold=6 big=0
03Z n=303 w=7.2M cold=9 big=0
04Z n=215 w=5.2M cold=7 big=0
05Z n=229 w=6.2M cold=8 big=6

== per session kind: calls, weighted (share), cold rewrites, cold weighted (share of day), cold after a gap of 5 min or more, first-call colds
agent-tool subagent                        n=92 w=1.7M (5.0%) cold=4 cold_w=0.2M (0.49% of day) cold_after_gap>=300s=0 first_call_colds=4
seat session                               n=1404 w=32.0M (95.0%) cold=35 cold_w=2.3M (6.87% of day) cold_after_gap>=300s=4 first_call_colds=1

== per project/session file: weighted, calls, cold rewrites, max context, compactions
15.1M n=682 cold=16 maxctx=166K kind=seat session -Users-wido-LocalStorage-GitHub-agentic-tools-m1c/c3501ca6-53be-49b4-a50c-41759aa4ca4d.jsonl
8.9M n=451 cold=8 maxctx=166K kind=seat session -Users-wido-LocalStorage-GitHub-agentic-tools-m1e/19c52dfc-6c4f-4551-b5cf-731e5a444eec.jsonl
7.4M n=265 cold=10 maxctx=166K kind=seat session -Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f.jsonl
0.6M n=6 cold=1 maxctx=341K kind=seat session -Users-wido-LocalStorage/c53cbef5-7be0-48fa-83f1-876f40a3c193.jsonl
0.5M n=24 cold=1 maxctx=125K kind=agent-tool subagent -Users-wido-LocalStorage-GitHub-agentic-tools-m1e/19c52dfc-6c4f-4551-b5cf-731e5a444eec/subagents/agent-a825aa0325abc47dd.jsonl
0.5M n=25 cold=1 maxctx=133K kind=agent-tool subagent -Users-wido-LocalStorage-GitHub-agentic-tools-m1e/19c52dfc-6c4f-4551-b5cf-731e5a444eec/subagents/agent-a3502305bf1ed3de8.jsonl
0.4M n=26 cold=1 maxctx=97K kind=agent-tool subagent -Users-wido-LocalStorage-GitHub-agentic-tools-m1e/19c52dfc-6c4f-4551-b5cf-731e5a444eec/subagents/agent-a8492e591bc7d5f87.jsonl
0.3M n=17 cold=1 maxctx=100K kind=agent-tool subagent -Users-wido-LocalStorage-GitHub-agentic-tools-m1e/19c52dfc-6c4f-4551-b5cf-731e5a444eec/subagents/agent-a18dbbc4c04c977a5.jsonl

== compactions (compact_boundary rows) per project: {'m1c': 16, 'm1e': 8, 'm1b': 10}

== per model
claude-fable-5-1 n=1398 w=31.4M
claude-opus-5 n=98 w=2.3M

== subagent cold rewrites, each: hour, file, context K, cache-write K, gap s
03Z agent-a3502305bf1ed3de8.jsonl ctx=35K cw=22K gap=- first=True
03Z agent-a18dbbc4c04c977a5.jsonl ctx=34K cw=34K gap=- first=True
01Z agent-a8492e591bc7d5f87.jsonl ctx=34K cw=34K gap=- first=True
01Z agent-a825aa0325abc47dd.jsonl ctx=35K cw=35K gap=- first=True
```
