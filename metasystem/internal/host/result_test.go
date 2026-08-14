package host

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFinishTurnTaxonomy(t *testing.T) {
	dir := t.TempDir()
	raw := filepath.Join(dir, "raw.out")
	os.WriteFile(raw, []byte("reply"), 0o644)
	empty := filepath.Join(dir, "empty.out")
	os.WriteFile(empty, nil, 0o644)
	result := filepath.Join(dir, "result.json")

	read := func() map[string]any {
		data, err := os.ReadFile(result)
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err := json.Unmarshal(data, &value); err != nil {
			t.Fatal(err)
		}
		return value
	}

	code, err := FinishTurn(result, "sess-1", "", raw, filepath.Join(dir, "return.json"), 7, false)
	if err != nil || code != 3 || read()["outcome"] != "failed" || read()["returnPath"] != nil {
		t.Fatalf("cli failure = (%d,%v,%v)", code, err, read())
	}
	code, err = FinishTurn(result, "sess-1", "", empty, "r.json", 0, true)
	if err != nil || code != 3 || read()["outcome"] != "failed" {
		t.Fatalf("empty required reply = (%d,%v,%v)", code, err, read())
	}
	code, err = FinishTurn(result, "", "", raw, "r.json", 0, false)
	if err != nil || code != 6 || read()["outcome"] != "unresumable" {
		t.Fatalf("missing session = (%d,%v,%v)", code, err, read())
	}
	code, err = FinishTurn(result, "sess-1", "", raw, "r.json", 0, true)
	if err != nil || code != 0 || read()["outcome"] != "completed" || read()["returnPath"] != "r.json" {
		t.Fatalf("completed = (%d,%v,%v)", code, err, read())
	}
}
