package validate

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

type admittedBoundsFixture struct {
	root, rootJob, job, roundText, roundDir string
	admitted                                []byte
	record                                  dispatch.BriefBoundsRecord
	roundComposition, jobComposition        dispatch.CompositionRecord
}

func mustBoundsFixture(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newAdmittedBoundsFixture(t *testing.T, rootJob, job string, round int64, bounds dispatch.BriefBounds) *admittedBoundsFixture {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if rootJob == "" {
		rootJob = "test-review-bounds-" + strings.NewReplacer("/", "-", "_", "-").Replace(t.Name())
	}
	roundText := strconv.FormatInt(round, 10)
	roundDir := filepath.Join(root, "artifacts", "agents", rootJob, "rounds", roundText)
	if err := os.MkdirAll(roundDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(root, "artifacts", "agents", rootJob)) })
	admitted := []byte("Working Mode: implement\ntext that is never parsed\n")
	record := dispatch.BriefBoundsRecord{SchemaVersion: 1, JobID: job, RootJob: rootJob, Round: round, AdmittedSHA256: sourceDigest(admitted), AdmittedBytes: int64(len(admitted)), Boundary: bounds.Boundary, Ceiling: bounds.Ceiling}
	encoded, err := dispatch.MarshalBriefBoundsRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(roundDir, "brief-bounds.json"), encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(roundDir, "admitted-brief.md"), admitted, 0o644); err != nil {
		t.Fatal(err)
	}
	source := composeBriefSourceFor(t, admitted, job, round, filepath.Join(roundDir, "brief-bounds.json"))
	if err := os.WriteFile(filepath.Join(roundDir, "prompt.md"), source.prompt, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(roundDir, "composition.json"), source.encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	return &admittedBoundsFixture{root: root, rootJob: rootJob, job: job, roundText: roundText, roundDir: roundDir, admitted: admitted, record: record, roundComposition: cloneComposition(t, source.record), jobComposition: cloneComposition(t, source.record)}
}

func cloneComposition(t *testing.T, record dispatch.CompositionRecord) dispatch.CompositionRecord {
	t.Helper()
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var clone dispatch.CompositionRecord
	if err := json.Unmarshal(data, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func (f *admittedBoundsFixture) jobMap(t *testing.T) map[string]any {
	t.Helper()
	data, err := json.Marshal(f.jobComposition)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	return record
}

func (f *admittedBoundsFixture) writeCompositions(t *testing.T) {
	t.Helper()
	data, err := json.Marshal(f.roundComposition)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.roundDir, "composition.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *admittedBoundsFixture) writeRecord(t *testing.T, synchronizeMarkers bool) {
	t.Helper()
	data, err := dispatch.MarshalBriefBoundsRecord(f.record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.roundDir, "brief-bounds.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if synchronizeMarkers {
		f.synchronizeMarkers(t, data)
	}
}

func (f *admittedBoundsFixture) synchronizeMarkers(t *testing.T, record []byte) {
	t.Helper()
	digest := sourceDigest(record)
	f.roundComposition.Sources[0].AdmittedBrief.RecordSHA256 = digest
	f.jobComposition.Sources[0].AdmittedBrief.RecordSHA256 = digest
	f.writeCompositions(t)
}

func (f *admittedBoundsFixture) read(t *testing.T) (dispatch.BriefBounds, error) {
	t.Helper()
	return ReadRoundBriefBounds(f.root, f.rootJob, f.job, f.roundText, f.jobMap(t))
}

func boundedForTest(member string) dispatch.BriefBounds {
	ceiling := int64(7)
	return dispatch.BriefBounds{Boundary: []string{member}, Ceiling: &ceiling}
}

func requireStoredBounds(t *testing.T, got dispatch.BriefBounds, member string) {
	t.Helper()
	if len(got.Boundary) != 1 || got.Boundary[0] != member || got.Ceiling == nil || *got.Ceiling != 7 {
		t.Fatalf("bounds = %+v, want boundary %q and ceiling 7", got, member)
	}
}

func runReviewBriefBoundsSelectionCases(t *testing.T) {
	t.Helper()
	t.Run("current", func(t *testing.T) {
		f := newAdmittedBoundsFixture(t, "", "current-job", 2, boundedForTest("current.go"))
		run := conformanceRun{root: f.root, rootJob: f.rootJob, job: f.job, roundText: f.roundText, record: map[string]any{"composition": f.jobMap(t)}}
		got, err := run.reviewBriefBounds()
		if err != nil {
			t.Fatal(err)
		}
		requireStoredBounds(t, got, "current.go")
	})
	t.Run("later", func(t *testing.T) {
		current := newAdmittedBoundsFixture(t, "", "current-job", 2, boundedForTest("current.go"))
		_ = newAdmittedBoundsFixture(t, current.rootJob, "later-job", 3, boundedForTest("later.go"))
		got, err := current.read(t)
		if err != nil {
			t.Fatal(err)
		}
		requireStoredBounds(t, got, "current.go")
	})
	t.Run("headerless-follow-up", func(t *testing.T) {
		prior := newAdmittedBoundsFixture(t, "", "prior-job", 2, boundedForTest("prior.go"))
		followUp := newAdmittedBoundsFixture(t, prior.rootJob, "follow-up-job", 3, dispatch.BriefBounds{})
		got, err := followUp.read(t)
		if err != nil {
			t.Fatal(err)
		}
		if got.Boundary != nil || got.Ceiling != nil {
			t.Fatalf("bounds = %+v, want unbounded", got)
		}
	})
}

func TestReviewBriefBoundsAdmittedRecord(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *admittedBoundsFixture)
	}{
		{"missing-bounded-record", func(t *testing.T, f *admittedBoundsFixture) {
			mustBoundsFixture(t, os.Remove(filepath.Join(f.roundDir, "brief-bounds.json")))
		}},
		{"missing-unbounded-record", func(t *testing.T, f *admittedBoundsFixture) {
			f.record.Boundary, f.record.Ceiling = nil, nil
			f.roundComposition.Sources[0].AdmittedBrief.Bounded = false
			f.jobComposition.Sources[0].AdmittedBrief.Bounded = false
			f.writeRecord(t, true)
			mustBoundsFixture(t, os.Remove(filepath.Join(f.roundDir, "brief-bounds.json")))
		}},
		{"missing-copy", func(t *testing.T, f *admittedBoundsFixture) {
			mustBoundsFixture(t, os.Remove(filepath.Join(f.roundDir, "admitted-brief.md")))
		}},
		{"missing-prompt", func(t *testing.T, f *admittedBoundsFixture) {
			mustBoundsFixture(t, os.Remove(filepath.Join(f.roundDir, "prompt.md")))
		}},
		{"copy-symlink", func(t *testing.T, f *admittedBoundsFixture) {
			path, target := filepath.Join(f.roundDir, "admitted-brief.md"), filepath.Join(t.TempDir(), "copy")
			mustBoundsFixture(t, os.WriteFile(target, f.admitted, 0o644))
			mustBoundsFixture(t, os.Remove(path))
			mustBoundsFixture(t, os.Symlink(target, path))
		}},
		{"record-symlink", func(t *testing.T, f *admittedBoundsFixture) {
			path, target := filepath.Join(f.roundDir, "brief-bounds.json"), filepath.Join(t.TempDir(), "record")
			data, err := os.ReadFile(path)
			mustBoundsFixture(t, err)
			mustBoundsFixture(t, os.WriteFile(target, data, 0o644))
			mustBoundsFixture(t, os.Remove(path))
			mustBoundsFixture(t, os.Symlink(target, path))
		}},
		{"record-hash", func(t *testing.T, f *admittedBoundsFixture) {
			path := filepath.Join(f.roundDir, "brief-bounds.json")
			data, err := os.ReadFile(path)
			mustBoundsFixture(t, err)
			mustBoundsFixture(t, os.WriteFile(path, append(data, '\n'), 0o644))
		}},
		{"record-schema", func(t *testing.T, f *admittedBoundsFixture) {
			path := filepath.Join(f.roundDir, "brief-bounds.json")
			data, err := os.ReadFile(path)
			mustBoundsFixture(t, err)
			data = bytes.Replace(data, []byte("{"), []byte("{\n  \"unknown\": true,"), 1)
			mustBoundsFixture(t, os.WriteFile(path, data, 0o644))
			f.synchronizeMarkers(t, data)
		}},
		{"record-job", func(t *testing.T, f *admittedBoundsFixture) { f.record.JobID = "other-job"; f.writeRecord(t, true) }},
		{"record-root", func(t *testing.T, f *admittedBoundsFixture) { f.record.RootJob = "other-root"; f.writeRecord(t, true) }},
		{"record-round", func(t *testing.T, f *admittedBoundsFixture) { f.record.Round = 3; f.writeRecord(t, true) }},
		{"pair-marker", func(t *testing.T, f *admittedBoundsFixture) {
			f.roundComposition.Sources[0].AdmittedBrief.Bounded = false
			f.jobComposition.Sources[0].AdmittedBrief.Bounded = false
			f.writeCompositions(t)
		}},
		{"admitted-length", func(t *testing.T, f *admittedBoundsFixture) { f.record.AdmittedBytes++; f.writeRecord(t, true) }},
		{"admitted-digest", func(t *testing.T, f *admittedBoundsFixture) {
			f.record.AdmittedSHA256 = strings.Repeat("0", 64)
			f.writeRecord(t, true)
		}},
		{"job-marker", func(t *testing.T, f *admittedBoundsFixture) {
			f.roundComposition.Sources[0].AdmittedBrief = nil
			f.writeCompositions(t)
		}},
		{"round-marker", func(t *testing.T, f *admittedBoundsFixture) {
			f.jobComposition.Sources[0].AdmittedBrief = nil
		}},
		{"marker-agreement", func(t *testing.T, f *admittedBoundsFixture) {
			f.jobComposition.Sources[0].AdmittedBrief.RecordSHA256 = strings.Repeat("0", 64)
		}},
		{"marker-schema", func(t *testing.T, f *admittedBoundsFixture) {
			f.roundComposition.Sources[0].AdmittedBrief.SchemaVersion = 2
			f.jobComposition.Sources[0].AdmittedBrief.SchemaVersion = 2
			f.writeCompositions(t)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newAdmittedBoundsFixture(t, "", "bounds-job", 2, boundedForTest("kept.go"))
			test.mutate(t, f)
			_, err := f.read(t)
			var unreadable *BriefBoundsUnreadable
			if !errors.As(err, &unreadable) {
				t.Fatalf("error = %#v, want BRIEF_BOUNDS_UNREADABLE", err)
			}
		})
	}

	t.Run("no-prose-parse", func(t *testing.T) {
		for _, path := range []string{"brief_bounds_source.go", filepath.Join("..", "dispatch", "composition.go")} {
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				selector, selected := callFunctionSelector(call, ok)
				if selected && selector.Sel.Name == "ParseBriefBounds" {
					t.Errorf("%s calls ParseBriefBounds", path)
				}
				return true
			})
		}
	})

	t.Run("legacy-root", func(t *testing.T) {
		root := t.TempDir()
		mustBoundsFixture(t, os.WriteFile(filepath.Join(root, "brief.md"), []byte("Boundary: [\"must-not-read\"]\nCeiling: 1\n"), 0o644))
		got, err := ReadRoundBriefBounds(root, "legacy", "legacy-job", "1", nil)
		if err != nil || got.Boundary != nil || got.Ceiling != nil {
			t.Fatalf("bounds = %+v, error = %v; want unbounded", got, err)
		}
	})

	t.Run("legacy-follow-up", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "artifacts", "agents", "legacy", "rounds", "2")
		mustBoundsFixture(t, os.MkdirAll(dir, 0o755))
		mustBoundsFixture(t, os.WriteFile(filepath.Join(dir, "prompt.md"), []byte("Boundary: [\"must-not-read\"]\nCeiling: 1\n"), 0o644))
		got, err := ReadRoundBriefBounds(root, "legacy", "legacy-job", "2", nil)
		if err != nil || got.Boundary != nil || got.Ceiling != nil {
			t.Fatalf("bounds = %+v, error = %v; want unbounded", got, err)
		}
	})

	t.Run("legacy-composition", func(t *testing.T) {
		f := newAdmittedBoundsFixture(t, "", "legacy-job", 2, boundedForTest("ignored.go"))
		f.roundComposition.Sources[0].AdmittedBrief = nil
		f.jobComposition.Sources[0].AdmittedBrief = nil
		f.writeCompositions(t)
		mustBoundsFixture(t, os.Remove(filepath.Join(f.roundDir, "brief-bounds.json")))
		mustBoundsFixture(t, os.Remove(filepath.Join(f.roundDir, "admitted-brief.md")))
		got, err := f.read(t)
		if err != nil || got.Boundary != nil || got.Ceiling != nil {
			t.Fatalf("bounds = %+v, error = %v; want unbounded", got, err)
		}
	})

	for _, name := range []string{"orphan", "bad-composition"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "artifacts", "agents", "legacy", "rounds", "1")
			mustBoundsFixture(t, os.MkdirAll(dir, 0o755))
			if name == "orphan" {
				mustBoundsFixture(t, os.WriteFile(filepath.Join(dir, "brief-bounds.json"), []byte("{}"), 0o644))
			} else {
				mustBoundsFixture(t, os.WriteFile(filepath.Join(dir, "composition.json"), []byte("{"), 0o644))
				mustBoundsFixture(t, os.WriteFile(filepath.Join(dir, "prompt.md"), nil, 0o644))
			}
			_, err := ReadRoundBriefBounds(root, "legacy", "legacy-job", "1", nil)
			var unreadable *BriefBoundsUnreadable
			if !errors.As(err, &unreadable) {
				t.Fatalf("error = %#v, want BRIEF_BOUNDS_UNREADABLE", err)
			}
		})
	}
}

func callFunctionSelector(call *ast.CallExpr, ok bool) (*ast.SelectorExpr, bool) {
	if !ok {
		return nil, false
	}
	selector, selected := call.Fun.(*ast.SelectorExpr)
	return selector, selected
}
