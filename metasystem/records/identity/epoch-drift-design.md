# Epoch-drift identity design: the kernel token is the comparator, the epoch is display

- Goal: vm-epoch-identity-drift (design leg; Ruling R enumeration included)
- Mode: design, critique-ready
- Author: implementer job epoch-drift-design-r1e, 2026-09-01; revision 2 by
  implementer job epoch-drift-design-r2, 2026-09-02
- Status: PROPOSED, revision 2 — all twelve round-1 findings
  (ED-R1-001..012) folded; disposition matrix in section 12
- Convergence: critique round 1 found 12 material findings; this revision
  targets zero

## 0. One-page summary (R-11)

Linux derives a process's recorded start second from `btime`, which the
kernel recomputes from the current wall clock; NTP discipline on a VM
guest moves it, so a live process's derived start second drifts between
probes while its kernel pair `(startTicks, bootId)` cannot. Two production
strikes (a census refusal and an armed-owner refusal) came from the
unconverted half of an in-flight migration to pair comparison.

The design: **the platform-native kernel token is THE comparator** — the
Linux pair `(startTicks, bootId)`, the Darwin spawn-record microsecond —
and the epoch second is display plus a confined legacy fallback. Five
laws make it total:

1. **Token wins when present.** Any join where a live probe carries the
   native token never lets a recorded second overrule it. A legacy
   (seconds-only) record that MISMATCHES such a probe yields **Unknown,
   never Dead** — and Unknown never authorizes anything (ED-R1-010).
2. **Seconds may still decide** only where no drifting side exists: probe
   joins on fixture/tokenless platforms, and record-vs-record joins
   (`SameRef`), whose mismatches never authorize destruction.
3. **One comparator, one owner** — every start-identity comparison in
   production Go code goes through `internal/identity`; a syntax-level
   (AST) guard with no comparison allowlist enforces it (ED-R1-011).
4. **Identity flows by copy, never by re-derivation**; every writer
   persists the platform-native token (pair on Linux, microsecond on
   Darwin) beside the display second (ED-R1-004).
5. **The claim is scoped**: the token distinguishes processes within one
   boot of one kernel PID namespace on one machine; the design declares
   that domain rather than inventing a machine-incarnation field
   (ED-R1-001).

Migration rewrites no record: drifted seconds become display values the
moment readers prefer the pair; pairless legacy records become
non-authorizing on mismatch and are enriched on their next natural
rewrite or the one-per-machine re-arm heal. Net machinery: one
record-comparison function, one presence-rule fix, one loader with an
explicit field map, one lock-identity constructor, one AST guard test.
No new processes, files, locks, or configuration.

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
display and a confined, non-authorizing legacy fallback.** This is the
brief's a-priori-strongest option, and the trace confirms it: the codebase
has already converged on this rule in half its sites, the kernel already
provides exactly the frozen token a "canonical time source" would have to
invent, and the two strikes plus the terminate-flake cost curve all live
in the unconverted residue. The design completes the migration, centralizes
the rule so a caller can no longer hand-roll a seconds equality, and adds a
recurrence guard.

### 2.1 The comparison domain and the narrowed uniqueness claim (ED-R1-001)

The token is NOT a globally unique identifier and this design does not
claim it is. The claim, exactly:

- **Within one boot of one kernel PID namespace on one machine**, the pair
  `(pid, startTicks)` separates any two processes unless a pid is freed
  and reused within the same 10 ms tick — which requires the kernel to
  cycle the entire pid space (`kernel.pid_max` = 4194304 on m0, read from
  `/proc/sys/kernel/pid_max`) inside one tick. The claim is
  practically-unreachable-collision, not a cryptographic guarantee, and it
  is strictly stronger than the whole-second rule it replaces (a second is
  100 ticks).
- **Across boots**, `bootId` separates records; across machines, PID
  namespaces, and virtual-machine clone incarnations, the token separates
  NOTHING, and the design carries that load by declaring the domain:
  `Compare` and `SameRef` are defined only over probes and records
  produced by engines observing the same kernel PID namespace via the
  same `/proc`. Every record this system compares lives under one
  repository's artifacts root on one machine and is only ever joined
  there; the durable evidence mirror copies records as content and never
  identity-joins them. There is no code path that joins records from two
  machines, so there is nothing to reject mechanically — the domain
  declaration is a documented precondition, not a runtime check.
- **VM snapshot cloning** of a machine with an armed engine duplicates
  boot id, tick state, pid table, and live processes; two clones sharing
  one artifacts root (e.g. a shared mount) would defeat any kernel-derived
  token. Named as an out-of-scope operator hazard: the remedy would be a
  per-incarnation machine identity in every record — a subsystem this
  defect does not need (convergence discipline: no subsystem to save a
  sentence). The one-machine domain above already excludes it.

Consumers need no new fields; the domain is what the system already does.
A future cross-machine feature must revisit this section before reusing
`SameRef`.

### 2.2 The comparator law, per platform

1. **Linux:** identity is `(pid, startTicks, bootId)`. **Pair presence is
   keyed to `bootId`** (ED-R1-002): a ref carries the Linux pair iff
   `BootID != ""`, and `StartTicks` is then compared at any value
   INCLUDING ZERO — a start tick of zero is a legitimate kernel value for
   a process born in the boot's first tick, and the shipped prober
   accepts it (`identity_linux.go:51` rejects only negative ticks).
   `pidStartedAt` is written for display and cross-version readability and
   is never compared when the probe side carries the pair.
2. **Darwin:** identity is `(pid, pidStartedAtExactMicro)`. The Darwin
   microsecond token is read from the kernel's per-process start record,
   which is written once at spawn and not re-derived from the current wall
   clock, so it does not drift. Comparator unchanged; what changes is
   PROPAGATION (ED-R1-004): every schema that gains the Linux pair in 4.2
   gains the Darwin microsecond under the same prefix, and the schemas
   previously listed as "already correct" but Linux-pair-only gain it too
   (4.2b), so the strongest-identity writer law holds on both platforms.
3. **Legacy fallback, confined (ED-R1-010, the critical fold).** A record
   carrying no native token compares by exact whole seconds — but the
   VERDICT a mismatch may produce depends on what the other side is:
   - **Probe-vs-record, probe carries the Linux pair:** the recorded
     second was btime-derived and can drift, so a seconds MISMATCH proves
     nothing. The comparison is marked **non-decisive** and every liveness
     mapper returns **Unknown**, which never authorizes anything: no lock
     takeover, no lease takeover or succession, no `DeadOwner`
     classification, no reap terminalization, no pruning, no acknowledged
     expiry. A seconds MATCH still proves the join (drift cannot forge a
     match against a recycled pid any better than before). Mechanism in
     4.1.2.
   - **Probe-vs-record, probe carries no native token** (fixture probes;
     hypothetical tokenless platforms): seconds are the strongest
     evidence either side can have; exact seconds decide both ways,
     unchanged. On Darwin the probe's second is derived from the stable
     spawn record, not from a drifting anchor, so a legacy mismatch there
     is genuine pid reuse and remains decisive Dead.
   - **Record-vs-record (`SameRef`):** no live probe exists, so
     "token-wins-when-present" has no token side to protect; seconds may
     decide both ways when either record is legacy. This is safe because
     no record-vs-record consumer authorizes destruction on a mismatch:
     the sites are dedupe joins, custody-map joins, and attestation
     joins, whose mismatch outcome is "treat as distinct entries" or a
     fail-closed refusal (an availability cost during migration, healed
     by enrichment, never a wrong kill or takeover). Section 5 marks
     every such site.
   - No tolerance is added in any branch (section 7A).
4. **Fixture identities** (controlled test clocks) keep the seconds rule
   by declared design; fixtures never carry a pair.

### 2.3 Subsidiary laws

- **One comparator, one owner — and it wins everywhere (ED-R1-011).** All
  probe-vs-record joins go through
  `identity.Compare`/`AliveRef`/`AliveTaggedRef`/`Custodian`; all
  record-vs-record joins go through the new `identity.SameRef` (4.1.1).
  This law is the END STATE for all production Go code, with no
  exemptions: the sites Ruling R marks SAFE-PAIR or SAFE-STRUCTURAL are
  safe from DRIFT, not exempt from centralization — every hand-rolled
  comparator, including the currently-correct ones
  (`internal/lease/identity.go:88-98` `LiveRef`,
  `internal/lease/claim.go:348` `sameLeaseProcess`,
  `internal/steward/health.go:1029-1037` `sameComponentProcess`,
  `internal/census/run.go:296` `sameProcessIdentity`,
  `internal/dispatch/ownership.go:120-134` `sameRecordedIdentity`),
  converts to the core API before the recurrence guard lands (4.4). The
  conversion ORDER in 4.3 is sequencing under one law, not a partition
  into converted and sanctioned.
- **Identity flows by copy, never by re-derivation.** A component that
  writes another process's identity into a record copies the bytes from
  that process's existing record (announcement, owner.json, job record).
  A component may re-probe only to VERIFY through the comparator, never
  to mint a second recorded value for the same process. (This is what
  makes the SAFE-STRUCTURAL sites in section 5 safe, and it is the rule
  the supervision-hook already follows at
  `scripts/agents/supervision-hook.sh:125`.)
- **Every writer persists the strongest identity it observed** — the
  platform-native token plus the display second — via the shared writer
  helper (4.1.3). Schemas that today persist only `(pid, pidStartedAt)`
  gain the token fields (omitempty), enumerated in 4.2.

## 3. Consequence classes (why the residue matters beyond refusals)

The residue does not only produce false refusals against live processes
(the observed strikes). Because `identity.Compare`'s mismatch verdict is
today mapped to "the pid was reused: definitively gone"
(`internal/identity/identity.go:212-215`), a drifted second in legacy mode
produces **false Dead**, and false Dead AUTHORIZES things:

- lock takeover of a live holder's lock (census writer lock,
  `internal/supervise/censuslock.go:83` — two concurrent census writers);
- registry slot classification `DeadOwner` for a live owner
  (`internal/registry/slots.go:70`);
- lease succession or takeover against a live holder whose record is
  legacy (`internal/lease/claim.go:85,124-129`: a false-not-live holder
  falls through to `succeed`/`takeover`);
- kill-less group-death conclusions in the mission runner's drain
  (`internal/missionrunner/drain.go:245,401` via `identity.Custodian`);
- a fresh main lineage id minted for a live, already-announced process
  whose announcement is legacy and drifted
  (`internal/lease/verbs.go:126-139,182` — the KI-24 split-identity
  shape).

The 2.2.3 law closes this class structurally: every one of these
authorizations requires a DECISIVE Dead, and a legacy mismatch against a
pair-carrying probe is no longer decisive.

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

1. **Add `SameRef`** — the one record-vs-record equality rule
   (ED-R1-003 folds the round-1 spec, which weakened invalid shapes to
   seconds):

   ```go
   // SameRef is the one record-vs-record equality rule. Validity is
   // checked before any fallback: a malformed ref never weakens itself
   // to seconds. The strongest representation BOTH valid refs carry
   // decides.
   func SameRef(a, b Ref) Comparison
   ```

   Mechanical rule, in order:
   - `a.Pid != b.Pid` → `{Matches: false, Mode: mode-of-a}`.
   - Compute `a.Mode()` and `b.Mode()`. **If either is `CompareInvalid`,
     the result is `{Matches: false, Mode: CompareInvalid}`** — mirroring
     `Compare`, which already refuses invalid refs
     (`identity.go:125`); a partial pair, a mixed exact shape, or a
     record carrying both exact shapes never falls back to seconds.
   - Both modes `CompareLinuxTicksBootID` → compare
     `(StartTicks, BootID)`, ticks compared at any value including zero.
   - Both modes `CompareDarwinMicroseconds` → compare micro.
   - One `CompareLinuxTicksBootID`, the other `CompareDarwinMicroseconds`
     → `{Matches: false, Mode: CompareInvalid}`: within the one-machine
     domain (2.1) one process cannot have two platforms, so this shape is
     data corruption, not a legacy record; no seconds fallback.
   - Remaining combinations (valid legacy + valid native exact, or two
     valid legacies): compare `StartedAtSec`,
     `Mode = CompareLegacySeconds`. This is the confined record-vs-record
     fallback of 2.2.3; both sides are records, no live probe exists,
     and no consumer of SameRef authorizes destruction on mismatch.
     (This deliberately differs from `dispatch.sameRecordedIdentity`
     (`internal/dispatch/ownership.go:120-134`), which refuses on any
     mode mismatch; a pair-carrying record and a legacy record for the
     SAME process must still be joinable by their common seconds.
     `sameRecordedIdentity` converts to `SameRef`; its stricter behavior
     was a per-schema accident, not a law.)

2. **Non-decisive legacy comparisons (ED-R1-010's mechanism).** Add one
   field to the existing result type:

   ```go
   type Comparison struct {
       Matches  bool
       Mode     ComparisonMode
       Decisive bool // false iff Mode is CompareLegacySeconds and the
                     // live side carried the Linux pair: a btime-derived
                     // recorded second cannot overrule a live token.
   }
   ```

   - `Compare(exact, ref)` sets `Decisive = !(mode ==
     CompareLegacySeconds && exact.BootID != "")`; all other modes are
     decisive. (Only Linux derives seconds from a re-read drifting
     anchor; Darwin's second descends from the stable spawn record, so
     Darwin legacy comparisons stay decisive — see 2.2.3.)
   - `AliveRefComparison` (`identity.go:194-217`) changes its mismatch
     mapping: `!Matches && Decisive` → `Dead` (pid reuse, as today);
     `!Matches && !Decisive` → **`Unknown`** (drift or reuse,
     unprovable). `SameRef` results are decisive by construction (no
     live side).
   - `AliveTaggedRef` (`identity.go:223-241`) reorders for the
     non-decisive case: on a non-decisive seconds mismatch it consults
     argv BEFORE concluding — an exact instance-tag hit
     (`HasExactToken`) is the secondary proof that this IS the recorded
     process (tags are unique per launch) → `Alive`; tag absent with
     `ArgvKnown` → `Unknown` (drift plus a self-exec is
     indistinguishable from reuse; stay conservative); `ArgvKnown=false`
     → `Unknown`. Decisive mismatches keep today's `Dead`.
   - `Custodian` keeps its existing argv tag-hit override
     (`internal/identity/custodian.go:50-58`) and gains the same
     non-decisive `Unknown` mapping for tagless legacy mismatches.
   - **Consumers of the new Unknown**: every authorizing consumer
     already treats Unknown as "act not" (the package law,
     `identity.go:4-6`; lock takeover is death-only; the reaper acts
     only on Dead). One consumer must convert from bool to three-way:
     the lease claim's holder-liveness branch
     (`internal/lease/claim.go:85`) currently collapses liveness to a
     bool, so Unknown would fall through to `succeed`/`takeover`. It
     converts to three-way: `Unknown` REFUSES the claim with a
     holder-liveness-unprovable message (fail-closed, like
     OWNED-ELSEWHERE), and only decisive `Dead` reaches the succession
     and takeover arms. Ruling R marks it.

3. **Presence fixes and the one writer helper (ED-R1-002).**
   - `Exact.Ref()` (`identity.go:50-59`) keys the pair branch on
     `e.BootID != ""` alone (the Linux prober guarantees a non-empty
     boot id on success, `identity_linux.go:136-148`; ticks may be
     zero).
   - `Ref.Mode()` (`identity.go:74-94`) redefines presence:
     `hasPair = BootID != ""` (ticks compared at any non-negative
     value); `StartTicks > 0 && BootID == ""` remains `CompareInvalid`
     (a partial pair); `hasMicro && hasPair` remains `CompareInvalid`.
     With `omitempty` JSON, a zero-tick pair round-trips correctly:
     `bootId` is present, `pidStartTicks` absent, loads as
     `{StartTicks: 0, BootID: set}` → valid pair. No schema change
     needed for the zero case.
   - `dispatch.exactIdentityFields` (`internal/dispatch/ownership.go:84-97`)
     moves into `internal/identity` beside the loader as the ONE
     canonical writer helper; its pair branch keys on the same presence
     rule.

4. **`Custodian` takes a full `Ref` — and so does its whole binding
   chain (ED-R1-007).** Changing only the core signature would let an
   implementer construct a seconds-only `Ref` at the call site and
   compile. The design therefore names every link:
   - `identity.Custodian` (`internal/identity/custodian.go:24`):
     signature becomes `Custodian(ref Ref, tag string, probe …)`;
     compares through `Compare` so the pair decides when the record
     carries it; argv tag-hit override stays.
   - `supervise.ReaperConfig.Custodian`
     (`internal/supervise/reaper.go:48`): field type becomes
     `func(ref identity.Ref, tag string) identity.Liveness`.
   - `reapOne` (`internal/supervise/reaper.go:131,145-147`): loads the
     full ref from the job record via the canonical loader (4.1.5) —
     job records already persist the exact fields
     (`internal/dispatch/ownership.go:84-96`) — and passes it whole.
   - Production bindings, all converted:
     `cmd/metasystem/supervise_component.go:320-329` (`kernelCustodian`,
     the standing reaper), `internal/steward/reap.go:93-104`
     (continuation-custody reconciliation),
     `internal/mission/fence.go:620` (the `custodianProver` alias) and
     its call at `fence.go:781-794` (mission usage recovery, which
     currently loads only `pid`/`pidStartedAt` from the record),
     `internal/missionrunner/drain.go:342` (mission drain),
     `internal/supervise/acknowledged.go:229` (acknowledged-entry
     expiry, whose ref is the stored entry).

5. **The one loader, with explicit dialects (ED-R1-012).** The promise
   "one canonical JSON-object→Ref loader" round 1 made is unimplementable
   as stated because durable schemas spell the fields differently
   (`pidStartTicks` in job records and announcements; `startTicks` in
   `steward.RunnerRecord` (`internal/steward/runner.go:39-45`),
   `goal.OwnerIdentity` (`internal/goal/journal.go:71-76`), and
   `humanauthority.ProcessRef` (`internal/humanauthority/authority.go:40-46`);
   `ownerPidStartTicks`-style prefixes in 4.2's registry rows). The
   specified resolution:
   - **Typed structs never go through a map loader.** A schema with a Go
     struct keeps its JSON tags exactly as shipped (durable records are
     not renamed, ever) and converts with a plain constructor:
     `identity.RefFromParts(pid, startedAtSec, exactMicro, startTicks,
     bootID)` — one function, no reflection, no aliases.
   - **Untyped `map[string]any` records** load through
     `identity.RefFromObject(value map[string]any, fields FieldMap)
     (Ref, bool)` where `FieldMap` names the five keys explicitly:

     ```go
     type FieldMap struct{ Pid, StartedAtSec, ExactMicro, StartTicks, BootID string }
     var CanonicalFields = FieldMap{"pid", "pidStartedAt",
         "pidStartedAtExactMicro", "pidStartTicks", "bootId"}
     ```

     The dispatch loader (`internal/dispatch/ownership.go:99-116`)
     relocates as the `CanonicalFields` implementation with verbatim
     behavior (including its `Mode() != CompareInvalid` validity
     return). Each divergently-spelled schema declares its own
     `FieldMap` beside its schema definition. A loader consults exactly
     the named keys — never a second spelling — so alias ambiguity
     cannot arise; a record carrying two spellings is simply read by
     the one its schema declares.
   - **New 4.2 fields follow the schema's existing prefix convention**
     (a registry record that says `ownerPid` gains `ownerPidStartTicks`;
     a bare-`pid` record gains `pidStartTicks`), and the schema's
     FieldMap or struct tags are the single place that spelling is
     bound.

6. **`lock.Identity` carries the token, via one constructor
   (ED-R1-008).** Adding fields to the struct
   (`internal/lock/lock.go:31`) is not enough; round 1 missed the
   command-layer constructors. The mechanism: `lock.Identity` gains the
   native token fields plus a constructor
   `lock.IdentityFromRef(ref identity.Ref, tag, label string)`, and
   every struct-literal construction of a SELF or recorded identity
   converts to it. Enumerated constructors (from a tree-wide sweep of
   `lock.Identity{` literals):
   - `cmd/metasystem/supervise_owner.go:106` (`lockSelf`, the registry
     lock's holder — the live registry writer the finding names) —
     builds from the owner's own probed `self` ref.
   - `internal/supervise/censuslock.go:67,71` (census writer lock,
     recorded-owner and self sides).
   - `internal/supervise/acknowledged.go:187-189` (acknowledger lock
     self).
   - `internal/gaterun/guard.go:91` (execution-guard lock identity).
   - `internal/goalrevision/lock.go:63,134` (revision lock holder and
     self; its holder record already carries the pair, which the
     constructor now preserves into the lock identity).
   - `internal/dispatch/ownerlock.go:57,89,107` record NO start second
     at all (pid + tag only) — outside this defect's class; unchanged,
     noted for completeness.
   Every `lock.Probe` binding builds the full `Ref` from the (now
   token-carrying) holder identity: `cmd/metasystem/supervise_owner.go:200-211`
   (`kernelProbe`, which today rebuilds a seconds-only ref and would
   discard the new fields — the finding's exact site),
   `internal/supervise/acknowledged.go:192-201`, the censuslock probe
   (`internal/supervise/censuslock.go:70-90`), and
   `internal/gaterun/guard.go:110,136`. The lock package's death-only
   takeover already treats Unknown as keep, so the 4.1.2 non-decisive
   mapping composes with it unchanged.
   The diagnostic status reader `cmd/metasystem/supervise.go:124-134`
   (drops the pair from owner.json before printing a liveness verdict)
   loads the full ref through the canonical loader — a wrong verdict in
   a diagnostic is how humans get talked into killing live owners.

7. `Compare`, `AliveRef`, `SameIdentity`, `VerifyProcess` keep their
   signatures; `Compare` gains only the `Decisive` field, and the
   liveness mappers gain the 4.1.2 mapping.

### 4.2 Writers: schemas that gain the native token (all fields omitempty)

Every row gains BOTH platform tokens under the schema's existing prefix
convention (ED-R1-004): on Linux `…StartTicks` + `…BootId`, on Darwin
`…StartedAtExactMicro`. Exactly one is written per record (the writer
persists what its platform's prober returned); `Ref.Mode()` keeps
rejecting records carrying both.

| Schema | Site | Fields added (Linux / Darwin) |
| --- | --- | --- |
| `lock.Identity` | `internal/lock/lock.go:31` via the 4.1.6 constructor | `pidStartTicks`+`bootId` / `pidStartedAtExactMicro` |
| supervision component ledger record | `internal/supervise/ledger.go:75` | persist `held.Identity`'s token beside `pidStartedAt` |
| component heartbeat **(ED-R1-005, new row)** | `internal/supervise/component.go:39-54` (`WriteHeartbeat`) and the armed watcher entry in `state.json` it is joined against | `pidStartTicks`+`bootId` / `pidStartedAtExactMicro`; the dispatch admission join `dispatch.WatcherCeiling` (`internal/dispatch/attest.go:131-135`) replaces its per-key `looseEqual` on `pid`/`pidStartedAt` with one `SameRef` over refs loaded by `CanonicalFields` — today the two sides come from two INDEPENDENT probes (owner probes the child, `internal/supervise/proc.go:66-86`; child probes itself, `cmd/metasystem/supervise_component.go:76-81`), the design's one sanctioned probe-twice exception, made drift-immune by the token rather than forbidden |
| census `InventoryItem` **(ED-R1-009, new row)** | `internal/census/run.go:51-64,253-258` | `pidStartTicks`+`bootId` / (micro already present); `Process` already carries the pair (`run.go:42-43`) and the constructor copies it through, so acknowledgement can bind to THE observed process by token instead of by btime-derived micro |
| `registry.ProcessRef` and armed/custody records | `internal/registry/reduce.go:29`, `internal/registry/records.go:143,172,207` | `ownerPidStartTicks`+`ownerBootId` etc., per prefix / same-prefix micro |
| gaterun gate marker | `internal/gaterun/gaterun.go:117-121` | `pidStartTicks`+`bootId` / micro |
| gaterun execution-guard member/holder | `internal/gaterun/guard.go:26,32` | `pidStartTicks`+`bootId` / micro |
| proofrun execution-guard member + `SuiteIdentity` input | `internal/proofrun/watchdog.go:246` and the cmd-layer caller that builds `WatchdogOptions.SuiteIdentity` | `pidStartTicks`+`bootId` / micro |
| watcher scan attestation | consumed at `internal/report/scan.go:281` (`WatcherPid`/`WatcherStart`) | `watcherStartTicks`+`watcherBootId` / `watcherStartedAtExactMicro` |
| acknowledged process entry | `internal/supervise/acknowledged.go` (`AcknowledgedProcess`) | `pidStartTicks`+`bootId` / micro; on Linux the pair REPLACES the role of `pidStartedAtExactMicro`, which is btime-derived and drifts (see 5, supervise block); the census-to-live binding in `Acknowledge` (`acknowledged.go:162-181`) compares the live probe to the census item's recorded TOKEN (available per the InventoryItem row) instead of the derived micro |
| wrapper token | `internal/validate/wrappertoken.go:57` | `wrapperPidStartTicks`+`wrapperBootId` / `wrapperPidStartedAtExactMicro` |
| watcher restart request | `internal/supervise/watcher_repair.go:101` | `pidStartTicks`+`bootId` / micro (dedupe join then uses `SameRef`) |

Readers of every schema above accept records WITHOUT the new fields
(legacy fallback, with 2.2.3's non-decisive semantics), so mixed fleets
and old records keep working. No reader ever requires the token.

### 4.2b Darwin token propagation for Linux-pair-only schemas (ED-R1-004)

Round 1 called these writers "already correct"; they are correct on
Linux only — each persists the pair but has no Darwin microsecond field,
so on Darwin they fall to seconds and the strongest-identity law fails
there. Each gains `pidStartedAtExactMicro` (or its prefix-consistent
spelling), written only on Darwin, read through the schema's
FieldMap/struct:

- announcements — `internal/lease/classify.go:21-29` (`Announcement`)
  and the writer `internal/lease/verbs.go:183-197`;
- job records — already carry micro via `exactIdentityFields`
  (`internal/dispatch/ownership.go:84-97`); no change, listed for the
  audit trail;
- owner.json/state.json — `internal/supervise/disk.go:77-83,118-133`;
- run records — `internal/run/run.go:169-171`;
- steward runner record — `internal/steward/runner.go:39-45`
  (`RunnerRecord`, dialect `startTicks`/`bootId`, gains
  `startedAtExactMicro`); steward identity/evidence records
  (`internal/steward/health.go:1022-1026` loader keeps all fields);
- goal journal owner — `internal/goal/journal.go:71-76`
  (`OwnerIdentity`, dialect `startTicks`/`bootId`);
- goalrevision lock holder — `internal/goalrevision/lock.go:53-54`;
- lease records — `internal/lease/claim.go:352` and the lease file
  schema;
- mission runner records — `internal/missionstate/missionstate.go:118-121`;
- human-authority `ProcessRef` — `internal/humanauthority/authority.go:40-46`
  (dialect `startTicks`/`bootId`);
- census `ProcIdentity` — `internal/census/verbs.go:33-39` already
  carries the pair in memory; gains the micro member so Darwin callers
  of `AuthIdentity` receive the token (`kernelIdentity`,
  `verbs.go:52-65`, copies it from the probe).

On Linux these writers change nothing; the fields are Darwin-only
output. Gap G1 (Darwin drift-stability unverified in this sandbox)
stands, but no longer gates correctness: the schemas carry the Darwin
token either way, so if Darwin's token ever needs replacing, it is a
token-value change, not another schema migration — this is what makes
G1's non-foreclosure claim true now (it was not in round 1, as the
critic showed).

### 4.3 Readers and comparison sites: the conversion list

Every VULNERABLE site in the section 5 table converts as stated in its
"required change" column. The three shapes of conversion:

- **VULNERABLE-DROP** (record carries the token; the loader or wrapper
  drops it): load the token and pass a full `Ref`. No schema change.
- **VULNERABLE-PROBE** (fresh probe compared to recorded seconds by hand):
  route through `identity.Compare`/`AliveRef`/`Custodian`/`SameRef` with
  the fullest ref the record carries. Where the record is a schema from
  4.2, the schema change lands first.
- **REDUNDANT-GATE** (a seconds pre-check in front of an already-exact
  mechanism): delete it (one site, ED-R1-006, below).

The custody-add gate (ED-R1-006): `cmd/metasystem/dispatch_verbs.go:1188`
re-probes the pid and refuses when the freshly derived second differs
from the second the shell read moments earlier
(`scripts/agents/dispatch.sh`, `internal_register_custody`, via
`proc started-at`). Both reads are btime-derived; a clock step between
them makes one process unequal to itself and strands custody
registration. The gate is DELETED rather than converted: the mechanism
it fronts, `dispatchcore.CustodyAdd`
(`internal/dispatch/custody.go:16-27,59-66,81-85`), already performs an
exact start/group/start sandwich and records the native token, which is
strictly stronger than any seconds pre-check; and the pre-check's only
theoretical catch (a pid recycled in the milliseconds between shell read
and command probe) is equally caught by the sandwich. The shell's
`proc started-at` call remains only as an early is-it-alive failure. The
"L11 binding cross-check" comment at both sites is updated to name
CustodyAdd's sandwich as the binding proof.

Order of landing (each step independently shippable, no flag days):
(1) identity core additions and presence fixes (4.1.1-4.1.3);
(2) VULNERABLE-DROP conversions — these alone close both observed strike
paths; (3) schema additions (4.2, 4.2b) writers-first, plus the
`Custodian`/reaper chain (4.1.4) and lock constructors (4.1.6);
(4) remaining VULNERABLE-PROBE conversions and the REDUNDANT-GATE
deletion; (5) conversion of ALL remaining hand-rolled comparators —
including the SAFE-labelled ones (2.3's law) — to the core API;
(6) the recurrence guard (4.4) last, over a tree with zero hand-rolled
comparisons left.

### 4.4 The recurrence guard (ED-R1-011 rebuilds it)

Round 1 proposed a lexical grep with a comparison allowlist; the critic
showed both halves fail (aliases like `looseEqual`, spelling variants
like `PIDStartedAt`, inequality forms like
`exact.StartedAt.Unix() != *pidStarted`, and an allowlist that sanctions
exactly the hand-rolled comparators the law forbids). Replaced:

- **Structure, not lexicon:** a validation-suite conformance test runs a
  small `go/ast` analyzer over every production package outside
  `internal/identity` (`_test.go` files excluded structurally, not by
  list). It fails on any binary comparison expression (`==`, `!=`, `<`,
  `<=`, `>`, `>=`) where either operand resolves to a selector whose
  field name is one of the identity spellings — `PidStartedAt`,
  `PIDStartedAt`, `StartedAtSec`, `StartTicks`, `PidStartTicks`,
  `BootID`, `StartedAtUnixMicro`, `PidStartedAtExactMicro` — or a call
  of `Unix()`/`UnixMicro()` on a field named `StartedAt`.
- **One structural exemption, no allowlist:** a comparison whose other
  operand is an integer literal (`> 0`, `!= 0`, `< 1`) is a presence
  check, not an identity join, and passes. There is no file, package, or
  site allowlist; by landing step 5 every production join already calls
  the identity API, so the analyzer's clean baseline is the converted
  tree itself, and any future hand-rolled comparator — drifting or not —
  fails the suite by construction.
- **Shell boundary, stated honestly:** the analyzer covers Go. After the
  ED-R1-006 deletion, no shipped shell script compares start identities
  — `scripts/agents/dispatch.sh` was swept and touches `startedAt` only
  in a comment and as transported values; the load-bearing hook
  (`scripts/agents/supervision-hook.sh:114-149,340-349`) only copies.
  Fixture scripts (G4) handle `pidStartedAt` as fixture data. The shell
  guard is the flows-by-copy law plus review, not a mechanism; a
  shell-side ratchet is declined as machinery without a demonstrated
  defect class (convergence discipline).

This is the cheapest mechanism that makes the THIRD strike of this class
impossible to write silently; KI-24 (split identity, 2026-08-07) and
these strikes are the two paid lessons.

## 5. Ruling R: every caller of the comparison, with verdict

Verdict key — **SAFE-PAIR**: already decides by the pair/exact token.
**SAFE-STRUCTURAL**: record-vs-record where both values descend from one
recorded observation (flows-by-copy), so drift cannot separate them.
**FIXTURE**: fixture-only seconds by declared design. **VULN-DROP** /
**VULN-PROBE** / **REDUNDANT-GATE**: as defined in 4.3. **DISPLAY**:
value shown, never compared. Per 2.3, SAFE verdicts mean drift-immune,
not conversion-exempt: every hand-rolled site converts to the core API
in landing step 5.

### identity core
| Site | What it is | Verdict / required change |
| --- | --- | --- |
| `internal/identity/identity.go:122` `Compare` | the one probe-vs-record rule | SAFE-PAIR; gains `Decisive` (4.1.2) and the presence fix (4.1.3) |
| `internal/identity/identity.go:187,194,223` `AliveRef`/`AliveRefComparison`/`AliveTaggedRef` | liveness over `Compare` | SAFE-PAIR given a full ref; gain the non-decisive Unknown mapping (4.1.2) |
| `internal/identity/custodian.go:24,61` `Custodian` | pid+seconds+tag liveness | VULN-PROBE (false Dead when no tag hit) → full-Ref signature + whole binding chain, 4.1.4 |
| `internal/identity/verification.go:53` `VerifyProcess` | start/argv/start sandwich | SAFE-PAIR (two fresh probes; pair rides both) |
| `internal/identity/fixture.go` probes | fixture identity table | FIXTURE |

### census
| Site | Verdict / required change |
| --- | --- |
| `internal/census/run.go:296` `sameProcessIdentity` (custody/announced classify join at :309,:314) | SAFE-PAIR; converts to core API in step 5 |
| `internal/census/run.go:380` run-owner leader join | VULN-DROP (run record carries pair, `internal/run/run.go:170`) → join via `SameRef` |
| `internal/census/run.go:435-478` `readSupervisionSnapshot` | VULN-DROP (drops pair from state.json) → load pair; **strike-1 feeder** |
| `internal/census/run.go:491` `verifySupervisionSnapshot` → `identityAlive` | VULN-PROBE → full-ref liveness; **strike-1 refusal site** |
| `internal/census/run.go:512-543` `identityAlive`/`alivePair` | SAFE-PAIR when pair supplied; legacy path gains non-decisive semantics via the core |
| `internal/census/run.go:545-598` `liveCustody` | VULN-DROP (drops pair from job/mission-turn records) → load pair |
| `internal/census/run.go:640-694` `announcementsList` | SAFE-PAIR (both branches :665-670, :675-681) |
| `internal/census/run.go:673` fixture branch | FIXTURE |
| `internal/census/run.go:51-64,253-258` `InventoryItem` + constructor | **VULN-FORMAT (ED-R1-009)**: observes then discards the pair `Process` carries (:42-43) → schema row 4.2; acknowledgement binds by token |
| `internal/census/verbs.go:17` `Alive` | seconds-only wrapper; every caller converts to full-ref (callers: `verifyarmed.go:33`, `watchdog.go:123`, `contract.go:1451`); wrapper then serves legacy/fixture only |
| `internal/census/verbs.go:26` `AlivePair` | SAFE-PAIR |
| `internal/census/verbs.go:33-39,45-65` `ProcIdentity`/`AuthIdentity` | returns the pair; gains the Darwin micro member (4.2b) |

### supervise
| Site | Verdict / required change |
| --- | --- |
| `internal/supervise/verifyarmed.go:32-44,79,103` `armedIdentityAlive` | VULN-PROBE + VULN-DROP (state records carry the pair; the signature is pid+seconds) → thread the pair through; **strike-2 refusal site (:80)** |
| `internal/supervise/arming.go:142-150` `sameArmingOwner` | SAFE-PAIR; converts in step 5 |
| `internal/supervise/arming.go:153-158` `ownerLiveness` | SAFE-PAIR (full ref + tag) |
| `internal/supervise/arming.go:369` component teardown `AliveTaggedRef(held.Identity, …)` | SAFE in-process; VULN-DROP across restarts because the component ledger persists only the second (`internal/supervise/ledger.go:75`) → 4.2 ledger change |
| `internal/supervise/proc.go:133,182,189,199` wind-down liveness over `held.Identity` | same as above: SAFE once the ledger persists the pair |
| `internal/supervise/proc.go:66-86` owner's post-launch child probe | probe-side WRITER feeding the heartbeat join — see the 4.2 heartbeat row (ED-R1-005) |
| `internal/supervise/component.go:39-54` `WriteHeartbeat` | **VULN-FORMAT (ED-R1-005)**: persists pid+seconds from the child's independent self-probe (`cmd/metasystem/supervise_component.go:76-81`) → carries the full token, 4.2 row |
| `internal/supervise/reaper.go:35-49,126-147` `ReaperConfig`/`reapOne` | **VULN-DROP (ED-R1-007)**: interface truncates to pid+seconds although job records carry the token → full-Ref field + loader, 4.1.4 |
| `internal/supervise/censuslock.go:62-71,70-90` census-writer lock identity + probe | VULN-PROBE with false-Dead TAKEOVER hazard (two live census writers) → constructor + full-ref probe (4.1.6) |
| `internal/supervise/acknowledged.go:137-144` `silencedByAcknowledgement` | VULN-PROBE twice: the `(pid, startSec)` index key joins a fresh census second against a recorded one, and `pidStartedAtExactMicro` is btime-derived micro on Linux → key becomes pid-only with `SameRef` confirmation; exact token becomes the pair on Linux (4.2) |
| `internal/supervise/acknowledged.go:162-181` `Acknowledge` census binding | VULN-PROBE (live micro vs census micro, both btime-derived on Linux) → compare live token to the census item's recorded token (4.2 InventoryItem row) |
| `internal/supervise/acknowledged.go:187-201` acknowledger lock identity + probe | VULN-PROBE → constructor + full ref (4.1.6) |
| `internal/supervise/acknowledged.go:221-230` entry replace/expiry (`Custodian` at :229) | VULN-PROBE (false-dead expiry re-nags an acknowledged live process) → full-Ref `Custodian` (4.1.4) |
| `internal/supervise/watchdog.go:110-126` steward-side component liveness | VULN-DROP (reads state.json pid/seconds only) + VULN-PROBE (`census.Alive`) → load pair, full-ref liveness; false alarm restarts live components |
| `internal/supervise/watcher_repair.go:100-108` restart-request dedupe | SAFE-STRUCTURAL (both sides copied from state.json); converts to `SameRef` in step 5 |
| `internal/supervise/disk.go:97,166,298` `Currency`/`StateNamesSelf`/intent target | SAFE-STRUCTURAL (records written by this process from its one captured `Self`; intent writers copy from owner.json) — the flows-by-copy law is what keeps these safe; converts to `SameRef` |
| `internal/supervise/verifyarmed.go` fixture probe path | FIXTURE |

### up and the session hook
| Site | Verdict / required change |
| --- | --- |
| `internal/up/up.go:137` `sameAuthenticatedProcess` | DEFECTIVE ORDER: seconds gate runs BEFORE the pair check, so a matching pair cannot rescue a drifted second → pair decides first, seconds only when either side lacks it |
| `internal/up/up.go:200` explicit `--pid/--start-time` vs fresh probe | VULN-PROBE — the hook replays the RECORDED start (`scripts/agents/supervision-hook.sh:125,149`), the probe re-derives it → verify by pid liveness + pair from the parent announcement when available; seconds only for pairless callers, non-decisive on mismatch |
| `scripts/agents/supervision-hook.sh:114-149,340-349` | writer/transport only; copies recorded identity (flows-by-copy) — no comparison; DISPLAY |

### lease
| Site | Verdict / required change |
| --- | --- |
| `internal/lease/classify.go:152-177` `authenticatedAnnouncement` finder | SAFE-PAIR for pair announcements; legacy branch (:172) gains non-decisive semantics via core conversion; converts in step 5 |
| `internal/lease/classify.go:248,277,281` `procKey{pid, pidStartedAt}` map builds + `:357-362` lookup keyed from a FRESH probe | VULN-PROBE (confirmed at the lookup: a drifted caller misclassifies — a delegate adapter or supervisor unrecognized) + VULN-DROP (`supervisedProc`, :288-291, and the job-record struct, :265-270, drop the token their sources carry) → key on pid, confirm with `SameRef`; closes round-1 gap G5 |
| `internal/lease/claim.go:85` holder liveness `LiveRef` | was SAFE-PAIR for pair records; **converts to THREE-WAY** (4.1.2): the bool collapse would let a legacy holder's non-decisive Unknown fall through to `succeed`/`takeover` (:124-129); Unknown now refuses the claim; only decisive Dead reaches succession/takeover — **the ED-R1-010 lease-takeover hole, closed** |
| `internal/lease/identity.go:88-98` `LiveRef` | hand-rolled pair-or-seconds comparator → replaced by core API (step 5) |
| `internal/lease/claim.go:348` `sameLeaseProcess` | SAFE-PAIR; converts to `SameRef` in step 5 |
| `internal/lease/claim.go:366-383` `holderIsMissionRunner` | SAFE-PAIR |
| `internal/lease/verbs.go:227` `Retire` join | VULN-PROBE on the ancestor-derived path (`up.go:182` re-probes; announce may hold an older second) → match by pid+session, confirm `SameRef` |
| `internal/lease/verbs.go:126-139` `Announce` reuse finder | VULN-PROBE for legacy announcements (:137 exact-seconds skip → false mint) → the 6.6 legacy-announcement reuse path |
| `internal/lease/verbs.go:182` mainId minting | see section 8 (KI-33 coupling); minting is guarded by the pair-aware finder plus the 6.6 reuse path, which together prevent drift-triggered re-minting (the KI-24 split-identity recurrence) |

### steward
| Site | Verdict / required change |
| --- | --- |
| `internal/steward/health.go:457-458,485,514,650,882` component/runner liveness | SAFE-PAIR (loader `health.go:1022-1026` keeps the pair) |
| `internal/steward/health.go:1029-1037` `sameComponentProcess` | SAFE-PAIR; converts to core API in step 5 |
| `internal/steward/runner.go:562-570` runner record vs probe | SAFE-PAIR; record gains Darwin micro (4.2b) |
| `internal/steward/reap.go:93-104` continuation-custody reaper binding | VULN-DROP (binds the seconds `Custodian`) → full-Ref chain (4.1.4) |
| `internal/steward/universe.go:100-105` worker-gone proof | SAFE-PAIR |
| `internal/steward/component_evidence.go:367` | presence validation only; DISPLAY |
| `internal/steward/alert_episode.go:40,370` invoker identity | presence validation + record; DISPLAY (no comparison found) |

### dispatch
| Site | Verdict / required change |
| --- | --- |
| `internal/dispatch/ownership.go:99-116` `identityRefFromObject` | SAFE-PAIR loader; relocates as the `CanonicalFields` loader (4.1.5) |
| `internal/dispatch/ownership.go:84-97` `exactIdentityFields` | SAFE writer; relocates as the one writer helper with the presence fix (4.1.3) |
| `internal/dispatch/ownership.go:120-134` `sameRecordedIdentity` | SAFE-PAIR record-vs-record; converts to `identity.SameRef` (behavior note in 4.1.1) |
| `internal/dispatch/attest.go:109-149` `WatcherCeiling` heartbeat join | **VULN-PROBE-FED RECORDS (ED-R1-005)**: `looseEqual` per-key join of two independently-probed seconds (:131-135) refuses every dispatch on drift → `SameRef` over token-carrying heartbeat/state (4.2 row) |
| `cmd/metasystem/dispatch_verbs.go:1164-1193` custody-add pre-gate (:1188) | **REDUNDANT-GATE (ED-R1-006)** → deleted; `CustodyAdd`'s exact sandwich is the binding proof (4.3) |
| `internal/dispatch/custody.go:63` | SAFE-PAIR |
| `internal/dispatch/custody_death.go:227-242` `processDefinitelyPredatesSupervisor` | SAFE-PAIR in pair/micro modes; the legacy branch (:238) is an ORDERING on seconds where ±1s can only flip an exact-boundary case toward "keeps the marker standing" (the safe direction) — accepted |
| `internal/dispatch/custody_death.go:244-255` `recordedRefLiveness` | SAFE-PAIR |
| dispatch census admission gate | consumes the census verdict; healed transitively by the census fixes |

### missionrunner, mission, janitor
| Site | Verdict / required change |
| --- | --- |
| `internal/missionrunner/host.go:109,143` + `proc.go:111` wind-down `groupOwnership` | SAFE-PAIR (argv sandwich over `VerifyProcess`; the "not provably ours"/"provably foreign" refusals are argv-positional, not epoch) |
| `internal/missionrunner/drain.go:245,401` kill-less death proofs via the binding at `drain.go:342` | VULN-PROBE (false Dead concludes a live group) → full-Ref `Custodian` chain (4.1.4); primary suspect for the terminate-flake lead, unproven (gap G2) |
| `internal/mission/fence.go:620` `custodianProver` alias + `:781-794` usage-recovery gate | VULN-DROP (loads only pid/seconds from a token-carrying job record) → full-Ref chain (4.1.4) |
| `internal/missionrunner/status.go:66-70` | DISPLAY |
| `internal/missionstate/missionstate.go:118-121` runner-record liveness | SAFE-PAIR |
| `internal/janitor/killproof.go:129` `GroupOwnership` | SAFE-PAIR |
| `internal/janitor/killproof.go:206-210` `Killable` recorded-identity gate | VULN-PROBE (`registry.ProcessRef` is seconds-only; false NOT-KILLABLE leaks survivors) → `ProcessRef` gains the token (4.2), gate uses `identity.Compare` |

### run, proofrun, gaterun
| Site | Verdict / required change |
| --- | --- |
| `internal/run/conclude.go:100`, `verbs.go:296`, `waiter.go:124,207` | SAFE-PAIR (full refs from run records) |
| `internal/run/waiter.go:176-184` cleanup self-match | SAFE-PAIR |
| `internal/run/run.go:550` | presence check; DISPLAY |
| `internal/proofrun/watchdog.go:132,221,294` suite liveness over `SuiteIdentity` | VULN-PROBE (caller-supplied seconds-only ref) → `SuiteIdentity` carries the token (4.2) |
| `internal/proofrun/watchdog.go:268` self-exclusion join, `:271` member refs | VULN-PROBE (worst case: fails to exclude self, then the mismatch makes `signalAuthenticated` refuse — leak direction, not wrong-kill) → members carry the token (4.2) |
| `internal/gaterun/fence.go:120-133` ancestry re-confirm | SAFE-PAIR (chain captured from probes with pair) |
| `internal/gaterun/fence.go:136-144` `matchesRef` controller join | VULN-DROP for pairless marker records → marker schema (4.2) |
| `internal/gaterun/gaterun.go:112-130` stale-marker pruning | pid-only `provablyGone` + seconds marker → marker schema (4.2); prune only on decisive `Compare` death |
| `internal/gaterun/guard.go:91` lock identity, `:110,136` `livenessOf` | VULN-PROBE (false-Dead prunes a live guard member / seats a successor) → constructor + token via 4.1.6/4.2 |
| `internal/gaterun/guard.go:165,206,319` member joins | SAFE-STRUCTURAL (same-origin recorded values); `SameRef` in step 5 |

### registry, report, contract, goal, validate, command layer
| Site | Verdict / required change |
| --- | --- |
| `internal/registry/slots.go:70` slot owner liveness | VULN-PROBE (false `DeadOwner` invites acting on a live owner's slot) → token via 4.2; non-decisive mismatch classifies Unknown, which acts not |
| `internal/registry/append.go:20-33` `LockedAppend` | consumer of `lock.Identity` + probe; healed by 4.1.6 (ED-R1-008's registry-append hazard) |
| `cmd/metasystem/supervise_owner.go:106` registry-lock self, `:200-211` `kernelProbe` | **VULN-DROP + VULN-PROBE (ED-R1-008)** → constructor + full-ref probe (4.1.6) |
| `cmd/metasystem/supervise.go:124-134` supervision-status owner liveness | VULN-DROP (diagnostic, but its printed verdict steers humans) → canonical loader (4.1.6) |
| `internal/report/scan.go:218-226` run probe state | VULN-DROP (run record carries pair; ref built without it) → full ref |
| `internal/report/scan.go:281` watcher attestation liveness | VULN-PROBE (attestation is seconds-only) → attestation schema (4.2) |
| `internal/contract/contract.go:1451` mission process liveness | VULN-PROBE (`census.Alive`) → full-ref liveness; record already carries what dispatch wrote |
| `internal/contract/contract.go:1490-1493` fixture table | FIXTURE |
| `internal/goal/journal.go:158-190` owner liveness + `callerIsOwner` | SAFE-PAIR; owner record gains Darwin micro (4.2b) |
| `internal/goalrevision/lock.go:90-110` revision-lock holder | SAFE-PAIR (pair preferred at :94; nano/seconds fallbacks for legacy holders gain non-decisive semantics via core conversion) |
| `internal/validate/wrappertoken.go:66-70` wrapper ancestry token | VULN-PROBE (`tree.StartedAtSec` fresh vs recorded second) → token carries pair (4.2), compare via `Compare` |

## 6. Migration: records already on disk with drifted epochs (four machines)

The decisive fact: a drifted epoch in a record is HARMLESS once every
reader prefers the token, because the token in the same record is
correct. Drift never corrupted the pair — both strikes showed identical
ticks/bootId. Therefore:

1. **No record rewrite, ever.** Existing records keep their drifted
   epochs; those become display values. A migration that rewrote epochs
   would need its own identity proof to know which process a record names
   — circular, and pointless once readers prefer the token.
2. **Token-carrying records** (announcements, owner.json, state.json, job
   records, run records, steward records, lease records, goal journal,
   goalrevision locks — the bulk of the fleet's records) are immune the
   moment the reader-side changes land. Nothing to do per machine.
3. **Tokenless legacy records** (the 4.2 schemas before their writers
   land: locks, registry claims, guard members, markers, attestations,
   heartbeats, acknowledged entries, wrapper tokens, census inventory)
   continue under exact legacy seconds — **with 2.2.3's non-decisive
   semantics**: a seconds match still proves the join; a mismatch against
   a token-carrying probe yields Unknown and authorizes nothing
   (ED-R1-010). The residual costs, honestly: (a) availability — a
   drifted legacy record can refuse (dispatch admission, a lease claim,
   an acknowledged silence) until enriched or healed; (b) parked
   husks — a legacy record whose process died AND whose pid was recycled
   by a live stranger reads Unknown, so its lock/slot/entry waits until
   the squatter exits (then probes decisively Dead) or the heal below
   rewrites it. Neither cost authorizes action against a live process,
   which is the property round 1 lacked.
4. **Per-schema enrichment** resolves legacy records without a rewrite
   campaign, because every 4.2 schema is rewritten in its normal
   lifecycle: heartbeats every interval, locks at each acquire,
   acknowledged entries at the next acknowledge, guard members per gate
   run, markers/attestations per watcher interval, census inventory per
   scan, announcements through the 6.6 reuse path. Each rewrite goes
   through the new writers and lands the token.
5. **Per-machine step, once, after landing:** the already-recorded heal
   sequence — lawful owner shutdown + `metasystem up` re-arm (exercised
   twice on m0, self-servable under the R-34-m0 approval) — rewrites
   owner.json, state.json, the component ledger, heartbeats, and locks
   with token-carrying identities from the new writers. Announcements
   refresh on each session start; the census prunes stale ones. Four
   machines, one bounded action each, no coordination required (mixed
   versions read both shapes).
6. **The legacy-announcement reuse path (ED-R1-010's lineage half).**
   The `Announce` finder (`internal/lease/verbs.go:126-139`) today skips
   a legacy announcement whose recorded second drifted (:137) and falls
   through to minting a fresh mainId for the SAME live process — a new
   lineage, the KI-24 split-identity shape. Defined path: when the pid
   matches but the legacy seconds do not, the announcement is REUSED —
   not shadowed by a mint — iff two recorded copies agree with the
   caller: `SessionId` equals the caller's session AND `CommandHash`
   equals the hash of the caller's command (both fields are already in
   the record, `internal/lease/classify.go:22,33`, and both are
   flows-by-copy values drift cannot touch). On reuse the announcement
   is rewritten in place with the caller's live token (enrichment,
   preserving `MainId`). If either secondary proof fails, the pid was
   genuinely recycled from a dead session: mint fresh, as today, and the
   census prunes the stale record. No new mechanism — two existing
   fields become the secondary proof the remedy asked for.
7. **No tolerance window during migration.** The legacy rule stays exact
   (a match must be exact to prove anything; 2.2.3 only changes what a
   MISMATCH may authorize). Loosening seconds fleet-wide to smooth the
   transition would widen the false-match surface for every legacy
   record at once (section 7A).

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
   toleranced equality cannot be a map key; those sites need restructuring
   anyway — at which point the token is the same amount of work with none
   of the false-match surface.
4. It leaves the Custodian/Killable false-verdict classes (section 3)
   merely narrowed, not closed, and does nothing for mainId stability.

Note the 2.2.3 non-decisive rule is NOT a band: an exact match is still
required to prove sameness in every mode; the rule only stops an
unprovable mismatch from authorizing destruction. Bands weaken the
match; 2.2.3 weakens only the mismatch's authority, in the direction
Unknown already points.

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

### D. A machine/namespace incarnation field in every record

Raised by ED-R1-001's alternate remedy. Rejected in favor of the 2.1
domain declaration: every existing join runs inside one machine's
artifacts root, so the field would be written everywhere and compared
nowhere — a subsystem to save a sentence. Revisited only if a
cross-machine identity join is ever proposed (2.1 marks the tripwire).

## 8. Coupling to same-process-succession (KI-33) — stated, not solved

`mainId` embeds the epoch at mint time: `main-<pidStartedAt>-<pid>-<hex>`
(`internal/lease/verbs.go:182`). Two consequences, no more:

1. **This design freezes mainId's epoch as an opaque token.** After it, no
   code may expect the epoch inside a mainId to equal any probe's derived
   second — comparisons of mainId are string-vs-string (lease lineage,
   goal-history actors, census announcement schema), and the re-mint
   guard is the pair-aware announcement finder
   (`internal/lease/classify.go:166`) plus the 6.6 legacy-announcement
   reuse path, which together prevent drift from minting a second mainId
   for one live process whatever shape its announcement has (the KI-24
   split-identity recurrence, closed for both record generations).
2. **Same-process-succession work must inherit that reading.** KI-33's
   succession chain construction consumes lineage strings that carry these
   frozen epochs; any succession rule that re-derives or re-validates the
   embedded epoch against a live probe would reintroduce this defect. That
   goal decides its own semantics; this design only forbids it one wrong
   move.

## 9. Simplicity accounting (R-11)

Net machinery added: one record-comparison function (`SameRef`), one
result field (`Decisive`), one relocated loader with an explicit
five-key `FieldMap`, one relocated writer helper, one `lock.Identity`
constructor, one AST-based conformance test. Net machinery removed: the
hand-rolled comparison idioms — including the previously-sanctioned
"safe" ones — collapse into the identity core; one redundant seconds
gate is deleted outright; the census gains one loader instead of three
field-extraction sites; the round-1 grep ratchet's allowlist (a standing
maintenance surface) is gone. Schema growth is optional token fields on
records that already carry identity fields, spelled by each schema's
declared FieldMap. No new processes, files, locks, or configuration. The
alternative designs each add a cache file, a tunable window, an
incarnation field, or an allowlist.

## 10. Gaps (stop-and-report; none filled silently)

- **G1 — Darwin start-token stability is asserted, not tested here.** The
  claim that Darwin's kernel start record does not drift under clock
  discipline (2.2.2) matches the existing code's design comments and
  `identity_darwin.go`'s use of the kernel's spawn-time record, but the
  goal record says "macOS unchecked" and this sandbox is Linux. Revision
  2 removes the schema consequence (4.2b carries the Darwin token
  everywhere regardless), so a Darwin drift discovery would change a
  token VALUE, not the schemas; the residual gap is verification only.
- **G2 — The terminate-flake causal link is a lead, not a proof.** I
  traced the drift-vulnerable members of the kill/wind-down paths
  (`janitor.Killable`, the `Custodian` binding chain, proofrun guard) but
  cannot run the failing tests here to confirm which one produces the
  observed reds. The design fixes all of them; the goal's verification
  leg should re-run the family on m0 after landing.
- **G3 — Chrony/NTP forensics were not performed** (goal: "chrony logs
  unchecked"). The design does not depend on which discipline event moved
  btime, only on the kernel-documented fact that btime is wall-clock
  derived; noted for completeness.
- **G4 — Fixture shell scripts not read line-by-line.** Narrowed in
  revision 2: `scripts/agents/dispatch.sh` was swept and contains no
  start-identity comparison (one comment, transport values only), and
  the one Go-side gate it fed is deleted (ED-R1-006). The remaining
  fixture scripts matching `pidStartedAt` are fixture/test/transport
  code on inspection of their roles; a hidden comparison there would be
  test-only exposure, and 4.4 states the shell boundary honestly.
- **G5 — CLOSED in revision 2.** The lease classify lookup was read:
  `internal/lease/classify.go:357-362` builds the join key from a fresh
  probe (`StartedAt(current, probe)`), confirming VULN-PROBE; the
  loaders at :265-270 and :288-291 additionally drop the token
  (VULN-DROP). Both are in Ruling R with conversions.

## 11. Self-grade (R-24)

**Trajectory: 12 material findings in round 1 → 0 targeted by this
revision; every finding is folded (none rebutted away) and section 12
gives the critic a mechanical join.**

**Grade: A−.** Every round-1 finding is folded with the mechanism named
at file:line, and the folds obey the convergence discipline: the
critical legacy hole is closed by narrowing what a mismatch may
authorize (one result field, no new subsystem); the uniqueness claim is
narrowed to a declared domain instead of growing an incarnation field;
the guard trades its allowlist for structure; and one gate is deleted
rather than converted. Two of round 1's five open gaps closed against
the tree (G5 read to the line, G4's load-bearing half swept). Held back
from A: G1 and G2 remain asserted-not-verified on the motivation path
(both need machines this sandbox is not), and the AST guard's field-name
list, while structural at the comparison level, still enumerates
spellings — a renamed field would need the test touched, which is the
intended ratchet but a ratchet nonetheless.

## 12. Round-1 disposition matrix (mechanical join for the next round)

| Finding | Disposition | Folded at |
| --- | --- | --- |
| ED-R1-010 (critical, legacy fallback authorizes) | FOLDED | 2.2.3 (token-wins law), 4.1.2 (`Decisive` + Unknown mapping, lease claim three-way), 6.3 (residual costs), 6.6 (announcement reuse path), 7A note |
| ED-R1-001 (uniqueness scope) | FOLDED (claim narrowed; incarnation field rejected as 7D) | 2.1, 7D |
| ED-R1-002 (zero start tick) | FOLDED | 2.2.1, 4.1.3 (presence keyed to bootId; `Exact.Ref`/`Ref.Mode` fixes; omitempty round-trip shown) |
| ED-R1-003 (SameRef weakens invalid shapes) | FOLDED | 4.1.1 (invalid-first rule; cross-platform exact → invalid; fallback only for valid shapes) |
| ED-R1-004 (Darwin token not propagated) | FOLDED | 2.2.2, 4.2 (both-token rule), 4.2b (eleven-schema propagation list), G1 narrowed |
| ED-R1-005 (watcher-ceiling attestation join) | FOLDED | 4.2 heartbeat row, Ruling R supervise + dispatch tables |
| ED-R1-006 (custody-add seconds gate) | FOLDED (gate deleted) | 4.3 REDUNDANT-GATE, Ruling R dispatch table |
| ED-R1-007 (Custodian binding chain) | FOLDED | 4.1.4 (six-link chain enumerated), Ruling R supervise/steward/mission tables |
| ED-R1-008 (lock constructors and probes) | FOLDED | 4.1.6 (constructor + nine enumerated literals + probe bindings + status reader), Ruling R command-layer table |
| ED-R1-009 (census InventoryItem discards the pair) | FOLDED | 4.2 InventoryItem row, `Acknowledge` binding row in Ruling R |
| ED-R1-011 (centralization vs conversion conflict) | FOLDED | 2.3 (the law wins everywhere; SAFE ≠ exempt), 4.3 step 5, 4.4 (AST guard, no allowlist) |
| ED-R1-012 (loader dialects) | FOLDED | 4.1.5 (`FieldMap`/`RefFromParts`; no renames; prefix rule; ambiguity impossible by construction) |
