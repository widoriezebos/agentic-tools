package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/durable"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processchange"
)

// Fail one actual directory barrier, leaving file sync and rename untouched.
type settingBarriers struct {
	dirs []string
	path string
	nth  int
}

func (f *settingBarriers) sync(dir string) error {
	f.dirs = append(f.dirs, dir)
	if dir == f.path {
		f.nth--
		if f.nth == 0 {
			return os.ErrPermission
		}
	}
	return durable.SyncDir(dir)
}

func TestLocalSettingPublication(t *testing.T) {
	t.Parallel()
	const key = "role.default.model.claude"
	for _, tc := range []struct {
		name  string
		nth   int
		first bool
	}{
		{"confirmed", 0, false}, {"before rename", 1, false}, {"empty first use", 1, true}, {"after rename", 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newHelmBed(t, 20, false)
			inst := realpathOf(t, b.inst)
			path := filepath.Join(inst, "metasystem.conf.local")
			if !tc.first {
				helmMust(t, os.WriteFile(path, []byte(key+"=old\n"), 0600))
			}
			f := settingBarriers{path: inst, nth: tc.nth}
			b.owners.syncDir = f.sync
			code, result := runSettingsSet(t, b.root, b.owners, key, "new")
			v, found, err := config.ConfLookup(path, key)
			helmMust(t, err)
			if tc.nth == 1 {
				if code != 1 || !strings.Contains(result.Summary, "nothing was set") || found == tc.first || found && v != "old" {
					t.Fatal("pre-publication failure changed the key or claimed success")
				}
				return
			}
			if code != 0 || !found || v != "new" || !slices.Equal(f.dirs, []string{inst, inst}) {
				t.Fatalf("set or installation barriers failed: %d %v", code, f.dirs)
			}
			data := resultData(t, result)
			if data["key"] != key || data["value"] != "new" || data["file"] != path {
				t.Fatal("missing setting data")
			}
			if tc.nth == 2 && (data["durable"] != false || !strings.Contains(result.Summary, "set") || !strings.Contains(result.Summary, "survival")) {
				t.Fatal("published setting lost its durability warning")
			}
			if tc.nth == 0 && result.Outcome != intentConfirmed {
				t.Fatal(result.Outcome)
			}
			code, result = runSettingsSet(t, b.root, b.owners, key, "new")
			if code != 0 || result.Outcome != intentUnchanged || len(f.dirs) != 4 {
				t.Fatal("repeat did not durably publish the identical value")
			}
		})
	}
}

func TestAttributedSettingPublication(t *testing.T) {
	t.Parallel()
	const key = "landing.batch"
	for _, tc := range []struct {
		name string
		dir  string
		nth  int
	}{
		{"confirmed", "", 0}, {"value failed", "value", 1}, {"value uncertain", "value", 2}, {"author failed", "common", 1}, {"author uncertain", "policy", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newPolicyBed(t)
			b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
			common := filepath.Join(b.lane, ".git")
			policy := filepath.Join(common, "metasystem", "policy")
			f := settingBarriers{path: map[string]string{"value": b.lane, "common": common, "policy": policy}[tc.dir], nth: tc.nth}
			b.owners.syncDir = f.sync
			code, r, _ := b.run(t, b.seat, "set", key, "2")
			p := b.show(t, key)
			if tc.name == "value failed" {
				if code != 1 || p.Value == "2" || slices.Contains(f.dirs, policy) {
					t.Fatal("failed value was acknowledged or attributed")
				}
				return
			}
			if code != 0 || p.Value != "2" || p.Source != "conf-local" {
				t.Fatalf("published value was lost: code %d, source %s", code, p.Source)
			}
			if tc.nth > 0 && (resultData(t, r)["durable"] != false || !strings.Contains(r.Summary, "set") || !strings.Contains(r.Summary, "survival")) {
				t.Fatal("missing publication warning")
			}
			missing := tc.dir == "common"
			if missing && (!strings.Contains(p.SetBy, "unknown") || !strings.Contains(r.Summary, "unknown")) {
				t.Fatal("failed attribution credited a person")
			}
			if !missing && (p.SetBy != "Wido" || !p.At.Equal(b.now)) {
				t.Fatal("wrong original attribution")
			}
			want := []string{b.lane, b.lane, policy, filepath.Dir(policy), common}
			if !missing {
				want = append(want, policy)
			}
			if !slices.Equal(f.dirs, want) {
				t.Fatalf("first write barriers %v", f.dirs)
			}
			var original []byte
			if !missing {
				original = policyBytes(t, b.lane, key)
			}
			originalAt := b.now
			b.now = b.now.Add(time.Hour)
			_, err := humanauthority.Enroll(b.seat, 20, person(), "Noor", b.now)
			helmMust(t, err)
			f.dirs = nil
			code, r, _ = b.run(t, b.seat, "set", key, "2")
			p = b.show(t, key)
			if code != 0 || !slices.Equal(f.dirs, append(slices.Clone(want[:5]), policy)) {
				t.Fatalf("repeat omitted publication: %d %v", code, f.dirs)
			}
			if missing {
				if !strings.Contains(r.Summary, "reaffirmed by Noor") || !strings.Contains(p.SetBy, "reaffirmed by Noor") {
					t.Fatal("retry did not identify the reaffirmation")
				}
			} else if r.Outcome != intentUnchanged || resultData(t, r)["set-by"] != "Wido" || resultData(t, r)["at"] != originalAt.Format(time.RFC3339) || !p.At.Equal(originalAt) || !bytes.Equal(original, policyBytes(t, b.lane, key)) {
				t.Fatal("repeat changed the original author or time")
			}
		})
	}
}

func TestPolicyAttributionAcrossRepeatedValues(t *testing.T) {
	t.Parallel()
	b := newPolicyBed(t)
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	const key = "landing.batch"
	if c, _, _ := b.run(t, b.seat, "set", key, "2"); c != 0 {
		t.Fatal("first set")
	}
	first := policyBytes(t, b.lane, key)
	common := filepath.Join(b.lane, ".git")
	for _, value := range []string{"3", "2"} {
		// The old record survives both interruptions between the two renames.
		f := settingBarriers{path: common, nth: 1}
		b.owners.syncDir = f.sync
		if c, _, _ := b.run(t, b.seat, "set", key, value); c != 0 {
			t.Fatal("partial write refused")
		}
		if p := b.show(t, key); p.Value != value || !strings.Contains(p.SetBy, "unknown") || !p.At.IsZero() {
			t.Fatal("stale attribution survived a value change")
		}
		if !bytes.Equal(first, policyBytes(t, b.lane, key)) {
			t.Fatal("fixture lost its stale record")
		}
	}
	b.owners.syncDir = nil
	for _, value := range []string{"3", "2"} {
		b.now = b.now.Add(time.Hour)
		_, err := humanauthority.Enroll(b.seat, 20, person(), "Noor", b.now)
		helmMust(t, err)
		if c, _, _ := b.run(t, b.seat, "set", key, value); c != 0 {
			t.Fatal("replacement failed")
		}
		if p := b.show(t, key); p.SetBy != "Noor" || !p.At.Equal(b.now) {
			t.Fatal("latest writer lost")
		}
	}
}

func TestPolicyAttributionRequiresMatchingEvidence(t *testing.T) {
	t.Parallel()
	const key = "landing.batch"
	b := newPolicyBed(t)
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	if c, _, _ := b.run(t, b.seat, "set", key, "2"); c != 0 {
		t.Fatal("set failed")
	}
	path := filepath.Join(b.lane, ".git", "metasystem", "policy", key+".json")
	original := policyBytes(t, b.lane, key)
	for field, wrong := range map[string]string{"name": "review.stop", "checkout": b.seat, "value": "3", "fingerprint": "", "set-by": "", "at": "0001-01-01T00:00:00Z"} {
		var r map[string]any
		helmMust(t, json.Unmarshal(original, &r))
		r[field] = wrong
		data, err := json.Marshal(r)
		helmMust(t, err, os.WriteFile(path, data, 0600))
		if p := b.show(t, key); p.Value != "2" || !strings.Contains(p.SetBy, "unknown") || !p.At.IsZero() {
			t.Fatalf("credited mismatched %s", field)
		}
	}
	helmMust(t, os.WriteFile(path, original, 0600), os.Remove(filepath.Join(b.lane, "metasystem.conf.local")))
	if p := b.show(t, key); p.Value == "2" || p.SetBy == "Wido" {
		t.Fatal("orphan provided a setting")
	}
}

func TestHelmSignaturePublication(t *testing.T) {
	t.Parallel()
	for _, which := range []string{"chain", "final", "confirmed"} {
		t.Run(which, func(t *testing.T) {
			b := newHelmBed(t, 20, true)
			seat, err := helm.Locate(b.root)
			helmMust(t, err)
			f := settingBarriers{}
			switch which {
			case "chain":
				f.path, f.nth = seat.CommonDir, 1
			case "final":
				f.path, f.nth = seat.Dir, 2
			}
			b.owners.syncDir = f.sync
			run := func() (int, string) { return b.run("helm", "take", "--reason", "by hand", "--json") }
			c, out := run()
			if which == "chain" {
				if c == 0 || helm.Active(b.root).Active || !strings.Contains(out, "not confirmed") {
					t.Fatal("pre-publication helm failure lost")
				}
				return
			}
			p := helm.Active(b.root)
			if !p.Active || !slices.Equal(f.dirs, []string{seat.Dir, seat.CommonDir, seat.Dir}) {
				t.Fatal("helm publication or anchor missing")
			}
			if which == "final" && (c == 0 || !strings.Contains(out, "held but its durability is unknown")) {
				t.Fatal("missing held warning")
			}
			policy, err := config.ResolvePolicy(config.GetParams{Key: "landing.batch", ConfPath: filepath.Join(b.inst, "metasystem.conf")})
			helmMust(t, err)
			if policy.Source != "helm" || policy.SetBy != p.By {
				t.Fatal("visible helm did not take effect")
			}
			c, _ = run()
			if c != 0 || helm.Active(b.root).At != p.At || helm.Active(b.root).By != p.By || len(f.dirs) != 6 {
				t.Fatal("helm repeat lost identity or durability")
			}
			if c, _ := b.run("helm", "return", "--json"); c != 0 || helm.Active(b.root).Active {
				t.Fatal("return failed")
			}
		})
	}
}

func TestRetainedProcessSettingPublication(t *testing.T) {
	t.Parallel()
	const key = "launch.codex.sandbox"
	const value = "danger-full-access"
	for _, tc := range []struct {
		name, where string
		nth         int
	}{
		{"value uncertain", "value", 2}, {"author absent", "common", 1}, {"author uncertain", "policy", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, local := processSettingBed(t)
			_, r, _ := b.run(t, b.seat, "set", key, value)
			proposal := processAct(t, r)
			id := proposal.ID
			b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
			common := filepath.Join(b.seat, ".git")
			f := settingBarriers{path: map[string]string{"value": filepath.Dir(local), "common": common, "policy": filepath.Join(common, "metasystem", "policy")}[tc.where], nth: tc.nth}
			b.owners.syncDir = f.sync
			c, r, _ := b.run(t, b.seat, "set", key, value, "--act", id)
			v, ok, err := config.ConfLookup(local, key)
			helmMust(t, err)
			if c != 0 || !ok || v != value || !strings.Contains(r.Summary, "survival") || resultData(t, r)["durable"] != false {
				t.Fatalf("publication warning missing or value lost: %d %s", c, r.Summary)
			}
			applied := processAct(t, r)
			if applied.Status != "applied" || !applied.AppliedAt.Equal(b.now) || applied.AppliedProof.TerminalGeneration == 0 {
				t.Errorf("published act has status %q, time %v, terminal generation %d", applied.Status, applied.AppliedAt, applied.AppliedProof.TerminalGeneration)
			}
			q, err := channel.ReadQuestion(b.seat, proposal.Question)
			helmMust(t, err)
			if q.ProcessAct != id || q.State != "closed" {
				t.Errorf("published act's question is %q", q.State)
			}
			b.now = b.now.Add(time.Hour)
			_, err = humanauthority.Enroll(b.seat, 20, person(), "Noor", b.now)
			helmMust(t, err)
			c, r, _ = b.run(t, b.seat, "set", key, value, "--act", id)
			retry := processAct(t, r)
			if c != 0 || retry.Status != "applied" || !retry.AppliedAt.Equal(applied.AppliedAt) || resultData(t, r)["durable"] != true {
				t.Fatal("exact repeat did not confirm the original publication")
			}
			var record config.PolicyRecord
			helmMust(t, json.Unmarshal(policyBytes(t, b.seat, key), &record))
			if tc.where == "common" {
				if !strings.Contains(record.SetBy, "reaffirmed by Noor") || !strings.Contains(r.Summary, "reaffirmed by Noor") {
					t.Fatal("recovery lost the reaffirmation")
				}
			} else if record.SetBy != "Wido" || !record.At.Equal(b.now.Add(-time.Hour)) {
				t.Fatal("retry claimed the first person's act")
			}
		})
	}
}

func TestPolicySetKeepsTheSettingsLock(t *testing.T) {
	t.Parallel()
	b := newPolicyBed(t)
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	err := config.WithPolicyLock(b.lane, func() error {
		c, _, _ := b.run(t, b.seat, "set", "landing.batch", "2")
		if c == 0 || b.show(t, "landing.batch").Value == "2" {
			t.Fatal("concurrent set entered the locked checkout")
		}
		return nil
	})
	helmMust(t, err)
}

func TestPlainSetAtMissingInstallation(t *testing.T) {
	t.Parallel()
	b := newHelmBed(t, 20, false)
	layout, err := b.owners.resolver.ResolveLayout(b.root)
	helmMust(t, err)
	// The installation goes away after the command resolved its destination.
	helmMust(t, os.Rename(b.inst, b.inst+"-moved"))
	var output bytes.Buffer
	inv := intentInvocation{command: mustIntentCommand(t, "settings set"), owners: b.owners, cwd: b.root, layout: layout,
		stdout: &output, stderr: &output, input: intentInput{args: []string{"role.default.model.claude", "new"}, values: map[string][]string{"json": {"true"}}}}
	if c := runIntentSettingsSet(&inv); c != 1 || !strings.Contains(output.String(), "nothing was set") {
		t.Fatal("missing installation was accepted")
	}
	if _, err := os.Stat(b.inst); !os.IsNotExist(err) {
		t.Fatal("settings set recreated a missing installation")
	}
}

func TestProcessSettingRepeatWithNoActID(t *testing.T) {
	t.Parallel()
	b, local := processSettingBed(t)
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	key := "launch.codex.sandbox"
	f := settingBarriers{path: filepath.Join(b.seat, ".git"), nth: 1}
	b.owners.syncDir = f.sync
	c, r, _ := b.run(t, b.seat, "set", key, "danger-full-access")
	first := processAct(t, r)
	if c != 0 || first.ID == "" || !strings.Contains(r.Summary, "survival after a crash is not confirmed") || resultData(t, r)["durable"] != false || r.Next == nil || !slices.Contains(r.Next.Argv, "--act") {
		t.Fatal("missing publication warning or exact retry command")
	}
	b.now = b.now.Add(time.Hour)
	_, err := humanauthority.Enroll(b.seat, 20, person(), "Noor", b.now)
	helmMust(t, err)
	c, r, _ = b.run(t, b.seat, "set", key, "danger-full-access")
	if c != 0 || r.Outcome != intentUnchanged || !strings.Contains(r.Summary, "reaffirmed by Noor") {
		t.Fatal("direct repeat lost its reaffirmation")
	}
	// The ordinary repeat also reports a fresh failure to confirm the same bytes.
	f.nth = 1
	c, r, _ = b.run(t, b.seat, "set", key, "danger-full-access")
	if c != 0 || processAct(t, r).Status != "unchanged" || resultData(t, r)["durable"] != false || !strings.Contains(r.Summary, "survival after a crash is not confirmed") {
		t.Fatal("ordinary repeat must report its own durability warning")
	}
	acts, unknown, err := processchange.ReadActs(b.seat, "")
	helmMust(t, err)
	if len(unknown) != 0 || len(acts) != 1 || acts[0].ID != first.ID || acts[0].Status != "applied" || !acts[0].AppliedAt.Equal(b.now.Add(-time.Hour)) {
		t.Fatal("ordinary repeat must preserve the single original applied act")
	}
	v, _, err := config.ConfLookup(local, key)
	helmMust(t, err)
	if v != "danger-full-access" {
		t.Fatal("direct repeat lost the value")
	}
}

func TestProcessSettingUndoAfterDurabilityWarning(t *testing.T) {
	t.Parallel()
	b, local := processSettingBed(t)
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	const key = "launch.codex.sandbox"
	f := settingBarriers{path: filepath.Dir(local), nth: 2}
	b.owners.syncDir = f.sync
	c, r, _ := b.run(t, b.seat, "set", key, "danger-full-access")
	original := processAct(t, r)
	if c != 0 || resultData(t, r)["durable"] != false || !strings.Contains(r.Summary, "survival after a crash is not confirmed") {
		t.Fatal("published act lost its warning")
	}
	c, r, _ = b.run(t, b.seat, "set", key, "workspace-write", "--undo", original.ID)
	undo := processAct(t, r)
	v, ok, err := config.ConfLookup(local, key)
	helmMust(t, err)
	if c != 0 || undo.Status != "applied" || undo.Undo != original.ID || !ok || v != "workspace-write" {
		t.Fatalf("undo after the warning: exit %d, %s", c, r.Summary)
	}
}

func TestPublicationWarningInTerminal(t *testing.T) {
	t.Parallel()
	b := newPolicyBed(t)
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	for _, tc := range []struct{ key, value, directory string }{{"landing.batch", "2", b.lane}, {"role.default.model.claude", "example", b.seat}} {
		f := settingBarriers{path: tc.directory, nth: 2}
		b.owners.syncDir = f.sync
		var stdout, stderr bytes.Buffer
		c := runIntentIn(mustIntentCommand(t, "settings set"), []string{tc.key, tc.value}, &stdout, &stderr, b.seat, b.owners)
		if c != 0 || !strings.Contains(stdout.String(), "survival") || !strings.Contains(stdout.String(), tc.value) || strings.Contains(stdout.String(), "nothing was set") {
			t.Fatalf("missing terminal warning for %s", tc.key)
		}
	}
}
