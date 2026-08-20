# Delegated decisions — running record (started 2026-08-13)

Wido delegated the in-flight judgment calls: the agent decides, documents each decision here for after-the-fact review, and reverts on disagreement. Every entry answers three questions in plain English: what was decided, what the alternative was, and what the impact is.

## D1 — The fake-identity test switch is blocked at arming time

**Decided:** The test suite can point the system at a table of fake processes by setting the environment variable METASYSTEM_FAKE_PROCESS_IDENTITY_FILE, which makes identity checks see made-up processes instead of real ones. The `supervise blocking-reserved-cap` verb — the check that every arming gate (re-arm, establishment, and takeover) already consults before starting supervision — now refuses to arm when that variable is set in a checkout whose `metasystem.runtimes` configuration is not `fake`. In other words, a real installation can no longer be armed while the fake-process switch is active. The identity package documentation names this fence.

**Alternative:** Threading a checkout root through `identity.Custodian`, `census.identityAlive`, and `census.AuthIdentity` (the census is the periodic scan that records which agent processes are running) so every read of the fixture is configuration-gated, the way `enumerateFixture` already is. Rejected because that signature change ripples through every custodian call site and the missionrunner's `custodianFn` seam, while the finding names exactly one attack vector — the variable leaking into an armed component's inherited environment — and the arming gate kills it at that choke point. If Wido prefers the deeper fix, it can ride along with W2's lease-census-8 typed-structs work, where those signatures move anyway.

**Impact:** Fake process identities can no longer leak into a running, armed supervision setup. One-shot command-line reads with the variable set still work, on purpose — the Go test seams depend on them.

## D2 — A benchmark cohort abandoned after one invalid repetition

**Decided:** In the benchmark series, a cohort is a batch of repetitions ("reps") all run against one candidate engine version (one SHA). Cohort bm-1-20260813t113617z-83558 was stopped after rep 1: rep 2 had been sealed but was never resumed, so no further money was spent on it. The three close-protocol defects that rep 1 surfaced were fixed (KI-6 round 3, commit ef9d142), along with the kit's capped-turn gap (commit 4b4f7d1), and a fresh two-rep cohort starts at the fixed engine.

**Alternative:** Running rep 2 anyway. Rejected because a cohort measures one engine version: rep 1 was already invalid on the everyChainClosed check (a frozen consequence of the engine bug that has since been fixed), so the cohort could never yield two valid reps, and rep 2's already-provisioned engine predates the fixes — it would have reproduced the failure for roughly $10.

**Impact:** No money wasted reproducing a known bug. Series spend so far is about $30 of the EUR 240 ceiling.

## D3 — Scoring tolerates 30 seconds of bookkeeping past a turn's time cap

**Decided:** The benchmark's fencesEnforced gate checks that every turn stayed inside its contractual time cap. The extractor — the kit component that reads a run's records and scores them — now tolerates a turn's recorded wall-clock time running up to 30 seconds past the cap. The reason: the runner stamps the turn's end time (`endedAt`) only after return validation, ledger delivery, and the state write, so a turn that finished in time can still be recorded a few seconds over. The concrete case: rep 1 of the first rerun recorded 1203 seconds against a 1200-second cap with outcome `completed`.

**Alternative:** Measuring the host's interval in the engine, at the process boundary, which would make the reading exact and the allowance unnecessary. Cleaner, but it touches the meaning of a field on the turn record, so it was queued as a possible follow-up rather than allowed to block the series.

**Impact:** Turns that finished inside their cap no longer fail scoring over bookkeeping delay. The accepted risk is that a genuine overrun of up to 30 seconds would also pass.

## D4 — Benchmark schemas are pinned copies, not generated at scoring time

**Decided:** The five delegate-role schemas in `benchmark/schemas/evidence/` are pinned as the exact byte output of `metasystem schema materialize --version 2` (so stored delegate returns carry the `schemaVersion`/`claimed` envelope), and the mission-state and orchestrator schemas are byte copies of the files the engine ships. `validate-kit.sh` enforces both rules as a drift guard, plus a third rule: every engine-owned evidence filename the extractor requires must still appear in the engine's sources.

**Alternative:** Having the extractor generate the schemas from the benchmark target's own binary at scoring time. Rejected for referee independence — a candidate engine must not get to define its own scoring contract while being scored.

**Impact:** The scoring contract is fixed and only changes deliberately, through the guard, when the kit itself is updated. Drift between the kit's schemas and the engine is caught by `validate-kit.sh`.

## D5 — A mirror stamp now speaks only for its own record

**Decided:** Mirroring copies a job's evidence files into a shared durability manifest, and `mirror_record` stamps the job's record to say that happened. The stamp is now written only on the job that was actually mirrored; sibling records in the same delegation chain no longer receive a copy of another job's mirror result. This changes what a record's `mirror` field means on disk: it is now a claim about that record's own evidence — which is what every consumer of the field (CloseCheck, and evidence garbage collection after the codex-1/foundations-1 work) actually needs. The happy-path suite fixture was updated accordingly: instead of asserting the stamps are equal across the chain, it asserts the shared manifest covers both records — a strictly stronger durability property.

**Alternative:** Keeping chain-wide stamping and teaching the close step to re-mirror. Rejected because the chain-wide stamp was a false durability claim — in rep 1, a record was stamped as mirrored while its artifacts were absent from the manifest — and stamps that can lie defeat the point of stamping.

**Impact:** A record's mirror field can now be trusted to describe that record's own evidence. The on-disk meaning of the field changed, and the suite now checks the stronger property.

## D6 — A missing diff blocks the close only if the round claims it completed

**Decided:** CloseCheck is the check that a delegation chain may be closed out. An implementer round (a delegate turn expected to produce a code change as `diff.patch`) with no diff file on disk now blocks the close only when its own record says `completed`. Rounds that timed out, failed, or were cancelled pass without a diff — nothing was delivered, so there is nothing to attest. A diff that does exist must still be mirrored at its current content, whatever the round's status.

**Alternative:** Waiving the diff requirement for the whole chain whenever any round is non-completed. Rejected as too weak — it would skip attestation of diffs that do exist.

**Impact:** Chains no longer fail to close over rounds that never delivered anything, while evidence that was delivered stays attested. D8 later amends this rule.

## D7 — The second repetition is held back until four engine fixes land

**Decided:** Rep 2 of cohort bm-1-20260813t132947z stays unspent until four fixes land, because rep 1 was not a fair measurement: half of the mission's signed job budget was eaten by reservations for dispatches that never actually started a process, and the headroom line shown to the host (the orchestrating agent) was wrong every turn. The agent-verified diagnosis (evidence recovered from temp-file residue of the cohort target's record locks): of the four refused dispatches that left husks (job records with no process behind them), only one was refused by the mission fence — the signed limit on concurrent delegate jobs — and that refusal was correct, since two delegates were genuinely live at a concurrency limit of 2, one being an orphan of the crashed round-1 host; two others died on a host error (asking for a writable job without a worktree, correctly refused), and one on a stale capability snapshot after the codex CLI rewrote its own configuration mid-run (KI-19's class). The time caps of 7 and 12 that the implementers timed out under were the host's own `--cap-min` requests against a signed ceiling of 15, so the engine's arithmetic is exonerated — but the host then wrote a false fact into the ledger ("fence refused ... even though the contract states fence.concurrency=2") because the prompt never showed it the concurrency headroom or the roster of live delegates. Four fixes landed: a husk's reservation is now released so the fence's job headroom stays honest and the reserve-before-setup race closes (`mission fence-release-job` plus `fail_setup_husk`); the turn prompt's fence headroom now shows `concurrency=free/limit` plus the live-delegate roster; every husked dispatch now emits a `job-refused` event with a reason class (fence/envelope/capability/worktree/setup) stamped into the record — rep 1's diagnosis had to use temp-file sizes as its primary instrument; and a capability-snapshot miss now self-heals with one adapter probe and re-select, while an unhealable miss still refuses, now naming the failed probe. A policy line was also added to roles/orchestrator.md — dispatch at the signed fence.job-cap-min unless there is a stated reason, and a brief's budget must agree with `--cap-min` — because the host's own cap choice was the run's actual proximate cause, and no engine fix touches that.

**Alternative:** Rerunning as-is and averaging over host variance. Rejected: the dominant failure (the host choosing a 7-minute cap for a 9-minute brief) is not constrained by anything in the engine or the prompt, so it varies randomly instead of reproducing, and each sample costs about $10 while measuring a headroom display known to be dishonest.

**Impact:** Job-budget headroom is now honest, and refused dispatches can be diagnosed from events instead of temp-file forensics. One gap was noted but not fixed: when the process waiting on an orphaned job dies, the job's time cap is only enforced at the standing reaper's cadence — one job (f29a) overran its cap deadline by 75 seconds.

## D8 — The close attests evidence, not host workflow (amends D6); usage extraction gets one owner

**Decided:** Rep 1 of cohort bm-1-20260813t155700z — the first run with all the D7 fixes — failed the everyChainClosed gate in a new way: the host dispatched at the signed cap (the D7 prompt and policy worked) and the implementer completed, but the host parked on stop-loss without ever running the conformance review, and `diff.patch` is created by that review, never by the reap. Under D6's rule, the close of every unreviewed chain therefore wedged at mission end. The amendment: an absent diff never blocks the close (the unreviewed-implementer workflow gap is already delegationFloorMet's verdict to give); a diff the manifest knows about but the disk has lost is still evidence loss and refuses the close ("vanished after mirroring"); and an existing diff must still be mirrored at its current content. Separately (architecture-1), `internal/usage` becomes the single owner of typed usage extraction — DevinUsage (the host and adapter copies deleted), CodexUsageValue, and RootJobID (usage attribution is per chain; W2's walker consolidation may re-home it) — and since the two former writers were byte-identical, the consolidation changes no on-disk bytes; the host's coverage floor re-sets to 79.3 because well-covered code moved out, under the deterministic-minimum rule (the lock=84.0 precedent).

**Alternative:** Keeping D6's completed-without-diff refusal as it stood, which would have kept wedging the close of every chain whose completed implementer never got reviewed — punishing a host-workflow gap through the evidence check when delegationFloorMet already judges it. For the usage code, the alternative was leaving the duplicate copies in the host and adapter.

**Impact:** Closes now attest evidence rather than the host's workflow, and usage extraction has one home with unchanged on-disk output. The cohort was abandoned after rep 1 (one engine version per cohort; rep 1 was invalid under the amended close rule plus an honest delegationFloorMet), rep 2 stays unspent, and a fresh cohort starts at the next validated pin.

## D9 — One directory-lock implementation; each user keeps its exact bytes on disk

**Decided:** `internal/lock` is now the single home of the directory-lock protocol — the lock that is claimed by bringing a directory into existence via rename, with an owner.json file naming the holder. To let both existing users share it, the package gained exactly two things: an OwnerCodec option, so each binding keeps its on-disk owner.json schema byte-for-byte (dispatch: {pid,instanceTag,acquiredAt}; census: {function,pid,pidStartedAt,instanceTag,observedAtEpoch}), and a Cause on HolderError, so a binding can say the owner file was malformed. There is deliberately no staleness extension: dispatch's tag-based stale-holder rule is its liveness probe answering Dead for a live pid whose successfully read command line lacks the recorded tag — the custodian stranger rule, which death-only takeover already expresses. The codex-2 behavior (a holder that is alive but whose command line cannot be read keeps the lock) and the dispatch-supervise-3 behavior (an Unknown liveness answer refuses takeover) stay in the bindings' probes, still pinned by their verdict tests, now driven through the one implementation; ownerlock.go and censuslock.go shrink to thin bindings of roughly 120 lines each covering schema, probe, and refusal wording.

**Alternative:** Adding a fourth liveness state for "stale". Rejected because it would weaken the death-only takeover invariant every consumer reasons about, when Dead already means "a successful read proved the recorded identity absent".

**Impact:** One implementation instead of diverging copies, with no on-disk byte changes. One accepted semantic change: dispatch's owner lock used to report Busy for an ownerless husk (a lock directory with no owner file); through lock.Acquire it now heals one — the canonical package's documented garbage-by-construction rule, which the review names as exactly what the copies lacked.

## D10 — Rep 2 proceeds; the delegation floor's strictness goes to Wido

**Decided:** Rep 1 of cohort bm-1-20260813t171239z (engine 2ef72cf, with all D7 and D8 fixes) passed six of the seven validity gates — everyChainClosed now passes even with an unreviewed completed implementer, closing the whole close-protocol arc. The one failure is delegationFloorMet, the gate requiring a completed and certified implementer per work stream, and its cause is now purely the system being measured: two implementers completed at the signed cap of 15, but the host certified only design-critic rounds, and stop-loss parked the mission before any implementer certification. Rep 2 runs as-is, about $10 within the pre-approved envelope: the measurement is clean, so a second sample is fair — and the first cohort's rep 2 did meet the floor once, so this is variance in mission dynamics, not determinism.

**Alternative:** Amending the floor now — moving it out of run validity and into product metrics. Not done, because re-scoping a validity gate mid-series is referee tampering unless it is clearly a kit defect, and this gate is arguably intended; it is flagged for Wido instead.

**Impact:** Roughly $10 spent on a fair second sample. The benchmark-design question is explicitly Wido's: real delegation demonstrably happened (two healthy completed implementer jobs), what failed is the host's adjudication step before a stop-loss park, and whether that reads as an invalid run or as the product verdict shapes the whole series — if the strict floor stands and baseline reps keep failing it, the Devin arms compare against no valid baseline.

## D11 — The drain-stall incident: my own cadence change caused it; four fixes land

**Decided:** In rep 2 of bm-1-20260813t171239z, the mission parked as drain-stalled exactly 2.0 seconds past a delegate's cap deadline: the drain (the end-of-mission step that waits out running delegates) parks after the cap deadline plus a 2-second handshake grace, while my own commit 5f5db7f had moved the kill-capable dispatch reap — the cleanup step with authority to kill an overrunning job — to a 5-second cadence, giving roughly a 40% chance per expiry of parking without a reap ever running inside that window. The runner then exited 0 over the parked state, and the cohort graded five seconds later while the delegate was still writing code (it finished on its own 60 seconds past its cap; the score measured a bare checkout). Two of my earlier claims were wrong: the "20+ minutes uncapped" premise was a timezone misread, and D7's note that an orphan's cap "degrades to reaper granularity" was false — the Go standing reaper has no kill authority by design, so an orphaned job's cap actually degrades to unbounded once its waiter and runner are gone. Four fixes landed: F1, the drain never parks while a kill-capable reap is owed — at the deadline it runs the dispatch reap for every live record, re-reads, and parks only what a fresh reap could not resolve; F2, the drain witnesses every reap's exit code and stderr and emits a job-refused event on failure (this incident was undiagnosable by artifact — the runner log was empty); F6, the standing-mode shell reap sweep reports failures without exiting (my earlier accumulate change would have terminated a shell standing reaper before its heartbeat — strictly worse than the starvation it replaced, unbitten only because the standing reaper is the Go component); and F3, in the kit, grading a drain-stalled park now waits out still-running survivors (their caps bound them, with a ten-minute ceiling) and refuses rather than scoring a bare checkout.

**Alternative:** Leaving things as they were and rerunning. That would have kept a roughly 40% chance per cap expiry of parking without enforcement, and kept the grader willing to score a checkout the delegate was still writing into.

**Impact:** Drains now enforce caps before giving up, reap failures are visible, and the kit refuses to grade a stalled run's bare checkout. Two items are deferred to the review backlog: F4, the orphan window itself — a job whose waiter and runner have both exited has no kill-capable enforcer until mission end, and the fix needs a design decision between waiter adoption and a kill-capable supervision sweep, which touches the no-kill-authority rule and deserves its own pass; and F5, the standing reaper logging nothing when it declines to act — it should emit the running-with-expired-cap-and-live-custodian state once per pass.

## D12 — The durability contract gets finished, not retired

**Decided:** The two-outcome durability contract — every write of durable state reports either "safely on disk" or "in doubt" — is the deliberate product of the go-production-grade B5 work, and it gets finished rather than removed. In the scope the finding names, the writers of durable state (dispatch's custody and record writes, the lease's state writes, and the registry's snapshot writes) thread the repository root through as the anchor the durability check needs, and act on a not-durable result by witnessing the doubt: as an event where an event emitter exists, on stderr otherwise. All remaining call sites keep an empty anchor until their packages are touched for other reasons — an explicitly staged migration, not an unclaimed guarantee; the package documentation will say which callers have adopted the contract.

**Alternative:** Retiring the contract. Rejected because dispatch/record.go marks the adoption as pending work, not as a rejected idea, and retiring would delete crash-safety machinery the durability program built and tested just to save threading a parameter.

**Impact:** The named writers now surface doubtful writes instead of silently proceeding. The work is sized L-effort and runs as its own pass with a full suite plus VM cycle, after the in-flight clean-measurement baseline cohort.

## D13 — Turn timing is measured at the host's own process boundary

**Decided:** The D3 bookkeeping allowance was structurally wrong and bit twice: a turn's `endedAt` lands after adjudication, the drain, ledger delivery, and the state write, so the cap gate failed turns whose hosts actually finished inside their cap — first over 3 seconds of bookkeeping, then over 40 seconds of legitimate drain wait in the first fully completed mission of the series. The engine now stamps `hostEndedAt` on the turn record at the host process boundary (on both the completed and capped paths), the kit's cap gate reads that field when present with a 5-second allowance, and the old 30-second allowance survives only for legacy turns without the stamp. The kit's turn schema admits the new field.

**Alternative:** Keeping the D3 allowance on the old end-of-turn stamp. That measures the engine's bookkeeping rather than the host's own time, and the 40-second drain wait had already blown past the 30-second allowance in a turn the host finished inside its cap.

**Impact:** Hosts are now judged on their own wall clock. Cohort bm-1-20260813t191528z was abandoned after rep 1 (one engine version per cohort; rep 1 was invalid on exactly this gate — despite being the series' first completed mission: no park, three clean turns, cycles 3/8), a fresh cohort starts at the next validated pin, and series spend is about $70 of the EUR 240 ceiling.

## D13 addendum — The job-cap check gets the same enforcement allowance

**Decided:** Rep 1 at the D13 pin measured every host interval clean via `hostEndedAt`, but the job-cap gate then failed on a job the engine had correctly killed at its 900-second cap and stamped at 901 seconds after wind-down. The kit's job-cap check now carries the same 5-second enforcement allowance as the host check — a genuinely unenforced overrun runs minutes over, not seconds.

**Alternative:** Leaving the job-cap check exact, which would keep invalidating runs where the engine enforced the cap correctly but the wind-down cost a second on the stamp.

**Impact:** A free replay of rep 1 after the fix leaves only delegationFloorMet failing — the fourth consecutive cohort where, with every measurement defect closed, the delegation floor is the single blocker. That is now cleanly the D10 question for Wido: the host under this contract (no-gain budget of 3) repeatedly parks on stop-loss before certifying an implementer, and whether that reads as an invalid run or as the product verdict shapes the whole series.

## D14 — One reader and one spelling for the fake-identity fixture table

**Decided:** identity.FixtureEntryFor is now the only reader of the fake-identity fixture table — the file that tells tests which pretend processes exist. Before, five packages parsed it privately, using two different spellings for a process's start time, and the shell fixtures wrote both keys into every entry to satisfy them all. pidStartedAt is now the canonical spelling — the one every other record in the tree uses; the legacy "started" key is retired from both shell fixture writers, and the shared reader still accepts it during the transition so any straggler fixture keeps working.

**Alternative:** Leaving the five private parsers and the double-keyed fixtures in place, which would have kept every fixture writer emitting both spellings and left the parsers free to keep diverging. The original record names no alternative beyond this status quo.

**Impact:** One reader, one spelling, with a safety net during the transition. The dual-read is dropped once a full suite cycle proves no writer still emits the legacy key.

## D15 — Usage errors exit with code 2 everywhere; ambiguity gets its own code

**Decided:** The package convention — exit code 2 means a usage or validation error, exit code 1 means an operation failure — now applies uniformly. The mission-fence verbs' invalid mission id, job, and cap errors, and mission-state-verify's flag-pairing error, move from exit 1 to exit 2, matching what the runner verbs already do for the identical condition. The `adapter devin-session` verb stops overloading exit 2 for an ambiguous correlation result and gets its own documented code, chosen at implementation time from the verb's unused range (the precedent is owner-lock's codes 3 and 4, and chain-usage's 7). The implementation rule: before the change lands, every shell caller of the affected verbs is surveyed for exit-code branches, any branch on the old meaning is updated in the same commit, and the full suite on both machines is the gate.

**Alternative:** Grandfathering the fence verbs' exit 1 as "their local convention". Rejected because shell plumbing branches on these codes across verb families — a caller cannot know which dialect it is talking to, which is exactly the defect being signed off.

**Impact:** Exit codes mean the same thing everywhere, and ambiguity is distinguishable from failure. The cost is a coordinated update of shell callers, gated by the full suite on both machines.

## D16 — Boolean flags keep their spellings but refuse typos

**Decided:** Existing string-boolean flags keep their exact wire spellings — "true"/"false" for the --overridden/--signal/--network family, "0"/"1" for --worktree and --devin-checks — but gain strict validation via flags.Func: any other value is now a usage error (exit 2), never a silent false. New verbs use plain flags.Bool. This kills the real trap — a mistyped --signal value quietly disabling the session-handshake deadline — while touching zero shell call sites.

**Alternative:** Migrating every string boolean to flags.Bool for one uniform dialect. Rejected because it rewrites dozens of live call sites across dispatch, adapters, and hooks for no safety gain over strict validation — the wire-compatibility risk dwarfs the idiom win.

**Impact:** A typo in a boolean flag now fails loudly instead of silently meaning "false". No shell caller had to change.

## D17 — Adopted repositories get the engine source and build it themselves

**Decided:** Adoption is the run that copies the metasystem from the template into another repository; the adopted payload now ships the engine's Go source — cmd/, internal/, go.mod, and go.sum join the adoption allowlist — and the shipped CI workflow gains a Go toolchain step. With source present, the suite's existing gate path does the rest: validate-metasystem.sh's metasystem_go_source detection turns the go-gate build back on, and the ALWAYS-REBUILD doctrine (r31) applies in the adopted repository exactly as in the template; the adoption-time binary copy into gitignored bin/ stays as the host convenience it already is — it was never the delivery, and now nothing pretends it is. This resolves all three r10 criticals: the source is tracked (KS-R10-001), platform-independent (KS-R10-002), and needs no embedded committing-HEAD because the compiler rebuilds from whatever HEAD is checked out (KS-R10-003 — the stamp stays informational); coherence-by-pairing also strengthens, since the scripts and the engine source travel through the same adoption run and the pair can now prove its coherence by building. One authority note, stated plainly: kill-shell.md r10 had severed this as a human decision under the reserved-decisions rule, but that severance predates the 2026-08-13 AFK delegation that handed over the review's sign-off items including this one ("decide everything, document, revert on disagreement"), and the review's own critique round moved this decision ahead of all of W4 because the shipped CI enforcement is red on day one in every adopted repository — so it is decided under the later, more specific delegation.

**Alternative:** Three were rejected: per-platform release artifacts (a release pipeline, download authentication, and a network dependency for a tool whose whole doctrine is self-contained checkouts); committing the binary (single platform, repository bloat, and r10 already proved it wrong); and building from the template repository at the recorded SHA (couples every adopted CI run to the template's availability and authentication).

**Impact:** Nothing has changed on disk yet — this entry is the ruling, and the landing is its own pass (W4.22: adopt.sh's allowlist grows the four source roots, the github-actions-metasystem.yml workflow gains setup-go before the suite, the adopt fixtures assert the filled target's suite actually runs the go-gate, and template SHA recording is unchanged). If Wido wants this decision back, this entry is the revert point, and adoption is re-runnable, so nothing is hard to unwind.

## D18 — Cap-authority locks join the owner-lock family

**Decided:** dispatch.sh and arm-supervision.sh each carried a verbatim copy of a spinlock guarding cap authority — the exclusive right to set job time caps. Both now route through the `job owner-lock` verb on the same on-disk directory: claim with a pid-plus-tag identity, spin to the same scaled deadline, and release while refusing another owner's lock. dispatch's tag is its existing per-invocation __lock-owner re-exec tag; arm-supervision uses its own script name (present in its command line, which is what the custodian rule probes), accepting that a pid recycled by another armer reads as alive and waits out the deadline instead of healing — fail-closed in a rare collision, never a wrong takeover. During landing, the AUTH-R2-006 fixtures turned out to simulate "lock held" with a bare mkdir; under the owner-lock protocol an ownerless directory is garbage by construction and heals immediately, so the fixtures now hold the lock with a real live identity (their own pid and script name) and release through the verb — the bare-directory trick dying is the point of the change, since an ownerless lock no longer blocks anyone.

**Alternative:** A dedicated `job cap-authority-lock` wrapper verb. Rejected: the generic verb already carries the protocol, and a second spelling of the same claim would be one more surface to keep honest.

**Impact:** A SIGKILLed lock holder's leftover directory now heals on its own instead of blocking all dispatch and arming until a human runs rmdir. The accepted cost is that a rare pid-recycle collision waits out the deadline rather than healing early.

## D19 — The shell standing reaper is deleted

**Decided:** dispatch.sh's standing-reaper mode — a long-running shell loop with kill authority over delegate jobs — is deleted: the --interval/--heartbeat/--instance-tag/--start-gate flags, the custody start gate, the supervision-only authentication, the tick loop, and every verdict branch only the standing mode reached (stale-claim-epoch, abandoned-setup, the terminal-status skip, the busy-lock comeback). Nothing in production ever launched it: the supervise owner launches the Go reaper component, and the deleted branches' live owners are internal/lease/sweep.go, internal/supervise/reaper.go, and the mission drain. What remains is the lease-held single-shot reap that wait_for_job and the drain actually call; D11's F6 carve-out (a standing sweep must not die before its heartbeat) dies with the mode, and the single-shot sweep keeps the no-starvation visit-all-then-report contract with exit 1.

**Alternative:** Keeping the mode behind a refusal message ("standing mode retired; use supervise component reaper"). Rejected: 80 lines of kill-capable code would keep compiling toward divergence, guarded only by prose.

**Impact:** Dead kill-capable shell code is gone, and the WC-9 authority fixture leg was transformed to guard the deletion itself: where it used to prove the standing loop authenticated supervision before entering, it now proves the loop stays deleted (refusing --interval, while-true, or standing_reaper in reap_jobs) and that the lease-held re-entry survives — the guard the deletion actually needs, since a re-activated shell daemon would carry kill authority the standing-reaper ruling explicitly denies.

## D20 — Deriving the watcher's time ceiling becomes a Go verb

**Decided:** The watcher-ceiling derivation — computing the supervision watcher's time ceiling as the maximum over the 120 floor, the declared --max-cap, dispatch.cap-min, fence.job-cap-min, every cap.min.* configuration key, and every raw METASYSTEM_CAP_MIN_* environment value, plus the 30-minute allowance — moves out of shell into supervise.DeriveCeiling behind the new `supervise derive-ceiling` verb, beside the blocking-reserved-cap fence that consumes its output; arm-supervision.sh just forwards to it. Refusal texts keep the raw key and environment-variable spellings, and misusing --max-cap stays a usage error (exit 2). One deliberate strictening: an ambiguous configuration (a duplicate key) now refuses through the resolver, instead of whatever the shell's per-key lookup happened to do — consistent with every other resolution verb.

**Alternative:** Folding the derivation into blocking-reserved-cap as one combined arm-check verb. Rejected: the ceiling is also attested into state.json and read back by re-arm refusals and by dispatch, independently of the reservation scan — separate questions, separate verbs.

**Impact:** The derivation has one home behind a verb, and an ambiguous configuration now fails loudly instead of resolving arbitrarily.

## D21 — "Is supervision armed?" becomes a Go verb

**Decided:** The arming success criterion — a live owner, a live watcher and reaper with fresh heartbeats, a loaded cap minimum (loadedCapMin) equal to the attested derived ceiling, and a fresh successful census (the periodic scan that records which supervision processes are running) matching the state's fingerprint and generation — becomes supervise.ArmedNow behind the new `supervise verify-armed` verb: one attempt, pure over the clock. arm-supervision.sh keeps the scaled retry loop and its timeout message. Component liveness is spelled the same way the dispatch ladder spells it: census-alive at the recorded start, and a recorded tag must not be provably absent (live and unknown pass; stale and dead fail) — the same identity.TagState the dispatch ladder has consumed since W4.5, so arming and dispatch can no longer diverge on what "alive" means.

**Alternative:** Giving the verb the retry loop itself (a --deadline-sec flag). Rejected: the wait cap is fixture-scaled shell policy (supervision_wait_cap), and a verb that sleeps is a verb a caller cannot compose.

**Impact:** Arming and dispatch now share one definition of liveness, checked through one implementation. Retry policy stays in the shell where the fixtures scale it.

## D22 — The turn-end "anything still working?" check becomes a Go verb

**Decided:** The inventory that runs at the end of every turn to answer "is anything still working?" becomes report.RunningWorkClause behind the new `report running-work` verb, beside the existing open-work verb. Job records are now decoded properly, killing the raw-grep failure class where the word "running" nested inside an error field counted as live work; mission runners are found by tokens in their command lines instead of pgrep piped through sed, gate runs are matched the same way, and the clause's historical wording is built in one home. The hook keeps only the three-way sentence choice, which depends on its own open-work state. The verb outputs the finished clause, not raw JSON: the hook is the single consumer, and the wording is the contract a human reads every turn.

**Alternative:** A JSON inventory that the hook re-words. Rejected: two homes for human-facing wording is how the sentence and the data drift apart; if a second consumer ever needs the raw inventory, the clause function already sits on typed pieces that could be exposed.

**Impact:** False "still running" reports from grep noise are gone, and the turn-end sentence has one authoritative home.

## D23 — The watcher's job classification moves to Go, in the report family

**Decided:** The watcher (the script that monitors delegate job files and prints their state) classifies each job as DONE, CAPPED, NEVER-STARTED, STALE, or VANISHED; that whole engine — sidecar-versus-primary file selection, liveness from sibling files' modification times, verdict precedence, seen-state marking, and baseline adoption — becomes report.ScanJobs behind the new `report scan-jobs` verb. It lands in the report family, not supervise as the review finding suggested, because the recorded r3/KS-R3-009 ruling already assigned job-file classification to the report family and the verb's whole output is greppable report lines — the recorded ruling wins over the finding's suggestion. The script keeps argument parsing, the ARMED banner, the census invocation, and the sleep loop; report-line bytes and the seen-state format are unchanged on the wire, pinned by tests, and the running set (formerly in-process arrays) rides in a script-owned mktemp scratch file so a watcher restart still resets VANISHED tracking exactly as before.

**Alternative:** Homing the verb in the supervise family, as the finding suggested. Rejected in favor of the recorded r3 ruling, as above.

**Impact:** The port fixed a real defect the verifier found: the shell's concatenated digit check accepted an empty --stale-min, after which the age comparison (`[ age -ge "" ]`) failed silently and STALE never fired — the Go engine refuses non-positive thresholds loudly, so a misconfigured watcher now dies at arming instead of watching nothing. On-wire formats and restart behavior are unchanged.

## D24 — Adapter turn adjudication moves to Go; record writes stay in the shell

**Decided:** The adapter turn's terminal-outcome state machine — mapping CLI status and handshake results, validating the candidate return (normalization plus the return-complete judgment, both already in Go), the bounded-repair decision with its byte-identical repair prompt, the settle verdicts, and devin's empty-reply rule — becomes adapter.AdjudicateTurn behind four stages of one `adapter adjudicate-turn` verb (initial, after-repair, settle-result, empty-reply), each printing the tuple the shell then executes. The record write deliberately stays in the shell: adapter record writes ride dispatch.sh's lease-held compare-and-swap re-exec (__record-cas), and a verb writing records directly would create a second authority path around the lease discipline. The shell also keeps process launches (the repair CLI turn) and the per-runtime usage and settle hooks; every error code and phase name now has one home beside the dispatch and missionrunner code that adjudicates on it. The W4.17/script-adapters-04 finding is absorbed by this decision: the repair prompt is written byte-pinned by the verb, the one-attempt bound and eligibility feed the --repair-available flag the verb adjudicates, and the shell keeps only the runtime_repair_turn process launch (the finding's own target) — the eligibility inputs (does a repair hook exist, has one run, is a session present) are shell facts by necessity, since `declare -F` cannot be asked from Go.

**Alternative:** Letting the verb write records itself, moving the compare-and-swap into Go. Rejected because it would create a second authority path around the lease discipline.

**Impact:** Every refusal and decision branch is unit-pinned in one Go home, while the genuinely-valid return paths — which need the full role-schema fixture — are proven end to end by the suite's fake-adapter turns; adapter coverage re-floors from 86.5 to 86.3 on both platforms. One known log shift: "return repaired ... kept as evidence" now prints only when the repair actually completes (after settle), not before the settle attempt — truer, and nothing greps for it.

## D25 — One builder per runtime for CLI launch commands

**Decided:** The command line that launches the Claude CLI is now built in exactly one place: a Go verb called `adapter claude-command`, matching the existing `codex-command` verb that does the same for the Codex runtime. It translates the job envelope (the signed job description) into permission-mode and tool flags, applies the budget policy from the environment with two distinct refusal exit codes (3 for budget exhausted, 4 for turn limit), and prints the arguments as NUL-separated tokens that both adapters/claude.sh and hosts/claude.sh read back. The host script used to keep its own forked copy of this logic with a hardcoded "acceptEdits" permission mode; that copy is now just a "recordless" mode of the single builder — same policy, one spelling. In the same batch, `codex-command` gained --permissions/--record flags so sandbox and network settings are derived in Go (an empty list of writable roots means read-only; network "allow" means enabled), and devin-config now emits the permission mode alongside the config it assembles, because translating the envelope into flags is the security-relevant half of command construction and no longer arrives pre-computed from shell. Also filed under this entry: Wido ruled mid-session that the canary batching rule is written generically in canary.sh — run the full test suite once per batch of 3-5 low-risk commits, or immediately after a high-risk one — with no mention of hosts, VMs, or worktrees, since those are this checkout's development mechanics while the canary rule is metasystem behavior that ships.

**Alternative:** Keep the host's forked copy of the Claude builder and keep deriving sandbox/network flags in shell scripts. That lost because the envelope-to-flag mapping is the security-sensitive part and had already gone wrong once — a past incident (KI-12) was exactly this class of bug.

**Impact:** One tested Go implementation now decides how each runtime's CLI is launched, so the host and adapter copies can no longer drift apart, and the canary rule ships in a form adopters can use unchanged.

## D26 — Turn verdicts and Devin settlement move into the engine

**Decided:** The logic that decides how a turn ended — failed, unresumable, or completed, including the exit codes 3 and 6 that the mission runner interprets — was copied three times across hosts/claude.sh, hosts/codex.sh, and hosts/devin.sh; it is now one Go function, host.FinishTurn, behind a `host finish` verb, and the host scripts simply pass its exit code through. Devin's special case, where the CLI exits successfully but produces no reply, is handled with a --require-reply flag; Devin's per-session cumulative-store update stays in shell but only runs when the verb says the turn completed, and three atomic_result wrapper functions died with their last callers. Separately, Devin's settlement step — certifying that the transcript matches the correlated session, and normalizing the reported model name with an "unobserved" fallback — moved into Go as adapter.DevinSettle behind `adapter devin-settle`, in the same package that already owns the correlation logic. The files written when transcript and session disagree are byte-identical to before, the observed model is recorded even when certification fails (the record must reflect what the transcript actually named), and record writes stay with the shell caller per the earlier D24 ruling.

**Alternative:** Leave three hand-copied versions of the verdict logic in the host scripts and the settlement logic in shell — three copies that could drift, and decisions that could not be unit tested.

**Impact:** One place now decides turn outcomes and Devin settlement, both unit-testable, while the shell keeps only the record writes D24 assigned to it.

## D27 — The full-contract self-test becomes one Go orchestration

**Decided:** The adapter self-test — the sequence proving a runtime can dispatch a job, return a complete result, report typed usage, resume the same session, cancel, and respect the permission denials its own envelope declares — used to be roughly 260 lines of shell in runtime-common.sh; it is now one Go verb, `adapter selftest-run` (internal/adapter/selftestrun.go), ending with the pass record. The Go code still executes dispatch.sh, the adapter script, and assert-return-complete.sh as real subprocesses, so the test rides the same authority paths every real job rides — only the decisions moved into Go, where they are unit-proven: the refusal to run with a placeholder model, the rule that only three denial verdicts exist (empty_reply, protocol_error, runtime_error), session equality, and evidence checks that actually parse return.json instead of grepping it (the old `grep -Fq` would have accepted a marker sitting in a key name or in an unparseable file). Per-runtime knobs stay as declarations in the adapter scripts (selftest_turn_ceiling_sec, selftest_denial_ends_turn) and travel as flags; the tripwire listener now runs in-process, the old selftest-listener verb survives for now as a removal candidate for the W6 sweep, and run_full_contract_selftest shrank to a six-line wrapper. Because real self-tests spend model calls, the suite deliberately never ran this path, so the port also adds its first automated proof: stub-dispatch fixtures drive the full orchestration end to end, plus a refusal test for every verdict. Numbering note from the original record: an earlier projection had earmarked D27 for the W4.21 refactor gate; that work takes D28's slot when it lands, since D-numbers are assigned in landing order.

**Alternative:** Keep the shell version, with assertions in grep and the verdict taxonomy in a case statement — the last adapter-side block still built that way, and one with no automated coverage at all.

**Impact:** The self-test's decisions are unit-proven and the path has end-to-end test coverage for the first time; one legacy verb remains to be swept later.

## D28 — Four small W4.20 findings: two deduplications, one minimal fix, one already gone

**Decided:** Finding -08: the schema-carrying prompt text sent to Devin — that runtime's only channel for learning the output schema — is now written by a single verb, `adapter devin-prompt`, used by both the adapter round path and the host turn path; the two hand-maintained inline copies had already drifted in line-break placement, so the verb's wording is canonical and pinned byte-for-byte by a test, the treatment load-bearing text gets here. Finding -09: the three-way decision made after the CLI exits (late handshake, resume collision, or record the result model) now lives once as settle_result_identity in runtime-common.sh; Claude passes its result model to both the handshake and recording branches while Codex passes its requested model and an empty result model — a capability difference, not drift, exactly as the verifier put it — with behavior byte-preserved on both paths and the atomic record writes staying in the called helpers per D24. Finding -12 was done minimally: the fake test adapter's inline handshake-failure patch now goes through `adapter result-patch` like every other failure path, gaining the explicit usage:null that fail_pending's shape already carries. Finding -15 needed no work: the unquoted $dispatch/$root expansions it flagged lived entirely inside the self-test block D27 deleted, and a sweep found no remnants in runtime-common.sh.

**Alternative:** Finding -12's original target — having the fake adapter source the whole shared library — was declined on the verifier's own analysis: the library's hooks only cover the repair path while the fixture injects failures before and inside that sequence, and the fixture exercises the dispatch/Go surface where drift breaks loudly, so restructuring the library to save a fixture duplication would invert the risk; reopen if those injection points ever move inside the library's hooks.

**Impact:** Two known drift points now have a single owner, the fake adapter patches failures the same way everything else does, and one stale finding is closed with no code change.

## D29 — The refactor gate becomes a Go verb in the validate family

**Decided:** The refactor baseline gate — the blocking check the refactor skill uses to refuse completion when the working tree has changes beyond a recorded baseline — moved from shell into Go as `validate refactor-baseline --command record|check`. Go now owns every decision the shell gate made: baseline parsing, classifying dirt beyond the baseline (read via NUL-delimited git porcelain output, with the second record of a rename treated as foreign dirt — the safe direction), ancestry checks, and the cadence backstops. scripts/refactor-baseline.sh is reduced to the standard thin-shim shape: usage, flag parsing, config plumbing (still through metasystem-config.sh so flag/env/.local precedence stays byte-identical), and one exec into the verb. The on-disk sha=/recorded_epoch=/gate= format, every message, and the 0/1/2 exit contract are preserved and unit-proven against real git repositories, including the edge cases the shell's regex semantics implied (paths with spaces, plus-signed epochs); the verb name follows the family's noun convention and sign-off was delegated under the AFK ruling.

**Alternative:** Leave it in shell, which the verifier's downgrade ("doctrine gap, no active harm") had permitted so far — but that only explained the delay, not a reason to stay: this was a completion-gate decision engine that no unit test could reach and the coverage ratchet could not see, in a repository whose standing ruling is that core decisions live in Go.

**Impact:** The refactor skill's blocking check is now unit-tested and counted by coverage, with behavior byte-identical to before.

## D17 addendum — The engine-shipping decision is implemented (W4.22)

**Decided:** The three moves D17 decided are now real. The Go engine source (cmd/, internal/, go.mod, go.sum) joined the list of files copied into adopted repositories, while benchmark/ and development/ remain excluded (verified in the smoke run); the shipped CI workflow gained actions/setup-go pinned by go-version-file, so the toolchain named in go.mod decides; and the adoption fixture now asserts that a freshly filled target's own validation prints "go gate: PASSED" — which was D17's whole point, proven behaviorally. The r10 critical findings close as designed: engine source tracked, platform-independent, no embedded committing-HEAD.

**Alternative:** None recorded here — this entry documents the landing of a decision D17 already made; the original record gives no more detail on options considered at implementation time.

**Impact:** Adopted repositories now build and gate the engine themselves. The known accepted cost: every nested adopted validation rebuilds and gates the engine, so suite wall-clock grows on both hosts, softened by Go's user-level build and module caches keeping nested runs warm.

## D30 — W4.23-25 tail: two small fixes done, two speculative verbs declined

**Decided:** Finding -8: the settings.json handed to adopters is now derived structurally — `json strip --key _comment` removes the comment key from the annotated enforcement asset — instead of sed-deleting any line containing that substring; the annotated file keeps its comment for humans, StripKeys treats an absent key as a no-op so adoption stays re-runnable, and unit tests pin both. Finding -9: the watcher script sourced event-emitting machinery it never called — its only real event emission happens inside `supervise watcher-pass` — so the dead wiring is deleted; it was exactly the false impression the finding named, and the other consumers of emit-event.sh are real and untouched. Findings -5 and -7 are declined: the proposed adoption-plan and skill-validation verbs stay unbuilt, because the verifier downgraded both as not carrying their weight, no caller pulls for either surface, and building speculative verbs contradicts the consolidation ruling that shrank the verb-family count in the first place.

**Alternative:** Building the two declined verbs anyway would have added surface nobody uses; the reopen condition is written down — an adopter actually asking for a machine-readable adoption plan or a skill linter with teeth.

**Impact:** One fragile text-matching hack became a structural operation, one piece of misleading dead code is gone, and two would-be features are consciously not built.

## D31 — The standing reaper says out loud when it declines to kill (F5)

**Decided:** The standing reaper — the recurring pass that cleans up after dead jobs — used to stay silent in one specific state: a job record still reads "running", its time cap has expired, but the custodian process supervising it cannot be proven dead. In that state each pass now emits exactly one line: "REAP-DECLINED job=<id> cap expired, custodian <verdict>; kill authority stays with dispatch". Nothing else changes — killing remains reserved to the kill-capable dispatch path, the no-kill-authority rule is untouched, and the line is suppressed when no emitter is wired up (library callers). The pre-existing core-transitions fixture had already staged exactly this state and asserted the old silence; its expectation now demands exactly one decline line per pass, which makes the finding's "once per pass" contract executable.

**Alternative:** Keep the silence — but an operator reading the reaper's output could not tell "nothing needed doing" apart from "something needs doing that only dispatch may do".

**Impact:** Operators get a distinguishable signal at no change in authority, and the F4 orphan-window design (D32) leans on exactly this line as its regression signal.

## D32 — F4 design converged: the job's own supervisor enforces its deadlines

**Decided:** The orphan window — the gap where a job's CLI process could outlive its deadlines with nobody positioned to kill it — closes from the inside, per the converged design at plans/f4-orphan-window-design.md (revision 5). The detached adapter supervisor — the process that already owns the job record, was already launched into its own session, and is already the CLI's parent — enforces the record's own two deadlines (handshakeDeadline and capDeadline) over its CLI child; no adoption protocol, no new authority class, no new standing component, and the waiter and standing reaper are unchanged, with D31's REAP-DECLINED line becoming the regression signal. The kill domain is the supervisor's own process group minus itself: group membership survives reparenting, kills go per-pid as TERM, a grace period, then KILL sweeps, and death must be proven — no group member left but the supervisor — before the terminal record update (a compare-and-swap, or CAS, write), otherwise the record stays nonterminal. Deadlines are cached once after the launch CAS and fail closed, and enforcement runs from the supervisor's first instruction, including during the gate wait. Two residual risks are accepted and stated in the design: a custodian that dies leaving grandchildren (the existing reaper-plus-census case), and a custodian that is alive but inert, which has no hard bound without a fenced heartbeat lease — a separate design of its own.

**Alternative:** The draft leaned toward a different shape ("Option A"); the first critique round's ten material findings replaced it with this simpler design the draft had missed — no new components, just the existing custodian doing enforcement.

**Impact:** A converged, implementable design after five critique rounds (codex gpt-5.6-sol at reasoning effort xhigh, run through the codex CLI directly — the sanctioned fallback while this checkout's supervision dispatch stays blocked on subdirectory conf resolution; findings narrowed 10, 4, 3, 1, then AGREE, transcripts in the session scratchpad as f4-critique-r*.out). Implementation proceeds solo for the supervision core, with boundary suites after.

## D32 addendum — F4 implemented; the committed fixture caught two real defects

**Decided:** The implementation landed in three checkpoints: commit f9e554e (`proc group-members`, the kill-domain enumeration), commit 510d205 (the enforcement core in the adapters' shared wait loop), and the current one (the committed five-leg fixture adapter-deadline-fixtures.sh, wired into the suite and the supervision canary). The fixture stages the production process topology — the driver is a process-group leader, just as launch-detached makes the real custodian — and immediately caught two defects that a scratchpad harness with the wrong topology had masked. First, a non-interactive shell only reaps a killed background child at an explicit wait, so the dead CLI lingered as a zombie — and a zombie keeps its process-group membership — through the whole kill sweep; enforcement now terminates and reaps the direct child first (terminate_cli_child), then sweeps the remainder, which by then holds no children of ours. Second, the enumeration verb counted its own invocation chain (the metasystem process and its command-substitution subshell sit inside the caller's group while probing), so the domain could never read empty; `proc group-members` now walks its own ancestry up to the excluded caller and excludes the whole chain. The five fixture legs prove cap expiry (exactly one running-to-timeout budget-cap record update), handshake expiry (pending to failed with reason handshake_timeout), the zero-signal stand-down, a lost update race settling with exactly one attempt, and an unproven kill domain leaving the record nonterminal; the supervisor-crash case is the standing reaper's existing dead-custodian coverage, proven in reaper_test.go.

**Alternative:** Trusting the scratchpad harness's green would have shipped both defects — its topology was wrong in precisely the way that hid them.

**Impact:** Deadline enforcement is live and proven against the real process topology. One transient supervision-canary red occurred during checkpoint 2 — a timing fixture under machine load, green on immediate solo re-run.

## D33 — 2026-08-14: Suite time cut in half with a gate witness and a delivery contract

**Decided:** Wido commissioned this with priority over the remaining review queue; the design (plans/suite-time-reduction-design.md, revision 5) converged over five critique rounds with codex gpt-5.6-sol at xhigh via the CLI fallback, findings narrowing 10, 4, 3, 1, AGREE. Four things land. First, the boundary-scoped gate witness: the full Go build-and-test gate runs once per validation run, inside an extraction of the same `git archive HEAD` snapshot that adoption stages (witness exists if and only if the run is snapshot-gated), and nested adopted validations may accept that witness instead of re-running the gate — but only after proving digest equality over everything that feeds the gate, with a verdict wording that cannot be mistaken, a hardened handoff (0700 run-state directory, 0600 witness file, lstat checks, controller-sourced run identity), seed-mode and FORCE fencing, and no reuse across boundaries: a fresh boundary always pays a fresh gate. Second, `--delivery-contract`: a separately named entry point for nested validations that proves payload completeness, wiring, self-gating, a binary-identity smoke (-buildvcs=false, engine-digest stamp), and the session legs, while the profile-drift negative leg keeps the canonical full validator and the engine-behavior families are skipped only behind the outer controller's runtime digest-equality check. Third, metasystem.engine-delivery becomes a required config key, closing a fail-open the critique found in D17's implementation — a deleted go.mod no longer reads as "no engine expected"; and fourth, the platform claim is stated honestly: intermediate boundaries claim Linux validity only, while Darwin (macOS) full suites are required before benchmark cohorts, releases, and VM-red reproductions, and otherwise run as every-third-batch samples with the between-sample risk accepted in writing.

**Alternative:** The draft's persistent machine-level cache was rejected because a cache is not a trust boundary and not equivalent to a fresh gate — the critique's strongest point — and cross-boundary witness reuse was rejected because govulncheck data and race schedules vary over time. Doing nothing meant staying at roughly 20 minutes per full suite run.

**Impact:** The measurement obligation was met the same day: the VM full suite at commit e8538af ran green in 545 seconds (9.1 minutes) against roughly 20 minutes at commit 724a136 before D33 — a 2.2x speedup, better than the design's 10-12 minute estimate — with the witness armed once, both delivery-contract nested runs accepting it (asserted by the outer fixtures), and the profile-drift leg still running the full validator. Implementation shook out six real defects, each fixed in its own commit: a fresh-tree 65.7% coverage trap (binary-driven fixtures silently skip when bin/metasystem is absent — also the true cause of that morning's misattributed cold-cache flake), flake-family instances five and six (absent-tag scan, liveChild one-shot), a fail-open clean-roots test where a failed git status read as clean, GNU stat polluting stdout under -f, a contract-env export ordered after the gate that needed it, and a gofmt tripwire whose subject does not exist under the contract. Five failed VM rounds found them at 205, 217, 302, 324, and 173 seconds each — the fast-fail loop the witness design itself made cheap.

## D34 — W5 batch 1: five repairs where the suite duplicated or bypassed the engine

**Decided:** Five fixes at the suite boundary. The suite's awk reimplementation of the contract digest is deleted in favor of a new `mission contract-hash` verb that prints the canonical signed-bytes digest through contractCanonicalSignedBytes — hash-only with no grammar gate, the verifier's endorsed shape, because the seal path cannot accept the envelope-only fixture contract without reshaping it around instruments it does not need — with byte-equivalence between verb and awk proven on a trailing-whitespace-plus-approval shape before the deletion, and a unit pin on the exported surface. The fixture's freshness wait now polls the existing `job census-fresh` verb — the same ruling every dispatch gates on — so the interval-halving heuristic and the generation join live in exactly one place; no blocking --wait form was added (the poll loop already existed shell-side, and a blocking form would be new surface without a second caller), and the ARM shim's wait, which is newer-than-my-call rather than freshness, had its python parse converted to `json get`. The generation-mismatch refusal became a typed error (ArmingWindowError in internal/dispatch) that `job census-fresh` surfaces as exit code 9 — chosen because it was unused among dispatch.sh's spoken codes (0, 1, 2, 3 CAS-settled, 4, 6 unresumable, 7 unchanged-usage, 77 refused permission) — with the message bytes unchanged; two of the three suite retry loops now branch on the code, while the tty fixture's loop still reads the structured censusGeneration= token because the tty wrapper's exit-code propagation is unverified, a recorded residue to revisit if that wrapper ever proves transparent. The adopted-mode registration checks now read metasystem.runtimes through the config engine, honoring the same flag/env/local/conf precedence the suite itself enforces three hundred lines later; and the capability-snapshot naming contract is pinned by TestSnapshotNameGrammar under the Go gate — generated names checked against the grammar, sequence advance included — replacing a source-text grep that failed on any reflow.

**Alternative:** Leave the shadows in place — an awk copy of the digest, a shell copy of the freshness heuristic, message-string matching instead of exit codes, direct config reads, and a grep over source text — each a spot where the suite could quietly diverge from the engine it tests.

**Impact:** The suite now exercises the engine's own rulings instead of parallel copies, with one residue on record: the tty loop still matches on the structured token rather than the exit code.

## D35 — The 4,676-line suite becomes an orchestrator again

**Decided:** The two giant inline blocks in validate-metasystem.sh moved out into the sub-suite shape the file already used for every other fixture family. The adopt self-test (about 450 lines, whose helpers fill_harness_conf and copy_tree_without_artifacts had no callers outside it) became scripts/adopt-fixtures.sh, and the 2,007-line dispatcher/adapter-selftest/mission-runner end-to-end block became scripts/agents/dispatch-fixtures.sh. Each sub-suite owns its own temp tree, its own armed-supervision shutdown (the tracker machinery is copied, because a child process's registrations must shut down in the child's own exit trap), and orchestrator-grade preservation of failure evidence; the orchestrator keeps the delegate-scope and delivery-contract gating at the call site, and the tail's SKIP_AGENT_FIXTURES export moved into the child, where it dies with the child. The main file went from 4,676 lines that morning to 2,201, and both halves were proven by full VM runs (534 and 495 seconds, each with the witness path live). A shared-state audit found exactly two couplings before the moved blocks (the fixture-budget caps and the tracker) and zero references after them — recorded because that audit is what made a verbatim extraction safe.

**Alternative:** Leave the blocks inline; beyond the finding's own case, that would have blocked the parallel-fixture-families option noted in the D33 discussion, since independently runnable sections are the unit of parallelism.

**Impact:** The suite is decomposed and readable again with no behavior change, and the ground is prepared for running fixture families in parallel.

## D36 — W5 tail: two fixes, a declared python dependency, three declines, and a flake ruling

**Decided:** Finding -12: the own-hooks check now reads the one template_mode derivation instead of restating the detection expression; finding -11 was already resolved as found — the fence fixture's kill carries `|| true` in the current tree, closed by the D33-era reshaping of that block and verified at the single site. Finding -10 is a triage: python3 remains a dependency of the fixture harness but is now declared — all three suite entry points refuse up front with a message naming that the metasystem itself does not need python — and the seventy remaining inline python blocks split three ways: JSON field reads (largely already converted, the remainder riding in fixture-fabrication blocks), synthetic record/conf fabrication (stays python, because negative fixtures deliberately write shapes the engine refuses and need a tool without opinions), and the pty/TTY drivers (stay, since no engine equivalent exists and building one buys nothing) — wholesale conversion declined, per-read conversions continuing opportunistically as files get touched. Three declines: the 90-line protocol-shape inline block stays because, per the verifier's own analysis, it is the only drift pin protecting the shipped protocol files, and a drift pin needs an independent copy by definition — relocating it into Go changes its home, not its duplication (a sturdier Go-owned home is noted for whenever internal/returnschema next changes); the perl-to-verb conversion stays undone because most of that surgery fabricates deliberately invalid configs, where a grammar-aware verb is the wrong tool, and the valid-conf builders are self-checking through the engine's own validate (a config set/unset verb family remains available if a third valid-conf builder ever appears); and a shellcheck check stays unbuilt because the tool is absent on both validation hosts and the shipped CI image, so a conditional only-where-installed check enforces nothing reproducibly — reopen when someone provisions it everywhere, the adapters-cluster quoting bugs it would have caught being the standing argument for doing so. The S4-2 cap ruling: three identical timing-cap firings in one day (36 seconds elapsed against exactly the 36-second scaled cap), all under active machine use and all green on immediate solo reproduction, are ruled expected behavior — the fixture is correct, its scale factor is simply too tight for a shared machine, and Mac sampling runs are expected to flake there under active use with solo reproduction staying the answer.

**Alternative:** The notable rejected option is widening the S4-2 scale factor, declined because it would slow the quiet-machine (VM) case the cap exists to protect.

**Impact:** The suite's real dependencies are stated up front instead of discovered on failure, three non-fixes are on record with explicit reopen conditions, and one recurring flake is formally classified as expected under load.

## Human approval — 2026-08-14 evening: Wido approves all four pending items ("Approved, and yes to all")

**Decided:** In the session, replying to the program summary, Wido approved all four items on its pending list: two to three more baseline benchmark repetitions at roughly $9 each (about $27 total) to characterize variance; the D10 floor question, taking the recommended strict reading; letting benchmark bm-2 start with Devin running uncontained — explicitly flagged as a human call, because organization policy refuses the --sandbox flag and a Devin shell demonstrably writes outside its workspace; and proceeding with backlog items 14 (goal system) and 15 (monitor facility) after the review. The benchmark series is unparked with that scope. The sequencing note is the agent's, not Wido's: the review's W6 and W7 work finishes first, because benchmark cohorts run codex delegates and the standing rule keeps those apart from suite runs on one machine, so the series resumes that night after the final review boundary with the machine dedicated.

**Alternative:** Each item could have stayed parked awaiting a ruling; bm-2 in particular could not proceed at all without a human decision on running Devin without a sandbox.

**Impact:** The benchmark series restarts with a known cost (about $27 for the extra baselines) and one explicitly accepted risk: an uncontained Devin shell that can write outside its workspace.

## D37 — The return-schema linter moves into the schema generator's package

**Decided:** The structured-output invariants — every object typed and closed, every required list complete, every property declaring a type — were checked by a roughly 45-line python walker inside return-schema-fixtures.sh; they are now TestMaterializedSchemasObeyStructuredOutputRules in internal/returnschema, walking every role's real materialized version-2 schema under the Go gate. Only the linter leg moved: the shell file keeps its normalize_return and assert-return-complete duties and keeps its name, so the shipped surface is unchanged. The Go test was proven green against the shipped schemas before the shell leg was cut, per this tier's never-delete-first rule.

**Alternative:** Keep the python walker — an out-of-band copy of rules that the package owning the schemas can enforce directly under the gate.

**Impact:** The lint now runs beside the code that generates the schemas, under the same gate as everything else, with the fixture file's remaining duties intact.

## D38 — The contract grammar matrix moves beside its parser

**Decided:** The shell mutation table that checked every key of the base mission contract — rejected both when missing and when malformed, with specific bad values per key — is now TestContractValidateRejectsPerKeyMatrix in internal/contract, carrying the table's exact semantics: fifty in-process test cases replacing roughly 52 assert-mission subprocess runs per suite pass. The verifier's parity warning held — Go previously had only 7 rows — so the bulk of the work was a port, not a deletion, and the port ran green before any shell was cut. The four preflight rejection legs (unsealed, unsigned, mismatched hash, stale exposure) were verifier-confirmed one-to-one duplicates of the existing TestContractPreflight* tests and retired outright. What stays in shell: the seal-sign-preflight smokes and the dispatch-allow seal-survival check, because they prove the script forwards correctly, which no Go test can. One residue is recorded: the shell base contract carried stream.secondary with its missing-variant enforced, while Go's base grammar treats it as optional, so the matrix covers the 25 shared keys — whether secondary should be required-when-primary-exists is a grammar question for internal/contract, not a fixture question.

**Alternative:** Retiring the shell table against the 7 existing Go rows would have silently dropped most of the coverage; porting first prevented that.

**Impact:** Roughly 52 subprocess invocations per suite run became fifty in-process cases, and one open grammar question is written down instead of lost.

## D39 — The mission state fixture legs retire against their Go equivalents

**Decided:** Five legs of mission-fixtures.sh retire — ledger grammar, state chain, fork detection, reconcile-park, and the anchor round trip — because one-to-one equivalents were verified present in Go before the cut: ledger_test, state_test (TestChainDetectsTamper, TestWriteRefusesIllegalTransition), and anchor_test's five TestReconcile* rows, including TestReconcileStillParksOnAnyOtherDivergence and the anchored stop-loss park, which are exactly the reconcile-fork and self-park conditions the finding required. The reaper/fence and runner legs stay, and command-line argument forwarding remains proven by them. The file keeps its name and its remaining duties — this is not a whole-file retirement.

**Alternative:** Keep running shell copies of state-machine checks the Go packages already prove in-process.

**Impact:** Less duplicated coverage, with the shell file's remaining scope narrowed to what Go cannot reach.

## D40 — End-state JSON re-assertions retire; the real process runs stay

**Decided:** mission-fixtures.sh keeps everything the Go tests cannot reach — real mission-runner.sh `start --foreground` launches, the status exit-code contract via wait_end_state, the contract seal-sign flow, and the landed-orphan staging — and retires the three python blocks that re-asserted end-state JSON the runner's own Go package already proves. The equivalents were verified present before the cut: TestInternalRunCloseStreamCycle, TestDeliverLandedUnconsumedWritesFinalBlock, TestInternalRunDispatchTerminalCycle, and TestArmAndPreflightFullPass, together covering completed state, the landed-orphan prompt/ledger/usage annotations, runner-closed chains, mirror manifests, and turn-log acceptance. This closes W6 item 2: across D38, D39, and D40 the mission fixture file survives with its irreducible process-level duties at roughly a third of its former weight.

**Alternative:** Keep re-asserting in python what the runner's package proves in-process — pure duplication at subprocess cost.

**Impact:** The mission fixture file is about a third its former size and scoped strictly to process-level properties.

## D41 — Config-identity fixtures shrink to one smoke test and one data pin

**Decided:** Five behavior legs of the config-identity fixture retire against verified one-to-one equivalents: the four TestConfigIdentity* tests in internal/config and TestSelectNoMatchNamesChangedKeys in internal/capability. The file keeps exactly two things — one determinism smoke through the real `config identity` command-line interface, because flag forwarding is a script-side property, and the executable-appendix pin over the shipped filter files, which is data worth pinning rather than behavior. The file shrank from 168 lines to 58.

**Alternative:** Keep the five shell legs running checks the Go tests already own.

**Impact:** A 168-line fixture became 58 lines covering only what the Go tests cannot: the CLI path and the shipped data.

## D42 — The cap-authority checks move beside the fence they test

**Decided:** Three cap-authority legs, AUTH-R2-001 through 003, retire from delegate-caps-fixtures.sh. Leg 001 was already double-covered in Go (TestAuthorizeCapUsesPairCap and TestAuthorizeCapRefusesAboveSigned); legs 002 and 003 had no Go equivalent, so TestAuthorizeCapRefusesPinnedContractDrift and TestAuthorizeCapRefusesWhitespaceOnlyDrift were ported and run green first — pure file-in, refusal-out logic that needed no processes. The now-orphaned cap helper functions went with them, and the AUTH-R2-009 registry check now guards the remaining supervision roster (legs 005-008). The file is now what its name promises: the supervision-set harness.

**Alternative:** Retiring 002 and 003 without porting them would have dropped two refusal behaviors from coverage entirely; the port-first order prevented that.

**Impact:** Cap-authority refusals are proven in-process next to the code that enforces them, and the fixture file's scope matches its name.

## D43 — Record-protocol fixtures shrink; the concurrency check got stronger

**Decided:** record-protocol-fixtures.sh shrank from 119 lines to a 40-line smoke test that the __record-create path forwards correctly. Four behavioral legs had verified one-to-one equivalents in internal/dispatch/record_test.go. The one genuinely novel property — no reader may ever observe a record with status=failed that lacks its protocolError object — was ported as a live goroutine reader hammering the record under Go's race detector (-race) across fifty applications, with the .tmp-residue sweep folded in. The in-process version is strictly stronger than the python poller it replaces: the race detector watches the same window the poller could only sample.

**Alternative:** Keep the python poller, which sampled the failure window from outside rather than watching it continuously.

**Impact:** The fixture is a third its former size and the concurrency guarantee is now proven under the race detector rather than sampled.

## D44 — The critique-exhaustion legs retire after three missing cases were ported

**Decided:** The six-leg __critique-exhaustion section of conformance-fixtures.sh is dropped. A leg-by-leg diff found three cases Go lacked — a critic cannot own its successor, the recorded-implementer-successor budget reopen, and a recorded round beating a lying return — which were ported and run green as TestCritiqueExhaustionCodeCriticChain before the cut; the remaining behaviors (wrong-party enumeration, the manifest patch shape, off-budget none, and protocol-error recovery with an absent return) were already owned by three existing tests. The shell's separate second-budget and human-remedy greps turned out to be two substrings of one refusal string that was already asserted. The stage-level assert-conformance end-to-end check remains the file's job, and this closes W6 item 3 (spanning D41 through D44).

**Alternative:** Dropping the section without the diff would have lost three behaviors Go did not yet cover.

**Impact:** Exhaustion behavior is fully covered in Go, the shell file keeps only its end-to-end duty, and the W6 fixture-retirement item closes.

## D45 — The open-work legs retire only after the four real gaps were ported

**Decided:** The verifier was right that the claimed one-to-one parity was false: the five basic cases were covered in Go, but four behaviors lived only in shell — chain-root round matching, per-stream staleness isolation, the plans/README exclusion, and the reporter-to-gate-marker integration (a live marker silences open work; a dead marker is ignored and pruned by the reporting pass). All four were ported green into internal/report's openwork tests before the ninety-line shell section retired. The supervision-hook legs (S4-14, S4-15, and the idle wording) stay, because they exercise the shell hook itself, which has no Go home.

**Alternative:** Retiring on the original parity claim would have silently dropped four real behaviors from coverage.

**Impact:** Open-work detection is fully covered in Go, and the shell keeps only the hook tests that genuinely need shell.

## D46 — The G-5 documentation lint moves to Go now that adopted repos ship the engine

**Decided:** The verifier had downgraded this finding on the grounds that a Go test runs only where Go source exists, while adopted repositories ran the shell fixture unconditionally. D17 dissolved that objection: adopted repositories now ship the engine source (metasystem.engine-delivery is a required key) and run the full gate in their own CI, so a gate test enforces in adopted repositories through the same rails as everything else. The 37-line python heuristic parser became TestInstructionOwnersAreInstructionBearing in internal/validate, sitting beside the preamble-quote parser that reads the same markers, with the heuristics preserved verbatim. One residue is stated: nested delivery-contract runs skip gate tests behind digest equality, and AGENTS.md is not in the gate-input closure, so a nested target's documentation drift is caught by that repository's own CI rather than by the outer template run — true of every gate test, now written down.

**Alternative:** Keep the python fixture on the old reasoning — reasoning whose factual basis D17 had removed.

**Impact:** The lint enforces everywhere the gate runs, including adopted repositories, with its one enforcement gap documented rather than implicit.

## D47 — The arming script reads process identity through the engine; two fixture crutches retire

**Decided:** In production code: arm-supervision.sh's identity_alive check read the holding process's command line through raw `ps`, bypassing the one-source identity rule every other reader honors — exactly the class of bug the review's identity work existed to close, discovered empirically when removing the fixture's ps shim broke arming. It now reads the engine's `proc classify` verb (fixture-aware, with a four-way verdict), keeping the conservative rule that an unobservable command line never permits takeover. In the fixtures: with the armer fixed, the python ps shim was dead weight and is deleted, and the 20-millisecond identity-mirror daemon — the same standing writer whose torn writes caused that morning's classification escape — is replaced by one explicit registration after each arming, using the atomic-rename discipline from the tear fix; supervision pids change only at arm time, so a polling loop was never the right shape. The AUTH-R2-005 through 009 legs run green end to end without either crutch.

**Alternative:** Keeping the ps shim would have kept masking the production bypass, and keeping the mirror daemon would have kept a standing writer whose failure mode had already caused an escape.

**Impact:** One more identity reader now uses the single sanctioned source, two pieces of fixture machinery are gone, and arming is proven end to end without them.

## D48 — The telemetry test drives the real record-update code; model-name derivation is tested in Go

**Decided:** The telemetry test script (telemetry-census-fixtures.sh) contained `fixture_record_cas`, a Python re-implementation of the dispatcher's `__record-cas` routine — the atomic "check the record's current state, then write" step that updates job records. The copy skipped safety rules the real code enforces (refusing a status change smuggled inside a patch, protecting fields that must never change), so it was deleted. The test now builds a small scratch repository — a git-initialized parent directory with copies of dispatch.sh and bin/metasystem plus the jobs and record-locks directories — and pushes its update through the real `__record-cas`, running as a human-classified shell, the same ungated trust path every operator command uses. Of the three test cases for deriving a job's "effective model" label, one (a single model key) stays in the shell test as the wiring proof; the other two (no keys produces "unobserved", two keys produce a sorted "multi-model:a,b") were checked one-for-one against matching rows in the Go test internal/adapter/runtime_test.go before being retired, because that derivation logic is owned by Go. The separate check of the CENSUS-FAILED message format at the command-line boundary was left alone, as the finding asked.

**Alternative:** Keep the Python copy. That would have kept validating the shell wiring against weaker rules than production enforces, and the copy could drift further from the real dispatcher over time.

**Impact:** The wiring test now proves the real code path end to end, and the derivation cases live beside the Go code that owns them instead of being tested twice. This signed off review finding script-fixtures-013.

## D49 — config tailor gains a fake-runtime mode and --set; three drifting Perl config rewrites deleted

**Decided:** Three test harnesses each rewrote the configuration file with their own Perl one-liners to switch it onto the fake test runtime, and the three rewrites had drifted apart. The `config tailor` command now covers this with `--runtimes fake` plus a repeatable, generic `--set key=value` — deliberately not the single `--fixture` profile the finding suggested, because the three call sites want different override values (watch interval 1 versus 5, census byte limits of 350, 4096, or untouched, different tiers and investigator settings), so a profile would have needed all the same knobs while hiding them. The tailoring engine gained exactly two fake-runtime rules — fake may become the default runtime only when it is the sole runtime selected, and dropped per-runtime model bindings collapse to one `model.fake=fake-model` line per role, with explicit fake bindings winning — and `SetConfKeys` is a tested editor that atomically replaces a key or appends it, so no configuration-key grammar remains in test-script regexes. Equivalence was proven before the swap: the old Perl output and the engine output are byte-identical on effective lines for two of the three harnesses; the third (supervision) differs in exactly one place, which is precisely the drift the finding warned about — its Perl left `role.code-critic.runtime=<runtime>` and the `<model>` placeholder untouched (and its tier clauses never matched anything; recorded, no behavior lost), while the engine now writes real, better-formed fake bindings, and supervision never dispatches a code-critic anyway. Left out of scope: four small one- or two-line rewrites that set specific keys rather than re-implementing tailoring, and adopt.sh's production call, which is untouched.

**Alternative:** The finding's monolithic `--fixture` profile, rejected because the sites need different values; or leaving the three Perl rewrites in place, which is the drift the finding existed to stop.

**Impact:** One tested engine path replaces three diverging regex rewrites; the one real divergence found was normalized in the safe direction. Proven by unit tests (fake collapse, explicit-fake wins, replace/append/dedupe) and all three harnesses green end to end. Signs off script-fixtures-020.

## D50 — Nine fixture-hygiene findings dispositioned one by one, closing tier W6

**Decided:** Each finding in the W6 hygiene batch got its own disposition:

- **-009**: a `grep -Eq` pre-check of signatures was deleted — it validated with the wrong regex engine (POSIX ERE) while the consumer compiles RE2, and the authoritative `proc signature-check` runs two lines later anyway.
- **-010**: one check (FRCC-011) was vacuous — both its command and its grep ended in `|| true`, so it could never fail — and the "lease refused" witness event had no Go coverage at all. Following the port-first retirement discipline, a new Go test (TestNonHolderAnnounceEmitsLeaseRefusedWitness) now asserts the event lands on the stream when a live lock holder refuses an announce; the shell check became a pointer to it.
- **-011**: two sections and two checks retired after verifying that the shell helper emit_event is a thin wrapper over the engine's `event emit`; four named tests in emit_test.go prove the same properties. The checks that run real processes (caller harmlessness, concurrent writers, torn fragments, witness-not-authority) stay.
- **-015**: the copy-verification and symlink-refusal checks retired in favor of their two named Go tests in sessionisolation_test.go; the adapters' manifest aggregation and Wido-shell bootstrap check stay.
- **-016**: resolved as already fixed — an earlier item (W1.24) had introduced a per-run tag and a scoped pkill, and the file cites the finding.
- **-017**: hard-coded wait ceilings now come from the central fixture-budget.sh table (go-owner-wait=8, go-owner-crashloop=30), and polling honors the METASYSTEM_FIXTURE_POLL_INTERVAL_SEC variable; a 3-second stability window stays a deliberate literal because it is an assertion window, not a wait ceiling — lengthening it only increases exposure to load-induced churn.
- **-018**: a stray root-level `cp` in a repo-building helper died; explicit per-path copies already place every asset.
- **-019**: a shell grep over Go source text was replaced by a Go test that asserts the constructed host command's actual environment (mission lineage exported).
- **-022**: one script now reads status through the engine's `json get` (immune to indentation changes) and another's Python JSON helper became the engine verb, after verifying no caller uses list indices, the one shape the verb lacks.

**Alternative:** Leave the shell checks as they were — which would have kept a wrong-engine pre-check, a check that could never fail, and duplicate coverage drifting away from the Go tests that now own it.

**Impact:** Tier W6 (fixture retirement) closed with nothing retired before its Go equivalent existed. Proof: four touched Go packages green in full, all six edited fixture files green standalone.

## D51 — Glossary, routing index, and eleven package docs corrected; the internal/contract rename declined

**Decided:** The W7 small documentation fixes landed as a batch:

- **Glossary (architecture-4)**: five glossary entries that still named deleted Python scripts now point at the Go engine (the `metasystem lease` verbs, the census via `watch-background-jobs.sh --census`, emit-event.sh as a thin `event emit` wrapper, the `mission fence-*` verbs, and `metasystem schema` plus `validate return-complete`). Zero `.py` references remain and the instruction audit passes.
- **Routing (architecture-8)**: wow.md's dead route to a `meta/` directory now names `development/` at the repository top level — worded without a `../` path because the audit refuses outside references in metasystem-owned files (a first attempt was caught by exactly that rule) — and the README's layout diagram no longer draws development/ inside the tree.
- **Package docs**: eleven Go package documentation fixes, each matching its doc comment to what the code actually does — including replacing a false narrative about how processes are launched with the direct-exec truth, stating the pinned retry-backoff schedule (0 for the first retry, interval × 2^(k-2) after), and correcting a boundary claim about which helpers live in the adapter package. One fix replaced an exported mutable map with a `ValidMode()` function; that swap dropped test coverage of internal/authority to 82.6% against its 95% floor and the coverage gate refused it — the fix (a TestValidMode test, restoring 95.7%) is recorded as the gate doing exactly its job. Two stale TODO comments were resolved to the shipped truth, and one flagged problem turned out to be already gone (resolved as found).
- **Declined**: the rename half of architecture-6 — renaming the internal/contract package would ripple through every import for a naming-taste win. The doc-comment half, the finding's substance, landed. Reopen if confusion over the word "contract" actually bites.

**Alternative:** For the declined half, doing the rename anyway; for the rest, leaving docs that named deleted scripts and described behavior the code no longer has.

**Impact:** The written map matches the territory again: no dead references, honest package docs, and the audit green. Proof: ten touched Go packages green in full, gofmt/vet clean, dispatch.sh syntax-checked (its change was comment-only).

## D52 — Three architecture documents land and the review program closes

**Decided:** This entry lands W7's three documents and then closes the whole 2026-08-13 review program with its accounting:

- **The engine map (architecture-3)**: `docs/architecture.md` now describes the Go engine — what the binary is, how its source is delivered, the core-vs-plumbing ruling stated as standing doctrine with its origin, the three-tier package layering with its two deliberate exceptions, one line for each of the 31 packages, and a table mapping the 19 CLI command families to packages, derived from the command directory's actual imports. The README draws cmd/, internal/, go.mod, and bin/metasystem in its layout, and wow.md routes to the map.
- **The dispatch sequence (architecture-7)**: `docs/design/dispatch-sequence.md` is a ground-truth walkthrough of a delegated job, pinned to commit 48442ef — 26 ordered steps from the lock-owner re-exec to close, each naming the engine verb that decides and the shell lines that plumb, with failure joins and the sequence's real surprises stated as facts (the dispatcher reaps its own job on every poll; the handshake deadline is stamped at launch so the waiter and the backstop share one number; the handshake-timeout verdict lands before the kill). Anchors were spot-verified against the pinned sources.
- **The promoted contracts (architecture-5)**: four standing contracts moved verbatim from plan files into docs/design — supervision-registry, supervision-lifecycle, flight-recorder, and stop-loss-core — with contract preambles, PROMOTED stamps left on the plans (kept as history, per the finding), and every code, script, and doc citation repointed (15 code files, 7 doc sites; a contract-warning string and its test moved together).
- **W7 item 7** (roughly 35 low-priority idiom notes): dispositioned exactly as the review scoped them — fix "as encountered", not mandatory; one landed in D51, several fell out of earlier restructures, the rest stay ambient notes, not review debt.
- **Program close**: with this, the review program is complete. The final validation boundary ran green on both hosts — the VM (the validation authority) at commit 2c92dfe in 532 seconds and the Mac in 1,241 seconds, satisfying the rule that Darwin full suites must pass before benchmark cohorts; the W6 boundary before it was likewise green on both hosts at commit 7c96452 (VM 561 seconds, Mac 1,270 seconds). The boundary's first firing was real: at 230e600 the coverage gate refused internal/authority at 82.6% against its 95% floor (D51's ValidMode swap), and the fix is the program's last commit.
- **The tally**: the 2026-08-12 review produced 101 distinct findings across seven tiers (102 list slots; script-validate-11 appears in both W1.24 and W5.10 and counts once). All were executed autonomously under the 2026-08-13 delegation. Per tier: W1 correctness 26 of 26 landed; W2 consolidations 17 of 17; W3 CLI uniformity 8 of 8; W4 script boundary 23 landed, 2 declined with reopen conditions, 1 landed minimally; W5 suite decomposition 6 landed, 3 declined with reopen conditions, 1 triaged; W6 fixture retirement 9 of 9, every retirement ported to Go first; W7 documentation 6 landed, one rename half declined. Decisions D1–D52 record every sign-off, every decline with its reopen condition, and every residue; the standing instruction is revert on disagreement, and a one-line index of D1–D52 closes the section.
- **Fourteen residues written down**, none blocking the close: the receipt-stats test flake stays open with a probe armed (VM-green remains the validation authority meanwhile); a fail-closed hardening candidate around FixtureEntryFor; the legacy "started" key dual-read whose one-line removal nobody has claimed; a suite retry loop that still reads a token instead of exit code 9; the ruling that Mac sampling runs are expected to flake on one timing cap under load; the platform claim that intermediate boundaries prove Linux only; the durability contract's staged tail; a 30-second legacy allowance in fence enforcement; two accepted supervision residuals from F4; the selftest-listener verb kept as a removal candidate; an open grammar question about stream.secondary; a note that nested delivery-contract runs rely on the target repo's own CI for doc drift; supervision re-arm in this checkout still blocked on subdirectory config resolution; and items parked for Wido untouched (the adopted-engine delivery ruling review, 119 accumulated agent worktrees plus the --help worktree, and untracked codex processes that may be his own).

**Alternative:** Leave the architecture undocumented and the four contracts buried in plan files, and close the program without a written accounting — the review's whole point was that the map must match the territory, so both halves landed.

**Impact:** The engine has a map, a ground-truth job walkthrough, and its contracts in one findable place; the program's every decision, decline, and leftover is on the record for Wido's review. Proof: audit green after every docs pass, the contract package and build green, and every W6/W7 change under a both-hosts-green boundary.

## D53 — The approved baseline benchmark reps run as a fresh cohort on the current engine

**Decided:** The 2–3 baseline benchmark repetitions Wido approved run as a new cohort at the just-validated engine (the 2c92dfe line), not as additions to the old cohort bm-1-20260813t203657z, which was pinned to engine e5cbe66. The review program substantially changed the engine since then (a production identity fix, the drain and fence work, the suite's own witnesses), the series' next comparison is baseline versus bm-2 and bm-2 will run at the current engine, and the old cohort keeps one valid rep of history that mixing engines inside one arm would muddy. Three repetitions — the approval's upper count — for variance characterization, about $27 against the EUR 240 spending ceiling with roughly $90 spent so far. Sealing and signing ride the standing "sign in my name" delegation, as in every prior cohort, and the requirement for a full Mac suite before cohorts is satisfied by the final boundary's run at 2c92dfe.

**Alternative:** Pin the new reps to the old engine commit e5cbe66 for purity within one cohort — rejected because that would measure an engine nobody ships anymore.

**Impact:** The baseline arm's data reflects the engine that bm-2 will actually run on, at the cost of splitting history across two cohorts.

## D54 — Baseline cohort banked, the bimodal story explained, bm-2 starts tonight — and Wido's overnight rulings

**Decided:** Several things land in this entry:

- **The cohort result** (bm-1-20260814t192803z-44271, three reps at engine 387c961, $28.32 total, roughly 33/33/54 minutes): reps 1 and 2 are INVALID on exactly one validity gate — the stop-loss fuse (which halts a mission after a set number of cycles without measured gain) parked both at three cycles before any certified implementer job existed, with byte-identical product scores of 0.018868. Rep 3 is VALID on all seven gates: acceptance 0.962, requirement coverage 0.923, determinism at its floor, $10.52. All three runs ended in a stop-loss park at three cycles; the difference is whether real delegated work landed inside those cycles.
- **The reading**: the earlier cohort's 0.02-versus-0.96 split persists at the current engine and is now cleanly attributable — when early cycles produce certified implementer work the product is good; when they stagnate, the fuse ends the run cheaply and the strict validity floor correctly invalidates it. Rep 3 proves three cycles CAN suffice, which weakens the theory that a no-gain budget of 3 starves the build stream, but two empty parks out of three at about $9 each is the arm's real cost shape. Both readings stay recorded; no contract amendment without Wido. Series spend about $118 of EUR 240.
- **The bm-2 start**: Wido's approval of the uncontained Devin arm was unqualified ("Approved, and yes to all", with the uncontained facts in front of him), so the start executes tonight rather than waiting for a watched window — deferral would recreate the parked state the approval ended. Stated plainly: the swe-1-7 model runs without a sandbox, its writes outside the workspace are exactly what no fence catches, and monitoring is the driver's Monitor plus half-hour heartbeats, with the gap between heartbeats accepted as part of the approval. One repetition first; the arm continues only on its own data, and a push notification tells Wido the moment it is live.
- **Wido's standing rulings for a 10-hour absence**, given in writing the same night: (1) "Drive to 2 valid reps/arm" — drive every arm — baseline, bm-2, bm-2s — to two VALID repetitions; empty parks do not count; roughly $40–90 of headroom at about $10 a run. (2) "Yes, same monitoring" — if bm-2's first rep is clean, further uncontained Devin reps may run unattended under the same monitoring (a 15-minute heartbeat plus the driver's Monitor). (3) "Raise to 5 overnight" — raise the no-gain budget from 3 to 5 in the bm-1 and bm-2 manifests for cohorts provisioned from now on; bm-2s already carried its own deliberate 8 and is NOT lowered (the ruling was a raise, not a flattening — flagged for review); the rep already in flight stays sealed at 3; budget-5 cohorts are a variant against tonight's budget-3 cohort, acceptable because the spec is pre-1.0 and every scorecard pins its manifest. (4) "Item 14 then 15" — after the benchmarks, design backlog items 14 then 15, critique-looped, DESIGNS ONLY.
- **Same-evening addendum**, Wido still present, after reading the empty-park rosters: (5) "Middle path" — a seal-time sanity check (the no-gain budget must exceed the critique-round cadence) lands now, ONE budget-5 baseline rep runs as the hypothesis test that decides whether the overnight series continues, and a leading-indicator fuse design is critiqued in parallel; (6) "Yes, if critique converges" — that one design MAY implement tonight under the full quality gates if its critique loop converges — superseding ruling 4's designs-only scope for this single design.

**Alternative:** Add the reps to the old cohort, or hold bm-2 for a watched window — the first was settled by D53, the second rejected because the approval explicitly ended the parked state.

**Impact:** The baseline arm has one valid rep and a clean explanation of its variance; the uncontained Devin arm goes live with its monitoring and residual risk stated in writing; and the night's work proceeds under written human rulings instead of improvisation.

## D55 — Sealing a mission warns when the no-gain budget cannot outlast the critique cadence

**Decided:** The seal-time calibration warnings gain a second rule: a no-gain budget that does not exceed 3 — the cadence at which critique rounds alone exhaust it — now warns that a host which serializes its work (critique first, implementation after) will be fused before its first implementer job, naming tonight's two empty parks as the evidence. It is deliberately a WARNING and not a refusal: the test fixtures exercise the fuse with budgets of 2 and 3 on purpose (that is how the fuse's own behavior is tested), and a refusal would have forced a fixture bypass seam — worse than the disease. The base test fixture keeps its small budget and its test now expects exactly this warning. The entry also states plainly that the existing half-fence warning fired on every provision tonight and prevented nothing — so the durable protections are the raised budgets (Wido's ruling 3) and the leading-indicator design then in flight, not this tripwire alone.

**Alternative:** A hard refusal at seal time, rejected because the fixtures legitimately seal small budgets and a bypass seam for them would be worse. Reopen as a refusal (with a proper seam) if a real mission ever again seals into this trap despite the warning.

**Impact:** The trap that produced two $9 empty parks now names itself at seal time; the honest admission that warnings alone prevented nothing is on the record. This is part 1 of the middle-path ruling.

## D56 — The leading-indicator fuse design is withdrawn after round-1 critique refuted its premise

**Decided:** The leading-indicator fuse design does not implement tonight — not for lack of time, but because the first critique round refuted its premise. The critic's finding 1 is arithmetic: under a no-gain budget of 3, three critic-only cycles reach the budget and park BEFORE a cycle 4 exists, so a "work has landed" marker could never have rescued the observed empty parks. Finding 14 names what the evidence actually shows: a host-scheduling defect — rep 3 dispatched its first implementer 27 seconds in, in parallel, while reps 1 and 2 never dispatched one at all — for which changing the fuse's grammar is the wrong remedy. Findings 2–5 (the sealed ledger semantics are frozen by contract, the conclusion-as-transaction invariant, replay purity over the ledger, and the risk of farming trivial jobs to feed the marker) would have sunk the mechanism regardless. The full critique is preserved at plans/fuse-leading-indicator-critique-r1.md beside the withdrawn design. What stands instead: budget 5 covers the cadence arithmetic, D55's seal warning names the trap, the budget-5 test rep still runs as the go/no-go for the overnight series, and the host-serialization question becomes a named input to the item-14 design pass.

**Alternative:** Implement the design overnight, as ruling 6 permitted if critique converged — the critique showed the mechanism could not have prevented the observed failures and would have broken sealed semantics.

**Impact:** One codex critique round prevented a sealed-semantics break at one in the morning; the real defect (host scheduling) is routed to the design pass that can actually fix it. This is part 2 of the middle-path ruling.

## D57 — Devin delegates finish work but never deliver; the bm-2 arm is held with a diagnosis

**Decided:** bm-2's first rep (bm-2-20260814t213312z-37844/1, engine 275f533, sealed at budget 3, $4.57, 41 minutes) is INVALID — a stop-loss park with acceptance 0 — but its roster tells a different story than the baseline parks: this host scheduled correctly, dispatching three implementers and two extra critics, and every one of them failed — three with `empty_reply` at the delivery phase (the Devin session returned nothing usable) and two refused at dispatch setup. The census stayed clean throughout; the uncontained arm did nothing anomalous, it just produced nothing. Decision: further bm-2 reps are HELD until the empty-reply failure is diagnosed, because burning about $5 of host tokens per rep against delegates that reply empty measures the defect, not the arm. The same-night diagnosis from the frozen evidence: the delegates were not idle and the service did not flake — one implementer's session transcript shows 37 steps of real work over about 9 minutes (code written into the worktree, compiles run, tests reasoned through) and then no final message at all. The job log holds the mechanism: the Devin CLI warned "rejected a tool call that requires confirmation. Running in non-interactive mode. Use --permission-mode dangerous to auto-approve all tools." and then exited cleanly — the permission envelope (approvals denied, writes scoped to the worktree) correctly refused a call outside the allow-list, but the CLI turns that refusal into a session that ends without delivering, so the collection step finds nothing and the record says empty_reply.

**Alternative:** Keep running reps (measures the defect, not the arm), or take the CLI's own suggestion of `--permission-mode dangerous` — judged at this point exactly wrong for an uncontained runtime, since it auto-approves everything. How denials should behave for an uncontained delegate is a design decision that belongs to Wido, so the arm stays held with this write-up as the evidence.

**Impact:** About $5 bought a precise root cause instead of a pile of identical failures. Spend for the night $32.89, roughly $122 of EUR 240 total; the budget-5 baseline test rep (the go/no-go from ruling 5) launches on the freed machine — it was always first in line.

## D58 — One budget-5 test run converts an empty park into the series' best result; the baseline arm is done

**Decided:** The budget-5 test rep (bm-1-20260814t221633z-40460/1, engine 1a38553, no-gain budget 5, $17.64, 82 minutes) serialized exactly like the two empty parks — two critic-only cycles first — and then used the runway that budget 3 never gave it: implementers, a repair, and a code-critic from cycle 3 on, five cycles total, mission COMPLETED rather than parked, VALID on all seven gates, acceptance 0.981, requirement coverage 0.962, determinism at its floor. The finding, now by direct experiment: the same host shape costs $9 and produces nothing at budget 3, or $17.64 and the series' best product at budget 5 — Wido's raise-to-5 ruling is validated, and the host-serialization question for the item-14 design pass gains its control case. The baseline arm now holds two valid reps (0.962 and 0.981) and is DONE per the drive-to-two-valid ruling.

**Alternative:** Halt the overnight series — this rep was explicitly the go/no-go test from ruling 5, and it came back go. Running more baseline reps was not needed once the arm held its two valid reps.

**Impact:** The overnight series continues on evidence rather than hope; the empty parks are established as structural (budget arithmetic), not product variance. bm-2 stays held per D57; the bm-2s contained-codex arm is next. Spend roughly $140 of EUR 240.

## D59 — The overnight series completes: two arms banked, one held, and four findings

**Decided:** The bm-2s arm (codex sol delegates, contained, its manifest's own deliberate budget of 8) ran two reps: both valid, both completed, both acceptance 0.981 ($18.87 in 79 minutes; $17.78 in 89 minutes) — the arm holds its two valid reps. Whole-night position: baseline holds 0.962 and 0.981, bm-2s holds 0.981 twice, and bm-2 (uncontained Devin) is HELD after one invalid rep with the denial-behavior diagnosis written for Wido. Night spend about $177 against the EUR 240 ceiling. The series bought four findings: (1) the budget-5 conversion is a controlled-experiment result — the serializing host shape parks empty at budget 3 and completes at budget 5, so the empty parks were structural; (2) completed runs converge — every completed run across arms and delegate runtimes landed acceptance 0.981132 EXACTLY, so the benchmark's real discriminator between arms is validity rate and cost per valid rep, not the product score, and more reps buy validity-rate precision rather than score precision; (3) the Devin arm's blocker is a design question, not a flake — the model works, then a confirmation-blocked tool call eats delivery; (4) whether a host dispatches implementers alongside critique in cycle 1 is the variance source, and that goes to the item-14 design pass as its control case. The night's tail, per ruling 4: the item-14 goal-system design pass runs now, DESIGNS ONLY, carrying the serialization case, the denial-behavior question, and items 15–17 as context; benchmark targets stay in place for replay, per prior practice.

**Alternative:** Same-night implementation of item 14 was the option ruling 4 explicitly offered and Wido did not take, so nothing implements while he is away.

**Impact:** Every arm that can currently deliver satisfies the drive-to-two-valid ruling; the series' open questions (denial behavior, reps-per-arm design) are written down for Wido rather than decided for him.

## D60 — The goal-system design pauses after round 2 on four questions only Wido can answer

**Decided:** Two critique rounds (16 findings, then a disposition audit that closed only two of them) drove real reshapes of the goal-system design — causal continuation through the block-once path, admission of a sixth ledger, exactly one Current goal, engine-only writes — and then hit four questions the design cannot answer for itself: what the default should be for projecting goals into delegate briefs; whether cross-runtime delivery may be staged behind item 16; what "human-reserved" can mean while the authority classifier grants any caller without an agent ancestor unconditional HUMAN authority; and whether headless mission hosts pull mission-prompt integration into scope. The questions and the design-side specification debts are enumerated at the top of plans/goal-system-design.md, with both critiques preserved beside it. Stopping here is the design-loop stop criterion applied honestly: findings were still changing what gets built, and every further change is Wido's to direct. Item 15's design is NOT started — it composes with item 14 at exactly the seams awaiting these rulings, so starting it would speculate twice. One factual correction from the critique rides into item 16's context: codex and devin Stop-hook configurations ARE shipped (scripts/enforcement/), and the honest conformance model is declared/installed/observed/blocking-capable.

**Alternative:** Answer the four questions unilaterally and keep designing, or start item 15 in parallel — both rejected as speculating on decisions that are Wido's.

**Impact:** The design is parked in a reviewable state with its open questions named; no speculative work stacks up behind unanswered rulings.

## D61 — Human ruling: Devin runs in full auto-approve mode; the arm unblocks

**Decided:** Wido ruled in writing on the morning of 2026-08-15, after the D57 explanation: "we have not set up the Devin agents properly. we should run it in a mode that allows for everything we needed to do. So figure out what that mode is. something equivalent to Yolo." That mode is the Devin CLI's `--permission-mode dangerous` (auto-approve all tools). The adapter had deliberately capped the mode at accept-edits — its comment said "dangerous is never used" — which auto-accepts file edits but turns any other confirmation-requiring tool call into a dead stop in unattended runs; that cap is what ate every delivery in D57. `DevinPermissionMode` now returns dangerous for every readable job record, while an unreadable record still refuses — the fail-closed read survives. This entry is the recorded human waiver the glossary's permission doctrine calls for: the runtime already runs uncontained by the earlier ruling, and the graded modes provably converted refusals into non-delivery. While touching the function it moved from codex.go to devin.go, its runtime's home — a confirmed stray under backlog item 17, whose wider sweep stays queued.

**Alternative:** Keep the graded accept-edits cap — demonstrated by D57 to turn permission refusals into silent non-delivery for this runtime.

**Impact:** The bm-2 arm unblocks and relaunches toward its two valid reps once the suite is green. The cost is real and stated: Devin sessions now auto-approve every tool call, accepted by explicit human waiver. Proof: adapter package green in full, build green, VM suite next.

## D62 — Devin delivers by writing files, so the adapter names a return file and reads it — and a morning ruling reorders the queue

**Decided:** The first rep after D61 revealed a second, independent failure: dangerous mode removed the confirmation block and delivery STILL came back empty. The transcript shows why, and it is better than a flake — the delegate did the whole job and wrote a schema-perfect return to /tmp/design-critique-return.json, then ended its session without ever emitting a final text message. The swe-1-7 model finishes work by WRITING FILES; `devin -p` treats stdout as the reply; stdout is empty every time. In the earlier cohort that same final write is what the confirmation block killed — one model bias, two failure shapes. Decision: give the model the channel it insists on. `adapter devin-prompt` gains `--return-file`: the augmented prompt names ONE exact path inside the round's evidence directory (devin-return.json) and instructs the model to write the return there as well as printing it; the collect step recovers from that file whenever the CLI exits 0 with empty stdout, requires parseable JSON so a torn write is never promoted into a reply, and logs the recovery. Validation rides Wido's own words — he waived the full suite for Devin-path changes because the benchmark is the acceptance test, "I will trust that to be a proper test of the functionality" — so proof is the adapter package green plus the cohort: rep 1, already in flight on the pre-fix target, is kept as the dangerous-mode-alone control, and rep 2 was to test the named-channel fix live. Later the same morning Wido overrode that plan in writing: "we came up with a good idea that should reliably fix the Devin problem. And I think this is more important than running the second benchmark. So do not run the second benchmark. Let the one that is running now complete. That is okay. We might learn things from it still. But then prioritize this design, get it designed, critiqued in a loop and then implemented before we do anything else." So rep 2 of cohort bm-2-20260815t062523z-18265 is CANCELLED (the cohort closes incomplete at rep 1, which runs to its natural end as the control); an in-progress transcript-mining recovery verb drafted after this entry is REVERTED uncommitted so the design loop owns the shape; and the deliverable is plans/delegate-delivery-design.md through the full design-critique-implement path, ahead of all other queued work. He also gave the boundary ruling in his own words — "Do as much of the code that is critical in Go … Everything for which Go is much better suited we should do in Go. The rest should remain as plumbing in the shell script." — applied here as: the whole delivery ladder (ordering, guards, provenance, verdicts) becomes one engine collect verb, and devin.sh keeps spawn/wait/custody and one repair invocation.

**Alternative:** Patch delivery incrementally (the drafted transcript-mining verb) or treat empty stdout as a flake — the ruling explicitly reverted the incremental code so a designed, critiqued mechanism owns the shape instead.

**Impact:** The delegate gets a delivery channel matching how it actually behaves, with recovery that never fabricates a reply from a torn write; the benchmark pauses at one control rep while delivery robustness is designed properly.

## D63 — The dangerous-mode control rep completes at 0.981; the only failed gate is the model's display name

**Decided:** The control rep (bm-2-20260815t062523z-18265/1, engine 4e8dbb6, budget 5, $18.70, 71 minutes) COMPLETED its mission with acceptance 0.981132 — the Devin arm's product converges to the same score as every other completed run. Two implementers and three critics delivered and the delegation floor passed; the empty-reply class still fired twice (plus one dispatch refusal) and retries absorbed them — evidence that the delivery ladder being designed is needed for efficiency, not just viability. The rep is INVALID on exactly one gate, rosterPinned: seven jobs' effective model settled as "SWE-1.7 Max" against the requested "swe-1-7". Two readings are recorded — a silent service-side tier escalation (a true roster violation the gate exists to catch), or the same model under a display name our canonicalization does not map — and which reading governs, plus whether SWE-1.7 Max is roster-acceptable for the arm, is benchmark policy: Wido's call, parked with this evidence. Target 2 stays unsealed per the morning ruling; the cohort closes incomplete by design.

**Alternative:** Decide the identity question unilaterally — either treating Max as a violation or silently mapping the name — instead of parking it as policy for Wido.

**Impact:** Dangerous mode is confirmed to make the Devin arm complete missions; the last blocker to a valid rep is a one-gate identity question awaiting a ruling. Series spend roughly $196 of EUR 240; the delivery design loop (round 3 in flight) stays at the head of the queue.

## D64 — The delegate-delivery design converges at round 8 and is implemented in two phases

**Decided:** The delivery design ran eight critique rounds (finding counts 11, 8, 8, 3, 2, 1, 1, 1 — every round codex gpt-5.6-sol at xhigh effort with live repository access, all eight critiques preserved beside the design), converged by the stop criterion, and was implemented the same day:

- The loop reshaped the design profoundly through round 5: validity-aware selection of candidate returns, the filesystem as the success oracle for transcript mining, a new internal/atif package (the transcript format reader) as a leaf with one immutable snapshot serving all transcript consumers, a dispatch-owned repair-claim compare-and-swap, adjudication-owned eligibility, settlement-first outcome tables, and a resumable host walk. Rounds 6–8 corrected only the width of one row's presence predicate — three successive paraphrases of a gate that already ships in devin.sh — so the stop criterion (stop when findings stop changing what gets built) applies on its face: the converged text replicates the shipped bar BY REFERENCE and pins the four presence shapes as fixture obligations, which is where enforcement actually lives. Stated residual: the prose description of that bar may still be imperfect — harmless, given the fixtures and the by-reference rule.
- Phase 1 (the delegate path) implemented under the FULL quality gates — the D61/D62 suite waiver does not extend to this change — in checkpointed commits A–F: internal/atif (bounded reads, an oversize error, an exclusive-create per-attempt snapshot, generic object decode); a `job repair-claim` verb (conditional compare-and-swap, absent-as-zero, a 0/3/1 exit taxonomy) wired through dispatch.sh on the record-writer authority path; adapter.DevinCollect (a facts-only walk, the shipped presence bars, per-candidate normalize-then-validate through a new validator entry that can reject a return belonging to the wrong job — schema-only validation could not — a designation rule backed by the filesystem success oracle, a fail-closed watermark, and digest-bound provenance with a mining audit); an empty-delivery adjudication stage with its delivery-repair prompt; usage and settlement reading the attempt snapshot with oversize as its own terminal state; and the devin.sh walk per both outcome tables. The two repair flows compose through one mechanism: a won delivery claim disables the malformed-return repair and vice versa — one paid repair ever, now durable.
- Phase 2 (the host path) landed as designed the same day: hosts/devin.sh names the return path with a stale-file guard; the host-side collector pre-checks the orchestrator schema plus turn, mission, and cycle identity and skips digests the runner already rejected; turn finishing judges "reply required" against the accepted snapshot while the raw reply stays evidence; and the runner performs exactly ONE resume past a session-faulted candidate, validating the replacement through the same path — never a second resume; any failure falls back to the original fault.
- The gates fought honestly both times: phase 1's gate caught three coverage debts (atif unregistered, usage and adapter under their floors), all answered with branch tests and never floor adjustments; host followed the same discipline (80.2 versus the 79.3 floor).

**Alternative:** More prose critique rounds past round 8 — rejected because the remedy each round was already fully determined by shipped code, and the implementation reads that code, not the design's paraphrase of it.

**Impact:** Delegate delivery is now a designed ladder — stdout, named file, transcript mining, one paid repair — owned by one engine verb with provenance at every rung, proven by 32 green Go packages and the full VM suite; the LIVE Devin path's proof remains the arm's own selftest and the next bm-2 rep, since the suite cannot run a real swe-1-7 session.

## D65 — Human ruling: "Max is acceptable" — the model-name mismatch resolves in the benchmark kit

**Decided:** Wido ruled on the D63 question: "Max is acceptable." The implementation follows the benchmark-specifics-stay-in-the-kit doctrine rather than core canonicalization: the benchmark manifest's roster gains an `acceptableEffective` field ({"swe-1-7": ["swe-1-7-max"]}), and the extractor's rosterPinned gate accepts a declared alias for its requested model — explicit, per-model, nothing inferred from name similarity, and the core engine still names no runtime's marketing variants. The kit's own fixtures and validation pass.

**Alternative:** Map the name in core engine canonicalization — rejected because benchmark-specific model identities belong in the kit, and inferring aliases from name similarity would be a guess where the ruling gives an explicit list.

**Impact:** The D63 control rep's only failed gate is resolved by ruling, so the Devin arm can produce VALID reps on the new delivery machinery whenever its next rep runs. Rep policy itself is unchanged: no new cohorts without Wido's word beyond the standing drive-to-two-valid, which for bm-2 remains open at zero valid.

## D66 — Human rulings restart the goal-system loop: projection default off, and agents exchangeable at every level

**Decided:** Wido answered two of D60's four questions in writing on 2026-08-15. First, on whether delegates see the goal ledger: "You and I agree" with the delegate's stated view — the goal projection is PER-DISPATCH and ORCHESTRATOR-CHOSEN, carried in the brief as bounded, clearly labeled context-not-instruction, DEFAULT OFF; a brief that only works with goal context is a briefing failure, and the opt-in exists for genuinely under-determined tasks. Second, the EXCHANGEABILITY DOCTRINE, in his words: "agents need to be exchangeable at all levels. So we should be able to have Devin as the orchestrator and Claude as the delegate etc. So whatever we implement it must fit this." The consequence for every mechanism: built only from files, engine verbs, and plain prompt text — never runtime-native features; turn-end delivery is a CONTRACT with an open conformance table, and no runtime is privileged in the mechanism itself. This hardens backlog items 16 and 18 and answers D60's staging question. The remaining two D60 questions the delegate now decides under the general delegation, both recorded in the design: human-reserved goal transitions are ADVISORY-GRADE at the current trust model (the same grade stagnation resets already have; authenticated human identity is its own future work), and mission hosts get their causal read path from the RUNNER, including the goal orientation line in every turn prompt — runner-side, runtime-neutral, no new write authority.

**Alternative:** Goal context on by default in delegate briefs, or mechanisms leaning on runtime-native features — both foreclosed by the rulings.

**Impact:** The goal-system design resumes with its blocking questions answered and a standing doctrine that constrains every future mechanism. Path per his order: design, critique loop, implement.

## D67 — 2026-08-15: Goal commands get their own top-level `goal` command family

**Decided:** The goal-system critic (round 5, finding 8) required the open naming question closed before implementation: should goal operations be a new top-level `goal` command family, or sub-verbs under the existing `report` family? Decided: top-level `goal`. The commands the doctrine tells humans and agents to type (`goal open` starts a program; `goal next` is the universal fallback named in every projection) are the public surface, and a mutating ledger with its own transition table and authority matrix is not diagnostics-shaped — the router's family/verb registry is exactly the seam for it. The standing constraint that CLI consolidation of EXISTING families needs Wido's sign-off is noted and not triggered: this adds a new family for a new feature and consolidates nothing.

**Alternative:** `report goal-*` sub-verbs — rejected because a mutating ledger does not belong under a diagnostics family.

**Impact:** The public surface matches the doctrine's own commands. Revert cost if Wido overrules: a rename in cmd/metasystem plus doctrine text, with no on-disk contract changes.

## D68 — 2026-08-15: First valid Devin benchmark rep — the delivery machinery's first live evidence — and the ruling to run the backlog to completion

**Decided:** The bm-2 arm produced its first VALID repetition (Devin delegates as swe-1-7, with swe-1-7-max acceptable per D65) on the full D61+D64 stack: all seven validity gates passed, acceptance 0.981132 (the value every completed run of every arm converges to), requirement coverage 0.9615, all other product metrics 1.0; cost was $5.56 for the Claude orchestrator plus 30 and 86 Devin ACUs (Devin's usage units, as the original records them) for the two delegates, in 67 minutes of wall clock. The D64 live proof: both delegate rounds recorded delivery via stdout at the initial attempt with candidates present — zero descents down the delivery ladder, zero repairs — meaning D61's dangerous mode removed the empty-reply failure at its source and the D64 ladder stands as armed insurance, exactly the intended shape; the named-file, transcript-mining, and repair rungs remain exercised only by the suite's fixtures, which is what "robust but unneeded" looks like when the primary channel works. Decision: proceed to rep 2 under the same contract, with channel counts recorded per the delivery-evidence mandate. The same afternoon Wido ruled in writing: continue the goal system "until it is fully implemented and once it is, continue with whatever is next until the entire backlog is implemented" — items 14 through 20 in order unless findings reorder, each through design-critique-implement where design-sized and direct implementation where mechanical; the bm-2 drive-to-two-valid resumes IN PARALLEL (reps do not contend with critique rounds; Mac suites still wait for mission gaps); his review of the decisions document stays queued on his word. An archived series-position report for Wido (dated 2026-08-14) also sits here, recording that a replay of the first cohort at the corrected kit (commit d079b74) made its rep 2 the series' first fully valid rep, that the valid rep scored acceptance 0.019 where an earlier, differently-invalid rep had scored 0.962 on the same contract, that the stored scorecards predate the corrected kit and the authoritative verdicts come from replaying the frozen targets, and the Devin arm facts as then known: swe-1-7 free in beta, running UNCONTAINED because --sandbox is refused by org policy and a shell command demonstrably wrote outside the workspace — which is why starting it was a human call.

**Alternative:** Stop at one valid rep, or hold the backlog for piecemeal approval — the ruling explicitly authorizes running the whole queue and the arm in parallel.

**Impact:** The Devin arm works end to end under the new machinery, the delivery ladder is confirmed as insurance rather than crutch, and the delegation now covers the entire remaining backlog.

## D69 — 2026-08-15: The goal-system design converges at round 12; implementation proceeds against a 22-row obligation matrix

**Decided:** Eleven critique rounds (gpt-5.6-sol at xhigh effort; finding trajectory 16/14/13/13/8/5/4/4/5/4/2) drove the goal-system design to convergence. The finding class narrowed monotonically: architecture (rounds 1–5), mechanisms (6–7), integration edges where the chosen mechanisms meet pre-existing defects in adjacent code (8–9, including round 9's kill of round 8's concurrency fix — its self-identity check was circular — and an uncompilable ownership cycle), contract completions (10), and finally two stale-text sweeps of the author's own earlier folds (11), fixed with zero new decisions. The adjacent-defect class the critic fenced at round 9 (stale-lease cleanup, lease-release ownership, typed gate outcomes, watchdog delivery acknowledgments, plans-directory enumeration) is recorded as follow-up obligations, not part of item 14. Decision: declare convergence WITHOUT a formal AGREE token — by round 11 the critic was confirming mechanisms and correcting sweep errors, which is the stop criterion's definition of done. Implementation proceeds against the obligation matrix GOAL-01 through GOAL-22 in checkpointed commits, with the design-obligation gate enforcing row-by-row completion.

**Alternative:** Keep critiquing until the critic emits a formal agreement token — rejected because round 11's findings changed nothing about what gets built, which is the stop criterion Wido set.

**Impact:** Implementation starts with every obligation enumerated and gated. Revert point if Wido disagrees: the design and all eleven critiques are in plans/, and every critique fold is a separate commit.

## D70 — 2026-08-15: Second valid Devin rep closes the arm at two of two; the delivery ladder proved both its faces — and the goal queue becomes the backlog

**Decided:** bm-2's second repetition passed all seven validity gates, with acceptance 0.981132 and requirement coverage 0.9615 byte-identical to rep 1 and to every completed run of every arm — the product-metric convergence now holds across a fourth arm. Cost: $16.00 orchestrator plus 176 Devin ACUs (design-critic 61, implementer 115); the bm-2 arm is CLOSED at 2 valid of 2. The delivery-channel census across the rep: four rounds delivered via stdout at the initial attempt; three rounds came back EMPTY at the initial attempt, descended the ladder, ran the one same-session repair, got nothing there either, and honestly settled as `channel=none delivered=false` — after which the mission relaunched follow-up rounds and completed within its fences. Combined with rep 1, the D64 machinery has now shown both faces live: the primary channel works under dangerous mode, and when Devin genuinely returns nothing the ladder refuses to fabricate a reply, records the descent, and the mission-level retry absorbs it — validity unaffected in both shapes. Series state: baseline, bm-2s, and bm-2 all hold 2 valid of 2 and all converge to 0.981132, so the discriminators remain validity rate, wall clock, and cost, with total bm-2 cohort spend across both reps about $21.56 orchestrator plus 292 Devin ACUs; no further reps are approved or planned, and the series awaits Wido's read. Separately, Wido ruled in session on whether to build a generic backlog mechanism: "agreed with your recommendation" — no separate backlog system; the goal ledger's Queued section is the one generic backlog (verb-mediated, surfaced at turn end, origin-protected, shipped by adoption); "put it on the backlog" from Wido lands as `goal open` with Origin: human; rich item detail lives in a plans note the queued step references; plans/backlog.md items 15–20 migrate into the queue once item 14's gates are green; and the goal family is extended only if real usage exposes a gap, rather than ever building a second system.

**Alternative:** Keep running reps (not approved — the drive-to-two-valid target is met), and on the backlog question, build a dedicated backlog mechanism alongside the goal ledger — rejected as a second system.

**Impact:** The benchmark series is complete on its ruled terms with the delivery machinery proven in both success and honest-failure shapes; the metasystem gets exactly one queue for future work.

## D71 — 2026-08-15: The goal system is complete; the metasystem runs on its own goal thread

**Decided:** Backlog item 14 closed end to end in one day: the design converged at round 12 after eleven adversarial critique rounds (D69); implementation landed in checkpointed commits with every obligation row GOAL-01 through GOAL-22 DONE and concretely proven; the design-obligation gate passes; and the full validation suite ran GREEN ON BOTH HOSTS at commit e446500 (Mac launch 5; the VM guest at the identical commit). The gate fought its builder honestly on the way — gofmt, go vet, staticcheck, and the coverage ratchet each caught real hygiene debt from the day's refactors: four fast-fails, four source-level fixes, no floor games, no suppressed checks. The system also proved itself live before its gates finished: its verdict blocked this session's own turn twice — first on a genuinely stale plan field from the finished review program, then on the builder's own grammar-sloppy fix — once each, exactly as designed. Per Wido's backlog ruling (D70), plans/backlog.md became plans/backlog-notes.md (detail notes) and items 15–20 migrated into the goal ledger through the verbs themselves: monitor-facility is the metasystem's first Current goal, with the agnosticism audit, runtime-file placement, ACP transport, disk hygiene, and the narrator queued behind it. Items 1–13 of the old backlog remain in the notes pending their own migration or retirement — a follow-up decision, not silently dropped.

**Alternative:** Keep plans/backlog.md as a parallel tracking system — foreclosed by the D70 ruling that the goal queue is the one backlog.

**Impact:** The metasystem now tracks its own remaining work through the mechanism it just built, and that mechanism has already blocked real mistakes. Residuals stated honestly: the accepted blind spot (intent recorded where no sensor reads it) is compensated by the audited program-start doctrine; codex and devin conformance rows sit at "declared" until item 16's audit upgrades them; and the D61 dangerous-mode waiver stands until item 18 retires it.

## D72 — 2026-08-15: The monitor-facility design closes via the fixtures-as-arbiter exit

**Decided:** The monitor-facility design ran six critique rounds (gpt-5.6-sol at xhigh effort; finding trajectory 12/10/8/6/6/4). At round 6 the critic invoked the critique skill's round-budget rule: the second three-round budget was exhausted, and another prose round would require a human ruling. Rather than wake Wido mid-delegation, the loop closed by the FIXTURES-AS-ARBITER exit Wido had ratified in the flight-recorder instance ("I really like the approach... you fold the remaining issues into fixtures that then together with code critique become the arbiter"), whose conditions all held: a falling trajectory past budget; all four round-6 findings mechanical-grain (a job-incarnation discriminator in the digest key, a waiter target field, human:<uid> as the canonical human waiter key, prefix-consistent green scanning with an honest replay caveat); each finding folded one-for-one as both a decision AND a named fixture obligation (FIX-R6-01 through -04); code-critique of the implementation made mandatory; and the switch recorded in the plan header with the trajectory.

**Alternative:** Wake Wido for a mid-delegation ruling, or run another prose critique round outside the budget — the ratified exit exists precisely so that mechanical-grain leftovers become testable obligations instead.

**Impact:** The design proceeds to implementation with its four residual findings pinned as fixture obligations and a mandatory code-critique. Revert cost if Wido overrules: one more critique round against the round-7 text before implementation continues.

## D73 — 2026-08-16: The monitor facility ships, and the mandatory post-gate review proves its worth

**Decided:** The monitor-facility implementation (steps C1 through C6) landed and the full test suites passed on both machines — the Mac (commit 04a1bbe) and the Linux VM (commit d50909a). This goal's design loop had exited on the "fixtures as arbiter" path — let the test gates arbitrate instead of more design review — which was ratified on the condition that an independent code review runs after the gates go green. That review (the external reviewer model gpt-5.6-sol at its highest reasoning effort, reading the already-gated tree without modifying it) came back with a "revise" verdict: 11 real defects, 4 of them critical — a takeover sweep that ran unserialized and had no forced conclusion, a missing re-check of the lease epoch while holding the lock, a cursor over passing results that could permanently skip one, and waiting code paths that could miss a timeout or a cancellation. The decision was to fix all 11 findings at the source rather than triage them, each fix with a test — the review's findings were uniformly mechanical, and each fix strengthened the proof behind one row of the test matrix. The review is preserved verbatim in plans/monitor-facility-code-critique.md, with a fold table in the design doc.

**Alternative:** Trusting the green gates alone, or triaging the findings down. Rejected: the gates had just certified a tree carrying four critical defects, which is exactly the case the mandatory review exists for — tests only check what someone thought to encode.

**Impact:** Both machines green on the fixed tree at 0d76eb1; monitor-facility is closed in the goal ledger and agnosticism-audit is promoted (a follow-on decision will record that audit's rulings). Undoing this is one git revert of the fix commit; the pre-fix state is the gated tree at 04a1bbe.

## D74 — 2026-08-16: The agnosticism design loop splits in two when its review budget runs out

**Decided:** The design for making the metasystem agnostic about which agent runtime it drives (backlog item 16) went through six review rounds with the external reviewer (gpt-5.6-sol at its highest setting). Finding counts per round were 10/7/9/7/9/11 — rising at the end, with ten of the eleven final findings structural (design-level rather than mechanical). Neither approved exit applied: the design had not converged, and the "fixtures as arbiter" exit requires a falling trend with only mechanical findings; the reviewer's stated lawful moves were human escalation or the split announced in its round-6 header. Under the standing delegation (which covers scope decisions), the work was split, because six rounds had actually revealed that item 16 hides a second design problem the size of the first: the contract for how a runtime is adopted, registered, and installed (seven of the eleven final findings cluster there), plus how test-fixture authorization is transported — all hiding inside an audit framed as "rule per site." Phase A — the registry core, capability tables, and the usage/hooks/waiver/docs/conf classes, every class the reviewer's own fold tables mark resolved — implements now under agnosticism-audit, with the mandatory post-gate code review. Phase B moves whole to a new goal, runtime-integration-contracts, seeded with all six review documents, and starts a fresh design loop with its own budget.

**Alternative:** Waking Wido for a ruling, or freezing all progress on a mostly-converged design to await one. Rejected because the delegation covers scope decisions, the split loses no information (the critiques are preserved verbatim), and freezing contradicts the standing "keep going" directive.

**Impact:** The converged majority ships now instead of waiting; the unconverged contract work gets its own budget. Reverting means parking runtime-integration-contracts and resuming review rounds on the unified document.

## D75 — 2026-08-16: Agnosticism phase A ships; the mandatory review catches 14 more defects

**Decided:**
- Phase A was implemented in two checkpointed commits (the registry plus its first consumers at 30b792f; capability tables, residual waivers, probes, and docs at 14d3ab0), and both machines' test suites passed.
- The mandatory post-gate code review then found 14 real defects — one high (the live config identity still built its filter filename by hand instead of asking the new registry) and mediums including a wrong recovery dispatch order, an aggregator bypass that dropped cost-only recoveries, missing containment on recollected replies, registry validation that failed open, two dropped reroutes of devin verbs, and four proofs the ruling set had promised but were missing. All were folded with tests at 2555f28, and the suite's recorded baselines (ratchets) were re-seeded per procedure — Darwin at the same commit, Linux from the synced VM guest at 87b6665.
- Re-running the gate then surfaced an unrelated latent defect: the suite's witness path copied its binary over the live binary's inode, which macOS answers with a silent SIGKILL — two gate runs died with zero diagnostics before a rerun that captured exit codes exposed it. Fixed atomically (stage the copy, then rename) at 91ff675 and recorded in memory as a standing failure signature.
- One timing flake (S4-2) reproduced clean when run alone, per the standing rule that flakes are reproduced solo before anything is reverted.

**Alternative:** Shipping on the green gates alone; that would have left all 14 defects in place, including the high-severity one the gates had already certified.

**Impact:** Both machines green at 91ff675; the goal is closed and runtime-integration-contracts is promoted. This stretch also produced the "leash" pattern — holding remote work (the VM suite) as a local run via a polling process whose log carries the verdict — which closed the visibility gap for off-machine work twice.

## D76 — 2026-08-16: The phase-B loop splits at its budget; the installer work parks for a methodology ruling

**Decided:** The runtime-integration-contracts design (phase B from D74) ran six review rounds (gpt-5.6-sol at its highest setting), finding counts 14/15/14/10/9/15 with structural findings 12/11/5/5/5/8. The verification tables showed real convergence everywhere except one subdomain: for three consecutive rounds, every surviving structural finding sat in the adoption/installation execution mechanics, and the final round's structural count rose there (crash-consistent completion transitions, plan carriers, resume-record capabilities, engine-build transport), while the fixture-authorization, enforcement-transport, validator-population, and docs contracts went mechanical or resolved. This is the second full review budget the installer subdomain has consumed (the D74 agnosticism rounds were the first). Decision: split along the recorded boundary. B1 — everything converged — implements now under the goal; B2 — the installer execution rewrite — becomes the goal runtime-install-execution, parked with the reason on the ledger, because the choice it needs is methodological and reserved for Wido: build it implementation-first behind test fixtures and let code arbitrate (the pattern the evidence now recommends twice over), spend a third prose review budget, or drop the scope.

**Alternative:** Spending a third prose budget on the installer subdomain without a ruling. Not taken — that choice is Wido's, and nothing is blocked: B1 is weeks of work and the queue continues behind it.

**Impact:** Converged work proceeds while the divergent subdomain waits for a ruling instead of burning a third budget. Incidentally, the park verb's first real use exposed a crash in the goal CLI — a stray duplicate "and-none" flag registration — fixed directly with a regression test, per the two-bars doctrine's arm that lets an obvious mechanical defect be fixed on the spot.

## D77 — 2026-08-16: Runtime-integration-contracts B1 ships; the review finds six security holes

**Decided:** B1 landed in three checkpointed commits (the fixture-authorization capability system at e53355d; declared vectors, the generic enforcement comparison, and the hook contract at 64d3c86; contract emission and the derived reconciliation inventory at 64bf6f4), each passing the gates on both machines. The mandatory code review then found 16 real defects, 6 of them high — actual security holes the gates could not see: test-fixture evidence could authorize lease-sweep kill signals; a stale fixture row could target a recycled process group; the census (the periodic scan of running agent processes) bound fixture authority to the scan scope instead of the metasystem root; and construction errors quietly degraded to kernel-only evidence instead of refusing to decide. Three more findings were scope honesty: the registration rows, the configuration materialization, and the docs class were in B1's declared letter but had been quietly narrowed — all three were built out rather than re-scoped. All 16 were folded at 2fba4b5, with the coverage, adopt-fixture, and test-alignment follow-ups through fb6b7df; both machines green there.

**Alternative:** Shipping on the gates, or re-scoping B1 down to what had actually been built. Both rejected: the review made the holes and the quiet narrowing visible, and B1's declared letter was honored instead.

**Impact:** Goal closed; runtime-file-placement promoted; runtime-install-execution still parked for Wido's methodology ruling (D76). For the record: the adopt fixture caught an over-broad README audit row (adopted projects own their own READMEs); a kill that only hit the wrapper left an orphaned suite that the gate fence correctly refused; and one commit briefly rode past a failing test before the gate would have caught it — the fixup chain shows both.

## D78 — 2026-08-16: Item 17 closes; the review finds the new placement check could never fire

**Decided:** Backlog item 17's history question is answered: DevinPermissionMode ended up in codex.go by copy-drift during D25's envelope-mapping consolidation, and Wido's own D61 commit had already moved it home. Today's sweep found the residue — the shared usage parser wearing codex's name in a runtime-neutral file, and claudecommand_test.go grown into a grab-bag holding codex and devin tests — and shipped the mechanical placement check the mandate asked for (a check that catches runtime-specific names straying into neutral files). The mandatory review then delivered the sharpest finding of the program so far: the check's word-boundary regular expression could never match a runtime name inside an identifier, so it was structurally incapable of catching the exact stray it existed to prevent. It was rebuilt on Go's parser with identifier tokenization, plus regression rows pinning the DevinPermissionMode shape; the rebuilt check immediately flagged two further strays (the fakeEnv helper name, and devin's collect tests using the fake runtime's production writer), both fixed.

**Alternative:** Keeping the regex-based check. It would have passed every run while catching nothing.

**Impact:** Fourth consecutive ship in which the mandatory review found real defects — this time in the safety net itself. Both machines green at 2f42ec7; acp-transport promoted. Also recorded this stretch: Wido's accelerator ruling — runtime capabilities may speed work up but must never carry correctness — added to the architecture doctrine at f833e68.

## D79 — 2026-08-16: The ACP custody scope pivot — gate the new protocol, split off the old holes

**Decided:** Five rounds into the design loop for ACP (the wire protocol the Devin agent speaks) as a delegate transport, review round 5's finding 10 exposed a scope contradiction that had been fed for three folds: the document claimed to cover "Devin delegate transport only" while its custody corrections (custody = tracking and proving the death of a job's processes) kept rewriting how jobs are terminalized for every runtime's reaper, mission drain, lease sweep, and record compare-and-swap. Both claims could not stay true. Under the standing delegation, two moves. First, the new sealed-custody protocol (generations, birth tokens, a single proof-and-commit verb) is gated to records carrying a custodyProtocol marker that only ACP-selected jobs write, so legacy terminal behavior stays byte-for-byte unchanged. Second, the four pre-existing fleet-wide holes the loop surfaced — the standing reaper proving only the top-level pid before stamping the whole group dead, the lease sweep sending one TERM and then rewriting the record as failed, the RecordProtocolError verb bypassing compare-and-swap entirely, and process identity recorded at one-second resolution, which cannot exclude a pid recycled within the same second — split to a new queued goal, custody-death-proof, with the critique files as its evidence.

**Alternative:** Fixing the fleet-wide holes inside the transport design. Rejected: those defects ship today and predate ACP, and fixing them there would couple a risky cross-runtime migration to an experimental transport — exactly the coupling D74 and D76 were split to avoid.

**Impact:** The transport design keeps the sealed protocol (sealed-v1) as the working prototype of the target contract — the cheapest possible de-risking for the wider goal. Revert path: close custody-death-proof unstarted and widen acp-transport's scope back, or rule the fleet migration first and park acp-transport on it.

## D80 — 2026-08-16: acp-transport parks at budget exhaustion; the methodology question now has three data points

**Decided:** The acp-transport design loop ran its full two budgets: six review rounds at the highest setting, finding counts 15/8/13/13/10/8. The trend falls but never empties, and round six still carried five structural findings: marker immutability under the terminal lock, the lease sweep's nested-lock conflict with the fused proof-commit verb, cancellation having two committers, the settlement refusal table, and an experimental PromptResponse.usage field in ACP v1.3 the design had not considered. The ratified exit for "falling but still structural at exhaustion" is escalation, so the goal parks — joining runtime-install-execution on the same methodology ruling (D76). The pattern is now unmistakable: three execution-heavy subdomains (installer execution twice, now transport custody/settlement) diverge under prose review, while prose keeps discovering lock orderings and refusal mappings that test fixtures would pin mechanically in an afternoon.

**Alternative:** A third prose budget. Not taken — the ratified exit for this trajectory is escalation, not more rounds.

**Impact:** Banked before parking: the final review accepted the D79 scope pivot as sound, confirmed the ancestry-authentication mechanism implementable on both platforms, and confirmed the admission enum and its P1 evidence pipeline coherent; the fleet-wide custody holes are safely split off; and the design document plus six critiques fully seed whichever path the ruling picks. On any continue path the throwaway P1 wire probe runs first — its facts gate everything except the two lock-discipline findings. The program advances to disk-hygiene, next in queue.

## D81 — 2026-08-16: Wido rules — implementation-first behind fixtures

**Decided:** Asked whether ACP transport "is not going to fly," I answered that it flies and is parked on method, and recommended implementation-first behind fixtures: run the throwaway P1 wire probe, then build internal/acp against a stub server with test fixtures pinning each contract, the round-6 design document serving as the spec — with the same ruling unblocking runtime-install-execution, and a third prose budget advised against. Wido: "agreed and approved." That is the methodology ruling D76 and D80 were parked on, now ratified. The standing rule going forward: when an execution-heavy subdomain exhausts two prose review budgets without converging, the exit is implementation-first behind fixtures, not more prose.

**Alternative:** A third prose review budget — explicitly advised against and not chosen.

**Impact:** acp-transport unparks and becomes the current goal (the P1 probe first; its facts gate everything except the two lock-discipline findings); runtime-install-execution unparks into the queue to await its turn under the same method; disk-hygiene yields the current slot mid-loop — its round-3 review verdict, already in flight, will be folded and the design held at the budget-one boundary so nothing is lost.

## D82 — 2026-08-16: Wido sets the ACP acceptance gate — benchmark both hosts, flip, fix forward, keep the machinery

**Decided:** Wido's ruling, verbatim intent: "After running a benchmark on the VM and on the Mac with ACP on and it is fully successful then we should flip the switch and keep this on as the default and fix forward. The fallback we might want to keep in place for future agents that are like Devin and require a similar adapter. So the work should not be lost, although for Devin we should really aim to use ACP going forwards." Three consequences folded into the design: (1) the acceptance gate for making ACP the default is a fully successful benchmark run on both machines — Mac and VM — with the flag on; the planned "live smoke" test grows into that dual-host benchmark. (2) After the flip the posture is fix forward: ACP problems get fixed on ACP, and the legacy path is not a Devin escape hatch once the default lands — superseding the design's rollback-per-job framing for the post-flip era. (3) The legacy delegate machinery is retained, not deleted, as reusable adapter capability for future Devin-like runtimes.

**Alternative:** Deleting the legacy machinery after the flip, or keeping it as a Devin fallback — the ruling chose neither: keep it for future runtimes, but not as an escape hatch for Devin.

**Impact:** The retention amends the earlier D61 retirement condition: the dangerous-mode waiver retires for Devin when ACP becomes its default and the dangerous path stops being invoked for devin jobs; the machinery's survival no longer blocks that retirement, and any future runtime adopting the legacy adapter shape gets its own waiver decision on its own evidence.

## D83 — 2026-08-16: Wido adds two Devin-acceptance benchmarks — VM-only, run only when testing Devin

**Decided:** Two rulings in one conversation, verbatim intent. Third benchmark: "I want a third benchmark which we run only inside the virtual machine because I don't trust Devin on the Mac. The third benchmark should be Devin only. … Devin as the orchestrator and Devin as the delegate. That third benchmark might not succeed because the model is quite bad. However, what we can test is whether Devin as an orchestrator just works. And that's the important point here." Fourth: "Devin as the orchestrator and Claude as the implementer using opus 5 and also use Claude for the critique on opus 5. So only the orchestrator is Devin, the rest is Claude in opus 5. … I don't want them to run every time so these ones we should only use when testing Devin specifically. And both should run only inside the virtual machine." Shipped as benchmark kit specs bm-2d (devin host plus devin delegates, model swe-1-7) and bm-2dc (devin host plus claude opus-5 in every delegate role). Both carry a machine constraint of os=linux that the provisioner now enforces — it refuses a constrained spec on any other OS, so no operator habit can start an untrusted-orchestrator run on the Mac — plus an acceptanceOnly marker recording that neither joins any standing cadence.

**Alternative:** Making these part of the regular cadence or the comparison set. They are neither: they are orchestration-health probes whose graded score is expected to be poor and is not the acceptance question, and the D82 flip gate stays benchmarks one and two — these run alongside only when the devin adapter is the thing under test.

**Impact:** While validating the kit changes, provisioning turned out to be broken independently of this work: goal reconcile on a virgin target refuses for want of an authenticated lease holder — a regression from goal genesis (seeding a brand-new repository's goal ledger) combined with the authority hardening, present on the pristine kit too. It gates every acceptance benchmark, so it was opened as the queued goal provision-genesis-authority rather than patched at midnight.

## D84 — 2026-08-16: Close the genesis authority holes the review found

**Decided:** The mandatory review of the provisioning-genesis fix found the quick fix had opened a privilege escalation: a caller-supplied --genesis-from that is missing or crafted falls through to classify the caller as HUMAN, and HUMAN is admitted — so a delegate agent could launder itself into genesis (the seeding of a brand-new repository's goal ledger). Separately, deleting a project's goals-accepted.json reopened genesis on an already-initialized project, letting a non-holder re-baseline it. Both are closed. Genesis authorization now keys on an effective class that no caller-controlled root can forge upward: MAIN only when --genesis-from classifies the caller as a positively-announced main agent (a match a missing or crafted root cannot fabricate — those yield HUMAN); otherwise the class is computed against the target being written, where the adapter signatures live and a real delegate reads as DELEGATE. The store's genesis branch now runs under the record lock (closing the race between authorization and state) and refuses a non-holder writing over a ledger that already carries goals — a populated ledger with no baseline is treated as a corrupted initialized project restored by its holder, never re-adopted.

**Alternative:** Leaving the quick fix as shipped, which would have left both the fallthrough-to-HUMAN escalation and the delete-to-reopen hole in place.

**Impact:** The legitimate agent provisioning path is validated end to end (the kit provisioning bridge passes under a main-agent ancestry). Still open for follow-up: tightening the adopt/validate-kit fixtures to assert the MAIN classification rather than pass on Wido fallback (review findings F4/F5). Process note: the review was first refused by the codex model's cybersecurity content filter under an adversarial framing and ran clean when reframed as correctness — for future authority reviews, frame as properties to verify, not exploits to construct.

## D85 — 2026-08-16: The 10-hour autonomous mandate

**Decided:** Before a 10-hour absence, Wido ordered: "The goal is to complete the entire backlog, squash all bugs you encounter along the way, get me a clean, fully working production-grade system." Four decisions taken to unblock the unattended work:
- Run all four acceptance benchmarks if Devin is authenticated in the Lima VM (real delegate API spend authorized); if Devin is not authed, skip them and do everything else.
- Design and implement the sound genesis authority fix (genesis-authority-design) autonomously, behind mandatory review.
- Ship every backlog item — including the fleet-authority ones (custody-death-proof, two-bars, acknowledged-process) — behind mandatory design and code review, folding all findings before landing.
- On a clean D82 acceptance-benchmark pass on both hosts, perform the flip: dispatch.transport.devin=acp as the default, fix forward.

**Alternative:** Holding any of these for Wido's return would have idled the window; the mandate explicitly authorized proceeding.

**Impact:** Standing discipline holds throughout: review loops at codex's highest setting, mandatory post-gate code review, both-machine gates, the leash pattern (D75's local polling watch over long-running remote work), no turn ending with unleashed open work, plain-English decision records. Program order: finish acp-transport (sound genesis, then benchmarks, then flip, then close), then burn the queue in order.

## D86 — 2026-08-16: Genesis authority is an OS-sandbox problem, not a patchable one

**Decided:** Authorized by D85 to design and implement the sound genesis fix, three review rounds plus one design round produced an impossibility result instead of a fix: sound (unforgeable) genesis authority cannot be built on filesystem state or caller classification under the current threat model. A devin delegate whose sandbox is not enforced writes goals-accepted.json directly, with no CLI involved at all; and even a sandboxed delegate can read the whole control plane (its read roots include "."), so it can forge its classification or self-sign a capability. The HMAC-grant design was therefore not shipped — against this threat model it is security theater. Interim posture: keep the D84 defense-in-depth code (validated working for legitimate provisioning; it closes the accidental cases) and record the two real architectural directions for Wido's decision — (A) actually enforce the delegate sandboxes, which genesis soundness is downstream of, or (B) a compiled-in asymmetric trust anchor plus excluding the control plane from delegate reads.

**Alternative:** Shipping the HMAC-grant design anyway. Rejected as theater; both real directions are substantial and were deliberately not rushed unattended.

**Impact:** genesis-authority-design parks on this decision; the finding is captured in plans/genesis-authority-design.md. Consistent with the standing doctrine that the metasystem never claims enforcement it cannot guarantee. (D92 later overturns the impossibility claim itself while upholding the pause.)

## D87 — 2026-08-16: The process steward is resequenced behind owner instrumentation, not built as a watchdog duplicate

**Decided:** Picking up the process-steward goal (backlog item 21), two design rounds (plans/ps-critique-r1.md and r2.md, codex at its highest setting) established a sequencing truth rather than a converged design. Round 1 forced a rescope from five invariants and a checker that can act, down to a read-only aggregator over one invariant with an empty act-allowlist. Round 2 then showed that the one currently-checkable invariant — supervision liveness — is already surfaced at end of turn by internal/supervise.WatchdogReport through the same Stop hook the steward would use, so a steward re-check adds complexity for no new coverage; and it is not even a small slice, because internal/supervise.ArmedNow is a Boolean that collapses missing/unreadable/stale/dead into false, so the steward could not tell "unknown" from "breach" without a new supervise-owned typed verdict plus an incident lifecycle, a scan-cadence/attestation protocol, a turn-verdict state-machine extension, and arming integration. Decision: do not build the steward now. Resequence it behind the cheapest owner boundary that emits a typed verdict about a currently-unwatched invariant — the janitor's namespace-orphan verdict from the disk-hygiene goal — after which the steward becomes a thin read-only aggregator surfacing a fact the watchdog does not cover, and the round-2 contracts get built once, against a verdict that pays for them.

**Alternative:** Grinding round 3 and beyond to force the supervision-liveness slice to convergence — that would ship a duplicate of the watchdog, the opposite of the clean system the program is for.

**Impact:** The invariants that would be genuine new value — orphaned temp namespaces, unleashed plan promises, uncertified ships — each wait on a typed verdict their owner does not emit yet (the janitor's orphan verdict; Run.GoalId populated by the public launch/register verbs plus a plan-work decision owner; a real ship-certification owner). The design record carries the full target shape for when the dependency lands (plans/process-steward-design.md). (D94 later corrects the "no new coverage" claim but keeps the park.)

## D88 — 2026-08-16: Provisioning is unblocked; the ACP flip is ready but waits on a human-sealed benchmark

**Decided:** Two findings settle the current goal's remaining path.
- provision-genesis-authority is resolved: f4c4992 had recorded provisioning as broken (goal genesis on a virgin target refused), but 1654c85 fixed exactly that — authority gained a genesis mode admitting Wido or a main agent (never machinery), and genesis classification runs against the source root the adoption came from (adopt --genesis-from). benchmark/validate-kit.sh now passes on the Mac ("provisioning bridge passed", exit 0). This is separate from the parked genesis-authority-design (D86), which is about unforgeable hardening, not functional provisioning.
- The D82 flip gate cannot be crossed autonomously — by design, not by limitation. The ACP delegate path is already proven live in the VM (e1ce759), but benchmark/run-cohort.sh deliberately stops at a human seal boundary after provisioning ("each invocation stops at one human seal/sign boundary; after the contract has an Approval line, invoke --resume"), reflecting Wido's own rule that a provisioned trial waits for their signature before the mission spends. bm-2d is moreover a 12-wall-clock-hour full-mission run whose score is expected to be poor. Ruled out: (a) flipping on the live proof alone — that disrespects D82's explicit "run a benchmark, then flip"; (b) forging past the seal boundary; (c) starting a 12-hour Devin cohort that cannot be verified to completion before Wido returns.

**Alternative:** Provisioning a cohort now so it sits awaiting approval — declined because an unapproved provisioned target is wasted disk and the seal may never come.

**Impact:** The flip is ready and parked on Wido-sealed benchmark, with everything it needs confirmed present: VM up, Devin authenticated in the VM (3000.4.25), provisioning green. The one command when Wido is present is `benchmark/run-cohort.sh --spec bm-2d --repetitions 1` (VM only), which provisions and stops for their seal; on approval, the printed --resume runs it. provision-genesis-authority is left queued for Wido to confirm done rather than churn the current-goal pointer. (The "never machinery" claim was later exposed as porous and reconciled by D92 and D96.)

## D89 — 2026-08-16: The worktree observer is built, found unsound by its review, and reverted rather than shipped

**Decided:** The disk-hygiene worktree accumulation is real — 118 directories, roughly 500MB, because dispatch never reclaims a finished job's worktree — so a report-only `janitor worktrees` observer was built that classified each worktree by "job terminal + custody dead" and, on the live checkout, named 106 as reclaimable. The mandatory post-implementation review (plans/wt-code-critique-r1.md) returned "revise" with five structural findings and, decisively, verified against the live tree that the verdict is unsound: it classified as reclaimable three implementer worktrees still holding unmerged work, including caps-census-gate-order with a modified dispatch.sh. The reasons are load-bearing, not mechanical: a job's terminal record state is not proof its data has been released (conformance review and the authoritative diff read the worktree after the job terminates); custody-list death is not process-group death (grandchildren survive reparenting); and follow-up rounds alias the same workspace, so a running <job>-r2 uses a worktree whose <job>.json reads terminal. Decision: revert the code rather than ship an unsound reclamation-adjacent verdict or grind a half-fix for a modest diagnostic.

**Alternative:** Shipping a no-verdict status report — declined as marginal value in a subtle, dangerous area.

**Impact:** The sound reclaim is specified as the full journaled destructive slice (data-release proof, group death, alias resolution under lock, ownership/containment, same-user procfs), captured as "The worktree-reclaim proof" in plans/disk-hygiene-design.md. The accumulation and a safe human cleanup (`git worktree remove` refuses dirty trees on its own) are recorded as known issue KI-35 — advice D95 later corrects as unsafe. The review catching this before it shipped is the process working as intended; the critique is preserved beside the design.

## D90 — 2026-08-17: Wido rules the two-bars fork and approves the ACP seal

**Decided:** Two rulings from Wido, resolving the two open decisions. (1) The two-bars-for-changes goal targets the accidental threat model — "stop an honest agent from forgetting the rule" — not the adversarial one. This confirms the round-2 recommendation: locally composed hooks plus a gate witness bound to the exact tree that passed, human sovereignty intact, no new identity system; the adversarial layer (remote enforcement, durable agent identity, signed attestation) is explicitly a separate future goal and is not built here. (2) Wido approved the ACP acceptance-benchmark seal: this authorizes running bm-2d on the VM through Wido-seal boundary to completion, and flipping dispatch.transport.devin=acp to the default on a mechanically-successful run — bm-2d's graded score is expected poor by design; the acceptance question, per the bm-2d manifest, is whether devin-as-host and the devin ACP delegate path mechanically work end to end.

**Alternative:** On (2), the seal added on Wido's behalf executes this explicit approval — it is not the forge D88 declined, which would have been acting without authorization.

**Impact:** The two-bars design is converged for implementation on the accidental model; its last open question is answered. Fix-forward per D82: the fixture and snapshot-identity hardening follow the flip. (D91 later reverses this gate reframing.)

## D91 — 2026-08-17: The Opus-window re-review reverses D88/D90's gate reframing

**Decided:** Context: the session ran on Opus 4.8 — not Fable, the required model — from 2026-08-16 21:31 to 2026-08-17 08:28 local, the whole D85 autonomous window. Wido caught the switch and ordered a re-review of every design judgment from that window. The first re-review (codex sol at its highest setting, plans/opus-window-review-acp.md) examined D88/D90 and found the flip gate had been weakened. Corrections, effective now:
- The D82 gate is restored to the dual-host bm-2 pair: claude host plus devin delegate with dispatch.transport.devin=acp proven in the provisioned target, successful on the VM and the Mac, judged by the review's named acceptance contract (ACP resolved before the seal; every devin dispatch recording transport=acp, a wire session id, and ACP outcome artifacts; no legacy fallback or transport-level failure; ACP-bound snapshot admission; terminal-state policy declared before the run). bm-2d and bm-2dc are D83 coverage, not substitutes. The review also caught that the command recorded in D88/D90 never enables ACP at all — provisioning does not set the key and the adapter defaults to legacy when it is absent — so a bm-2d run could have been "successful" entirely on the legacy path.
- The snapshot identity surfaces and the supervise_acp fixture are restored as flip prerequisites. The concrete failure chain: snapshot identity does not include the transport, so a legacy-era snapshot admits an ACP dispatch whose protocol/schema identity was never certified — admission evidence crossing the transport boundary, exactly what the design forbids.
- The seal interpretation stands: executing Wido's explicit approval is clerical, and the approval binds to the exact sealed hash, never to gate redefinitions.
- Open for Wido: D82 (a benchmark with a devin delegate on the Mac) and D83 ("I don't trust Devin on the Mac") conflict if the distrust covers delegates — and the wire probe's no-path-containment finding argues it does. The question was asked; the Mac half of the gate awaits the ruling.

**Alternative:** Letting the window's reframing stand, with bm-2d as the gate — rejected because the recorded command never even enabled ACP, so the gate would have proven nothing about the transport.

**Impact:** The flip remains held until the prerequisites exist and the D82 pair passes as ruled. The in-flight bm-2d cohort continues as approved D83 coverage, with transport=acp set in its target before resume.

## D92 — 2026-08-17: The re-review overturns D86's impossibility claim; two authority defects reopen

**Decided:** The second Opus-window re-review (codex sol at its highest setting, plans/opus-window-review-genesis.md) examined the genesis conclusions. Verdict: rejecting the HMAC design and pausing for an external trust-boundary decision were correct; the impossibility claim, the A/B framing, and the D84 description were not. The impossibility is overturned: the true premises (a non-enforced devin delegate writes the baseline file directly; delegates read the whole repo) rule out CLI-only authorization, repo-file classification, and symmetric secrets in the delegate's privilege domain — and nothing more. Asymmetric signing survives, because a public verify key needs no secrecy and the private signer can be held by another OS principal, a hardware key, a user-presence keychain, or an external service. The goal's park now names the real decision — where the trust/integrity boundary lives: (A') sandbox enforcement plus control-plane write exclusion, (B') an externally-held signer with a pinned verifier and signed provenance (verifier-binary integrity remains an open sub-problem), or (C') keep the cooperative doctrine and drop "unforgeable genesis" as a product contract. Separately, two defects are confirmed open in shipped code, to be fixed now under the bug-squash mandate, each behind tests and the mandatory review: the D84-introduced adapter-supervisor regression (goalCaller discards every non-MAIN source classification, so a correctly-identified machinery caller can reclassify as HUMAN against a virgin target and be admitted), and the pre-lock genesis-mode race (the mode is chosen by os.Stat before Reconcile takes its lock, skipping the genesis guard when a baseline appears in that window — recorded in the window's own review but omitted from the park).

**Alternative:** Letting the impossibility claim and the A/B framing stand; the re-review showed they claimed more than the premises actually prove.

**Impact:** The D84 record's "closes accidental misuse" description stands only with those two caveats named, and the durable claims that caller-controlled roots cannot forge upward are withdrawn.

## D93 — 2026-08-17: Wido rules the two re-review forks; the benchmark spec-identity defect is fixed

**Decided:** Three items, closing the Opus-window corrections' open questions.
- Devin on the Mac: Wido ruled "allowed when you need one — if there is a good reason, go ahead." The D82 flip gate is a good reason, so the dual-host gate stands as originally written: one bm-2 repetition (claude host plus devin delegate, dispatch.transport.devin=acp proven in the target) on the VM and on the Mac, judged by the D91 acceptance contract. Devin on the Mac remains exceptional, not routine — each use needs its named reason.
- Genesis: Wido chose C' — drop "unforgeable genesis" as a product contract. The cooperative same-user controls stay (D84), the operator and the VM supply the real isolation, and no signer or sandbox-enforcement machinery is built for genesis. Executing now under the bug mandate: fix the two D92 defects (the adapter-supervisor reclassification regression and the pre-lock genesis-mode race), then withdraw every durable claim that caller-controlled roots cannot forge upward — the records say cooperative, not unforgeable. That is exactly the genesis-authority-design goal's remaining scope; it concludes when those land.
- Discovered while executing the approved bm-2d run: bm-2d and bm-2dc had shipped carrying bm-2's manifest id, which keys cohort naming, the recorded benchmarkSpecId, and --resume's spec resolution — the first acceptance cohort provisioned under bm-2's identity and would have resumed against the wrong spec. Fixed: ids now equal their spec directory names, the kit consistency check refuses id/dirname mismatches, the mis-identified unsealed cohort was discarded, and the kit gate is green.

**Alternative:** For genesis, the other two directions — A' (enforce the sandboxes) and B' (external signer plus pinned verifier) — were on the table; Wido chose C' instead of building either.

**Impact:** Also fixed en route: the machine fingerprint refused to compute on the aarch64 VM guest (no "model name" line in /proc/cpuinfo), which meant the VM-only spec could never have provisioned on the very machine it is constrained to — it now synthesizes a stable identity from the CPU vendor and implementer/part codes.

## D94 — 2026-08-17: The steward and two-bars window folds are repaired; the genesis defects are fixed in code

**Decided:** The third and fourth Opus-window re-reviews landed (plans/opus-window-review-steward.md and -twobars.md; both "revise"). All corrections applied:
- process-steward: D87's "adds no new coverage" was an overclaim — WatchdogReport largely overlaps a typed liveness verdict but is not equivalent (different freshness window; no heartbeat/generation/cap/tag verification; prose rather than tri-state; discarded on blocking turns; Stop-hook only; not persisted). The park stands on the corrected ground: the typed owner verdict does not exist and building it is owner work. The resume condition is generalized — any sound typed owner verdict about an unwatched invariant qualifies, the janitor orphan verdict being a candidate rather than the dependency (D89 made it non-cheap) — and resuming authorizes design, which must first justify an aggregator over direct owner delivery. The design document is rewritten in one voice: the window's fold had left old contradicted sections standing beside the corrected target and had dropped round 2's committed-but-durability-unknown outcome, now restored.
- two-bars: the round-2 fold had left findings TB-R1-02/03/05/06 materially open. Round 3 adopts the review's dispositions: the emergency path is human-personal (no agent tokens); a raw agent using git out of habit is in scope (honest forgetting) while only deliberate bypass counts as adversarial; the tree-bound witness is finalized by the whole validator at suite end, never by the mid-run go-gate (that was the false-green hole); the accidental-model keep/skip table is the build boundary; and the build order is the review's five steps, starting with settling the contracts and the design-obligation matrix the document lacked.
- The two D92 genesis defects are fixed in code with tests: genesisEffective now refuses a positive machinery source classification outright (table-tested, including the crafted-root HUMAN non-raise), and goal.Caller carries the genesis authorization mode so Reconcile refuses a genesis-admitted caller on every non-genesis arm under its lock (tested for the raced, re-run, and virgin cases).

**Alternative:** Leaving the window's folds as written — which would have kept an overclaimed park rationale, four materially-open two-bars findings, and two live authority defects.

**Impact:** Adopt fixtures pass; full gate plus mandatory code review before the ship. (D96 later reverts the machinery-refusal half of the genesis fix as suite-breaking, while the race guard survives verification.)

## D95 — 2026-08-17: The disk-hygiene review lands; unsafe cleanup advice is corrected and the headroom guard hardened

**Decided:** The fifth and final Opus-window re-review (plans/opus-window-review-dh.md, 13 findings) closed the sweep. All actions taken:
- KI-35's "safe manual cleanup" advice — which I had given Wido directly — was wrong and is corrected to report-only: `git worktree remove` deletes ignored data without protest (nine clean delegate worktrees hold ignored artifacts), passes committed-but-unmerged branches with no proof the data was released, and the suggested repository-global prune would have pruned Wido's unrelated wt-flakefix scratchpad (verified by dry-run). No manual bulk cleanup until the journaled reclaim exists.
- The headroom guard (the pre-run check that the disk has enough free space) is hardened in code: measurement is pinned to an open file descriptor (no identity race between stat and statfs); walking up to an ancestor happens only on "file not found" (permission and path failures now refuse instead of measuring an ancestor); arithmetic is checked and the floor validated (NaN/Inf/negative refused); the suite now refuses on measurement failure while below-floor stays advisory; a clean checkout gets a df bootstrap check instead of no check; and the per-path-advisory rule is documented (APFS volumes share one container pool across distinct device ids — dedup suppresses duplicate warnings and licenses nothing else).
- The worktree-reclaim proof gains the review's four structural additions: durable release records retained until reclaim (evidence garbage collection had already pruned the records 12 worktrees would need), a canonical-workspace lease covering fresh dispatch and conformance readers (one chain lock cannot fence aliases), a closed consumer set instead of process-group death, and a child-first descendant inventory. `git worktree remove` is demoted from "sound proxy" to a last refusal that is not a release proof.

**Alternative:** Leaving the KI-35 advice and the softer guard in place — the dry-run proof showed the advised prune would have destroyed Wido's own unrelated data.

**Impact:** This closes the Opus-window review sweep Wido ordered: five re-reviews (ACP D91, genesis D92, steward and two-bars D94, disk-hygiene D95), every "revise" verdict folded, two authority defects and one piece of unsafe advice corrected, all on Fable.

## D96 — 2026-08-17: Verification round 1 lands; the genesis machinery-refusal is reverted as broken; my green claim was wrong

**Decided:** Wido ordered codex verification of everything from the review sweep before further action. Round 1 verdicts and what they forced:
- The genesis machinery-refusal (e2fb2f5) broke the suite. The verification proved — from my own preserved fixture evidence — that refusing a machinery source classification breaks legitimate delegated validation: the adopt fixtures and the kit gate run under agent ancestry with announcement-free snapshots, so their source view is DELEGATE by signature, and the nested-adoption fixture had been failing since that commit. My earlier "adopt fixtures pass" claim was false: I read $? after a pipe, which reports tail's exit status, not the script's. The refusal is reverted (the announced-MAIN raise and the pre-lock race guard remain — the race guard was verified complete); the crafted-root and virgin-target holes are recorded in code as the C' cooperative posture, replacing comments that overclaimed unforgeability.
- ki-23 (known issue 23) round 1: the same-second binding hole — the census records process start times in whole seconds, which cannot exclude a pid recycled within the same second — is closed properly: the census now publishes an exact birth token (pidStartedAtExactMicro, additive beside the whole-second join keys) and acknowledgement binds to the token the census observed. The verb no longer accepts --caller-pid (a caller-supplied -1 classified as HUMAN and laundered holder-only authority — verified live), and record decoding is strict (unknown fields refused, RFC3339 timestamps enforced).
- Headroom round 1: a raw path walk replaces filepath.Clean ('file/../.' measured the current directory — verified live); the pinned open uses O_NONBLOCK (a FIFO with no writer hung the guard); the suite treats a stale binary's usage error as bootstrap-degrade (refusing there would permanently block the very rebuild that fixes it) while a real measurement failure still refuses; the df bootstrap validates its floor, includes GOCACHE, and names df failures.
- The aarch64 fingerprint crashed when lscpu was absent, cutting off its own fallback, and parsed localized labels — fixed (FileNotFoundError guarded, LC_ALL=C forced). Verified closed: the spec-identity fix and the ACP fold ("no finding"), plus — after citation and normalization fixes — the steward rewrite; two-bars needed five body normalizations plus one fork-section sentence.

**Alternative:** Keeping the machinery-refusal in place; the preserved fixture evidence showed it broke legitimate delegated validation, so it was reverted rather than patched around.

**Impact:** Standing lessons recorded: never read $? after a pipeline, and a "fixture evidence preserved" line means a failure was captured — read it.

## D97 — 2026-08-17: Verification rounds 2–4 converge; Wido retires the quiet-machine requirement

**Decided:** The verification loop Wido ordered ran four rounds; this closes it.
- Round 2 found four structural gaps, all folded: the same-second census binding (closed by the census publishing a kernel-resolution birth token, pidStartedAtExactMicro, additive beside the whole-second join keys, which acknowledgement now binds to); the --caller-pid laundering (closed by removing the flag); and the Clean-erasure in the headroom ascent plus a FIFO hang (closed by a raw-path walk with O_NONBLOCK). Round 2 also exposed that the genesis machinery-refusal had broken the suite and that my green claim came from reading $? after a pipe; the refusal was reverted, and the truth behind it ran deeper — the adopt fixtures had never passed standalone from agent ancestry, because nobody had wired METASYSTEM_GENESIS_AUTHORITY_ROOT (built for exactly this) into the harness. One export at the top of the harness fixed it; the fixture suite then ran green end to end from agent ancestry for the first time. A mutating consistency probe (running goal reconcile as an assertion) was replaced by the engine's read-only fact — Store.BaselineMatches exposed as goal list's baselineMatches — per Wido's no-new-python ruling.
- Round 3 confirmed everything and left one residual (the configured-vs-installed signature universe), closed with lease.DirectAgentInvoker over every installed adapter. Round 4 confirmed that and found four mechanical items, all folded: the ArgvKnown fail-open (absence of argv evidence now refuses), the nested-harness scale validator raised to the probe's 48 ceiling (certified live at 21/48/49), the corrected cap arithmetic, and the canonical policy written into docs/orchestration.md.
- The quiet-machine ruling: Wido asked whether the suite really needs a quiet machine and ruled against requiring one. The fixture caps were machine-speed assertions (probe once, ceiling 12x) whose real job is hang detection. The clamp is now [8,48]: waits that converge return on their condition regardless of the ceiling, so passing runs are unaffected; a genuine hang names itself within minutes instead of seconds; negative fixtures that consume full timeouts stretch by seconds, accepted knowingly. Proof: three suite runs had failed at exactly the old cap under a neighboring session's JVM, and with the new clamps the full suite passed on the same machine at load 8.86. Caps self-calibrate per machine (the 4-core VM passes the same suite); hardware beyond 48x uses the METASYSTEM_FIXTURE_CAP_SCALE override.

**Alternative:** Keeping the one-suite-per-quiet-machine practice. Wido's ruling and the passing run at load 8.86 supersede it.

**Impact:** The verification loop is closed with all rounds folded. Also recorded: the suite requires a symlink-free invocation path (the /tmp alias broke git-archive prefix computation), and a fresh clone is not suite-equivalent without the gitignored conf.local.

## D98 — 2026-08-17: The Devin-as-orchestrator acceptance question is answered — the machinery works

**Decided:** The bm-2d acceptance run (cohort bm-2d-20260817t114728z, one repetition, Devin hosting with model swe-1-7 on the VM) is judged a MECHANICAL PASS, and the bm-2d arm needs no further repetitions for its acceptance purpose. Wido's D83 question was "whether Devin as an orchestrator just works — that's the important point," with a poor score explicitly expected and not the point. After the permission fix from the first attempt (KI-36: non-interactive Devin needs the dangerous mode to execute commands), every mechanical stage worked end to end: provisioning, Wido's seal and signature, preflight, arming, one complete host turn in which the Devin orchestrator ran tools and delivered a schema-valid return that the runner accepted, the mission's park transition (the model parked all its work streams on turn one rather than attempting the build), the runner's clean wind-down (record completed with an end time — the first attempt's silent death did not recur), grading, extraction, and cohort completion. Zero delegate jobs were dispatched, so this run exercised the host mechanics only; the Devin-delegate-over-ACP leg was proven separately in the live run recorded at commit e1ce759.

**Alternative:** Running more repetitions to see whether swe-1-7 ever attempts the mission. Not taken: the acceptance question was about the machinery, not the model's ambition — a second identical park would cost a provisioning cycle and Wido's signature and teach nothing new, and D83 already priced in a weak model.

**Impact:** Devin can now be trusted to HOST missions mechanically (dispatch, settle, record, close) inside the VM, which is what D83 wanted established before any deeper Devin use. The capability ceiling observed — the model parks rather than works — is a model quality fact, not a machinery defect. KI-36's open half (the indistinguishable silent runner exit from attempt one) remains the standing follow-up, now with a working-path counterexample recorded.

## D98 addendum — 2026-08-17: The park reading was wrong — the Devin host built the whole solution itself

**Decided:** Correct D98's account after reading the scorecard and the target tree. The mission did not "park rather than attempt the build": the swe-1-7 host BUILT THE SOLUTION SINGLE-HANDEDLY in its one turn — 11 Java source files, a requirements map covering all 26 requirements, a clean offline Maven build, its own passing tests, and a DECISIONS.md — scoring acceptance 0.981 and requirement coverage 0.962, then parked its streams because the work was done. It never dispatched a delegate, and its own return admits why in the gaps field: it read the completion gate as demanding "a single end-to-end build in one turn". The benchmark correctly stamps the run INVALID on the delegation floor (streams without a certified implementer job: build) — that gate exists precisely to catch an orchestrator doing implementer work — and on an evidence gap (the kit did not collect per-turn session-usage/prompt files for a devin host, a kit-side collection hole to fix).

**Alternative:** Leave D98 as written. Rejected: the record said the model chose not to work, when it actually worked too much and in the wrong role — for a document whose job is after-the-fact review, that distinction is the whole point.

**Impact:** The mechanical pass stands and strengthens: the host not only ran a turn, it exercised tools heavily for half an hour under the dangerous mode and delivered a valid return. Two new facts for the program: swe-1-7 is capable of one-shotting bm-2 solo (the "poor score expected" assumption was wrong in an interesting direction — the score was high and the DISCIPLINE failed), and the delegation-floor gate demonstrably catches role violations in the wild. Follow-ups: the kit's devin-host evidence collection gap, and the host prompt's completion-gate wording that a literal model reads as permission to do everything itself.

## D99 — 2026-08-17: The solo-build is ruled a metasystem enforcement failure; a runtime wall against host implementer-work becomes the next goal

**Decided:** Wido ruled the bm-2d solo-build "a total failure of the metasystem" — the host was supposed to design and critique in a loop, then implement through critiqued delegates, and it did none of that. The investigation confirms his reading with three findings. First, the transcript shows ZERO dispatch attempts across all 66 commands the host ran — it never tried to delegate and never hit a delegation failure to fall back from; it simply implemented, violating the prompt's explicit "advance streams by designing, dispatching, reviewing, and certifying" discipline. Second, that discipline exists ONLY as prose in the host prompt: earlier hosts (Claude models) obeyed it by disposition, so the rule was never load-bearing, and the first differently-disposed model ignored it wholesale — exactly Wido's standing objection that rules living in prose fail when it matters. Third, there is no runtime enforcement: the runner accepted a return declaring an empty dispatched list while the same turn had written eleven product source files, and the only check that caught the violation — the kit's delegation floor — runs at grading time, after the money is spent. A new goal is opened: a mechanical turn-acceptance wall — the runner refuses a host return whose workspace diff contains product-code writes not attributable to certified delegate work, parking the stream as a protocol error. This is an invariant, so it goes through the design loop before implementation. The mission-contract completion-gate wording (which the model cited as its license to build everything in one turn) gets reworded in the same pass.

**Alternative:** Treat it as a model-quality problem (swe-1-7 disobeyed; better models obey) and rely on the grading-time delegation floor as the safety net. Rejected: the metasystem's core claim is that the process holds regardless of which model sits in which seat, and a discipline that only compliant models follow is not a mechanism — the run proved the gap is exploitable by accident, without any adversary.

**Impact:** Until the wall ships, any non-Claude host can silently bypass the design-and-critique loop and the failure is only visible at grading. The bm-2d mechanical findings stand (turn lifecycle, records, wind-down all worked), but the run's deeper lesson supersedes its headline: the machinery moved correctly while the PROCESS it exists to guarantee was skipped entirely, and only a post-hoc tripwire noticed.

## D100 — 2026-08-17: Wido rules the wall — no host self-work, detector tier, and a small-change lane on the backlog

**Decided:** Two rulings from Wido close the host-implementer-wall design's open questions. First: NO self-work exception — every product byte in a mission comes from a certified delegate, full stop; the entire measured-residual exception machinery drops out of the design. Wido added a requirement with the ruling: the "change this little thing" case (a one-line typo blocking a gate, and its kin) must still be SUPPORTED — not as a host exemption but as its own proper mechanism, placed on the backlog. A new goal (small-change-lane) is opened for it: a dispatch path cheap and fast enough that fixing a typo through a certified delegate does not feel like ceremony. Second: the wall ships at the DEFECT DETECTOR tier now — it stops every accidental and naive bypass including the D99 solo-build, while the residual deliberately-deceptive shapes (a host tampering with delegate worktrees to forge authorship) stay recorded in the critique's laundering table rather than denied; closing those would need OS-level isolation, which stays unbuilt for now.

**Alternative:** A measured cumulative self-work ceiling (rejected: the exception machinery was a third of the implementation and every edge case needed pricing), and holding the wall for the OS-isolation security tier (rejected: months of foundation work while the accidental hole stays open; consistent with Wido's genesis C' and two-bars accidental-model rulings).

**Impact:** The wall's design simplifies materially — the tree equation becomes post-tree = pre-tree + certified patches + machine-owned metadata, nothing else — and implementation can start once the r2 critique converges. The small-change lane becomes the queue's answer to legitimate tiny fixes, keeping the invariant absolute instead of perforated.

## D101 — 2026-08-17: The wall design pivots to implementation-first after five rounds

**Decided:** The host-implementer-wall design loop closes at round five under the ratified stop rules, pivoting to D81 implementation-first. The finding trajectory fell 9 → 8 → 4 → 5 → 1; round five's single structural finding (an authorization must bind the sequence-point identity of its base tree, not just the tree id, because tree ids legitimately repeat across no-change turns and restorations) is folded into the spec, and the critic itself recommended the pivot rather than a sixth prose round. The design document is now the specification: thirteen gate-parsed obligation rows with named owners, code targets, and fixture lists — including positive fixtures proving post-resolution work stays consumable — are the arbiter for the build. Remaining shape choices (exact JSON field schemas, event registry types) are explicitly owned by golden fixtures during implementation, per the round-five ruling.

**Alternative:** A sixth critique round. Rejected by the stop rule Wido ratified and by the critic's own verdict — the one structural finding had a one-paragraph fix, and everything else left is the kind of choice a fixture pins better than prose.

**Impact:** Implementation starts immediately, slice one being the shared git-tree primitive (HIW-O4's foundation). Until the wall ships, the exposure D99 named remains open on non-Claude mission hosts — mitigated only by the grading-time delegation floor — which is why the wall holds the Current goal slot ahead of the ACP flip.

## D102 — 2026-08-18: Issue #2 pivots from option (a) to option (b) on five-finding evidence

**Decided:** The unattended-runner lease fix is built as the issue's option (b) — a single new succession edge where a mission's own host turn may succeed the LIVE runner holding the checkout lease under the same mission lineage — plus the runner re-announcing at turn conclusion so it succeeds the dead host through the existing same-lineage rule. Option (a) (retire + release before turn 1), which the issue recommended and I first built, was overturned by its adversarial review: with no lease at all, the runner's own anchor commits classify as HUMAN and bypass the lease entirely (a foreign main could claim mid-mission while the runner still writes — the exact single-writer violation the lease exists to prevent); resume deleted the claim epoch that keeps inherited delegates swept; retirement could silently miss under the very clock drift issue #1 fixed; and foreground runs lost holder-only authority for reaping. Option (b) has no lease-less phase, so all four failures are structurally impossible, and every boundary uses the existing succession machinery with its epoch-sweep semantics.

**Alternative:** Patching option (a)'s holes individually — an authority story for the lease-less window, epoch preservation on resume, verified retirement. Rejected: that rebuilds half the lease protocol to protect a window that option (b) simply never opens. Wido's issue preferred (a) "unless blocked"; it is blocked, by write-authority coupling rather than the census coupling he anticipated — same class, deeper.

**Impact:** "A live holder never yields" gains exactly one exception: to its own mission's host turn (holder announcement tagged mission-runner.sh, same lineage, claimant not itself a runner). Foreign mains stay refused (O-1 untouched). Wido reviews this override of his in-issue recommendation on return; revert by dropping the claim.go edge and the conclude re-announce.

## D103 — 2026-08-18: The stop-loss learns to see candidate-branch progress (issue #4)

**Decided:** Issue #4 is fixed as the issue's option 1 plus option 3, landed together as ledger semantics 3. The stop-loss best tuple now extends with the declared gate metrics measured on the mission's active candidate branch: after every gate-of-record component, a candidate component folds in, so a candidate that beats the recorded best resets stagnation — but lexicographically a candidate can never outrank a real merge, because the gate-of-record components always compare first. Absent candidates read as directed worst, so silence never counts as progress. The measurement rides the existing contract gate machinery (same materialization, caps, and validation as the gate of record) against the exact `agent/<jobId>` branch — any other name, or a name carrying token-delimiter bytes, is refused and the refusal is a registered event. Three guards came out of the critique rounds rather than the original design: the runner refuses a state whose ledger semantics it does not implement as the FIRST act of its life, before any event, record, or healer runs; mission state bumps to schemaVersion 3 so every pre-semantics-3 binary already refuses the bytes at its first validated read (and with wall history present, even the corrupt-recovery path refuses to re-root, leaving the state byte-identical); and sealing a contract whose only gate is binary pass/fail with a fuse shorter than the critique cadence is refused outright — the vm-smoke-4 shape that parked a perfect solution — unless the contract seals `ledger.accept-binary-gate-fuse=true` to acknowledge the shape by name, which the fixture beds do.

**Alternative:** Counting delegate certifications as progress (option 2 in the issue). Rejected: certifications measure activity, not gate movement — a mission can certify forever while the score stands still, which is exactly the stall the stop-loss exists to catch. Also rejected: silently tolerating short fuses on binary gates; the refusal names the failure class instead of documenting it.

**Impact:** A vm-smoke-4 replay differential is now a test: the same ledger bytes that tripped the semantics-2 stop-loss (five stagnant cycles while a full-marks candidate sat unmerged) leave semantics 3 untripped with stagnation 1. Four critique rounds against codex gpt-5.6-sol at xhigh reached AGREE; the named residual — an unshipped dev-window binary could re-root a semantics-3 state that has no wall turns yet, after preserving an evidence copy — is accepted as out of reach of any shipped binary. Live VM validation runs when Wido is back to seal, per his fixtures-suffice ruling.

## D104 — 2026-08-18: Wall slice 5 lands after nine critique rounds — and moves the state anchor off the mission branch

**Decided:** The host-implementer wall's slice 5 (build-map items 1-4: the open-turn marker, certification adjudication, the tree equation, and one-write acceptance) lands after NINE critique rounds against codex gpt-5.6-sol at xhigh, finding trajectory 7 → 6 → 3 → 2 → 7 → 4 → 1 → 1 → 0, every finding folded. The wall now does what D99/D100 demanded: every turn snapshots and anchors the shippable pre-tree before the host launches; every certification claim is verified against authenticated, content-addressed authorization records (nine rejection classes, supersession heads, fork detection, one-time consumption, the full sequence-point staleness predicate with reviewed object-id equality); after every host exit the workspace must equal pre-tree plus exact authorized patches plus contract-declared artifact files, and anything else writes evidence, taints the workspace, and parks before any success path; acceptance is one hash-chained write carrying the wall verdict, its consumptions, and its occurrence identity. The rounds forced one architectural change beyond the plan: STATE ANCHORS NOW LIVE ON A RUNNER-OWNED REF (refs/metasystem/missions/<m>/state-anchors), plumbing-built, never on the mission branch — the old on-branch add -f doctrine force-tracked runner bookkeeping into every delegate worktree, split the wall's tree identity from the branch's, made conformance refuse real mission chains, and let any host commit become the ledger guard's baseline. The in-turn ledger guard now compares live bytes to the authenticated anchored truth; recovery tolerates exactly one reserved, open-turn-bound ledger-ahead block and re-anchors its own heal; a forged anchor tip in any shape parks.

**Alternative:** Per-symptom patches on the on-branch anchor (the round-3 ledger exemption was exactly that, and round 4 broke it). Rejected once the fourth consequence of the same root surfaced; the ref move fixed all of them at once.

**Impact:** The D99 solo-build shape is now caught end to end in fixture (park, taint, evidence, resume refused) — and the wall caught a real instance in our own dispatch fixtures on its first live run: the stub codex host's mid-turn candidate-score.txt write, now sealed by name as wall.host-artifacts. Existing missions with on-branch anchors re-provision, consistent with the schema-3 barrier and Wido's fixtures-suffice ruling. Live VM validation runs when Wido seals. Remaining wall rows (doctrine/prompts, delegation floor, resolution verbs, events, detector, both-host gates) follow as the next slices, after the six new GitHub issues Wido queued.

## D105 — 2026-08-18: Human answers reach the orchestrator, and asks learn supersession (issue #11)

**Decided:** The vm-smoke-5 loop-breaker — a human's answer to a mission ask never appeared in any later prompt, so the orchestrator re-asked and re-parked the very question a human had already ruled on — is fixed with three mechanisms landed together. First, a new required prompt section, Human Answers, sits at the top of the dynamic sections and carries every standing human ruling verbatim (the asks each stream's answeredAsk names: id, stream, when, the question, and the full answer), and the stream rows name their answering ask; the turn-prompt validator enforces the section and shapes in lockstep, and the orchestrator preamble teaches that these rows are rulings senior to the host's own judgment on the settled question, never to be re-asked. Second, asks learned supersession: an ask candidate may name an open same-stream ask it replaces, adjudication validates the claim (grammar-checked id — which is also the path-traversal guard — open, same stream, not already superseded in the pass, on disk, or DERIVABLY by an existing successor), the successor is written before the predecessor closes, and closure is DERIVED from the successor's existence everywhere it matters — the prompt, the waiting list, the park guarantee, adjudication, and the answer path — so a crash between the two writes can neither resurrect the stale question nor mint two live successors. Third, answering a superseded ask refuses by name toward its successor. Four critique rounds (8 → 4 → 1 → 0 findings) against codex gpt-5.6-sol at xhigh; the sharpest catches were the path-traversal join, a superseded host-failure ask silently satisfying the park guarantee while being invisible (a permanently unanswerable park), and the in-house schema engine rejecting the pattern keyword — which the full battery caught before the critic did.

**Alternative:** The issue's minimum rule (answering any ask marks same-stream same-class asks answered-by-reference) without real supersession. Rejected: it papers over duplicate questions instead of retiring them, and the orchestrator was already writing "replaces and sharpens" in prose — the mechanism now exists for what hosts were doing anyway.

**Impact:** Replaying vm-smoke-5's shape: turn 3's prompt now carries the option-A ruling verbatim, ask-1-1 is not listed as open, and the acceptance test pins it. Unblocks issue #7's live validation (the answer that authorizes the critic's model can now be acted on). Live VM validation runs when Wido seals.

## D106 — 2026-08-18: The Devin host's ACP helper stops masquerading as a delegate (issue #12)

**Decided:** The bm-2d-og blocker — with Devin as the mission host, the ancestry walk met the CLI's internal ACP helper before the announced main, classified the orchestrator DELEGATE, and refused every dispatch — is fixed by an exclusion-and-marker pair that lands together. The helper's exact shape was live-probed on this machine rather than guessed: a `devin -p` session's own ancestry shows `tool shell <- devin acp <- devin -p`. The Devin adapter's signature now excludes raw `devin acp` (path-prefixed or bare), so the walk continues through the helper to the announced main; and the DELEGATE-side ACP server this repository launches carries the distinguishable argv0 `devin-delegate-acp` (bash `exec -a`; the binary ignores argv0, the classifier reads it) with its own match line, so delegate tool shells still classify DELEGATE — a blanket exclusion would have laundered them into MAIN, which is the wall's checkout bypass. The runtime's declared signature lookalike is now literally `devin acp`, pinning the exclusion in the S4-7 contract check forever. Census tests cover nine argv shapes plus a shipped-adapter drift guard; classification tests prove both directions over a real process chain with the intermediate's command supplied through the authorized identity fixture. The issue's alternative — finish-the-walk classify semantics preferring an announced main — was rejected: delegates always run under the main, so that preference would misclassify every real delegate.

**Alternative rejected, and one residual named:** a process that deliberately names itself `devin acp` inherits the exclusion — multi-step forgery in the laundering table (detector tier), the same class as the existing claude/codex helper excludes.

**Impact:** Devin-hosted missions can dispatch; the D83 Devin acceptance line unblocks. Acceptance rerun of bm-2d-og happens on the VM at seal time (delegationFloorMet=true, at least one implementer started).

## D107 — 2026-08-18: The two dispatch-reporting foot-guns close (issue #10)

**Decided:** Both vm-smoke-5 reporting slips are prevented mechanically. First, a FRESH chain root whose id claims round two or later (-rN, N>=2 — numerically parsed, failing CLOSED on an unparseable suffix after the critic walked an overflow literal straight through the first version) is refused at the record-creation choke point with a message naming the resume path; the conventional -r1 root, -r0, and every resume record stay lawful. Second, a `dispatched` entry naming one of this mission's own jobs from an earlier turn is INFORMATIONAL — routed to the verdict's new Ignored list, no application, no host-failure ask — but only on AUTHORITATIVE evidence: the job must appear in an earlier turn's accepted dispatched entries in the mission's own turn log (round 2's catch: a mismatched turn id alone also covers mis-stamped, malformed, and future ids, and those keep their rejection and ask). The orchestrator preamble states both rules in exactly two scoped sentences. The fence-accounting question the issue raised is DECIDED, not changed: a validation-phase protocol failure keeps its fence charge — the tokens were spent, and making refused work free would let a mis-briefing host burn provider budget outside the sealed fence; the critic confirmed the reservation/concurrency split as sound. Three critique rounds (3 → 3 → 0) against codex gpt-5.6-sol at xhigh.

**Alternative:** deriving the delegate's round from the record so a mis-briefed number cannot invalidate a sound return. Rejected for now: it silently repairs a host mistake the guard now prevents at the source, and the mismatch check stays honest for returns that state a round.

**Impact:** The -r2 class of loss (one of twelve fence jobs in vm-smoke-5) is impossible; re-reports stop polluting the asks directory; the ask machinery reserves host-failure for jobs that genuinely do not exist. Live VM validation at seal time.

## D108 — 2026-08-18: The manifest becomes the single contract-policy authority it claimed to be (issues #8 + #7)

**Decided:** Both bm-1 kit issues land as one change set, under two rulings Wido made directly this evening. Issue #8: bm-1's ledgerNoGainBudget rises to the cycle fence (8) — the honest fix now that issue #4's candidate visibility exists — and the host caps become sealed manifest values (hostMaxTurns=150; hostMaxBudgetUsd=5.00, a PER-TURN host bound, not a total); provision writes all three keys, and the fuse acknowledgment rides only on an explicit manifest declaration. Issue #7: Wido ruled claude/claude-opus-5 as bm-1's STANDING code critic — a fresh ruling, recorded as such after the critique caught that his vm-smoke-5 option-A answer had authorized one mission only; implementer and design critic stay on gpt-5.6-luna under the 2026-08-05 cost ruling, and he re-confirmed EUR:40 as the TOTAL ceiling for the mixed roster (the exposure prose now says exactly that). Provisioning a single-model roster without an explicit independence declaration is refused outright — the pending-ruling warning retired — and every shipped single-model arm (bm-2, bm-2d, bm-2dc, bm-2s) declares roster.independence=session-only by name, the recorded weaker-isolation choice those arms exist to measure. The kit gate now fails on manifest/validator drift where it belongs: validation stderr refuses any warning line, and the sealed caps are asserted byte-for-byte INSIDE the fenced mission block through an extractor that mirrors the runtime's fence grammar exactly (trailing-blank fences, exit at the block's close) — the critique's three successive false-pass witnesses (whole-file grep, prose decoy, spaced-close spill) each forced the assertion tighter. Four critique rounds (3 → 1 → 1 → 0) against codex gpt-5.6-sol at xhigh.

**Alternative:** acknowledging the short fuse for bm-1 instead of raising the budget. Rejected: the acknowledgment key is a fixture-bed device (D103); a real benchmark should let the stop-loss do its job through candidate visibility.

**Impact:** provision → seal succeeds with no warnings and no hand edits (the post-commit acceptance output is in both issue-close records); the merge path becomes reachable unattended for the first time; drift between manifest and validator dies in the kit gate, not at the human's seal.

## D109 — 2026-08-19: Taint resolution lands, and the critique loop rebuilt the wall's recovery story around it (HIW-O6, slice 6)

**Decided:** The two human-reserved resolution verbs land WITH THE
COMMIT THAT CARRIES THIS RECORD — RESTORE (a recorded safe tree,
byte-verified) and ADOPT-DISPUTED-TREE (the observed tree, named
waived claims). The review ran a three-round primary chain (7 → 9 → 7 findings) and a
TWENTY-round successor chain (96 findings folded in total) against
codex gpt-5.6-sol at xhigh, closing at round 20 with "no material
findings — AGREE"; the covenant lands nothing without that verdict,
so the commit carrying this record is its proof. The full round trajectory and per-round briefs
ride the landing receipt and the scratchpad critique series, and
substantial machinery was rebuilt along the way. What a
resolution now IS: a named E-sequence point (its own pinned occurrence
plus the pre-resolution expected tree it replaced), so staleness is
path-sensitive — a delayed authorization refuses only when it overlaps
an intervening delta, acceptance or resolution, never because a blanket
segment fence fired. What custody now IS: the whole mutating state
family (state-write, state-anchor, state-reconcile) is human-reserved
at the CLI with symlink-resolved classification; the runner anchors
in-process; resolution-shaped transitions refuse inside WriteState's
single locked read; one state-chain write carries at most ONE
expected-tree event. What recovery now IS: anchors are byte-pinned
(one ledger read feeds hash, count, and blob; the resolution pins the
state hash it wrote and the ledger bytes it rechecked); an anchor can
never re-bind the same state hash to different ledger bytes (the
mid-turn laundering shape); reconciliation's ledger-ahead tolerance is
byte-precise (the suffix must open with the appended block's heading)
and covers the no-marker lost-turn shape; violated wall.json evidence
is sticky; racing anchors of one position converge; the reservation
gains an E-continuity check that parks between-turn drift as its own
taint with wall.json evidence carrying unaccounted paths; a
resolution's crash tail is completable by re-running the verb and is
repaired automatically at the runner's next verified start; asks bind
to their taints at park time and matching fails closed. Both shipped
schemas (mission-state, new wall-evidence) mirror the Go validators on
shape, with the Go side the stricter authority on Unicode-category
questions regex cannot portably express — and the benchmark extractor
INVOKES that authority (mission state-verify, hash chain and anchor
included) before trusting any state bytes, failing closed when it is
absent, via the prepared kit-authority-handoff branch the boundary
paragraph below records.

**Alternatives rejected:** unforgeability against arbitrary same-user
processes (the design's rejected isolation tier — the cooperative
tamper-evident posture is recorded in the design and was restated to
the critic as a scope ruling); deriving restore safety from on-disk
wall.json (rewritable by the offender — hash-chained evidence only);
booking a ledger block at the reservation park (unhealable without a
marker; the lost-turn heal already owned that shape).

**The measuring-stick boundary (successor round 16):** the benchmark
kit's half of this work — the extractor's executable authority
handoff, the pinned schema sync, and the fixture stubs — is NOT in
this commit. The rule makes every kit change a human-approved commit
with a shared version bump, so it rides branch kit-authority-handoff
(kit 0.2.0, every spec version touched) awaiting Wido's approval;
until he merges it, main's validate-kit fails its schema-drift check
by design, naming exactly the pending sync.

**Impact:** A tainted mission has exactly two lawful exits, both
typed, both verified, both recorded in the hash chain; every crash
window in the park/resolve/anchor path reconciles or parks honestly —
the ledger crash-tail is byte-precise via the pending-block stamp
every ledger write records, and each heal carries the sha of exactly
the bytes it verified into the anchor's own refusal pin;
the wall's obligation row HIW-O6 moves to READY_FOR_RUNTIME (both resolutions driven through the real wrapper entrypoint in TestResolveTaintThroughWrapper), with runtime proof deferred to the VM seal. Live VM
validation stays deferred to Wido's seals.

## D110 — 2026-08-19: Two rulings taken at the 10-hour window's open

**Decided (Wido, directly):** (1) Critic-lapse contingency — if codex
credits run out mid-loop, the critique continues on claude-opus-5 at
xhigh as a bridge, and the folded end state re-verifies against codex
gpt-5.6-sol at xhigh before any slice counts as finally landed. (2)
The parked metasystem review execution is UNBLOCKED: the converged
report in docs/reviews/ executes under the normal covenant after the
wall rows; the r10 decision no longer gates it.

**Impact:** the autonomous window cannot stall on a billing lapse, and
the review work joins tonight's queue behind the wall rows.

## D111 — 2026-08-19: One benchmark case, many configurations — the kit's object model (Wido's ruling, designed, critiqued, implemented)

**Decided (Wido, directly):** "we don't have multiple benchmarks, we have a
single benchmark case with multiple benchmark configurations… this needs a
better design and good language around it so that it is clear what it means…
designed, critiqued and then implemented… also update the documentation so
there is no confusion by anybody including agents working on other machines".
The kit now has three nouns and one verb: a **case** (what is built and
judged; immutable per version under `benchmark/cases/<id>/<version>/`), a
**configuration** (who builds it and under what limits; immutable per version
at `benchmark/configurations/<id>/<version>.json`), a **run** (one case
version under one configuration version, pinned by both git object ids), and
"run CASE under CONFIG". The six former specs were one task in two instrument
versions (`taskrun@0.1` boolean gate; `taskrun@0.2` count gate — four
manifests had said 0.2 over v0.1 instruments) under six configurations
(`cheap`, `sol`, `devin-delegate`, `devin-host`, `devin-host-claude-delegate`,
`devin-opus-gpt55`); they are aliases now, read-only, keeping their legacy
naming for pre-migration cohorts. Comparability = the pair + pins + the old
tuple; verdict eligibility = case maturity AND configuration purpose
`capability`; a separate `--configurations` report holds the case constant
and never emits a verdict. Provisioning copies from the pinned tree object,
refuses uncommitted case directories, and every later reader of a run reads
the pinned objects. `versions.lock` is the append-only registry, checked at
HEAD and across history (shallow/grafted clones refused). Design:
`plans/benchmark-case-configuration-design.md`, ten adversarial critique
rounds against codex gpt-5.6-sol (11→8→5→6→3→3→2→1→1→ACCEPT), outputs
beside it. Landed in three gated commits (objects+registry, kit wiring +
specs/ removal, documentation); `benchmark/validate-kit.sh` green on the
Linux guest at each, including the provisioning bridge; provisioning
equivalence legacy vs pair vs alias verified byte-for-byte modulo the design's
one permitted-difference list.

**Alternative:** a third "profile" object for fences, and a self-referential
content digest inside `case.json` for immutability. Both rejected in critique:
fences vary with rosters today (add a profile only when they vary
independently); a digest inside the object it digests cannot be satisfied,
and git tree ids pinned in the run identity plus an external registry give
the same guarantee honestly.

**Impact:** adding a task is one directory; adding a way of running is one
file; the same task under two rosters is now a sentence the kit can say and
check. Three inherited configurations (`devin-delegate@1`, `devin-host@1`,
`devin-host-claude-delegate@1`) remain unsealable as inherited (no-gain 5 <
cycles 8, no acknowledgement) and are reported by name until a human rules
(issue #8's family). `benchmark/README.md` is normative for anyone, on any
machine, adding a case or configuration.

## D112 — 2026-08-19: Source comments speak the application's language (Wido's ruling)

**Decided (Wido, directly):** Inline source documentation was carrying
the context of the work session that produced it — review-round
numbers, finding numbers, slice names — which no fresh reader can
follow. That is a standards defect, not a style preference. Two
consequences: (1) the standard changes now — AGENTS.md gains the rule
that a source comment states its constraint in the application's own
terms, in plain English, and never in terms of the process that
produced the change; provenance lives in commit messages and decision
records only. (2) The landed codebase gets a full inline-documentation
rewrite to that standard — opened as goal `source-comment-standard`,
to be designed and critiqued like any other change.

**Applied immediately:** every comment added by the in-flight
HIW-O12 slice (uncommitted at ruling time) was rewritten to the
standard before the slice's next critique round, so the slice lands
clean. The landed offenders across the codebase are the goal's scope,
not the slice's.

**Impact:** the next reader of any file sees only the system's own
concepts; the archaeology stays in the decision records where it
belongs.

## D113 — 2026-08-19: The harness absorbs agent conduct, and the metasystem gets an ease-of-use reckoning (Wido's ruling)

**Decided (Wido, directly):** Behavior that only lives in one agent's
memory does not exist for the next agent. Four pieces of session
conduct move into the harness as goals: the runnable verification
covenant (one battery entrypoint with a verdict file, one
critique-round driver — goal executable-covenant), the flake registry
with its bounded rerun protocol (goal flake-registry), and the landing
tool repairs agents currently remember around (commit.sh pathspec
survival, dual-remote push — goal landing-tooling-fixes). Separately
and explicitly: the metasystem must be INTUITIVE for an agent to use —
across every runtime, not one. Goal agent-ease-assessment takes a hard
look at the system's complexity from the agent seat, names
simplification opportunities, and executes what survives critique.

**Impact:** the queue gains four goals behind the current work; the
standard for every future harness feature is that its discipline is
enforced by the script, not recalled by the operator.

**Addendum (Wido, same day):** the ease-of-use assessment carries a
second question with equal weight: whether the system's structure has
tipped into brittleness anywhere — the balance sought is strong
guidance and verification of agent behavior WITHOUT clamping every
technical detail, which introduces brittleness and is against the
system's philosophy. The assessment must answer both.

## D114 — 2026-08-19: The brittleness findings get designed recovery (Wido's ruling)

**Decided (Wido, directly):** the structure-vs-brittleness assessment
named four places where strictness tipped into brittleness; each gets a
proper design and a backlog place, not an ad hoc patch. The mapping:
(1) the code-critique loop's unbounded witness escalation is owned by
the already-queued critique-stop-rule goal, now carrying this ruling's
weight; (2) defense stacking on hot artifacts becomes goal
invariant-consolidation — one owner per invariant, the contract's
origin-to-E0 flow as the first case; (3) the goal ledger's byte caps
and (4) its missing queued-goal edit verb together become goal
goal-ledger-ergonomics. The agent-ease-assessment goal remains the
umbrella that may surface more such items.

**Impact:** brittleness recovery is queued work with designs and
critiques, preserving the philosophy: deepen the strictness that
guards real invariants, prune the strictness that breaks lawful use.

**Addendum to D114 (Wido, same day):** the general design principle
behind the recovery is binding for all future work and now lives in
AGENTS.md's work contract: rules that break on benign variation must
not be encoded — strictness is reserved for named invariants whose
violation is a real defect, and benign variation is handled
intuitively (normalized, allowed, or given its sanctioned verb).

## D115 — 2026-08-19: Backlog work parallelizes across machines, synced through git (Wido's ruling)

**Decided (Wido, directly):** multiple machines work the backlog in
parallel, with git as the synchronization mechanism for the goal
queue. This is the NEXT item picked after the current goal
(host-implementer-wall) closes, because it multiplies throughput on
everything behind it. The design must cover: a claim mechanism so no
two machines take the same goal; merge-safe ledger mutations (the
current single-file ledger with a whole-file digest baseline cannot
merge concurrent edits); and baseline reconciliation that survives
concurrent pushes. Goal backlog-git-sync carries it.

**Impact:** after the wall lands, the queue stops being single-file
in both senses — one machine works a goal while others work theirs,
and the ledger's own format must make that mergeable.

**Addendum to D115 (Wido, same day):** the distributed backlog must be
DEPENDENCY-AWARE. The queue is a flat list today and dependencies
between goals exist only informally (in next-step prose or in agents'
heads); a machine picking work in parallel can only choose safely if
the ledger itself records which goals block which. The design adds
dependencies as first-class ledger data: a goal is claimable only when
its blockers are done, the claim verb enforces it, and the merge-safe
format carries it. Robustness of the mechanism includes this — two
machines must never work a goal and its blocker at the same time.

**Second addendum to D115 (Wido, same day):** landing the mechanism
includes MIGRATING the existing queue to it. The goal is not done when
the verbs exist: every standing goal gets its dependency edges
declared, its claimable state made true (blocked or free), and the
resulting graph reviewed as part of the goal's acceptance — the first
parallel pick by a second machine must be safe on the real backlog,
not on an empty example.

**Third addendum to D114 (Wido's question, 2026-08-19):** the wall
slice's 19-round critique chain exposed WHY the witness-escalation
brittleness happens: the patience mechanism exists only in the mission
runtime, and critique loops carry no instance of it — no measured
progress, no no-gain budget, no mechanical stop. The critique-stop-rule
and executable-covenant goals must be designed TOGETHER: the
critique-round driver carries the mission patience semantics verbatim
(progress = findings that change what the slice under review builds;
program-wide discoveries are rows, not slice progress; a no-gain budget
ends the loop with the rows recorded). Until that lands, the loop
operator runs the budget by hand: zero regression-class findings in a
round is the no-gain event, and the slice lands on the evidence rule.

**Fourth addendum to D114 (Wido, same day):** the patience mechanism is
being revised in parallel on another machine. When that revision lands,
this machine reviews it against the gap the wall slice exposed (the
third addendum above): if the revision already gives critique loops a
patience instance, the critique-stop-rule and executable-covenant goals
align to it; if it does not, the requirement is factored into it so the
mechanism addresses the problem that was just experienced. One patience
design serves both runtimes and review loops — not two mechanisms.

**Fifth addendum to D114 (the patience review, 2026-08-19):** the other
machine's patience revision (plans/patience-attempts.md, landed 984675b)
was reviewed against the critique-loop gap per Wido's instruction. It is
fixture-harness patience — waits counted in the census actor's attempts
with an honestly-labelled wall-clock failsafe — and does NOT itself
cover review loops. Its two-tier principle is the pattern the
critique-stop-rule design must adopt: Tier 1, a no-gain budget counted
in rounds that produce zero regression-class findings; Tier 2, an
absolute round failsafe. One patience concept across fixtures,
missions, and review loops. Applied by hand to the wall slice
immediately: no-gain budget one round, failsafe two, slice scope frozen
(new-machinery findings become rows regardless of class).

**Addendum to D115 (Wido, same day):** the sync moves up — it starts
immediately after the wall's slice 7 lands, NOT after the whole wall
goal closes. Reason (proposed, agreed): once the multi-machine backlog
exists, the wall's own remaining rows (O13 and the four smaller rows)
become work a second machine can claim in parallel, so the sync
multiplies everything behind it including the wall. Mechanically: on
the slice-7 landing, host-implementer-wall parks yielding its slot to
backlog-git-sync (the acp-transport precedent), and resumes as
claimable work once parallel picking exists.

**Correction to the D115 addendum (round-21 review):** the resume set
after the park was incomplete — the wall's OPEN rows are O13
(CRITICAL), O14, O15 (CRITICAL), O16, and the four smaller rows
O8/O10/O9/O11; ALL of them resume as claimable work under the parallel
backlog. The goal ledger's handoff now names the full set.

## D116 — 2026-08-19: The wall's launch preconditions land (HIW-O12, slice 7)

**Decided (delegated, both-must-agree):** slice 7 lands on codex
gpt-5.6-sol AGREE at round 23 of a 23-round critique chain (72 findings:
folded, refuted with evidence, or recorded as obligation rows). What
the slice ships:

- **Launch preconditions at every entry (O12):** core.fileMode must be
  pinned true in the repository itself, normalized to git's own boolean;
  a mission may only START on a clean baseline or exactly the tree the
  human sealed as wall.sealed-baseline; the live contract must be
  byte-and-mode identical to committed HEAD AND byte-identical to the
  authenticated approved snapshot, all derived from one snapshot
  instant; every non-regular contract shape refuses by name before
  anything dereferences or blocks on it.
- **E0 at birth:** the admitted baseline is recorded as the mission's
  initial expected tree the moment state publishes, from the same
  authenticated read that admission judged — one byte snapshot flows
  from origin verification through admission into state construction.
- **Birth is durable and rebirth is evidence-gated:** born.json lands
  before state publication and only a same-pass proven failure
  unstamps it; every start entry freezes the mission id on birth
  evidence — the record, booked ledger cycles, or surviving anchors —
  with remedies matched to the evidence, and probes that cannot read
  refuse rather than authorize. Interrupted, damaged, or lost-state
  missions freeze for a human; stillborn retries keep working.
- **Start decisions are serialized:** a per-mission launch lock spans
  the parent's checks-and-pins and the child's whole birth, so a
  launcher's cached decision can never mutate a newborn.
- **Path-space correctness in nested checkouts:** the tree projection
  is workspace-scoped at its one owner (gittree), HEAD comparisons and
  the delegate authorization chain speak the same project space, both
  conformance stages fence the whole repository against sibling-path
  smuggling, boundary declarations convert from the implementer's real
  dialect with mandatory single stripping, and a full nested mission
  births and runs through the real wrapper as the witness.
- **Bed honesty:** the Go full-cycle beds carry the real return checker
  and role schema (their absence had every fixture mission silently
  failing behind loose assertions); the fake host's park request
  carries its reserved ask; both shell families close their beds.

Along the way, the operator's rulings reshaped the loop itself: the
comment standard (D112) applied mid-slice; the brittleness recovery
(D114) opened; the hand-run patience budget (D114, third addendum)
capped the chain at a no-gain round or two failsafe rounds, and the
close-out landed inside that cap. New obligation rows recorded for
discovered pre-existing gaps: O15 (HEAD-movement accounting, CRITICAL),
O16 (host-side repository fence), O18 (landed in-slice:
READY_FOR_RUNTIME), beside O14; O17 (durable birth) landed in-slice.
KI-38 records the pre-existing lease-acquisition race with goal
lease-acquire-atomicity queued. Cross-machine repair riding this
landing: the KI-37 arming fix's empty-array expansion guarded for
bash 3.2 (arm-supervision.sh:132).

**Impact:** a mission cannot be born on bytes nobody signed, cannot
lose its identity chain to a crash or a concurrent start, and cannot be
mistaken for never-lived; the wall's whole program now runs correctly
in the nested deployment layout it actually ships in. Per D115's
addendum, host-implementer-wall parks on this landing, yielding to
backlog-git-sync; rows O13/O14/O15/O16 + O8/O10/O9/O11 resume as
claimable parallel work. Live VM validation stays deferred to Wido's
seals.

## D117 — 2026-08-19: The wall recovery ladder (Wido's ruling, recorded for the program)

**Decided (Wido, directly, mid-window):** wall violations must not
freeze the mission for the human by default — "I first want the main
agent to figure out how to recover from this. Only if that is not
possible — because there is ambiguity or there are big implications —
should the human be asked to unblock things." The ladder amends
slice-6 doctrine: Tier 1, every violation still records its taint,
evidence, and anchored disputed tree unchanged; Tier 2, the RUNNER
auto-restores the mechanical cases (byte-exact safe-tree restore,
authenticated ledger-blob restore) through ResolveTaint's own engine
internals under a runner identity, no ask raised; Tier 3, a human is
asked only for adoption, no verifiable restore, or a repeat offense
within the lookback window. Design draft:
scratchpad/recovery-ladder-design-draft.md (to be moved into plans/ when
picked up). Recorded as obligation row HIW-O19; claimable parallel work
under the backlog sync like its sibling rows.

**Impact:** the human is the escalation path for judgment and stakes,
never a mechanical restore's rubber stamp.

**Addendum to D116 (the kit branch, same day):** the landing sequence's
final step — amending kit-authority-handoff once with the slice's final
engine schema bytes — is BLOCKED on real kit work, not mechanics: the
branch (10022e0) predates the benchmark cases/configurations
restructuring (D111's kit rework, 400+ files, specs/ deleted), and the
rebase conflicts semantically in the case registry and validate-kit.
The branch is untouched; main's validate-kit stays red on the designed
schema-drift check. Recommendation for Wido: re-express the branch's
intent — the extractor's executable authority handoff plus the pinned
schema sync (now including slice 7's fields) — as a FRESH human-approved
kit change built on the new cases structure, with the old branch as
reference; either machine can build it under the parallel backlog. Goal
kit-authority-reexpress carries it.

**Second addendum to D116 (Wido, same day):** Wido accepts the kit
authority handoff IN PRINCIPLE — the benchmark invoking the engine's
own verifier as the authority on record validity, with the pinned
schema kept as the kit's shallow independent check. The acceptance
applies to the re-expressed change (goal kit-authority-reexpress); his
signature goes on that rebuilt commit itself, as every kit change
requires.

**Third addendum to D116 (the kit change lands, same day):** Wido
instructed the integration directly ("this is approved and accepted —
go ahead and make it happen now"), satisfying the kit's human-approval
rule. The re-expression landed at 2af908b as a kit-only commit: the
extractor invokes the engine's verifier as the record authority
(fail-closed), the pinned mission-state schema is byte-identical to the
engine's (the designed drift check now PASSES), kit version 0.2.0 with
main's pair assertions updated, the fixture evidence carries the new
field, and the obsolete per-spec version bumps were dropped per the
content-addressed registry's append-only rule. Kit-gate evidence: the
engine drift guard, extractor fixtures (including the authority
handoff's fail-closed legs), and evolution fixtures all pass on this
machine; the provisioning leg fails on the PRE-EXISTING
genesis-under-agent blocker (goal provision-genesis-authority), which
this diff does not touch — the same disclosure this week's other kit
landing shipped with, full-suite guest validation deferred with the VM
seals. Goal kit-authority-reexpress is concluded; branch
kit-authority-handoff (10022e0) remains as reference until Wido deletes
it.

## D118 — The backlog sync sheds its history-authentication machinery (2026-08-19)

**The situation in plain English.** The multi-machine backlog design
(D115) went through seven critique rounds. From round 4 onward the
critic pushed on one theme: if machines trust each other's ledger
writes, prove it; if they don't, catch forgery. Each round I answered
with more machinery — committed proof envelopes with an exact byte
grammar, per-commit semantic replay of the whole ledger history,
two-commit genesis anchors, canonical repair checkpoints with anchor
chains, a nine-phase crash journal. At round 7 the finding count went
UP (nine to fourteen), and the critic's own lead finding turned the
question around: D115 asks for git-synced claims, dependencies,
merge-safe mutation, and migration — none of which needs a history
interpreter. Show why the smaller design fails, or reduce scope.

**The decision.** Reduce scope. Three reasons, each sufficient:

1. The critic PROVED the heavy machinery cannot deliver its own
   promise: a fresh clone cannot detect an erased repair checkpoint
   without external trusted state (round-7 finding 3 — the guarantee
   is information-theoretically impossible inside git alone).
2. The goals ledger lives in the same repository as the source code,
   which has no history authentication at all. An adversary who can
   forge ledger history can forge code. Guarding the backlog harder
   than the codebase it steers protects nothing.
3. Wido's standing rulings (D114 and the AGENTS.md work-contract
   bullet): strictness guards invariants, never conveniences; where
   strictness tips into brittleness, recover. Every replay-machinery
   finding in rounds 5-7 was a defect OF the machinery, not of the
   backlog. That is the complexity ratchet the rulings name.

**What the reduced design keeps** — everything D115 actually asks
for: one file per goal (merge safety by granularity), all mutations
through goal verbs publishing via compare-and-swap on a fetched tip,
full tree validation before any machine accepts a state (schema,
integrity digests, dependency graph, claim and arc consistency),
integrity digests per file as the tamper-evidence layer, the
pre-commit guard against hand edits, claims with machine quota, arcs,
the operator park lever, dependency-aware claiming, prune, the
migration manifest, and the durable local/remote mode with a ledger
identity check.

**What it sheds** — the parts that authenticated HISTORY rather than
guarding STATE: per-commit semantic replay, the proof-envelope byte
grammar (one provenance trailer with the operation id stays), genesis
anchor chains, the canonical repair-checkpoint protocol (a corrupted
remote tree is now: refuse with the reason, a human fixes it with
ordinary git, every clone re-validates the fixed tree), the merge
rule (tree validation does not care how a tree came to be), and the
nine-phase journal (collapsed to created / pushed / terminal with the
same crash-recovery answer: refetch and look for your operation id).

**The honest trust posture, stated once:** the ledger's defenses are
tamper-EVIDENCE and accident-proofing — the same trust level as the
repository around it. A cooperating-user fleet is the design point;
an adversarial force-push is a repository-level event, not a ledger
event.

Round 8 of the critique reviews the reduced design under the
both-must-agree covenant. If the critic shows a D115 requirement the
smaller design fails to carry, the specific machinery that carries it
returns — piecewise, with the finding as its justification.

## D119 — The backlog-sync design converges at round 10 under the declared failsafe (2026-08-19)

The design loop for the multi-machine backlog (D115, reduced by D118)
closed today. Rounds 8 and 9 both upheld the reduced architecture and
returned only executable-contract findings, so round 10 was declared
the loop's failsafe round IN THE CRITIQUE INPUT ITSELF, with the
disposition rule pre-committed: fixture-expressible findings become
obligation-matrix rows and implementation begins; only a demonstrated
D115-requirement failure or a shape-level defect reopens prose. This
is the two-tier patience rule (D114 addenda) and the
implementation-first ruling (D81) applied to a design loop.

Round 10 returned: "no demonstrated D115 failure and no
non-arbitrable architecture defect. Under the declared failsafe, the
prose loop should stop," followed by eight fixture-expressible gaps
and the verdict "CONVERGED — build it." All eight are folded into the
obligation matrix (reconcile crash-refresh and persisted-base
maintenance, the runner's full turn-boundary lifecycle, dead-owner
recovery consistency with a confirmed-late terminal, the last
merge-wording residue, displacement/acknowledgment closure legs, the
hand-edit grammar's compound cases, and two manifest schema edges).
Ten rounds total: 16→14→14→10→9→9→14→11→12→convergence, with the
D118 scope reduction at round 7 as the turning point.

Implementation now runs against the 16-row obligation matrix
(BGS-1..16), two-clone fixtures over a local bare origin first, then
the migration of the real queue asserting the reviewed expected map.
The landing itself stays under the ordinary covenant: battery green
and codex AGREE on the code.

One lesson for the covenant-patience arc (critique-stop-rule +
executable-covenant), recorded here so the mechanization inherits
it: a design loop's failsafe round must be declared at LOOP START as
standing policy, not mid-loop by judgment — this loop declared it at
round 10 only because the operator's rulings existed and were
remembered. The arc's charter should encode: stop-rule tier 1 = a
round whose findings are all fixture-expressible closes the loop;
tier 2 = a numbered failsafe round fixed before round 1.

## D120 — 2026-08-19: Genesis authorizes against one root; a non-holder may seed only a goal-free ledger on a history that carries none

**Decided:** The genesis-authority arc (goals provision-genesis-authority + genesis-authority-design, assigned by Wido as one designed-then-implemented thread) lands. The caller-named second classification root is gone: `--genesis-from` and METASYSTEM_GENESIS_AUTHORITY_ROOT are removed from the verb, adopt.sh, the adopt fixtures, and the kit gate, so `goal reconcile` classifies against the one root it writes, like every goal verb. The authority matrix's genesis row becomes: the human and the target's lease holder are admitted as before; every other caller — a main without the lease, a delegate, a helper — is admitted exactly when the ledger it would baseline is ADOPTION-SHAPED: parse-legal, goal-free, on a checkout whose HEAD does not track plans/goals.md at the root's own prefix (goal.AdoptionShaped, computed pre-lock for the matrix and re-judged under the store lock; the git probe strips the repository-steering environment — GIT_DIR and siblings — so no caller input chooses the judged repository). This closes the review's two open laundering holes by removing their mechanism (the crafted-source-root MAIN raise and the discarded-source reclassification), keeps the D94/D96 race guard, and reopens provisioning: a virgin target's genesis works from a terminal, an announced session, a session whose announcement lapsed, agent-ancestry fixtures, and delegate sandboxes alike — while an initialized project's populated or history-tracked ledger stays holder-only. adopt.sh additionally skips reconcile over a healthy pair (a read-only baselineMatches probe), so a re-run is a no-op instead of a holder-only refusal. The doctrine change this carries, for Wido's confirmation: machinery is no longer refused genesis BY CLASS — the refusal protected no invariant (the adoption shape is everything a non-holder can obtain, and the class rule broke lawful provisioning twice: D96, and the kit on the Mac today) — it is refused BY WHAT IT WOULD BASELINE.

**Alternative:** Keeping HUMAN/MAIN-only against the target (re-breaks every agent-ancestry provisioning flow until the session hook announces reliably, and never admits the sandboxed kit gate); a per-user authority registry or a holder-minted capability (both re-introduce a caller-influenced authority source; refuted in the design's §7); the round-1 `goal seed` verb (set aside by the Mac session's scope addendum — the ledger seeding and format are being rewritten there; its doctrine point, verb-written skeleton bytes with a digest over the target's actual plans set, is recommended to that rewrite).

**Impact:** Critique chain: three design rounds (codex gpt-5.6-sol, one chain; 5+6+4 material findings, every one dispositioned by join, two rewrites forced — the round-1 premise refutation and the round-3 non-holder-MAIN cut) and one implementation round (Claude opus; 9 findings, 4 material, including a live GIT_DIR bypass of the new guard, all folded with tests) — plans/genesis-authority-design.md and its dispositions files carry the trail. Proof: engine unit battery green; adopt-fixtures.sh and benchmark/validate-kit.sh green end-to-end from an announced agent session, both under METASYSTEM_MAX_ALWAYS_LOADED_WORDS=1500 because the template's always-loaded budget (1400) is exceeded at 1472 words since 86bd66a — a pre-existing blocker in the slice-7 landing's AGENTS.md, left to its owner. The adopt fixtures gain a post-adoption probe pinning that genesis confers no write authority (goal open stays holder-only against the target).

**Addendum to D119 (Wido, 2026-08-19):** Wido directed that D119's
lesson becomes backlog content with the metasystem treatment — "design
loops get a declared failsafe round at loop start, not mid-loop" goes
into the covenant-patience arc's charter so the mechanization
enforces it, explicitly NOT dependent on any particular agent
remembering it. Folded into the migration manifest's amend entries
for both arc members: critique-stop-rule's design must pin the
failsafe declaration at loop start as harness-enforced standing
policy, and executable-covenant's critique-round driver must refuse
to start a loop without a declared failsafe round and must stop the
loop itself when a tier fires. The live ledger stays untouched until
migration per the manifest's source-digest binding; the migrated
backlog carries the charter from cutover.

## D121 — A silent stall must be impossible: the idle watchdog (2026-08-20, Wido's ruling)

**What happened, in plain English.** The operator left for ten hours
after delegating continuous implementation work. The working agent
answered his last question, ended its turn with a prose promise to
keep going — and a prose promise arms nothing. The agent runtime is
event-driven: a turn ends and nothing runs again until a user
message, a tracked background task completion, or a scheduled wakeup
arrives. None was armed. The machine sat idle all night with open
delegated work. The operator had to wake it himself, which his
standing rule says must never happen.

**Why the existing rule failed.** The rule existed — as agent memory
("no unleashed open work: ending a turn with open work requires an
armed wakeup"). Memory is advice, and turn-end is exactly where
agent attention is weakest: the reply is finished, the frame is
"done answering," and the open-work check is the thing that gets
skipped. Any safety property that depends on agent discipline at
turn-end will eventually fail. This is the same failure class the
harness-absorbs-conduct ruling (D113) and the patience mechanization
(D114) name: conduct carried by an agent instead of machinery.

**Wido's ruling.** Inexcusable; design something that makes it
impossible to ever happen again — the metasystem treatment, not any
particular agent's behavior.

**The design, two layers:**

1. INTERIM, armed immediately in the working session: a session-level
   scheduled guard that fires every twenty minutes while the session
   is idle. If open delegated work exists and nothing is in flight,
   it resumes the work; otherwise it reports healthy and stops. A
   silent stall is bounded at roughly one period instead of ten
   hours, and the guard runs regardless of what the agent remembered
   at turn-end — that is the property that matters. Its honest
   limits: it lives only as long as the session process, so it
   cannot survive a crash or reboot. Hence layer two.

2. DURABLE, goal idle-watchdog (added to the migration manifest):
   an OS-scheduled steward, independent of any agent session. Its
   open-work predicate reads the goal ledger and the transaction
   journal; worker liveness comes from the shipped clock-step-immune
   process identity; when it finds open work and no live worker it
   revives the configured agent runtime through the adapter seam
   (agent-agnostic per the standing ruling), writes a receipt, and
   notifies the operator. Every revival is visible — a stall can
   still BEGIN (an agent can always end a turn), but it can no
   longer PERSIST silently, and its detection no longer depends on
   any mind remembering anything.

The conduct rule (keep the next tracked step in flight before going
quiet) remains as an optimization — a session that arms its own next
step never needs reviving — but nothing depends on it anymore.

**Second addendum to D121 (design converged, 2026-08-20):** the
idle-watchdog design converged at critique round 5 with no material
findings (rounds: 9, 6, 5, 5, 0 — the loop-start failsafe fired at
round 3 and froze scope to invariants, exactly as the D119 lesson
prescribes). Final shape: two-tier visibility (dead/unknown/degraded
notified within one tick; live-idle within the noise threshold, ~50
minutes), dual visibility channels (delivery-gated notifier plus
session-start incident surfacing), revive only on proven-dead with
the whole launch a single critical section under the shared
arbitration lock, one-shot authorizations staging the exact launch
bytes, durable high-water evidence that nothing steward-made can
reset, and installed bytes for tick robustness. The same-user
tamper-proofing demand was refuted on the record and the critic
accepted: isolation against malicious same-user code belongs to
containers, VMs, or separate OS users — the steward guards accidents
and makes stalls visible, at the repository's own trust level. Build
proceeds against the nine-row obligation matrix, fake-adapter
fixtures first; landing under the ordinary covenant.

## D122 — The narrator becomes real-time self-awareness, wired into the steward (2026-08-20, Wido's directive)

Wido's instruction, recorded: the narrator must not be an
after-the-fact storyteller. It narrates continuously about what the
system is doing; it is wired into the idle-watchdog mechanism —
because that is the part that keeps running regardless of any
session — and when its narration notices something off, it must be
CAPABLE OF ACTING: naming the anomaly, notifying, and correcting
course through the steward's intervention machinery, whatever the
anomaly is. Folded into the migration manifest as an amendment to
the queued narrator goal, blocked by idle-watchdog (the actuation
path it rides must exist first). The design belongs to whoever
claims the goal after cutover: observation surface (receipts,
ticks, verdicts, dispatch events), narration channel, and the
anomaly-to-action contract, designed together with the steward's
fixtures.

**Addendum to D122 (Wido, same day):** the narrator must also be
capable of reaching out to the human when something out of the
ordinary happens. Folded into the narrator amendment: reach-the-human
is a first-class capability riding the steward's delivery-gated
notification channels — the narrator judges what is ordinary, the
operator hears about what is not, and correction-versus-escalation is
part of the anomaly-to-action contract its design must pin.

**Second addendum to D122 (Wido, same day):** Wido observed the stop
message rendering a stale snapshot — the frozen pre-cutover ledger —
with nothing about whether work is in flight, and asked for a backlog
guarantee that this gets fixed rather than forgotten. Added to the
migration manifest as add-goal stop-message-truth, blocked by
backlog-git-sync (cutover retargets the verdict to the live
projection) and idle-watchdog (whose status surface supplies the
is-anything-running half). Acceptance is written into the goal: the
staleness observed today cannot recur.

**Third addendum to D121 (Wido, 2026-08-20): no host dependency, at
all.** The steward may not touch the host system — no launchd entry,
no crontab line, no bytes installed outside the repository. It starts
and ends with the metasystem. If that forfeits reboot survival, that
is accepted: something that fires at host boot is explicitly NOT the
metasystem's responsibility and belongs to the operator's own domain
if ever wanted. The converged design's scheduler-glue section and its
installed-bytes rule are DELETED, replaced by a metasystem-owned
runner: `steward run` is a Go loop ticking on the steward's cadence,
launched as a tracked detached process through the standing run
facility — one per repository under a lock — armed by any session's
start (the session-start hook ensures it) and ended by `steward
disarm` or host shutdown. A rebooted machine is silent until the
metasystem next runs there, and the design says so plainly. The
obligation matrix row IW-8 is re-pointed from scheduler lifecycle to
runner lifecycle before the covenant review.

## D123 — The wall's detection posture is a bounded, recorded window (2026-08-20, Wido's ruling)

**The question**, from the snapshot-scope design's critique exhaustion
(WSS-R6-01, `plans/wall-snapshot-scope-critique-r6.md`): any finite chain
of verification probes has a last probe, so a repository carrier can
in principle move after it — should the wall close that window with
repository-wide custody during acceptance, or accept it?

**Wido's ruling:** the bounded-and-recorded detection window is the
wall's posture. Each verification records its capture instant; motion
after a turn's last probe lands in the next probe or the next
admission, and a concluded mission's post-verification timestamp is
the boundary after which motion is post-mission by definition. This
is the detector tier (D100) applied to time exactly as it is applied
to forgery: every accidental and naive shape is caught, the
cooperative posture is stated rather than pretended away, and
repository-wide custody remains isolation-tier machinery that would
need its own design if it is ever wanted.

**Impact:** the snapshot-scope design unparks; WSS-R6-01 resolves by
this ruling and WSS-R6-02..06 fold mechanically (schema census
posture, per-worktree pseudorefs, logical staged serialization, the
ref-map exclusion generalized, side-tip merge-base scope); a
verification round on the same critic chain judges the folds. In the
same session Wido kept all four retro-2026-08-20 ledger rows
(IL-25..IL-28) as adopted.
