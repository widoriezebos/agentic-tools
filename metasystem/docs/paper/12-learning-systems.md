# 12. A System That Learns

**Recovering is not learning; a system has learned only when next time goes differently.**

A later bounded release of the repaired session change reaches live traffic on the weekend clocks move forward. A few sessions expire at the wrong moment: one comparison uses local clock time instead of elapsed time. The releaser stops the expansion and restores the previous behavior. That is recovery, and by itself it changes nothing; the same mistake can ship again next month. The problem this chapter solves: turn one failure into changed future behavior, without piling up rules nobody owns or notes nobody reads.

## From failure to lesson

The path runs through records. The builder investigating the failure assembles the containment actions and observations into an incident record: released version, affected interval, wrong behavior, containment, evidence preserved, questions open, with observation kept separate from suspicion. From it the builder proposes a candidate lesson: a security timeout must measure elapsed time, not local clock time, which jumps when clocks change. An independent examiner tries to reproduce the failure, looks for other causes and checks whether the lesson covers too much or too little. The lesson then takes one of two forms: an enforced rule, where a reliable check can tell the danger from legitimate work, or a ruling for human judgment, where none can. The incident is evidence for either and authorizes neither.

## A rule in the metasystem

An enforced rule is a feature like any other: it enters the backlog, is built, examined and accepted. What lands is a handful of files in the application's rule set: the check, which is code; any threshold it reads, which is data; a known-bad case that must fail and known-good cases that must pass; and a governance record naming the owner, the review date and the appeal route. The check fires at the acceptance boundary, against each submitted candidate: session code reads local wall time, submission refused. The same check run by a builder while working is advice.

The governance record is what keeps the rule from outliving its need; without it, incident-born checks pile up into Chapter 4's ceremonies come back as machinery. A gate reads the record: no owner, no working known-bad fixture, no future review date, no adoption; fixture stops failing or review passes unrenewed, the gate demotes the rule from refusing to marking and tells the owner. A rule nobody maintains loses its power by itself. Human teams mostly carry these reasons in team memory, which a machine worker cannot draw on; here the reasons are a record, and the record has consequences. How much evidence adoption takes is the responsible authority's call, sized to the consequence and recorded. The rule's delivery records close at landing like any feature's; the governance record stays open as long as the power does.

## Proving and trusting the rule

Before refusing anything, the check runs marking-only and its sponsor reviews the marks. One mark is a false alarm: a calendar formatting a timestamp for display. One miss carries no mark at all: a helper hiding the same comparison under another name. The miss is repaired in the application, not in the check: route all time reading through one module, and the rule becomes an import ban anyone can verify. An independent examiner then supplies changes that must be blocked and changes that must pass, and the whole rule set runs against every rule's must-pass cases, because two sensible refusals can combine to block all valid work. Activation is gradual and owner-advanced: warning, refusal in isolation, refusal for a limited class, wider power only while observations stay in bounds. Even then the rule is watched like any other machinery: its proxy can go stale, its platform can change, builders can route around it.

## What lessons become

A landed rule becomes a tested floor. Months later a faster session implementation arrives, the known-bad case shows it revives an expired session, and the check refuses it; nobody had to remember the incident. A floor holds a minimum, not a direction, and can be repealed through its appeal route when the condition it protects no longer exists. A near-miss, release stopped before any recorded user lost access, is recorded by the releaser with its uncertainty kept, and enters the same path as an incident. A lesson too weak or rare for a reliable check stays marking-only until repetition supplies the evidence. And a lesson no check can hold, such as how much reauthentication burden a shorter session is worth, becomes a ruling by the responsible authority: recorded with reasons, scope and a reconsideration date, and open to challenge by the people it affects.

## Repair, learn and revisit

Care comes first: containment never waits for the lesson. Production judges last: a recurring clock failure discredits the rule, blocked legitimate work shrinks its scope, an overturned burden changes the ruling. A system has not learned because it gathered prohibitions. It has learned when recorded experience changes future behavior in a way that can itself be revised.
