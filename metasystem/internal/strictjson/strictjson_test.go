package strictjson

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadAcceptsOnlyOneKnownDocument: a document naming only the target's
// fields decodes; an unknown field, trailing content and an unreadable file
// are each refused.
func TestReadAcceptsOnlyOneKnownDocument(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	var target struct {
		Name string `json:"name"`
	}
	if err := Read(write("ok.json", `{"name":"a"}`+"\n"), &target); err != nil || target.Name != "a" {
		t.Fatalf("Read of a known document = %+v, %v", target, err)
	}
	if err := Read(write("unknown.json", `{"name":"a","extra":1}`), &target); err == nil || !strings.Contains(err.Error(), "extra") {
		t.Fatalf("Read of an unknown field = %v, want the field refused", err)
	}
	trailing := write("trailing.json", `{"name":"a"} {}`)
	if err := Read(trailing, &target); err == nil || err.Error() != "trailing JSON in "+trailing {
		t.Fatalf("Read of trailing content = %v", err)
	}
	if err := Read(filepath.Join(dir, "absent.json"), &target); !os.IsNotExist(err) {
		t.Fatalf("Read of a missing file = %v, want its not-exist error", err)
	}
}
