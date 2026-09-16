# Evidence that can only refute

A gate is only as good as what its evidence can see. When a rule is written
about something the recorder never witnesses, the rule can still be broken
visibly, but it can never be kept provably. Such a rule can refute and it
cannot confirm, and a system that reports it as satisfied is reporting a
state it did not verify.

## The shape

Three questions separate a refutable rule from a confirmable one.

1. What does the recorder actually witness? Not what it is near, or what it
   can infer from adjacency, but what it writes down itself.
2. Is the thing the rule judges one of those witnessed facts? If it is
   reached only by inference from a timestamp, an ordering or a name, the
   rule is about something the recorder does not see.
3. Can a party other than the subject produce the same evidence? If a
   process that never did the thing can leave the same trace, the trace is
   not evidence that the thing happened.

When the answers are no, the honest design says so and splits the rule:
what the records can refute, and what would have to be attested by whoever
does witness it.

## Where this was found

Goal coordinator-wakes-on-events-not-polls asks, in clause 5, for the time
from a wait's durable publication to the first resumed model turn that
consumes its result, with every sample under 60 seconds.

Two design revisions tried to establish which model request consumed the
result. The first argued from an adapter's static answer about blocking.
The second minted a token at the moment of return and treated its echo as
proof. A critique broke each in turn, and the third revision conceded the
general point with the tree in hand: the engine never sees a model request.
It sees its own subprocesses and its own files. Every carrier it can hand
the model passes through the harness, and any process that can read a row
or an append-only event file can produce the same echo. See
metasystem/plans/coordinator-wakes-clause5-amendment-r3.md, section 7.1.

So the measurement was split. From records alone the engine reports the
interval between the two edges it does witness, with its uncertainty, and
proves a failure whenever that interval is certainly at or above the
threshold. It reports no pass. A pass needs a per-request attestation from
the party that does witness the request: the harness or the provider,
declared through the adapter.

## Why the split is worth more than a weaker rule

A rule quietly redefined until the available evidence satisfies it stops
being falsifiable, and the gate that enforces it starts reporting comfort.
A rule split into "what we can refute" and "what must be attested" keeps
its edge: the refutation runs today on every runtime, and the attestation
is a named piece of work with a known cost, which a person can schedule or
decline with open eyes.
