# Recovery to a good state: analysis

Goal: `plans/goals/recovery-to-good-state.md`. Brief: `plans/recovery-analysis-brief.md`.
Round: analysis, not design. Every claim about the tree cites file and line
as read on branch `agent/recovery-analysis-r1b` (HEAD 23cb4169) on 2026-09-02.
Records were read from the m1 checkout's artifact tree, which the worktree does
not carry; those citations name the artifact path under the metasystem root.

Short verdict: the seat's root cause is confirmed in substance and corrected in
four places (section 6). The one-line version: `metasystem up` already repairs
most partial states by itself, but its refusals name prose or a terminal
instead of a command, two identity comparators disagree so up can create the
partial state it then refuses, the only stop verb is labelled an internal
fixture option, and nothing owns machinery that outlives its custodian.

## 1. The state machine as it is

### 1.1 What `metasystem up` does, in order

`up.Run` (`internal/up/up.go:568-574`) branches to `ordinary` or `recovery`.
The ordinary path (`up.go:412-500`) reports components in this order and stops
at the first failure, returning `failure(...)` with one remedy string:

| Component | Where | What it does | Refusal and remedy text |
| --- | --- | --- | --- |
| host-preflight | `up.go:287-298`, `414-417` | LookPath on 33 commands | "install the named commands and rerun metasystem up" (`415`) |
| accepted-engine | `up.go:399-410`, `418-421`, `433-435` | opens the enrolled pin (`internal/steward/identity.go:242-287`) and refuses when the invoking binary's canonical path or digest differs | outcome `ENROLLMENT_DRIFT`, remedy "from the enrolled agent-free terminal, explicitly run metasystem steward arm or steward restart for this repository" (`up.go:392`) |
| session-identity | `up.go:163-242`, `428-432` | proves the caller descends from a runtime-signature ancestor, or from an explicit `--pid/--start-time` pair | "pass --pid <session-pid> and --start-time <epoch-seconds>, or configure a runtime signature and invoke up from that session" (`430-431`) |
| session-announcement | `up.go:440-448` | writes the main announcement | "repair the named announcement or lease state, then rerun metasystem up" (`443`) |
| checkout-lease | `up.go:449-474` | classifies holder or advisor | holder: none. Advisor remedy "run scripts/agents/second-session.sh to create an isolated writer worktree" (`468`); the script exists (`scripts/agents/second-session.sh`). Lease read errors: "repair the checkout lease and rerun metasystem up" (`451`, `459`) |
| supervision-owner, repo-watcher, job-reaper | `up.go:348-389` calling `supervise.EnsureArmed` (`internal/supervise/arming.go:680-760`) | see 1.2 | three remedies: "fix the named supervision configuration or fingerprint input, then rerun metasystem up" (`351`); "inspect artifacts/agents/supervision/owner.log, repair the named blocker, then rerun metasystem up" (`358`); "prove the recorded component identity and process group are gone, then rerun metasystem up" (`362`); "inspect artifacts/agents/supervision/owner.log and rerun metasystem up after the component can complete one pass" (`376`) |
| steward-runner | `up.go:480-492` calling `steward.EnsureRunner` (`internal/steward/runner.go:215-260`) | verifies a live runner completes a generation-bound tick, else restarts it from the enrolled pin without minting | "configure a working notification channel, inspect artifacts/agents/steward/runner.log, then rerun metasystem up" (`482-483`) |

All component lines are printed only after `Run` returns
(`cmd/metasystem/up.go:63-68`, `151`). Nothing is printed while up waits.

The recovery-only path (`up.go:502-547`) needs `--if-down` (`504-506`), opens
the same enrollment (`508-515`), ensures supervision with `OnlyIfDown`
(`343`), and repairs the runner through `RepairEnrolledRunner`
(`runner.go:273-289`); its remedies are "invoke ordinary metasystem up from a
session, or add --if-down" (`506`), "run ordinary metasystem up from a session
to repair steward enrollment" (`528`) and "... to establish the steward
generation" (`534`). It writes no announcement and no lease.

Two other modes exist and are labelled internal: `--retire` (`up.go:577-588`,
`cmd/metasystem/up.go:84` "internal compatibility option") and `--shutdown`
(`up.go:590-602` "compatibility operation, not part of the daily operator
surface"; `cmd/metasystem/up.go:85` "internal fixture compatibility option").
Shutdown requires the caller to be the checkout holder
(`up.go:594`), then `supervise.ShutdownAt` (`arming.go:773-791`): refuse if the
lock's owner tag belongs to another repository (`781-783`), `stopOwner`
(`269-300`: write shutdown intent, SIGTERM the owner's group, wait 5 scaled
seconds), `stopTakeoverComponents` (`517-541`: SIGTERM each recorded component
group), release the lock (`543`). It does not touch the steward runner and does
not retire the announcement. The steward runner's stop is a separate verb,
`steward disarm` (`cmd/metasystem/main.go:437`; `runner.go:523-549`: write a
stop file, wait 5 seconds, then SIGTERM).

`scripts/agents/arm-supervision.sh:16` execs `metasystem up`, so every
`arm-supervision.sh --shutdown` in the fixtures is `up --shutdown`.

### 1.2 How the owner branch decides (the seam every partial state hits)

`EnsureArmed` (`arming.go:680-760`), once the lock directory exists:

1. Read the recorded owner (`arming.go:701-711`); an unreadable record refuses
   with "supervision lock has no provable owner".
2. Probe liveness through `ownerLiveness` (`arming.go:152-158`), which builds a
   full reference (pid, start second, start ticks, boot id) and calls
   `identity.AliveTaggedRef` (`internal/identity/identity.go:223-241`). That
   function returns `Dead` when the probed identity differs from the record
   (`231-233`) or the instance tag is absent from argv (`237-239`), `Unknown`
   only when the kernel probe is unknown or argv is unreadable (`228-229`,
   `234-236`).
3. `Unknown` refuses: "supervision owner pid %d is uninspectable; takeover is
   not authorized" (`714`).
4. `Dead` takes over: check the cap ceiling (`716-718`), SIGTERM every recorded
   component group (`719-721`), release the lock (`722-725`), loop and launch a
   fresh owner (`684-696`).
5. Alive and generation matches: `verified` (`744-747`). Alive and generation
   differs: `stopOwner` through the shutdown intent (`751-753`), stop
   components (`754-756`), release, `replaced` (`757-760`).
6. In every branch that ends with an owner, `waitUntilArmed` (`233-245`) polls
   `InspectArmedAt` (`internal/supervise/verifyarmed.go:66-115`) for
   `watch.interval-sec + 10` scaled seconds (`234`).

`InspectArmedAt` compares the owner with `armedIdentityAlive(ownerPid,
ownerStart, ownerTag, probe)` (`verifyarmed.go:79`, defined at `32-41`), which
calls `census.Alive(pid, start, probe)` on the start SECOND only (`33`); it
carries no start ticks and no boot id. `census.Alive` is the seconds-only
comparison (`internal/census/verbs.go:17-19`); the clock-step-immune
`census.AlivePair` exists beside it (`verbs.go:21-28`) and is not the one
called here. The reason it returns is "the recorded
owner identity is not live" (`80`), and the same seconds-only check is applied
to the watcher and reaper (`103-104`). This is the comparator the vm-epoch
specimen tripped (section 2, specimen 4).

### 1.3 The health roles and their remedies

`metasystem health` (`cmd/metasystem/main.go:429`) renders the roles in
`internal/steward/health.go`. Remedies per role:

| Role | Dead when | Remedy text | Escalation |
| --- | --- | --- | --- |
| steward-runner | no record, recorded pid dead, or no generation-bound success (`health.go:448-470`) | `metasystem up --repo <root>` (`448`) | always auto-heal eligible (`426-427`) |
| supervision-owner | no lock, lock owner pid dead (`473-491`) | `supervisionRemedy` = `metasystem up --repo <root>` (`1070-1072`) | per `hasLawfulAutomaticRemedy` (`416-...`) |
| repo-watcher | no state, no recorded process, pid dead, stale success (`496-527`) | same | lawful only for "recorded pid", "lastSuccess is stale", "latest attempt passed its deadline" reasons (`428-431`) |
| census-freshness | no success, last census failed, generation mismatch, stale (`530-567`) | same | see `432-...` |
| session-main | no announced main, every announced main dead (`617-665`) | same (`617`) | not in the lawful list as read |
| hook-freshness | no hook turn generation recorded, pending or incomplete turns (`286-318`) | `metasystem health --repo <root>` (`286`) | NO_LAWFUL_REMEDY (`387-389`) |
| narrator-freshness | (`572-578`) | `metasystem up --repo <root>` (`572`) | |
| counselor | brief render failed (`counselor_carriage.go:134`) | `metasystem steward tick --repo <root>` | |
| retro-debt | (`265-276`) | `scripts/receipt.sh add --type retro ...` | |
| ledger-attention | (`ledgerattention.go:644`) | "run a journaling goal verb that examines the canonical tip" | names a class of verbs, not one |

A dead role with no lawful automatic remedy is escalated as
`NO_LAWFUL_REMEDY` on the first failure (`health.go:386-389`); the alert text
renders it as "[failure n/5; no lawful remedy]" (m1 alerts
`artifacts/agents/steward/alerts/alert-debf22c2581c3301-1.json` and siblings).
For hook-freshness the remedy named is `metasystem health` itself, which reads
and never repairs, so the remedy is circular by construction.

### 1.4 The partial states, one row each

"Seat-runnable" below means: the command exists, and the caller class the seat
has (MAIN, an announced main, or a process descending from it) is accepted.
`steward arm` without a word and `steward restart` require class HUMAN, which
`internal/lease/classify.go:296-391` grants only to a caller with no
recognised agent ancestor and a controlling terminal (`369`, `377`);
`cmd/metasystem/steward_verbs.go:561-578` refuses every other class with
"explicit engine enrollment requires an agent-free terminal; caller classified
%s" (`573`). `steward arm --temporary-human-word <word> --review-by <date>`
skips that gate (`steward_verbs.go:498-513`) and mints through
`steward.ArmTemporary` (`runner.go:155-164`). `steward restart` has no
temporary-word path (`steward_verbs.go:520-545`).

| State | What the seat can run | What it does | What it refuses | Remedy seat-runnable? |
| --- | --- | --- | --- | --- |
| Engine drifted from the enrolled pin (rebuild, pull) | `up` | refuses before announcing anything (`up.go:418-421`) | `ENROLLMENT_DRIFT` | No: remedy names the terminal only (`up.go:392`). The seat-runnable path, `steward arm --temporary-human-word`, is named nowhere in the refusal. `steward restart` is terminal-only. |
| Owner alive, identity drifted by one second, ticks and boot id equal | `up` | `ownerLiveness` (full reference) says Alive, generation matches, `verified`; then `InspectArmedAt` (seconds only) says "the recorded owner identity is not live" (`verifyarmed.go:80`) | failed, remedy `up.go:376` "inspect owner.log and rerun ... after the component can complete one pass" | No: names inspection, and rerunning repeats the same comparison. |
| Owner alive, identity drifted so that the FULL reference also mismatches | `up` | `AliveTaggedRef` returns Dead (`identity.go:231-233`); the Dead branch SIGTERMs live components and releases the lock under a live owner (`arming.go:716-725`) | may then fail the inspection as above | The repair itself creates a second owner beside a live one. |
| Owner dead, stale lock | `up` | Dead branch: takeover (`arming.go:716-725`), action `taken-over` | refuses only if the ceiling is blocked (`196-205`) or a component identity is uninspectable (`373`) | Yes, up itself. (The 2026-08-28 sighting in `plans/goals/arming-dead-owner-takeover.md` predates `up`'s Go port; today's code takes over.) |
| Owner alive, generation differs (after a lawful re-arm) | `up` | shutdown intent, SIGTERM, replace (`arming.go:749-760`) | "owner identity is uninspectable; replacement is not authorized" (`274`, `286`) when the probe is unknown | Yes for the happy case; the refusal names nothing. |
| Runner dead | `up` | `EnsureRunner` restarts from the pin (`runner.go:236-259`) | requires a notify command (`runner.go:221-223`) and the pin (`224-226`); after a rebuild the pin check fails first as ENROLLMENT_DRIFT | Health names `metasystem up` (`health.go:448`): yes, unless the engine drifted. |
| Session main dead (announced main gone) | `up` from the new session | new announcement (`up.go:440`); the old announcement is not retired by up | none | Yes. |
| Census failed | `up` | replaces nothing when the owner is alive and current; waits for a fresh inspection | dispatch refuses "last census verdict is CENSUS-FAILED" (`internal/dispatch/attest.go:51-52`), naming nothing | The health remedy is `up`, which does not fix a census that fails on scope resolution. |
| Hook dead (`hook-freshness`) | `health` (named remedy) | prints the verdict | NO_LAWFUL_REMEDY | No: the remedy is a read. The cause on nested checkouts is `scripts/agents/supervision-hook.sh:65` resolving the git toplevel (`plans/goals/supervision-hook-wrong-root.md`). |
| Leaked non-custody machinery | `health`, `census` | census classifies UNTRACKED (`internal/census/run.go:307-332`) and never signals anything (`tagged.go:153` is a zero-signal probe); the steward counts UNTRACKED as blocking a death proof (`internal/steward/census.go:56-77`) | dispatch refuses when the census is not SUCCESS | No verb reaps: the janitor family has only `headroom` (`cmd/metasystem/main.go:339-345`). |
| Whole checkout must stop | `up --shutdown` plus `steward disarm` plus `up --retire` | three verbs, three custodians | shutdown refuses for a non-holder (`up.go:594-595`) and for an unknown owner probe (`arming.go:274`) | Yes, but undocumented as an operator surface and never proven by a census. |

## 2. The specimen map

The eight specimens live only in the goal's Intent and in
`plans/handoff-m1-2026-09-02.md`; the m3 and m2 handoffs
(`plans/handoff-m3-standby-2026-09-01.md`, `plans/handoff-m2-standby-2026-09-01.md`)
carry the hook-freshness and load-generator leaks. Where a specimen has no
independent record in the tree, the row says so.

| # | State | Command tried | Refusal or hang, file and line | Hand surgery that worked | Record |
| --- | --- | --- | --- | --- | --- |
| 1 | engine drifted after a pull rebuilt it | `up` | `ENROLLMENT_DRIFT` from `openInvokingEnrollment` (`up.go:404-407`, `identity.go:283-286` digest mismatch); remedy `up.go:392` names the terminal | `steward arm --temporary-human-word ... --review-by 2026-09-06` under R-37-m3, refused twice by the harness classifier, passed on the third identical try | `artifacts/agents/steward/identity.json` (generation 5, mintedAt 2026-09-02T11:21:41Z, the temporary word recorded); handoff m1 lines 37-41 |
| 2 | up replacing supervision after the re-arm | `up` | no line is printed until `Run` returns (`cmd/metasystem/up.go:63-68`); the waits are bounded per step (`arming.go:234` interval+10 s; `291` 5 s; `411` 5 s; `runner.go:192` scaled runner wait) but the owner branch loops up to four times (`684`) and each replaced owner re-waits | none; it finished | `arming.log`: announcement written 11:35:48Z, first-census-complete 11:35:49Z; owner pidStartedAt 1788348949 = 11:35:49Z (`supervision/state.json`); steward runner started 11:21:42Z (`steward/runner.json`). Owner.log tail shows the 5-second first-heartbeat ceiling tripping repeatedly ("supervision first-heartbeat ceiling reached: reaper (elapsed: 5s; scaled cap: 5s)") and six "owner exit: reason=terminated" lines, the load-fragile cap class R-35-m3 names. The 13 minutes fall between the arm (11:21:42Z) and the announcement (11:35:48Z), which is BEFORE the supervision step in `ordinary` (`up.go:428-448`); the records cannot say which of preflight, enrollment open, ancestry proof (`census.FindAncestorProduction`, `up.go:178`) or announcement took the time. Gap G2. |
| 3 | 488 orphaned fixture processes, 8,789 stale beds | nothing lawful | no reaping verb (section 4); census reports UNTRACKED only | raw kills, refused by the classifier for bulk kills (handoff m1 line 38), then done by hand | handoff m1 lines 27-31; `plans/goals/proof-harness-process-custody.md` second specimen |
| 4 | m0b's owner pid 27632 alive ten hours with a drifted identity, census CENSUS-FAILED | `up`, `delegate` | `up` refuses at `verifyarmed.go:80` after `ownerLiveness` accepted the owner (section 1.2); `delegate` refuses at `attest.go:51-52` naming nothing; `up --shutdown` would have taken the Alive path (`arming.go:270-300`) but is labelled internal (`cmd/metasystem/up.go:85`) | a raw kill, refused by both harnesses; eventually hand surgery | No record besides the goal Intent: "27632" appears only in `plans/goals/recovery-to-good-state.md`. Gap G1. The mechanism is recorded on the sibling m0 sighting in `plans/goals/vm-epoch-identity-drift.md` (recorded second +1 over every probe; ticks and boot id identical). |
| 5 | m0's session on a Claude Code version that rejects the fleet model | none | outside the engine; the delegate adapter parses `claude --version` for its own launches (`scripts/agents/adapters/claude.sh:29`), the seat's harness version is never read | hand restart and re-brief | goal Intent only |
| 6 | remote-control messages stranding on three seats | none | outside the engine; nothing in `internal/steward` reads a seat channel (`plans/never-idle-analysis.md:71`, `470-477`) | m1 pushed messages by hand (handoff m1 line 35-36) | never-idle analysis, handoff m1 |
| 7 | fresh clone cannot join | `up` | no engine binary, no roster, no ledger refs; `up` stops at ENROLLMENT_DRIFT before announcing (handoff m1b lines 40-41); `internal/goal/project.go:39` "no accepted tree; the first fetch or the migration bootstraps it" names no command; `project.go:73` names `goal list --fetch`, and `runGoalList` accepts no `--fetch` flag (`cmd/metasystem/goalsync_verbs.go`, no match) while `goal fetch` is its own verb (`main.go:419`) | hand-fixing per machine | `plans/goals/fleet-join-bootstrap.md` |
| 8 | steward and hooks report dead components with no lawful remedy | `health` | `hook-freshness=dead ... remedy: metasystem health ... [failure 5/5; no lawful remedy]` (`health.go:286-290`, `387-389`; alerts) | none; standing | `artifacts/agents/steward/health.json` sequence 54 (hook-freshness dead, failureEscalation NO_LAWFUL_REMEDY, every other role alive); the 21 alert files in `steward/alerts` |

Note on specimen 1's "refused twice, passed on the third": the refusals were
the harness permission classifier, not the engine. The engine's own gate for
the temporary word is `humanauthority.ValidateTemporaryWordPair`
(`steward_verbs.go:494-497`), which accepts the same input every time.

## 3. The refusal inventory

Verdict key: A = names a command that exists and the seat can run; T = names a
terminal-only command; X = names a command or flag that does not exist; N =
names nothing executable (prose such as "inspect", "repair", "prove").

### 3.1 up

| Text | Where | Verdict |
| --- | --- | --- |
| install the named commands and rerun metasystem up | `up.go:415` | A |
| from the enrolled agent-free terminal, explicitly run metasystem steward arm or steward restart for this repository | `up.go:392` | T (the seat-runnable temporary-word form is not named) |
| pass --pid <session-pid> and --start-time <epoch-seconds>, or configure a runtime signature and invoke up from that session | `up.go:430-431` | A, but the pair is only accepted from a descendant of that pid (`up.go:211-213`), which the hook satisfies and a seat's shell may not |
| repair the named announcement or lease state, then rerun metasystem up | `up.go:443` | N |
| repair the checkout lease and rerun metasystem up | `up.go:451`, `459` | N |
| run scripts/agents/second-session.sh to create an isolated writer worktree | `up.go:468` | A |
| fix the named supervision configuration or fingerprint input, then rerun metasystem up | `up.go:351` | N |
| inspect artifacts/agents/supervision/owner.log, repair the named blocker, then rerun metasystem up | `up.go:358` | N |
| prove the recorded component identity and process group are gone, then rerun metasystem up | `up.go:362` | N (the proof is a raw kill the classifier refuses) |
| inspect artifacts/agents/supervision/owner.log and rerun metasystem up after the component can complete one pass | `up.go:376` | N |
| configure a working notification channel, inspect artifacts/agents/steward/runner.log, then rerun metasystem up | `up.go:482-483` | N |
| invoke ordinary metasystem up from a session, or add --if-down for the scheduler recovery path | `up.go:506` | A |
| run ordinary metasystem up from a session to repair steward enrollment / to establish the steward generation | `up.go:528`, `534` | A, but circular when the cause is drift |
| pass --pid <session-pid> and --start-time <epoch-seconds> | `up.go:582` | A |
| inspect the announcement registry and retry retirement | `up.go:585` | N |
| run shutdown from the checkout holder | `up.go:595` | N (names a role, not a command) |
| inspect the recorded owner identity before retrying shutdown | `up.go:599` | N |

### 3.2 supervise (reached through up)

| Text | Where | Verdict |
| --- | --- | --- |
| supervision owner pid %d is uninspectable; takeover is not authorized | `arming.go:714` | N |
| dead-owner takeover refused: %w | `arming.go:720` | N |
| generation replacement refused: %w | `arming.go:755` | N |
| owner identity is uninspectable; replacement is not authorized | `arming.go:274`, `286` | N |
| recorded %s identity is uninspectable; takeover is not authorized | `arming.go:373` | N |
| derived %dm watcher ceiling does not strictly clear reserved cap %dm for job %s | `arming.go:202` | N |
| supervision lock has no provable owner | `arming.go:711` | N |
| live owner did not publish a verifiable generation | `arming.go:732`, `742` | N |
| supervision lock names an owner armed for another repository | `arming.go:782` | N |
| the recorded owner identity is not live / the recorded component identity is not live / the component heartbeat is stale | `verifyarmed.go:80`, `104`, `113` | N |
| census writer liveness is unprovable; refusing takeover | `censuslock.go:107` | N |

### 3.3 steward arm, restart, health

| Text | Where | Verdict |
| --- | --- | --- |
| explicit engine enrollment requires an agent-free terminal; caller classified %s | `steward_verbs.go:573` | T |
| human ancestry proof failed / cannot resolve the installed engine | `steward_verbs.go:565`, `569` | N |
| TEMPORARY enrollment under a recorded remote human word; re-approval due %s at an agent-free terminal | `steward_verbs.go:513` | informational, T for the follow-up |
| metasystem up --repo <root> | `health.go:448`, `572`, `1071` | A |
| metasystem health --repo <root> | `health.go:199`, `286`, `345` | A but read-only |
| repair the exact BUDGET_UNKNOWN record, then run metasystem health | `health.go:681`, `751` | N |
| record the retro receipt with scripts/receipt.sh add ... | `health.go:265` | A |
| repair the counselor renderer and run metasystem steward tick | `counselor_carriage.go:134` | N then A |
| run a journaling goal verb that examines the canonical tip; 'metasystem goal fetch' does not examine | `ledgerattention.go:644` | N (a class, not a command) |
| no notification channel is configured; ... set metasystem.steward.notify-command | `runner.go:222` | A (a config key) |
| steward runner repair stopped with %s / cannot be verified / completed a pass but its process identity is no longer live | `runner.go:229`, `245`, `257` | N |
| replacement runner pid %d did not complete generation %d within %s | `runner.go:359` | N |
| a runner already guards this repository | `runner.go:78` | N |

### 3.4 census and dispatch (delegate path)

| Text | Where | Verdict |
| --- | --- | --- |
| dispatch refused: last census verdict is CENSUS-FAILED | `attest.go:51-52` | N |
| dispatch refused: census verdict is not successful / unreadable / schema or writer is invalid / freshness fields invalid / generation fields invalid / arming record unreadable | `attest.go:31-34`, `38-47`, `54`, `59`, `67`, `71-76` | N |
| dispatch refused: census verdict is stale (...); retry in a moment; re-arm with %s --repo %s if supervision is dead | `attest.go:88-91`, `98-100` | A when the caller's hint names `metasystem up` or `arm-supervision.sh`; the hint is caller-supplied (`cmd/metasystem/dispatch_verbs.go:1260`) |
| dispatch refused: census verdict does not attest the current supervision state / fingerprint does not match | `attest.go:95`, `103` | N |
| dispatch refused: watcher ceiling attestation ... (six texts) | `attest.go:117-146` | N |
| caller classification refused: ... (nine texts) | `classify.go:106-274` | N |
| checkout lease renewal refused: caller is not the authenticated holder | `lease/verbs.go:438` | N |
| SPLIT_RATIFY_REFUSED: ... run metasystem up to establish the authenticated lease, then retry | `goalsync_mutations.go:534` | A |

### 3.5 goal fetch path

| Text | Where | Verdict |
| --- | --- | --- |
| no accepted tree; the first fetch or the migration bootstraps it | `internal/goal/project.go:39` | N |
| the accepted tree is %s old; goal list --fetch validates and advances it | `project.go:73` | X (`goal list` has no `--fetch`; the verb is `goal fetch`, `main.go:419`) |
| rewound canonical branch refused: ... repair --accept-remote is the deliberate path | `fetchadvance.go:101` | A since `goal repair --accept-remote --by` landed (`main.go:420`, `goalsync_verbs.go:419-429`); T in spirit (human-only) |
| repair --accept-remote is a human-reserved act and names its human (--by) | `accepted.go:24` | A |

Totals over the texts above: 14 A, 3 T, 1 X, roughly 45 N. On the up and
supervise paths specifically, 5 of 17 up remedies and 0 of 11 supervise
refusals name a command.

## 4. The leak sources

### 4.1 Who spawns real machinery

- `steward arm` and `steward restart` launch a runner with `Setsid`
  (`runner.go:442-453`); `up` launches the owner with `Setsid`
  (`arming.go:613`) and the owner launches its components the same way
  (`supervise/proc.go:60-72`). None of them is in the launching process
  group, so a harness killed by signal leaves all of them alive.
- The runner loop stops only on a stop file (`runner.go:95-98`) or a failed
  lock (`77-78`). It never checks that its repository still exists: a bed
  removed under it keeps the runner ticking against a missing tree. The
  303 steward runners on m1 are this shape (goal proof-harness-process-custody,
  second specimen).
- Shell fixtures that arm real machinery: `health-fixtures.sh` (three
  `steward arm`/`restart` calls at lines 330, 378, 453, two `up` calls),
  `supervision-fixtures.sh` (one `up`, five `--shutdown` calls),
  `dispatch-fixtures.sh` (ten `--shutdown` calls; its agent-fixture bed at
  `dispatch-fixtures.sh:427-445` is the "agent-fixture" name in the leaked
  process argvs), `delegate-caps-fixtures.sh` (two `--shutdown` calls).
  Every other `*-fixtures.sh` arms nothing. Counted with
  `grep -c` over `scripts/agents/*-fixtures.sh`.
- Go tests that arm real supervision or run a runner loop:
  `internal/supervise/arming_test.go` and `internal/steward/runner_test.go`
  (the only test files matching `EnsureArmed(`, `RunLoop(`, `launchRunner(`);
  `internal/up/up_test.go:351` exercises `Shutdown`. Their children die with
  the test binary only when the test itself reaches its cleanup.

### 4.2 How cleanup is supposed to work

- `scripts/validate-metasystem.sh:1439-1497`: tracks every armed repository
  (`1447-1454`), calls `arm-supervision.sh --shutdown` for each in the EXIT
  trap (`1456-1473`), waits up to five seconds for stragglers by `pgrep -f`
  on the temp path (`1481-1484`), then removes the bed only on a green run;
  a red run MOVES the bed aside under `artifacts/agents/suite-failures/`
  (`1486-1497`). Signal traps for INT and TERM (`1509-1510`).
- The fixture scripts share the `fixture_bed_parent_cleanup` shape
  (`fixture-bed-scenarios.sh:20-42`): TERM the child, `trap ... EXIT`.
  `supervision-fixtures.sh:359-416` and `dispatch-fixtures.sh:256-307`
  additionally `--shutdown` every armed repository and `kill -KILL $(pgrep -f
  "$tmp")` as a last resort.

### 4.3 Why 488 processes and 8,789 beds survived

1. Shutdown covers the owner and its components (`arming.go:773-791`) but
   not the steward runner; only `steward disarm` stops a runner. One fixture
   calls it, `health-fixtures.sh` (one `steward disarm` line against three
   arm/restart calls); `validate-metasystem.sh` and every other fixture call
   none. Health-fixtures' cleanup at `health-fixtures.sh:144-189` otherwise
   kills by recorded pid, which misses a runner restarted by `steward restart`
   under a new pid unless re-recorded.
2. Every trap dies with SIGKILL, a machine sleep, a tmux pane kill, or the
   2026-08-03 style hang kill; the `Setsid` children survive all of these.
3. A preserved failure bed (`validate-metasystem.sh:1486-1497`) keeps its
   directory by design while its runner keeps writing narration into it.
4. Beds made by `mktemp -d` at the fixture level (`health-fixtures.sh:101`,
   `dispatch-fixtures.sh:202`, `supervision-fixtures.sh:103`) are removed only
   by the same traps. Go's `t.TempDir` (1,136 uses under `internal/`) is
   removed only when the test function returns.
5. Nothing sweeps afterwards: the census sees each survivor as UNTRACKED
   (`run.go:332`) when it is in the checkout's scope, and as nothing at all when
   its bed is under the user's temp directory, outside every live checkout's
   scope (`scope.go:9-12`). The steward's worker census treats UNTRACKED as
   blocking a death proof (`steward/census.go:64-74`) and treats a failed
   census as unprovable (`43-46`). No verb signals a process that no live
   record owns; custody is "only a live record joined on pid, start and tag"
   by design, so an orphan can never re-enter custody.

## 5. The harness layer

Three failures live outside the engine and the tree confirms the engine has
no hook into them:

- The Claude Code permission classifier refusing the engine's own verbs
  (specimen 1's two refusals, the bulk-kill refusal, settings writes: handoff
  m1 lines 37-41; m1b line 39 "could not identify the immediate claude agent
  process" for the hook from a tool call). Nothing in `cmd/metasystem`,
  `internal/up` or `internal/steward` reads or records a harness permission
  decision. The engine can detect only the absence of its own effect (a verb
  that never ran leaves no journal line) and can name the fact in a remedy;
  it cannot route around a refused verb.
- Remote-control messages stranding (specimen 6): no seat channel exists in
  the tree (`plans/never-idle-analysis.md:71`, `470-477`, and its proposal G
  at `577`). The engine can only document until a channel registry exists.
- A session on a stale harness version rejecting the fleet model (specimen 5):
  the adapter parses `claude --version` for delegate launches
  (`adapters/claude.sh:29`) and the capability snapshot records the CLI
  version, but the SEAT's own harness version is never observed. The engine
  could detect it at `up` time (the ancestry proof already finds the harness
  process, `up.go:178`) and name it; it cannot restart the seat.

What the engine can do about all three today: nothing automatically. The
lawful engine-side act is to make every refusal name the seat-runnable
command and to stamp harness-layer causes as such, so a seat stops probing
variants (handoff m1 line 41 records the probing as a dead end).

## 6. Root cause and split

### 6.1 The seat's statement, confirmed and corrected

Confirmed: there is no designed path for the partial states, and refusals on
the partial-state paths name prose or a terminal (section 3: 0 of 11 supervise
refusals and 12 of 17 up remedies name no seat-runnable command). Confirmed:
recovery became hand surgery through a classifier that refuses exactly that
surgery (section 5).

Correction A, the human-only lock is narrower than the statement reads. Only
`steward arm` without a word and `steward restart` are terminal-gated
(`steward_verbs.go:561-578`). `up` is seat-runnable in every state, already
takes over a dead owner (`arming.go:716-725`), already replaces a drifted
generation (`749-760`), and already restarts a dead runner (`runner.go:236-259`).
The temporary-word arm makes engine drift seat-recoverable today
(`steward_verbs.go:498-513`) and the standing word is on m1's identity record.
So the drift wall is a remedy TEXT defect (`up.go:392`) and a classifier
defect, not a law defect. The law question the goal raises (TENSION) reduces
to one act: after a rebuild under a standing relayed word, may `up` re-pin
itself, or must the seat call `steward arm --temporary-human-word` first?

Correction B, identity drift is not merely "refused"; up can CREATE the
partial state. Two comparators disagree: `ownerLiveness` uses the full
reference (`arming.go:152-158`, `identity.go:223-241`) and `InspectArmedAt`
uses the start second only (`verifyarmed.go:32-41`, `79`, `103`). A one-second
drift passes the first and fails the second, so up reports the owner
"verified" and then fails with "the recorded owner identity is not live". When
the full reference also fails, the Dead branch signals live components and
releases a live owner's lock (`arming.go:716-725`), which is how a checkout
ends with a drifted owner alive beside a new one (specimen 4's shape).

Correction C, a stop exists but is hidden and partial. `up --shutdown` stops
owner and components from the holder (`up.go:592-602`) and `steward disarm`
stops the runner (`runner.go:523-549`); `up --retire` drops the announcement.
Nothing composes them, nothing proves the result by census, and both flags are
labelled internal (`cmd/metasystem/up.go:84-85`). Wido's stop requirement is
therefore a composition and promotion, not a new mechanism.

Correction D, the leak is a custody-by-design gap, not a census gap. The
census classifies correctly; custody is a live record only; `Setsid` children
outlive every custodian and the runner never checks its bed (section 4). A
reaper needs a new custody fact (a recorded bed root plus a bound) because the
existing three-factor join can never re-own an orphan.

Correction E, the 13-minute silence has two parts: up prints nothing until it
returns (`cmd/metasystem/up.go:63-68`) and the owner's 5-second first-heartbeat
cap tripped repeatedly under load (owner.log tail), the load-fragile cap class
R-35-m3 already names. The records place the 13 minutes before the
announcement (section 2, specimen 2), which contradicts the goal's "while it
replaced supervision"; gap G2.

Corrected statement: the engine has one recovery verb, `up`, that already
repairs most partial states, but (1) its refusals name prose or a terminal
instead of the command that exists, (2) its two identity comparators disagree
so it can fail and even create the drifted-owner state, (3) its stop is hidden,
split across three flags and unproven, (4) nothing owns machinery that outlives
its custodian, and (5) it is silent while it waits, so a seat cannot tell a
slow repair from a hang and reaches for surgery the harness classifier refuses.

### 6.2 The arc

Slices in dependency order, each within 240 reserved minutes, each with the
rehearsal fixture that replays a specimen and must recover it. Each slice
travels the R-38-m2 ladder; the fixture named is the slice's acceptance
fixture, not its only test.

| Slice | Content | Fixture replays | Absorbs, re-scopes, leaves |
| --- | --- | --- | --- |
| S1 Remedy truth and progress lines (mechanical, 120 min) | Every remedy on the up, supervise, steward, attest and goal-fetch paths names one command that exists and the seat's class can run, or says "no seat-runnable command; harness layer" by name; `up` prints each component line when it completes instead of at return; `project.go:73` names `goal fetch`; `up.go:392` names the temporary-word arm beside the terminal arm | specimens 1 (drift refusal text), 2 (silence), 7 (wrong flag), 8 (circular remedy) | absorbs the remedy-text list from fleet-join-bootstrap slice 1; leaves repair-accept-remote-verb (done) |
| S2 One identity comparator (design-bearing, 240 min) | `InspectArmedAt` and every seconds-only caller use the same full reference as `ownerLiveness`; the tolerance rule the vm-epoch design revision 2 chose; the Dead branch never signals a group whose full reference is alive | specimen 4, and the m0 sighting in vm-epoch-identity-drift | re-scopes vm-epoch-identity-drift to its comparator core (its design and Sol round 2 are input); the fork on that goal is Wido's |
| S3 Idempotent up and one stop verb (design-bearing, 240 min) | `up` on a healthy checkout is a no-op; on stale lock, dead runner, drifted owner and drifted generation it repairs without flags; one operator stop (promote `--shutdown` or a `down` verb) composes owner, components, runner and announcement and ends with a census that reports zero CUSTODY or ANNOUNCED for the checkout; both verbs holder-gated, never terminal-gated | specimens 4 (stop from a drifted state) and the arming-dead-owner-takeover fixture (stale record plus plain up) | absorbs arming-dead-owner-takeover; depends on S2 |
| S4 Custody for fixtures and a seat-runnable sweep (design-bearing, 240 min) | Runners and owners record their bed root and exit when it vanishes; fixtures arm through one custodian that stops runners as well as owners; a `janitor` sweep verb reaps engine-signature processes whose recorded bed is gone or older than a bound, under the census's eye, and removes their beds | specimen 3 (harness killed mid-run leaves zero orphans; a surviving orphan is reaped by the verb) | absorbs proof-harness-process-custody (re-scoped from load generators to all fixture machinery, the load-generator group custody becomes one case) |
| S5 Engine drift without a terminal (design-bearing, 240 min; the TENSION slice) | Under a standing temporary word on the identity record (`identity.go:41-46`, `reviewBy`), `up` re-pins the invoking engine itself and records the word; without a word it refuses naming `steward arm --temporary-human-word`; terminal re-approval by `reviewBy` stays human | specimen 1 (pull, rebuild, plain up reaches armed) | absorbs the "existing seat wall" of fleet-join-bootstrap; leaves the fresh-clone join (engine build, roster, ledger refs) to fleet-join-bootstrap |
| S6 Hook root and harness-layer naming (mechanical after its landed design, 120 min) | The hook resolves the metasystem root on nested checkouts; health names harness-layer causes (classifier refusal, seat harness version) as detect-and-document with no remedy claim | specimen 8 (hook-freshness alive on a nested layout), specimen 5 (version named at up) | absorbs supervision-hook-wrong-root (design revision 3, waiting on the attempts fence); leaves specimen 6 to never-idle-ironclad's seat channel proposal |
| S7 Recovery rehearsal matrix (mechanical, 240 min) | One fixture group that puts a bed into each of the eight states and requires one `up` (or the stop verb) to reach green health or a refusal naming a seat-runnable command; it is the goal's DONE gate | all eight | absorbs recovery-rehearsal's fixture leg; leaves its enrolled-ring leg (agent-free arm) to the human |

Left untouched by this arc: codex-handshake-budget-load-fragile (claimed by
m0b, its own ladder) and repair-accept-remote-verb (landed).

## 7. Self-grade

Confidence: high on sections 1, 3 and 4 (every row was read at the cited
line); medium on section 2 where a specimen has no record beyond the goal
Intent; medium on correction B's second half (that the Dead branch fires on a
live owner needs a full-reference mismatch, which the m0 record says did NOT
happen because ticks and boot id matched; the first half, the two-comparator
disagreement, is read directly).

Weakest claim: the placement of specimen 2's 13 minutes before the
announcement. It rests on three timestamps (steward runner start 11:21:42Z,
announcement 11:35:48Z, owner start 11:35:49Z) and the assumption that up was
launched right after the arm; the up transcript itself is not in the tree.

Reject condition: if `identityAlive` behind `census.Alive`
(`internal/census/verbs.go:17-19`) turns out to consult ticks and boot id
despite its seconds-only signature, correction B falls and S2 shrinks to a
tolerance rule only. If the 303 leaked runners came from beds that never ran
`steward arm` or `steward restart`, section 4.1's attribution to
health-fixtures falls and the runner leak needs another source.

## 8. Gaps (reported, not filled)

- G1: specimen 4 (m0b, owner pid 27632) has no record in the tree besides the
  goal Intent; the m0b handoff or session record is needed to confirm which
  refusal text it saw and whether `up --shutdown` was tried.
- G2: specimen 2's up transcript is not recorded; the arming log carries only
  the announcement and first-census lines. Which pre-announcement step took the
  time is unknown.
- G3: the delegate path's re-arm hint (`attest.go:88-91`) is caller-supplied;
  the value the seat's `metasystem delegate` passes was not traced to its call
  site in this round.
- G4 (closed in this round): `steward disarm` is called once, in
  `health-fixtures.sh`; the validator and every other fixture never call it.
