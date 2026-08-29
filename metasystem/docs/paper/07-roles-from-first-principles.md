# 7. Roles from First Principles

**Roles come from hazards, not from the organization chart.**

Chapter 1's day records the ruling, construction, independent challenge, refusal, release and report. Here we turn them into a distribution of power. Giving all of those functions to one capable actor would let the same actor form a theory, defend it, approve it and describe its success.

Copying a familiar software department into machinery is not the answer. A role in this system is a temporary combination of two things: the actor's relation to the work (constructing it, challenging it, preserving its history or explaining it) and its permissions (what it may read, change, accept, stop or release). The combination exists because a particular hazard requires it, and it lasts only as long as that hazard does. A hazard here is a specific way the work can go wrong: a builder judging its own claim, a worker stopping in silence, a convincing retelling steering a decision. Risk, as Chapters 6 and 11 use it, sets verification depth and spending through four questions: severity, novelty, exposure and accumulated change. Hazards decide which roles must exist and stay separate. The two are handled differently. Risk shifts as the application evolves and runs, so it is weighed again for every change. A hazard is answered in the design, by the roles it requires.

## The premise: only some limits were removed

Chapter 6 establishes that a fresh perspective needs contextual independence: an examiner not already exposed to the builder's path. Here that becomes an authority question: a new machine worker can take a temporary role without a reporting line or a permanent title, but prior exposure can still disqualify it from serving as the independent examiner.

Several constraints that once justified separate roles have weakened. Working hours no longer require one shift to hand work to another. A career ladder need not determine who performs a check. Scarcity among specialists counts for less when a bounded capability can be supplied when needed. One manager's span of attention no longer sets the number of active attempts.

Other limits remain. The meaning of "active work" still needs judgment. The application still contains more context than any one attempt can safely consider. Permission to alter live accounts still creates danger. An independent examiner can still share the builder's mistaken assumption. A worker can still stop without saying so. A persuasive explanation can still distort a decision. Cheap copying removes some causes of organization, but it does not remove scarce judgment, incomplete knowledge, authority, workers that can disappear or the need for a truly independent view.

Roles begin with the limits that survive. They are not permanent identities assigned to workers. The same kind of worker may build on one change and examine another, provided prior exposure does not defeat the required independence and the permissions do not create a prohibited combination. The design question is what danger appears when a necessary relation or power is missing; titles do not enter into it.

## Hazards determine required separations

The first session-expiry candidate treats a delayed background response as fresh activity. The builder can explain why the design looked reasonable: the application already uses recent requests as evidence that a session is alive. That explanation is useful, but it also reveals a commitment. The builder has selected a model of the problem and invested effort in making it work. Authorship does not make the builder dishonest. It makes acceptance from the same viewpoint weaker: the assumptions that most need challenging are the ones the builder now sees the result through. So building and accepting are kept separate, and acceptance needs an independent examiner.

Now suppose the change must update existing sessions. Construction needs permission to prepare that update, but a mistake in applying it could sign out every user or damage stored state. The power to explore and construct is broad in what it may try and narrow in what it may touch, compared with the power to act on live data. Because of that destructive reach, permissions are also kept separate: permission to make a candidate does not imply permission to replace data, stop work or release the candidate. Authority narrows as an action becomes harder to reverse or affects more people.

The candidate and its evidence also need to survive the actors that produced them. A builder can stop. An independent examiner can be replaced. A responsible authority can return the next day. If the meaning and state of the work live only in any one of them, the work is orphaned when that actor disappears. Because every worker can disappear, records need ownership that outlives the current worker, and an actor must be responsible for preserving the chain between intent, candidate, evidence and acceptance. That actor is the custodian.

Two further hazards arise around the work rather than inside the candidate. A stopped worker may just look slow unless another actor observes whether it is alive and making progress. A technically complete record may be unreadable to the person who must rule on an exception. Silent death requires a liveness watcher. A misleading retelling requires a narrator.

## The roles

Those hazards give the delivery system six working roles. Here is what each may do and may not do; the rest of the paper relies on these names.

A builder constructs a candidate against the recorded intent. It explores the application, chooses a design and changes only what its task permits it to change. It does not judge its own claim, accept its own work or authorize release.

An independent examiner judges a finished candidate without prior exposure to the builder's path. It starts from the claims that must survive and tries to break them. It does not change the candidate; repairs go back to a builder.

A custodian keeps the records that connect intent, candidate, evidence and acceptance, so the work survives any single worker. When every required fact is present, it performs the acceptance action. It builds nothing and examines nothing.

A releaser moves an accepted candidate into production and stands watch over it. It exposes the change to a small part of live traffic first, compares what production reports against the bounds recorded with the intent, and expands, pauses, contains or restores within those bounds. It builds nothing, examines nothing and accepts nothing; its authority begins where the custodian's ends and reaches no further than its recorded bounds.

A liveness watcher observes whether active workers are alive and making progress. Within narrow, checked authority it may stop or replace work that has gone silent, and it may not erase the record of why a worker stopped.

A narrator turns the record into a report a person can act on. Of everything the system writes, that report is the one output made for people rather than machinery. It changes no state and makes no decision.

Any of the six can be held by a person or by machinery; the configurations later in this chapter return to that choice. All six work under the responsible authority from Chapter 1: a named person who rules on values and exceptions and answers for the consequences.

## Prohibited combinations

Suppose the builder of the session change runs its own checks, declares them sufficient and accepts the candidate. The record may look complete, yet the original assumption about background refresh can pass from design into test without encountering another point of view. The builder may check its work, but it may not be the authority that accepts its own claim. The combination is prohibited because it puts building and accepting back in the same actor.

The separation cuts the other way too. An independent examiner discovers that a late response revives an expired session. If it changes the candidate itself and then continues examining it, it becomes an unrecorded builder and begins judging its own repair. It may propose a correction or return a precise finding, but a builder must perform the change and a fresh independent examination must judge the new candidate.

Custody and observation have similar limits. The actor that binds an exact candidate to its passed evidence may not also construct or examine that candidate. Otherwise the custodian could accept a candidate it constructed or rely on an examination it performed itself. A liveness watcher may stop or replace work only through narrow, checked authority, and it may not erase the records needed to determine why a worker stopped. A narrator may read enough to explain the outcome but may not alter state, accept work, authorize release or hide an action by performing it while describing it.

These prohibitions do not reproduce job boundaries. They preserve the conditions from which the roles were derived. Compatible functions can still combine. An actor may watch several unrelated workers or keep custody for several candidates, because those combinations do not let it approve its own construction or destroy the evidence it observes. Separation is specific to the hazard; it is not a demand for maximum distance everywhere.

## Why the custodian may accept a change

At the end of examination, the session record names an exact candidate. It also contains the human ruling on passive reading, the failure of the first attempt, the checks on refresh and upload behavior and the proof that rollback remains possible. Someone must perform the small but consequential act that moves this candidate from proposed to accepted. Leaving that act unowned would break the chain at its final link.

The custodian holds that link. It verifies that the candidate presented for acceptance is the one the independent examiner challenged, that every required result belongs to that candidate and that every enforced rule is satisfied. It then performs only the authorized acceptance action. It cannot waive a refusal, reinterpret a human ruling, repair a test or substitute a later version. If any required fact is missing, it leaves the candidate unaccepted.

That authority involves no creative judgment. The custodian makes no product judgment and no claim about the design beyond what the required record establishes. Its narrow acceptance power completes the chain of custody without allowing the custodian to construct or review the candidate.

What the custodian hands on is an accepted candidate; what runs after the releaser exposes it is a released version.

## Why the narrator has no power

At 4:30, the responsible authority does not need every event from the day. They need to know what changed, which ambiguity required their ruling, what failed, what passed, what was accepted and what happened after release. A narrator can turn the record into that report. The report has influence because it shapes what the responsible authority notices and may shape a later decision.

That influence is exactly why the narrator is given no power. Every material claim points back to the source record. The narrator cannot remove the refused candidate, soften a failed check, accept the work or release it. If the report states a material result, the responsible authority can reach the exact observation supporting it. The report becomes inspectable, while authority remains with actors whose powers are named.

The role is easy to undervalue because it decides nothing. The decisions this paper reserves for people (value rulings, irreversible acts, rule changes and accountability) are only as good as the person's understanding of the situation, and every other record in the system is written for machinery to act on. The narration is what keeps the responsible authority able to rule. Without it, human authority exists in name only.

## Configurations rather than job titles

A low-risk wording correction may need one builder, an automatic examination and a custodian that accepts the exact checked result. The same worker might later narrate an unrelated change. A session-expiry change needs more distance because it affects identity and every signed-in user. It can include a human who holds the intent, a builder, one or more independent examiners, a custodian, a releaser, a liveness watcher and a narrator. An adviser may retrieve relevant enforced rules and earlier rulings, but retrieval does not grant power to decide what they mean in the new case.

These are configurations; none of them is a job title. A human can serve as the independent examiner when consequence, shared machine assumptions or accountability requires human judgment. Several independent people or machines can examine the same candidate when the possible harm justifies the cost. On smaller work, compatible functions can be combined. The configuration changes with the hazards; the protections do not change just to keep every role occupied.

The configuration is also where cost meets risk. Every role staffed for a change costs computation and judging; every role left out either has its hazard controlled another way (a rule, a record or a narrower permission) or its risk accepted for that change. Choosing the configuration is a spending decision, priced by the same four questions that set verification depth, and it is remade when risk shifts rather than inherited from the last change. Chapter 11 develops the economics: the right configuration is the smallest one that protects the outcome at stake.

A role also does not have to be a program that runs all the time. The liveness watcher is one: silence can begin at any moment while workers act, so something must always be watching. The custodian is not: the records and the rules that guard acceptance hold at all times, but the custodian itself acts only at the moment of acceptance, when one actor briefly holds the narrow permission to accept. Both are real roles with real separations, whether their machinery runs all day or only for a moment.

## The anti-mimicry test

Suppose someone proposes a coordinator because every engineering team has one. The proposal is incomplete. It must name the hazard the coordinator controls, the permission needed to control it and the permissions that would compromise that protection. Perhaps the real hazard is two builders overwriting the same candidate. A rule that records state changes before replacement may control it without a coordinator. Perhaps the hazard is a value conflict between security and convenience. No scheduling role can resolve it; a responsible authority's ruling is required.

This is the anti-mimicry test. Every role must trace back through permission and separation to a real hazard. A liveness watcher survives the test because silent stopping is observable, because the watcher needs permission to detect and sometimes replace stopped work and because it must not erase the evidence of failure. A narrator survives because human decisions need readable reports, because it needs read access to source records and because changing state would let it conceal the very actions it explains.

A role fails when its only defense is familiarity. It also fails when its stated hazard is better controlled by an enforced rule, a durable record or a narrower permission. The test does not ask whether a role resembles a human job. It asks whether removing the role destroys a necessary condition and whether combining it with another function recreates the hazard. A separation stays only while evidence shows that it protects something real.
