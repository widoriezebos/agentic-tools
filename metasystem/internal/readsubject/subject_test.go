package readsubject

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadSubjectWireAndDigestCompatibility(t *testing.T) {
	tests := []struct {
		name       string
		subject    ReadSubject
		wire       string
		digest     string
		provenance func(ReadSubject) ReadSubject
	}{
		{
			name:    "live",
			subject: ReadSubject{Kind: SubjectLive, ImplementerRoot: `impl<&>`, ReviewedMember: "impl-r1", ReviewedProjectTree: "tree", DiffDigest: "diff"},
			wire:    `{"kind":"live","implementerRoot":"impl\u003c\u0026\u003e","reviewedMember":"impl-r1","reviewedProjectTree":"tree","diffDigest":"diff"}`,
			digest:  "aaf9bec2d37a42eaf870f4ee9aaaacec2840c5714039ba20011b9987ae616995",
			provenance: func(subject ReadSubject) ReadSubject {
				subject.ReviewedMember = "impl-r2"
				return subject
			},
		},
		{
			name:    "design",
			subject: ReadSubject{Kind: SubjectDesign, DesignPath: `metasystem/plans/<design>.md`, ContentDigest: "content", DeclaredOutputsDigest: "outputs", ReviewedCommit: "commit-a"},
			wire:    `{"kind":"design","designPath":"metasystem/plans/\u003cdesign\u003e.md","contentDigest":"content","declaredOutputsDigest":"outputs","reviewedCommit":"commit-a"}`,
			digest:  "6f98044a77557a2c881c161169d5ae9060b8d65f9367b52558c3928c60054644",
			provenance: func(subject ReadSubject) ReadSubject {
				subject.ReviewedCommit = "commit-b"
				return subject
			},
		},
		{
			name:    "commit",
			subject: ReadSubject{Kind: SubjectCommit, Commit: "commit", Parent: "parent-a", Tree: `tree<&>`, DiffDigest: "diff"},
			wire:    `{"kind":"commit","diffDigest":"diff","commit":"commit","parent":"parent-a","tree":"tree\u003c\u0026\u003e"}`,
			digest:  "ea400e4e8b8157a92e07e9e727cd313c00c77418c8af4ff3c3c4b2c7c30ef049",
			provenance: func(subject ReadSubject) ReadSubject {
				subject.Parent = "parent-b"
				return subject
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(test.subject)
			if err != nil {
				t.Fatal(err)
			}
			if string(encoded) != test.wire {
				t.Fatalf("wire = %s, want %s", encoded, test.wire)
			}
			if got := test.subject.Digest(); got != test.digest {
				t.Fatalf("digest = %s, want %s", got, test.digest)
			}
			changed := test.provenance(test.subject)
			if !test.subject.Equal(changed) || test.subject.Digest() != changed.Digest() {
				t.Fatal("provenance-only change altered identity")
			}
			var value any
			if err := json.Unmarshal(encoded, &value); err != nil {
				t.Fatal(err)
			}
			decoded, present, err := DecodeReadSubject(value)
			if err != nil || !present || decoded != test.subject {
				t.Fatalf("decoded = %+v, %v, %v", decoded, present, err)
			}
		})
	}

	if (ReadSubject{Kind: "unknown"}).Equal(ReadSubject{Kind: "unknown"}) {
		t.Fatal("unknown subjects compare equal")
	}
	if (ReadSubject{Kind: "unknown"}).Digest() == "" {
		t.Fatal("unknown subject digest is empty")
	}
	if (ReadSubject{Kind: SubjectLive, ImplementerRoot: "a", ReviewedProjectTree: "b", DiffDigest: "c"}).Equal(
		ReadSubject{Kind: SubjectCommit, Commit: "a", Tree: "b", DiffDigest: "c"}) {
		t.Fatal("different subject kinds compare equal")
	}
}

func TestReadSubjectReadersRejectMalformedEvidence(t *testing.T) {
	if subject, present, err := DecodeReadSubject(nil); err != nil || present || subject != (ReadSubject{}) {
		t.Fatalf("nil subject = %+v, %v, %v", subject, present, err)
	}
	for name, value := range map[string]any{
		"not object": []any{},
		"bad field":  map[string]any{"kind": "live", "implementerRoot": make(chan int)},
		"bad type":   map[string]any{"kind": "live", "implementerRoot": 7},
		"bad kind":   map[string]any{"kind": "other"},
	} {
		if _, _, err := DecodeReadSubject(value); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}

	agents := t.TempDir()
	if _, _, err := ReadRoundSubject(agents, "Bad Root", 1); err == nil {
		t.Fatal("invalid root was accepted")
	}
	if _, _, err := ReadRoundSubject(agents, "critic", 0); err == nil {
		t.Fatal("invalid round was accepted")
	}
	if subject, present, err := ReadRoundSubject(agents, "critic", 1); err != nil || present || subject != (ReadSubject{}) {
		t.Fatalf("missing subject = %+v, %v, %v", subject, present, err)
	}
	roundDir := filepath.Join(agents, "critic", "rounds", "1")
	if err := os.MkdirAll(roundDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(roundDir, "subject.json")
	for name, data := range map[string]string{
		"malformed": "{",
		"array":     "[]",
	} {
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := ReadRoundSubject(agents, "critic", 1); err == nil {
			t.Fatalf("%s subject document was accepted", name)
		}
	}
	if err := os.WriteFile(path, []byte(`{"kind":"live","implementerRoot":"first","implementerRoot":"last"} trailing bytes`), 0o644); err != nil {
		t.Fatal(err)
	}
	if subject, present, err := ReadRoundSubject(agents, "critic", 1); err != nil || !present || subject.ImplementerRoot != "last" {
		t.Fatalf("permissive wire grammar = %+v, %v, %v", subject, present, err)
	}
	if err := os.WriteFile(path, []byte(`{"kind":"commit","commit":"c","parent":"p","tree":"t","diffDigest":"d"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	subject, present, err := ReadRoundSubject(agents, "critic", 1)
	if err != nil || !present || subject.Parent != "p" || !subject.Equal(ReadSubject{Kind: SubjectCommit, Commit: "c", Tree: "t", DiffDigest: "d"}) {
		t.Fatalf("round subject = %+v, %v, %v", subject, present, err)
	}
}
