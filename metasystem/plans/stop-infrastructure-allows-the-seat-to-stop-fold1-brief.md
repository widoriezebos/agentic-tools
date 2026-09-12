Finding Id: sia-build-fold-1
Disposition: accepted

# Finding Being Corrected

Round 1 stopped under the gap rule on three defects of the brief, not of the
page's mechanism: the acceptance criteria allowed one new test in
metasystem/internal/goal/turnverdict_idle_test.go while section 5 names two;
three existing tests assert that infrastructure failures block, which is the
very behavior this goal removes; and the hook-log line's cause code and
component had no mechanical mapping. All three are answered here. Build the
page as written with these answers; do not redesign.

# Disposition Reasoning and Evidence

1. Tests in metasystem/internal/goal/turnverdict_idle_test.go: add both
   TestIdleRefusalSurvivesALostCounter and
   TestSessionStopInfrastructurePreservesAuthority. The tests that must pass
   unchanged are the idle-handoff ones
   (TestIdleBacklogBlocksTwiceThenDefersClaimAndPreparesStewardContinuation,
   TestIdleEscalationPreservesAnIndependentOpenWorkBlock,
   TestThreeUnreadableLedgerStopsRecordIncidentRaiseAlarmAndEnd) and every
   single-use session-stop test that does not assert a block on an
   infrastructure failure.
2. The three tests that assert fail-closed blocking are rewritten to the
   new contract, in place, keeping their names' subjects:
   TestUnreadableTurnVerdictStateBlocksAsUncertainty becomes the assertion
   that an unreadable verdict state yields an allowing verdict with class
   "infrastructure", LedgerStatus degraded and the detail (rename it
   TestUnreadableTurnVerdictStateAllowsAsInfrastructure);
   TestBlockedHumanVerdictLeavesValidSessionStopUnspent keeps its
   authorization assertion (a valid session-stop is left unspent when the
   verdict blocks on real work) and drops only the infrastructure-block
   premise if it has one; TestSessionStopLibraryAndConsumerRequireHumanClassificationProof
   keeps its proof requirement (no authorization is invented or consumed
   without the human classification proof) and asserts that a failed marker
   read now allows the stop without consuming, instead of blocking. If a
   test's old assertion is the goal's own DONE reversed, the new assertion
   is the DONE; say so in a comment above it.
3. The hook-log line's fields, mechanically: `<cause code>` is the fixed
   diagnostic text of the record_stop_failure call with every run of
   non-alphanumeric characters replaced by one hyphen, lower-case, leading
   "the-" dropped (so "the narrator digest could not be read" becomes
   `narrator-digest-could-not-be-read`; "supervision arming failed" becomes
   `supervision-arming-failed`; the deadline path uses `stop-deadline-expired`;
   the launcher fallback uses `hook-bootstrap-failed`; a turn-verdict
   producer uses the function's own error prefix the same way).
   `<component>` comes from this table by hook site: runtime identity
   (lines 514 to 549) `runtime-identity`; turn and attempt evidence (745 to
   757) `turn-evidence`; checkout holder (766, 772) `checkout-holder`;
   arming (836) `supervision-arming`; health (842) `health`; narrator (855,
   859) `narrator`; holder protocol (1037, 1044) `holder-protocol`; lease
   renewal (1072) `holder-lease`; watchdog (1082, 1087) `watchdog`; hook
   evidence (1098) `hook-evidence`; the deadline `stop-deadline`; the
   launcher `hook-launcher`; the turn verdict's producers `verdict-state`,
   `goal-fence`, `state-root`, `status-write`; the session-stop marker
   `session-stop-marker`. Put both mappings in one place in the hook (a
   function beside record_stop_failure) and one table in the engine for the
   verdict producers, so a new site adds one row.

Everything else in round 1's build brief (the brief this chain was opened
with, in your round 1 prompt) and the design page
(metasystem/plans/stop-infrastructure-allows-the-seat-to-stop-design.md)
stands. Same workspace list, same commands, same return shape, same wall
clock (3 hours). Do not commit.

# Unchanged Return Contract

The original role and return schema remain binding without additions, removals, or relaxations.

Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.

Schema: the same scripts/agents/schemas/implementer.schema.json used for the original dispatch
