# 6. Proof over Trust

Thesis: verification is the organizing center of machine engineering.

## Gates, not guidelines
The anatomy of a refusing gate: where it sits (the door, not the
manual), what it binds (the exact bytes being landed, not a
description of them), and how it speaks (a named, plain-language
refusal). Why advisory checks decay and refusing checks do not.

## Discriminating tests
A test that passes on the broken implementation proves nothing. The
standard: for every guard, ask "would this fail on the code we were
just about to ship without it?" — and keep the demonstrated answer.
Fixtures are the arbiter of disputes between designs and opinions.

## Adversarial convergence
Independent critics with fresh context, briefed to refute rather than
approve; builders and critics as separate workers; rounds continuing
until findings stop changing what gets built — a defined stop
criterion rather than fatigue. Why fresh context matters: a reviewer
who watched the work being made shares its blind spots; independence
is an epistemic resource that agents can supply cheaply and humans
never could.

## The convergence ladder and its honest end
Successive critiques find successively deeper issues; each round
narrows scope. Two disciplined exits: closure (a finding-free round on
a genuinely closed state space) and the raised asymptote (findings
continue but stop being reachable in the medium — a human ruling
takes it from there). Both are legitimate; drift between them is not.

## Layered proof economics
Cheap constant checks (canaries) always on; medium proofs per change;
expensive global proofs on accumulated weight. Verification spend is
budgeted like any other resource — the point is calibrated confidence
per unit cost, not maximal ritual.
