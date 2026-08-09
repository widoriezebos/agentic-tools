package registry

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The framing tests state REG-1's contract as executable rows: every
// tail state the repair rule names, the exact bytes the repair may
// write, and the one shape reading must refuse.

func appendOrFail(t *testing.T, path, payload string) {
	t.Helper()
	if err := AppendFrame(path, []byte(payload)); err != nil {
		t.Fatalf("AppendFrame(%q): %v", payload, err)
	}
}

func records(t *testing.T, path string) []map[string]any {
	t.Helper()
	frames, err := ReadFrames(path)
	if err != nil {
		t.Fatalf("ReadFrames: %v", err)
	}
	var out []map[string]any
	for _, frame := range frames {
		if frame.Record != nil {
			out = append(out, frame.Record)
		}
	}
	return out
}

func TestAppendFrameTailRepair(t *testing.T) {
	cases := []struct {
		name string
		// seed is the file's pre-append content; "" with create=false
		// means the file does not exist yet.
		seed        string
		create      bool
		wantRecords int // parsed records after appending one payload
		wantTorn    int // torn markers among them
	}{
		{name: "missing file", create: false, wantRecords: 1, wantTorn: 0},
		{name: "empty file", seed: "", create: true, wantRecords: 1, wantTorn: 0},
		{name: "well-formed tail", seed: "{\"event\":\"armed\"}\n", create: true, wantRecords: 2, wantTorn: 0},
		{
			// REG-1 first part: the record was fully written and only
			// its newline was lost — completed, never fenced.
			name: "valid final line without newline", seed: "{\"event\":\"armed\"}", create: true,
			wantRecords: 2, wantTorn: 0,
		},
		{
			// REG-1 second part: a torn fragment is terminated and
			// fenced before the payload.
			name: "torn fragment without newline", seed: "{\"event\":\"arm", create: true,
			wantRecords: 2, wantTorn: 1,
		},
		{
			// The fragment already got its newline (a repair crashed
			// between termination and marker): the marker must still
			// be written or the next reader sees corruption.
			name: "terminated garbage line", seed: "{\"event\":\"arm\n", create: true,
			wantRecords: 2, wantTorn: 1,
		},
		{
			// A crashed repair that already landed its marker: re-run
			// is idempotent in outcome — one more marker at worst,
			// and here the tail is the valid marker so none is added.
			name:   "tail is a torn marker",
			seed:   "{\"event\":\"arm\n{\"schemaVersion\":1,\"event\":\"torn\",\"checkoutPath\":\"\",\"at\":\"x\"}\n",
			create: true, wantRecords: 2, wantTorn: 1,
		},
	}
	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "armed-checkouts.jsonl")
			if row.create {
				if err := os.WriteFile(path, []byte(row.seed), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			appendOrFail(t, path, `{"event":"arming","ownerTag":"t"}`)

			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(string(content), "\n") {
				t.Fatalf("file does not end with a newline after append: %q", content)
			}
			if !strings.HasPrefix(string(content), row.seed) {
				t.Fatalf("append rewrote existing bytes: %q does not start with seed %q", content, row.seed)
			}
			parsed := records(t, path)
			if len(parsed) != row.wantRecords {
				t.Fatalf("got %d records, want %d (content %q)", len(parsed), row.wantRecords, content)
			}
			torn := 0
			for _, record := range parsed {
				if record["event"] == TornEvent {
					torn++
				}
			}
			if torn != row.wantTorn {
				t.Fatalf("got %d torn markers, want %d (content %q)", torn, row.wantTorn, content)
			}
			last := parsed[len(parsed)-1]
			if last["event"] != "arming" {
				t.Fatalf("payload is not the final record: %v", last)
			}
		})
	}
}

func TestAppendFrameRefusesUnframablePayloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.jsonl")
	if err := AppendFrame(path, []byte("not json")); err == nil {
		t.Fatal("append accepted a non-JSON payload")
	}
	if err := AppendFrame(path, []byte("{\"a\":1}\n{\"b\":2}")); err == nil {
		t.Fatal("append accepted a multi-line payload")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("a refused append still created the registry file")
	}
}

func TestReadFramesRunTolerance(t *testing.T) {
	torn := `{"schemaVersion":1,"event":"torn","checkoutPath":"","at":"x"}`
	record := `{"schemaVersion":1,"event":"arming","checkoutPath":"/r","at":"x"}`
	cases := []struct {
		name    string
		content string
		corrupt bool
		valid   int // parsed records including torn markers
	}{
		{name: "clean file", content: record + "\n", valid: 1},
		{name: "trailing fragment", content: record + "\n" + `{"torn`, valid: 1},
		{
			// The final line is valid but unterminated: REG-1 says the
			// record was fully written, so it counts.
			name: "unterminated valid tail", content: record + "\n" + record, valid: 2,
		},
		{name: "fenced garbage", content: "garbage\n" + torn + "\n" + record + "\n", valid: 2},
		{
			// Two garbage lines from a twice-crashed repair, one fence:
			// tolerated — the rule is per-run, not per-line.
			name: "garbage run with one fence", content: "g1\ng2\n" + torn + "\n" + record + "\n", valid: 2,
		},
		{name: "trailing garbage after records", content: record + "\n" + "garbage", valid: 1},
		{
			// The one intolerable shape (REG-5): a valid record follows
			// garbage with no intervening marker.
			name: "unfenced garbage before a record", content: "garbage\n" + record + "\n", corrupt: true,
		},
		{
			name:    "unfenced garbage between records",
			content: record + "\n" + "garbage\n" + record + "\n",
			corrupt: true,
		},
	}
	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "registry.jsonl")
			if err := os.WriteFile(path, []byte(row.content), 0o644); err != nil {
				t.Fatal(err)
			}
			frames, err := ReadFrames(path)
			if row.corrupt {
				var corruption *CorruptionError
				if !errors.As(err, &corruption) {
					t.Fatalf("want CorruptionError, got %v (frames %v)", err, frames)
				}
				return
			}
			if err != nil {
				t.Fatalf("ReadFrames: %v", err)
			}
			valid := 0
			for _, frame := range frames {
				if frame.Record != nil {
					valid++
				}
			}
			if valid != row.valid {
				t.Fatalf("got %d valid records, want %d", valid, row.valid)
			}
		})
	}
}

func TestReadFramesMissingFileIsEmptyRegistry(t *testing.T) {
	frames, err := ReadFrames(filepath.Join(t.TempDir(), "never-written.jsonl"))
	if err != nil || frames != nil {
		t.Fatalf("missing registry must read as empty: frames=%v err=%v", frames, err)
	}
}

// TestRepairThenAppendConverges replays SLC-R6-009's crash storm: whatever
// prefix of the repair a crash left behind, the next append leaves a file
// whose reader tolerates everything and whose final record is the payload.
func TestRepairThenAppendConverges(t *testing.T) {
	fragment := `{"event":"arm`
	torn := `{"schemaVersion":1,"event":"torn","checkoutPath":"","at":"x"}`
	prefixes := []string{
		fragment,
		fragment + "\n",
		fragment + "\n" + torn,
		fragment + "\n" + torn + "\n",
	}
	for i, prefix := range prefixes {
		path := filepath.Join(t.TempDir(), "registry.jsonl")
		if err := os.WriteFile(path, []byte(prefix), 0o644); err != nil {
			t.Fatal(err)
		}
		appendOrFail(t, path, `{"event":"armed"}`)
		parsed := records(t, path)
		if len(parsed) == 0 || parsed[len(parsed)-1]["event"] != "armed" {
			t.Fatalf("crash prefix %d: payload did not survive repair: %v", i, parsed)
		}
	}
}
