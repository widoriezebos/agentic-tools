# 9. The Living System

**The protections around a worker cannot live inside it: a failed worker cannot report its own failure.**

The session-expiry change is under way when the migration worker goes quiet. It has not reported failure; its last message sounds plausible, and the work it promised should take only a few minutes. At the same time, another worker prepares to replace session data, while a third examines code assembled from sources outside the system. Treating all three as reliable because they began with good instructions would make silence, confusion and hostile material look like ordinary work.

A delivery system is living in a limited but important sense. Its workers start, act, wait, stop and are replaced while the application and its surroundings continue to change. Workers can disappear at any moment, and the world in which they act is not automatically friendly. Such a system needs to know whether work is alive, who is acting, where effects can spread and what authority exists at the instant of action. These protections belong to the environment around the workers, because a failed or compromised worker cannot be trusted to provide them for itself.

## Silent death and deadlines

At 10:03, the migration worker last records the set of existing sessions it intends to update. At 10:04, an independent signal confirms that the worker is still running. By 10:09, neither the intended update nor another signal appears. A status that still says "working" cannot distinguish slow progress from a stopped worker.

The system observes two things. It looks for recent work products that change the recorded state of the task, and it looks for a heartbeat, a small independently observed sign that the worker is still able to act. Neither is sufficient alone. A worker can remain alive while making no progress, and a finished work product can remain visible after its worker has died. Together they allow a liveness watcher to distinguish activity, waiting and silence more reliably.

Every wait also has a deadline and an owner. The deadline says when uncertainty must stop being treated as normal delay. The owner is the actor authorized to decide what follows: retry, replacement, reversal or human escalation. "Wait until it finishes" is not an operating rule unless someone can say when the wait ends and who acts then.

Timeout does not mean blind repetition. Before a task begins, the system records enough state to make a later attempt safe. The session migration identifies which version it is changing, what has already completed and which action remains. If the worker stops, the replacement can continue from the last complete state or reverse it. Silent death becomes a contained interruption rather than an ambiguous result.

## Identity must be checked at use

A liveness watcher decides to stop the silent worker. Between observation and action, that worker ends and its short numeric identifier is assigned to a new, unrelated task. If the watcher acts only on the remembered number, it stops the wrong work. The identifier was once accurate; it is not authority now.

Consequential actions check identity at the moment they occur. The system verifies both the actor or object being named and its current claim to the resource. Before stopping work, it confirms that the target is still the same worker and still belongs to the timed-out task. Before replacing session data, it confirms that the candidate is still authorized for that exact data. Before release, it confirms that the accepted change is still the change whose evidence passed.

This check closes a gap between decision and use. Names, labels and process numbers help locate a subject, but they can become stale or be reused. A current claim ties the subject to the particular task, resource and permission. If that relationship cannot be proved at the point of action, the action does not proceed.

## Isolation by construction

An independent examiner advances a controlled clock to the expiry boundary while the builder continues revising a different candidate. If both act on the same session data, the builder can change the very state the examination is measuring. The result may pass or fail for reasons that have nothing to do with the candidate. A slow examination could also hold a shared resource long enough to stop unrelated work.

The system prevents that interference by giving the examination an isolated copy. The clock, session data, candidate and observations belong to that examination until it ends. Active construction cannot alter its result, and the examination cannot freeze the builder or another examination. Isolation is a property of the work's surroundings rather than a request that workers avoid colliding.

Isolation also limits authority. The builder receives the application material and test data needed to construct the session change, but not live account secrets or permission to release. The independent examiner receives the finished candidate and an isolated place to challenge it, but not power to repair or accept it. The migration worker receives authority over the named migration and no broader access to stored data. A mistake can then harm only the resources intentionally placed within reach.

This is least authority: each task receives only the information, resources and actions needed for that task. Chapter 8 applies the same rule to the shared record, where each role reads a limited view while the full history remains preserved for recovery and audit. Isolation and narrow permissions turn the possible reach of an error into a design decision.

## Safe against itself

A well-meaning worker encounters a failed expiry check and decides that the check is obsolete. It attempts to remove the refusal and continue. Another worker sees two similar session stores and prepares to replace the broader one. Neither action requires malice. Confidence, stale context or an overly broad instruction is enough.

Protection is enforced where the action happens, not in the worker's judgment. The first worker cannot remove the refusal: changing an enforced rule requires the rule's authority, and a builder's confidence is not that authority. The second worker's replacement does not start on a loose description: it must name the exact store to be replaced, the authority that permits it and the limit of what it may touch. If the target is unknown, described too broadly or not owned by the task, the replacement is refused.

When it cannot be sure, the system preserves the current state. If the system cannot prove that an abandoned copy is safe to delete, it keeps the copy and raises the question to the actor that owns the decision. If it cannot distinguish the timed-out worker from a new one, it stops neither. This can consume space or delay progress, but the cost is bounded and visible. Destruction after an uncertain guess may not be recoverable.

The surrounding controls also protect records from the workers they describe. A builder cannot rewrite a failed result. A liveness watcher cannot erase the last state of the worker it replaces. A custodian cannot substitute an unchecked candidate during acceptance. The system assumes that ordinary actors can be wrong about their own work and makes that error survivable.

## The hostile world

During construction, the builder reads a package that promises to simplify session handling. Its instructions urge any tool using it to reveal a secret before continuing. Elsewhere, generated code opens a path that accepts a forged session marker. A compromised checking tool reports success without running the boundary case. An attacker who can alter the record tries to make the refused candidate appear accepted.

These possibilities define the hostile world. Harm can enter through user input, generated code, tools, outside dependencies, stolen secrets or tampered evidence. Good intent inside the delivery system does not make those sources trustworthy. The design begins by asking where information or authority crosses between parts with different reasons to trust them.

Such a crossing is a trust boundary, and defending it takes no new machinery: the roles, records, verification and access rules of the earlier chapters do the work, applied at the boundary. The package that urges tools to reveal a secret finds nothing to take: the builder works under least authority and holds no live secrets, text inside a dependency carries no permission of its own, and the dependency is recorded by exact version and checksum, so what was reviewed is what runs. Generated code, whatever produced it, is only a candidate: it reaches users through the same independent examination and custodian acceptance as any other change. The checking tool that claims success without running is caught twice: its results are recorded against the exact candidate, tool and conditions, and a check that cannot fail on the preserved known-bad candidate proves nothing. The attacker who edits history runs into the custody of records: workers cannot rewrite the records that describe them, the custodian accepts only a candidate whose chain of intent, examination and passed rules is complete, and the preserved history shows the alteration.

Least authority limits what a compromised part can reach, and isolation limits how far the harm can spread. None of this proves that the world is safe. It replaces blanket trust with named boundaries that can refuse, contain and expose hostile action.
