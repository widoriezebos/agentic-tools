# receipt-admission-caps-concurrent-batteries

- Owner: m1e (goal 1 of plans/delivery-efficiency-plan.md), claimed 2026-09-12 17:50Z.
- Goal and current status: concurrent batteries on one host stop failing each other within R-35-m3 (no machine reservations between seats). Slice 1, the attribution, landed 0c621e4b (2026-09-12) after one critique round and is verified through the engine (attempt proof-mtykoip9 carries the load block): every attempt record carries the host's load and the overlapping attempts at its start and its end, a failed terminal under a crowded host is attributed to the load in the record, a retry decision names the load it retries against, and a consumption-bounded group that failed under load is filed as its own patience defect. The cap (slice 2) is designed here and waits on Wido's word.
- In flight right now: nothing. Slice 2, the temporary cap, is built and landed (units U1 and U2 of 2026-09-16, section 4); the follow-up of section 6 (retiring the machine's shell lock) is not started.
- Decisions made (and who made them): Wido, 2026-09-11 (R-92-m1e): no admission cap for now; build the attribution and put the cap question again with a week of post-c08fe2cb numbers. Wido, 2026-09-12 (R-107-m1e): a receipt cap only if it is configurable and works robustly on any machine. Wido, 2026-09-15 (R-111-m1e item 2): a TEMPORARY admission cap on concurrent heavy runs SUPERSEDES R-35-m3; until it is built, seats run one engine test run at a time on a machine by agreement (the mkdir lock /tmp/metasystem-testrun-lock); the cap expires once tests-never-wait-on-wall-time and engine-policy-binding-survives-drift-and-load land, and is then re-evaluated. Seat ruling, 2026-09-16: that expiry is satisfied by named units, not by the whole pages (section 4).
- Waiting on the human: nothing. Wido decided the cap question at the terminal on 2026-09-15 (R-111-m1e item 2, below); what remains for him is the cap's re-evaluation when it expires (section 4's expiry).
- Dead ends (do not retry without new evidence): none yet.
- Next step: STATE 2026-09-16 (m1e): slice 2 is built and landed in two units; the shell lock stays until the follow-up of section 6 is done on every seat's engine.

## 1. What the audit found

The five-day audit (artifacts/reports/delivery-deep-dive-2026-09-11/proof-attempts.md, section 4) bucketed 97 attempts by the other attempts whose interval intersected theirs. By distinct overlapping attempts over the record interval: 0 overlaps failed 44 percent (7 of 16 decided), 1 overlap 17 percent, 2 overlaps 30 percent, 3 or more 51 percent (19 of 37); the full runs' median wall grew from 24.5 min alone to 56.5 min at 3 or more. Time-weighted, 2 overlaps already failed 53 percent. The largest single load failure, go test groups bounded by the clock, was removed by c08fe2cb (groups bounded by consumption). What remained unknown was the attribution: no record said what the host looked like when an attempt failed, so a red under load and a red from a defect read alike, and every retry decision could say "flaky".

## 2. Slice 1: the attribution (built)

- `internal/hostload` reads the load averages and the core count (darwin `vm.loadavg` sysctl decoded from the kernel's fixed-point struct, linux `/proc/loadavg`, other platforms report unavailable in the record rather than failing the attempt). `Sample.Saturated()` is the one-minute load at or above the cores.
- `proofrun.LoadSample` is the host sample plus `overlappingLocal` (this checkout's other live attempts whose launcher is alive) and `overlappingHost` (the top-level proof launchers alive anywhere on the host: a metasystem engine running `proof-run launch` as its first verb, from any seat's binary; a launcher with a launcher above it, a battery's nested bed launchers and joined launches, counts as its battery's, and the attempt's own launcher family, ancestors and descendants, never counts; `overlapKnown` is false when the process table cannot be read). `Loaded()` is a saturated host (the one-minute load at or above the cores), or another top-level launcher with the load already at half the cores. One other launcher on an otherwise idle box is overlap, recorded as such, not load: the label must discriminate or the audit's gap is inverted rather than closed. The sample includes the attempt's own load; a lone battery's tail can saturate its own host, and `overlappingHost` at zero is what separates that from seat interference.
- `Attempt.Load` carries the start sample (taken in `ReserveLocked`) and the end sample (taken in the one terminal commit, `FinalizeAttemptWithTestResultLocked`). `AttemptTerminal.Attribution` is `load` when a failed terminal was committed under a loaded end sample; a cancellation or an unknown terminal carries the sample without the label. An attempt reserved before this landed finalizes with a start sample that says "not sampled at start". Schema 2 is unchanged and the block is optional; an engine built before this change refuses a record that carries it (the attempt reader rejects unknown fields), so every root a new engine writes to must run the new engine: this seat re-arms after landing, and the shared landing checkout rebuilds its engine at each landing.
- `RetryEvidence.PriorLoad` and `PriorAttribution` copy the prior attempt's samples and attribution into the decision that retries it, so the decision names the load ("load: 2 other proof launcher(s) on the host, 0 other live attempt(s) in this checkout, load 25.20/21.80/17.80 on 18 cores") or says "no load attribution".
- A failed group whose progress rule is a consumption bound (a CPU budget or a zero-consumption window, read from the rule; every supervised go test group since c08fe2cb) on a failed attempt whose end sample is loaded is appended to `artifacts/agents/proof-runs/patience-defects.jsonl` under the control root, one line per group with the sample, after the terminal is committed and never fatal to it: the group's patience is the defect to fix, never a reason to serialize seats.
- Readers of the whole attempt set (`ReconcileAttempts`, `LiveAttempts`, the overlap count) skip a record they cannot read and name it, instead of failing on it: on 2026-09-12 one record whose control root no longer matched its path hid every attempt from the status verb.
- The samplers are seams (`loadSeams`); the tests script the host and the launcher count and run in no wall time (R-104-m1e).

## 3. Proof

- Unit: `internal/hostload` (a recorded sysctl buffer from the eighteen-core box, `/proc/loadavg` text, the saturation rule); `internal/proofrun/attemptload_test.go` (reserve and finalize record both samples; a failure under load is attributed and a success or a quiet failure is not; a retry decision carries the prior's load; the patience filing names exactly the failed consumption-bounded groups; an unreadable record is skipped and named). The whole proofrun and hostload packages and the attempt-reading command tests pass under the race detector; the fast gate is green.
- Through the engine: on the landed tip, attempt proof-mtykoip9 (2026-09-12 16:01Z) carries `load.start` (8.04 on 18 cores, no other launchers, overlap known) and `load.end` (9.48); `supervise status` lists live attempts beside any record it had to skip.

## 4. Slice 2: the temporary cap (built 2026-09-16, units U1 and U2)

Wido decided the cap at the terminal on 2026-09-15 (R-111-m1e item 2), over R-35-m3 and with an expiry: it is
TEMPORARY, and it replaces the shell lock that serializes one engine test run per machine.

- Key `proof.admission.top-level-max`, resolved at admission through `config.Get` (environment, then the seat's
  `.local`, then the committed file). Absent means the derived default `max(1, cores/6)` (18 cores: 3; 8 cores:
  1); `0` disables the cap; anything that is not an integer from 0 through 64 is an error naming the key, at the
  admission that read it, and `config validate` reports it too. The committed metasystem.conf documents the key
  as a comment and does not set it: the default is per host and a seat pins its own value in `.local`.
- What it admits, and why that is safe: the cap is the TOTAL of top-level proof attempts on the host, so a
  reserve is refused when it already observes `max` others (`observed >= max`). On the eighteen-core box that is
  3 attempts side by side, against 1 under the shell lock, so the cap can never be slower than what it replaces;
  and every admitted attempt sees at most 2 overlapping attempts, the audit's <=2-overlap regime (section 1: 17
  to 30 percent), never its 3-or-more regime (51 percent). Measured on the night of 2026-09-15 to 09-16, three
  seats running with the lock held load averages of 5 to 15 on 18 cores, so three concurrent batteries stay
  inside the saturation line the attribution labels; a fourth is what the cap refuses.
- Scope (seat correction to the design's "top-level delivery attempts only"): every top-level reserve, whatever
  its purpose and whether or not it has a reservation owner. The cap refuses exactly the class it counts, so
  counted and capped can never disagree; the seats' own runs are `--purpose diagnostic`, and capping only
  delivery would leave the fleet's real contention uncapped while those runs still filled everyone else's count.
- Nested receipts are exempt twice over: a joined or component admission never reaches the cap, and a reserve
  whose own process has a live top-level proof launcher above it is admitted by the predicate. A nested receipt
  whose parent already holds the slot would otherwise deadlock its battery.
- An unknown host admits. When the process table cannot be read (`overlapKnown` false) the cap never refuses: a
  transient blind reading must not stall every seat, and the record already says the reading was blind.
- The count is the census of section 2, widened in U2: a metasystem engine running `proof-run launch` OR
  `test run` as the verb pair right after the binary. An engine test run reserves a top-level attempt and runs
  the groups, so it is a battery in every sense the cap and the attribution care about, and it is exactly what
  the shell lock serializes. One consequence, intended: `Loaded()` now fires on records where another seat's
  test run is the contention, which is the attribution naming the fleet's real load for the first time.
- No shared state between seats: the slot is the process census under the reserving checkout's proof mutation
  lock, so a dead seat can never hold a slot (R-35-m3's reason), and a slot is released by the launcher's
  termination, which is what the reading sees.
- Refusal, one producer (`AdmissionCap.RefusalReason`), disposition `admission-refused`, exit 78, no attempt
  record written, and the caller retries rather than queueing:
  `ADMISSION_REFUSED rank=host-load key=proof.admission.top-level-max admitted=3 observed=3 source=cores retry=retry-when-a-launcher-ends temporary=yes ruling=R-111-m1e expires-when=tests-never-wait-on-wall-time:1e,2,3b+engine-policy-binding-survives-drift-and-load:U4a,U4b land`
- THE EXPIRY, in the code and in this record. `AdmissionCapExpiry` in internal/proofrun/admission.go names the
  landings that retire the cap, and the refusal carries them, so nobody has to read prose to know when it goes:
  units 1e (gate on, initial manifest), 2 (protection) and 3b (the judge) of goal tests-never-wait-on-wall-time,
  and units U4a (progress deadline, steward side) and U4b (progress deadline and typed facts, command side) of
  goal engine-policy-binding-survives-drift-and-load. Those five remove the wall-time waits and completion
  deadlines that made concurrent batteries unsafe (the engine-policy page names U4a "the load fix on the steward
  side" and U4b's after-state "row 8's cap condition half met"); the rest of both pages lands at its own pace and
  does not hold the cap open. R-111-m1e's re-evaluation is Wido's, with the records those landings produce.
- Proof. Unit witnesses in internal/proofrun (the cap's resolution from configuration and cores, the refusal at
  the cap with nothing written, the refusal predicate clause by clause with a non-zero count, the nested and
  unknown exemptions through a real reserve, one census read per reserve, the expiry and ruling in the text) and
  in internal/config (validate accepts 0 and refuses a malformed, negative or too-large value); the census
  witness pins a top-level engine test run as one launcher and a nested one as none. The seat ran every mutation
  named in the two briefs: each fails exactly its witness (the builder's sandbox could reach no Go build cache,
  so the seat ran them). Through the engine: the deep diagnostic battery on the candidate tree, every selected
  group green.

## 6. Follow-up, named and not done here

Retiring the shell lock (/tmp/metasystem-testrun-lock and hact-20260912/testrun-lock.sh) is NOT part of slice 2.
It needs the cap on every seat's engine (each seat re-arms after landing) and one crowded day whose records show
no load-attributed failure the cap did not bound. Until then the lock stays as the seats' agreement and the cap
is the engine's floor under it. Whoever retires it states in the goal's next step which records carried the day.

## 5. Critique record

Round 1 (2026-09-12, independent code critic): six material findings, all folded. F-1: the label fired on any non-success terminal (a cancellation, an unknown), now only on a failed one. F-2: the launcher count included the attempt's own family (a battery's nested bed launchers) and any argv that mentioned the verb (a perl sleep was counted); now the first verb of a metasystem engine, top-level launchers only, self's family excluded, with the attempt's launcher pid as self on every path. F-3: one launcher anywhere meant "loaded" whatever the load; now a launcher counts as load only with the box at half its cores, saturation always. F-4: the patience filing ran before the commit and could sink the terminal; now after it and never fatal. F-5: an old record finalized with a fabricated start sample; now "not sampled at start". F-6: engines built before the change refuse the new records; recorded above as the re-arm scope. Non-material notes folded: the consumption rule is parsed (the Linux suffix defeated the literal), a retry against a prior without a load block says so.
