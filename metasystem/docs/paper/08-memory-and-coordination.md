# 8. Memory and Coordination

Thesis: durable records replace private memory and make coordination
recoverable, but they do not replace human deliberation.

## A durable record at the center of work

Goals, claims, states, decisions, and results live in a structured,
history-preserving record with controlled state changes. Every worker
and authorized human can read the same current state and trace how it
was reached without reconstructing it from meetings or chat history.

## Handoff by record

Every unit of work must be resumable from its recorded intent, current
state, evidence, open questions, and next authorized action. The test
is concrete: stop a worker at any moment and require a fresh one to
continue safely without asking what happened.

## Coordination is not agreement

Atomic state changes can prevent two workers from overwriting one
another and can record which proposal superseded another. They cannot
decide whether security should outweigh convenience or which
stakeholder should prevail; Chapter 13 assigns those value conflicts
to authorized human governance.

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
implementation, the checks run, and the accepted version. If a worker
stops halfway through migrating existing sessions, another can see the
last safe state and continue or reverse it.
