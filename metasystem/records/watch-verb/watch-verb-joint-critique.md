# Watch-verb joint round — Fable critique, with seat dispositions

Critic: claude-fable-5 (job watch-verb-joint-critique) against the
certified joint diff (reviewedTree 91e1537d, design revision 3 sha256
c80843fe). 7 findings: 2 MATERIAL (WVJC-01 high, WVJC-02 medium), 5
non-material lows. The chain's budget exhausted at this verdict
(attempts 10/10, minutes 1200/1200): per the critique stop rule the
chain REMAINS OPEN awaiting the human's word; the successor round, if
granted, binds on exactly WVJC-01 and WVJC-02. Trajectory: 13 -> 9 -> 2.

## WVJC-01 [high|material=True]

The read surface reports HEALTHY with exit 0 over a known persisted goal-bound failure whenever the steward's health record is absent or stale, and because the surface computes no freshness anywhere, a wholly dead supervision stack also renders as healthy. The design codifies this ('A goal-bound failed job alone does not get re-judged; its owning persisted delivery verdict determines attention') and a test locks it in, so the guarantee is narrowed to what the code happens to do: the watch verb can only surface failures the steward already noticed, which contradicts the design's own rule that a known failure must not disappear behind another source and defeats the silent-death visibility the goal requires.

Evidence: internal/watch/watch.go itemNeedsAttention (jobs case fires only on an explicit-null goal field, roughly line 218 of the new file) plus readDelivery/healthSection returning EMPTY when health.json is absent (roughly lines 377-400); design section 3.2 aggregate rules in the diff (diff.patch lines 1690-1695); internal/watch/watch_test.go 'bound goal' case asserting AggregateHealthy for a failed goal-bound job (diff.patch line 985); the human projection in cmd/metasystem/watch_verb.go printWatchSnapshot never prints observedAt, so a stale health verdict is indistinguishable from a fresh one in the default output. Remedy sketch: without re-running the racy delivery join, treat a terminal goal-bound failed/timeout record whose owning delivery verdict is absent (or older than the record's end) as UNKNOWN, and print observedAt in the human projection.

DISPOSITION: ACCEPTED, binds the successor round.

## WVJC-02 [medium|material=True]

The completed-but-unconsumed round class — a delegate round that finished and whose return nobody consumed, the seat's own recorded incident shape — is silently absent from the design: it is not in the closed tracked enumeration, not in any deferred-slice section, and not recorded as an explicit narrowing, even though revision 3 claims every narrowing is explicit. Such a round renders as an ordinary healthy 'completed' item. The substrate genuinely lacks a durable consumption marker, so the honest move the design applied elsewhere (name the class, name the absent substrate, defer explicitly, as it did for W-HEAL and the scan-jobs verdicts) was available and skipped.

Evidence: Design section 3.2's closed seven-class table (diff.patch lines 1665-1673) has no consumption dimension and sections 4-9 never mention the class; grep of internal/dispatch for consumedAt/returnConsumed shows the only consumption markers belong to launch capabilities (launch_capability.go:131,267), none to round returns; internal/watch/watch.go readJobs emits completed records as plain READABLE items with no attention or unknown contribution. Remedy sketch: add one design paragraph naming the class, stating the missing durable consumption marker as future substrate, and recording the explicit deferral.

DISPOSITION: ACCEPTED, binds the successor round.

## WVJC-03 [medium|material=False]

internal/watch/watch.go knownJobStatus re-declares the complete job status vocabulary that internal/dispatch/record.go explicitly reserves ('This predicate is the vocabulary's one exported home; consumers outside dispatch must not re-declare the set', record.go:49-52). If dispatch ever adds a status, watch will misclassify valid records as UNREADABLE and degrade the section — fail-safe in direction (exit 2, not silent-healthy) but a drift hazard against a stated in-tree contract.

Evidence: internal/dispatch/record.go:45-52 (terminalStatuses plus the re-declaration prohibition) versus knownJobStatus in the new internal/watch/watch.go (diff.patch lines 607-614). Remedy sketch: use dispatch.TerminalStatus for the terminal half and export the non-terminal set, or move the closed vocabulary to its declared home.

DISPOSITION: ACCEPTED as non-material; rides the goal to later slices.

## WVJC-04 [low|material=False]

The zero-write proof is weaker than the design's phrase 'the slice's executable zero-write claim' implies: the before/after tree hash covers path set, mode, symlink target, and file bytes, so it cannot detect a transient create-then-delete (a lock file taken and released), a byte-identical rewrite, or any write outside the fixture root. By inspection the code has none of these (plain Stat/ReadDir/ReadFile only, and the lock-creating steward.AlertEpisodes helper is correctly avoided), so no defect ships — the proof claim is just overstated.

Evidence: watchTreeHash in cmd/metasystem/watch_verb_test.go (diff.patch lines 277-316) hashes only persistent state under root; design section 3.3 (diff.patch lines 1697-1708). Remedy sketch: soften the design's proof claim or add an strace/opensnoop-style check in a later slice.

DISPOSITION: ACCEPTED as non-material; rides the goal to later slices.

## WVJC-05 [low|material=False]

'metasystem watch --job <id>' is not zero-write: it delegates to the landed job waiter, which durably registers waiter state before polling. The design discloses the delegation and scopes the zero-write claim to the no-argument surface, but section 2's one-command phrasing ('metasystem watch ... writes no reader state') can mislead an operator into believing the whole verb is inert.

Evidence: internal/dispatch/watch.go:23-28 (RegisterWaiter/RemoveWaiter around the poll loop), reached via requestsJobWait in the new cmd/metasystem/watch_verb.go; design sections 2 and 3.1. Remedy sketch: one design sentence noting the --job route inherits the waiter's registration write.

DISPOSITION: ACCEPTED as non-material; rides the goal to later slices.

## WVJC-06 [low|material=False]

Three judgment calls remain open in the deferred slices, all fail-safe in direction but each a place where an implementer must invent: (1) section 5.3 requires a promotion validator to 'prove all goal-attributed landing entry points use the owner' with no stated mechanism for proving universal adoption; (2) section 5.3's launch refusal list includes 'returned' as a job status, which does not exist in the record vocabulary (a returned round is a completed record plus a return artifact); (3) section 5.4 says the 'permissions preset' is 'copied exactly from the source record', but the record persists the expanded permissions envelope under permissions.requested, not a preset name.

Evidence: Design sections 5.3-5.4 (diff.patch lines 2273-2338); internal/dispatch/record.go terminal vocabulary; internal/dispatch/build.go:381-393 (record stores destructiveReach, reasoningEffort, and the expanded requested-permissions object). Remedy sketch: before the slice-2 brief is cut, define the validator's adoption proof, replace 'returned' with the typed record-plus-return predicate, and name the envelope (not a preset) as the copied source.

DISPOSITION: ACCEPTED as non-material; rides the goal to later slices.

## WVJC-07 [low|material=False]

A single non-regular file or an unreadable directory in a multi-record store degrades the whole section and hides every record behind it, so a known attention item (for example a failed goal-less job behind a stray symlink in artifacts/agents/jobs) downgrades the aggregate to UNKNOWN rather than ATTENTION. Fail-safe (exit 2, never silent-healthy), but it softens the design's 'known attention outranks unknown' rule when the attention evidence itself is what got hidden.

Evidence: jsonFiles in internal/watch/watch.go returns nil paths on the first irregular entry or Info error (diff.patch lines 904-929), discarding all sibling records. Remedy sketch: skip-and-degrade per entry instead of aborting the whole directory listing.

DISPOSITION: ACCEPTED as non-material; rides the goal to later slices.
