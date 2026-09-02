# Two bars for changes — the caller-class slice (design)

- Goal: two-bars-for-changes (plans/goals/two-bars-for-changes.md,
  revision 26, the paragraph "NEW SLICE, THE REFUSE BIT BINDS ON THE
  CALLER'S CLASS")
- Wido's word: ruling R-46-m1b, selection (4), verbatim: the
  "two-bars caller-class slice" is m1b's first claim, so that "the commit
  gate branches on the caller's class so a worker-classified session can
  no longer commit on the human branch" (memory/rulings.md:92)
- Hole: records/misc/design-gate-audit-2026-09-02.md, section C1
- Base tree read: dd45f392 (this worktree's HEAD, equal to origin/main)
- Mode: design only; no code in this change
- Author: implementer job implementer-178d269e0852ac7a8e897657, design
  mode, for dispatch delegate m1b+main-1788333346-60696-6a3256

## 0. The whole slice in one paragraph

Today `scripts/agents/commit.sh` decides "human or agent" from whether
the lease verb returned a claim epoch, and the lease verb returns no
epoch and no error for a worker-classified caller, so an unannounced
agent session commits on the sovereign human path where no landing
refusal fires. This slice adds one engine verb, `lease
commit-authority`, that classifies the caller and returns a typed
verdict: HUMAN takes the human path unchanged; an announced MAIN that
holds the lease takes the agent path with its claim epoch; every other
class (DELEGATE, ADAPTER-SUPERVISOR, SUPERVISION, STEWARD, UNTRUSTED)
is refused with one fixed sentence naming the class and the lawful
path. The wrapper relays that verdict on both of its halves and derives
the Machine trailer's suffix from the same verdict, so an agent landing
can no longer be stamped "+human". `lease require-holder` and `lease
run-held` keep their contracts byte-for-byte; the commit-authority
answer is a distinct surface because run-held must go on passing the
supervision and adapter helpers that reap and handshake under it.

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
signature makes the caller DELEGATE (:343-346). The comment block
:293-307 states the terminal rule; the check itself is :368-370 (a
controlling terminal) with the fixture-staged terminal fact :375-379,
and the fall-through is UNTRUSTED (:391). The audit proved the shape
live: from an unannounced Claude session, require-holder answered
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
:468, :488, :502, :522, :581, :600, :608, :631), and the bed's stub
engine answers `lease require-holder` with `{}` (:29, :223, :403).
internal/behaviorsurface/consumer_wiring_test.go:36 and :71 do the
same. Lines 8-18 of the wrapper are therefore never exercised by a
fixture today, and the inner half's re-check at :30 accepts `{}`.

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

## 2. The branch rule

The wrapper branches on the engine's `path` field, never on the epoch.

| Reported class | Verdict `path` | Wrapper behaviour |
| --- | --- | --- |
| HUMAN | `human` | Unchanged: `lease run-held` without an epoch, `__lease-held human`, `agent_commit=0`, landing verdict stamped observe-only. |
| MAIN, holder, claim sweep complete | `agent` | Unchanged mechanics: `lease run-held --expected-epoch <claimEpoch>`, `__lease-held <claimEpoch>`, `agent_commit=1`, the promoted codes refuse at :319. |
| MAIN, not the holder / sweep incomplete / epoch moved | (error) | The verb exits non-zero with today's exact messages (OWNED-ELSEWHERE verbs.go:409-413; "checkout lease claim sweep is incomplete" :398; "checkout lease claim epoch changed before the final mutation" :401); the wrapper's existing `|| exit $?` relays. Unchanged. |
| DELEGATE, ADAPTER-SUPERVISOR, SUPERVISION, STEWARD, UNTRUSTED | `refused` | The wrapper prints the verdict's `message` to stderr verbatim and exits 2. No lock, no token, no proof runs. |
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
terminal"); the goal text is the recorded one and is used.

Both halves of the wrapper consult the engine. The outer half (today's
:8-18) decides the path from the verdict. The inner half (today's
:23-31) asks again, with `--expected-epoch <token>` for a numeric token
and without one for `human`, and requires the verdict's `path` to match
the token it was re-entered with: `agent` for a numeric token, `human`
for `human`. A `refused` verdict on the inner half prints the message
and exits 2 exactly as the outer half does; a mismatch prints `commit
refused: the lease-held re-entry token does not match the caller's
commit authority` and exits 2. This closes the direct
`commit.sh __lease-held human` entry for a worker: today that entry
re-checks nothing but an exit status (:30), and the fixture bed shows
agents know the spelling (F6). The wrapper composes no policy sentence
of its own; the two wrapper-side messages above are consistency checks
between its own token and the engine's verdict.

Why the inner re-check agrees with the outer one: the inner wrapper is
a child of `lease run-held`, which the classifier treats as transparent
plumbing (classify.go:299-302, :347-356; tests
internal/lease/classify_test.go:492-521 and :545-568), and a person's
child inherits the controlling terminal. A person under `setsid` loses
it and is UNTRUSTED (classify.go:371-373); such a caller is already
refused at today's :30 (RequireHolderAt :375-378 or :390-391), so this
is not a regression.

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
	Lineage    string  `json:"lineage,omitempty"`   // "human" | the lease's lineage
	MainId     *string `json:"mainId"`              // agent only, else null
	Message    string  `json:"message,omitempty"`   // the refusal sentence
	Path       string  `json:"path"`                // "human" | "agent" | "refused"
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
- DELEGATE, ADAPTER-SUPERVISOR, SUPERVISION, STEWARD, UNTRUSTED:
  `{claimEpoch:null, class:<class>, code:"caller-class-refused",
  message:<the section 2 sentence with <CLASS> substituted>, mainId:null,
  path:"refused"}`, nil error, exit 0. A refusal is a verdict, not a
  failure; the wrapper reads `path`, mirroring how it already consumes
  `landing observe` (commit.sh:306-318).
- Any ClassifyAt error: returned as the error, exit 1, message on
  stderr (the runLease* pattern, lease.go:96-100).

The MAIN branch is shared code with RequireHolderAt, not a second copy:
extract verbs.go:371-406 into one unexported helper that takes the
already-computed Classification and returns HolderView, called by both
verbs, so the three error strings have one home and the classification
walk runs once per call. The helper's name is the implementer's; its
boundary (input Classification, output HolderView plus error, the
lease object needed for lineage) is fixed here.

Wire-shape pin: TestVerbResultWireShapes gains one case per path with
the expected bytes, for example the human verdict
`{"claimEpoch":null,"class":"HUMAN","lineage":"human","mainId":null,"path":"human"}`.

## 4. Run-held's and require-holder's callers (R-18)

R-18 (memory/rulings.md:43): a changed contract runs its callers. This
slice changes exactly one caller's contract, commit.sh's, and leaves
`require-holder` and `run-held` byte-identical. The enumeration below
is the mechanical proof that nothing else moves, and records per caller
whether a DELEGATE-classified caller is lawful there.

| Caller | Verb | DELEGATE today | Lawful there? | This slice |
| --- | --- | --- | --- | --- |
| scripts/agents/commit.sh:9 (outer), :26 and :30 (inner) | require-holder | passes, no epoch, takes the human path | No: a worker never commits on the human branch (R-46-m1b) | Replaced by `lease commit-authority`; DELEGATE refused. |
| scripts/agents/commit.sh:14, :17 | run-held | reached via the human path | Never reached after the branch (only `agent` and `human` verdicts exec it) | Unchanged call shape. |
| scripts/agents/evidence-gc.sh:21, :36, :40 | require-holder | passes, takes the script's own "human" re-entry (:28-29) | Pass-through by today's contract; not a commit; not this slice's question | Unchanged. Recorded as a residual for the goal's owner, not touched. |
| scripts/agents/evidence-gc.sh:25, :28 | run-held | passes (gateHolder :481-483) | as above | Unchanged. |
| scripts/agents/dispatch.sh:239 (lease_entry_check, called from dispatch_job :1261, follow_up :1700, cancel_job :2049, close_chain :2078, reap_jobs :2135) | require-holder | passes with empty epoch | The lease gate is not the authority: every internal entry runs `internal_authority` and Authorize refuses DELEGATE (F7). So a delegate dispatching, closing or cancelling is refused today, by the authority matrix, and stays refused. | Unchanged. |
| scripts/agents/dispatch.sh:256, :259 (lease_run_held, called at :893, :905, :910 await_handshake; :1602, :1609, :1619 dispatch_job; :1857, :1868 follow_up; :2059, :2062 cancel_job; :2092, :2112 close_chain; :2137, :2139 reap_jobs) | run-held | passes (gateHolder) | DELEGATE: no lawful caller here. ADAPTER-SUPERVISOR and SUPERVISION ARE lawful helpers under run-held (the adapter's own job record writes and the standing reaper's transitions, Authorize :97-111), which is why gateHolder must keep passing them. | Unchanged; gateHolder untouched. |
| cmd/metasystem/lease.go:96, :134 | require-holder / run-held CLI wiring | n/a | n/a | Unchanged; the new verb sits beside them. |
| internal/up/up.go:594 (Shutdown, parent pid) | RequireHolderAt | passes `Holder:false`, then the shutdown proceeds | Administrative cleanup verb; pass-through by today's contract | Unchanged; recorded as a residual, not touched. |
| internal/missionrunner/loop.go:2257, :2272 (reclaimCheckout) | RequireHolder | tests `Class == "HOLDER"` exactly | A runner is a MAIN after Announce; a DELEGATE answer fails the equality and the reclaim reports itself | Unchanged. |
| scripts/agents/pre-commit-guard.sh:47 | classify | any non-HUMAN class requires the wrapper token (:57-66) | Unchanged relation: the wrapper still mints the token (:98) only after the verdict | Unchanged. |
| scripts/agents/static-reproof-fixtures.sh:29, :223, :403; internal/behaviorsurface/consumer_wiring_test.go:71 | stub `require-holder` → `{}` | n/a (stubs) | n/a | Stubs gain a `lease commit-authority` answer (section 7); the `{}` line may stay or go, the wrapper no longer calls require-holder. |
| scripts/agents/land-fixtures.sh:46 | writes its own fake commit.sh | n/a | n/a | Unaffected. |
| scripts/agents/land.sh:252 | invokes commit.sh | inherits the branch | A delegate running land.sh is refused at the wrapper, which is the intended teeth | Unchanged file. |

Commit authority and run-held authority are distinct answers and stay
distinct: run-held is a lock plus a holder gate for helpers of an
already-authorised operation (verbs.go:478-479); commit authority is
"may this class commit at all". Collapsing them would either refuse the
reaper's and adapter's lawful run-held entries or let those classes
commit. Neither is acceptable, so gateHolder is not edited.

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
`lineage` on a `human` or `agent` verdict refuses with `commit
refused: the commit-authority verdict carries no lineage` and exit 2,
never defaults to "human".

Values: HUMAN → `human`; MAIN → `leaseLineage(lease)` (section 3),
which reproduces today's correct stamps (a seat with the variable
exported stamps its mainId, F4; a mission host stamps its mission
lineage because the announcement carries it) and corrects the wrong
ones (a seat without the variable, F4). Reason: same seam, one extra
field on a verdict the wrapper already holds, and the trailer is the
provenance the after-the-fact sweep (audit D5) will read; a trailer
that can claim a person for an agent landing defeats that sweep.

Leg 10 of the bed (static-reproof-fixtures.sh:611-617) matches only the
nickname prefix and keeps passing; the new legs match the full suffix.

## 7. Fixtures and tests

Shell bed, scripts/agents/static-reproof-fixtures.sh. The stub engine
(:220-252 and :400-424) gains two answers: `lease commit-authority`
returns the JSON in `STATIC_REPROOF_COMMIT_AUTHORITY` when that variable
is set, else an `agent` verdict with the `--expected-epoch` value when
the flag is present, else a `human` verdict (so every existing
`__lease-held 1` and `__lease-held human` leg keeps its meaning); `lease
run-held` executes the argv after `--` in place. The variable lives in
the bed's stub, not the wrapper; the escape scan (:192-198) reads the
wrapper only and the wrapper reads no new environment variable. New
legs enter at the TOP of the wrapper (no `__lease-held`), named:

- `human commits with no landing gate`: verdict `{class:HUMAN,
  path:human, lineage:human}`; a staged README with no declaration
  concludes; the message carries `Machine: fixture-machine+human` and
  `Landing-Provenance-Verdict: would-refuse code=missing-declaration`.
- `main with an epoch is gated`: verdict `{class:MAIN, path:agent,
  claimEpoch:1, mainId:main-1-1-abcdef, lineage:main-1-1-abcdef}` in the
  real-observer bed with the promotion record installed (:86-90): the
  undeclared README refuses with `would-refuse code=missing-declaration`
  and HEAD is unchanged; the same verdict with `--chain fixture-chain`
  concludes (chain-record-unreadable is not promoted) and stamps
  `Machine: fixture-machine+main-1-1-abcdef`.
- `delegate is refused with the exact message`: verdict
  `{class:DELEGATE, path:refused, code:caller-class-refused,
  message:<section 2 sentence with DELEGATE>}`: exit 2, stderr equals
  the sentence byte-for-byte, HEAD unchanged, the index still holds the
  staged change, no wrapper token file was created.
- `delegate cannot re-enter as human`: the same verdict, entering at
  `__lease-held human`: exit 2 with the same sentence, HEAD unchanged.
- `claim epoch changed before the final mutation is unchanged`:
  entering at `__lease-held 1` while the stub answers
  `commit-authority --expected-epoch 1` with exit 1 and stderr
  `checkout lease claim epoch changed before the final mutation`: the
  wrapper exits non-zero with that text. This leg proves the relay;
  the check itself is Go-tested below.
- Every existing leg keeps its assertions unchanged.

internal/behaviorsurface/consumer_wiring_test.go:66-77: the stub gains
the same `lease commit-authority` answer (a `human` verdict, since the
test enters at `__lease-held human`, :36).

Go unit tests, package internal/lease, new file
`commit_authority_test.go`, staged with the helpers the package already
has:

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
- `TestCommitAuthorityRefusesDelegate`: writeDevinAdapter, grandchild,
  fake identity table with a `devin-delegate-acp acp` intermediate
  (classify_test.go:230-241 shape) → `Path=="refused"`,
  `Code=="caller-class-refused"`, `Message` equals the section 2
  sentence with `DELEGATE`.
- `TestCommitAuthorityRefusesEveryNonCommitClass`: table over UNTRUSTED
  (childOf with no terminal fact in an empty root, classify.go:391),
  SUPERVISION (classify_test.go:105 staging), ADAPTER-SUPERVISOR
  (:119 staging), STEWARD (:523 staging) → each `Path=="refused"` with
  its own class in the message.
- `TestVerbResultWireShapes` (verbs_test.go:35): one added case per
  path pinning the bytes.

Coverage floor: internal/lease is ratcheted at 78.6 on macOS and 79.3
on Linux (scripts/agents/coverage-ratchet.json:39,
coverage-ratchet-linux.json:39); commit.sh:112-122 runs
coverage-delta.sh over the staged packages and refuses below the floor,
and R-18's companion clause makes every landing gate check the delta of
its touched packages. The new verb's tests must hold internal/lease at
or above both floors; cmd/metasystem carries no numeric floor
(coverage-ratchet.json:3, "thin verb wiring exercised end to end by
the shell fixtures"), so the CLI wiring is proved by the bed legs.
The brief pointed at docs/project-rules.md "Local Invariants" for this
rule; that section (project-rules.md:38-44) does not state it, the
ratchet files and R-18 do, and they are what is cited.

Shell contracts that read commit.sh's bytes and must stay green:
internal/landing/observe_test.go:630-638 requires the three literal
`landing observe` and trailer strings (untouched); static-reproof-
fixtures.sh:181-198 requires the fast-gate call before the commit and
no `METASYSTEM…(SKIP|FAST|REPROOF)` name (none added);
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
  HolderView's wire shape, the pre-commit guard, evidence-gc.sh and
  dispatch.sh are not edited.
- The residuals named in sections 4 and 5 (evidence-gc and up Shutdown
  pass-through for workers; the unrecognised-binary-from-a-terminal
  case; the hand-typed duplicate `Machine: m0b` trailer on today's m0b
  commits) are recorded, not closed.

## 9. Seams the design cannot see

- The exact factoring of RequireHolderAt's MAIN branch into the shared
  helper (section 3) and the lease object's hand-off for lineage: the
  boundary is fixed, the names are the implementer's. If extraction
  changes any of the three error strings, that is a gap, not a choice.
- `json get --default ""` on a missing `path` or `lineage` field is
  assumed to yield the empty string as it does for `claimEpoch`
  (commit.sh:10-12); the wrapper's fail-closed branches rely on it. If
  the helper errors instead, the wrapper must treat the error as the
  same refusal.
- Whether the bed's `lease run-held` stub can `exec` without a lock is
  assumed yes (a bed has no concurrent writer); if the wrapper's
  `--expected-epoch` re-entry depends on run-held's lock in the bed,
  the implementer stops.

## 10. Self-grade

- Confidence: high on the branch rule and the engine surface (every
  seam cited is read in this tree and the live DELEGATE answer is on
  record); medium on the bed mechanics, which rely on one stub
  answering two verbs and on the `--expected-epoch` heuristic keeping
  the interleaved legacy legs meaningful.
- Weakest claim: that no rostered flow has a DELEGATE-classified caller
  committing through commit.sh lawfully today (a worker committing in
  its own worktree). Evidence for the claim: roles/implementer.md has no
  commit instruction, conformance reads the working tree, and no script
  outside land.sh, the beds and two Go tests invokes commit.sh; but a
  worker following "prefer a temporary WIP commit" guidance through the
  wrapper would be refused after this slice and must use plain `git
  commit` in its worktree.
- Reject condition: reject this design if the critic or implementer
  finds a lawful DELEGATE commit path through commit.sh (the fix is a
  worktree-scoped rule, which reopens design), or if the inner-half
  re-check disagrees with the outer-half classification for any real
  launch shape other than the tty-less person already refused today.
