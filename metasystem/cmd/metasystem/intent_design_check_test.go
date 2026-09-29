package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func init() {
	registerIdempotency("design check", idemRead, "", nil)
}

// TestDesignCheckJudgesAnObligationMatrix is the public home of the former
// validate design-obligations the completion check names: by default a
// critical or high obligation may await its runtime proof; --complete
// requires every one done.
func TestDesignCheckJudgesAnObligationMatrix(t *testing.T) {
	t.Parallel()
	b := newIntentBed(t, false, nil)
	example, err := filepath.Abs(filepath.Join("..", "..", "docs", "examples", "design-obligation-matrix.md"))
	if err != nil {
		t.Fatal(err)
	}
	code, result := b.runJSON(b.owners(), "design", "check", example)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("design check of the example = %d %+v", code, result)
	}
	code, result = b.runJSON(b.owners(), "design", "check", example, "--complete")
	data, _ := result.Data.(map[string]any)
	if code != 1 || result.Outcome != intentRefused || !strings.Contains(fmt.Sprint(data["problems"]), "READY_FOR_RUNTIME") {
		t.Fatalf("design check --complete of the example = %d %+v", code, result)
	}
	if code, result := b.runJSON(b.owners(), "design", "check"); code != 2 || result.Outcome != intentRefused {
		t.Fatalf("design check without a file = %d %+v", code, result)
	}
}
