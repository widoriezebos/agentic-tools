# Application work without obsolete CLI choices

- Kind: design
- Id: 01M3FG9P2H020EKJDZHEFNMRB0
- Status: done
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

Remove the ten compatibility intent descriptors ready, decide, resolve, recover,
red, fleet, doctor, ui, fold and close (54 total descriptors become 44). Keep the
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
administrative usages within their help: start/stop checkout/ui/machine,
status checkout/ui/machines, check settings/system, and repair of goal-storage
migration/history. Preserve stop job/review/design, goal status, ordinary goal
recovery and review recovery in the work sections. Sessions, durable wait resumptions and missions are assigned work,
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
floor record, plus authority/dependent safety. No coverage floors are lowered. Preserve root.go history validation and recover.go
refusal to replay obsolete engine-floor journals. Remove both read and write
settings compatibility. Update scripts/agents/land-fixtures.sh and
scripts/agents/goal-cli-fixtures.sh: remove only floor setup and retain every
dependent-safety and landing assertion after it. Run both sections.

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
| VC-1 | HIGH | Obsolete choices | One current public route per task, no legacy help or returned aliases | CLI intent/dispatch | cmd/metasystem/intent*.go, main.go, internal/ui/act, internal/ui/lifecycle and internal/dispatch/read_admission.go | 173 current command tests pass (cleanup-command-assessment.json); changed message-owner suites pass | 201 actual CLI help/refusal checks; plans/verb-cleanup-verification.md | DONE | None |
| VC-2 | HIGH | Administration | Separate justified administration without losing mixed-command work | command descriptors/help | intent.go and intent_help.go | final text/JSON/Partner catalogue checks pass | actual human/agent help and fresh Opus trial; plans/verb-cleanup-agent-help-trial.md | DONE | None |
| VC-3 | HIGH | Floor retirement | Abandon works without engine history, retains authority and dependency safety | goal.Abandon | internal/goal/abandon.go | internal/goal full suite with race and coverage; TestAbandonNeedsNoEngineHistoryInAnUnrelatedProject | 29 goal CLI scenarios and 45 combined landing scenarios; existing skipped recertification subcase explicitly bounded in plans/verb-cleanup-verification.md | DONE | None for this behavior; retained fixture gap is documented |
| VC-4 | HIGH | Audit | Every public verb assessed; current callers migrated, private consumers retained intentionally | cmd/metasystem descriptors and root audit | plans/verb-portability-audit.md | current surface and caller checks pass (cleanup-runtime-help-results.json) | built CLI; plans/verb-cleanup-fable-code-review.md and design round 2 have zero material findings | DONE | None |

Deferred: bulk renaming of private process protocols has no application consumer
benefit in this slice and risks process identity/permission regressions. A future
change must use the existing internal dispatcher and owner-specific callers,
not add a second protocol or promise compatibility. No natural-language router,
automatic authority, or universal effects/retry schema is introduced.


## Finding dispositions and exact capability mapping

Fable design round 1 (`20260926t184245-603bfab6fa`) found three bounded material
omissions. Root accepts all three. VC-D2 (shell fixture consumers) and VC-D3
(UI/evidence-owner continuations) are amended above and ready to implement.
VC-D1 is resolved by the explicit mapping below; Fable round 2 (`20260926t185228-8e0f93ded1`) verified every mapping and found
zero material findings. The design loop is closed and all portions are ready to
implement. Root still owns the computed-diff implementation review and proof.

| Removed form | Current task and retained owner |
| --- | --- |
| ready [G] | land G --queue-only, same landReady and human authority; omitted G inference is existing claim information, not a lost operation |
| decide G --review R --finding F --reason TEXT | accept-risk G with the same review/finding/reason and provenance flags; it already calls runIntentDecide after subject inference |
| resolve G --review R --finding F --test NAME | review G --review R --finding F --test NAME, same runIntentResolve |
| resolve G with implementation-chain/artifact/result/critic | review G --finding F with the same four advanced proof fields and --review R; same owner, test/fixture alternatives exclusive |
| recover [--session S] | repair goals and repair waits [--session S]; both real recovery owners remain, neither makes a finding decision |
| red own I --goal G [--branch B --to MACHINE --by NAME] | incidents claim I with the same options and authority; same runIntentRed owner |
| red close I --reason TEXT | incidents close I --reason TEXT, same owner and human proof |
| fleet | status --machines, same runIntentFleet owner |
| doctor | check, same runIntentDoctor owner |
| ui [status/start/stop/restart] | status/start/stop/restart ui; preserve --listen on start/restart, --wait-seconds on stop/restart, and installation selection. Retain only the actual serve/tools family consumers; delete unused lifecycle wrappers. |
| fold review R --dispositions FILE --brief FILE | revise job R with the same fields; same foldReview owner |
| fold unit RUN --brief FILE | revise run RUN --brief FILE, same retained Continue/FollowUp owner; run is the returned execution reference |
| close J [--dispositions FILE] [--reconcile-evidence R] | done job J [--dispositions FILE] [--evidence R], same complete closeChain owner, including authority/terminal/evidence requirements |

`done job J` completes exactly the returned job's records; it does not conclude
its goal, land, or grant approval. This describes completion directly rather than
pretending an investigator/implementer job is a new review. `done G --reason TEXT`
keeps its existing goal semantics; the two-word job form is unambiguous. Preserve
qualified job references and resolve the store; unsupported launch jobs refuse
before effects. --evidence is a review reference forwarded to the same private
reconcile-evidence mechanism, never proof accepted on trust. Evidence/dispositions
are refused on the goal form; goal-only options are refused on the job form.
Existing `review job <critic> --dispositions` still completes that review as part
of reviewing; it must not start a review of the critic.

`revise run RUN` corrects a retained run using its original plan/proof/approved
round limit. Reject conflicting goal work/after/dispositions options explicitly;
retain its immutable-input, request repetition and budget checks. Expose the
matching current `review run RUN` and `wait run RUN` forms by routing to the
existing unit-review/wait owners. Public unit spellings and returned unit words
become run, including living docs/skills; internal unit API and historical records
remain implementation/history. Goal-directed build/revise/review/wait stay the
primary route; no caller must manufacture a run ID.

VC-D3 recovery text uses `done job` (optional --evidence) or the subject's review
form and public UI verbs. Test the actual changed message producers; avoid a
brittle global prose linter that mistakes private protocols or historical records
for application guidance. Their concrete current output is the requirement.

Dry-run note: Wido asked for an effort estimate, not implementation. A future
check-command feature must reuse actual verb validation and distinguish syntax,
current prerequisites and unexamined effects. It is not added to this cleanup.

Root implementation finding: the private UI lifecycle routes accepted a listen
address and shutdown duration that the ordinary forms did not expose. Preserve
those controls on the ordinary administration forms, using the same validated
listen owner and lifecycle options. Refuse those flags on other targets before
effects. Retire the unused family lifecycle wrappers; serve and tools retain
their real process callers. This is capability preservation, not a new feature.
