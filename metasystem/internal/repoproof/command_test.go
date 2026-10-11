package repoproof

import (
	"bytes"
	"testing"
)

func TestExecuteKeepsDeclaredArgumentZero(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	if err := Execute([]string{"sh", "-c", `printf '%s' "$0"`}, &output, &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "sh" {
		t.Fatalf("declared argument zero: %q", output.String())
	}
}
