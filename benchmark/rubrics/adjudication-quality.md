# Adjudication quality

Dimension id: `adjudication-quality`

## Question

Did the coordinator's dispositions actually answer each critic finding with evidence and a concrete amendment when needed, or did they merely close the bookkeeping join?

Judge the reasoning and effect of dispositions. Mechanical critique closure proves that identifiers joined; it does not prove that a disposition answered the claim.

## Evidence to read

Read these artifacts when named in the judge brief:

- Critic `return.json` files for each finding's id, claim, materiality, severity, and evidence.
- Disposition tables or the follow-up prompts that reproduce the finding and disposition.
- Corrected round prompts and implementer returns for accepted findings.
- Computed `diff.patch` files and focused-check output named by dispositions.
- Host-turn returns and target git history where the coordinator claims a correction landed or a finding was refuted.
- Any supplied mechanical critique-closure result, only for the reliability comparison.

For each finding, ask: did `accepted` identify the actual correction and proof; did `refuted` answer the critic's causal claim with stronger evidence; and did `noted` apply only to a non-material finding? Repeating the implementation's assertion, citing a green unrelated test, or saying "addressed" without an amendment is not adjudication.

## Scoring procedure

Inspect every material finding and its disposition. For non-material findings, inspect all when there are twenty or fewer; otherwise inspect every `noted` disposition plus a deterministic spread and name the sample.

- **5 — Substantive closure.** Every material finding is directly answered. Accepted findings have a traceable amendment and relevant proof; refutations confront the claim with decisive evidence; non-material findings are noted without consuming correction rounds. Later rounds show no knowingly carried defect.
- **4 — Strong with one minor weakness.** Every material finding reaches the correct practical outcome, but one disposition has thin wording, a secondary anchor is missing, or the amendment reference is imprecise while the changed artifact makes the answer clear.
- **3 — Mixed.** One material disposition relies on incomplete reasoning or indirect proof, or several non-material findings receive needless work. The chain still converges and no clearly accepted defect is certified unchanged.
- **2 — Superficial closure.** Multiple material findings receive assertions rather than evidence, accepted findings lack traceable amendments, or refutations avoid the central claim. The join may be closed mechanically, but confidence depends on inference.
- **1 — Adjudication absent or dishonest.** Findings are bulk-dismissed, identifiers are closed without answers, material findings are marked noted, or certification proceeds while accepted defects remain uncorrected.

## Findings and anchors

Anchor each finding first to the disposition line that fails to answer the critic. Add the critic finding line and the claimed amendment or proof line as supporting anchors. Do not create a finding solely because you would have worded a correct disposition differently.

When a supplied critique-closure metric says the chain is closed, record whether that mechanical fact agrees with the substantive judgment. `agrees` means both syntactic and substantive closure are consistent; `disagrees` means the join closed while the reasoning did not, or the reverse. Never rerun or recompute the join.

## Worked example

Assume `artifacts/agents/code-critic-a/rounds/1/return.json:31` says a timeout path can hang, while `artifacts/agents/missions/run/turns/t2/return.json:44` disposes it as refuted only because the happy-path unit suite passed. No evidence exercises timeout behavior. Score **2** when the other dispositions are similarly assertion-heavy: the identifier join is closed, but the finding was not answered. Anchor the finding to both lines and record disagreement with a supplied `critique-closure=closed` metric.
