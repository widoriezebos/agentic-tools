# The journey

This is the story of how the metasystem got built — what happened, why
it happened, and who did it. It is written for reading, not for
reference: the mechanisms have their own docs, and this one only cares
about how they came to exist. It grows at the end as the work goes on;
each goal that concludes earns its paragraph here. The narrator goal
owns this file.

## The seed (July 13, 2026)

The repository opens with two commits and one conviction: Wido did not
want to build systems by hand anymore. He wanted to build the system
that builds them — a metasystem: rules, roles, and machinery that let
AI agents carry real engineering work from intent to landed, verified
code, with a human steering by decisions instead of keystrokes.

The first weeks were paper more than machinery. A collaboration layer
that said how agents report to a human. Working modes distilled from
earlier projects. A refactor discipline. The first skills — including
an adversarial design critique with a stop rule, which would go on to
shape nearly everything that followed, because from the very start the
rule here was that designs get attacked by an independent model before
they get built.

## One machine, many hands (late July)

As soon as more than one agent could touch the checkout, the real
problems arrived. Who may write? Who watches the processes? What
happens when a session dies mid-write? The answers became the oldest
load-bearing pieces of the system: the checkout lease — one writer at
a time, held by provable process identity, not by promises — and the
supervision layer around it. The first big critique round tore into
that supervision design and left nineteen accepted findings; it was
redesigned rather than patched. This set the second lasting habit:
findings do not get argued with, they get either refuted with evidence
or folded in.

## Runs must explain themselves (early August)

On August 3rd a validation run hung for 112 minutes with zero
progress, and nobody — human or agent — could say from the outside
what it was doing. Two things came out of that day. Every fixture wait
got a named ceiling, so a hang names itself instead of consuming a
night. And the flight recorder was born: runs write down what they do
as they do it, because a system operated by agents cannot depend on
anyone remembering to look.

Missions followed — the machinery for handing a goal to a host agent
that orchestrates delegate agents across different runtimes (Claude,
Codex, Devin), unsupervised, with budgets, stop-losses, and a wall
between orchestrating and implementing. The benchmarks that exercised
those missions found twelve real defects, and each one became a
tracked issue with its own fix, its own fixtures, and its own landing.

## The Go port (mid-August)

The decision helpers had grown into a pile of Python and bash that was
getting harder to trust. The ruling that reshaped the codebase:
decisions live in Go, plumbing stays in scripts. Over roughly a week
the lease, the census, dispatch, adoption, the mission runner, and the
rest were ported — not transliterated but redesigned to clean Go,
fixing known defects on the way. The Python was deleted. A sprawl of
29 script families collapsed into a handful of domain families behind
one binary, and a production-grade pass gave the whole thing the
error-handling and test coverage a foundation deserves.

## Learning when to stop (mid-August)

The patience program asked a question that sounds soft and is not:
when should an agent keep waiting, and when is waiting a stall? Four
design satellites went through critique loops — one of them to round
22 before reaching zero material findings — and produced the working
vocabulary the fleet uses now: patience, progress, stall. Slower
progress is still progress; silence is not.

The question turned personal on August 19th, when the coordinator —
the agent writing this — stalled silently for ten hours on a one-off
migration task, polishing a corner nobody had asked about. Wido's
verdict was blunt and correct: be practical, stop wasting tokens, and
why did you not ask? The retrospective found the real answer: every
guardrail in the system pointed at *continue* and no role owned the
question *is this still worth it*. That vacancy produced two
mechanisms. The steward — an always-running watchdog so that open
work is never silently idle — and, later that week, the appetite law.

## The backlog becomes a fleet (August 22)

The last week's work turned a pile of plan files into a real
multi-machine backlog. The goal ledger — the thread of intent that
survives every turn — was converted into a synced tree that publishes
directly to the shared remote: any machine can claim, work, and land,
and every machine sees the same truth. The conversion itself was
rehearsed on a clone before it touched the real thing, because the
one instruction that mattered was *do not damage the backlog*.

Around the ledger grew the working laws, each agreed with Wido, each
enforced by the machinery rather than by memory. The appetite law:
every item carries a worth-sizing agreed before work starts, checked
as effort accumulates, and a blown appetite stops the work and raises
the human. The slicing law: large work is never embarked on in one
piece — it is split into iterative, independently deployable slices.
The two compose: appetite says what a feature is worth, slicing says
how anything big gets delivered. Intake got a draft state so the
backlog itself stays clean, and the coordinator owns its order.

The same day, a second machine joined. Its first act was to orient,
read the laws, propose an appetite, and have it ratified — the
mechanism working on its first customer. The two machines have been
landing interleaved work on the shared ledger since, absorbed by
ordinary rebases.

And in a fitting close to the week, the day reviewed itself: an
ease-of-use review of the agent-facing tooling found its sharpest
defects in the tools built that same morning — a help flag that
launched a forty-minute test suite, a review template no skill linked
yet. The newest code is where discipline slips; the system now knows
to review at birth.

## The narrator wakes up (August 22)

This document is the narrator's first act. The goal behind it carries
a larger charter — continuous, real-time narration of what the system
is doing, wired into the steward, empowered to name anomalies and to
reach the human when something is out of the ordinary. That charter
is deliberately not built yet; it exceeds a day's appetite and will be
sliced like everything else. What could be delivered today is this:
the story so far, and the covenant that it keeps growing — every goal
that concludes from now on adds its paragraph before it is called
done.

---

*Chapters below are appended as goals conclude.*
