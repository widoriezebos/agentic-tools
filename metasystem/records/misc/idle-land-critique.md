# Idle gate critique — one hole from landing

Critic: codex (idle-land-critique). 1 material: F-SESSION-END-REPLAY-004.
Marker consumption is deferred to a best-effort SessionEnd that can fail/
timeout, leaving the marker reusable by a later lifecycle in the same
process - an HONEST path. Fix: consume-on-use at authorization.

## F-SESSION-END-REPLAY-004

Terminal statement: No. Even accepting the documented finite Claude Stop-hook block cap, the gate does not fail closed on every honest or accidental path within the hook's control. Finding F-SESSION-END-REPLAY-004 remains open: if SessionEnd cannot spend the marker and announcement retirement is absent, times out, or also fails, a later lifecycle in the same live Claude process can consume the unchanged legitimate marker. The later agent performs no forgery or deliberate replay.

Evidence: metasystem/scripts/enforcement/claude-code-hooks.json:31-38 gives SessionEnd three seconds. metasystem/internal/goal/goalverbs.go:91-117 permits marker retirement to spend two seconds waiting for the goal lock before failing. metasystem/scripts/agents/supervision-hook.sh:555-570 merely surfaces that failure and suppresses failure of the subsequent announcement retirement. metasystem/internal/goal/sessionstop.go:371-405 records the authorization as spent only when EndSessionStop completes, while lines 443-485 authorize an unrecorded marker when the holder, human process, and lifecycle still match. metasystem/internal/lease/verbs.go:126-170 reuses the same live process's unchanged announcement. The fixtures at metasystem/scripts/agents/supervision-hook-fixtures.sh:477-506 and metasystem/internal/goal/turnverdict_idle_test.go:318-350 make EndSessionStop succeed and fail only the later announcement retirement, leaving the combined failure path untested.