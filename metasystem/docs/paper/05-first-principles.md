# 5. First Principles

**A condition that must hold cannot depend on every worker choosing to remember it.**

A candidate reaches the release boundary with a persuasive explanation and a passing check. The explanation says the session change is safe. The builder's own check never tried a late background response, and the candidate can still revive an expired session. If the system proceeds because the builder sounds confident, it has made trust the control. If it refuses because a required observation is missing, records the refusal and returns the candidate for repair, it has begun to act from principles.

Later chapters develop these principles: their mechanisms, their limits, the arguments against them. Here we state them briefly, so the machinery can be judged against a small set of purposes rather than a collection of inherited steps.

## What the legal metaphor means and where it stops

Suppose release is forbidden unless the exact candidate has passed a rollback check. A builder cannot waive the condition by explaining that the change is small. Authorized release cannot ignore it because the day is nearly over. The rule has power at the action it controls: it refuses release and says what evidence is absent.

Chapter 1 called such an enforced rule a law. The term names authority; it adds no ceremony. A legislator is the person or group authorized to change the rule. A judge is a person authorized to decide a named exception. A precedent is a recorded ruling that later cases may use, and an appeal is a named route for asking another authorized person or group to reconsider a decision. These terms describe permissions and continuity; they do not prescribe job titles. One person may hold several permissions, and a group may hold one permission together.

The metaphor stops at that boundary. Software has no moral agency, cannot accept accountability and cannot make a political disagreement disappear by enforcing one side. A law can refuse an unauthorized action. It cannot decide whether a rule is just. A recorded precedent can make a ruling visible. It cannot make the ruling wise. Value and political disagreement remain with people who have the authority and responsibility to decide them.

## Intent is controlling and revisable

A builder finds an easy way to sign every user out after thirty minutes from sign-in. The candidate is internally consistent and simple to test. It still fails the authorized outcome, which is based on inactivity and must not interrupt active work. The system exists to serve that human intent. Neither the builder's preferred design nor the chance to finish sooner outranks it.

Intent controls construction, checks, release and care because it states the outcome those activities serve. It does not become infallible by controlling them. Responsive users may still be ejected after release, revealing that the authorized signal for activity was wrong. Chapter 2 makes intent versioned, challengeable and revisable. When several human intentions conflict, the conflict stays visible until the authority and appeal paths described in Chapter 13 produce a binding decision.

The principle says that machinery acts under the current authorized intent, while evidence can challenge both the candidate and the intent itself. "Obey the first requirement" would be a different and worse principle. A revision produces a new controlling record; the purpose of work already performed is never rewritten in place.

## Evidence over trust

After the builder revises the candidate, it reports that expiry is now final. An independent examination sends the late response again and observes that access remains closed. The observation supports the claim in a way the builder's assurance cannot, because it distinguishes the repaired behavior from the earlier failure.

Completion claims must be tied to such observations. Evidence here means traceable results: test outcomes, independent findings, release observations and records of reversal. Proof is narrower: a conclusion demonstrated only within named boundaries and assumptions.

Trust in a capable builder can guide where to look. It cannot authorize a consequence. Chapter 6 develops the evidence required to separate a supported claim from a convincing one.

## Records are the only durable memory

A builder learns that existing sessions need separate treatment to adopt the new limit, records nothing and disappears when its session ends. The next builder sees the request but not the discovery. The system has memory only in the conversational sense, which is no memory once that conversation is gone.

Intent, current state, decisions, candidate identity, checks, refusals and observed results must live in durable records. Another authorized actor should be able to continue without guessing what the missing worker knew. The records also allow an independent examiner to receive the evidence it needs without receiving the builder's entire path of reasoning.

This principle does not claim that a record settles disagreement. Two authorities can read the same facts and choose different values. Chapter 8 develops records as the basis of continuity and coordination while limiting access by role and leaving substantive conflicts to responsible authorities.

## Important rules refuse

A release guide says that live observation should distinguish an expected rise in sign-ins from a broken sign-in loop. Under pressure, a worker may treat "should" as optional. The same condition placed at the release boundary prevents expansion until the observation exists. The first is advice. The second is an enforced rule.

When breaking a condition would make an action unacceptable, put the condition where the action can be refused. The refusal must identify the exact candidate, the unmet condition and the route by which work may continue. Otherwise enforcement is an unexplained obstacle instead of a protection whose reason and route forward are clear.

An important rule is not automatically permanent or correct. A rule can encode a bad proxy, outlive its purpose or cause harm through false refusals. Chapters 6 and 12 develop how evidence supports enforced rules and how those rules are tested, challenged, revised or repealed. The principle here is narrower: a condition that must hold cannot depend on every worker choosing to remember it.

## Spend and human authority are designed

Five parallel builders can explore the session change quickly. They also create five candidates to examine, five sets of evidence to compare and five opportunities to share the same mistaken assumption. An open-ended instruction to "keep trying" can consume computation and scarce judgment without bringing the decision closer.

Construction and verification receive explicit budgets proportional to risk. A budget is not a hope recorded beside the work. When it is exhausted, machinery stops, records what it learned and escalates or closes the attempt according to its authority. Later chapters tie that effort to consequence, novelty, exposure and accumulated change rather than to line count or confidence.

Some decisions cannot be bought with more computation. Machinery may gather information, construct alternatives and test claims. Value rulings, irreversible acts, changes to the governing rules, and accountability always end with responsible authorities. This boundary is unconditional. More evidence can improve a human decision; it cannot turn the machinery into the responsible authority.

## Failure leaves a record

An examination reaches its spending limit after finding that two devices disagree near the expiry boundary. It has not shown the candidate safe, but it has found a precise uncertainty. Calling the work complete would hide the only result that counts. Stopping with the conflicting observations, the attempted cases and the exhausted budget gives whoever makes the next authorized decision something real to use.

The system should prefer a clear refusal supported by evidence to a success it cannot justify. Failure, near-miss (a dangerous path stopped before observed harm) and uncertainty are outcomes in their own right, and they are recorded as such. They remain connected to the intent and candidate so that care can repair immediate harm and learning can improve later work. A worker may disappear; the fact that it failed and what it learned must not disappear with it.
