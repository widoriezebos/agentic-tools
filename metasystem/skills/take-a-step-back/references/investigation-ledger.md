# Investigation Ledger Template

```markdown
# <Investigation>

## Contract
- Symptom and impact:
- Reproduction and exact state:
- Success/non-goals:
- Budget and stop conditions:
- Cycle budget: <number, parsed by scripts/assert-stop-loss.sh>
- No-gain budget: <optional; trailing cycles allowed without a contract-improved, parsed by scripts/assert-stop-loss.sh; improve mode sets 3>

## Existing Evidence
| Artifact | Fact established | Reliability/limits |
| --- | --- | --- |

## Theories
| Id | Theory | Support | Contradiction | Decisive check | Status |
| --- | --- | --- | --- | --- | --- |

## Do Not Retry
| Mechanism | Evidence | Reopen condition |
| --- | --- | --- |

## Cycles
### Cycle C1
- Contract:
- Result:
- Classification:
- Checkpoint/revert:
- Next action:

## Local Learning Memo
- Supported diagnosis:
- Owning boundary and required facts:
- Rejected approaches:
- Design constraints:
```
