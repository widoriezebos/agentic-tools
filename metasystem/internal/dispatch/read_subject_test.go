package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadSubjectEqualityIgnoresProvenance(t *testing.T) {
	tests := []struct {
		name  string
		left  ReadSubject
		right ReadSubject
		equal bool
	}{
		{
			name:  "live reviewed member",
			left:  ReadSubject{Kind: SubjectLive, ImplementerRoot: "impl", ReviewedMember: "impl", ReviewedProjectTree: "tree", DiffDigest: "diff"},
			right: ReadSubject{Kind: SubjectLive, ImplementerRoot: "impl", ReviewedMember: "impl-r2", ReviewedProjectTree: "tree", DiffDigest: "diff"},
			equal: true,
		},
		{
			name:  "live reviewed project tree",
			left:  ReadSubject{Kind: SubjectLive, ImplementerRoot: "impl", ReviewedProjectTree: "tree-a", DiffDigest: "diff"},
			right: ReadSubject{Kind: SubjectLive, ImplementerRoot: "impl", ReviewedProjectTree: "tree-b", DiffDigest: "diff"},
		},
		{
			name:  "live diff digest",
			left:  ReadSubject{Kind: SubjectLive, ImplementerRoot: "impl", ReviewedProjectTree: "tree", DiffDigest: "diff-a"},
			right: ReadSubject{Kind: SubjectLive, ImplementerRoot: "impl", ReviewedProjectTree: "tree", DiffDigest: "diff-b"},
		},
		{
			name:  "design reviewed commit",
			left:  ReadSubject{Kind: SubjectDesign, DesignPath: "plans/design.md", ContentDigest: "content", DeclaredOutputsDigest: "outputs", ReviewedCommit: "commit-a"},
			right: ReadSubject{Kind: SubjectDesign, DesignPath: "plans/design.md", ContentDigest: "content", DeclaredOutputsDigest: "outputs", ReviewedCommit: "commit-b"},
			equal: true,
		},
		{
			name:  "commit parent",
			left:  ReadSubject{Kind: SubjectCommit, Commit: "commit", Parent: "parent-a", Tree: "tree", DiffDigest: "diff"},
			right: ReadSubject{Kind: SubjectCommit, Commit: "commit", Parent: "parent-b", Tree: "tree", DiffDigest: "diff"},
			equal: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.left.Equal(test.right); got != test.equal {
				t.Fatalf("Equal() = %v, want %v", got, test.equal)
			}
			if got := test.left.Digest() == test.right.Digest(); got != test.equal {
				t.Fatalf("digest equality = %v, want %v", got, test.equal)
			}
		})
	}
}

func TestReadSubjectKindsNeverEqual(t *testing.T) {
	fields := ReadSubject{ImplementerRoot: "same", ReviewedProjectTree: "same", DiffDigest: "same", DesignPath: "same", ContentDigest: "same", DeclaredOutputsDigest: "same", Commit: "same", Tree: "same"}
	subjects := []ReadSubject{fields, fields, fields}
	subjects[0].Kind = SubjectLive
	subjects[1].Kind = SubjectDesign
	subjects[2].Kind = SubjectCommit
	for i := range subjects {
		for j := range subjects {
			if i != j && subjects[i].Equal(subjects[j]) {
				t.Fatalf("%q subject equals %q subject", subjects[i].Kind, subjects[j].Kind)
			}
		}
	}
}

func TestReadRoundSubjectAbsentOnOldRounds(t *testing.T) {
	agents := filepath.Join(t.TempDir(), "agents")
	roundDir := filepath.Join(agents, "critic", "rounds", "1")
	if err := os.MkdirAll(roundDir, 0o755); err != nil {
		t.Fatal(err)
	}

	subject, present, err := readRoundSubject(agents, "critic", 1)
	if err != nil || present || subject != (ReadSubject{}) {
		t.Fatalf("absent subject = %+v, %v, %v; want zero, false, nil", subject, present, err)
	}
	if err := os.WriteFile(filepath.Join(roundDir, "subject.json"), []byte("not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readRoundSubject(agents, "critic", 1); err == nil {
		t.Fatal("malformed subject.json was accepted")
	}
}

func TestReadClosureAbsentOnOldRoots(t *testing.T) {
	closure, present, err := ReadClosure(map[string]any{"jobId": "critic"})
	if err != nil || present || closure != (Closure{}) {
		t.Fatalf("absent closure = %+v, %v, %v; want zero, false, nil", closure, present, err)
	}

	_, present, err = ReadClosure(map[string]any{
		closureField: map[string]any{
			"criticRoot": "critic", "round": 1,
			"subject": map[string]any{"kind": "live"}, "mechanism": "certified",
		},
	})
	if err == nil || !present {
		t.Fatalf("unknown mechanism = present %v, error %v; want present with error", present, err)
	}
}

func TestLoadReadRefusalsMissingAndDeduplicated(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing.jsonl")
	first := filepath.Join(root, "first.jsonl")
	second := filepath.Join(root, "second.jsonl")
	malformed := filepath.Join(root, "malformed.jsonl")
	if err := os.WriteFile(first, []byte("\n"+
		`{"id":"event-1","reason":"first","role":"code-critic","criticRoot":"critic","round":1,"subject":{"kind":"live"},"refusedAt":"time-1"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte(
		`{"id":"event-1","reason":"duplicate","role":"code-critic","criticRoot":"other","round":2,"subject":{"kind":"live"},"refusedAt":"time-2"}`+"\n"+
			`{"id":"event-2","reason":"second","role":"design-critic","criticRoot":"critic","round":2,"subject":{"kind":"design"},"refusedAt":"time-3"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	refusals, err := LoadReadRefusals(missing, first, second)
	if err != nil {
		t.Fatal(err)
	}
	if len(refusals) != 2 || refusals[0].ID != "event-1" || refusals[0].Reason != "first" || refusals[1].ID != "event-2" {
		t.Fatalf("deduplicated refusals = %+v", refusals)
	}
	if err := os.WriteFile(malformed, []byte("\n{broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReadRefusals(malformed); err == nil || !strings.Contains(err.Error(), malformed) || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("malformed line error = %v; want path and line 2", err)
	}
}

func TestSliceZeroEmitsNothing(t *testing.T) {
	repo := t.TempDir()
	writeCriticRound(t, repo, "critic", "critic", 1, []any{}, []any{})
	setCriticSubject(t, repo, "critic", "implementer", "reviewed-tree")
	if outcome, err := CritiqueRegisterAdvance(repo, "critic", "critic"); err != nil || outcome != "advanced" {
		t.Fatalf("advance = %q, %v", outcome, err)
	}
	if outcome, err := CritiqueRegisterClose(repo, "critic"); err != nil || outcome != "closed" {
		t.Fatalf("close = %q, %v", outcome, err)
	}

	root := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json"))
	if _, present := root[closureField]; present {
		t.Fatalf("slice zero wrote closure: %v", root[closureField])
	}
	for _, path := range []string{
		filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "1", "subject.json"),
		filepath.Join(repo, "artifacts", "agents", "critic", "reads-refused.jsonl"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("slice zero emitted %s: %v", path, err)
		}
	}
}

// The accept side of the reader contract slice 1 relies on: every identity
// field of the design and commit kinds participates, kinds never share a
// digest, a valid closure and a valid round subject decode, and a closure
// with a bad round or critic root is refused (Opus read of slice 0, F-1).
func TestReadSubjectIdentityFieldsAndDigestsPerKind(t *testing.T) {
	design := ReadSubject{Kind: SubjectDesign, DesignPath: "plans/d.md", ContentDigest: "c", DeclaredOutputsDigest: "o"}
	for name, changed := range map[string]ReadSubject{
		"design path":             {Kind: SubjectDesign, DesignPath: "plans/e.md", ContentDigest: "c", DeclaredOutputsDigest: "o"},
		"design content":          {Kind: SubjectDesign, DesignPath: "plans/d.md", ContentDigest: "x", DeclaredOutputsDigest: "o"},
		"design declared outputs": {Kind: SubjectDesign, DesignPath: "plans/d.md", ContentDigest: "c", DeclaredOutputsDigest: "x"},
	} {
		if design.Equal(changed) || design.Digest() == changed.Digest() {
			t.Fatalf("%s: a changed identity field compared equal", name)
		}
	}
	commit := ReadSubject{Kind: SubjectCommit, Commit: "k", Tree: "t", DiffDigest: "d"}
	for name, changed := range map[string]ReadSubject{
		"commit":      {Kind: SubjectCommit, Commit: "x", Tree: "t", DiffDigest: "d"},
		"commit tree": {Kind: SubjectCommit, Commit: "k", Tree: "x", DiffDigest: "d"},
		"commit diff": {Kind: SubjectCommit, Commit: "k", Tree: "t", DiffDigest: "x"},
	} {
		if commit.Equal(changed) || commit.Digest() == changed.Digest() {
			t.Fatalf("%s: a changed identity field compared equal", name)
		}
	}
	live := ReadSubject{Kind: SubjectLive, ImplementerRoot: "a", ReviewedProjectTree: "b", DiffDigest: "c"}
	liveAsCommit := ReadSubject{Kind: SubjectCommit, Commit: "a", Tree: "b", DiffDigest: "c"}
	liveAsDesign := ReadSubject{Kind: SubjectDesign, DesignPath: "a", ContentDigest: "b", DeclaredOutputsDigest: "c"}
	if live.Digest() == liveAsCommit.Digest() || live.Digest() == liveAsDesign.Digest() || liveAsCommit.Digest() == liveAsDesign.Digest() {
		t.Fatal("subjects of different kinds with the same field values share a digest")
	}
	if live.Digest() != (ReadSubject{Kind: SubjectLive, ImplementerRoot: "a", ReviewedMember: "m", ReviewedProjectTree: "b", DiffDigest: "c"}).Digest() {
		t.Fatal("a provenance field changed the digest")
	}
}

func TestReadClosureAcceptsAValidClosureAndRefusesBadCoordinates(t *testing.T) {
	subject := map[string]any{"kind": "live", "implementerRoot": "impl", "reviewedProjectTree": "tree", "diffDigest": "diff"}
	valid := map[string]any{closureField: map[string]any{"criticRoot": "critic-root", "round": float64(2), "subject": subject, "mechanism": "clean"}}
	closure, present, err := ReadClosure(valid)
	if err != nil || !present {
		t.Fatalf("a valid closure was refused: present=%v err=%v", present, err)
	}
	want := ReadSubject{Kind: SubjectLive, ImplementerRoot: "impl", ReviewedProjectTree: "tree", DiffDigest: "diff"}
	if closure.CriticRoot != "critic-root" || closure.Round != 2 || closure.Mechanism != "clean" || !closure.Subject.Equal(want) {
		t.Fatalf("closure decoded wrong: %+v", closure)
	}
	for name, mutate := range map[string]func(map[string]any){
		"round zero":          func(c map[string]any) { c["round"] = float64(0) },
		"round negative":      func(c map[string]any) { c["round"] = float64(-1) },
		"round as text":       func(c map[string]any) { c["round"] = "2" },
		"critic root invalid": func(c map[string]any) { c["criticRoot"] = "Critic Root" },
		"critic root absent":  func(c map[string]any) { delete(c, "criticRoot") },
		"subject absent":      func(c map[string]any) { delete(c, "subject") },
	} {
		c := map[string]any{"criticRoot": "critic-root", "round": float64(2), "subject": subject, "mechanism": "clean"}
		mutate(c)
		if _, _, err := ReadClosure(map[string]any{closureField: c}); err == nil {
			t.Fatalf("%s: the closure was accepted", name)
		}
	}
}

func TestReadRoundSubjectDecodesAValidFile(t *testing.T) {
	agents := t.TempDir()
	dir := filepath.Join(agents, "critic-root", "rounds", "3")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "subject.json"), []byte(`{"kind":"commit","commit":"k","parent":"p","tree":"t","diffDigest":"d"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	subject, present, err := readRoundSubject(agents, "critic-root", 3)
	if err != nil || !present {
		t.Fatalf("a valid subject.json was not read: present=%v err=%v", present, err)
	}
	if !subject.Equal(ReadSubject{Kind: SubjectCommit, Commit: "k", Tree: "t", DiffDigest: "d"}) || subject.Parent != "p" {
		t.Fatalf("subject decoded wrong: %+v", subject)
	}
}
