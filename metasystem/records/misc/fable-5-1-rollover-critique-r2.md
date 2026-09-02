# Fable 5.1 rollover design — Sol critique, round 2 (closing)

Critic: codex gpt-5.6-sol (job f51-crit-2, DESIGN-BEARING, read-only, source-level verification with the dispatcher's test run as evidence). Zero material findings, zero gaps: the design phase closes.

```json
{
 "claimed": {
  "model": null,
  "sessionId": null
 },
 "evidence": [
  {
   "command": "git rev-parse HEAD; git status --short",
   "level": "ran",
   "observed": "The exact reviewed commit is 6b3b3e3fc27f7b3456b271713933c1168e0e1da4. The runtime had modified only metasystem/records/narrator-digest.log; no reviewed source file was modified."
  },
  {
   "command": "Dispatcher-supplied run from metasystem/: go test ./internal/dispatch -run TestHazardConfigurationAcceptsConfiguredMaximalModel",
   "level": "read",
   "observed": "The supplied 2026-09-02 07:38 UTC run failed at metasystem/internal/dispatch/composition_test.go line 260 because runtime claude had no executable maximal-effort mapping for the DESIGN-BEARING destructive-reach class."
  },
  {
   "command": "Read metasystem/internal/dispatch/composition_test.go lines 13-20 and 247-267, metasystem/internal/dispatch/composition.go lines 137-142, and metasystem/internal/dispatch/hazard.go lines 91-138",
   "level": "read",
   "observed": "compositionRepoRoot resolves ../.. from the dispatch package to the real metasystem root. The test submits claude-fable-5 for a DESIGN-BEARING packet. ComposeRolePacket invokes ValidateRuntimeHazardConfiguration, whose resolver reads metasystem/metasystem.conf and its local overlay; the tracked configuration admits only claude-fable-5-1, explaining the supplied failure. Changing the model input and result assertion at lines 256 and 262 to claude-fable-5-1 is the correct repair."
  },
  {
   "command": "rg -n -C 12 'claude-fable-5' metasystem/internal/dispatch/decisions_test.go metasystem/internal/dispatch/claim_test.go metasystem/internal/config/validate_test.go metasystem/cmd/metasystem/delegate_reroute_test.go",
   "level": "read",
   "observed": "The decisions and claim tests create temporary repository roots and write their own metasystem.conf files. The configuration-validation tests create a temporary repository from the validConf string. The reroute test operates on a standalone JSON modelUsage fixture and does not read configuration. None reads the committed configuration."
  },
  {
   "command": "Read metasystem/internal/dispatch/composition_test.go lines 269-284 and 439-462, plus metasystem/internal/dispatch/decisions_test.go lines 728-747",
   "level": "read",
   "observed": "The other old-model occurrences in the composition test are intentionally isolated: one test creates a temporary configuration and local overlay, while the closure test uses mirrorFixture, which creates a temporary repository, and writes its own configuration. They do not need rollover edits."
  },
  {
   "command": "Read metasystem/internal/dispatch/claim.go lines 203-233, metasystem/internal/dispatch/close.go lines 10-30, and metasystem/internal/dispatch/hazard.go lines 165-213 and 261-301",
   "level": "read",
   "observed": "The maximal-model gate runs during authoritative claim admission. CloseCheck also calls hazard completion validation, which rereads the current configuration and checks the completed critic's requestedModel at lines 293-296. Therefore an unclosed chain whose critic used claude-fable-5 can be refused after the tracked list drops that identifier."
  },
  {
   "command": "Read metasystem/internal/config/resolve.go lines 42-118 and metasystem/internal/config/resolve_test.go lines 89-105",
   "level": "read",
   "observed": "For this unscoped key, configuration resolution checks the environment first, then metasystem.conf.local, then tracked metasystem.conf. Consequently the proposed local dual value wins over the tracked new-only value and restores closure admission, provided no higher-precedence environment override replaces it. The design correctly states the governing condition as the seat's effective list containing every unclosed chain's critic model."
  },
  {
   "command": "Read metasystem/memory/rulings.md lines 24-89 and inspect commit d081ef075d365cd899ad95c06466f50bc196e0e9",
   "level": "read",
   "observed": "R-25-m1 assigns Fable to design authoring and implementation critique. Proposed row R-46-m0b changes only those lanes' model identifier, preserves the Sol lanes and effort structure, accurately records the tracked new-only maximal-model value and local dual-list remedy, and is specified as an append after R-45-m0b. It does not edit R-25-m1."
  },
  {
   "command": "git grep -l 'claude-fable-5' -- metasystem/; git grep the current Git blob and SHA-256 hashes of metasystem/internal/dispatch/composition_test.go",
   "level": "read",
   "observed": "Tracked Go-code occurrences of the old identifier are confined to the five test files examined above. No tracked file pins the composition test's current Git blob hash or SHA-256 hash, and no additional test asserts the old identifier against the real repository root."
  },
  {
   "command": "Read the old-model job records under metasystem/artifacts/agents/jobs/ and follow code-critic-1d7716a3e5e141468637ff63 to its reviewed root",
   "level": "read",
   "observed": "On this seat, every old-model job is terminal. The sole old-model independent critic completed at 2026-09-01 19:40:36 UTC and reviews root implementer-d1947930c9b516cb64dffdb8, whose chainClosed value is true. This agrees with the design's seat-local drain claim."
  }
 ],
 "findings": [],
 "gaps": [],
 "jobId": "f51-crit-2",
 "mode": "design",
 "model": {
  "effective": "gpt-5.6-sol",
  "requested": "gpt-5.6-sol"
 },
 "reviewedCommit": "6b3b3e3fc27f7b3456b271713933c1168e0e1da4",
 "rigor": [],
 "round": 1,
 "runtime": "codex",
 "schemaVersion": 3,
 "sessionId": "01a06108-74d3-7b30-a3b2-fbe80c1ea0a6",
 "verdictMaterialCount": 0
}
```
