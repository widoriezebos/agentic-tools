## Findings

- **SOL-ER-01 — material.** **Evidence (read):** [internal/config/evidenceroot.go:ResolveEvidenceRoot](/Users/wido/LocalStorage/GitHub/agentic-tools-erd/metasystem/internal/config/evidenceroot.go:98) checks containment for configured values but returns the `$HOME` default without that check. If `HOME` is the checkout, the default lies inside it; [app.go:preserveRunEvidence](/Users/wido/LocalStorage/GitHub/agentic-tools-erd/metasystem/cmd/metasystem/app.go:441) then copies evidence there. This violates the outside-checkout safety requirement. **Smallest fix:** apply the same resolved-path containment check to the default, with a fixture test where `HOME` is the checkout.

- **SOL-ER-02 — material.** **Evidence (read):** [internal/evidence/retired.go:WriteRetired](/Users/wido/LocalStorage/GitHub/agentic-tools-erd/metasystem/internal/evidence/retired.go:34) implements the pointer writer, but no production caller exposes it. The [design’s migration step](/Users/wido/LocalStorage/GitHub/agentic-tools-erd/metasystem/plans/evidence-root-default-design.md:378) requires a rerunnable engine command; the build report confirms that command was omitted. The specified migration therefore cannot run through the engine. **Smallest fix:** route `WriteRetired` through a tested command and resolve the internal-verb ratchet deliberately.

## Deferred

The other reported U1–U4 deviations preserve the design’s intent on the code read. Changed writer tests use fixture roots or environment seams; I found no test writing to the real `HOME`. Pinned refusal rows point to their current emission lines. The final merged tree has no recorded static-gate rerun; I ran no tests in this read-only review.

VERDICT: 2 material

Codex session ID: 01a0e9e4-113f-7db2-9faa-862f91d824a8
Resume in Codex: codex resume 01a0e9e4-113f-7db2-9faa-862f91d824a8
