# Lifecycle lessons from the morpheus/yoda session of 2026-08-13/14

Empirical incidents from a 30+ hour production session (Claude Code
harness, morpheus + yoda-internal repos), exported per the two-way
exchange convention. Each lesson is dated, artifact-backed
(morpheus-evidence/ + the session's SESSION-STATUS.md), and mapped to
the orchestration watch-list area it evidences. These are inputs for
the design's own change gate — not edits to the design.

## L1 — Background-task reaping must be a published contract with attributed stops

Observed: the harness silently reaps its own long-running background
bash tasks (~90 min horizon in our sample) with a graceful SIGTERM and
a bare "was stopped" notification. Three monitoring shells and one
production yoda server died this way; the server death was initially
unattributable and consumed an owner-escalation ("this is a bit
scary") plus a canary experiment to exonerate foreign sessions.
Proof: a canary PAIR — one harness-owned background task, one
independent nohup process — the harness one was reaped at ~90 min, the
independent one survived (2026-08-14 14:20 check).

Requirements for the future harness: (a) the task-lifecycle contract
(what is reaped, when, what survives — e.g. persistent monitors
survived >24h) is PUBLISHED to the agent at launch (a TTL in the
launch acknowledgment); (b) every stop notification carries
ATTRIBUTION (policy-reap | user-stop | error | supervisor), so no
agent ever runs a canary experiment to learn who stopped its process;
(c) a sanctioned long-runner primitive exists for servers, distinct
from background shells. Maps to watch-list "host and runner lifecycle"
(C11-2 death verification, C11-5 verified start).

## L2 — Terminal-event delivery is lossy; job status must be pull-queryable truth

Observed: at least two job-completion events never surfaced to the
agent (one cost ~8 hours of an unattended window — the job had
completed 2.5 minutes after dispatch at a gate; the agent believed it
running because a leftover write-canary file read as liveness).
Countermeasure that then worked ~15 times: at first check-in AND
before every "waiting on X" claim, read the job RECORD's status field
directly; arm a dedicated per-critical-job watch with BOTH terminal
and staleness (30 min log-idle) conditions.

Requirements: at-least-once delivery of terminal events; a cheap
first-class status query the agent is EXPECTED to poll at check-ins;
watch primitives with terminal+staleness semantics built in (a
status-only watcher loops forever on a hung job — the morpheus
watchdog-staleness rule, re-proven here). Maps to C11-1
(heartbeat/STALE suppression) — note our variant: the false liveness
signal was an agent-side artifact (canary file), reinforcing that
liveness must come from the record, never from side effects.

## L3 — Verified start is real: a flag-order typo produced a 52-second silent death

Observed: a server launch with a misplaced CLI flag exited in under a
minute; only the record-status check caught it (the launch itself
returned success). Separately, a dispatch subagent's first attempt
misfired into a help query and completed "successfully" in 52 seconds
having done nothing. Requirements: the C11-5 verified-start handshake
(signal + grace + failed-start transition) is worth its cost; and
dispatch acknowledgments should carry enough of the child's first
output to distinguish "started the task" from "answered a different
question".

## L4 — Write-canary-first for delegated write work

Observed: a 40-minute analysis was lost to a silently read-only
sandbox; after adopting canary-first (the delegate's FIRST action
creates its output file as a stub, stop-on-failure), three subsequent
sandbox-rooting mistakes were caught in seconds instead of hours.
Requirement: make output-writability probing part of the delegation
contract, not agent folklore. (Complement of L2: the canary proves
writability ONLY — never liveness.)

## L5 — Owner approval cannot reach the automated permission layer

Observed: the machine owner explicitly approved an action ("This is
approved") and the automated classifier still blocked the agent three
times; the work proceeded only when the owner hand-ran commands or the
agent found a differently-shaped route (the harness's own background
runner instead of nohup — legitimate, but discovered by trial).
Requirement: the future harness should support scoped, recorded,
owner-granted permissions that the enforcement layer actually
consults, so an explicit grant does not require command-shape
archaeology.

## L6 — The canary-pair discriminator (recipe worth keeping)

When something kills processes and no log names the sender: launch one
supervised task and one independent tagged process, watch both, and
snapshot all agent processes at the moment either dies. The surviving
member identifies the killer's CATEGORY (supervisor policy vs
machine-wide actor) without root-level tooling. Cheap, decisive,
reusable.

## L7 — Shared-machine session hygiene

Observed: four Claude sessions with --allow-dangerously-skip-permissions
from Aug 4-6 still resident during the investigation (workdirs:
morpheus, yoda-internal, agentic-tools, agent-workbench) — exonerated
in this incident but each a standing member of every future suspect
list, and each holding whatever they hold. Requirement: the future
harness's session registry should make resident sessions and their
ages VISIBLE and cheaply closeable; stale skip-permissions sessions
are the single largest unaccounted actor class on a shared machine.
