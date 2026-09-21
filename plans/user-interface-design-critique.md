# User Interface Design: critique, round 1

Verdict: **15 material findings** (4 HIGH, 9 MEDIUM, 2 LOW).

| | |
| --- | --- |
| Design reviewed | `plans/user-interface-design.md`, all 736 lines, sha256 `4c172f38adee2da9af83266fc226032163e86f8bb9fe2e6198769de0b6d7adea` |
| Source read at | `7ebb8e308f0210cb66632774f3564a53a0273de2`; every cited file is unmodified in the working tree |
| Written | 2026-09-20 22:50 CEST, by Fable 5.1 on m1e, which did not write the design |
| Evidence | Every `file:line` below was **read** in this session. Nothing was built, run or tested. Counterexamples are reasoned, not reproduced. |
| Round | 1 of at most 2, the design-critique cap. Failsafe round: 2. |

No review brief existed, so I set the threat model and scope myself. The designer should confirm or replace both before adjudicating; a finding outside the confirmed model closes as out of scope.

**Threat model.** One trusted human. Agents that are not hostile but are goal-seeking and error-prone: if a path to an outcome is open, some session will eventually take it. This is the model the shipped guards already assume when they make human acts "human-only by construction". Because the design puts authority-bearing acts behind HTTP, a web page open in the same browser is in scope. A remote network attacker is out of scope until the access model (line 718) is decided.

**Scope.** Would an implementer working from this page build something different or wrong? The confirmed navigation structure was attacked but not re-opened as a matter of taste. Paths are relative to `metasystem/` unless they start with `plans/`.

## Where the findings cluster

The interaction principle holds up (see [What held](#what-held)). The trouble is one level down. The design treats the Go application as a service that has principals, events and callable operations. The shipped application is a set of commands that read their caller from the operating-system process and write to a git record replicated between machines. Findings 01 to 04, 09 and 10 are that one gap seen from six sides. Patching each paragraph separately will leave seams; one new section, "The server as a new command edge", would answer them together.

## Material findings

### UID-R1-01 · HIGH · The owners read the caller from the OS process, so the adapters cannot be thin

**Where:** "Reuse the application owners", lines 193–207; first slice step 1, line 255; acceptance scenario, line 683.

The design says adapters "translate inputs and outputs", that each operation "retains its domain's ... authority checks", and that the first slice exposes a goal owner "preserving actor identity and existing authority".

**Evidence.** The actor and authority of a goal verb are assembled at the command edge from the calling process. `cmd/metasystem/goalsync_mutations.go:576-683` builds `goal.VerbRequest` from ancestry proofs over `os.Getppid()`, from `METASYSTEM_OWNER_LINEAGE` or the enrolled terminal's lineage, from `goal.ResolveMachine(root)`, and from a `lease.ClassifyResult`. The caller classes are MAIN, DELEGATE, SUPERVISION, ADAPTER-SUPERVISOR, STEWARD, HUMAN and UNTRUSTED (`internal/lease/classify.go:81-87`). `VerbRequest.Authority` is "the fresh in-process human proof" (`internal/goal/verbs.go:205-207`). Below that edge, `goal.Edit` and `goal.Publish` are ordinary callable functions (`verbs.go:2825`, `txn.go:549`). That half of the premise is true.

**Counterexample.** The server is one long-lived process. Every browser request and every agent tool call reaches the owner with the same parent pid, the same environment and the same lease classification. An implementer told to write a thin adapter has three ways out, and the design picks none: shell out to the CLI (forbidden at line 197), pass a human name without a proof (refused by the owner, rightly), or mint a new kind of actor and proof inside the server. The third is a change to the authority owner, not an adapter.

**Minimal correction.** Say that the server is a new command edge. Name what it mints for each principal: for the brain session a caller class and lineage (MAIN with an announced session and lease, or a new class), for the browser human a proof grade (finding 02). Say that endpoint, machine, operation id and clock come from the server's own clone (finding 03). Move "preserving actor identity and existing authority" out of slice step 1 and into a decision the slice depends on.

**Acceptance fixture.** A tool call and a form submission make the same edit. The two History lines carry two different actors, and neither is the server process. A request that carries a human name without the new proof is refused by the owner, not by the adapter.

**Artifact changed:** "Reuse the application owners"; first slice step 1; the authority and lease owners, named as changed.

### UID-R1-02 · HIGH · The browser human act has a requirement but no property, and the design rules out its own root of trust

**Where:** line 106; line 306; "Settings and setup", line 550; open decisions, lines 721 and 731; acceptance scenario, line 677.

**Evidence.** The shipped property fits in one sentence: a human-reserved command "descends from either its current interactive terminal or the enrolled terminal without crossing an agent process" (`internal/humanauthority/authority.go:1-3`). A second grade already covers a human act that does not come from a terminal: a channel reply from the authority's account, signed with a current and unused one-time code (`authority.go:41, 250-260`; `internal/channel/totp.go`; paper appendix, line 165). The design never mentions it. An approval binds a digest of what was approved (`ApprovalDigest`, `internal/goal/verbs.go:2980`). In a checkout declared the brain, a human's word is refused unless the caller classifies HUMAN: "the brain never carries a human's word ... not as --by, not as a relayed word, not as a channel reference" (`goalsync_mutations.go:685-699`).

**Problem.** The brain session runs as the same OS user, on the same host, as the server and the browser, with whatever shell and file tools its envelope grants. A login cookie tells the server that a browser is calling. It does not tell the server that no agent is. The design says the agent "cannot press its approval control" but gives no property a test could assert, and the process that would carry the human's approval is the same process that hosts the brain conversation, which is the relay the brain fence exists to prevent. Line 550 then excludes a terminal verb even for setup, so the browser's authenticator would have to vouch for its own enrolment.

**Minimal correction.**
1. State the property: nothing but the human's deliberate act can produce an accepted human act. That excludes the brain with its full envelope, any delegate, and a web page in the same browser.
2. Choose a mechanism class that meets it per act: a user-presence gesture on a platform authenticator, or the existing one-time code. Bind it to the digest of the exact subject, revision and budget on screen.
3. Record it as a new authority outcome beside the terminal, enrolled and channel grades. The history validation that enumerates outcomes changes with it (`internal/goal/file.go:862-866, 2098-2132`).
4. Allow one out-of-band root: enrolling or replacing the browser authenticator happens at the enrolled terminal, as `goal enroll-terminal` does today (`main.go:572`). This is also the recovery path when the authenticator is lost.
5. Say whether the server may run in a brain-declared checkout, and if so how the human path passes that fence.
6. Because the act is now reachable over HTTP: loopback bind until remote access is decided, Host and Origin checks, an anti-forgery token on every mutation.

**Acceptance fixture.** The brain, using every tool its envelope grants, calls the approval operation and the approval endpoint, and is refused both times. A cross-origin page posting to the endpoint is refused. An approval replayed against a goal whose digest has changed is refused. History shows the new outcome and the authenticator's identity.

**Artifact changed:** "Settings and setup"; a new human-act subsection; acceptance scenarios; the authority outcome set.

### UID-R1-03 · HIGH · The record is a replicated git transaction log, and the design never says which clone the server writes through

**Where:** line 108 ("the same application state update refreshes the board"); lines 232–236; "Drag and drop", line 418; line 157; open decision, line 722.

**Evidence.** Every goal verb is a transaction committed and pushed to a canonical remote under a journal (`internal/goal/txn.go:544-561`). Its outcomes are confirmed, confirmed-late, lost, abandoned, rejected and expired (`internal/goal/journal.go:40-45`); the compare-and-swap beneath can end landed, refused or unknown (`txn.go:328-335`). A pushed entry with an unknown outcome stops the clone: "this clone mutates nothing until it is classified" (`txn.go:553-557`). The actor's machine comes from the checkout (`goalsync_mutations.go:585`). Other machines' writes reach a clone only through `goal fetch`, which validates the canonical tip and moves the accepted ref (`main.go:588`).

**What the design leaves to a guess.**
1. *Which checkout the server owns.* A seat's working checkout carries staged work. The brain checkout must be quiescent and is fenced (finding 04). A dedicated clone is a third choice. The answer fixes the machine name recorded on every browser and brain act.
2. *"Unresolved" is not per card.* One unknown outcome blocks every later write from the server's clone, human and agent alike, until `goal recover` classifies it (`main.go:591`). The UI needs a workspace-wide state and a recovery action for it. This is one of the design's own no-terminal flows.
3. *A drag is a network push.* It can lose a race against three seats writing the same ref, and a reorder re-sequences peers across many files (`set-priority`, `main.go:581`). "Display a pending move until the server confirms" will last seconds. The design should say whether the server serialises its own writes, which the journal in effect requires, and what the board shows meanwhile.
4. *The board is fresh only for the server's own writes.* Claims, parks and landings from seats appear after a fetch. Someone must own the fetch loop and its cadence, and the board's observation time is the accepted ref's age. "Ordinary application machinery observes events and updates state" (line 157) has no shipped counterpart between machines.

**Minimal correction.** A short section, "The server and the record": the server's clone and machine identity; one write at a time through the journal; the six journal outcomes mapped onto the design's five outcome words (line 219); the blocked-clone state and its recovery action; reads from the accepted ref, with a server-owned fetch loop whose age is shown.

**Acceptance fixture.** A write is left at unknown outcome: every mutation control, in the UI and in the agent tools, reports the blocked clone and offers recovery, and nothing retries blind. A seat on another machine claims a goal: the board shows it after the next fetch with its observation time, with no browser write involved.

**Artifact changed:** a new section; the outcome table at lines 215–222; drag-and-drop confirmation; acceptance scenarios.

### UID-R1-04 · HIGH · Fleet controls for another machine have no transport, and the brain checkout refuses the rest

**Where:** "Fleet", lines 524–526; operations map, lines 564, 568 and 569.

**Evidence.** `up`, `stop`, `status`, `arm` and `health` are process verbs on the local checkout (`cmd/metasystem/main.go:744-760`). In a brain-declared checkout, dispatch, follow-up, cancel, close, reap, land and claim are refused, each with the instruction that the owning node runs the verb from its own checkout (`internal/brain/brain.go:429-452`). The paper's fleet shares two media only, the git-synced record and the channel, and its nodes "proceed on rules and records, not on instruction" (appendix, line 137).

**Problem.** The design flags remote *observation* as unverified (line 526) and is silent on remote *control*. No shipped operation lets a browser attached to one server stop an engine, cancel a delegate or release a claim on another machine. An implementer would guess between a server on every machine with the browser talking to all of them, remote shell, or control requests written to the shared record and honoured by the node's own steward. Only the last fits the paper, and it makes every remote control asynchronous with an unknown delay. That changes the Fleet interaction (requested, pending, honoured or expired) and the pattern in "Consistent action behavior".

**Minimal correction.** Decide the transport. If it is the record: name the request record, who honours it on the node, when it expires, and what the UI shows before the node answers. List the Fleet controls that are local-only in the first version. Say how dispatch and cancel from the UI relate to the brain fence when the server shares the brain's checkout.

**Acceptance fixture.** A stop requested for a seat on an unreachable machine shows as requested and not honoured, with its age, and never as stopped. The same control on the local machine completes through the local verb.

**Artifact changed:** "Fleet"; operations map rows at lines 564, 568, 569; "Consistent action behavior".

### UID-R1-05 · MEDIUM · Four owner capabilities are presumed, and none is listed as backend work

**Where:** line 590 ("The application supplies current allowed actions and refusal reasons from the owning domain"); line 594; result table, lines 215–222; impact previews, lines 308, 441–447 and 482.

**Evidence.**
- *Preview.* No dry-run exists in the goal owner or its command files: a search for `dry-run|dryrun|DryRun` over non-test `internal/goal/*.go` and `cmd/metasystem/goal*.go` returned nothing. A verb's effects are known only by running its mutation inside `Publish`.
- *Allowed actions.* There is no query. A refusal is discovered by attempting the verb.
- *Refusals.* A registry of coded refusals exists (`internal/refusal`: code, owner, site, shape), but the goal edit refusals are uncoded prose (`internal/goal/verbs.go:2880, 2911, 2925, 2928`), and several give a CLI command as the recovery ("Wido runs, from an agent-free terminal: metasystem goal ...", `goalsync_mutations.go:691, 698`).
- *Changed references.* `PublishResult` carries outcome, tip, commit, detail and a risk flag (`txn.go:536-542`), and no affected objects.

**Problem.** The design names slice-plan ownership and browser authority as backend requirements, but not these four, and every form depends on them. Without an owner-level preview the adapter must work out for itself which approvals go stale, which claims park and which dependents are redirected. That is the reimplementation of transition policy that lines 195 and 590 forbid.

**Minimal correction.** List as backend requirements: (a) preview, which runs the verb's mutation against the current tip and returns the change set without publishing; (b) allowed actions with reasons, for one subject and one caller; (c) coded refusals carrying subject, unmet condition and recovery operation as data, for the goal owner first; (d) changed references derived inside the owner from the transaction's change set.

**Acceptance fixture.** A preview of `unapprove` on a claimed goal names the claim that will park. When the tip has not moved, the published result equals the preview; when it has, the form shows the difference before anything is published.

**Artifact changed:** "Consistent action behavior"; the result table; "Existing information homes and integration gaps".

### UID-R1-06 · MEDIUM · The agent's direct path skips the consequence step, and the headline example is refused by the owner

**Where:** line 104; the flow at lines 116–122; the example at line 114; line 588; line 453.

**Evidence.** An intent edit on an approved goal is refused: "unapprove the goal, edit it, then approve the new intent" (`internal/goal/verbs.go:2879-2880`). Editing another holder's claimed goal, or a parked goal, is a human act (`verbs.go:2924-2928`). A risk edit on an approved goal can raise the derived tier and rewrite the approval record (`verbs.go:2890-2980`; `PublishResult.RiskRaised`). `unapprove` parks any standing claim (`main.go:561`). `detach` releases a riding claim (`main.go:582`). The paper says an edit invalidates the approval (appendix, line 193); the code refuses instead, and the design follows the paper's wording at line 453.

**Problem.** The human path is select, state, inspect the consequence, perform, show (line 588). The agent path is attach, call, return, refresh, card. It has no inspection step, its input is natural language read by a model, and line 104 lets it "directly initiate an operation". The capability inventory records read, propose and perform per action, but the design gives no rule for who gets "perform". And "Change this goal's wording" fails on any goal that matters: approved, claimed by a seat, or parked. The real flow is unapprove, edit, approve: three transactions, two of them human-only, the first of which stops a seat's work.

**Minimal correction.** Give the rule. One candidate: the brain performs directly only operations that touch no approval, claim, budget, priority or other goal, and that the same actor can reverse; everything else is proposal-first, using the preview from finding 05. Add "revise an approved goal" as a guided compound flow with its interrupted states. Add the refused edit to the checks in slice step 4.

**Acceptance fixture.** Asked to reword an approved goal, the brain returns a proposal that names the unapprove and re-approve it entails and performs nothing. Asked to reword a queued goal, it performs the edit.

**Artifact changed:** "Working with the underlying objects"; "Show action results in the UI"; first slice step 4; the capability inventory's columns.

### UID-R1-07 · MEDIUM · The goal editor edits fields the goal record does not have

**Where:** line 384; Definition tab, line 427; line 369; evidence chain, line 494; dispatch binding, line 465.

**Evidence.** The editable fields are Intent, Tier, Risk, NextStep, Blocked and Labels, with Why and Evidence as the edit's reasons (`EditFields`, `internal/goal/verbs.go:2812-2822`). Priority, arc, pin and budget have their own verbs. There is no field for constraints, success conditions, or a design reference with its revision. The file grammar is parsed strictly and sealed by an integrity line (`internal/goal/file.go:3-8`), replicated to every seat, and guarded by an engine floor (`main.go:545`).

**Problem.** The design flags this limit for split members only (line 451). For the goal itself an implementer must choose between extending the grammar across the fleet, conventions inside the Intent prose, or a side index, which line 626 forbids as a source of truth. The link from a goal to a design revision carries the impact view, the evidence chain and the dispatch binding.

**Minimal correction.** Decide which definition fields become record fields (at least design references with a revision) and which stay prose inside Intent and are displayed as prose. Name the grammar extension and its migration as backend work.

**Acceptance fixture.** A design revision changes, and the goals referring to it are found from record fields alone, without parsing prose and without the index.

**Artifact changed:** "Backlog: definitions and planning"; "Existing information homes and integration gaps".

### UID-R1-08 · MEDIUM · File revision is the wrong binding token for goals in flight

**Where:** lines 90–92, 249, 308 and 592; acceptance scenario, line 689.

**Evidence.** `Revision` is "1 at creation, +1 per verb write", by any actor (`internal/goal/file.go:38`, and lines 5–6), so seat writes such as next step, claim, restamp and park all bump it. No verb accepts an expected revision apart from `extend-budget --revision` (`goalsync_mutations.go:2537`). Approval already binds a digest of the approved fields rather than the revision (`verbs.go:2980`).

**Problem.** A proposal, a pending decision or an open editor bound to file revision 12 goes stale when a seat rewrites its next step. On exactly the goals being worked, "exposes the changed basis and requires the decision to address the current version" fires for changes that have no bearing on the decision.

**Minimal correction.** Bind a review to a digest of the fields reviewed, following `ApprovalDigest`, and keep the file revision for display and history. Put the expected-basis check inside the transaction's mutation, where it is free of races, and list it as an owner change.

**Acceptance fixture.** A seat updates the next step while the human considers a reworded intent, and the decision submits cleanly. Another actor changes the intent, and the decision is refused with the difference shown.

**Artifact changed:** "Explicit shared context"; "From conversation to authoritative records"; the acceptance scenario at line 689.

### UID-R1-09 · MEDIUM · "Through ACP" hides two components that have to be built, and the brain's envelope is left open

**Where:** lines 9, 185 and 189; component table, lines 163–169; open decisions, lines 714–715.

**Evidence.** The pinned protocol schema requires an `mcpServers` list on `session/new` (`internal/acp/schema/acp-v1-schema.json:4660-4676`); that list is how a client hands an agent extra tools. The shipped driver sends it empty on both new and load (`internal/acp/turn.go:211, 229`). The driver runs one prompt attempt per invocation over prepared pipes and answers permission requests by policy (`cmd/metasystem/acp_verbs.go:17-23`; `internal/acp/decide.go:179-195`). The envelope has five fields: read roots, write roots, network, approvals and tools (`acp_verbs.go:25-31`). Goal records are plain files under `plans/goals/`; bytes the verbs did not write are found afterwards by the integrity line and handled by `goal reconcile` (`main.go:586`).

**Problem.** What the design calls "the agent connected through ACP" is two new components: an interactive driver (many turns, streaming to a browser, permission prompts answered by a person, cancel) and an MCP server inside the Go application that exposes the domain tools under the brain's credential. Neither exists, and the wording suggests reuse. Line 185 says a checked API is not enough if the agent can write around it, and then leaves the envelope undecided. With a write root over the checkout the brain can edit goal files directly, and detection arrives later as foreign bytes.

**Minimal correction.** Name both components in the component table. Fix the brain's envelope in the design: no write root over the record tree or the server's state, no shell or a sandboxed one, no network path to the human endpoints, and tools limited to the application's MCP server plus reads. State which of these each candidate provider can enforce. That, and not transcript format, decides whether providers are interchangeable.

**Acceptance fixture.** A brain session asked to "just edit the file" holds no tool that can, and the attempt is recorded as refused.

**Artifact changed:** "Component responsibilities"; "Decisions still needed", lines 714–715.

### UID-R1-10 · MEDIUM · Two things are now called the brain

**Where:** "The brain", lines 53–74; operations map, line 567.

**Evidence.** The shipped brain is a checkout-local designation made by a human-only verb on a quiescent checkout, with boot context, fences, a status line and a role packet (`cmd/metasystem/main.go:54-62`; `internal/brain/brain.go:1-3, 31`). Today a main session in a terminal occupies it.

**Problem.** The design's brain is a session owned by the server. It does not say whether starting one requires `brain declare`, whether it boots from `brain boot`, whether the terminal brain seat carries on beside it, or which of the two has its hand on the queue. Two occupants would both draft and order the backlog under a role whose point is that there is one.

**Minimal correction.** State the relation. One candidate: the server's brain session is the occupant of the declared brain seat; the declaration is a precondition for a sitting with write tools; the boot context feeds session start; a terminal main in that checkout and a UI session exclude each other.

**Acceptance fixture.** A sitting opened in an undeclared checkout gets a read-only brain and says why.

**Artifact changed:** "The brain"; operations map, line 567.

### UID-R1-11 · MEDIUM · Where sittings are stored decides whether examiners stay independent

**Where:** lines 84, 269–277 and 624; open decision, line 720; acceptance scenario, line 691.

**Problem.** Two stores are defensible and they behave oppositely. The shared git record gives continuity across machines, and any node's agent can read it. A server-local store is out of delegates' reach and ties the brain to one machine. The design asks for both continuity across providers and sessions, and examiners who never see the exploratory reasoning. In the shared record a transcript sits inside every delegate's read roots unless every envelope excludes it, so "did not receive" would rest on the examiner not looking.

**Minimal correction.** Split them. Working material (facts, proposals, decisions, open questions) goes to the shared record under the existing categories at line 624. The raw transcript stays in a server-local store outside every delegate read root. Continuity across machines then rests on the working material alone, which the design already says is what carries the meaning (line 57). Reword line 691 from "did not receive" to "could not read".

**Acceptance fixture.** The examiner's envelope for a design born in a sitting lists no root that contains the transcript. A fresh brain on another machine resumes from the working material only.

**Artifact changed:** "Conversation and session lifecycle"; line 624; line 691.

### UID-R1-12 · MEDIUM · The reveal policy needs a signal that nobody produces

**Where:** line 128; lines 240–245; acceptance scenario, line 679.

**Problem.** The frontend must tell apart three kinds of tool result: the requested change or proposal (reveal it), a supporting read (do not), and an explicit inspection request served by a read (reveal it). Mutations and reads differ by operation kind. The last two are both reads, and the only difference is the human's intent, which lives in the message and in the model's reading of it. The design forbids inferring from prose (line 124) and forbids letting the model choose a page (line 226).

**Minimal correction.** Pick one. Either accept a single narrow signal, such as a `present` tool that takes result references and that the brain calls when the human asked to see something, with the frontend still deciding under rules 3 and 4 whether to honour it; then say plainly that this is a small agent-to-UI signal, instead of claiming there is none (lines 39 and 189). Or decide that reads never reveal and only leave cards.

**Acceptance fixture.** The brain makes five supporting reads and one `present` call, and the work area changes once.

**Artifact changed:** "Show action results in the UI"; "Render results with the existing interface"; the claims at lines 39 and 189.

### UID-R1-13 · MEDIUM · The first slice runs through the undecided structures, and the first version contains a programme of unbuilt backends

**Where:** Status, line 3; "Build one complete interaction first", lines 251–261; "Delivery scope", lines 649–666.

**Problem.** The Status line calls what remains "supporting implementation decisions". Findings 01, 02, 03 and 09 show that at least four of them are structural: decided one way they invalidate confirmed text. The first slice, an agent edit and a manual edit through one provider, needs 01, 03 and 09 settled before its step 1. The "first useful version" then depends on a slice-plan owner (line 467), a durable seat inventory with remote aggregation (line 526), evidence retention and retrieval (line 500), a sitting store (line 624), browser authority (line 550), and the four capabilities of finding 05. That is a programme, and a critique loop capped at two rounds cannot close on it. Meanwhile line 664 asks for the navigation to be validated before many forms are built, which a read-only projection of today's records would do at no authority risk.

**Minimal correction.** Split the page into a core that can close and satellites routed from these findings.
- *Core:* the server on its own clone; read-only Backlog, goal detail, local Fleet and Decisions views over today's owners; the context and result-reference model.
- *Satellites, in dependency order:* the server command edge and its principals (01, 03); the human act (02); the brain session, its tools and envelope (09, 10, 06, 12); owner capabilities (05, 07, 08); the planning editors; fleet-wide observation and control (04); sittings and evidence retention (11).

Keeping the agent edit first in order to meet the hardest risk early is a fair refutation of the ordering. In that case 01, 03 and 09 move from "Decisions still needed" into the design before the slice starts.

**Artifact changed:** Status line; "Build one complete interaction first"; "Delivery scope"; "Decisions still needed".

### UID-R1-14 · LOW · The brain is the only agent in the system without a budget

**Where:** line 80 (the dock is open by default); lines 263–279; "Settings and setup", line 546.

**Problem.** Every other agent runs inside a box of time, attempts and spend. A permanent conversation draws on the same provider limits the fleet builds with. The design rightly keeps navigation from invoking the model (line 98), but it specifies no meter, no limit, and no behaviour at exhaustion.

**Minimal correction.** Show the sitting's spend and context use in the dock header, from the existing metrics and context reports (line 574). Put a per-sitting limit under execution defaults. At the limit, or when the provider is exhausted, hand the sitting to a fresh session or another provider from its working records, which the continuity section already makes possible.

**Acceptance fixture.** A sitting that reaches its limit continues in a fresh session with the same working material, and the dock shows the handover.

**Artifact changed:** "A permanent place beside the work"; "Conversation and session lifecycle"; Settings.

### UID-R1-15 · LOW · The tool surface's shape is unstated, and complete coverage makes it matter

**Where:** lines 102, 126, 259 and 582; acceptance scenario, line 686.

**Problem.** One tool per operation, across roughly twenty operation families, puts well over a hundred tool definitions into every brain turn. A handful of generic tools (get by reference, query by kind and filter, preview, propose, perform by operation name), with the capability inventory served as data, keeps the context bounded, which the design asks for elsewhere (line 98). The two choices produce different MCP servers and different tests. Line 126 rules out a tool per visual component and says nothing about a tool per domain operation.

**Minimal correction.** Pick one and say so.

**Artifact changed:** "Working with the underlying objects"; the coverage inventory's columns, line 582.

## Nonmaterial notes

Recorded, not to be actioned, and never blocking.

- **Command map accuracy.** Every goal verb the map names is routed (`cmd/metasystem/main.go:537-592`). It omits routed verbs a person uses: `done`, `promote`, `claim`, `land-ready`, `prune`, `engine-floor`, `trunk-red`, `classify-sweep`, `tier-probe`, `enroll-terminal`, `restamp`, `carrying`, `carried` and `branch`. The coverage inventory at line 582 would catch them. `set-budget` is "kept for one release" (`main.go:562`); `goal budget` is the current form (`main.go:558`).
- **Information homes.** All seventeen paths in the table at lines 602–614 exist at the commit read. The host checkout has no root `covenant.json`, which the "where present" wording allows.
- **Delegations.** A power of attorney covers tiers 1 and 2, approve and set-budget only, and expires within seven days (`main.go:564`). The Delegations view should show these as owner facts, not as form choices.
- **Session resume.** `session/load` replays history as live-looking updates (`internal/acp/assemble.go:9`). Result cards come from server-held operation results, so they are safe; streamed conversation text on resume needs the same care.
- **Overview.** "Changes since the last visit" needs a per-human visit marker. Server-local state is enough, and it is not a record.
- **Restatement.** The shared-operation flow is told four times (lines 110–130, 191–261, the component section, and the acceptance scenarios). An implementer reads all four the same way today. Amendments from this round will land in one copy and drift from the others, so fold them into one telling while amending.
- **Lane sourcing, seat inventory, evidence joins.** The design lists these as open (lines 724–728) and claims nothing about them, so they are not findings.

## What held

These were attacked and survived. They need not be re-opened in round 2.

- Two callers of the same operations, with result references and frontend-owned routing, and no DOM automation or agent-written UI.
- Context captured with the message at submission, so later navigation cannot retarget an operation being prepared.
- Persisting the operation outcome instead of a queue of navigation commands; a display failure never repeats a mutation. The journal's operation-id replay (`txn.go:558-561`) supports this directly.
- Status from authoritative records, with the flight recorder as a diagnostic witness only.
- Evidence gaps named explicitly, and an empty view never read as proof that nothing happened.
- A drag requests an operation and cannot manufacture a claim, a review or a completion.
- The brain does not dispatch. This matches the shipped fence (`internal/brain/brain.go:438-439`) and the paper (appendix, line 137).

## Review limits

- The seat registry, census, lease and supervision owners were not read. Whether a durable fleet-wide seat inventory already exists is unchecked.
- The phase owners behind the Review and Verification lane were not read.
- Mission, run, launch and landing owners were sampled only through their verb summaries in `main.go`. Finding 01 is proven for the goal owner and assumed to hold for the others.
- Where the one-time-code secret is kept, and whether a same-user process can read it, was not checked. It bears on whether finding 02 can reuse that mechanism as it stands.
- The claim that `mcpServers` is the protocol's only client-to-agent tool channel rests on the pinned schema's `session/new` parameters and on my knowledge of the protocol. The schema was not read end to end.
- No frontend technology, accessibility or visual design was reviewed. The design specifies none.
