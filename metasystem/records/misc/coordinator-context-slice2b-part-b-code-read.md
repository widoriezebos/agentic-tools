# Independent code read: coordinator-context slice 2b part B

Worktree: `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccb-s2b/metasystem`
Base: HEAD `06dc9aef` (part A). Change under read: the working-tree `git diff`
(`cmd/metasystem/main.go`, `internal/output/output.go`, `internal/output/output_test.go`,
`internal/steward/health.go`) plus untracked `cmd/metasystem/context_verbs.go`,
`cmd/metasystem/context_verbs_test.go`, `internal/steward/context.go`,
`internal/steward/context_test.go`. Nothing else is modified.

Spec: `artifacts/reports/codex-ccb-slice2b-brief-v2.md` part B,
`artifacts/reports/ccb-8c10-slice2b-amendment.md`, and section 8c of
`plans/coordinator-context-stays-under-budget-design.md`. Parts C and D absent by
instruction; the missing `context report` verb, `internal/steward/contextreport.go`,
`usage.CallSessions`, fixture wiring, `testing.json` and the cost proof are
therefore not findings. I did not run build, vet, the race tests or the fast gate;
the seat owns those.

## What conforms

- Signatures, constants and option struct match the brief exactly
  (`internal/steward/context.go:20-56`, `:107`).
- `RoleContext` sits directly after `RoleStopHookDuration` in both
  `healthRoleOrder` (`internal/steward/health.go:76`) and the timed evaluation list
  (`internal/steward/health.go:366`), and it is wrapped in `timed(...)` so it carries
  `DurationMillis`.
- One evaluation owns identity, capability lookup, registration, the part A read and
  the verdict; health and the CLI both call it
  (`internal/steward/context.go:56-110`, `cmd/metasystem/context_verbs.go:63`).
- Verdict boundaries are exact and compared on `int64` before display rounding
  (`internal/steward/context.go:231-244`): 150,000 alive with the bound sentence,
  150,001 alive plus the handoff sentence, 200,000 alive plus the handoff sentence,
  200,001 dead with `NoAutomaticRemedy = true`. The table test asserts all seven
  points (`internal/steward/context_test.go:21-27`).
- **A dead verdict cannot fire on missing evidence.** `contextVerdict` returns dead
  only inside the `reading.Latest != nil` branch (`internal/steward/context.go:218`,
  `:233`). Every nil-sample path returns alive or unknown. Every identity, registry,
  toplevel and reader error path returns `roleUnknown`, never dead
  (`internal/steward/context.go:60, 71, 77, 85, 97`). I found no route from absent,
  unreadable or ambiguous evidence to `HealthDead`.
- Benign unknowns are exactly the three no-call reasons plus the two unsupported
  capabilities (`internal/steward/context.go:247-259`); the strings match
  `internal/usage/cursor.go:531`, `:533` and `internal/usage/calls.go:149`.
  `cannot read rollout directory` (`internal/usage/calls_codex.go:100`) is correctly
  *not* benign and is proven unknown at `internal/steward/context_test.go:65-78`.
  No operational failure is swallowed: `LatestCall` errors are returned as both a
  role reason and a Go error (`internal/steward/context.go:97`).
- Explicit identity does not register a synthetic session
  (`internal/steward/context.go:75`), proven at
  `cmd/metasystem/context_verbs_test.go:51-53`.
- `NewestSince` uses `Lstat`, requires `IsRegular`, requires strictly `After(since)`,
  breaks ties on lexical basename and returns false for a missing or unreadable
  directory (`internal/output/output.go:48-71`). It cannot return a file at or older
  than the previous validated read. Repeated unchanged reads keep naming the same
  spill, which the brief explicitly permits.
- Status JSON is a bounded projection with lower-camel names and no `Seen`/`Tail`
  (`cmd/metasystem/context_verbs.go:20-36`, `:78-88`); `RoleVerdict` already carries
  lower-camel tags (`internal/steward/health.go:89-99`). The 6,638-id test bounds the
  payload at 4 KB and asserts exactly five cursor members
  (`cmd/metasystem/context_verbs_test.go:97-133`).
- The `Line()` extraction is a faithful move: `HealthVerdict.Line()` now delegates per
  role to `RoleVerdict.Line()` with byte-identical formatting
  (`internal/steward/health.go:186-220`).
- Adding a role to `healthRoleOrder` is backward compatible with existing
  `health.json`: `loadHealthRecord` rejects only roles *not* in the order, never a
  missing one (`internal/steward/health.go:1548-1577`). `healthFindingDigest` keys on
  role and status only, so the variable spill hint in the reason cannot churn alert
  episodes (`internal/steward/health.go:1523-1532`).
- Samples-log recovery is a truncate plus fsync, not a scan, so it adds O(1) work to
  the Stop path (`internal/usage/cursor.go:455-471`).
- Scope is clean: no fixture script, `testing.json`, coverage-ratchet, role contract
  or template change; `scripts/agents/supervision-hook.sh` untouched;
  `memory/receipts.log` is tracked and **unmodified** (`git status` is empty for it,
  and it contains no ccb/slice2b row), so there is no stray receipt.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| F-1 | medium | yes | CCB-2-15 requires `context status` to exit 0 on an `unknown` role, and the brief's mapping table requires exit 0 for a readable unknown `Reading` and for a reported ceiling breach. No test asserts either. The named proof covers only alive statuses, and every unknown case in the CLI tests asserts exit 1. A regression that returned 1 for all unknowns, or that failed on a breach, passes the suite. | `cmd/metasystem/context_verbs_test.go:14-95` (alive only), `:135-213` (all unknown cases assert `code != 1` fatal); behaviour lives at `cmd/metasystem/context_verbs.go:71-75`; row text `plans/coordinator-context-stays-under-budget-design.md:277` and the brief's "0 for a readable `Reading`" / "A reported breach exits 0" rows |
| F-2 | high | yes | The holder resolver never probes process liveness, so a lease whose holder has died but has not yet been taken over still yields that session's last sample. If it was over the ceiling, every health evaluation in the checkout reports `context-budget=dead` with `NoAutomaticRemedy`, which escalates `NO_LAWFUL_REMEDY` to the human against a seat that has no such context. D2-7 says the role "finds the seat as `checkSessionMain` does", and `checkSessionMain` classifies each candidate with `identity.AliveRef(prober, ref)`; the new resolver decodes `pid`/`pidStartedAt` only to hand them to `RegisterSession`, and `checkContextBudget` is not even given the prober. | `internal/steward/context.go:112-167` (no prober, no liveness), `:107-110` (signature drops the prober while every neighbouring check takes one at `internal/steward/health.go:363-365`), versus `internal/steward/health.go:974` (`identity.AliveRef`); design sentence at `plans/coordinator-context-stays-under-budget-design.md:239` |
| F-3 | medium | yes | The announcement scan validates and errors on **any** file in `artifacts/agents/mains`, not just the holder's, and it does so **before** comparing `mainId` to `holderMainId`. One foreign, legacy or half-written announcement missing `sessionId`, `runtime`, `pid` or `pidStartedAt` turns the role `unknown` even though the holder's own announcement is present and well formed; two consecutive observations then escalate an alert. This contradicts the two readers the design cites as the pattern: `checkSessionMain` skips such entries, and the lease classifier documents the rule explicitly ("A record without the one-writer identity fields is always skipped ... so one such file cannot refuse every write in a checkout"). | `internal/steward/context.go:145-158` (returns an error for every entry, before the `mainID == lease.HolderMainID` test at `:159`) versus `internal/steward/health.go:951-963` (`continue` on unreadable/empty) and `internal/lease/classify.go:136-139` |
| F-4 | medium | yes | The role can consume or exhaust a Stop attempt. `checkContextBudget` reaches `unix.Flock(fd, LOCK_EX)` with no timeout and no `LOCK_NB` fallback, and the hook calls `health --hook-preview` with no timeout wrapper. `PreviewHealthAt` takes no health lock, so a hook preview and the steward tick's `ObserveHealth` can reach the same per-session cursor lock concurrently; the loser blocks for the whole of the winner's cold parse. The only backstop is the parent's 57-second deadline, which converts the Stop into a provider-level refusal. Every neighbouring role that touches a contended lock uses a non-blocking "busy" unknown instead. Part D/CCB-2-46 owns the *measurement*, but a measurement does not make the wait bounded. | `internal/steward/context.go:88` → `internal/usage/cursor.go:35` → `internal/usage/cursor.go:549-563` (blocking `LOCK_EX`); `scripts/agents/supervision-hook.sh:959` (no timeout); `internal/steward/health.go:281-286` (preview takes no health lock) versus `internal/steward/health.go:237` (observe does); `internal/steward/tick.go:271` (concurrent observer); the non-blocking precedent at `internal/steward/health.go:532`; budget at `scripts/agents/supervision-hook.sh:99-107` |
| F-5 | medium | yes | `--transcript` combined with an **inferred** holder repoints the live holder's cursor at the override file. `readUnderCursor` sees `cursor.Path != path`, restarts the cursor and clears `Seen` while keeping `SamplesBytes`. Three consequences: (a) the override's calls are all appended as "new" committed samples; (b) the next health read restarts again on the real path and re-parses the entire real transcript from offset 0 inside the Stop hook, re-appending every call as a duplicate committed sample; (c) while the cursor points at an empty or irrelevant override, `SampleCount == 0` makes the reading report `unknown (no call recorded yet)`, which is a **benign alive** role, silently masking a real ceiling breach until the next real read. The brief does authorise the flag on inferred sessions, so this may close as out of scope, but as shipped an operator-run status command mutates the holder's reader state, injects duplicate evidence into the store the week report will count, and forces one cold Stop-path re-parse. | `cmd/metasystem/context_verbs.go:63-65` (transcript passed with an empty runtime/session); `internal/usage/cursor.go:76-81` (path-change restart keeps `SamplesBytes`, clears `Seen`); `internal/usage/cursor.go:521-533` (`SampleCount == 0` → benign reason); `internal/steward/context.go:247-258` (that reason is benign-alive); replay hazard described at `artifacts/reports/ccb-8c10-slice2b-amendment.md` decision 10.1 |
| F-6 | low | no | `contextGitToplevel` hard-errors when no ancestor `.git` exists, turning a benign layout into an `unknown` role and CLI exit 1, even though the reader tolerates an empty `Toplevel` and falls back to `Installation`. Also, an unrelated ancestor `.git` (for example a versioned `$HOME`) silently produces the wrong Claude project slug. Every supported installation today is inside a checkout, so no current path is broken. | `internal/steward/context.go:81-87`, `:193-215` versus the tolerant candidate loop at `internal/usage/calls_claude.go:22-38` |
| F-7 | low | no | The local variable `identity` in `ContextBudgetLine` shadows the `internal/identity` package name that the same Go package uses elsewhere (`identity.Prober`, `identity.AliveRef`). It compiles because `context.go` does not import that package, but it is a trap for the next editor, and F-2's fix would need the package here. | `internal/steward/context.go:58`, `:62`, `:66`, `:76` versus `internal/steward/health.go:974` |
| F-8 | low | no | The symlink-exclusion assertion in the `NewestSince` test proves nothing on its own terms. The symlink's mtime is never set with `Chtimes`, so it is real wall-clock time; the assertion that it is excluded only holds because the machine's clock happens to be later than the hard-coded `2026-09-13T10:04Z` bound. On a clock before that instant the case passes vacuously. Adding one `os.Chtimes` on the link with an explicitly newer time would make it a real proof. | `internal/output/output_test.go:153-157` (symlink created, never `Chtimes`d), `:169-171` (the exclusion assertion uses `since.Add(4*time.Minute)`) |
| F-9 | low | no | `context status` writes straight to stdout and never routes through the `output.Spill` envelope that `test` and `output spill` use. The brief does not ask for it and the JSON is bounded under 4 KB by construction, so nothing overflows; recording it because the read was asked to check it. | `cmd/metasystem/context_verbs.go:66-70` versus `cmd/metasystem/test.go:1491-1495`, `cmd/metasystem/output_verbs.go:39` |
| F-10 | low | no | The builder's result lists `memory/receipts.log` as a changed file. It is tracked and unmodified, and contains no ccb/slice2b row. The scope outcome is the desired one (no stray receipt), but the returned changed-file list is inaccurate, which weakens the return as evidence. | `artifacts/reports/codex-ccb-slice2b-result.md` "Changed files" section versus empty `git status --porcelain -- memory/receipts.log` and `grep -c 'slice2b\|ccb' memory/receipts.log` = 0 |

## Answers to the named questions

1. **Conformance.** Every part B row is implemented with the specified signature,
   constant, ordering, reason string and exit mapping. The one row whose named test
   does not assert what the row claims is CCB-2-15's "exit 0 on `unknown`" (F-1).
2. **Boundaries.** 150,000 alive, 150,001 alive plus handoff sentence, 200,000 alive
   plus handoff sentence, 200,001 dead. Comparison is exact `int64` before rounding.
   A dead verdict is structurally unreachable from missing, unreadable or ambiguous
   evidence. It **is** reachable from *stale* evidence about a departed holder (F-2)
   and can be temporarily *suppressed* by a transcript override (F-5).
3. **Identity.** Explicit pair bypasses the lease and does not register; unpaired
   flags exit 2; unregistered explicit runtime exits 1 (matching part A's
   `runtime context-sample` convention); unregistered inferred runtime stays alive
   with a precise reason; missing lease, empty `holderMainId` and no matching
   announcement each give alive with a precise no-holder reason; malformed lease is
   unknown plus an error. A stale holder is not detected (F-2), and a foreign
   malformed announcement poisons the read (F-3). Any seat running health in a
   checkout is judged on the lease holder's budget; that is D2-7's design, and it is
   only a problem when the holder is stale.
4. **Unknowns.** Correct split. Nothing operational is swallowed: every failure
   returns both a role reason and a Go error, and the CLI exits 1 on exactly those.
5. **Newest spill.** Correct on symlinks, directories, equal mtimes, older files and a
   missing or unreadable directory; deterministic on ties; cannot return a file at or
   older than the previous validated read. Test weakness at F-8.
6. **Placement and timing.** Placement and timing instrumentation are correct. The
   role adds a blocking, untimed exclusive flock plus a potentially full cold
   transcript parse to the Stop path (F-4). Samples-log recovery itself is cheap
   (truncate plus fsync), so recovery is not the cost driver; the cold parse and the
   whole-cursor re-render are.
7. **The verb.** Exit codes, bounded output, transcript override and template-root
   resolution are right; the unknown-session exit-0 case is untested (F-1); output
   does not use the spill envelope but does not need to (F-9).
8. **Scope.** Clean. No part C or D artefact, no fixture, no `testing.json`, no
   coverage floor, no role text, no hook change, no receipt row.

## Verdict

**Not yet fit to land.** Five material findings stand. To land, F-1 needs the two
missing CLI assertions (an unknown-source read and a ceiling breach each exiting 0),
F-2 needs either a liveness probe on the resolved holder or an explicit recorded
decision that a stale holder may raise `NO_LAWFUL_REMEDY`, F-3 needs the announcement
validation narrowed to the matching holder so a foreign record cannot make the role
unknown, F-4 needs a bounded or non-blocking cursor-lock acquisition with a "busy"
unknown in the Stop path (or a recorded decision deferring it to part D with the risk
named), and F-5 needs `--transcript` either restricted to an explicit runtime/session
pair or made non-destructive to the holder's live cursor. F-6 through F-10 are
recorded only.

Material findings: 5 (F-1, F-2, F-3, F-4, F-5).
# Closing read: coordinator-context slice 2b part B, folds 1 and 2

Worktree: `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccb-s2b/metasystem`
Base: HEAD `06dc9aef` (part A). Under read: the working-tree `git diff`
(`cmd/metasystem/main.go`, `internal/output/output.go`, `internal/output/output_test.go`,
`internal/steward/health.go`, `internal/usage/calls.go`, `internal/usage/cursor.go`,
`internal/usage/cursor_test.go`) plus untracked `cmd/metasystem/context_verbs.go`,
`cmd/metasystem/context_verbs_test.go`, `internal/steward/context.go`,
`internal/steward/context_test.go`. Nothing else is modified.

First read: `artifacts/reports/opus-read-part-b.md`. Fold 2 design:
`artifacts/reports/ccb-8c11-transcript-override-amendment.md`.

I did not repeat the seat's build, vet, race and gate runs. I ran two mutation
probes through `go test -overlay` (no repository file was edited, no commit
made); they are cited in F-1 below.

## 1. Fold 1, findings F-1 to F-4

**F-1 (exit codes): fixed, with one branch of the row still unproven.**
`TestContextStatusExitCodesForReadableUnknownAndCeilingBreach`
(`cmd/metasystem/context_verbs_test.go:101`) adds the two assertions. The
readable-unknown case (`:104-109`) drives the real live path with an explicit
pair and no transcript, reaches `unknown (no transcript at ...)` and asserts
exit 0. That one is a true proof. The ceiling-breach case (`:111-117`) passes
`--transcript`, so it proves exit 0 on a *diagnostic* breach, not on a live
one. The mapping itself is shared (`cmd/metasystem/context_verbs.go:78-82`
keys only on `readErr`), so the substance holds, but the row's live branch has
no assertion. Recorded as G-2. Neither assertion fails on pre-fold behaviour,
which is expected: F-1 was a proof gap, not a behaviour gap.

**F-2 (holder liveness): fixed and right.**
`resolveContextIdentity` now takes a prober and classifies each matching
announcement with `identity.AliveRef` (`internal/steward/context.go:153`,
`:220`), the same call `checkSessionMain` uses (`internal/steward/health.go:974`).
`ContextBudgetLine` keeps its public signature and delegates to
`contextBudgetLineWithProber` with `identity.KernelProber{}`
(`internal/steward/context.go:63-67`); health passes the evaluation's prober
(`internal/steward/health.go:366`).

- *Can a live seat now be judged unknown when it should be judged?* No. The
  ref is built from exactly the fields `processRef` builds it from —
  `pid`, `pidStartedAt`, `pidStartTicks`, `bootId`
  (`internal/steward/context.go:212-215` versus
  `internal/steward/health.go:1425-1434`) — so `Ref.Mode()` picks the same
  comparison on both paths: ticks-plus-boot on Linux, legacy seconds on
  darwin, `CompareInvalid` (hence unknown) only when ticks and bootId
  disagree about being present, which is identical for `checkSessionMain`.
  The filename filter is `census.IsAnnouncementFile`, which excludes only the
  three protocol files and protocol cursors
  (`internal/census/announcement.go:16-26`), so no real announcement is
  skipped by name. Alive on the first matching alive record, unknown
  preferred over dead — the same precedence as `checkSessionMain`.
- *Can a dead holder still produce a dead verdict?* No. A dead or
  unknown-liveness holder yields `unobservableReason`, and for the only
  capability that can produce a sample the function returns `roleUnknown`
  before registration and before any read
  (`internal/steward/context.go:88-90`).
  `TestContextBudgetDoesNotAttributeUsageToAStaleHolder`
  (`internal/steward/context_test.go:251`) seeds 210,000 tokens — over the
  ceiling — and requires unknown, no `NoAutomaticRemedy`, no `Latest`, and no
  cursor file, for both dead and unknown probes. Pre-fold that case reached
  the reader and returned `HealthDead`, so the test fails on pre-fold
  behaviour. For a `per-invocation` or `none` capability the guard is skipped
  and the role answers alive with the capability reason
  (`internal/steward/context_test.go:122-136`); that is harmless, since no
  sample can exist to attribute, and it is recorded as G-6.

**F-3 (announcement scan): fixed.**
Validation now runs only after `mainID != lease.HolderMainID` has filtered the
entry (`internal/steward/context.go:197-211`); an entry that will not decode is
skipped (`:190-192`); a fully empty record cannot match a non-empty holder id
and is skipped; only the *holder's own* record missing `sessionId`, `runtime`,
`pid` or `pidStartedAt` produces an error, and even then only if no matching
record resolved alive, unknown or dead (`:239-241`).
`TestContextHolderResolutionSkipsForeignMalformedAnnouncements`
(`internal/steward/context_test.go:274`) puts an unparseable foreign record, an
incomplete foreign record and an incomplete holder record in the directory in
lexical order *before* the good holder record, and requires alive with the
real sample. Pre-fold the first entry errored, so the test fails on pre-fold
behaviour. The holder's own incomplete record still errors, proven at
`cmd/metasystem/context_verbs_test.go:268-289`. One divergence from
`checkSessionMain` is recorded as G-3.

**F-4 (busy unknown): fixed; every lock on this path is bounded.**
`ReadOptions.NonBlocking` (`internal/usage/calls.go:71`) selects
`tryLockCallFile` in `readUnderCursor` (`internal/usage/cursor.go:55-67`), and
`RegisterSessionNonBlocking` does the same for the session registry
(`internal/usage/cursor.go:289-321`), mapping `EWOULDBLOCK`/`EAGAIN` to typed
`CursorBusyError` / `SessionRegistryBusyError` (`:21-38`, `:606-623`). The
role sets `NonBlocking: true` (`internal/steward/context.go:98`).

I enumerated every lock acquisition in `internal/usage`: `cursor.go:62`
(cursor, now conditional), `cursor.go:223` (`Calls`, blocking — the week
report, not on this path and not present in this tree), and `cursor.go:308`
(registry, now conditional). Nothing else takes a lock: `reconcileCallRows`,
`appendCallRows`, `loadCallCursor` and `atomicWriteJSON` are plain file
operations, and `atomicfile.WriteText` and `wiredoc` contain no `Flock` at
all. Registration and the read take their locks sequentially, never nested.
The two proofs
(`TestContextBudgetReturnsBusyUnknownWithoutWaiting`,
`TestContextBudgetReturnsRegistryBusyUnknownWithoutWaiting`,
`internal/steward/context_test.go:301`, `:357`) each hold the lock in the test
process, run the check in a goroutine and fail on a one-second timeout, so
they fail on pre-fold behaviour by hanging. Two matching unit proofs sit in
`internal/usage/cursor_test.go:50`, `:96`.

The residual cost the first read named — a cold parse of the whole transcript
inside the Stop hook — is unchanged. The design assigns that measurement to
part D / CCB-2-46, so it is out of scope here, not a finding.

## 2. Fold 2, the private diagnostic root

**Isolation is real.** `readContextTranscriptOverride`
(`internal/steward/context.go:135-146`) allocates with
`os.MkdirTemp("", "metasystem-context-diagnostic-")` — a unique directory per
call by construction, mode 0700, outside every live evidence directory — hands
*only* that root to `usage.LatestCall`, and removes it unconditionally after
the reader has returned and released its private lock, joining a read error
and a cleanup error. Allocation failure returns an error and never falls back
to the real root. Interrupted-process residue lands in TMPDIR, where no cohort
scan looks.

**No remaining path where an override touches live evidence.** I traced every
exit of `contextBudgetLineWithProber` with `diagnostic == true`:

- `resolveContextIdentity` reads only `worktree-lease.json` and the mains
  directory, read-only, no lock.
- The five early returns (`:72`, `:75`, `:83`, `:85`, `:89`) return before any
  usage-store access.
- The diagnostic branch (`:100-106`) sits before
  `RegisterSessionNonBlocking` (`:108`), before `contextGitToplevel`
  (`:115`), before the live `usage.LatestCall` (`:121`), and before
  `output.NewestSince` (`:127`). Skipping the toplevel derivation is safe
  because both readers return `opts.Transcript` first
  (`internal/usage/calls_claude.go:15-17`, `internal/usage/calls_codex.go:13-15`).
- Inside the reader, `cursorPath` and `samplesPath` derive from the passed
  state root only (`internal/usage/calls.go:163-169`), so the cursor, the
  samples log and the cursor lock are all inside the temporary directory.

A useful side effect: `--transcript` pointing at the live samples log is now
harmless, where pre-fold it would have appended to the file it was reading.

The proofs are strong. `TestContextTranscriptOverrideUsesPrivateEvidence`
(`internal/steward/context_test.go:427`) runs both runtimes across inferred,
explicit-holder, another-explicit and override-equals-real-path, and compares a
full recursive snapshot of `artifacts/agents/context` (paths, modes, bytes)
before and after; the `fresh-inferred` case requires the directory not to
exist at all.
`TestContextTranscriptOverridePreservesTheNextHealthRead` (`:490`) seeds a live
210,001-token dead verdict, runs an empty, a copied and a distinct override,
and after each requires the next live evaluation to be dead at 210,001 with
zero new samples and markers, then appends one real call and requires exactly
one new committed sample and two total.
`TestContextTranscriptOverrideIgnoresLiveStoreFailures` (`:562`) proves the
override succeeds beside an uncommitted suffix, a corrupt cursor, a short log
and directories occupying both lock paths, without changing them.
`TestContextTranscriptOverrideDisposesPrivateCursor` (`:631`) uses the two
package-local seams `makeContextDiagnosticRoot` / `removeContextDiagnosticRoot`
(`internal/steward/context.go:56-59`) to prove three unique roots, cleanup
after a sample, an empty source and a non-regular source, an allocation
failure that reaches the caller with live evidence untouched, and a read error
preserved alongside a cleanup error. All of these fail on pre-fold behaviour,
which wrote a cursor under the caller's root.

**An override still answers correctly.** Threshold comparison is the same
shared code; only the wording and remedy branch on `diagnostic`
(`internal/steward/context.go:293-330`), matching decision 11.3: prefix
`diagnostic transcript override; ` on every return including early errors
(`labelContextDiagnostic`, `:332-337`), `; over the bound` without the handoff
sentence, `metasystem context status --root <installation>` as the dead
remedy, `NoAutomaticRemedy` kept, `PreviousReadAt` zero, no spill hint, and
`"diagnostic"` on the JSON form (`cmd/metasystem/context_verbs.go:16`, `:74`).
Empty `--transcript` is rejected with exit 2 through `FlagSet.Visit`
(`cmd/metasystem/context_verbs.go:49-58`), so the command's `Diagnostic` field
and the steward's `opts.Transcript != ""` can never disagree.

**An empty or unreadable override does not mask a real breach.** An unreadable
source errors to unknown and exit 1 (`cmd/metasystem/context_verbs_test.go:177-187`).
An empty source stays alive with `unknown (no call recorded yet)`, which
decision 11.3 chose deliberately, and it cannot suppress the live verdict
because the live cursor is untouched — proven directly at 210,001 by the
preserve test. The benign-unknown classification is computed from the raw
reader reason before the prefix is applied (`internal/steward/context.go:296`
then `:297`), so the label cannot change which reasons are benign.

**Obligation rows 48 to 52 are implemented with the named test names**, and
the amendment's coverage instruction was followed: the persistence assertions
moved to normal reads (`cmd/metasystem/context_verbs_test.go:26`, `:96`), and
CCB-2-45's corrupt-cursor, short-log and registry-error proofs now run on
normal reads through `seedContextCommandReading` (`:236-312`).

## 3. What the folds weakened

One real weakening, F-1 below. Otherwise:

- **Boundaries.** The comparisons at 150,000 and 200,000 are untouched and
  exact on `int64` (`internal/steward/context.go:310`, `:322`), and all seven
  points are still asserted. What changed is the branch they are asserted on.
- **Benign-unknown set.** Unchanged (`internal/steward/context.go:339-351`).
  The three no-call reasons are now asserted through the diagnostic branch and
  the two capability reasons through the live branch; the classification code
  is shared, so nothing is lost.
- **No dead verdict on missing evidence.** Intact and strengthened. `roleDead`
  is still reachable only inside `reading.Latest != nil`
  (`internal/steward/context.go:295`, `:310`), and fold 1 closes the one route
  the first read found from *stale* evidence.
- **Role ordering.** Unchanged and still asserted against
  `RoleStopHookDuration` at `internal/steward/context_test.go:229-236`.
- **NewestSince.** `internal/output/output.go:45-71` is byte-identical to what
  the first read cleared; the live proof at
  `internal/steward/context_test.go:401` still runs. It is now skipped for
  diagnostics by design. The first read's F-8 (the symlink case never gets an
  explicit `Chtimes`) is still open and still non-material.

## 4. Can the Stop path be slowed or wedged?

No, and fold 1 removes the only unbounded wait the first read named. Health
never sets `Transcript` (`checkContextBudget` passes `ContextOptions{}`,
`internal/steward/context.go:149`), so fold 2 adds nothing to the Stop path.
Fold 1 replaces two blocking `LOCK_EX` acquisitions with non-blocking ones and
adds one process probe per announcement that matches the holder id — a single
sysctl or procfs read. The one new behaviour worth knowing is that contention
now produces an unknown rather than a wait, and two consecutive unknown
observations raise `ShouldAlert` (`internal/steward/health.go:619-624`);
`PreviewHealthAt` persists nothing (`internal/steward/health.go:281-303`), so
only the steward tick can advance that counter. Recorded as G-4.

## 5. Scope

Clean. `git status` is exactly the seven modified files and the four untracked
part B files. No `internal/steward/contextreport.go`, no `usage.CallSessions`,
no fixture script, no `testing.json`, no coverage floor, no role or contract
text, no hook change. `memory/receipts.log` is tracked, unmodified, and
contains zero `ccb`/`slice2b` rows. The one CLI surface added is the `context`
family with a single `status` verb (`cmd/metasystem/main.go:422-429`), which
part B owns.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| F-1 | medium | yes | Fold 2 moved the boundary table test onto the diagnostic branch, and nothing now proves the live over-bound sentence or the live over-ceiling remedy. Every row of `TestRoleContextRendersBoundCeilingAndUnknowns` passes `Transcript:`, so all seven boundary points are evaluated with `diagnostic == true`, and the dead row's assertion was rewritten to *require* `context status` in the remedy. The string `context handoff` now appears nowhere in any test in the tree. The first read explicitly cleared "150,001 alive plus the handoff sentence" and "200,000 alive plus the handoff sentence" on this test; that clearance no longer holds. Proven by mutation: replacing the live dead remedy with `statusRemedy` leaves `go test -count=1 ./internal/steward` green (`ok ... 119.944s`) and `go test -count=1 -run Context ./cmd/metasystem` green; separately, changing `handoff` to `status` inside the live over-bound sentence leaves `-run 'Context\|Health' ./internal/steward` green (`ok ... 7.421s`) and the cmd Context tests green. A regression that told an over-ceiling coordinator to re-run `context status` instead of `context handoff` ships silently. | `internal/steward/context_test.go:37-39` (every row supplies `Transcript`), `:44-46` (dead row requires `context status` in the remedy and forbids `transcript`); the only two occurrences of the live wording, `internal/steward/context.go:311` (`remedy := "metasystem context handoff --root " + installationRoot`) and `internal/steward/context.go:326` (`reason += "; over the bound: run metasystem context handoff --root " + installationRoot`); partial live coverage that stops before the command at `cmd/metasystem/context_verbs_test.go:23`; `grep -rn "context handoff" cmd internal` returns only those two source lines |
| G-2 | low | no | CCB-2-15's "a reported breach exits 0" row is proven only on the diagnostic branch: the ceiling case of the new exit-code test supplies `--transcript`. The exit mapping keys on `readErr` alone and is shared by both branches, so the substance holds, but no test drives a live over-ceiling read through the command. | `cmd/metasystem/context_verbs_test.go:111-117` (`--transcript transcript` for the breach case) versus the live readable-unknown case at `:104-109`; mapping at `cmd/metasystem/context_verbs.go:78-82` |
| G-3 | low | no | A holder announcement that will not decode is skipped silently, and with no other matching record the role answers alive with "no announced holder: no announcement matches <id>" — a statement that is false when a corrupt file for that holder is sitting there. `checkSessionMain`, the reader the design names as the pattern, raises unknown for an unreadable announcement instead. The posture is safe (alive, never dead, on ambiguous evidence) and matches the lease classifier's lax mode, so this is recorded, not actioned. | `internal/steward/context.go:190-192` (`continue` on any decode error, before the holder-id test at `:197`) versus `internal/steward/health.go:956-959` (`unknown = true`) and the lax/strict split at `internal/lease/classify.go:134-140` |
| G-4 | low | no | Two consecutive contended observations now raise a context-budget alert where the tick previously just waited, and a dead-but-unreaped lease holder produces `unknown` on every observation, so it alerts from the second one. Both are improvements on the pre-fold behaviour (an unbounded wait, and a false `dead` with `NO_LAWFUL_REMEDY`), and unknown-on-contention is the pattern the first read asked for, so this is recorded only. | `internal/steward/context.go:98` (`NonBlocking: true` unconditional), `:88-90` (stale holder to unknown); escalation at `internal/steward/health.go:619-624`; preview persists nothing, `internal/steward/health.go:281-303` |
| G-5 | low | no | `NonBlocking: true` is set for the operator command as well as for health, so `metasystem context status` fails with exit 1 whenever the Stop hook or the steward tick happens to hold the session cursor or registry lock, rather than waiting a few milliseconds. Nothing in the brief asks the command to be non-blocking. | `internal/steward/context.go:98` reaching the command through `ContextBudgetLine` at `cmd/metasystem/context_verbs.go:70`; error to exit 1 at `cmd/metasystem/context_verbs.go:78-81` |
| G-6 | low | no | The stale-holder guard is gated on `capability == usage.PerCall`, so for a `per-invocation` or `none` runtime a dead holder still reaches `usage.LatestCall`. It is harmless today — those capabilities return their benign reason before touching the state root — and it is covered by tests, but the guard reads as capability-specific when the liveness fact is not. | `internal/steward/context.go:88` versus the early returns at `internal/usage/calls.go:98-104`; covered at `internal/steward/context_test.go:122-136` |
| G-7 | low | no | When cleanup of the private root also fails, `errors.Join` produces a newline-separated message that becomes the role `Reason` and is then embedded in the single-line `HEALTH ...` rendering. Only reachable when `os.RemoveAll` on a fresh temp directory fails. | `internal/steward/context.go:145` (join), `:103` (`readErr.Error()` becomes the reason), `internal/steward/health.go:205-221` (`RoleVerdict.Line()` assumes one line); shape exercised at `internal/steward/context_test.go:722-726` |
| G-8 | low | no | `TestContextStatusExitCodesForReadableUnknownAndCeilingBreach` does not set `HOME`, so the readable-unknown case resolves its candidate paths under the developer's real home directory. It is deterministic because the root is a fresh temp directory, but every other context command test sets `HOME` explicitly, and an `os.UserHomeDir()` failure would change the reason to "home directory unreadable" and break the assertion. | `cmd/metasystem/context_verbs_test.go:101-109` (no `t.Setenv("HOME", ...)`) versus `writeDerivedContextCommandTranscript` at `:392-395`; candidate construction at `internal/usage/calls_claude.go:18-38` |
| G-9 | low | no | `usage.RegisterSession` (blocking) now has no production caller; only tests and the shared private helper use it. Presumably part C or D will, but as shipped it is an exported blocking API with no consumer. | `internal/usage/cursor.go:283`; the only production registration is `RegisterSessionNonBlocking` at `internal/steward/context.go:108` |

## Verdict

**Not fit to land as it stands. One material finding.** Everything the two
folds set out to do is done and done correctly: F-2's liveness probe matches
`checkSessionMain` field for field and cannot turn a live seat unknown or let
a dead holder reach a dead verdict; F-3's scan is narrowed to the holder's own
record; F-4's two lock acquisitions are the only two on this path and both are
now bounded; and fold 2's private root is genuinely isolated in allocation,
permissions, uniqueness and cleanup, with no remaining route from an override
to the live cursor, samples log or lock. Nothing the first read cleared about
the dead-verdict rule, the benign-unknown set, role ordering or `NewestSince`
was weakened, and neither fold can slow or wedge the Stop path.

To land, F-1 needs one assertion restored: a live (no `--transcript`) read at
200,001 that requires `Remedy == "metasystem context handoff --root " +
installationRoot`, and a live read above the bound that requires the full
`; over the bound: run metasystem context handoff --root ...` sentence. Adding
a `diagnostic bool` column to the existing table and running each row twice
would cover both branches without new fixtures. G-2 through G-9 are recorded
only and must not block.

Material findings: 1 (F-1).
