# 12. A System That Learns

**Recovering is not learning; a system has learned only when next time goes differently.**

In a continuation of the hypothetical session-expiry change, a later bounded release of the repaired design reaches live traffic on the weekend when clocks move forward. A few sessions expire at the wrong moment: one comparison uses local clock time instead of elapsed time. The releaser stops the expansion and restores the previous behavior. That is recovery, and by itself it changes nothing; the same mistake can ship again next month. This chapter solves one problem: how one failure becomes changed future behavior, without piling up rules nobody owns or notes nobody reads. What follows is design, told through the example, not a description of an existing system.

## From failure to lesson

The builder investigating the failure assembles the containment actions and observations into an incident record. It names the released version, the affected time window, what went wrong, what contained it, what evidence was kept and what questions are still open, and it keeps what was observed separate from what is suspected. From that record, the builder proposes a candidate lesson: a security timeout must measure elapsed time, not local clock time, which jumps when clocks change. An independent examiner tries to reproduce the failure, looks for other causes and checks whether the lesson covers too much or too little.

A lesson can become one of two things. If a reliable check can tell the danger from legitimate work, the responsible authority can adopt it as an enforced rule. If no check can, it becomes a question for human judgment. The incident is evidence for either and authorizes neither.

## A rule in the metasystem

An enforced rule is a feature like any other: it enters the backlog, is built, examined and accepted. What lands is a handful of files in the application's rule set. The check is code. Any threshold it reads is data. A known-bad case must fail and known-good cases must pass. And a governance record names the owner, who may maintain or withdraw the rule, the review date and the appeal route. The check runs at the acceptance boundary against each submitted candidate: if session code reads local wall time, the submission is refused. A builder can run the same check while working; that run is advice.

The governance record keeps the rule from outliving its need. Without it, checks born from incidents pile up into ceremonies again, Chapter 4's problem rebuilt in machinery. A gate reads the record. A rule without an owner, a working known-bad case and a future review date is not adopted. When the known-bad case stops failing, or the review date passes without renewal, the gate takes away the rule's power to refuse, leaves it marking and tells the owner. A rule nobody maintains loses its power by itself. Human teams mostly carry these reasons in team memory; a machine worker cannot use that. Here the reasons are a record, and the record has consequences. How much evidence adoption takes is the responsible authority's call, sized to the consequence and recorded. The delivery records close at landing, like any feature's. The governance record stays open for as long as the rule holds power.

## Proving and trusting the rule

Every new rule goes through the same trial. Before it may refuse anything, it runs in marking mode: it records what it would have refused, and the builder who proposed it reviews those marks. In the clock rule's trial, one mark turns out to be a false alarm: a calendar that formats a timestamp for display. One real problem gets no mark at all: a helper that hides the same comparison under another name. A builder repairs that miss in the application, not in the check: all time reading moves into one module, and the rule becomes an import ban that is easy to verify.

From here the path is the same for every rule. An independent examiner tests it with changes that must be blocked and changes that must pass. The whole rule set also runs against every rule's must-pass cases, because two sensible refusals can combine to block all valid work. Activation is gradual, and the owner advances each step: first warnings, then refusal in an isolated setting, then refusal for a limited class of changes, and wider power only while observations stay inside the stated bounds. Even then the rule is watched like any other machinery. What it measures can stop tracking the danger. The platform can change under it. Builders can route around it.

## What lessons become

A landed rule becomes a tested floor. Months later a faster session implementation arrives, the known-bad case shows it revives an expired session, and the check refuses it. Nobody had to remember the incident. A floor holds a minimum, not a direction, and can be repealed through its appeal route when the condition it protects no longer exists.

Sometimes the release stops before any recorded user loses access. That is a near-miss: the releaser records it, keeps the uncertainty, and it enters the same path as an incident. A lesson too weak or rare for a reliable check stays in marking mode until repetition supplies the evidence. And some lessons no check can hold, such as how much extra signing in is acceptable in exchange for a shorter session. Those become rulings by the responsible authority, recorded with reasons, scope and a reconsideration date, and open to challenge by the people they affect.

## Repair, learn and revisit

Care comes first: containment never waits for the lesson. Production judges last. If the clock failure recurs, the rule has failed and loses its standing. If legitimate work gets blocked, the rule's scope is wrong. If the people affected overturn the sign-in balance, the ruling changes. A system has not learned because it gathered prohibitions. It has learned when recorded experience changes future behavior in a way that can itself be revised.
