package dispatch

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testBoundsRecord(bounded bool) BriefBoundsRecord {
	r := BriefBoundsRecord{SchemaVersion: 1, JobID: "bounds-r2", RootJob: "bounds", Round: 2, AdmittedSHA256: strings.Repeat("a", 64), AdmittedBytes: 12}
	if bounded {
		r.Boundary = []string{}
		ceiling := int64(4)
		r.Ceiling = &ceiling
	}
	return r
}

func composeBounds(t *testing.T, bounds *BriefBoundsRecord, body []byte) (ComposeRolePacketParams, CompositionRecord, []byte, []byte, error) {
	t.Helper()
	root, temp, stage := compositionRepoRoot(t), t.TempDir(), compositionStageDir(t)
	brief := filepath.Join(temp, "brief.md")
	if err := os.WriteFile(brief, body, 0o644); err != nil {
		t.Fatal(err)
	}
	p := ComposeRolePacketParams{Root: root, Role: "verifier", Brief: brief, JobID: "bounds-r2", Runtime: "fake", Model: "fake-model", Round: 2, DestructiveReach: HazardMechanical, ToolPolicy: "read-only", Output: filepath.Join(temp, "prompt.md"), CompositionOutput: filepath.Join(temp, "composition.json"), StageDir: stage, ReferenceDir: stage}
	var encoded []byte
	if bounds != nil {
		var err error
		encoded, err = MarshalBriefBoundsRecord(*bounds)
		if err != nil {
			t.Fatal(err)
		}
		p.AdmittedBounds = filepath.Join(temp, "brief-bounds.json")
		if err := os.WriteFile(p.AdmittedBounds, encoded, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	record, err := ComposeRolePacket(p)
	if err != nil {
		return p, record, nil, encoded, err
	}
	packet, readErr := os.ReadFile(p.Output)
	if readErr != nil {
		t.Fatal(readErr)
	}
	return p, record, packet, encoded, nil
}

func mustComposeBounds(t *testing.T, bounds *BriefBoundsRecord, body []byte) (ComposeRolePacketParams, CompositionRecord, []byte, []byte) {
	t.Helper()
	p, record, packet, encoded, err := composeBounds(t, bounds, body)
	if err != nil {
		t.Fatal(err)
	}
	return p, record, packet, encoded
}

func TestCompositionAdmittedBounds(t *testing.T) {
	t.Run("marker", func(t *testing.T) {
		r := testBoundsRecord(true)
		p, _, _, encoded := mustComposeBounds(t, &r, []byte("delivered\n"))
		encoded = append([]byte(" \n"), encoded...)
		if err := os.WriteFile(p.AdmittedBounds, encoded, 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := ComposeRolePacket(p)
		if err != nil {
			t.Fatal(err)
		}
		m := got.Sources[0].AdmittedBrief
		if m == nil || m.SchemaVersion != 1 || m.RecordSHA256 != digestBytes(encoded) {
			t.Fatalf("marker = %#v", m)
		}
		stored, err := readObject(p.CompositionOutput)
		if err != nil {
			t.Fatal(err)
		}
		sources, ok := stored["sources"].([]any)
		if !ok || len(sources) == 0 {
			t.Fatalf("stored sources = %#v", stored["sources"])
		}
		storedMarker, ok := sourceObject(sources, 0)["admittedBrief"].(map[string]any)
		version, versionOK := numInt(storedMarker["schemaVersion"])
		if !ok || len(storedMarker) != 3 || !versionOK || version != 1 || asString(storedMarker["recordSha256"]) != digestBytes(encoded) {
			t.Fatalf("stored marker = %#v", storedMarker)
		}
		unknown := bytes.Replace(encoded, []byte("{\n"), []byte("{\n  \"unknown\": true,\n"), 1)
		if err := os.WriteFile(p.AdmittedBounds, unknown, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ComposeRolePacket(p); err == nil {
			t.Fatal("unknown admitted record field was accepted")
		}
	})
	t.Run("bounded", func(t *testing.T) {
		r := testBoundsRecord(true)
		_, got, _, _ := mustComposeBounds(t, &r, []byte("x"))
		if got.Sources[0].AdmittedBrief == nil || !got.Sources[0].AdmittedBrief.Bounded {
			t.Fatal("empty Boundary was not bounded")
		}
	})
	t.Run("unbounded", func(t *testing.T) {
		r := testBoundsRecord(false)
		p, got, _, _ := mustComposeBounds(t, &r, []byte("x"))
		if got.Sources[0].AdmittedBrief == nil || got.Sources[0].AdmittedBrief.Bounded {
			t.Fatal("null pair was bounded")
		}
		stored, err := readObject(p.CompositionOutput)
		if err != nil {
			t.Fatal(err)
		}
		bounded, ok := markerObject(stored["sources"].([]any), 0)["bounded"].(bool)
		if !ok || bounded {
			t.Fatalf("stored bounded = %#v", markerObject(stored["sources"].([]any), 0)["bounded"])
		}
	})
	t.Run("job", func(t *testing.T) {
		r := testBoundsRecord(true)
		r.JobID = "other"
		if _, _, _, _, err := composeBounds(t, &r, []byte("x")); err == nil {
			t.Fatal("wrong job admitted")
		}
	})
	t.Run("round", func(t *testing.T) {
		r := testBoundsRecord(true)
		r.Round = 1
		if _, _, _, _, err := composeBounds(t, &r, []byte("x")); err == nil {
			t.Fatal("wrong round admitted")
		}
	})
	t.Run("legacy", func(t *testing.T) {
		_, got, _, _ := mustComposeBounds(t, nil, []byte("x"))
		if got.Sources[0].AdmittedBrief != nil {
			t.Fatal("legacy source gained marker")
		}
	})
	t.Run("delivered-source", func(t *testing.T) {
		r := testBoundsRecord(true)
		body := []byte("augmented delivery\n")
		_, got, _, _ := mustComposeBounds(t, &r, body)
		source := got.Sources[0]
		if source.SourceDigest != digestBytes(body) || source.SourceBytes != len(body) {
			t.Fatalf("source = %#v", source)
		}
	})
	t.Run("inline-body", func(t *testing.T) {
		r := testBoundsRecord(true)
		body := []byte("exact inline\n")
		_, got, packet, _ := mustComposeBounds(t, &r, body)
		source := got.Sources[0]
		if !bytes.Equal(packet[source.StartByte:source.EndByte], []byte("# Task Direction\n\nexact inline\n\n")) {
			t.Fatalf("range = %q", packet[source.StartByte:source.EndByte])
		}
	})
	t.Run("referenced-body", func(t *testing.T) {
		r := testBoundsRecord(true)
		body := bytes.Repeat([]byte("z"), MaxDirectiveBytes+1)
		p, got, packet, _ := mustComposeBounds(t, &r, body)
		staged, err := os.ReadFile(filepath.Join(p.StageDir, "task-direction.md"))
		if err != nil || !bytes.Equal(staged, body) || bytes.Contains(packet[got.Sources[0].StartByte:got.Sources[0].EndByte], body) {
			t.Fatalf("referenced body changed or was inlined: %v", err)
		}
	})
}

func compositionAdmission(t *testing.T, bounds *BriefBoundsRecord, body []byte, mutate func([]any)) error {
	t.Helper()
	p, composed, packet, _ := mustComposeBounds(t, bounds, body)
	object, err := readObject(p.CompositionOutput)
	if err != nil {
		t.Fatal(err)
	}
	sources := object["sources"].([]any)
	if mutate != nil {
		mutate(sources)
		if err := writeRecord(p.CompositionOutput, object); err != nil {
			t.Fatal(err)
		}
	}
	_, err = readCompositionForJob(p.CompositionOutput, p.JobID, p.Role, p.Runtime, p.Model, "", p.DestructiveReach, 0, p.Round, int64(len(packet)), composed.PacketDigest)
	return err
}

func sourceObject(sources []any, index int) map[string]any { return sources[index].(map[string]any) }
func markerObject(sources []any, index int) map[string]any {
	return sourceObject(sources, index)["admittedBrief"].(map[string]any)
}

func TestCompositionAdmittedBoundsAdmission(t *testing.T) {
	bounded := testBoundsRecord(true)
	unbounded := testBoundsRecord(false)
	t.Run("legacy-seven-fields", func(t *testing.T) {
		if err := compositionAdmission(t, nil, []byte("x"), nil); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("eight-fields", func(t *testing.T) {
		if err := compositionAdmission(t, &bounded, []byte("x"), nil); err != nil {
			t.Fatal(err)
		}
		if err := compositionAdmission(t, &unbounded, []byte("x"), nil); err != nil {
			t.Fatal(err)
		}
		if err := compositionAdmission(t, &unbounded, bytes.Repeat([]byte("x"), MaxDirectiveBytes+1), nil); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("marker-shape", func(t *testing.T) {
		for _, extra := range []bool{false, true} {
			err := compositionAdmission(t, &bounded, []byte("x"), func(s []any) {
				m := markerObject(s, 0)
				if extra {
					m["extra"] = true
				} else {
					delete(m, "bounded")
				}
			})
			if err == nil {
				t.Fatalf("extra=%t admitted", extra)
			}
		}
	})
	t.Run("marker-version", func(t *testing.T) {
		for _, version := range []int{2, 0, -1} {
			if err := compositionAdmission(t, &bounded, []byte("x"), func(s []any) { markerObject(s, 0)["schemaVersion"] = version }); err == nil {
				t.Fatalf("version %d admitted", version)
			}
		}
	})
	t.Run("marker-bounded", func(t *testing.T) {
		if err := compositionAdmission(t, &bounded, []byte("x"), func(s []any) { markerObject(s, 0)["bounded"] = "true" }); err == nil {
			t.Fatal("non-boolean admitted")
		}
	})
	t.Run("marker-digest", func(t *testing.T) {
		for _, digest := range []string{strings.Repeat("A", 64), strings.Repeat("a", 63), strings.Repeat("a", 65)} {
			if err := compositionAdmission(t, &bounded, []byte("x"), func(s []any) { markerObject(s, 0)["recordSha256"] = digest }); err == nil {
				t.Fatalf("%d-character digest admitted", len(digest))
			}
		}
	})
	t.Run("wrong-slot", func(t *testing.T) {
		if err := compositionAdmission(t, &bounded, []byte("x"), func(s []any) {
			sourceObject(s, 1)["source"] = "caller:brief"
			sourceObject(s, 1)["admittedBrief"] = sourceObject(s, 0)["admittedBrief"]
			delete(sourceObject(s, 0), "admittedBrief")
		}); err == nil {
			t.Fatal("wrong slot admitted")
		}
	})
	t.Run("wrong-source", func(t *testing.T) {
		if err := compositionAdmission(t, &bounded, []byte("x"), func(s []any) {
			sourceObject(s, 1)["slot"] = "task-direction"
			sourceObject(s, 1)["admittedBrief"] = sourceObject(s, 0)["admittedBrief"]
			delete(sourceObject(s, 0), "admittedBrief")
		}); err == nil {
			t.Fatal("wrong source admitted")
		}
	})
	t.Run("extra-source-field", func(t *testing.T) {
		if err := compositionAdmission(t, &bounded, []byte("x"), func(s []any) { sourceObject(s, 1)["extra"] = true }); err == nil {
			t.Fatal("extra field admitted")
		}
	})
	t.Run("reference-reader", func(t *testing.T) {
		p, _, _, _ := mustComposeBounds(t, &bounded, bytes.Repeat([]byte("r"), MaxDirectiveBytes+1))
		mismatches, err := VerifyReferences(p.Root, p.CompositionOutput)
		if err != nil || len(mismatches) != 0 {
			t.Fatalf("typed reader = %v, %#v", err, mismatches)
		}
	})
}
