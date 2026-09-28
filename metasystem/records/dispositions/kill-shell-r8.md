# Dispositions: kill-shell plan, round 8

Job: design-critic-20260812t001609z-f547 (codex gpt-5.6-sol, xhigh).
4 findings, 4 material, all accepted.

| id | disposition |
| --- | --- |
| KS-R8-001 | accepted — the template test is the metasystem's OWN module path in go.mod (exact match), not go.mod presence: adoption into ordinary Go repositories keeps their module untouched, so identity, not existence, is the boundary. |
| KS-R8-002 | accepted — publication is serialized: replacing bin/metasystem happens only under the Go owner-lock (the shipped primitive) followed by an atomic rename; two racing first-builds serialize on the lock, and fence-clear grants the right to CONTEND, not to publish. |
| KS-R8-003 | accepted — registry script entries gain an export condition (always, or with-skill:<name>); the manifest projection respects conditions, so the optional-skill preflight is registered without being unconditionally shipped. |
| KS-R8-004 | accepted — the Go section is honest about grain: package-grain is the enforcement boundary (an unreachable package without an entry fails); function-grain findings from the sweep are recorded as registry debt with deadlines, matching what the analyzer actually reports. |
