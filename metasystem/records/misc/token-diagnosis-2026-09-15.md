# Claude token diagnosis, 2026-09-15 (Europe/Amsterdam day)

Generated 2026-09-15 10:51Z by `token_diag.py`. Window: 2026-09-14T22:00Z to generation time. Goal: seats-spend-tokens-in-bounded-sessions (seat m1e).

## Findings

| seat | all-in (CEST day to generation) | cache_read share |
|---|---:|---:|
| m1b | 317.0M | 98.6% |
| m1c | 165.4M | 96.9% |
| m1e | 389.2M | 96.7% |
| three seats | **871.7M** | |

Top three causes across the three seats (turn-starter attribution, table 2a):

1. **notification**: 299.7M, 34.4% of the day, 538 calls. Mechanism: a harness task notification wakes an idle main session: background Bash completions 211.6M, Monitor events 56.4M, Agent finished 31.6M; 6.1 calls per wake at 555k mean context.
2. **stop_hook**: 284.2M, 32.6% of the day, 439 calls. Mechanism: the metasystem stop gate blocks the stop ("Stop blocked; needs your decision [and supervision repair]; Do not stop. Run this command"), so each attempted stop buys 4.3 more calls at 646k mean context (103 refusals).
3. **peer**: 118.2M, 13.6% of the day, 181 calls. Mechanism: cross-session messages wake the receiving seat: from M1c 65.4M, from M1e 32.2M, from M1b 20.5M; 3.4 calls per message at 652k.

The multiplier under all three: main interactive sessions carry 779.5M (89.4%), and 89.4% of that comes from calls at 400k context or more. Every wake-up, whatever its cause, re-reads a 550-650k context.

Attribution caveat: if a notification, peer or human message delivered mid-turn (queued_command attachment) starts its own segment (table 2c), the order becomes notification 381.7M (43.8%), peer 229.6M (26.3%), stop_hook 127.1M (14.6%). Stop-hook refusals then fall to third, because many of their calls follow a mid-turn delivery.

Largest session: m1e main 36c93128, 653 calls, 335.8M, peak 966k, 2 compactions today. Largest delegate: m1e "Fable design page for leaked processes" (general-purpose), 32 calls, 14.5M, peak 728k, resumed.

Large reads (>20k chars) are not a top cause: 16 results, about 4.7M estimated re-read tokens across seats.

Cross-check with the alert: the spend fence ledger for m1e (observed 10:45Z) reads seat dayTokens 264,430,431 and day tokens 322,528,167. This report's fence-style reading of m1e (UTC day, top-level session files only) gives 264.4M up to 11:00Z, a match. The fence therefore counts only m1e's own main transcripts from 00:00Z (02:00 CEST) plus 58.1M of engine job records of any runtime on this machine: 322.5M - 264.4M now, 301.8M - 243.7M at the alert. It leaves out m1b and m1c transcripts, every subagents/ file, and the 22:00-00:00Z slice. That is why the Claude total across the three seats (above) is about 2.9 times the alert. The fence-style m1e reading crossed 243.7M between 09:00Z and 09:45Z (about 11:30 CEST), so the '13:45 CEST' in the handoff notes is about two hours late. Both count cache_read at full weight; cache reads are 96-99% of every seat's figure.

## 1. Per-seat day totals

| seat | API calls | all-in tokens | cache_read | cache_read share | main | delegates | engine jobs |
|---|---:|---:|---:|---:|---:|---:|---:|
| m1b | 584 | 317.0M | 312.7M | 98.6% | 304.8M | 0.0M | 12.2M |
| m1c | 373 | 165.4M | 160.3M | 96.9% | 137.4M | 28.0M | 0.0M |
| m1e | 991 | 389.2M | 376.2M | 96.7% | 337.3M | 51.9M | 0.0M |
| other | 33 | 4.9M | 4.5M | 90.1% | 4.9M | 0.0M | 0.0M |
| **m1b+m1c+m1e** | 1948 | **871.7M** | 849.3M | | | | |

## 2. Cause split per seat (turn-starter attribution)

**m1b** (day 317.0M)

| cause | calls | all-in | share | cache_read | mean context/call | peak context |
|---|---:|---:|---:|---:|---:|---:|
| notification | 167 | 113.4M | 35.8% | 112.7M | 677k | 949k |
| peer | 119 | 82.7M | 26.1% | 82.4M | 694k | 959k |
| stop_hook | 123 | 82.6M | 26.1% | 82.3M | 671k | 965k |
| other:compaction_summary | 62 | 14.3M | 4.5% | 13.7M | 227k | 365k |
| engine jobs (headless sdk-cli) | 100 | 12.2M | 3.9% | 10.8M | 117k | 274k |
| human | 10 | 9.3M | 2.9% | 8.4M | 930k | 962k |
| other:usage_limit_resume | 3 | 2.5M | 0.8% | 2.5M | 831k | 833k |

**m1c** (day 165.4M)

| cause | calls | all-in | share | cache_read | mean context/call | peak context |
|---|---:|---:|---:|---:|---:|---:|
| stop_hook | 124 | 89.4M | 54.1% | 89.0M | 720k | 867k |
| notification | 62 | 29.3M | 17.7% | 29.0M | 470k | 609k |
| delegates (subagents) | 156 | 28.0M | 17.0% | 24.4M | 175k | 478k |
| other:usage_limit_resume | 25 | 14.5M | 8.7% | 14.4M | 577k | 605k |
| human | 4 | 2.5M | 1.5% | 1.9M | 614k | 617k |
| peer | 2 | 1.7M | 1.1% | 1.7M | 869k | 869k |

**m1e** (day 389.2M)

| cause | calls | all-in | share | cache_read | mean context/call | peak context |
|---|---:|---:|---:|---:|---:|---:|
| notification | 309 | 157.0M | 40.3% | 155.1M | 506k | 948k |
| stop_hook | 192 | 112.1M | 28.8% | 111.5M | 583k | 966k |
| delegates (subagents) | 322 | 51.9M | 13.3% | 43.8M | 159k | 728k |
| peer | 60 | 33.7M | 8.7% | 33.5M | 560k | 903k |
| human | 40 | 23.1M | 5.9% | 21.5M | 575k | 959k |
| other:compaction_summary | 62 | 7.8M | 2.0% | 7.3M | 124k | 210k |
| other:usage_limit_resume | 6 | 3.5M | 0.9% | 3.5M | 585k | 593k |

### 2a. Causes across the three seats

| cause | calls | all-in | share of 3-seat day | cache_read | mean context/call |
|---|---:|---:|---:|---:|---:|
| notification | 538 | 299.7M | 34.4% | 296.8M | 555k |
| stop_hook | 439 | 284.2M | 32.6% | 282.7M | 646k |
| peer | 181 | 118.2M | 13.6% | 117.6M | 652k |
| delegates (subagents) | 478 | 79.9M | 9.2% | 68.2M | 164k |
| human | 54 | 34.9M | 4.0% | 31.8M | 644k |
| other:compaction_summary | 124 | 22.1M | 2.5% | 21.1M | 175k |
| other:usage_limit_resume | 34 | 20.5M | 2.4% | 20.3M | 601k |
| engine jobs (headless sdk-cli) | 100 | 12.2M | 1.4% | 10.8M | 117k |

### 2b. Sensitivity: delegate tokens folded into the cause of the turn that launched them

| cause | calls | all-in | share |
|---|---:|---:|---:|
| notification | 538 | 299.7M | 34.4% |
| stop_hook | 439 | 284.2M | 32.6% |
| peer | 181 | 118.2M | 13.6% |
| human | 54 | 34.9M | 4.0% |
| other:compaction_summary (via delegate) | 83 | 28.7M | 3.3% |
| notification (via delegate) | 215 | 26.4M | 3.0% |
| other:compaction_summary | 124 | 22.1M | 2.5% |
| other:usage_limit_resume | 34 | 20.5M | 2.4% |
| stop_hook (via delegate) | 97 | 14.5M | 1.7% |
| engine jobs (headless sdk-cli) | 100 | 12.2M | 1.4% |
| other:usage_limit_resume (via delegate) | 59 | 8.0M | 0.9% |
| peer (via delegate) | 13 | 1.4M | 0.2% |
| human (via delegate) | 11 | 0.8M | 0.1% |

### 2c. Sensitivity: mid-turn queued deliveries (queued_command attachments) also start a segment

| cause | calls | all-in | share | change vs 2a |
|---|---:|---:|---:|---:|
| notification | 707 | 381.7M | 43.8% | 82.0M |
| peer | 382 | 229.6M | 26.3% | 111.4M |
| stop_hook | 204 | 127.1M | 14.6% | -157.1M |
| delegates (subagents) | 478 | 79.9M | 9.2% | 0.0M |
| human | 60 | 35.3M | 4.0% | 0.4M |
| engine jobs (headless sdk-cli) | 100 | 12.2M | 1.4% | 0.0M |
| other:usage_limit_resume | 8 | 5.3M | 0.6% | -15.2M |
| other:compaction_summary | 9 | 0.6M | 0.1% | -21.5M |

Turn starters and mid-turn queued deliveries counted today:

| seat | file kind | starter or queued | cause | count |
|---|---|---|---|---:|
| m1b | main/engine | starter | engine_job_brief | 5 |
| m1b | main/engine | starter | human | 3 |
| m1b | main/engine | starter | notification | 23 |
| m1b | main/engine | starter | other:compaction_summary | 1 |
| m1b | main/engine | starter | other:usage_limit_resume | 1 |
| m1b | main/engine | starter | peer | 35 |
| m1b | main/engine | starter | stop_hook | 41 |
| m1c | main/engine | starter | human | 1 |
| m1c | main/engine | starter | notification | 8 |
| m1c | main/engine | starter | other:usage_limit_resume | 1 |
| m1c | main/engine | starter | peer | 2 |
| m1c | main/engine | starter | stop_hook | 7 |
| m1c | sub | starter | coordinator_msg | 2 |
| m1c | sub | starter | delegate_brief | 7 |
| m1c | sub | starter | notification | 2 |
| m1e | main/engine | starter | human | 15 |
| m1e | main/engine | starter | notification | 57 |
| m1e | main/engine | starter | other:compaction_summary | 2 |
| m1e | main/engine | starter | other:usage_limit_resume | 1 |
| m1e | main/engine | starter | peer | 16 |
| m1e | main/engine | starter | stop_hook | 55 |
| m1e | sub | starter | coordinator_msg | 2 |
| m1e | sub | starter | delegate_brief | 19 |
| other | main/engine | starter | human | 10 |
| other | main/engine | starter | notification | 1 |
| m1b | main/engine | queued mid-turn | human | 1 |
| m1b | main/engine | queued mid-turn | notification | 44 |
| m1b | main/engine | queued mid-turn | peer | 20 |
| m1c | main/engine | queued mid-turn | human | 2 |
| m1c | main/engine | queued mid-turn | notification | 54 |
| m1c | main/engine | queued mid-turn | peer | 42 |
| m1c | sub | queued mid-turn | coordinator_msg | 1 |
| m1c | sub | queued mid-turn | notification | 11 |
| m1e | main/engine | queued mid-turn | human | 11 |
| m1e | main/engine | queued mid-turn | notification | 49 |
| m1e | main/engine | queued mid-turn | peer | 37 |
| m1e | sub | queued mid-turn | coordinator_msg | 1 |
| m1e | sub | queued mid-turn | notification | 74 |
| other | main/engine | queued mid-turn | notification | 1 |

### 2d. Calls per turn by cause (main interactive sessions, today)

| cause | turns | calls | calls/turn | all-in/turn |
|---|---:|---:|---:|---:|
| notification | 88 | 538 | 6.1 | 3.4M |
| stop_hook | 103 | 439 | 4.3 | 2.8M |
| peer | 53 | 181 | 3.4 | 2.2M |
| human | 19 | 54 | 2.8 | 1.8M |
| other:compaction_summary | 3 | 124 | 41.3 | 7.4M |
| other:usage_limit_resume | 3 | 34 | 11.3 | 6.8M |
| engine_job_brief | 5 | 0 | 0.0 | 0.0M |

### 2e. What the stop-hook, notification and peer turns were (main interactive sessions, today)

| cause | sub-label (hook task / needs; notification summary; peer sender) | turns | calls | all-in |
|---|---|---:|---:|---:|
| notification | Background command "..." completed (exit code #) | 61 | 361 | 204.9M |
| notification | Monitor "..." stream ended | 5 | 74 | 35.9M |
| notification | Agent "..." finished | 11 | 61 | 31.6M |
| notification | Monitor event: "..." | 8 | 33 | 20.6M |
| notification | Background command "..." failed with exit code # | 2 | 9 | 6.7M |
| stop_hook | failures show observed against expected / needs your decision and supervision repair | 27 | 88 | 57.9M |
| stop_hook | go tests never inherit a candidate engine / needs your decision | 2 | 63 | 42.4M |
| stop_hook | coordinator context stays under budget / needs your decision and supervision repair | 14 | 50 | 34.2M |
| stop_hook | brief declares the round boundary / needs your decision | 14 | 38 | 28.1M |
| stop_hook | coordinator context stays under budget / needs your decision | 15 | 53 | 25.2M |
| stop_hook | brief declares the round boundary / needs your decision and supervision repair | 9 | 44 | 22.2M |
| stop_hook | delegates never write the receipts log / needs your decision | 1 | 15 | 12.4M |
| peer | from M1c | 24 | 107 | 65.4M |
| peer | from M1e | 20 | 44 | 32.2M |
| peer | from M1b | 9 | 30 | 20.5M |

Stop-hook feedback variants (digits normalised):

- x27: `Stop hook feedback: Just completed: unknown for this turn. Task: failures show observed against expected; Stop blocked; needs your decision and supervision repa`
- x15: `Stop hook feedback: Just completed: unknown for this turn. Task: coordinator context stays under budget; Stop blocked; needs your decision; Do not stop. Run thi`
- x14: `Stop hook feedback: Just completed: unknown for this turn. Task: coordinator context stays under budget; Stop blocked; needs your decision and supervision repai`
- x14: `Stop hook feedback: Just completed: unknown for this turn. Task: brief declares the round boundary; Stop blocked; needs your decision; Do not stop. Run this com`
- x9: `Stop hook feedback: Just completed: unknown for this turn. Task: brief declares the round boundary; Stop blocked; needs your decision and supervision repair; Do`

## 3. Context-size bands (tokens by per-call context)

| seat | kind | <150k | 150-400k | 400-700k | >=700k |
|---|---|---:|---:|---:|---:|
| m1b | main | 1.5M (15) | 15.5M (54) | 128.3M (220) | 159.6M (195) |
| m1b | engine | 5.5M (66) | 6.8M (34) | - | - |
| m1c | main | - | 7.5M (20) | 64.8M (115) | 65.1M (82) |
| m1c | sub | 6.2M (69) | 17.4M (77) | 4.5M (10) | - |
| m1e | main | 6.4M (62) | 52.1M (184) | 144.4M (266) | 134.4M (157) |
| m1e | sub | 14.9M (187) | 23.6M (113) | 10.5M (18) | 2.9M (4) |

Main interactive sessions: 779.5M of 871.7M (89.4%); calls at >=400k context carry 696.6M (89.4% of main).

## 4. Models

| seat | kind | model | calls | all-in |
|---|---|---|---:|---:|
| m1b | engine | claude-opus-5 | 100 | 12.2M |
| m1b | main | claude-opus-5 | 484 | 304.8M |
| m1c | main | claude-opus-5 | 217 | 137.4M |
| m1c | sub | claude-fable-5-1 | 6 | 2.8M |
| m1c | sub | claude-opus-4-8 | 15 | 6.0M |
| m1c | sub | claude-opus-5 | 135 | 19.3M |
| m1e | main | claude-fable-5-1 | 40 | 9.7M |
| m1e | main | claude-opus-5 | 629 | 327.6M |
| m1e | sub | claude-fable-5-1 | 35 | 15.4M |
| m1e | sub | claude-opus-5 | 287 | 36.4M |
| other | main | claude-fable-5-1 | 9 | 0.8M |
| other | main | claude-opus-5 | 24 | 4.2M |

## 5. Sessions and delegate files active today

| seat | kind | session / agent | label | calls | all-in | cache_read | peak ctx | CEST span | compactions |
|---|---|---|---|---:|---:|---:|---:|---|---:|
| m1e | main | 36c93128-ec16- | main seat session | 653 | 335.8M | 331.1M | 966k | 00:00-12:04 | 2 |
| m1b | main | e90b57fc-9012- | main seat session | 484 | 304.8M | 302.0M | 965k | 00:00-12:16 | 1 |
| m1c | main | 96f72de7-79c7- | main seat session | 217 | 137.4M | 135.9M | 869k | 00:00-11:58 | 0 |
| m1e | sub | agent-a2fa74ec | Fable design page for leaked processes | 32 | 14.5M | 11.4M | 728k | 09:11-11:15 | 0 |
| m1c | sub | agent-a0479449 | Design registered-wait session matching | 21 | 8.8M | 7.5M | 478k | 00:20-00:38 | 0 |
| m1e | sub | agent-a653d7d3 | Opus closing read of unit C1a | 30 | 5.4M | 4.6M | 305k | 00:51-01:33 | 0 |
| m1e | sub | agent-a776b4a0 | Opus closing read of unit 1b-i | 23 | 4.3M | 3.9M | 298k | 00:31-00:57 | 0 |
| other | main | c53cbef5-7be0- | main seat session | 24 | 4.2M | 3.8M | 207k | 07:36-11:22 | 0 |
| m1e | sub | agent-a46a7080 | Relaunch Opus confirmation read of 1b-ii | 25 | 3.8M | 3.5M | 259k | 03:11-03:33 | 0 |
| m1c | sub | agent-a1b11d0a | Focused read of goal 11's round 5 race | 23 | 3.8M | 3.5M | 247k | 08:22-08:52 | 0 |
| m1b | engine | 5c90bc99-641a- | engine job (headless sdk-cli, # Task Direction brief) | 22 | 3.8M | 3.4M | 274k | 07:41-08:12 | 0 |
| m1c | sub | agent-a05b21b0 | Independent read of goal 11's build | 18 | 3.7M | 3.4M | 230k | 00:01-00:15 | 0 |
| m1c | sub | agent-a908ec2a | Independent read of goal 11's round 3 | 22 | 2.9M | 2.5M | 178k | 03:11-03:28 | 0 |
| m1e | sub | agent-a9a42159 | Opus closing read of unit 3a-ii | 21 | 2.9M | 2.7M | 213k | 10:27-10:44 | 0 |
| m1e | sub | agent-a2da4e7e | Opus closing read of unit C2 | 23 | 2.8M | 2.6M | 185k | 10:16-10:29 | 0 |
| m1c | sub | agent-ac41e1cf | Independent read of goal 11's round 2 | 20 | 2.8M | 2.5M | 224k | 01:54-02:10 | 0 |
| m1e | sub | agent-a3717dda | Opus closing read of unit C1b | 19 | 2.7M | 2.4M | 226k | 03:53-04:10 | 0 |
| m1b | engine | b9eb4796-e18d- | engine job (headless sdk-cli, # Task Direction brief) | 22 | 2.5M | 2.3M | 190k | 01:45-02:05 | 0 |
| m1e | sub | agent-adfaa231 | Opus closing read of unit 1b-ii | 15 | 2.3M | 2.0M | 251k | 01:50-02:15 | 0 |
| m1b | engine | a47b83be-854e- | engine job (headless sdk-cli, # Task Direction brief) | 20 | 2.1M | 1.9M | 163k | 08:16-08:32 | 0 |
| m1e | sub | agent-a845cb33 | Opus confirmation read of C1a fold | 17 | 2.1M | 1.9M | 187k | 01:46-01:59 | 0 |
| m1b | engine | d840481d-fddb- | engine job (headless sdk-cli, # Task Direction brief) | 19 | 2.0M | 1.7M | 167k | 01:01-01:16 | 0 |
| m1c | sub | agent-a93d519f | Independent read of the wait-line regression fix | 15 | 1.9M | 1.6M | 224k | 00:20-00:39 | 0 |
| m1b | engine | 73eb029e-f9f4- | engine job (headless sdk-cli, # Task Direction brief) | 17 | 1.8M | 1.5M | 179k | 09:02-09:20 | 0 |
| m1c | sub | agent-a82cdc07 | Independent read of the forged-hint fix | 14 | 1.5M | 1.3M | 165k | 02:05-02:17 | 0 |
| m1c | sub | agent-a9398298 | Independent read of goal 11's round 4 | 13 | 1.5M | 1.2M | 177k | 07:44-08:00 | 0 |
| m1e | main | 9292cf37-9f88- | m1e coordinator (in flight) | 16 | 1.5M | 1.3M | 122k | 12:36-12:45 | 0 |
| m1e | sub | agent-aceb1934 | Opus closing read of unit 3a-i | 13 | 1.4M | 1.0M | 207k | 08:08-08:31 | 0 |
| m1e | sub | agent-ad75bf6d | Opus confirmation read of 1b-i round 3 | 13 | 1.4M | 1.2M | 170k | 01:09-01:25 | 0 |
| m1e | sub | agent-aeba6417 | Relaunch Opus closing read of unit 2 | 12 | 1.3M | 1.0M | 181k | 03:11-03:30 | 0 |
| m1e | sub | agent-a0a899b0 | Opus confirmation read of 3a-ii fold | 14 | 1.1M | 1.0M | 120k | 10:55-11:03 | 0 |
| m1c | sub | agent-a51dc903 | Independent read of the brain-boot wait-line fix | 10 | 1.1M | 0.9M | 185k | 07:45-07:58 | 0 |
| m1e | sub | agent-a033b54f | Opus confirmation read of unit 2 fold | 12 | 1.1M | 0.9M | 128k | 03:45-03:55 | 0 |
| m1e | sub | agent-a728743c | Opus closing read of unit 2 | 10 | 1.0M | 0.8M | 168k | 02:29-02:45 | 0 |
| m1e | sub | agent-a3b5666e | Opus confirmation read of 1b-ii round 3 | 12 | 0.9M | 0.8M | 114k | 03:44-03:51 | 0 |
| m1e | sub | agent-a0b7976f | Fable final fold: follow-up design revision 3 | 3 | 0.9M | 0.6M | 297k | 00:00-00:01 | 0 |
| m1e | sub | agent-ab3e7e70 | this diagnosis delegate (in flight): Token diagnosis for 202 | 11 | 0.8M | 0.7M | 118k | 12:40-12:51 | 0 |
| other | main | 540d8c2a-4b78- | main seat session | 9 | 0.8M | 0.7M | 127k | 11:46-11:51 | 0 |
| m1e | sub | agent-ace19c94 | Opus confirmation read of 3a-i fold | 10 | 0.7M | 0.5M | 97k | 08:41-08:49 | 0 |
| m1e | sub | agent-ada85614 | Opus confirmation read of 1b-ii fold | 7 | 0.4M | 0.3M | 114k | 02:39-02:44 | 0 |

## 6. Delegates

| seat | by model | calls | all-in |
|---|---|---:|---:|
| m1c | claude-fable-5-1 | 6 | 2.8M |
| m1c | claude-opus-4-8 | 15 | 6.0M |
| m1c | claude-opus-5 | 135 | 19.3M |
| m1e | claude-fable-5-1 | 35 | 15.4M |
| m1e | claude-opus-5 | 287 | 36.4M |

| seat | by agent type | files | calls | all-in |
|---|---|---:|---:|---:|
| m1c | code-critique | 8 | 135 | 19.3M |
| m1c | general-purpose | 1 | 21 | 8.8M |
| m1e | code-critique | 14 | 218 | 28.7M |
| m1e | general-purpose | 6 | 104 | 23.1M |

Top five delegates by tokens (all seats; engine jobs listed after):

| seat | description | type | model(s) | calls | all-in | peak ctx | briefs/messages in file | resumed |
|---|---|---|---|---:|---:|---:|---:|---|
| m1e | Fable design page for leaked processes | general-purpose | claude-fable-5-1 | 32 | 14.5M | 728k | 3 (2 coordinator) | yes (SendMessage x2) |
| m1c | Design registered-wait session matching | general-purpose | claude-fable-5-1,claude-opus-4-8 | 21 | 8.8M | 478k | 4 (3 coordinator) | yes (SendMessage x4) |
| m1e | Opus closing read of unit C1a | code-critique | claude-opus-5 | 30 | 5.4M | 305k | 1 (0 coordinator) | yes (SendMessage x1) |
| m1e | Opus closing read of unit 1b-i | code-critique | claude-opus-5 | 23 | 4.3M | 298k | 1 (0 coordinator) | no (SendMessage x0) |
| m1e | Relaunch Opus confirmation read of 1b-ii | code-critique | claude-opus-5 | 25 | 3.8M | 259k | 1 (0 coordinator) | no (SendMessage x0) |
| m1b | engine job 5c90bc99 | sdk-cli | claude-opus-5 | 22 | 3.8M | 274k | 1 | no |
| m1b | engine job b9eb4796 | sdk-cli | claude-opus-5 | 22 | 2.5M | 190k | 1 | no |
| m1b | engine job a47b83be | sdk-cli | claude-opus-5 | 20 | 2.1M | 163k | 1 | no |
| m1b | engine job d840481d | sdk-cli | claude-opus-5 | 19 | 2.0M | 167k | 1 | no |
| m1b | engine job 73eb029e | sdk-cli | claude-opus-5 | 17 | 1.8M | 179k | 1 | no |

## 7. Large tool results in main sessions (>20,000 chars)

| seat | large results | chars | est. re-read tokens | share of seat day |
|---|---:|---:|---:|---:|
| m1b | 13 | 386,090 | 2.9M | 0.9% |
| m1e | 3 | 84,534 | 1.8M | 0.5% |
| other | 3 | 63,653 | 0.2M | 3.6% |

| seat | session | UTC date, CEST time | tool | command or file | chars | later calls | est. re-read |
|---|---|---|---|---|---:|---:|---:|
| m1b | e90b57fc | 09-14 22:12 | Bash | `R=/Users/wido/LocalStorage/GitHub/agentic-tools-m1b; git -C $R show origin/main:` | 24,482 | 260 | 1.6M |
| m1e | 36c93128 | 09-15 10:01 | Read | `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/36c931` | 34,913 | 130 | 1.1M |
| m1e | 36c93128 | 09-15 10:44 | Read | `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/36c931` | 21,581 | 76 | 0.4M |
| m1e | 36c93128 | 09-14 17:10 | Bash | `cd /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccb-s3/me` | 28,040 | 32 | 0.2M |
| m1b | 5c90bc99 | 09-15 07:41 | Read | `/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/metasystem/plans/verification-` | 49,585 | 18 | 0.2M |

## 8. Cross-check windows (all-in, Claude transcripts only)

| window | m1b | m1c | m1e | total | total without cache_read | cache_read at 0.1 weight |
|---|---:|---:|---:|---:|---:|---:|
| 09-15 CEST day so far | 317.0M | 165.4M | 389.2M | 871.7M | 22.4M | 107.3M |
| rolling 24 h to now | 602.3M | 321.5M | 658.5M | 1,582.3M | 43.7M | 197.5M |
| 09-14 CEST full day | 537.1M | 292.9M | 612.9M | 1,443.0M | 28.6M | 170.0M |
| 09-14 00:00-14:00 CEST | 272.2M | 151.5M | 353.0M | 776.7M | 7.4M | 84.4M |

Note: the 09-14 rows only include transcript files modified since 2026-09-13T22:00Z.

Fence-style reading (UTC day from 2026-09-15T00:00Z; top-level session files only, no subagents/ files; engine job sessions excluded), cumulative:

| up to (UTC) | CEST | m1b | m1c | m1e | 3 seats | m1e calls | m1e mean ctx |
|---|---|---:|---:|---:|---:|---:|---:|
| 06:00 | 08:00 | 83.6M | 42.4M | 133.0M | 259.0M | 209 | 635k |
| 07:00 | 09:00 | 110.5M | 62.9M | 185.6M | 359.0M | 273 | 678k |
| 08:00 | 10:00 | 137.6M | 81.5M | 197.2M | 416.3M | 325 | 605k |
| 09:00 | 11:00 | 165.7M | 96.1M | 228.0M | 489.8M | 399 | 570k |
| 09:45 | 11:45 | 194.6M | 107.8M | 254.1M | 556.4M | 445 | 569k |
| 10:00 | 12:00 | 205.5M | 116.4M | 260.4M | 582.3M | 455 | 571k |
| 11:00 | 13:00 | 213.1M | 116.4M | 264.4M | 593.9M | 475 | 555k |

## 9. Other directories with 09-15 activity (not in the seat totals)

| directory | session | calls | all-in | turn starters |
|---|---|---:|---:|---|
| -Users-wido-LocalStorage | c53cbef5 | 24 | 4.2M | 13 starters |
| -Users-wido-LocalStorage-GitHub-writing-tools | 540d8c2a | 9 | 0.8M | 3 starters |

## 10. Method and caveats

- **Sources.** Every `~/.claude/projects/*` directory with a transcript modified since 2026-09-14T22:00Z. The seat directories are `-Users-wido-LocalStorage-GitHub-agentic-tools-m1b`, `-m1c` and `-m1e`. No worktree-named, /private/tmp-named, hact-20260912 or unsuffixed agentic-tools directory had 09-15 activity. Two other directories did (section 9). Main files are `<dir>/<session>.jsonl`. Delegate files are `<dir>/<session>/subagents/agent-*.jsonl`, and every isSidechain entry is in those files. Type, description, model and the launching toolUseId come from `agent-*.meta.json`.
- **Window.** Entries timestamped at or after 2026-09-14T22:00Z (00:00 CEST), up to generation time. Whole files are parsed, so a turn that started before the window keeps its cause. Only calls inside the window are counted.
- **API calls.** One call is one `message.id` (falling back to `requestId`) on assistant entries. Repeated entries for content blocks collapse to one call, taking the maximum output_tokens. No id appeared in more than one file. `<synthetic>` entries are skipped because they are not API calls. All-in = input + cache_creation + cache_read + output. Context = all-in minus output.
- **Turn starter.** The latest user-role entry that is not a tool_result.
  - Stop-hook refusal: text starting `Stop hook feedback:`. Each has a matching `hook_blocking_error` attachment with hookEvent Stop.
  - Notification: `origin.kind` task-notification, or text starting `<task-notification>`.
  - Peer: `origin.kind` peer, or text starting `Another Claude session sent a message` with `<cross-session-message from-name=...>`.
  - Human: `origin.kind` human, `promptSource` typed or queued, local-command entries, or `[Request interrupted by user]`.
  - Other: compaction summaries (`This session is being continued`) and usage-limit auto-continuations (`origin.kind` auto-continuation).
  - isMeta entries matching none of these (skill bodies) do not start a turn. No starter was left unclassified.
- **Engine jobs.** m1b's five `entrypoint: sdk-cli` sessions, briefed with `# Task Direction`, are headless engine jobs. They get their own row. Table 2b re-attributes subagent tokens to the cause of the turn that issued the Agent call.
- **Mid-turn deliveries.** A message that arrives while the model is working is written as a `queued_command` attachment. It does not start a turn under the rule. Table 2c shows the ranking when it does.
- **Large reads.** Tool results over 20,000 characters in main and engine sessions. Cost = characters / 4 x later calls in the window, in the same file, before the next compact_boundary. Results from before the window count only their re-reads inside it. For very large outputs the transcript keeps only the harness's truncated preview, so those estimates are a floor.
- **Resumed delegate.** A SendMessage result names the agent (`resumedAgentId`, or `queued for delivery to <id>`), or the file holds a coordinator message.
- **Not counted.** Codex usage, which is not in Claude transcripts. Cache reads count at full weight, as the spend fence counts them. Section 8 also shows totals without cache_read and with cache_read at 0.1 weight.
- **In-flight sessions.** The m1e coordinator 9292cf37 and this delegate are included and labelled. Their totals grow while the report is written.
