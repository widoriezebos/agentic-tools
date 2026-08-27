# 6. Proof over Trust

In Chapter 1's hypothetical day, the first session-expiry candidate arrives with a clear explanation. It treats a late background response as activity, so an expired session can become usable again. A controlled clock and a delayed response expose the fault in seconds.

That difference between explanation and observation is the organizing principle: convincing claims can guide inquiry, but bounded proof and well-grounded evidence authorize consequential action. The aim is an accurate account of what one exact result has and has not demonstrated.

## Evidence, proof and the boundary between them

A check sets a clock to one second after expiry, sends a request and observes rejection. That result is evidence for the claim that expired sessions cannot be used. A production measure showing that responsive users are not entering repeated sign-in loops is evidence of a different kind. An independent examiner's finding is evidence when it identifies the exact candidate, condition and observable result.

What this check can prove is narrower. Within its controlled clock, a specified session state, a stated definition of activity and the exact candidate examined, it demonstrates that rejection follows. The conclusion holds inside those boundaries and assumptions. It does not become a universal statement about every device, delay, future revision or user just because the check passed.

## Enforced rules instead of guidelines

The first candidate reaches a release step with the late-response check failing. A guide beside the work says that all session checks should pass. The builder can still proceed if the guide has no power. When the same condition sits at the release action and refuses the candidate, the missing protection becomes effective rather than advisory.

This is an enforced rule. It controls a named action, judges the exact candidate presented for that action and explains a refusal in plain language. "Release refused: a response received after expiry restores access" is governable. "Policy failed" is not. The refusal must also say what may happen next, such as returning the candidate to construction or asking a responsible authority to decide an exception.

Binding to the exact candidate is essential. Evidence from an earlier version cannot authorize a later revision, even when the difference looks harmless. Release evidence must also bind to the environment; one configuration cannot authorize another that changes the behavior. An enforced rule remains trustworthy only while its evidence refers to the exact candidate and environment that will reach users and no actor can bypass it without the named authority.

## Discriminating tests

A test passes on the repaired candidate. The independent examiner then runs it against the earlier candidate that revives the expired session. If the test also passes there, it does not distinguish the claimed protection from its known failure. The passing result measures only the test's ability to agree with both versions.

A discriminating test must fail on a relevant broken version and pass on the supported one. A preserved known-bad candidate can provide that comparison. Where no natural example exists, the independent examiner can introduce a small fault, such as reversing the expiry comparison or allowing refresh to extend the session, and confirm that the test detects it. The fault does not show that every fault will be found; it shows that this test responds to the behavior it claims to protect.

Expected results need their own source. The rule that background refresh does not extend a session comes from the responsible authority's ruling. It does not come from the builder's candidate, and it does not come from a test generator guessing what seems sensible. The test keeps that source and its own change history, so the origin of every expected answer stays traceable. Without that, a changed test can make a broken candidate look correct by changing the expected answer to match the implementation.

## Independent examiners look for faults

An independent examiner asked "does this solution look correct?" after reading the builder's full argument may follow the same path and admire the same choices. An independent examiner given the authorized intent, relevant constraints, finished candidate and resulting evidence can instead begin from the claims that must survive and look for ways to break them. The object of judgment is the work itself rather than the builder's story about reaching it.

Each new independent examiner receives the materials needed to examine the claim but not the builder's private reasoning trace or path to the work. That boundary protects a fresh perspective only when the independent examiner is actually fresh: a distinct person or a newly started machine worker that has not already seen the withheld reasoning. Removing access from an actor after exposure does not erase what that actor knows. Role-scoped access still provides least authority (no more access than the task needs), but it cannot manufacture independence after the fact. Chapter 8 develops how one durable record can support both continuity and appropriately limited access.

The independent examiner's job is active fault-finding. It tries boundary times, stale pages, sleeping devices, crossed requests and assumptions shared by the checks. A material finding returns the candidate for repair and produces another examination of the changed result. Repeated rounds stop for one of three explicit reasons: a bounded search completes without a new material issue, the judging budget is exhausted and forces escalation, or an open question requires a human ruling. Stopping is a recorded decision rather than the moment a tireless process happens to stop speaking.

## Four questions set verification depth

Changing a comma in internal help text and reversing one comparison in account authorization can each alter one line. Line count says little about the evidence either change deserves. The authorization error can expose every signed-in account; the text change may be immediately reversible and affect no behavior.

Four questions set verification depth. How severe could the harm be if the change is wrong? How unfamiliar is the approach to the system and its independent examiners? How many users or systems can it affect? How much change has accumulated since the last broad examination? Consequence sets the strength of evidence. Novelty widens challenge beyond checks shaped by the old design. Broad exposure raises the cost of one missed fault. Accumulation justifies a broad examination that catches interactions among modest changes.

Chapter 11 applies the same questions to production, comparison and the decision whether the next unit of effort is worth spending.

## What proof cannot prove

Every session check passes, but both the candidate and the checks define a visible page as proof of human activity. An unattended computer with the page open then stays signed in forever. The tests accurately demonstrate behavior under their rule. The rule is the mistake.

No proof exceeds its boundary, source of expected results and assumptions. A test derived from the same mistaken interpretation as the candidate can make agreement look like correctness. Several independent examiners using the same model or data can repeat one blind spot. A fixed set of cases cannot cover every network delay, assistive technology, future browser or user behavior the service will meet.

The response is careful language about what was shown, not despair. The system records what was examined, which expected results came from authorized intent, which sources were independent and which assumptions remain. Live observation then tests the result against situations the controlled examination did not include. Evidence can challenge intent as well as construction. Whether a burden on affected people is acceptable remains a judgment; more passing checks cannot settle it.

## Evidence that triggers human review

A reversible wording correction has well-understood behavior, a check that fails on the old wording, narrow exposure and no value dispute. Machine evidence can be sufficient to authorize it under an established rule. Requiring a person to repeat the same inspection adds delay without adding an independent source of judgment.

A one-line change that decides who may access an account is different. So is a permanent deletion, a new approach with weak tests or a choice that trades one group's safety against another's access. Independent human review is required when the evidence exposes a value judgment; when the action is irreversible or its possible harm is severe; when the work is unfamiliar and the tests do not discriminate strongly; or when builders, independent examiners and test generators may share a model, data source or assumption that could produce correlated agreement. These triggers come from the evidence, the possible consequences and the independence of the sources; job titles and calendar stages play no part. Chapter 13 says who may perform that review, how accountability is assigned and how an appeal proceeds.

Low risk does not mean no control. A path authorized by enforced rules still needs traceable intent, evidence tied to the exact candidate and a way to reverse the result. Human review adds an independent judgment required by named conditions; it does not replace those protections.

## Repair a mistaken classification

In a separate hypothetical release, a session change passes its assigned checks and proceeds without human review because it was classified as a routine timeout adjustment. Live observation then shows responsive users being signed out during long reading. Authorized release contains or reverses the change, the builder repairs the defect, and the responsible authority handles any value or rule change revealed by it. Together those acts form care.

The classification is a problem of its own. The rule failed to recognize that the activity signal involved a value choice and broad exposure. That missed trigger is treated as a defect rather than bad luck. Its record includes the evidence available before release, the evidence found afterward and the reason the earlier rule did not escalate.

Repair then tests a revised classification against this case and against cases that should remain on the path governed by automatic checks. A rule that sends every harmless wording change to a person may prevent one miss by making the system unusable. Some lessons can become an automatic refusal; others remain guidance for an independent examiner because their meaning depends on context. The aim is a better boundary rather than an ever-growing collection of permanent enforced rules.

## The change, continued

Chapter 1 already records the ruling, named checks, refused candidate and one human interruption. Here those facts set an evidentiary task: determine which observations distinguish supported behavior from the known failure.

The first discriminating check fails against the earlier behavior because the expired session still works, then passes against the repaired candidate. The independent examiner reverses the comparison on purpose and confirms that the check fails. A controlled clock covers one second before expiry, the exact boundary and one second after. Other checks introduce small differences between device and service clocks and verify that existing sessions adopt the new limit.

The four risk questions make the depth decision clear. Authentication failure can expose accounts or lock every signed-in user out; the consequence is severe. The boundary behavior is subtle enough to be unfamiliar in important cases. Every current session may be exposed. Related changes to warnings, uploads and restoration have accumulated around the expiry rule. Independent human review is triggered even if the final repair is a one-line comparison.
