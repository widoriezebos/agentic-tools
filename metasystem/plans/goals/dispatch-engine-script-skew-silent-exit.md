# dispatch-engine-script-skew-silent-exit

- State: queued
- Intent: scripts/agents/dispatch.sh dies silently (bare exit 1 under set -e, no message, the delegate wrapper reports only 'exit status 1') when the checkout's dispatch.sh is newer than the built engine and reads a roster field the engine does not emit: on m2 2026-09-03 22:4x, after a pull that brought the model-alias landing (2c3776b8), json_value "$roster_json" aliasedFrom failed because bin/metasystem was still the pre-alias build, and three fresh dispatches were refused with no reason until a bash trace found the line. DONE means the dispatcher refuses LOUDLY when the engine's build stamp is behind the checkout's scripts (one preflight comparing the engine's commit stamp with the script tree, naming go-build.sh as the remedy), and json_value on a missing field names the field instead of exiting bare.
- Origin: main
- Next step: TIER 1 per R-54-m1 (a message and a preflight check in an existing script): build, run dispatch-fixtures.sh, land as a declared direct fix; box 1h/3/60m/1. Waits for human approval for execution.
- OpenedAt: 2026-09-03T20:24:20Z
- Revision: 1
- Labels: robustness

History:
- 2026-09-03T20:24:20Z 4F8P5HKWQHZXCT3KXMMX4T04NY-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=dispatch-engine-script-skew-silent-exit
Integrity: sha256=24d2b74385002da4204ea71e44ef53d3bb233a2e2bf5431c9113c41b73f412e5
