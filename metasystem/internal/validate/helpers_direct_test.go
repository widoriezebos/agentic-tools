package validate

import (
	"encoding/json"
	"strings"
	"testing"
)

// The rendering and position helpers, case by case.

func TestJSONErrorPosition(t *testing.T) {
	data := []byte("{\n  \"a\": 1,\n  broken\n}")
	var value any
	err := json.Unmarshal(data, &value)
	if err == nil {
		t.Fatal("fixture must not parse")
	}
	line, column := jsonErrorPosition(data, err)
	if line != 3 {
		t.Fatalf("error line: got %d, want 3", line)
	}
	if column < 1 {
		t.Fatalf("column must be positive: %d", column)
	}
	// A non-syntax error clamps to the document start.
	line, column = jsonErrorPosition([]byte("xy"), json.Unmarshal([]byte(`"a"`), new(int)))
	if line != 1 || column != 1 {
		t.Fatalf("non-syntax error position: %d:%d", line, column)
	}
}

func TestQuotedRenderings(t *testing.T) {
	cases := map[any]string{
		nil:   "None",
		true:  "True",
		false: "False",
	}
	for input, want := range cases {
		if got := quoted(input); got != want {
			t.Fatalf("quoted(%v) = %q, want %q", input, got, want)
		}
	}
	if got := quoted("text"); !strings.Contains(got, "text") {
		t.Fatalf("quoted string lost its content: %q", got)
	}
	if got := quotedList(nil); got != "[]" {
		t.Fatalf("empty list: %q", got)
	}
	if got := quotedList([]string{"a", "b"}); got != "['a', 'b']" {
		t.Fatalf("list: %q", got)
	}
}

func TestScalarTextRenderings(t *testing.T) {
	if got := scalarText(float64(3)); got != "3" {
		t.Fatalf("whole float: %q", got)
	}
	if got := scalarText(0.5); got != "0.5" {
		t.Fatalf("fraction: %q", got)
	}
	if got := scalarText("s"); got != "s" {
		t.Fatalf("string: %q", got)
	}
	if got := scalarText(nil); got != "None" {
		t.Fatalf("nil: %q", got)
	}
	if got := scalarText(true); got != "True" {
		t.Fatalf("bool: %q", got)
	}
}
