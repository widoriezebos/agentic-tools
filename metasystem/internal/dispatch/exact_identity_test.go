package dispatch

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type fixedStartReader struct {
	exact identity.Exact
	state identity.Liveness
}

type recordedGroupProofGrant bool

func (g recordedGroupProofGrant) AllowsRecordedGroupProof() bool { return bool(g) }

func (r fixedStartReader) Probe(int64) (identity.Exact, identity.Liveness, error) {
	return r.exact, r.state, nil
}

func nativeTestExact(pid, generation int64) identity.Exact {
	if runtime.GOOS == "linux" {
		return identity.Exact{
			Pid: pid, StartedAt: time.Unix(100, 0),
			StartTicks: 7000 + generation, BootID: "boot-a",
		}
	}
	// Ported: distinct generations differ by whole SECONDS — main's
	// darwin identity has second resolution, and a sub-second recycle
	// is indistinguishable by design (the wip's microsecond token had
	// no home to land in).
	return identity.Exact{Pid: pid, StartedAt: time.Unix(100_000+generation, 0)}
}

func assertNativeIdentityFields(t *testing.T, value map[string]any, generation int64) {
	t.Helper()
	if runtime.GOOS == "linux" {
		if !looseEqual(value["pidStartTicks"], 7000+generation) || value["bootId"] != "boot-a" {
			t.Fatalf("linux identity fields missing: %+v", value)
		}
		if value["pidStartedAtExactMicro"] != nil {
			t.Fatalf("linux record carried the darwin representation: %+v", value)
		}
		return
	}
	// Ported semantics: main's records carry no darwin microsecond
	// token — the fork-captured start SECOND is darwin's native shape,
	// so the assertion is exactly the absence of every exact extension.
	if value["pidStartedAtExactMicro"] != nil || value["pidStartTicks"] != nil || value["bootId"] != nil {
		t.Fatalf("darwin record carried an exact extension main does not define: %+v", value)
	}
	if !looseEqual(value["pidStartedAt"], 100_000+generation) {
		t.Fatalf("darwin identity second missing: %+v", value)
	}
}

func TestOwnershipPatchPersistsTheNativeExactIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ownership.json")
	exact := nativeTestExact(41, 1)
	if err := BuildOwnershipPatch(path, 41, 41, "job-tag", "2026-08-27T10:00:00Z", 1234, fixedStartReader{exact: exact, state: identity.Alive}); err != nil {
		t.Fatal(err)
	}
	patch := readJSONFile(t, path)
	assertNativeIdentityFields(t, patch, 1)
	proof, ok := patch["ownershipProof"].(map[string]any)
	if !ok {
		t.Fatalf("ownership proof missing: %+v", patch)
	}
	assertNativeIdentityFields(t, proof, 1)
}

func TestRecordCASAcceptsOneNativeExactOwnershipWrite(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	writeJSONFile(t, jobs, "job-a.json", map[string]any{
		"jobId": "job-a", "status": "pending", "phase": "handshake", "endedAt": nil,
		"instanceTag": "job-tag", "pid": nil, "pidStartedAt": nil,
	})
	patch := filepath.Join(t.TempDir(), "ownership.json")
	if err := BuildOwnershipPatch(patch, 41, 41, "job-tag", "2026-08-27T10:00:00Z", 1234,
		fixedStartReader{exact: nativeTestExact(41, 1), state: identity.Alive}); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordCAS(root, "job-a", "pending", "pending", patch); err != nil {
		t.Fatalf("native exact ownership write: %v", err)
	}
	record := readJSONFile(t, filepath.Join(jobs, "job-a.json"))
	assertNativeIdentityFields(t, record, 1)
	if _, err := RecordCAS(root, "job-a", "pending", "pending", patch); err == nil {
		t.Fatal("a second ownership write must not rewrite the primary identity")
	}
}

func TestRecordCASRefusesNewSecondsOnlyOwnership(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	writeJSONFile(t, jobs, "job-a.json", map[string]any{
		"jobId": "job-a", "status": "pending", "phase": "handshake", "endedAt": nil,
		"instanceTag": "job-tag", "pid": nil, "pidStartedAt": nil,
	})
	secondsOnly := writeJSONFile(t, t.TempDir(), "seconds.json", map[string]any{
		"pid": 41, "pidStartedAt": 100, "pgid": 41,
		"ownershipProof": map[string]any{
			"pid": 41, "pidStartedAt": 100, "pgid": 41, "instanceTag": "job-tag",
			"provenAt": "2026-08-27T10:00:00Z", "source": "trusted-launcher",
		},
	})
	_, casErr := RecordCAS(root, "job-a", "pending", "pending", secondsOnly)
	if runtime.GOOS == "linux" {
		// On linux the ticks+bootID pair is the native shape; a
		// seconds-only write is weaker than the platform's best and
		// refuses.
		if casErr == nil {
			t.Fatal("a linux ownership write without the ticks+bootID pair must be refused")
		}
		return
	}
	// Ported semantics: darwin's native shape IS the start second, so
	// the same write is the strongest darwin can record and stands.
	if casErr != nil {
		t.Fatalf("darwin seconds ownership is the native shape: %v", casErr)
	}
}

func TestRecordedGroupProofRequiresFixtureAuthorityAndExactIdentity(t *testing.T) {
	ref := nativeTestExact(41, 1).Ref()
	record := exactIdentityFields(ref)
	record["runtime"] = "fake"
	record["instanceTag"] = "job-tag"
	record["pgid"] = int64(41)
	proof := exactIdentityFields(ref)
	proof["pgid"] = int64(41)
	proof["instanceTag"] = "job-tag"
	proof["source"] = "trusted-launcher"
	proof["provenAt"] = "2026-08-27T10:00:00Z"
	record["ownershipProof"] = proof
	path := writeJSONFile(t, t.TempDir(), "job.json", record)

	if matches, err := RecordedGroupProofMatches(path, 41, "job-tag", recordedGroupProofGrant(true)); err != nil || !matches {
		t.Fatalf("authorized exact launch proof: matches=%v err=%v", matches, err)
	}
	if matches, err := RecordedGroupProofMatches(path, 41, "job-tag", recordedGroupProofGrant(false)); err != nil || matches {
		t.Fatalf("unauthorized launch proof: matches=%v err=%v", matches, err)
	}

	secondsOnly := writeJSONFile(t, t.TempDir(), "legacy.json", map[string]any{
		"runtime": "fake", "instanceTag": "job-tag", "pid": 41,
		"pidStartedAt": 100, "pgid": 41,
		"ownershipProof": map[string]any{
			"pid": 41, "pidStartedAt": 100, "pgid": 41,
			"instanceTag": "job-tag", "source": "trusted-launcher",
		},
	})
	if matches, err := RecordedGroupProofMatches(secondsOnly, 41, "job-tag", recordedGroupProofGrant(true)); err != nil || matches {
		t.Fatalf("seconds-only launch proof: matches=%v err=%v", matches, err)
	}
}

func TestCustodyAddPersistsAndDeduplicatesTheNativeExactIdentity(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	writeJSONFile(t, jobs, "job-a.json", map[string]any{
		"jobId": "job-a", "status": "running", "instanceTag": "job-tag",
		"custodyProcesses": []any{},
	})
	first := nativeTestExact(41, 1)
	group := func(int64) (int64, error) { return 41, nil }
	if err := CustodyAdd(root, "job-a", 41, fixedStartReader{exact: first, state: identity.Alive}, group); err != nil {
		t.Fatal(err)
	}
	if err := CustodyAdd(root, "job-a", 41, fixedStartReader{exact: first, state: identity.Alive}, group); err != nil {
		t.Fatal(err)
	}
	recycled := nativeTestExact(41, 2)
	if err := CustodyAdd(root, "job-a", 41, fixedStartReader{exact: recycled, state: identity.Alive}, group); err != nil {
		t.Fatal(err)
	}
	record := readJSONFile(t, filepath.Join(jobs, "job-a.json"))
	items := record["custodyProcesses"].([]any)
	if len(items) != 2 {
		t.Fatalf("custody entries = %d, want exact duplicate collapsed and recycled pid retained", len(items))
	}
	assertNativeIdentityFields(t, items[0].(map[string]any), 1)
	assertNativeIdentityFields(t, items[1].(map[string]any), 2)
}
