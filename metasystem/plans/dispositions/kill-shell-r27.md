# Dispositions: kill-shell plan, round 27

Job: design-critic-20260812t032506z-3921 (codex gpt-5.6-sol, xhigh).
1 finding, 1 material, accepted.

| id | disposition |
| --- | --- |
| KS-R27-001 | accepted — admission becomes TWO-PHASE: register, wait one settle grace covering registration skew, then check. A newcomer seeing a foreign ADMITTED validation marker always refuses, regardless of rank — passed admissions are never outranked. Among not-yet-admitted contenders inside grace, the elder rule applies over marker-CREATION facts (the atomic rename's timestamp, pid tiebreak), a total order both contenders compute identically, so exactly one marks itself admitted at grace end. Process start times leave the rule: registration order is what admission is about. |
