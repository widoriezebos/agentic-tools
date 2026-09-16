package report

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJobFactsCarryCritiqueCycleAndOmitAbsentFields(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "artifacts/agents/jobs/critic-a.json", `{"jobId":"critic-a","status":"completed","role":"design-critic","reviewRoundLimit":2,"findingRegisterRound":2,"materialByRound":[{"round":1,"material":3},{"round":2,"material":1}],"findingRegister":[{"grain":"mechanical","status":"open"},{"grain":"invariant","status":"resolved"}]}`)
	writeFile(t, root, "artifacts/agents/jobs/critic-b.json", `{"jobId":"critic-b","status":"running","role":"design-critic"}`)
	writeFile(t, root, "artifacts/agents/jobs/critic-closed.json", `{"jobId":"critic-closed","status":"completed","role":"design-critic","reviewRoundLimit":2,"findingRegisterRound":2,"materialByRound":[{"round":1,"material":2},{"round":2,"material":0}],"chainClosed":true}`)
	facts, _, unavailable := jobFacts(root, scanProber{}, map[string]bool{"running": true}, nil)
	if len(unavailable) != 0 || len(facts) != 2 {
		t.Fatalf("job facts = %+v, unavailable=%v", facts, unavailable)
	}
	for _, fact := range facts {
		if fact.Id == "critic-closed" {
			t.Fatalf("closed design critic was admitted: %+v", fact)
		}
	}
	got := facts[0]
	if got.ReviewRoundLimit != 2 || got.CritiqueRound != 2 || got.CritiqueMaterial != 1 || !got.CritiqueMechanical || !got.CritiqueFalling {
		t.Fatalf("critique cycle = %+v", got)
	}
	plain, err := json.Marshal(facts[1])
	if err != nil || strings.Contains(string(plain), `"critiqueRound"`) || facts[1].ReviewRoundLimit != 0 || facts[1].CritiqueMechanical || facts[1].CritiqueFalling {
		t.Fatalf("plain fact leaked critique fields: %s, fact=%+v, err=%v", plain, facts[1], err)
	}
}

func TestJobFactsRequireEveryOpenFindingMechanical(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "artifacts/agents/jobs/critic-mixed.json", `{"jobId":"critic-mixed","status":"completed","role":"design-critic","reviewRoundLimit":2,"findingRegisterRound":2,"materialByRound":[{"round":1,"material":3},{"round":2,"material":1}],"findingRegister":[{"grain":"mechanical","status":"open"},{"grain":"invariant","status":"open"}]}`)
	facts, _, _ := jobFacts(root, scanProber{}, nil, nil)
	if len(facts) != 1 || facts[0].CritiqueRound != 2 || facts[0].CritiqueMaterial != 1 || facts[0].CritiqueMechanical {
		t.Fatalf("mixed critique cycle = %+v", facts)
	}
}
