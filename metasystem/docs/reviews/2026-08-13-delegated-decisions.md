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
