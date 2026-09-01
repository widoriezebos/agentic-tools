# Epoch-drift identity design: the kernel token is the comparator, the epoch is display

- Goal: vm-epoch-identity-drift (design leg; Ruling R enumeration included)
- Mode: design, critique-ready
- Author: implementer job epoch-drift-design-r1e, 2026-09-01
- Status: PROPOSED — awaiting design critique

## 1. The defect, traced to its source

Process identity on this system is recorded as `(pid, pidStartedAt)` plus,
on Linux, the exact pair `(pidStartTicks, bootId)`. The epoch second
`pidStartedAt` is DERIVED on every Linux probe, not read:

- `internal/identity/identity_linux.go:58` — `started = time.Unix(boot, 0) +
  startTicks/USER_HZ`, where
- `internal/identity/identity_linux.go:123-134` (`bootTimeEpoch`) re-reads
  `btime` from `/proc/stat` on EVERY probe.

The kernel computes `btime` as (current wall clock − time since boot). On a
clock-disciplined VM guest (chrony/NTP on m0, a Debian aarch64 guest), wall
clock adjustments move `btime`, so the derived start second of a process
that has not moved at all changes between probes — while `startTicks`
(field 22 of `/proc/<pid>/stat`, a monotonic tick count fixed at spawn) and
`bootId` (`/proc/sys/kernel/random/boot_id`, fixed per boot) cannot change.
`btime` is whole seconds, so even sub-second slew flips the reported second
at a boundary. This matches both strikes exactly: recorded epoch +1s over
every fresh probe, pid/startTicks/bootId identical, twice in ~30 hours.

The two strike refusals map to code as follows:

- Strike "census CENSUS-FAILED → dispatch admission refused":
  `internal/census/run.go:435-478` (`readSupervisionSnapshot`) loads only
  `pid`/`pidStartedAt`/`instanceTag` from `state.json`, discarding the
  recorded pair, and `internal/census/run.go:491`
  (`verifySupervisionSnapshot`) then calls the seconds-only
  `identityAlive`, emitting `supervision-not-live` on a one-second
  disagreement. Dispatch admission requires a SUCCESS census and refuses.
- Strike "the recorded owner identity is not live" (verbatim,
  `internal/supervise/verifyarmed.go:80`): `armedIdentityAlive`
  (`verifyarmed.go:32-44`) calls `census.Alive(pid, start, probe)`
  (`internal/census/verbs.go:17`), a seconds-only wrapper, even though the
  owner record on disk carried the matching pair.

The repository is MID-MIGRATION to pair-based comparison: `identity.Compare`
(`internal/identity/identity.go:122-138`) already decides by
`(startTicks, bootId)` when the ref carries it, and roughly half the
consumers were converted (each carries the comment "the pair decides when
both sides carry it"). The strikes came from the unconverted residue: sites
that compare epochs directly, and loaders/writers that drop the pair a
record already carries. Section 5 enumerates every site (Ruling R).

## 2. Decision

**Chosen: the stable kernel token is THE comparator; the epoch second is
display and legacy fallback only.** This is the brief's a-priori-strongest
option, and the trace confirms it: the codebase has already converged on
this rule in half its sites, the kernel already provides exactly the frozen
token a "canonical time source" would have to invent, and the two strikes
plus the terminate-flake cost curve all live in the unconverted residue.
The design completes the migration, centralizes the rule so a caller can no
longer hand-roll a seconds equality, and adds a recurrence guard.

The comparator law, per platform:

1. **Linux:** identity is `(pid, startTicks, bootId)`. `pidStartedAt` is
   written for display and cross-version readability, and is NEVER compared
   when the pair is available on the side that carries fewer facts.
2. **Darwin:** identity is `(pid, pidStartedAtExactMicro)`. The Darwin
   microsecond token is read from the kernel's per-process start record,
   which is written once at spawn and not re-derived from the current wall
   clock, so it does not drift. Unchanged.
3. **Legacy fallback:** a record carrying neither exact shape compares by
   exact whole seconds, unchanged — no tolerance is added (section 7,
   rejected alternative A, explains why). Legacy records remain
   drift-vulnerable until natural turnover; section 6 bounds that window.
4. **Fixture identities** (controlled test clocks) keep the seconds rule by
   declared design; fixtures never carry a pair.

Three subsidiary laws make the rule total:

- **One comparator, one owner.** All probe-vs-record joins go through
  `identity.Compare`/`AliveRef`/`AliveTaggedRef`/`Custodian`; all
  record-vs-record joins go through a new `identity.SameRef(a, b Ref)
  Comparison` (specified in section 4.1). No package outside
  `internal/identity` may compare start identities field-by-field.
- **Identity flows by copy, never by re-derivation.** A component that
  writes another process's identity into a record copies the bytes from
  that process's existing record (announcement, owner.json, job record). A
  component may re-probe only to VERIFY through the comparator, never to
  mint a second recorded value for the same process. (This is what makes
  the SAFE-STRUCTURAL sites in section 5 safe, and it is the rule the
  supervision-hook already follows at `scripts/agents/supervision-hook.sh:125`.)
- **Every writer persists the strongest identity it observed.** On Linux
  that is the pair; schemas that today persist only `(pid, pidStartedAt)`
  gain `pidStartTicks`/`bootId` (omitempty), enumerated in section 4.2.

## 3. Consequence classes (why the residue matters beyond refusals)

The residue does not only produce false refusals against live processes
(the observed strikes). Because `identity.Compare`'s mismatch verdict is
"the pid was reused: definitively gone"
(`internal/identity/identity.go:212-215`), a drifted second in legacy mode
produces **false Dead**, and false Dead AUTHORIZES things:

- lock takeover of a live holder's lock (census writer lock,
  `internal/supervise/censuslock.go:83` — two concurrent census writers);
- registry slot classification `DeadOwner` for a live owner
  (`internal/registry/slots.go:70`);
- kill-less group-death conclusions in the mission runner's drain
  (`internal/missionrunner/drain.go:245,401` via `identity.Custodian`).

And the inverse direction, false NOT-OURS, makes kill proofs refuse and
leak survivors (`internal/janitor/killproof.go:208`), which is the
root-cause lead for the missionrunner terminate-flake family
(`plans/goals/missionrunner-terminate-flake.md`): the wind-down family's
"leaks groups" reds are the cost curve of refused kill proofs. This lead
is plausible but NOT proven in this sandbox (gap G2, section 10); the
argv-sandwich path (`identity.VerifyProcess`,
`internal/identity/verification.go:53`) compares two fresh probes and is
already pair-immune, so the drift exposure in that family is confined to
the seconds sites named in section 5.

## 4. Specification

### 4.1 The identity core (`internal/identity`)

1. Add `SameRef`:

   ```go
   // SameRef is the one record-vs-record equality rule. The strongest
   // representation BOTH refs carry decides: linux pair, darwin micro,
   // else exact legacy seconds. Mixed representations that share no
   // exact shape fall back to seconds only when both sides carry a
   // positive StartedAtSec; otherwise the comparison is invalid.
   func SameRef(a, b Ref) Comparison
   ```

   Mechanical rule: `a.Pid != b.Pid` → no match. If both refs' `Mode()` is
   `CompareLinuxTicksBootID` → compare pair. If both are
   `CompareDarwinMicroseconds` → compare micro. Otherwise, if both
   `StartedAtSec > 0` → compare seconds, `Mode = CompareLegacySeconds`.
   Otherwise `CompareInvalid`, no match. (Note this deliberately differs
   from `dispatch.sameRecordedIdentity`, which refuses on mode mismatch; a
   pair-carrying record and a legacy record for the SAME process must still
   be joinable by their common seconds — both are records, neither is a
   probe, so drift cannot separate them unless they were written from
   different probes, which the flows-by-copy law forbids going forward.)
2. Change `Custodian` (`internal/identity/custodian.go:24`) to accept a
   full `Ref` instead of `(pid, start int64)`, and compare through
   `sameIdentity(exact, ref)` so the pair decides when the caller's record
   carries it. The argv tag-hit override at `custodian.go:50-58` stays: it
   is the mitigation for legacy records.
3. `Compare`, `AliveRef`, `AliveRefComparison`, `AliveTaggedRef`,
   `SameIdentity`, `VerifyProcess` are unchanged — they already implement
   the law.
4. `dispatch.identityRefFromObject` (`internal/dispatch/ownership.go:99`)
   moves (verbatim behavior) into `internal/identity` as the ONE canonical
   JSON-object→Ref loader, so census/steward/report/mission loaders stop
   hand-rolling field extraction and silently dropping the pair. Existing
   duplicate loaders convert to it.

### 4.2 Writers: schemas that gain the pair (Linux; all `omitempty`)

| Schema | Site | Fields added |
| --- | --- | --- |
| `lock.Identity` | `internal/lock/lock.go:31` | `pidStartTicks`, `bootId` (carried by every lock: census writer, acknowledged, gaterun guard) |
| supervision component ledger record | `internal/supervise/ledger.go:75` | persist `held.Identity.StartTicks`/`BootID` beside `pidStartedAt` |
| `registry.ProcessRef` and armed/custody records | `internal/registry/reduce.go:29`, `internal/registry/records.go:143,172,207` | `ownerPidStartTicks`/`ownerBootId`, `pidStartTicks`/`bootId`, `custodianPidStartTicks`/`custodianBootId` |
| gaterun gate marker | `internal/gaterun/gaterun.go:117-121` | `pidStartTicks`, `bootId` |
| gaterun execution-guard member/holder | `internal/gaterun/guard.go:26,32` | `pidStartTicks`, `bootId` |
| proofrun execution-guard member + `SuiteIdentity` input | `internal/proofrun/watchdog.go:246` and the cmd-layer caller that builds `WatchdogOptions.SuiteIdentity` | `pidStartTicks`, `bootId` |
| watcher scan attestation | consumed at `internal/report/scan.go:281` (`WatcherPid`/`WatcherStart`) | `watcherStartTicks`, `watcherBootId` |
| acknowledged process entry | `internal/supervise/acknowledged.go` (`AcknowledgedProcess`) | `pidStartTicks`, `bootId`; on Linux these REPLACE the role of `pidStartedAtExactMicro`, which is btime-derived and therefore drifts (see 5, supervise block) |
| wrapper token | `internal/validate/wrappertoken.go:57` | `wrapperPidStartTicks`, `wrapperBootId` |
| watcher restart request | `internal/supervise/watcher_repair.go:101` | `pidStartTicks`, `bootId` (dedupe join then uses `SameRef`) |

Readers of every schema above accept records WITHOUT the new fields
(legacy fallback), so mixed fleets and old records keep working. No reader
ever requires the pair.

Already-correct writers (verified, no change): announcements
(`internal/lease/verbs.go:186-190`), job records
(`internal/dispatch/ownership.go:84-96`), owner.json/state.json
(`internal/supervise/disk.go:77-83,118-133`), run records
(`internal/run/run.go:169-171`), steward runner/identity/evidence records
(`internal/steward/runner.go`, `health.go:1022-1026`), goal journal owner
(`internal/goal/journal.go:167`), goalrevision lock holder
(`internal/goalrevision/lock.go:53-54`), lease records
(`internal/lease/claim.go:352`), mission runner records
(`internal/missionstate/missionstate.go:118-121`).

### 4.3 Readers and comparison sites: the conversion list

Every VULNERABLE site in the section 5 table converts as stated in its
"required change" column. The two shapes of conversion:

- **VULNERABLE-DROP** (record carries the pair; the loader or wrapper
  drops it): load the pair and pass a full `Ref`. No schema change.
- **VULNERABLE-PROBE** (fresh probe compared to recorded seconds by hand):
  route through `identity.Compare`/`AliveRef`/`Custodian`/`SameRef` with
  the fullest ref the record carries. Where the record is a schema from
  4.2, the schema change lands first.

Order of landing (each step independently shippable, no flag days):
(1) identity core additions (4.1); (2) VULNERABLE-DROP conversions —
these alone close both observed strike paths; (3) schema additions (4.2)
writers-first; (4) remaining VULNERABLE-PROBE conversions; (5) the
recurrence guard (4.4) last, with its allowlist emptied by then.

### 4.4 The recurrence guard (the mechanism repetition earned)

A validation-suite conformance test (mirroring the existing wiredoc/grep
ratchet style) fails when a Go file outside `internal/identity` contains a
direct start-identity equality — pattern class: `PidStartedAt ==`,
`== .*PidStartedAt`, `StartedAtSec ==`, `StartedAt.Unix() ==`,
`.StartedAt != `, `StartTicks ==` outside the core — with an explicit
allowlist for the deliberate residue (fixture-only sites, presence checks,
and SAFE-STRUCTURAL same-record dedupe joins converted to `SameRef` at
leisure). The allowlist is a ratchet: additions require touching the test.
This is the cheapest mechanism that makes the THIRD strike of this class
impossible to write silently; KI-24 (split identity, 2026-08-07) and these
strikes are the two paid lessons.

## 5. Ruling R: every caller of the comparison, with verdict

Verdict key — **SAFE-PAIR**: already decides by the pair/exact token.
**SAFE-STRUCTURAL**: record-vs-record where both values descend from one
recorded observation (flows-by-copy), so drift cannot separate them.
**FIXTURE**: fixture-only seconds by declared design. **VULN-DROP** /
**VULN-PROBE**: as defined in 4.3. **DISPLAY**: value shown, never compared.

### identity core
| Site | What it is | Verdict / required change |
| --- | --- | --- |
| `internal/identity/identity.go:122` `Compare` | the one probe-vs-record rule | SAFE-PAIR (linux mode at :133; darwin :129; legacy :135 by design) |
| `internal/identity/identity.go:187,194,223` `AliveRef`/`AliveRefComparison`/`AliveTaggedRef` | liveness over `Compare` | SAFE-PAIR given a full ref; every caller listed below |
| `internal/identity/custodian.go:24,61` `Custodian` | pid+seconds+tag liveness | VULN-PROBE (false Dead when no tag hit) → full-Ref signature, 4.1.2 |
| `internal/identity/verification.go:53` `VerifyProcess` | start/argv/start sandwich | SAFE-PAIR (two fresh probes; pair rides both) |
| `internal/identity/fixture.go` probes | fixture identity table | FIXTURE |

### census
| Site | Verdict / required change |
| --- | --- |
| `internal/census/run.go:296` `sameProcessIdentity` (custody/announced classify join at :309,:314) | SAFE-PAIR |
| `internal/census/run.go:380` run-owner leader join | VULN-DROP (run record carries pair, `internal/run/run.go:170`) → join via `identity.SameRef`-shaped process/record compare |
| `internal/census/run.go:435-478` `readSupervisionSnapshot` | VULN-DROP (drops pair from state.json) → load pair; **strike-1 feeder** |
| `internal/census/run.go:491` `verifySupervisionSnapshot` → `identityAlive` | VULN-PROBE → `alivePair` with loaded pair; **strike-1 refusal site** |
| `internal/census/run.go:512-543` `identityAlive`/`alivePair` | SAFE-PAIR when pair supplied; seconds wrapper remains for legacy/fixture |
| `internal/census/run.go:545-598` `liveCustody` | VULN-DROP (drops pair from job/mission-turn records) → load pair |
| `internal/census/run.go:640-694` `announcementsList` | SAFE-PAIR (both the enumerated-map branch :665-670 and the direct-probe branch :675-681) |
| `internal/census/run.go:673` fixture branch | FIXTURE |
| `internal/census/verbs.go:17` `Alive` | seconds-only wrapper; every caller converts to `AlivePair`/full ref (callers: `verifyarmed.go:33`, `watchdog.go:123`, `contract.go:1451`); wrapper then serves legacy/fixture only |
| `internal/census/verbs.go:26` `AlivePair` | SAFE-PAIR |
| `internal/census/verbs.go:45` `AuthIdentity` | returns the pair (`ProcIdentity.PidStartTicks/BootID`); consumers below |

### supervise
| Site | Verdict / required change |
| --- | --- |
| `internal/supervise/verifyarmed.go:32-44,79,103` `armedIdentityAlive` | VULN-PROBE + VULN-DROP (state records carry the pair; the signature is pid+seconds) → thread the pair through; **strike-2 refusal site (:80)** |
| `internal/supervise/arming.go:142-150` `sameArmingOwner` | SAFE-PAIR |
| `internal/supervise/arming.go:153-158` `ownerLiveness` | SAFE-PAIR (full ref + tag) |
| `internal/supervise/arming.go:369` component teardown `AliveTaggedRef(held.Identity, …)` | SAFE in-process; VULN-DROP across restarts because the component ledger persists only the second (`internal/supervise/ledger.go:75`) → 4.2 ledger change |
| `internal/supervise/proc.go:133,182,189,199` wind-down liveness over `held.Identity` | same as above: SAFE once the ledger persists the pair |
| `internal/supervise/censuslock.go:70-90` census-writer lock probe | VULN-PROBE with false-Dead TAKEOVER hazard (two live census writers) → `lock.Identity` gains pair (4.2), probe passes full ref |
| `internal/supervise/acknowledged.go:137-143` `silencedByAcknowledgement` | VULN-PROBE twice: the `(pid, startSec)` index key joins a fresh census second against a recorded one, and `pidStartedAtExactMicro` is btime-derived micro on Linux → key becomes pid-only with `SameRef` confirmation; exact token becomes the pair on Linux (4.2) |
| `internal/supervise/acknowledged.go:188-201` acknowledger lock probe | VULN-PROBE → full ref via 4.2 |
| `internal/supervise/acknowledged.go:221-230` entry replace/expiry (`Custodian`) | VULN-PROBE (false-dead expiry re-nags an acknowledged live process) → full-Ref `Custodian` |
| `internal/supervise/watchdog.go:110-126` steward-side component liveness | VULN-DROP (reads state.json pid/seconds only) + VULN-PROBE (`census.Alive`) → load pair, `AlivePair`; false alarm restarts live components |
| `internal/supervise/watcher_repair.go:100-108` restart-request dedupe | SAFE-STRUCTURAL (both sides copied from state.json); converts to `SameRef` under the 4.4 ratchet |
| `internal/supervise/disk.go:97,166,298` `Currency`/`StateNamesSelf`/intent target | SAFE-STRUCTURAL (records written by this process from its one captured `Self`; intent writers copy from owner.json) — the flows-by-copy law is what keeps these safe; converts to `SameRef` |
| `internal/supervise/verifyarmed.go` fixture probe path | FIXTURE |

### up and the session hook
| Site | Verdict / required change |
| --- | --- |
| `internal/up/up.go:137` `sameAuthenticatedProcess` | DEFECTIVE ORDER: seconds gate runs BEFORE the pair check, so a matching pair cannot rescue a drifted second → pair decides first, seconds only when either side lacks it |
| `internal/up/up.go:200` explicit `--pid/--start-time` vs fresh probe | VULN-PROBE — the hook replays the RECORDED start (`scripts/agents/supervision-hook.sh:125,149`), the probe re-derives it → verify by pid liveness + pair from the parent announcement when available; seconds only for pairless callers |
| `scripts/agents/supervision-hook.sh:114-149,340-349` | writer/transport only; copies recorded identity (flows-by-copy) — no comparison; DISPLAY |

### lease
| Site | Verdict / required change |
| --- | --- |
| `internal/lease/classify.go:166-173` announcement finder | SAFE-PAIR |
| `internal/lease/classify.go:248,277,281` `procKey{pid, pidStartedAt}` maps (supervision + job-adapter custody), joined against the caller's fresh-probe identity | VULN-PROBE (drifted caller misclassifies: a delegate adapter or supervisor unrecognized) → key on pid, confirm with `SameRef`/pair |
| `internal/lease/claim.go:85` holder liveness `LiveRef` | SAFE-PAIR (`internal/lease/identity.go:84-88`) |
| `internal/lease/claim.go:348` `sameLeaseProcess` | SAFE-PAIR |
| `internal/lease/claim.go:366-383` `holderIsMissionRunner` | SAFE-PAIR |
| `internal/lease/verbs.go:227` `Retire` join | VULN-PROBE on the ancestor-derived path (`up.go:182` re-probes; announce may hold an older second) → match by pid+session, confirm `SameRef` |
| `internal/lease/verbs.go:182` mainId minting | see section 8 (KI-33 coupling); minting is guarded by the SAFE-PAIR finder at classify.go:166, which is what prevents drift-triggered re-minting (the KI-24 split-identity recurrence) |

### steward
| Site | Verdict / required change |
| --- | --- |
| `internal/steward/health.go:457-458,485,514,650,882` component/runner liveness | SAFE-PAIR (loader `health.go:1022-1026` keeps the pair) |
| `internal/steward/health.go:1029-1037` `sameComponentProcess` | SAFE-PAIR |
| `internal/steward/runner.go:562-570` runner record vs probe | SAFE-PAIR |
| `internal/steward/universe.go:100-105` worker-gone proof | SAFE-PAIR |
| `internal/steward/component_evidence.go:367` | presence validation only; DISPLAY |
| `internal/steward/alert_episode.go:40,370` invoker identity | presence validation + record; DISPLAY (no comparison found) |

### dispatch
| Site | Verdict / required change |
| --- | --- |
| `internal/dispatch/ownership.go:99-115` `identityRefFromObject` | SAFE-PAIR loader; promotes to `internal/identity` (4.1.4) |
| `internal/dispatch/ownership.go:120-135` `sameRecordedIdentity` | SAFE-PAIR record-vs-record; converts to `identity.SameRef` (behavior note in 4.1.1) |
| `internal/dispatch/custody.go:63` | SAFE-PAIR |
| `internal/dispatch/custody_death.go:227-242` `processDefinitelyPredatesSupervisor` | SAFE-PAIR in pair/micro modes; the legacy branch (:238) is an ORDERING on seconds where ±1s can only flip an exact-boundary case toward "keeps the marker standing" (the safe direction) — accepted |
| `internal/dispatch/custody_death.go:244-255` `recordedRefLiveness` | SAFE-PAIR |
| dispatch census admission gate | consumes the census verdict; healed transitively by the census fixes |

### missionrunner and janitor
| Site | Verdict / required change |
| --- | --- |
| `internal/missionrunner/host.go:109,143` + `proc.go:111` wind-down `groupOwnership` | SAFE-PAIR (argv sandwich over `VerifyProcess`; the "not provably ours"/"provably foreign" refusals are argv-positional, not epoch) |
| `internal/missionrunner/drain.go:245,401` kill-less death proofs via `identity.Custodian` | VULN-PROBE (false Dead concludes a live group) → full-Ref `Custodian`; primary suspect for the terminate-flake lead, unproven (gap G2) |
| `internal/missionrunner/status.go:66-70` | DISPLAY |
| `internal/missionstate/missionstate.go:118-121` runner-record liveness | SAFE-PAIR |
| `internal/janitor/killproof.go:129` `GroupOwnership` | SAFE-PAIR |
| `internal/janitor/killproof.go:206-210` `Killable` recorded-identity gate | VULN-PROBE (`registry.ProcessRef` is seconds-only; false NOT-KILLABLE leaks survivors) → `ProcessRef` gains pair (4.2), gate uses `identity.Compare` |

### run, proofrun, gaterun
| Site | Verdict / required change |
| --- | --- |
| `internal/run/conclude.go:100`, `verbs.go:296`, `waiter.go:124,207` | SAFE-PAIR (full refs from run records) |
| `internal/run/waiter.go:176-184` cleanup self-match | SAFE-PAIR |
| `internal/run/run.go:550` | presence check; DISPLAY |
| `internal/proofrun/watchdog.go:132,221,294` suite liveness over `SuiteIdentity` | VULN-PROBE (caller-supplied seconds-only ref) → `SuiteIdentity` carries pair (4.2) |
| `internal/proofrun/watchdog.go:268` self-exclusion join, `:271` member refs | VULN-PROBE (worst case: fails to exclude self, then the mismatch makes `signalAuthenticated` refuse — leak direction, not wrong-kill) → members carry pair (4.2) |
| `internal/gaterun/fence.go:120-133` ancestry re-confirm | SAFE-PAIR (chain captured from probes with pair) |
| `internal/gaterun/fence.go:136-144` `matchesRef` controller join | VULN-DROP for pairless marker records → marker schema (4.2) |
| `internal/gaterun/gaterun.go:112-130` stale-marker pruning | pid-only `provablyGone` + seconds marker → marker schema (4.2); prune only on pair/`Compare` death |
| `internal/gaterun/guard.go:91` lock identity, `:110,136` `livenessOf` | VULN-PROBE (false-Dead prunes a live guard member / seats a successor) → pair via 4.2 |
| `internal/gaterun/guard.go:165,206,319` member joins | SAFE-STRUCTURAL (same-origin recorded values); `SameRef` under the ratchet |

### registry, report, contract, goal, validate
| Site | Verdict / required change |
| --- | --- |
| `internal/registry/slots.go:70` slot owner liveness | VULN-PROBE (false `DeadOwner` invites acting on a live owner's slot) → pair via 4.2 |
| `internal/report/scan.go:218-226` run probe state | VULN-DROP (run record carries pair; ref built without it) → full ref |
| `internal/report/scan.go:281` watcher attestation liveness | VULN-PROBE (attestation is seconds-only) → attestation schema (4.2) |
| `internal/contract/contract.go:1451` mission process liveness | VULN-PROBE (`census.Alive`) → `AlivePair`, record already carries what dispatch wrote |
| `internal/contract/contract.go:1490-1493` fixture table | FIXTURE |
| `internal/goal/journal.go:158-190` owner liveness + `callerIsOwner` | SAFE-PAIR |
| `internal/goalrevision/lock.go:90-110` revision-lock holder | SAFE-PAIR (pair preferred at :94; nano/seconds fallbacks for legacy holders remain, false-Dead risk confined to legacy holders until turnover) |
| `internal/validate/wrappertoken.go:66-70` wrapper ancestry token | VULN-PROBE (`tree.StartedAtSec` fresh vs recorded second) → token carries pair (4.2), compare via `Compare` |

Callers I could NOT trace to a verdict are listed as gaps in section 10.

## 6. Migration: records already on disk with drifted epochs (four machines)

The decisive fact: a drifted epoch in a record is HARMLESS once every
reader prefers the pair, because the pair in the same record is correct.
Drift never corrupted the pair — both strikes showed identical
ticks/bootId. Therefore:

1. **No record rewrite, ever.** Existing records keep their drifted
   epochs; those become display values. A migration that rewrote epochs
   would need its own identity proof to know which process a record names
   — circular, and pointless once readers prefer the pair.
2. **Pair-carrying records** (announcements, owner.json, state.json, job
   records, run records, steward records, lease records, goal journal,
   goalrevision locks — the bulk of the fleet's records) are immune the
   moment the reader-side changes land. Nothing to do per machine.
3. **Pairless records** (the 4.2 schemas before their writers land:
   locks, registry claims, guard members, markers, attestations,
   acknowledged entries, wrapper tokens) continue under exact legacy
   seconds and stay drift-vulnerable until natural turnover. Every one of
   these is short-lived by construction: locks are per-operation, guard
   members live for a gate run, markers/attestations for a
   watcher interval, acknowledged entries until the process dies. The one
   long-lived pairless class is an acknowledged entry for a long-lived
   process; its failure mode is a re-nag, not an authorization.
4. **Per-machine step, once, after landing:** the already-recorded heal
   sequence — lawful owner shutdown + `metasystem up` re-arm (exercised
   twice on m0, self-servable under the R-34-m0 approval) — rewrites
   owner.json, state.json, the component ledger, and locks with
   pair-carrying identities from the new writers. Announcements refresh on
   each session start; the census prunes stale ones. Four machines, one
   bounded action each, no coordination required (mixed versions read both
   shapes).
5. **No tolerance window during migration.** The legacy rule stays exact.
   Loosening seconds fleet-wide to smooth the transition would widen the
   false-match surface for every legacy record at once (section 7A).

## 7. Rejected alternatives

### A. Tolerance band on the seconds comparison (±N seconds)

Rejected on four grounds:

1. **The drift is not bounded by the observed ±1s.** btime moves with
   every wall-clock adjustment; a VM pause/resume, live migration, or a
   large NTP step after resume moves it by arbitrary amounts. Any finite N
   fails the next larger step — this exact class already struck twice with
   different magnitudes of cause, and a band converts "defect fixed" into
   "defect threshold raised".
2. **Pid-recycling risk is real at delegate timescales.** This platform
   (m0, Linux aarch64 guest) has `kernel.pid_max = 4194304` (read from
   `/proc/sys/kernel/pid_max`), so wrap-around recycling is slow — but the
   band's false-match does not need a wrap. It needs a pid freed and
   reused within N seconds of the ORIGINAL process's birth second, which
   is precisely the short-lived-child pattern this system mass-produces:
   adapters, probes, and fixture processes that live under a second. A
   band of even ±2s makes a recycled short-lived delegate pid
   indistinguishable from its predecessor for every consumer that
   authorizes on Dead/NOT-OURS. The exact-seconds rule's whole value is
   that a recycled pid virtually never lands on the same birth second;
   a band surrenders that.
3. **Bands do not compose with joins.** Several failing sites are hash-map
   key joins (`procKey{pid, startedAt}` at `internal/lease/classify.go:248`,
   the acknowledged index at `internal/supervise/acknowledged.go:137`). A
   toleranced equality cannot be a map key; those sites would need
   restructuring anyway — at which point the pair is the same amount of
   work with none of the false-match surface.
4. It leaves the Custodian/Killable false-verdict classes (section 3)
   merely narrowed, not closed, and does nothing for mainId stability.

### B. One canonical time source for write and probe

The idea: freeze one boot-epoch value (first `btime` read, cached in a
file per boot) and derive every recorded and probed start second from it,
so writer and prober always agree. Rejected:

1. **It re-implements the kernel's own token, worse.** "startTicks since a
   frozen anchor, plus an anchor identity to detect reboots" IS
   `(startTicks, bootId)` — except the kernel's version needs no cache
   file, no cross-process initialization order, no invalidation logic, and
   is already recorded in every major schema. R-11: the mechanism with
   fewer moving parts that already exists wins.
2. **The cache is shared mutable state with failure modes of its own:**
   who writes it first, what happens when it is deleted mid-flight, how
   two engine generations (or the shell hook vs the Go engine) agree
   during rollout. Identity infrastructure should not depend on one more
   file being right.
3. **It does not fix Darwin/legacy asymmetry or old records**: everything
   recorded before the freeze still disagrees, so it needs the same
   migration reasoning as the chosen option anyway.

### C. Stricter recorded resolution (microseconds on Linux)

Rejected in one move: Linux microsecond start times are derived from the
same re-read `btime` anchor (`identity_linux.go:58`), so finer resolution
makes the comparison MORE drift-fragile, not less — a sub-second slew that
today only sometimes flips the whole second would always change the
microsecond. The acknowledged-entry `pidStartedAtExactMicro` path
(`internal/supervise/acknowledged.go:143`) demonstrates the failure today
and is converted AWAY from micro on Linux by this design. (On Darwin,
micro is the kernel's spawn-time record, not a derivation — which is why
it stays.)

## 8. Coupling to same-process-succession (KI-33) — stated, not solved

`mainId` embeds the epoch at mint time: `main-<pidStartedAt>-<pid>-<hex>`
(`internal/lease/verbs.go:182`). Two consequences, no more:

1. **This design freezes mainId's epoch as an opaque token.** After it, no
   code may expect the epoch inside a mainId to equal any probe's derived
   second — comparisons of mainId are string-vs-string (lease lineage,
   goal-history actors, census announcement schema), and the
   re-mint guard is the pair-aware announcement finder
   (`internal/lease/classify.go:166`), which prevents drift from minting a
   second mainId for one live process (the KI-24 split-identity
   recurrence, closed).
2. **Same-process-succession work must inherit that reading.** KI-33's
   succession chain construction consumes lineage strings that carry these
   frozen epochs; any succession rule that re-derives or re-validates the
   embedded epoch against a live probe would reintroduce this defect. That
   goal decides its own semantics; this design only forbids it one wrong
   move.

## 9. Simplicity accounting (R-11)

Net machinery added: one function (`SameRef`), one relocated loader, one
grep-ratchet test. Net machinery removed: two duplicated comparison idioms
(hand-rolled seconds equalities and per-package "pair decides" copies)
collapse into the identity core; the census gains one loader instead of
three field-extraction sites. Schema growth is two optional fields on
records that already carry three identity fields. No new processes, files,
locks, or configuration. The alternative designs each add a cache file, a
tunable window, or both.

## 10. Gaps (stop-and-report; none filled silently)

- **G1 — Darwin start-token stability is asserted, not tested here.** The
  claim that Darwin's kernel start record does not drift under clock
  discipline (section 2.2) matches the existing code's design comments and
  `identity_darwin.go`'s use of the kernel's spawn-time record, but the
  goal record says "macOS unchecked" and this sandbox is Linux. If Darwin
  drifts, `CompareDarwinMicroseconds` needs its own token; nothing in this
  design forecloses that.
- **G2 — The terminate-flake causal link is a lead, not a proof.** I traced
  the drift-vulnerable members of the kill/wind-down paths
  (`janitor.Killable`, `missionrunner` drain's `Custodian`, proofrun guard)
  but cannot run the failing tests here to confirm which one produces the
  observed reds. The design fixes all of them; the goal's verification leg
  should re-run the family on m0 after landing.
- **G3 — Chrony/NTP forensics were not performed** (goal: "chrony logs
  unchecked"). The design does not depend on which discipline event moved
  btime, only on the kernel-documented fact that btime is wall-clock
  derived; noted for completeness.
- **G4 — Shell-side readers not exhaustively enumerated.** The shell
  scripts matched on `pidStartedAt` (`supervision-fixtures.sh`,
  `emit-event.sh`, `fingerprint-harness.sh`, `health-fixtures.sh`,
  `mission-fixtures.sh`, `dispatch-fixtures.sh`, `delegate-caps-fixtures.sh`,
  `supervision-go-fixtures.sh`) are fixture/test/transport code on
  inspection of their roles, and the load-bearing hook
  (`supervision-hook.sh`) only copies values — but I did not read every
  fixture script line-by-line for a hidden comparison. A critique round or
  the implementation leg should sweep them under the 4.4 ratchet's
  shell-side sibling if one is wanted.
- **G5 — `internal/lease/classify.go` procKey lookup site.** I verified the
  maps are built from recorded seconds (:248,:277,:281) and that the
  caller side derives from a fresh probe, but did not read the exact
  lookup lines; the conversion in section 5 covers both ends regardless.

## 11. Self-grade (R-24)

**Grade: B+.** The caller enumeration is the widest part of Ruling R and it
is grounded file:line with a verdict and a mechanical conversion for every
site found; the decision follows the codebase's own proven direction
rather than inventing one; the migration needs no data rewrite; and the
rejected alternatives are argued from platform facts (pid_max read on the
affected machine, btime semantics in the shipped prober) rather than
taste. Held back from A: two asserted-but-unverified facts (G1 Darwin
stability, G2 flake causality) sit on the path of claims the design uses
for motivation, and the shell-side sweep (G4) is role-level rather than
line-level — a critique round can attack any of the three.
