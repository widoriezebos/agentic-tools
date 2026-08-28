# 12. A System That Learns

**Recovering is not learning; a system has learned only when next time goes differently.**

In a continuation of the hypothetical session-expiry change, a later bounded release of the repaired design reaches a small share of live traffic on the weekend when clocks move forward. Most sessions behave as expected. A few expire at the wrong moment because one comparison uses local clock time while the rest use a continuous measure of elapsed time. Authorized release stops the expansion and restores the previous behavior. The immediate danger passes, but the delivery system has not yet learned anything. It has only recovered.

Learning begins when experience changes what happens next time. That change needs discipline. A hurried ban on every use of local time may stop the known failure and also prevent legitimate work such as displaying an appointment in the user's time zone. A note in an incident report may avoid that damage and still be forgotten by the next builder. The useful lesson is a governed change to future behavior, supported by evidence and open to challenge.

## Incidents produce candidate lessons

During recovery, the team knows that some people were signed out at the wrong time. It does not yet know whether every affected session followed the same path, whether another clock error remains or whether the observed support messages represent all the harm. The incident record separates what was observed from what is suspected. It names the released version, the affected interval, the wrong behavior, the action that contained it, the evidence preserved and the questions still open.

That separation is needed because the first explanation after a failure is often plausible and incomplete. If the explanation becomes a rule before it survives examination, the system can preserve the wrong lesson with more force than a person's mistaken memory ever had. The record instead produces a candidate lesson: elapsed security limits must not depend on civil-clock jumps. An independent examiner can then try to reproduce the failure, seek other causes and show whether the proposed lesson covers too much or too little.

The candidate may take one of two forms. Where a reliable check can distinguish the known danger from legitimate work, a responsible authority may adopt it as an enforced rule with power to refuse an action. Where the lesson concerns a value choice, a weak signal or a situation no reliable check can recognize, it becomes a question or prior ruling for later human judgment. An incident supplies evidence for either path. It does not authorize either path by itself.

## Govern each enforced rule

Suppose the proposed clock rule refuses any session-limit calculation that reads local wall time. Before it receives that power, its sponsor must show more than the original failure. An intentionally broken candidate using local time should fail the check. Correct candidates using an elapsed-time source should pass. Code that only formats a local time for display should also pass. These cases establish both sides of the boundary: what must be stopped and what must remain possible.

The enforced rule's record names how much evidence is required before adoption and why that amount fits the possible harm. It defines the parts of the system it may judge, the condition it may refuse and the evidence it must attach to a refusal. It names an accountable owner with authority to maintain or withdraw it. It also names the signals that would reveal side effects, the date or event that forces review, and the route by which a builder or affected person can appeal.

These details follow from the power being granted. Because an enforced rule can stop future work, its scope cannot be left to implication. Because the world and the software will change, its owner and review point cannot be omitted. Because its test may be wrong, refusal cannot be the end of the conversation. Authority without those limits would turn one incident into an indefinite veto held by an unexamined mechanism.

Adoption can still be proportionate. A narrowly understood recurrence with severe consequences may justify a firm rule after strong reproduction and independent examination. A weak association may justify observation only. The evidence threshold is part of the decision; there is no universal number. The responsible authority decides what uncertainty is acceptable for the consequence at hand and leaves that reason in the record.

## Test the enforced rule itself

The new check first runs without refusing anything. It marks proposed changes that it would have stopped and records why. One mark reveals a false alarm: a user-facing calendar legitimately converts a timestamp to local time after the security decision has already been made. Another reveals a gap: a helper hides the same unsafe comparison behind a different name. The trial turns two abstract concerns (overreach and evasion) into observed cases before the check controls delivery.

The enforced rule is then revised and examined independently of its author. The independent examiner supplies changes that must be blocked and changes that must be permitted. It also tests interaction with existing rules, because two individually sensible refusals can make all valid work impossible when combined. Gradual activation follows: warning, refusal in an isolated setting, refusal for a limited class of live changes and wider authority only while the observations remain inside stated bounds.

A passed trial does not make the enforced rule permanently trustworthy. The check may measure a proxy that later stops tracking the danger. A platform change may make its assumptions false. Builders may route around it to finish legitimate work, creating a less visible risk. The same skepticism applied to a software change applies to the mechanism judging changes. Its results bind only within stated conditions, and its continued authority depends on later evidence.

## A tested floor, not one-way progress

Months later, a new session implementation arrives. It is faster and simpler, but one intentionally broken case shows that a late background response can revive an expired session. The existing check refuses it. No reviewer needs to remember the earlier rejected candidate from the original expiry change; the protection has become a tested floor that a later change cannot cross unnoticed.

Such a floor preserves a minimum behavior; it does not set a permanent direction. A dependency may remain below a known vulnerable version. An expired session may remain unable to revive. A recovery path may remain demonstrable before release. These are conditions that later work must either satisfy or challenge through the named appeal route.

The floor is not a ratchet under every measure. Shorter session lifetimes are not automatically safer when repeated sign-ins lock people out or destroy work. More tests are not automatically better when they repeat one assumption and consume attention needed elsewhere. Even a well-tested floor can be repealed when the condition it protects no longer exists or a better mechanism replaces it. The record makes such a change reviewable instead of impossible.

## Near-misses widen the evidence base

In another region, observation shows a cluster of sessions approaching the same faulty clock path, but gradual release stops before any recorded user loses access. No harm is observed. The event still reveals that the failure could escape under conditions not covered by the first reproduction.

The record calls this a near-miss and preserves its uncertainty. It does not claim that nobody was harmed just because no report arrived, nor that harm certainly occurred because it was possible. It records the exposure, the intervention and the missing evidence. That wider base can strengthen the clock rule, expose a weak production signal or justify a narrower rollout next time.

Near-misses enter the same governed path as incidents because absence of visible harm does not make their interpretation reliable. If every alarming trace created a new refusal, noisy systems would accumulate rules faster than they could examine them. If only proven damage counted, the system would discard warnings that arrived cheaply. Preserving the uncertainty allows later evidence to change the conclusion without rewriting history.

## Some lessons cannot become automatic refusals

The expiry release also raises a different question. Support data shows that more people must sign in again, and accessibility representatives explain that repeated authentication places unequal burdens on some users. No clock check can decide how much reauthentication burden is acceptable in exchange for a shorter unattended session. The disagreement concerns the product's values and the distribution of harm; it is not a defect with one mechanically correct answer.

Machinery can gather the sign-in rate, identify affected paths, compare alternatives and show where the evidence is weak. It cannot make the value ruling or take accountability for it. The lesson stays a question routed to the responsible authority. The ruling records the chosen balance, its reasons, its scope, the evidence considered and a date for reconsideration. Later builders receive it as prior guidance, while affected people retain a route to challenge it.

Other lessons may remain warnings because the event is rare, the signal is weak, or a fixed test would be easy to satisfy while missing the real danger. Requiring human judgment each time has a cost, and the system should make that cost visible. Repetition may eventually supply enough evidence for a narrow enforced rule. Until then, pretending that an unreliable test has settled the matter would hide uncertainty rather than reduce it.

## Repair, learn and revisit

When the clock failure appears, the first action is containment. Authorized release narrows exposure or restores the previous safe version while the responsible authority receives a clear account of possible user harm. A complete lesson is not made a condition of recovery. Delaying repair until every cause is known would make learning compete with care.

After containment, the builder corrects the elapsed-time comparison, an independent examiner reproduces the old failure and challenges the repair, and an enforced rule binds the passing evidence to the exact candidate. The candidate lesson then follows its own path through evidence, ownership, testing, limited activation and appeal. One result becomes the clock rule. The question of acceptable reauthentication burden remains a human ruling with an owner and review date.

Learning is complete only when later production behavior tests both outcomes. If the clock failure recurs, the evidence no longer supports the enforced rule's authority. If legitimate work is blocked, its scope is wrong. If the reauthentication burden changes or affected people overturn the earlier balance, the ruling must change. A system learns not when it accumulates prohibitions, but when recorded experience changes future behavior in a way that can itself be revised.
