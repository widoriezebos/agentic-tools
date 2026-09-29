package receipt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLedger(t *testing.T, opts Options, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(opts.File), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(opts.File, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

const (
	lineA = "1788000001|2026-08-30T00:00:01Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|delegate=claude:opus:chain-a|note=a"
	lineB = "1788000002|2026-08-30T00:00:02Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|delegate=claude:opus:chain-b|note=b"
	lineC = "1788000003|2026-08-30T00:00:03Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|delegate=claude:opus:chain-c|note=c"
)

// The retro workflow covers exactly what it read: a line that arrives
// between the read and the marker stays uncovered and is handed back by the
// next read; a legacy RETRO row without covered= covers nothing.
func TestRetroCoversExactlyTheLinesItRead(t *testing.T) {
	t.Parallel()
	opts := baseOptions(t)
	writeLedger(t, opts, lineA, "1787000000|2026-08-20T00:00:00Z|RETRO|note=a legacy retro", lineB)
	read := CoverageOf(mustRead(t, opts.File))
	if len(read.Lines) != 2 || read.Lines[0] != lineA || read.Lines[1] != lineB {
		t.Fatalf("a RETRO row without covered= covers nothing: %+v", read.Lines)
	}
	shown := Uncovered(opts)
	if shown.Code != 0 || len(shown.Out) != 3 || shown.Out[0] != lineA || !strings.Contains(shown.Out[2], read.Token) {
		t.Fatalf("status --uncovered prints the lines and the token: %+v", shown)
	}
	// A line lands after the read and before the marker.
	appended, err := os.OpenFile(opts.File, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	appended.WriteString(lineC + "\n")
	appended.Close()
	marker := opts
	marker.Summary, marker.Covered = "kept 3", read.Token
	if result := Retro(marker); result.Code != 0 {
		t.Fatalf("the marker with a current token: %+v", result)
	}
	after := CoverageOf(mustRead(t, opts.File))
	if len(after.Lines) != 1 || after.Lines[0] != lineC {
		t.Fatalf("exactly the line after the read is handed back: %+v", after.Lines)
	}
	ledger := mustRead(t, opts.File)
	if !strings.Contains(ledger, "|covered="+LineDigest(lineA)+","+LineDigest(lineB)+"\n") {
		t.Fatalf("the RETRO row names the digests it covered:\n%s", ledger)
	}
}

func TestRetroWithAStaleTokenWritesNothing(t *testing.T) {
	t.Parallel()
	opts := baseOptions(t)
	writeLedger(t, opts, lineA, lineB)
	stale := CoverageToken([]string{LineDigest(lineB)})
	before := mustRead(t, opts.File)
	marker := opts
	marker.Summary, marker.Covered = "kept 3", stale
	result := Retro(marker)
	if result.Code == 0 || !strings.Contains(strings.Join(result.Err, " "), "metasystem receipt status --uncovered") {
		t.Fatalf("a stale token is refused naming the re-read: %+v", result)
	}
	if mustRead(t, opts.File) != before {
		t.Fatal("a stale token writes nothing")
	}
}

// Coverage is per line, never a position: a merged ledger whose lines are
// reordered inside the covered set classifies the same.
func TestCoverageIsPerLineNotAPosition(t *testing.T) {
	t.Parallel()
	opts := baseOptions(t)
	writeLedger(t, opts, lineA, lineB)
	read := CoverageOf(mustRead(t, opts.File))
	marker := opts
	marker.Summary, marker.Covered = "covered two", read.Token
	if result := Retro(marker); result.Code != 0 {
		t.Fatal(result)
	}
	retroLine := strings.TrimSpace(strings.Split(mustRead(t, opts.File), "\n")[2])
	writeLedger(t, opts, retroLine, lineC, lineB, lineA)
	merged := CoverageOf(mustRead(t, opts.File))
	if len(merged.Lines) != 1 || merged.Lines[0] != lineC {
		t.Fatalf("a reordered, merged ledger classifies the same: %+v", merged.Lines)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
