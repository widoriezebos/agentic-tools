Verdict: **1 material finding, high.**

1. **HIGH — `candidatesPresent` is broader than today’s pinned “reply present” predicate.** Current Devin behavior promotes a named file only when it is non-empty and parseable JSON ([devin.sh:357](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:357>)). Otherwise, empty stdout enters `empty-reply`, including without a handshake ([devin.sh:368](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:368>), [adjudicate.go:142](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/adjudicate.go:142>)). Only a promoted/non-empty reply reaches the missing-session gate ([devin.sh:380](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:380>), [adjudicate.go:95](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/adjudicate.go:95>)).

R7 instead makes any named file—including an empty or torn one—and even a transcript designation call without a persisted return set `candidatesPresent=true` ([design:41](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/delegate-delivery-design.md:41>)). With no session, those cases become `handshake_missing_session_id`, while real code records `empty_reply`. That changes downstream started-work classification because missing-session is a never-started error ([patience.go:44](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/patience.go:44>)).

Evidence: code read at `92968a6`; no tests run and no files changed.

Proposed receipt: `review-only | delegate-delivery r7 | 1 high material finding: existence-only no-session predicate remains broader than shipped reply gate; no tests run`

REVISE: The no-session split still reclassifies empty, torn, or non-persisted file attempts as `handshake_missing_session_id` because existence-only `candidatesPresent` does not match the real adapter’s non-empty, parseable-reply gate.
