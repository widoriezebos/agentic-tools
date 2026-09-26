# Agent help critique dispositions

Root owns adjudication. Fable 5.1 launch `20260926t180359-91cd2b1378`, provider
session `e75da544-a286-4820-8637-13a7edd8ae2c`, critiqued the complete design.
This is a standalone review under the human-authorized machinery bypass.
Round 1 has two material findings, both bounded and fixture-expressible; design
critique closes under the declared early exit. Independent code critique also
closed at round 1 with zero material findings, as recorded below.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| AH-D1 | accepted | The protocol must explain decision. Refute the proposed universal human-only meaning: `intent_delivery.go` reviewCommit sets Decision to `name the goal with --goal G`; runIntentReview sets it to `omit --model, or review the built unit...`. These are missing-input guidance for the agent. Root read both exact assignments. Treat decision as a prerequisite requiring judgment and never as automatic authority. | Protocol includes decision with conditional human authority; TestIntentAgentResultProtocol. |
| AH-D2 | accepted | Diagnostic changes/diff must never advertise submission flags. Pin all eight form mappings and exact option subsets to the existing routing contracts. | Explicit design mapping; TestIntentReviewHelpForms verifies argv and options, existing real-owner fixtures prove effects. |
| AH-D3 | noted | Structured command description includes a forms index; clarified with no mechanism change. | Design wording clarified. |
| AH-D4 | noted | Root, agent and all JSON index is complete, independent of the nine-verb text orientation; clarified. | Design wording clarified. |
| AH-D5 | noted | Parser/argv tests alone do not prove execution effects. Keep existing owner fixtures and source-traced metadata; no new classifier. | Proof boundary clarified. |
| AH-D6 | noted | review unit is deliberately compatible but not advertised; the public goal/work route preserves its capability. This is not a missing public task and will not be re-exposed. | No implementation change. |
| AH-D7 | noted | Partner already directs readers to help agent and help COMMAND. Extending its handwritten help summary is optional; keep this slice scoped. | Deferred until a Partner discovery need is observed. |

Fable's non-finding suggestion to marshal a separate result directly is not
adopted: constructing a render-only intentInvocation requires no owners or state.
Use the existing renderer, with sentinel tests protecting the no-execution boundary,
instead of maintaining a second success/refusal envelope implementation.


## Implementation review and final root correction

Fable 5.1 launch `20260926t182056-0b1e5a2a17` reviewed candidate
`556676ff6345921a9db1267ea3493a3a31397f64`, tree
`7bb5f7aa0e1a2b9598efb4e44396a5b4c0c0c426`, against base `0a38b2e01`.
It independently computed the complete diff including new files (SHA-256
`9ce7481def7c8b798d607abee1e1a538aa84e11594f9fd72722ac14e361249cf`).
The report is `plans/agent-help-fable-code-review.md`: zero material findings;
close at round 1 under the declared stop rule.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| AH-N1 | noted | Only review owns focused forms; the text header is correct for every current consumer. Generalizing for an absent second consumer adds no current value. | None. |
| AH-N2 | noted | Unknown help options retain refusal exit 2 and empty text stdout; a clearer error is permitted. Top-level help aliases retain their bytes. | None. |
| AH-N3 | noted | The shared work option remains accurate; submit's repetition contract supplies its default. | None. |
| AH-N4 | noted | Parsing tests prove syntax only. Root separately ran the existing real-owner review fixtures; the help trial selected commands without executing them. | Evidence boundary retained. |

After Fable's immutable review, the independent Opus help-only trial exposed
submit's misleading `--after N` value label. Root changed focused option notes
to optionally supply a value label and set this one to `COMMIT`, with a direct
assertion. Execution parsing and full human help are unchanged. This final
three-file correction was root-reviewed and verified with focused race tests
and the real CLI; it was not part of Fable's reviewed bytes. No further critique
round is needed for this bounded, reproduced display defect.
