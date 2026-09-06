Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal idle-with-backlog-alarm)
Date: 2026-09-06

# Goal

Goal idle-with-backlog-alarm (approved; direction confirmed by Wido on
2026-09-06: "I think this is correct", and "can the steward maybe
intervene and take care of this?"). Its record,
metasystem/plans/goals/idle-with-backlog-alarm.md, is the contract; the
tail of its next step carries the fix set this brief implements.

# Facts (read on 2026-09-06)

- metasystem/internal/goal/turnverdict.go: enforceIdleBacklog blocks a
  Stop whenever claimable goals exist and no delegate job is in flight,
  on every stop, with no per-session slot; the open-work branch blocks
  once per signature using sessionState (fields OpenWorkSignature,
  BlockedGoalRevisions, BlockedQueueDigests). The display text says
  "This refusal does not repeat for the same work", which is false for
  the idle branch. The hard gate itself is deliberate (a74ca7cb,
  2026-09-02, after a seat idled quietly for ninety minutes).
- metasystem/scripts/agents/supervision-hook.sh reads session_id from the
  Stop payload and calls `report turn-verdict --root --session
  --watchdog-surfaced`; it never reads the harness's stop_hook_active
  flag, which Claude Code sets when it re-runs a Stop hook that already
  blocked once. Claude Code also caps consecutive blocks (nine) and then
  overrides, but the count restarts on the next stop sequence, so the
  cap does not end a loop.
- Specimen: this seat, 2026-09-06, about thirty refusal rounds after its
  pinned goals were done under an instruction to stop; each round
  replayed the whole conversation as input.
- metasystem/internal/steward/intervene.go: Intent is the durable
  one-shot record of a continuation (Goal = the claim being continued,
  Role always steward-continuation); metasystem/internal/steward/revive.go
  PrepareIntent mints it under the arbitration lock and writes a receipt;
  the runner's critical section re-runs the predicate, consumes the
  intent and launches the continuation through the LaunchSeam; a
  one-active-continuation guard prevents duplicates.
  metasystem/scripts/agents/roles/steward-continuation.md is the
  unattended session that orients on `goal next` and continues the
  claim; nobody waits on its reply.
- The launch predicate today fires for "claimed work with open state and
  no provably live worker"; whether a live but idle seat main suppresses
  it is the first fact to establish (read the predicate in
  metasystem/internal/steward and name the file and line in the return).

# Decisions (the orchestrator's; decided, not open)

D1. Bounded idle block. sessionState gains `idleBlockDigest` (string)
and `idleBlocks` (int). enforceIdleBacklog computes an idle digest over
the sorted claimable goal ids, this machine's claimed goal ids and the
set of non-terminal job ids. Same digest as stored: idleBlocks
increments; different: idleBlocks becomes one and the digest is stored.
While idleBlocks is below three the verdict blocks as today, and its
text says "refusal N of 3 for this unchanged backlog; at 3 the steward
claims and continues the next goal". State is saved on every path.

D2. Escalation at three. On the third block for an unchanged digest the
verdict, instead of blocking: (a) claims the first claimable goal for
this machine through the goal package's Claim verb with the seat's own
actor (machine plus the announced main's lineage, which the hook already
passes as --main-id; if the claim refuses, fall through to (d)); (b)
mints a steward continuation intent for that goal through the existing
PrepareIntent path with a reason field `seatIdle` set, so the steward's
next tick launches the steward-continuation session; the launch
predicate must accept a seatIdle intent even when the seat's main is
alive (that is the point: the main is alive and refusing); (c) records
the seat's refusal as an incident the steward surfaces in its status
(reuse the steward's existing incident or alert record; name which in
the return); (d) returns success with a display that says exactly what
it did or could not do, so the turn ends. The human idle alarm the
steward already raises stays as it is and fires only when (a) or (b)
could not happen.

D3. The harness flag. supervision-hook.sh reads stop_hook_active from
the Stop payload (missing means false) and passes `--stop-hook-active`
to `report turn-verdict`; the verdict records it on the incident and the
display, and treats a true flag with an unchanged digest as one more
repeat even if the digest computation is unavailable (uncertainty must
never reset the count).

D4. Truth of text. The idle branch's display never claims non-repetition;
it states the count, the bound and the escalation.

D5. Non-goals: the open-work, unreadable and human-blocked branches are
unchanged; the session-stop human verb is unchanged; no change to the
continuation role's contract; nothing under plans.

D6. Pins. Verdict tests: blocks one and two with the counted text; the
third escalates (claim made with the seat actor, intent minted, success
returned, display names both); a changed digest resets the count; a
refused claim still returns success and names the refusal; the human
alarm is not raised when the steward path succeeded. Steward tests: a
seatIdle intent launches the continuation although the main is alive;
the one-active-continuation guard still holds. Hook fixture
(metasystem/scripts/agents/supervision-hook-fixtures.sh): the flag is
read and passed; the third stop for an unchanged backlog ends the turn.

# Gate

From the metasystem directory: `gofmt -l .` prints nothing; `go vet
./...`; `go build ./...`; `go test ./internal/goal/ ./internal/steward/
-count=1`; `bash -n scripts/agents/supervision-hook.sh`; `bash
scripts/agents/supervision-hook-fixtures.sh` (say so if the sandbox
cannot run it). Report each with its evidence level.

# Constraints

Wall-clock budget: 75 minutes. DESIGN-BEARING reach: it amends the
hard turn-exit gate; a code critic reviews the tree next. Declare the
boundary as every file that differs from main. Gap rule: stop and report
a gap with your proposed contract written out; never fill it silently.
The decisions above are decided; a fact above that the code contradicts
is a gap to report, not a decision to remake.
