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
	// (this model delivers by writing files, not by final message).
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

func TestDevinPermissionMode(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "job.json")
	// Every readable record runs dangerous under the standing human
	// waiver; a graded mode turns an envelope refusal into
	// non-delivery.
	os.WriteFile(record, []byte(`{"permissions":{"requested":{"writeRoots":[]}}}`), 0o644)
	if mode, err := DevinPermissionMode(record); err != nil || mode != "dangerous" {
		t.Fatalf("read-only role = (%s,%v)", mode, err)
	}
	os.WriteFile(record, []byte(`{"permissions":{"requested":{"writeRoots":["/ws"]}}}`), 0o644)
	if mode, err := DevinPermissionMode(record); err != nil || mode != "dangerous" {
		t.Fatalf("write role = (%s,%v)", mode, err)
	}
	if _, err := DevinPermissionMode(filepath.Join(dir, "absent.json")); err == nil {
		t.Fatal("an unreadable record must refuse, never default open")
	}
}

// The transcript-vs-correlated-session certification and the
// effective-model derivation, over every disagreement shape.
func TestDevinSettle(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, "t.json")
	write := func(body string) {
		if err := os.WriteFile(transcript, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	disagreement := filepath.Join(dir, "session-disagreement.txt")

	write(`{"session_id":"sess-1","agent":{"model_name":"SWE-1.7"}}`)
	model, certified, err := DevinSettle(transcript, "", "sess-1", dir, false)
	if err != nil || !certified || model != "swe-1-7" {
		t.Fatalf("agreeing settle = (%s,%v,%v)", model, certified, err)
	}
	model, certified, err = DevinSettle(transcript, "", "sess-OTHER", dir, false)
	if err != nil || certified {
		t.Fatalf("disagreement = (%s,%v,%v)", model, certified, err)
	}
	body, _ := os.ReadFile(disagreement)
	if string(body) != "transcript session sess-1 disagrees with correlated session sess-OTHER\n" {
		t.Fatalf("artifact = %q", body)
	}
	write(`{"agent":{"model_name":null}}`)
	model, certified, err = DevinSettle(transcript, "", "sess-1", dir, false)
	if err != nil || certified || model != "unobserved" {
		t.Fatalf("nameless transcript = (%s,%v,%v)", model, certified, err)
	}
	body, _ = os.ReadFile(disagreement)
	if string(body) != "correlated session sess-1 but the transcript names no session\n" {
		t.Fatalf("artifact = %q", body)
	}
	if model, certified, err = DevinSettle(transcript, "", "", dir, false); err != nil || !certified {
		t.Fatalf("nothing correlated settles = (%s,%v,%v)", model, certified, err)
	}
	if err := os.Remove(transcript); err != nil {
		t.Fatal(err)
	}
	model, certified, err = DevinSettle(transcript, "", "sess-1", dir, true)
	if err != nil || certified || model != "" {
		t.Fatalf("repair without transcript = (%s,%v,%v)", model, certified, err)
	}
	body, _ = os.ReadFile(disagreement)
	if string(body) != "repair produced no transcript; session and model are unconfirmable\n" {
		t.Fatalf("artifact = %q", body)
	}
}
