# Complete tasks through intent

- Kind: design
- Id: 01M3EC3QT7M2TC36P7ZVNRF0RW
- Status: accepted
- Goals: verbs-match-intent

Design critique: closed at round 2 on 2 fixture obligations (IW-C7 and IW-C8),
with material findings falling from 6 to 2. Root accepted the bounded binding
and grammar corrections below. Independent implementation critique is mandatory;
this authorizes implementation and does not claim that the proof is complete.

Author: root Codex. Wido, 26 September 2026, authorized design, Fable critique,
implementation and repeated usability improvement until the interface is easy,
elegant, robust and fully capable. Existing machinery bypass applies. This
replaces the public-boundary decisions of plans/designs/verbs-match-intent.md;
it preserves that release's execution owners, authority and proof guarantees.

## Contract and step 1

A person or agent names the outcome and supplies real decisions. MetaSystem
handles selection, preparation, execution bookkeeping, collection and recovery.
No advertised task or its ordinary failure path requires an engine family,
chain root, storage directory, internal state transition, or a sequence of
close/collect/restamp calls. Root help is an orientation to common work, not
an inventory. Full public capability stays discoverable through focused help.

Step 1 is a complete goal-directed delivery: build requested work, independently
review its exact result, supply decisions about findings, revise when necessary,
and land. Clean review closes and collects itself. Repeated requests do not
create duplicate work. Discovery and the Partner describe that same surface.
The release also completes the human, question and recovery paths below; these
are required before this user request is complete, even if built after step 1.

Deferred beyond this release: natural-language command interpretation, new
authority rules, a new scheduler, a new workflow store, automatic semantic
acceptance of findings, and the separate evidence-reuse design. Existing
adapters/owners are reused; opaque machine protocols remain compatibility APIs.
No capability is counted as preserved merely because an internal escape hatch
still exists. Every meaningful existing public action and recovery outcome
listed below has a public route and a tested consumer.

## Grounded facts

Source: c8ecd4bb62706c02331f12a4de4ae0025b634568; relevant owners unchanged
from c0ea6788c. The 48 descriptors advertise 39 human/both and nine agent-only
commands. New root help is 72 lines; the older local executable is 527 lines.

| Fact | Source owner |
| --- | --- |
| Root help prints every descriptor; test mandates it despite the design's promise to hide advanced acts | cmd/metasystem/intent.go:850,886; intent_test.go:176; old design:196 |
| Public command help appends whole internal family help on name collision | intent_work.go:260; main.go help routing |
| Partner appends all internal families, and its renderer doubles the executable for the public family | ui_describe.go:85; internal/ui/uitools/kit.go:154 |
| Build already prepares its own goal worktree; caller-side checkout instructions are stale | intent_worktree.go:132-250; intent_work.go:462 |
| Named builds retain exact request identity; original requests are immutable | internal/launch/unit_named.go:130-183 |
| Repeated FollowUp after a finished round can start another revision | internal/launch/unit_run.go:183-195 |
| Review freezes the actual built result and binds commit/push/review under existing owners | intent_unit_review.go:69-241; internal/launch/unit_review.go:69-137 |
| Finished review asks the caller to close in another checkout, then repeat review to collect | intent_delivery.go:834-860; intent_unit_review.go:364-412 |
| Full close includes locking, evidence mirroring, register close and close checks | scripts/agents/dispatch.sh:2975-3044 |
| Ask prescribes internal channel wait/poll; mission answer recovery prescribes internal resume | intent_process.go:754-917 |
| Doctor forwards owner repair commands; recover G actually recovers installation-wide journals | intent_process.go:958; intent_planning.go:1645 |
| Landing already handles exact proof, interrupted publication and cleanup | intent_delivery.go:1166-1380 |

These are implementation facts, not permission to bypass them. The internal
launch, dispatch, goal, branch, channel, mission, health and configuration owners
retain their data, locks, authority and failure semantics.

## Public language and discovery

Keep familiar verbs for real choices. Do not compress distinct approval, budget,
pause, cancellation and completion decisions into one ambiguous command.
Introduce `revise` for requested corrections, `accept-risk` for the actual
reserved risk act, `check`/`repair` for diagnosis and explicit recovery, and
`incidents` for broken-main responsibility. Old spellings keep behavior as
unadvertised compatibility aliases; public responses always use current terms.
`doctor` may stay an alias for `check`; `recover` for the existing repair mode;
`fold`, `close`, `ready`, `resolve`, `red`, `decide` stop being primary public
workflow steps. Their capabilities are accounted for below.

One existing descriptor table owns grammar, audience, help group and visibility
(primary, advanced, compatibility). No second command inventory. Root help is
one short orientation, grouped by outcome, showing common actions and examples,
with `help goals`, `help work`, `help operations`, `help questions`, `help human`,
`help agent`, `help all`, and `help VERB` for progressively complete discovery.
Topics and command names must not collide: where `goals` is a command, its
command help includes the related planning group; do not silently shadow it.
`help all` lists all supported public capabilities, not technical engine verbs.
Command help advertises actual accepted flags only; deliberately refused flags
are not options. Compatibility-only fixture and internal session/approval-binding
flags remain parseable but are absent from help and suggestions. Human identity,
explicit authority decisions and ordinary budget choices stay understandable. Detailed formats include a complete runnable example.

`metasystem` with no arguments and `--help` are successful, read-only help.
Old internal/direct family calls still execute for scripts, but default help,
public command help, unknown-command suggestions and the Partner never list
those families. `internal` remains an explicitly requested compatibility route,
not a user-facing recovery instruction. The Partner consumes the same public
catalogue; no doubled `metasystem`, hidden aliases or engine families appear.
Keep full machine protocols for existing agents/hooks, without making them
necessary to execute an intent.

## Work selection and complete delivery

Canonical ordinary grammar:

```text
metasystem build G [--work NAME] --brief FILE --check COMMAND...
metasystem status G [--work NAME]
metasystem review G [--work NAME] [--dispositions FILE]
metasystem revise G [--work NAME] [--after N] --brief FILE [--dispositions FILE]
metasystem wait G [--work NAME] [--timeout DURATION]
metasystem wait G --for landing|human-act [--verb V] [--since TIP]
metasystem land G
```

A work name is a caller-chosen part of a goal, not a run or chain identity.
The first unnamed build uses `main`. Existing positional G UNIT stays compatible.
Build on an approved, ready, unclaimed goal acquires its claim through the existing
claim owner, preserving quota, readiness, elapsed fences and exact session proof.
It never takes another holder's claim or approves a goal. A foreign holder,
missing approval, or another current claim returns the actual public decision.
`claim [G]`, `release [G]`, `goals --ready` and `enroll` remain advertised. Session
identity failure directs the agent to `start session`, never to copy an identity.

Selection is explicit and stage-based. Status lists all named work. Wait selects
running work; review selects a newest built result without a collected current
read; revise first selects failed work or work with unresolved findings, otherwise
an explicitly named or unique completed work item. Build selects the matching
retained named request, and creates `main` only when no work exists. If no work
needs review, review reports the already collected results unchanged; it never
starts a new read just to choose something. Finished/landed work is read-only
history unless a new revision is explicitly requested and the goal is still live.
Exactly one eligible item may be inferred; zero gives the appropriate completed,
waiting or missing-prerequisite result; multiple items list names and exact public
commands. No command picks the most recent timestamp.
Goal-event waits remain first-class: `--for landing` waits for landing and
`--for human-act` optionally filters the actual human verb. `--since` names the
observed goal revision; existing `--event` and `--after` spellings remain aliases.
If no work is running but landing is queued, bare wait offers `wait G --for landing`.
The selection belongs to UnitRunner beside named build identity. Add a read API
there for (real worktree, goal, optional work); reject corrupt bindings and
foreign roots. CLI code does not scan private storage or create another index.
Existing job/commit/design review forms remain advanced public targets where
they name a deliberate review subject; callers never need a critic root.

Brief/design own intent, scope and proof. Keep explicit --check and optional
size estimates for agents who need them; ordinary use derives size from the
accepted design/brief and uses the configured reader allowance. Add one bound
`intent.review.tool-calls=48` to the existing config owner, overridable by the
already supported explicit reader option. Missing design decisions, proof or
size allocation produce the missing planning question before any paid launch;
never invent a plan or understate a budget. `brief` explains these inputs using
a complete example and can still produce a scaffold without launching work.

Build repeats through AdvancePrepared; no silent rewrite of prior inputs.
Revision uses a small UnitRunner owner operation under the existing named/run
locks. A public work attempt has its existing monotonically numbered round N,
whether that attempt passed or failed. `--after N` explicitly selects the attempt
being corrected. Retain (goal/work, after N, immutable brief bytes, disposition
digest, resulting attempt) before any launch. Return the same resulting attempt
on every retry, including terminal failure; do not silently consume another paid
attempt because a response was lost. When --after is omitted, first rejoin an
existing identical request for that work; only a genuinely new input binds the
current attempt. A matching older request after later work reports its original
result plus the current version, without effects; `--after CURRENT` explicitly
asks for the new correction. A failed attempt still has N: its remedy is the same
brief with `--after N`, which deliberately creates one new attempt under the normal
budget. New requests against stale N refuse. Repeated/concurrent calls for the
same after-N request join once. This preserves retry and rerun as different intents
without editing a brief to force another run or adding another workflow store.

Review G resolves the work and uses ReviewSubject and the existing committed
review path. The exact built result is committed/published as today; help and
results say that review records the candidate for independent examination.
Preliminary build feedback never substitutes for that committed examination.

A running review returns a public continuation of the same intent. An authentic
completed review with zero findings generates the structurally empty disposition
join and invokes the existing full close owner, then collects/publishes the read
within the same operation. Nonempty findings stop at the author's actual
decision and present their text plus a complete disposition template. Supplying
`review G --dispositions FILE` validates the exact current subject and every
finding. `accepted` means a fix is required; `refuted` needs evidence;
`out-of-scope` cites the declared brief; `noted` is nonmaterial only. Any accepted
material finding without an already recorded lawful resolution requires `revise`:
review refuses before closure and prints that exact public correction request.
Only a complete set with no unresolved material finding may close/collect/publish.
A genuine human risk acceptance is recognized only through its exact existing
owner record and retains an explicit exception outcome, never a caller's label.
The owner still decides closure and whether the resulting read authorizes landing.

The generated disposition file includes a machine-written binding to goal, work,
reviewed attempt, immutable reviewed subject and return digest. Users fill decisions and
evidence, not that binding. Canonical review and revise validate it and freeze
its bytes with the request before effects; a stale file cannot resolve a different
review. The reviewed attempt is distinct from the correction base `--after N`.
The current review is the newest authentic examination of that work, including
unresolved findings not yet eligible for clean closure/collection. A failed
examination with no findings does not supersede those findings. A correction
after N may reuse that binding when N is at or after the reviewed attempt and
no later authentic examination supersedes it. Retain the same frozen findings
and decisions as builder input across intervening failed correction attempts;
the failed attempt's empty read outputs cannot replace them. A superseded file
refuses naming the current examination. Help and failed-attempt remedies explain
this distinction. Replaying a completed decision rejoins its stored operation. Legacy
explicit job/disposition APIs retain their existing contract. For manually prepared
files the public command first offers the bound template; it does not silently
assume that a same-named finding belongs to the current subject. No separate close
command or second collection invocation is required after the decision.
A partial close or publish repeats the same public request and cannot falsely
claim that review is accepted. Failed/missing critic returns remain failures.
Closure refusals have these concrete routes: a temporary coordination fence is
in-progress, with `status G` and the same review command; missing holder authority
names the holding session and asks that holder to run the same public command
(or an explicit lawful `claim`/take-over); missing mirrored evidence uses
`repair review G [--work NAME]`, which replays the existing mirror/close-check
without inventing a verdict; corrupt or missing producer evidence requires a
fresh examination using `review G --retry N` for the displayed failed examination
number, or reports the exact unavailable external dependency. Record-writer
permission failures remain authority refusals with the required actor, not repair
instructions that grant authority. No route prints a bash close-check command.
The result says examination finished, completion pending until closure and
publication are actually complete. Retry N is accepted only for an owner-proven terminal failed examination with
no completed findings; it cannot bypass a live process, findings, budget or
authority. This route does not exist today for every failure: internal/goal/branch/read.go
retains a root, and dispatch follow-up currently admits completed/protocol-error
critics only (dispatch.sh:2480-2490). Extend those existing owners with a bounded
failed-examination follow-up, preserving the original critic chain/round budget,
frozen subject, fresh independent session and old failed record. A lost process
must be proven stopped before retry admission. Store the mapping from failed
examination N to the newly admitted examination under the existing read lock
before dispatch. Repeating retry N rejoins that attempt even if it too failed;
its failure offers retry of its own new number. No new critic root to evade the
round cap and no second review store. The Go dispatch policy owns eligibility;
the shell remains execution plumbing.

`revise` validates supplied findings decisions against the exact review when
there is one, then invokes the appropriate existing follow-up owner. A build
failure with no review permits a correction brief alone. It never erases the
old review, auto-refutes a finding or grants a risk acceptance. A request to fold
an already closed review refuses with the current subject; correcting previously
completed work is a new revision against its explicit work attempt, not reopening
a closed review chain. A changed result
needs a new independently reviewed subject. For a design review, the author
edits the design and invokes review again; explain that user action without
mentioning implementer-chain absence. For explicitly selected legacy job
reviews, use the existing follow-up owner behind the same revise intention.

Land retains its exact-candidate proof and admission rules. It handles mechanical
read collection and readiness/handover when the required decisions are already
recorded; it never invents dispositions or silently starts an unrequested paid
review. Missing review points to `review G`; multiple unread work items are named.
`land G --queue-only` performs exactly the existing land-ready act: make the
goal available for later landing and release its active-claim quota/elapsed fence
as that owner already does. It runs no proof, no read collection and no push.
The old ready spelling remains compatible. Ordinary land keeps its existing
route-specific handover order; it does not prematurely release authority to
force the two paths into one sequence.
Goal conclusion remains `done G`. Supplying proof for a prior review obligation
becomes `review G --finding F --test NAME` with review selection inferred only
when unique; advanced explicit review/artifact evidence remains available with
user-facing meanings and the old authority checks.

## Human capabilities, questions and operations

Existing goal creation, editing, claim/release, enrollment, approval, budgets, pause/resume, abandonment,
reopening, dependencies, priority, assignment, splitting, grouping, grants,
notes and conclusion stay explicit public capabilities in focused help. Their
less frequent verbs need not appear on the root page. `split` documents and
emits an example of its actual input format; no instruction says to discover an
internal manifest. Grouping describes related goals, not storage arcs. No
mandatory terminal question is added to otherwise complete scripted input.

Use `accept-risk G --finding F [--review R] --reason TEXT` for the existing
reserved act; infer the review only when unique. `incidents` lists incidents,
`incidents claim I --goal G` and `incidents close I --reason TEXT` preserve
ownership/closure authority. `show G` presents the goal record; `status G` presents live work. They also present questions,
project records and machines using recognizable targets. List current work to
make cancellation/status discoverable without knowing the launch store.

`ask` retains its authenticated delivery. `show question Q`, `wait question Q`,
`answer Q [TEXT]`, and `ask --retry Q` complete its public path. Resolve question
references through channel and mission owners: exactly one match wins; ambiguity
lists explicit public choices, never chooses a store by precedence. Mission
questions may use M/Q as a caller reference. A channel answer continues to direct
the person to the authenticated thread, with its concrete link/instructions;
plain local text does not manufacture authentication. Help says TEXT applies
to mission questions; channel questions show their reply location. A mission answer uses its
existing authority and resumes through its owner when lawful; partial success
provides the same answer/resume public command. Withdrawing a question is
`ask --withdraw Q --reason TEXT`, through the existing owner. Waiting and retrying
must neither send a new question nor lose cancellation. Output says when delivery
has not happened and gives a public settings/retry action.

Extend existing start/stop/restart/status targets consistently for interface,
session, autonomous mission and machine work where the owner supports the act.
Keep checkout default; never treat a bare stop as all machines. `start machine
NAME ...` wraps existing fleet creation; `start mission M` and `resume mission M`
wrap existing mission owners; `status --machines` replaces the fleet-only
inventory verb while the alias remains. Unsupported target/action combinations
refuse with supported public forms and no effects.

`settings` lists the configuration owner's full nonsecret metadata and selected
values with masking/provenance, not only launch settings. Existing file-based
configuration remains; no new config setter is required. `show designs`, `show
decisions`, `show record ID` and `show design --goal G` compose project readers.
Administrative actions live under focused operations help: `settings coordinator`
for existing brain declaration/show/withdraw; `settings compatibility` for the
existing engine floor; `repair goals` for journal recovery, reviewed manual edits,
legacy migration and explicitly accepting rewritten history; `repair mission M`
for existing restore/accept-workspace decisions. Their flags describe the actual
human choice and must preserve exact bytes/digests, actor proof and refusal rules.
They are not an automatically executed doctor program.

`check` diagnoses without writing. `repair` performs the explicitly selected
recovery through its existing owner, reporting affected scope (installation-wide
journal recovery is never presented as goal-local). Safe claim restamping and
wait recovery are part of continuation. Recovery of a changed authoritative
history, contaminated workspace, or a proof exception requires the original
specific human decision. Exceptional landing remains possible through an
explicit `land G --exception CODE --reason TEXT` public act, with all existing
carry proof/token/one-exception rules; ordinary land never implies that act.

## Results, recovery and migration

Keep the existing structured envelope and truthful outcome distinctions.
Public human text and actionable JSON continuation use the same public intent.
Normal data describes goal, named work, stage, findings, observations and needed
decisions. Internal diagnostic data remains available only in explicit diagnostic
output or compatibility APIs; never require a caller to parse it for continuation.
Rendering a confirmed result must still display a real outstanding Decision.
Do not replace errors with vague success or discard their evidence. Record the
technical cause for diagnosis and state the user-visible effect and remedy.

Translate remediation at the owning typed boundary, not a general regex that
rewrites arbitrary error text or guesses authority. Known mechanical continuations
run inside the operation. Known user decisions emit executable public commands.
An unrecognized failure remains explicit and offers `check` with the same target;
this is not complete recovery coverage until a named fixture proves that check
leads to an applicable public repair or an honest outside-system dependency.
Every existing public return that prints `internal`, raw goal/branch/dispatch
commands, an internal schema, or private-file instructions is audited and mapped.

Old scripts retain exact legacy routing and output. Public schema-1 consumers keep
existing fields where compatibility requires them; new public structured views
are additive, and diagnostic fields may be retained without being the task API.
Do not silently change a state mutation merely to reduce command count. Instructions,
skills and ordinary examples migrate to public workflows; maintainer protocol docs
can still document compatibility mechanisms. Rebuild and exercise the local CLI;
no service restart or remote production install is implied.

## Proof, iteration and implementation map

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| IW-1 | HIGH | Public language and discovery | Small orientation; complete public catalogue; no internal help or doubled command; bare success | Intent descriptors and Partner projection | intent.go/main.go/ui_describe.go/uitools kit | TestIntentPublicDiscovery; TestRealPartnerPublicCatalogue | Built CLI help and Partner kit | MISSING | Implement and drive |
| IW-2 | CRITICAL | Work selection and complete delivery | Exact goal/work and stable revision request; no duplicate failed retry | UnitRunner named/revision owner | internal/launch/unit_named.go/unit_run.go | TestNamedWorkSelection; TestRevisionRequestReplay | CLI lost-response and explicit failed-attempt retry | MISSING | Implement and drive |
| IW-3 | CRITICAL | Work selection and complete delivery | Bound dispositions, authentic close, no unresolved acceptance, public recovery | Existing review/close/branch owners | intent_unit_review.go/intent_delivery.go | TestIntentGoalReviewCompletion; TestIntentGoalRevisionJourney | Connected build/review/revise/land with actual owners | MISSING | Implement and drive |
| IW-4 | CRITICAL | Human capabilities | All lifecycle/authority acts; queue-only and exception retain exact semantics | Goal/landing adapters | intent_planning.go/intent_delivery.go | TestIntentAuthorityCapabilityMatrix; TestIntentQueueOnly | Human/agent refusal, partial publication, landing replay | MISSING | Implement and drive |
| IW-5 | HIGH | Human capabilities, questions and operations | Complete authenticated questions and mission operation | Channel/mission adapters | intent_process.go and focused adapter files | TestIntentQuestionJourney; TestIntentMissionRecovery | Fake-provider CLI delivered/undelivered/ambiguous journeys | MISSING | Implement and drive |
| IW-6 | HIGH | Human capabilities, questions and operations | Honest diagnosis with public repair; settings/records/maintenance | Health/config/project adapters | Intent operation adapters | TestIntentOperationsJourney; TestIntentRepairAuthority | Synthetic config/health/record and exact recovery | MISSING | Implement and drive |
| IW-7 | HIGH | Results, recovery and migration | Full capability and truthful results without required internals | Descriptor and result owners | intent.go and each typed outcome adapter | TestIntentCapabilityPreservation; TestIntentPublicContinuations | Repeat complete fresh-orientation journey corpus | MISSING | Implement and drive |


Named fixture obligations from final design critique:

- IW-C7 / `TestRevisionRetainsReviewedFindingsAfterFailure`: review attempt 1,
  bind decisions, fail correction 2 without result, explicitly retry after 2;
  launch once with the original findings and decisions. Refuse that file once a
  later authentic review supersedes it. Unresolved uncollected findings qualify.
- IW-C8 / `TestIntentGoalEventWait`: wait for queued landing and filtered human
  acts through `--for`/`--since`, retain legacy aliases, and show the queued hint.
- IW-2 / `TestNamedWorkSelection`: collected/published, collected/unpublished and
  built/unread work coexist; selection consults or retains the read owner's binding.
- IW-2 / `TestRevisionRequestReplay`: crash after request retention before launch;
  retry and a concurrent identical request admit exactly one launch.
- IW-3 / `TestIntentReviewRetryAuthority`: live old critic refuses retry, proved
  stopped admits within the same chain cap, replay rejoins even after retry failure.
- IW-4 / `TestIntentQueueOnly`: a second landing slot refuses with the earlier
  goal's public land command; no proof or publication runs during queue-only.
- IW-6 / `TestIntentRepairAuthority`: every source-grounded administration choice
  exercises its exact inputs with and without the actor proof its owner requires.

Implementation units, each with its own tests: discovery/public projection
(about 1000 changed lines), complete delivery (about 1800), human/questions/
operations (about 1800), and integration/docs/remaining user-journey corrections
(about 900). Estimates are not caps. No test weakening: expectations that demanded
all 48 commands on root or manual close are intentionally superseded by the user's
new contract; retain capability, custody, authority and replay assertions.

Fable critiques this complete design (at most two substantive rounds on one
thread). Root dispositions every finding. Usability defects against this user's
explicit requirements are material even when the old workflow could be made to
work by an expert. Opus implements; independent Sol critique first checks the
whole diff against this design, then defects. Root drives the built CLI and real
owner fixtures, not just mock command routing.

After implementation, repeat the same human and agent journeys from a fresh
orientation: create/steer/authorize work, build/critique/revise/deliver, ask/wait/
answer, inspect/stop, and diagnose/recover. Include invalid input, multiple work
items, lost responses and authority refusal. Record observed pain and preserve
the best verified candidate. A new substantive gap gets an explicit design
amendment, Fable critique where behavior changes, implementation and verification.
Do not repeat broad tests for unchanged inputs. Stop only when all required
journeys need no internal knowledge, all existing meaningful capabilities have a
public route, all critical/high proof obligations are done, no material critique
remains, and root's repeat usability pass identifies no further justified change.
This is the user's iteration authorization; no extra approval is requested for
ordinary corrections within this contract.

## Executable namespace correction from the fresh usability audit

The same explicit-subject rule accepted for manual `review goal G` applies to
status and wait: `status goal G [--work NAME]` shows goal work; `wait goal G
[--work NAME]` waits for work, and explicit `--for landing|human-act` selects the
existing event observer. Legacy flag event selectors keep their defaults. Public
continuations qualify reserved goal names; lookup never guesses from whether a
goal happens to exist. TestIntentReservedGoalNames is an IW-1/2 proof for ui,
checkout, job, unit, file, proof, question and resume, alongside original target
behavior. This bounded dispatcher correction preserves valid goal capability;
it adds no owner, state or authority and goes to mandatory implementation critique.

## Truthful review-close refusal boundary

The existing whole-close shell flattens later failures, so closability, caller
identity or opening a job file cannot establish a failed operation's cause.
Preflight the actual record-writer authority owner for the root before the join
may write its register, using the executing engine's actual classification and
all custody fields. A refusal there is authoritative; unreadable classification
is uncertainty. Preserve all actual close checks and report later failures as
completion pending with owner detail and public repair. Supplemental CloseCheck
facts do not justify a guessed permission/I/O label. This bounded owner-boundary
correction replaces posthoc guessing, requires no new wire protocol, and is part
of IW review proof and mandatory Sol critique.
