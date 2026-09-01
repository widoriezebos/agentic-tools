# 12. A System That Learns

**Recovering is not learning; a system has learned only when next time goes differently.**

> *In a continuation of the hypothetical session-expiry change, a later bounded release of the repaired design reaches live traffic on the weekend when clocks move forward. A few sessions expire at the wrong moment: one comparison uses local clock time instead of elapsed time. The releaser stops the expansion and restores the previous behavior.*

That is recovery, and by itself it changes nothing; the same mistake can ship again next month. Here we solve one problem: how one failure becomes changed future behavior, without piling up rules nobody owns or notes nobody reads. What follows is design, told through the example, not a description of an existing system.

## From failure to lesson

The builder investigating the failure assembles the containment actions and observations into an incident record. It names the released version, the affected time window, what went wrong, what contained it, what evidence was kept and what questions are still open, and it keeps what was observed separate from what is suspected. From that record, the builder proposes a candidate lesson: a security timeout must measure elapsed time, not local clock time, which jumps when clocks change. An independent examiner tries to reproduce the failure, looks for other causes and checks whether the lesson covers too much or too little.

A lesson can become one of two things. If a reliable check can tell the danger from legitimate work, the responsible authority can adopt it as an enforced rule. If no check can, it becomes a question for human judgment. The incident is evidence for either and authorizes neither.

Nobody has learning as a job of their own; each step already has an owner. The releaser records containment actions and near-misses. A builder assembles the incident record, investigates and proposes the lesson. An independent examiner challenges it. The responsible authority adopts it, turns it into a ruling or declines it, and an adopted lesson enters the backlog as a feature like anything else. One thing must not be left to memory: that the loop starts at all. A rule's review date meets the gate whenever the rule fires; a record that crosses no boundary needs a standing watch instead, and the design adds one over all dated records. An incident record carries a decide-by date. If it is still undecided after that date, the watch flags it to the responsible authority, who assigns work or declines it with a recorded reason.

## A rule in the metasystem

An enforced rule is a feature like any other: it enters the backlog, is built, examined and accepted. What lands is a handful of files in the application's rule set. The check is code. Any threshold it reads is data. A known-bad case must fail and known-good cases must pass. And a governance record names the owner, who may maintain or withdraw the rule, the review date and the appeal route. The check runs at the acceptance boundary against each submitted candidate: if session code reads local wall time, the submission is refused. A builder can run the same check while working; that run is advice.

The governance record keeps the rule from outliving its need. Without it, checks born from incidents pile up into ceremonies again, rebuilding Chapter 4's problem in machinery. At the acceptance boundary, the gate runs every rule and checks three facts in the rule's governance record: a current owner is named, the review date is still in the future, and the check still fails on its known-bad case. A new rule that lacks an owner, a known-bad fixture or a review date never receives refusal power.

After adoption, governance decay never weakens an otherwise working rule. A missing owner or passed review date leaves the rule enforcing its existing condition, while the watch over dated records flags a forced decision to the responsible authority: renew the rule, assign a new owner or retire it. Every resulting refusal says that governance is overdue and gives the appeal route. The rule keeps its power until that decision is recorded.

A broken check is different. When the known-bad case no longer fails, the check has failed its minimum discrimination test, so a refusal produced by that check cannot justify rejecting a candidate. The report records the check failure separately from candidate results and tells the owner. For an ordinary rule, the gate puts the rule in marking mode: it records what the check would have refused but refuses nothing. For a rule protecting an outcome classified as severe, the gate instead refuses every submission in the rule's recorded scope except a repair proceeding through the separate rule-change path. That refusal comes from the adopted fail-closed response, not from the broken check.

Every broken check creates a repair record with a budget and a decide-by date. The watch flags it to the responsible authority, who must assign a builder to restore and independently examine a discriminating check, replace the protection, narrow the scope or retire the rule. Keeping the scope closed beyond that date requires another recorded decision, budget and date. The broken check neither approves nor blocks its own repair.

The governance record states the broken-check response and its scope. At adoption, the responsible authority chooses them from the four risk questions used for verification and spending, keeps the four answers separate, and records why passing unverified work or closing the scope is the lesser risk. Human teams mostly carry these reasons in team memory; a machine worker cannot use that. Here the reasons are a record, and the record has consequences. How much evidence adoption takes is the responsible authority's call, sized to the consequence and recorded. The delivery records close at landing, like any feature's. The governance record stays open until the rule is retired, whether it is marking, refusing or awaiting repair.

## Proving and trusting the rule

Every new rule goes through the same trial. Before it may refuse anything, it runs in marking mode, and the builder who proposed it reviews what it marked.

> *In the clock rule's trial, one mark turns out to be a false alarm: a calendar that formats a timestamp for display. One real problem gets no mark at all: a helper that hides the same comparison under another name. A builder repairs that miss in the application, not in the check: all time reading moves into one module, and the rule becomes an import ban that is easy to verify.*

From here the path is the same for every rule. An independent examiner tests it with changes that must be blocked and changes that must pass. The whole rule set also runs against every rule's must-pass cases, because two sensible refusals can combine to block all valid work. Activation is gradual, and the owner advances each step: warnings first, then refusal in an isolated setting, then refusal for a limited class of changes, then full power. A step is advanced only while the rule behaves within the bounds its record states, such as how many of its refusals turn out to be false alarms. And full power is not permanent trust. The check tests for local clock time, not for the danger itself, and the two can drift apart: a new time interface can carry the same danger past the old pattern, and a builder blocked by the rule can reach the same result another way. The known-bad case, the review date and the appeal route are how that drift gets seen.

## What lessons become

While its check still discriminates, a landed rule is a tested floor. When an ordinary rule falls back to marking, the report states that its floor is not being enforced. When a severe rule's check breaks, the gate closes its recorded scope until a tested protection is restored or the responsible authority changes the rule.

> *Months later a faster session implementation arrives, and the clock rule catches it: the new code reads local wall time in its expiry comparison. The check refuses it, and nobody had to remember the incident.*

A floor holds a minimum, not a direction, and can be repealed through its appeal route when the condition it protects no longer exists.

Sometimes the release stops before any recorded user loses access. That is a near-miss: the releaser records it, keeps the uncertainty, and it enters the same path as an incident. A lesson too weak or rare for a reliable check stays in marking mode until repetition supplies the evidence. And some lessons no check can hold, such as how much extra signing in is acceptable in exchange for a shorter session. Those become rulings by the responsible authority, recorded with reasons, scope and a reconsideration date, and open to challenge by the people they affect.

No lesson is final. If the clock failure comes back, the rule failed, and its owner must fix it or withdraw it. If the rule blocks legitimate work, its scope is wrong and gets narrowed. If the people affected overturn the sign-in balance, the ruling changes. Every learned change stays open to revision on new evidence.
