# Two bars for changes — the caller-class slice (design, revision 3)

- Goal: two-bars-for-changes (plans/goals/two-bars-for-changes.md,
  revision 26, the paragraph "NEW SLICE, THE REFUSE BIT BINDS ON THE
  CALLER'S CLASS")
- Wido's word: ruling R-46-m1b, selection (4), verbatim: the
  "two-bars caller-class slice" is m1b's first claim, so that "the commit
  gate branches on the caller's class so a worker-classified session can
  no longer commit on the human branch" (memory/rulings.md:92)
- Hole: records/misc/design-gate-audit-2026-09-02.md, section C1
- Base tree read: e101670f (this worktree's HEAD; the code seams cited
  are unchanged since dd45f392, the revision 1 base)
- Mode: design only; no code in this change
- Author: implementer job implementer-178d269e0852ac7a8e897657 (round 2
  as …-r2, round 3 as …-r3), design mode, for dispatch delegate
  m1b+main-1788333346-60696-6a3256
- Revision 2 changelog: folds the three accepted round-1 findings of
  critic chain two-bars-cc-crit-3 (plans/two-bars-caller-class-dispositions.md):
  TBCC-R1-LAWFUL-DELEGATE-COMMIT-PATH (a fourth verdict path, `worker`,
  sections 1-4, 6, 7, 10), TBCC-R1-FIXTURE-STUB-CONTRACT (the bed's stub
  contract, section 7), TBCC-R1-NEGATIVE-BRANCH-PROOFS (three negative
  legs, section 7).
- Revision 3 changelog (the declared failsafe round): folds the three
  accepted round-2 findings (plans/two-bars-caller-class-dispositions-r2.md):
  TBCC-R2-LAND-SIDE-DOOR (land.sh refuses inside a job worktree through
  a new geometry verb; sections 1, 3, 4, 7, 8), TBCC-R2-NONRUNNING-WORKER
  (the worker rule requires the `running` literal; sections 1, 3, 7),
  TBCC-R2-AMBIGUOUS-MACHINE-LINEAGE (the wrapper's trailer monopoly and
  its fleet consequence; sections 1, 2, 6, 7).

## 0. The whole slice in one paragraph

Today `scripts/agents/commit.sh` decides "human or agent" from whether
the lease verb returned a claim epoch, and the lease verb returns no
epoch and no error for a worker-classified caller, so an unannounced
agent session commits on the sovereign human path where no landing
refusal fires. This slice adds one engine verb, `lease
commit-authority`, that classifies the caller and returns a typed
verdict with four paths: HUMAN takes the human path unchanged; an
announced MAIN that holds the lease takes the agent path with its claim
epoch; a DELEGATE whose runtime process is in a RUNNING dispatched
job's custody and whose repository is that job's own worktree takes the
`worker` path, ungated by the landing bar because a worktree commit
never lands; every other caller (a DELEGATE anywhere else,
ADAPTER-SUPERVISOR, SUPERVISION, STEWARD, UNTRUSTED) is refused with one
fixed sentence naming the class and the lawful path. "Never lands" is
made mechanically true by a second small verb, `lease job-worktree`,
that the landing driver consults first and refuses on. The wrapper
relays the verdict on both of its halves, derives the Machine trailer's
suffix from the same verdict, and refuses a message that already carries
any wrapper-owned trailer, so an agent landing can no longer be stamped
"+human", a worker commit names its job, and no landing carries two
Machine lines. `lease require-holder` and `lease run-held` keep their
contracts byte-for-byte; the commit-authority answer is a distinct
surface because run-held must go on passing the supervision and adapter
helpers that reap and handshake under it.

## 1. Facts traced (how the mechanism works today)

Every line number is against the base tree named above.

F1. The wrapper's branch is on the epoch, not the class.
commit.sh:9 asks `lease require-holder`; :12 collapses an absent or
null `claimEpoch` to empty ("so the human-commit branch below is taken
when there is no epoch", :10-11); :13-16 re-executes the wrapper under
`lease run-held --expected-epoch` as `__lease-held <epoch>`; :17
re-executes it under `lease run-held` as `__lease-held human`. The
inner half, :23-31, sets `agent_commit=1` only for a numeric token
(:24-27) and otherwise requires the literal `human` (:29) and re-asks
require-holder for its exit status alone (:30, output discarded). The
landing refusal at :319 fires only when `agent_commit` is 1.

F2. The engine passes the worker classes through without an epoch.
internal/lease/verbs.go RequireHolderAt :359-407: HUMAN returns
`Holder:true` (:366-367); DELEGATE, ADAPTER-SUPERVISOR and SUPERVISION
return `Holder:false` with a nil error and no epoch (:368-369). Only a
MAIN reaches the lease (:371-406); a STEWARD or UNTRUSTED caller
reaches :375-378 and errors when the lease is absent, or :390-391
(OWNED-ELSEWHERE) when it is held. gateHolder :480-496 passes HUMAN,
DELEGATE, ADAPTER-SUPERVISOR and SUPERVISION outright (:481-483), then
gates a MAIN on holdership (:489-491) and epoch (:492-494). RunHeld
:458-476 runs a HUMAN ungated before taking the lock (:464-466).

F3. Where DELEGATE comes from. internal/lease/classify.go ClassifyAt
:315-392: an ancestor whose command matches any runtime adapter
signature makes the caller DELEGATE, and the Classification carries
that ancestor's pid (:343-346). The comment block :293-307 states the
terminal rule; the check itself is :368-370 (a controlling terminal)
with the fixture-staged terminal fact :375-379, and the fall-through is
UNTRUSTED (:391). The audit proved the shape live: from an unannounced
Claude session, require-holder answered
`{"claimEpoch":null,"class":"DELEGATE","holder":false}` exit 0
(design-gate-audit-2026-09-02.md:148-150).

F4. The trailer's lineage comes from the environment. commit.sh:363
stamps `Machine: <nickname>+${METASYSTEM_OWNER_LINEAGE:-human}`. The
variable is exported by the mission runner (internal/missionrunner/
host.go:299) and by nobody else on the seat path; `lease announce`
stores the same value as the announcement's `ownerLineage`
(cmd/metasystem/lease.go:37-43), and the lease derives its lineage
from the announcement, falling back to the holder's mainId
(internal/lease/lease.go:127-143). Measured on this tree's history for
2026-09-02 (`git log --since=2026-09-02T00:00:00Z` over the Machine
trailer): 50 commits carry `m0b+human` (8 alone, 42 beside a second
hand-typed `Machine: m0b` trailer, the hand-typed one first), 5 carry
`m0+human`, 2 carry `m1b+main-1788333346-60696-6a3256`, 1 carries
`m1+coordinator`. The brief's figure of fifteen is not what this range
measures; the mechanism is the same either way: a seat without the
variable says "+human".

F5. What `lease classify` already returns. ClassifyResult
(verbs.go:239-253) carries `class`, `holder`, `claimEpoch`, `mainId`
and is marshalled in sorted wire order; TestVerbResultWireShapes
(internal/lease/verbs_test.go:30-60) pins the bytes of every typed
verb result, including HolderView's explicit nulls (verbs.go:339-348,
"the exact shape the historical map form produced").

F6. The fixture bed enters below the branch. Every commit.sh
invocation in scripts/agents/static-reproof-fixtures.sh starts at
`__lease-held 1` or `__lease-held human` (:75, :95, :109, :120, :129,
:143, :155, :164, :308, :335, :356, :369, :383, :428, :441, :449,
:468, :488, :502, :522, :581, :600, :608, :631). The bed's stub engine
answers `lease require-holder` with `{}` (:29, :223, :403), makes
`lease commit-token` a no-op (:32, :226, :406), and implements
`json get` only for the four landing-observation fields, sniffing the
`--value` argument for one code (:227-242, :407-413).
internal/behaviorsurface/consumer_wiring_test.go:36 enters at
`__lease-held human` and its stub (:66-77) answers require-holder with
`{}` and implements no `json get` at all (every other verb is a silent
no-op, :75). Lines 8-18 of the wrapper are therefore never exercised by
a fixture today, and the inner half's re-check at :30 accepts `{}`.

F7. A DELEGATE cannot write the control plane through dispatch.sh even
though the lease gate passes it. dispatch.sh lease_entry_check
:237-244 reads the class and epoch and never the holder bit; every
internal entry then calls `internal_authority` (:263-269), which runs
`job authority-check` against internal/authority Authorize
(authority.go:24-113): HUMAN and a holding MAIN are admitted (:34-42),
STEWARD, SUPERVISION and ADAPTER-SUPERVISOR per mode (:70-111), and
every other class, DELEGATE included, falls to the refusal at :112.
The shell regression WC-1 pins the DELEGATE refusal
(scripts/agents/authority-regression-fixtures.sh:20-33).

F8. A worker committing in its own worktree is an anticipated flow
(the round-1 reject condition fired on this). The codex adapter grants
the worktree's git metadata "the worktree's git metadata a commit
needs (issue #5)" because "every worktree codex implementer's commit
died read-only" without it (internal/adapter/codex.go:59-64,
:167-171); the dispatcher builds the object quarantine so "the
delegate's git writes its loose objects into a private directory
INSIDE the worktree" (scripts/agents/dispatch.sh:1374-1402, the
worktree at :1376 is `$worktrees/$job` with `worktrees="$agents/worktrees"`
:43 and `agents="$root/artifacts/agents"` :37, on the branch
`agent/<job>` :1379); the pre-commit guard requires the wrapper token
for every non-HUMAN caller (scripts/agents/pre-commit-guard.sh:44-66,
the refusal :63-66), so a worker cannot fall back to raw `git commit`;
and docs/concepts.md:104-106 states "a raw commit is refused; a wrapped
one carries the wrapper's kernel identity". Today such a commit takes
the wrapper's human path (F1, F2) with every proof the human path runs.

F9. What a job record carries, and its status vocabulary. Set at
dispatch: `workspaceRoot` (internal/dispatch/build.go:389, the resolved
worktree path; a follow-up inherits its parent's, :593), `productRoots`
(:646), `launchMode` (`worktree` or `shared-checkout`, dispatch.sh:1375,
:1405), `instanceTag`, `pid`, `pidStartedAt`, `parentJob`, `status`,
and `custodyProcesses`, each entry `{pid, pidStartedAt,
pidStartedAtExactMicro, instanceTag, pgid}` written by
`__register-custody` (dispatch.sh:2496-2499; internal/dispatch/
custody.go:68-85). The status vocabulary is the transition table at
internal/dispatch/record.go:38-42: `pending-setup` may become `failed`
or `cancelled`; `pending` may become `running`, `failed` or
`cancelled`; `running` may become `completed`, `failed`, `cancelled` or
`timeout`; the terminal set is :45-47 and is exported as
`TerminalStatus` (:52). Custody registration is accepted while a job is
`pending` OR `running` (custody.go:14-15, :40-42), so a pending job's
record can already carry a runtime pid. Only `running` means a delegate
turn is executing. internal/lease already reads job records without
importing dispatch (classify.go:253-284 reads jobId, pid, pidStartedAt,
custodyProcesses into the `adapters` custody map, and :357-364 joins a
walked ancestor's (pid, start) against it). Import directions verified
with `go list -deps`: lease does not depend on dispatch, dispatch does
not depend on lease, and gittree depends on neither, so lease may import
gittree for git plumbing without a cycle; it needs nothing from
dispatch. Live example on this machine: the root record
implementer-178d269e0852ac7a8e897657 has status `completed`, launchMode
`worktree`, workspaceRoot equal to this worktree's path, and one custody
entry; the follow-up record `…-r2` has status `running`, parentJob
equal to the root, the SAME workspaceRoot, and its own custody entry
carrying its own tag.

F10. The adapter-signed ancestor IS the custody-registered process.
claude.sh:141-149 launches the built command in a subshell that `exec`s
the CLI and registers `$!` (`register_cli_custody "$cli_pid"`);
codex.sh:151-153 does the same ("`exec` keeps the pid, which custody
registration depends on", :147); devin.sh registers the ACP server
(:340), the ACP client (:356) and the legacy CLI (:581).
register_cli_custody (runtime-common.sh:102-113) polls
`__register-custody` for up to five seconds while the child lives and
fails the handshake otherwise, so a delegate that runs at all has its
runtime pid in its record before its first turn. The signatures match
exactly those processes: `claude` (claude.sh signature),
`codex` (codex.sh), `devin` and `devin-delegate-acp` (devin.sh; the
host's raw `devin acp` excluded), read by classify.go:343-344.

F11. The instance tag rides argv for two runtimes and a half. The tag
format is `metasystem-job-<jobId>-<32 hex>` (the live records in F9).
Claude: `--name <instanceTag>` (internal/adapter/claude.go:296-297).
Codex: `-c metasystem_instance_tag="<tag>"` on dispatch and on resume
(codex.go:58, :74, :97). Devin legacy CLI: the per-turn config file is
named after the tag and passed as `--config <round_dir>/<tag>`
(devin.sh:497, :560, "naming that file with the instance tag gives
ownership checks the same exact positional proof"). Devin ACP: the
server is `exec -a devin-delegate-acp "$(command -v devin)" acp`
(devin.sh:338) and carries NO tag in argv; the census joins custody on
pid and start, not on the tag (internal/census/run.go:296-311), and
uses the tag only to verify supervision processes (:488-503).

F12. The worktree's git geometry, measured on this worktree.
`git rev-parse --show-toplevel` from the wrapper's root gives
`<main>/metasystem/artifacts/agents/worktrees/implementer-178d269e0852ac7a8e897657`;
`--git-common-dir` gives `<main>/.git`; `--show-prefix` gives
`metasystem/`. The main metasystem root is therefore the common dir's
parent joined with the prefix, and the worktree's parent directory is
`<main root>/artifacts/agents/worktrees`. The wrapper computes prefix
and toplevel already (commit.sh:141-142). internal/gittree runs the
same plumbing (gittree.go:161 `--show-toplevel`, :179 `--show-prefix`,
snapshotscope.go:461 `--git-common-dir`) under `ScrubbedEnviron`
(gittree.go:83). The machine nickname resolves from a worktree
(`git config --get metasystem.goal.machine` here prints `m1b`,
because worktrees share the common config). The machine's git is
2.50.1; `--path-format=absolute` (used by goalsync_verbs.go:71) is
available.

F13. The pre-commit guard's token path is the MAIN root's. Enrollment
pins the guard by absolute path under the enrolling root
(cmd/metasystem/goalsync_verbs.go:36-42) into the hooks directory
resolved through `--git-path hooks` (:71-80), which for a worktree is
the shared common-dir hooks; the guard derives `guard_root` from its
own location (pre-commit-guard.sh:26), classifies with `--root
"$guard_root"` (:47), and looks for the token at
`$guard_root/artifacts/agents/mains/worktree-commit-token.json` (:58,
:63). The wrapper mints its token under ITS root
(commit.sh:6, :98), which in a worktree is the worktree's metasystem
directory. On a guard-enrolled machine the two paths differ; on m1b
the guard is not enrolled (audit C7). This is recorded as a gap in
section 9, not decided here.

F14. The landing driver is a side door from a worktree. land.sh
resolves its root from its own location and enters it (:12-13), reads
the CURRENT branch from `git symbolic-ref --short HEAD` (:202), commits
through the wrapper with `-F <file>` and never `--push` (:244-252),
then fetches, rebases and pushes that branch to origin (:265-275, run
at :288-313) and mirrors it to transport (:315-317). land.sh calls no
engine verb today (its only commands are git, commit.sh and
sync-transport.sh). Run inside a job worktree after a worker-path
commit, it would publish `agent/<job>` (F8) with the landing bar never
having fired.

F15. The wrapper does not own its trailers yet. commit.sh:86-89 passes
every unrecognised argument to `git commit`, including a caller's own
`--trailer`, `-m`, `-F`, `-C` or `--amend`; :363-366 then appends the
wrapper's three trailers. Git's behaviour, probed on this machine in a
temporary repository: a message whose last paragraph is `Machine: m0b`
committed with `--trailer 'Machine: m0b+human'` records BOTH lines (the
42-commit shape of F4); a message whose last paragraph is `machine:
same` committed with `--trailer 'Machine: same'` records ONE line,
proving git matches trailer keys case-insensitively; `git
interpret-trailers --parse` prints exactly the trailers of the block
git recognises, preserving the key's case as written, and prints
nothing for a last paragraph that is prose mentioning `Machine:`. The
block rule, from `man git-interpret-trailers`: "a group of one or more
lines that (i) is all trailers, or (ii) contains at least one
Git-generated or user-configured trailer and consists of at least 25%
trailers. The group must be preceded by one or more empty (or
whitespace-only) lines. The group must either be at the end of the
input or be the last non-whitespace lines before a line that starts
with ---"; `--parse` is "a convenience alias for --only-trailers
--only-input --unfold"; "there can be no whitespace before or inside
the <key>".

## 2. The branch rule

The wrapper branches on the engine's `path` field, never on the epoch.

| Reported class | Verdict `path` | Wrapper behaviour |
| --- | --- | --- |
| HUMAN | `human` | Unchanged: `lease run-held` without an epoch, `__lease-held human`, `agent_commit=0`, landing verdict stamped observe-only. |
| MAIN, holder, claim sweep complete | `agent` | Unchanged mechanics: `lease run-held --expected-epoch <claimEpoch>`, `__lease-held <claimEpoch>`, `agent_commit=1`, the promoted codes refuse at :319. |
| MAIN, not the holder / sweep incomplete / epoch moved | (error) | The verb exits non-zero with today's exact messages (OWNED-ELSEWHERE verbs.go:409-413; "checkout lease claim sweep is incomplete" :398; "checkout lease claim epoch changed before the final mutation" :401); the wrapper's existing `|| exit $?` relays. Unchanged. |
| DELEGATE in a RUNNING dispatched job's custody, committing in that job's own worktree (section 3, the worker rule) | `worker` | `lease run-held` without an epoch (RunHeld passes DELEGATE today, F2; today's worker commits already run under it), `__lease-held worker`, `agent_commit=0`: the landing bar never refuses, its verdict is still stamped. Every proof the human path runs also runs (section 3). `--push` is refused. |
| DELEGATE anywhere else; ADAPTER-SUPERVISOR; SUPERVISION; STEWARD; UNTRUSTED | `refused` | The wrapper prints the verdict's `message` to stderr verbatim and exits 2. No lock, no token, no proof runs. |
| classification error (unreadable state, leaked fixture) | (error) | Verb exits non-zero, wrapper `|| exit $?`. Unchanged fail-closed. |
| empty or unknown `path` | — | Wrapper prints `commit refused: the engine returned no commit-authority verdict` and exits 2. Fail closed, never default to human. |

Exit code 2 is the wrapper's existing "commit refused:" family
(commit.sh:48-49, :56-57, :107-108, :362).

The refusal sentence, fixed here so the fixture can match it
byte-for-byte, is the goal record's recorded shape with the class
substituted:

```
commit refused: caller is <CLASS>, not a person or an announced main; run metasystem up from the session (steward armed) or commit from a human terminal
```

The brief paraphrases the same lawful path ("run metasystem up from
the session with the steward armed, or commit from a person's
terminal"); the goal text is the recorded one and is used. A DELEGATE
outside its own worktree receives exactly this sentence with
`DELEGATE`; the verdict does not say why the worker rule failed, because
the lawful path for that caller is the same sentence's.

Both halves of the wrapper consult the engine. The outer half (today's
:8-18) decides the path from the verdict: `agent` execs run-held with
`--expected-epoch <claimEpoch>` and the numeric token (a non-numeric
`claimEpoch` on an `agent` verdict refuses with the no-verdict sentence
above); `human` execs run-held with token `human`; `worker` execs
run-held with token `worker`; `refused` prints the message. The inner
half (today's :23-31) asks again, with `--expected-epoch <token>` for a
numeric token and without one for `human` and `worker`, and then runs
these checks in this order, every one before the token is minted (:98)
and before any proof:

1. Path match: the verdict's `path` must match the token it was
   re-entered with (`agent` for a numeric token, `human` for `human`,
   `worker` for `worker`). A `refused` verdict prints the message and
   exits 2 exactly as the outer half does; a mismatch prints `commit
   refused: the lease-held re-entry token does not match the caller's
   commit authority` and exits 2.
2. Lineage: an accepted verdict with an empty `lineage` prints `commit
   refused: the commit-authority verdict carries no lineage` and exits
   2.
3. Push rule: after the flag parse (:33-37), a `worker` token with
   `--push` prints `commit refused: a worker commit never lands;
   landings ride land.sh --chain from the main checkout` and exits 2.
   The push leg is the landing leg ("The landing is both remotes or it
   is not a landing", commit.sh:378-380, and the transport mirror
   :390-398), and a worker commit is not a landing. land.sh never
   passes `--push` (land.sh:244-252), so the landing driver is
   unaffected.
4. Trailer monopoly (TBCC-R2-AMBIGUOUS-MACHINE-LINEAGE). The wrapper
   owns three trailer keys: `Machine`, `Landing-Provenance`,
   `Landing-Provenance-Verdict` (:363-365). It reads the message it is
   about to commit and refuses when any of the three already appears:
   - Readable message sources are `-m <text>` / `--message=<text>`
     (repeatable) and `-F <path>` / `--file=<path>` with a real path.
     Any other message source refuses with `commit refused: the
     wrapper stamps only a message it can read (-m or -F <path>);
     <option> is not supported`, exit 2, naming the option as given:
     `-F -`, `--file=-`, `-C`, `-c`, `--reuse-message=`,
     `--reedit-message=`, `--fixup`, `--squash`, `--amend`, `-t`,
     `--template=`, `-e`, `--edit`, and an invocation that supplies no
     message at all (git would open an editor). The wrapper cannot
     prove the monopoly over bytes it cannot read, so it refuses them.
     land.sh passes `-F <file>` (:245); the beds and the consumer-wiring
     test pass `-m`; no shipped caller uses a refused form.
   - The message bytes are the `-F` file's bytes, or the `-m` texts
     joined by one blank line in argument order (git's own joining of
     repeated `-m`: `man git-commit`, "If multiple -m options are given,
     their values are concatenated as separate paragraphs"). The
     wrapper pipes them to `git interpret-trailers
     --parse` (F15: git's own block rule, no reimplementation), takes
     each output line's key (the text before the first `:`), lowercases
     it, and compares byte-exactly with `machine`, `landing-provenance`
     and `landing-provenance-verdict`. The same comparison is applied to
     every caller-supplied `--trailer <arg>` / `--trailer=<arg>` in the
     arguments (key = the text before the first `:` or `=`, lowercased):
     a caller trailer is another way to put a wrapper-owned key into the
     message. Lowercasing is exactly git's case-insensitive key matching
     (F15); byte-exact means `Machine-Id` or `Machine ` is not
     wrapper-owned (git itself refuses whitespace inside a key, F15).
   - The refusal: `commit refused: the message already carries a
     wrapper-owned trailer (<Name>); the wrapper stamps it`, exit 2,
     where `<Name>` is the canonical spelling (`Machine`,
     `Landing-Provenance` or `Landing-Provenance-Verdict`) whatever
     case the author typed, so the sentence is byte-exact for the
     fixture. A wrapper-owned key mentioned in prose is not a trailer
     (F15's probe) and passes.
   - Closure for hooks: a repository's own `prepare-commit-msg` or
     `commit-msg` hook can add a trailer after this check. The
     wrapper's existing postcondition (:367-376) gains one more test:
     `git log -1 --format=%B | git interpret-trailers --parse` must
     contain each wrapper-owned key exactly once (case-insensitively);
     otherwise the same soft rollback (:369-373) and the monopoly
     sentence naming the key, exit 1 like the postcondition family
     (:374-375). Index and worktree stay untouched.
5. Token mint (:98) and the proofs, unchanged.

This ordering closes the direct `commit.sh __lease-held human` entry for
a worker or a stray delegate: today that entry re-checks nothing but an
exit status (:30), and the fixture bed shows agents know the spelling
(F6). The wrapper composes no policy sentence of its own; the
wrapper-side messages are consistency checks between its own token, its
own trailers and the engine's verdict, plus the one flag rule.

Why the inner re-check agrees with the outer one: the inner wrapper is
a child of `lease run-held`, which the classifier treats as transparent
plumbing (classify.go:299-302, :347-356; tests
internal/lease/classify_test.go:492-521 and :545-568), so a worker's
walk reaches the same adapter-signed ancestor and a person's child
inherits the controlling terminal. A person under `setsid` loses it and
is UNTRUSTED (classify.go:371-373); such a caller is already refused at
today's :30 (RequireHolderAt :375-378 or :390-391), so this is not a
regression.

## 3. The engine surface: `lease commit-authority` and `lease job-worktree`

Decision: one new lease verb for commit authority, not a flag on
require-holder, and the refusal is minted in Go. Reasons: (i) Wido's
standard, behaviour enforced in the engine, the wrapper relays a typed
verdict; (ii) require-holder's wire shape is pinned and parsed by three
shell gates and three Go callers (F5, section 4), so a semantic flag on
it would change a contract every one of them would then have to re-run
under R-18; a new verb changes nobody's contract but commit.sh's; (iii)
the commit-authority answer differs from the run-held answer by design
(section 4), so it must not share gateHolder.

CLI: `metasystem lease commit-authority --root <root> --caller-pid
<pid> [--expected-epoch <n>]`, wired in cmd/metasystem/lease.go beside
runLeaseRequireHolder (:88-103) with the same `optionalEpoch` handling
(:18-27), and registered in the lease verb table at
cmd/metasystem/main.go:468-470 with the one-line description "decide
whether the caller may commit and on which path".

Go: `lease.CommitAuthorityAt(root, metasystemRoot string, callerPid
int64, expectedEpoch *int64) (CommitAuthority, error)` in
internal/lease/verbs.go, with `CommitAuthority(root, callerPid,
expectedEpoch)` delegating to it exactly as RequireHolder does
(:353-355). The result struct, field order alphabetical because that is
the wire order every typed verb result keeps (F5):

```go
// CommitAuthority is the commit gate's verdict: which path a caller may
// commit on, or the refusal the wrapper relays verbatim.
type CommitAuthority struct {
	ClaimEpoch *int64  `json:"claimEpoch"`          // agent only, else null
	Class      string  `json:"class"`               // the classification's class
	Code       string  `json:"code,omitempty"`      // "caller-class-refused" on refusal
	JobId      string  `json:"jobId,omitempty"`     // worker only: the custody-joined job
	Lineage    string  `json:"lineage,omitempty"`   // "human" | the lease's lineage | "delegate:<jobId>"
	MainId     *string `json:"mainId"`              // agent only, else null
	Message    string  `json:"message,omitempty"`   // the refusal sentence
	Path       string  `json:"path"`                // "human" | "agent" | "worker" | "refused"
}
```

Semantics, by classification (ClassifyAt, classify.go:315):

- HUMAN: `{claimEpoch:null, class:"HUMAN", lineage:"human",
  mainId:null, path:"human"}`, nil error. No lease is read.
- MAIN: run the MAIN branch of RequireHolderAt (verbs.go:371-406:
  claim if unheld, holder match, stampComplete, expected-epoch match)
  and return its errors unchanged. On success:
  `{claimEpoch:<lease.ClaimEpoch>, class:"MAIN", lineage:<leaseLineage(lease)>,
  mainId:<lease.HolderMainId>, path:"agent"}`. `leaseLineage` is
  lease.go:138-143: the announcement's ownerLineage when present (a
  mission host, launch.go:474-479), else the holder's mainId (a
  coordinator seat: on m1b, artifacts/agents/mains/worktree-lease.json
  carries `ownerLineage` equal to `holderMainId`).
- DELEGATE: apply the worker rule below. When it holds:
  `{claimEpoch:null, class:"DELEGATE", jobId:<J.jobId>,
  lineage:"delegate:<J.jobId>", mainId:null, path:"worker"}`. When any
  step fails: the refusal below with `DELEGATE`. `expectedEpoch` is
  ignored for this class, as RequireHolderAt ignores it for HUMAN.
- DELEGATE outside the worker rule, ADAPTER-SUPERVISOR, SUPERVISION,
  STEWARD, UNTRUSTED: `{claimEpoch:null, class:<class>,
  code:"caller-class-refused", message:<the section 2 sentence with
  <CLASS> substituted>, mainId:null, path:"refused"}`, nil error, exit 0.
  A refusal is a verdict, not a failure; the wrapper reads `path`,
  mirroring how it already consumes `landing observe`
  (commit.sh:306-318).
- Any ClassifyAt error: returned as the error, exit 1, message on
  stderr (the runLease* pattern, lease.go:96-100). A git plumbing
  failure inside the worker rule (the root is not a repository, a
  probe cannot answer) is likewise an error, never a silent refusal:
  the verdict must be explainable from stderr.

The worker rule (DELEGATE only), every step mechanical, evaluated in
this order, the first failing step refusing:

1. Geometry (F12), the shared function `jobWorktreeGeometry` below.
   From `root`, run under `ScrubbedEnviron`: `rev-parse --show-toplevel`
   → T; `rev-parse --path-format=absolute --git-common-dir` → C;
   `rev-parse --show-prefix` → P. Canonicalise T and dir(C) with
   `filepath.EvalSymlinks`. Main root M := dir(C) joined with P (P may
   be empty). The root is a job worktree exactly when
   dir(T) == M/artifacts/agents/worktrees (dispatch.sh:37, :43); then
   the worktree's chain root R := base(T). A delegate in the main
   checkout has dir(T) == dir(C), which fails this step.
2. Identity. p := identity.Pid (classify.go:345); s := StartedAt(p,
   probe) (the same call the walk makes, :357); c := ProcessCommand(p,
   probe) (:343). An unreadable start refuses.
3. Custody join (F9, F10; the census's own join, run.go:296-311). Read
   every record under M/artifacts/agents/jobs/*.json with the reader
   classify.go:253-284 already has, extended by `status`, `launchMode`,
   `workspaceRoot`, `parentJob` and `instanceTag`. The record J is the
   one whose own (pid, pidStartedAt) or one of whose custodyProcesses
   entries equals (p, s). No record, or more than one, refuses. A
   corrupt or unreadable record refuses the whole read exactly as
   custodyIdentities does (:263, :274): silently dropping custody is the
   fail-open the classifier already forbids.
4. Agreement, all required: J.launchMode == "worktree";
   canonical(J.workspaceRoot) == T; J.status == "running" EXACTLY, the
   literal of the transition table (record.go:41), the same literal
   custody.go:41 compares (TBCC-R2-NONRUNNING-WORKER: `pending`,
   `pending-setup`, an empty status and an unknown status are all
   refused even when custody is present, because custody is accepted
   while pending, F9; dispatch exports no constant for the literal and
   record.go:49-51 reserves only the terminal set, which this rule no
   longer consults, so lease imports nothing from dispatch); the chain
   root of J, reached by following `parentJob` until null with the walk
   bounded by the record count, equals R (the root record may itself be
   terminal, as the live `completed` root beside the `running` follow-up
   shows, F9; the RUNNING job is J); and, when c contains any record's
   `instanceTag`, that record is J (a foreign tag refuses; no tag at all
   is not a disagreement, because the Devin ACP server carries none,
   F11).
5. Verdict: `path:"worker"`, `jobId:J.jobId`, `lineage:"delegate:"+J.jobId`.

Why custody first and the tag second: the tag is absent from one lawful
runtime shape (F11), so it cannot be the primary join; the (pid, start)
custody join is present for every runtime before the delegate's first
turn (F10) and is the same identity the census and the classifier's
own ADAPTER-SUPERVISOR branch already trust. The tag is kept as a
cross-check because it costs one substring test and catches a record
mix-up between two jobs with recycled pids. Why the location must
agree: a worker with a shell can `cd` into the main checkout or a
sibling worktree; the custody join alone would still say "worker", and
the goal's word is that a worker never commits on the human branch.
Why `running` and not "not terminal": a pending job has a record and
may have custody, but no delegate turn of it is executing; a process
claiming to be that job's worker is not one.

Which proofs run on the worker path: all of them, unchanged from the
path a worker takes today (F8): the coverage delta (commit.sh:115-122),
the fast gate that builds the proof engine from the worktree's bytes
(:154-173), the index-closure proofs (:187-254), the audit (:269-274),
the settled-tree re-proof (:275-286), the landing observation stamped
observe-only (:297-318, with `agent_commit=0` so :319 never refuses),
the postcondition rollback (:367-376, now also the trailer count of
section 2) and the non-fatal weight bookkeeping (:405-408). Nothing is
skipped: the proofs bind the commit to the proved index, which is the
property conformance's later snapshot relies on, and a second proof set
for one wrapper would be the dual primary path design-principles.md:39
forbids. The only authority-bearing check is the promoted landing
refusal, and that is what "ungated by the landing bar" removes.

The second verb, `lease job-worktree` (TBCC-R2-LAND-SIDE-DOOR). CLI:
`metasystem lease job-worktree --root <root>`, wired beside the other
lease verbs and registered in the same table, description "report
whether the root is a dispatched job's worktree". Go:
`lease.JobWorktree(root string) (JobWorktreeView, error)`, which runs
step 1's `jobWorktreeGeometry` and nothing else:

```go
// JobWorktreeView is the geometry verdict the landing driver consults:
// a root under <main>/artifacts/agents/worktrees is a job worktree.
type JobWorktreeView struct {
	ChainRoot   *string `json:"chainRoot"`   // the worktree's directory name, else null
	JobWorktree bool    `json:"jobWorktree"`
	MainRoot    string  `json:"mainRoot"`
}
```

Exit 0 with the view for both answers; a plumbing failure (not a
repository, a probe that cannot answer) is an error, exit 1, message on
stderr. Decision, engine verb rather than three `rev-parse` lines in
land.sh: the geometry rule then has ONE owner (design-principles.md:23,
"one clear owner and one obvious home"; :25, entrypoints "must not
rebuild collaborator decisions inline"), the symlink canonicalisation
lives in Go once, the pure table test covers both consumers, and land.sh
already depends on the engine through the wrapper it invokes
(commit.sh:5) while the landing bed copies the engine into every leg
(land-fixtures.sh:45), so no new precondition appears. land.sh gains,
immediately after its root resolution (:12-13) and before option
parsing, so that a worker is refused whatever flags it passes:

```bash
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
geometry=$("$ms" lease job-worktree --root "$root") \
  || { echo "land refused: the checkout geometry cannot be proven (lease job-worktree failed)" >&2; exit 2; }
case "$("$ms" json get --value "$geometry" --field jobWorktree --default "")" in
  true)  echo "land refused: this is a job worktree; landings ride land.sh --chain from the main checkout" >&2; exit 2 ;;
  false) ;;
  *)     echo "land refused: the checkout geometry cannot be proven (lease job-worktree failed)" >&2; exit 2 ;;
esac
```

`json get` prints a boolean as `true`/`false` (jsonedit.go:119-120);
any other answer is fail-closed. With this, "a worker commit never
lands" holds mechanically: the wrapper refuses `--push` on the worker
path (section 2), and the only other publisher, land.sh, refuses to run
where the worker path can occur. `commit.sh` keeps its own copy of the
prefix and toplevel probes (:141-142) for the proof projection; those
serve a different rule and are untouched.

The MAIN branch is shared code with RequireHolderAt, not a second copy:
extract verbs.go:371-406 into one unexported helper that takes the
already-computed Classification and returns HolderView, called by both
verbs, so the three error strings have one home and the classification
walk runs once per call. The helper's name is the implementer's; its
boundary (input Classification, output HolderView plus error, the
lease object needed for lineage) is fixed here. The worker rule lives in
its own unexported function in internal/lease taking (root,
metasystemRoot, Classification, probe) and returning (jobId string,
ok bool, err error); `jobWorktreeGeometry` is a pure function of the
three strings (T, C, P) returning (mainRoot, chainRoot, isJobWorktree)
so it can be table-tested without git, and one thin wrapper runs the
three probes and feeds it, shared by the worker rule and the verb.

Wire-shape pin: TestVerbResultWireShapes gains one case per path with
the expected bytes, for example the human verdict
`{"claimEpoch":null,"class":"HUMAN","lineage":"human","mainId":null,"path":"human"}`,
the worker verdict
`{"claimEpoch":null,"class":"DELEGATE","jobId":"job-1","lineage":"delegate:job-1","mainId":null,"path":"worker"}`,
and the two geometry answers
`{"chainRoot":"job-1","jobWorktree":true,"mainRoot":"/m"}` and
`{"chainRoot":null,"jobWorktree":false,"mainRoot":"/m"}`.

## 4. Run-held's and require-holder's callers (R-18)

R-18 (memory/rulings.md:43): a changed contract runs its callers. This
slice changes two callers' contracts, commit.sh's and land.sh's, and
leaves `require-holder` and `run-held` byte-identical. The enumeration
below is the mechanical proof that nothing else moves, and records per
caller whether a DELEGATE-classified caller is lawful there.

| Caller | Verb | DELEGATE today | Lawful there? | This slice |
| --- | --- | --- | --- | --- |
| scripts/agents/commit.sh:9 (outer), :26 and :30 (inner), run from the MAIN checkout | require-holder | passes, no epoch, takes the human path | No: a worker never commits on the human branch (R-46-m1b) | Replaced by `lease commit-authority`; DELEGATE refused (worker rule step 1 fails). |
| scripts/agents/commit.sh, the same lines, run from the DELEGATE's OWN job worktree while its job is `running` | require-holder | passes, takes the human path with every proof (F8) | Yes: the anticipated worktree commit (F8) | `worker` path: same proofs, no landing refusal, trailer names the job, `--push` refused. |
| scripts/agents/commit.sh, run from ANOTHER job's worktree, or a worktree whose chain has no `running` job (pending, terminal, empty or unknown status) | require-holder | passes, takes the human path | No | Refused (steps 3-4). |
| scripts/agents/commit.sh:14, :17 | run-held | reached via the human path | Reached by `agent`, `human` and `worker` verdicts only | Unchanged call shape; gateHolder's DELEGATE pass (F2) is what lets the worker path run under it. |
| scripts/agents/land.sh:12-13 (new call), then :252 into commit.sh | `lease job-worktree`, then the wrapper | land.sh runs from a worktree and publishes the agent branch (F14) | No: a landing rides from the main checkout | land.sh refuses inside a job worktree before any step (section 3); from the main checkout a delegate is still refused at the wrapper. land.sh's own fetch, rebase and push from the main checkout are unchanged. |
| scripts/agents/evidence-gc.sh:21, :36, :40 | require-holder | passes, takes the script's own "human" re-entry (:28-29) | Pass-through by today's contract; not a commit; not this slice's question | Unchanged. Recorded as a residual for the goal's owner, not touched. |
| scripts/agents/evidence-gc.sh:25, :28 | run-held | passes (gateHolder :481-483) | as above | Unchanged. |
| scripts/agents/dispatch.sh:239 (lease_entry_check, called from dispatch_job :1261, follow_up :1700, cancel_job :2049, close_chain :2078, reap_jobs :2135) | require-holder | passes with empty epoch | The lease gate is not the authority: every internal entry runs `internal_authority` and Authorize refuses DELEGATE (F7). So a delegate dispatching, closing or cancelling is refused today, by the authority matrix, and stays refused. | Unchanged. |
| scripts/agents/dispatch.sh:256, :259 (lease_run_held, called at :893, :905, :910 await_handshake; :1602, :1609, :1619 dispatch_job; :1857, :1868 follow_up; :2059, :2062 cancel_job; :2092, :2112 close_chain; :2137, :2139 reap_jobs) | run-held | passes (gateHolder) | DELEGATE: no lawful caller here. ADAPTER-SUPERVISOR and SUPERVISION ARE lawful helpers under run-held (the adapter's own job record writes and the standing reaper's transitions, Authorize :97-111), which is why gateHolder must keep passing them. | Unchanged; gateHolder untouched. |
| cmd/metasystem/lease.go:96, :134 | require-holder / run-held CLI wiring | n/a | n/a | Unchanged; the two new verbs sit beside them. |
| internal/up/up.go:594 (Shutdown, parent pid) | RequireHolderAt | passes `Holder:false`, then the shutdown proceeds | Administrative cleanup verb; pass-through by today's contract | Unchanged; recorded as a residual, not touched. |
| internal/missionrunner/loop.go:2257, :2272 (reclaimCheckout) | RequireHolder | tests `Class == "HOLDER"` exactly | A runner is a MAIN after Announce; a DELEGATE answer fails the equality and the reclaim reports itself | Unchanged. |
| scripts/agents/pre-commit-guard.sh:47 (and :58, :63 in a worktree) | classify | any non-HUMAN class requires the wrapper token (:57-66) | In the main checkout: unchanged relation, the wrapper mints the token (:98) only after the verdict. In a job worktree: the guard looks for the token under the MAIN root while the wrapper mints it under the worktree (F13) | Unchanged file; the worktree geometry is the section 9 gap. |
| scripts/agents/static-reproof-fixtures.sh:29, :223, :403; internal/behaviorsurface/consumer_wiring_test.go:71 | stub `require-holder` → `{}` | n/a (stubs) | n/a | Stubs gain the section 7 contract; the `{}` line may stay or go, the wrapper no longer calls require-holder. |
| scripts/agents/land-fixtures.sh:41, :46 | copies the real land.sh; writes its own fake commit.sh | n/a | n/a | The copied land.sh carries the new refusal; the bed gains the `job-worktree` scenario (section 7.5). The fake commit.sh is unaffected. |

Commit authority and run-held authority are distinct answers and stay
distinct: run-held is a lock plus a holder gate for helpers of an
already-authorised operation (verbs.go:478-479); commit authority is
"may this class commit here, and on which path". Collapsing them would
either refuse the reaper's and adapter's lawful run-held entries or let
those classes commit. Neither is acceptable, so gateHolder is not
edited.

## 5. The human proof stays; the residual out of scope

The human path stays ungated by design. HUMAN is decided by "no
recognised ancestor and a controlling terminal" (classify.go:303-307,
:368-370); a HUMAN verdict yields `path:"human"`, `agent_commit=0`, and
every landing verdict is stamped observe-only (commit.sh:288-291,
internal/landing/observe.go:59-61, audit C8). Wido's own commits are
never gated by the agents' bar; this slice changes nothing about that.
The trailer monopoly (section 2, check 4) applies to every path,
including the human one: it is the wrapper owning its own stamp, not a
bar on the person; a person who wants a different Machine line edits
the enrolled nickname (commit.sh:360-362), not the message.

Residual, named and out of scope: a person who launches an
unrecognised agent binary from a terminal (one whose command matches
no adapter signature under scripts/agents/adapters/) is classified
HUMAN by that terminal and commits sovereign. Closing it needs a
positive human proof rather than an absence-of-agent proof, which is
not this slice and not this goal's leg. The critic is asked not to
re-litigate it here.

## 6. The Machine trailer's suffix, and the wrapper's monopoly on it

Yes: derive the suffix from the verdict, not the environment. The
inner half reads `lineage` from its own commit-authority verdict and
stamps `Machine: <nickname>+<lineage>`; `METASYSTEM_OWNER_LINEAGE` is
no longer read by commit.sh (its other readers, host.go:299,
goalsync_mutations.go:40-43, dispatch.sh:541, are untouched). An empty
`lineage` on an accepted verdict refuses with `commit refused: the
commit-authority verdict carries no lineage` and exit 2 (section 2),
never defaults to "human".

Values: HUMAN → `human`; MAIN → `leaseLineage(lease)` (section 3),
which reproduces today's correct stamps (a seat with the variable
exported stamps its mainId, F4; a mission host stamps its mission
lineage because the announcement carries it) and corrects the wrong
ones (a seat without the variable, F4); DELEGATE on the worker path →
`delegate:<jobId>`, where jobId is the custody-joined record's own id
(the running job, a follow-up's `-rN` id when the commit is made in a
follow-up round), so a worker commit reads for example `Machine:
m1b+delegate:implementer-178d269e0852ac7a8e897657-r2` and never
`+human`. The engine sets the worker lineage in step 5 of the worker
rule; the wrapper never composes it. Reason: same seam, one field on a
verdict the wrapper already holds, and the trailer is the provenance
the after-the-fact sweep (audit D5) will read; a trailer that can claim
a person for an agent commit defeats that sweep. The chosen spelling is
the brief's fixed form.

The monopoly, and its fleet consequence (TBCC-R2-AMBIGUOUS-MACHINE-
LINEAGE). A derived suffix is worthless if a second Machine line sits
above it: git keeps both (F15), and a reader taking the first trailer
reads the borrowed identity. So the wrapper refuses any message that
already carries a wrapper-owned trailer (section 2, check 4) and rolls
back a commit into which a hook injected one. Stated plainly for the
fleet: from the day this slice lands, a landing message that hand-types
`Machine: <nickname>` (or `Landing-Provenance:` or
`Landing-Provenance-Verdict:`, in any letter case) refuses with `commit
refused: the message already carries a wrapper-owned trailer (Machine);
the wrapper stamps it`. Forty-two of today's commits carry such a line
(F4, F15), all from one seat's landing template whose body ends with
`Machine: m0b`; that template must drop the line. The receipt of the
landing that ships this slice announces the refusal and names the
template, so no seat meets it as a surprise. A goal or session line in
the body (`Goal-Item: …`, `Landed by …`) is not wrapper-owned and is
untouched.

Leg 10 of the bed (static-reproof-fixtures.sh:611-617) matches only the
nickname prefix and keeps passing; the new legs match the full suffix.

## 7. Fixtures and tests

### 7.1 The bed's stub contract (scripts/agents/static-reproof-fixtures.sh)

The stubbed bed has two stub engines (:220-252, :400-424) and the
real-observer bed one (:26-35, which intercepts lease verbs and execs
the real engine for everything else, so its `json get` is real). All
three gain the same four answers; nothing else in them changes.

`lease commit-authority`, answer selection in this order:

1. If `STATIC_REPROOF_COMMIT_AUTHORITY` is set: print it verbatim to
   stdout; if `STATIC_REPROOF_COMMIT_AUTHORITY_STDERR` is set, print it
   to stderr; exit `${STATIC_REPROOF_COMMIT_AUTHORITY_EXIT:-0}`.
2. Else, if the arguments contain `--expected-epoch <N>`: print
   `{"claimEpoch":N,"class":"MAIN","lineage":"main-1-1-abcdef","mainId":"main-1-1-abcdef","path":"agent"}`.
3. Else: print
   `{"claimEpoch":null,"class":"HUMAN","lineage":"human","mainId":null,"path":"human"}`.

Rules 2 and 3 are the default-verdict rule: every existing
`__lease-held 1` leg re-enters the inner half with `--expected-epoch 1`
and receives an `agent` verdict whose epoch matches its token, and every
existing `__lease-held human` leg receives a `human` verdict, so each
legacy leg keeps exactly the `agent_commit` value it has today and its
assertions are unchanged. The three variables live in the bed, are
exported per leg and unset after it; the wrapper reads none of them,
and the escape scan (:192-198) reads the wrapper only.

`json get` (stubbed beds only): parse `--value`, `--field` and
`--default` from the arguments. When the value contains the substring
`"path":` it is a commit-authority verdict, and the answer is: a
string field `"<field>":"<x>"` prints x; a number field `"<field>":<n>`
prints n; a null or absent field prints the `--default` value when one
was given, else exits 1 with nothing printed. That is the real verb's
contract (cmd/metasystem/json.go:82, "value to print when the field is
missing or null (exit 0)"; internal/jsonedit/jsonedit.go:85-125), applied
to the seven flat fields the wrapper asks for: `path`, `claimEpoch`,
`mainId`, `lineage`, `message`, `code`, `jobId`. The verdict JSON the
bed stages never contains an escaped quote, so the regex extractor is
exact. When the value is not a verdict, the existing landing-field
branch (:227-242, :407-413, with its promotion-base-unreadable sniff)
answers as today.

`lease commit-token`: when `STATIC_REPROOF_TOKEN_MARKER` is set, create
that file (the `STATIC_REPROOF_POLICY_ENGINE_MARKER` pattern, :257);
otherwise no-op as today. The marker, not the token path, is the
discriminator: the wrapper removes its own token on exit (:99), so the
token's absence after a run proves nothing. A leg that must not mint a
token deletes the marker before the run and asserts it absent after; a
concluding leg asserts it present so absence is meaningful.

`lease run-held`: drop arguments up to and including `--`, then `exec`
the remainder in place. The bed has no concurrent writer, so no lock is
taken.

The wrapper's new `json get` calls all pass `--default ""`, so a
missing field reaches the wrapper as the empty string and the
fail-closed branches of section 2 fire on it (the real verb prints the
default for a missing or null field, json.go:82). The trailer-monopoly
check uses `git interpret-trailers`, which the bed's real git provides;
no stub is involved.

### 7.2 Shell legs, by name (scripts/agents/static-reproof-fixtures.sh)

New legs enter at the TOP of the wrapper (no `__lease-held`) unless the
entry point is stated. "HEAD unchanged" means `git rev-parse HEAD`
before and after are equal and the staged change is still in the index.

Accepting paths:

- `human commits with no landing gate`: override the human verdict of
  rule 3 explicitly; a staged README with no declaration concludes; the
  message carries `Machine: fixture-machine+human` and
  `Landing-Provenance-Verdict: would-refuse code=missing-declaration`;
  the token marker is present.
- `main with an epoch is gated`: override
  `{"claimEpoch":1,"class":"MAIN","lineage":"main-1-1-abcdef","mainId":"main-1-1-abcdef","path":"agent"}`
  in the real-observer bed with the promotion record installed
  (:86-90): the undeclared README refuses with `would-refuse
  code=missing-declaration`, exit 1, HEAD unchanged; the same verdict
  with `--chain fixture-chain` concludes (chain-record-unreadable is not
  promoted) and stamps `Machine: fixture-machine+main-1-1-abcdef`.
- `worker commits in its own worktree ungated`: override
  `{"claimEpoch":null,"class":"DELEGATE","jobId":"implementer-fixture","lineage":"delegate:implementer-fixture","mainId":null,"path":"worker"}`
  in the real-observer bed with the promotion record installed: the
  undeclared README CONCLUDES (no landing refusal), the message carries
  `Machine: fixture-machine+delegate:implementer-fixture` and
  `Landing-Provenance-Verdict: would-refuse code=missing-declaration`,
  the token marker is present, and the policy-engine marker (:257)
  proves the proofs ran.
- `wrapper-owned key in prose is not a trailer`: human verdict,
  `-m $'subject\n\nMachine: m0b is mentioned in prose here\nand this line is not a trailer'`
  concludes with exactly one `Machine:` line in the landed message, the
  wrapper's (F15's probe: git recognises no trailer block there). This
  is the positive control that the check uses git's block rule, not a
  substring scan.

Refusing paths, each asserting exit code, stderr bytes, HEAD unchanged
and the token marker absent:

- `delegate is refused with the exact message`: override
  `{"claimEpoch":null,"class":"DELEGATE","code":"caller-class-refused","mainId":null,"message":"<section 2 sentence with DELEGATE>","path":"refused"}`;
  exit 2; stderr equals the sentence byte-for-byte.
- `delegate cannot re-enter as human`: the same override, entering at
  `__lease-held human`; exit 2; the same sentence.
- `worker cannot push`: the worker override above with `--push`; exit
  2; stderr equals `commit refused: a worker commit never lands;
  landings ride land.sh --chain from the main checkout`.
- `empty or unknown path refuses`: two runs, override `{"class":"HUMAN"}`
  (no path) and `{"class":"HUMAN","path":"sideways"}` (unknown path);
  exit 2 each; stderr equals `commit refused: the engine returned no
  commit-authority verdict`.
- `accepted verdict with empty lineage refuses`: two runs, override
  `{"claimEpoch":null,"class":"HUMAN","mainId":null,"path":"human"}`
  entering at `__lease-held human`, and
  `{"claimEpoch":1,"class":"MAIN","mainId":"main-1-1-abcdef","path":"agent"}`
  entering at `__lease-held 1`; exit 2 each; stderr equals `commit
  refused: the commit-authority verdict carries no lineage`.
- `agent verdict while re-entering as human refuses`: override the
  `agent` verdict with claimEpoch 1 and lineage `main-1-1-abcdef`,
  entering at `__lease-held human`; exit 2; stderr equals `commit
  refused: the lease-held re-entry token does not match the caller's
  commit authority`.
- `hand-typed wrapper trailer refuses` (TBCC-R2-AMBIGUOUS-MACHINE-
  LINEAGE), human verdict, three runs: (a) `-m $'subject\n\nMachine: fixture-machine'`;
  (b) `-F <file>` whose last paragraph is `landing-provenance: none change=0`
  (lowercase key); (c) `--trailer "Landing-Provenance-Verdict: pass" -m subject`;
  exit 2 each; stderr equals `commit refused: the message already
  carries a wrapper-owned trailer (Machine); the wrapper stamps it`,
  respectively with `Landing-Provenance` and `Landing-Provenance-Verdict`.
- `unreadable message source refuses`: human verdict, `-C HEAD`; exit
  2; stderr equals `commit refused: the wrapper stamps only a message
  it can read (-m or -F <path>); -C is not supported`.
- `hook-injected wrapper trailer rolls back`: the bed installs
  `.git/hooks/commit-msg` appending `Machine: injected` to its argument
  file; human verdict, `-m subject`; exit 1; stderr contains the
  monopoly sentence with `Machine`; HEAD unchanged (rolled back) and
  `git write-tree` equals the pre-run staged tree (the postcondition's
  own preservation rule, :497-500). The hook is removed after the leg.
- `claim epoch changed before the final mutation is unchanged`:
  entering at `__lease-held 1` with `STATIC_REPROOF_COMMIT_AUTHORITY_EXIT=1`
  and `STATIC_REPROOF_COMMIT_AUTHORITY_STDERR="checkout lease claim
  epoch changed before the final mutation"` (stdout empty); the wrapper
  exits 1 and stderr contains that text. This leg proves the relay; the
  check itself is Go-tested below.

Every existing leg keeps its assertions unchanged.

### 7.3 The consumer-wiring stub (internal/behaviorsurface/consumer_wiring_test.go:66-77)

The test enters at `__lease-held human` (:36), so its stub gains:
`"lease commit-authority")` printing the rule-3 human verdict; and a
`"json get")` case that, when the `--value` contains `"path":`, prints
`human` for `--field path` and for `--field lineage` and the `--default`
value (empty) for any other field, and otherwise prints nothing as
today (every landing field falls to the wrapper's defaults exactly as it
does now). No run-held stub is needed there. Its messages are plain
`-m` texts without trailers (:112, :127, :148, :173), so the monopoly
check passes them.

### 7.4 Go unit tests, package internal/lease

New file `commit_authority_test.go`, staged with the helpers the
package already has:

- `TestCommitAuthorityHumanIsUngated`: childOf plus stageTerminalFact
  (refusals_test.go:100-104 shape) → `Path=="human"`, `Lineage=="human"`,
  nil epoch.
- `TestCommitAuthorityMainReportsEpochAndLineage`: Announce self
  (classify_test.go:438-455 shape) → `Path=="agent"`, `ClaimEpoch` equals
  the lease's, `Lineage` equals the mainId; a second case announcing with
  ownerLineage `mission-x` (verbs_test.go:210-227 shape) → `Lineage==
  "mission-x"`.
- `TestCommitAuthorityRefusesChangedEpoch`: Announce self, expectedEpoch
  = claimEpoch+1 → error text exactly `checkout lease claim epoch changed
  before the final mutation`.
- `TestCommitAuthorityRefusesNonHolderMain`: announceLiveChild then
  Announce self (refusals_test.go:113-125 shape) → error contains
  `OWNED-ELSEWHERE`.
- `TestCommitAuthorityRefusesDelegateWithoutCustody`: writeDevinAdapter,
  grandchild, fake identity table with a `devin-delegate-acp acp`
  intermediate (classify_test.go:230-241 shape), no job record →
  `Path=="refused"`, `Code=="caller-class-refused"`, `Message` equals the
  section 2 sentence with `DELEGATE`.
- `TestJobWorktreeGeometry`: the pure geometry function over a table of
  (toplevel, common dir, prefix): the measured F12 shape with prefix
  `metasystem/` → main root and chain root, job worktree true; the same
  with an empty prefix; a toplevel equal to the main toplevel (a
  delegate in the main checkout) → false, chain root nil; a toplevel
  under a sibling directory → false.
- `TestJobWorktreeVerbReportsGeometry`: `git init` a main root in a
  temp dir under `gittree.ScrubbedEnviron`, `git worktree add
  <main>/artifacts/agents/worktrees/job-1`; `JobWorktree(<worktree>)` →
  `JobWorktree==true`, `*ChainRoot=="job-1"`, `MainRoot` the main root;
  `JobWorktree(<main>)` → `false`, `ChainRoot==nil`; a non-repository
  temp dir → error.
- `TestCommitAuthorityWorkerInOwnWorktree`: the same bed; write
  `<main>/artifacts/agents/jobs/job-1.json` with status `running`,
  launchMode `worktree`, workspaceRoot the worktree path, and one custody
  entry `{pid:<intermediate>, pidStartedAt:1, instanceTag:"metasystem-job-job-1-<hex>"}`;
  stage the intermediate's command as `devin-delegate-acp acp` (no tag,
  the F11 shape) through the fake identity table; call CommitAuthorityAt
  with root = the worktree → `Path=="worker"`, `JobId=="job-1"`,
  `Lineage=="delegate:job-1"`, `Class=="DELEGATE"`.
- `TestCommitAuthorityWorkerNamesTheRunningFollowUp`: the same bed with
  the root record `completed` and a follow-up record `job-1-r2`
  (`parentJob:"job-1"`, status `running`, same workspaceRoot) carrying
  the custody entry → `JobId=="job-1-r2"`, `Lineage=="delegate:job-1-r2"`.
- `TestCommitAuthorityRefusesDelegateOutsideItsWorktree`: a table over
  the same bed, each row `Path=="refused"` with the DELEGATE sentence:
  root = the main checkout; root = a second worktree `job-2` whose
  record's custody does not carry the pid; the only matching record
  with status `completed`; status `pending` (custody present, F9);
  status `pending-setup`; status `""` (empty); status `"bogus"`
  (unknown); `workspaceRoot` pointing elsewhere;
  `launchMode:"shared-checkout"`; the intermediate's command carrying
  job-2's instanceTag while custody joins job-1.
- `TestCommitAuthorityRefusesEveryNonCommitClass`: table over UNTRUSTED
  (childOf with no terminal fact in an empty root, classify.go:391),
  SUPERVISION (classify_test.go:105 staging), ADAPTER-SUPERVISOR
  (:119 staging), STEWARD (:523 staging) → each `Path=="refused"` with
  its own class in the message.
- `TestVerbResultWireShapes` (verbs_test.go:35): one added case per
  path and the two geometry answers pinning the bytes (section 3).

Coverage floor: internal/lease is ratcheted at 78.6 on macOS and 79.3
on Linux (scripts/agents/coverage-ratchet.json:39,
coverage-ratchet-linux.json:39); commit.sh:112-122 runs
coverage-delta.sh over the staged packages and refuses below the floor,
and R-18's companion clause makes every landing gate check the delta of
its touched packages. The new verbs' tests must hold internal/lease at
or above both floors; cmd/metasystem carries no numeric floor
(coverage-ratchet.json:3, "thin verb wiring exercised end to end by
the shell fixtures"), so the CLI wiring is proved by the bed legs.
The round-1 brief pointed at docs/project-rules.md "Local Invariants"
for this rule; that section (project-rules.md:38-44) does not state it,
the ratchet files and R-18 do, and they are what is cited.

### 7.5 The landing bed (scripts/agents/land-fixtures.sh)

The bed runs each scenario as an isolated child
(run_fixture_bed_scenarios, fixture-bed-scenarios.sh:32; the scenario
list and its label at land-fixtures.sh:26-27). One scenario is added,
`job-worktree`, and the label becomes "land fixtures passed (4 isolated
legs)". The scenario: `make_leg job-worktree` (the seeded checkout
commits `scripts` and `bin`, :75, so a worktree of it carries the
copied land.sh and the engine); `git -C "$leg_local" worktree add -q -b
agent/job-x "$leg_local/artifacts/agents/worktrees/job-x"` (the
dispatcher's exact shape, F8; `artifacts/` is ignored by the seed's
.gitignore :70, as in the real tree); then, from INSIDE the worktree,
`bash scripts/agents/land.sh -m "$message" --skip-transport payload.txt`
after editing `payload.txt` there. Asserts: exit 2; the output contains
the line `land refused: this is a job worktree; landings ride land.sh
--chain from the main checkout`; no `== STEP:` line appears (nothing
ran); the worktree's HEAD and origin's `main` are unchanged; the
worktree's index is still clean (land.sh staged nothing). Then the
PASS from the main checkout: from `$leg_local`, the same command with
the same edit lands: exit 0, `== STEP: commit` and `== STEP: push
origin (attempt 1 of 3)` appear, and origin's `main` equals the local
HEAD (the shape of the new-plan leg's second half, :254-266).

Shell contracts that read commit.sh's bytes and must stay green:
internal/landing/observe_test.go:630-638 requires the three literal
`landing observe` and trailer strings (untouched); static-reproof-
fixtures.sh:181-198 requires the fast-gate call before the commit and
no `METASYSTEM…(SKIP|FAST|REPROOF)` name (none added: the bed's
variables are `STATIC_REPROOF_*` and live outside the wrapper);
authority-regression-fixtures.sh:36-41 scans for the retired lease
marker (none added). observe_test.go:639-643 requires land.sh to carry
`--chain`, `--direct-fix` and `--revert-of` (untouched).

## 8. Non-goals

- scripts/agents/landing-promotion.json stays at its two codes; no
  promotion moves here.
- The never-direct-fix floor (internal/landing/observe.go:795-816) and
  the register-carriage allowlist (scripts/agents/register-carriage-
  paths.txt) are untouched; the allowlist growth is R-46-m1b's selection
  (2), the next slice, not this one.
- `lease require-holder`, `lease run-held`, gateHolder, ClassifyAt,
  HolderView's wire shape, the pre-commit guard, evidence-gc.sh,
  dispatch.sh and the adapters are not edited. land.sh gains exactly the
  geometry refusal of section 3 at its start and nothing else; its
  steps, flags and messages are unchanged.
- The residuals named in sections 4 and 5 (evidence-gc and up Shutdown
  pass-through for workers; the unrecognised-binary-from-a-terminal
  case; the hand-typed duplicate `Machine: m0b` trailer on today's
  commits, which this slice refuses going forward but does not rewrite
  in history) are recorded, not closed.
- The guard's token-path geometry in a worktree (F13) is not decided
  here; it is the section 9 gap.

## 9. Seams the design cannot see, and the open gap

- The exact factoring of RequireHolderAt's MAIN branch into the shared
  helper (section 3) and the lease object's hand-off for lineage: the
  boundary is fixed, the names are the implementer's. If extraction
  changes any of the three error strings, that is a gap, not a choice.
- The git plumbing for the geometry step: whether the implementer calls
  the three `rev-parse` forms through internal/gittree's helpers
  (gittree.go:161, :179; snapshotscope.go:461) or directly under
  `ScrubbedEnviron` is theirs; the three values, their
  canonicalisation and the equalities are fixed, and both the worker
  rule and `lease job-worktree` must call the one function.
- The job-record reader: whether the worker rule extends the inline
  struct at classify.go:265-270 or adds a second reader beside it is
  the implementer's; the fields read and the fail-closed rule on a
  corrupt record are fixed.
- The `-m` joining rule: the wrapper joins repeated `-m` texts with one
  blank line before parsing, which is how git records them as separate
  paragraphs; if the implementer finds git's joining differs for some
  input (a trailing newline inside a `-m` text), the parse must follow
  git's bytes, not the wrapper's guess, and the implementer stops.
- Staging a worktree in Go tests requires `git worktree add` to accept
  a path under the temp main root and the fixture probe
  (`fixtureProbe(metasystemRoot)`, identity.go:135-143, root-checked by
  fixtureauth) to authorise the worktree as its root; if the fixture
  authority refuses a root that is not the staged main root, the
  implementer stops and reports it.
- OPEN GAP (F13), for the orchestrator: on a machine where the
  pre-commit guard is enrolled, a worker's WRAPPED worktree commit is
  refused by the guard regardless of this design, because the guard
  reads the token under the main root while the wrapper mints it under
  the worktree. This design specifies the worker path at the engine and
  the wrapper and does not edit the guard. Two resolutions are visible:
  the guard derives its root from the committing repository
  (`--show-toplevel` plus `--show-prefix`, the wrapper's own geometry
  commit.sh:141-142, or the new `lease job-worktree` view) instead of
  its install location; or the wrapper mints the token where the guard
  looks. The first keeps the delegate's writes inside its worktree; the
  second writes into the main checkout's artifacts from a sandboxed
  delegate. The choice is not mechanically determined by the brief and
  is reported, not made.

## 10. Self-grade

- Confidence: high on the branch rule, the engine surface, the worker
  rule's identity join and the two new refusals (every seam is read in
  this tree; git's trailer semantics are probed on this machine, F15;
  the landing driver's side door is traced line by line, F14); medium
  on the bed mechanics, which now rely on one stub answering four verbs
  with a regex extractor, on the follow-up chain walk matching every
  real `parentJob` shape, and on the bed's git producing the same
  trailer parse as the fleet's (git 2.50.1 here; the block rule is old
  and stable).
- Weakest claim: that the pre-mint trailer check sees exactly the bytes
  git will parse. It does for `-m` and `-F <path>`, the only shapes any
  shipped caller uses; every other source is refused, and a hook
  injection is caught by the post-commit count and rolled back. The
  unproven part is a `commit.cleanup` or `commit.template`
  configuration on some machine that rewrites the message between the
  wrapper's read and git's parse; the post-commit count catches a
  rewrite that adds a wrapper-owned key, and a rewrite that removes one
  cannot occur (git strips comments, never trailers).
- Reject condition: reject this design if (a) any shipped adapter's
  delegate ancestry makes the classifier stop at a signed process that
  is not in the job's custody (the worker rule would refuse a lawful
  worker); (b) the inner-half re-check disagrees with the outer-half
  classification for any real launch shape other than the tty-less
  person already refused today; (c) any shipped caller of commit.sh
  supplies its message through a form the monopoly check refuses (none
  found: land.sh `-F <path>`, the beds and the consumer-wiring test
  `-m`); (d) `git interpret-trailers --parse` and `git commit
  --trailer` are found to disagree on what a trailer block is for some
  message shape (both are git's one trailer implementation, F15); or
  (e) the orchestrator resolves the F13 gap by a rule that changes where
  the wrapper mints its token, in which case section 2's "no token
  before the checks" ordering and the section 7 marker legs must be
  re-derived.
