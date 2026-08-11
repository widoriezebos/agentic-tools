# Dispositions: patience-satellite-4, round 2

Job: design-critic-20260811t181104z-1c19 (codex gpt-5.6-sol, xhigh).
13 findings, 13 material, all accepted. The pillars survive (one
observable, contract-only sealed floors, vocal-only booking); the
fixes rework the count function, selection, encoding, handoff, and
compatibility text across most sections, so the document is
regenerated whole again.

| id | disposition |
| --- | --- |
| P4-016 | accepted — the "no crash window" claim was false as written. Replaced by an explicit contract: derivation reads the durable log plus the in-flight conclusion (suppression allowed); a crash between AppendCycle and the state write can leave one cycle's vocal line stale in either direction; that is documented one-booking lag in a non-acting audit surface, repaired at the next booking or mooted by terminal mission state. |
| P4-017 | accepted — vocality gets a durable handoff: the booking writes annotations only; the NEXT turn's This Turn lines are projected at assembly time from the final cycle block's Patience annotations in the ledger. Restart-deterministic, replayable, no new state field. Requires P4-028's read grammar. |
| P4-018 | accepted — floor lookup canonicalizes the record's effective model with the same canonical-model mapping cap keys use (config/model.go; the design's `capability.IsCanonicalModel` cite was a phantom symbol and is corrected). Keys and records meet in one string domain. |
| P4-019 | accepted — selection fallback chain: newest terminal round with a canonicalizable effectiveModel; else the chain's round-1 requestedModel canonicalized; else the SMALLEST floor among the contract's (role, runtime, *) entries — evidence damage must never widen patience; else (nothing configured for role+runtime) infinite, which is configured-nothing, not damage. |
| P4-020 | accepted — the input set is the mission's OWN jobs under the existing missionJobs ownership authority (mission stamp + fence reservation). Fully unreadable, unattributable records are out of patience scope (janitor/usage jurisdiction, satellite 3); the counts-barren rule applies to attributable-but-damaged records. The no-disappear promise is narrowed to attributable evidence. |
| P4-021 | accepted — identifier encoding bound: chain roots and jobIds appear in annotations and prompt lines only when they match the job-id grammar; a violating value is replaced by `invalid-<sha256 prefix>` (deterministic, grammar-safe). Both surfaces interpolate grammar-safe tokens only. |
| P4-022 | accepted — terminal = missionrunner.TerminalJobStatuses (includes cancelled), named as the single source; the F Q3.3 citation was the dispatch transition map and stale for this purpose. Cancellation cannot launder a spent round. |
| P4-023 | accepted — round numbers drop out of the definition entirely: certifications join by jobId, and the patience count is the length of the uncertified terminal SUFFIX of the chain's lineage (parentJob walk). Duplicate or regressing round numbers cannot heal unrelated work. |
| P4-024 | accepted — a certification counts only when its jobId resolves inside the mission's own job set AND the job is a lineage member of the chain under evaluation; foreign or nonexistent jobIds are ignored by patience. Certification hygiene adjudication stays out of scope (pre-existing runner behavior, noted for a possible future satellite). |
| P4-025 | accepted — the pre-append read is declared the linearization point: booked counts are counts-as-read; a reaper CAS landing during the write books at the next evaluation. Final-booking staleness is moot (completed) or superseded by the park ask (parked). No retry loop around the flocked append. |
| P4-026 | accepted — the old-binary direction is declared unsupported with a loud cutover: pre-feature binaries refuse patience-bearing contracts via the existing unknown-key preflight rule, which is fail-toward-refusal; no silent compatibility is claimed in either wording. |
| P4-027 | accepted — the bound copies the landed-returns implementation exactly: at most 20 lines total; on overflow, 19 detail lines plus 1 overflow line. |
| P4-028 | accepted — both Patience forms are added to the read grammar (annotationLineRe) as well as the write grammar, with a parse round-trip test; required by the P4-017 assembly-time projection. |
