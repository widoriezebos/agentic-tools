# Application work without obsolete CLI choices

- Kind: design
- Id: 01M3FG9P2H020EKJDZHEFNMRB0
- Status: draft
- Goals: verbs-match-intent

Root authors this design under Wido's 26 September 2026 instruction. He explicitly
confirmed there is only this installation and authorized removing obsolete help
and spellings. This supersedes agent-help.md's byte-for-byte preservation rule
where the new administration separation or deletion requires a change. Capability,
human readability, authority and existing records remain the preservation contract.

## Step 1

The public CLI offers one current spelling for each supported task. Application
work stays generic. Help separately identifies operations whose subject is the
MetaSystem installation itself, including such forms of otherwise generic verbs.
There is no public historical family catalogue or obsolete intent alias. The
private machinery retains the technical handlers its scripts and processes call;
those are current internal implementation interfaces, not application tasks.

Do not migrate thousands of private process-protocol calls just to add a word to
their argv: their actual consumers, process identity and permission checks are a
real reason to retain the existing private protocol. Its one dispatcher remains.
The explicit `internal` entry is its maintainer discovery boundary. Public help
never falls back to it, and normal results never instruct a caller to use it.
This is an intentional boundary, not a promise of backward compatibility for
outside installations. Root audits actual private consumers before deleting an
owner; no adapter is retained solely for a hypothetical external script.

## Obsolete choices to delete

Remove compatibility intent descriptors ready, decide, resolve, recover, red,
fleet, doctor, ui, fold and close (verify the exact inventory in source). Keep the
existing current commands land --queue-only, accept-risk, review --finding,
repair, incidents, status --machines, check, start/stop/status ui, revise and
review. Delete descriptor compatibility/replacedBy machinery once no consumer
exists. Move tests to these real current routes, retaining authority, idempotency,
recovery and side-effect assertions. Delete alias-only dispatch code when unused;
shared execution owners remain under names describing their behavior.

Migrate all returned continuations and live docs/skills to current public syntax.
A returned job-review continuation must refer to the original reviewed job, or
to its existing subject route, never manufacture a new review of a critic job.
Keep the close-chain and correction owners used by review/revise; they still do
real work. Private script verbs named close/fold are implementation APIs, not
obsolete intent descriptors; do not delete them based on a word match.

`help FAMILY` and FAMILY --help no longer publish the internal family catalogue.
Explicit `internal FAMILY --help` remains a maintainer reference with its prefix
shown in usage/examples. Root help advertises current task topics. Unknown public
help refuses before execution, with public guidance. Help on removed intent
spellings refuses; no replacement alias shim or compatibility catalogue remains.
Internal help describes current machinery only and must not assert that every
internal operation is a recommended task.

## Justified administration

Reuse the command descriptor as the sole help owner. Add the administration topic
and put wholly administrative commands there: enroll, settings and restart.
Their rationale: authenticate the human terminal, inspect/configure the work
system, and restart that system/interface. Their subject cannot honestly be
confused with running or configuring the application being developed.

Mixed commands retain their generic names and application-work group. Separate
administrative usages within their help: start/stop checkout/session/ui/machine,
status checkout/ui/machines, check settings/system, and repair of goal-storage
migration/history. Preserve stop job/review/design, goal status, ordinary goal
recovery and review recovery in the work sections. Missions are assigned work,
not automatically administration. Scope is attached to syntax at the owning
descriptor, never guessed from words at runtime. Structured help exposes the same
separation. Full index remains complete; ordinary work lookup need not read admin.
`help human` also separates administration instead of flattening it into planning.

Use one small representation for these usage sections; do not create an action
policy, duplicated command registry, dynamic permission evaluator or parser.
A metadata field is justified only by these current mixed consumers. All usage
strings remain discoverable to the Partner and errors as well as text/JSON help.
Keep values such as --repo, review/disposition references and human authority
conditions: they identify the user's work or permission, not incidental storage.

## Retire the engine-floor rollout gate

The floor is a migration guard for engines predating the abandoned-goal state.
All current supported engines already understand that state; the person confirms
there are no other installations to preserve. An unrelated application must not
need a metasystem-source Git commit merely to say a goal will never be worked.
The current code compares the engine commit in the application's Git history and
scans the host registry without ledger selection. This is not a sound portable
application prerequisite. Do not replace it with another manual assertion verb.

Remove the engine-floor mutation, settings compatibility --minimum-engine, the
abandon floor prerequisite and its otherwise unused supervisor checker. Preserve
abandon's actual human proof, dependency dispositions, claim locks, publication,
history and recovery invariants. Historical engine-floor ledger lines remain
readable and validated; history is not rewritten and an old journal is not
silently replayed. Remove only obsolete floor-specific tests; replace acceptance
coverage with abandonment in an unrelated project with no engine Git history or
floor record, plus authority/dependent safety. No coverage floors are lowered.

## Audit, verification and completion

Record a verdict for all 44 public verbs and their exposed forms/options. Separate
retained administration with its reason, generic workflow, and remaining defects.
Read the fresh Fable audit and Opus caller facts; root decides. Fable critiques
this complete delta before implementation and the final computed diff afterward.
Two design rounds maximum (exit when obligations are bounded and fixture-ready),
three code rounds maximum (exit on zero material findings). Materiality: would the
change ship a defect, violate its brief, or damage what certifies it? Optional
framework polish never extends the loop. New concrete loss or leaked internal
prerequisite must be fixed or explicitly left open, never declared fully shielded.

Focused tests first: public/maintainer help boundary, complete catalogue, mixed
usage sections, all removed aliases refused, review/revise/landing journey and
returned argv, unchanged human authority, abandonment with no floor and preserved
historical records. Run the real CLI outside a repository for read-only help,
current machine JSON/text, and a synthetic unrelated app for the changed abandon
adapter. Select final regression proof after the final source is stable; the
initial agent-help command-suite run is evidence only for its own source.

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| VC-1 | HIGH | Obsolete choices | One current public route per task, no legacy help or returned aliases | CLI intent/dispatch | cmd/metasystem/intent*.go and main.go | refusal, connected workflow and capability tests | actual help/continuation outputs | PARTIAL | Critique then implement |
| VC-2 | HIGH | Administration | Separate justified administration without losing mixed-command work | command descriptors/help | intent.go and intent_help.go | text/JSON/Partner catalogue checks | actual human/agent help trial | PARTIAL | Critique then implement |
| VC-3 | HIGH | Floor retirement | Abandon works without engine history, retains authority and dependency safety | goal abandon owner | internal/goal/abandon.go | no-floor, authority, history and dependent tests | unrelated repository adapter | PARTIAL | Critique then implement |
| VC-4 | HIGH | Audit | Every public verb assessed; current callers migrated, private consumers retained intentionally | root audit | plans/verb-portability-audit.md | current surface and caller checks | built CLI and fresh critique | PARTIAL | Complete audit |

Deferred: bulk renaming of private process protocols has no application consumer
benefit in this slice and risks process identity/permission regressions. A future
change must use the existing internal dispatcher and owner-specific callers,
not add a second protocol or promise compatibility. No natural-language router,
automatic authority, or universal effects/retry schema is introduced.
