Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, follow-up round five of chain shr-build1 under goal stop-hook-refusal-carries-verdict, tier 3, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

Fold critic round two of chain shr-build1. The critic's return is
metasystem/artifacts/agents/shr-critic3/rounds/1/return.json; its
findings bind as stated there. Fold F-1 and F-2 by id; F-3 to F-6 are
noted and change nothing. Everything else stands as reviewed.

# The folds, by id

- F-1 (material, unproven): in compose_failed_stop in
  metasystem/scripts/agents/supervision-hook.sh, when the refusal
  record verb fails (record unreadable or unwritable, its lock busy
  past its wait, or an unreadable repeated response), the function
  answers with a system-message-only allow naming the cause and remedy,
  although the verdict, which now runs first, may have said block and
  has already consumed its block-once state (open-work signature,
  unwatched-work digest or goal revision). That one refusal is then
  never shown. Fold: in both record-failure fallbacks, when the
  verdict's decision was block, emit the ordinary verdict block
  (stop_block_json with the display as its detail) and put the
  record-failure notice first in its system message, then the extras,
  then the check-in tail; when the verdict did not block, keep the
  allow but compose its message as the record-failure notice, the
  verdict display, the extras and the check-in tail, in that order.
  Either way the display and the HEALTH line are present.
- F-2 (material, unproven): the round-three fold passes extras, a
  newline and the check-in tail to the refusal record verb; on the
  repeated-cause branch the verb appends that whole supplied message to
  its repeated notice, and the hook strips only the trailing newline
  plus check-in tail, so the extras stay inside the notice and are
  appended again, and an arming line appears three times. Fold: build
  the supplied system message once into a variable, pass that variable
  to the verb, and on the repeated-cause branch strip a trailing
  newline plus that whole variable from the verb's message before
  appending the extras and the check-in tail once. The arming line then
  appears in the Remedy line and once in the extras, as the round-one
  disposition of F-5 accepted.

# Workspace

Your existing worktree for chain shr-build1, on top of rounds one to
four.
May touch: metasystem/scripts/agents/supervision-hook.sh
Must not touch: anything else.

# Constraints

- No test or fixture changes. Do not run the suite; the orchestrator
  runs it seat-side. At most 40 minutes of wall clock.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `bash -n ./scripts/agents/supervision-hook.sh` (expected: exit 0)
- `grep -n 'record failure' ./scripts/agents/supervision-hook.sh` (expected: the two fallbacks, each branching on the verdict decision)
- `git diff --stat HEAD` (expected: the chain's seven files)

Lead riskiestPart with a trace of the four cases: first occurrence
with a working record, first occurrence with a failed record and a
blocking verdict, the same with a non-blocking verdict, and a repeated
cause with an arming failure and a surfaced watchdog report; give the
system message each emits.

# Acceptance Criteria

1. A record failure never drops a verdict block, and its allow (when
   the verdict did not block) carries the display and the HEALTH line.
2. On the repeated-cause branch every extra appears exactly once
   outside the Remedy line.

# Gap Rule

stop and report a gap; never fill it silently.
