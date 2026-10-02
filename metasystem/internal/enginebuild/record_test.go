package enginebuild

import (
	"debug/buildinfo"
	"errors"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeStampFile(t *testing.T, content string) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "engine")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

// TestReadStampRoundTripsEveryStampShapeInUse writes each stamp's record into
// otherwise opaque bytes and reads it back, including a record split across
// the scanner's chunk boundary.
func TestReadStampRoundTripsEveryStampShapeInUse(t *testing.T) {
	t.Parallel()
	commit := strings.Repeat("0123456789", 4)
	shapes := []string{commit, DevelopmentStamp(commit), "unknown", "dev", "witness-abcdef012345", "fixture-shared-build", strings.Repeat("x", 64)}
	for _, stamp := range shapes {
		if !ValidStamp(stamp) {
			t.Fatalf("ValidStamp(%q) = false", stamp)
		}
		if got, ok := ParseStampRecord(StampRecord(stamp)); !ok || got != stamp {
			t.Fatalf("ParseStampRecord(StampRecord(%q)) = %q, %v", stamp, got, ok)
		}
		for _, offset := range []int{0, 7, 64<<10 - 10, 64<<10 + 3} {
			file := writeStampFile(t, strings.Repeat("\x00", offset)+StampRecord(stamp)+"\x00tail")
			if got, err := ReadStamp(file); err != nil || got != stamp {
				t.Fatalf("ReadStamp(%q at %d) = %q, %v", stamp, offset, got, err)
			}
		}
	}
	for _, bad := range []string{"", strings.Repeat("x", 65), "has space", "semi;colon", "under_score"} {
		if ValidStamp(bad) {
			t.Fatalf("ValidStamp(%q) = true", bad)
		}
	}
}

func TestReadStampAcceptsOnlyAgreeingGrammarValidRecords(t *testing.T) {
	t.Parallel()
	sentinel := stampSentinel()
	tests := []struct {
		name    string
		content string
		want    string
		wantErr error
	}{
		{name: "two agreeing records read one", content: StampRecord("abc") + "\x00\x00" + StampRecord("abc"), want: "abc"},
		{name: "two disagreeing records read none", content: StampRecord("abc") + "\x00" + StampRecord("abd"), wantErr: ErrStampDisagreement},
		{name: "disagreement across chunks reads none", content: StampRecord("abc") + strings.Repeat("\x00", 200<<10) + StampRecord("abd"), wantErr: ErrStampDisagreement},
		{name: "unterminated token is ignored", content: sentinel + "abc\x00" + StampRecord("good"), want: "good"},
		{name: "empty token is ignored", content: sentinel + ";" + StampRecord("good"), want: "good"},
		{name: "over-long token is ignored", content: sentinel + strings.Repeat("a", 65) + ";" + StampRecord("good"), want: "good"},
		{name: "uppercase sentinel is not a record", content: stampSentinelUpper + "abc;"},
		{name: "no record and no build info", content: "plain bytes"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := ReadStamp(writeStampFile(t, test.content))
			if got != test.want || !errors.Is(err, test.wantErr) || (test.wantErr == nil && err != nil) {
				t.Fatalf("ReadStamp = %q, %v; want %q, %v", got, err, test.want, test.wantErr)
			}
		})
	}
}

func TestStampLinkerFlagsLinksTheLegacyVariableAndTheRecord(t *testing.T) {
	t.Parallel()
	want := "-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp=witness-abcdef012345" +
		" -X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStampRecord=" + stampSentinel() + "witness-abcdef012345;"
	if got := StampLinkerFlags("witness-abcdef012345"); got != want {
		t.Fatalf("StampLinkerFlags = %q, want %q", got, want)
	}
}

func TestLinkedStampReconcilesTheRecordWithTheLegacyValue(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		record, legacy, want string
		wantErr              error
	}{
		{record: "", legacy: "dev", want: "dev"},
		{record: "", legacy: "old", want: "old"},
		{record: StampRecord("new"), legacy: "dev", want: "new"},
		{record: StampRecord("new"), legacy: "new", want: "new"},
		{record: StampRecord("new"), legacy: "old", wantErr: ErrStampDisagreement},
	} {
		got, err := LinkedStamp(test.record, test.legacy, "dev")
		if got != test.want || !errors.Is(err, test.wantErr) || test.wantErr == nil && err != nil {
			t.Fatalf("LinkedStamp(%q, %q) = %q, %v; want %q, %v", test.record, test.legacy, got, err, test.want, test.wantErr)
		}
	}
}

// baseReader is the stamp reader as it stood before the record
// (internal/seat/launch/host.go and internal/steward/identity.go at
// 5401952777a0), kept verbatim as the old-launcher witness: an older
// sequencer or steward must still read what the new builder links.
func baseReader(file *os.File) string {
	info, err := buildinfo.Read(file)
	if err != nil {
		return ""
	}
	const assignment = "supervise.BuildStamp="
	for _, setting := range info.Settings {
		if setting.Key != "-ldflags" {
			continue
		}
		at := strings.Index(setting.Value, assignment)
		if at < 0 {
			continue
		}
		value := setting.Value[at+len(assignment):]
		if end := strings.IndexAny(value, " \t\r\n\"'"); end >= 0 {
			value = value[:end]
		}
		return strings.TrimSpace(value)
	}
	return ""
}

// TestReadStampFromRealEngineBuilds builds a probe linking internal/supervise
// the ways engines are built: dual-stamped under -trimpath (which drops the
// -ldflags build setting, so only the record reads), dual-stamped untrimmed
// (which the pre-record reader still reads), untrimmed with only the
// pre-record -X (the fallback), and with nothing linked (the compiled reader
// itself must not look like a record). The probe prints the BuildStamp that
// init derived.
func TestReadStampFromRealEngineBuilds(t *testing.T) {
	t.Parallel()
	commit := strings.Repeat("ab", 20)
	legacyOnly := "-X " + LegacyStampVariable + "=" + commit
	tests := []struct {
		name      string
		trimpath  bool
		ldflags   string
		wantStamp string
		wantBase  string // the fallback, the pre-record Go reader and the old shell reader
		wantRun   string // "" when init must refuse
		wantErr   error
	}{
		{name: "both forms with different values read none and init refuses", ldflags: legacyOnly + " -X " + StampRecordVariable + "=" + StampRecord("other"), wantBase: commit, wantErr: ErrStampDisagreement},
		{name: "both forms under trimpath read the record", trimpath: true, ldflags: StampLinkerFlags(commit), wantStamp: commit, wantRun: commit},
		{name: "both forms untrimmed read the same through the record and the fallback", ldflags: StampLinkerFlags(DevelopmentStamp(commit)), wantStamp: DevelopmentStamp(commit), wantBase: DevelopmentStamp(commit), wantRun: DevelopmentStamp(commit)},
		{name: "legacy flag untrimmed reads through the fallback", ldflags: legacyOnly, wantStamp: commit, wantBase: commit, wantRun: commit},
		{name: "nothing linked", wantStamp: "", wantRun: "dev"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			output := filepath.Join(t.TempDir(), "probe")
			args := []string{"build", "-buildvcs=false"}
			if test.trimpath {
				args = append(args, "-trimpath")
			}
			if test.ldflags != "" {
				args = append(args, "-ldflags", test.ldflags)
			}
			args = append(args, "-o", output, "./testdata/stampprobe")
			build := testenv.Go(args...)
			build.Env = append(os.Environ(), "GOFLAGS=", "CGO_ENABLED=0")
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("go %v: %v\n%s", args, err, out)
			}
			file, err := os.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if got, err := ReadStamp(file); !errors.Is(err, test.wantErr) || test.wantErr == nil && err != nil || got != test.wantStamp {
				t.Fatalf("ReadStamp = %q, %v; want %q, %v", got, err, test.wantStamp, test.wantErr)
			}
			if got := legacyStamp(file); got != test.wantBase {
				t.Fatalf("fallback = %q, want %q", got, test.wantBase)
			}
			if got := baseReader(file); got != test.wantBase {
				t.Fatalf("pre-record reader = %q, want %q", got, test.wantBase)
			}
			// The old-shaped shell reader of land-fixtures.sh before A1.
			shell := exec.Command("bash", "-c", `go version -m "$1" | sed -n 's/.*BuildStamp=\([a-z0-9-]*\).*/\1/p'`, "bash", output)
			if got, err := shell.Output(); err != nil || strings.TrimSpace(string(got)) != test.wantBase {
				t.Fatalf("old shell reader = %q (%v), want %q", got, err, test.wantBase)
			}
			probe := exec.Command(output)
			var stderr strings.Builder
			probe.Stderr = &stderr
			ran, err := probe.Output()
			if test.wantRun == "" {
				if err == nil || !strings.Contains(stderr.String(), "refusing to run a mis-built engine") {
					t.Fatalf("probe ran with disagreeing stamps: %q %v; stderr %q", ran, err, stderr.String())
				}
				return
			}
			if err != nil || strings.TrimSpace(string(ran)) != test.wantRun {
				t.Fatalf("probe printed %q (%v), want %q", ran, err, test.wantRun)
			}
		})
	}
}
