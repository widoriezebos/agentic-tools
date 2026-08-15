package host

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The host walk's fixture: a root with the shipped orchestrator schema,
// a turn record, and a schema-valid orchestrator return.
type hostCollectFixture struct {
	root, turnDir, workspace string
	validReturn              []byte
	params                   HostCollectParams
}

func newHostCollectFixture(t *testing.T) *hostCollectFixture {
	t.Helper()
	f := &hostCollectFixture{root: t.TempDir()}
	schemaDir := filepath.Join(f.root, "scripts", "agents", "schemas")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	shipped, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "schemas", "orchestrator.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(schemaDir, "orchestrator.schema.json"), shipped, 0o644); err != nil {
		t.Fatal(err)
	}

	f.turnDir = filepath.Join(f.root, "turn")
	f.workspace = filepath.Join(f.root, "ws")
	for _, dir := range []string{f.turnDir, f.workspace} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	turnRecord := filepath.Join(f.turnDir, "turn.json")
	if err := os.WriteFile(turnRecord,
		[]byte(`{"turnId":"t-1","missionId":"m-1","cycle":3,"runtime":"devin","model":"swe-1-7"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// A minimal orchestrator return satisfying the shipped schema: built
	// from the schema's required list so schema drift breaks loudly here.
	var schemaDoc struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(shipped, &schemaDoc); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{
		"turnId": "t-1", "missionId": "m-1", "cycle": 3,
	}
	f.validReturn = buildOrchestratorReturn(t, shipped, body)

	f.params = HostCollectParams{
		Root: f.root, TurnRecordPath: turnRecord, TurnDir: f.turnDir,
		Workspace:  f.workspace,
		StdoutPath: filepath.Join(f.turnDir, "raw.out"),
		NamedPath:  filepath.Join(f.turnDir, "devin-return.json"),
	}
	return f
}

// buildOrchestratorReturn loads the repo's example-shaped orchestrator
// return if the schema requires more than the identity trio; the test
// fails loudly when it cannot satisfy the shipped schema.
func buildOrchestratorReturn(t *testing.T, schema []byte, base map[string]any) []byte {
	t.Helper()
	var doc struct {
		Required   []string                  `json:"required"`
		Properties map[string]map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(schema, &doc); err != nil {
		t.Fatal(err)
	}
	for _, field := range doc.Required {
		if _, ok := base[field]; ok {
			continue
		}
		prop := doc.Properties[field]
		switch prop["type"] {
		case "string":
			if enum, ok := prop["enum"].([]any); ok && len(enum) > 0 {
				base[field] = enum[0]
			} else if constant, ok := prop["const"]; ok {
				base[field] = constant
			} else {
				base[field] = "x"
			}
		case "integer", "number":
			if constant, ok := prop["const"]; ok {
				base[field] = constant
			} else {
				base[field] = 1
			}
		case "array":
			base[field] = []any{}
		case "object":
			base[field] = minimalObject(prop)
		case "boolean":
			base[field] = false
		default:
			if constant, ok := prop["const"]; ok {
				base[field] = constant
			} else {
				base[field] = "x"
			}
		}
	}
	data, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func minimalObject(prop map[string]any) map[string]any {
	out := map[string]any{}
	required, _ := prop["required"].([]any)
	properties, _ := prop["properties"].(map[string]any)
	for _, nameAny := range required {
		name, _ := nameAny.(string)
		child, _ := properties[name].(map[string]any)
		switch child["type"] {
		case "string":
			if enum, ok := child["enum"].([]any); ok && len(enum) > 0 {
				out[name] = enum[0]
			} else {
				out[name] = "x"
			}
		case "integer", "number":
			out[name] = 1
		case "array":
			out[name] = []any{}
		case "object":
			out[name] = minimalObject(child)
		case "boolean":
			out[name] = false
		default:
			out[name] = nil
		}
	}
	return out
}

func TestHostCollectWalksAndResumesPastRejects(t *testing.T) {
	f := newHostCollectFixture(t)
	if err := os.WriteFile(f.params.StdoutPath, f.validReturn, 0o644); err != nil {
		t.Fatal(err)
	}
	verdict, err := HostDevinCollect(f.params)
	if err != nil {
		t.Fatal(err)
	}
	if !verdict.Delivered || verdict.Channel != "stdout" {
		t.Fatalf("valid stdout must deliver: %+v", verdict)
	}
	accepted, _ := os.ReadFile(verdict.Reply)

	// The runner rejected that candidate post-envelope (wrong session):
	// the walk resumes and a valid named file delivers instead.
	if err := os.WriteFile(f.params.NamedPath, f.validReturn, 0o644); err != nil {
		t.Fatal(err)
	}
	// Make the named candidate byte-distinct so its digest differs.
	distinct := append([]byte(nil), f.validReturn...)
	distinct = append(distinct[:len(distinct)-1], []byte(` }`)...)
	if !json.Valid(distinct) {
		t.Fatal("test bug: distinct candidate must stay valid JSON")
	}
	if err := os.WriteFile(f.params.NamedPath, distinct, 0o644); err != nil {
		t.Fatal(err)
	}
	f.params.RejectDigests = []string{sha256Hex(accepted)}
	verdict, err = HostDevinCollect(f.params)
	if err != nil {
		t.Fatal(err)
	}
	if !verdict.Delivered || verdict.Channel != "named-file" {
		t.Fatalf("the walk must resume past the rejected digest: %+v", verdict)
	}
	if !strings.Contains(strings.Join(verdict.Rejected, "|"), "rejected by the runner") {
		t.Fatalf("the runner rejection must be recorded: %+v", verdict.Rejected)
	}
}

func TestHostCollectPreEnvelopeIdentity(t *testing.T) {
	f := newHostCollectFixture(t)
	wrongTurn := strings.Replace(string(f.validReturn), `"turnId":"t-1"`, `"turnId":"t-9"`, 1)
	if err := os.WriteFile(f.params.StdoutPath, []byte(wrongTurn), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.params.NamedPath, f.validReturn, 0o644); err != nil {
		t.Fatal(err)
	}
	verdict, err := HostDevinCollect(f.params)
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Channel != "named-file" {
		t.Fatalf("a wrong-turn stdout must not shadow the valid named file: %+v", verdict)
	}
	if !strings.Contains(strings.Join(verdict.Rejected, "|"), "turnId mismatch") {
		t.Fatalf("the identity rejection must be named: %+v", verdict.Rejected)
	}
}

func TestHostCollectMinesDesignatedWrite(t *testing.T) {
	f := newHostCollectFixture(t)
	target := filepath.Join(f.workspace, "devin-return.json")
	if err := os.WriteFile(target, f.validReturn, 0o644); err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]string{"file_path": target, "content": string(f.validReturn)})
	transcript := filepath.Join(f.root, "export.json")
	body := fmt.Sprintf(`{"session_id":"s","steps":[{"step_id":1,"tool_calls":[{"tool_call_id":"t1","function_name":"write","arguments":%s}]}]}`, args)
	if err := os.WriteFile(transcript, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	f.params.TranscriptPath = transcript
	verdict, err := HostDevinCollect(f.params)
	if err != nil {
		t.Fatal(err)
	}
	if !verdict.Delivered || verdict.Channel != "transcript" {
		t.Fatalf("the designated persisted write must deliver: %+v", verdict)
	}
	source, _ := os.ReadFile(filepath.Join(f.turnDir, "reply-source.json"))
	if !strings.Contains(string(source), `"channel": "transcript"`) {
		t.Fatalf("provenance must bind the channel: %s", source)
	}
}
