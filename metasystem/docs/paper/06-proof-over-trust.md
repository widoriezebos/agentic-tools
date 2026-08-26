# 6. Proof over Trust

Thesis: bounded proof and well-grounded evidence, not fluent claims,
organize machine engineering.

## Evidence, proof, and the boundary between them

Evidence is a traceable observation such as a test result, production
measure, or independent review; proof is the narrower claim that a
conclusion follows within stated boundaries and assumptions. This
distinction will prevent a passing check from being presented as
mathematical or universal certainty.

## Gates, not guidelines

A gate is a check placed at the action it controls, with power to refuse
that exact change and explain the refusal in plain language. The
chapter will show why advice drifts when workers may ignore it, while a
gate remains effective only if it binds the bytes and environment that
will actually be released.

## Discriminating tests

A useful test must fail on a relevant broken version, not merely pass
on the proposed version. The section will cover known-bad fixtures,
mutation or fault injection, trustworthy sources for expected results,
and the need to preserve the test’s own provenance — its traceable
source and change history.

## Independent critics look for faults

Each new critic receives the authorized intent, relevant constraints,
the builder’s finished work, and the resulting evidence, but not the
builder’s reasoning trace or path to the work. Judging the work rather
than the worker’s process preserves an independent view; Chapter 8
explains how access to one durable record is limited by role to protect
that fresh context. Repeated rounds stop when a bounded search finds no
new material issue, a budget forces escalation, or a human must decide
an open question. Stopping is explicit rather than a product of fatigue.

## Four questions set verification depth

Verification depth is set by how severe the harm would be if the work
were wrong, how unfamiliar the approach is, how many users or systems it
can affect, and how much change has accumulated since the last broad
check. The same four questions govern spending in Chapter 11, and the
price of parallel attempts includes comparing and judging them rather
than producing alternatives alone. A one-line authorization change can
therefore require human review and deep testing while a large batch of
low-impact text changes can use cheap automated checks.

## What proof cannot prove

Proof cannot establish more than its boundary, expected-result source,
and assumptions allow: green tests can encode the same mistaken rule
as the implementation, and a fixed set of test cases cannot cover every
situation the software will meet. Intent alignment, unknown effects, and
whether a result is acceptable to affected people therefore remain
questions for live evidence and judgment.

## Evidence that triggers human review

Machine evidence can suffice for reversible, well-understood changes
with strong tests that distinguish working from broken behavior and low
risk under the four questions above. It must trigger human review for
value judgments, irreversible or high-consequence actions, unfamiliar
weakly tested work, and cases where builders, critics, or test generators
may share a model, data source, or assumption. These are the evidence
conditions that require review; Chapter 13 owns reviewer authority,
accountability, and appeal.

## Repair a mistaken classification

Evidence found after release may show that work was wrongly classified
as not requiring human review. Treat that missed trigger as both an
incident and a defect in the classification rule: repair the harm first,
then test and revise the rule without assuming every lesson can become
an automatic refusal.

## The change, continued

For session expiry, a discriminating test first demonstrates that an
expired session still works before the change and fails afterward,
while boundary tests cover small differences between clocks and existing
sessions. Because a one-line comparison in authentication can affect
every signed-in user and cause severe harm if reversed, the four risk
questions call for independent human review even though only one line
changed.
