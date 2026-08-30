package dispatch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestLaunchCapabilityIsBoundAndSingleUse(t *testing.T) {
	root := t.TempDir()
	job := "cap-job"
	raw := "one-shot-launch-word"
	digest := sha256.Sum256([]byte(raw))
	tag := "metasystem-job-cap-job-nonce"
	record := map[string]any{
		"jobId": job, "operationId": "operation-a", "status": "pending", "instanceTag": tag,
		"launchCapability": map[string]any{
			"digest": hex.EncodeToString(digest[:]), "jobId": job, "operationId": "operation-a",
			"instanceTag": tag, "adapterVerb": "dispatch", "status": "minted",
			"mintedAt": "2026-08-30T10:00:00Z",
		},
	}
	path := filepath.Join(root, "artifacts", "agents", "jobs", job+".json")
	if err := writeRecord(path, record); err != nil {
		t.Fatal(err)
	}
	supervisor := nativeTestExact(7001, 3)
	prober := fixedStartReader{exact: supervisor, state: identity.Alive}
	if err := ConsumeLaunchCapability(root, job, raw, "follow-up", tag, supervisor.Pid, prober); err == nil || !strings.Contains(err.Error(), "does not bind") {
		t.Fatalf("wrong adapter verb consumed the capability: %v", err)
	}
	if err := ConsumeLaunchCapability(root, job, raw, "dispatch", tag, supervisor.Pid, prober); err != nil {
		t.Fatal(err)
	}
	if err := ConsumeLaunchCapability(root, job, raw, "dispatch", tag, supervisor.Pid, prober); err == nil || !strings.Contains(err.Error(), "already consumed") {
		t.Fatalf("capability replay was accepted: %v", err)
	}
}

func TestDelegateClaimCapabilityIsPrivateBoundShortLivedAndSingleUse(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	raw, err := mintDelegateClaimCapability(root, DispatchModeFresh, now, bytes.NewReader(bytes.Repeat([]byte{0x2a}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	digest := claimCapabilityDigest(raw)
	path := delegateClaimCapabilityPath(root, digest)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("capability mode = %o, want no group or other access", info.Mode().Perm())
	}
	binding := DelegateClaimCapabilityBinding{
		JobID: "job-a", OperationID: "operation-a", DispatchMode: DispatchModeFresh, AdapterVerb: "dispatch",
	}
	if err := authorizeDelegateClaimCapability(root, raw, binding, now.Add(time.Second), false); err != nil {
		t.Fatalf("preflight validation: %v", err)
	}
	if err := authorizeDelegateClaimCapability(root, raw, binding, now.Add(2*time.Second), true); err != nil {
		t.Fatalf("consume: %v", err)
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(encoded, &record); err != nil {
		t.Fatal(err)
	}
	if record["status"] != "consumed" || record["jobId"] != "job-a" || record["operationId"] != "operation-a" || record["adapterVerb"] != "dispatch" {
		t.Fatalf("consumed binding = %#v", record)
	}
	if err := authorizeDelegateClaimCapability(root, raw, binding, now.Add(3*time.Second), true); err == nil {
		t.Fatal("delegate claim capability replay was accepted")
	}
	if err := RemoveDelegateClaimCapability(root, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("capability cleanup left %s: %v", path, err)
	}
}

func TestDelegateClaimCapabilityRejectsWrongModeAndExpiry(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	raw, err := mintDelegateClaimCapability(root, DispatchModeFresh, now, bytes.NewReader(bytes.Repeat([]byte{0x19}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	wrongMode := DelegateClaimCapabilityBinding{
		JobID: "job-a", OperationID: "operation-a", DispatchMode: DispatchModeFollowUp, AdapterVerb: "follow-up",
	}
	if err := authorizeDelegateClaimCapability(root, raw, wrongMode, now.Add(time.Second), false); err == nil {
		t.Fatal("fresh delegate capability authorized a follow-up")
	}
	fresh := DelegateClaimCapabilityBinding{
		JobID: "job-a", OperationID: "operation-a", DispatchMode: DispatchModeFresh, AdapterVerb: "dispatch",
	}
	if err := authorizeDelegateClaimCapability(root, raw, fresh, now.Add(DelegateClaimCapabilityTTL), true); err == nil {
		t.Fatal("expired delegate capability was consumed")
	}
}

func TestDelegateClaimCapabilityCannotRaceAcrossJobs(t *testing.T) {
	root := t.TempDir()
	raw, err := MintDelegateClaimCapability(root, DispatchModeFresh)
	if err != nil {
		t.Fatal(err)
	}
	bindings := []DelegateClaimCapabilityBinding{
		{JobID: "job-a", OperationID: "operation-a", DispatchMode: DispatchModeFresh, AdapterVerb: "dispatch"},
		{JobID: "job-b", OperationID: "operation-b", DispatchMode: DispatchModeFresh, AdapterVerb: "dispatch"},
	}
	start := make(chan struct{})
	results := make(chan error, len(bindings))
	var wait sync.WaitGroup
	for _, binding := range bindings {
		binding := binding
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			results <- ConsumeDelegateClaimCapability(root, raw, binding)
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	wins := 0
	for err := range results {
		if err == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("concurrent capability consumers won %d times, want exactly one", wins)
	}
}
