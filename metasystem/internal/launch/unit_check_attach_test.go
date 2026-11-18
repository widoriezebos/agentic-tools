package launch

import (
	"os/exec"
	"slices"
	"testing"
)

func TestUnitCheckAttachesBothCommandsAfterSettingTheirEnvironment(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		attach bool
	}{{"attached", true}, {"nil", false}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			guard := `[ "$UNIT_CHECK_FROZEN" = frozen ]`
			if row.attach {
				guard += ` && [ "$UNIT_CHECK_ATTACHED" = 1 ]`
			}
			check := UnitCheck{Cheap: guard + ` && printf cheap`, Audits: guard + ` && printf audits`,
				Minutes: 1, Directory: t.TempDir(), Environment: []string{"UNIT_CHECK_FROZEN=frozen"}}
			var calls []string
			context := CheckContext{}
			if row.attach {
				context.Attach = func(process *exec.Cmd) {
					calls = append(calls, process.Args[2])
					process.Env = append(process.Env, "UNIT_CHECK_ATTACHED=1")
				}
			}
			_, exits, err := check.Run(check.Directory, t.TempDir(), context)
			if err != nil || len(exits) != 2 {
				t.Fatalf("both frozen commands must run and pass: exits=%+v error=%v", exits, err)
			}
			for i, name := range []string{"cheap", "audits"} {
				if exits[i].Name != name || exits[i].Exit != 0 || exits[i].Output != name {
					t.Fatalf("%s command did not pass: %+v", name, exits[i])
				}
			}
			var want []string
			if row.attach {
				want = []string{check.Cheap, check.Audits}
			}
			if !slices.Equal(calls, want) {
				t.Fatalf("Attach calls=%q, want %q", calls, want)
			}
		})
	}
}
