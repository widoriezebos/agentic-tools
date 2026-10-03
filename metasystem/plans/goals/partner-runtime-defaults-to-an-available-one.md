# partner-runtime-defaults-to-an-available-one

- State: approved
- Priority: 1
- Sequence: 24
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="Reuses the existing auto-runtime resolver for one more setting; the Partner's read-only contract and sign-in rule are unchanged"
- Tier: 1
- Intent: The Project Partner never refuses for want of a configured runtime (Wido 2026-10-03: the UI said 'no Partner runtime is configured on this seat; set ui.partner.runtime to claude, codex or devin'; 'this must never happen'). With ui.partner.runtime unset, the Partner uses the existing auto runtime resolution (internal/config/autoruntime.go, runtime registry TailoringPriority: claude, then codex, then devin, the first whose executable is on PATH); an explicit setting still wins. Only when none of the three is installed does the UI say so, in plain words naming what to install.
- Origin: main
- Next step: Make an unset ui.partner.runtime resolve through config.AutoRuntime (internal/ui/partner/runtime.go Admit, internal/ui/httpd/partner.go:218, internal/config/ui.go and defaults.go docs); test unset->claude, claude missing->codex, none installed->plain message; check the walkthrough and the UI's Partner panel.
- OpenedAt: 2026-10-03T06:23:34Z
- Revision: 3
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-03T06:23:43Z revision=2 opid=KWRF480M23PPCFAA5VYFHRKWTK-m1e-718ba0eb authority=proven digest=0e3e9d507d2bf83f85d94f495d4bbd9df24763e79a16a6b0b1f19420563ad4cc episode=2

History:
- 2026-10-03T06:23:34Z 1YVD5BX033JAJRM6FXJMW14SC9-m1e-718ba0eb open actor=human:Wido targets=partner-runtime-defaults-to-an-available-one
- 2026-10-03T06:23:43Z KWRF480M23PPCFAA5VYFHRKWTK-m1e-718ba0eb approve actor=human:Wido targets=partner-runtime-defaults-to-an-available-one
- 2026-10-03T06:23:52Z TNRD1T5KKYBWF9QBF9C5467JHR-m1e-718ba0eb set-priority actor=human:Wido targets=partner-runtime-defaults-to-an-available-one reason=priority-order subject=partner-runtime-defaults-to-an-available-one from=unranked to=1:24 requested-sequence=append
Integrity: sha256=cec6e18685ebc2fbe0671b232687f5b9663b60a6cb395b7553daa796de7f562c
