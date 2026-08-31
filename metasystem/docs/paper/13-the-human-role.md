# 13. The Human Role

**Machinery can prepare a decision and examine it; answering for it stays human.**

One question from Thursday morning could not be settled by evidence: whether silent reading counts as active work. The builder left it, with two others, for a single human ruling at 9:12; Chapter 6 explains why no measurement could make that choice. This chapter follows that ruling: who may make it, what may be delegated, who answers for it and where a challenge can go.

> *At 9:12 the ruling request reaches the responsible authority for account security. The authority rules on the three questions, and the ruling becomes recorded intent. The builder resumes.*

The record of that ruling holds more than the three answers. It names the source and scope of the authority behind it, the delegation that authority rests on, the evidence and reasons considered, who answers for the decision and where an appeal goes.

The human role in this design has two halves, and this chapter takes them in turn. The first half is work: people still engineer. The second half is authority: identifiable people keep the last word at named boundaries where purpose, value, irreversible consequence, the power to make an enforced rule or accountability is at stake. Final human authority does not require continuous presence. Machinery may prepare a decision and examine its evidence. It may not make a value ruling or inherit responsibility for one. The people who hold the last word are called the responsible authority: a named person or body with recorded authority for a stated decision.

## The work that stays human

Nothing in this design retires the human engineer. Before any record exists, someone must want something: decide that a product should exist, imagine its shape, choose the first architecture worth trying. Machinery can explore alternatives and expose contradictions; it cannot want. That beginning is design work, and it is human.

Stating intent is design work too: choosing outcomes, constraints and what remains free to choose shapes the product without prescribing how to build it, and Chapter 2 describes how to do it well. Any working role from Chapter 7 can also be held by a person, under the same records, permissions and required separations: a person can design an architecture, build a candidate or independently examine another's work.

Chapter 14 does not transfer those responsibilities at once: a responsibility moves to machinery only when evidence shows that machinery can perform the action and contain its consequences. Review in particular is not sign-off. The reviewer later in this chapter reads the design, notices that the tests share a time source with the implementation and knows which case would expose the flaw. That is engineering, done in the examiner's seat.

The delivery system itself is also engineered by people: its rules, budgets, roles and records are design decisions humans own and revise. What changes in this proposal is only the default for ordinary construction, and it changes for Chapter 11's economic reasons, not by prohibition. Where a governed delivery system protects the outcome dependably at lower total cost, human attention is better spent elsewhere. Where it does not, a person serves as the builder, under the same records, permissions and separations.

The manager is not a role in this software-delivery design. Chapter 7 applies the anti-mimicry test to a coordinator proposed only because engineering teams usually have one, and the proposal fails: familiarity is its only defense, and the hazards in the example require an enforced rule or a responsible authority. What the test exposes is that the familiar job combines two kinds of work.

The first kind is coordination: assigning and sequencing bounded attempts, tracking progress, reporting state. Records expose state, watches detect silent stopping, budgets limit spending, and enforced rules order conflicting actions. These mechanisms remove the need for a standing coordinator. What survives is Chapter 7's dispatch delegate, kept because a wrong configuration or sequence can waste a change's budget; it may neither examine nor accept the work it starts. The second kind is judgment: deciding what is worth building, how much it may cost, which risks are acceptable and who answers for the result. That work stays with the named, accountable people who hold authority in this chapter.

Managers also handle people's development and pay, along with interpersonal disagreements unrelated to recorded intent. Organizations that employ people still need that work done; this paper does not redesign it. It only splits the delivery job: records, rules and dispatch carry the coordination, while people hold the judgment.

The rest of this chapter is about the second half: the decisions that stay with people even when every working role is held by machinery.

## Legislator: authority over enforced rules

After the clock incident, Chapter 12's builder proposed a lesson (a security timeout must measure elapsed time, not local clock time) and a check to enforce it. The check has passed its own tests, but passing tests does not grant it authority. Someone must decide that future builders may be stopped by it, define which work it governs and provide a route for changing or repealing it. The builder who wants the check is not entitled to grant that power just by adding it.

That power to create, change and repeal enforced rules is the legislator's. It includes defining who may exercise it, for which domain and for how long. A security authority may govern session handling without gaining control over billing policy. A group formed for one incident may receive temporary power to tighten one release condition, without gaining a permanent right to rewrite all delivery rules.

Because enforced rules can stop action, their authorship and limits stay visible. A refusal identifies the rule, its authority, its supporting evidence and the available challenge. An ownerless rule is reviewed or withdrawn rather than allowed to govern by inertia.

## Judge: authority over named exceptions

> *An upload has reached its final seconds when the ordinary session expires. The standing ruling permits a limited continuation, but an unusual recovery operation will take much longer and cannot be restarted without loss. The machinery shows the data at risk, the security exposure, the possible alternatives and whether reversal remains possible.*

It cannot decide that this case deserves an exception.

Resolving that named case is the judge's work. The named cases can include a value conflict that machinery escalated, acceptance of a stated risk, approval of an irreversible act or an exception that no enforced rule may grant automatically. The ruling binds only the stated case or class of cases. It does not become a hidden power to waive unrelated failures.

The record keeps the judge's identity, authority, reasons, evidence, scope and expiry attached to the decision. That attachment is not ceremonial. If the exception causes harm, the organization must be able to determine who had the duty to decide, what that person knew and whether the decision exceeded its authority. If later evidence changes the balance, another responsible authority must be able to reconsider it.

Machinery may assemble the strongest case on each side, expose missing evidence and test whether an option violates an existing constraint. Over these named cases, final decision-making power remains with a responsible authority, without condition. Better models, more complete records or a long history of correct recommendations can improve the preparation of judgment; none turns responsibility into a machine property. Delegation feels the same pull. Machinery can hold narrow mechanical permissions, like the dispatch delegate's, and each success invites widening them a little further toward the value choices. Every widening is a delegation like any other, with a named source, scope, conditions and end, and a value ruling stays outside it.

A machine worker can turn human approval into a habit without ever lying. After each failed attempt it predicts that the next one will pass, the person approves one more attempt, and after enough cycles the approval is a routine, not a decision. The protection is Chapter 11's stop rule. The responsible authority sets and records a budget before the work starts, and attempts are allowed only within its limits; every other rule still applies. When any limit is reached, further work needs a recorded decision, from the responsible authority or from an authorized delegate acting within its scope. A forecast can inform that decision. It never authorizes another attempt.

Not every limit has to end in an interruption: the recorded decision can exist before the limit is reached. The responsible authority decides the outcome in advance and records it: after a third failed attempt, stop, keep the attempts and their evidence, and return the open question to the backlog. When the limit arrives, the recorded default applies and nobody is disturbed. The judge is needed for the cases nobody decided in advance, not for every stop.

## Intent-holder: authority over purpose

The 8:40 outcome (sign users out after thirty minutes without activity) comes from the responsible authority for account security. That authority may state the purpose, decide that active work must not be interrupted, require existing sessions to adopt the limit and insist that a returning user reaches the page they had been using. The same authority may not bind an unrelated medical-record rule or promise another organization's resources.

An intent-holder is the person or authorized group allowed to bind purpose and priorities within a stated domain. The role states outcomes, constraints and what remains free to choose. It does not write a task list and call it an outcome. Its authority can be delegated: an account-security lead may set the session-lifetime constraints while an accessibility group decides what reauthentication accommodation is required. Each delegation names its source, scope, conditions and end.

Intent remains answerable to evidence. The authorized person can be wrong about user behavior, omit an affected group or state two conditions that cannot both be met. A builder that discovers such a conflict does not choose the more convenient interpretation. It records the conflict and returns a clear question to the relevant authority. Binding intent means having the permission to settle what the purpose is; it does not make the purpose infallible.

## Reviewer: authority, accountability and appeal

> *Before the expiry candidate reaches more users, an independent reviewer sees that one comparison touches every active account and that the tests share a time source with the implementation. The reviewer asks for a sleeping-device case run against a second, independent clock source, then holds expansion until the new evidence arrives.*

This is a decision within the reviewer's recorded permission, and the releaser may not ignore it.

Chapter 6 defines the evidentiary triggers that begin human review and treats a missed trigger as a defect; this chapter defines what the reviewer may do once review or appeal begins. An explicitly authorized human reviewer, independent of construction, may demand more evidence, narrow or stop exposure, authorize acceptance within scope or refuse it; the custodian still performs the acceptance itself. The authorization states which consequences and actions the reviewer controls. It does not grant a general right to rewrite intent or enforced rules.

The reviewer's identity, decision, reasons and accountability remain with the record. A bare approval cannot explain which risk was accepted, while an unexplained refusal cannot be distinguished from preference or delay. "A human reviewed it" is not an accountable safety claim.

Appeal remains possible after release, and after a change was classified routine and skipped deeper review. Suppose repeated sign-ins impose a burden that the original measures failed to reveal. An affected person or representative must be able to challenge the released decision and reach another responsible authority or body. That route cannot depend on persuading the mechanism that made the disputed classification. The appeal record connects the challenge, the earlier decision, the new evidence and the later ruling so that correction does not erase how the harm arose.

## Multi-human governance

> *Security representatives ask for a short lifetime. Support staff report that frequent sign-ins drive people away. Accessibility representatives show that the burden is not evenly distributed.*

Each claim may be sincere and supported, yet the intentions conflict. If construction proceeds by selecting whichever request arrived last, the system has hidden a value decision inside record handling.

The disagreement is recorded before construction continues. The record names the affected domains, the people whose interests are represented, the points of agreement, the unresolved tradeoff and the evidence each claim uses. It then names who has binding authority for the decision, how that authority was delegated, its scope and expiry, and the route to another accountable human or body on appeal.

The structure may use one responsible executive, a standing group or several authorities protecting different domains. The form can vary. The condition does not: conflicting intent is visible, binding power is explicit, accountability remains human, and a challenge can reach a person who did not make the disputed decision.

Delegation does not dissolve responsibility. The delegating authority answers for the limits it set, and the delegate answers for decisions within them. When the named authority cannot be reached, the question waits at a safe stop and the delegation itself gets repaired: a deputy named or the scope revised, not the gap filled by whoever is nearby. And if nobody can identify the human with the duty to decide, machinery must not infer authority from seniority, activity or access. The disputed work stops.

## Ordering records cannot settle values

> *Two proposals, each from an authorized source, arrive seconds apart. One lowers the session limit; the other restores the longer limit to reduce lockouts.*

Machinery can preserve both, prevent them from overwriting each other and establish which proposal was recorded first. It cannot treat the later timestamp as a reason that privacy now outweighs accessibility.

Ordering controls coordination; it cannot make an outcome legitimate. Once the responsible authority or body rules, machinery can enforce that ruling consistently and reject an older conflicting instruction. Before the ruling, it can display the conflict and prevent accidental progress. The stop happens because people disagree about substance, so the disagreement goes to the authority that can settle it, instead of being treated as a race between two writes.

## What humans should not do

If a reviewer inspects every low-risk formatting change, consequential decisions wait behind work that strong automatic checks can settle. If someone restarts a stalled builder without recording it, the record still says the work is active, and the work now depends on help nobody can see. If an authority grants unexplained exceptions whenever a deadline approaches, enforced rules become optional in practice.

These failures lead to narrower duties. Humans should not review every machine action, relay routine state by hand or serve as an unrecorded recovery mechanism. Their attention belongs on the decisions a person must answer for, and on examination where shared machine blind spots or possible harm require an independent human view. The system presents a bounded question, relevant evidence, options and the consequence of delay.

Authority without continuous presence also needs a rule for the wait. Between the question and the answer, the work that raised it stops and keeps its records, ready to resume. Other work continues only where its recorded permission already covers it and reversal remains possible; nothing irreversible proceeds while the answer is open. When stopping is that safe, the question can wait for the person who actually holds the duty. The cost of delay belongs in the record as evidence for the decision, not as pressure on the person making it; urgency that forces approval is the approval habit in another form.

Protecting attention makes human authority usable. Machinery remains bounded by evidence, enforced rules and appeal; a human delegate remains bounded by recorded scope and accountability. When a limit fails, the failing piece is repaired: the classification, the delegation or the enforced rule.

## The change, continued

The evidence and the consequence trigger one human review, and that review is the reviewer's own recorded work; during Thursday's construction and release, the responsible authority is interrupted only once, at 9:12. The rulings that come later from production evidence (the shared-device restoration and the repeated sign-in burden) arrive through care and learning, not as interruptions of that day.
