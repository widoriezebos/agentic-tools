package goal

import (
	"strings"
	"testing"
)

func rootGolden() *RootRecord {
	return &RootRecord{
		Identity:       "01J5X000000000000000000000",
		FormatVersion:  "1",
		SyncMode:       SyncRemote,
		MigrationEpoch: "2026-08-20T00:00:00Z",
		ManifestDigest: "266f3dc6a7c3c2cbb884349e54fca0c1f0f33db9b188a6d39ddd245f35e11a94",
		MigrationMode:  "manifest",
		Revision:       1,
		Legacy:         []string{"Root-level prose the legacy parser tolerated."},
		History: []HistoryLine{
			{At: "2026-08-20T00:31:00Z", Opid: "01J5X0000000000000000000A0-mac-studio-1a2b3c4d", Verb: "migrate", Actor: "mac-studio+session-a", Keep: -1},
		},
	}
}

func TestGoldenRootRecordRoundTrips(t *testing.T) {
	bytes1 := RenderRoot(rootGolden())
	parsed, problems := ParseRoot(bytes1)
	if len(problems) != 0 {
		t.Fatalf("golden root must parse clean, got %v", problems)
	}
	if string(RenderRoot(parsed)) != string(bytes1) {
		t.Fatal("root record is not a render fixed point")
	}
	if parsed.Identity != "01J5X000000000000000000000" || parsed.SyncMode != SyncRemote {
		t.Fatalf("identity/mode lost: %+v", parsed)
	}
	if len(parsed.Legacy) != 1 {
		t.Fatalf("root LegacyNotes lost: %v", parsed.Legacy)
	}
}

func TestRootWithGoalFreeRoundTrips(t *testing.T) {
	r := rootGolden()
	r.Free = &FreeRecord{Declared: "2026-08-21T00:00:00Z", Origin: "main", Digest: "abc123"}
	parsed, problems := ParseRoot(RenderRoot(r))
	if len(problems) != 0 {
		t.Fatalf("root with Goal-free must parse clean, got %v", problems)
	}
	if parsed.Free == nil || parsed.Free.Digest != "abc123" {
		t.Fatalf("Goal-free lost: %+v", parsed.Free)
	}
}

func TestRootRefusalsAreNamed(t *testing.T) {
	r := rootGolden()
	r.Identity = ""
	_, problems := ParseRoot(RenderRoot(r))
	if !problemsContain(problems, "missing Identity") {
		t.Fatalf("missing identity must refuse by name, got %v", problems)
	}

	r = rootGolden()
	r.SyncMode = "sometimes"
	_, problems = ParseRoot(RenderRoot(r))
	if !problemsContain(problems, "not remote|local") {
		t.Fatalf("bad sync mode must refuse by name, got %v", problems)
	}

	tampered := strings.Replace(string(RenderRoot(rootGolden())), "manifest", "bare", 1)
	_, problems = ParseRoot([]byte(tampered))
	if !problemsContain(problems, "Integrity mismatch") {
		t.Fatalf("tampered root must fail Integrity, got %v", problems)
	}
}

func problemsContain(problems []Problem, substr string) bool {
	for _, p := range problems {
		if strings.Contains(string(p), substr) {
			return true
		}
	}
	return false
}
