# Role context composition

Working Mode: design

Goal: `role-context-composition`

Authority: Wido's ruling of 2026-08-28, paper chapters 7 and 8,
Rulings A-S, and R-11. L13, the `delegate` verb, is the enforcement
point. This file authorizes no build.

## Decision

Every delegated job receives one closed role packet inside one enforced
capsule. One delegation record binds the packet, the capsule, every later
delivery, the exact candidate, and the result.

The packet limits what the job is deliberately told. The capsule limits what
the job can reach. The record proves what crossed that boundary. None of the
three substitutes for another.

An independent examiner is admitted only when all three are complete. A new
chat, a narrow prompt, a clean worktree, a list of canaries, or a runtime's
claim that read roots are mapped is not enough. If the isolation or provider
boundary cannot be proved, `delegate` may run the job only as an advisory
pass. Its result cannot enter custody as independent examination.

This is the mechanical fresh-mind law. An examiner starts in a newly created
capsule and a newly created model inference lineage. It receives no builder
path and cannot reach one. A candidate change starts another fresh examiner;
the former examiner's path is not carried forward.

## One boundary: the role capsule

The enforcing primitive is an ephemeral virtual machine, not a directory
convention. The accepted capsule launcher creates a clean VM for each job
from a measured, immutable runtime image. The VM has:

- one read-only mount containing the closed role packet and the exact subject
  snapshot;
- one empty, disposable scratch volume;
- no host, home, repository, Git, sibling-worktree, job-artifact, device or
  container-control mount;
- no inherited file descriptor, host socket, provider credential, shell
  startup file, user configuration, project memory, plugin or MCP
  configuration;
- a fixed working directory and a closed environment; and
- no general network route.

The hypervisor boundary, not a sentinel test, prevents host reads. The
launcher closes descriptors before boot and records the VM image, launcher,
hypervisor, host build, mounts, devices, sockets, environment names, command
and working directory. A change to any measured part creates a new runtime
profile and invalidates earlier qualification.

The effective VM configuration, rather than the launcher's requested
configuration, is attested by the accepted hypervisor before any task byte is
released. The packet stays broker-side until that attestation and the pending
delegation record have committed. A hypervisor that cannot attest its actual
mounts, devices, sockets and network routes is not a qualifying boundary.

The agent does not run a provider CLI. L13's broker is the only metasystem
principal allowed to hold provider credentials. The capsule can ask the
broker for a model turn or an admitted tool action through one narrow
channel. It cannot open another model, native subagent, network connection or
host command. Provider CLIs, native subagent facilities and provider tokens
are absent from the capsule.

Each model turn is made through a stateless interface. L13 supplies the full
allowed transcript on every turn and records the exact request and response.
Provider threads, account memory, project memory, hosted tools, automatic
retrieval and provider-created compaction are disabled. A retry is a new
request over the same recorded transcript; provider retry and diagnostic text
does not become model input.

An inference profile qualifies for independent examination in either of two
ways. A local model runs from a measured image inside the capsule, or a remote
provider returns a verifiable input-completeness attestation that identifies
the endpoint and profile and states that no account memory, thread history,
server tool, generated summary or other task input was added. A remote
profile without that proof may build or advise, but it may not supply an
independent examination. Provider safety machinery that cannot be measured
is outside the proof boundary; a profile that cannot show that it is
task-independent is refused for the examiner role.

Negative canaries are qualification tests for the exact measured launch
tuple. They cover an absolute host path, an ancestor instruction file, user
and project memory, Git logs and objects, a sibling checkout, another job,
shell startup state, an inherited descriptor, an unlisted socket and an
unlisted network destination. They do not enforce the boundary and never
turn an otherwise open runtime into a qualifying one.

## Closed role packets

There is no general allow table and no free-form source grant. Each role has
one fixed packet recipe. The recipe names every field and admits no
implementation-selected projection.

Every agent packet begins with the exact task direction, role instructions,
required skill instructions, response contract, tool names and generated
runtime notices. Those bytes are accepted engine material selected by the
role and hazard recipe. A coordinator cannot add a skill, notice or source at
dispatch. Task-specific history is not a skill instruction.

The public `--brief` file required by Ruling B contains only the task
direction. It contains no source list or copied context. For an independent
examination its bytes and hash are part of the accepted goal revision before
construction begins. A later coordinator explanation is not an examiner
brief.

The rest of each packet is fixed as follows.

| Function | Exact input | Permitted effect |
| --- | --- | --- |
| Builder | Goal id and revision; outcome; constraints; acceptance-condition ids and text; hazard class; cited law, ruling and task-document blobs; immutable base snapshot; same-lineage builder continuation when present; accepted examination findings bound to the parent candidate when the task is correction. | Write a private candidate workspace and builder record; run admitted tools. It cannot examine, accept, land or alter live state. |
| Independent examiner | Goal id and revision; outcome; constraints; acceptance-condition ids and text; cited law, ruling and task-document blobs; exact normative candidate snapshot with base and digest; raw captured results for the predeclared evidence conditions. | Read the candidate; write scratch data and an examination result; run admitted read/test tools. It cannot change the candidate, read construction history, accept or land. |
| Coordinator | Goal id, revision and state; job id, role, state and typed outcome; candidate, evidence and examination ids, digests and states; the next permitted transition. | Request a transition. It cannot create qualifying evidence, examine a candidate it directed, accept it, or read sealed delegation payloads. |
| Custodian | Goal id and revision; exact candidate and base digests; complete required-condition id set; each required evidence and examination id, candidate digest and pass/refuse state; reversal-proof id and state when required; delegation proof state; proposed custody transition. | Append the authorized acceptance or refusal for that exact tuple. It cannot explore, reinterpret, waive, repair or substitute. This is a deterministic function, not an agent session. |
| Steward | Job id; lifecycle state; heartbeat and lease times; budget state; capsule state; candidate digest-change flag; permitted recovery action. | Probe, stop, restart or reassign through the existing narrow authority. It cannot read task, prompt, tool or result bytes and cannot erase history. |
| Narrator | Accepted intent fields; typed state transitions; ruling, candidate, evidence, examination and custody ids and states; source links for every material claim. | Produce an account. It cannot change, accept, release or hide state. |
| Auditor | One named incident or delegation record, including its sealed packet, event payloads and boundary proof. | Reconstruct and report read-only. It cannot continue, accept, mutate or release the work. |

The full required-condition set, candidate binding, examination binding,
reversal proof and context proof are mandatory custodian inputs. An omitted
field refuses acceptance; a wider field set is not another conforming
implementation.

## Necessity, not topical relevance

The accepted goal revision owns a dependency list before construction. Each
law, ruling and task document must be cited by one exact constraint or
acceptance condition. Each offered result must satisfy one evidence condition
declared in that same revision. L13 derives the packet from those links. The
coordinator cannot select additional sources, and the builder cannot add a
persuasive result whose evidence condition was not already named.

The task direction, role instruction, response contract and tool surface are
fixed by the role and hazard recipe. The goal-linked dependency closure, the
exact subject snapshot and the required evidence closure are the only other
slots. A byte that fills no slot is refused even if it is relevant by topic.

If a needed dependency was omitted, the authority revises the goal. A goal
revision invalidates later packets and any examination or custody claim made
against the earlier revision. The machinery proves this declared necessity
closure. It cannot prove the semantic claim that a human-authored sentence
was truly indispensable; responsibility for the preconstruction dependency
decision remains with the authority that accepts the goal.

The subject snapshot is a unit, not a route to the repository. For code it is
the exact candidate tree without `.git`, records, plans, other worktrees or
job artifacts unless one of those paths is itself explicitly part of the
product candidate. Tools may inspect that mounted snapshot and their measured
toolchain, nothing else.

For prose and design work, candidate publication separates the normative
candidate from design history. A Markdown candidate has a separately hashed
normative range; critique dispositions, rejected options, construction notes,
defences and earlier examination results are outside that range. L13 refuses
an unstructured mixed artifact for independent examination. This file's
normative range ends before the `Round 1 dispositions` heading below.

Range separation proves that excluded bytes were not delivered. No byte-level
system can prove that allowed free prose is not a paraphrase of forbidden
reasoning. A candidate whose independence depends on that semantic judgment
requires a responsible human examiner or a structured candidate form; the
record must say which limit applies. It may not report mechanical proof of
semantic cleanliness.

## Tools, results and actions

Tool output is not classified by tool identity. Examiner tools run inside the
same capsule against the read-only candidate and measured toolchain. Their
stdout and stderr may be arbitrary, but the processes that produced them
have no readable source containing builder history, host configuration or
credentials. Provenance comes from bounded reach, not a semantic label.

Every tool event records the executable digest, arguments, working directory,
environment names, candidate digest, exit state, and exact stdout and stderr
bytes. The event is appended before those bytes enter the next model request.
If recording fails, delivery stops.

Host, launcher and provider diagnostics are control-plane output. They are
never returned to the model and are not copied into the delegation's context
attachments. The broker maps them from an ephemeral buffer to a typed failure
and control-event digest, then discards the buffer. Any independent platform
logging is an operator-controlled incident surface and is never mounted into
a capsule. This prevents a withheld error from becoming a durable context
leak.

The examiner has no arbitrary shell, editor or write-file tool. Its recipe
contains read and search operations plus exact predeclared test commands.
The candidate mount is read-only. Compilers and tests must put caches and
generated output in scratch. A test that requires writing the candidate is
incompatible with independent examination until it supports a separate
output directory. The candidate digest is checked again before the result is
sealed. A finding may propose a repair, but the examiner cannot perform or
test one. Any repaired candidate goes to a builder and then to a new
examiner.

The builder receives a writable candidate workspace but no acceptance,
landing or live-state capability. The custodian and steward have only the
listed deterministic transitions. These action limits make chapter 7's
prohibited combinations mechanical rather than a role-name convention.

## One authoritative delegation record

The `DelegationRecord` is the only context authority. It is append-only, and
its attachments share the job identity and transaction boundary. The public
brief does not duplicate its source declarations.

The record moves through pending, refused, running, completed or breached.
A refusal is a terminal record, so failed admission remains auditable without
a second store. Each append commits the state, event and referenced attachment
hashes together. Recovery trusts the last complete append and never
reconciles competing context authorities.

Its sealed audit section contains:

- the accepted engine generation, role and hazard recipe, goal revision,
  task-direction hash and subject identity;
- every packet field and byte range, its owner record, source digest and the
  exact assembled model request;
- the normative candidate range, excluded-range manifest, base and candidate
  snapshots and digests;
- role, skill, response-contract, working-directory and generated-notice
  bytes and digests;
- VM image, launcher, hypervisor and host measurements; the complete mount,
  descriptor, socket, device, environment and network manifests; and the
  effective-configuration attestation and exact qualification proof for that
  launch tuple;
- the qualification run time, measured tuple, canary locations, unique
  canary digests and each denied-read result;
- model location, endpoint, profile, fresh-lineage identity and local
  measurement or provider input-completeness attestation;
- every model request and response, every delivered tool stdout and stderr,
  and an unbroken prior-event hash;
- the candidate's final digest, the exact result bytes and terminal state.

The ordinary projection exposes only record id, role, goal revision,
candidate digest, proof state, result digest, lifecycle state and refusal
code. Coordinator and custodian receive that projection. They cannot open
the prompt, locators, packet bytes or event payloads. Only the named auditor
can open the sealed audit section. Thus a composition record cannot launder
builder or examiner paths into a role whose packet excludes them.

### What the audit proves

An auditor reports `no-leak-proven` only after all of these checks succeed:

1. Rebuild the packet and every model request from retained bytes and match
   every hash and range.
2. Show that each packet byte fills one fixed recipe slot and that no design-
   history range entered the normative candidate.
3. Verify the accepted launcher, complete VM measurement and hypervisor
   attestation of the effective configuration, including the absence of host
   mounts, inherited descriptors, unlisted sockets, devices and network
   routes.
4. Verify a new model lineage and either local measured inference or the
   provider's input-completeness attestation.
5. Replay the complete event chain and show that every byte delivered after
   launch was a recorded model response or output from a process confined to
   the capsule.
6. Match the candidate before tools, after tools and in the examination
   result.

A missing attachment, discontinuous event chain, stale qualification,
unmeasured control path, provider session, server-created summary, unknown
descriptor, writable candidate or incomplete provider proof produces
`no-leak-not-proven`. It never produces a clean result by absence of an
incident. Custody and `land` accept independent examination only when the
record is completed, its result and candidate hashes match, and its proof
state is `no-leak-proven`.

This proof is against the recorded threat: builder and repository context
crossing into the examiner through system-controlled inputs. It trusts the
accepted engine, hypervisor measurement and, for a remote model, the named
provider attestation. A malicious host administrator or dishonest provider
is outside that proof. The record states those trust roots rather than
claiming physical certainty it does not possess.

## L13 exclusivity and cutover

L13 is exclusive in two independent ways.

First, the provider credential and model egress belong only to its broker.
Agents have neither. Direct provider CLIs, APIs and native subagents therefore
cannot be invoked from a delegated job.

Second, custody and `land` reject every claimed examination that lacks a
completed L13 record with matching goal revision, candidate digest, result
digest, examiner recipe and `no-leak-proven` state. Text produced outside
L13 is ordinary advice, even if it resembles a valid return. With no L13
record it is untrusted, not silently clean.

Rulings N and R require the replacement and caller cutover in the same
landing. The mechanical caller inventory covers raw dispatch and follow-up
assembly, mission prompts, resident hosts, continuation roles, critique and
verification helpers, adapters, schema and hook builders, native subagents,
provider CLIs, templates, fixtures, documentation and resident process
generations. Old prompt assembly and agent-visible provider credentials are
removed; there is no compatibility path. Stale residents refuse until
rearmed on the accepted generation.

Ruling M still applies: the job enters the steward's watched set before the
first model request. Ruling O fixes the role and inference configuration from
the hazard recipe. Ruling S requires judging and landing capacity before a
batch launches. Context composition does not create a second budget, custody
or authority system; it uses the accepted ones.

## Honest bootstrap before L13

Before the capsule and L13 custody gate land, recorded conduct can narrow a
brief. The coordinator can use a new provider session, send only accepted
intent, a normative candidate projection and required raw evidence, omit
known construction history, disable visible tools and memory, and record the
chosen bytes.

That conduct cannot guarantee any of the following:

- that the agent cannot read the rest of the repository, Git objects, sibling
  worktrees, home files, ancestor instructions or other job artifacts;
- that inherited environment, descriptors, hooks, plugins, role or skill
  routing and response schemas add no bytes;
- that tools, compiler errors, launcher diagnostics or network results reveal
  an unlisted source;
- that the provider adds no account memory, prior session state, retry text
  or compaction summary;
- that an agent cannot invoke another model path with available credentials;
- that the examined candidate remained immutable; or
- that the retained prompt proves no other exposure occurred.

The current repository launch grants broad read reach, and the current Codex
adapter reports that read roots are not enforced. Composition today can limit
the brief but cannot limit agent reach. Therefore no current Codex, Claude or
Devin launch is a proved independent examiner under this design. Any such
pass is advisory and cannot satisfy custody. The current interactive
coordinator is also outside the boundary and cannot compose its own already-
exposed session.

If the capsule or a qualifying provider profile cannot be supplied, the
recorded operating limitation is simple: machine independent examination is
unavailable. The system must use a separately situated human examiner or
leave the independence gate open. It must not relabel careful conduct as
enforcement.

## Acceptance conditions

The design is ready to build only when the following proofs have an owner and
a fast mechanical fixture:

- Absolute paths, Git internals, sibling checkouts, startup files, inherited
  descriptors, host sockets and unlisted network destinations are absent from
  the capsule, not merely unread in a sample.
- An attempted direct provider or native-subagent call has no credential or
  route, and an externally produced review is refused by custody and `land`.
- A mixed design artifact publishes only its normative range; its history and
  dispositions are absent from the examiner packet. An unstructured mixed
  artifact refuses.
- Role instructions, required skills, response contract, generated notices,
  working directory and every later transcript byte appear in the sealed
  record. Provider compaction and server tools are absent.
- An extra relevant document, optional evidence result or post-construction
  explanation fills no packet slot and is refused.
- Arbitrary tool stdout and stderr are traceable to a confined process. Raw
  host and provider diagnostics never enter model context or context
  attachments.
- The examiner cannot write the candidate, and the result is refused if its
  candidate digest differs at any binding point.
- Coordinator and custodian can read the proof projection but cannot open
  sealed packet or event bytes. An auditor can rebuild every delivery and
  obtain a binary proved/not-proven verdict.
- Every prompt caller and resident generation uses L13 in the replacement
  landing, and every delegated job is watched before its first model request.

No build starts from this design. Wido's separate instruction remains the
build boundary.

## Round 1 dispositions

| Finding | What changed |
| --- | --- |
| 1. Runtime boundary | Selected an ephemeral VM as the read boundary; canaries are tests only, and current broad-read runtimes do not qualify. |
| 2. L13 bypass | Confined credentials and model egress to L13 and made a valid L13 proof mandatory at custody and landing. |
| 3. Whole-artifact laundering | Split normative candidate ranges from design history; mixed unstructured artifacts refuse, with the semantic-prose limit stated. |
| 4. Tool and error output | Replaced tool-name classification with capsule provenance; raw control-plane diagnostics are withheld from context storage. |
| 5. Missing prompt sources | Added role, skill, schema, notice, working-directory and complete transcript bytes; disabled provider summaries and hidden retrieval for qualifying profiles. |
| 6. Unspecified projections | Replaced `P` cells with exact per-function field recipes, including required tests and reversal proof. |
| 7. Composition laundering | Split the one record into a narrow ordinary projection and an auditor-only sealed section. |
| 8. Necessity | Removed free-form grants; packets are derived from preconstruction condition citations and fixed role slots, with semantic necessity left honestly to authority. |
| 9. Prohibited actions | Added immutable examiner mounts, scratch-only writes, final digest checks and exact action limits for every function. |
| 10. Non-exposure proof | Bound the full enforcement tuple, provider input proof, every delivery and candidate identity into one record with a binary audit verdict. |
| 11. R-11 complexity | Reduced the design to one packet, one capsule and one delegation record; removed the general kind matrix and parallel grant, catalog, exposure and manifest authorities. |
