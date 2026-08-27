# 4. The Mimicry Trap

Imagine that at nine each morning, six machine workers receive a prompt called "standup." Each produces three tidy sentences about yesterday, today, and blockers. The reports arrive on time. One worker has already stopped without recording why, two are repeating the same failed approach, and none has challenged the assumption that a background refresh proves a user is active. The ceremony is present, but the protection is not.

This is the mimicry trap. A familiar process is copied into a medium with different limits, so the result looks controlled while the condition that once justified the process remains unexamined. Avoiding the trap does not require rejecting history. It requires reading each inherited practice as evidence about the problem it was built to solve.

## What each ceremony protected

The reasons that follow are recurring observations from human software work, not a claim that every team uses each practice. A developer works for a day on a difficult change while the rest of the team cannot see whether the work is moving or stuck. A short daily meeting forces private state into the open. The standup's durable contribution is visible progress, visible blockage, and a chance to coordinate before hidden delay becomes expensive. The circle of people speaking in turn is only the form it took.

A team with ten people and more requests than attention chooses a small batch for the next two weeks. The fixed interval gives people a stable horizon, limits interruption, and creates a regular moment to reconsider priority. The sprint protects scarce human attention from constant reshuffling. Its cadence is only a means; what is protected is the team's attention.

A manager deciding between two projects asks how long each will take. The answers are imperfect, but they make commitments and contention discussable before expensive labor is assigned. Estimates help allocate scarce people and expose disagreement about scope. What they protect is a decision made with a visible account of expected cost and uncertainty. The number itself is secondary.

A developer finishes a change that looks correct to its author. Another developer reads it, finds an unchecked boundary, and learns how that part of the application works. Code review adds a second perspective while spreading knowledge that would otherwise remain with one person. Its protected conditions are scrutiny independent enough to find faults and continuity beyond the original author.

When the person who understands a service leaves, the next person reconstructs its assumptions from incidents and old messages. Documentation reduces that loss by putting decisions, behavior, and operating knowledge somewhere more durable than memory. What it protects is recoverable context; the document shape and the writing moment are incidental.

After the same release failure happens twice, a team stops normal work and asks why its process has not learned. The retrospective creates protected time to compare expectation with outcome and change future behavior. Its contribution is a learning loop with authority to alter the system; the meeting invitation is not the point.

The method is the same each time: take a concrete practice, recover the human limit that produced it, and name the condition that still needs protection. The history is observed; the proposed approach is to preserve that protected condition only where the new workforce still threatens it.

## Why mimicry is attractive

An auditor asks how a release was reviewed. "It passed code review" fits an established form. A manager asks when work will arrive. "It is in the next sprint" fits a familiar calendar. Both answers are recognizable, and recognizability has real value. It helps people know where to ask, gives authorities a place to intervene, and makes accountability easier to describe.

Familiarity also leaves people in a reassuring position: a machine proposes each step and a person approves it. That arrangement can be appropriate when the person is deciding a reserved question or the consequences require independent human judgment. It is less convincing when the approval consists of glancing at a polished report of evidence the person cannot reproduce. The visible human remains, but the claimed scrutiny may not.

A ceremony cannot justify itself by being recognizable. The relevant question is whether its protected condition holds. A named review with no independent challenge is weaker than an unfamiliar examination that reliably finds a known fault. A two-week boundary with no scarcity to manage is only delay. Accountability requires traceable authority and consequences; keeping every familiar form alive does not supply them.

## The cost of preserving the form

Suppose every machine worker must write a morning report, attend a planning exchange, estimate its attempt, and produce a closing summary. Most of the information already exists in the work record. Rewriting it consumes computation and creates several versions of state that can disagree. A responsible authority must either read them all or accept that some reports exist only because the process asks for them.

The greater cost is misplaced confidence. A polished review summary can look like scrutiny even when its author received the builder's reasoning and repeated the same assumption. An estimate can make uncertain exploration look controlled without limiting spend. A retrospective can collect fluent lessons that never change a check, permission, or decision. The copied form becomes an empty performance when no observation can distinguish performing the ceremony from protecting its condition.

Attention then follows the old problems. Managers ask whether each worker reported on time while a stopped attempt has no deadline or terminal record. Reviewers count approvals while every independent examiner shares the same source of expected results. Planners optimize a cadence while weak verification allows a plausible mistake to approach release. Mimicry is expensive not just because it adds steps, but because those steps can conceal the failure modes of the new workforce.

## Find the protected condition, then choose

Consider an inherited approval meeting. First ask what event made it necessary. Perhaps information was scattered, two groups held conflicting goals, or a consequential action needed a named authority. Next ask whether that condition remains. A common record may solve scattered information; it cannot settle a conflict of values or accept accountability. Then ask what observable evidence would show that a replacement works. If work is meant to be visible, can a reader see current state, last progress, deadline, and failure without asking the worker? If scrutiny is meant to find faults, does the replacement reject a known-bad candidate?

Only after those questions is a choice made about the form. When no protected condition remains, discard it and record why. When the condition is mechanical and observable, machinery can replace the form. When the condition remains but the old form fits poorly, adapt it. When the act contains a value ruling or accountability that machinery cannot hold, keep it as an explicitly human act.

The evidence standard applies to removal as well as replacement. A team should not delete a practice merely because it feels old. It should identify the condition, show that the condition no longer exists or is protected elsewhere, and observe the result. Nor should it demand a replacement for a condition that is really gone. That would preserve process for its own sake.

## Four worked cases

A product group once used two-week sprints because people needed a stable batch of work and switching direction midweek was costly. In a proposed agentic delivery loop, each bounded attempt can start when intent and capacity are available, while budgets limit work and responsible authorities can change priority without interrupting a tired person. If no release, regulatory, or coordination boundary depends on the two-week interval, the analysis finds no surviving condition. The cadence is discarded. The record states what it used to protect, why that constraint is absent for these attempts, and what observation would reopen the decision. No imitation sprint and no replacement clock is added.

A daily status meeting has a different result. Its protected condition remains: people and machinery need to know whether work is moving, blocked, complete, or dead. The meeting is replaced by a machine-produced record updated by events in the work itself. It shows the authorized intent, current attempt, last meaningful progress, deadline, blocking condition, and terminal result. A responsible authority can inspect it when a decision is needed instead of collecting a new recital each morning. The replacement is accepted only if a stopped attempt becomes visible promptly and another authorized actor can continue from the record.

Code review survives in adapted form. The builder finishes a candidate and supplies the exact result and its evidence. An independent examiner receives the authorized intent, relevant constraints, and finished work, but not the builder's path of reasoning. The independent examiner tries to find faults and ties each conclusion to a result that others can reproduce. A known-bad version tests whether the examination can discriminate. This adaptation preserves scrutiny and continuity without requiring line-by-line approval as a social ritual. Machinery may gather information and perform much of the scrutiny; independence and evidence, rather than the old meeting or approval shape, carry the protection.

The fourth case ends differently. A shorter session limit protects unattended accounts but may exclude people who use assistive technology and respond more slowly to a warning. Machinery can measure response times, model alternatives, identify the affected tasks, and test whether each proposed rule behaves as described. It cannot decide what burden is fair, which safety tradeoff is acceptable, or who accepts responsibility for the outcome. Those value rulings and that accountability always end with responsible authorities, without exception. No report, however polished, makes the machinery the responsible authority. Chapters 5 and 13 make that boundary explicit.

## What cannot first be extracted

Three people enter a review believing they agree that a session warning should be "accessible." One means that a screen reader announces it. Another means that every user receives enough time to act. The third means that the security limit must not be extended at all. The meeting does not merely execute an existing rule. It reveals that the shared word concealed incompatible intentions.

Some ceremonies hold unspoken knowledge and create the encounter in which disagreement becomes visible. Their protection cannot be fully extracted in advance because discovery is part of the act. The method must therefore name what remains unknown and preserve a human place for negotiation. Machinery can prepare examples, expose conflicting statements, record rulings, and later enforce what can be made checkable. It cannot automate the disagreement away by turning the surrounding paperwork into a form.

The proposed end state is work in which every retained form has a stated reason, every mechanical replacement is answerable to observable evidence, and every human act is reserved because human authority or discovery is actually required. That is a stricter use of process than mimicry, and it does not abolish process.
