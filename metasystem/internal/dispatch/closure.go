package dispatch

import "fmt"

const closureField = "closure"

type Closure struct {
	CriticRoot string      `json:"criticRoot"`
	Round      int64       `json:"round"`
	Subject    ReadSubject `json:"subject"`
	Mechanism  string      `json:"mechanism"`
}

func ReadClosure(root map[string]any) (Closure, bool, error) {
	value, present := root[closureField]
	if !present {
		return Closure{}, false, nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return Closure{}, true, fmt.Errorf("closure must be an object")
	}

	criticRoot, ok := object["criticRoot"].(string)
	if !ok || !validJobID.MatchString(criticRoot) {
		return Closure{}, true, fmt.Errorf("closure criticRoot %q is not a valid job identifier", criticRoot)
	}
	round, ok := numInt(object["round"])
	if !ok || round < 1 {
		return Closure{}, true, fmt.Errorf("closure round must be a positive integer")
	}
	subject, subjectPresent, err := decodeReadSubject(object["subject"])
	if err != nil {
		return Closure{}, true, fmt.Errorf("closure subject: %w", err)
	}
	if !subjectPresent {
		return Closure{}, true, fmt.Errorf("closure subject is missing")
	}
	mechanism, ok := object["mechanism"].(string)
	if !ok || mechanism != "clean" {
		return Closure{}, true, fmt.Errorf("closure mechanism %q is unknown", mechanism)
	}

	return Closure{
		CriticRoot: criticRoot,
		Round:      round,
		Subject:    subject,
		Mechanism:  mechanism,
	}, true, nil
}
