package dispatch

import (
	"os"
	"path/filepath"
	"testing"
)

// The frozen accepted-input grammar for job records (Phase 5.1, TD-3):
// each case pins a shape the CURRENT reader accepts or refuses. The typed
// reader must reproduce every verdict below before any writer converts;
// narrowing the grammar is a behavior change Phase 5 does not carry.
func TestJobRecordGrammarFrozen(t *testing.T) {
	cases := []struct {
		name     string
		document string
		accepted bool
		probe    func(map[string]any) bool
	}{
		{
			// encoding/json keeps the LAST duplicate; the reader inherits that.
			name:     "duplicate-keys-last-wins",
			document: `{"jobId":"first","jobId":"second","status":"running"}`,
			accepted: true,
			probe:    func(doc map[string]any) bool { return doc["jobId"] == "second" },
		},
		{
			// A single Decode with no EOF check tolerates trailing bytes —
			// including a whole second document (record.go:291's contract).
			name:     "trailing-second-document-tolerated",
			document: `{"jobId":"kept","status":"running"}` + "\n" + `{"jobId":"ignored"}`,
			accepted: true,
			probe:    func(doc map[string]any) bool { return doc["jobId"] == "kept" },
		},
		{
			name:     "trailing-garbage-tolerated",
			document: `{"jobId":"kept","status":"running"}` + "\ngarbage not json",
			accepted: true,
			probe:    func(doc map[string]any) bool { return doc["jobId"] == "kept" },
		},
		{
			// Ill-typed known fields decode; refusal is the CONSUMER's business.
			name:     "ill-typed-known-fields-decode",
			document: `{"jobId":42,"status":["not","a","string"],"capMin":"thirty"}`,
			accepted: true,
			probe: func(doc map[string]any) bool {
				_, jobIsString := doc["jobId"].(string)
				return !jobIsString
			},
		},
		{
			// Null and absent are distinct decoded states.
			name:     "null-vs-absent-distinct",
			document: `{"jobId":"n","status":"running","error":null}`,
			accepted: true,
			probe: func(doc map[string]any) bool {
				value, present := doc["error"]
				_, absent := doc["never-set"]
				return present && value == nil && !absent
			},
		},
		{
			// Numbers keep their literal spellings under UseNumber.
			name:     "number-spellings-preserved",
			document: `{"jobId":"num","status":"running","a":1,"b":1.0,"c":1e0,"d":0.5}`,
			accepted: true,
			probe: func(doc map[string]any) bool {
				return jsonLiteral(doc["a"]) == "1" && jsonLiteral(doc["b"]) == "1.0" &&
					jsonLiteral(doc["c"]) == "1e0" && jsonLiteral(doc["d"]) == "0.5"
			},
		},
		{name: "top-level-array-refused-by-readObject", document: `["not","an","object"]`, accepted: false},
		{name: "top-level-scalar-refused-by-readObject", document: `"just a string"`, accepted: false},
		{name: "empty-file-refused", document: ``, accepted: false},
		{name: "leading-garbage-refused", document: "garbage\n" + `{"jobId":"x"}`, accepted: false},
	}
	for _, entry := range cases {
		path := filepath.Join(t.TempDir(), "record.json")
		if err := os.WriteFile(path, []byte(entry.document), 0o644); err != nil {
			t.Fatal(err)
		}
		doc, err := readObject(path)
		if entry.accepted {
			if err != nil {
				t.Fatalf("%s: the current reader accepts this and the reader under test refused: %v", entry.name, err)
			}
			if entry.probe != nil && !entry.probe(doc) {
				t.Fatalf("%s: accepted, but the decoded shape drifted: %v", entry.name, doc)
			}
			continue
		}
		if err == nil {
			t.Fatalf("%s: the current reader refuses this and the reader under test accepted: %v", entry.name, doc)
		}
	}
}

// jsonLiteral reads the literal spelling a json.Number preserved.
func jsonLiteral(value any) string {
	if number, ok := value.(interface{ String() string }); ok {
		return number.String()
	}
	return ""
}
