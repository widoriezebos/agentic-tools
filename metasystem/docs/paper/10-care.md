# 10. Care

**Only production can show what the tests could not imagine.**

The session-expiry change passes every required examination and reaches a small part of live traffic. The application remains available. Error rates stay low. Yet several people sign in again and do not return to the page where they were working. One loses text entered into a form. Another completes the sign-in step but is immediately sent out again. None of these experiences appeared in any check the candidate passed.

Release does not end delivery. Construction can create controlled evidence about a proposed behavior, but only production can show how that behavior meets real devices, real data, changing conditions and people pursuing work the tests did not imagine. Care is the continuing work of observing those outcomes, containing harm, repairing failure and revising the system when live evidence disproves an earlier claim. It is distributed: authorized release observes, contains or restores within its bounds; the builder repairs; and the responsible authority revises intent or governing rules.

## Observe production outcomes

At the start of the gradual release, the service answers requests and reports no unusual failures. By that measure the release is healthy. The intent asks for more: inactive sessions end, active work is not interrupted, an authorized upload finishes without reviving its session, and a returning person reaches the page they were using. Availability establishes none of that.

Authorized release does the watching here, and it does not decide for itself what to look at. The conditions worth watching were recorded with the intent, and the enforced rule from Chapter 1 refused release until live observation could tell an expected rise in sign-ins from a broken loop. So while the release expands, authorized release compares what production reports against the bounds it was given: expiry after inactivity, refresh that does not extend, uploads that finish, places restored. It watches the harm signals too: sign-in loops, abandoned forms, sessions that outlive the authorized limit.

Not everything arrives as a measure. Counts of expiry events and failed sign-ins come from the service itself. Patterns of use can show people abandoning a task after expiry without saying why. A support report can describe lost work that no counter names. Direct feedback from an affected person can reveal what the service cannot see at all: a warning that confused people, a shared device that now behaves unsafely. Every observation, measured or told, enters the record the same way: against the exact released version, with its source, time, affected group and uncertainty, beside the checks and rulings that authorized that version. The limits travel with it. A fall in active sessions may be successful expiry or frustrated departure. A support spike may be a severe problem for a few people or an easy-to-report nuisance. Silence from users proves nothing.

Recorded observations then move the actors with authority to act. One inside the bounds moves no one. One that crosses a bound makes authorized release contain or restore, within its own permission, and that action is recorded too. One that disproves a claim the candidate passed on becomes repair work for a builder, through the same construction and examination as any change. And one that questions the intent itself, such as whether the chosen signal reflects how people actually work, goes to the responsible authority in the narrator's report. Production reaches the system only as observations, observations become records, and records move the actors. Observation serves the outcome; it is not a dashboard of convenient activity.

## Detect drift

Three months after release, a browser update changes how sleeping pages resume. No session code changes. A response that used to arrive before expiry now arrives after it, and the application treats it differently. The behavior that passed examination in August can become wrong in November without a new candidate passing through the delivery loop.

Software lives among changing dependencies, data, users, devices, traffic and rules. Expected variation occurs inside the range the evidence already supports: more sign-ins on a busy morning, for example, do not by themselves invalidate the design. Chapter 3 used drift for a builder moving off the authorized outcome; production has its own version: a changing condition breaks an assumption that made the earlier evidence meaningful. A new device pattern, a changed outside service, a different population or accumulating data can move the application outside the conditions that were examined.

Detecting drift requires preserving those assumptions. If the expiry checks rely on a browser sending a warning response within a known interval, live observation can compare later behavior with that interval. If the release assumes that page restoration succeeds for nearly all supported paths, a sustained fall demands attention even when the application remains available. Without recorded assumptions, the system can notice difference but cannot say what the difference threatens.

Not every deviation justifies intervention. Ordinary variation remains within authorized bounds. A short-lived change can call for closer watching. A sustained change that invalidates a test, a safety claim or the intended outcome requires a new decision. Care distinguishes these cases by their relation to protected outcomes rather than by whether a graph looks unusual.

## Incidents begin with harm containment

During expansion, support receives credible reports that people on a particular device lose unsaved work after the warning. The overall error rate remains normal, and the cause is not yet known. Waiting for a complete explanation of the cause would leave more people exposed.

This is an incident: an observed or credible threat to an outcome the system is responsible for protecting. The definition includes user harm that does not appear as a technical failure. A repeated sign-in loop is an incident even if every request returns a formally successful response. A session left open on a shared device is an incident even if no user reports it. Credible evidence of harm is enough to begin containment.

The first action limits exposure. Authorized release stops expansion, removes the affected group from the new behavior when that is safe or restores the earlier version. It preserves the exact released candidate, observations, reports and actions so that containment does not destroy the evidence needed for diagnosis. It notifies the people who hold responsibility for the affected outcome and states what is known, what remains uncertain and what has been done.

Containment does not wait for a finished explanation of the cause. Its authority comes from the threatened outcome and the previously established bounds. Diagnosis continues after the immediate reach of the harm is limited. This order is important because explanations can take longer than users can safely wait.

## Rollback and repair

Before release, the session change proves that the previous behavior can be restored while old and new session records coexist. When lost-work reports appear, authorized release uses that path. Restoration is not improvised during the incident, and it does not depend on the failed candidate remaining able to repair itself.

That path is rollback: a tested return to a known safe state for a reversible change. "Known safe" is bounded. The earlier version may still have the weakness that motivated the change, but its behavior and risks are understood well enough to use while the new harm is contained. The rollback record names what was restored, which live data was affected and what evidence confirms the result.

Some actions cannot simply be reversed. A message already sent cannot be unsent. Data already disclosed cannot become secret again. A session migration may transform information in a way the older application no longer understands. Such changes need smaller stages, explicit authority before each irreversible step, and a compensating action that reduces harm when restoration is impossible. Compensation may notify affected people, revoke exposed access, reconstruct data from preserved sources or provide another path to finish interrupted work. It does not pretend that the original state has returned.

Repair then proceeds in two parts. First it restores the protected outcome: people stop losing work, unsafe sessions end, and sign-in returns them to a usable state. Then it addresses the path that allowed the failure, whether that path lies in the implementation, a weak examination, an unsafe release bound or a mistaken governing rule. The repaired candidate receives fresh verification because evidence for the failed candidate cannot authorize a later one. Rollback reduces harm; repair opens the way to another attempt.

## Live evidence revises intent

After restoration, the investigation finds that the implementation followed the written rule. A shared device used in a clinic moves between people throughout the day. Restoring the last page after sign-in exposes the previous person's context to the next authorized user. The problem is not a broken implementation. One part of the stated outcome is harmful in a setting the original ruling did not consider.

Production evidence can disprove claims at several levels. It may show that construction failed to implement the authorized behavior. It may show that a test passed without distinguishing a harmful case. It may show that the risk classification understated exposure. It may also show that the intent itself fails to serve the people it governs. Treating the original request as untouchable would preserve a clean chain from intent to behavior at the cost of preserving the wrong behavior.

Care sends the evidence back. A construction failure becomes a new candidate and new examination. A weak check becomes a stronger check whose own value must be demonstrated. A misclassified risk changes the release and review conditions. A harmful outcome returns to the responsible authority for a revised ruling. Enforced rules may also need amendment when they required or allowed the wrong action. Live evidence does not automatically rewrite intent; it creates a recorded challenge that the proper authority must resolve.

This return path keeps responsibility real. Machinery can detect the pattern, assemble the affected evidence, contain harm within prior authority and refuse further expansion. It cannot decide whether privacy on shared devices outweighs page restoration for individual devices. That value choice belongs to a responsible authority, informed by the people who live with its effects.

Later evidence from shared devices challenges the original restoration rule itself. The responsible authority revises the intent so that restoration depends on the kind of device and the privacy risk, rather than applying one rule everywhere. The change is cared for not by keeping its first statement intact, but by keeping the protected outcomes visible and revising the intent when production evidence exposes what construction could not cover.
