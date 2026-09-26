# Fresh public surface audit before completion

Root notes from real checkpoint4 help and current source, 26 September 2026.
Required by the user's full intent/no-internals contract, not an expanded design.

- The reduced root has nine starting points and 24 lines. This is meaningful only
  when the complete workflow behind build/review/land, design/review, manual review
  and recovery exists. Keep every capability in intent-workflows-capabilities.md.
- Current review help still instructs a human to provide internal design record
  headers (`- Kind`, `- Id`, `- Goals`). Public design creates those automatically;
  its review help should describe a design and selection/recovery, not require
  knowledge of record schema. Author/maintainer protocol may retain schema details.
- Current commit review help still prescribes close then collect although the
  actual public review --dispositions now does that. Remove stale manual sequencing
  from normal help/docs/skills once clean design closure is also connected.
- Ordinary output should describe work, independent feedback, decisions, readiness,
  delivery and recovery. Internal record/lock/custody names belong in diagnostic
  detail only when they explain a concrete user choice; never as required next steps.
- Check all generated continuations, not just help: missing/failed/ambiguous input,
  author conflict, failed read, question pending, carry partial, manual source same
  as destination, goals matching reserved target names. A continuation must run
  from the caller's location with the displayed public reference and same identity.
- Root must drive a rebuilt immutable CLI and realistic complete fake-provider
  journeys at final candidate. Component green, a stand-in owner or hidden command
  invoked manually by the test is not evidence of a public complete journey.

No extra top-level commands or speculative parser framework are requested.


## Actual author transport needs a visible output path

Read-only recipe preparation found a first-use gap: intent_design.go's generated
contract says "the staged page you were given" but names no path.
design_request.go seeds the draft and sets StartSpec.Page/Outputs; ClaudeHeadless
Command sends the brief and read packet only, so those fields alone do not tell
the author where to write. Confirm against final source and fix through the existing
author prompt/declared-output contract. A fake author that secretly reads store
metadata or receives Page from a test callback would hide the bug. The real-process
fake-author fixture must obtain its output path from the actual prompt, just as
the paid author would. Independent Sol must inspect this boundary.
