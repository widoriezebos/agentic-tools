# 9. The Living System

Thesis: a system of mortal workers operating in a hostile world must
provide liveness, identity, isolation, and bounded authority.

## Silent death and deadlines

Recent work products and independently observed heartbeats will
distinguish progress from a worker that has silently stopped. Every
wait has a deadline and an owner, and a timed-out task leaves enough
state for safe retry or recovery.

## Identity must be checked at use

Names and recycled process numbers are not sufficient authority for a
consequential action. The system must verify both the subject and its
current claim to the resource at the moment of use, especially before
stopping a process, replacing data, or releasing a change.

## Isolation by construction

Checks run against isolated copies so that active work cannot alter
their result and a long check cannot freeze unrelated work. Each
worker receives only the resources and permissions its task requires,
making the possible harm of a mistake a design choice rather than a
hope. Chapter 8 applies the same least-authority rule to which parts of
the shared record each role may read while retaining the full history
for recovery and audit.

## Safe against itself

Well-meaning workers can overwrite data, stop the wrong process, or
try to bypass a failed check. Safety checks placed where actions occur
must refuse attempts to act on an unknown or overly broad target, and
uncertain ownership must lead to retention and escalation rather than
destruction.

## The hostile world

The threat model includes malicious input, hostile generated code,
compromised tools or dependencies, stolen secrets, and tampering with
records or test results. From it follow trust boundaries (points where
data crosses between differently trusted components), least authority
(only the permissions a task requires), limits on how far harm can
spread, provenance (a traceable source and history), and verification
independent of the component making the claim.

## The change, continued

The session-expiry worker receives access to an isolated test store but
not production secrets or permission to release. Dependency sources,
test results, and the identity of the accepted change remain traceable,
and a stopped migration cannot be mistaken for a completed one.
