# Delegated decisions — 2026-08-13 (AFK window)

Wido delegated, in writing before leaving: all review sign-off items and
in-flight judgment calls, decided by the agent and documented here for
after-the-fact review. Revert on disagreement. Entries are appended as
decisions are made; each names the alternative that was NOT taken and why.

## D1 — lease-census-7: identity fixture fenced at arming, not root-threaded

**Decision:** `supervise blocking-reserved-cap` — the verb every arming gate
(re-arm, establishment, takeover) already consults — refuses to arm when
`METASYSTEM_FAKE_PROCESS_IDENTITY_FILE` is set in a checkout whose
`metasystem.runtimes` is not `fake`. The identity package doc names the
fence.

**Alternative not taken:** threading a checkout root through
`identity.Custodian`, `census.identityAlive`, and `census.AuthIdentity` so
each read is conf-gated like `enumerateFixture`. Rejected because the
signature ripples through every custodian call site and the missionrunner's
`custodianFn` seam, while the finding's named attack vector is exactly one:
the env var leaking into an ARMED component's inherited environment. The
arming gate kills that vector at its choke point; one-shot CLI reads with
the var set remain sanctioned test usage (Go test seams depend on them).
Follow-up if Wido prefers depth: do the threading as part of W2's
lease-census-8 typed-structs work, where those signatures move anyway.

## D2 — Benchmark: cohort bm-1-20260813t113617z-83558 abandoned after rep 1

**Decision:** stop the cohort (rep 2 sealed but never resumed — no further
spend), fix the three close-protocol defects rep 1 surfaced (KI-6 round 3,
commit ef9d142) plus the kit's capped-turn gap (4b4f7d1), and start a fresh
two-rep cohort at the fixed engine.

**Why not run rep 2:** a cohort is one candidate SHA; rep 1 was already
invalid on everyChainClosed (frozen consequence of the now-fixed engine
bug), so the cohort could never yield two valid reps. Rep 2's provisioned
engine predates the fixes and would have reproduced the failure for ~$10.
Series spend so far ≈ $30 of the EUR 240 ceiling.

## D3 — fencesEnforced carries a 30-second bookkeeping allowance (kit)

**Decision:** the extractor tolerates turn wall-clock overshoot up to 30s
past the contract cap, because the runner stamps `endedAt` after return
validation, ledger delivery, and the state write (rep 1 of the first rerun:
1203s against 1200s, outcome `completed`).

**Alternative not taken (yet):** engine-side measurement of the host
interval at the process boundary, which would make the reading exact and
the allowance unnecessary. Cleaner, but touches the turn record's field
semantics — queued as a possible follow-up rather than blocking the series.

## D4 — Kit role schemas pinned as the engine-materialized v2 envelope

**Decision:** the five delegate-role schemas in `benchmark/schemas/evidence/`
are the byte output of `metasystem schema materialize --version 2` (stored
returns carry the `schemaVersion`/`claimed` envelope); mission-state and
orchestrator schemas are byte copies of the engine's shipped files.
`validate-kit.sh` enforces both rules (drift guard), plus the rule that
every engine-owned evidence filename the extractor requires still appears
in the engine's sources.

**Alternative not taken:** the extractor materializing schemas from the
TARGET's binary at scoring time. Rejected for referee independence: a
candidate must not define its own scoring contract at run time. The pin
updates deliberately, via the guard, at kit-update time.

## D5 — mirror stamps are per-job (on-disk record semantics change)

**Decision:** `mirror_record` stamps only the job actually mirrored;
sibling chain records no longer receive a copy of another job's mirror
result. This changes what a record's `mirror` field MEANS: it is now a
claim about that record's own evidence, which is what every consumer
(CloseCheck, evidence GC after codex-1/foundations-1) actually needs.
The happy-path suite fixture was updated from asserting stamp equality
across the chain to asserting the shared manifest COVERS both records —
a strictly stronger durability property.

**Alternative not taken:** keeping chain-wide stamping and teaching close
to re-mirror instead. Rejected: the chain-wide stamp was a false
durability claim (rep 1: a record stamped mirrored while its artifacts
were absent from the manifest), and stamps that can lie defeat the point
of stamping.

## D6 — CloseCheck tolerates undelivered implementer rounds

**Decision:** an implementer round with no `diff.patch` on disk blocks the
close only when its record says `completed`; timeout/failed/cancelled
rounds without a diff pass (nothing was delivered, nothing to attest). An
EXISTING diff must still be mirrored at current content, whatever the
status.

**Alternative not taken:** waiving the diff requirement for the whole
chain when any round is non-completed — too weak, would skip attestation
of diffs that do exist.

## D7 — Rep 2 no-go; four engine fixes land first (fence-refusal diagnosis)

**Decision:** rep 2 of cohort bm-1-20260813t132947z stays unspent until four
fixes land, because rep 1 was not a fair measurement: half its signed job
budget was consumed by reservations of dispatches that never started a
process, and the headroom line lied to the host every turn.

The diagnosis (agent-verified, evidence in the cohort target's record-locks
mktemp residue): of the four "dispatch-refused" husks, only ONE was the
mission fence — and that refusal was CORRECT (two genuinely live delegates
at concurrency 2, one an orphan of the crashed round-1 host). Two died on
writable-without-worktree (host error, correct refusal), one on a
capability-snapshot miss after the codex CLI rewrote its own config
mid-run (KI-19's class). The capMin 7 and 12 the implementers timed out
under were the HOST's own `--cap-min` requests against a signed ceiling of
15 — the engine's arithmetic is exonerated. The host then wrote a false
ledger fact ("fence refused ... even though the contract states
fence.concurrency=2") because the prompt never showed concurrency headroom
or the live roster.

**Fixes landed:** (1) `mission fence-release-job` + `fail_setup_husk`
releases a husk's reservation — fence.jobs headroom becomes honest and the
reserve-before-setup concurrency race closes. (2) The turn prompt's fence
headroom now carries `concurrency=free/limit` plus the live-delegate
roster. (3) Every husked dispatch emits a `job-refused` event with a
reason class (fence/envelope/capability/worktree/setup) and stamps the
class into the record — rep 1's diagnosis needed mktemp file sizes as its
primary instrument. (4) A capability-snapshot miss self-heals with ONE
adapter probe and re-select; an unhealable miss still refuses, now naming
the failed probe. Plus the policy line in roles/orchestrator.md: dispatch
at the signed fence.job-cap-min unless there is a stated reason, and a
brief's budget must agree with --cap-min — the run's actual proximate
cause, which no engine fix touches.

**Alternative not taken:** rerunning as-is and averaging over host
variance. Rejected: the dominant failure mode (host-chosen 7-minute cap
for a 9-minute brief) is unconstrained by anything in the engine or
prompt, so it varies randomly rather than reproducing, and each sample
costs ~$10 while measuring a known-dishonest headroom display.

**Also noted for later:** an orphaned job's cap enforcement degrades to
standing-reaper granularity when its waiting dispatcher dies (f29a overran
its capDeadline by 75s); not fixed in this pass.

## D8 — Close attests evidence, not host workflow (amends D6); usage owns extraction

**Decision (CloseCheck):** rep 1 of cohort bm-1-20260813t155700z — the first
run with all D7 fixes — failed everyChainClosed on a NEW shape: the host
dispatched at the signed cap (the D7 prompt/policy worked), the implementer
COMPLETED, but the host parked on stop-loss without ever running the
conformance review — and diff.patch is CREATED by that review, never by the
reap. D6's completed-without-diff refusal therefore wedged the close of
every unreviewed chain at mission end. Amendment: an absent diff never
blocks the close (the unreviewed-implementer workflow gap is already
delegationFloorMet's verdict); a diff the MANIFEST knows but the disk lost
is still evidence loss and refuses ("vanished after mirroring"); an
existing diff must still be mirrored at current content.

**Decision (architecture-1):** internal/usage is the single owner of typed
usage extraction — DevinUsage (the host and adapter copies deleted),
CodexUsageValue, and RootJobID (usage attribution is per chain; W2's
walker consolidation may re-home it). mission/fence.go is off its adapter
import. The two former writers (host canonicalJSON, adapter encodeJSON)
were byte-identical wiredoc bodies, so the consolidation changes no
on-disk bytes; usage's writer carries the same body. host re-floors at
79.3 (well-covered code moved out, shifting the remaining ratio — the
deterministic-minimum rule, lock=84.0 precedent).

**Cohort consequence:** bm-1-20260813t155700z abandoned after rep 1 (one
SHA per cohort; rep 1 invalid on the now-amended close rule plus an honest
delegationFloorMet). Rep 2 unspent. Fresh cohort at the next validated pin.

## D9 — One directory-lock protocol; bindings keep their bytes (W2.2)

**Decision:** internal/lock is the single home of the rename-born
directory-lock protocol. It gained exactly two things: an OwnerCodec
option, so each binding's on-disk owner.json schema stays byte-compatible
(dispatch: {pid,instanceTag,acquiredAt}; census:
{function,pid,pidStartedAt,instanceTag,observedAtEpoch}); and a Cause on
HolderError so bindings can name a malformed owner file. NO staleness
extension: dispatch's tag-based stale-holder rule is its PROBE answering
Dead for a live pid whose successfully READ argv lacks the recorded tag —
the custodian stranger rule, which death-only takeover already expresses.
codex-2's row (Alive with unreadable argv keeps the lock) and
dispatch-supervise-3's row (Unknown refuses takeover) live in the
bindings' probes and are still pinned by their verdict tests, now driven
through the one implementation. ownerlock.go and censuslock.go are thin
bindings (~120 lines each of schema, probe, and refusal wording).

**Semantic change accepted:** dispatch's owner lock previously returned
Busy for an OWNERLESS husk; via lock.Acquire it now heals one — the
canonical package's documented garbage-by-construction rule, which the
review names as exactly what the copies lacked.

**Alternative not taken:** a fourth Liveness state for staleness.
Rejected: it would weaken the death-only takeover invariant every
consumer reasons about, when Dead already means "a successful read proved
the RECORDED identity absent".

## D10 — Rep 2 proceeds; the delegation floor's strictness is flagged for Wido

**Decision:** rep 1 of cohort bm-1-20260813t171239z (engine 2ef72cf, all
D7+D8 fixes) passed six of seven validity gates — everyChainClosed passes
with an unreviewed completed implementer, closing the whole close-protocol
arc. The one failure is delegationFloorMet, and its cause is now purely
the measured system: TWO implementers completed at the signed cap 15, the
host certified only design-critic rounds, and stop-loss parked the mission
before any implementer certification. Rep 2 runs as-is (~$10, within the
pre-approved envelope): the measurement is clean, so a second sample is
fair — the first cohort's rep 2 DID meet the floor once, so this is
mission-dynamics variance, not determinism.

**Flagged for Wido (kit measurement design, not decided under
delegation):** the floor requires a completed AND CERTIFIED implementer
per stream. Real delegation demonstrably happened (two healthy completed
implementer jobs); what failed is the host's adjudication step before a
stop-loss park. Whether that belongs in runValidity (as now) or in
productMetrics is a benchmark-design judgment that shapes the whole
series — if the strict floor stands and baseline reps keep failing it,
the Devin arms compare against no valid baseline. Not amended: re-scoping
a validity gate mid-series is referee tampering unless it is clearly a
kit defect, and this one is arguably intent.

## D11 — The drain-stall incident: my cadence change caused it; four fixes land

**What happened (rep 2 of bm-1-20260813t171239z, agent-diagnosed):** the
mission parked drain-stalled exactly 2.0s past a delegate's capDeadline,
because the drain's park window is capDeadline + 2s handshake grace while
my own 5f5db7f moved the kill-capable dispatch reap to a 5s cadence — a
~40% chance per expiry of parking without ever serving a reap inside the
window. The runner then exited 0 over the parked state and the cohort
graded FIVE SECONDS later while the delegate was still writing code (it
self-finished 60s past its cap; the score measured a bare checkout). My
earlier premise ("20+ minutes uncapped") was a timezone misread, and D7's
note that an orphan's cap "degrades to reaper granularity" was WRONG: the
Go standing reaper has no kill authority by design, so an orphaned job's
cap degrades to UNBOUNDED once its waiter and runner are gone.

**Landed:** F1 — the drain never parks while a kill-capable reap is owed:
at the deadline it runs the dispatch reap for every live record, re-reads,
and only parks what a fresh reap could not resolve. F2 — the drain
witnesses every reap's exit and stderr (this incident was undiagnosable by
artifact; the runner log was empty) and emits a job-refused event on
failure. F6 — the standing-mode shell reap sweep reports failures without
exiting (my accumulate change would have terminated a shell standing
reaper before its heartbeat — strictly worse than the starvation it
replaced; unbitten only because the standing reaper is the Go component).
F3 (kit) — grading a drain-stalled park waits out still-running survivors
(their caps bound them, ten-minute ceiling) and refuses rather than
scoring a bare checkout.

**Deferred, flagged for the review backlog:** F4 — the orphan window
itself (a job whose waiter and runner both exit has NO kill-capable
enforcer until mission end); needs a design decision between waiter
adoption and a kill-capable supervision sweep, which touches the
no-kill-authority rule and deserves its own pass. F5 — the standing
reaper logs nothing when it declines to act; emit the
running∧CapExpired∧custodian-alive state once per pass.

## D12 — The durability contract gets FINISHED, not retired (W2.6 decision)

**Decision:** finish the B5 two-outcome migration for the writers of
durable state, in the scope the finding names: dispatch's custody/record
writes, the lease's state writes, and the registry's snapshot writes
thread the repository root as anchor and ACT on durable=false by
witnessing doubt (an event where an emitter exists, stderr otherwise).
Remaining call sites keep anchor "" until their packages are otherwise
touched — an explicitly staged migration, not an unclaimed guarantee:
the package doc will say which callers have adopted the contract.

**Why not retire:** the contract is go-production-grade B5's deliberate
product; dispatch/record.go marks the adoption as pending work, not as a
rejected idea. Retiring would delete crash-safety machinery the
durability program built and tested, to save threading a parameter.

**Execution note:** L-effort; runs as its own pass with a full suite +
VM cycle, after the in-flight clean-measurement baseline cohort.

## D13 — The host's wall clock ends at the host's own boundary

**Decision:** the D3 allowance was structurally wrong and bit twice: a
turn's endedAt lands after adjudication, drain, ledger delivery, and the
state write, so the cap gate failed turns whose hosts finished INSIDE
their cap — first 3s of bookkeeping, then 40s of legitimate drain wait in
the first fully-completed mission of the series. The engine now stamps
hostEndedAt on the turn record at the host PROCESS boundary (both the
completed and capped paths), the kit's cap gate reads it when present
with a 5s allowance, and the 30s bookkeeping allowance survives only for
legacy turns without the stamp. Kit turn schema admits the field.

**Cohort consequence:** bm-1-20260813t191528z abandoned after rep 1 (one
SHA per cohort; rep 1 invalid on exactly this gate — while being the
series' FIRST COMPLETED mission: no park, three clean turns, cycles 3/8).
Fresh cohort at the next validated pin. Series spend ~$70 of EUR 240.

## D13 addendum — the enforcement stamp, one layer down

Rep 1 at the D13 pin: every host interval measured clean via hostEndedAt,
but the job-cap gate then failed on a job the engine CORRECTLY killed at
its 900s cap and stamped 901s after wind-down. The kit's job-cap check
now carries the same 5-second enforcement allowance as the host check;
a real unenforced overrun runs minutes over, not seconds. Free replay of
rep 1 post-fix: only delegationFloorMet remains — the fourth consecutive
cohort where, with every measurement defect closed, the delegation floor
is the single blocker. That is now cleanly the D10 question for Wido:
the host under this contract (no-gain-budget 3) repeatedly parks on
stop-loss before certifying an implementer, and whether that reads as
run-INVALID or as the product verdict shapes the whole series.

## D14 — pidStartedAt is the fixture table's one spelling (W2.11 sign-off)

**Decision:** identity.FixtureEntryFor is the ONE reader of the
fake-identity fixture table (five packages parsed it privately, with two
start-time spellings, and the shell fixtures wrote both keys into every
entry to satisfy them all). pidStartedAt is canonical — the spelling every
other record in the tree uses; the legacy "started" key is retired from
both shell fixture writers, and the shared reader still accepts it during
the transition so any straggler fixture keeps working. The dual-read drops
once a full suite cycle proves no writer emits it.

## D15 — Usage errors exit 2 everywhere; ambiguity gets its own code (cli-6 sign-off)

**Decision:** the package convention (2 = usage/validation error, 1 =
operation failure) applies uniformly: the mission-fence verbs' invalid
mission id / job / cap and mission-state-verify's flag-pairing error move
from 1 to 2, matching what the runner verbs already do for the identical
condition. adapter devin-session stops overloading 2 for ambiguous
correlation and gets its own documented code, chosen at implementation
from the verb's unused range (the precedent is owner-lock's 3/4 and
chain-usage's 7). Implementation rule: before the change lands, every
shell caller of the affected verbs is surveyed for exit-code branches, and
any branch on the old meaning updates in the same commit; the full suite
on both machines is the gate.

**Alternative not taken:** grandfathering the fence verbs' 1 as "their
local convention". Rejected because shell plumbing branches on these codes
across verb families — a caller cannot know which dialect it is talking
to, which is the defect being signed off.

## D16 — Boolean flags: keep the wire, refuse the typo (cli-7 sign-off)

**Decision:** existing string-boolean flags KEEP their exact wire
spellings ("true"/"false" for the --overridden/--signal/--network family,
"0"/"1" for --worktree/--devin-checks) but gain strict validation via
flags.Func: any other value is a usage error (exit 2), never a silent
false. New verbs use flags.Bool. This kills the real trap (a mistyped
--signal value quietly disabling the session-handshake deadline) while
touching zero shell call sites.

**Alternative not taken:** migrating every string boolean to flags.Bool
for one uniform dialect. Rejected because it rewrites dozens of live call
sites across dispatch/adapters/hooks for no safety gain over strict
validation — the wire compatibility risk dwarfs the idiom win.

## D17 — Adopted-engine delivery: the payload ships the source, CI rebuilds (script-misc-1 / r10 sign-off)

**Authority note, stated plainly:** kill-shell.md r10 severed this as "a
HUMAN decision under the reserved-decisions rule" — that severance predates
the 2026-08-13 AFK delegation, which handed me the review's sign-off items
including this one ("decide everything, document, revert on disagreement").
The review's own critique round moved this decision AHEAD of all of W4
because the shipped CI enforcement is red-on-day-one in every adopted repo.
I am deciding it under the later, more specific delegation; if Wido wants
this one back, this entry is the revert point and nothing below is hard to
unwind (adoption is re-runnable).

**Decision:** the adopted payload ships the engine SOURCE — cmd/,
internal/, go.mod, go.sum join the adoption allowlist — and the shipped CI
workflow gains a Go toolchain step, after which the suite's EXISTING gate
path does the rest: with source present, validate-metasystem.sh's
metasystem_go_source detection turns the go-gate build back on, and
ALWAYS-REBUILD (r31) becomes the adopted repo's doctrine exactly as it is
the template's. The adoption-time binary copy into gitignored bin/ stays as
the host convenience it already is — it was never the delivery, and now
nothing pretends it is.

**Why this resolves all three r10 criticals:** source is tracked
(KS-R10-001), platform-independent (KS-R10-002), and needs no embedded
committing-HEAD because the compiler rebuilds from whatever HEAD is checked
out (KS-R10-003 — the stamp stays informational). COHERENCE BY PAIRING
strengthens: the scripts and the engine source travel through the same
adoption run, and the pair can now PROVE its coherence by building.

**Alternatives not taken:** per-platform release artifacts (a release
pipeline, download auth, and a network dependency for a tool whose whole
doctrine is self-contained checkouts); committing the binary (single
platform, repo bloat, r10 already proved it wrong); building from the
template repo at the recorded SHA (couples every adopted CI run to the
template's availability and auth).

**Implementation contract (W4.22, own pass):** adopt.sh allowlist grows the
four source roots; github-actions-metasystem.yml gains setup-go before the
suite; the adopt fixtures assert the filled target's suite now RUNS the
go-gate (the red-on-day-one defect becomes the tested path); template SHA
recording unchanged. Until that pass lands, nothing changed on disk — this
entry is the ruling, not the landing.

## D18 — Cap-authority joins the owner-lock family (script-orchestration-01 sign-off)

**Decision:** both cap-authority spinlocks (dispatch.sh and
arm-supervision.sh, verbatim duplicates) route through `job owner-lock` on
the same on-disk directory — claim with pid+tag identity, spin to the same
scaled deadline, release refusing another owner's lock. dispatch's tag is
its existing per-invocation __lock-owner re-exec tag; arm-supervision uses
its own script name (present in its argv, which is what the custodian rule
probes), accepting that a pid recycled by ANOTHER armer reads alive and
waits out the deadline instead of healing — fail-closed in a rare
collision, never a wrong takeover. A SIGKILLed holder's husk now heals
instead of bricking all dispatch and arming until a human runs rmdir.

**Found during landing:** the AUTH-R2-006 fixtures simulated "lock held"
with a bare mkdir — under the owner-lock protocol an ownerless directory
is garbage by construction and heals immediately, so the fixtures now hold
with a real live identity (their own pid and script name) and release
through the verb. The bare-dir trick dying is the point of the change: an
ownerless lock no longer blocks anyone.

**Alternative not taken:** a dedicated `job cap-authority-lock` wrapper
verb. The generic verb already carries the protocol; a second spelling of
the same claim would be one more surface to keep honest.

## D19 — The shell standing reaper dies (script-orchestration-08 sign-off)

**Decision:** dispatch.sh's standing-reaper mode is deleted — the
--interval/--heartbeat/--instance-tag/--start-gate flags, the custody start
gate, the supervision-only authentication, the tick loop, and every
standing-gated verdict branch (stale-claim-epoch, abandoned-setup, the
terminal-status skip, the busy-lock comeback). Nothing in production ever
launched it: the supervise owner launches the GO reaper component, and the
shadow verdicts' live owners are internal/lease/sweep.go,
internal/supervise/reaper.go, and the mission drain. What remains is the
lease-held single-shot reap that wait_for_job and the drain actually call.
The F6 report-without-exit carve-out (a standing sweep must not die before
its heartbeat) dies with the mode; the single-shot sweep keeps the
no-starvation visit-all-then-report contract with exit 1.

**Fixture transform:** the WC-9 authority leg proved the standing loop
authenticated supervision before entering it; with no loop, the leg now
proves the loop STAYS deleted (refusing --interval / while-true /
standing_reaper in reap_jobs) and that the lease-held re-entry survives —
the guard the deletion actually needs, since a re-activated shell daemon
would carry kill authority the standing-reaper ruling explicitly denies.

**Alternative not taken:** keeping the mode behind a refusal ("standing
mode retired; use supervise component reaper"). Rejected: 80 lines of
kill-capable code kept compiling toward divergence, guarded only by prose.

## D20 — `supervise derive-ceiling` is a verb (script-orchestration-04 sign-off)

**Decision:** the watcher-ceiling derivation — max over the 120 floor, the
declared --max-cap, dispatch.cap-min, fence.job-cap-min, every cap.min.*
key, and every raw METASYSTEM_CAP_MIN_* environment value, plus the
30-minute allowance — moves into supervise.DeriveCeiling behind the new
verb, beside the blocking-reserved-cap fence that consumes its output.
arm-supervision.sh forwards. Refusal texts keep the raw key and env-name
spellings; --max-cap misuse stays a usage exit 2. One deliberate
strictening: an ambiguous conf (duplicate key) now refuses through the
resolver instead of whatever the shell's per-key get happened to do —
consistent with every other resolution verb.

**Alternative not taken:** folding the derivation into
blocking-reserved-cap as a combined arm-check verb. Rejected: the ceiling
is also attested into state.json and read back by re-arm refusals and
dispatch independently of the reservation scan — separate questions,
separate verbs.

## D21 — `supervise verify-armed` is a verb (script-orchestration-10 sign-off)

**Decision:** the arming success criterion — live owner, live watcher and
reaper with fresh heartbeats, loadedCapMin equal to the attested derived
ceiling, and a fresh successful census matching the state's fingerprint
and generation — is supervise.ArmedNow behind the new verify-armed verb:
one attempt, pure over the clock. arm-supervision.sh keeps the scaled
retry loop and its timeout message. The component-liveness rule is spelled
as the ladder's: census-alive at the recorded start, and a recorded tag
must not be PROVABLY absent (live and unknown pass; stale and dead fail) —
the same identity.TagState the dispatch ladder consumes since W4.5, so
arming and dispatch can no longer diverge on what alive means.

**Alternative not taken:** giving the verb the retry loop (--deadline-sec).
Rejected: the wait cap is fixture-scaled shell policy
(supervision_wait_cap), and a verb that sleeps is a verb a caller cannot
compose.

## D22 — `report running-work` is a verb (script-orchestration-05 sign-off)

**Decision:** the turn-end "is anything still working?" inventory becomes
report.RunningWorkClause behind the new verb, beside open-work: job
records decoded properly (killing the raw-grep class where a nested
"running" inside an error field counted as live work), mission runners
found by argv tokens instead of pgrep-plus-sed, gate runs matched the same
way, and the historical clause wording built in one home. The hook keeps
only the three-way sentence choice, which depends on its own open-work
state. The clause (not raw JSON) is the verb's output: the hook is the
single consumer and the wording is the contract a human reads every turn.

**Alternative not taken:** a JSON inventory the hook re-words. Rejected:
two homes for human-facing wording is how the sentence and the data drift;
if a second consumer ever needs the raw inventory, the clause function
already sits on top of typed pieces to expose.

## D23 — The watcher's classification engine joins the REPORT family (script-orchestration-06 = script-misc-3 sign-off)

**Decision:** the DONE/CAPPED/NEVER-STARTED/STALE/VANISHED engine —
sidecar-vs-primary selection, sibling-mtime liveness, verdict precedence,
seen-state marking, baseline adoption — is report.ScanJobs behind `report
scan-jobs`. FAMILY: report, not the review Target's supervise — the
recorded r3/KS-R3-009 ruling already assigned job-file classification to
the report family, and the verb's whole output is greppable report lines;
the recorded ruling wins over the finding's suggestion. The script keeps
argument parsing, the ARMED banner, the census invocation, and the sleep
loop. Report-line bytes and the seen-state format are unchanged wire,
pinned by tests. The running set (formerly in-process arrays) rides in a
script-owned mktemp scratch file, so a watcher restart still resets
VANISHED tracking exactly as before.

**Defect fixed in the port (the verifier's find):** the shell's
concatenated digit check accepted an EMPTY --stale-min, after which
`[ age -ge "" ]` failed silently and STALE never fired. The engine refuses
non-positive thresholds loudly — a misconfigured watcher now dies at
arming instead of watching nothing.

## D24 — `adapter adjudicate-turn` is pure decision; the CAS stays shell-side (script-adapters-01)

**Decision:** the adapter turn's terminal-outcome state machine —
cli-status/handshake mapping, candidate validation (normalization plus the
return-complete judgment, both already Go), the bounded-repair decision
with the byte-identical repair prompt, the settle verdicts, and devin's
empty-reply rule — is adapter.AdjudicateTurn behind four stages of one
verb (initial, after-repair, settle-result, empty-reply), each printing
the tuple the shell executes. THE CAS DELIBERATELY STAYS in the shell
wrappers: adapter record writes ride dispatch.sh's lease-held
__record-cas re-exec, and a verb writing records directly would create a
second authority path around the lease discipline. The shell keeps
process launches (the repair CLI turn) and the per-runtime usage/settle
hooks; every error code and phase name now has one home beside the
dispatch and missionrunner code that adjudicates on it.

**Known log-line shift:** "return repaired ... kept as evidence" now
prints only when the repair actually completes (after settle), not before
the settle attempt — truer, and nothing greps it.

**Coverage note:** adapter re-floors 86.5 → 86.3 on both platforms: the
genuinely-valid return paths need the full role-schema fixture and are
proven by the suite's fake-adapter turns end to end; every refusal and
decision branch is unit-pinned.

**Addendum (W4.17/script-adapters-04):** absorbed by this decision — the
repair prompt is written byte-pinned by the verb, the one-attempt bound
and eligibility feed the --repair-available flag the verb adjudicates,
and the shell keeps only the runtime_repair_turn process launch, which is
the finding's own Target. The eligibility INPUTS (does a repair hook
exist, has one run, is a session present) are shell facts by necessity:
`declare -F` cannot be asked from Go. No further change.

## D25 — Command construction has one home per runtime (script-adapters-02/-06 sign-offs)

**Decision:** `adapter claude-command` joins codex-command as the one
builder of the Claude argv — permission-mode/tool mapping from the
envelope, native budget policy from the environment (distinct refusals: 3
budget, 4 turn limit), NUL-separated tokens both adapters/claude.sh and
hosts/claude.sh read back. The host's previously forked copy (hardcoded
acceptEdits) is now the RECORDLESS MODE of the one builder — same policy,
one spelling. For script-adapters-06, codex-command grows
--permissions/--record to derive sandbox/network in Go (writeRoots empty
means read-only, network allow means true), and devin-config emits the
permission mode beside the config it assembles — the envelope-to-flag
mapping is the security-relevant half of command construction (KI-12 was
exactly this going wrong) and no longer arrives pre-chewed from shell.

**Also under this entry (Wido, mid-session):** the canary doctrine's new
batching rule is written GENERICALLY in canary.sh — full suite per batch
of 3-5 low-risk commits or immediately after a high-risk one — with no
mention of hosts, VMs, or worktrees: those are this checkout's
development mechanics, and the canary is metasystem behavior that ships.

## D26 — Host finish and Devin settle join the engine (script-adapters-10/-07 sign-offs)

**Decision (host finish):** the turn-outcome adjudication triplicated
across hosts/claude.sh, hosts/codex.sh, and hosts/devin.sh — failed vs
unresumable vs completed, and the 3/6 exit taxonomy the mission runner
interprets — is host.FinishTurn behind `host finish`; the hosts propagate
its exit. Devin's exit-0-with-no-reply shape is the --require-reply flag;
its per-session cumulative-store copy stays shell, gated on the verb's
completed verdict. The three atomic_result wrappers die with their last
callers.

**Decision (devin settle):** the transcript-vs-correlated-session
certification and the effective-model canonicalization with the
`unobserved` fallback are adapter.DevinSettle behind `adapter
devin-settle` — the same package that owns the correlation half. The
disagreement artifacts are byte-identical; the observed model records
even when certification fails (the record must reflect what the
transcript named); record writes stay with the shell caller per D24.

## D27 — The full-contract self-test is one Go orchestration (script-adapters-05 sign-off)

**Decision**: `adapter selftest-run` (internal/adapter/selftestrun.go) now
runs the entire full-contract sequence the three real adapters drove
through ~260 lines of runtime-common.sh: dispatch, return completeness,
typed usage, session-identity resume, cancellation, the permission legs
against the snapshot's own envelope declaration, and the pass record.
The orchestration EXECS dispatch.sh, the adapter script, and
assert-return-complete.sh — composition rides the same authority paths
every real job rides, nothing is reimplemented. The decisions moved into
Go and are unit-proven: the model-placeholder refusal, the denial
taxonomy (empty_reply / protocol_error / runtime_error and nothing
else), session equality, and the evidence assertions as PARSED reads of
return.json — a marker in key position or in an unparseable file no
longer satisfies an evidence claim, which the old `grep -Fq` would have
accepted. Per-runtime knobs stay declarations in the adapters
(selftest_turn_ceiling_sec, selftest_denial_ends_turn) and travel as
flags; the tripwire listener now runs in-process (the selftest-listener
verb remains for now and is a removal candidate once W6 sweeps unused
verbs). run_full_contract_selftest is a six-line wrapper.

**Why**: the sequence was the last adapter-side block where assertions
lived in grep and taxonomy in a case statement. The suite deliberately
never runs it (real selftests spend model calls), so the port ALSO adds
the first automated proof of this path: stub-dispatch fixtures drive the
full orchestration end to end, plus refusal tests for every verdict.

**Numbering note**: an earlier projection earmarked D27 for the W4.21
refactor gate; that work takes D28's slot when it lands. D-numbers are
assigned in landing order.

## D28 — The W4.20 smalls: two dedups done, one done minimally, one already resolved (script-adapters-08/-09/-12/-15 sign-offs)

**script-adapters-08 (done)**: `adapter devin-prompt` is the one writer of
the schema-augmented prompt copy, consumed by both the adapter round path
and the host turn path. The two hand-maintained heredocs had drifted in
line-break placement; the adapter's wording is the canonical text, pinned
by a byte test. This prose is the runtime's only schema channel, so it
gets the byte-pin treatment load-bearing text gets here.

**script-adapters-09 (done)**: `settle_result_identity` in
runtime-common.sh is the one copy of the post-CLI three-way decision
(late handshake / resume collision / result-model recording). claude
passes its result model for both the handshake and the recording branch;
codex passes its requested model for the handshake and an empty result
model — the quiet third branch is a capability fact, not drift, exactly
as the verifier put it. Behavior byte-preserved on both paths; the CAS
writes stay in the called helpers (D24 authority principle untouched).

**script-adapters-12 (minimal, rest declined)**: the fake adapter's
inline handshake-failure patch now goes through `adapter result-patch`
like every other failure path; the patch gains the explicit usage:null
that fail_pending's shape already carries. The finding's original target
— wholesale sourcing of runtime-common by fake.sh — is DECLINED on the
verifier's own analysis: the library's hooks cover only the repair path
while the fixture injects failures before and inside the fixed sequence,
and the fixture exercises the dispatch/Go surface, where drift breaks
loudly. Restructuring the library to save a fixture duplication would
invert the risk.

**script-adapters-15 (resolved by D27)**: the unquoted $dispatch/$root
expansions lived entirely inside the selftest block the D27 port
deleted; a sweep finds no unquoted remnants in runtime-common.sh.

## D29 — The refactor gate joins the validate family (script-misc-2 sign-off)

**Decision**: `validate refactor-baseline --command record|check` owns
every decision the last shell policy gate made — baseline parsing, the
dirt-beyond-baseline classification (NUL-delimited porcelain, rename
second-records read as foreign dirt: the safe direction), ancestry, and
the cadence backstops. scripts/refactor-baseline.sh is now the Phase A
shim shape: usage, flag parsing, config plumbing (still through
metasystem-config.sh, so flag/env/.local precedence is byte-identical),
one exec. The on-disk sha=/recorded_epoch=/gate= format, every message,
and the 0/1/2 exit contract are preserved and unit-proven against real
git repositories, including the spaced-path and +signed-epoch edges the
shell's regex semantics implied. Verb name follows the family's noun
convention; sign-off delegated per the AFK ruling.

**Why**: the gate is the refactor skill's blocking check — a
completion-gate decision engine that was untestable by unit test and
invisible to the coverage ratchet, in a repo whose ruling is that core
decisions live in Go. The verifier's downgrade (doctrine gap, no active
harm) is why it waited this long, not a reason to leave it.

## D17 addendum — the implementation pass landed (W4.22)

The three moves D17 decided are now real: cmd/, internal/, go.mod, and
go.sum joined the adoption allowlist (benchmark/ and development/ remain
excluded — verified in the smoke run); the shipped CI workflow gained
actions/setup-go pinned by go-version-file so the go.mod toolchain
decides; and the adopt fixture asserts the filled target's own
validation prints "go gate: PASSED" — D17's whole point, behaviorally.
Known cost accepted: every nested adopted validation now rebuilds and
gates the engine, so suite wall-clock grows on both hosts (Go build and
module caches are user-level, so nested runs stay warm). The r10
criticals close as designed: source tracked, platform-independent, no
embedded committing-HEAD.

## D30 — The W4.23-25 tail: two smalls done, two optional verbs declined (script-misc-8/-9/-5/-7 sign-offs)

**script-misc-8 (done)**: settings.json is now derived structurally —
`json strip --key _comment` over the annotated enforcement asset —
instead of sed-deleting any line containing the substring. The annotated
file keeps its comment for humans; StripKeys treats an absent key as a
no-op so adoption stays re-runnable; unit tests pin both.

**script-misc-9 (done)**: the watcher's dead emit-event wiring is
deleted. Its only event emission happens inside `supervise
watcher-pass`, and the sourced-but-never-called block was exactly the
false impression the finding named. The other emit-event.sh consumers
are real and untouched.

**script-misc-5 and -7 (DECLINED)**: the adoption-plan and
skill-validation verbs stay unbuilt. The verifier downgraded both as
not carrying their weight, and nothing has changed that: no caller
pulls for either surface, and building speculative verbs contradicts
the consolidation ruling that shrank the family count in the first
place. Reopen if an adopter actually asks for a machine-readable
adoption plan or a skill linter with teeth.

## D31 — The standing reaper says its decline out loud (F5)

**Decision**: the reaper's no-kill decline is now an emitted state, not
silence. When a record reads running, its cap has expired, and the
custodian is not provably dead, each pass emits one
"REAP-DECLINED job=<id> cap expired, custodian <verdict>; kill
authority stays with dispatch" line. Nothing else changes: the record
stays with the kill-capable dispatch path, the no-kill-authority rule
is untouched, and the emit is suppressed when no emitter is wired
(library callers). The pre-existing core-transitions fixture already
staged this exact state and asserted the old silence — its expectation
now demands exactly one decline per pass, which is the finding's
"once per pass" contract made executable.

**Why**: an operator reading the reaper's output could not distinguish
"nothing needed doing" from "something needs doing that only dispatch
may do" — F4's orphan-window design will lean on exactly this signal.

## D32 — The orphan window closes from the inside (F4 design, critique-converged)

**Decision**: F4 is designed and converged (plans/f4-orphan-window-design.md,
revision 5): the DETACHED ADAPTER SUPERVISOR — already the record's
custodian, launched into its own session, already the CLI's parent —
enforces the record's own handshakeDeadline and capDeadline over its
CLI child. No adoption protocol, no new authority class, no new
standing component; the waiter and the standing reaper are unchanged
(D31's REAP-DECLINED line becomes the regression signal). Kill domain:
the supervisor's own process group minus itself (membership survives
reparenting; per-pid TERM/grace/KILL sweeps; death proven — no member
but self — before the terminal CAS, else the record stays
nonterminal). Deadlines are cached once after the launch CAS,
fail-closed; enforcement runs from the first instruction including the
gate wait. Accepted residuals, stated in the design: a custodian that
dies leaving grandchildren (existing reaper + census case) and an
inert-but-alive custodian (no hard bound without a fenced heartbeat
lease, which would be its own design).

**Process note**: five critique rounds, codex gpt-5.6-sol at
reasoning_effort=xhigh through the codex CLI directly — the sanctioned
fallback while this checkout's supervision dispatch stays blocked on
subdirectory conf resolution. Round 1 (ten material findings) replaced
my Option A lean with this simpler shape the draft had missed; rounds
2-4 narrowed to mechanics (4, 3, 1 findings); round 5: AGREE.
Critique transcripts in the session scratchpad (f4-critique-r*.out).
Implementation is SOLO (supervision core) with boundary suites after.

## D32 addendum — F4 implemented; the fixture caught two real defects

The implementation landed in three checkpoints: f9e554e (proc
group-members, the kill domain), 510d205 (the enforcement core in the
adapters' shared wait loop), and this one (the committed five-leg
fixture, adapter-deadline-fixtures.sh, wired into the suite and the
supervision canary). The fixture stages the PRODUCTION topology (the
driver is a process-group leader, as launch-detached makes the real
custodian) and immediately caught two defects the scratchpad harness's
wrong topology had masked: (1) a non-interactive shell reaps a killed
background child only at an explicit wait, so the dead CLI stayed a
zombie — and a zombie keeps its group membership — through the whole
sweep; the enforcement now terminates AND REAPS the direct child first
(terminate_cli_child), then sweeps the remainder, which holds no
children of ours. (2) The enumerating verb counted its own invocation
chain (the metasystem process and its command-substitution subshell
live inside the caller's group while probing), so the domain could
never read empty; proc group-members now walks its own ancestry up to
the excluded caller and excludes the whole chain. Five legs prove cap
expiry (one running→timeout budget-cap CAS), handshake expiry
(pending→failed handshake_timeout), the zero-signal stand-down, the
lost-CAS race settling with exactly one attempt, and the unproven
domain staying nonterminal. The supervisor-crash leg is the standing
reaper's existing dead-custodian case, proven in reaper_test.go. One
transient supervision-canary RED occurred during checkpoint 2 (timing
fixture under machine load, green on immediate solo re-run).

## D33 — Suite-time reduction: the boundary-scoped gate witness and the delivery contract (Wido-commissioned, critique-converged)

**Commissioned by Wido 2026-08-14 with priority over the remaining
review queue.** Design at plans/suite-time-reduction-design.md,
revision 5; five critique rounds (codex gpt-5.6-sol, xhigh, CLI
fallback), findings 10 → 4 → 3 → 1 → AGREE.

**What lands**: (1) the boundary-scoped gate witness — one full
go-gate per validation run, executed INSIDE an extraction of the same
`git archive HEAD` snapshot adoption stages (witness if and only if
snapshot-gated); nested adopted validations accept it only after
digest equality over the gate-input closure, with a
cannot-be-mistaken verdict wording, hardened handoff (0700 run-state
dir, 0600 witness, lstat checks, controller-sourced run identity),
seed-mode and FORCE fencing, and NO cross-boundary reuse — a fresh
boundary always pays a fresh gate. (2) `--delivery-contract`: a
separately named nested-validation entry point proving payload
completeness, wiring, self-gating, the binary-identity smoke
(-buildvcs=false, engine-digest stamp), and the session legs; the
profile-drift negative leg keeps the canonical full validator; the
engine-behavior families are skipped only behind the outer
controller's runtime digest-equality check. (3)
metasystem.engine-delivery becomes a REQUIRED key (closes the D17
fail-open the critique found: a deleted go.mod no longer reads as "no
engine expected"). (4) The platform claim stated honestly:
intermediate boundaries claim Linux validity; Darwin full suites are
required before benchmark cohorts, releases, and VM-red reproductions,
and are otherwise every-third-batch sampling with the between-sample
risk accepted in writing.

**Notable rejections along the way**: the draft's persistent
machine-level cache (not a trust boundary, not equivalent to a fresh
gate — the critique's strongest point), and cross-boundary witness
reuse (govulncheck data and race schedules are time-varying).

**Measurement obligation, met (2026-08-14 late afternoon)**: the VM
full suite at e8538af ran GREEN in 545 seconds (9.1 minutes) against
~20 minutes at 724a136 pre-D33 — a 2.2x speedup, better than the
design's 10-12 minute estimate. The witness armed once, both
delivery-contract nested runs accepted it (asserted by the outer
fixtures), and the profile-drift leg still ran the full validator.
The implementation shook out six real defects on the way in, each its
own commit: the fresh-tree 65.7% coverage trap (binary-driven
fixtures skip without bin/metasystem — also the true cause of the
morning's misattributed cold-cache flake), flake-family instances
five and six (absent-tag scan, liveChild one-shot), a fail-open
clean-roots test (a failed git status read as clean), GNU stat's
stdout pollution under -f, a contract-env export ordered after the
gate that needed it, and the gofmt tripwire whose subject does not
exist under the contract. Five failed VM rounds found them at 205,
217, 302, 324, and 173 seconds each — the fast-fail loop the witness
design itself made cheap.

## D34 — W5 batch 1: five suite-boundary repairs (script-validate-1/-2/-3/-8/-9 sign-offs)

**-1 (verb sign-off)**: `mission contract-hash` prints the canonical
signed-bytes digest through contractCanonicalSignedBytes — hash-only,
no grammar gate, exactly the verifier's endorsed shape (the seal path
cannot accept the envelope-only fixture contract without reshaping it
around instruments it does not need). The suite's awk shadow is
deleted; byte-equivalence between verb and awk was proven on a
trailing-whitespace-plus-approval shape before the deletion, and the
exported surface carries a unit pin.

**-2 (delegated sign-off)**: the fixture's freshness wait polls the
EXISTING `job census-fresh` verb — the same ruling every dispatch
gates on — so the interval-halving heuristic and the generation join
live in exactly one place. No --wait convenience form was added (the
poll loop already existed shell-side; a blocking form would be new
surface without a second caller). The ARM shim's wait is
newer-than-my-call, not freshness; its python parse became `json get`.

**-3 (exit-code choice)**: the generation-mismatch refusal is
ArmingWindowError in internal/dispatch, surfaced by `job census-fresh`
as EXIT 9 — chosen as unused across dispatch.sh's spoken codes (0, 1,
2, 3 CAS-settled, 4, 6 unresumable, 7 unchanged-usage, 77 refused
permission) — with the message bytes unchanged. Two of the three suite
retry loops now branch on the code; the tty fixture's loop still reads
the structured censusGeneration= token because the tty wrapper's
exit-code propagation is unverified — recorded residue, revisit if
that wrapper ever proves transparent.

**-8**: the adopted-mode registration checks read metasystem.runtimes
through the config engine, honoring the flag/env/local/conf precedence
the suite itself enforces three hundred lines later.

**-9**: the capability-snapshot naming contract is pinned by
TestSnapshotNameGrammar under the go gate (generated names against the
grammar, sequence advance included); the source-text grep that failed
on any reflow is gone.

## D35 — The suite is an orchestrator again (script-validate-4 sign-off)

**Decision**: both giant inline blocks moved to the sub-suite shape the
file already used for every other fixture family. The adopt self-test
(~450 lines, with fill_harness_conf and copy_tree_without_artifacts,
whose only callers lived inside it) became scripts/adopt-fixtures.sh;
the dispatcher/adapter-selftest/mission-runner E2E block (2,007 lines)
became scripts/agents/dispatch-fixtures.sh. Each sub-suite owns its
temp tree, its armed-supervision shutdown (the tracker machinery is
copied, because a child's registrations must shut down in the child's
own trap), and orchestrator-grade failure-evidence preservation. The
orchestrator keeps the delegate-scope and delivery-contract gating at
the invocation, and makes the tail's SKIP_AGENT_FIXTURES export itself
— a child's export dies with it. validate-metasystem.sh went from
4,676 lines this morning to 2,201; both halves proven by full VM runs
(534s and 495s, each with the witness path live). The shared-state
audit found exactly two pre-block couplings (the fixture-budget caps
and the tracker) and zero post-block references — recorded here
because that audit is what made a verbatim extraction safe.

**Why now**: beyond the finding's own case, the decomposition is the
prerequisite for the parallel-fixture-families option noted in the
D33 discussion — independently runnable sections are the unit of
parallelism.

## D36 — The W5 tail: two fixes, one guard, three declines, one cap ruling

**script-validate-12 (done)**: the own-hooks check reads the one
template_mode derivation instead of restating the detection expression.

**script-validate-11 (resolved as found)**: the fence fixture's kill
already carries `|| true` in the current tree — the D33-era reshaping
of that block closed it; no edit needed, verified at the single site.

**script-validate-10 (triage, delegated sign-off)**: python3 remains a
fixture-harness dependency, now DECLARED — all three suite entry
points refuse up front with a message naming that the metasystem
itself does not need python. The seventy remaining heredocs triage as:
JSON field reads (largely already converted — D34 removed the census
pair; the remainder ride in fixture-fabrication blocks), synthetic
record/conf fabrication (STAYS python: it deliberately writes shapes
the engine refuses, and negative fixtures need a tool without
opinions), and the pty/TTY drivers (STAYS: no engine equivalent, and
building one buys nothing). Wholesale conversion is declined; the
per-read conversions continue opportunistically as files get touched.

**script-validate-5 (declined)**: the 90-line protocol-shape heredoc
is, per the verifier's own analysis, the ONLY drift pin protecting the
shipped protocol files — a drift pin needs an independent copy by
definition, and relocating it into Go changes its home, not its
duplication. The residual (a Go-owned home under the gate would be
sturdier) is noted for whenever internal/returnschema next changes.

**script-validate-7 (declined)**: most of the perl surgery fabricates
deliberately INVALID confs, where a grammar-aware verb is the wrong
tool; the valid-conf builders are self-checking through the engine's
own validate. A config set/unset verb family remains available if a
third valid-conf builder ever appears.

**shellcheck (declined, reopen-when)**: absent on BOTH validation
hosts and the shipped CI image. A conditional only-where-installed
check enforces nothing reproducibly. Reopen when someone provisions it
everywhere; the adapters-cluster quoting bugs it would have caught are
the standing argument for doing so.

**S4-2 cap ruling**: three identical timing-cap firings today (36s
elapsed = the 36s scaled cap exactly), all under active machine use,
all green on immediate solo reproduction. The fixture is correct; its
scale factor is too tight for a shared machine. RULING: Mac sampling
runs are expected to flake at S4-2 under active use and the solo
reproduction stays the answer; widening the scale factor is declined
because it would slow the quiet-machine (VM) case the cap protects.

## HUMAN APPROVAL 2026-08-14 evening — "Approved, and yes to all"

Wido, in the session, in reply to the program summary whose pending
list was: (1) 2-3 more baseline reps (~$9 each, ~$27) to characterize
variance; (2) the D10 floor question, recommended strict; (3) bm-2's
UNCONTAINED-Devin start — explicitly flagged as a human call because
org policy refuses --sandbox and a shell demonstrably writes outside
the workspace; (4) proceeding with backlog items 14 (goal system) and
15 (monitor facility) after the review. All four are approved. The
benchmark series is UNPARKED with that scope; the D10 floor takes the
strict reading; bm-2 may start uncontained. SEQUENCING (mine, not
his): the review's W6+W7 finish first — benchmark cohorts run codex
delegates, which the standing rule keeps apart from suite runs on one
machine — so the series resumes tonight after the final review
boundary, machine dedicated.

## D37 — The schema linter lives in the generator's package (script-fixtures-002 sign-off)

**Decision**: the structured-output invariants — every object typed and
closed, every required list complete, every property declaring a type —
moved from the ~45-line python walker in return-schema-fixtures.sh to
TestMaterializedSchemasObeyStructuredOutputRules in
internal/returnschema, walking every role's REAL materialized version-2
schema under the go gate. The shell file keeps its normalize_return and
assert-return-complete legs (the file itself is NOT retired — only the
linter leg — so the shipped surface keeps its name and its remaining
duties). Proven green against the shipped schemas before the shell leg
was cut, per the tier's never-delete-first rule.

## D38 — The contract grammar matrix moved beside its parser (script-fixtures-003 sign-off)

**Decision**: TestContractValidateRejectsPerKeyMatrix (internal/contract)
carries the shell mutation table's exact semantics — every key of the
base contract rejected when missing AND when malformed with the table's
exact bad values, fifty in-process cases replacing ~52 assert-mission
subprocesses per suite run. The verifier's parity warning held: Go had
7 rows; the bulk was PORT, not delete, and the port ran green before
any shell was cut. The four preflight rejection legs (unsealed,
unsigned, mismatched hash, stale exposure) were verifier-confirmed 1:1
duplicates of TestContractPreflight* and retired outright. What stays
shell: the seal-sign-preflight smokes and the dispatch-allow
seal-survival check — they prove the SCRIPT forwards, which no Go test
can. One residue: the shell base carried stream.secondary (its
missing-variant enforced), Go's base grammar treats it as optional —
the matrix covers the 25 shared keys; if secondary is meant to be
required-when-primary-exists, that is a grammar question for
internal/contract, not a fixture question, noted here.

## D39 — The mission state legs retire against their Go equivalents (script-fixtures-004 sign-off)

**Decision**: the ledger-grammar, state-chain, fork-detection,
reconcile-park, and anchor round-trip legs of mission-fixtures.sh
retire. Equivalents verified present BEFORE the cut: ledger_test,
state_test (TestChainDetectsTamper, TestWriteRefusesIllegalTransition),
and anchor_test's five TestReconcile* rows — including
TestReconcileStillParksOnAnyOtherDivergence and the anchored stop-loss
park, which are the reconcile-fork and self-park conditions the
finding required. The reaper/fence and runner legs stay; CLI
arg-forwarding remains proven by them. mission-fixtures.sh keeps its
name and its remaining duties (no whole-file retirement).

## D40 — The end-state assertions retire; the process-level runs stay (script-fixtures-005 sign-off)

**Decision**: mission-fixtures.sh keeps everything the Go fixtures
cannot reach — the real mission-runner.sh `start --foreground`
launches, the status exit-code contract via wait_end_state, the
contract seal-sign flow, and the landed-orphan staging — and retires
the three python blocks that re-asserted end-state JSON the runner's
own package proves: TestInternalRunCloseStreamCycle,
TestDeliverLandedUnconsumedWritesFinalBlock,
TestInternalRunDispatchTerminalCycle, and TestArmAndPreflightFullPass
were verified present before the cut, covering completed state, the
landed-orphan prompt/ledger/usage annotations, runner-closed chains,
mirror manifests, and turn-log acceptance. This closes W6 item 2: the
mission fixture file survives with its irreducible process-level
duties, roughly a third its former weight across D38+D39+D40.

## D41 — Config-identity legs shrink to the smoke and the appendix pin (script-fixtures-014 sign-off)

**Decision**: the five behavior legs retire against their verified
one-to-one equivalents (the four TestConfigIdentity* tests in
internal/config and TestSelectNoMatchNamesChangedKeys in
internal/capability). The file keeps one determinism smoke through the
real `config identity` CLI — flag forwarding is the script-side
property — and the executable-appendix pin over the SHIPPED filter
files, which is data worth pinning, not behavior. 168 lines → 58.

## D42 — The cap-authority legs move beside the fence (script-fixtures-006 sign-off)

**Decision**: AUTH-R2-001..003 retire from delegate-caps-fixtures.sh.
001 was already double-covered (TestAuthorizeCapUsesPairCap,
TestAuthorizeCapRefusesAboveSigned); 002/003 had NO Go equivalent, so
TestAuthorizeCapRefusesPinnedContractDrift and
TestAuthorizeCapRefusesWhitespaceOnlyDrift were ported GREEN first —
pure file-in/refusal-out logic that needed no processes. The orphaned
cap helpers went with them, the AUTH-R2-009 registry check now guards
the remaining supervision roster (005-008), and the file is what its
name promises: the supervision-set harness.

## D43 — The record-protocol legs retire; the concurrency property got stronger (script-fixtures-012 sign-off)

**Decision**: record-protocol-fixtures.sh shrinks from 119 lines to a
40-line __record-create forwarding smoke. The four behavioral legs had
verified one-to-one equivalents in internal/dispatch/record_test.go;
the ONE novel property — no reader ever observes status=failed without
its protocolError object — was PORTED as a live goroutine reader
hammering the record under -race across fifty applications, with the
.tmp-residue sweep folded in. The in-process version is strictly
stronger than the python poller it replaces: the race detector watches
the same window the poller sampled.

## D44 — The exhaustion legs retire; three cases ported first (script-fixtures-021 sign-off)

**Decision**: conformance-fixtures.sh's six-leg __critique-exhaustion
section drops. The leg-by-leg diff found three cases Go lacked —
critic-cannot-own-successor, the recorded-implementer-successor budget
reopen, and record-round-beats-a-lying-return — ported GREEN as
TestCritiqueExhaustionCodeCriticChain before the cut; wrong-party
enumeration, the manifest patch shape, off-budget none, and
protocol-error recovery with an absent return were already owned by
the three existing tests. The shell's separate second-budget and
human-remedy greps proved to be two substrings of ONE refusal string,
already asserted. The stage-level assert-conformance E2E remains the
file's job. This closes W6 item 3 (D41-D44).

## D45 — Open-work legs retire; the verifier's four missing cases ported first (script-fixtures-008 sign-off)

**Decision**: the verifier was right that the 1:1 parity claim was
false — the five basic cases were covered, but chain-root round
matching, per-stream staleness isolation, the plans/README exclusion,
and the reporter-to-gate-marker integration (live marker silences
open work; dead marker ignored AND pruned by the reporting pass) lived
only in shell. All four ported green into internal/report's openwork
tests before the ninety-line section retired. The supervision-hook
legs (S4-14, S4-15, the idle wording) stay: they exercise the shell
hook itself, which has no Go home.

## D46 — The G-5 lint moves to Go; D17 dissolved the scope objection (script-fixtures-001 sign-off)

**Scope decision, delegated**: the verifier downgraded this finding
because a Go test "runs only where Go source exists" and adopted repos
ran the fixture unconditionally. D17 changed the facts: adopted
repositories now SHIP the engine source (metasystem.engine-delivery is
required) and run the full gate in their own CI, so a gate test
enforces in adopted repos through the same rails as everything else.
The 37-line python heuristic parser is now
TestInstructionOwnersAreInstructionBearing in internal/validate,
beside the preamble-quote parser that reads the same markers, with the
heuristics preserved verbatim. Residue, stated: nested
delivery-contract runs skip gate tests behind digest equality, and
AGENTS.md is not in the gate-input closure — a nested target's doc
drift is caught by that repo's own CI, not by the outer template run;
same as every gate test, now written down.

## D47 — The armer reads identity through the engine; the ps shim and mirror daemon retire (script-fixtures-007 sign-off)

**Decision, in two halves. The production half**: arm-supervision.sh's
identity_alive read the holder's argv through raw `ps`, bypassing the
one-source identity rule every other reader honors — the exact class
the review's identity work existed to close, found empirically when
removing the fixture's ps shim broke arming. It now reads `proc
classify` (fixture-aware, four-way), keeping the conservative rule
that an unobservable argv never permits takeover. **The fixture half**:
with the armer fixed, the python ps shim is dead weight and is gone;
the 20ms identity-mirror daemon — the same standing writer whose torn
writes caused the morning's classification escape — is replaced by ONE
explicit registration after each arming, with the atomic-rename
discipline from the tear fix. Supervision pids change only at arm
time, so a loop was never the right shape. AUTH-R2-005..009 green end
to end without either.

## D48 — The telemetry wiring test runs the real dispatcher CAS; the derivation cases live in Go (script-fixtures-013 sign-off)

**Decision**: telemetry-census-fixtures.sh's `fixture_record_cas` — a
python reimplementation of `__record-cas` that skipped the transition
rules production enforces (status-in-patch refusal, immutable fields) —
is deleted. The wiring leg now stages the record-protocol fixture's
scratch-repository shape (git-inited parent, copied dispatch.sh +
bin/metasystem, jobs and record-locks dirs) and drives
`record_result_effective_model` through the REAL `__record-cas`; the
scratch shell classifies HUMAN, the same ungated trust story every
operator command rides. Of the three model-derivation cases, one-key
stays as the wiring vehicle; zero-keys ("unobserved") and two-keys
(sorted "multi-model:a,b") were verified 1:1 against ClaudeResultField
rows in internal/adapter/runtime_test.go before retiring — derivation
is Go-owned, the shell proves only that the adapter's plumbing reaches
the dispatcher and the return schema. The CENSUS-FAILED schema leg at
the CLI boundary stays untouched, per the finding.

## D49 — config tailor learns the fake runtime and --set; the three divergent perl rewrites die (script-fixtures-020 sign-off)

**Decision**: the verb-surface extension the finding needed sign-off
for is shaped as `--runtimes fake` plus a repeatable generic
`--set key=value`, not the suggested monolithic `--fixture` profile:
the three sites want different override values (watch interval 1 vs 5,
census bytes 350/4096/untouched, tiers, investigator), so a profile
would have needed all the same knobs while hiding them. TailorConf
gains exactly two fake-runtime rules — fake may be the default runtime
only when it is the sole selection, and dropped per-runtime model
bindings collapse to one `model.fake=fake-model` per role (explicit
fake bindings win) — and SetConfKeys is a tested atomic
replace-or-append editor, so no conf-key grammar remains in fixture
regex. Equivalence was proven before the swap: old-perl output vs
engine output on the shipped conf is byte-identical on effective lines
for the dispatch-fixtures and fingerprint-harness variants. The
supervision variant differs in exactly one place, and it is the drift
the finding warned about: its perl left `role.code-critic.runtime=
<runtime>` and the `<model>` placeholder untouched (its tier clauses
also never matched anything — recorded, no behavior lost); the engine
now normalizes those to real fake bindings, strictly better-formed,
and supervision never dispatches a code-critic. Out of scope and left
alone: the one- and two-clause rewrites (supervision's operator
harness, mission-fixtures' evidence-root line, delegate-caps'
runtimes append, adopt-fixtures' placeholder-emptying) — they set
specific keys rather than reimplementing tailoring semantics.
adopt.sh's production call is untouched. Proof: validate unit tests
(fake collapse, explicit-fake wins, SetConfKeys
replace/append/dedupe), then all three harnesses green end to end.

## D50 — The W6 hygiene batch, one disposition per finding (closes W6)

**script-fixtures-009**: the grep -Eq ERE pre-check died — it validated
signatures with the wrong engine (POSIX ERE) while the consumer
compiles RE2, and the authoritative `proc signature-check` runs two
lines later. **-010**: FRCC-011 was vacuous (command and grep both
ended in || true) and the lease-refused witness had NO Go coverage, so
the retirement discipline held: TestNonHolderAnnounceEmitsLeaseRefused-
Witness now asserts the event lands on the stream when a live holder
refuses an announce, and the shell leg is a pointer. **-011**: sections
4/5 and FRCC-001/002 retired after verifying emit_event is a thin
wrapper over `event emit` — the four named emit_test.go tests prove
the same properties; the real-process legs (caller harmlessness,
concurrent writers, torn fragment, witness-not-authority) stay.
**-015**: copy-verification and symlink-refusal legs retired to their
two named sessionisolation_test.go tests; the adapters' manifest
aggregation and WC-8 human-shell bootstrap stay. **-016**:
RESOLVED-AS-FOUND — W1.24 already fixed it (per-run gofix-$$ tag,
scoped pkill) and the file cites the finding id. **-017**: the wait
ceilings now come from fixture-budget.sh (go-owner-wait=8,
go-owner-crashloop=30 in the owner's table) and the poll rides
METASYSTEM_FIXTURE_POLL_INTERVAL_SEC; the 3s stability window stays a
deliberate literal — it is an assertion window, not a wait ceiling,
and scaling it up only lengthens exposure to load-induced churn.
**-018**: the stray root-level cp in make_repo died; the explicit
per-path copies already place every asset. **-019**: the shell grep of
host.go source text is replaced by TestAssembleHostCommandExports-
MissionLineage, which asserts the constructed host command's actual
environment. **-022**: mission-fixtures' fast-fail now reads status
through `json get` (indentation-proof) and supervision-fixtures'
python json_field is the engine verb — verified first that no caller
uses list indices, the one shape the verb lacks. Proof: four touched
Go packages green in full, all six edited fixture files green
standalone.

## D51 — W7 smalls: the glossary tells the truth, the routing index routes, and eleven package docs match their territory

**architecture-4**: the five glossary owners that named deleted Python
scripts now point at the engine (`metasystem lease`/internal/lease,
the census via `watch-background-jobs.sh --census`/internal/census,
emit-event.sh as a thin `event emit` wrapper, the `mission fence-*`
verbs, `metasystem schema` + `validate return-complete`); zero .py
references remain and the instruction audit passes. **architecture-8**:
wow.md's dead `meta/` route now names `development/` at the repository
toplevel — phrased without a `../` path because the audit rightly
refuses outside references in metasystem-owned files (first attempt
was caught by exactly that rule); README's layout block no longer
draws development/ inside the tree. **Package-doc batch**:
mission-contract-11 (contract's overview moved above the package
clause where go doc surfaces it; mission got a package-level sentence
distinct from ledger.go's file role), dispatch-supervise-8 (Launch's
false launcher-shell/reparent-to-launchd narrative replaced with the
direct-exec Setsid truth), -10 (dispatch's doc owns all ten
responsibilities by file), -12 (backoff docs now state the pinned
schedule: 0 for k=1, interval × 2^(k-2) for k≥2), adapter-host-
registry-7 (the "only the reusable core lives here" boundary claim
corrected — runtime decision helpers live in the package since the
port; stacked encodeJSON comment collapsed), validate-report-14/
architecture-10 (report owns frontier honestly), validate-report-16
(audit states both concerns), foundations-13/architecture-9 (config's
three-depth span), foundations-15 (doc claims a refusal error, not a
typed one; exported mutable Modes map replaced by ValidMode()),
missionrunner-8 (cycleContext.values, hostProcess.err, and the stray
pragma on a used local deleted; addendum: the final boundary's first
firing was the coverage ratchet refusing authority at 82.6% against
its 95% floor — the ValidMode swap added uncovered branches the map
lookup never had; TestValidMode restores 95.7%, and the ratchet doing
exactly its job is worth recording), script-orchestration-14 (both
stale TODO(go-wiring) comments resolved to the shipped truth;
lock_owner_state's ps shadow-classification was ALREADY gone —
resolved-as-found). **Declined: architecture-6's rename half** —
renaming internal/contract ripples every import for a naming-taste
win; the doc-comment half (the finding's substance) landed. Reopen if
contract-the-word confusion actually bites. Proof: ten touched Go
packages green in full, gofmt/vet clean, audit passes, dispatch.sh
syntax-checked (comment-only change).

## D52 — W7's three documents land; the map matches the territory (closes W7's numbered items)

**architecture-3**: `docs/architecture.md` is the engine map — what
the binary is, the source-delivery story, the core-vs-plumbing ruling
stated as standing doctrine with its origin, the three-tier layering
with its two deliberate exceptions, one line per package (31), and
the family-to-package table (19) derived from cmd's actual imports.
README's layout block now draws cmd/, internal/, go.mod, and
bin/metasystem, and wow.md routes to the map. **architecture-7**:
`docs/design/dispatch-sequence.md`, the delegate-job ground-truth
map in mission-cycle-sequence.md's genre, pinned to 48442ef —
26 ordered steps from the `__lock-owner` re-exec to close, each
naming the verb that decides and the shell lines that plumb, with
the failure joins and the sequence's real surprises stated as facts
(the dispatcher reaps its own job on every poll; the handshake
deadline is stamped at launch so waiter and backstop share one
number; the handshake-timeout verdict lands before the kill).
Anchors spot-verified against the pinned sources. **architecture-5**:
the four standing contracts are promoted to docs/design following
the wire-documents pattern — supervision-registry (REG-1..7),
supervision-lifecycle (D-1..D-7), flight-recorder (D-1..D-8),
stop-loss-core (C1..C3 + invariants + failure behavior) — verbatim
moves with contract preambles, PROMOTED stamps on the plans (kept
as history per the finding, unlike wire-documents' deleted plan),
and every code, script, and doc citation repointed to the promoted
homes (15 code files, 7 doc sites; the contract-warning string and
its test moved together). **W7 item 7 (idiom lows)**: dispositioned
as the review itself scoped them — "as encountered", not mandatory;
missionrunner-8 landed in D51, several others fell out of W1-W6
restructures; the remainder stay ambient idiom notes, not review
debt. Proof: audit green after every docs pass, contract package
green in full, build green.

## Closing tally — the review program is complete

**The final program boundary is GREEN ON BOTH HOSTS**: the full suite
passed on the VM (the validation authority) at 2c92dfe in 532 seconds
and on the Mac in 1,241 seconds — the Darwin full suite the D33
platform claim requires before benchmark cohorts. The boundary earned
its keep on the way: its first firing, at 230e600, was the coverage
ratchet refusing internal/authority at 82.6% against its 95% floor —
D51's ValidMode swap had added uncovered branches — and the fix
(TestValidMode, 95.7%) is the program's last commit. Every W6 and W7
change sits under a both-hosts-green boundary (W6 at 7c96452: VM 561s,
Mac 1270s).

### The program in one paragraph

The 2026-08-12 full-system review produced an execution backlog of 101
distinct findings across seven tiers (the tier lists carry 102 slots;
script-validate-11 appears in both W1.24 and W5.10 and counts once). Under
the 2026-08-13 AFK delegation, all of it was executed autonomously:
decisions D1 through D52 record every sign-off call, every decline with its
reopen condition, and every residue, with one entry per decision and the
alternative not taken named in each. Wido's review surface is this
document; the standing instruction is revert on disagreement.

### Per-tier accounting

**W1 — correctness (26 items): 26 landed, 0 declined.** The fail-closed and
proof-discipline fixes all went in as specified, several with expanded
scope from the critique rounds (the classification abort, the sweep's
only-ESRCH rule, the bounded record locks).

**W2 — consolidations (17 items): 17 landed, 0 declined.** The notable
judgment calls are D9 (one lock protocol, bindings keep their bytes, no
fourth liveness state) and D12 (the durability contract is FINISHED for
the writers of durable state and explicitly staged for the rest, not
retired and not left as an unclaimed guarantee).

**W3 — CLI uniformity (8 items): 8 landed, 0 declined.** D15 (usage errors
exit 2 everywhere) and D16 (string booleans keep their wire spellings but
refuse typos) are the two sign-offs; staticcheck joined the gate.

**W4 — script boundary (25 items): 23 landed, 2 declined, 1 landed
minimally.** The big one is D17: the adopted payload ships the engine
source and CI rebuilds it — decided under the later, more specific
delegation over the r10 human-severance, flagged as the revert point.
Declined: script-misc-5 (adoption-plan verb) and script-misc-7
(skill-validation verb), both verifier-downgraded as not carrying their
weight; reopen if an adopter actually asks for a machine-readable adoption
plan or a skill linter with teeth (D30). Landed minimally:
script-adapters-12 — the fake adapter's failure patch now rides `adapter
result-patch`, but the finding's wholesale sourcing of runtime-common by
fake.sh is declined on the verifier's own analysis (the fixture injects
failures outside the library's hook coverage; restructuring would invert
the risk); reopen if those injection points ever move inside the library's
hooks (D28).

**W5 — suite decomposition (10 items): 6 landed, 3 declined, 1 triaged
with the wholesale half declined.** Landed: the five smalls (D34) and the
decomposition itself (D35, 4,676 lines to 2,201). Declined with reopen
conditions (all D36): script-validate-5 (the protocol-shape heredoc IS the
drift pin; a pin needs an independent copy — reopen when
internal/returnschema next changes), script-validate-7 (most of the perl
fabricates deliberately invalid confs, where a grammar-aware verb is the
wrong tool — reopen if a third valid-conf builder appears; D49 later added
`config tailor --set` for the valid-conf tailoring class specifically),
and shellcheck (absent on both validation hosts and the shipped CI image —
reopen when someone provisions it everywhere). Triaged:
script-validate-10 — python3 is now a DECLARED fixture-harness dependency
refused up front by all three suite entry points; wholesale heredoc
conversion is declined, per-read conversions continue as files get
touched.

**W6 — fixture retirement (9 items): 9 landed, 0 declined.** Every
retirement obeyed the port-first rule, and the verifier's parity warnings
held every single time: D38 ported the 25-key contract matrix before
cutting, D42 ported two drift tests that had no Go equivalent, D44 ported
three exhaustion cases, D45 ported the four open-work cases the 1:1 claim
missed, D48 verified the derivation rows, D50 ported the lease-refused
witness and the lineage-export assertion. Two findings resolved as found
(-016 was already fixed by W1.24; script-orchestration-14's ps
shadow-classification was already gone). D47 found a production defect on
the way (the armer's raw-ps identity read) and D49 killed the three
divergent perl conf rewrites behind `config tailor --runtimes fake
--set`.

**W7 — documentation (7 items): 6 landed, the rename half of one
declined, item 7 dispositioned as scoped.** The glossary points at the
engine (D51), wow.md's dead route is fixed (D51), eleven package docs
match their code (D51), and the three documents exist: the engine map
(docs/architecture.md), the dispatch-sequence ground truth
(docs/design/dispatch-sequence.md), and the four promoted contracts in
docs/design (D52). Declined: architecture-6's rename of internal/contract
— every import ripples for a naming-taste win; the doc-comment half
landed; reopen if contract-the-word confusion actually bites (D51). W7
item 7 (the ~35 idiom lows) is dispositioned exactly as the review scoped
it: "as encountered" — missionrunner-8 landed in D51, several fell out of
the W1–W6 restructures, and the remainder are ambient idiom notes, not
review debt.

### Residues and open items the program leaves behind

Each is recorded where stated; nothing here blocks the program's close.

1. **The receipt-stats flake stays OPEN, probe armed.** The next Mac-suite
   firing names its dying line and leaves evidence
   (plans/known-issue-receipt-stats-flake.md). VM-green remains the
   validation authority while it stays open.
2. **FixtureEntryFor corrupt-vs-absent hardening** — a fail-closed
   follow-up candidate that goes solo because the signature ripples
   across the auth path (same plan file, D14's neighborhood).
3. **The "started" key dual-read** (D14) — the shared reader still
   accepts the legacy spelling; it drops once a full suite cycle proves
   no writer emits it. That cycle has long since happened; the drop is a
   one-line cleanup nobody has claimed yet.
4. **The tty-loop residue** (D34) — one suite retry loop still reads the
   structured censusGeneration= token instead of exit 9 because the tty
   wrapper's exit-code propagation is unverified. Revisit if that wrapper
   ever proves transparent.
5. **The S4-2 cap ruling** (D36) — Mac sampling runs are EXPECTED to
   flake at S4-2's 36s scaled cap under active machine use; solo
   reproduction is the answer; the scale factor deliberately stays.
6. **The D33 platform claim** — intermediate boundaries claim Linux
   validity; Darwin full suites are required before benchmark cohorts,
   releases, and VM-red reproductions, and are otherwise sampling with
   the between-sample risk accepted in writing.
7. **The durability contract's staged tail** (D12) — call sites outside
   the durable-state writers keep anchor "" until their packages are
   otherwise touched; the package doc names the adopters.
8. **fencesEnforced's 30-second legacy allowance** (D3, D13) — survives
   only for turns without the hostEndedAt stamp; engine-side
   host-interval measurement stays the cleaner unbuilt option.
9. **F4's accepted residuals** (D32) — a custodian that dies leaving
   grandchildren (the existing reaper-plus-census case) and an
   inert-but-alive custodian (no hard bound without a fenced heartbeat
   lease, which would be its own design).
10. **selftest-listener** (D27) — verb kept as a removal candidate; the
    W6 sweep did not remove it.
11. **stream.secondary** (D38) — the shell matrix enforced its
    missing-variant, Go's grammar reads it as optional; whether it is
    required-when-primary-exists is an internal/contract grammar
    question, recorded there.
12. **The D46 gate-scope note** — nested delivery-contract runs skip
    gate tests behind digest equality and AGENTS.md is not in the
    gate-input closure, so a nested target's doc drift is caught by that
    repo's own CI, not the outer template run. True of every gate test;
    now written down.
13. **Supervision re-arm in this checkout** stays blocked on subdirectory
    conf resolution (plans/review-execution.md); the review work never
    needed dispatch, so it proceeded unarmed.
14. **Parked for Wido, untouched by the program**: the adopted-engine
    delivery ruling review (D17 is the revert point), the 119 accumulated
    agent worktrees plus the --help worktree, and the untracked codex
    pids that may be his own sessions.

### The decision index, D1–D52

- D1 — the identity fixture env var is fenced at the arming gate, not threaded through every custodian signature.
- D2 — cohort bm-1-…83558 abandoned after rep 1; the close-protocol fixes land before more spend.
- D3 — the benchmark kit tolerates a 30-second bookkeeping overshoot on turn wall-clock (later replaced by D13).
- D4 — kit role schemas are pinned as the engine-materialized v2 bytes; the referee does not let the candidate define scoring.
- D5 — mirror stamps are per-job; a record's mirror field is a claim about its own evidence only.
- D6 — close tolerates an undelivered non-completed implementer round (later amended by D8).
- D7 — rep 2 waits for four fence/headroom fixes; husked dispatches release reservations and emit reason classes.
- D8 — close attests evidence, not host workflow; internal/usage becomes the one owner of typed usage extraction.
- D9 — internal/lock is the one directory-lock protocol; bindings keep their on-disk bytes; no fourth liveness state.
- D10 — rep 2 proceeds; the delegation floor's strictness is flagged for Wido rather than amended mid-series.
- D11 — the drain-stall incident is owned as my own cadence change; the drain never parks while a kill-capable reap is owed.
- D12 — the durability contract gets finished for durable-state writers and explicitly staged for the rest.
- D13 — hostEndedAt stamps the host process boundary; the cap gates read it; the enforcement stamp gets the same allowance one layer down.
- D14 — pidStartedAt is the fixture identity table's one spelling; one shared reader.
- D15 — usage errors exit 2 everywhere; ambiguous devin correlation gets its own code.
- D16 — string booleans keep their wire spellings and refuse everything else.
- D17 — the adopted payload ships the engine source and CI rebuilds it; decided under the delegation, flagged as the revert point (addendum: landed as W4.22).
- D18 — cap-authority rides the owner-lock verb; SIGKILLed holders' husks heal instead of bricking dispatch.
- D19 — the shell standing reaper is deleted; the fixture now proves it stays deleted.
- D20 — `supervise derive-ceiling` owns the watcher-ceiling derivation.
- D21 — `supervise verify-armed` owns the arming success criterion, one attempt, pure over the clock.
- D22 — `report running-work` owns the turn-end inventory and its human-facing clause.
- D23 — the watcher's classification engine is `report scan-jobs` (and the port fixed the empty stale-min silent failure).
- D24 — `adapter adjudicate-turn` is pure decision; the CAS deliberately stays on the shell's lease-held path.
- D25 — one command builder per runtime; the canary batching doctrine is written generically.
- D26 — `host finish` and `adapter devin-settle` join the engine.
- D27 — `adapter selftest-run` orchestrates the full-contract sequence, with its first automated proof.
- D28 — the W4.20 smalls; script-adapters-12's wholesale restructuring declined on the verifier's own analysis.
- D29 — the refactor gate is `validate refactor-baseline`.
- D30 — settings.json is derived structurally; the dead emit-event wiring dies; two speculative verbs declined.
- D31 — the standing reaper says its no-kill decline out loud, once per pass.
- D32 — F4: the detached adapter supervisor enforces its record's own deadlines (addendum: implemented; the fixture caught the zombie-reap and prober-in-domain defects).
- D33 — the boundary-scoped gate witness and --delivery-contract; measured 2.2x on the VM suite.
- D34 — five suite-boundary repairs, including `mission contract-hash` and the exit-9 arming-window code.
- D35 — the suite is an orchestrator again; adopt and dispatch sub-suites extracted (4,676 → 2,201 lines).
- D36 — the W5 tail: python3 declared, three declines with reopen conditions, and the S4-2 cap ruling.
- HUMAN APPROVAL — "Approved, and yes to all": baseline reps, the strict floor, uncontained bm-2, and backlog items 14+15.
- D37 — the schema linter lives beside the generator, under the gate.
- D38 — the 25-key contract grammar matrix lives beside its parser; ported before the shell was cut.
- D39 — the mission state legs retire against verified Go equivalents.
- D40 — the mission end-state assertions retire; the process-level runner legs stay.
- D41 — config-identity shrinks to one CLI smoke and the executable-appendix pin.
- D42 — the cap-authority legs move beside the fence; the two missing drift tests ported first.
- D43 — the record-protocol legs retire; the -race goroutine reader is stronger than the poller it replaces.
- D44 — the exhaustion legs retire; three uncovered cases ported first.
- D45 — the open-work legs retire; the verifier's four missing cases ported first.
- D46 — the G-5 instruction-owners lint moves to Go; D17 dissolved the adopted-repo scope objection.
- D47 — the armer reads identity through `proc classify` (a production fix); the ps shim and mirror daemon retire.
- D48 — the telemetry wiring test drives the real `__record-cas`; derivation stays Go-owned.
- D49 — `config tailor` learns the fake runtime and `--set`; the three divergent perl conf rewrites die.
- D50 — the W6 hygiene batch, one disposition per finding; closes W6.
- D51 — the W7 smalls: the glossary tells the truth, the routing index routes, eleven package docs match their code; the contract rename declined.
- D52 — the W7 documents: the engine map, the dispatch-sequence ground truth, and the four promoted contracts; closes W7's numbered items.

## D53 — The approved baseline reps run as a FRESH cohort at the current engine

**Decision**: the 2-3 approved baseline repetitions run as a new
cohort at the just-validated engine (2c92dfe line), not as additions
to bm-1-20260813t203657z at e5cbe66. Reasoning: the review program
substantially changed the engine since e5cbe66 (a production identity
fix, the drain and fence work, the suite's own witnesses), the series'
next comparison is baseline-vs-bm-2 and bm-2 will run at the current
engine, and the old cohort contributes one valid rep of history that
mixing engines inside one arm would muddy rather than strengthen.
Three repetitions (the approval's upper count) for variance
characterization, ~$27 against the EUR 240 ceiling with ~$90 spent.
The alternative not taken: pinning new reps to e5cbe66 for
within-cohort purity — that would measure an engine nobody ships
anymore. Sealing and signing ride the standing "sign in my name"
delegation, as in every prior cohort. D33's Darwin-suite precondition
is satisfied by the final boundary's Mac run at 2c92dfe (1,241s,
green).

## D54 — The fresh baseline cohort is banked (one valid rep, the bimodal story confirmed); bm-2 starts tonight

**The cohort result (bm-1-20260814t192803z-44271, three reps at
engine 387c961, $28.32 total, ~33/33/54 minutes)**: reps 1 and 2 are
INVALID on exactly one gate — the stop-loss parked both at three
cycles before the build stream had a certified implementer job, with
byte-identical acceptance 0.018868. Rep 3 is VALID on all seven gates:
acceptance 0.962, requirement coverage 0.923, determinism at its
floor, $10.52. All three runs ended in a stop-loss park at three
cycles; the difference is whether real delegated work landed inside
those cycles. **The reading**: the old cohort's 0.02-vs-0.96
bimodality persists at the current engine and is now cleanly
attributable — when the early cycles produce certified implementer
work, the product is good; when they stagnate, the fuse ends the run
cheaply and the D10 strict floor correctly invalidates it. Rep 3
proves three cycles CAN suffice, which weakens the
"no-gain-budget=3 starves the build stream" reading, but two empty
parks out of three at ~$9 each is the arm's real cost shape. Both
readings stay recorded; no contract amendment without Wido. Series
spend ~$118 of EUR 240.

**The bm-2 call**: Wido's approval of the uncontained-Devin start was
unqualified ("Approved, and yes to all", with the uncontained facts in
front of him), so the start executes tonight rather than waiting for a
watched window — deferral would recreate the parked state the approval
explicitly ended. Stated plainly: swe-1-7 runs uncontained, its writes
outside the workspace are exactly what no fence catches, and the
monitoring story is the driver Monitor plus half-hour heartbeats — the
residual between heartbeats is accepted as part of the approval. One
repetition first; the arm's continuation is decided on its data.
A push notification tells Wido bm-2 is live the moment it starts.

## HUMAN RULINGS — Wido, 2026-08-14 night, before a 10-hour absence

Asked four questions; the answers below are standing rulings for the
unattended stretch, given in writing:

1. **Series plan**: "Drive to 2 valid reps/arm" — baseline, bm-2, and
   bm-2s each run until they hold two VALID repetitions; empty parks
   do not count. Roughly $40-90 of remaining headroom at ~$10 a run,
   inside the EUR 240 ceiling.
2. **Devin overnight**: "Yes, same monitoring" — if bm-2 rep 1 is
   clean, further uncontained swe-1-7 reps run unattended under the
   15-minute heartbeat and driver Monitor.
3. **Fuse setting**: "Raise to 5 overnight" — Wido's contract
   amendment. ledgerNoGainBudget goes 3 → 5 in the bm-1 and bm-2
   manifests for cohorts provisioned from now on. Delegate's note:
   bm-2s already carried 8, its own deliberate setting, so it is NOT
   lowered to 5 — the ruling was a raise from 3, not a flattening;
   flagged here for review. The bm-2 rep already in flight was sealed
   at 3 and stays 3. Comparability note: overnight cohorts at budget 5
   are a variant against tonight's budget-3 cohort; the spec is
   pre-1.0 and comparison-ineligible anyway, and every scorecard
   pins its manifest.
4. **After benchmarks**: "Item 14 then 15" — the goal-system design
   pass, then the monitor facility, critique-looped, DESIGNS ONLY:
   the option allowing same-night implementation was offered and not
   taken, so nothing implements while he is away.

## Series position for Wido (2026-08-14, after the first valid rep)

**The measurement pipeline is done.** Cohort bm-1-20260813t203657z at
engine e5cbe66, replayed at the corrected kit (d079b74): rep 2 passes ALL
SEVEN validity gates — the series' first fully valid rep — and rep 1
fails only the honest delegation floor. Every defect found across five
cohorts is fixed and suite-proven: fence reservation leaks, three
close-protocol teeth, park-over-live-delegate grading, the drain's owed
reap, and both enforcement-stamp allowances. Note: the stored scorecards
for this cohort were written by the pinned (pre-allowance) kit; the
authoritative verdicts above are reproducible by replaying the frozen
targets with the current kit.

**The product story is the next question.** The valid rep scored
acceptance 0.019; an earlier, differently-invalid rep scored 0.962 on the
same contract. Baseline product variance is enormous (stop-loss parks
roughly every other run under no-gain-budget 3), so a meaningful baseline
needs more reps — at ~$9/rep — and a decision on how many valid reps the
series design wants per arm.

**Devin arm facts (from the kit, not improvised):** bm-2 is the Devin arm
— devin swe-1-7, FREE in beta (bm-1's luna cost ruling explicitly does
not transfer), but Devin runs UNCONTAINED: --sandbox is refused by org
policy and a shell command demonstrably writes outside the workspace.
Starting bm-2 means uncontained delegates on this machine — that is a
human call, not a delegated one, regardless of budget. bm-2s is a codex
sol arm ruled 2026-08-09. Spend so far ~$90 of EUR 240.

**Recommended defaults awaiting your word:** (1) run 2-3 more baseline
reps to characterize the variance (~$27); (2) decide the D10 floor
question (rep 2 proves the floor is passable, so the strict reading now
looks right); (3) bm-2's uncontained-Devin start is yours to green-light.
