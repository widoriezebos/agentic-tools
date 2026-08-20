# The idle watchdog: open work is never silently idle

STATUS: CONVERGED at round 5 (2026-08-20) — "No material findings…
CONVERGED — build it." Five rounds: 9, 6, 5 (the declared failsafe,
which froze scope to invariants), 5, 0. The same-user refutation
stands on D118 and the orchestration record (isolation against
malicious same-user code belongs to containers, VMs, or separate
users — never the metasystem); the critic's wording correction is
folded: it is UNCONTAINED code running as the account user that
already holds the account's scheduling authority — contained
runtimes enforce workspace-scoped writes, and the steward
introduces no new privilege domain either way. Build proceeds
against the matrix, fake-adapter fixtures first. Goal idle-watchdog
(D121).

## The invariant

Stated honestly in two tiers (IW-R3-01): a DEAD, UNKNOWN, or
DEGRADED condition with open work is notified within ONE tick. A
LIVE worker with no progress is notified within the noise threshold
(steward.stale-ticks, default 5 ≈ fifty minutes) — physics allows
nothing faster without false-alarming every long quiet reasoning
stretch; the D121 incident becomes a fifty-minute notification, not
a ten-hour silence. Where a worker is PROVABLY DEAD, a continuation
launches through the standing dispatch machinery — after the
operator notification is delivered, never before. VISIBILITY HAS
TWO CHANNELS (IW-R3-02): the configured notifier (delivery =
command exit 0, the defined acknowledgment), and — independent of
it — every interactive session opened in the repository surfaces
pending steward incidents at session start, and `steward status`
shows them; so the precise claim is: no stall persists unseen by an
operator reachable through the notifier OR opening any session. A
notifier outage is itself a pending incident, retried each tick. No
response depends on any agent remembering anything.

## The verdict ladder (IW-R2-01, IW-R2-02)

DEAD is a PROOF, not an absence (IW-R2-01): the worker set is the
census over enrolled sessions, their delegate jobs, live gates,
mission runners, and monitored runs. Only a COMPLETE census in which
every relevant identity is provably absent returns DEAD. Any owned
live worker means LIVE. Any relevant UNTRACKED record, unreadable
store, weaker-identity record (delegate epoch-seconds identities
cannot prove death), or incomplete census means UNKNOWN — and
UNKNOWN dominates DEAD.

- DEAD + open work → notify (delivery-gated), then revive this
  tick. No quiet threshold: dead seconds after a fresh commit is
  still dead (round-1 fixture).
- LIVE + evidence high-water advanced within steward.stale-ticks →
  HEALTHY.
- LIVE + no advance past the threshold → STALLED-IDLE →
  NOTIFY-ONLY, repeating each tick. v1 NEVER launches a second
  writer beside a live holder (IW-R2-02): the standing dispatcher
  requires the checkout holder and lineage owner, and displacing a
  live one has no lawful path today. Live-holder succession is the
  named follow-up goal steward-succession; until it lands,
  visibility IS the response for live-idle — the D121 incident
  becomes a phone notification within one tick period instead of a
  ten-hour silence.
- UNKNOWN → notify, never spawn.

## Open work, both formats, degraded-honest (unchanged from round 2)

Legacy: a Current goal exists. New format: a goal claimed by THIS
machine (quota key), read from the validated accepted projection;
v1's one-claim quota makes machine scope claim scope; an arc is one
unit; foreign claims are never this steward's business. A
non-terminal owned transaction-journal entry is open work. Missing,
malformed, mixed, or unreadable ledger state is DEGRADED → notify,
never no-work; only a valid Goal-free (or validly empty claims and
journal) is no-work.

## Progress evidence: durable high-water marks (IW-R2-06)

No wall-clock freshness anywhere. Evidence = exactly two durable
identities, persisted as high-water marks with the tick count of
their last change:

- the checkout HEAD object id;
- the opid set of this machine's claimed goal files' History.

Staleness = ticks since either mark changed (steward.stale-ticks,
default 5 at ten-minute ticks ≈ 50 min). Receipts are NOT evidence:
the wire format carries no provenance, so "not steward-written" is
not computable — dropped rather than faked; the honest cost
(receipt-only activity reads idle) lands in notify-only territory
by construction. HEAD is repository-wide, so an unrelated commit
does refresh the machine-level verdict — accepted and stated: with
the one-claim quota the machine IS the claim, and per-path
attribution would be D114 brittleness. Identical marks across ticks
age monotonically; future-dated anything is irrelevant because no
timestamps participate.

THE DRY-COUNT (spawn-loop fence): steward.max-revivals (default 3)
consecutive revivals without a HIGH-WATER ADVANCE switch to
notify-only. Continuation outcomes — receipts, return records,
blocked or parked results — NEVER reset the count; only HEAD-oid or
History-opid advance does. ONE ACTIVE CONTINUATION: an open
continuation job record suppresses any further dispatch until the
reaper closes it; crash windows consume at most one durable attempt
(the intent record below is the attempt's identity).

## Revival: notify, arbitrate, dispatch, reap

ORDER, pinned (IW-R2-03): (1) durable intervention-intent record +
receipt; (2) operator notification SENT AND DELIVERED — delivery
gates launch; a notifier outage means no launch, the intent stays
pending, retried each tick; (3) full predicate re-run inside the
arbitration lock (a worker turned live, fresh high-water, or an
open continuation job aborts before any job record exists); (4)
dispatch; (5) launch stamp. LINEARIZATION, the atomic point named
(IW-R3-03, IW-R4-01): the shared lock is HELD from the final fence
check through the adapter's dispatch return — reservation, intent
consumption, and launch are one critical section; an enrollment
arriving during it blocks on the lock and, on acquiring it, finds
the open continuation job. An enrollment landing BEFORE the
critical section bumps the fence and CANCELS the reservation
(record cleaned, incident noted). The honest residue, stated: a
worker enrolling in the instant after launch coexists with the
continuation exactly as two operator terminals coexist today — the
checkout lease and claim machinery arbitrate writes; the fence
minimizes the window, the lease owns the residue. Continuation job
records carry the checkout-lease generation so takeover invalidates
them like any delegate. INTENT CONSUMPTION IS LAST (IW-R4-02): the
dispatch owner admits consumption only inside that same critical
section, after delivered-notification state and fence validation —
an intent is never usable before its prerequisites hold. Crash
between 4 and 5 reconciles next tick as a notified unknown
outcome.

THE LAUNCH PATH IS THE DISPATCHER (IW-R2-05): ordinary NO-WAIT
dispatch with a FORMAL RETURN CONTRACT — the steward-continuation
role ships with a role file, requirements, and return schema (goal
worked, outcome, receipts written); roster resolution owns runtime
selection (no steward runtime key; the core names no runtime; the
fake adapter registers the role in fixtures). THE STEWARD'S NEXT
TICK IS THE REAPER: it validates returns, terminalizes, mirrors,
closes the chain, and retries failed bookkeeping — no permanently
open continuation exists and no waiting consumer is required.

## Authority: an authenticated installation identity (IW-R2-04)

STEWARD is not a process guess; it is an INSTALLATION IDENTITY: the
human-run install mints a protected record (operator-owned, 0600,
pinned path and content) that the classifier authenticates by
ownership and content. Because same-user code could replay a static
credential, the capability is a ONE-SHOT BOUND AUTHORIZATION
(IW-R3-05, IW-R4-03): at mint time the steward STAGES the effective
launch inputs into the intent — the role file, requirements, and
return schema bytes; the brief bytes; the EXPANDED permissions
envelope; the roster-resolved runtime AND model — and dispatch's
steward mode launches from those staged bytes, so what was
authorized is what runs, immune to config drift between mint and
launch. The tuple: repository identity, install generation, the
claim being continued, digests of every staged input, workspace,
job identity, and a nonce. The steward dispatch path validates the
caller's request against that tuple EXACTLY (no caller-selected
role, brief, model, workspace, or permissions exist in this mode)
and CONSUMES the intent atomically — a replay finds it consumed and
refuses; a mismatched tuple refuses by field. Every other
steward-attempted verb refuses by name. The
unknown-process fall-through to HUMAN closes in the same change:
HUMAN keeps only its positive classification paths; unrecognized
non-interactive processes classify UNTRUSTED and refuse. ONE SHARED
ARBITRATION LOCK: worker enrollment and steward job reservation
take the same named lock, closing the check-to-reservation race —
an enrollment injected before reservation means no job record.

## The schedule glue (unchanged from round 2)

INSTALLED BYTES, for ROBUSTNESS, with the trust posture stated
(IW-R3-04, refuting IW-R4-04/05 on the record): the metasystem's
threat model is D118's, repo-wide — accident-proofing and a
cooperative fleet; same-user MALICIOUS code is out of scope
everywhere, because a process running as the user already owns the
account's crontab, LaunchAgents, shell profile, and every binary in
the home directory, steward or no steward. The steward therefore
does NOT claim tamper-proof bytes (no same-user mechanism short of
a second OS principal could deliver that, and none is built); it
claims TICK ROBUSTNESS: the install copies the steward binary and
tick script outside the checkout so a broken checkout — mid-rebase,
half-deleted, wedged — can still tick, go DEGRADED, and notify.
Decisions and checks run from those installed bytes; the dispatch
of a continuation runs the checkout's own machinery, exactly as
every session on this machine does, because the checkout is the
product being operated. Upgrades are a human-run reinstall.

scripts/steward.sh install|uninstall|status|tick, bash 3.2, all
decisions in Go verbs. darwin: LaunchAgent, StartCalendarInterval
minute lists (the documented catch-up variant), RunAtLoad. linux:
marker-fenced crontab entry preserving unrelated lines
byte-for-byte; no missed-tick replay exists and none is claimed;
install REFUSES without a configured notifier on linux (darwin
defaults to the platform notifier). Relocation: the entry pins
path AND identity; mismatch → DEGRADED + notify, never operate on
whatever now lives there. Idempotent reinstall; quoted paths; two
repositories isolated.

## Honest limits

The steward guards installed machines while awake, for installed
repositories. It cannot stop a stall from starting. For a live-idle
worker its response is the operator's attention, by design, until
steward-succession lands. The session-level cron guard from D121
remains a belt inside live sessions.

## Obligation matrix

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
|---|---|---|---|---|---|---|---|---|---|
| IW-1 | CRITICAL | IW-R1-01, IW-R2-01 | Verdict ladder: DEAD only from a complete census with every relevant identity provably absent; UNKNOWN dominates; live gates/runners/monitored runs count; dead-after-fresh-commit revives; live-quiet untouched inside threshold; live-idle notify-only; unknown notify-only | steward core | internal/steward/check.go | fixtures: live-unannounced-main-not-dead; empty-enrollment-not-dead; live-plus-dead-aggregates-live; unknown-dominates-dead; live-gate-prevents-revival; clock-step-live-delegate-stays-live; dead-after-fresh-commit | steward on this repo | MISSING | implement |
| IW-2 | CRITICAL | IW-R1-02, IW-R2-06 | High-water evidence: HEAD oid + claim-History opids only; receipts excluded; steward/continuation outputs never reset the dry count; identical marks age; one active continuation suppresses dispatch; at most one durable attempt per crash window | steward core | internal/steward/evidence.go | fixtures: continuation-receipt-no-reset; blocked-parked-no-reset; identical-evidence-stays-old; one-active-continuation; single-attempt-per-crash-window | steward on this repo | MISSING | implement |
| IW-3 | CRITICAL | IW-R1-03 | Machine-scoped open work: foreign claims never revived; one claimed arc = one revival; legacy Current recognized | steward core | internal/steward/openwork.go | fixtures: foreign-claim-ignored; arc-one-revival; legacy-current | both formats | MISSING | implement |
| IW-4 | CRITICAL | IW-R1-04 | Journal-aware, degraded-honest: owned non-terminal journal entry = open work; unreadable/malformed = degraded-notify; only valid Goal-free = no-work | steward core | internal/steward/openwork.go | fixtures: dead-owner-journal-entry; malformed-ledger-degraded; goal-free-no-work | both formats | MISSING | implement |
| IW-5 | CRITICAL | IW-R1-05, IW-R2-04 | Arbitration inside the ONE shared lock (enrollment + reservation): live-between-check-and-reserve means no job record | steward + dispatch | internal/steward/revive.go | fixtures: live-between-check-and-reserve-aborts; enrollment-injected-before-reservation-no-record; the three enrollment interleavings (before section = cancel; during = blocks then sees the job; after = lease-arbitrated coexistence); lease-generation invalidation on takeover | fake adapter | MISSING | implement |
| IW-6 | CRITICAL | IW-R1-06, IW-R2-04, IW-R2-05 | Authenticated installation identity granting exactly unattended-continuation dispatch; forged/stale identities refuse; HUMAN positive-only, unknown non-interactive → UNTRUSTED; no-wait dispatch with formal return schema; steward tick reaps/mirrors/closes; fake adapter registers the role | dispatch + authority | internal/authority + scripts/agents/dispatch.sh | fixtures: genuine/forged/stale identity; cron-untrusted; interactive-HUMAN-preserved; steward-refused-other-verbs; pre-delivery and fence-invalid intent consumption refused; staged-input drift caught by digest; tuple mismatch refused by field; consumed-intent replay refused; schema-materialization; success+protocol-error returns; mirror-retry; auto-closure; no-open-chain | fake adapter | MISSING | implement |
| IW-7 | HIGH | IW-R1-07, IW-R2-03 | Visible before action: intent+receipt then DELIVERED notification gates launch; notifier failure = no launch, durable pending, retry without redispatch; crash-after-dispatch reconciles to notified unknown; linux install refuses without notifier | steward core + glue | internal/steward/intervene.go | fixtures: notifier-failure-no-launch; delayed-delivery-single-launch-after-rearbitration; intent-before-launch; crash-reconciled; linux-install-refusal | darwin + linux | MISSING | implement |
| IW-8 | HIGH | IW-R1-08 | Scheduler lifecycle: StartCalendarInterval; marker-fenced crontab preserving unrelated lines; relocation → degraded; idempotent reinstall; spaces; two repos isolated | glue | scripts/steward.sh | install fixtures: two-repos, reinstall, spaces, foreign-crontab-preserved, moved-repo-degraded | real macOS + Debian VM | MISSING | implement |
| IW-9 | HIGH | IW-R1-09 | Matrix gate-conformant; every finding family owns a row and named fixtures | design | this file | the gate parses every row without format errors now, and exits 0 when the rows above turn DONE | gate run | MISSING | verify at landing |
