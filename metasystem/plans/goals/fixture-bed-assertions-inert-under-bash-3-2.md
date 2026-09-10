# fixture-bed-assertions-inert-under-bash-3-2

- State: queued
- Risk: severity=3 novelty=1 exposure=3 accumulation=2 basis="severity 3: every fixture bed's bare assertions cannot fail a leg on this host, so green runs prove less than the beds claim and a real regression in a secondary invariant lands unseen; novelty 1: the fix is a known shell idiom or a lint; exposure 3: every bed on every macOS host whose bash is the stock 3.2, which is every fleet Mac today; accumulation 2: main's land bed alone carries 17 such lines and the carried receipt-drift change adds 28 in the same form"
- Tier: 3
- Intent: Every assertion in a fixture bed stops the leg when it is false, on every host the fleet runs beds on. Found by the round-2 read of the carried receipt-drift chain (lrsrd-carry-cc2-20260910, findings LRD2-01 and LRD2-02) and reproduced by the coordinator on the m1 Mac: bash resolves to /bin/bash 3.2.57 there, and under set -euo pipefail a false bare [[ ]] line does not stop the script, so a bed's bare [[ ... ]] assertion lines are inert (probe: /bin/bash -c 'set -euo pipefail; [[ 1 == 2 ]]; echo reached' prints reached and exits 0). The land bed on main carries 17 such lines, the receipt-drift change adds 28 in the same form, and the other beds have not been counted. What a green bed run proves today on such a host is only its exit-status, grep and cmp checks. DONE means a bed's false assertion fails the leg on bash 3.2 and on bash 5, proven by a fixture that runs a deliberately false assertion under the stock shell, and the form is enforced mechanically (a stopping tail on each assertion, a lint over *-fixtures.sh that refuses bare [[ ]] lines, or a declared bash floor the beds check at start) so it cannot regress.
- Origin: main
- Next step: INTENT: make the beds' assertions honest on the shells the fleet actually has. CONSTRAINTS: no bed's existing checks get weaker; the fix must be provable by a fixture that shows a false assertion failing a leg under /bin/bash 3.2; count every bed's bare assertions first (grep over scripts/agents/*-fixtures.sh for lines that are only a [[ ... ]] test) and record the number in the design. FREEDOMS: choose among a stopping tail on each line, a lint in the dependency-ratchet or validation section, or a declared bash floor; a lint that refuses the bare form is the smallest robust answer if a mechanical rewrite of the existing lines is cheap. Evidence: the critic's return under artifacts/agents/lrsrd-carry-cc2-20260910 on m1e and the dispositions record plans/dispositions/landing-receipt-survives-records-drift-code-critique-r2.md.
- OpenedAt: 2026-09-10T10:51:44Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T10:51:44Z 26RTV08VN0HJZX2S27DG6TNM34-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=fixture-bed-assertions-inert-under-bash-3-2
Integrity: sha256=f0df15a7c7ac8628846f69a7f87baa8b5e17a01d55cf6290bd5a054627863c35
