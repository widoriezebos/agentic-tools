# app-launch-contract-follow-ups

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="Small corrections inside a landed package on a developer host: low harm (severity 1), known shapes (novelty 1), one human on loopback (exposure 1), a fresh package (accumulation 1)."
- Tier: 1
- Intent: What: Four follow-ups to the app launch contract, which is how the metasystem starts and stops an adopted app. Why: First, the launcher stops its direct child process through a handle it took earlier, instead of re-checking that the process is still the one it started. Second, when a start fails while a supervisor still owns a leftover process, the refusal hint describes the state wrongly. Third, a run at another commit does not say which version of the app's launch settings it used, and the design has not decided which one it should use. Fourth, no end-to-end check runs a real app against a real test group. Pros: No risk of stopping the wrong process, honest messages, a clear rule for which settings apply, and finally proof of the whole path. Cons: None of these has hurt yet; the end-to-end check cannot start until an adopted-app test bed exists, and none does today.
- Origin: human
- Next step: Next: Do the first three items in order, each with a test that fails first; leave the end-to-end check until an adopted-app test bed exists (or split it off as its own parked goal). Done when: tests for the re-checked stop, the corrected refusal hint and the named launch settings pass on main.
- OpenedAt: 2026-09-28T13:29:57Z
- Revision: 2
- Labels: launch
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-28T13:29:57Z CQVY18FPY81Z7RZFRY9PHF6J8C-ui-966d857e open actor=human:Wido targets=app-launch-contract-follow-ups
- 2026-09-30T18:46:17Z BZ3ZMZ49V9VWEV91XPJ51KF5CQ-ui-bc2fda53 edit actor=human:Wido targets=app-launch-contract-follow-ups
Integrity: sha256=a8966c360cb5fd79a751ed1e24445c27b8a05e25c75e2758a883c90693914368
