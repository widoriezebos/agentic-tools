# 14. How Engineers Learn

**The system must develop the human judgment it depends on.**

In Chapter 13, a reviewer notices that the session tests share a time source with the implementation, and knows which experiment could expose the mistake. Where did that judgment come from?

Years of building software may have supplied it. A change behaved differently from what the engineer expected, and finding the cause changed how they understood the application. Reading another person's code exposed a design they would never have chosen. Those encounters taught them what to question next time.

Construction produces software and experience. When machinery takes over the work, the software can keep improving while the people responsible for it lose opportunities to learn. Experienced engineers can lose touch with the application. New engineers may never acquire the judgment the delivery system expects of them. A complete record and a clear report leave this problem open: the reader still needs enough understanding to question what they are shown.

## What the engineer needs to learn

The knowledge required follows the work a person is expected to do. A session reviewer needs to understand how expiry interacts with requests already in flight. Someone responsible for recovery needs practice in restoring service when part of a migration has completed. The authority deciding how much interruption is acceptable needs to understand its effect on users, and may rely on a qualified examiner for the technical judgment.

The application also changes. An engineer may understand concurrent requests well and still be unaware that the current release has moved the expiry check. Keeping that engineer informed requires evidence of what changed. Developing their ability to reason about a failure requires practice in which they have to work something out themselves. The narrator helps with the first, and can direct attention to the second.

Each responsibility therefore needs an explicit account of the understanding it requires, and of how a person can demonstrate that understanding. For the session reviewer, one useful demonstration is finding a case in which passing tests leave access unprotected, then choosing an experiment that exposes the gap. A record that says someone read the report establishes much less.

## Learn by asking questions

Engineers also learn through conversation about how the application behaves. An engineer asks why a request succeeded after the session expired. The machinery explains where access was checked and shows the evidence. A follow-up question explores whether the same explanation applies to an upload already in progress.

The engineer's questions guide the conversation. The machinery can connect the behaviour to earlier design decisions and explain consequences the engineer has yet to consider. Claims stay connected to evidence, and uncertainty stays visible. When an explanation leaves something unresolved, the engineer keeps the unknown, and the engineer's questions can lead the conversation to an experiment on it.

## Learn by predicting and investigating

> *Months after Thursday's change, a new engineer works through its first refused candidate in an isolated copy of the application. The earlier diagnosis is withheld. A session expires while a background request is still in flight. The engineer predicts that access will remain closed and asks to send another request after the response arrives.*

> *The request succeeds. The engineer follows the response through the application, finds where it updates activity and predicts what will happen if that update is removed. The machinery makes the trial change and repeats the experiment. Access stays closed.*

The machinery prepares the environment and carries out the experiment the engineer chose. In this exercise, the engineer has to commit to an expectation before seeing the result, and then investigate the difference. That is an encounter from which judgment can grow. An explanation supplied afterwards can help the engineer understand the result and its limits.

The source of the expected behaviour stays visible: the recorded ruling says that a background refresh must not extend the session. The experiment shows what this candidate does. A teaching agent's agreement establishes neither fact.

Sometimes the useful work is reading the relevant implementation. Sometimes it is writing a small version personally, or diagnosing a failure without asking the agent for its answer. The choice depends on what the engineer needs to learn. Machinery can remove the setup work while leaving the investigation with the person. The exercise ends when the person can account for the result and apply the reasoning to a changed case.

## More experience than chance provides

Ordinary development exposes an engineer to whichever problems happen to arrive. The metasystem retains cases that can be chosen for what they teach. A refused candidate shows a mistake caught before release. A production incident shows what the examination missed. Both can be investigated safely with the earlier answer withheld.

The session case can be varied by changing which clock the test controls. In another case, an authorized upload finishes after the ordinary session closes. The engineer chooses an observation that would reveal whether the upload has acquired too much permission. Each variation asks them to decide whether the earlier explanation still applies.

This can give people useful experience faster than waiting for failures in their own work. Engineers can encounter rare faults and compare approaches from other applications. But that benefit remains a claim to test. Generated cases may repeat the teaching agent's assumptions, and an isolated environment may omit the condition that caused a real failure. Retained observations and examination by experienced people keep the cases grounded. Contact with users is still needed to learn what the software's behaviour means for the work they are trying to do.

A newcomer needs a progression through work they can understand and check. They can begin by constructing a small session handler, then diagnose faults in another version before examining a proposed change. As they learn to choose their own experiments, they may need less help. Maintaining the expertise of today's reviewers is insufficient if nobody can develop the people who will replace them.

## Find out whether learning lasts

Repeating an explanation immediately after hearing it is weak evidence of learning. A later case must require the engineer to work out what matters again.

> *Weeks later, the new engineer examines a different session failure. An agent blames a sleeping device and recommends extending the timeout. The engineer asks to repeat the case while the device stays awake. The same failure occurs. The agent's explanation is insufficient, and the engineer asks for further investigation before accepting the proposed remedy.*

A useful assessment includes sound work and also plausible wrong advice. The engineer must be able to challenge that advice without rejecting every recommendation. And they should recognize when the available evidence is insufficient, and seek help before acting.

The expected results need support that is independent of the agent that taught the case. That support can come from reproduced failures, and it can also come from checks examined by another qualified engineer. Assessment uses unfamiliar cases, and it returns to the subject after time has passed. It establishes what the person could do under those conditions. But it cannot certify every future judgment.

The paper's separation of construction and examination still holds. A person who learns through a builder's private reasoning about a candidate has seen the path that an independent examiner must be free of. They may investigate it for learning. But its independent examination belongs to someone else.

## Give learning an owner and a budget

The responsible authority funds the learning required by the work it delegates. Qualified engineers help identify what must be learned and examine whether the practice develops that learning. Machinery can prepare cases and retain the evidence. But a machine score grants no authority over people.

When suitable expertise is missing, the authority brings in a qualified examiner or narrows the work while the necessary understanding develops. A human approval appended to the work cannot supply absent expertise. The same applies when the application changes faster than its reviewers can follow: the scope of delegated work has to fit the available judgment.

Learning consumes time, and that time belongs in Chapter 11's account of total cost. A person constructing a small component may cost more than a machine producing it, yet the experience can improve later examination. The value is tested through what the person can subsequently do. Completing exercises or attending a recurring meeting is an activity count, with the same limits as every other activity count in this paper.

No fixed practice can promise complete compensation for the experience that delegation removes. The test is whether people retain the abilities their responsibilities require, and whether newcomers can acquire them. Compare that evidence over time with the earlier way of working, including the cost of learning. Where the new arrangement falls short, change the practice or the scope of delegation. Where the new arrangement develops stronger judgment, that improvement is part of what the metasystem has produced.
