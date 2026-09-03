package main

import (
	"os"
	"strings"
	"testing"
)

func dispatchShellSection(t *testing.T, start, end string) string {
	t.Helper()
	data, err := os.ReadFile("../../scripts/agents/dispatch.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	left := strings.Index(text, start)
	if left < 0 {
		t.Fatalf("dispatch shell section start %q is absent", start)
	}
	right := strings.Index(text[left:], end)
	if right < 0 {
		t.Fatalf("dispatch shell section end %q is absent", end)
	}
	return text[left : left+right]
}

func assertShellOrder(t *testing.T, section string, tokens ...string) {
	t.Helper()
	position := 0
	for _, token := range tokens {
		next := strings.Index(section[position:], token)
		if next < 0 {
			t.Fatalf("dispatch section does not contain %q after byte %d", token, position)
		}
		position += next + len(token)
	}
}

func TestOrdinaryLaunchCallSitesUsePreparedClaimStateMachineUnderLock(t *testing.T) {
	fresh := dispatchShellSection(t, "dispatch_job() {", "\nauthorize_job_cap() {")
	if strings.Contains(fresh, "goal_admission_checked") || strings.Count(fresh, "require_goal_admission") != 2 {
		t.Fatal("fresh dispatch does not preserve an advisory read plus an unsuppressed cap-locked revalidation")
	}
	assertShellOrder(t, fresh,
		`acquire_launch_chain_lock "$job"`,
		"acquire_cap_authority_lock",
		"job claim-launch --preflight",
		"require_goal_admission",
		"require_goal_revision_admission",
		"require_slice_admission",
		"acquire_lifecycle_lock_until",
		"job claim-occupancy-prepare",
		"claim_output=",
		`--occupancy-preparation "$occupancy_preparation"`,
		"release_cap_authority_lock",
		"job build-record",
	)
	for _, forbidden := range []string{"job build-setup", "__record-create"} {
		if strings.Contains(fresh, forbidden) {
			t.Fatalf("fresh dispatch still uses the partial reservation path %q", forbidden)
		}
	}
	if strings.Contains(fresh, `die 1 "job id collision: $job"`) {
		t.Fatal("fresh dispatch still refuses a standing operation before claim-launch")
	}
	if !strings.Contains(fresh, `--main-id "$current_main_id" --claim-epoch "$reservation_claim_epoch" --goal "$goal"`) {
		t.Fatal("fresh dispatch does not bind lease and goal provenance into its claim reservation")
	}
	if !strings.Contains(fresh, `goal_tier=$(json_value "$goal_binding" goalTier)`) || strings.Count(fresh, `--goal-tier "$goal_tier"`) < 3 {
		t.Fatal("fresh dispatch does not carry one claimed-revision goalTier through preflight, claim, and final record")
	}

	follow := dispatchShellSection(t, "follow_up() {", "\nstatus_job() {")
	assertShellOrder(t, follow,
		`acquire_launch_chain_lock "$root_id"`,
		"acquire_cap_authority_lock",
		"job claim-launch --preflight",
		"require_goal_admission",
		"require_goal_revision_admission",
		"require_slice_admission",
		"acquire_lifecycle_lock_until",
		"job claim-occupancy-prepare",
		"claim_output=",
		`--occupancy-preparation "$occupancy_preparation"`,
		"release_cap_authority_lock",
		"job build-follow-record",
	)
	for _, forbidden := range []string{"job build-setup", "__record-create"} {
		if strings.Contains(follow, forbidden) {
			t.Fatalf("follow-up still uses the partial reservation path %q", forbidden)
		}
	}
	assertShellOrder(t, follow,
		`[[ "$status" == pending-setup`,
		`[[ "$(json_field "$latest" dispatchMode`,
		`repeated_follow_up=1`,
		"job claim-launch",
	)
	if !strings.Contains(follow, `--main-id "$current_main_id" --claim-epoch "$reservation_claim_epoch" --goal "$goal"`) {
		t.Fatal("follow-up does not bind lease and goal provenance into its claim reservation")
	}
	if !strings.Contains(follow, `goal_tier=$(json_value "$goal_binding" goalTier)`) || strings.Count(follow, `--goal-tier "$goal_tier"`) < 3 {
		t.Fatal("follow-up does not carry one claimed-revision goalTier through preflight, claim, and final record")
	}
}

func TestGoalAdmissionDropsAuthorityLocksBeforeBreachStop(t *testing.T) {
	section := dispatchShellSection(t, "require_goal_admission() {", "\nrun_breach_stop_routes() {")
	assertShellOrder(t, section,
		"10)",
		"release_cap_authority_lock",
		"release_goal_revision_lock",
		`release_chain_lock "$exit_cleanup_chain"`,
		"run_breach_stop_routes",
	)
}

func TestClaimAuthorizationUsesTheDelegateCapabilityInsteadOfProcessAncestry(t *testing.T) {
	data, err := os.ReadFile("dispatch_verbs.go")
	if err != nil {
		t.Fatal(err)
	}
	section := string(data)
	start := strings.Index(section, "func claimLaunchInternalAuthorized")
	if start < 0 {
		t.Fatal("claim-launch authorization section is absent")
	}
	end := strings.Index(section[start:], "func claimLaunchFakeFixtureAuthorized")
	if end < 0 {
		t.Fatal("claim-launch authorization section has no fixture boundary")
	}
	section = section[start : start+end]
	for _, required := range []string{"METASYSTEM_DELEGATE_INTERNAL", "ValidateDelegateClaimCapability", "ConsumeDelegateClaimCapability"} {
		if !strings.Contains(section, required) {
			t.Fatalf("internal claim authorization lacks %q", required)
		}
	}
	for _, forbidden := range []string{"ParentPid", "ExecutablePath", "os.Args[0]"} {
		if strings.Contains(section, forbidden) {
			t.Fatalf("internal claim authorization still depends on %q", forbidden)
		}
	}
}

func TestAllAdapterLaunchRoutesConsumeTheOneShotCapability(t *testing.T) {
	common, err := os.ReadFile("../../scripts/agents/adapters/runtime-common.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"--launch-capability", "job launch-capability-consume", `--adapter-verb "$adapter_verb"`, `--supervisor-pid "$$"`} {
		if !strings.Contains(string(common), token) {
			t.Fatalf("real-adapter common path lacks %q", token)
		}
	}
	for _, runtime := range []string{"codex", "claude", "devin"} {
		data, readErr := os.ReadFile("../../scripts/agents/adapters/" + runtime + ".sh")
		if readErr != nil {
			t.Fatal(readErr)
		}
		text := string(data)
		if !strings.Contains(text, `runtime-common.sh"`) || !strings.Contains(text, `dispatch|follow-up) supervise "$command_name" "$@"`) {
			t.Fatalf("%s does not route both launch verbs through common supervision", runtime)
		}
	}
	fake, err := os.ReadFile("../../scripts/agents/adapters/fake.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"--launch-capability", "job launch-capability-consume", `dispatch|follow-up) supervise "$command" "$@"`} {
		if !strings.Contains(string(fake), token) {
			t.Fatalf("fake adapter path lacks %q", token)
		}
	}
}

func TestEveryProductionClaimTakesOccupancyAfterLifecycleAdmission(t *testing.T) {
	sections := map[string]string{
		"fresh":     dispatchShellSection(t, "dispatch_job() {", "\nauthorize_job_cap() {"),
		"follow-up": dispatchShellSection(t, "follow_up() {", "\nstatus_job() {"),
	}
	for name, section := range sections {
		t.Run(name, func(t *testing.T) {
			assertShellOrder(t, section,
				"acquire_cap_authority_lock",
				"acquire_lifecycle_lock_until",
				"job claim-occupancy-prepare",
				"claim_output=",
			)
		})
	}
}

func TestFreshDispatchCreatesItsPayloadOnlyAfterWinningTheClaim(t *testing.T) {
	fresh := dispatchShellSection(t, "dispatch_job() {", "\nauthorize_job_cap() {")
	assertShellOrder(t, fresh,
		"job claim-launch",
		`claim_outcome=$(json_value "$claim_output" outcome`,
		`if [[ "$claim_outcome" != WON ]]`,
		`mkdir -p "$round_dir"`,
		`cp "$brief" "$payload/brief.md"`,
		`mv "$prompt_temp" "$round_dir/prompt.md"`,
	)
}
