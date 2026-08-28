# 14. The Transition

**Authority transfers only as fast as evidence arrives, and never without a tested way back.**

Picture a kind of team I have seen many times: the session-expiry request arrives while releases still depend on a familiar chain of tickets, code review, manual checks and a person following a production runbook. The team cannot declare that chain obsolete and replace it on Friday. Some steps may be habit. Others may hide an unwritten warning about shared devices, an approval required by policy or the only reliable way to reverse a failed session migration.

The transition begins inside that uncertainty. The proposed delivery system first builds knowledge, then takes over verification work and only later receives authority. Existing protection remains until its purpose and a possible replacement are understood. Where no need survives, the ceremony is removed with a recorded reason. This is a transfer from a running system rather than a clean start.

## Begin with the existing system

Before changing session behavior, a builder traces what the application actually does. New sessions last twelve hours, an older mobile client renews them differently, administrators receive a shorter limit, and a nightly task removes abandoned records. A test expects the twelve-hour value. A support guide tells people that closing the browser signs them out, although observation shows that this is not always true. None of these facts alone establishes intent.

Running behavior, production measures, tests, policies, incident history and current ceremonies are evidence of prior decisions. They also contain accidents, stale assumptions and known defects. The twelve-hour test may protect an intentional promise or may only freeze the behavior present when the test was written. The manual release step may preserve an important separation of authority or may remain because nobody has examined it since releases became reversible.

The team writes hypotheses that the responsible authorities, authorized representatives and users can challenge. It asks whether existing sessions change immediately, whether administrators share the policy, what closing a browser means and which interruptions are unacceptable. Reports and live behavior inform those questions without answering their value choices. The relevant intent-holders confirm requirements, identify defects and record what remains unknown.

This work is a controlled conversion of scattered evidence into revisable intent, not an attempt to recover one perfect specification from history. No such document may ever have existed. Where the evidence conflicts, the conflict stays visible until the responsible authority rules. That prevents the new system from giving accidental behavior the force of law just because it is observable.

## Establish a measurable baseline

The current team takes two days to prepare and release an ordinary authentication change. A reviewer spends several hours reconstructing which paths it touches. Manual checks find some regressions, while production incidents show that sleeping devices and delayed responses have escaped in the past. Recovery usually takes forty minutes after the release owner decides to reverse. These facts form a baseline only when their sources and uncertainty are recorded.

The baseline connects cost to protected outcomes. It measures elapsed delivery time and human attention, but also failures found before release, failures missed, the number of people exposed, time to containment, success of reversal, repeated sign-ins, lost work and unresolved support reports. Activity counts alone cannot show whether the old process is safe. A quick release that harms users is not an efficient reference point, and an expensive review that never catches a relevant fault may not be protection.

Some entries are blank. The team has never measured whether responsive readers are ejected or whether reversal restores rewritten sessions. That absence belongs in the baseline because present protection is unknown. A new measure is valuable only if it distinguishes an outcome the team cares about.

The baseline makes later claims testable. If the new path finds clock-boundary failures earlier but doubles recovery time, the trade is visible. If it lowers routine review effort while more people lose work, the transfer has failed. If both paths miss the same background-refresh error, running them side by side has revealed a shared blind spot rather than agreement.

## Coexist before replacement

For the first expiry candidate, the new builder traces behavior and raises the three interpretations for one ruling, but it does not bypass the current request and approval path. Its independent examination runs beside the existing manual checks. Its release plan is compared with the runbook. Each path records what it would accept, refuse or ask a responsible authority to decide.

The paths agree that existing sessions need coverage and disagree about a late background response. The old checklist misses that case; the new examination catches it but overlooks an administrative exception known to the support team. Neither can replace the other yet. Together they expose missing intent and checks while the current authority boundary limits harm.

This side-by-side operation is coexistence. It keeps separate records of agreement, disagreement, cost, delay and missed cases. Each path records its result before seeing the other's, so mutual influence cannot look like independent confirmation. Neither path receives credit for a failure revealed only by the other.

Coexistence should have a declared purpose and end condition. Without them, the team can acquire two permanent processes, each justified by the existence of the other. The purpose here is to test inferred intent, discriminating checks, recovery and reporting across a stated set of authentication changes. The end condition is evidence sufficient to decide which protections can transfer, which require more work and which were never real.

## Transfer verification and authority progressively

After several low-risk changes, the new checks reproduce known failures, permit legitimate behavior, bind results to the candidate and preserve recovery state. The responsible authority then lets them satisfy part of the old manual verification and still authorizes release. Construction has moved; release authority has not.

That separation is intentional. Machinery may receive permission to propose a change before it receives permission to test against sensitive data. It may receive permission to test before it may accept the result. It may accept a low-risk candidate before it may expose users, and it may release to a small group before it may expand further. Each permission is transferred only after evidence shows that the machinery can perform the action and contain its consequences.

The session-expiry change begins high on several risk dimensions: a small comparison affects every signed-in user, changes a security boundary and includes behavior not previously tested. During transition, the machinery can build and examine the candidate, while a responsible authority makes the three-interpretation ruling and retains release approval. Authorized release first observes what it would do. On a later attempt, it may release to internal accounts and stop automatically on named signals. Wider expansion remains under the responsible authority's approval until repeated evidence shows that automatic expansion and reversal protect the declared bounds.

Scope widens from reversible, familiar, limited work because failures are cheaper to study and contain. That evidence does not authorize an irreversible migration or policy waiver. Authority follows demonstrated protection across the relevant risk range.

## Retain, replace or discard according to the surviving need

The weekly authentication meeting appears to be a status ceremony. Closer observation shows that a support representative uses it to raise lockout patterns that no technical measure records, while a security representative explains new threats. The meeting carries discovery and negotiation, not just status. Until another practice reliably brings that evidence and those people into the decision, removing the meeting would leave the need for discovery and negotiation unmet.

The method described in Chapter 4 asks for that need before choosing a form. A durable live record may replace a daily status recital. A repeatable independent check may replace a manual test when it catches the same failures and permits valid cases. A discussion that exposes conflicting values remains human even when machinery prepares its evidence.

There is also a deletion branch. Chapter 4 already established the sprint case in which no need survives and the cadence is discarded. Transition does not repeat that analysis. It confirms that the recorded decision still applies, removes the ceremony without replacement and preserves the evidence and authority for the deletion. Without that record, simplification would be indistinguishable from neglect.

Replacement evidence is required only when the need remains. In that case, the old practice stays until the proposed mechanism demonstrably serves that need as well or better across a declared period and range of risk. The team may adapt the old practice while learning; it must not claim that a new dashboard preserves negotiation, accountability or independent scrutiny just because it has the same inputs on a screen.

## Make rollback part of adoption

During a limited release, the new path reports healthy sign-in rates while support receives credible reports of lost work. The measures disagree, and the new appeal path has not yet routed those reports to the release authority. The system narrows its own permission, restores the previous release procedure and session behavior and preserves both sets of evidence.

That response was designed before authority moved. Every transferred responsibility has a tested route back to the last safe process, data state and authority boundary. Reversing the application alone is insufficient if sessions have been irreversibly rewritten. Restoring the old runbook is insufficient if the people named in it no longer hold authority. A rollback identifies what can be restored automatically, what requires compensating action and which responsible authority may order it.

Rollback contains harm and restores known protection while the team examines the divergence. The next attempt may add the missing user signal, narrow exposure or retain human release authority longer. The failed comparison remains available for learning.

## Finish by removing proven duplication

After a declared period, the new examination has caught every failure found by the old session checklist, found additional boundary failures, permitted legitimate administrative cases and supported faster recovery across the agreed risk range. The responsible authority explicitly retires that checklist. Its history and retirement reason remain available, but builders no longer perform it for reassurance.

Other practices end differently. The fixed batching ceremony is removed because Chapter 4's established deletion decision still holds. The cross-functional discussion remains because it continues to expose conflicting intent. Human release approval narrows to unfamiliar or high-consequence changes, while authorized release handles established low-risk cases within observed bounds. The transition completes one responsibility at a time; no single declaration makes the new system ready.

Once duplication no longer protects anything, the team removes it with a record. The destination is not two systems forever; it is one governed path that received its authority piece by piece while the old one still ran.
