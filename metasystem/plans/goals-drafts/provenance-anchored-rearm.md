# Draft: provenance-anchored-rearm (Wido's question, 2026-08-31)

His words: "Why did we have to re-arm btw? This feels like something
that should be automatic?"

Today's behavior is the enrollment law: the steward is armed against
one engine binary's fingerprint, human-blessed; a changed binary is
refused until a human re-arms. Correct, because an agent that can
build a binary must not be able to swap the engine under the armed
supervisor and inherit its authority. The 2026-08-31 evening specimen:
the m0 reconciliation landing changed the engine fleet-wide, and every
machine needed a human word to resume dispatching.

The feature this draft proposes for the route: the steward re-arms
AUTOMATICALLY when the replacement engine is reproducibly built from a
commit that reached main through the landing gates carrying full
provenance — the human blesses the governed pipeline once instead of
each artifact. HARD PREREQUISITE, not negotiable: the
two-bars-for-changes goal must land first (landings consume provenance:
closed-chain candidate digest or declared direct-fix class). Until
then the landing gate binds no provenance and auto-re-arm would
launder seat-authored engine bytes into armed authority — the R-21
incident shape with engine power. Behind two-bars, the design also
needs: reproducible-build verification (same commit, same bytes), a
recorded auto-re-arm event naming the provenance consumed, and a
human-notified audit trail (the alert channel's digest class).

Status: draft awaiting the route — independent critique, then Wido's
word. Sequenced behind goal:two-bars-for-changes by construction.
