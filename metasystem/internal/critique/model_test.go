package critique

import "testing"

func lawfulFacts() EvidenceFacts {
	return EvidenceFacts{Local: true, Recoverable: true}
}

func wireFacts() map[string]any {
	return map[string]any{
		"local": true, "recoverable": true,
		"proofBoundaryCrossed": false, "authorityBoundaryCrossed": false,
		"secretsBoundaryCrossed": false, "irreversibleDataBoundaryCrossed": false,
		"externalSideEffectBoundaryCrossed": false,
	}
}

func TestNormalizeLawfulClasses(t *testing.T) {
	for _, class := range []RigorClass{Severe, Bounded, Unproven} {
		if got := Normalize(class, lawfulFacts(), true, false); got != class {
			t.Fatalf("Normalize(%q) = %q", class, got)
		}
	}
	if Bounded.FailsClosed() || !Severe.FailsClosed() || !Unproven.FailsClosed() {
		t.Fatal("unproven and severe must fail closed while bounded does not")
	}
}

func TestNormalizeFailsClosed(t *testing.T) {
	if got := Normalize(Bounded, lawfulFacts(), false, false); got != Unproven {
		t.Fatalf("malformed bounded declaration normalized to %q", got)
	}
	if got := Normalize(RigorClass(""), lawfulFacts(), true, false); got != Unproven {
		t.Fatalf("missing class normalized to %q", got)
	}
	if got := Normalize(RigorClass("invented"), lawfulFacts(), true, false); got != Unproven {
		t.Fatalf("unknown class normalized to %q", got)
	}
}

func TestNormalizeRecurrenceToUnproven(t *testing.T) {
	if got := Normalize(Bounded, lawfulFacts(), true, true); got != Unproven {
		t.Fatalf("recurrent bounded finding normalized to %q", got)
	}
}

func TestNormalizeUnknownDangerousClassStaysUnproven(t *testing.T) {
	dangerous := lawfulFacts()
	dangerous.AuthorityBoundaryCrossed = true
	if got := Normalize(RigorClass("invented"), dangerous, true, false); got != Unproven {
		t.Fatalf("unknown class with dangerous facts normalized to %q", got)
	}
}

func TestNormalizeDangerousFactsToSevere(t *testing.T) {
	tests := map[string]func(*EvidenceFacts){
		"non-local":            func(f *EvidenceFacts) { f.Local = false },
		"not recoverable":      func(f *EvidenceFacts) { f.Recoverable = false },
		"proof boundary":       func(f *EvidenceFacts) { f.ProofBoundaryCrossed = true },
		"authority boundary":   func(f *EvidenceFacts) { f.AuthorityBoundaryCrossed = true },
		"secrets boundary":     func(f *EvidenceFacts) { f.SecretsBoundaryCrossed = true },
		"irreversible data":    func(f *EvidenceFacts) { f.IrreversibleDataBoundaryCrossed = true },
		"external side effect": func(f *EvidenceFacts) { f.ExternalSideEffectBoundaryCrossed = true },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			facts := lawfulFacts()
			mutate(&facts)
			if got := Normalize(Bounded, facts, true, false); got != Severe {
				t.Fatalf("dangerous bounded finding normalized to %q", got)
			}
		})
	}
}

func TestNormalizeWireRequiresCompleteFactsAndTrigger(t *testing.T) {
	if got := NormalizeWire("bounded", wireFacts(), "reopen if it recurs", false); got != Bounded {
		t.Fatalf("lawful wire declaration normalized to %q", got)
	}
	missing := wireFacts()
	delete(missing, "local")
	if got := NormalizeWire("bounded", missing, "reopen if it recurs", false); got != Unproven {
		t.Fatalf("missing facts normalized to %q", got)
	}
	extra := wireFacts()
	extra["invented"] = false
	if got := NormalizeWire("bounded", extra, "reopen if it recurs", false); got != Unproven {
		t.Fatalf("extra facts normalized to %q", got)
	}
	if got := NormalizeWire("bounded", wireFacts(), "  ", false); got != Unproven {
		t.Fatalf("blank reopening trigger normalized to %q", got)
	}
	dangerous := wireFacts()
	dangerous["authorityBoundaryCrossed"] = true
	if got := NormalizeWire("invented", dangerous, "reopen when classified", false); got != Unproven {
		t.Fatalf("unknown wire class with dangerous facts normalized to %q", got)
	}
}
