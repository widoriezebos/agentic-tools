# bed-child-death-reported-as-pass, critique round two: dispositions

Chain bcd-build1 after its round-three fold, critic bcd-critic2 (claude,
claude-fable-5-1, xhigh), reviewed tree
e705a17216bae984cbb09e5528ef4563c3728cd1. The critic returned zero
material findings and two notes; the chain closes on it. The critic had
no shell; the orchestrator's seat-side runs on the reviewed worktree
under the stock bash 3.2 (the direct self-test probe exiting 70, the
empty-runtime mode passing, the whole bed passing every ordinary scenario
with the self-test reported passed under its expected-to-die rule and no
evidence directory left for it) stand as the executed proof.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F-1 | noted | A filesystem failure inside cleanup under errexit can replace a signal or death status with 1; the parent still reports the scenario failed, so no death is hidden. Pre-existing shape of the cleanup body. | none |
| F-2 | noted | The self-test proves deaths before the completion line only; a statement appended after that line would escape it. Inherent to the sentinel design; the rule that nothing follows the completion line is documented in the brief and held by review. | none |
