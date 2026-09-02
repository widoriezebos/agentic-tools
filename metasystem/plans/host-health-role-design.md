# Host health role design (host-health-role)

Working Mode: design
Goal: `plans/goals/host-health-role.md` (revision 3, claimed by m1)
Brief: `plans/host-health-role-design-brief.md`
Author: dispatch delegate host-health-design-r1, 2026-09-02, round 1
Every tree reference below was read in this worktree today. No command that
touches the live machine was run: the delegate sandbox refuses `ps` and
`sysctl` outright, so the portable formats in section 3 are read from the
tree and from the POSIX and platform manuals, not observed here.

## 1. Verdict up front

One new steward health role, `host-pressure`, evaluated on every steward
tick beside the fourteen roles that exist today. It reads six host facts
through one injected reader, compares them with six configuration keys,
counts consecutive over-threshold ticks in the health record the steward
already keeps, and goes dead only when a threshold has held for the
configured number of ticks. A dead verdict names each offending process,
says whether it is ours by the census's own scope and ownership rules, and
carries a remedy: for a foreign process the exact text "not ours, tell the
operator"; for ours, the command the custody design owns. The steward never
kills anything on the strength of this role. When healthy, the role adds
one short clause to the existing health line and nothing else.

The alert path is unchanged. A dead role with no lawful automatic remedy
raises `ShouldAlert` on its first dead tick (`internal/steward/health.go:386-389`,
default branch of `hasLawfulAutomaticRemedy` at `health.go:442-443`), which
opens and submits one alert episode keyed by the finding digest
(`internal/steward/alert_episode.go:285-302`, `324-341`; called from
`internal/steward/tick.go:293`). That is the "immediate named action" of
Wido's role-liveness order: the episode opens on the same tick the role
first goes dead.

## 2. Facts from the tree

| Fact | Where |
| --- | --- |
| Roles are a fixed vocabulary and an ordered list; each returns alive, dead or unknown with a reason and an optional remedy | `health.go:40-76`, `78-89`, `991-1001` |
| All roles are evaluated by one function; the steward tick and the hook preview share it; the preview never advances durable state | `health.go:243-261`, `175-211`, `216-241` |
| The health record durably carries per-role counters and is validated on load; unknown roles or negative counters refuse the record | `health.go:102-111`, `1127-1172` |
| The tick calls `ObserveHealth`, narrates `Line()`, then joins the verdict to the episode store | `tick.go:256`, `270-297` |
| A dead role whose remedy is not automatic escalates to `NO_LAWFUL_REMEDY` and alerts on the first dead observation | `health.go:377-400`, `416-445` |
| The episode message is set only when the episode is created; a later observation with the same digest reuses the stored episode and never rewrites it | `alert_episode.go:285-302` (message assigned at `293-296` only); same-episode reuse proven at `internal/steward/notify_test.go:124-130` |
| The finding digest hashes only `role=status` pairs, so one role's changing reason keeps one episode | `health.go:1116-1125` |
| The tick cadence defaults to 600 seconds | `internal/steward/runner.go:51-60` |
| Threshold-style keys live in `metasystem.conf` and are read with a minimum through `boundedConfig` | `metasystem.conf:11`, `health.go:1051-1068`, `internal/config/attention.go:9` |
| A role reads installed configuration from the metasystem root, not the repository root, when the two differ | `health.go:899-912` |
| The most recent role (`claimed-goal-delivery`) sets `NoAutomaticRemedy` and names inspection as its remedy | `internal/steward/delivery.go:297-301` |
| The census lists only runtime-signature processes in scope; scope is cwd or an argv path below the checkout | `internal/census/run.go:171-199`, `218-243`; `internal/census/signature.go:65-75`; `internal/census/scope.go:52-62`, `69-106` |
| Ownership inside the census is CUSTODY, ANNOUNCED or UNTRACKED, joined on live records; the verdict file carries pid, class, tag, registry and argv per item | `run.go:306-333`, `51-64`; read by health at `health.go:531` |
| The engine's own processes carry no runtime signature; the leaked runners were invisible to the census for that reason | `plans/recovery-analysis.md:398-402`; `plans/proof-harness-custody-design.md:47-49` |
| Every engine shape the janitor proves ownership for includes the word `metasystem` in argv | `internal/janitor/killproof.go:45`, `57-58` |
| The enrolled engine path is `InstallPath` on the identity record | `internal/steward/identity.go:34-35` |
| Temp beds are `mktemp -d` and `metasystem-*.XXXXXX` under `${TMPDIR:-/tmp}` | `proof-harness-custody-design.md:45` |
| The lawful sweep of unowned engine processes is `metasystem janitor orphans`, report-only by default; the verb is designed, not landed | `proof-harness-custody-design.md:227-255`; `cmd/metasystem/main.go` has no `orphans` word |
| The restart remedy the health roles already name is `metasystem up --repo <root>` | `health.go:448`, `1070-1072` |
| Native sysctl reads exist on darwin through `unix.SysctlRaw` with a hand decoder; procfs reads exist on linux | `internal/identity/enumerate_darwin.go:13-29`; `internal/identity/identity_linux.go:118-133` |
| Free-space measurement by `Fstatfs` exists, with the rule that APFS entries are per-path advisories | `internal/janitor/headroom.go:83-131`, `44-52` |
| `ps` is in the engine's required command list and in the command inventory contract; `sysctl` and `uptime` are not | `internal/up/up.go:281-285`; `docs/project-rules.md:104-111` |
| The verified platforms are macOS on arm64 and Debian 12 on arm64 | `docs/project-rules.md:94-100` |
| `golang.org/x/sys` v0.47.0 is already a dependency; it has `SysctlRaw` but no typed swap or load structs for darwin | `go.mod:7`; module cache `unix/syscall_bsd.go:471` |
| Role tests drive `applyHealthObservation` and `UpdateAlertEpisodes` directly on a temporary bed | `internal/steward/health_test.go:280`; `notify_test.go:106-146` |
| The 2026-09-02 numbers: fseventsd at 100 percent CPU and 17 GB resident for 17 days, swap 94 percent, 488 leaked fixture processes, load 6 | goal Intent, `plans/goals/host-health-role.md:4`; the leaked runners' argv names `agent-fixture` (`proof-harness-custody-design.md:46`) |

## 3. The role

Name: `RoleHostPressure HealthRole = "host-pressure"`, appended last in
`healthRoleOrder` (`health.go:61-76`) so every existing line prefix is
unchanged. Interval: every steward tick, through `ObserveHealth`
(`tick.go:271`), default 600 seconds (`runner.go:59`); the hook's
`PreviewHealth` (`health.go:216`) evaluates the same facts read-only.

What it reads, through one interface `HostFactsReader` with one method
`ReadHostFacts(repoRoot string) (HostFacts, error)`, defaulted to the
kernel reader when nil exactly as the prober is defaulted
(`health.go:176-178`):

| Fact | darwin | linux | both |
| --- | --- | --- | --- |
| Core count | | | `runtime.NumCPU()` |
| One-minute load | `unix.SysctlRaw("vm.loadavg")`: three `uint32` fixed-point values, 4 bytes padding, one `int64` scale; load = `ldavg[0] / fscale`; decoded like `enumerate_darwin.go:23-29` | first field of `/proc/loadavg`, read like `identity_linux.go:124-133` | |
| Swap used and total | `unix.SysctlRaw("vm.swapusage")`: `xsu_total`, `xsu_avail`, `xsu_used` as three `uint64` | `SwapTotal` and `SwapFree` lines of `/proc/meminfo`, kB | total 0 means no swap; the fact reads "no swap" and no threshold applies |
| Disk free and total on the checkout's volume | | | `syscall.Statfs` on `repoRoot`; free = `Bavail*Bsize`, total = `Blocks*Bsize`, checked multiply as `headroom.go:120-129` |
| Process table | | | one `ps -A -ww -o pid=,pcpu=,rss=,args=` exec; the first three whitespace fields are pid, CPU percent and resident KiB, the remainder is argv |

The `sysctl` in the brief is the kernel interface, satisfied natively by
`x/sys/unix` as the identity package already does; no `sysctl` binary is
executed, so the command inventory (`project-rules.md:104-111`) gains
nothing. `ps -A` and the `pid`, `pcpu`, `args` columns are POSIX; `rss` is
not POSIX but is accepted by both the macOS BSD `ps` and procps `ps`, in
KiB on both; `-ww` is accepted by both and prevents argv truncation. The
reader returns the whole table; the role sorts it and keeps the top three
by CPU and the top three by resident memory. Known difference, named in
the source comment: macOS `pcpu` is a decaying recent average, procps
`pcpu` is CPU time over process lifetime, so a process that has just
started burning shows up later on linux. The consecutive-tick rule below
tolerates that.

The role does not count processes. The 488 leaked processes of 2026-09-02
are the custody sweep's fact (`proof-harness-custody-design.md:227-255`);
this role sees a leaked process only when it climbs into a top three.

## 4. Thresholds, streaks and the verdict

Keys in `metasystem.conf`, read from the metasystem root with
`boundedConfig(metasystemRoot, key, default, 1)` (`health.go:1055-1068`):

| Key | Default | Meaning |
| --- | --- | --- |
| `steward.host-load-per-core-percent` | 200 | one-minute load above this percent of the core count |
| `steward.host-swap-used-percent` | 80 | swap used above this percent of swap total |
| `steward.host-process-cpu-percent` | 90 | a single process at or above this CPU percent |
| `steward.host-process-resident-gb` | 8 | a single process at or above this many GiB resident |
| `steward.host-disk-free-percent` | 10 | free space on the checkout's volume below this percent |
| `steward.host-alert-ticks` | 3 | consecutive ticks a threshold must hold before the role goes dead |

An unreadable key makes the role unknown with the remedy
`metasystem config validate --conf <metasystemRoot>/metasystem.conf`
(precedent `health.go:904`). A reader error makes the role unknown with the
reason `host facts unreadable: <error>` and the remedy `metasystem health
--repo <root>` (precedent `health.go:199`); two unknowns in a row alert
through the existing rule (`health.go:365-370`).

Streaks. `HealthObservationState` (`health.go:105-111`) gains one field,
`HostStreaks map[string]int` with `omitempty`. Keys are `load`, `swap`,
`disk`, and `process:<pid>:<basename of argv[0]>`. On each observation the
role receives the previous map read-only and returns the next one: a key
over its threshold this tick is `previous+1`, every other key is dropped,
so the map holds at most nine entries. `ObserveHealth` stores the returned
map in the state it saves (`health.go:206-207`); `PreviewHealth` discards
it. `loadHealthRecord` accepts a nil map and refuses a negative count, the
same way it treats `FailureEpisodes` (`health.go:1158-1170`). The signature
change: `evaluateHealthRoles` takes the reader and the previous streaks and
returns the roles plus the next streaks; `ObserveHealth` and
`PreviewHealthAt` pass them through. Pid reuse within one tick would
inherit a streak; the residue is named and accepted, because the portable
`ps` format carries no start time.

One tick rule for every fact. The brief attaches the tick count to load and
processes only. This design applies `steward.host-alert-ticks` to swap and
disk as well, for one reason from the tree: the episode message is frozen
when the episode opens (assigned only at `alert_episode.go:293-296`; the
reuse of the stored episode for a later rendering is proven at
`notify_test.go:124-130`) and the digest does not change with the reason
(`health.go:1116-1125`), so a single-tick swap verdict on tick one would
open the 2026-09-02 episode with a message that never names fseventsd. With
one rule, the role goes dead on tick three naming every crossing at once.
The cost is thirty minutes of delay on swap and disk at the default tick.

Verdict. Dead when any key's streak reaches the tick count. Reason:

```
HOST_PRESSURE <n>/<M> ticks: <finding>[; <finding>...]
```

Findings, in this order: `load <l> on <c> cores (threshold <N>% per core)`;
`swap <p>% used of <total> (threshold <P>%)`; `disk <f>% free on <repoRoot>
(threshold <S>%)`; then one per process, `process <basename> pid <pid> at
<cpu>% cpu, <rss> resident (<ownership>)`. Alive reason, the quiet clause
on the existing line: `load <l>/<c>, swap <p>%, disk <f>% free, cpu:
<a> <x>%, <b> <y>%, <c> <z>%, rss: <a> <x>, <b> <y>, <c> <z>`, plus
` (building: <key> <n>/<M>)` for any key with a streak, so the operator sees
a threshold that is about to fire. `NoAutomaticRemedy` is always true
(precedent `delivery.go:297-301`).

Ownership, decided in this order and stopping at the first match:

1. The pid appears in the inventory of
   `artifacts/agents/supervision/last-census.json` (read as at
   `health.go:531`, item shape `run.go:51-64`): ours, with the census
   class and tag. Text: `ours: custody <tag>`, `ours: announced main <tag>`,
   `ours: untracked`.
2. `census.ArgvPaths(argv, "")` (`scope.go:69-106`) names a path that
   `census.PathBelow` (`scope.go:52-62`) places under the metasystem root,
   the repository root, or the directory of the enrolled `InstallPath`
   (`identity.go:34-35`, read through `VerifyIdentity` as
   `installedGeneration` does at `health.go:1003-1016`): `ours: engine
   process, no custody`.
3. `ArgvPaths` names a path under `realpath(${TMPDIR:-/tmp})` (resolver at
   `scope.go:20-38`) AND an argv word equals `metasystem` or ends in
   `/metasystem` (every janitor shape includes that word,
   `killproof.go:45,57-58`): `ours: fixture bed <first such path>`. Both
   halves are required; a foreign process that merely uses `/tmp` is not
   ours.
4. Otherwise `not ours`.

Rules 2 and 3 exist because rule 1 never sees the engine's own binaries
(section 2, recovery-analysis row). Rule 1 also depends on census
freshness, which its own role already guards (`health.go:529-569`).

Remedy, one per finding, joined with `; ` after removing duplicates:

| Ownership | Remedy text |
| --- | --- |
| not ours | `not ours, tell the operator` |
| ours: custody | `ours: job record <registry> owns it; the reaper's cap ends it; no health action` |
| ours: announced main | `ours: session main <tag>; tell the operator` |
| ours: untracked, engine process, fixture bed | `metasystem janitor orphans --root <metasystemRoot> --older-than-min 0` when that verb exists in `cmd/metasystem/main.go` at build time; otherwise `no lawful sweep has landed; kill by exact pid after proving ownership, or land proof-harness-process-custody` |
| load finding | the remedy of the top process by CPU |
| swap finding | the remedy of the top process by resident memory |
| disk finding | `free space on <repoRoot>'s volume; tell the operator` |

The build-time choice for the sweep remedy is mechanical: the implementer
greps `main.go` for the word `orphans`; the custody goal
(`proof-harness-custody-design.md`, claimed by m1) lands that verb on its
own slice, and whichever lands second updates one string. A remedy must
name a command that exists (`recovery-analysis.md:428`).

Restart is not a remedy here. `metasystem up` (`health.go:1070-1072`)
repairs supervision it owns; a hot engine process outside custody is the
sweep's, and a hot process inside custody is the reaper's by cap.

The 2026-09-02 line, at tick three with the defaults:

```
host-pressure=dead (HOST_PRESSURE 3/3 ticks: swap 94% used of 8.0 GiB (threshold 80%); process fseventsd pid 312 at 100% cpu, 17.0 GiB resident (not ours); remedy: not ours, tell the operator) [failure 1/5; no lawful remedy]
```

## 5. The fixture

One Go test file, `internal/steward/hosthealth_test.go`, with a scripted
reader that returns one `HostFacts` per call and never touches the machine.
Snapshot A is the 2026-09-02 record: 8 cores, load 6.0, swap 94 percent
used (fixture total 8 GiB, used 7.52 GiB; the percent is the recorded fact,
the absolute is filler), disk 40 percent free, and three processes:
fseventsd pid 312 at 100 percent CPU and 17 GiB resident; a Java process
carrying `-Dmorpheus.instance.id=yoda-prod-head` at 35 percent and 4 GiB
(foreign, under threshold); and a leaked runner whose argv is
`<TMPDIR>/metasystem-agent-fixture.Ab12Cd/bin/metasystem steward run --repo
<TMPDIR>/metasystem-agent-fixture.Ab12Cd/repo` at 12 percent and 60 MiB
(ours by rule 3, under threshold, present in the top three). Snapshot B is
healthy: load 1.2, swap 20 percent, disk 40 percent free, every process
under threshold. Snapshot C is B with the leaked runner at 95 percent CPU.

| Case | Feed | Assert |
| --- | --- | --- |
| 1 | A for three ticks through `checkHostPressure`, `applyHealthObservation` (precedent `health_test.go:280`) and `UpdateAlertEpisodes` on a bed (precedent `notify_test.go:106`) | ticks one and two: role alive, reason contains `building: swap 1/3` then `2/3`, no episode file; tick three: role dead, `NoLawfulRemedy`, `ShouldAlert`, exactly one file under `artifacts/agents/steward/alerts`, its message contains `fseventsd`, `not ours` and `not ours, tell the operator`, and does not name the Java process as an offender |
| 2 | B for four ticks | role alive every tick, streak map empty, no episode file, reason contains `load 1.2/8` |
| 3 | A for two ticks then B for two ticks | no episode file at any tick; the streak map is empty after the first B tick |
| 4 | C for three ticks | tick three dead; the finding names the runner as `ours: fixture bed <TMPDIR>/metasystem-agent-fixture.Ab12Cd` and the remedy is the sweep text of section 4 |
| 5 | A with the fseventsd pid also present in a written `last-census.json` inventory as CUSTODY with a tag | ownership reads `ours: custody <tag>`, proving rule 1 precedes rule 4 |
| 6 | a reader that returns an error | role unknown, reason starts with `host facts unreadable` |
| 7 | decoder units: a fixed 24-byte `vm.loadavg` sample and a fixed 32-byte `vm.swapusage` sample (darwin file, build-tagged); fixed `/proc/loadavg` and `/proc/meminfo` text (linux file); a fixed `ps` sample with spaces in argv | exact numbers |

The kernel reader gets one smoke test that calls it once and checks cores
at least 1 and load at least 0, following the live `AllPids` test the
identity package keeps (`enumerate_darwin.go:20-22`). It skips, not fails,
when the `ps` exec is refused with a permission error, because this very
sandbox refuses `ps`. No role test reads the machine.

## 6. Size and blast radius

Files: `internal/steward/hosthealth.go` (role, ownership, text, about 200
lines), `hostfacts.go` (interface, `ps` parser, statfs, about 120),
`hostfacts_darwin.go` and `hostfacts_linux.go` (about 60 each),
`hosthealth_test.go` (about 250), `health.go` (about 25 lines: the role
constant and order entry, the streak field and its validation, the reader
and streak plumbing through `evaluateHealthRoles`, `ObserveHealth` and
`PreviewHealthAt`), `metasystem.conf` (six keys). No document lists the
health roles today (grep of `docs/` for `claimed-goal-delivery` finds
nothing), so no document changes. Untouched: the alert episode store, the
notifier, the census, the janitor, the tick's structure.

Precedent: the most recent role, `claimed-goal-delivery`, landed on
2026-09-01 as one codex job with zero corrections
(`memory/receipts.log:219`), at 245 lines of role code, 168 of tests and 3
in `health.go` (commit d252c785). This slice is about 1.6 times that by
lines and adds a platform-split reader, so it fits the 240-minute box with
the reader as the part most likely to eat the margin. The precedent job's
elapsed time is not in this tree (its job record lives on m0), so the
estimate is supported by line count only and unsupported beyond that.

## 7. Self-grade

Confidence: high on sections 1, 2 and 4, where every mechanism was read at
the cited line; medium on section 3's platform details, because the
sandbox refused `ps` and `sysctl` and the formats rest on the manuals and
on the identity package's existing native reads rather than on a run here.

Weakest claim: that `ps -A -ww -o pid=,pcpu=,rss=,args=` prints the same
four columns in the same units on the Debian 12 host. The `rss` column is
not POSIX; both manuals document it in KiB, but the build must run the
parser fixture on both verified platforms before the role ships.

Reject condition: if the hook's `PreviewHealth` path is required to alert
on host pressure without the steward tick, the streak state must move out
of the health record into a store the preview may advance, and section 4's
"preview discards" rule is wrong. Also reject if `UpdateAlertEpisodes` is
found to rewrite an open episode's message on a changed reason; then the
uniform tick rule loses its stated reason and swap and disk may go back to
single-tick verdicts.
