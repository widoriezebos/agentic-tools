# Idle closing build — critique, 4 material (2 bar-(a) holes, 2 forge-class)

Critic: codex gpt-5.6-sol (idle-close-critique) over reviewedTree 4e20ff3d.
Trajectory 7->4. Two are genuine bar-(a) fail-open holes (F-002 stop-hook,
F-003 template root). Two are DELIBERATE-FORGE class (F-001 agent mints the
classification proof; F-004 marker replay) - same territory as raw-byte
forgery, scoped to goal ledger-authentication (bar b), NOT bar (a)'s honest-
agent bar. Bar (a) closes when F-002 and F-003 close.

## F-CALLER-PROOF-MINT-001 [critical]

The sanctioned library still lets an agent mint the supposedly human-only classification proof. An in-repository agent caller can read the enrolled terminal process identifier, call the public proof function with that terminal identifier rather than its own caller ancestry, and pass the returned valid proof to WriteSessionStop. This is the original library authorization bypass, not raw-byte forgery, and requires a different caller-binding contract and adversarial fixture.

Evidence: metasystem/internal/humanauthority/authority.go:381 publicly returns the enrollment, including TerminalRef.PID. metasystem/internal/humanauthority/authority.go:478 accepts an arbitrary invokerPID without proving that it is the calling process or its real parent; starting at the enrolled terminal reaches the success branch at lines 539-553 without traversing the agent. metasystem/internal/goal/sessionstop.go:226-243 accepts any in-process Proof whose ValidFor check passes and never classifies the actual library caller. The fixture at metasystem/internal/goal/turnverdict_idle_test.go:185-197 rejects only an empty Proof and does not exercise this public proof-minting path.

## F-STOP-ERROR-NONBLOCKING-002 [critical]

The Claude Stop hook still has ordinary failure paths that end the turn without a blocking decision. A missing engine emits only a system message, and a turn-verdict command failure is deliberately converted to the same non-blocking message form. Either can occur accidentally while claimable backlog exists, so fail-closed completeness is not achieved even though deadline overruns themselves now block.

Evidence: metasystem/scripts/agents/supervision-hook.sh:85-89 emits only systemMessage and exits successfully when the engine is missing. The helper at lines 198-200 creates only systemMessage JSON. Lines 365-379 use that helper when turn-verdict fails, whereas only lines 349-356 construct the provider's decision=block response. The missing-engine fixture at metasystem/scripts/agents/supervision-hook-fixtures.sh:116-134 asserts loudness but never asserts a blocking decision.

## F-TEMPLATE-HOLDER-ROOT-SPLIT-003 [high]

Template-mode state-root resolution is incomplete at the hook boundary, so the attended-human escape cannot reach its consumer. Session arming and stop markers live beneath the metasystem installation, but the hook classifies the main against the containing Git root. It therefore supplies no matching holder main identifier to the verdict, which refuses the valid marker. A real human cannot reliably end a template session until the hook's classification and renewal calls use the same resolved state root.

Evidence: metasystem/cmd/metasystem/up.go:139-145 deliberately maps template announcement and lease state to the nested installation; metasystem/cmd/metasystem/up_test.go:36-90 proves that the containing Git root is a separate state scope. The hook nevertheless derives the containing Git root at metasystem/scripts/agents/supervision-hook.sh:124 and passes it to both classification calls at lines 181 and 190. TurnVerdict resolves its Store to the nested installation at metasystem/internal/goal/turnverdict.go:167-171, and the session-stop command does likewise at metasystem/cmd/metasystem/session_stop.go:51-55. Consumption then requires the hook-supplied main identifier to match at metasystem/internal/goal/sessionstop.go:377-379. The template fixture at metasystem/internal/goal/turnverdict_idle_test.go:130-158 bypasses the hook by supplying main-1 directly, so it does not cover this split.

## F-SESSION-END-REPLAY-004 [high]

The marker is still not bound to a durable session-end event. Its lifecycle token is only a hash of the currently present announcement; no-hook runtimes do not retire that announcement, and the Claude SessionEnd path suppresses retirement failure. If the same session and holder processes remain alive, an unconsumed marker survives the logical end and authorizes a later stop. This is a second remaining every-runtime gap and therefore disproves the claim that notify-only enforcement is the only deferred item.

Evidence: metasystem/internal/goal/sessionstop.go:180-220 derives the lifecycle token solely from the current announcement. metasystem/scripts/agents/supervision-hook.sh:400-404 is the only supplied lifecycle-end integration and discards every retirement error. metasystem/internal/lease/verbs.go:126-170 reuses an unchanged announcement for the same live process. The consumer at metasystem/internal/goal/sessionstop.go:398-424 accepts the marker whenever that unchanged hash and live identities still match. The fixture at metasystem/internal/goal/turnverdict_idle_test.go:266-287 simulates a successful lifecycle change by manually rewriting AnnouncedAt; it does not cover a missing hook or failed retirement that leaves the announcement unchanged.
