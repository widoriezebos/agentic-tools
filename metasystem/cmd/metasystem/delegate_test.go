package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeDelegateFreshRequiresExplicitGoalAndMapsOperation(t *testing.T) {
	got, mode, err := normalizeDelegateArgs([]string{
		"--role", "verifier", "--brief", "brief.md", "--goal", "none-explicit", "--destructive-reach", "MECHANICAL", "--op", "operation-a", "--source", "skills/verify/SKILL.md",
	})
	want := []string{"dispatch", "--role", "verifier", "--brief", "brief.md", "--destructive-reach", "MECHANICAL", "--job-id", "operation-a", "--source", "skills/verify/SKILL.md"}
	if err != nil || mode != "dispatch" || !reflect.DeepEqual(got, want) {
		t.Fatalf("normalize = %v %s %v, want %v dispatch", got, mode, err, want)
	}
	if _, _, err := normalizeDelegateArgs([]string{"--role", "verifier", "--brief", "brief.md"}); err == nil {
		t.Fatal("fresh delegate without explicit goal selection was accepted")
	}
}

func TestNormalizeDelegateFreshForwardsReviewsForCriticRoles(t *testing.T) {
	got, mode, err := normalizeDelegateArgs([]string{
		"--reviews", "implementer-a", "--role", "code-critic", "--brief", "brief.md", "--goal", "goal-a", "--destructive-reach", "MECHANICAL",
	})
	want := []string{"dispatch", "--reviews", "implementer-a", "--role", "code-critic", "--brief", "brief.md", "--goal", "goal-a", "--destructive-reach", "MECHANICAL"}
	if err != nil || mode != "dispatch" || !reflect.DeepEqual(got, want) {
		t.Fatalf("normalize = %v %s %v, want %v dispatch", got, mode, err, want)
	}
}

func TestDelegateDesignCriticWithoutOutputsRefusesAtFrontDoor(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_DELEGATE_ROOT", root)
	want := "design-critic dispatch requires --outputs <file> and --design <file>"
	out, code := captureStdout(t, func() int {
		return runDelegate([]string{
			"--role", "design-critic", "--brief", "brief.md", "--goal", "goal-a",
			"--destructive-reach", "DESIGN-BEARING", "--design", "metasystem/plans/design.md",
		})
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if code != 2 || result["detail"] != want {
		t.Fatalf("missing outputs front-door refusal = exit %d output %q, want %q", code, out, want)
	}
}

func TestNormalizeDelegateFreshForwardsReviewsForVerifier(t *testing.T) {
	got, mode, err := normalizeDelegateArgs([]string{
		"--reviews", "implementer-a", "--role", "verifier", "--brief", "brief.md", "--goal", "goal-a", "--destructive-reach", "DESTRUCTIVE-REACH",
	})
	want := []string{"dispatch", "--reviews", "implementer-a", "--role", "verifier", "--brief", "brief.md", "--goal", "goal-a", "--destructive-reach", "DESTRUCTIVE-REACH"}
	if err != nil || mode != "dispatch" || !reflect.DeepEqual(got, want) {
		t.Fatalf("normalize verifier reviews = %v, %q, %v; want %v", got, mode, err, want)
	}
}

func TestDelegateReviewsRefusesNonCriticRole(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_DELEGATE_ROOT", root)
	out, code := captureStdout(t, func() int {
		return runDelegate([]string{
			"--role", "implementer", "--reviews", "implementer-a", "--brief", "brief.md", "--goal", "goal-a", "--destructive-reach", "MECHANICAL",
		})
	})
	if code != 2 || !strings.Contains(out, `"outcome":"REFUSED-REQUEST"`) || !strings.Contains(out, "--reviews is only valid for the code-critic, warden, and verifier roles") {
		t.Fatalf("non-critic reviews exit=%d output=%q", code, out)
	}
}

func TestDelegateAdapterSelftestRefusesADirectInvocation(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_DELEGATE_ROOT", root)
	t.Setenv("METASYSTEM_DELEGATE_SELFTEST_INTERNAL", "")
	out, code := captureStdout(t, func() int {
		return runDelegate([]string{"--adapter-selftest", "fake", "--brief", "brief.md", "--workspace", root, "--op", "test-job"})
	})
	if code != 2 || !strings.Contains(out, `"outcome":"REFUSED-REQUEST"`) || !strings.Contains(out, "reserved for the metasystem adapter self-test") {
		t.Fatalf("direct adapter self-test exit=%d output=%q", code, out)
	}
}

func TestNormalizeDelegateFollowUpAndCancel(t *testing.T) {
	follow, mode, err := normalizeDelegateArgs([]string{"--follow-up", "job-a", "--brief", "correction.md", "--op", "operation-b", "--wait"})
	if err != nil || mode != "follow-up" || !reflect.DeepEqual(follow, []string{"follow-up", "--job", "job-a", "--message", "correction.md", "--operation-id", "operation-b", "--wait"}) {
		t.Fatalf("follow-up normalize = %v %s %v", follow, mode, err)
	}
	cancel, mode, err := normalizeDelegateArgs([]string{"--cancel", "job-a"})
	if err != nil || mode != "cancel" || !reflect.DeepEqual(cancel, []string{"cancel", "--job", "job-a"}) {
		t.Fatalf("cancel normalize = %v %s %v", cancel, mode, err)
	}
}

func TestNormalizeDelegateRejectsLegacySelectionFlags(t *testing.T) {
	for _, retired := range []string{"--serving-goal", "--runtime", "--model", "--workspace", "--worktree", "--permissions", "--cap-min", "--approve-escalation"} {
		args := []string{"--role", "verifier", "--brief", "brief.md", "--goal", "none-explicit", "--destructive-reach", "MECHANICAL", retired}
		if retired == "--runtime" || retired == "--model" || retired == "--workspace" || retired == "--permissions" || retired == "--cap-min" {
			args = append(args, "retired")
		}
		if _, _, err := normalizeDelegateArgs(args); err == nil {
			t.Fatalf("public delegate accepted retired flag %s", retired)
		}
	}
}

func TestNormalizeDelegateRevivalUsesTypedPath(t *testing.T) {
	got, mode, err := normalizeDelegateArgs([]string{"--revive", "intent-a"})
	want := []string{"dispatch", "--steward-intent", "intent-a"}
	if err != nil || mode != "dispatch" || !reflect.DeepEqual(got, want) {
		t.Fatalf("revival normalize = %v %s %v", got, mode, err)
	}
}

func TestNormalizeDelegateAdapterSelftestIsFixedConfiguration(t *testing.T) {
	got, mode, err := normalizeDelegateArgs([]string{"--adapter-selftest", "fake", "--brief", "brief.md", "--workspace", "/tmp/scratch", "--op", "test-job"})
	want := []string{"dispatch", "--role", "implementer", "--brief", "brief.md", "--runtime", "fake", "--workspace", "/tmp/scratch", "--permissions", "none", "--job-id", "test-job", "--destructive-reach", "MECHANICAL"}
	if err != nil || mode != "dispatch" || !reflect.DeepEqual(got, want) {
		t.Fatalf("selftest normalize = %v %s %v", got, mode, err)
	}
}

func TestDelegateInternalRefusalDetailPreservesInternalFailure(t *testing.T) {
	if got := delegateInternalRefusalDetail("\n writable permissions require --worktree \n", errors.New("exit status 1"), 1); got != "writable permissions require --worktree" {
		t.Fatalf("detail = %q, want internal stderr", got)
	}
	if got := delegateInternalRefusalDetail("", errors.New("fork failed"), 1); got != "fork failed" {
		t.Fatalf("detail = %q, want command error", got)
	}
	if got := delegateInternalRefusalDetail("", nil, 3); got != "delegate internal exited with status 3 without detail" {
		t.Fatalf("detail = %q, want bounded fallback", got)
	}
}

func TestDelegateCommandEnvironmentReplacesInheritedInternalAuthority(t *testing.T) {
	environment := delegateCommandEnvironment([]string{
		"KEEP=value",
		"METASYSTEM_DELEGATE_INTERNAL=stale",
		"METASYSTEM_DELEGATE_OUTCOME_FILE=stale",
		delegateClaimCapabilityEnv + "=stale",
	}, "/tmp/outcome", "fresh-capability")
	want := map[string]int{
		"KEEP=value":                                     1,
		"METASYSTEM_DELEGATE_INTERNAL=1":                 1,
		"METASYSTEM_DELEGATE_OUTCOME_FILE=/tmp/outcome":  1,
		delegateClaimCapabilityEnv + "=fresh-capability": 1,
	}
	got := make(map[string]int)
	for _, entry := range environment {
		got[entry]++
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("delegate environment = %#v, want %#v", got, want)
	}
}
