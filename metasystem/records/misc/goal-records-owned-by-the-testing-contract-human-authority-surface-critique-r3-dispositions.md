# goal-records-owned-by-the-testing-contract, human-authority surface, critique round three: dispositions

Chain ha-surface1-20260910, round three (the receipt repairs: the
steward fixture's process tree, the proc detach verb replacing perl, the
demotion of section/goal-cli-fixtures). The critic ha-surfacecrit3x-20260910
(claude-opus-5) reviewed tree 6b1cbbd9c70f44d0f012cc59d5c7a6df17fe657b and
returned three notes and no material finding. The chain lands as reviewed.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| HAS-08 | noted | proc detach returns 1 for a child killed by a signal, where a shell would return 128 plus the signal; its only caller checks a nonzero status plus the TERMINAL_NOT_REACHED text, which a signal death cannot satisfy either way, and the stderr line names the signal. | No change; a later general-purpose caller would ask for the shell convention with its own brief. |
| HAS-09 | noted | The stdin half of the new unit test would pass without the explicit null-device assignment because os/exec already gives an unset stdin the null device; the session-leader half discriminates. The code is correct. | No change. |
| HAS-10 | noted | The fold brief's closing line named gate-fence-returns-to-per-landing-proof as the re-promotion goal; the review brief and the goal ledger name goal-cli-fixtures-return-to-per-landing-proof, whose DONE sentence restores exactly what round three removed. | No change in the diff; the landing message names the right goal. |
