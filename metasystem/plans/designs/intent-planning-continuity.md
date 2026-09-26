# Complete design work through intent

- Kind: design
- Id: 01M3EFDSFTKWEMSDCP1BB7TDGQ
- Status: accepted
- Goals: verbs-match-intent

Design critique: closed at round 2 on 2 fixture obligations (IP-C4 and IP-C5),
material trajectory 3 to 2. Root accepts the explicit replay lookup and capped
retry corrections below; they are bounded existing-owner changes with executable
arbiters. All five findings are joined in intent-planning-dispositions.md.
Mandatory independent Sol critique covers these fixtures plus all parent duties.
This authorizes implementation, not acceptance of unproven product behavior.

Author: root Codex. This is the next usability iteration authorized by Wido,
26 September 2026. It complements Complete tasks through intent; it does not
replace that accepted design or reopen its eight dispositioned findings.

## Why this iteration exists

The first built checkpoint proves bare help succeeds and shrinks from 72 to 44
lines, but still says read units and certified job chain. The independent source
audit also found a real missing task: assigning a design author and continuing
a design critique require internal delegation. Built `help design` exits 2.
The user's complete capability/shielding contract is not met by those routes.

Step 1: an agent requests a design, receives a draft in the project's design
home, reviews it, makes actual finding decisions, requests corrections, and
continues the same bounded critique through public commands. Replays do not
launch again or overwrite later edits. Existing proof/file waits are discoverable.
Deferred: natural-language planning, automatic finding judgment, a new model
roster, automatic design acceptance, and any general-purpose workflow engine.
The parent implementation's delivery, authority, questions and repair work remains
required and continues independently.

## Grounded owners

- `intent_work.go:679,478`: build can work from a brief without an accepted design,
  but uses UnitBuildPlan and the build roster, not the design-author lane.
- `intent_delivery.go:615-685`: design review hashes the page into its generated
  brief and always requests a fresh design-critic dispatch.
- `internal/dispatch/operation.go:14`: changed brief bytes change fresh operation
  identity; follow-up identity depends on the parent. Replaying a finished follow-up
  with the latest parent is therefore not a stable retry.
- `internal/dispatch/read_admission.go:228-252`: the read owner already locates
  critic roots by design path and holds finding-register locks. Aggregate goal
  critique admission still applies; the defect is lost chain continuity.
- `dispatch.sh:2394`: existing follow-up retains chain locking, subject refresh,
  finding registers, budget/exhaustion and runtime continuation.
- `internal/launch/launch.go:39,75`: the existing design lane accepts Kind design,
  an explicit ID, WorkingDirectory, Page and Outputs; it selects launch.design
  runtime/model. Store.Create reserves an ID; the supervisor owns process custody.
- `internal/launch/declared_outputs.go:41,161`: output custody/collection already
  exists. It removes previous declared outputs after archiving, so a live project
  design must never be passed as the delegate's disposable output.
- `scripts/agents/roles/implementer.md:3,13`: the normal implementer forbids design
  decisions and plans edits. Merely selecting mode.design on that role is not an
  author contract. Reuse the dedicated design launch lane.
- `intent_work.go` legacy wait selectors and `wait_verb.go:157`: proof-attempt and
  filesystem waits already execute through the shared wait owner.

## Public task

```
metasystem design G --brief FILE [--out FILE] [--after N]
metasystem show design --goal G [--out FILE] [--attempt N]
metasystem review design FILE [--dispositions FILE] [--after N] [--retry N]
```

`design` is one real additional intention, in focused work/agent help. It uses
the existing review admission's approved-goal and positive review-round-budget
facts, followed by the existing launch design admission. It does not acquire a
build claim or create/select a goal execution worktree. The invoking checkout is
the working and publication checkout, including when invoked in another worktree.
No approval, claim takeover or higher model tier is inferred. Design authoring
uses the existing launch.design lane's limits and measurement; it does not consume
execution attempts or start an execution claim's elapsed clock. Those launch
limits are not an aggregate spending budget, and this design does not pretend
they are or invent a new budget store. An explicit new author request is required
to spend again. Review rounds retain their separate existing aggregate admission.
The existing design-lane settings select author runtime/model. The brief supplies the design request, boundaries and
missing human decisions; the adapter adds the design-author contract, never the
normal implementer's contradictory preamble. The generated contract names the
reserved Kind design, Id, Status draft and Goals header, the single staged output,
the frozen prior draft and the instruction to stop on missing human decisions.
It forbids edits to the live document or other project records. It never approves
its own result.

When --out is omitted, use the goal's unique active draft design, or choose the
first nonhistorical KindDesign home in project.Homes order, with <goal>.md,
when none exists. A collision with anything except this goal's draft refuses
with an explicit --out choice. Multiple
records require an explicit displayed document choice. An explicit file must be
within the project's resolved design home. Existing record ID is stable; first
creation reserves a project ID. Output status is draft. An accepted or done record
is not silently rewritten: request a new draft output or the existing explicit
record lifecycle decision. No private path or job identity is required as input.

The launch owner adds one focused Design request operation beside its other
retained operations. Under a per-document lock in its existing store, bind goal,
record ID/path, public attempt N, expected document bytes, exact brief bytes,
resolved design runtime/model, and resulting launch ID. Store those facts in the
existing launch records and one per-document retained request entry, following
the named operation's .inputs/<key>/request.json precedent. This entry owns exact
request replay and the current attempt; no separate job index or workflow database.
The lock key is the canonical absolute destination path, not just the record ID:
separate worktrees have separate documents, the same resolved file shares a lock.
Freeze inputs before admission. A stable request reserves its launch ID before
spawning; a crash rejoins that same launch. Add a bounded starting-record recovery
operation in the launch owner: after the existing StartCap expires, atomically
recheck State=Starting, absent supervisor/child/process-group references and no
OutputOwnerUnproven flag under Store.Update, then mark Failed with reason
supervisor-start-unrecorded. Do not use an unlocked status read followed by m.fail.
The supervisor's existing first action claims its reference under that same
record lock and refuses terminal records before any child can spawn. Thus either
it wins the claim and recovery refuses, or recovery wins and even an already
spawned but delayed supervisor cannot start a child. Elapsed time and absent PIDs
alone are not death proof. Any recorded owner or uncertain child custody retains
the existing proof-of-death rules; this recovery cannot erase it. Replay reports
the same failed N with design G --after N, never silently relaunches N.
Same-request replay is checked before inspecting the now-mutated output document.
It rejoins terminal failures as failures. A new attempt requires the prior writer
to be proven stopped; an older retained proposal cannot publish after a newer
request has become current. `--after N` deliberately requests one
new author attempt after N, reusing a brief when appropriate; stale N refuses.
Changed inputs bind the current attempt only after identical prior requests have
been considered. Follow the parent revision request's retry/rerun semantics.

The author writes a fresh staged draft under the invoking checkout's ignored
artifacts/agents/intent-design/<record>-<attempt>/draft.md, declared as Page/Output to the existing
launch owner. It reads the project plus a frozen copy of the prior design; it
never writes the current project document as its launch output. After successful
exit, validate the retained output as the exact design record and goal with draft
status. Add a focused publication operation to internal/project, which currently owns
record parsing/home resolution but no document writer. It publishes only if
the destination still has
its bound prior bytes (or is still absent for creation), using the existing atomicfile
primitive and this document lock. Replaying after publication recognizes the identical
published result. Invalid output or an intervening user edit leaves the current
file intact and retains the proposal. `show design --goal G --attempt N` displays
that proposal; the refusal explains the user can merge it into the public document
or explicitly request another design attempt against the new version. No automatic
merge or acceptance is claimed. This is explicitly a small new record-publication operation, not a claim of an
existing writer API.

`status G` includes design work using launch-owner reads alongside named build
work; `wait G` waits for it when uniquely eligible, with normal named ambiguity.
The design command repeated while running reports the same attempt. Cancellation
uses the existing launch stop owner behind an explicit public design target,
`stop design G [--out FILE] [--attempt N]`; it never kills another document's author. When a goal has several designs,
show/stop use the same public --out FILE selector as design; choices print exact
commands. A claimed supervisor that died before its child is outside the new
unclaimed-start recovery; status offers stop design G with that document selector
through the existing cancellation owner. A complete
draft names the visible document and `review design FILE` as its continuation.

## Design review stays one bounded examination

Extend the existing dispatch read owner to select design critique state by goal
and canonical design record/path, under its existing locks. Do not scan private
job files in CLI code. Design authoring admission does not imply claim-free
critique: existing dispatch requires a claimed goal, stop authority and tier 3.
Before the first new paid critique, review checks the existing tier/budget facts
and acquires an ordinary lawful claim for an eligible authenticated agent only
when the goal is unclaimed, using the parent's acquireClaim/goal.Claim owner.
It creates no build worktree. Existing holder/epoch exceptions stay owned by
dispatch; foreign claims, quota, readiness, stop fences and lower tiers refuse
with public goal/approval/assignment remedies, never silent policy changes.
This starts the existing claim accounting; critique's existing first-slice and
aggregate budget rules remain. A retained result is rejoined before admission
of a new paid round. TestIntentDesignCritiqueAdmission proves these distinctions.
No prior review then admits the first critique through the existing dispatch. The
same immutable subject rejoins its examination and can complete through the
parent's bound disposition/full-close behavior.

A changed design with an open critique uses that same chain. Before any new
examination, every previous finding has the author's bound decision. The parent
binding uses work=design:<recordID>, attempt=<examination round>, the exact old
subject digest, root, round and return digest. It applies to the old subject/return; it is not confused with the
new design being examined. `accepted` means the author changed the design to
address it; the new independent critique decides whether that correction works.
An unresolved decision prints the complete bound template. Manual design edits
remain supported; a separate author launch is optional for correction.

`review design FILE --dispositions FILE` binds the current changed document and
prior reviewed round. Before launch the dispatch owner retains exact goal,
canonical document, prior round, decision digest, new subject digest and frozen
operation ID. The existing typed follow-up mints the child under its chain lock;
the adapter must not pre-mint a child or hold that lock across follow-up. Retain
the resulting child after the owner returns. Replay first reads that retained
child; if the response was lost, use an exposed read of the dispatch owner's
existing findOperationRecord scan to resolve the frozen operation ID. Validate
the actual retained operation/subject provenance, not just an arbitrary matching
ID. Rejoin that child in any state, including after completion or an intervening
round; never re-call follow-up merely to discover its completed result.
The structured operation-id-bound-to-another-job response supplies recordedJobId
for this same validated rejoin, not an instruction to spend again. `--after N`
is the public previous examination number when explicit selection is needed. Frozen round limit, fresh required critic context, finding
history, authority, and aggregate goal allowance all remain enforced. Multiple
legacy chains produce explicit review choices; none is selected by timestamp.
A closed/exhausted chain stays closed/exhausted and gives its existing public
budget/re-scope/recorded-decision remedy; changed bytes cannot silently buy a root.
Failed examination recovery is review design FILE --retry N. It uses the
parent's explicit failed-examination extension, including timeout/budget-cap,
only after old-process death is proved and under the same frozen round cap.
The dispatch read owner refuses a fresh root for this goal and canonical design
path while its selected chain is open, failed, capped or exhausted; the current
live-subject guard does not already do that for design subjects. The public route
rejoins it and offers --retry N when admitted, or the bounded public cap remedy.
Repeating retry N rejoins its retained child even after that child fails. Closing
a clean critique records review completion, never human acceptance of the design.

## Orientation and retained waits

Use the descriptor table for a compact root orientation showing nine starting
points: goals, open, approve, build, review, land, status, ask, check. Other commands
remain fully public in the four focused topics and audience/all views. Root rows
show the verb and a short outcome summary, without full flag grammars. The existing
three-command delivery example stays. Command help carries complete syntax and
examples. This changes discovery, not capability or mutation semantics.

Rewrite ordinary summaries and continuations in task language: goal, work,
design, review, findings, tests, delivery, question, decision. Internal units,
chains, state stores and collection mechanics cannot be required concepts.
Add public `wait proof ATTEMPT [--timeout DURATION]` and `wait file PATH
[--until present|absent] [--timeout DURATION]`; default file event is present.
Resolve a relative file path against the invoking working directory. Public
status G and test/landing results expose the actual proof reference and the
continuation wait proof REF while proof is running. Read it through the proof
owner's goal/candidate observation, not CLI scans of private files. Ambiguous
proofs list their visible references; no timestamp guessing. The wait fixture
must obtain REF from public output, never private fixture knowledge. These forms
directly compose existing proof/path observers and durable wait behavior.
Legacy flags remain compatible; registration/custody are not extra caller steps.
Update AGENTS and ordinary orchestration/skill examples to the working public
routes, leaving maintainer protocol explanations explicitly diagnostic.

## Proof and critique

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| IP-1 | CRITICAL | Public task | Correct design roster; exact retained retry; no overwritten user document | Launch design request plus project record writer | internal/launch and internal/project; intent design adapter | TestIntentDesignAuthorJourney; TestIntentDesignAdmissionNoClaim; TestDesignRequestReplay; TestDesignRequestCrashBeforeSupervisor; TestDesignPublicationConflict | Public design through fake provider and actual retention/publication owners | MISSING | Implement and drive |
| IP-2 | CRITICAL | Bounded examination | Same critic chain, frozen old findings/new subject, replay and cap | Dispatch read/follow-up owners | internal/dispatch; intent design review | TestIntentDesignCritiqueAdmission; TestIntentDesignCritiqueContinuation; TestDesignCritiqueReplayAndCap | Two design versions, decision, one follow-up, replay, exhausted refusal | MISSING | Implement and drive |
| IP-3 | HIGH | Public task | Status, wait, stop and failed-result recovery need only goal/document | Existing launch observation/cancellation | intent status/wait/stop adapters | TestIntentDesignLifecycle | Running, ambiguous, failed and stopped author outcomes | MISSING | Implement and drive |
| IP-4 | HIGH | Orientation and waits | Compact task language; proof/file waits discoverable and owner-driven | Descriptors and wait owner | intent help/wait plus agent docs | TestIntentPublicOrientation; TestIntentPublicProofAndFileWait | Built CLI help and real isolated proof/file observers | MISSING | Implement and drive |

Fable critiques this newly observed requirement gap in at most two rounds, with
round 2 declared as failsafe before round 1. This is a new user-authorized usability
iteration, not another round on the parent's already-folded C1-C8. Root adjudicates.
Opus then implements; mandatory independent Sol code critique covers the combined
implementation and all parent/follow-on fixtures. Completion still requires the
parent's full capability audit and repeated real public journeys.

Final named fixture details:

- IP-C4 / TestDesignCritiqueReplayAndCap: completed follow-up replay with retained
  child; lost response with operation-to-job lookup; moved goal revision. Every
  replay rejoins the authentic same child, launches nothing, and cannot adopt a
  mismatched subject. A stored-record fixture covers an intervening foreign round;
  the real design owner freezes the cap at two, so a third round after completed
  round two is not a lawful runtime scenario and must not raise that cap for proof.
- IP-C5 / same fixture plus actual follow-up owner: failed and timeout/budget-cap
  critic offer public --retry; old live process refuses; original cap is enforced;
  no fresh root for the same goal/document; repeated failed retry rejoins.
- TestIntentDesignCritiqueAdmission: author from a second checkout writes no
  execution claim, then review lawfully claims an eligible goal without making a
  build worktree; foreign holder/lower tier/budget refusal starts no critic.
- TestDesignRequestCrashBeforeSupervisor: both winners of the locked starting
  transition, a delayed Supervise after recovery, and uncertain writer custody.
- TestIntentDesignLifecycle includes explicit document choice and dead claimed
  supervisor recovery. Sol checks actual subject refresh on follow-up and the
  supervisor command's refused-claim exit, which Fable did not certify.
