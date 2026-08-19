# Issue #13: the supervision arming path stops comparing btime-derived seconds

Working Mode: implement

Owner: main session (Claude, lease holder), under Wido's ruling of 2026-08-19
("do the complete fix"). Issue #1 SWEEP 3: the supervision subsystem's identity handling, which
sweeps 1 (lease+census) and 2 (run+waiter records) did not reach. r1 of this
design under-scoped it to two call sites; codex gpt-5.6-sol's critique
(plans/issue-13-critique-r1.json, 9 findings, REVISE) showed the whole
supervision identity chain is seconds-only. r2 folds them. Critique:
codex gpt-5.6-sol.

## The defect (KI-37 / GitHub #13)

On a time-synced guest (Lima steps the clock ~every 20s), `/proc/stat` btime
moves, so a process's btime-derived start SECOND differs between two reads of
the same live process. Two places in the arming path compare such seconds and
so false-death a live process, flaking `scripts/validate-metasystem.sh` at the
supervision fixtures ("announcement pid identity is not live"):

1. `internal/lease/verbs.go:46` `Announce`: re-probes `actualStart` and fails
   when `actualStart != start` (the caller's second, read moments earlier).
2. `internal/census/run.go:496` `identityAlive` (backing `proc alive`):
   `return exact == expectedStart` — seconds equality against a re-probe.

Issue #1 already fixed the drift by carrying the clock-step-immune pair
(`StartTicks` + `BootID`) and comparing via `identity.sameIdentity` /
`AliveRef`. Records already store the pair; the census SNAPSHOT verifier
already uses it (`run.go:284`, `:629`). These two call sites and the shell
verbs feeding them are the unswept remainder.

## The fix — sweep the supervision identity chain to the pair (F-1, F-2)

Every place the supervision subsystem RECORDS a process identity also stores
the clock-step-immune pair (`StartTicks`+`BootID`, already on `identity.Exact`),
and every place it VERIFIES one prefers the pair, falling back to seconds only
when the pair is absent (old records, darwin). Reuse `identity.sameIdentity`
via `identity.AliveRef` / a `Ref`; no new comparison logic. The sites, from the
critique's inventory:

1. **The main announcement** — `internal/lease/verbs.go` `Announce`. ONE
   authoritative probe (F-3): read the live process once (`identity.Exact`),
   derive start-second, ticks, bootid and command hash from that single
   `Exact`, not from four independent reads. Verify: alive+readable, and when
   the caller passes the pair, `sameIdentity`; the RECORDED `PidStartedAt`,
   `PidStartTicks`, `BootID`, and the `MainId` seed are all from that one probe
   (F-4). Drop the `actualStart != start` gate.

2. **The supervision owner lock** — `supervise write-owner-identity` →
   `owner.json`. Store the pair beside the second; the join / rearm / shutdown /
   takeover reads (`arm-supervision.sh`, `internal/supervise`) that compare the
   owner's liveness use the pair (F-2).

3. **Component identities** — the watcher/reaper records in
   supervision `state.json`, and the census SUPERVISION-SNAPSHOT verifier that
   checks owner/watcher/reaper liveness (`internal/census`, the
   `verifySupervisionSnapshot` path — NOT run.go:284/629, which are the
   inventory join and announcement pruning) (F-1). Store and compare the pair.

4. **The `proc alive` verb backend** — `internal/census/run.go` `identityAlive`
   and `census.Alive`: accept the pair; compare via `sameIdentity` when present,
   seconds otherwise.

5. **The verbs** — `proc alive` gains `--start-ticks`/`--boot-id`;
   `proc started-at` gains `--emit pair` (SECONDS TICKS BOOTID); `proc probe`'s
   JSON gains `startTicks`/`bootId` (it omits them today — F-9). `lease announce`
   gains `--start-ticks`/`--boot-id`.

6. **The shell** — `arm-supervision.sh`, `fingerprint-harness.sh`,
   `supervision-fixtures.sh`: derive the pair in ONE read and pass it through
   `identity_alive`/`proc alive`, `lease announce`, and the owner-identity
   write. The two-read window and the seconds gates disappear.

### Backward and mixed-version compatibility (F-5)

- A NEW record ALWAYS keeps the second (old binaries still read it; they were
  seconds-flaky before and are no worse). A NEW reader prefers the pair when
  the record has it, else the second. An OLD record (no pair) read by a NEW
  reader → seconds (unchanged). So new+new is drift-immune; every other
  combination is exactly today's behaviour. The second is never dropped.

### Partial pair and supplied-start (F-6, F-7)

- The pair is BOTH-OR-NEITHER: a record/flag with exactly one of ticks/bootid
  is rejected (`ticks>0 XOR bootid!=""` → error), never silently downgraded.
- A caller that passes only `--start-time` (e.g. a mission driver) keeps the
  seconds path for THAT call — but `arm-supervision.sh` always derives the pair
  itself from the pid it is arming, so the supervision fixtures exercise the
  fixed path regardless.

## Safety

- No new comparison logic: every pair comparison reuses the rule issue #1
  shipped and tested (`sameIdentity` / the `run.go:284` idiom).
- Pair absent (darwin, or a legacy caller) → the seconds comparison stands
  verbatim. Strictly additive.
- Reuse detection is NOT weakened: a reused pid has different ticks, so the
  pair comparison still reads Dead; the second is only consulted when no pair
  exists (where it was the only signal anyway).
- `lease.Announce` recording the fresh probe instead of gating on the caller's
  stale second removes a false-negative; a pid handed to Announce and re-probed
  alive with matching pair IS that process.

## Acceptance

- `scripts/validate-metasystem.sh` green on the Linux guest across repeated
  runs (the flake is load/drift-sensitive; prove it with several back-to-back
  green runs on a quiescent guest), and on macOS.
- New Go fixtures: (a) `census.Alive`/`AliveRef` with a `FixtureProbe` whose
  btime-second differs between reads (pair unchanged) → alive with the pair,
  dead on seconds-only fallback; (b) the SUPERVISION-SNAPSHOT verifier reads
  owner/watcher/reaper alive across a simulated btime step when the records
  carry the pair; (c) partial-pair rejection; (d) a reused pid (different
  ticks) reads dead.
- `lease.Announce` fixture: same live pid, caller's second stale by a btime
  step, pair supplied → announces; pair mismatch (reused pid) → refuses.
- go build, go vet, coverage ratchet, full suite green on both platforms
  (guest via the KI-37 retry until this fix makes the retry unnecessary).
