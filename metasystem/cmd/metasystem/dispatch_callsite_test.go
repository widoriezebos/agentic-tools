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
	assertShellOrder(t, fresh,
		`acquire_launch_chain_lock "$job"`,
		"job claim-occupancy-prepare",
		"acquire_cap_authority_lock",
		"job claim-launch",
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
	if !strings.Contains(fresh, `--main-id "$current_main_id" --claim-epoch "$current_claim_epoch" --goal "$goal"`) {
		t.Fatal("fresh dispatch does not bind lease and goal provenance into its claim reservation")
	}

	follow := dispatchShellSection(t, "follow_up() {", "\nstatus_job() {")
	assertShellOrder(t, follow,
		`acquire_launch_chain_lock "$root_id"`,
		"job claim-occupancy-prepare",
		"acquire_cap_authority_lock",
		"job claim-launch",
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
	if !strings.Contains(follow, `--main-id "$current_main_id" --claim-epoch "$current_claim_epoch" --goal "$goal"`) {
		t.Fatal("follow-up does not bind lease and goal provenance into its claim reservation")
	}
}

func TestCustodialLaunchPreservesClaimProvenanceThroughRecordSetup(t *testing.T) {
	custodial := dispatchShellSection(t, "custodial_exec() {", "\nwatch_job() {")
	assertShellOrder(t, custodial,
		"job claim-launch",
		`--main-id "$current_main_id" --claim-epoch "$current_claim_epoch"`,
		"job build-record",
		`--main-id "$current_main_id" --claim-epoch "$current_claim_epoch"`,
		"__record-setup",
	)
}

func TestEveryProductionClaimPreparesOccupancyBeforeTheCapLock(t *testing.T) {
	sections := map[string]string{
		"fresh":     dispatchShellSection(t, "dispatch_job() {", "\nauthorize_job_cap() {"),
		"custodial": dispatchShellSection(t, "custodial_exec() {", "\nwatch_job() {"),
		"follow-up": dispatchShellSection(t, "follow_up() {", "\nstatus_job() {"),
	}
	for name, section := range sections {
		t.Run(name, func(t *testing.T) {
			assertShellOrder(t, section,
				"job claim-occupancy-prepare",
				"acquire_cap_authority_lock",
				"job claim-launch",
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
		`write_prompt "$round_dir/prompt.md"`,
	)
}
