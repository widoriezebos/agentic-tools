package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLedger(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ledger.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestStopLoss(t *testing.T) {
	cases := []struct {
		name    string
		content string
		code    int
		want    string
	}{
		{"dead end", "### Cycle C1\n- Classification: falsified-dead-end\n", 1,
			"a cycle was classified falsified-dead-end"},
		{"two no-progress", "### Cycle C1\n- Classification: no-progress\n### Cycle C2\n- Classification: no-progress\n", 1,
			"2 cycles classified no-progress"},
		{"cycle budget spent", "- Cycle budget: 2\n### Cycle C1\n- Classification: contract-improved\n### Cycle C2\n- Classification: falsified-continue\n", 1,
			"2 cycles recorded against a budget of 2"},
		{"no-gain budget spent", "- No-gain budget: 3\n### Cycle E1\n- Classification: falsified-continue\n### Cycle E2\n- Classification: unresolved\n### Cycle E3\n- Classification: falsified-continue\n", 1,
			"3 trailing cycles without a contract-improved against a no-gain budget of 3"},
		{"gain resets the trailing count", "- No-gain budget: 3\n### Cycle E1\n- Classification: falsified-continue\n### Cycle E2\n- Classification: contract-improved\n### Cycle E3\n- Classification: falsified-continue\n", 0,
			"stop-loss not triggered: 3 cycles, 0 no-progress, budget none, no-gain budget 3"},
		{"unclassified cycle still counts", "- No-gain budget: 3\n### Cycle E1\n- Classification: unresolved\n### Cycle E2\n### Cycle E3\n- Classification: falsified-continue\n", 1,
			"3 trailing cycles"},
		{"fake gain does not reset", "- No-gain budget: 3\n### Cycle E1\n- Classification: unresolved\n### Cycle E2\n- Classification: not-contract-improved\n### Cycle E3\n- Classification: falsified-continue\n", 1,
			"3 trailing cycles"},
		{"backticked gain resets", "### Cycle C1\n- Classification: `contract-improved`\n- No-gain budget: 1\n", 0,
			"stop-loss not triggered"},
		{"unresolved never triggers no-progress", "### Cycle C1\n- Classification: unresolved\n- Classification: unresolved\n", 0,
			"stop-loss not triggered: 1 cycles, 0 no-progress, budget none, no-gain budget none"},
		{"empty ledger", "no cycles here\n", 0, "stop-loss not triggered: 0 cycles"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, errs, code := StopLoss(writeLedger(t, tc.content))
			if code != tc.code {
				t.Fatalf("exit %d, want %d\nout: %v\nerr: %v", code, tc.code, out, errs)
			}
			joined := strings.Join(append(append([]string{}, out...), errs...), "\n")
			if !strings.Contains(joined, tc.want) {
				t.Fatalf("output lacks %q:\n%s", tc.want, joined)
			}
		})
	}
	if _, errs, code := StopLoss(filepath.Join(t.TempDir(), "absent.md")); code != 2 ||
		errs[0] != "missing --file ledger" {
		t.Fatalf("absent ledger wrong: %v %d", errs, code)
	}
}
