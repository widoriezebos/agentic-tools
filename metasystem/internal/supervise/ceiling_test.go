package supervise

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func ceilingConf(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "metasystem.conf")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDeriveCeiling(t *testing.T) {
	cases := []struct {
		name     string
		conf     []string
		declared int64
		environ  []string
		want     int64
		refusal  string
	}{
		{name: "bare floor is 120 plus the allowance", conf: []string{"metasystem.runtimes=fake"}, want: 150},
		{name: "declared maximum wins", conf: []string{"metasystem.runtimes=fake"}, declared: 200, want: 230},
		{name: "dispatch.cap-min raises", conf: []string{"dispatch.cap-min=180"}, want: 210},
		{name: "fence.job-cap-min raises", conf: []string{"fence.job-cap-min=240"}, want: 270},
		{
			name: "the largest cap.min key wins",
			conf: []string{"cap.min.codex.gpt-5.6=300", "cap.min.implementer.codex.gpt-5.6=90"},
			want: 330,
		},
		{
			name:    "raw environment cap raises",
			conf:    []string{"metasystem.runtimes=fake"},
			environ: []string{"METASYSTEM_CAP_MIN_CODEX_GPT_5_6=400", "PATH=/bin"},
			want:    430,
		},
		{
			name:    "malformed dispatch.cap-min refuses by name",
			conf:    []string{"dispatch.cap-min=soon"},
			refusal: "dispatch.cap-min must be a positive integer",
		},
		{
			name:    "malformed cap.min key refuses by its own name",
			conf:    []string{"cap.min.codex.gpt-5.6=big"},
			refusal: "cap.min.codex.gpt-5.6 must be a positive integer",
		},
		{
			name:    "malformed environment cap refuses by env name",
			conf:    []string{"metasystem.runtimes=fake"},
			environ: []string{"METASYSTEM_CAP_MIN_X=zero"},
			refusal: "METASYSTEM_CAP_MIN_X must be a positive integer",
		},
		{
			name:    "an ambiguous conf refuses instead of picking a winner",
			conf:    []string{"dispatch.cap-min=60", "dispatch.cap-min=90"},
			refusal: "duplicate metasystem configuration key: dispatch.cap-min",
		},
		{
			name:    "a duplicate cap.min key refuses through the enumeration",
			conf:    []string{"cap.min.codex.gpt-5.6=60", "cap.min.codex.gpt-5.6=90"},
			refusal: "duplicate metasystem configuration key: cap.min.codex.gpt-5.6",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := DeriveCeiling(ceilingConf(t, c.conf...), c.declared, c.environ)
			if c.refusal != "" {
				if err == nil || !strings.Contains(err.Error(), c.refusal) {
					t.Fatalf("want refusal %q, got %d err=%v", c.refusal, got, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("ceiling = %d, want %d", got, c.want)
			}
		})
	}
}
