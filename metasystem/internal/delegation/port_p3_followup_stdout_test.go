package delegation_test

import (
	"testing"
)

// A critic follow-up's standard output is the child job id alone
// (dispatch-fixtures.sh 2178-2189): the critique register fold's outcome is
// captured, as the retired follow_up captured it, because the delegate
// boundary (cmd/metasystem writeDelegateResult) reads the first stdout line
// as the started job.
func TestCriticFollowUpIntegrationStdoutIsTheChildJobAlone(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	outputs := b.criticSubject("deviation.md")
	brief := b.brief("brief.md", "design", "Review the deviation design.")
	requireExit(t, b.dispatchAs(criticArgv(outputs, "deviation.md", brief, "critic-chain")...), 0, b.stderr.String())
	result := b.followCritic("critic-chain", "deviation.md")
	requireExit(t, result, 0, b.stderr.String())
	if string(result.Stdout) != "critic-chain-r2\n" {
		t.Fatalf("follow-up stdout %q, want the child job id alone %q", result.Stdout, "critic-chain-r2\n")
	}
}
