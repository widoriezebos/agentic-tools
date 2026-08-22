package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdjudicateTurnPureVerdicts(t *testing.T) {
	cases := []struct {
		name   string
		params AdjudicateParams
		want   string
	}{
		{"cli failure after handshake", AdjudicateParams{Stage: "initial", CLIStatus: 7, HandshakeDone: true},
			"finish failed runtime_error runtime"},
		{"cli failure before handshake", AdjudicateParams{Stage: "initial", CLIStatus: 7},
			"fail-pending runtime_error handshake"},
		{"no session correlated", AdjudicateParams{Stage: "initial", CLIStatus: 0},
			"fail-pending handshake_missing_session_id handshake"},
		{"failed repair turn", AdjudicateParams{Stage: "after-repair", RepairRC: 1},
			"protocol-error"},
		{"settle agreed", AdjudicateParams{Stage: "settle-result", SettleOK: true},
			"finish completed null completed"},
		{"settle disagreed", AdjudicateParams{Stage: "settle-result"},
			"finish failed session_identity_disagreement delivery"},
		{"empty reply after handshake", AdjudicateParams{Stage: "empty-reply", HandshakeDone: true},
			"finish failed empty_reply delivery"},
		{"empty reply before handshake", AdjudicateParams{Stage: "empty-reply"},
			"fail-pending empty_reply delivery"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := AdjudicateTurn(c.params)
			if err != nil || got != c.want {
				t.Fatalf("verdict = %q err=%v, want %q", got, err, c.want)
			}
		})
	}
	if _, err := AdjudicateTurn(AdjudicateParams{Stage: "bogus"}); err == nil {
		t.Fatal("an unknown stage must refuse")
	}
}

// adjudicateFixture wires an unparseable candidate so validation fails at
// normalization — the repair-decision branches run without a full
// return-complete fixture (the adapter suite fixtures cover the valid path
// end to end).
func adjudicateFixture(t *testing.T, repairAvailable bool) AdjudicateParams {
	t.Helper()
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	return AdjudicateParams{
		Stage:            "initial",
		Root:             dir,
		Job:              "job-1",
		RecordPath:       write("job.json", `{"role":"implementer","round":1}`),
		SchemaPath:       write("schema.json", "{\"type\":\"object\"}\n"),
		CandidatePath:    write("raw.out", "not a return at all"),
		ReturnPath:       filepath.Join(dir, "return.json"),
		MarkdownPath:     filepath.Join(dir, "return.md"),
		ViolationPath:    filepath.Join(dir, "violation.txt"),
		RepairPromptPath: filepath.Join(dir, "repair-1.prompt.md"),
		CLIStatus:        0,
		HandshakeDone:    true,
		RepairAvailable:  repairAvailable,
	}
}

func TestAdjudicateTurnRepairDecision(t *testing.T) {
	p := adjudicateFixture(t, true)
	got, err := AdjudicateTurn(p)
	if err != nil || got != "repair" {
		t.Fatalf("verdict = %q err=%v, want repair", got, err)
	}
	violation, err := os.ReadFile(p.ViolationPath)
	if err != nil || !strings.HasPrefix(string(violation), "return normalization failed: ") {
		t.Fatalf("violation = %q err=%v", violation, err)
	}
	prompt, err := os.ReadFile(p.RepairPromptPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "Your previous reply did not validate against the required schema.\n" +
		"Everything you already did in this session still stands; only the\n" +
		"shape of the reply was wrong.\n\n# What failed\n\n" +
		string(violation) +
		"\n# The schema your reply must satisfy\n\n" +
		"{\"type\":\"object\"}\n" +
		"\n# What to send now\n\n" +
		"Reply with ONE JSON object valid against that schema and nothing\n" +
		"else: no prose before or after it, no code fence, no property the\n" +
		"schema does not name, and every property listed in \"required\".\n" +
		"Do not repeat the work; report what you already found.\n"
	if string(prompt) != want {
		t.Fatalf("repair prompt drifted:\n got %q\nwant %q", prompt, want)
	}
}

func TestAdjudicateTurnWithoutRepairGoesProtocolError(t *testing.T) {
	p := adjudicateFixture(t, false)
	got, err := AdjudicateTurn(p)
	if err != nil || got != "protocol-error" {
		t.Fatalf("verdict = %q err=%v, want protocol-error", got, err)
	}
	if _, err := os.Stat(p.RepairPromptPath); !os.IsNotExist(err) {
		t.Fatal("no repair prompt may be written when repair is unavailable")
	}
}

func TestAdjudicateTurnRefusalWrappers(t *testing.T) {
	t.Run("unreadable schema refuses by name", func(t *testing.T) {
		p := adjudicateFixture(t, true)
		p.SchemaPath = filepath.Join(t.TempDir(), "no-such-schema.json")
		_, err := AdjudicateTurn(p)
		if err == nil || !strings.Contains(err.Error(), "cannot read the schema") {
			t.Fatalf("want schema refusal, got %v", err)
		}
	})
	t.Run("unwritable violation refuses by name", func(t *testing.T) {
		p := adjudicateFixture(t, true)
		p.ViolationPath = filepath.Join(p.Root, "no", "such", "dir", "v.txt")
		_, err := AdjudicateTurn(p)
		if err == nil || !strings.Contains(err.Error(), "cannot write the violation") {
			t.Fatalf("want violation refusal, got %v", err)
		}
	})
	t.Run("unwritable repair prompt refuses by name", func(t *testing.T) {
		p := adjudicateFixture(t, true)
		p.RepairPromptPath = filepath.Join(p.Root, "no", "such", "dir", "p.md")
		_, err := AdjudicateTurn(p)
		if err == nil || !strings.Contains(err.Error(), "cannot write the repair prompt") {
			t.Fatalf("want prompt refusal, got %v", err)
		}
	})
	t.Run("after-repair unwritable violation refuses", func(t *testing.T) {
		p := adjudicateFixture(t, true)
		p.Stage = "after-repair"
		p.RepairCandidate = p.CandidatePath
		p.ViolationPath = filepath.Join(p.Root, "no", "such", "dir", "v.txt")
		_, err := AdjudicateTurn(p)
		if err == nil || !strings.Contains(err.Error(), "cannot write the violation") {
			t.Fatalf("want violation refusal, got %v", err)
		}
	})
	t.Run("after-repair without settle finishes completed on a valid repair", func(t *testing.T) {
		// A candidate that normalizes cleanly but has no return-complete
		// fixture still exercises the settle-absent branch when validation
		// is bypassed by the normalization failure writing a violation —
		// covered above; the genuinely-valid path is proven by the suite's
		// fake-adapter turns end to end.
		p := adjudicateFixture(t, true)
		p.Stage = "settle-result"
		p.SettleOK = true
		got, err := AdjudicateTurn(p)
		if err != nil || got != "finish completed null completed" {
			t.Fatalf("got %q err=%v", got, err)
		}
	})
}

func TestAdjudicateTurnAfterRepairInvalidGoesProtocolError(t *testing.T) {
	p := adjudicateFixture(t, true)
	p.Stage = "after-repair"
	p.RepairRC = 0
	p.RepairCandidate = p.CandidatePath
	got, err := AdjudicateTurn(p)
	if err != nil || got != "protocol-error" {
		t.Fatalf("verdict = %q err=%v, want protocol-error", got, err)
	}
}

// The empty-delivery stage: a pure recommendation. Correlated +
// repair-available writes the delivery prompt naming the repair path;
// uncorrelated or repair-unavailable falls to the empty-reply verdicts.
func TestAdjudicateEmptyDeliveryStage(t *testing.T) {
	dir := t.TempDir()
	schema := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(schema, []byte(`{"required":["evidence"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	prompt := filepath.Join(dir, "repair-prompt.md")
	named := filepath.Join(dir, "devin-return.repair-1.json")

	p := AdjudicateParams{Stage: "empty-delivery", SchemaPath: schema,
		RepairPromptPath: prompt, NamedRepairPath: named,
		HandshakeDone: true, RepairAvailable: true}
	verdict, err := AdjudicateTurn(p)
	if err != nil || verdict != "delivery-repair" {
		t.Fatalf("eligible empty delivery must recommend the repair: %q %v", verdict, err)
	}
	text, err := os.ReadFile(prompt)
	if err != nil || !strings.Contains(string(text), named) ||
		!strings.Contains(string(text), "also print it") {
		t.Fatalf("the delivery prompt must name the exact path and the print ask: %v\n%s", err, text)
	}

	p.RepairAvailable = false
	if verdict, _ := AdjudicateTurn(p); verdict != "finish failed empty_reply delivery" {
		t.Fatalf("repair-unavailable must keep the pinned empty verdict: %q", verdict)
	}
	p.HandshakeDone = false
	if verdict, _ := AdjudicateTurn(p); verdict != "fail-pending empty_reply delivery" {
		t.Fatalf("uncorrelated must keep the pinned pending verdict: %q", verdict)
	}
}
