# The dispatch path's actual call sequence

Status: MAP, not design. This document records what `dispatch.sh` does
today, step by step, with file:line anchors into the code as of commit
`48442ef`. It exists because the core-vs-plumbing ruling (recorded in
`records/kill-shell/kill-shell.md`'s header) decided the delegate-job choreography
stays shell permanently, and the mission path already learned what that
costs without a sequence map: a reference design died of being written
against an assumed sequence (`docs/design/mission-cycle-sequence.md`
records that lesson). Every claim below is anchored; where behavior is
surprising the surprise is stated as a fact, not a proposal.

Paths are relative to the `metasystem/` root. The shell side is
`scripts/agents/dispatch.sh` (1,493 lines at the pinned commit,
"d.sh" below) and `scripts/agents/adapters/runtime-common.sh`
("rc.sh"). The engine side is `internal/dispatch/*.go` unless another
package is named.

## The shape in one paragraph

`dispatch.sh` is plumbing wrapped around engine verdicts. It parses
arguments, holds directory locks, launches processes, polls files, and
wires environments; at every point where something is decided — may
this caller write, which runtime and model, how many minutes, is this
transition lawful, is this job dead — it calls a `metasystem` verb and
obeys the answer. The record on disk is the single source of truth,
and the only writer of that record is the engine's record lifecycle
(record.go:1-11), reached through `dispatch.sh`'s own `__record-*`
entry points so that every write passes an authority check first.

## Identity and authority, before anything else

Every public command re-execs itself once as
`__lock-owner <tag> <original args>` so that a fresh lease tag is part
of the process command line (d.sh:1440-1449). That is not decoration:
the owner locks below record `(pid, tag)`, and a contender probing a
recorded owner can distinguish a live holder from pid reuse only
because the tag is greppable in the live process's command line.
Internal `__*` entry points never re-exec and never take chain locks.

Entry authority is two-layered:

- Public commands call `lease_entry_check` (d.sh:182-189), which runs
  `lease require-holder` (internal/lease) and captures the claim
  epoch, main id, and caller class. Mutating steps then run through
  `lease run-held` (d.sh:191-200), which re-verifies the epoch around
  the child command.
- Internal entry points each declare a write mode at the router
  (d.sh:1460-1490): `__record-create` and `__record-setup` are
  holder-only, `__record-cas` is record-writer, `__handshake`,
  `__protocol-error`, and `__register-custody` are adapter-writer,
  `__reap-held` and `__launch` are holder-only. The verdict is the
  engine's: `internal_authority` (d.sh:202-213) runs `lease classify`
  and hands the classification to `job authority-check`
  (internal/authority via cmd/metasystem/authority.go). Shell never
  decides who may write; it only picks which mode to ask about.

## The dispatch sequence, in execution order

**1. Argument and role validation (plumbing).** d.sh:799-832: flag
parsing, role file existence, the code-critic `--reviews` rules
(d.sh:820-828), the `--workspace`/`--worktree` exclusivity, and the
rule that `--approve-escalation` requires an interactive TTY
(d.sh:830-832).

**2. Goal admission (engine).** `job goal-admission` evaluates every
claim owned by the dispatching machine and lineage at the one
pre-reservation seam. A structured claim uses its four structured
limits. A claimed goal without that tuple receives a typed refusal
naming its exact goal record. Refusal creates no job record and does
not act on already admitted jobs.

**3. Brief mode (engine).** `job brief-mode` (BriefMode, brief.go:13)
extracts the one filled Working Mode header; a `--mode` override may
agree with the brief but not contradict it (d.sh:833-834).

**4. Roster, tiers, escalation classification (engine).**
`job resolve-roster` returns the roster pair, the requested pair, the
cost direction, and whether escalation approval is required
(d.sh:840-854). The shell keeps only the approval ladder
(d.sh:858-871): an interactive `APPROVE <name>` confirmation
(d.sh:429-444), or — the non-interactive path — a signed mission
envelope allowing the exact pair, checked by
`mission contract-envelope-allows` (d.sh:425-427, 862).

**5. Mission resolution (engine decides validity).**
`resolve_mission` (d.sh:450-467) reconciles `--mission` with inherited
`METASYSTEM_MISSION_ID`/`_LEASE`/`_TURN` environment — disagreement is
fatal, partial inheritance is fatal — then `job validate-mission`
(ValidateMission, mission.go:21) proves the mission has a live,
matching lease.

**6. Census freshness (engine).** Before any state is created,
`require_fresh_census` (d.sh:104-111) runs `job census-fresh`: the
freshness window and the fingerprint match are both the engine's
judgment. This runs before the job id is reserved, deliberately — a
stale census used to cost the caller their chosen job id as well as
their dispatch (d.sh:877-881).

**7. Locks (engine protocol, shell holds them).** The chain lock
(d.sh:321-332) and the repository-wide cap-authority lock
(d.sh:365-384) are both `job owner-lock` claims — one protocol
(OwnerLockClaim, ownerlock.go:88): a directory rename publishes the
lock and its owner in one step, a provably dead holder's husk is
healed, an unprovable one is kept. The EXIT trap releases authorization
if failure occurs before publication; after publication it instead
fails the pending-setup husk and releases both locks.

**8. Cap authorization (engine).** `authorize_job_cap`
(d.sh:949-967): a mission job first refuses any unsigned cap override
(`job resolve-cap --mission`, d.sh:407-409), then asks the mission
fence itself (`mission fence-authorize-cap`, internal/mission); a
non-mission job resolves through `job resolve-cap`'s precedence chain.
Either way the shell then enforces one inequality the engine reported:
the cap must stay below the live watcher's attested ceiling
(`job watcher-ceiling`, d.sh:964-966). This decision completes before
the reservation record is built or published.

**9. The two-phase reservation (engine writes, shell sequences —
by ruling).** `job build-setup` reads the final cap resolution and
renders a pending-setup record whose immutable `capMin`, operation ID,
goal ID, and goal revision are already complete. `__record-create`
(RecordCreate, record.go) publishes that spending fact under the
per-record lock before workspace or adapter setup. A crash before
publication consumes nothing; a crash afterward retains the attempt
and reserved minutes even if setup is later terminalized.

**10. Workspace (plumbing).** `--worktree` creates
`artifacts/agents/worktrees/<job>` on a fresh `agent/<job>` branch;
otherwise the workspace is the repository scope or the named directory
(d.sh:906-913).

**11. Permission envelope (engine).** `job expand-permissions`
(ExpandPermissions, envelope.go:14) expands the preset or envelope
file against the workspace, honoring the repository-wide network floor
`dispatch.permissions.network`, which only ever narrows
(d.sh:469-482).

**12. Capability snapshot (engine, with one shell-owned retry).**
`job snapshot-select` matches runtime, role, adapter config identity,
and the requested envelope (d.sh:484-515). The shell self-heals
exactly one case — a genuine MISS (absent or stale snapshot) triggers
one adapter probe and one re-select — and deliberately refuses to
retry a policy refusal, because a fresh probe would launder an
unenforceable envelope field away (d.sh:498-514).

**13. Payload and prompt (plumbing).** Input size gate from
`dispatch.max-inline-input-kb` (d.sh:969-976), brief hash, the
`artifacts/agents/<job>/rounds/1/prompt.md` assembly from the role
file plus brief (d.sh:525-533, 921-926).

**14. Full record (engine).** `job build-record` renders the complete
record document; `__record-setup` (RecordSetup, record.go:179) may
fill the reservation only while main identity and claim epoch still
match it (d.sh:928-938, 986-988).

**15. Launch (engine launches, shell stamps the deadline).** Locks are
released first — the launch itself runs outside them (d.sh:989-991).
`launch_adapter` (d.sh:535-573) starts the runtime adapter through
`supervise launch-detached` (cmd/metasystem/supervise_arming.go:112):
its own session, logged output, the job's git author identity in the
environment. The shell then waits for the pid's start time to be
readable, and CASes a pending→pending patch carrying pid, pgid, the
trusted-launcher ownership proof, and `handshakeDeadline` — stamped
HERE, at launch, because that is when the dispatcher starts waiting;
deriving it from the record's creation time made the reaper's backstop
run early and overwrite the dispatcher's own verdict (d.sh:554-563).
Only after that patch lands does the start gate file open
(d.sh:571-572).

**16. The handshake window (adapter reports, engine judges).**
`await_handshake` (d.sh:575-600) polls the record until the deadline —
the one from the record, so waiter and backstop work from ONE number
(d.sh:579-584). The transition itself comes from the adapter side:
rc.sh:117 calls `__handshake`, and `job handshake-eval` (HandshakeEval,
handshake.go:8-16) compares the effective permissions the adapter
measured against the requested floor — a grant wider than requested,
or a missing session where the runtime promised one, fails the
handshake — and emits the target status plus patch, applied through
the record CAS (d.sh:1287-1306). Custody registration is also
adapter-initiated: rc.sh:85 calls `__register-custody`, and
`job custody-add` (CustodyAdd, custody.go:8) appends the child pid
under the record lock so registration can never race a status
transition (d.sh:1275-1285).

**17. Waiting, if asked (shell loop, engine verdicts).** With
`--wait`, `wait_for_job` (d.sh:602-625) loops: read status, run a full
lease-held reap pass (`__reap-held`), sleep 100ms. The reap on every
iteration is the surprise worth knowing: the waiting dispatcher IS the
reaper for its own job, so handshake timeouts and budget expiry are
judged by the process that is actually waiting, not left to a backstop.
Without `--wait`, the job id is printed and the caller returns
(d.sh:997-998).

## The reap decision

`reap_one_locked` (d.sh:686-777) is the single-shot, lease-held reap
that `wait_for_job`, `reap`, and the mission drain use. The standing
reaper mode is deliberately gone from shell: Go owns the standing
sweep, and the standing-reaper ruling denies shell reapers kill
authority (d.sh:1233-1239). The decision inputs come from ONE verb —
`job reap-facts` (ComputeReapFacts, reapfacts.go:44) — so the waiting
dispatcher and the Go reaper can never disagree about one record:

- **Terminal records** get bookkeeping only: chain usage aggregation
  (`job chain-usage`, rc 7 means not-yet-aggregatable and is a no-op,
  d.sh:627-640), mission usage aggregation, and the evidence mirror
  (below).
- **pending-setup** is left alone: abandoned-setup debris belongs to
  the Go reapers (internal/supervise's reaper and the missionrunner
  drain), and the grace window is the exported AbandonedSetupGrace
  (reapfacts.go:28-33) so all three measure the same ten minutes
  (d.sh:706-711).
- **The handshake window defers to a live dispatcher.** While
  `handshakeWaiting` is true, the reap stands down if the record names
  no supervisor yet (between create and the adapter publishing its
  identity there is no pid to match — treating that absence as death
  reaped every job in its own launch window) or if the recorded
  supervisor is still live (d.sh:712-734). Liveness is the engine's
  four-way `proc classify` verdict, and `unknown` never acts
  (d.sh:215-222, 237-256).
- **Budget expiry is judged BEFORE process liveness** (d.sh:735-746).
  An expired budget is a fact of the record alone; judging liveness
  first made the verdict a race between the waiting dispatcher
  (timeout) and the standing reaper (process-lost), and the fence's
  cap-min refusal was skipped on the losing side.
- **process-lost:** a non-expired job whose supervisor is provably
  gone gets its group wound down and a `failed`/process-lost CAS
  (d.sh:750-759). **timeout:** an expired running job gets wound down,
  a `timeout`/budget-cap CAS, and — for mission jobs — a batched fence
  ask whose reason distinguishes job-cap-min from wall-clock-hours by
  the record's own capResolution (d.sh:761-776).

Wind-down (d.sh:284-304) is TERM, a bounded wait, an ownership
re-check, then KILL — and it refuses to signal a group it cannot prove
it owns. Ownership proof (d.sh:264-282) is the one deliberate raw-`ps`
read left in this file: the process table is plumbing, and the fake
runtime (whose sandboxes may deny table reads) substitutes the
trusted-launcher proof recorded at launch. A zombie group leader is
explicitly reaped mid-wind-down so it cannot masquerade as a live
writer (d.sh:297-299).

## The record CAS, and why shell never edits a record

Every status change in this file goes through `record_cas`
(d.sh:121-139) into `job record-cas` (RecordCAS, record.go:250). The
engine enforces the lawful graph (record.go:37-46):
pending-setup→failed, pending→{running,failed,cancelled},
running→{completed,failed,cancelled,timeout}; terminal states have no
outgoing edges, identity fields are immutable (record.go:55,
279-280), and a terminal record accepts only its mirror, closure
flags, usage, and recorded exhaustions. The shell wrapper adds two
things only: flight-recorder witnesses for genuine transitions
(FRCC-010), and one-shot patch-file cleanup — twenty call sites used
to mktemp patches nobody deleted, which is how record-locks reached
142k files (d.sh:134-137). A refused CAS (exit 3) is witnessed as
`verdict-refused` with the observed status (d.sh:53-62).

## Failure joins, each with its deciding owner

- **Refusal during setup** (fence, worktree, envelope, capability):
  the EXIT trap fails the dispatcher's OWN pending-setup husk and
  releases its fence slot; the refusal class is derived from the die
  message (d.sh:149-176). Decided by whichever verb refused; recorded
  by the trap.
- **Launch failure:** a pending→failed CAS stamped launch_failed
  (d.sh:991-995).
- **Handshake timeout:** the dispatcher's own verdict, written by
  `__handshake-timeout` (d.sh:1369-1435). Three facts define it: the
  verdict is recorded BEFORE the group is killed (winding down first
  let the standing reaper see a freshly-killed supervisor and write
  process-lost ahead of the true diagnosis); a session that landed
  late stands the timeout down, twice-checked (a session in the record
  means the wait was won, just late); and the CAS retries three times
  because an adapter can move pending→running in exactly this window.
  Each step logs to the job log — this path used to fail silently.
- **Protocol error:** adapter-initiated (`__protocol-error`, rc.sh:179;
  RecordProtocolError, record.go:203). The engine stamps the violation
  idempotently; no reader ever sees failed without its protocolError
  object (proven by internal/dispatch's -race reader test).
- **Cancel:** `cancel` routes through the runtime adapter's own cancel
  verb (d.sh:1181-1188), which calls back into `__cancel-owned`
  (d.sh:1308-1320): lifecycle lock, wind-down, cancelled CAS, mirror.
- **Refused CAS anywhere:** the observed status is surfaced and
  witnessed; the caller's verdict loses to the record (d.sh:53-62).

## Follow-up rounds

`follow_up` (d.sh:1017-1155) reuses the sequence with these
deviations, in order: the chain lock is taken on the ROOT id; a
worktree chain that has fallen behind main gets a WORKTREE-BEHIND
warning, not a refusal (d.sh:1044-1052); the newest chain record must
be `completed` or `failed` with protocol_error — anything else needs a
fresh dispatch (d.sh:1055-1058) — and must carry a session id; the
child id is `<root>-r<round>`; a design-critic's workspace is
synchronized only when it shares history with this repository
(fast-forward to HEAD), while an independent repository reviews its
own head and nothing is merged into it (d.sh:1070-1099); the
critique-exhaustion policy (`job critique-exhaustion`, critique.go)
may record successor patches across the chain before the new round
launches (d.sh:1100-1109); the cap is re-authorized for the child; the
parent's REQUESTED permissions are reused verbatim while the snapshot
is re-selected fresh (d.sh:1124-1128); and a runtime whose snapshot
says it cannot resume gets the fresh-context fallback — prior brief,
prior return, and correction concatenated into one prompt, dispatched
as a new session rather than resumed (d.sh:1130-1141).

## Close

`close` (d.sh:1190-1231) takes the chain lock, mirrors every terminal
member first (a reap mirrors only its own job's round; follow-up
rounds otherwise reach the close unmirrored, d.sh:1210-1218), then
lets `job close-check` (CloseCheck, close.go:13) decide: every record
terminal, evidence durable, exhaustions resolved. The chainClosed flag
lands as a self-edge CAS on the root. Mirroring itself
(d.sh:655-684) stamps ONLY the job that was actually mirrored —
stamping the whole chain from one job's mirror once wrote durability
claims for evidence that never landed, and the close then refused a
chain every record of which CLAIMED to be mirrored.

## What is deliberately shell

Reading this map back against the ruling: the two-phase reservation
sequencing (by explicit INVARIANT), the EXIT-trap husk cleanup, the
lock holding, the launch-and-poll loops, the prompt assembly, the raw
`ps` group-ownership read, and the wind-down signalling are plumbing
and stay here. Every verdict they sequence — authority, freshness,
roster, caps, envelopes, snapshots, transitions, reap facts, handshake
grades, close proof — is an engine verb, and the record CAS refuses
anything the graph forbids no matter what the shell asks for.
