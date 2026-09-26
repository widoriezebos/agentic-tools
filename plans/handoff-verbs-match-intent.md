# Human and agent verbs that follow intent

The complete redesign and coverage repair are integrated in this commit.
The final integration candidate is `29086561a4f74dc6fdd7a71fac88bb4316e705eb`, tree
`ccf74df15f01491d8173f6af8236f24167e6774f`. It combines the local redesign
`25f0e2e04267fe626b5622e95e0c71b756d48aae` with remote main
`dbafa75fae548ae7ff6010d51d47303a43a49e53`. Product bytes are identical to
the tested candidate; this commit adds completion records and its receipt.

Wido authorized the complete public redesign, root-authored design, Fable or
Opus critique, implementation, coverage repair, integration with main and
push. Machinery bypass remains authorized and the stop hook is ignored.
Ruling R-126-m1e requires public verbs to follow human and agent intent rather
than internal structure. Do not resume the unrelated test-repair goal.

## Where to look

- [Accepted design and completed obligations](designs/verbs-match-intent.md).
- [Independent Fable critique and root dispositions](verbs-match-intent-design-review.md).
- [All 23 related goals](verbs-match-intent-goal-index.md).
- [Verification and coverage follow-up](verbs-match-intent-verification.md).
- Exact sources, logs, reviews, preservation backups and publication status:
  `/Users/wido/LocalStorage/agentic-tools-evidence/verbs-match-intent-20260925`.

## Implementation and review

Root authored the design. Fable agreed in `20260925t151710-e1ded7896c`;
the committed-review connection amendment was reviewed in
`20260925t164057-54edd2fa11`. Opus 5.5 authored implementation and repairs;
independent Sol reviews covered conformance and defects. Root read the final
changes and adjudicated all findings. No material finding remains open.

The coverage follow-up preserves remote main's four added package floors
and all UI work. Meaningful session and failure-mapper tests close the
remaining measured shortfalls. Frontend fixtures preserve existing assertions
while using the current surface and a live asynchronous registry. Delegate
fixture cleanup now stops its own steward before removing identity records,
then verifies process absence. No production lifecycle change, new dependency,
coverage-floor reduction, or new coverage exemption was introduced.

The merge/session/frontend review is `20260925t214158-ef59e15747`; mapper
review is `20260925t220940-7db121f6f8`; fixture cleanup review is
`20260925t222443-5ebbfa8d9e`; the shared TestMain correction review is
`20260926t061454-25ae7dce51`. The final one-line audit correction is independently reviewed in
`20260926t065608-d6743d1591`. All five close with zero material findings and
retained root dispositions. These launched reviews use the authorized bypass;
no registered critic chain was invented.

## Verification

The final integration includes main's subsequent edit-sheet simplification
and design records. No Go source changed after the full-gate candidate
`d374f08c9daef03cbe20ff131e34215fc242b47b`. All 1,058 frontend tests, typecheck
and bundle were rerun on `58815db14`, followed by passing HTTP and
embedded-bundle race/coverage tests (`coverage-final-main-delta/`). The final
candidate `29086561a4f74dc6fdd7a71fac88bb4316e705eb` changes only one literal
expected actor-site line in the brain audit: gofmt-aligned spaces match the
existing Go caller. Its full brain-fixture entrypoint and the affected
filled-delivery adoption body passed separately. No full Go run was repeated
for that audit text; each result retains its actual source identity.


The unchanged full native race/coverage gate passed at `d374f08c9`, with
12,435 passing test terminals, ten explicit skips, no failures or missing
terminals, and all 118 registered package floors enforced. The existing
coverage-ratchet owner independently accepted its retained output with exit
zero (`coverage-final-r4-owner-judge/status.json`).

The outer adoption run actually exited one: nine scenarios passed, while
filled-delivery found only the stale whitespace-sensitive audit expectation.
That historical result remains unchanged in
`coverage-final-r4-gate/adoption-status.json`. After the one-line correction,
the original filled-delivery scenario body passed at `29086561a4`
(`coverage-filled-final/result.json`). Its external diagnostic wrapper omits
only parent proof admission and witness production under the bypass; it
asserts neither authority nor a witness and changes no scenario body.
All ten scenarios therefore have passing evidence at the stated revisions;
there is no claim that the original parent became green.

The original six coverage failures and the additional failure-mapper shortfall
are resolved. Earlier red and interrupted attempts remain recorded under their
actual revisions and outcomes.

The selection contains 201 groups: 163 native and 38 sections. Frontend tests
(1,058), typecheck and bundle passed. Separate tagged tests, stop-cost checks
and seven affected sections retain their earlier source revisions. The
subsequent main changes affect the UI, covered by the current frontend tests,
focused Go race/coverage checks and final full native run. Historical
section and corrected scenario evidence retains its source boundaries;
`coverage-final-evidence-reuse-assessment.md` explains the composition.
This is diagnostic evidence under the bypass, not a fabricated governed
201-group certificate. The original 72-command runtime walkthrough and
connected delivery proofs remain linked from the verification record.

The runtime-required design matrix, goal-to-design resolver and project
record validation pass. All critical and high obligations are done.

## Integration and remaining authority

The integration preserves both local and remote main history. Its publication
status and exact remote commit are retained outside the repository. Existing
uncommitted receipt and narrator rows are preserved separately from the new
receipt included with this work. The live checkout executable is unchanged;
there was no installation or service restart. Never read or copy the actual
`metasystem/metasystem.conf.local`.

The goal `verbs-match-intent` remains formally queued. Conversation authority
does not invent a ledger approval or held claim. Its next step is updated
through the goal owner with the actual publication result. Formal conclusion
of this human-opened goal remains a human act; no related goal is closed
automatically. The existing due retro remains separate work.

## Requested follow-up: evidence reuse judgment

Wido asked how these source-aware reuse decisions should become normal machinery
without brittle deterministic rules or unnecessary repeated testing. The proposed
smallest extension is to retain existing group/scenario terminals and add one
source-bound agent reuse decision consumed by the current plan/run/verify owner.
Deterministic checks validate identity, provenance, completeness and contradictions;
the agent judges whether a source delta invalidates an observation and chooses
residual tests. Original failures remain failures. Exact reuse already exists,
including passing groups in failed outer attempts. Keep executed pass, exact reuse
and agent-justified carry-forward distinguishable. No implementation or policy
change is included here; the concrete source reads and proposal are retained in
`evidence-reuse-machinery-proposal.md` in the external evidence root.
