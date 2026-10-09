package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

type helmFleetBed struct {
	*policyBed
	home, hostRegistry, second, linked string
}

func newHelmFleetBed(t *testing.T) *helmFleetBed {
	t.Helper()
	b := &helmFleetBed{policyBed: newPolicyBed(t), home: filepath.Join(t.TempDir(), ".metasystem")}
	b.second = newPolicyBed(t).seat
	b.linked = t.TempDir()
	b.linked, _ = filepath.EvalSymlinks(b.linked)
	gitdir := filepath.Join(b.seat, ".git", "worktrees", "linked")
	helmMust(t, os.MkdirAll(gitdir, 0700), os.WriteFile(filepath.Join(gitdir, "commondir"), []byte("../..\n"), 0600),
		os.WriteFile(filepath.Join(b.linked, ".git"), []byte("gitdir: "+gitdir+"\n"), 0600))
	helmMust(t, os.MkdirAll(b.home, 0700))
	b.hostRegistry = filepath.Join(b.home, "host.jsonl")
	for index, path := range []string{b.seat, b.second, b.linked} {
		b.register(t, path, index)
	}
	data, _ := json.Marshal(lane.Record{Root: b.lane, Install: b.lane, CustodyEpoch: 1, RegisteredBy: "Wido", At: b.now.Format(time.RFC3339)})
	helmMust(t, os.MkdirAll(filepath.Dir(lane.RecordPath(b.home)), 0700), os.WriteFile(lane.RecordPath(b.home), data, 0600))
	b.owners.machines.registryPath = func() (string, error) { return b.hostRegistry, nil }
	b.owners.landing.home = func() (string, error) { return b.home, nil }
	b.owners.lookupEnv = func(key string) (string, bool) {
		return filepath.Dir(b.home), key == "METASYSTEM_SUPERVISION_REGISTRY_HOME"
	}
	b.owners.helm = helmOwners{reader: person(), pid: func() int64 { return 20 }, now: func() time.Time { return b.now },
		machine: func(string) (string, error) { return "fixture", nil }, account: func() string { return "Wido" }, stdinTerminal: func() bool { return false },
		git: func(string, ...string) (string, error) { return "", fmt.Errorf("no Git in this fixture") }, fence: func(stateroot.Installation) error { return nil }}
	return b
}

func (b *helmFleetBed) register(t *testing.T, path string, index int) {
	t.Helper()
	tag := fmt.Sprintf("tag-%d", index)
	payload, _ := json.Marshal(map[string]any{"schemaVersion": 1, "event": registry.EventRelaunched, "checkoutPath": path, "ownerTag": tag,
		"at": b.now.Format(time.RFC3339), "generation": 1, "watcherTag": tag + "-w", "reaperTag": tag + "-r", "retiredThrough": 0})
	helmMust(t, registry.AppendFrame(b.hostRegistry, payload))
}

func (b *helmFleetBed) helm(t *testing.T, cwd, act string, args ...string) (int, intentResult, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runIntentIn(mustIntentCommand(t, "helm "+act), append(args, "--json"), &stdout, &stderr, cwd, b.owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("%s: exit %d: %v %s %s", act, code, err, stdout.String(), stderr.String())
	}
	return code, result, stdout.String()
}

func (b *helmFleetBed) policies(t *testing.T, root string) []config.PolicyResolution {
	t.Helper()
	code, result, raw := b.helm(t, b.seat, "status", "--repo", root)
	if code != 0 {
		t.Fatalf("status: %d %s", code, raw)
	}
	data, _ := json.Marshal(result.Data)
	var view struct {
		Policies []config.PolicyResolution `json:"policies"`
	}
	helmMust(t, json.Unmarshal(data, &view))
	return view.Policies
}

func TestHelmPolicyHostAcceptance(t *testing.T) {
	t.Parallel()
	b := newHelmFleetBed(t)
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	for _, setting := range []struct{ key, value string }{{"review.stop", "2"}, {"landing.batch", "3"}, {"question.route", "auto"}} {
		if code, result, _ := b.run(t, b.seat, "set", setting.key, setting.value); code != 0 {
			t.Fatalf("set: %d %+v", code, result)
		}
	}
	pauseBefore, paused := lane.ReadPause(b.home)
	if paused {
		t.Fatal("fixture is paused")
	}
	code, _, raw := b.helm(t, b.linked, "take", "--all", "--reason", "by hand")
	if code != 0 {
		t.Fatalf("all take: %d %s", code, raw)
	}
	var envelope struct {
		Data struct {
			Checkouts []helmHostResult `json:"checkouts"`
		} `json:"data"`
	}
	helmMust(t, json.Unmarshal([]byte(raw), &envelope))
	if len(envelope.Data.Checkouts) != 4 {
		t.Fatalf("primary checkouts were not deduplicated: %s", raw)
	}
	for root, count := range map[string]int{b.seat: 4, b.second: 4, b.lane: 8, b.coordinator: 5} {
		state := helm.Active(root)
		if !state.Active || len(state.Policies) != count {
			t.Fatalf("%s signature: %+v", root, state)
		}
		if policy := state.Policies["process.change"]; policy.Name != "process.change" || policy.Value != "person" || policy.Previous.Value != "person" || policy.Previous.Source != "built-in" {
			t.Fatalf("%s process-change hold: %+v", root, policy)
		}
		policies := b.policies(t, root)
		if len(policies) != count {
			t.Fatalf("%s effective policies: %+v", root, policies)
		}
		for _, policy := range policies {
			snapshot := state.Policies[policy.Name]
			if policy.Value != "person" || policy.Source != "helm" || policy.Previous == nil || snapshot.Value != "person" || snapshot.Name != policy.Name || snapshot.Checkout != root || snapshot.SetBy != "Wido" || !snapshot.At.Equal(b.now) || !reflect.DeepEqual(snapshot.Previous, *policy.Previous) {
				t.Fatalf("policy/snapshot mismatch: %+v %+v", policy, snapshot)
			}
		}
	}
	if after, paused := lane.ReadPause(b.home); paused || !reflect.DeepEqual(after, pauseBefore) {
		t.Fatal("take created a pause")
	}
	requireHelmDrain(t, b.lane, helmDrainSource(helm.Active(b.lane).Record))
	for _, root := range []string{b.seat, b.second, b.coordinator} {
		if drain, err := plain.ReadDrain(root); err != nil || drain != nil {
			t.Fatalf("non-lane checkout drained: %s %+v %v", root, drain, err)
		}
	}
	before := helm.Active(b.lane).Record
	b.now = b.now.Add(time.Hour)
	if code, _, raw := b.helm(t, b.seat, "take", "--all", "--reason", "by hand"); code != 0 {
		t.Fatalf("repeat: %s", raw)
	}
	if after := helm.Active(b.lane).Record; !reflect.DeepEqual(before, after) {
		t.Fatalf("repeat replaced originals: %+v %+v", before, after)
	}
	if code, result, _ := b.run(t, b.seat, "set", "landing.batch", "5"); code != 0 || !strings.Contains(result.Summary, "applies when") {
		t.Fatalf("held write: %d %+v", code, result)
	}
	for _, policy := range b.policies(t, b.lane) {
		if policy.Name == "landing.batch" && (policy.Previous.Value != "5" || policy.Previous.SetBy != "Wido") {
			t.Fatalf("current underlying value hidden: %+v", policy)
		}
	}
	if code, _, raw := b.helm(t, b.seat, "status", "--all"); code != 0 || !strings.Contains(raw, "landing.batch") || !strings.Contains(raw, "draining") {
		t.Fatalf("all status: %d %s", code, raw)
	}
	_, err := lane.SetPause(b.home, "Wido", b.now)
	helmMust(t, err)
	pauseBefore, _ = lane.ReadPause(b.home)
	if code, _, raw := b.helm(t, b.seat, "return", "--all"); code != 0 {
		t.Fatalf("all return: %d %s", code, raw)
	}
	for _, root := range []string{b.seat, b.second, b.lane, b.coordinator} {
		if helm.Active(root).Active {
			t.Fatalf("%s remains held", root)
		}
		if root != b.seat {
			if _, err := humanauthority.ReadEnrollment(root); !os.IsNotExist(err) {
				t.Fatalf("target was separately enrolled: %s: %v", root, err)
			}
		}
	}
	if after, paused := lane.ReadPause(b.home); !paused || !reflect.DeepEqual(pauseBefore, after) {
		t.Fatal("return removed an immediate pause")
	}
	if drain, err := plain.ReadDrain(b.lane); err != nil || drain != nil {
		t.Fatalf("all return left a matching drain: %+v %v", drain, err)
	}
	if policy := b.show(t, "landing.batch"); policy.Value != "5" || policy.SetBy != "Wido" {
		t.Fatalf("return lost a later setting: %+v", policy)
	}
	if code, _, raw := b.helm(t, b.seat, "return", "--all"); code != 0 {
		t.Fatalf("repeat return: %s", raw)
	}
}

func TestHelmPolicyPartialRecovery(t *testing.T) {
	t.Parallel()
	b := newHelmFleetBed(t)
	blocked := filepath.Join(b.second, ".git", "metasystem", "helm.json")
	helmMust(t, os.MkdirAll(filepath.Join(blocked, "blocker"), 0700))
	code, result, raw := b.helm(t, b.seat, "take", "--all", "--reason", "repair at my terminal")
	if code != 1 || result.Outcome != intentFailed || !strings.Contains(result.Summary, "partial") || !strings.Contains(raw, b.second) {
		t.Fatalf("partial take: %d %s", code, raw)
	}
	for _, root := range []string{b.seat, b.lane, b.coordinator} {
		if !helm.Active(root).Active {
			t.Fatalf("%s rolled back or skipped", root)
		}
	}
	if !helm.Active(b.second).Active {
		t.Fatal("unconfirmed take lost the existing hold")
	}
	if code, result, raw := b.helm(t, b.seat, "status", "--all"); code != 1 || !strings.Contains(result.Summary, "partial") {
		t.Fatalf("unavailable target status: %d %s", code, raw)
	}
	helmMust(t, os.Remove(filepath.Join(blocked, "blocker")), os.Remove(blocked))
	if code, _, raw := b.helm(t, b.seat, "take", "--reason", "repair at my terminal", "--repo", b.second); code != 0 {
		t.Fatalf("direct retry: %s", raw)
	}
	// A genuinely unreadable registry, with all other sources still usable.
	helmMust(t, os.Remove(b.hostRegistry), os.Mkdir(b.hostRegistry, 0700))
	for _, act := range []string{"take", "status", "return"} {
		args := []string{"--all"}
		if act == "take" {
			args = append(args, "--reason", "repair at my terminal")
		}
		code, result, raw := b.helm(t, b.seat, act, args...)
		if code != 1 || !strings.Contains(result.Summary, "partial") || !strings.Contains(raw, "PATH") {
			t.Fatalf("unreadable registry %s: %d %s", act, code, raw)
		}
		var text bytes.Buffer
		command := mustIntentCommand(t, "helm "+act)
		if code := runIntentIn(command, args, &text, &text, b.seat, b.owners); code != 1 || !strings.Contains(text.String(), "--repo PATH") || (act == "take" && !strings.Contains(text.String(), "--reason 'repair at my terminal'")) {
			t.Fatalf("direct remedy %s: %d %s", act, code, text.String())
		}
	}
	if code, _, raw := b.helm(t, b.outside, "return", "--repo", b.second); code != 0 || helm.Active(b.second).Active {
		t.Fatalf("direct return outside a checkout: %d %s", code, raw)
	}
}

func TestHelmPolicyCorruptRecoveryAndAuthority(t *testing.T) {
	t.Parallel()
	b := newHelmFleetBed(t)
	seat, err := helm.Locate(b.lane)
	helmMust(t, err, os.MkdirAll(seat.Dir, 0700))
	broken := []byte("{broken signature\n")
	helmMust(t, os.WriteFile(seat.Signature, broken, 0600), os.WriteFile(filepath.Join(b.lane, "metasystem.conf"), []byte("landing.batch=nonsense\n"), 0600))
	b.owners.helm.pid = func() int64 { return 80 }
	for _, act := range []string{"take", "return"} {
		args := []string{"--repo", b.lane}
		if act == "take" {
			args = append(args, "--reason", "recover")
		}
		if code, _, raw := b.helm(t, b.seat, act, args...); code != 3 || !strings.Contains(raw, "an agent started this shell") {
			t.Fatalf("agent %s: %d %s", act, code, raw)
		}
		after, _ := os.ReadFile(seat.Signature)
		if !bytes.Equal(after, broken) {
			t.Fatal("agent changed a corrupt signature")
		}
	}
	b.owners.helm.pid = func() int64 { return 20 }
	if code, _, raw := b.helm(t, b.seat, "take", "--repo", b.lane, "--reason", "recover"); code != 0 {
		t.Fatalf("person take: %d %s", code, raw)
	}
	if state := helm.Active(b.lane); state.Malformed != "" || state.Policies["landing.batch"].Previous.Value != "unknown" {
		t.Fatalf("damaged settings vetoed recovery: %+v", state)
	}
	data, err := os.ReadFile(seat.Log)
	helmMust(t, err)
	var entry helm.Entry
	helmMust(t, json.Unmarshal(bytes.TrimSpace(data), &entry))
	if !bytes.Equal(entry.Diagnostic, broken) {
		t.Fatalf("diagnostic bytes lost: %s", data)
	}
	configured, _ := os.ReadFile(filepath.Join(b.lane, "metasystem.conf"))
	if code, _, raw := b.helm(t, b.seat, "return", "--repo", b.lane); code != 0 {
		t.Fatalf("person return: %s", raw)
	}
	after, _ := os.ReadFile(filepath.Join(b.lane, "metasystem.conf"))
	if !bytes.Equal(configured, after) {
		t.Fatal("return rewrote malformed configuration")
	}
	// Returning a legacy corrupt record remains a person-only recovery path.
	helmMust(t, os.WriteFile(seat.Signature, broken, 0600))
	if code, _, raw := b.helm(t, b.seat, "return", "--repo", b.lane); code != 0 || helm.Active(b.lane).Active {
		t.Fatalf("corrupt return: %d %s", code, raw)
	}
	data, err = os.ReadFile(seat.Log)
	helmMust(t, err)
	entries := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	entry = helm.Entry{}
	helmMust(t, json.Unmarshal(entries[len(entries)-1], &entry))
	if !bytes.Equal(entry.Diagnostic, broken) {
		t.Fatalf("corrupt return lost diagnostic bytes: %s", data)
	}
}

func TestHelmPolicyWriteFailureRetry(t *testing.T) {
	t.Parallel()
	b := newHelmFleetBed(t)
	seat, err := helm.Locate(b.seat)
	helmMust(t, err, os.WriteFile(seat.Dir, []byte("not a directory"), 0600))
	if code, result, raw := b.helm(t, b.seat, "take", "--reason", "repair"); code != 1 || result.Outcome == intentConfirmed {
		t.Fatalf("failed write confirmed: %d %s", code, raw)
	}
	helmMust(t, os.Remove(seat.Dir))
	if code, _, raw := b.helm(t, b.seat, "take", "--reason", "repair"); code != 0 || !helm.Active(b.seat).Active {
		t.Fatalf("same-owner retry: %d %s", code, raw)
	}
}

func TestHelmPolicyLegacyAndOriginalHolder(t *testing.T) {
	t.Parallel()
	b := newHelmFleetBed(t)
	_, err := helm.Write(b.lane, helm.Record{By: "Original", At: b.now.Format(time.RFC3339), Reason: "legacy", Checkout: b.lane})
	helmMust(t, err)
	if policies := b.policies(t, b.lane); len(policies) != 8 {
		t.Fatalf("legacy scope lost: %+v", policies)
	} else {
		for _, policy := range policies {
			if policy.Value != "person" {
				t.Fatalf("legacy policy: %+v", policy)
			}
		}
	}
	original := helm.Active(b.lane).Record
	_, err = humanauthority.Enroll(b.seat, 20, person(), "Ann", b.now)
	helmMust(t, err)
	original.Leader, original.LeaderRef, original.Enrollment, original.EnrolledAs = "login", "10@100", "proven", "Ann"
	if code, _, raw := b.helm(t, b.seat, "take", "--repo", b.lane, "--reason", "legacy"); code != 0 || !reflect.DeepEqual(original, helm.Active(b.lane).Record) {
		t.Fatalf("implicit repeat changed the holder or did not refresh the terminal: %d %s", code, raw)
	}
	if code, _, raw := b.helm(t, b.seat, "take", "--repo", b.lane, "--reason", "replace", "--name", "Ann"); code != 0 || helm.Active(b.lane).By != "Ann" || len(helm.Active(b.lane).Policies) != 8 {
		t.Fatalf("explicit replacement: %d %s", code, raw)
	}
}

func TestHelmPolicyHostReadsCoordinatorDeclaration(t *testing.T) {
	t.Parallel()
	b := newHelmFleetBed(t)
	b.owners.policies.Registry = nil
	ledger := newIntentBed(t, false, nil)
	dependencies := ledger.owners().dependencies
	b.owners.dependencies.endpoint = func(string) (goal.Endpoint, error) { return dependencies.endpoint(ledger.root()) }
	endpoint, err := dependencies.endpoint(ledger.root())
	helmMust(t, err)
	id := goal.ExistingLedgerIdentityAtEndpoint(endpoint)
	if id == "" {
		t.Fatal("fixture ledger has no identity")
	}
	_, err = brain.Declare(brain.DeclareOptions{StateRoot: b.coordinator, RegistryHome: filepath.Dir(b.home), LedgerIdentity: id, Machine: "fixture", DeclaredBy: "Wido", Now: b.now})
	helmMust(t, err)
	if code, _, raw := b.helm(t, b.seat, "take", "--all", "--reason", "declarations"); code != 0 || !helm.Active(b.coordinator).Active || len(helm.Active(b.coordinator).Policies) != 5 {
		t.Fatalf("declaration was not enumerated: %d %s", code, raw)
	}
	// An unreadable lane must not hide the independently readable coordinator.
	helmMust(t, os.WriteFile(lane.RecordPath(b.home), []byte("broken"), 0600))
	if code, result, raw := b.helm(t, b.seat, "return", "--all"); code != 1 || !strings.Contains(result.Summary, "partial") || helm.Active(b.coordinator).Active {
		t.Fatalf("damaged lane hid coordinator: %d %s", code, raw)
	}
}
