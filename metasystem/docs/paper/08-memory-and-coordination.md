# 8. Memory and Coordination

**Work that lives only inside a worker loses its memory the moment the worker stops.**

Halfway through preparing existing sessions for the new expiry rule, a builder stops. No colleague remembers the last command, because there is no colleague watching over its shoulder. The next builder must determine whether any session was changed, which candidate was in use, which decisions already bind the work and whether continuing would repeat an unsafe action. If those facts live only in the first builder's private context, the work has not just paused. It has lost its memory.

The system keeps memory outside its workers, because any worker can disappear. Intent, decisions, claims, actions, results and current state become durable records as the work proceeds. That memory also coordinates workers that do not share a conversation or a working day. It can show where work is, who may act next and what evidence an action must preserve. It cannot decide which human value should prevail.

## One authoritative record, with access limited by role

Consider the first session-expiry candidate. One account says it is ready. Another says a delayed background response revives an expired session. A third says the candidate was repaired, but the evidence belongs to the earlier version. If all three accounts can determine the state of the work, "ready" has no stable meaning. Release can follow whichever story an actor happens to read.

The remedy is one authoritative record: a structured, history-preserving source that alone determines the current state. For the case established in Chapter 1, it binds every state transition to the actor, authority, exact candidate and evidence that support it. Earlier states remain visible to authorized readers rather than being rewritten to make the final path look clean. A private note can help a worker think, but it cannot make a candidate accepted or an enforced rule passed until the relevant fact enters the authoritative record.

One source does not mean one view for everyone. The record must be complete enough to recover and audit the work, while each role sees only what its task requires. Chapter 9 applies the same limit to both information and action: permission to examine a finished change does not imply permission to read every thought that produced it. This protects sensitive information and reduces the chance that irrelevant material steers the task.

Access limits have a hard boundary. They can prevent a mind from reading something next; they cannot remove what that mind has already learned. A person who built the session change does not become independent by receiving a new label and losing access to the earlier notes. A machine worker that has already absorbed the builder's sequence of ideas and failed attempts cannot supply a fresh view even if later permissions hide that sequence. Role-scoped access provides least authority. Only a fresh mind provides a fresh perspective.

When Chapter 6 requires independent examination free from the builder's path, the independent examiner must be a distinct person or a newly started machine worker that has never received that path. Machine workers make this separation inexpensive because a new mind can begin with selected context. Human exposure is largely permanent.

## What each role may read

A builder begins with the authorized session outcome, its constraints, the human ruling on passive reading, refresh and uploads, and the parts of the application needed to make the change. It can read earlier rulings that bear on the task and the evidence needed to check its own work. Its sequence of attempts, discoveries and unresolved questions is recorded so that another builder can recover if it stops. That path belongs to construction and later audit; it is not automatically part of examination.

An independent examiner receives the authorized outcome and the conditions the candidate must meet. It sees the relevant application behavior, the finished candidate and the results that the builder offers in support. It does not see the builder's private sequence of ideas, the arguments discarded along the way or the explanations used to defend the chosen design. Those omissions are intentional. The independent examiner needs enough context to find faults, but not a guided tour through the assumptions it is meant to challenge.

The custodian sees a narrower conclusion. It receives the exact finished candidate, the required examination and test results, the human rulings and the state history needed to accept or reverse that candidate. It does not need the freedom to explore possible designs. It needs to determine whether the chain is complete and whether the proposed acceptance action is authorized.

An auditor has a different purpose. When an incident or challenge requires reconstruction, the auditor may read the full retained history: the builder's path, the independent examiner's findings, decisions about roles and access, state changes, evidence and acceptance. That breadth does not grant power to alter the history or accept the work. Read access answers what happened. Authority answers what may happen next. The two remain distinct.

All of these views draw on one preserved history; none of them is a separate record. Information hidden from an independent examiner is not destroyed; it remains available to a replacement builder and to a later auditor. Recovery, independence and audit can all coexist without pretending that every actor must know everything.

## Handoff by record

Suppose the builder stops after it has identified the sessions that need a new expiry time but before it has changed them. The replacement must not infer progress from a half-written explanation. It needs the authorized intent, the last completed state, the exact actions already taken, the evidence those actions produced, the open question and the next action it is permitted to perform. If the record says only "migration in progress," safe continuation is impossible.

This gives handoff a concrete test. Stop any builder, independent examiner, custodian or auditor at any moment and ask a replacement in the same role to continue from the records that role may read. The replacement should be able to distinguish completed work from proposed work, identify unresolved questions and either continue or reverse without private instruction from its predecessor. A handoff succeeds when the record carries the work; two actors overlapping long enough to exchange a convincing story is not a handoff.

The test also exposes what must be recorded before an action completes. A builder records which sessions a change will affect before it applies the change. An independent examiner records the candidate it challenged before it reports a finding. A custodian records the evidence set before it accepts the candidate. These records are part of the action's safety rather than paperwork assembled after the fact.

## Coordination is not agreement

Two builders propose different ways to protect an upload while ending the ordinary session. The record can ensure that they do not overwrite each other's candidates. It can show which proposal arrived first, preserve both and require an authorized choice before either becomes current. None of those operations decides whether uninterrupted work is worth a wider security exception.

Coordination orders actions and preserves their relationships. It can require that one state change finish before another begins, prevent two actors from claiming the same authority and show which proposal replaced which. It cannot resolve a conflict of values or decide whose claim is stronger. When security, usability, legal obligation or accountability conflict, a responsible authority must weigh the choice and bind the decision. Chapter 13 develops that governance. The record preserves the ruling and its reasons; it does not manufacture agreement.

## Two audiences, one source

At 4:30, machinery needs to know the exact accepted candidate and whether every required check passed. The responsible authority needs a readable report of the ruling, refusal, accepted result and production bounds. Presenting the responsible authority with raw state changes would hide the decision in detail. Presenting machinery with a polished summary would hide the exact state in prose.

The system produces two views from one source. Structured fields allow machinery to determine state and enforce permissions. A readable report lets a person understand the outcome, exceptions, failures and consequences. Every material sentence in that report remains linked to the record from which it was drawn. The human view may compress; it may not create a second truth.

## Decisions, precedents and institutional memory

Months later, another change must decide whether a long-running export may continue after a session ends. The earlier upload ruling is relevant: a user-started operation received a separate, limited continuation without reviving the session. Retrieving that ruling can expose a useful distinction and prevent the new decision from beginning in ignorance.

The stored ruling is a precedent: it informs the later decision without binding it. The new operation may expose different data, last much longer or have a separate responsible authority. Only an enforced rule with the proper authority can make the earlier result compulsory. Institutional memory supports consistency and makes a change of mind visible, but a lookup is not judgment. The legal metaphor ends at that boundary.
