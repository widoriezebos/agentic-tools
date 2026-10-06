package plain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func writePushRecords(t *testing.T, install string, pushes ...Pushed) {
	t.Helper()
	if err := withLock(install, func() error {
		for _, push := range pushes {
			if err := appendLine(pushesPath(install), push); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAScopedPushOwesAFullProofWithinTheHour(t *testing.T) {
	t.Parallel()
	pushedAt := bedNow.Add(-time.Minute)
	full := func(scope, result string, at time.Time) *Result {
		return &Result{Tree: "another-tree", Scope: scope, Result: result, At: at.Format(time.RFC3339)}
	}
	cases := []struct {
		name   string
		age    time.Duration
		proof  *Result
		queued bool
		want   []string
	}{
		{"61 minutes idle", 61 * time.Minute, nil, false, []string{WakeFullDue}},
		{"59 minutes idle", 59 * time.Minute, nil, false, []string{}},
		{"exactly one hour", time.Hour, nil, false, []string{}},
		{"full green after push", 61 * time.Minute, full("full", Green, pushedAt.Add(time.Second)), false, []string{}},
		{"full green before push", 61 * time.Minute, full("full", Green, pushedAt.Add(-time.Second)), false, []string{WakeFullDue}},
		{"full green at push", 61 * time.Minute, full("full", Green, pushedAt), false, []string{WakeFullDue}},
		{"full red after push", 61 * time.Minute, full("full", Red, bedNow), false, []string{WakeFullDue}},
		{"legacy green after push", 61 * time.Minute, full("", Green, bedNow), false, []string{WakeFullDue}},
		{"61 minutes queued", 61 * time.Minute, nil, true, []string{WakeQueued, WakeFullDue}},
		{"finished and queued", 61 * time.Minute, full("scoped", Green, bedNow.Add(time.Second)), true, []string{WakeQueued, WakeProofFinished, WakeFullDue}},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
			scoped := Result{Tree: "pushed-tree", Scope: "scoped", Result: Green,
				At: pushedAt.Add(-time.Second).Format(time.RFC3339), FullAt: bedNow.Add(-each.age).Format(time.RFC3339)}
			results := []Result{{Trunk: true, Result: Green, At: pushedAt.Format(time.RFC3339)}, scoped}
			if each.proof != nil {
				results = append(results, *each.proof)
			}
			var lines []string
			for _, result := range results {
				data, err := json.Marshal(result)
				if err != nil {
					t.Fatal(err)
				}
				lines = append(lines, string(data))
			}
			writeProofRecords(t, install, lines, "")
			writePushRecords(t, install, Pushed{Tree: scoped.Tree, At: pushedAt.Format(time.RFC3339)})
			var got []string
			var err error
			if each.queued {
				// The record decision takes the queue's derived pending state;
				// queue containment is tested separately.
				got, err = wakeReasons(install, true, bedNow, bedNow)
			} else {
				got, err = WakeReasons(install, install, bedNow, bedNow)
			}
			if err != nil || !reflect.DeepEqual(got, each.want) {
				t.Fatalf("wake reasons = %v, want %v: %v", got, each.want, err)
			}
			// An older scoped push cannot make a newer full push owe proof.
			writePushRecords(t, install, Pushed{Tree: "not-scoped", At: bedNow.Format(time.RFC3339)})
			got, err = WakeReasons(install, install, bedNow, bedNow)
			if err != nil || len(got) != 0 {
				t.Fatalf("the newest push has no scoped green: %v, %v", got, err)
			}
		})
	}
}
