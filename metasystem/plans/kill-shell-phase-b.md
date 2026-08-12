# Kill-shell Phase B: the dispatch lifecycle layer

Working Mode: design
Mission Stream: kill-shell

Status: DRAFT under critique. Parent authority: plans/kill-shell.md
(ACCEPTED), whose Phase B paragraph fixes this phase's scope. This
document turns that paragraph into an implementable design; where the
two disagree, the parent plan wins and the disagreement is a finding.

## Scope

Phase B ports dispatch.sh's lifecycle CHOREOGRAPHY to Go: when each
lock is taken, held, and released; liveness sequencing; wind-down
ramps; CAS choreography; cap resolution. The lock PRIMITIVE for chain
and lifecycle locks already lives in Go
(internal/dispatch/ownerlock.go, relayed through `metasystem dispatch
owner-lock`). The cap-authority lock is still a bare mkdir/rmdir
directory in BOTH dispatch.sh and arm-supervision.sh; this phase ports
that primitive to the Go owner-lock discipline under a shared-lock
interoperability contract. Launch/handshake/wait choreography stays
with Phase C (it is the adapter boundary); arm-supervision.sh itself
stays with Phase D.

## What the shell does today (the behavior to pin)

Locks. Three locks, two protocols:

1. Chain lock, `artifacts/agents/locks/<root>.d` — rename-born
   owner-lock (Go primitive). Shell choreography: on busy, read
   owner.json, classify the holder (lock_owner_state: dead, live,
   stale, unknown), then die "chain is busy" or "owner liveness cannot
   be verified" or "no owner lease". Release dies only on not-owner.
2. Lifecycle lock, `artifacts/agents/records/locks/<job>.lifecycle.d`
   — same primitive. Two acquisition modes: try-once (standing reaper
   backs off to its next tick) and until-deadline
   (acquire_lifecycle_lock_until: 0.05s poll, cap scaled by
   dispatch_fixture_wait_cap, timeout message carries elapsed and the
   scaled cap). Release swallows every failure.
3. Cap-authority lock, `artifacts/agents/supervision/
   cap-authority.lock.d` — bare `mkdir` polling with the same scaled
   deadline (base 10s), `rmdir` release that DIES when the directory
   disappeared or is not empty, and a held-flag so release is a no-op
   when this process never acquired. arm-supervision.sh carries a
   byte-similar copy coordinating the same directory.

Liveness sequencing. job_supervisor_matches (pid+tag process match,
with the fake runtime's ownershipProof+heartbeat fallback);
group_owned (pgid+tag in the process table, with the fake runtime's
ownershipProof fallback); wind_down_group's ramp: refuse an unowned
group; TERM the group; poll 2s; if still alive re-prove ownership
(refuse when lost) then KILL; reap the direct child's wait status so a
zombie leader cannot masquerade as a live writer; poll 2s; fail loudly
when the group survived KILL.

Reap choreography (reap_one / reap_one_locked). The contention rule:
a standing reaper that finds the lifecycle lock busy returns success
and comes back next tick; an explicit `reap --job` waits up to a
scaled 5s and FAILS on timeout. The verdict ladder, in order:
terminal statuses only mirror/aggregate (standing reaper skips them);
the stale-claim-epoch sweep (standing only, record epoch older than
the live lease epoch: wind down unless pending-setup, CAS to failed
with error stale-claim-epoch); reap-facts (one Go verb call already);
pending-setup abandonment (standing only, CAS to failed
abandoned-setup); the handshake window (defer while a dispatcher is
still waiting, unless the record names a supervisor that is provably
gone; a record with no pid yet is deferred, never reaped); budget
expiry judged BEFORE process liveness (verdict priority: an expired
budget is a fact of the record alone); process-lost (wind down, CAS
to failed, events, mirror); budget-cap (wind down, CAS to timeout,
events, mission fence refusal with job-cap-min or wall-clock-hours by
capResolution.truncatedBy, usage aggregation, mirror).

CAS choreography. record_cas relays to the Go verb and appends a
custody event on refusal (cas-refused with the holder). The
choreography decides WHICH transitions are attempted with WHICH patch
under WHICH lock; the transitions themselves are already Go.

Cap resolution. resolve_nonmission_cap (flag over env over conf over
default, provenance via config_key_origin, signed-envelope refusal
for mission jobs, watcher-ceiling attestation),
refuse_unsigned_mission_cap_override, model_tier /
configured_tier_indices / assert_tiers_contiguous,
confirm_escalation (interactive confirmation with cost direction),
signed_dispatch_envelope_allows. All decision-shaped; some already
consult Go verbs (dispatch cap-resolution, dispatch watcher-ceiling).

## Design

### B0 — characterization fixtures first (no port before pinning)

A new sequencer, scripts/agents/lifecycle-fixtures.sh, registered in
the disposition registry and budgeted, pins every branch the suite
does not reach today, running against the SHELL implementation and
kept green over the port. One fixture per branch:

1. Rename-born publication: a contender that loses the staging rename
   never observes an ownerless lock (loop a claimant against a
   holder; assert no "no owner lease" diagnosis ever fires).
2. Holder classifications, all six observable outcomes of a claim
   against an existing lock: ownerless husk healed and claimed; dead
   holder taken over; stale holder (pid alive, tag gone) taken over;
   live tagged holder refused busy; live-by-EPERM holder refused
   busy; unreadable-identity holder refused busy (unknown).
3. Non-owner release: chain-lock release by a non-owner dies loudly;
   lifecycle release by a non-owner is swallowed; cap-authority
   release when the directory vanished dies loudly ("disappeared or
   is not empty").
4. Lifecycle-lock timeout scaling: with the fixture wait-cap knob
   set, the timeout message carries the elapsed and the scaled cap.
5. Standing-vs-explicit reaper contention: a held lifecycle lock
   makes the standing reaper return success untouched while an
   explicit reap of the same job waits and then fails on timeout.
6. Cap-authority acquisition timeout AND lock disappearance: a held
   directory times out an acquirer with the scaled-cap message; a
   release after external rmdir dies loudly.
7. Wind-down refusal ramps: an unowned group is refused before any
   signal; ownership lost between TERM and KILL is refused; a group
   surviving KILL fails loudly. (Real process groups via the fake
   runtime's trusted launcher, as the suite's conformance fixtures
   already do.)
8. Reap verdict ladder ordering: budget-expired-with-dead-process
   reads timeout/budget-cap from BOTH a waiting dispatcher and a
   standing reaper (the recorded race), pending job never reads
   timeout, handshake window defers until the supervisor is provably
   gone.

These fixtures are the acceptance instrument for every later step:
each port lands only when the fixture file is green against it
unchanged.

### B1 — cap-authority lock to the Go discipline, protocol frozen

New verb pair on the dispatch family:

    metasystem dispatch cap-lock --acquire --root R --wait-cap-sec N
    metasystem dispatch cap-lock --release --root R --held BOOL

The verb keeps the ON-DISK PROTOCOL bit-compatible: a bare directory
made with mkdir and freed with rmdir, no owner.json, because
arm-supervision.sh (Phase D) still coordinates the same directory
with shell mkdir/rmdir. The Go side adds no takeover and no healing —
under the bare protocol a holder cannot be identified, so takeover
would be theft; the only remedies stay timeout (acquire) and loud
death (release after disappearance). SHARED-LOCK REGISTRY entry: the
cap-authority lock carries this interoperability contract until
arm-supervision.sh ports in Phase D, PLUS a mixed-era contention
fixture (shell holder vs Go acquirer, Go holder vs shell acquirer)
in lifecycle-fixtures.sh. Upgrading the protocol to the rename-born
owner-lock (identifiable holders, dead-holder takeover) is RECORDED
as a Phase D step that lands only after both participants are Go.
dispatch.sh's and arm-supervision.sh's acquire/release functions
become one-line relays to the verb (arm-supervision's relay is
allowed early because the verb speaks the frozen protocol).

### B2 — the reaper moves whole

One verb owns the entire locked reap:

    metasystem dispatch reap-job --root R --job J --standing|--explicit

It performs lifecycle lock acquisition under the contention rule
(try-once standing, scaled 5s deadline explicit), the full verdict
ladder, wind-down ramps, CAS with custody events, mission fence
refusal, usage aggregation, and mirroring — in Go, whole. The TODO in
dispatch.sh already states the rule this follows: liveness probing
and lock custody are one atomic unit and move whole or not at all.
Process work uses the identity/census verbs' existing kernel probes
(internal/identity), signaling via unix.Kill on the process group;
the fake runtime's ownershipProof/heartbeat fallbacks port as read
paths of the same verb. reap_jobs in dispatch.sh becomes enumeration
plus one verb call per job; reap_one/reap_one_locked/wind_down_group/
group_owned/job_supervisor_matches/lock_owner_state are deleted from
the shell. The verdict ladder's decision fragments already in Go
(reap-facts, record CAS, mission-fence refuse) become internal calls
inside the verb — the shell stops sequencing them.

Authority: reap-job derives the caller classification from the same
live lease and process inspection the shell router performs
(internal_authority's lease classify), never from caller-supplied
claims — the Phase C provenance rule applies here first.

### B3 — chain and lifecycle lock choreography

The busy-diagnosis choreography (read owner, classify, die with the
exact message) moves into the owner-lock verb behind a new
--diagnose flag used by acquire paths:

    metasystem dispatch owner-lock --command claim --dir D --pid P \
        --tag T --diagnose chain|lifecycle-until:SECONDS

claim+diagnose chain reproduces acquire_chain_lock's three refusal
texts; lifecycle-until reproduces the polling loop, the scaled cap
(the fixture wait-cap knob resolves through the same config engine),
and the timeout message. dispatch.sh's acquire/release functions
become relays; the wrapper functions are deleted where call sites can
call the verb directly.

### B4 — cap resolution to one verb

    metasystem dispatch resolve-cap --root R --role ROLE --runtime RT \
        --model M [--override N] [--mission ID] [--interactive]

subsumes resolve_nonmission_cap, refuse_unsigned_mission_cap_override,
model tier arithmetic (tier lookup, contiguity assertion, escalation
direction), signed_dispatch_envelope_allows, and watcher-ceiling
attestation, emitting the cap JSON with provenance or the exact
refusal texts. confirm_escalation's terminal prompt is the one
interactive piece: the verb emits the question and expected-answer
contract on stdout with a distinguished exit code, and dispatch.sh
keeps the three-line read/compare (interactivity is harness custody,
not a decision). Fixture: the escalation refusal and confirmation
paths replayed against both implementations.

## Order and verification

B0 lands first and alone (fixtures green against shell). Then B1, B2,
B3, B4, each: port, lifecycle-fixtures.sh green unchanged, behavioral
diff on the touched verbs' stdout/stderr/exit codes, suite green from
a pristine worktree, registry updated (dispatch.sh's budget numbers
ratchet DOWN with every step), coverage floors only rise. The phase
ends with dispatch.sh parsing flags and consulting verbs for every
lifecycle decision, arm-supervision.sh still shell but speaking the
frozen cap-authority protocol through the same verb, and the
shared-lock registry carrying the Phase D upgrade note.

## Non-goals

No launch/handshake/wait port (Phase C), no arm-supervision.sh port
(Phase D), no protocol upgrade of the cap-authority lock (Phase D,
recorded), no new lock kinds, no behavior fixes — divergences found
while pinning become findings against this document, not silent
corrections.
