# partner-runtime-defaults-to-an-available-one

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="Reuses the existing auto-runtime resolver for one more setting; the Partner's read-only contract and sign-in rule are unchanged"
- Tier: 1
- Intent: The Project Partner never refuses for want of a configured runtime (Wido 2026-10-03: the UI said 'no Partner runtime is configured on this seat; set ui.partner.runtime to claude, codex or devin'; 'this must never happen'). With ui.partner.runtime unset, the Partner uses the existing auto runtime resolution (internal/config/autoruntime.go, runtime registry TailoringPriority: claude, then codex, then devin, the first whose executable is on PATH); an explicit setting still wins. Only when none of the three is installed does the UI say so, in plain words naming what to install.
- Origin: main
- Next step: Make an unset ui.partner.runtime resolve through config.AutoRuntime (internal/ui/partner/runtime.go Admit, internal/ui/httpd/partner.go:218, internal/config/ui.go and defaults.go docs); test unset->claude, claude missing->codex, none installed->plain message; check the walkthrough and the UI's Partner panel.
- OpenedAt: 2026-10-03T06:23:34Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-10-03T06:23:34Z 1YVD5BX033JAJRM6FXJMW14SC9-m1e-718ba0eb open actor=human:Wido targets=partner-runtime-defaults-to-an-available-one
Integrity: sha256=ce2c802c36dff27c7cd5731ab950654b7865f3e263c08bdabdecced7df8c9c50
