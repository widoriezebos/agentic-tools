# Complete tasks through intent

- Kind: design
- Id: 01M3EC3QT7M2TC36P7ZVNRF0RW
- Status: draft
- Goals: verbs-match-intent

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
are not options. Detailed formats include a complete runnable example.

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
metasystem revise G [--work NAME] --brief FILE [--dispositions FILE]
metasystem wait G [--work NAME] [--timeout DURATION]
metasystem land G
```

A work name is a caller-chosen part of a goal, not a run or chain identity.
The first unnamed build uses `main`. Existing positional G UNIT stays compatible.
Later commands omit the name only if exactly one relevant work item exists;
multiple items produce their names and exact public commands, never pick latest.
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
Revision repeats through a new small UnitRunner owner operation: retain the
revision request identity (goal/work, prior reviewed subject or build result,
brief bytes and disposition digest) in the existing run record under its lock
before any new round. A retry rejoins that recorded round even after completion.
Changed input is a new explicit revision only against the current subject;
stale input refuses with the current work and no effects. Reusing the same brief
for a later genuine revision requires the new subject, so hashes alone are not
global deduplication. Lost responses and concurrent calls cannot launch twice.

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
finding, then closes/collects/publishes through the same owner. No separate close
command or second collection invocation is required after the decision.
A partial close or publish repeats the same public request and cannot falsely
claim that review is accepted. Failed/missing critic returns remain failures.

`revise` validates supplied findings decisions against the exact review when
there is one, then invokes the appropriate existing follow-up owner. A build
failure with no review permits a correction brief alone. It never erases the
old review, auto-refutes a finding or grants a risk acceptance. A changed result
needs a new independently reviewed subject. For a design review, the author
edits the design and invokes review again; explain that user action without
mentioning implementer-chain absence. For explicitly selected legacy job
reviews, use the existing follow-up owner behind the same revise intention.

Land retains its exact-candidate proof and admission rules. It handles mechanical
read collection and readiness/handover when the required decisions are already
recorded; it never invents dispositions or silently starts an unrequested paid
review. Missing review points to `review G`; multiple unread work items are named.
`land G --prepare-only` preserves the old readiness-without-landing capability.
Goal conclusion remains `done G`. Supplying proof for a prior review obligation
becomes `review G --finding F --test NAME` with review selection inferred only
when unique; advanced explicit review/artifact evidence remains available with
user-facing meanings and the old authority checks.

## Human capabilities, questions and operations

Existing goal creation, editing, approval, budgets, pause/resume, abandonment,
reopening, dependencies, priority, assignment, splitting, grouping, grants,
notes and conclusion stay explicit public capabilities in focused help. Their
less frequent verbs need not appear on the root page. `split` documents and
emits an example of its actual input format; no instruction says to discover an
internal manifest. Grouping describes related goals, not storage arcs. No
mandatory terminal question is added to otherwise complete scripted input.

Use `accept-risk G --finding F [--review R] --reason TEXT` for the existing
reserved act; infer the review only when unique. `incidents` lists incidents,
`incidents claim I --goal G` and `incidents close I --reason TEXT` preserve
ownership/closure authority. `show`/`status` present goals, work, questions,
project records and machines using recognizable targets. List current work to
make cancellation/status discoverable without knowing the launch store.

`ask` retains its authenticated delivery. `show question Q`, `wait question Q`,
`answer Q [TEXT]`, and `ask --retry Q` complete its public path. Resolve question
references through channel and mission owners: exactly one match wins; ambiguity
lists explicit public choices, never chooses a store by precedence. Mission
questions may use M/Q as a caller reference. A channel answer continues to direct
the person to the authenticated thread, with its concrete link/instructions;
plain local text does not manufacture authentication. A mission answer uses its
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

| Obligation | Severity | Owner/code | Required observed behavior | Focused/runtime proof | Status |
| --- | --- | --- | --- | --- | --- |
| IW-1 | HIGH | intent descriptors, main help, ui_describe, uitools kit | Small orientation and full public discovery; no internal catalogue or doubled command; no-arg success | Real binary help corpus and real Partner catalogue | MISSING |
| IW-2 | CRITICAL | UnitRunner named selection and revision | Correct goal/work; ambiguity and stale/corrupt/foreign data refuse; repeated/concurrent revisions launch once | Owner tests and CLI fixture with lost response | MISSING |
| IW-3 | CRITICAL | reviewUnit/commitReview and full close owner | Clean review closes/collects; findings require actual exact-subject decisions; all completion survives retry | Connected build/review/revise/land fixture | MISSING |
| IW-4 | CRITICAL | goal/landing intent adapters | All lifecycle and authority capabilities retained, readiness and exceptions explicit, no silent grants | Human/agent authority matrix, partial publication and landing replay | MISSING |
| IW-5 | HIGH | channel/mission/process adapters | Ask/show/wait/answer/retry/withdraw and mission recovery need no internal commands, preserve authentication | Fake-provider CLI journeys; ambiguity and failed delivery | MISSING |
| IW-6 | HIGH | health/config/project/operation adapters | Diagnosis leads to public repair; settings and records discoverable; maintenance flags preserve exact human choice | Synthetic config/health/record and recovery journeys | MISSING |
| IW-7 | HIGH | result projection and capability catalogue | All old meaningful capabilities reachable; ordinary results and continuations shield internal structure | Inventory joins, behavioral fixtures and independent fresh-user read | MISSING |

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
