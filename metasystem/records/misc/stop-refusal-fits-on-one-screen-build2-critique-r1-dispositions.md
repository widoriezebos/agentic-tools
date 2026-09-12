# stop-refusal-fits-on-one-screen, second slice, critique round one: dispositions

Chain one-screen-build2b-20260909 (the allow path and the health line).
The critic one-screen-build2-crit1-20260909 (claude-opus-5) reviewed
tree 447fdb347dae4e750177a55f765d6077eb52a47c and returned two material
findings and three notes. Round two folds the two, one of the notes,
and the seat-side fixture failure the orchestrator found: the new
template-layout allow scenario dies silently because the hook's
completion is refused once the message is bounded, the same defect as
OSR-11 seen from the fixture's side. The critic's sandbox could not run
conformance, the hook suite or the steward package; the orchestrator
ran all three: conformance printed the same tree, internal/steward
79.1 percent and internal/report 87.6 percent green, and the hook suite
red on the new scenario for the reason above.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| OSR-11 | accepted | Real and self-worsening: steward hook-complete requires the payload to contain the health line verbatim (hookPayloadContainsHealthLine in internal/steward/component_evidence.go); the hook passes the full line while emitting the bounded message, so completion refuses whenever trimming happened and hook-freshness goes stale on the seats that are already unhealthy. | Round two: the hook completes the attempt with the health line it actually emitted, the compact preview line, and the engine's check compares against that; the fixture asserts the completion evidence is written for the over-bound allow. |
| OSR-12 | accepted | Real: health --hook-preview exits 2 with no output when the sibling health file cannot be written, so the hook substitutes HEALTH unknown, records a stop failure, and the first failure blocks an honest allow. | Round two: the preview prints its line and reports the write failure on stderr with a non-fatal code; the hook carries the failure as a diagnostic line, never as a substitute verdict. A test makes the directory read-only and asserts the line still prints. |
| OSR-13 | noted | The repository-root argument of PreviewHealthAt is read by nothing but the root chooser; a comment overstates it. No behaviour depends on it. | Round two updates the comment and the signature's name while it is in the file; no behaviour change. |
| OSR-14 | noted | Start-event notices stay outside the bound, correctly: health is computed only for Stop, and the DONE covers Stop messages. | none; recorded here. |
| OSR-15 | accepted | An engine that predates report bound-message makes every allow-path message vanish instead of degrade; the window is small but its failure mode is silence, the worst one. | Round two: surface_json falls back to the raw text when the verb fails, and says so in a diagnostic. |
