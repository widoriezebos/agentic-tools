package goal

import (
	"strings"
	"testing"
)

func TestAllowBuildWithoutDesignRecordRoundTrip(t *testing.T) {
	t.Parallel()
	permission, err := LookupPermission(PermissionBuildWithoutDesign)
	if err != nil {
		t.Fatal(err)
	}
	file := claimedGolden()
	permission.Set(file, true)
	rendered := RenderFile(file)
	parsed, problems := ParseFile(rendered)
	if len(problems) != 0 || !permission.Holds(parsed) || string(RenderFile(parsed)) != string(rendered) || !strings.Contains(string(rendered), "\n- DesignGate: off\n") {
		t.Fatalf("allowance did not survive the sealed record: problems=%v\n%s", problems, rendered)
	}
	invalid := strings.Replace(string(rendered), "- DesignGate: off", "- DesignGate: anything", 1)
	if _, problems := ParseFile([]byte(withFreshIntegrity(invalid))); !problemsContain(problems, `DesignGate "anything" is not off`) {
		t.Fatalf("invalid allowance: %v", problems)
	}
	permission.Set(parsed, false)
	if permission.Holds(parsed) || strings.Contains(string(RenderFile(parsed)), "- DesignGate:") {
		t.Fatal("withdrawn allowance still appears in the record")
	}
}
