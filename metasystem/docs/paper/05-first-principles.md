# 5. First Principles

Thesis: a small set of laws, consistently enforced, replaces the
ceremony stack.

## Intent is the only sacred input
Everything else — designs, plans, even the laws themselves — is
negotiable machinery in service of intent. Intent arrives from humans,
in plain language, with constraints and freedoms attached.

## Proof over trust
No claim of completion is accepted on fluency. Work is bound to
mechanical evidence: tests that fail on broken implementations,
verification that distinguishes fixed from unfixed, gates that refuse
rather than advise. Trust is a property of proofs, not of workers.

## Records are the only memory
Anything not written to a durable, structured record does not exist.
Session memory, chat history, and good intentions all evaporate;
ledgers, journals, and evidence envelopes survive. Corollary: the
system must remain correct if every worker is replaced mid-flight with
a fresh one reading only the records.

## Laws refuse; guidelines advise; only laws count
A rule that depends on a worker choosing to follow it will be violated
at the worst moment. Invariants live in machinery that physically
refuses violations — at commit, at landing, at teardown — with clear,
plain-language refusal messages.

## Economy is law
Every process must pay for itself in delivered intent, and the system
itself asks the question. Budgets and appetites are enforced stop
conditions, not estimates.

## The human is sovereign at named points
Irreversible acts, value judgments, and law changes are reserved to
humans by construction — and everything else deliberately is not.

## Honest failure is a feature
The system prefers a loud, evidenced refusal over a silent success it
cannot prove. Every failure leaves an envelope: what ran, what it saw,
what it concluded.
