# Dispositions: hgvf-cc1c-20260906, round 1 (the chain's first review)

Chain under review: hgvf-build1-20260906 (reviewed tree
f8abd52ce2bf721927dc506fc034956b68acf402, round 1). Critic:
hgvf-cc1c-20260906 (claude, Fable), three material findings, three low
notes. The chain folds once (follow-up job hgvf-build1-20260906-r2,
brief plans/human-goal-verbs-forgiving-fold-brief.md) and gets one
re-review. Orchestrator: m1d. An earlier critic on the same tree,
hgvf-cc1b-20260906, died on the Claude account's session limit before
returning; it is registered as a failed round with no findings. The
reviewer ran read-only and could not rerun the gate or the suites; the
seat's outside-sandbox replay on the reviewed tree covers that gap:
build, vet and gofmt clean; go test of internal/goal, internal/goalbudget,
internal/config, internal/humanauthority and cmd/metasystem green (the
ownership test the sandbox could not run passed); goal-cli-fixtures.sh
passed with the new forgiving-human-refusals scenario.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| HGV-01 | folded | True: approval.go raised BudgetExceptions on every over-box approve and skipped the idempotency guard for parked goals; the design's section 2 names admitting a parked goal as the one transaction rule that changes, and on main only set-budget raises the counter. With the terminal fold an identical over-box command run twice would record two lines and reach the steward's defect marker. The package test for the parked case passed only through the bypass. | fold brief item 1: remove the increment, restore the guard, the test approves a different box and proves nothing-to-do on the same box |
| HGV-02 | folded | True: the goal's DONE line says every refusal prints one command that would have succeeded with the values it saw. The sweep-plus-id row re-printed a refused command; budget refusals from the engine re-printed the failing command; keep with an approved reference printed a command resume refuses; six-member boxes and completions equal to the standing box printed commands that exit 1; classify-sweep printed an empty --draft; resume on a claimed goal without a Budget line printed words where the design's row says the norm command. | fold brief item 2, row by row |
| HGV-03 | folded | True: design section 6 requires each refusal scenario to run the printed command and compare the history and Budget lines with a long-form twin; the helper checked shape and exit code only, and the accept-risk pair scenario never ran its printed line. | fold brief item 3: the helper takes a twin and compares both records; the accept-risk row runs or names the bed's limit |
| HGV-04 | noted | True and intended: the classification sweep now proves authority before confirmation and records a proof per landed act, which follows from design section 4 (the name default resolves after proving authority). The fixture gained the fixture-authority flag for that reason. Acknowledged here as intended; no change. | none |
| HGV-05 | noted | True: under a temporary-word or channel proof the name-default refusal says the terminal has no recorded name, while the cause is the proof class. The refusal is correct and the words line names --by; the sentence only misleads. Backlogged with the wording notes of this goal's conclusion. | none |
| HGV-06 | noted | True: the exclusion of temporary-word and channel proofs from the over-norm branch is proven at the predicate, and the branch consults nothing but that predicate and the operation id. Belt and braces would be one norm-level test; not required. | none |
