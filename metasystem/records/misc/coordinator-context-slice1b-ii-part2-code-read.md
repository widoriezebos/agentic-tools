# Code read: CCB-1b-12, the bounded-read sentence (Opus, 2026-09-13)

Reviewer: Opus, independent of the author. I only read. I edited no repository file and made no commit.

Brief: row CCB-1b-12 of plans/coordinator-context-stays-under-budget-design.md (section 3, "Instruction", line 50; matrix row at line 129).

Diff reviewed: scratchpad/ccb-1bii-p2.diff. It is byte-identical to `git diff -- docs scripts` in the checkout (checked with diff).

Materiality test, applied to every finding: would the change ship a defect, violate its brief, or damage what certifies it?

## Layer 1: conformance to the row

- All five named files carry the sentence. In the brief template and the three role contracts it is a standalone paragraph starting with a capital "Request". In docs/orchestration.md:21 it sits after the lead-in "A delegate reads within its context:". This matches what the row asks the files to "say".
- The grep leg is at scripts/agents/conformance-fixtures.sh:8-15. The row names that file as the proof.
- Under docs/ and scripts/ the diff touches only the six expected paths. The working tree also has changes in plans/, memory/receipts.log and records/narrator-digest.log. Those are outside this diff and must not be staged with it.

## Layer 2: the specific checks asked for

1. Conformance: see Layer 1. Nothing is missing.
2. Things that pin bytes or line structure. None of them break:
   - role-packets.json lists source paths only, with no digests or sizes (scripts/agents/role-packets.json:38-60).
   - preamblequotes.go trims the last newline of a quote and then does a substring match (internal/validate/preamblequotes.go:94-99). I reran `validate preamble-quotes` and it returned rc=0.
   - The qualified-names audit only flags words like "the job" or "a gate" with no qualifier (internal/audit/metasystem.go:205). The new sentence has none of those words. I reran `audit metasystem --root .` and it returned rc=0.
   - fingerprint-harness.sh:187, supervision-fixtures.sh:1103-1104, 1397-1398 and 2367, dispatch-fixtures.sh:968, and adapters/fake.sh:449 and 453 only rewrite the `^Working Mode:` line with sed and keep everything else.
   - The happy leg in dispatch-fixtures compares the preamble and the brief against copies taken from the same fixture repository (dispatch-fixtures.sh:2103-2111), so no bytes are pinned there.
   - validate-metasystem.sh:1616-1624 checks only the template's header lines.
   - BriefMode reads only the Working Mode line (internal/dispatch/brief.go:41-55). The brief-authority path scan needs a token containing a slash (brief.go:14-15), and the sentence has none.
   - The adapter self-test brief and its byte pin do not use the template (internal/adapter/selftestrun.go:134-150, selftestrun_test.go:91-129).
   - Composition records a source digest but never refuses on drift (internal/dispatch/composition.go:244). No role digest is pinned anywhere I could find.
   - I ran `go test -count=1` on internal/dispatch, capability, mission, adapter, steward, validate and audit. All passed.
3. Does the brief.md placement survive? No code builds a brief by replacing a whole section of the template. The programmatic consumers listed above only sed one line or append lines. Human-written briefs keep the sentence as long as the author replaces only the placeholder line. Residual risk is recorded as N-3.
4. Does the grep leg run first and use the right root? Yes.
   - It runs right after `mktemp` and the EXIT trap, before any engine call or fixture case.
   - `source_root` is the metasystem root (conformance-fixtures.sh:4), so the five relative paths resolve.
   - I copied the files to scratch and ran the leg there (probes). With all five files intact it passes. With the sentence removed from any one of the five, it fails with exit 1 and names that file.
5. Does the implementer quote still validate? Yes. The implementer's quote "stop and report it, never fill it silently." is still in docs/orchestration.md:21.
   - The orchestrator's Delegation Contract quote (scripts/agents/roles/orchestrator.md:19-22) is now a byte prefix of the longer line, so the substring check still passes.
   - The mandated-block count for "## Delegation Contract" (validate-metasystem.sh:1861) is unchanged.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| N-1 | low | no | When a later change touches only a role contract or docs/orchestration.md, the landing never runs the grep leg. Such an edit could delete the sentence and still land green. The next cadence run would catch it. For this diff the leg does run: brief.md and the fixture script belong to no surface, so the residual surface forces the deep mode, which includes the conformance group. | Ran: a scratch test injected with `go test -overlay` called testpolicy.Select on testing.json. Role-only and docs-only delivery plans are "standard" without section/conformance-fixtures. This diff plans "deep" with it. The instructions surface (testing.json:24) owns docs/** and scripts/agents/roles/**. It selects static-contract-audits and return-schema-fixtures (standard) and policy-protection and runtime-contract-audits (deep), and its critical obligation is instruction-integrity. The conformance group provides only testing-policy-protected and declares inputs `metasystem/scripts/**` only (testing.json:70). Its reuse identity therefore ignores docs edits (internal/proofrun/test_build.go:1136-1160). It also runs as `needs-engine` and can be skipped under delivery reuse (validate-metasystem.sh:1129-1130). Cadence runs afresh (cmd/metasystem/test.go:819) and lists the group (testing.json:98). Not material because the row names this test location, the leg runs on this landing, and no existing proof gets weaker. The same undeclared-docs pattern already exists: runtime-contract-audits reads docs/orchestration.md through preamble-quotes (validate-metasystem.sh:1819) with inputs limited to scripts. Better home if wanted: section/static-contract-audits. It runs on every landing (testing.json:96) and declares both scripts/** and docs/** as inputs (validate-metasystem.sh:1035). |
| N-2 | low | no | The grep pattern leaves out the leading word "request" so that capitalisation does not matter. As a result, an altered lead-in still passes. | conformance-fixtures.sh:10. Ran a scratch probe: a copy of implementer.md with "Never request the smallest section you need, ..." passed the leg. Not material because the row asks for presence, and an accidental inversion like this is unlikely. |
| N-3 | low | no | The brief-template copy of the sentence is lost without any warning if an author treats everything under "# Constraints" as the placeholder. Nothing checks written briefs for the sentence. | scripts/agents/templates/brief.md:26-30. No Go or shell code builds a brief by section. The build-cache paragraph at brief.md:14-20 already relies on the same author habit. Delegates still get the sentence through the role-instructions slot for all three roles (role-packets.json:38-60). If the brief-level copy ever matters, the dispatcher already has a stronger pattern: `append_return_path_form` appends a required sentence when it is missing (scripts/agents/dispatch.sh:864-871). Not material because the row only asks the template to carry the sentence, and it does. |
| N-4 | low | no | Every implementer packet grows by 288 bytes: 162 in docs/orchestration.md and 126 in implementer.md. A brief started from the template adds 126 more, so one packet can carry the sentence three times. Because the whole composed prompt is capped at 64 KiB, the room left for an inline implementer brief shrinks by the same amount. | Byte counts from git HEAD versus the working tree: orchestration.md 48724 to 48886, implementer.md 1609 to 1735, brief.md 1715 to 1841. The cap is applied to the whole prompt file (dispatch.sh:1794 and 1938-1944, dispatch.max-inline-input-kb=64). Newest implementer prompt: artifacts/agents/implementer-d4c60cffc5ec34661ab5ea70/rounds/2/prompt.md, 50,581 bytes, with the whole orchestration doc under "# Required Skill". The largest recent one is 58,259 bytes. Not material because the row requires the text in both files, and an over-limit packet is refused loudly with "pass a file reference". |
| N-5 | low | no | The design critic now reads two instructions that pull in different directions: "request the smallest section you need" and the design-critique skill's "The critic reads the full design". | skills/design-critique/SKILL.md:27 versus scripts/agents/roles/design-critic.md:20. The wording "you need" leaves room to read the whole design in bounded views. Not material because the row fixes the wording, and nothing changes in what gets built or proven. |

Material findings: 0.

Verdict: conforms to row CCB-1b-12, and nothing that pins bytes or line structure breaks. The grep leg runs first, from the right root, and fails when any file loses the sentence. Zero material findings; five non-material notes. The most useful one is N-1: move the leg to static-contract-audits if you want every landing to guard against removal.
