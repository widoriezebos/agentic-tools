package stopreport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStopResponseRecordIsStrict(t *testing.T) {
	for _, runtime := range []string{"claude", "codex"} {
		t.Run(runtime, func(t *testing.T) {
			for _, blocked := range []bool{false, true} {
				t.Run(map[bool]string{false: "allowed", true: "blocked"}[blocked], func(t *testing.T) {
					root, err := filepath.EvalSymlinks(t.TempDir())
					if err != nil {
						t.Fatal(err)
					}
					payload := []byte(`{"systemMessage":"visible"}`)
					visibleField := "systemMessage"
					if blocked {
						payload = []byte(`{"decision":"block","reason":"visible"}`)
						visibleField = "reason"
					}
					id := SessionKey(runtime, "session") + "-" + strings.Repeat("a", 32)
					response := Response{
						SchemaVersion: ResponseSchemaVersion,
						Runtime:       runtime,
						ShouldBlock:   blocked,
						VisibleField:  visibleField,
						PayloadSHA256: PayloadSHA256(payload),
						Report: ResponseReportReference{
							Installation: root,
							ID:           id,
							Alias:        "a",
							Path:         filepath.Join(root, "artifacts", "agents", "supervision", "stop-verdicts", id+".md"),
							SHA256:       strings.Repeat("b", 64),
						},
					}
					if err := WriteResponse(root, append([]byte(" \t"), append(payload, '\n')...), response); err != nil {
						t.Fatal(err)
					}
					if got, err := ReadResponse(root, payload); err != nil || got != response {
						t.Fatalf("read response = %+v, %v", got, err)
					}
					if err := WriteResponse(root, payload, response); err != nil {
						t.Fatalf("identical response retry failed: %v", err)
					}
					changed := response
					changed.Runtime += "-other"
					if err := WriteResponse(root, payload, changed); err == nil || !strings.Contains(err.Error(), "different content") {
						t.Fatalf("different response content was accepted: %v", err)
					}
					valid, _ := json.Marshal(response)
					var object map[string]any
					if err := json.Unmarshal(valid, &object); err != nil {
						t.Fatal(err)
					}
					setReference := func(field string, value any) func(map[string]any) {
						return func(record map[string]any) { record["report"].(map[string]any)[field] = value }
					}
					cases := []struct {
						name, field string
						raw         []byte
						change      func(map[string]any)
					}{
						{name: "not one object", field: "one JSON object", raw: append(valid, []byte("\n{}")...)},
						{name: "unknown schema", field: "schemaVersion", change: func(value map[string]any) { value["schemaVersion"] = 2 }},
						{name: "missing report", field: "report", change: func(value map[string]any) { delete(value, "report") }},
						{name: "missing installation", field: "report.installation", change: setReference("installation", "")},
						{name: "malformed id", field: "report.id", change: setReference("id", "bad")},
						{name: "missing alias", field: "report.alias", change: setReference("alias", "")},
						{name: "malformed path", field: "report.path", change: setReference("path", filepath.Join(root, "wrong"))},
						{name: "missing digest", field: "report.sha256", change: setReference("sha256", "")},
						{name: "unknown field", field: "unknown", change: func(value map[string]any) { value["unknown"] = true }},
					}
					for _, test := range cases {
						t.Run(test.name, func(t *testing.T) {
							data := test.raw
							if data == nil {
								copy := cloneResponseObject(t, object)
								test.change(copy)
								data = marshalResponseObject(t, copy)
							}
							if err := os.WriteFile(ResponsePath(root, payload), data, 0o600); err != nil {
								t.Fatal(err)
							}
							_, err := ReadResponse(root, payload)
							if err == nil || !strings.Contains(err.Error(), "Stop response is unreadable") || !strings.Contains(err.Error(), test.field) {
								t.Fatalf("strict read error = %v, want unreadable response naming %s", err, test.field)
							}
						})
					}
				})
			}
		})
	}
}
func cloneResponseObject(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	data := marshalResponseObject(t, value)
	var clone map[string]any
	if err := json.Unmarshal(data, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}
func marshalResponseObject(t *testing.T, value map[string]any) []byte {
	t.Helper()
	data, _ := json.Marshal(value)
	return data
}
