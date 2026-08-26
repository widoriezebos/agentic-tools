# 11. The Economy of Machine Engineering

Thesis: construction, verification, and judgment all have costs, so
the delivery system must spend in proportion to expected value and
risk.

## Value before process

Every recurring process must identify the outcome it protects and the
evidence that the protection is worth its cost. A process that cannot
do so is reduced, redesigned, or stopped with a recorded reason rather
than continued by tradition.

## The shared risk model

The same model introduced in Chapter 6 sets both verification depth and
spending: consequence if wrong, novelty, exposure, and accumulated
change since the last broad check. These factors are considered
together rather than collapsed into lines changed, elapsed time, or a
single opaque score.

## Budgets are enforced stop rules

Work begins with explicit limits on time, attempts, and spend, plus a
named decision-maker for exceptions. Reaching a limit stops or narrows
the work and raises a decision; it does not silently consume more
because previous spending created attachment.

## Parallel attempts include the cost of judging

Several independent attempts can expose disagreement and improve a
decision, but each additional attempt consumes comparison, testing,
and human or machine judgment. Parallel work is worthwhile only when
its expected information or solution value exceeds both generation and
selection costs.

## Small changes can be risky; large changes can be routine

A one-line change to an authorization condition can have high
consequence and exposure and therefore justify deep tests and mandatory
human review. A bulk correction to low-impact text can touch thousands
of lines yet remain suitable for cheap automated checks; verification
follows risk, not size.

## When not to build the system

The break-even test compares the cost of building, operating, and
verifying delivery machinery with its expected reuse, avoided harm,
and learning value. One-off prototypes, disposable explorations,
small low-risk tools, and short-lived software can rationally use much
less machinery, especially when manual judgment is cheaper and clearer.

## The smallest-machinery rule

Choose the smallest delivery mechanism that protects the relevant
outcome at the expected level of reuse and risk, then add structure only
when evidence justifies it. The thesis is therefore a direction for
repeated or consequential engineering, not a command to build a
platform around every script.

## Measuring the system, not activity

Useful measures connect total spend and verification delay to delivered
intent, escaped harm, recovery time, and retained learning. Counts of
attempts, messages, or generated code describe activity but do not show
that the delivery system has earned its keep.

## The change, continued

Session expiry is a small code change with high exposure, so its budget
reserves more for verification and rollout observation than for
generation. Multiple proposed implementations are requested only if
their expected design insight is worth the cost of comparing them; the
one-line-versus-bulk contrast keeps that decision tied to risk.
