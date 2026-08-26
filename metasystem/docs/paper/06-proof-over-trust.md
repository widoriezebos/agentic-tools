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

## Adversarial convergence

Fresh critics will receive enough context to challenge the work but
not the builder’s chain of reasoning, and they will be asked to find
fault rather than approve. Repeated rounds stop when a bounded search
finds no new material issue, a budget forces escalation, or a human
must decide an open question; stopping is explicit rather than a
product of fatigue.

## One risk model for verification

Verification depth is set by consequence if wrong, novelty of the
approach, exposure to users or systems, and accumulated change since
the last broad check. The same four factors govern spending in Chapter
11, and the price of parallel attempts includes comparing and judging
them rather than generation alone. A one-line authorization change can
therefore require human review and deep testing while a large batch of
low-impact text changes can use cheap automated checks.

## What proof cannot prove

Proof cannot establish more than its boundary, expected-result source,
and assumptions allow: green tests can encode the same mistaken rule
as the implementation, and closed test cases cannot cover an open
world. Intent alignment, unknown effects, and whether a result is
acceptable to affected people therefore remain questions for live
evidence and judgment.

## Correlated blind spots and independent human review

Machine evidence can suffice for reversible, well-understood changes
with strong discriminating tests and low scores on the shared risk
factors. Independent human review is mandatory for value judgments,
irreversible or high-consequence actions, novel weakly tested work, and
cases where builders, critics, or test generators may share a model,
data source, or assumption.

## When escalation was misclassified

Every release must preserve a path for later human challenge, and
production signals must be able to stop or reverse it. A missed
escalation is treated as both an incident and a defect in the
classification rule: repair the harm first, then test and revise the
rule without assuming every lesson can become an automatic refusal.

## The change, continued

For session expiry, a discriminating test first demonstrates that an
expired session still works before the change and fails afterward,
while boundary tests cover clock skew and existing sessions. Because a
one-line comparison in authentication has broad exposure and severe
consequences if reversed, the risk model calls for independent human
review even though the diff is small.
