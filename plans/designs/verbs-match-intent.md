# Commands for what the human and agent want to do

- Kind: design
- Id: 01M3CGR7CNZTS2NNTQCYRCF4JX
- Status: accepted
- Goals: verbs-match-intent

Review exit: closed at round 2 with zero material findings and seven named
fixture obligations. Root accepts under Wido's explicit design/implementation
authorization; independent critic Fable 5.1 agrees full scope and smallest
sufficient design. Adjudication: `plans/verbs-match-intent-design-review.md`.

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
| `open G --intent TEXT [--next TEXT]` | Create goal; existing human/agent intake law and blocker arguments apply. |
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
| `review design FILE`; `review job J`; `review commit SHA` | Freeze and review the named subject through the respective owner; automatically prepare plumbing inputs and collect the result. |
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
| `notes G [--add TEXT\|--close ID]` | goal read-items. Findings stay findings, never certified by this convenience. |
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
| `--model MODEL`, `--effort VALUE` | Explicit supported runtime overrides. Defaults come from the existing roster. |
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
the caller's check argv. Require a current `goal/G` worktree, and print the
existing branch-preparation command if absent; never move a dirty checkout.
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
| VMI-1 | HIGH | 3-5 | Help and routing expose only implemented intent grammar; Partner retains public plus full internal catalogue; compatibility remains callable | cmd/metasystem | intent catalogue and dispatch | TestIntentHelpAndCompatibility | rebuilt binary help human/agent, Partner catalogue union and old commands | MISSING | slice 1 |
| VMI-2 | HIGH | 5 | Equivalent repository paths select one real world; foreign/missing roots refuse | stateroot plus intent boundary | ResolveLayout adapter | TestIntentRepositorySelection | top, installation, child, space and symlink paths | MISSING | slice 1 |
| VMI-3 | CRITICAL | 5-6 | Goal acts retain human proof, claim identity, exact approved budget and obligations | goal and humanauthority | intent goal routing | TestIntentGoalAuthorityAndState | isolated fake-authority public lifecycle plus real agent refusal | MISSING | slices 1-2 |
| VMI-4 | HIGH | 5,7 | Positional/flag variants are equivalent; conflicts fail before effects; remedies preserve literal inputs | cmd/metasystem | parser and existing refusal renderer | TestIntentArgumentsAndRemedies | bad flag, missing target, quoted reason and executable correction | MISSING | every slice |
| VMI-5 | HIGH | 6 | Build composes existing steps, resumes once, never claims judgement/landing | launch.UnitRunner | plan preparation and existing run lock | TestIntentBuildResume | fake adapters drive build/proof/read and interrupted resume | MISSING | slice 3 |
| VMI-6 | CRITICAL | 6 | Reviews bind their true subject; dispositions and closure retain independent evidence | dispatch and branch | review/fold/close composition | TestIntentReviewEvidenceKinds | design, chain and branch scenarios including unknown dispatch | MISSING | slice 3 |
| VMI-7 | CRITICAL | 6-7 | Landing preserves route authority, exact proof, partial outcomes and retry | existing landing owners | intent land composition | TestIntentLandRecovery | admitted/refused/partial-push paths with existing fixtures | MISSING | slice 3 |
| VMI-8 | HIGH | 6 | Process/answer targets preserve authority; health/status share runner truth; live enrollment survives rebuild | stoptransition, up, missionrunner, channel | explicit target routing and shared process records | TestIntentProcessAndAnswerTargets | checkout start/status/health/rebuild/stop fixtures, mission answer, channel instructions | MISSING | slice 2 |
| VMI-9 | HIGH | 4,8 | Complete authorized surface and advanced capabilities are discoverable and documented | cmd/metasystem and shipped docs | complete intent table and caller sweep | TestIntentPublicCoverage | task walkthroughs from fresh help, no internal steps required | MISSING | slice 4 |

Focused review obligations (all MISSING until implemented and observed):

| Finding | Parent | Named fixture and required observation |
| --- | --- | --- |
| VMI-R2-01 | VMI-3 | TestIntentResumeAttorney: parked resume uses existing unpark grant with verified evidence; stopped claim refuses the grant. |
| VMI-R2-02 | VMI-5 | TestIntentBuildConcurrentRepeat: two identical invocations share one reserved run and launch once. |
| VMI-R2-03 | VMI-5 | TestIntentBuildSizeInput: real units row or explicit estimate reaches buildSize; absent estimate names the needed input. |
| VMI-R2-04 | VMI-6 | TestIntentCloseWholeOwner: invokes full close command and observes locked mirror/register/closure behavior; no runner-closed shortcut. |
| VMI-R2-05 | VMI-7 | TestIntentLandRouteEvidence: configured admissible batch, retained hand evidence and missing proof take the stated routes without weaker fallback. |
| VMI-R2-06 | VMI-4 | TestIntentTierlessApprovalRemedy: preserves refusal and names the missing four risk answers/basis; never prints executable placeholder N or suggests unsupported tier-only classification. |
| VMI-R2-07 | VMI-5 | TestIntentGeneratedUnitPlan: generated build/read briefs and resolved independent read model satisfy ReadUnitPlan. |

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

Built: implementation started in an isolated checkout after design acceptance.
No slice is yet complete. The full Fable findings and root dispositions are
linked from `plans/verbs-match-intent-design-review.md`; the goal inventory
is `plans/verbs-match-intent-goal-index.md`. Follow progress in
`plans/handoff-verbs-match-intent.md`.
