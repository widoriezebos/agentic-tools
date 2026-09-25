# Commands for what the human and agent want to do

- Kind: design
- Id: 01M3CGR7CNZTS2NNTQCYRCF4JX
- Status: done
- Goals: verbs-match-intent

Review exit: closed at round 2 with zero material findings and seven named
fixture obligations. Root accepts under Wido's explicit design/implementation
authorization; independent critic Fable 5.1 agrees full scope and smallest
sufficient design. Adjudication: `plans/verbs-match-intent-design-review.md`.
Implementation exposed a full-workflow requirement failure: a built unit
could not reach the branch evidence consumed by landing. Fable confirmed
the focused connection is the smallest sufficient owner composition;
root folded its one material correction through the existing unit-amend
owner. The amended design is accepted with the named VMI-10 fixtures and
mandatory independent implementation review. Dispositions are in
`plans/verbs-match-intent-connection-review.md`.

Author: root Codex. Authority: Wido, 25 September 2026: write the full
design, have Fable or Opus 5.5 critique it, agree the smallest design that
works, then implement. Machinery may be bypassed; ignore the stop hook.
His acceptance test: "the most intuitive but also most powerful, forgiving
set of verbs" following human/agent intent. The number of verbs in the old
audit is illustrative, not a cap. Full scope is authorized; the slices
below are delivery order, not a request to shrink that scope.
Standing authority: R-126-m1e in `metasystem/memory/rulings.md`, recorded
on Wido's explicit request while implementation was starting.

## 1. The smallest complete change

Replace the public command boundary, not the engines behind it. A person
says **approve this goal**, **give it this budget**, **start this checkout**.
An agent says **build this unit**, **review this design**, **land this work**.
The command resolves the records and carries the whole mechanical sequence
to its result or to the next actual decision. It never asks the caller to
name an implementation package, copy a process identity, construct a
manifest, or remember a sequence of bookkeeping calls.

Step 1 is independently useful: the human can discover and use the goal
lifecycle through positional intent commands from any directory in the
repository. Help shows only supported arguments and runnable examples;
root ambiguity and the shared-goal flag dump disappear on this surface.
It includes `goals`, `show`, `approve`, `budget`, `pause`, `resume`, `done`,
`help`, and `internal`. It preserves existing transactions and old calls.

The complete release also supplies process control, work preparation,
build/review/fold/close/landing, questions, diagnostics and fleet views as
specified below. Step 1 defers those commands to their named slices and
existing owners; it does not advertise them as implemented. No new daemon,
workflow database, policy language, model-based command interpretation,
permission system or universal workflow engine is introduced.

## 2. Current facts and the evidence that changes this design

Read at `410a71c91`, with the live goal subsequently refreshed:

- Root help expands nearly every family and leaf; routing and help are in
  `metasystem/cmd/metasystem/main.go:778-914`. `verb` has no audience metadata.
  Goal parsing registers unrelated flags before rejecting their use
  (`goalsync_mutations.go:1090-1195`). Its `done` summary describes legacy
  flags while the synced handler takes `--id` and `--conclude` (`:1932`).
- `stateroot.ResolveLayout` already resolves descendants, files, symlinks,
  template and adopted layouts (`internal/stateroot/stateroot.go:158-212`).
  `pathFlag` only absolutizes (`cmd/metasystem/helpers.go:35-58`). In this
  session, `goal list --root .` reported legacy while `--root metasystem`
  read the actual ledger. Reuse the layout resolver, not another root search.
- `goal budget` already routes queued/approved/parked to approval and claimed
  to budget update or stopped-goal resume (`goalsync_mutations.go:1532-1680`).
  Human name defaults follow proof, not mere file existence (`:1485`). Claim
  epoch preservation is already owned by `internal/goal/verbs.go:1673-1679`.
- Dispatch already derives repeatable operation identity from goal revision,
  mode, role, brief digest and parent (`internal/dispatch/operation.go:10-37`).
  Do not recreate an operation-id service.
- `UnitRunner.Advance` owns build, proof, independent read and resumable
  rounds (`internal/launch/unit_run.go:104-177,330-404`). It ends at
  `awaiting-judgement`, even when green (`:696-701`). A standalone launch's
  process result does not certify a delegate chain (`launch/record.go:65-91`).
- Branch read freezes and locks its subject and retains dispatch/collection
  state (`internal/goal/branch/read.go:330-476`). Batch join already prepares
  transport, checks evidence, hands off the claim and wakes the landing
  owner (`cmd/metasystem/landing_batch_join.go:128-292`). Hand branch landing
  remains separately owned by `goal branch land-prep` and `land-push`.
- The existing full legacy close sequence is in
  `scripts/agents/dispatch.sh:2975-3043`; register advance is already
  idempotent (`internal/dispatch/finding_register.go:69`). A new public
  `close` must call that sequence, not just its final low-level operation.

The dated September 6 audit remains evidence, not an inventory of today's
build. The accompanying evidence record will retain concrete local Claude,
Codex and transaction specimens and its coverage limits. This refresh
already reproduced a missing-lineage refusal during `goal edit`: the
boundary should use the proven current session when available. A missing
or foreign session is a real refusal, never permission to copy another
session's identity. The old audit's requested cross-Mac sweep cannot be
claimed from local transcripts alone.

Related requirements are carried explicitly:

| Goal | Disposition in this release |
| --- | --- |
| human-goal-verbs-forgiving; verb-ergonomics | Delivered baseline: reuse compact budgets, human proof/actor filling and refusal rendering; do not claim that work anew. |
| repo-flag-resolves-one-root | Slice 1, section 5: use the existing layout resolver, not a second root authority. |
| hp-resume-takes-its-budget-from-the-ledger | Slice 1, section 6 and VMI-3: standing-budget resume with distinct parked transition. |
| up-is-the-human-start-command | Slice 2, section 6: explicit naming supersession, enrollment/rebuild and health truth retained. |
| hp-enrollment-before-migration | Slice 2 retains enrollment authority refusals; changing migration/enrollment authority is a separate owner fix, not implied permission here. |
| land-verb-pruning | Slice 3 removes the pipeline choice from ordinary use; compatibility remains internal. |
| goal-landing-needs-a-human-word; human-carried-landing | Existing human carry/landing exceptions remain explicit governed acts; `land` never grants them. Retain current owner commands as advanced internal recovery. |

Source inventories and the bounded local transcript sweep are retained in
`plans/verbs-match-intent-source-facts.md` and
`plans/verbs-match-intent-evidence-20260925.md`. The current registration
census is 44 families and 461 literal leaf verbs, plus eight special root
routes; those numbers are evidence, not a desired public command count.

## 3. Grammar and discovery

`metasystem VERB [KIND] [TARGET...] [OPTIONS]`. Most uses need one verb and
one target. Kinds are user objects (`goal`, `job`, `unit`, `design`, `commit`,
`mission`, `checkout`, `session`, `ui`), never implementation families.
Kinds are omitted when the command has one target type. A verb with several
types requires the kind unless the supplied target resolves to exactly one
supported type. A bare `stop`, `start` or `status` means this checkout.

`help` is one concise page with human and agent sections. `help human` and
`help agent` expand only their actions; `help VERB` shows the actual grammar,
defaults, advanced options and examples. `VERB --help` works without a
repository, identity or writes. The router and help share one small public
command table. The Project Partner catalogue is the union of that table
and the existing family catalogue, marked internal: append, remove nothing.
Commands whose implementation has not landed are absent from the public
table. Until the agent workflow slice lands, `help agent` points explicitly
to the still-complete internal catalogue. `internal FAMILY VERB ...` is a
pure routing alias to the existing
technical catalogue; `help internal` explains that escape hatch.

Existing family calls and existing top-level machine calls keep their
current behavior. They are compatibility entrypoints, omitted from default
help but available through `help internal` and their existing help. Default
human and agent documentation moves to the new forms. A compatibility hint
may appear on interactive stderr only; stdout, JSON, exit status and scripts
are unchanged. Removal of compatibility forms is deferred; no release date
is invented to make unconverted hooks stop working.

When a public name is also a family, a following registered family verb
selects that existing call; otherwise the public grammar applies. Preserve
old top-level flag-only calls through their existing handler; explicit
target kinds select the new grammar. The descriptors document both forms
where they overlap. Help search is deferred: the short grouped catalogue
must first demonstrate a need for it.

## 4. Complete public surface

`G`, `J`, `U`, `R`, `Q` mean goal, job, unit run, review and question identifiers.
`TEXT` is one quoted argument or supplied with the corresponding `--*-file`.
Existing advanced options remain reachable; they are documented on the
applicable intent rather than dumped onto every goal command.

| Intent and ordinary grammar | Effect and existing owner |
| --- | --- |
| `goals [--all] [--label LABEL]` | Read current backlog; `goal list` projection. `--all` includes archived goals. |
| `show G` | Goal, state, budget, next step and linked design; goal/project readers. |
| `open G --intent TEXT --next TEXT --risk ANSWERS --basis TEXT` | Create goal; existing human/agent intake law and blocker arguments apply. |
| `edit G --intent TEXT --next TEXT` | Existing edits, plus `--next-append TEXT` against accepted state; no local-file merge. Only supplied fields change. |
| `approve G... [--budget BOX]` | Human approves explicit goals; omitted BOX uses each tier's norm. Multiple goals use existing atomic approval, never a loop of partial approvals. |
| `budget G BOX` | Existing compact budget operation. BOX is `norm`, `keep`, or the complete compact tuple. Read form `budget G` prints the same budget block as `show G`, using existing ProjectConsumption/ProjectBudget projections. |
| `pause G --reason TEXT` | Existing goal park transaction. |
| `resume G` | Resume a stopped claim under its standing approved budget; unpark a parked goal through its owner. Never implicitly grant missing approval. |
| `done G --reason TEXT` | Existing conclusion and obligations checks; no automatic goal conclusion after landing. |
| `decide G --finding F --review R --reason TEXT` | Existing accept-risk act, with review root derived from R. This is a risk decision, not an arbitrary ruling writer. |
| `enroll --name NAME` | Enroll the invoking human terminal; existing local/fleet publication owner. Report partial publication accurately. |
| `start [checkout]` | Existing human arm sequence including engine refresh and supervision startup. Agent session startup is `start session`, invoking current `up` behavior. |
| `stop [checkout]`; `stop job J`; `stop session` | Existing checkout transition, exact job cancellation, or quiet session-stop owner respectively. These effects are separately described. |
| `restart checkout`; `restart ui` | Checkout stop then arm after successful stop, or existing UI restart. Failure names which state was reached; repeat converges through those owners. |
| `status [checkout]`; `status job J`; `status unit U` | Existing state readers, preserving stale/unknown and partial outcomes. |
| `claim [G]`; `release [G] --reason TEXT` | Existing claim/release. Without G, claim uses the ready frontier; release uses this session's unique held goal. Never silently switch work. |
| `brief G --out FILE` | Generate a usable brief scaffold from the goal and linked design; mark genuine missing decisions, do not invent them. |
| `build G UNIT --brief FILE --check COMMAND...` | Prepare the existing unit plan, then advance build/proof/read to judgement. `--check` ends command option parsing and passes an argv, not shell text. Advanced repeatable proof commands use existing `--plan FILE`. |
| `build --resume U`; `fold unit U --brief FILE` | Existing unit resume/follow-up; prior plan, proofs and reviews are reused by their owner. |
| `review design FILE`; `review job J`; `review commit SHA`; `review unit U` | Freeze and review the named subject through the respective owner; automatically prepare plumbing inputs and collect the result. Unit review commits and publishes the exact built result, then requests committed review. |
| `fold review R --dispositions FILE --brief FILE` | Validate recorded dispositions and request the existing implementation follow-up. Does not decide findings itself. |
| `close J --dispositions FILE` | Apply completed review records and perform the existing full close sequence, or report the unresolved finding/unfinished job. |
| `land G [--through COMMIT]`; `land job J` | Complete the applicable existing landing sequence, detailed below. No implicit conclusion. |
| `ask G --question TEXT --option TEXT... [--recommend TEXT]` | Existing channel question; advanced `--kind` and compact `--budget` select existing authority-specific questions. |
| `answer mission M Q TEXT` | Existing mission answer transition. Channel questions print the authenticated channel's reply instructions; this command never forges a channel answer. |
| `wait job J`; `wait unit U`; `wait goal G` | Existing event waits/unit continuation; print a stable continuation when the wait boundary expires. |
| `fleet [--refresh]`; `doctor` | Existing seat fleet and health diagnosis. Doctor is read-only and names the actual owner's repair command where one exists; there is no general repair coordinator. |
| `test [--goal G] [--mode auto|standard|deep]` | Existing selected test runner; `test plan`, `test verify`, `test report` retain their meaningful task grammar. |
| `settings [KEY]`; `ui [start|stop|status]` | Existing configuration read surface and UI lifecycle. Settings writes retain their own explicit operation. |

`brief`, `build` and `review` expose names describing what is produced:
design author, builder, design reviewer, code reviewer. Existing roster keys
are resolved inside the adapters. Renaming stored roster keys is unnecessary
for the usable surface and is deferred with aliases if later needed.

### Advanced goal acts and complete disposition

Keep the ordinary help short by grouping these under planning, authority
and recovery in `help human`/`help agent`. They remain first-class intents,
with per-command help. Advanced power does not require internal storage names.

| Public grammar | Existing owner / limits |
| --- | --- |
| `pin G MACHINE`; `pin G --clear`; `prioritize G 1\|2\|3` | goal set-pin / set-priority. |
| `reopen G --next TEXT`; `abandon G --reason TEXT [--successor G2]` | goal reopen / abandon; explicit successor composes carry only after abandonment is committed, with partial outcome on refusal. |
| `block G --on G2`; `unblock G --on G2` | goal block / unblock; preserve human authority for removing an unfinished dependency. |
| `unapprove G --reason TEXT` | goal unapprove; preserves parking of a standing claim. |
| `grant --tiers LIST --acts LIST --until TIME`; `revoke GRANT` | goal grant / revoke; maps acts/until to verbs/expires, exact existing authority limits. Grants are not scoped to a made-up recipient. |
| `split G --plan FILE`; `group G ARC`; `ungroup G` | goal split's existing format; set-arc / detach. |
| `claim G --take-over --reason TEXT`; `ready G` | goal steal / land-ready; displacement is explicit, never a claim fallback. |
| `edit G --obligation STATE --owner NAME --recurrence VALUE ...` | goal set-obligation's existing recurrence, observations, ceilings, effects and typed-review fields, all listed by this command's help; no new file schema. |
| `resolve G --review R --finding F --test NAME` | discharge-review-obligation; fixture-qualified alternative uses existing implementation-chain/artifact/result/critic proof fields. |
| `notes G --read R --add TEXT`; `notes G --close ID --fixed COMMIT\|--moved G2\|--accepted TEXT` | goal read-items. Findings stay findings, never certified by this convenience. |
| `recover [G]` | existing goal journal recovery and this session's durable wait recovery; reports each owner's result separately. No generic repair engine. |
| `red own ENTRY --goal G`; `red close ENTRY --reason TEXT` | existing trunk-red own/close. ENTRY is the retained incident id, not an assumed commit lookup. |

`--under GRANT` is available only where the owner supports that exact act:
approve, budget and the parked branch of resume (with `--verified TEXT`).
The stopped-claim branch refuses it. `grant --acts` maps the public spellings
approve, budget and resume-parked to approve, set-budget and unpark. All
other advanced owner inputs are enumerated on their relevant public help,
including split manifests and recurrence fields. Flags above describe the
human input; adapters convert to existing typed inputs, not a new format.

The following disposition is exhaustive for today's goal family. Rows
above cover open, abandon, carry, block, unblock, done, reopen, claim,
approve, budget, unapprove, grant, revoke, accept-risk,
discharge-review-obligation, split, set-obligation, enroll-terminal, resume,
release, steal, trunk-red, land-ready, edit, set-arc, set-pin, set-priority,
detach, list, show, and recover. `set-next` is `edit --next`; `set-budget` is
the budget command's advanced long form; `park`/`unpark` are pause/resume;
`read-items` is notes; `next` is `goals --ready`; `tier-probe` is
`goals --tiers`. `branch` operations are work preparation and land;
`handover`, `carrying`, `carried`, `restamp`, `extend-budget`, and `fetch` are internal steps
owned by those whole workflows, with diagnostic access retained.
In particular, earned extension belongs to dispatch admission with its
exact revision, cap, role and proof offer; a human budget command cannot
manufacture those dispatch facts (`goalsync_mutations.go:2849-2910`).
`engine-floor`, `classify-sweep`, `reconcile`, `migrate`, `repair` and
`source-digest` remain explicit internal maintenance: installation-wide
authority or reviewed recovery bytes, not ordinary goal execution.
`promote`, `declare-free` and `prune` are legacy-ledger maintenance and stay
internal. No existing capability is removed.

For session and top-level calls: `up` remains the compatibility agent
startup; `arm` is start checkout; stop/status have explicit target forms;
health is doctor; watch/wait retain monitoring under `wait` with their
existing advanced options. `session start` is durable recovery output and
belongs to `recover`, not startup; `session stop` is stop session;
`session end` remains runtime lifecycle cleanup. All other families retain
their full internal catalogue; the public workflow rows above own their
ordinary human/agent tasks. VMI-9's coverage fixture enumerates these
dispositions against registered commands so added or missed acts are visible.

## 5. Defaults, flags and forgiveness

The command descriptor owns its accepted flags, arity, aliases and examples.
There is no second hand-maintained help grammar. Use Go's flag value parsing
with a small interspersed-argument normalizer; no dependency or generic DSL.

| Public field | Meaning and compatibility |
| --- | --- |
| `--repo PATH` | Select a repository from any descendant path. `--root` is an accepted spelling on the public edge. |
| `--reason TEXT` | Why the action is requested; maps to the owner's `why`, `because` or `conclude` as appropriate. |
| `--budget BOX` | The compact tuple or preset, not the literal word `box`. Five existing long limit flags remain advanced alternatives; no mixing two contradictory specifications. |
| `--name NAME` / `--by NAME` | Enrollment name / explicit actor attribution. Missing `--by` is filled only after matching human proof. Strip one `human:` prefix; reject repeated prefixes with a corrected example. |
| `--goal G`, `--id ID` | Explicit equivalents of the command's target where applicable, never global selectors for unrelated stores. |
| `--brief FILE`, `--design FILE`, `--dispositions FILE` | Semantic input files. Relative paths are relative to the original working directory, not a generated artifact directory. |
| `--model MODEL`, `--effort VALUE` | Build accepts both; committed review (commit or built unit) accepts the owner-supported model override. Design/job review uses its roster, and review effort remains governed by its existing hazard class. Unsupported overrides refuse explicitly before dispatch. |
| `--timeout DURATION` | How long this invocation waits. Timeout reports continuing work and its exact continuation; it does not kill it. |
| `--json` | Existing machine-readable results or the named intent result projection. Never suppress failure status. |
| `--plan FILE` | Expert use of an existing unit/test plan; no new plan schema. |
| `--through COMMIT` | Explicit partial landing, preserving current approval rules. Omitted means the complete eligible goal unit set. |

Forgiving means: flags before or after positionals; `--flag=value`; familiar
old flag aliases where their meaning is identical; equal duplicate values
are harmless; conflicting duplicates explain both inputs before any act;
`--` preserves literal leading dashes. Unknown flags and spelling mistakes
suggest the actual accepted form. Never autocorrect a mutating command,
silently drop options, choose an arbitrary prefix match, or infer approval.
Legacy calls keep their old parsers; the new parser does not rewrite a
payload after `--check` or an explicit argument terminator.

All repository-scoped public calls resolve `Layout` once. Goal/state owners
receive the state root (`RootForInstallation`); process owners receive the
repository plus explicit installation; Git work uses `GitRoot`. Launch and
unit ids stay user-global; the repository selects work, not a second launch
store. Paths to files stay bound to the caller's directory. Missing or
ambiguous installations are errors, not an empty fresh world.

Human mutations require explicit target G. Agents may omit G only where
the grammar permits it and the live session owns exactly one matching
claim. Derive lineage through the existing verified runtime/session owner;
an enrollment or stale announcement alone is insufficient. The initial
implementation may require explicit lineage when no proven session exists
and must print the actual session-start remedy. No environment mutation is
needed: pass the resolved identity into the existing request builder.

## 6. State and whole-workflow decisions

**Goal acts.** Approve/budget reuse the current routing table. A queued or
parked budget act is explicitly reported as approval, as today. `resume`
of a stopped claim defaults to the standing budget; changing that budget
remains `budget`. A budget change refused by a stop fence must not restart
spending on the old budget as an invisible intermediate step. `resume` on
parked work calls unpark and preserves approval rules. Already-effective
acts report their current result where the underlying transaction supports
it; otherwise state why the act has no valid transition. Claim epoch,
budget episode and human proof remain domain decisions.
Approve without BOX passes nil to the existing atomic owner and reports
the recorded per-goal budget. That branch reads each goal's tier box
(`internal/goal/approval.go:589-649`); it does not invent a shared box or a
new norm approval token. VMI-3 asserts the resulting records.

**Build.** The explicit unit name gives a reusable task identity. Generate
the existing `UnitPlan` and build/read briefs beside that unit's retained
inputs, binding the current goal branch, base commit, selected design and
the caller's check argv. Reuse its current `goal/G` worktree. If absent,
prepare a separate goal worktree through the small composition below;
never move a dirty checkout or require a manual Git preparation sequence.
Use the linked design/brief's declared unit-size row for existing build
admission. If absent, require an honest `--lines N` estimate for the unit;
do not fabricate the admission input (`internal/launch/admit.go:166-255`).
The unit name selects the existing units-page row and populates unitsPage
and units; without a page, write the caller's `--lines N` into that unit's
row in the generated build brief so existing buildSize reads it. Both
absent means a missing estimate, with that explicit remedy. Generate the
read brief from `scripts/agents/templates/review-brief.md`, filling its
subject, checklist and independent read requirements; resolve read.model
through the existing `launch.read.model` setting (ReadModelKey).
Use UnitRunner for every step. Repeating the same goal/unit/input digest
resumes its existing run; changed inputs require a follow-up or a new unit
name. Put this reservation and lookup under UnitRunner's existing storage
and locking owner, before model launch; no external retry journal. Completion
uses a goal+unit keyed lock before newRun and a digest-to-run-id entry in
that same run store: concurrent identical calls must launch one run.
Successful execution
means awaiting judgement, with proof and review results named, not approved
or landed. A red proof stops review as the current runner specifies.

**Built unit to committed review.** The ordinary next action after a green
build is `review unit U`. A UnitRunner read is preliminary feedback; its
process success never constitutes accepted branch evidence. Source audit
of candidates `b78baa39d` and `032ff31a8` found that build deliberately
leaves changes uncommitted, while land consumes published Goal-Unit and
Goal-Read commits. The public boundary must connect these existing owners:

Before the first build, verify the current claim/lease through the existing
branch authority owner. Enumerate registered goal worktrees. If none exists,
use the existing endpoint and remote goal-branch resolvers to choose the
base: reuse a validated local goal branch, otherwise the existing remote
goal branch, otherwise the freshly resolved landing endpoint. Refuse
divergent local/remote goal histories through the branch owner's validation;
do not guess that the caller's HEAD is an acceptable base. Create the goal
worktree at the deterministic sibling `<checkout>-<goal>` with Git's
`worktree add`, without force or reset. A pre-existing occupied destination
is reused only when the registered worktree belongs to this repository and
the exact goal branch; otherwise report a path conflict. Concurrent creation
re-reads Git's registration after a creation failure and reuses only that
same valid worktree. The linked worktree uses the existing main-checkout
lease resolution; do not create a second session, enrollment or authority.
This new composition belongs to the public build boundary: no complete
goal-worktree preparation owner currently exists. Failed creation retains
and reports any branch/worktree created so retry can reconcile it. Resolve
selected settings from the original installation; do not copy local secret
configuration into a generated worktree.

1. Resolve the UnitRun and its latest completed round. Require successful
   proof and a retained result snapshot. Read findings remain visible for
   the author; requesting committed review does not accept those findings.
   `green`, `read-failed` and `read-compacted` may proceed when proof passed;
   `proof-red`, `proof-wrote`, running and capped runs may not.
   Bind goal, unit, worktree, original base, completed round and result tree.
   Reject a running build, a red proof, changed result bytes, or unrelated
   staged/worktree changes before staging or publication. An unchanged
   result already committed by this operation is a retry, not stale work.
2. Under the existing unit run lock, retain the intended result tree,
   round and operation identity with the run before effects. Use the
   existing claim check and worktree commit-token boundary. Stage only the
   frozen result's paths, verify the staged tree equals the retained result,
   then call `branch.CommitStaged` for its Goal-Unit commit. Never absorb
   work added after the completed build or reset the caller's worktree.
   The existing branch commit owner handles endpoint/claim validation.
3. Retain the committed subject before publishing it through `branch.Push`.
   Interrupted commit recording must reconcile the exact expected parent,
   tree and Goal-Unit trailer before attempting a new commit. A mismatch is
   an explicit state conflict. The same operation can publish a retained
   commit on retry; it never creates another subject or reader because a
   response was lost. Report committed-but-unpublished work as partial.
4. Call `RunBranchRead` on that immutable subject in its actual goal
   worktree. Use its existing frozen brief, dispatch identity, fast gate and
   retained read record. Return the real critic job and findings or an
   in-progress continuation. The preliminary UnitRunner read is not
   converted into a certified critic record. Requesting a committed review
   authorizes publication of this review branch, not landing or conclusion.
5. A terminal reader still needs explicit author dispositions and real
   closure. `close J --dispositions FILE` performs that existing sequence.
   Then repeat `review unit U` to collect the actual closed read through
   `CommitRead` and publish the resulting Goal-Read with `branch.Push`.
   This collection/publication also belongs to `review commit SHA`.
   If the reader has merely stopped, report the required close action;
   never call terminal status certification. If collection commits but
   publication fails, preserve the attestation and give the same retry.
6. After published accepted evidence, the next action is `land G`. Its
   existing admission remains unchanged. No implicit approval, dispositions,
   accepted-risk word or goal conclusion is created by this connection.

The UnitRun owner stores only its result-to-subject binding and retains its
existing lock. Branch-read records own critic/collection state; branch push
owns transport recovery. No parallel workflow ledger or new certificate is
introduced. A later completed round of the same unit uses the existing
`CommitRequest.Amend` path: replace that unit's subject, discard reads of
the replaced subject, replay the suffix and publish through the owner's
force-with-lease under the same claim. Keep the prior round's subject in
the run for diagnosis. A changed subject needs its own committed review;
never reuse the old attestation or invent a second unit name for a fix.
The owner's replay or concurrent-tip refusal remains a truthful partial
state; it never authorizes a reset. Bind the completed round's observed
HEAD as the expected pre-commit tip; `plan.Base` is the cumulative diff
base and need not equal that tip after a follow-up. The retained diff and
snapshot are verified before staging; a snapshot's raw Tree field must
not be misread as a Git tree object identifier.

This preserves the already implemented UnitRunner feedback pass and adds
one genuine committed review. Removing the preliminary read would require
changing that runner's mandatory read contract and is deferred; silently
promoting its output would weaken certification. The additional read must
fit the existing approved goal budget; absence of capacity is a real
decision, never an automatic budget increase.

First connected slice: build result, review unit, explicit close, review
unit collection, land. All are public tasks; callers need no internal
worktree/commit/push/attestation sequence. No dirty checkout is moved
automatically.

**Review.** Target type determines evidence, not operator knowledge of a
pipeline. A design file must be a discoverable design record; generate the
review task from that record, design-critique rules and explicit overrides,
then run the configured design-review lane. It yields findings and a verdict
for the design author to adjudicate. A job review uses current conformance,
frozen subject and design/code-critic dispatch, deriving root and manifest
from the recorded subject. A commit review uses `branch.Read`, including
its retained open/closed state and attestation. Wait/collect through the
existing readers and locked register owner; an unknown dispatch outcome is
reported and recovered, never retried as a fresh model call. Review does not
silently accept findings or certify standalone output as a chain review.

**Fold and close.** Dispositions remain the author's input. Validate their
join against actual findings before a follow-up. Derive parent/round/root
from the named review. Unit folds use UnitRunner follow-up; chain folds use
existing delegate follow-up and applicable proof/review. Close mirrors the
terminal members, advances the existing finding register, validates closure
and invokes the existing closure owner. Stop at the first genuine decision
or unresolved proof and print that state; do not add an automatic judgement
agent or replace the review-round law.
The complete closure owner is `scripts/agents/dispatch.sh close --job J`
with its existing `--reconcile-evidence` when supplied. Invoke it as
missionrunner does; never set `--runner-closed` or recompose just the Go
leaf verbs and omit its lock, mirrors, stamp or cleanup. Porting that
sequence into Go is outside this boundary redesign.

**Land.** First resolve the explicitly named goal or job to its actual
accepted evidence. A certified chain goes through the existing batch join
owner. A goal branch uses the existing branch batch route when its evidence
meets that owner's admission; retained hand-reader evidence uses hand
preparation/push. Resolve this from the recorded evidence kind and existing
landing policy before outward effects; the caller does not choose an
internal pipeline. Missing required proof or approval is a specific refusal,
not a reason to fall back to another route.
Specifically, configured `landing.batch-root` plus admissible chain or
branch-member evidence selects batch join, with the recorded last/through
selection. Retained hand-reader evidence selects land-prep then land-push
with the owner's current test receipt. Otherwise name the missing input.
The command owns test-receipt preparation, conformance/closure where needed,
transport, proof, push and result collection. It reuses sufficient matching
proof and the existing durable landing record on retry. Changed bases take
the owner's revalidation path; no receipt is synthesized from a green launch.
Keep the existing handoff/branch/prepared identifiers behind the command and
print them for diagnosis. Partial success names committed/pushed state and
the exact resume command. Never fall back to a weaker route after refusal.

**Questions and process control.** Keep mission answers, channel authority
and project questions distinct. A channel question's response is received
through its authenticated channel; display those reply instructions rather
than inventing a local authority path. `restart checkout` stops first and
arms only after a successful stopped state; reports fence/runner state if
the second step fails. Fleet stop is not advertised until its actual owner
supports it. Human and agent startup have different declared targets.
`start` is the human word; `start session` is the agent word; old `up` keeps
its agent behavior for compatibility. This explicitly supersedes the name
requested by `up-is-the-human-start-command`, while carrying its actual
requirements: status and health read the same enrollment/runner record,
and rebuilding an enrolled checkout needs no fresh human act while the
same enrolled terminal remains alive. Reconcile that sibling when slice 2
lands. The unsupported legacy `--all` stop stays an explicit refusal and
is absent from public help; no cross-installation stop is added here.

## 7. Results, refusals and safe retry

One public result rendering owner in `cmd/metasystem` says what happened,
to which target, what remains, and the next command when there is one. Reuse
`shellCommand` for recipes and extend the existing human/process refusal
renderers to supply structured next actions. Keep domain codes and detailed
evidence available in JSON/diagnostics. Do not infer state by regex over
stderr and do not redirect process-global stdout in order to guess outcomes.

New intent JSON has one envelope: `schemaVersion: 1`, `verb`, `targets`
(array of `{kind,id}`), `outcome` (`confirmed`, `unchanged`, `in-progress`,
`partial`, `refused`, or `failed`), `summary`, and `data` containing the
owner's typed projection. Optional `next` carries executable `argv` and
`reason`; optional `decision` names a missing human input. A read succeeds
as confirmed. The same result renders concise text. Legacy JSON is
unchanged. Refusals require semantic result/remedy coverage, not merely a
test that the adapter passed the expected argv to an old handler.

Boundary mistakes are repaired before effects: missing target shows the
specific command and candidate targets, bad flag names show the accepted
flag, malformed budget shows the compact format with the actual known
values. Domain refusals are rendered where their owner knows the state.
Use one executable next step only when its prerequisites are known; if a
human act or missing decision is required, name it and preserve the typed
inputs. Never claim a command would succeed when its missing data cannot
be inferred. A copied remedy is exercised in the relevant fixture.

Already committed primary acts with failed secondary publication report
partial success, retain transaction identity and use the owner's repair
path. An invocation timeout reports in-progress and does not resubmit.
Existing opids, job ids, branch-read records, unit runs and landing records
own retry. Public user mistakes do not require a new opaque operation id.

## 8. Implementation homes and slices

The production consumer of the command descriptors is the router, CLI help
and the Partner catalogue. Keep descriptors and argument normalization in
`cmd/metasystem/intent.go`, small handler groups in `intent_goals.go`,
`intent_process.go`, and `intent_work.go`. Existing packages remain owners
of transactions. Add typed inputs/results or reuse existing dependency
seams only where an intent must compose operations. No generic command bus.

| Slice | Usable outcome | Scope deferred to the next named owner |
| --- | --- | --- |
| 1 | Human goal surface, precise help, repository resolution and old-call compatibility | Process commands to slice 2; build/review/land to slice 3; complete advanced integration to slice 4. |
| 2 | Start/stop/restart/enroll/decide, goal authoring/claim and questions with truthful recovery | Composed agent workflows to existing launch/branch/dispatch owners in slice 3. |
| 3 | Brief preparation, full build/review/fold/close/land and resumable waits | Remaining documentation/caller cutover and release-wide evidence to slice 4. |
| 4 | Complete public catalogue, advanced options, shipped human/agent docs and runtime callers agree | Deleting old compatibility entrypoints, a generic natural-language shell, new authority schemes and new fleet-stop behavior are outside this release. |

All four slices are within Wido's authorized scope. Slice 1 is not the goal's
completion. Update this design's Built section and goal next step as each
lands. Preserve the existing instruction/skill semantics while changing
their command examples; any substantive policy change needs its own reason.

## 9. Proof and completion obligations

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| VMI-1 | HIGH | 3-5 | Help and routing expose only implemented intent grammar; Partner retains public plus full internal catalogue; compatibility remains callable | cmd/metasystem | metasystem/cmd/metasystem/intent.go and ui_describe.go | metasystem/cmd/metasystem/intent_test.go: TestIntentHelpAndCompatibility; intent_coverage_test.go | root-release-public-runtime-3e59.json; final command-interface-smoke and TestIntentPublicCoverage | DONE | None; see plans/verbs-match-intent-verification.md |
| VMI-2 | HIGH | 5 | Equivalent repository paths select one real world; foreign/missing roots refuse | internal/stateroot and cmd/metasystem | metasystem/cmd/metasystem/intent.go: selectRoot; internal/stateroot | metasystem/cmd/metasystem/intent_test.go: TestIntentRepositorySelection; intent_process_fix_test.go | root-release-public-runtime-3e59.json; root-runtime-root-equivalence.json; final adopted-root fixtures | DONE | None; see plans/verbs-match-intent-verification.md |
| VMI-3 | CRITICAL | 5-6 | Goal acts retain human proof, claim identity, exact approved budget and obligations | internal/goal and internal/humanauthority | metasystem/cmd/metasystem/intent_goals.go; goalsync_owner_report.go | metasystem/cmd/metasystem/intent_test.go: TestIntentGoalAuthorityAndState; intent_planning_test.go | root-release-public-runtime-3e59.json; root-runtime-lifecycle-r2-records.json; root-runtime-parked-final.json | DONE | None; see plans/verbs-match-intent-verification.md |
| VMI-4 | HIGH | 5,7 | Positional/flag variants are equivalent; conflicts fail before effects; remedies preserve literal inputs | cmd/metasystem | metasystem/cmd/metasystem/intent.go; intent_planning.go | metasystem/cmd/metasystem/intent_test.go: TestIntentArgumentsAndRemedies; intent_text_files_test.go | root-release-public-runtime-3e59.json: literal correction argv, exact budgets and caller-relative files | DONE | None; see plans/verbs-match-intent-verification.md |
| VMI-5 | HIGH | 6 | Build composes existing steps, resumes once, never claims judgement/landing | internal/launch.UnitRunner | metasystem/cmd/metasystem/intent_work.go; internal/launch/unit_named.go and unit_run.go | metasystem/cmd/metasystem/intent_work_test.go; intent_work_continue_test.go; internal/launch/unit_lock_test.go | root-runtime-work-final.json; root-unit-lock-descriptor-probe.json; final whole-command run and retained c78 batch-tagged run | DONE | None; see plans/verbs-match-intent-verification.md |
| VMI-6 | CRITICAL | 6 | Reviews bind their true subject; dispositions and closure retain independent evidence | internal/dispatch and internal/goal/branch | metasystem/cmd/metasystem/intent_delivery.go; intent_unit_review.go; internal/dispatch/chain.go | metasystem/cmd/metasystem/intent_delivery_owner_test.go; goal_branch_real_delegate_test.go | root-connection-final-delegate.log; final goal-decision-standard and TestIntentConnectedJourneyRealClose | DONE | None; see plans/verbs-match-intent-verification.md |
| VMI-7 | CRITICAL | 6-7 | Landing preserves route authority, exact proof, partial outcomes and retry | internal/goal/branch and internal/landing/batch | metasystem/cmd/metasystem/intent_delivery.go; internal/goal/branch/land.go and read_publish.go | metasystem/cmd/metasystem/intent_delivery_owner_test.go; intent_connected_journey_test.go | verification-final-c78-repeat/native-result.json: TestIntentLandWholeOwnerGitAdapter, TestIntentLandBatchAdmissionGitAdapter and TestBatchLandingLifecycleEndToEnd | DONE | None; see plans/verbs-match-intent-verification.md |
| VMI-8 | HIGH | 6 | Process/answer targets preserve authority; health/status share runner truth; live enrollment survives rebuild | internal/stoptransition, internal/up, internal/missionrunner and internal/channel | metasystem/cmd/metasystem/intent_process.go; internal/ui/lifecycle/roots.go | metasystem/cmd/metasystem/intent_process_test.go; intent_process_fix_test.go | root-runtime-process-final.json; root-release-public-runtime-3e59.json; final whole-command lifecycle and question fixtures | DONE | None; see plans/verbs-match-intent-verification.md |
| VMI-9 | HIGH | 4,8 | Complete authorized surface and advanced capabilities are discoverable and documented | cmd/metasystem | metasystem/cmd/metasystem/intent.go and intent_*.go; README.md and metasystem/docs/working-with-agents.md | metasystem/cmd/metasystem/intent_coverage_test.go: TestIntentPublicCoverage | root-release-public-runtime-3e59.json; final TestIntentPublicCoverage and Partner catalogue fixtures | DONE | None; see plans/verbs-match-intent-verification.md |
| VMI-10 | CRITICAL | 6 | Built result reaches published unit and genuine review evidence through public tasks, without importing later edits or accepting findings | internal/launch.UnitRunner, internal/goal/branch and internal/dispatch | metasystem/cmd/metasystem/intent_unit_review.go and intent_worktree.go; internal/goal/branch/read.go | metasystem/cmd/metasystem/intent_connected_journey_test.go; goal_branch_real_delegate_test.go; intent_connection_final_test.go | final TestIntentConnectedJourneyRealClose and TestIntentActualBuilderChild; root-connection-final-runtime.log and root-connection-final-delegate.log | DONE | None; see plans/verbs-match-intent-verification.md |

Focused review obligations (status follows their parent row and the evidence below; a candidate test alone does not certify the integrated release):

| Finding | Parent | Named fixture and required observation |
| --- | --- | --- |
| VMI-R2-01 | VMI-3 | TestIntentResumeAttorney: parked resume uses existing unpark grant with verified evidence; stopped claim refuses the grant. |
| VMI-R2-02 | VMI-5 | TestIntentBuildConcurrentRepeat: two identical invocations share one reserved run and launch once. |
| VMI-R2-03 | VMI-5 | TestIntentBuildSizeInput: real units row or explicit estimate reaches buildSize; absent estimate names the needed input. |
| VMI-R2-04 | VMI-6 | TestIntentCloseWholeOwner: invokes full close command and observes locked mirror/register/closure behavior; no runner-closed shortcut. |
| VMI-R2-05 | VMI-7 | TestIntentLandRouteEvidence: configured admissible batch, retained hand evidence and missing proof take the stated routes without weaker fallback. |
| VMI-R2-06 | VMI-4 | TestIntentTierlessApprovalRemedy: preserves refusal and names the missing four risk answers/basis; never prints executable placeholder N or suggests unsupported tier-only classification. |
| VMI-R2-07 | VMI-5 | TestIntentGeneratedUnitPlan: generated build/read briefs and resolved independent read model satisfy ReadUnitPlan. |
| VMI-CONN-01 | VMI-10 | TestIntentBuiltUnitToLanding: committed findings, fold and re-review amend the same unit, invalidate its old read, preserve replayed work and land exactly one current subject for that unit. |
| VMI-CONN-02 | VMI-6 | Existing close fixture joins actual findings/dispositions/register before the full close owner; dispositions alone do not certify a finding. |
| VMI-CONN-03 | VMI-10 | Terminal but unclosed critic gives the public close remedy and creates no attestation. |
| VMI-CONN-04 | VMI-10 | Worktree preparation covers fresh endpoint, remote-only adoption through branch.Push, existing valid worktree, occupied foreign path and divergent history. |
| VMI-CONN-05 | VMI-10 | Real builder entrypoint runs inside the generated worktree; use existing adapter-declared session-isolation manifest only if its local runtime configuration is required, never copy metasystem.conf.local. |
| VMI-CONN-06 | VMI-10 | Read-failed with passed proof can request committed review; proof-wrote and changed result bytes refuse before staging. |

Behavior tests use per-test dependency instances and Git stubs. Named native
integration cases prove real path/process or Git transport where needed.
Focused tests and old-command contract tests precede the selected shared
testing run. Run on this Mac with nine workers; no VM run is needed or
authorized by this design. Retain actual exit codes and observe the rebuilt
CLI. No test may call a real approval, stop, model job or push on this checkout
to simulate a user's workflow.

Release acceptance walks: discover and approve a goal; change/read a budget;
pause/resume; recover a typo and a wrong terminal; start/status/stop; claim,
brief, build, review, fold, close, land; answer the correct kind of question;
inspect fleet and diagnose a fault with an executable owner remedy. Each uses only public intents
and prints the meaningful result. Advanced options remain available through
help and the internal escape hatch. A green facade test is not proof that
the whole workflows exist.

## 10. Review contract and Built

Independent Fable/Opus critique, at most two rounds, failsafe round 2. Review
the complete release contract for missing work or false premises and the
first slice for implementability. A material finding changes what must be
built and makes that slice fail at first use or violate safety; name the
scenario, owner and proof. Optional polish is noted, never machinery added
for its own sake. Root adjudicates every finding. All bounded mechanical
residue becomes a named proof obligation; no unsafe unresolved design is
certified. Later code review is independent and includes brief conformance.

Built: the complete implementation from candidate
3e59d41077fe4cbfba6ad00f5b09f20e5c9da1a8 is integrated locally by the commit containing this completion record.
The live executable has not been replaced and no remote push was performed.
All component code reviews are closed, including the bounded
root corrections after the two independent connection reads. Root has read
every final source/test delta and the integration conflict resolutions.

The runtime evidence includes exact budgets and human-proof refusals,
stopped/parked resume, equivalent repository paths, nested UI roots,
post-publication partial results and guarded named-build retries. Root's
inherited-descriptor regression verifies explicit lock release; all five
unit-lock scopes use it. An interrupted worktree-settings copy now completes
on retry, and a real builder process observed its settings inside the
generated worktree without starting a second child on a repeat.

The committed-review path now composes a lawful default mode for plain
prose while preserving explicit modes. Root independently drove the actual
branch-read owner, built delegate, dispatch script and Claude adapter to a
real fixture child. It observed the selected mode's critic, its existing
maximal-model authorization and worktree cwd, with no override or escalation.
The former missing-mode and missing-authorization probes fail on the earlier
candidate and pass unchanged after the corrections. Runtime/configuration
owners remain unchanged; four resolved nonsecret settings cross the boundary.
No actual local secret configuration was copied.

One physical goal's actual built A/B and corrected A/current B subjects now
flow through real register advance, full critic closure, read collection and
publication into public batch admission. Repeated land preserves exact
membership and performs no duplicate admission. Model execution, proof and
fixture authority are declared synthetic at those boundaries; later batch
sealing/push is proved separately by the existing lifecycle fixture.

The 48-public-verb and 54-goal-act coverage boundary and documentation are
complete. Necessary process-wide fixtures remain serial; after eligible
tests were made parallel, the command-package serial baseline moved from
536 to 541 under the human's machinery bypass. Assertions, coverage and
other package baselines remain unchanged. All ten critical/high matrix rows are DONE from observed runtime evidence.
The rebuilt CLI passed 72 public command checks. The complete native
selection ran; ten affected or connected groups passed after fixture
corrections. Both final static prerequisite groups and all five final
affected native groups passed. All 38 installation sections completed,
with 32 passing and six failing. The corrected audit/protocol sections and
all ten adoption bodies passed on the final source. All 13 interrupted
landing, goal-completion and supervision scenarios also passed. Six unrelated legacy
coverage-ratchet failures were proven at baseline and remain disclosed. Evidence provenance and its limits are in
`plans/verbs-match-intent-verification.md`. Live model review, production
terminal ancestry and live delivery are not certified by fixture evidence.

Full design findings and root dispositions are linked from
`plans/verbs-match-intent-design-review.md`; the 23-goal inventory is
`plans/verbs-match-intent-goal-index.md`. Detailed evidence and component
revisions are recorded in `plans/handoff-verbs-match-intent.md`.
