package jsonedit

import (
	"errors"
	"strings"
	"testing"
)

// TestGetWireSpellings pins the shell contract: dozens of call sites
// string-compare this output, so every rendering rule is a wire byte.
func TestGetWireSpellings(t *testing.T) {
	def := "DEF"
	cases := []struct {
		name    string
		content string
		field   string
		def     *string
		want    string
		ok      bool
	}{
		{name: "string prints bare", content: `{"a":"x y"}`, field: "a", want: "x y", ok: true},
		{name: "whole number drops the decimal", content: `{"a":7}`, field: "a", want: "7", ok: true},
		{name: "fraction keeps its digits", content: `{"a":1.5}`, field: "a", want: "1.5", ok: true},
		{name: "bool prints lowercase", content: `{"a":true}`, field: "a", want: "true", ok: true},
		{name: "null without default prints null", content: `{"a":null}`, field: "a", want: "null", ok: true},
		{name: "null with default prints the default", content: `{"a":null}`, field: "a", def: &def, want: "DEF", ok: true},
		{name: "missing without default refuses", content: `{}`, field: "a"},
		{name: "missing with default prints the default", content: `{}`, field: "a", def: &def, want: "DEF", ok: true},
		{name: "dotted path resolves", content: `{"a":{"b":{"c":3}}}`, field: "a.b.c", want: "3", ok: true},
		{name: "traversal through a scalar refuses", content: `{"a":1}`, field: "a.b"},
		{name: "traversal through a scalar honors the default", content: `{"a":1}`, field: "a.b", def: &def, want: "DEF", ok: true},
		{name: "composite prints compact JSON", content: `{"a":{"y":1,"x":2}}`, field: "a", want: `{"x":2,"y":1}`, ok: true},
		{name: "unparsable content refuses", content: `{broken`, field: "a"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := Get([]byte(c.content), c.field, c.def)
			if ok != c.ok || got != c.want {
				t.Fatalf("Get(%q, %q) = (%q, %v), want (%q, %v)", c.content, c.field, got, ok, c.want, c.ok)
			}
		})
	}
}

func TestSetFieldsEditsAndClassifies(t *testing.T) {
	object, err := SetFields([]byte(`{"keep":true}`), []string{"name=x"}, []string{"count=42"})
	if err != nil {
		t.Fatal(err)
	}
	if object["keep"] != true || object["name"] != "x" || object["count"] != int64(42) {
		t.Fatalf("edits not applied: %+v", object)
	}
	if _, err := SetFields([]byte(`{broken`), nil, nil); err == nil || errors.Is(err, ErrUsage) {
		t.Fatalf("unparsable content is a data error, not usage: %v", err)
	}
	for _, c := range []struct {
		strings, ints []string
		wantText      string
	}{
		{strings: []string{"novalue"}, wantText: `--field "novalue" is not KEY=VALUE`},
		{ints: []string{"novalue"}, wantText: `--int "novalue" is not KEY=VALUE`},
		{ints: []string{"n=x"}, wantText: `--int "n=x" is not an integer`},
	} {
		_, err := SetFields([]byte(`{}`), c.strings, c.ints)
		if err == nil || !errors.Is(err, ErrUsage) || err.Error() != c.wantText {
			t.Fatalf("want usage error %q, got %v", c.wantText, err)
		}
	}
}

func TestObjectSpelling(t *testing.T) {
	line, err := Object([]string{"b=2", "a=<tag>", "skipped-no-equals"})
	if err != nil {
		t.Fatal(err)
	}
	if line != `{"a":"<tag>","b":"2"}` {
		t.Fatalf("object spelling changed: %s", line)
	}
	// Escaped output would replace the raw angle brackets with u-escape
	// sequences, so RAW-character PRESENCE is the non-escaping proof (the
	// recorded pitfall is asserting raw-character absence).
	if !strings.Contains(line, "<tag>") {
		t.Fatal("HTML escaping crept into json object output")
	}
}

// StripKeys: the structural settings.json derivation.
func TestStripKeys(t *testing.T) {
	data := []byte(`{"_comment":"note","hooks":{"Stop":[{"command":"x"}]},"systemMessage":"m"}`)
	object, err := StripKeys(data, []string{"_comment"})
	if err != nil {
		t.Fatal(err)
	}
	if _, present := object["_comment"]; present {
		t.Fatal("stripped key survived")
	}
	if object["systemMessage"] != "m" {
		t.Fatalf("unrelated keys disturbed: %v", object)
	}
	// Absent keys are a no-op: adoption is re-runnable.
	again, err := StripKeys(data, []string{"absent"})
	if err != nil || len(again) != 3 {
		t.Fatalf("absent-key strip changed the object: %v %v", again, err)
	}
	if _, err := StripKeys([]byte("not json"), []string{"k"}); err == nil {
		t.Fatal("invalid JSON accepted")
	}
}
