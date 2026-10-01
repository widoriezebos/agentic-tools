# authority-refusals-name-the-command

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="message text on five refusals; no behaviour change"
- Tier: 1
- Intent: What: When the machinery refuses an act because the wrong person or proof tried it, the refusal prints the one command that would have worked. Why: most refusals already do this, but about five still print only an error: a refused grade, an actor that could not be confirmed, a typed name with no proof, resolve-taint, and migrate and repair errors. A person then has to guess what to run. Pros: every refusal becomes something a person can act on straight away. Cons: small; each new refusal message must keep the rule.
- Origin: human
- Next step: Next: give each of the five refusals its runnable command and add a test per refusal that checks the command is printed. Done when: those five refusals print a command that, when run by the right person, succeeds.
- OpenedAt: 2026-09-30T19:06:47Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-30T19:06:47Z HM182WHSB9BZ7KY3XH6VMZD9VV-ui-bc2fda53 open actor=human:Wido targets=authority-refusals-name-the-command
Integrity: sha256=c19714583bece9465ee61684d9073abe7050154844fc8295fcff61ea65e5f0a8
