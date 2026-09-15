package dispatch

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type briefBoundsCompositionFixture struct {
	params ComposeRolePacketParams
	record CompositionRecord
	packet []byte
	data   []byte
}

func composeBriefBoundsFixture(t *testing.T, supplied, referenced, bounded bool) briefBoundsCompositionFixture {
	t.Helper()
	root := compositionRepoRoot(t)
	temp := t.TempDir()
	body := []byte("Working Mode: implement\nsmall body\n")
	if referenced {
		body = bytes.Repeat([]byte("b"), MaxDirectiveBytes+1)
	}
	brief := filepath.Join(temp, "brief.md")
	if err := os.WriteFile(brief, body, 0o644); err != nil {
		t.Fatal(err)
	}
	params := ComposeRolePacketParams{
		Root: root, Role: "implementer", Brief: brief, JobID: "bounds-job", Runtime: "fake", Model: "fake-model",
		ToolPolicy: "read-write", Round: 2, DestructiveReach: HazardMechanical,
		Output: filepath.Join(temp, "prompt.md"), CompositionOutput: filepath.Join(temp, "composition.json"),
		StageDir: compositionTemporaryStageDir(t), ReferenceDir: compositionStageDir(t),
	}
	if supplied {
		record := BriefBoundsRecord{
			SchemaVersion: 1, JobID: params.JobID, RootJob: params.JobID, Round: params.Round,
			AdmittedSHA256: digestBytes(body), AdmittedBytes: int64(len(body)),
		}
		if bounded {
			ceiling := int64(4)
			record.Boundary, record.Ceiling = []string{"metasystem/internal/"}, &ceiling
		}
		data, err := EncodeBriefBoundsRecord(record)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(temp, "brief-bounds.json")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		params.AdmittedBounds = path
	}
	record, err := ComposeRolePacket(params)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := os.ReadFile(params.Output)
	if err != nil {
		t.Fatal(err)
	}
	composition, err := os.ReadFile(params.CompositionOutput)
	if err != nil {
		t.Fatal(err)
	}
	if referenced {
		if err := os.MkdirAll(filepath.Dir(record.References[0].OpenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(filepath.Join(params.StageDir, "task-direction.md"), record.References[0].OpenPath); err != nil {
			t.Fatal(err)
		}
	}
	return briefBoundsCompositionFixture{params: params, record: record, packet: packet, data: composition}
}

func assertBriefBoundsCompositionAdmitted(t *testing.T, fixture briefBoundsCompositionFixture) {
	t.Helper()
	if _, err := readCompositionForJob(fixture.params.CompositionOutput, fixture.params.JobID, fixture.params.Role, fixture.params.Runtime, fixture.params.Model, fixture.params.Mission, fixture.params.DestructiveReach, fixture.params.GoalTier, fixture.params.Round, int64(len(fixture.packet)), fixture.record.PacketDigest); err != nil {
		t.Fatal(err)
	}
}

func TestCompositionAdmittedBounds(t *testing.T) {
	cases := []struct {
		name, marker                  string
		supplied, referenced, bounded bool
	}{
		{name: "legacy", marker: "legacy"},
		{name: "bounded", marker: "bounded", supplied: true, bounded: true},
		{name: "unbounded", marker: "unbounded", supplied: true},
		{name: "referenced-body", marker: "bounded", supplied: true, referenced: true, bounded: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := composeBriefBoundsFixture(t, testCase.supplied, testCase.referenced, testCase.bounded)
			assertBriefBoundsCompositionAdmitted(t, fixture)
			first := fixture.record.Sources[0]
			if testCase.marker == "legacy" {
				if first.AdmittedBrief != nil {
					t.Fatal("legacy source unexpectedly carried an admittedBrief marker")
				}
				return
			}
			if first.AdmittedBrief == nil || first.AdmittedBrief.RecordSHA256 != digestBytes(mustRead(t, fixture.params.AdmittedBounds)) || first.AdmittedBrief.Bounded != testCase.bounded {
				t.Fatalf("marker = %+v", first.AdmittedBrief)
			}
			if first.SourceBytes != len(mustRead(t, fixture.params.Brief)) || first.SourceDigest != digestBytes(mustRead(t, fixture.params.Brief)) {
				t.Fatal("delivered source identity changed")
			}
			if mismatches, err := VerifyReferences(fixture.params.Root, fixture.params.CompositionOutput); err != nil || mismatches != nil {
				t.Fatalf("typed composition reader = %+v, %v", mismatches, err)
			}
		})
	}
}

func TestCompositionAdmittedBoundsIdentity(t *testing.T) {
	fixture := composeBriefBoundsFixture(t, true, false, true)
	record, _, err := ReadBriefBoundsRecord(fixture.params.AdmittedBounds)
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		name string
		edit func(*BriefBoundsRecord)
	}{
		{name: "job", edit: func(record *BriefBoundsRecord) { record.JobID = "other-job" }},
		{name: "round", edit: func(record *BriefBoundsRecord) { record.Round = 3 }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			changed := record
			testCase.edit(&changed)
			data, err := EncodeBriefBoundsRecord(changed)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "brief-bounds.json")
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			params := fixture.params
			params.AdmittedBounds = path
			params.Output, params.CompositionOutput = filepath.Join(t.TempDir(), "prompt"), filepath.Join(t.TempDir(), "composition")
			if _, err := ComposeRolePacket(params); err == nil {
				t.Fatal("identity mismatch unexpectedly composed")
			}
		})
	}
}

func mutateComposition(t *testing.T, fixture briefBoundsCompositionFixture, mutate func(map[string]any, map[string]any)) error {
	t.Helper()
	var record map[string]any
	if err := json.Unmarshal(fixture.data, &record); err != nil {
		t.Fatal(err)
	}
	sources := record["sources"].([]any)
	mutate(record, sources[0].(map[string]any))
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "composition.json")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = readCompositionForJob(path, fixture.params.JobID, fixture.params.Role, fixture.params.Runtime, fixture.params.Model, fixture.params.Mission, fixture.params.DestructiveReach, fixture.params.GoalTier, fixture.params.Round, int64(len(fixture.packet)), fixture.record.PacketDigest)
	return err
}

func TestCompositionAdmittedBoundsAdmission(t *testing.T) {
	valid := func(t *testing.T) briefBoundsCompositionFixture {
		return composeBriefBoundsFixture(t, true, false, true)
	}
	cases := []struct {
		name   string
		mutate func(map[string]any, map[string]any)
	}{
		{"marker-shape", func(_, source map[string]any) { delete(source["admittedBrief"].(map[string]any), "recordSha256") }},
		{"marker-version", func(_, source map[string]any) { source["admittedBrief"].(map[string]any)["schemaVersion"] = float64(2) }},
		{"marker-bounded", func(_, source map[string]any) { source["admittedBrief"].(map[string]any)["bounded"] = "true" }},
		{"marker-digest", func(_, source map[string]any) {
			source["admittedBrief"].(map[string]any)["recordSha256"] = strings.Repeat("A", 64)
		}},
		{"wrong-slot", func(_, source map[string]any) { source["slot"] = "tool-names" }},
		{"wrong-source", func(_, source map[string]any) { source["source"] = "caller:other" }},
		{"extra-source-field", func(_, source map[string]any) { source["extra"] = true }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := mutateComposition(t, valid(t), testCase.mutate); err == nil {
				t.Fatal("invalid admittedBrief source unexpectedly admitted")
			}
		})
	}
	t.Run("legacy-seven-fields", func(t *testing.T) {
		fixture := composeBriefBoundsFixture(t, false, false, false)
		assertBriefBoundsCompositionAdmitted(t, fixture)
	})
	t.Run("reference-reader", func(t *testing.T) {
		fixture := composeBriefBoundsFixture(t, true, false, true)
		if mismatches, err := VerifyReferences(fixture.params.Root, fixture.params.CompositionOutput); err != nil || mismatches != nil {
			t.Fatalf("typed reference reader rejected marker: %+v, %v", mismatches, err)
		}
	})
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
