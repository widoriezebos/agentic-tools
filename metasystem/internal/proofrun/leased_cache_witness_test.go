package proofrun

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// TestLeasedLayoutGoTestCacheWitness is an opt-in adapter integration
// witness: one proof run of real testing groups through the real group runner
// (runTestGroup and its shard runner) under the v2 leased layout, with real
// git and go. Two invocations in a row, each its own process and scratch run
// like two proof runs, show whether the second's packages are served by Go's
// test cache. It is skipped unless METASYSTEM_LEASE_WITNESS_OUT names the
// JSON summary file; the other inputs are:
//
//	METASYSTEM_LEASE_WITNESS_CONTROL  control root holding the scratch store
//	METASYSTEM_LEASE_WITNESS_PROJECT  the checkout (repository top)
//	METASYSTEM_LEASE_WITNESS_TREE     candidate tree id of that checkout
//	METASYSTEM_LEASE_WITNESS_GROUPS   comma-separated testing.json group ids
//	METASYSTEM_LEASE_WITNESS_WORKERS  attempt worker allowance (default 4)
//
// METASYSTEM_LEASE_WITNESS_<NAME> overrides one inherited variable (HOME,
// TMPDIR, GOCACHE, STATICCHECK_CACHE, ...): testenv has already replaced this
// process's HOME and TMPDIR, and a real worker inherits the caller's.
func TestLeasedLayoutGoTestCacheWitness(t *testing.T) {
	out := os.Getenv("METASYSTEM_LEASE_WITNESS_OUT")
	if out == "" {
		t.Skip("opt-in witness: METASYSTEM_LEASE_WITNESS_OUT is unset")
	}
	control, project := os.Getenv("METASYSTEM_LEASE_WITNESS_CONTROL"), os.Getenv("METASYSTEM_LEASE_WITNESS_PROJECT")
	tree, groupIDs := os.Getenv("METASYSTEM_LEASE_WITNESS_TREE"), strings.Split(os.Getenv("METASYSTEM_LEASE_WITNESS_GROUPS"), ",")
	workers, err := strconv.Atoi(os.Getenv("METASYSTEM_LEASE_WITNESS_WORKERS"))
	if err != nil || workers < 1 {
		workers = 4
	}
	contract, err := testpolicy.Load(filepath.Join(project, "metasystem", "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	groups := map[string]testpolicy.Group{}
	for _, group := range contract.Groups {
		groups[group.ID] = group
	}
	base := []string{"GOFLAGS=-mod=readonly -trimpath", "GOTOOLCHAIN=local"}
	for _, name := range []string{"PATH", "HOME", "TMPDIR", "GOCACHE", "STATICCHECK_CACHE", "GOMODCACHE", "GOPATH", "GOROOT", "LANG", "TZ"} {
		if value, ok := os.LookupEnv("METASYSTEM_LEASE_WITNESS_" + name); ok {
			base = append(base, name+"="+value)
		} else if value, ok := os.LookupEnv(name); ok {
			base = append(base, name+"="+value)
		}
	}
	// A diagnostic GODEBUG (gocachetest=1) reaches the groups only when asked.
	if value, ok := os.LookupEnv("METASYSTEM_LEASE_WITNESS_GODEBUG"); ok {
		base = append(base, "GODEBUG="+value)
	}
	base, err = identity.ExportRunOwner(base)
	if err != nil {
		t.Fatal(err)
	}
	run, err := CreateScratchRun(control)
	if err != nil {
		t.Fatal(err)
	}
	// The proof locators vary per run, as a real worker inherits them.
	base = append(base, "METASYSTEM_PROOF_CONTROL_ROOT="+control, "METASYSTEM_PROOF_ATTEMPT=attempt-"+run.ID(),
		identity.FixtureAttemptEnv+"=fixture-"+run.ID())
	request := TestRunRequest{ProjectRoot: project, CandidateTree: tree, Workers: workers, Environment: base, ResultSchemaVersion: TestResultSchemaVersion,
		LogRoot: filepath.Join(filepath.Dir(out), "logs-"+run.ID()), Contract: contract}
	request.Contract.SchemaVersion = contract.SchemaVersion
	for _, id := range groupIDs {
		if _, ok := groups[id]; !ok {
			t.Fatalf("group %q is not in the contract", id)
		}
		request.Plan.SelectedGroups = append(request.Plan.SelectedGroups, id)
	}
	request.BindScratch(run, nil)
	if err := PrepareScratchEnvironmentFor(&request, run, ScratchEnvironmentPolicyV2); err != nil {
		t.Fatal(errorsJoinCleanup(err, run))
	}
	ctx := WithScratchRun(context.Background(), run)
	type groupSummary struct {
		ID         string             `json:"id"`
		Status     string             `json:"status"`
		Reason     string             `json:"reason,omitempty"`
		DurationMS int64              `json:"durationMs"`
		Lease      string             `json:"lease"`
		Packages   map[string]string  `json:"packages"`
		Execution  []PackageExecution `json:"execution,omitempty"`
	}
	summary := struct {
		Run    string         `json:"run"`
		Groups []groupSummary `json:"groups"`
	}{Run: run.ID()}
	started := time.Now()
	for _, id := range groupIDs {
		result := runTestGroup(ctx, request, groups[id])
		summary.Groups = append(summary.Groups, groupSummary{ID: id, Status: result.Status, Reason: result.NotRunReason,
			DurationMS: result.DurationMS, Lease: request.ScratchEnvironment.leaseOf(id), Packages: goLogPackageOutcomes(result.LogPath),
			Execution: result.Execution})
	}
	t.Logf("lease witness wall time %s", time.Since(started))
	if err := run.Cleanup(nil); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, append(encoded, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func errorsJoinCleanup(err error, run *ScratchRun) error {
	if cleanupErr := run.Cleanup(nil); cleanupErr != nil {
		return fmt.Errorf("%w; cleanup: %v", err, cleanupErr)
	}
	return err
}

// goLogPackageOutcomes reads a group log's package summary lines: "cached"
// for a replayed pass, else the recorded elapsed time or terminal word.
func goLogPackageOutcomes(path string) map[string]string {
	outcomes := map[string]string{}
	file, err := os.Open(path)
	if err != nil {
		return outcomes
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var event goEvent
		if json.Unmarshal(scanner.Bytes(), &event) != nil || event.Test != "" {
			continue
		}
		fields := strings.Split(strings.TrimSuffix(event.Output, "\n"), "\t")
		if len(fields) >= 3 && (strings.TrimSpace(fields[0]) == "ok" || strings.TrimSpace(fields[0]) == "FAIL") && fields[1] == event.Package {
			value := fields[2]
			if strings.HasPrefix(value, "(cached)") {
				value = "cached"
			}
			if previous, seen := outcomes[event.Package]; seen && previous != value {
				value = previous + "+" + value
			}
			outcomes[event.Package] = value
		}
	}
	return outcomes
}
