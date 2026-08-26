# 10. Care

Thesis: delivery continues after release because production behavior
and user experience are evidence the construction loop cannot create.

## Observe production outcomes

The system must compare live behavior with the observable success and
harm criteria attached to intent, not merely report uptime. Technical
signals, support reports, user behavior, and affected-user feedback
provide different evidence and must keep their sources and limits
visible.

## Detect drift

Software, dependencies, data, users, and operating conditions change
after a release, so a once-correct result can become wrong without a
new code change. The chapter will distinguish expected variation from
drift that invalidates assumptions, tests, or intent and requires a
new decision.

## Incidents begin with harm containment

An incident is an observed or credible threat to a protected outcome,
including user harm that does not appear in a technical error rate.
The first duty is to limit exposure, preserve evidence, notify the
people with responsibility, and avoid making recovery depend on a
complete explanation.

## Rollback and repair

Reversible releases need a tested path back to a known safe state;
irreversible changes need staged rollout, compensating action, and
explicit human authority. Repair restores the protected outcome and
then addresses the path that allowed the failure, with both steps tied
to records and fresh verification.

## Live evidence revises intent

Production evidence can show that the implementation is wrong, that a
test was weak, or that the stated intent itself causes harm or fails to
serve users. The care loop therefore sends evidence back to intent,
verification, risk classification, and governing rules rather than
treating the original request as untouchable.

## The change, continued

After session expiry is released gradually, the system watches failed
requests, reauthentication success, support contacts, and signs that
people are locked out or remain signed in too long. A spike in lost
work triggers rollback or repair, and evidence that a single expiry
rule harms shared devices can force revision of the original intent.
