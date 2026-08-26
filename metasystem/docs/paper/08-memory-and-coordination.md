# 8. Memory and Coordination

Thesis: durable records replace private memory and make coordination
recoverable, but they do not replace human deliberation.

## One authoritative record, with access limited by role

Goals, claims, states, decisions, and results live in one structured,
history-preserving record: the only source allowed to determine the
current state of work. The record is complete enough for recovery and
audit, but that does not mean everyone may read all of it; each role
sees only what its task requires. This is how Chapter 9 applies least
authority to information as well as actions. But access rules restrict
what a mind may read next; they cannot erase what it has already
absorbed. Least authority and fresh perspective are therefore two
different guarantees: permissions provide the first, and only a
different mind provides the second. When Chapter 6 demands a critic
with fresh context, the system must supply a distinct person or a
freshly instantiated worker that has never seen the builder's path —
which machine workers make cheap to provide and humans, whose exposure
cannot be revoked, largely cannot.

## What each role may read

Builders may read the authorized intent and constraints, relevant work
materials and prior rulings, evidence needed to construct and check the
change, and their own recorded path so another builder can recover.
Critics may read the authorized intent and acceptance criteria, relevant
system context, the builder’s finished work or output, and the resulting
evidence; they may not read the builder’s reasoning trace or path to the
work. Custodians may read the exact finished output, required test and
critique results, human rulings, and state history needed to accept or
reverse it. Auditors — people or machines charged with reconstructing
past work — may read the full retained history, including reasoning,
role and access decisions, evidence, and state changes, but that
read-only view grants no authority to alter or accept work.

## Handoff by record

Every unit of work must be resumable from its recorded intent, current
state, evidence, open questions, and next authorized action. The test is
concrete: stop a builder, critic, custodian, or auditor at any moment and
require a replacement in the same role to continue safely from the
records that role may read. Information hidden from one role is retained
for recovery within the originating role and for later audit rather than
erased.

## Coordination is not agreement

Recording each state change completely before another begins can prevent
two workers from overwriting one another and can show which proposal
replaced another. It cannot decide whether security should outweigh
convenience or which stakeholder should prevail; Chapter 13 assigns
those value conflicts to authorized human governance.

## Two audiences, one source

Machines need structured fields, while humans need a readable account
of what happened and why. Both views must be generated from the same
source records so that every narrative claim can be checked rather
than becoming a second, competing truth.

## Decisions, precedents, and institutional memory

A precedent is a stored prior ruling and its reasons, retrieved to
inform a later decision; it is not automatically binding unless an
authorized law says so. Records preserve consistency and make changes
of mind visible, but the legal metaphor stops short of treating a
database lookup as judgment.

## The change, continued

The session-expiry record links the original request, the decision to
expire from last activity, the affected interfaces, each attempted
implementation, the checks run, and the accepted version. A critic sees
the finished change and its evidence but not the builder’s path; a
custodian sees what it needs to bind the accepted version to its checks,
while an auditor can reconstruct the whole history. If a worker stops
halfway through migrating existing sessions, a replacement builder can
see the last safe state and continue or reverse it.
