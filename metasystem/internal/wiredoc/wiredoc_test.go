package wiredoc

import (
	"strings"
	"testing"
)

// The decoder carries the frozen grammar (TD-3's mechanism side): the same
// verdicts internal/dispatch pins against its current reader.
func TestDecodeGrammar(t *testing.T) {
	if doc, err := Decode([]byte(`{"a":"first","a":"second"}`)); err != nil {
		t.Fatal(err)
	} else if value, _ := doc.Get("a"); value != "second" {
		t.Fatalf("duplicate keys: got %v", value)
	}
	if doc, err := Decode([]byte(`{"kept":1}` + "\ntrailing garbage")); err != nil {
		t.Fatal("trailing bytes must stay tolerated:", err)
	} else if _, present := doc.Get("kept"); !present {
		t.Fatal("the first document was lost")
	}
	if _, err := Decode([]byte(`["array"]`)); err == nil {
		t.Fatal("a top-level array decoded as a document")
	}
	if _, err := Decode([]byte(``)); err == nil {
		t.Fatal("an empty input decoded")
	}
	if doc, err := Decode([]byte(`{"n":1.0,"e":1e6}`)); err != nil {
		t.Fatal(err)
	} else {
		n, _ := doc.Get("n")
		if literal, ok := n.(interface{ String() string }); !ok || literal.String() != "1.0" {
			t.Fatalf("number spelling lost: %v", n)
		}
	}
}

// LOSSLESS: a document full of unknown structure survives decode → mutate →
// render with everything untouched except the mutation, in canonical bytes.
func TestRoundTripIsLossless(t *testing.T) {
	source := `{
  "brief": "a<b && c>d & e",
  "known": "old",
  "nullField": null,
  "unknownBlock": {
    "list": [
      "one",
      2,
      null
    ],
    "z": 1
  }
}
`
	doc, err := Decode([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	doc.Set("known", "new")
	rendered, err := doc.Render()
	if err != nil {
		t.Fatal(err)
	}
	text := string(rendered)
	// The mutation landed; everything else is byte-identical to the
	// canonical form of the source (which the source above already is,
	// except the one field).
	if !strings.Contains(text, `"known": "new"`) {
		t.Fatalf("mutation lost:\n%s", text)
	}
	if strings.Replace(text, `"known": "new"`, `"known": "old"`, 1) != source {
		t.Fatalf("round trip is not lossless:\nsource:\n%s\nrendered:\n%s", source, text)
	}
	// null stayed null; absence stays absence.
	if value, present := doc.Get("nullField"); !present || value != nil {
		t.Fatal("null-vs-absent drifted")
	}
	doc.Delete("nullField")
	if _, present := doc.Get("nullField"); present {
		t.Fatal("delete did not produce absence")
	}
}

// The canonical encoder: sorted keys, no HTML escaping, trailing newline.
func TestRenderCanonicalForm(t *testing.T) {
	doc, _ := Decode([]byte(`{"z":1,"a":"x<y&z","m":{"bb":2,"aa":1}}`))
	rendered, err := doc.Render()
	if err != nil {
		t.Fatal(err)
	}
	text := string(rendered)
	if strings.Contains(text, `\u003c`) || strings.Contains(text, `\u0026`) {
		t.Fatalf("HTML escaped: %s", text)
	}
	if !strings.Contains(text, `x<y&z`) {
		t.Fatalf("the raw characters did not survive: %s", text)
	}
	if !strings.HasSuffix(text, "}\n") {
		t.Fatalf("no trailing newline: %q", text)
	}
	if strings.Index(text, `"a"`) > strings.Index(text, `"z"`) {
		t.Fatalf("keys not sorted: %s", text)
	}
	if strings.Index(text, `"aa"`) > strings.Index(text, `"bb"`) {
		t.Fatalf("nested keys not sorted: %s", text)
	}
}

// FromRaw wraps a live map: mutations through the map (the CAS path's
// pattern) are visible to the render, and the render is canonical.
func TestFromRawSharesTheMap(t *testing.T) {
	raw := map[string]any{"status": "running", "z": 1}
	doc := FromRaw(raw)
	raw["patched"] = "by-cas"
	rendered, err := doc.Render()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rendered), `"patched": "by-cas"`) {
		t.Fatalf("a raw-map mutation was invisible to the render: %s", rendered)
	}
	if value, present := doc.Get("status"); !present || value != "running" {
		t.Fatal("Get does not see the wrapped map")
	}
}
