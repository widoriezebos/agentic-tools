# 14. The Transition

Thesis: existing teams should transfer work and authority gradually,
using evidence from their current system rather than pretending to
start from a clean design.

## Begin with the existing system

Running behavior, production data, tests, policies, incident history,
and current ceremonies all contain clues about intent and protected
conditions. These clues produce hypotheses, not truth: users and
authorized stakeholders must confirm where observed behavior is a
requirement, an accident, or a known defect.

## Establish a measurable baseline

Before replacing a practice, record what it costs and what outcome it
appears to protect, including missed failures and recovery time. The
baseline makes later claims of improvement testable and exposes areas
where current protection is unknown rather than assumed.

## Coexist before replacement

New machinery first observes or operates beside the current process,
with separate records showing agreement, disagreement, cost, and
missed cases. Coexistence is temporary but useful: it reveals gaps in
inferred intent and permits comparison without granting unearned
authority.

## Transfer verification and authority progressively

Start with reversible, low-risk work; widen scope only after the new
checks can distinguish working from broken behavior, records support
recovery, and live outcomes meet the baseline. Authority transfers
separately from task execution, so machinery may propose or test a change
long before it may release, destroy, or waive anything.

## Retain, replace, or discard according to the surviving condition

The archaeology described in Chapter 4 first asks whether an invariant —
a condition that must remain true — still survives. If none survives,
discard the ceremony and record what was examined and why no replacement
is needed; do not drop it silently. Replacement evidence is required
only when a condition does remain and a new mechanism is proposed to
serve it: until that mechanism demonstrably protects the condition,
retain the ceremony, including any tacit knowledge, negotiation,
accountability, or independent human scrutiny it carries.

## Make rollback part of adoption

Every transferred responsibility needs a tested route back to the last
safe process, data state, and authority boundary. If comparison exposes
harm, unexplained divergence, or a missing appeal path, the system
narrows authority, restores the previous protection, and keeps the
evidence needed for another attempt.

## Finish by removing proven duplication

Coexistence that never ends becomes the double ceremony the transition
was meant to avoid. Retire the old practice explicitly either when the
record shows that no protected condition survives or when a replacement
has demonstrated equal or better protection across a declared period
and risk range. Retain the decision record and continue watching the
live outcome when a replacement has taken over.
