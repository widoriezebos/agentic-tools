package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adopt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func init() {
	registerIdempotency("system adopt", idemStateful,
		"the target is already this template's installation at the same commit: success, nothing written (the owner's recognition is TestAdoptGitIntegrationDefaultInstallsTheWholePayload's re-adoption leg)",
		witnessSystemAdoptRepeat)
}

// witnessSystemAdoptRepeat runs system adopt twice through the router. The
// adoption owner stands in by its contract: the first call installs (one
// marker file), a call against an installed target answers Already. The
// router must render the repeat as unchanged, exit 0, and leave the target's
// bytes as they were.
func witnessSystemAdoptRepeat(t *testing.T) {
	installation := t.TempDir()
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatal(err)
	}
	if marker, err := os.OpenFile(filepath.Join(installation, "metasystem.conf"), os.O_CREATE|os.O_WRONLY, 0o644); err != nil {
		t.Fatal(err)
	} else {
		marker.Close()
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	target := filepath.Join(cwd, "app")
	const sha = "0123456789abcdef0123456789abcdef01234567"
	calls := 0
	owners := defaultIntentOwners()
	owners.resolver = stateroot.NewResolver(func(path string) (string, error) { return path, nil }, os.Executable)
	owners.adopt = func(options adopt.Options) (adopt.Result, error) {
		calls++
		marker := filepath.Join(options.Target, "wow.md")
		if _, err := os.Stat(marker); err == nil {
			return adopt.Result{Already: true, SHA: sha, Target: options.Target}, nil
		}
		if err := os.MkdirAll(options.Target, 0o755); err != nil {
			return adopt.Result{}, err
		}
		if err := os.WriteFile(marker, []byte("installed\n"), 0o644); err != nil {
			return adopt.Result{}, err
		}
		return adopt.Result{SHA: sha, Target: options.Target}, nil
	}
	command := mustIntentCommand(t, "system adopt")
	run := func() (int, intentResult) {
		var stdout, stderr bytes.Buffer
		code := runIntentIn(command, []string{"app", "--repo", installation, "--json"}, &stdout, &stderr, cwd, owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("not one JSON result: %v\n%s%s", err, stdout.String(), stderr.String())
		}
		return code, result
	}
	if code, first := run(); code != 0 || first.Outcome != intentConfirmed {
		t.Fatalf("first adoption = %d %+v", code, first)
	}
	before := idemTreeDigest(t, target)
	code, again := run()
	if code != 0 || again.Outcome != intentUnchanged || calls != 2 {
		t.Fatalf("repeated adoption = %d %+v calls=%d", code, again, calls)
	}
	idemSameTree(t, "a repeated adoption", before, idemTreeDigest(t, target))
}
