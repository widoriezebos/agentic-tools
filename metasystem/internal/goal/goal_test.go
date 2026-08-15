package goal

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const canonical = `# Goals

## Current goal: ship-widget — Ship the widget end to end
- Origin: human
- Next step: Wire the widget into the release train.
- Evidence: plans/widget.md

## Queued goal: fix-docs — Bring the docs current
- Origin: main
- Next step: Rewrite the quickstart against the new CLI.

## Parked goal: perf-pass — Cut p99 latency in half
- Origin: main
- Parked because: Blocked on the vendor's profiler fix.
- Next step: Re-profile once the vendor ships.

## Done goal: port-engine — Port the engine to Go
- Origin: human
- Concluded: Landed and gated on both hosts.
`

// Parse and Serialize round-trip byte-stably on canonical input, and every
// section lands in its place.
func TestParseRoundTrip(t *testing.T) {
	ledger, problems := Parse([]byte(canonical))
	if len(problems) != 0 {
		t.Fatalf("canonical ledger has problems: %v", problems)
	}
	if ledger.Current == nil || ledger.Current.Id != "ship-widget" {
		t.Fatalf("current wrong: %+v", ledger.Current)
	}
	if len(ledger.Queued) != 1 || ledger.Queued[0].Id != "fix-docs" {
		t.Fatalf("queued wrong: %+v", ledger.Queued)
	}
	if len(ledger.Parked) != 1 || ledger.Parked[0].Parked == "" {
		t.Fatalf("parked wrong: %+v", ledger.Parked)
	}
	if len(ledger.Done) != 1 || ledger.Done[0].Conclude == "" {
		t.Fatalf("done wrong: %+v", ledger.Done)
	}
	if !bytes.Equal(Serialize(ledger), []byte(canonical)) {
		t.Fatalf("round-trip not byte-stable:\n%s", Serialize(ledger))
	}
}

// The revision scopes to the Current block alone: queued/done edits never
// re-arm it, any wording change to the step does.
func TestRevisionScopesToCurrentBlock(t *testing.T) {
	ledger, _ := Parse([]byte(canonical))
	before := ledger.Revision()

	edited := strings.Replace(canonical, "Bring the docs current", "Bring the docs way current", 1)
	ledger2, _ := Parse([]byte(edited))
	if ledger2.Revision() != before {
		t.Fatal("a queued edit re-armed the current revision")
	}

	stepEdit := strings.Replace(canonical, "Wire the widget", "Land the widget", 1)
	ledger3, _ := Parse([]byte(stepEdit))
	if ledger3.Revision() == before {
		t.Fatal("a step edit did not change the revision")
	}
}

// Zero-current legality: bare emptiness refuses; a queue or a declaration
// stands in; a declaration with standing queue refuses.
func TestZeroCurrentLegality(t *testing.T) {
	_, problems := Parse([]byte("# Goals\n"))
	if len(problems) == 0 {
		t.Fatal("undeclared absence parsed clean")
	}

	free := "# Goals\n\n## Goal-free: declared 2026-08-15T10:00:00Z by human over abc123\n"
	_, problems = Parse([]byte(free))
	if len(problems) != 0 {
		t.Fatalf("goal-free ledger has problems: %v", problems)
	}

	queuedOnly := `# Goals

## Queued goal: fix-docs — Bring the docs current
- Origin: main
- Next step: Rewrite the quickstart.
`
	ledger, problems := Parse([]byte(queuedOnly))
	if len(problems) != 0 {
		t.Fatalf("queued-only ledger has problems: %v", problems)
	}
	if ledger.QueuedDigest() == "" {
		t.Fatal("queued-only ledger has no queued digest")
	}

	contradiction := free + `
## Queued goal: fix-docs — Bring the docs current
- Origin: main
- Next step: Rewrite the quickstart.
`
	_, problems = Parse([]byte(contradiction))
	if len(problems) == 0 {
		t.Fatal("declaration with a standing queue parsed clean")
	}
}

// Every byte bound refuses, and structural defects name themselves.
func TestBoundsAndStructure(t *testing.T) {
	cases := []struct{ name, body string }{
		{"intent-bound", "# Goals\n\n## Current goal: a — " + strings.Repeat("x", 161) + "\n- Origin: main\n- Next step: Do.\n"},
		{"id-bound", "# Goals\n\n## Current goal: " + strings.Repeat("a", 65) + " — ok\n- Origin: main\n- Next step: Do.\n"},
		{"id-shape", "# Goals\n\n## Current goal: Bad_Id — ok\n- Origin: main\n- Next step: Do.\n"},
		{"step-bound", "# Goals\n\n## Current goal: a — ok\n- Origin: main\n- Next step: " + strings.Repeat("x", 241) + "\n"},
		{"origin", "# Goals\n\n## Current goal: a — ok\n- Origin: robot\n- Next step: Do.\n"},
		{"dup-ids", "# Goals\n\n## Current goal: a — ok\n- Origin: main\n- Next step: Do.\n\n## Queued goal: a — twice\n- Origin: main\n- Next step: Do.\n"},
		{"two-current", "# Goals\n\n## Current goal: a — ok\n- Origin: main\n- Next step: Do.\n\n## Current goal: b — two\n- Origin: main\n- Next step: Do.\n"},
		{"done-with-step", "# Goals\n\n## Current goal: a — ok\n- Origin: main\n- Next step: Do.\n\n## Done goal: b — done\n- Origin: main\n- Concluded: Yes.\n- Next step: leftover\n"},
		{"parked-without-reason", "# Goals\n\n## Current goal: a — ok\n- Origin: main\n- Next step: Do.\n\n## Parked goal: c — parked\n- Origin: main\n- Next step: Later.\n"},
		{"evidence-count", "# Goals\n\n## Current goal: a — ok\n- Origin: main\n- Next step: Do.\n- Evidence: one\n- Evidence: two\n- Evidence: three\n- Evidence: four\n"},
	}
	for _, c := range cases {
		if _, problems := Parse([]byte(c.body)); len(problems) == 0 {
			t.Errorf("%s: parsed clean", c.name)
		}
	}
}

// The scan digest is a set digest over plan streams: adding a stream
// changes it, editing content does not, goals.md never counts.
func TestScanDigestIsSetLevel(t *testing.T) {
	root := t.TempDir()
	plans := filepath.Join(root, "plans")
	os.MkdirAll(plans, 0o755)
	os.WriteFile(filepath.Join(plans, "alpha.md"), []byte("work"), 0o644)
	os.WriteFile(filepath.Join(plans, "goals.md"), []byte("# Goals\n"), 0o644)

	first, err := ScanDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(plans, "alpha.md"), []byte("more work"), 0o644)
	second, _ := ScanDigest(root)
	if first != second {
		t.Fatal("content edit changed the set digest")
	}
	os.WriteFile(filepath.Join(plans, "beta.md"), []byte("new stream"), 0o644)
	third, _ := ScanDigest(root)
	if third == first {
		t.Fatal("a new stream did not change the set digest")
	}
}
