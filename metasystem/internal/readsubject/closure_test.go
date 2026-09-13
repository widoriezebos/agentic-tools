package readsubject

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

type closureFixture struct {
	t       *testing.T
	agents  string
	root    map[string]any
	members []map[string]any
	subject ReadSubject
}

func newClosureFixture(t *testing.T) *closureFixture {
	t.Helper()
	subject := ReadSubject{Kind: SubjectLive, ImplementerRoot: "implementer", ReviewedMember: "implementer", ReviewedProjectTree: "tree-a", DiffDigest: "diff-a"}
	root := map[string]any{
		"jobId": "critic", "role": "code-critic", "round": json.Number("1"), "parentJob": nil,
		"status": "completed", "chainClosed": true,
		"findingRegister": []any{}, "findingRegisterRound": json.Number("1"),
		"findingRegisterSubjectDigest": subject.Digest(),
		closureField:                   map[string]any{"criticRoot": "critic", "round": json.Number("1"), "subject": subjectObject(t, subject), "mechanism": "clean"},
	}
	fixture := &closureFixture{t: t, agents: t.TempDir(), root: root, members: []map[string]any{root}, subject: subject}
	fixture.writeRound(1, "critic", subject, map[string]any{"jobId": "critic", "round": json.Number("1"), "reviewedTree": "tree-a"})
	return fixture
}

func subjectObject(t *testing.T, subject ReadSubject) map[string]any {
	t.Helper()
	data, err := json.Marshal(subject)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	return object
}

func (f *closureFixture) writeRound(round int64, job string, subject ReadSubject, result map[string]any) {
	f.t.Helper()
	dir := filepath.Join(f.agents, "critic", "rounds", strconv.FormatInt(round, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatal(err)
	}
	writeJSON(f.t, filepath.Join(dir, "subject.json"), subject)
	writeJSON(f.t, filepath.Join(dir, "return.json"), result)
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *closureFixture) requireValid() Closure {
	f.t.Helper()
	closure, present, err := ReadClosedClosure(f.agents, f.root, f.members)
	if err != nil || !present {
		f.t.Fatalf("valid closure = %+v, %v, %v", closure, present, err)
	}
	return closure
}

func (f *closureFixture) requireRefused() {
	f.t.Helper()
	closure, present, err := ReadClosedClosure(f.agents, f.root, f.members)
	if err == nil {
		f.t.Fatalf("invalid closure accepted: %+v, present=%v", closure, present)
	}
	// A malformed or inconsistent closure that is present in the record never
	// becomes absence: a consumer that read only the flag would take the
	// historical path and land work this gate just refused (design 9c, D1).
	// A record with no closure at all still reports absence with its error.
	_, stored := f.root[closureField]
	if present != stored {
		f.t.Fatalf("refused closure reported present=%v for stored=%v: %+v, %v", present, stored, closure, err)
	}
}

func TestClosureGateRequiresLastCriticRound(t *testing.T) {
	newClosureFixture(t).requireValid()

	for _, status := range []string{"running", "completed", "failed", "cancelled", "timeout"} {
		t.Run("later "+status, func(t *testing.T) {
			fixture := newClosureFixture(t)
			fixture.members = append(fixture.members, map[string]any{
				"jobId": "critic-r2", "role": "code-critic", "round": json.Number("2"),
				"parentJob": "critic", "status": status,
			})
			fixture.requireRefused()
		})
	}

	tests := map[string]func(*closureFixture){
		"round zero":     func(f *closureFixture) { f.members[0]["round"] = json.Number("0") },
		"round fraction": func(f *closureFixture) { f.members[0]["round"] = json.Number("1.0") },
		"round text":     func(f *closureFixture) { f.members[0]["round"] = "1" },
		"tie": func(f *closureFixture) {
			f.members = append(f.members, map[string]any{"jobId": "critic-copy", "role": "code-critic", "round": 1, "parentJob": "critic", "status": "completed"})
		},
		"bad member id": func(f *closureFixture) {
			f.members = append(f.members, map[string]any{"jobId": "Bad Member", "round": 2, "parentJob": "critic", "status": "completed"})
		},
		"wrong root":        func(f *closureFixture) { f.root["jobId"] = "other" },
		"non critic root":   func(f *closureFixture) { f.root["role"] = "implementer" },
		"follow-up root":    func(f *closureFixture) { f.root["parentJob"] = "older" },
		"open chain":        func(f *closureFixture) { f.root["chainClosed"] = false },
		"unknown mechanism": func(f *closureFixture) { f.root[closureField].(map[string]any)["mechanism"] = "certified" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			fixture := newClosureFixture(t)
			mutate(fixture)
			fixture.requireRefused()
		})
	}
}

func TestClosureGateRejectsMalformedEvidence(t *testing.T) {
	tests := map[string]func(*closureFixture){
		"root status": func(f *closureFixture) {
			rootMember := cloneMap(f.root)
			rootMember["status"] = "completed"
			f.members = []map[string]any{rootMember}
			f.root["status"] = "running"
		},
		"duplicate job": func(f *closureFixture) {
			f.members = append(f.members, map[string]any{"jobId": "critic", "round": 2, "parentJob": "critic", "status": "completed"})
		},
		"bad member parent": func(f *closureFixture) {
			f.members = append(f.members, map[string]any{"jobId": "critic-r2", "round": 2, "parentJob": 1, "status": "completed"})
		},
		"bad member round": func(f *closureFixture) {
			f.members = append(f.members, map[string]any{"jobId": "critic-r2", "round": json.Number("2.0"), "parentJob": "critic", "status": "completed"})
		},
		"root missing from membership": func(f *closureFixture) {
			f.members = []map[string]any{{"jobId": "critic-r2", "round": 2, "parentJob": "critic", "status": "completed"}}
		},
		"root member round": func(f *closureFixture) {
			member := cloneMap(f.root)
			member["round"] = 2
			f.members = []map[string]any{member}
		},
		"register absent":      func(f *closureFixture) { delete(f.root, "findingRegister") },
		"register malformed":   func(f *closureFixture) { f.root["findingRegister"] = "register" },
		"folded round invalid": func(f *closureFixture) { f.root["findingRegisterRound"] = json.Number("1.0") },
		"folded digest absent": func(f *closureFixture) { delete(f.root, "findingRegisterSubjectDigest") },
		"subject malformed": func(f *closureFixture) {
			if err := os.WriteFile(filepath.Join(f.agents, "critic", "rounds", "1", "subject.json"), []byte("{"), 0o644); err != nil {
				f.t.Fatal(err)
			}
		},
		"return malformed": func(f *closureFixture) {
			if err := os.WriteFile(filepath.Join(f.agents, "critic", "rounds", "1", "return.json"), []byte("[]"), 0o644); err != nil {
				f.t.Fatal(err)
			}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			fixture := newClosureFixture(t)
			mutate(fixture)
			fixture.requireRefused()
		})
	}
}

func TestClosureGateChecksFoldBinding(t *testing.T) {
	tests := map[string]func(*closureFixture){
		"wrong folded round": func(f *closureFixture) { f.root["findingRegisterRound"] = json.Number("2") },
		"changed subject": func(f *closureFixture) {
			f.root[closureField].(map[string]any)["subject"].(map[string]any)["diffDigest"] = "diff-b"
		},
		"changed digest": func(f *closureFixture) { f.root["findingRegisterSubjectDigest"] = "other" },
		"changed return job": func(f *closureFixture) {
			f.writeRound(1, "critic", f.subject, map[string]any{"jobId": "other", "round": 1, "reviewedTree": "tree-a"})
		},
		"changed return round": func(f *closureFixture) {
			f.writeRound(1, "critic", f.subject, map[string]any{"jobId": "critic", "round": 2, "reviewedTree": "tree-a"})
		},
		"unbound return": func(f *closureFixture) {
			f.writeRound(1, "critic", f.subject, map[string]any{"jobId": "critic", "round": 1, "reviewedTree": "tree-b"})
		},
		"missing subject": func(f *closureFixture) {
			if err := os.Remove(filepath.Join(f.agents, "critic", "rounds", "1", "subject.json")); err != nil {
				f.t.Fatal(err)
			}
		},
		"missing return": func(f *closureFixture) {
			if err := os.Remove(filepath.Join(f.agents, "critic", "rounds", "1", "return.json")); err != nil {
				f.t.Fatal(err)
			}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			fixture := newClosureFixture(t)
			mutate(fixture)
			fixture.requireRefused()
		})
	}
}

func TestClosureGateMissingCleanClosure(t *testing.T) {
	modern := newClosureFixture(t)
	delete(modern.root, closureField)
	modern.requireRefused()

	historical := newClosureFixture(t)
	delete(historical.root, closureField)
	delete(historical.root, "findingRegister")
	delete(historical.root, "findingRegisterRound")
	delete(historical.root, "findingRegisterSubjectDigest")
	if _, present, err := ReadClosedClosure(historical.agents, historical.root, historical.members); err != nil || present {
		t.Fatalf("historical absence = present %v, error %v", present, err)
	}

	for _, pair := range [][2]string{{"resolved", "out-of-scope"}, {"accepted-risk", "accepted-risk"}, {"deferred", "deferred"}} {
		t.Run(pair[0]+" "+pair[1], func(t *testing.T) {
			fixture := newClosureFixture(t)
			fixture.root["findingRegister"] = []any{modernRegisterEntry(pair[0], pair[1])}
			delete(fixture.root, closureField)
			if _, present, err := ReadClosedClosure(fixture.agents, fixture.root, fixture.members); err != nil || present {
				t.Fatalf("lawful non-clean absence = present %v, error %v", present, err)
			}
		})
	}

	lost := newClosureFixture(t)
	delete(lost.root, closureField)
	if err := os.Remove(filepath.Join(lost.agents, "critic", "rounds", "1", "subject.json")); err != nil {
		t.Fatal(err)
	}
	lost.requireRefused()

	nonCleanClosure := newClosureFixture(t)
	nonCleanClosure.root["findingRegister"] = []any{modernRegisterEntry("resolved", "out-of-scope")}
	nonCleanClosure.requireRefused()
}

func TestClosureGateClassifiesAbsentEvidence(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*closureFixture)
		wantErr bool
	}{
		{name: "malformed register", wantErr: true, mutate: func(f *closureFixture) { f.root["findingRegister"] = "register" }},
		{name: "digest without register", wantErr: true, mutate: func(f *closureFixture) { delete(f.root, "findingRegister") }},
		{name: "unfolded historical", mutate: func(f *closureFixture) {
			delete(f.root, "findingRegisterRound")
			delete(f.root, "findingRegisterSubjectDigest")
		}},
		{name: "digest without round", wantErr: true, mutate: func(f *closureFixture) { delete(f.root, "findingRegisterRound") }},
		{name: "invalid folded round", wantErr: true, mutate: func(f *closureFixture) { f.root["findingRegisterRound"] = "1" }},
		{name: "round zero historical", mutate: func(f *closureFixture) {
			f.root["findingRegisterRound"] = 0
			delete(f.root, "findingRegisterSubjectDigest")
		}},
		{name: "round zero digest", wantErr: true, mutate: func(f *closureFixture) { f.root["findingRegisterRound"] = 0 }},
		{name: "invalid digest", wantErr: true, mutate: func(f *closureFixture) { f.root["findingRegisterSubjectDigest"] = 1 }},
		{name: "invalid folded root", wantErr: true, mutate: func(f *closureFixture) {
			f.root["jobId"] = "Bad Root"
			delete(f.root, "findingRegisterSubjectDigest")
		}},
		{name: "unreadable folded subject", wantErr: true, mutate: func(f *closureFixture) {
			delete(f.root, "findingRegisterSubjectDigest")
			if err := os.WriteFile(filepath.Join(f.agents, "critic", "rounds", "1", "subject.json"), []byte("{"), 0o644); err != nil {
				f.t.Fatal(err)
			}
		}},
		{name: "subjectless historical", mutate: func(f *closureFixture) {
			delete(f.root, "findingRegisterSubjectDigest")
			if err := os.Remove(filepath.Join(f.agents, "critic", "rounds", "1", "subject.json")); err != nil {
				f.t.Fatal(err)
			}
		}},
		{name: "non-clean lost subject", wantErr: true, mutate: func(f *closureFixture) {
			f.root["findingRegister"] = []any{modernRegisterEntry("accepted-risk", "accepted-risk")}
			if err := os.Remove(filepath.Join(f.agents, "critic", "rounds", "1", "subject.json")); err != nil {
				f.t.Fatal(err)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newClosureFixture(t)
			delete(fixture.root, closureField)
			test.mutate(fixture)
			_, present, err := ReadClosedClosure(fixture.agents, fixture.root, fixture.members)
			if present || (err != nil) != test.wantErr {
				t.Fatalf("absence = present %v, error %v; want error %v", present, err, test.wantErr)
			}
		})
	}
}

func TestCleanRegisterAndClosureDecoders(t *testing.T) {
	legacyResolved := map[string]any{
		"findingId": "f", "critic": "critic", "rigorClass": "bounded", "factsDigest": "facts",
		"status": "resolved", "evidenceDigest": "evidence", "multiplicity": 1,
	}
	for name, value := range map[string]any{
		"empty":           []any{},
		"legacy resolved": []any{legacyResolved},
		"modern resolved": []any{modernRegisterEntry("resolved", "withdrawn")},
	} {
		if clean, err := CleanRegister(value); err != nil || !clean {
			t.Fatalf("%s = clean %v, error %v", name, clean, err)
		}
	}
	wrongModernFields := modernRegisterEntry("resolved", "withdrawn")
	delete(wrongModernFields, "findingId")
	wrongModernFields["unexpected"] = "field"
	for name, value := range map[string]any{
		"not array":         map[string]any{},
		"not object":        []any{"finding"},
		"wrong fields":      []any{map[string]any{"status": "resolved"}},
		"wrong modern keys": []any{wrongModernFields},
		"status type":       []any{modernRegisterEntryValue(1, "withdrawn")},
		"resolution type":   []any{modernRegisterEntryValue("resolved", 1)},
		"bad pair":          []any{modernRegisterEntry("resolved", "deferred")},
		"legacy resolution": []any{map[string]any{
			"findingId": "f", "critic": "critic", "rigorClass": "bounded", "factsDigest": "facts",
			"status": "resolved", "evidenceDigest": "evidence", "resolution": "withdrawn",
		}},
	} {
		if _, err := CleanRegister(value); err == nil {
			t.Fatalf("%s register was accepted", name)
		}
	}

	subject := subjectObject(t, ReadSubject{Kind: SubjectLive})
	valid := map[string]any{closureField: map[string]any{"criticRoot": "critic", "round": json.Number("1"), "subject": subject, "mechanism": "clean"}}
	if _, present, err := ReadClosure(map[string]any{}); err != nil || present {
		t.Fatalf("absent closure = present %v, error %v", present, err)
	}
	for name, value := range map[string]any{
		"not object":    "closure",
		"bad root":      map[string]any{"criticRoot": "Bad", "round": 1, "subject": subject, "mechanism": "clean"},
		"bad round":     map[string]any{"criticRoot": "critic", "round": json.Number("1.0"), "subject": subject, "mechanism": "clean"},
		"no subject":    map[string]any{"criticRoot": "critic", "round": 1, "mechanism": "clean"},
		"bad subject":   map[string]any{"criticRoot": "critic", "round": 1, "subject": map[string]any{"kind": "bad"}, "mechanism": "clean"},
		"bad mechanism": map[string]any{"criticRoot": "critic", "round": 1, "subject": subject, "mechanism": "other"},
	} {
		root := map[string]any{closureField: value}
		if _, present, err := ReadClosure(root); err == nil || !present {
			t.Fatalf("%s closure = present %v, error %v", name, present, err)
		}
	}
	if closure, present, err := ReadClosure(valid); err != nil || !present || closure.Round != 1 {
		t.Fatalf("valid closure = %+v, %v, %v", closure, present, err)
	}
	for _, round := range []any{float64(1), int64(1), int(1)} {
		valid[closureField].(map[string]any)["round"] = round
		if _, present, err := ReadClosure(valid); err != nil || !present {
			t.Fatalf("integral round %T was refused: present %v, error %v", round, present, err)
		}
	}

	if !ReturnBindsSubject(ReadSubject{Kind: SubjectLive, ReviewedProjectTree: "tree"}, map[string]any{"reviewedTree": "tree"}) ||
		!ReturnBindsSubject(ReadSubject{Kind: SubjectCommit, Tree: "tree"}, map[string]any{"reviewedTree": "tree"}) ||
		!ReturnBindsSubject(ReadSubject{Kind: SubjectDesign, ReviewedCommit: "commit"}, map[string]any{"reviewedCommit": "commit"}) ||
		ReturnBindsSubject(ReadSubject{Kind: "other"}, map[string]any{}) {
		t.Fatal("return binding classification is wrong")
	}
	if ReturnBindsSubject(ReadSubject{Kind: SubjectLive, ReviewedProjectTree: "tree"}, map[string]any{"reviewedTree": 1}) {
		t.Fatal("non-string return binding was accepted")
	}
}

func cloneMap(source map[string]any) map[string]any {
	clone := make(map[string]any, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func modernRegisterEntry(status, resolution string) map[string]any {
	return modernRegisterEntryValue(status, resolution)
}

func modernRegisterEntryValue(status, resolution any) map[string]any {
	return map[string]any{
		"findingId": "f", "critic": "critic", "rigorClass": "bounded", "factsDigest": "facts", "facts": nil,
		"artifact": "file", "title": "title", "status": status, "resolution": resolution, "decisionOpid": "decision",
		"evidence": "evidence", "evidenceDigest": "digest", "multiplicity": 1,
	}
}
