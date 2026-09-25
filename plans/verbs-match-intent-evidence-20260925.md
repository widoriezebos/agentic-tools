# Local verb-failure evidence sweep, 25 September 2026

Read-only sweep for the design author. No repository files changed. No config files or secret stores read. Times below are Europe/Amsterdam (CEST). Commands are relevant excerpts from recorded invocations, not commands executed by this sweep.

## Coverage and interpretation

- All 1,771 files in `~/.codex/sessions` were scanned; 712,660 JSON lines. Named session start dates span 20–25 September 2026. `~/.codex/archived_sessions` is empty. This is the available Codex retention, not the entire project's history.
- All 2,004 `.jsonl` files in `~/.claude/projects` were scanned; 923,064 JSON lines, including coordinator and subagent sessions. The scan included all local projects and then selected metasystem-related tool outputs; irrelevant projects were not included as evidence.
- Total local transcript bytes: approximately 7.2 GiB. A streaming keyword pass found tool-result blocks containing refusals, grammar failures, or usage. A second structured pass unwrapped Codex result blocks. 37,597 candidate outputs were retained temporarily; many are source reads, quoted records, or duplicates. **This is not an incident count.** The specimens below were individually checked as actual tool outputs or rejected journal transactions; command excerpts were correlated through tool-call identifiers.
- Across 10 discovered local checkouts, 4,756 goal journal files contain 4,732 unique operation filenames: 4,143 confirmed, 380 rejected files (379 distinct operation filenames), 226 abandoned, 6 lost, 1 unknown. Current storage is `metasystem/artifacts/agents/goal-transactions`, not the goal's outdated `artifacts/agents/goals` pointer.
- All discovered local `events.jsonl` (9,535), `return.md`/`return.json` (16,583), and stop-verdict JSON/Markdown (5,867) were also scanned for the narrow refusal patterns. They yielded 1,607 candidate lines, mostly duplicate source/history reads; not an incident count. No oversize files were skipped by the configured 10 MB bound.
- Other Macs, remote session stores, deleted/rotated transcripts, and shell commands run only in the human's terminal are not covered. Goal records and journals preserve some of that missing history. A scan of all locally retained text is not a claim that every historical refusal has been semantically assessed or reproduced against today's source.
- I did not run any failing command or recovery command. These are historical observations. Current defects versus already-fixed history must be checked by the design/build author.

## Verified specimens

### E1 — One human intention chooses a different transaction by hidden state

6 September, 14:37:12. Rejected `goal set-budget` for `seat-mutual-awareness`:

> budgets on unclaimed work are the human's approval act, set through goal approve --id seat-mutual-awareness with --budget

Source: `/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/artifacts/agents/goal-transactions/HB63GJ5NEP09MMYTM5MEVXMB04-m1-7cd0bd60.json` (`intent.verb=set-budget`, `outcome=rejected`, `evidence`). The same wording occurs in 11 rejected journal files across 10 goals, through 17 September. The printed text does not preserve the supplied budget into a runnable command.

### E2 — A printed budget exception grammar fails when followed

11 September, 07:30:05 then 07:30:23. A `goal set-budget` on `governed-exhaustion-reprojection` first refused an over-norm tuple and instructed the caller to pass `--approved-ref` in the strict form `goal=governed-exhaustion-reprojection minutes=1500 reviewRounds=3 goalRevision=<r>`. The next rejected transaction used that form:

> GOAL_NORM_REFUSED: --approved-ref goal=governed-exhaustion-reprojection minutes=1500 reviewRounds=3 goalRevision=41 does not name a rulings-register row or human goal-history operation

Sources under `/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/metasystem/artifacts/agents/goal-transactions/`:
- `56075YTE0AYEBMF0MSPPPJFTRN-m1b-c6925449.json`
- `9796QBTYVSD71FWX6ZQ7ESPH5W-m1b-c6925449.json`

The missing-rulings-row wording occurs in 11 rejected files across 10 goals. This is evidence about remedy correctness, not a proposal to relax authority.

### E3 — Restart state leaks through three other human/agent acts

8 September, 08:25:46: `goal steal --id account-provenance --by Wido`:

> goal account-provenance is breach-stopped by stop-account-provenance-r19-f1; only goal resume may replace its claim authority

At 08:28:03, and again at 22:47:01 and 22:47:49, `goal set-budget --id account-provenance` with five explicit limits:

> goal account-provenance revision 19 is breach-stopped by stop-account-provenance-r19-f1; only goal resume with its standing approved budget may reopen admission

Sources in `/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/metasystem/artifacts/agents/goal-transactions/`: `EH90YF02T8Q4EV1V88W9J6X2QB-m1b-c6925449.json`, `M0SKNSKFN7PS28Q60SRWV6MQ5P-m1b-c6925449.json`, `G2FN2552W3R166PAHDT4M5M70V-m1b-c6925449.json`, `XS8SKZ4SZRQPEXR4ST0V82X7TS-m1b-c6925449.json`.

Separate `goal release` attempts for `dispatch-cap-necessity` refused at 11:21:58, 11:37:02, 13:26:58 on 3 September with `only goal resume may clear its launch fence`. Across the local journals, 22 rejected files across 9 goals contain breach-stop refusals. These prove repeated attempts and transaction coupling; they alone do not prove today's dead-seat wedge still exists.

### E4 — A budget change forced an agent to manipulate claim ownership before retry

9 September, 12:06:15, and 10 September, 08:18:37. Claude m1d's dispatch output:

> {"detail":"goal breach-stop-wedges-seat revision 11 belongs to claim epoch 6, not current epoch 3","headline":"refused","jobId":"bsws-crit1-20260909","outcome":"REFUSED-INTERNAL"}

The second result said revision 21 and job `bsws-build1b-20260909-r4`, with the same epoch mismatch. The second call explicitly implements this manual recovery recipe: run `metasystem delegate --follow-up bsws-build1b-20260909-r4 --brief plans/breach-stop-wedges-seat-build-brief-r5.md`; if output mentions the claim epoch, run `goal release`, then `goal claim`, then the same delegate call. The recorded script labels it “lease epoch moved under the raise: release + claim, then retry”.

Source: `/Users/wido/.claude/projects/-Users-wido-LocalStorage-GitHub-agentic-tools-m1d/1256ce4b-2cff-499e-9e81-ed1cf7deb77a.jsonl`, result lines 1579 and 2313; command lines 1578 and 2312. These line numbers are JSONL lines, not code lines.

### E5 — Valid-looking human budget request hits a hidden configured ceiling

9 September, 15:21:19, and 11 September, 07:29:53:

> invalid budget: reviewRoundLimit 4 exceeds configured maximum 3

The first command was `goal set-budget --id landing-receipt-survives-records-drift --by Wido --elapsed-limit 1d --attempt-limit 10 --reserved-job-minutes-limit 1200 --active-job-limit 1 --review-round-limit 4`, sent to the enrolled human terminal. The second requested four review rounds for `governed-exhaustion-reprojection`. The refusal names neither the setting nor a workable alternative.

Source: `/Users/wido/.claude/projects/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/8661b3c4-a41e-4f86-8e44-7ee9a869273b.jsonl`, result lines 17678 and 25360, calls at 17670 and 25359.

### E6 — Review requires dispatcher identity and manifest knowledge

5 September, 23:34:55. Actual invocation:

`metasystem delegate --role design-critic --brief plans/hook-root-critique4-brief.md --goal supervision-hook-wrong-root --destructive-reach DESIGN-BEARING --design metasystem/plans/supervision-hook-root-design.md --outputs "$scratch/hook-root-critique4-outputs.txt"`

Refusal fields:

> headline: refused; outcome: REFUSED-OPID-MISMATCH; evidence.reason: fingerprint-equality-gate-failed; evidence.resolution: preflight-same-opid

Source: `/Users/wido/.claude/projects/-Users-wido-LocalStorage-GitHub-agentic-tools/b25c6dd6-68d7-4938-afcf-884efcff4749.jsonl`, call 2798/result 2799.

13 September, 15:34:36, another real `delegate --role design-critic ... --op wait-design-read-4` refused `REFUSED-OPID-MISMATCH`, `resolution=operation-id-bound-to-another-job`, `recordedJobId=wait-design-read-3-r2`.

Source: `/Users/wido/.claude/projects/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/e90b57fc-9012-459f-9031-d2deb9f37ea2.jsonl`, call 13028/result 13029.

These results contain structured evidence but no runnable recovery command.

### E7 — Same repository concept still has incompatible flag names in recent Codex use

24 September, 14:43:50. Codex invoked:

`metasystem goal list --repo "$PWD/metasystem" --id finish-test-repairs-and-integrate --json`

and `metasystem goal next --repo "$PWD/metasystem" --machine m1e --fetch --json`.

Both returned:

> flag provided but not defined: -repo

The displayed flags named `-root`, “checkout root”, with the current checkout default. The first command also assumes a list-by-id option; the first parse failure prevents learning whether that option is valid in this result.

Source: `/Users/wido/.codex/sessions/2026/09/24/rollout-2026-09-24T14-38-29-01a0d36c-abe4-7b80-935c-b3d6f5413a6b.jsonl`, call 201/result 204.

### E8 — Inconsistent JSON mode causes failed read commands

24 September, 11:17:47. Codex invoked:

`metasystem launch status --id 20260924t090108-904d44e981 --json`

Result:

> flag provided but not defined: -json
> Usage of launch status:
>   -id string
>     launch id
> usage: metasystem launch status --id <id>

Source: `/Users/wido/.codex/sessions/2026/09/24/rollout-2026-09-24T07-41-15-01a0d1ee-ad01-7291-b0a1-0bbb4ad24dc0.jsonl`, call 6469/result 6472.

23 September, 13:45:17. A separate Codex session ran a Linux `goal show --root "$root" --id finish-test-repairs-and-integrate --history --json`; it likewise returned `flag provided but not defined: -json` and usage with `--history`, `--id`, `--root`.

Source: `/Users/wido/.codex/sessions/2026/09/22/rollout-2026-09-22T08-53-53-01a0c7e4-7710-72a0-a19a-69996b5e6e3b.jsonl`, call 3335/result 3338.

### E9 — Waiting for an outcome is split by registration internals

22 September, 03:57:59. Codex invoked `metasystem wait register --root <checkout>/metasystem --path <retained-proof>/result.json --until present --timeout 10m --json`.

Result:

> flag provided but not defined: -path

Help listed `--human`, `--job`, `--json`, `--label`, `--pid`, `--question`, `--root`, `--timeout`. The agent's intent was to wait for a proof result file; it chose the registration surface, whose grammar only accepts process/human wait inputs.

Source: `/Users/wido/.codex/sessions/2026/09/20/rollout-2026-09-20T09-11-21-01a0bda7-bb74-72f2-9880-a0a026055892.jsonl`, call 46294/result 46298.

### E10 — Workflow dispatch rejects paths without a next action

2 September, 17:52:10. Actual command:

`metasystem delegate --role implementer --brief plans/fleet-join-bootstrap-design-brief.md --goal fleet-join-bootstrap --destructive-reach DESIGN-BEARING --op fleet-join-design-r1`

Result:

> BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/plans/fleet-join-bootstrap-design.md

Source: `/Users/wido/.claude/projects/-Users-wido-LocalStorage-GitHub-agentic-tools-metasystem/ce41dd4f-2930-4254-ac05-c158248ad5ea.jsonl`, call 1719/result 1721. A later operation in that session, result 3252 at 21:48:18, refused the missing `metasystem/internal/supervise/identity.go`. This evidence does not establish whether those paths should have existed; it establishes the refusal's lack of a usable next action.

## Historical requirements with indirect retained evidence

The committed audit `metasystem/records/misc/verb-surface-audit-2026-09-06.md` has the exact seven-command landing sequence and the repeated budget grammar failures. The goal `metasystem/plans/goals/verbs-match-intent.md` also records human-terminal `goal done --and-none` versus help drift, typed risk evidence grammar, enrollment resolving the wrong checkout, and the dead-session one-slot wedge. These are contemporaneous reports, not additional raw command results verified by this sweep. Do not inflate them into separate reproduced incidents.

The read in `metasystem/records/goal-system/goal-verb-ease-review.md` records an older root confusion: an empty legacy read had no indication where it looked. It also records three manual rebases after goal mutations advanced origin. Again, treat those as historical evidence, not a current runtime verdict.
