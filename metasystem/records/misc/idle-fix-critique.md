# Idle causal fix — codex critique, verdict: NOT impossible yet

Critic: codex gpt-5.6-sol (idle-fix-critique-d) over the certified fix
(reviewedTree e05fd089). 6 findings, ALL material (3 critical). Terminal:
idle-with-backlog is NOT yet impossible. All ACCEPTED; bind the
correction round.

## F-CAUSAL-FAILOPEN-001 [critical]

Terminal statement: No. Even with the Stop hook registered, a fresh-ledger error permits a non-human quiet stop. ReadClaimableBudgetedWork performs Project with fetch enabled; any fetch or accepted-tree failure aborts TurnVerdict. The command exits nonzero, and supervision-hook.sh turns that into a non-blocking system message and exits zero. Thus an unavailable canonical remote is a lawful sequence in which claimable local backlog exists, no work is in flight, no human marker exists, and the turn still ends. The five-second provider hook deadline also surrounds an unbounded git fetch.

Evidence: metasystem/internal/goal/turnverdict.go:112-123 and :216-223 propagate the fresh-read error before enforcing the invariant. metasystem/internal/goal/project.go:31-35 propagates FetchAdvance failure, while metasystem/internal/goal/txn.go:126-129 runs git fetch without a process deadline. metasystem/cmd/metasystem/goal.go:549-552 maps the error to a nonzero exit. metasystem/scripts/agents/supervision-hook.sh:306-325 emits only surface_json and exits zero; :139-141 defines that payload as systemMessage, while :290-297 is the only branch that emits stop-block. metasystem/scripts/enforcement/claude-code-hooks.json:24-26 caps the entire hook at five seconds.

## F-STOP-AUTH-002 [critical]

An agent can forge the supposedly human-only stop authorization. HUMAN classification protects only the new command; the authority crossing is then reduced to an unsigned JSON file in the agent-writable checkout. The exported Store.WriteSessionStop method performs no classification, and the consumer accepts any correctly shaped bytes. An agent can write, copy, or edit those bytes directly and obtain the same quiet-stop outcome as an attended human.

Evidence: metasystem/cmd/metasystem/session_stop.go:32-50 performs classification and then calls the exported writer. metasystem/internal/goal/sessionstop.go:19-24 defines a marker with only schemaVersion, sessionId, by, and writtenAt; :41-76 exposes a writer that checks only nonblank values before publishing the file; :91-108 validates only JSON shape, matching session text, parseable time, and human-name syntax. metasystem/internal/goal/turnverdict_idle_test.go:39 directly invokes the unclassified writer, demonstrating that the command is not the sole minting boundary.

## F-STOP-REPLAY-003 [high]

Even a genuinely human-written marker is replayable outside the act that authorized it. It carries no holder main identity, lease or claim epoch, turn nonce, expiration, or consumed-marker registry. Consumption is skipped whenever backlog is temporarily absent, a claim or job exists, or another verdict already blocks. Therefore a marker survives parking, in-flight work, and unrelated blocking work, and can silently authorize a later stop after the human has left. Reusing the same provider session identifier after a holder change also accepts the stale marker.

Evidence: metasystem/internal/goal/sessionstop.go:19-24 omits holder and freshness coordinates; :100-107 checks timestamp syntax but not age. metasystem/cmd/metasystem/session_stop.go:41-50 reads the current holder but stores only holder.SessionId. metasystem/internal/goal/turnverdict.go:265-273 returns without consuming the marker for zero claimable work, any local claim, any accepted job, or any pre-existing block; only :274-280 consumes it. The supplied test at metasystem/internal/goal/turnverdict_idle_test.go:34-65 proves different-session rejection and immediate one-shot consumption but has no park, unrelated-block, holder-change, expiry, or replay fixture.

## F-INFLIGHT-004 [high]

The in-flight exception still fails the prior live-work finding and is not one predicate. A stale pending or running job record suppresses every idle-backlog block and makes the steward return healthy with no action, without proving that its process is alive or relevant. Local goal claims are likewise accepted from ledger state without a session-liveness join. Conversely, pending-setup is a legal non-terminal job and the steward counts it, but the Stop scanner filters it out, so the two decision owners disagree and a legitimate in-flight reservation can be blocked.

Evidence: metasystem/internal/goal/project.go:93-99 calls every same-machine claimed record Claimed without a liveness check. metasystem/internal/goal/turnverdict.go:252-267 treats status pending-setup, pending, or running as sufficient and ignores all liveness fields. metasystem/internal/report/openwork.go:50 and metasystem/internal/report/scan.go:128-129 and :173-175 admit only pending and running, so pending-setup never reaches that helper. metasystem/internal/steward/openwork.go:73-106 reads only jobId and status and returns the first legal non-terminal record. metasystem/internal/steward/openwork_converted_test.go:145-154 requires the minimal bytes {"jobId":"delegate-one","status":"running"} to produce WorkInFlight, VerdictHealthy, and ActNone. This is the exact stale-record hole described at metasystem/records/misc/idle-rootcause-critique.md:44-48.

## F-LEGACY-OWNER-005 [high]

The old every-stop and steward-owner holes remain reachable in the still-supported legacy goal world. Work is populated only when NewWorld is true; otherwise enforceIdleBacklog immediately returns. The legacy queued-only ladder still blocks once per digest and then becomes display-only, while LegacyOpenWork still classifies every unclaimed queue as WorkNone. A legacy backlogged machine can therefore pass its second unchanged Stop and tick no-work without human authorization.

Evidence: metasystem/internal/goal/turnverdict.go:217-224 creates the invariant input only for NewWorld, and :265-267 returns when it is nil. The legacy queue at :513-531 retains the BlockedQueueDigests block-once behavior. metasystem/internal/steward/openwork.go:27-31 explicitly preserves the legacy route, and :136-141 maps a nonempty queued backlog to WorkNone. metasystem/internal/goal/turnverdict_test.go:170-181 still requires an already-seen legacy queue digest to remain non-blocking.

## F-HOOK-COVERAGE-006 [critical]

The invariant is not attached to every runtime turn boundary. This checkout enables three runtimes, but the repository's own conformance table says Codex and Devin are only declared and still lack live observation; the universal fallback is an instruction, not enforcement. The certified patch tests TurnVerdict directly and changes no lifecycle enrollment or provider-generated probe. Any backlogged session whose Stop hook is absent or unloaded can still end without invoking the new code, reproducing the original causal failure before marker or backlog logic runs.

Evidence: metasystem/metasystem.conf:5 enables claude,codex,devin. metasystem/docs/design/turn-verdict-delivery-contract.md:12-24 says conformance requires invoking TurnVerdict and honoring shouldBlock, while :26-32 describes the no-hook fallback as an instruction. Its table at :41-45 records Codex and Devin as declared with live observation pending. metasystem/internal/goal/turnverdict_idle_test.go:17-31 exercises the engine directly rather than a provider lifecycle event. The unresolved enrollment boundary is the prior F-CONFIG-003 and F-PROBE-004 evidence at metasystem/records/misc/idle-rootcause-critique.md:20-30.
