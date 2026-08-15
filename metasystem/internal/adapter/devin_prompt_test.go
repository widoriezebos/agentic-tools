package adapter

import (
	"os"
	"path/filepath"
	"testing"
)

// The augmented prompt is the only schema channel this runtime has; its
// bytes are pinned against the shell heredocs the one writer replaced.
func TestDevinPromptBytes(t *testing.T) {
	dir := t.TempDir()
	prompt := filepath.Join(dir, "prompt.md")
	schema := filepath.Join(dir, "schema.json")
	output := filepath.Join(dir, "prompt.devin.md")
	if err := os.WriteFile(prompt, []byte("Do the work.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(schema, []byte(`{"required":["evidence"]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := DevinPrompt(prompt, schema, output, ""); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	expected := "Do the work.\n" +
		"\n\n# Return schema, exact\n\n" +
		"Your reply must be ONE JSON object valid against this schema and nothing else:\n" +
		"no prose before or after it, no code fence, and no property this schema\n" +
		"does not name. Every property listed in \"required\" must be present.\n\n" +
		`{"required":["evidence"]}` + "\n"
	if string(got) != expected {
		t.Fatalf("augmented prompt drifted:\n%q\nwant:\n%q", got, expected)
	}
	// With a named return file the delivery section names the exact path
	// (D62: this model delivers by writing files, not by final message).
	if err := DevinPrompt(prompt, schema, output, "/round/devin-return.json"); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	withDelivery := expected +
		"\n\n# Delivery, exact\n\n" +
		"Write that ONE JSON object to this exact file path, and also print it\n" +
		"as your final message. Do not choose a different path:\n\n" +
		"/round/devin-return.json\n"
	if string(got) != withDelivery {
		t.Fatalf("delivery section drifted:\n%q\nwant:\n%q", got, withDelivery)
	}
	if err := DevinPrompt(filepath.Join(dir, "absent.md"), schema, output, ""); err == nil {
		t.Fatal("absent prompt accepted")
	}
	if err := DevinPrompt(prompt, filepath.Join(dir, "absent.json"), output, ""); err == nil {
		t.Fatal("absent schema accepted")
	}
}
