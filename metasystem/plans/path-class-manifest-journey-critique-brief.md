Working Mode: implement
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal path-class-manifest)
Date: 2026-09-04

# Review brief: the manifest's journey chapter (chain pcm-journey)

FINDING IDS: use chain-unique ids PCMJ-01, PCMJ-02, ... never F-n.

Round budget: 1 focused round. R-60-m1's rule: material only if it
changes what gets built and names the artifact.

Threat model: the chapter breaking the journey's standard
(metasystem/docs/backlog-mechanism.md, section "Concluding a goal"):
an acronym, identifier, decision number or commit hash in the prose;
a borrowed word not explained at first use; a sentence that would not
survive being read aloud to someone who was not there; a fact that
contradicts the record (metasystem/plans/goals/path-class-manifest.md
and metasystem/plans/path-class-manifest-journey-brief.md); any change
outside the appended chapter. Out: taste beyond the standard.

Scope: the computed diff of the implementer job under review (one
file, metasystem/docs/journey.md, one appended chapter).

If nothing material remains, say so; that closes the chain and the
chapter lands with the goal's conclusion.

# Constraints

Wall-clock budget: 15 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
pcm-journey.

# Gap Rule

stop and report a gap; never fill it silently.

# One thing to check first

The chapter explains a "carriage landing" as moving work between
machines. Check that against the record: under the manifest, register
carriage is the direct landing of RECORDS by the seat that owns the
stream (no chain), and the handover between machines was a pushed
branch. If the chapter's gloss is wrong, that is material: it names
the artifact (the sentence) and the fix (a correct plain-English gloss).
