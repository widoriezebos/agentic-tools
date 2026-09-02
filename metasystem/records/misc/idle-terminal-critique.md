# Idle gate terminal critique — the architectural wall

Critic: codex gpt-5.6-sol (idle-final-critique-b). 4 material. The
decisive finding: Claude Code's harness CAPS Stop-hook blocks, so no
hook-based gate can make idle impossible - after the cap the turn ends.
The critic also REFUTED the seat's forge-deferral of F-SESSION-END-
REPLAY-004: an honest agent reaches it, so the deferral was dishonest.
CONCLUSION: idle-impossible requires the STEWARD (runtime-independent)
to actively enforce - the every-runtime goal - not the turn-exit hook.

## F-CLAUDE-STOP-BLOCK-CAP-005 [critical]

Claude Code's finite Stop-hook block cap defeats the claimed invariant without forgery. A persistent honest failure such as unavailable ledger state makes this hook return the same refusal repeatedly, but Claude Code overrides a Stop hook after eight consecutive blocks without progress. The repository neither handles stop_hook_active nor establishes a different process-level cap, so an honest agent can eventually end the turn while backlog remains.

Evidence: metasystem/scripts/enforcement/claude-code-hooks.json:16-27 installs the command as a Claude Stop hook. metasystem/scripts/agents/supervision-hook.sh:431-496 repeatedly emits a block for unavailable or unsafe verdicts, but an exact repository search found no stop_hook_active or CLAUDE_CODE_STOP_HOOK_BLOCK_CAP handling. The official Claude Code hooks guide documents the eight-block override: https://code.claude.com/docs/en/hooks-guide.

## F-CLAUDE-HOOK-LAUNCH-FAILOPEN-005 [critical]

The Claude Stop-hook launcher remains fail-open before the repaired script can run. If the project directory cannot be entered, or the hook script is missing or not executable, the configured shell command exits with status 1 or 127 and emits no blocking decision. Claude Code treats those statuses as non-blocking, so ordinary installation drift or directory failure can end a backlogged turn.

Evidence: metasystem/scripts/enforcement/claude-code-hooks.json:25 runs `cd "$CLAUDE_PROJECT_DIR" && bash scripts/agents/supervision-hook.sh claude stop` without converting launcher failure to exit status 2 or decision=block. The official Claude Code hooks reference states that a hook that cannot start and exit statuses other than 2 are non-blocking for Stop events: https://code.claude.com/docs/en/hooks.

## F-STOP-PARTIAL-OUTPUT-006 [high]

The deadline parent mistakes any non-empty child output for a valid Stop decision. The child's emission helper deliberately returns success after a failed stdout write. If temporary storage fills after a response is partially written, the worker exits successfully, the parent forwards the non-empty truncated JSON, and Claude Code treats the malformed response as non-blocking. This is an accidental F-STOP-ERROR-NONBLOCKING-002 path.

Evidence: metasystem/scripts/agents/supervision-hook.sh:54-58 tests only worker status and whether the captured file contains a non-whitespace byte; it never parses the response. At metasystem/scripts/agents/supervision-hook.sh:326-331, a failed response printf is recorded but converted to return status 0. The official Claude Code hooks reference says malformed structured output on an exit status other than 2 is a non-blocking error: https://code.claude.com/docs/en/hooks.

## F-SESSION-END-REPLAY-004 [high]

F-SESSION-END-REPLAY-004 is reachable by an honest agent and therefore cannot honestly be deferred as deliberate forgery. A human can create a legitimate unused marker, a SessionEnd retirement can fail or be absent, and a later resumed lifecycle in the same still-live Claude process can encounter the unchanged announcement and automatically consume that marker. The later agent neither fabricates a proof nor copies, edits, or deliberately selects the marker.

Evidence: metasystem/scripts/agents/supervision-hook.sh:512-516 suppresses every retirement failure. metasystem/internal/lease/verbs.go:126-170 returns an existing same-process announcement without changing its session or announcement time. metasystem/internal/goal/sessionstop.go:172-224 hashes only those announcement bytes, and metasystem/internal/goal/sessionstop.go:404-430 authorizes and consumes the marker when the unchanged hash and identities still match. The supplied test at metasystem/internal/goal/turnverdict_idle_test.go:294-316 proves only that manually replacing the announcement changes the hash; it does not exercise failed retirement with the old announcement left intact.
