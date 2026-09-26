# Root audit of every public verb

Scope: all 44 current public commands, their usage, details and exposed flags,
read from the built CLI and traced at the relevant owners. Evidence directory:
`/Users/wido/LocalStorage/agentic-tools-evidence/agent-help-20260926`.
Fable 5.1 independently audited the same inventory (launch
`20260926t183643-80fefd98bd`); its report is `verb-portability-fable-audit.md`.
Opus 5.5 traced actual alias callers (launch `20260926t183931-ea2c2ab69e`).
This is root's adjudication, not wholesale acceptance of either report. It
records the cleanup design and must be joined to final implementation evidence.

The deciding question is the subject of the action. Managing application goals,
review, tests, authority and delivery works across applications. Authenticating
or configuring the work system itself is justified administration and belongs
in a separate section. A generic verb may have both kinds of form. No whole
workflow command is deleted merely because its implementation has a technical
owner. No historical alias survives solely for an absent outside installation.

| Verb | Decision | Reason / required correction |
| --- | --- | --- |
| goals | Generic application workflow | List the open goals. The subject is the application's work or a decision about it. |
| show | Generic application workflow | One goal's record, a project record, or a question. The subject is the application's work or a decision about it. |
| approve | Generic application workflow | Approve goals for execution. The subject is the application's work or a decision about it. |
| budget | Generic application workflow | Read a goal's budget, or give it a box. The subject is the application's work or a decision about it. |
| pause | Generic application workflow | Park a goal with a reason. The subject is the application's work or a decision about it. |
| resume | Generic application workflow | Resume a parked goal, or a stopped goal under its standing box. The subject is the application's work or a decision about it. |
| done | Generic application workflow | Conclude a goal with its conclusion. The subject is the application's work or a decision about it. |
| start | Generic, with separated administration forms | Keep starting an assigned session/mission as work; separate checkout supervision, interface and machine provisioning. |
| stop | Generic, with separated administration forms | Keep stopping a job, review, design or attended session as work; separate whole-checkout and interface shutdown. |
| restart | MetaSystem administration | Both forms restart MetaSystem supervision or its interface, not the application. |
| status | Generic, with separated administration forms | Keep goal/job/work/mission status; separate checkout supervision, interface and fleet status. |
| enroll | MetaSystem administration | Authenticating a human terminal belongs to MetaSystem setup; it cannot be represented as approving an application goal. |
| ask | Generic application workflow | Ask the person a question through the channel. The subject is the application's work or a decision about it. |
| answer | Generic application workflow | Answer a question, or see where a channel question is answered. The subject is the application's work or a decision about it. |
| check | Generic, with separated administration forms | Keep previewing goal edits; separate diagnosing MetaSystem and validating its configuration. |
| open | Generic application workflow | Provenance records whether the goal came from a person or the main session; it applies to any application. Keep advanced --origin, not an engine administration action. |
| edit | Generic application workflow | Recurring responsibilities and their proof limits can belong to any application. Advanced obligation fields are not automatically engine-only; preserve them with a clear purpose. |
| claim | Generic application workflow | Claim a goal for this session, or the next ready goal. The subject is the application's work or a decision about it. |
| release | Generic application workflow | Release a claim this session holds. The subject is the application's work or a decision about it. |
| accept-risk | Generic application workflow | Accept the risk of one severe or unproven review finding. The subject is the application's work or a decision about it. |
| pin | Generic application workflow | Assigning work to a machine is legitimate application scheduling; no engine knowledge required. |
| prioritize | Generic application workflow | Place an open goal in priority 1, 2 or 3. The subject is the application's work or a decision about it. |
| reopen | Generic application workflow | Return a done or abandoned goal to the queue with a fresh next step. The subject is the application's work or a decision about it. |
| abandon | Generic application workflow | Generic user intent. Remove the unrelated engine-floor rollout prerequisite while retaining human proof, dependencies and history. |
| block | Generic application workflow | Record that a goal waits for another. The subject is the application's work or a decision about it. |
| unblock | Generic application workflow | Remove one blocker from a goal. The subject is the application's work or a decision about it. |
| unapprove | Generic application workflow | Withdraw a goal's execution approval. The subject is the application's work or a decision about it. |
| grant | Generic application workflow | Delegated authority for approval/budget/resumption is governance of application work, not engine administration. |
| revoke | Generic application workflow | Ending a delegated authority is a distinct human intent; preserve. |
| split | Generic application workflow | Split a goal into independently claimable related goals. The subject is the application's work or a decision about it. |
| group | Generic application workflow | Put a goal into a group of related goals. The subject is the application's work or a decision about it. |
| ungroup | Generic application workflow | Take a goal out of its group of related goals. The subject is the application's work or a decision about it. |
| notes | Generic application workflow | Read, add or close a goal's non-breaking read findings. The subject is the application's work or a decision about it. |
| repair | Generic, with separated administration forms | Keep interrupted goal/wait/review and workspace recovery; separate reviewed storage-history acceptance and format migration. |
| incidents | Generic application workflow | A broken integration branch is an application delivery problem; retain claim/close and their authority. |
| design | Generic application workflow | Have a design author write a goal's design as a draft. The subject is the application's work or a decision about it. |
| brief | Generic application workflow | Write a brief scaffold from a goal and its accepted design. The subject is the application's work or a decision about it. |
| build | Generic application workflow | Build and test a goal's work, ready for independent review. The subject is the application's work or a decision about it. |
| wait | Generic application workflow | Generic waiting, including resuming a recorded wait. Fable misclassified wait resume as machine provisioning; intent_work.go runIntentWaitResume is the durable wait continuation. |
| test | Generic, with separated administration forms | Keep application testing, planning and proof inspection; document their public forms and distinguish contract maintenance. |
| settings | MetaSystem administration | These are the work system's configuration and coordinator settings; label the subject MetaSystem. Retire compatibility/engine-floor. |
| review | Generic application workflow | Independent review and finding proof apply to any application. Preserve advanced fixture evidence fields and authority; remove references to obsolete close/fold spellings. |
| revise | Generic application workflow | Correct a goal's work with a brief: one new attempt, reviewed again. The subject is the application's work or a decision about it. |
| land | Generic, with separated administration forms | Keep delivery and explicit human exceptions; label optional goal-format upgrade as administration. |

## Findings adjudicated

- **L1/L2, engine-floor:** source confirms the cross-repository ancestry problem;
  adoption builds at template HEAD and copies the engine, while an application
  rebuild stamps the application's HEAD. Equal stamps short-circuit, but a later
  comparison may name an object absent from the application repository. This is
  a functional defect, not merely wording. Reject Fable's suggestion to make
  users reassert another SHA. The obsolete rollout gate is retired under the
  user's explicit no-other-installations correction; real abandon safeguards stay.
- **L3, obligations:** reject the unproved claim that recurring obligations belong
  only to the engine. Application obligations can legitimately bind platform,
  toolchain, proof surface and exposure. Explain advanced purpose; do not mislabel
  application governance as tool administration.
- **L4, fixture proof:** preserve evidence needed to discharge a fixture finding;
  those fixtures may test another application. Advanced references identify proof,
  not permission. The common test-based discharge remains the obvious form.
- **L5, discovery:** add the missing review-reference help forms (show/wait/stop)
  and current test planning/verification/report guidance. Remove internal rollout
  recovery instructions with their obsolete owner. Do not redirect agents to a
  private technical reference for an ordinary task.
- **L6, legacy catalogue:** Fable's proposal to retain and advertise compatibility
  pages is superseded by the human's clean-house instruction. Public family-help
  aliases and the ten compatibility intent descriptors are removed. Current
  machinery handlers are retained for their proven in-repository consumers;
  explicit internal help is a maintainer reference, not a historical public API.
- **L7/L8, migrations and machine provisioning:** these change the work system,
  so separate and justify them. A current reviewed import/history repair remains
  useful; it is not ordinary application delivery. No automatic exception or
  authority is inferred from the section.
- **L9, origin:** recorded provenance is legitimate goal metadata. Keep its
  advanced form; do not invent a tool administration requirement.
- **Incorrect wait classification:** wait resume continues an interrupted durable
  wait, not start machine's provisioning session. Keep it in workflow help.
- **Authority correction:** administration is not universally human-only. Reading
  settings, diagnosis and lawful recovery may be agent acts. Preserve per-owner
  authority instead of Fable's proposed universal human-only sentence.

Opus found 10 compatibility intent descriptors, not 11, and no production caller
of those intent aliases. Their tests, returned guidance and separate ui-family
messages still need migration. Its recommendation to delete runIntentFleet and
runIntentRed outright is rejected: current status/incidents still use these
owners. Delete only unreferenced wrappers; keep shared behavior. Its fold-unit
capability concern must be resolved against the current goal/work revision route
before removing a runnable behavior; alias deletion alone is not proof of parity.

Private protocol census (regex bounds, not parser facts): about 834 shell and 91
Go production argv sites call the technical families. This is a real current
consumer, including pinned engines, unlike hypothetical outside installations.
Bulk argv renaming is outside this bounded cleanup. Human aliases and public help
are the user-facing debt being removed. Flags such as --repo/--root are forgiving
spellings of one value, not duplicate verbs; retain unless a concrete ambiguity
makes them defective.

No claim of having exercised all 44 mutating verbs in a foreign app: the audit
reads all descriptions and traces changed paths. Focused runtime fixtures and
regression tests supply the separate behavioral evidence.
