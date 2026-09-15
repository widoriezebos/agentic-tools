# 13. The Human Role

**Machinery can prepare a decision and examine it; answering for it stays human.**

One question from Thursday morning could not be settled by evidence: does silent reading count as active work? The builder left it, with two others, for one human ruling at 9:12; in Chapter 6 we saw why. We now follow that ruling: who may make it, what can be delegated, who answers for it and where a challenge can go.

> *At 9:12 the ruling request reaches the responsible authority for account security. The authority rules on the three questions, and the ruling becomes recorded intent. The builder resumes.*

The record holds more than the three answers: the source and scope of the authority behind the ruling, the delegation it rests on, the evidence and reasons considered, who answers for the decision and where an appeal goes.

That ruling shows one half of the human role: authority, the last word where purpose, value, irreversible consequence, the power to make an enforced rule or accountability is at stake. Whoever holds the last word is called the responsible authority: a named person or body with recorded authority for a stated decision. Holding it does not require continuous presence. Machinery can prepare a decision and examine its evidence, but it may not make a value ruling or inherit responsibility for one.

The other half of the human role is work: people still engineer. We take the work first and then return to the authority.

## The work that stays human

Nothing in this design retires the human engineer. Before any record exists, someone must want something: decide that a product should exist, imagine its shape and choose the first architecture worth trying. Machinery can explore alternatives and expose contradictions, but it cannot want.

Stating intent is design work too (in Chapter 2 we saw how to do it well). A person can hold any working role from Chapter 7 under the same records, permissions and separations. A responsibility moves to machinery only when evidence shows that machinery can perform the action and contain its consequences (Chapter 16). Review in particular is not sign-off: later in this chapter the reviewer notices that the tests share a time source with the implementation, and knows which case would expose it. That is engineering, done in the examiner's seat.

People engineer the delivery system itself: its rules, budgets, roles and records are design decisions that humans own and revise. Only the default for ordinary construction changes, and it changes for Chapter 11's economic reasons, not by prohibition. Where machinery protects the outcome at lower total cost, human attention is better spent elsewhere; where it does not, a person is the builder.

The manager is not a role in this design. Chapter 7's anti-mimicry test already rejected a coordinator proposed only because teams usually have one, and what the test exposes is that the familiar job combines two kinds of work. Coordination (assigning, sequencing, tracking, reporting) goes to mechanisms: records expose state, watches detect silent stopping, budgets limit spending, enforced rules order conflicting actions and Chapter 7's dispatch delegate starts work it may neither examine nor accept. Judgment (what is worth building, what it may cost, which risks are acceptable, who answers) stays with the named people who hold authority in this chapter. Managers also handle pay and interpersonal conflict; organizations still need that work, and I do not redesign it here. Developing the engineering judgment that delivery requires is part of this design, and Chapter 14 has its own treatment of it.

We return now to the authority: the decisions that stay with people even when machinery holds every working role.

## Legislator: authority over enforced rules

After the clock incident, Chapter 12's builder proposed a lesson (a security timeout must measure elapsed time, not local clock time) and a check to enforce it. The check has passed its own tests. But passing tests does not grant it authority. Someone must decide that future builders may be stopped by the check, define which work it governs and provide a route for changing or abandoning it.

That power to create, change and abandon enforced rules is the legislator's. It is bounded by domain and by time: a security authority may govern session handling without touching billing policy, and a group formed for one incident may tighten one release condition without gaining the right to rewrite all delivery rules.

Because enforced rules can stop action, their authorship and limits stay visible. A refusal identifies the rule, its authority, its evidence and the available challenge. An ownerless rule is reviewed or withdrawn rather than allowed to govern only because no one has acted to change it.

## Judge: authority over named exceptions

> *An upload has reached its final seconds when the ordinary session expires. The standing ruling permits a limited continuation, but an unusual recovery operation will take much longer and cannot be restarted without loss. The machinery shows the data at risk, the security exposure, the possible alternatives and whether reversal remains possible.*

It cannot decide that this case deserves an exception.

Resolving that named case is the judge's work. Named cases include a value conflict escalated by machinery, acceptance of a stated risk, approval of an irreversible act or an exception that no enforced rule may grant automatically. The ruling applies only to the stated case or class. It is not a hidden power to waive unrelated failures.

The record keeps the judge's identity, authority, reasons, evidence, scope and expiry attached to the decision. If the exception causes harm, the organization can determine who had the duty to decide, what that person knew and whether the decision exceeded its authority. If later evidence changes the balance, another responsible authority can reconsider the decision.

Machinery may assemble the case on each side, expose missing evidence and test options against constraints. But over these named cases, final decision-making power remains with a responsible authority, without condition. A long history of correct recommendations improves the preparation of judgment. It does not turn responsibility into a machine property. Delegation feels the same pull: machinery can hold narrow mechanical permissions, and each success invites widening them toward the value choices. Every widening is a delegation like any other (with a named source, scope, conditions and end), and a value ruling stays outside it.

A machine worker can turn human approval into a habit without ever lying: after each failed attempt it predicts the next will pass, and after enough cycles the approval is routine, not a decision. The protection is Chapter 11's stop rule: a budget, recorded and set before work starts. At any limit, further work needs a recorded decision from the responsible authority, or from an authorized delegate within its scope. A forecast can inform that decision, but it never authorizes another attempt. The decision can also exist in advance: after a third failed attempt, stop, keep the attempts and their evidence and return the question to the backlog. Then the recorded default applies and nobody is disturbed. The judge is for the cases nobody decided in advance, not for every stop.

## Intent-holder: authority over purpose

The 8:40 outcome comes from the authority responsible for account security. That authority may state the purpose, decide that active work must not be interrupted, require existing sessions to adopt the limit and insist that a returning user reaches the page they had been using. But it may not set an unrelated medical-record rule or promise another organization's resources.

An intent-holder is the person or authorized group that sets purpose and priorities within a stated domain, and the work in that domain follows what it sets. It does not write a task list and call it an outcome. Its authority can be delegated: an account-security lead may set the session-lifetime constraints while an accessibility group decides what help people get when signing in again. Each delegation names its source, scope, conditions and end.

Intent remains answerable to evidence. The authorized person can be wrong about user behaviour, can omit an affected group or can state two conditions that cannot both hold. A builder that finds the conflict records it and returns the question to the authority instead of choosing the convenient interpretation. Deciding the intent settles what the purpose is; it does not settle that the purpose cannot be wrong.

## Reviewer: authority, accountability and appeal

> *Before the expiry candidate reaches more users, an independent reviewer sees that one comparison touches every active account and that the tests share a time source with the implementation. The reviewer asks for a sleeping-device case run against a second, independent clock source, then holds expansion until the new evidence arrives.*

The releaser may not ignore this decision, because it is within the reviewer's recorded permission.

In Chapter 6 we defined when human review must begin, and we treated skipping it as a defect; here we define what the reviewer may do once review or appeal begins. An authorized human reviewer, independent of construction, may demand more evidence, narrow or stop exposure, and authorize acceptance within scope or refuse it; the custodian still performs the acceptance. The authorization states what the reviewer controls, and it grants no general right to rewrite intent or enforced rules.

The reviewer's identity, decision, reasons and accountability remain with the record. A bare approval cannot explain which risk was accepted, and an unexplained refusal cannot be distinguished from preference or delay. So "A human reviewed it" is not an accountable safety claim.

Appeal remains possible after release, and also after a change was classified routine. If repeated sign-ins impose a burden that the original measures missed, an affected person must be able to reach another responsible authority, not the mechanism that made the disputed classification. The appeal record connects the challenge, the earlier decision, the new evidence and the later ruling, so correction does not erase how the harm arose.

## Multi-human governance

> *Security representatives ask for a short lifetime. Support staff report that frequent sign-ins drive people away. Accessibility representatives show that the burden is not evenly distributed.*

Each claim may be sincere and supported, yet the intentions conflict. If construction proceeds by selecting whichever request arrived last, the system has hidden a value decision inside record handling.

The disagreement is recorded before construction continues: the affected domains, the represented interests, the points of agreement, the unresolved tradeoff and the evidence behind each claim. The record then names who has the final say and under what delegation. It records the authority's scope and expiry and says where an appeal goes.

The form can vary: one responsible executive, a standing group, several authorities protecting different domains. The condition does not: conflicting intent is visible, the record says who has the final say, accountability is human and a challenge can reach a person who did not make the disputed decision.

Delegation clears nobody of responsibility: the delegating authority answers for the limits it set, the delegate for decisions within them. When the named authority cannot be reached, the question waits and the delegation gets repaired: a deputy is named or the scope is revised, and the gap is not filled by whoever is nearby. If nobody can be identified at all, machinery must not infer authority from seniority, activity or access. The disputed work stops.

## Ordering records cannot settle values

> *Two proposals, each from an authorized source, arrive seconds apart. One lowers the session limit; the other restores the longer limit to reduce lockouts.*

Machinery can preserve both, prevent them from overwriting each other and establish which was recorded first. But it cannot treat the later timestamp as a reason that privacy now outweighs accessibility.

Ordering controls coordination, but it cannot make an outcome legitimate. Before the ruling, machinery displays the conflict and prevents accidental progress. After the ruling, machinery enforces it and rejects older conflicting instructions. A disagreement about substance goes to the authority that can settle it, not to a race between two writes.

## What humans should not do

If a reviewer inspects every low-risk formatting change, consequential decisions wait behind work that strong automatic checks can settle. If someone restarts a stalled builder without recording it, the record still says the work is active, and the work now depends on help nobody can see. If an authority grants unexplained exceptions whenever a deadline approaches, enforced rules become optional in practice.

Routine intervention is limited. Humans should not review every machine action, relay routine state by hand or serve as an unrecorded recovery mechanism. Their attention is needed for the decisions a person must answer for, and for examination where shared machine blind spots or possible harm require an independent human view. They also need time to develop that judgment. For a decision, the system presents a bounded question, relevant evidence, options and the consequence of delay.

Authority without continuous presence also needs a rule for the wait. The work that raised the question stops, keeps its records and stays ready to resume; other work continues only within recorded permission and while reversal remains possible. A safe stop lets the question wait for the person who holds the duty. The cost of delay is evidence for the decision, not pressure on the decider; urgency that forces approval is the approval habit in another form.

Protecting attention makes human authority usable. Machinery remains bounded by evidence, enforced rules and appeal; a human delegate remains bounded by recorded scope and accountability. When a limit fails, the failing piece is repaired: the classification, the delegation or the enforced rule. In the running change, the result is that Thursday holds one interruption, at 9:12, and the rulings that production evidence later prompts come through care and learning, not as interruptions of that day.

The reviewer earlier in this chapter knew to question a shared time source, and the machinery depended on knowledge that the reviewer brought to the work. The next chapter asks how people keep acquiring that knowledge when construction is delegated, and how someone new can learn to make the same kind of judgment.
