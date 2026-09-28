# Fourth code review dispositions

Goal: host-runtime-setup. R-85-m1c expressly authorizes this fourth overall
review. Reviewer host-setup-opus-r6-review completed at2026-09-07T15:00:34Z
on Claude Opus5, sessionaaa9af18-3ce4-4303-8138-ca26576ca022. Exact subject:
host-setup-recover1-r6, tree577daeddb106dd78e20ee2fbb3da02d509c8d7fb.

One material finding remains, narrowed by main's actual baseline comparison.
The reported new Stop-blocking regression is false: published main also blocks.
Start and End newly returning128 before Git initialization is confirmed and
accepted as a compatibility regression. No source amendment, risk waiver, fifth
review or further job is authorized. Both additional jobs are complete.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| HOST-C4-001 | accepted | Accepted only for the newly introduced non-Git SessionStart and SessionEnd errors. Main executed the published commands with the real published hook and canonical engine, including the complete Stop deadline parent; baseline Start/End exit0 with empty output and Stop emits a block. The exact R6 fresh adopted target's Start/End exit128 and Stop also emits a block. The critic's claim that old Stop silently allowed is contradicted: it read the inner Git-query exit and missed the outer invalid-worker-output block. Old startup never armed supervision without Git either. This narrower introduced regression still changes behavior under an explicitly preserved adoption form, so it is not dismissed to certify the patch. Actual provider lifecycle was not executed. | Required correction in internal/hooks/setup.go for non-Git Start/End behavior, retaining all Git safety and existing Stop safeguards. Extend actual fresh-target launcher execution; preserve current proof. Evidence: artifacts/host-runtime-setup/recovery-r6-nongit-before-after.json |
| HOST-C3-001 | noted | Independently verified repaired. Main real adoption, setup/check/repeat/readoption pass for nested and quoted installation paths, with registrations local to their requested target. Opus independently verified nearest-installation resolution, legacy migration, symlink selection and separation from a parent's own installation. Historical acceptance remains truthful; the old register is not rewritten by this new evidence. | R6 correction; recovery-r6-real-adoption-results.json; current canonical reviewer return |
| HOST-C4-002 | noted | The extra Git-root field is diagnostic information currently read by tests; optional naming/data-shape cleanup has no demonstrated runtime or proof defect. | none |
| HOST-C4-003 | noted | The nested install's seeded state and subsequent application-root state location differ in the existing published implementation as well. RootForInstallation and adoption seeding are unchanged by this patch. Existing general installer/state redesign is outside this brief. No first later write was executed by main; retain the source-derived limitation without broadening scope. | none |
| HOST-C1-007 | noted | Preserved optional unused-helper observation. The registry self-check field still has a command reader; it is not evidence of an active gate defect. | none |
| HOST-C1-008 | noted | Preserved non-material receipt-launcher diagnostic observation under the existing declared scope. | none |

Earlier HOST-C1-001 was independently verified repaired in review3;002..006
were withdrawn in review2. The critic's statement that all are closed is
interpreted as code findings, not canonical register closure: old registers
remain unchanged. No evidence-free historical refutation is recorded.

The full R6 validator remains running at this disposition. Its first internal
Go pass failed fake Telegram shutdown, ordinary group termination and compressed
group termination; its configured plain fallback is still active. Complete
primary logs and the eventual own exit control the result. Both failure
packages match published main byte-for-byte. No baseline waiver or whole-suite
pass is claimed. No source is landed and no main setup has been installed.

Final validator outcome: exit1/1226.32s,5 sections pass,2 fail,37 gated. First Go pass failed fake Telegram and both termination tests; fallback failed only compressed termination. Dependency inventory also failed. Complete logs were read; no gated adoption run is claimed.
