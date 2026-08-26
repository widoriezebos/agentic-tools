# 1. The Shift

Thesis: for software delivered repeatedly or carrying material risk,
engineering ownership is moving from the deliverable to the delivery
system.

## The ladder we have been climbing

This section will trace how compilers, version control, continuous
integration, infrastructure as code, and deployment pipelines moved
repeated human work into machinery engineers built once and improved.
It will argue that agentic engineering is the next rung because the
activity being absorbed is construction itself.

## What “building the system that builds the application” means

The application becomes an output that machinery can produce,
reproduce, check, release, and repair. Human attention moves to intent,
to laws — machine-enforced rules that can refuse a change — and to the
evidence needed for consequential decisions; the legal metaphor names
authority, not a copy of a human court.

## A concrete day

A human states the outcome and constraints; machinery proposes,
criticizes, builds, checks, and releases a change; the human is
interrupted only for decisions reserved to human authority. This
example will make the ownership shift visible before later chapters
separate each part of the system.

## Why now

Agents can carry a construction loop but fail through silent stopping,
drift, and plausible wrongness. This section will argue that the useful
response is structural: make those failures observable and survivable
without requiring a human to supervise every step.

## What this paper is and is not

This is a standalone paper about vision, concepts, and principles, not
a description of a particular tool, framework, vendor, or
implementation. Its claims must remain understandable without private
experience and testable across different applications and settings.
