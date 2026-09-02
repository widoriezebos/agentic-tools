# Two bars for changes — the caller-class slice (design, revision 2)

- Goal: two-bars-for-changes (plans/goals/two-bars-for-changes.md,
  revision 26, the paragraph "NEW SLICE, THE REFUSE BIT BINDS ON THE
  CALLER'S CLASS")
- Wido's word: ruling R-46-m1b, selection (4), verbatim: the
  "two-bars caller-class slice" is m1b's first claim, so that "the commit
  gate branches on the caller's class so a worker-classified session can
  no longer commit on the human branch" (memory/rulings.md:92)
- Hole: records/misc/design-gate-audit-2026-09-02.md, section C1
- Base tree read: ffb84aae (this worktree's HEAD; the code seams cited
  are unchanged since dd45f392, the revision 1 base)
- Mode: design only; no code in this change
- Author: implementer job implementer-178d269e0852ac7a8e897657 (round 2
  as implementer-178d269e0852ac7a8e897657-r2), design mode, for dispatch
  delegate m1b+main-1788333346-60696-6a3256
- Revision 2 changelog: folds the three accepted round-1 findings of
  critic chain two-bars-cc-crit-3 (plans/two-bars-caller-class-dispositions.md):
  TBCC-R1-LAWFUL-DELEGATE-COMMIT-PATH (a fourth verdict path, `worker`,
  sections 1-4, 6, 7, 10), TBCC-R1-FIXTURE-STUB-CONTRACT (the bed's stub
  contract, section 7), TBCC-R1-NEGATIVE-BRANCH-PROOFS (three negative
  legs, section 7).

## 0. The whole slice in one paragraph

Today `scripts/agents/commit.sh` decides "human or agent" from whether
the lease verb returned a claim epoch, and the lease verb returns no
epoch and no error for a worker-classified caller, so an unannounced
agent session commits on the sovereign human path where no landing
refusal fires. This slice adds one engine verb, `lease
commit-authority`, that classifies the caller and returns a typed
verdict with four paths: HUMAN takes the human path unchanged; an
announced MAIN that holds the lease takes the agent path with its claim
epoch; a DELEGATE whose runtime process is in a dispatched job's custody
and whose repository is that job's own worktree takes the `worker` path,
ungated by the landing bar because a worktree commit never lands; every
other caller (a DELEGATE anywhere else, ADAPTER-SUPERVISOR, SUPERVISION,
STEWARD, UNTRUSTED) is refused with one fixed sentence naming the class
and the lawful path. The wrapper relays that verdict on both of its
halves and derives the Machine trailer's suffix from the same verdict,
so an agent landing can no longer be stamped "+human" and a worker
commit names its job. `lease require-holder` and `lease run-held` keep
their contracts byte-for-byte; the commit-authority answer is a distinct
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
hand-typed `Machine: m0b` trailer), 5 carry `m0+human`, 2 carry
`m1b+main-1788333346-60696-6a3256`, 1 carries `m1+coordinator`. The
brief's figure of fifteen is not what this range measures; the
mechanism is the same either way: a seat without the variable says
"+human".

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
:43 and `agents="$root/artifacts/agents"` :37); the pre-commit guard
requires the wrapper token for every non-HUMAN caller
(scripts/agents/pre-commit-guard.sh:44-66, the refusal :63-66), so a
worker cannot fall back to raw `git commit`; and docs/concepts.md:104-106
states "a raw commit is refused; a wrapped one carries the wrapper's
kernel identity". Today such a commit takes the wrapper's human path
(F1, F2) with every proof the human path runs.

F9. What a job record carries. Set at dispatch: `workspaceRoot`
(internal/dispatch/build.go:389, the resolved worktree path; a
follow-up inherits its parent's, :593), `productRoots` (:646),
`launchMode` (`worktree` or `shared-checkout`, dispatch.sh:1375, :1405),
`instanceTag`, `pid`, `pidStartedAt`, `parentJob`, `status`, and
`custodyProcesses`, each entry `{pid, pidStartedAt,
pidStartedAtExactMicro, instanceTag, pgid}` written by
`__register-custody` (dispatch.sh:2496-2499; internal/dispatch/
custody.go:68-85). The terminal statuses are `completed`, `failed`,
`cancelled`, `timeout`, exported as `dispatch.TerminalStatus`
(internal/dispatch/record.go:45-52, "consumers outside dispatch must
not re-declare the set"). internal/lease already reads job records
without importing dispatch (classify.go:253-284 reads jobId, pid,
pidStartedAt, custodyProcesses into the `adapters` custody map, and
:357-364 joins a walked ancestor's (pid, start) against it). Import
directions verified with `go list -deps`: dispatch does not depend on
lease, lease does not depend on dispatch, and gittree depends on
neither, so lease may import both dispatch (for TerminalStatus) and
gittree (for git plumbing) without a cycle. Live example on this
machine: the root record implementer-178d269e0852ac7a8e897657 has
status `completed`, launchMode `worktree`, workspaceRoot equal to this
worktree's path, and one custody entry; the follow-up record
`…-r2` has status `running`, parentJob equal to the root, the SAME
workspaceRoot, and its own custody entry carrying its own tag.

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
because worktrees share the common config).

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

## 2. The branch rule

The wrapper branches on the engine's `path` field, never on the epoch.

| Reported class | Verdict `path` | Wrapper behaviour |
| --- | --- | --- |
| HUMAN | `human` | Unchanged: `lease run-held` without an epoch, `__lease-held human`, `agent_commit=0`, landing verdict stamped observe-only. |
| MAIN, holder, claim sweep complete | `agent` | Unchanged mechanics: `lease run-held --expected-epoch <claimEpoch>`, `__lease-held <claimEpoch>`, `agent_commit=1`, the promoted codes refuse at :319. |
| MAIN, not the holder / sweep incomplete / epoch moved | (error) | The verb exits non-zero with today's exact messages (OWNED-ELSEWHERE verbs.go:409-413; "checkout lease claim sweep is incomplete" :398; "checkout lease claim epoch changed before the final mutation" :401); the wrapper's existing `|| exit $?` relays. Unchanged. |
| DELEGATE in a dispatched job's custody, committing in that job's own worktree (section 3, the worker rule) | `worker` | `lease run-held` without an epoch (RunHeld passes DELEGATE today, F2; today's worker commits already run under it), `__lease-held worker`, `agent_commit=0`: the landing bar never refuses, its verdict is still stamped. Every proof the human path runs also runs (section 3). `--push` is refused. |
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
numeric token and without one for `human` and `worker`, and requires
the verdict's `path` to match the token it was re-entered with: `agent`
for a numeric token, `human` for `human`, `worker` for `worker`. A
`refused` verdict on the inner half prints the message and exits 2
exactly as the outer half does; a mismatch prints `commit refused: the
lease-held re-entry token does not match the caller's commit authority`
and exits 2; an accepted verdict with an empty `lineage` prints `commit
refused: the commit-authority verdict carries no lineage` and exits 2.
All three inner-half checks run before the wrapper mints its token
(:98) or runs any proof, so a refused re-entry leaves no token and no
proof artefact. This closes the direct `commit.sh __lease-held human`
entry for a worker or a stray delegate: today that entry re-checks
nothing but an exit status (:30), and the fixture bed shows agents know
the spelling (F6). The wrapper composes no policy sentence of its own;
the wrapper-side messages are consistency checks between its own token
and the engine's verdict, plus the one flag rule below.

`--push` on the worker path: the push leg is the landing leg ("The
landing is both remotes or it is not a landing", commit.sh:378-380,
and the transport mirror :390-398), and a worker commit is not a
landing. Immediately after the flag parse (:33-37) the inner half
refuses `commit refused: a worker commit never lands; landings ride
land.sh --chain from the main checkout` with exit 2 when the token is
`worker` and `--push` was given. land.sh never passes `--push`
(land.sh:244-252), so the landing driver is unaffected.

Why the inner re-check agrees with the outer one: the inner wrapper is
a child of `lease run-held`, which the classifier treats as transparent
plumbing (classify.go:299-302, :347-356; tests
internal/lease/classify_test.go:492-521 and :545-568), so a worker's
walk reaches the same adapter-signed ancestor and a person's child
inherits the controlling terminal. A person under `setsid` loses it and
is UNTRUSTED (classify.go:371-373); such a caller is already refused at
today's :30 (RequireHolderAt :375-378 or :390-391), so this is not a
regression.

## 3. The engine surface: `lease commit-authority`

Decision: one new lease verb, not a flag on require-holder, and the
refusal is minted in Go. Reasons: (i) Wido's standard, behaviour
enforced in the engine, the wrapper relays a typed verdict; (ii)
require-holder's wire shape is pinned and parsed by three shell gates
and three Go callers (F5, section 4), so a semantic flag on it would
change a contract every one of them would then have to re-run under
R-18; a new verb changes nobody's contract but commit.sh's; (iii) the
commit-authority answer differs from the run-held answer by design
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

1. Geometry (F12). From `root`, run under `ScrubbedEnviron`:
   `rev-parse --show-toplevel` → T; `rev-parse --path-format=absolute
   --git-common-dir` → C; `rev-parse --show-prefix` → P. Canonicalise T
   and dir(C) with `filepath.EvalSymlinks`. Main root
   M := dir(C) joined with P (P may be empty). Require
   dir(T) == M/artifacts/agents/worktrees (dispatch.sh:37, :43). The
   worktree's chain root R := base(T). A delegate in the main checkout
   has dir(T) == dir(C), which fails this step.
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
   canonical(J.workspaceRoot) == T; `!dispatch.TerminalStatus(J.status)`
   (record.go:52); the chain root of J, reached by following `parentJob`
   until null with the walk bounded by the record count, equals R
   (the root record may itself be terminal, as the live `completed`
   root beside the `running` follow-up shows, F9; the RUNNING job is J);
   and, when c contains any record's `instanceTag`, that record is J
   (a foreign tag refuses; no tag at all is not a disagreement, because
   the Devin ACP server carries none, F11).
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

Which proofs run on the worker path: all of them, unchanged from the
path a worker takes today (F8): the coverage delta (commit.sh:115-122),
the fast gate that builds the proof engine from the worktree's bytes
(:154-173), the index-closure proofs (:187-254), the audit (:269-274),
the settled-tree re-proof (:275-286), the landing observation stamped
observe-only (:297-318, with `agent_commit=0` so :319 never refuses),
the postcondition rollback (:367-376) and the non-fatal weight
bookkeeping (:405-408). Nothing is skipped: the proofs bind the commit
to the proved index, which is the property conformance's later
snapshot relies on, and a second proof set for one wrapper would be the
dual primary path design-principles.md:39 forbids. The only
authority-bearing check is the promoted landing refusal, and that is
what "ungated by the landing bar" removes.

The MAIN branch is shared code with RequireHolderAt, not a second copy:
extract verbs.go:371-406 into one unexported helper that takes the
already-computed Classification and returns HolderView, called by both
verbs, so the three error strings have one home and the classification
walk runs once per call. The helper's name is the implementer's; its
boundary (input Classification, output HolderView plus error, the
lease object needed for lineage) is fixed here. The worker rule lives in
its own unexported function in internal/lease taking (root,
metasystemRoot, Classification, probe) and returning (jobId string,
ok bool, err error); the geometry step is a pure function of the three
strings (T, C, P) so it can be table-tested without git.

Wire-shape pin: TestVerbResultWireShapes gains one case per path with
the expected bytes, for example the human verdict
`{"claimEpoch":null,"class":"HUMAN","lineage":"human","mainId":null,"path":"human"}`
and the worker verdict
`{"claimEpoch":null,"class":"DELEGATE","jobId":"job-1","lineage":"delegate:job-1","mainId":null,"path":"worker"}`.

## 4. Run-held's and require-holder's callers (R-18)

R-18 (memory/rulings.md:43): a changed contract runs its callers. This
slice changes exactly one caller's contract, commit.sh's, and leaves
`require-holder` and `run-held` byte-identical. The enumeration below
is the mechanical proof that nothing else moves, and records per caller
whether a DELEGATE-classified caller is lawful there.

| Caller | Verb | DELEGATE today | Lawful there? | This slice |
| --- | --- | --- | --- | --- |
| scripts/agents/commit.sh:9 (outer), :26 and :30 (inner), run from the MAIN checkout | require-holder | passes, no epoch, takes the human path | No: a worker never commits on the human branch (R-46-m1b) | Replaced by `lease commit-authority`; DELEGATE refused (worker rule step 1 fails). |
| scripts/agents/commit.sh, the same lines, run from the DELEGATE's OWN job worktree | require-holder | passes, takes the human path with every proof (F8) | Yes: the anticipated worktree commit (F8) | `worker` path: same proofs, no landing refusal, trailer names the job, `--push` refused. |
| scripts/agents/commit.sh, run from ANOTHER job's worktree or a worktree whose chain has no running job | require-holder | passes, takes the human path | No | Refused (steps 3-4). |
| scripts/agents/commit.sh:14, :17 | run-held | reached via the human path | Reached by `agent`, `human` and `worker` verdicts only | Unchanged call shape; gateHolder's DELEGATE pass (F2) is what lets the worker path run under it. |
| scripts/agents/evidence-gc.sh:21, :36, :40 | require-holder | passes, takes the script's own "human" re-entry (:28-29) | Pass-through by today's contract; not a commit; not this slice's question | Unchanged. Recorded as a residual for the goal's owner, not touched. |
| scripts/agents/evidence-gc.sh:25, :28 | run-held | passes (gateHolder :481-483) | as above | Unchanged. |
| scripts/agents/dispatch.sh:239 (lease_entry_check, called from dispatch_job :1261, follow_up :1700, cancel_job :2049, close_chain :2078, reap_jobs :2135) | require-holder | passes with empty epoch | The lease gate is not the authority: every internal entry runs `internal_authority` and Authorize refuses DELEGATE (F7). So a delegate dispatching, closing or cancelling is refused today, by the authority matrix, and stays refused. | Unchanged. |
| scripts/agents/dispatch.sh:256, :259 (lease_run_held, called at :893, :905, :910 await_handshake; :1602, :1609, :1619 dispatch_job; :1857, :1868 follow_up; :2059, :2062 cancel_job; :2092, :2112 close_chain; :2137, :2139 reap_jobs) | run-held | passes (gateHolder) | DELEGATE: no lawful caller here. ADAPTER-SUPERVISOR and SUPERVISION ARE lawful helpers under run-held (the adapter's own job record writes and the standing reaper's transitions, Authorize :97-111), which is why gateHolder must keep passing them. | Unchanged; gateHolder untouched. |
| cmd/metasystem/lease.go:96, :134 | require-holder / run-held CLI wiring | n/a | n/a | Unchanged; the new verb sits beside them. |
| internal/up/up.go:594 (Shutdown, parent pid) | RequireHolderAt | passes `Holder:false`, then the shutdown proceeds | Administrative cleanup verb; pass-through by today's contract | Unchanged; recorded as a residual, not touched. |
| internal/missionrunner/loop.go:2257, :2272 (reclaimCheckout) | RequireHolder | tests `Class == "HOLDER"` exactly | A runner is a MAIN after Announce; a DELEGATE answer fails the equality and the reclaim reports itself | Unchanged. |
| scripts/agents/pre-commit-guard.sh:47 (and :58, :63 in a worktree) | classify | any non-HUMAN class requires the wrapper token (:57-66) | In the main checkout: unchanged relation, the wrapper mints the token (:98) only after the verdict. In a job worktree: the guard looks for the token under the MAIN root while the wrapper mints it under the worktree (F13) | Unchanged file; the worktree geometry is the section 9 gap. |
| scripts/agents/static-reproof-fixtures.sh:29, :223, :403; internal/behaviorsurface/consumer_wiring_test.go:71 | stub `require-holder` → `{}` | n/a (stubs) | n/a | Stubs gain the section 7 contract; the `{}` line may stay or go, the wrapper no longer calls require-holder. |
| scripts/agents/land-fixtures.sh:46 | writes its own fake commit.sh | n/a | n/a | Unaffected. |
| scripts/agents/land.sh:252 | invokes commit.sh | inherits the branch | From the main checkout a delegate is refused at the wrapper, which is the intended teeth; from its own worktree its commit takes the worker path and land.sh's own fetch, rebase and push of `agent/<job>` are pre-existing behaviour outside this slice | Unchanged file. |

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

Residual, named and out of scope: a person who launches an
unrecognised agent binary from a terminal (one whose command matches
no adapter signature under scripts/agents/adapters/) is classified
HUMAN by that terminal and commits sovereign. Closing it needs a
positive human proof rather than an absence-of-agent proof, which is
not this slice and not this goal's leg. The critic is asked not to
re-litigate it here.

## 6. The Machine trailer's suffix

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
missing or null (exit 0)"; internal/jsonedit/jsonedit.go:85-99), applied
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
default for a missing or null field, json.go:82).

### 7.2 Shell legs, by name

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
- `empty or unknown path refuses` (TBCC-R1-NEGATIVE-BRANCH-PROOFS):
  two runs, override `{"class":"HUMAN"}` (no path) and
  `{"class":"HUMAN","path":"sideways"}` (unknown path); exit 2 each;
  stderr equals `commit refused: the engine returned no
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
does now). No run-held stub is needed there.

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
- `TestWorktreeGeometryDerivation`: the pure geometry function over a
  table of (toplevel, common dir, prefix): the measured F12 shape with
  prefix `metasystem/` → main root and chain root; the same with an
  empty prefix; a toplevel equal to the main toplevel (a delegate in the
  main checkout) → not a worktree; a toplevel under a sibling directory
  → not a worktree.
- `TestCommitAuthorityWorkerInOwnWorktree`: `git init` a main root in a
  temp dir under `gittree.ScrubbedEnviron`, `git worktree add
  <main>/artifacts/agents/worktrees/job-1`, write
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
  the same bed: root = the main checkout; root = a second worktree
  `job-2` whose record's custody does not carry the pid; the only
  matching record `completed`; `workspaceRoot` pointing elsewhere;
  `launchMode:"shared-checkout"`; the intermediate's command carrying
  job-2's instanceTag while custody joins job-1 → each `Path=="refused"`
  with the DELEGATE sentence.
- `TestCommitAuthorityRefusesEveryNonCommitClass`: table over UNTRUSTED
  (childOf with no terminal fact in an empty root, classify.go:391),
  SUPERVISION (classify_test.go:105 staging), ADAPTER-SUPERVISOR
  (:119 staging), STEWARD (:523 staging) → each `Path=="refused"` with
  its own class in the message.
- `TestVerbResultWireShapes` (verbs_test.go:35): one added case per
  path pinning the bytes (section 3).

Coverage floor: internal/lease is ratcheted at 78.6 on macOS and 79.3
on Linux (scripts/agents/coverage-ratchet.json:39,
coverage-ratchet-linux.json:39); commit.sh:112-122 runs
coverage-delta.sh over the staged packages and refuses below the floor,
and R-18's companion clause makes every landing gate check the delta of
its touched packages. The new verb's tests must hold internal/lease at
or above both floors; cmd/metasystem carries no numeric floor
(coverage-ratchet.json:3, "thin verb wiring exercised end to end by
the shell fixtures"), so the CLI wiring is proved by the bed legs.
The round-1 brief pointed at docs/project-rules.md "Local Invariants"
for this rule; that section (project-rules.md:38-44) does not state it,
the ratchet files and R-18 do, and they are what is cited.

Shell contracts that read commit.sh's bytes and must stay green:
internal/landing/observe_test.go:630-638 requires the three literal
`landing observe` and trailer strings (untouched); static-reproof-
fixtures.sh:181-198 requires the fast-gate call before the commit and
no `METASYSTEM…(SKIP|FAST|REPROOF)` name (none added: the bed's
variables are `STATIC_REPROOF_*` and live outside the wrapper);
authority-regression-fixtures.sh:36-41 scans for the retired lease
marker (none added).

## 8. Non-goals

- scripts/agents/landing-promotion.json stays at its two codes; no
  promotion moves here.
- The never-direct-fix floor (internal/landing/observe.go:795-816) and
  the register-carriage allowlist (scripts/agents/register-carriage-
  paths.txt) are untouched; the allowlist growth is R-46-m1b's selection
  (2), the next slice, not this one.
- `lease require-holder`, `lease run-held`, gateHolder, ClassifyAt,
  HolderView's wire shape, the pre-commit guard, evidence-gc.sh,
  dispatch.sh, land.sh and the adapters are not edited.
- The residuals named in sections 4 and 5 (evidence-gc and up Shutdown
  pass-through for workers; the unrecognised-binary-from-a-terminal
  case; the hand-typed duplicate `Machine: m0b` trailer on today's m0b
  commits; land.sh's own push of an agent branch from a worktree) are
  recorded, not closed.
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
  canonicalisation and the equalities are fixed.
- The job-record reader: whether the worker rule extends the inline
  struct at classify.go:265-270 or adds a second reader beside it is
  the implementer's; the fields read and the fail-closed rule on a
  corrupt record are fixed.
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
  commit.sh:141-142) instead of its install location; or the wrapper
  mints the token where the guard looks. The first keeps the delegate's
  writes inside its worktree; the second writes into the main
  checkout's artifacts from a sandboxed delegate. The choice is not
  mechanically determined by the brief and is reported, not made.

## 10. Self-grade

- Confidence: high on the branch rule, the engine surface and the
  worker rule's identity join (every seam is read in this tree: the
  adapters register exactly the process the classifier stops at, F10;
  the live records show the root-plus-follow-up shape the rule handles,
  F9; the geometry is measured on this worktree, F12); medium on the bed
  mechanics, which rely on one stub answering four verbs with a regex
  extractor, and on the follow-up chain walk matching every real
  `parentJob` shape.
- Weakest claim: that the custody join is present for every lawful
  worker at the moment it commits. Evidence: registration precedes the
  handshake and a failed registration terminates the child
  (runtime-common.sh:102-113; claude.sh:149; codex.sh:153; devin.sh:340,
  :356, :581), so a delegate that reaches a turn is registered. The
  unproven part is a runtime whose delegate-side process tree puts an
  unregistered signed process between the worker's shell and the
  registered one; none of the three shipped adapters does (the
  registered pid is the exec'd CLI itself, F10), and the ACP client is
  registered beside the server.
- Reject condition: reject this design if (a) any shipped adapter's
  delegate ancestry makes the classifier stop at a signed process that
  is not in the job's custody (the worker rule would refuse a lawful
  worker); (b) the inner-half re-check disagrees with the outer-half
  classification for any real launch shape other than the tty-less
  person already refused today; or (c) the orchestrator resolves the
  F13 gap by a rule that changes where the wrapper mints its token, in
  which case section 2's "no token before the verdict" ordering and the
  section 7 marker legs must be re-derived.
