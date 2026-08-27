# 2. Back to Intent

The request from Thursday morning says to change how long a user stays signed in. It does not say whether the change is complete when new behavior exists on a builder's machine, when a check passes, when every user receives it or when the service has lived with it without causing harm. Those moments are not the same, and treating them as one makes a short request look complete long before a dependable outcome exists.

Chapter 1's ladder shows that tools can absorb bounded work, not which parts of delivery remain necessary when the old division of labor changes. So we start simpler: describe what must happen on the way from a human need to dependable behavior, apart from the process used to arrange the work.

## What delivering software actually requires

A person first has to decide what better means. For the session change, better might mean reducing the time an unattended account remains usable without interrupting someone who is present and working. That choice comes before any construction. It also contains a tension: security improves when the service signs people out sooner, while continuity improves when their work survives. Deciding the outcome and its acceptable limits is the activity this paper calls forming 'intent'.

A builder then has to alter the application. It traces how sessions begin, what extends them, where expiry is checked and what the user sees afterward. It chooses among possible designs and produces a candidate. That is 'construction'. A candidate can be elegant and still serve the wrong outcome, so construction cannot certify itself just by ending.

The candidate then meets cases that can distinguish success from failure. A clock reaches the expiry boundary. A background response arrives late. A reader answers a warning. An upload crosses the limit. Existing sessions encounter the new rule. These observations determine whether the candidate supports the claims made for it. That activity is 'verification': finding out what the evidence supports saying about the candidate. It includes automatic checks and independent examination but is not identical to either.

Release does not end the obligation. Sign-ins may begin looping, uploads may fail, or a pattern that looked harmless in a controlled setting may interrupt real work. That continuing obligation is the activity this paper calls 'care'; Chapter 10 develops how it is shared.

Finally, experience must change what happens next. A missed boundary may become a stronger check. A confusing request may lead to a better way of stating success. A rule that produces false refusals may need revision or removal. That is 'learning': improving intent, construction, verification and care from recorded experience.

These five activities are how the problem divides, not how teams divide the work. They can overlap, and one actor may perform several when consequence is low. They are separated here because each answers a different question. What should happen? What candidate could make it happen? What supports the claim that it does? What keeps the result dependable? What should change after experience? A delivery design may distribute those questions in many ways, but it cannot omit one without leaving a corresponding obligation unanswered.

## Intent as the durable interface

Suppose the responsible authority sends the original sentence to three builders. One interprets it as thirty minutes after sign-in, even while the user is active. Another chooses thirty minutes without direct input. A third changes only newly created sessions. All three can claim to have followed the words. The variation appears before anyone writes the change; it lives in the distance between desire and a statement precise enough to guide action.

A durable record of what is wanted needs more than a task name. It states the desired outcome, the constraints that must hold, the freedoms left to construction, and the observations that would count for or against success. For the session change, it might state that an unattended account becomes unusable after thirty minutes without activity, active work is not destroyed without warning, existing sessions receive the new limit, and signing in again returns the user to the page they were using. It need not prescribe where the change belongs or how many steps produce it. Those are choices for construction unless a constraint makes them part of the outcome.

That record is the intent: the durable interface between the people who need an outcome and the system that constructs and cares for the application. It does not predict every situation, it is not a disguised task list, and a person having written it does not make it sacred. A builder can propose a design against it. An independent examiner can derive challenges from it. Authorized release can compare live behavior with it. A responsible authority can revise it without reconstructing the entire history from conversation.

Versioning is needed because a statement can change while work is under way. If the responsible authority rules that a user-started upload may finish under a narrow continuation, the earlier statement and the ruling must remain distinguishable. The candidate, checks and release decision then bind to the later version. Without that connection, a passing result may answer a question nobody is still asking.

As construction becomes easier to repeat, this interface becomes more important: ten candidates produced against an unclear outcome are not ten times the progress, just ten results to judge against the same uncertainty.

## Intent can be wrong

A support team asks for a warning before sign-out because users report lost work. The security authority asks for no warning because any extension could keep an unattended account open. Both requests sound precise. Together they conflict unless the warning requires direct action and has a bounded duration. Construction cannot resolve this conflict. The defect is an unresolved relationship between two desired outcomes.

Other defects are quieter. "Active work" may include typing, silent reading, a background refresh or just keeping a page visible. "Existing sessions" may mean sessions created before release, sessions on every device or only sessions that contact the service again. "No disruption" may conceal an impossible promise. Asking for examples at the edge exposes these meanings: what happens to a reader who does nothing, a reader who responds, a sleeping laptop and an upload already in progress? Asking what observation would reveal failure exposes weak success language: fewer complaints may reflect fewer users rather than a safer session policy.

Drawing out intent means turning unspoken expectations into statements people can inspect. It includes asking for counterexamples, naming affected people, comparing constraints and stating measures that could prove the desired result wrong. Machinery can help by finding ambiguous terms, generating boundary cases and showing where two constraints cannot both hold. It cannot decide which value should win. When shorter sessions protect accounts but interrupt legitimate reading, the choice returns to a responsible authority with the power to bind it.

Even a clear and internally consistent intent can be wrong. After release, responsive users may be signed out because the chosen signal does not reflect how they work. The outcome may protect the service while harming the people it was meant to protect. Live evidence then challenges not only the candidate but the statement that authorized it. The record must allow the responsible authority to revise the intent, connect the new decision to the evidence and show which earlier candidates and checks no longer support the current claim.

This is why intent is versioned rather than declared once. Revision is the expected response when ambiguity becomes visible, values conflict, or reality disproves an assumption. It is not a failure of discipline. The change must be explicit, authorized and traceable, so the delivery system cannot move from one purpose to another without anyone noticing.

## From stated desire to a checkable claim

Before construction begins, the sentence about staying signed in becomes a small set of examples. Before expiry, an inactive reader receives a warning. At thirty minutes without a direct response, the session no longer works; a reader who responds remains signed in. A background refresh does not extend the session. A permitted upload finishes without reopening the ordinary session. Existing sessions adopt the new limit, and signing in again restores the page. Each example gives a person something concrete to challenge before it becomes a claim about the application.

Examples alone are not enough. Boundaries name where behavior changes: before expiry, at expiry and after it; ordinary requests versus the narrow upload permission; new sessions versus existing ones. Observations name what can be seen: whether access succeeds, whether the warning appears, whether the upload finishes, whether the session revives and whether the page returns after sign-in. Together they turn a stated desire into a checkable claim without dictating how the builder must construct the result.

Turning words into checks can reveal uncertainty, but it cannot manufacture agreement. If the words do not determine whether passive reading counts as activity, the system leaves that question visible. If two authorities supply incompatible constraints, it records the conflict; it does not pick one on its own. The unresolved question returns to the person or body authorized to decide it. Only the resulting ruling becomes controlling intent.

Chapter 1 asked what software delivery actually requires, apart from the process inherited to organize it. The five activities are the answer. What remains is a design problem: arrange them for a workforce whose limits differ from ours, rather than copy the old workflow onto new workers.
