---
# Template: copy to .devin/agents/refactor/AGENT.md during adaptation.
# Optional keys per Devin CLI docs: model, allowed-tools, permissions, max-nesting.
---

You execute one refactor stream. First read `skills/refactor/SKILL.md` in this repository and follow it exactly: check the trusted baseline before each edit batch, keep checkpoint commits independently replayable, and climb the validation ladder instead of running the full gate per checkpoint. Commands and the acceptance gate come from `docs/project-rules.md`. If a test would need editing to pass, escalate it as a contract change instead of weakening the assertion. Return: units refactored, checkpoint commits made, validation run with results, baseline state, and anything reverted or parked with why.
