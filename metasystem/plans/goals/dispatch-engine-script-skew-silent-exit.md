# dispatch-engine-script-skew-silent-exit

- State: queued
- Tier: 1
- Intent: scripts/agents/dispatch.sh dies silently (bare exit 1 under set -e, no message, the delegate wrapper reports only 'exit status 1') when the checkout's dispatch.sh is newer than the built engine and reads a roster field the engine does not emit: on m2 2026-09-03 22:4x, after a pull that brought the model-alias landing (2c3776b8), json_value "$roster_json" aliasedFrom failed because bin/metasystem was still the pre-alias build, and three fresh dispatches were refused with no reason until a bash trace found the line. DONE means the dispatcher refuses LOUDLY when the engine's build stamp is behind the checkout's scripts (one preflight comparing the engine's commit stamp with the script tree, naming go-build.sh as the remedy), and json_value on a missing field names the field instead of exiting bare.
- Origin: main
- Next step: TIER 1 per R-54-m1 (a message and a preflight check in an existing script): build, run dispatch-fixtures.sh, land as a declared direct fix; box 1h/3/60m/1. Waits for human approval for execution.
- OpenedAt: 2026-09-03T20:24:20Z
- Revision: 2
- Labels: robustness

History:
- 2026-09-03T20:24:20Z 4F8P5HKWQHZXCT3KXMMX4T04NY-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=dispatch-engine-script-skew-silent-exit
- 2026-09-04T06:14:46Z WVTVAX6E4VTM6GM0VBJMR6RJ24-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=dispatch-engine-script-skew-silent-exit
Integrity: sha256=8b46e2a5a2528ad65c8b8480928d8bfd0a28c17c3eb146a0b57ee3cca2f5f9be
