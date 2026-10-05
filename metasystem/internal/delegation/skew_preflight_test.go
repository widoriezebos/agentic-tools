package delegation_test

import (
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation/fake"
)

func TestSkewPreflightListsOnlyMainSideCommits(t *testing.T) {
	t.Parallel()
	const stamp = "1111111"
	const base = "2222222222222222222222222222222222222222"
	const older = "3333333333333333333333333333333333333333"
	const goal = "4444444444444444444444444444444444444444"
	const mergeArgs = "merge-base HEAD refs/heads/main"
	const logArgs = "log --format=commit %H --name-only --ancestry-path " + stamp + ".."
	engineLog := "commit " + base + "\n\ninternal/delegation/infra.go\n\ncommit " + older + "\n\ncmd/metasystem/main.go\n"
	for _, tc := range []struct {
		name      string
		merge     fake.GitResponse
		log       fake.GitResponse
		wantCode  int
		wantNoLog bool
	}{
		{name: "goal engine commits after merge-base pass", merge: fake.GitResponse{Stdout: base + "\n"}},
		{name: "main engine commits refuse", merge: fake.GitResponse{Stdout: base + "\n"}, log: fake.GitResponse{Stdout: engineLog}, wantCode: 1},
		{name: "no local main passes", merge: fake.GitResponse{Code: 128, Stderr: "fatal: Not a valid object name refs/heads/main"}, wantNoLog: true},
		{name: "unrelated histories pass", merge: fake.GitResponse{Code: 1}, wantNoLog: true},
		{name: "shallow history passes", merge: fake.GitResponse{Code: 1}, wantNoLog: true},
		{name: "merge-base failure passes", merge: fake.GitResponse{Code: 128, Stderr: "fatal: could not read object"}, wantNoLog: true},
		{name: "stamp outside main ancestry passes", merge: fake.GitResponse{Stdout: base + "\n"}},
		{name: "unknown stamp passes", merge: fake.GitResponse{Stdout: base + "\n"}, log: fake.GitResponse{Code: 128, Stderr: "fatal: bad revision"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newBed(t)
			b.doubles.Git.Responses[b.root+"|"+mergeArgs] = tc.merge
			b.doubles.Git.Responses[b.root+"|"+logArgs+base] = tc.log
			// The goal's engine edits appear only when the log reaches HEAD.
			b.doubles.Git.Responses[b.root+"|"+logArgs+"HEAD"] = fake.GitResponse{Stdout: "commit " + goal + "\n\ninternal/delegation/infra.go\n\n" + engineLog}
			result := b.run("__engine-skew-preflight", stamp)
			requireExit(t, result, tc.wantCode, b.stderr.String())
			wantCalls := []string{b.root + "|" + mergeArgs}
			if !tc.wantNoLog {
				wantCalls = append(wantCalls, b.root+"|"+logArgs+base)
			}
			if !reflect.DeepEqual(b.doubles.Git.Calls, wantCalls) {
				t.Fatalf("Git calls = %v, want %v", b.doubles.Git.Calls, wantCalls)
			}
			if tc.wantCode == 1 {
				want := "dispatch refused: the engine (" + stamp + ") is older than this checkout (" + base + "), and engine scripts changed\nrebuild with go run ./cmd/devgate build, then arm the steward again\n"
				if b.stderr.String() != want {
					t.Fatalf("refusal = %q, want %q", b.stderr.String(), want)
				}
				if outcome := outcomeOf(t, result); outcome["outcome"] != "REFUSED-INTERNAL" {
					t.Fatalf("outcome %v", outcome)
				}
			} else if b.stderr.Len() != 0 || len(result.Stdout) != 0 || len(result.Outcome) != 0 {
				t.Fatalf("admitted skew check was not silent: stdout %q stderr %q outcome %q", result.Stdout, b.stderr.String(), result.Outcome)
			}
		})
	}
}
