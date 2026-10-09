package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func helmDrainSource(record helm.Record) plain.DrainSource {
	return plain.DrainSource{Kind: "helm", Checkout: record.Checkout, By: record.By, At: record.At}
}

func requireHelmDrain(t *testing.T, install string, source plain.DrainSource) plain.Drain {
	t.Helper()
	drain, err := plain.ReadDrain(install)
	if err != nil || drain == nil || drain.Source != source || drain.By != source.By {
		t.Fatalf("drain source: %+v, want %+v: %v", drain, source, err)
	}
	return *drain
}

func TestHelmDrainDirectLifecycle(t *testing.T) {
	t.Parallel()
	b := newHelmFleetBed(t)
	// The registered installation can be nested inside its checkout.
	install := filepath.Join(b.lane, "metasystem")
	helmMust(t, os.MkdirAll(install, 0700))
	_, _, err := lane.Register(b.home, mustLaneLayout(t, b.lane, install), "Wido", b.now)
	helmMust(t, err)
	if code, _, raw := b.helm(t, b.seat, "take", "--repo", b.lane, "--reason", "by hand"); code != 0 {
		t.Fatalf("take: %d %s", code, raw)
	}
	signature := helm.Active(b.lane).Record
	requireHelmDrain(t, install, helmDrainSource(signature))
	if drain, err := plain.ReadDrain(b.lane); err != nil || drain != nil {
		t.Fatalf("drained the checkout instead of its installation: %+v %v", drain, err)
	}
	if code, _, raw := b.helm(t, b.seat, "status", "--repo", b.lane); code != 0 || !strings.Contains(raw, "landing.batch") || !strings.Contains(raw, "draining") {
		t.Fatalf("status must show policy holds and drain: %d %s", code, raw)
	}
	// Person start reopens admission even when the keeper cannot run.
	b.owners.landing.person = func(string) (string, error) { return "Wido", nil }
	b.owners.landing.ready = func(string) error { return fmt.Errorf("keeper unavailable") }
	var out bytes.Buffer
	code := runIntentIn(mustIntentCommand(t, "landing start"), []string{"--json"}, &out, &out, b.seat, b.owners)
	if code == 0 || !strings.Contains(out.String(), "admission open") || !helm.Active(b.lane).Active {
		t.Fatalf("start must reopen without returning helm: %d %s", code, &out)
	}
	if code, _, raw := b.helm(t, b.seat, "status", "--repo", b.lane); code != 0 || !strings.Contains(raw, "admission open") {
		t.Fatalf("status recreated or hid open admission: %d %s", code, raw)
	}
	if drain, err := plain.ReadDrain(install); err != nil || drain != nil {
		t.Fatalf("observer recreated drain: %+v %v", drain, err)
	}
	b.now = b.now.Add(time.Hour)
	if code, _, raw := b.helm(t, b.seat, "take", "--repo", b.lane, "--reason", "by hand"); code != 0 {
		t.Fatalf("explicit repeated take: %d %s", code, raw)
	}
	requireHelmDrain(t, install, helmDrainSource(signature))
	_, err = lane.SetPause(b.home, "Wido", b.now)
	helmMust(t, err)
	pause, _ := lane.ReadPause(b.home)
	if code, _, raw := b.helm(t, b.seat, "return", "--repo", b.lane); code != 0 || helm.Active(b.lane).Active {
		t.Fatalf("return: %d %s", code, raw)
	}
	if drain, err := plain.ReadDrain(install); err != nil || drain != nil {
		t.Fatalf("matching drain remains: %+v %v", drain, err)
	}
	if after, paused := lane.ReadPause(b.home); !paused || !reflect.DeepEqual(pause, after) {
		t.Fatal("return cleared pause")
	}
}

func mustLaneLayout(t *testing.T, root, install string) lane.Layout {
	t.Helper()
	layout, err := (lane.Record{Root: root, Install: install}).Layout()
	helmMust(t, err)
	return layout
}

func TestHelmDrainReturnPreservesOtherSources(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"manual", "checkout", "holder", "time", "unknown-source", "unreadable", "unreadable-signature", "replaced-helm"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newHelmFleetBed(t)
			if code, _, raw := b.helm(t, b.seat, "take", "--repo", b.lane, "--reason", "by hand"); code != 0 {
				t.Fatalf("take: %s", raw)
			}
			signature := helm.Active(b.lane).Record
			source := helmDrainSource(signature)
			switch kind {
			case "manual":
				source = plain.DrainSource{Kind: "person"}
			case "checkout":
				source.Checkout = b.second
			case "holder":
				source.By = "Ann"
			case "time":
				source.At = b.now.Add(time.Second).Format(time.RFC3339)
			case "unknown-source":
				source.Kind = ""
			}
			_, err := plain.ClearDrain(b.lane)
			helmMust(t, err)
			_, _, err = plain.SetDrain(b.lane, plain.Drain{By: "Original", At: signature.At, Source: source})
			helmMust(t, err)
			if kind == "unreadable" {
				helmMust(t, os.WriteFile(plain.DrainPath(b.lane), []byte("broken"), 0600))
			}
			if kind == "unreadable-signature" {
				seat, err := helm.Locate(b.lane)
				helmMust(t, err, os.WriteFile(seat.Signature, []byte("broken"), 0600))
				if code, _, raw := b.helm(t, b.seat, "status", "--repo", b.lane); code != 0 || !strings.Contains(raw, "draining") || !strings.Contains(raw, "signature is unreadable") {
					t.Fatalf("unreadable signature hid drain: %d %s", code, raw)
				}
			}
			before, err := os.ReadFile(plain.DrainPath(b.lane))
			helmMust(t, err)
			if kind == "replaced-helm" {
				if code, _, raw := b.helm(t, b.seat, "take", "--repo", b.lane, "--reason", "replacement", "--name", "Ann"); code != 0 {
					t.Fatalf("replacement: %d %s", code, raw)
				}
			}
			code, _, raw := b.helm(t, b.seat, "return", "--repo", b.lane)
			after, err := os.ReadFile(plain.DrainPath(b.lane))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("return cleared or rewrote an unmatched drain: %v", err)
			}
			if code != 0 || !strings.Contains(raw, "landing start") || helm.Active(b.lane).Active {
				t.Fatalf("return must report remaining drain and remedy: %d %s", code, raw)
			}
		})
	}
}

func TestHelmDrainPartialTakeRetry(t *testing.T) {
	t.Parallel()
	for _, all := range []bool{false, true} {
		t.Run(fmt.Sprint(all), func(t *testing.T) {
			t.Parallel()
			b := newHelmFleetBed(t)
			blocked := filepath.Join(plain.DrainPath(b.lane), "blocker")
			helmMust(t, os.MkdirAll(blocked, 0700))
			args := []string{"--repo", b.lane, "--reason", "repair"}
			if all {
				args = []string{"--all", "--reason", "repair"}
			}
			code, result, raw := b.helm(t, b.seat, "take", args...)
			if code != 1 || result.Outcome != intentFailed || !strings.Contains(raw, "partial") || !strings.Contains(raw, "drain") {
				t.Fatalf("failed drain confirmed take: %d %s", code, raw)
			}
			signature := helm.Active(b.lane).Record
			if !helm.Active(b.lane).Active || len(signature.Policies) != 8 {
				t.Fatal("partial take rolled back or skipped signature")
			}
			helmMust(t, os.Remove(blocked), os.Remove(plain.DrainPath(b.lane)))
			// Direct recovery follows the command emitted by the failed take.
			if !all {
				if result.Next == nil || len(result.Next.Argv) < 3 || result.Next.Argv[0] != "metasystem" || result.Next.Argv[1] != "helm" || result.Next.Argv[2] != "take" {
					t.Fatalf("partial take has no usable retry: %+v", result.Next)
				}
				args = result.Next.Argv[3:]
			}
			if code, _, raw := b.helm(t, b.seat, "take", args...); code != 0 {
				t.Fatalf("same explicit retry: %d %s", code, raw)
			}
			requireHelmDrain(t, b.lane, helmDrainSource(signature))
			// The partial take is logged once: its retry is a repeat and logs nothing.
			seat, err := helm.Locate(b.lane)
			helmMust(t, err)
			logged, err := os.ReadFile(seat.Log)
			helmMust(t, err)
			if takes := strings.Count(string(logged), `"action":"take"`); takes != 1 {
				t.Fatalf("partial take logged %d times: %s", takes, logged)
			}
		})
	}
}

func TestHelmDrainManualHostLifecycle(t *testing.T) {
	t.Parallel()
	b := newHelmFleetBed(t)
	b.owners.landing.person = func(string) (string, error) { return "Wido", nil }
	b.owners.landing.now = func() time.Time { return b.now }
	var out bytes.Buffer
	if code := runIntentIn(mustIntentCommand(t, "landing drain"), []string{"--reason", "manual", "--json"}, &out, &out, b.seat, b.owners); code != 0 {
		t.Fatalf("manual drain: %d %s", code, &out)
	}
	before, err := os.ReadFile(plain.DrainPath(b.lane))
	helmMust(t, err)
	if code, _, raw := b.helm(t, b.seat, "take", "--all", "--reason", "by hand"); code != 0 {
		t.Fatalf("all take: %d %s", code, raw)
	}
	after, err := os.ReadFile(plain.DrainPath(b.lane))
	helmMust(t, err)
	if !bytes.Equal(before, after) {
		t.Fatal("take replaced an existing manual drain")
	}
	out.Reset()
	if code := runIntentIn(mustIntentCommand(t, "helm status"), []string{"--all"}, &out, &out, b.seat, b.owners); code != 0 || !strings.Contains(out.String(), "draining") || !strings.Contains(out.String(), "landing.batch") {
		t.Fatalf("all text status hid drain or policy: %d %s", code, &out)
	}
	if code, _, raw := b.helm(t, b.seat, "return", "--all"); code != 0 || !strings.Contains(raw, "landing start") {
		t.Fatalf("all return hid manual drain remedy: %d %s", code, raw)
	}
	after, err = os.ReadFile(plain.DrainPath(b.lane))
	helmMust(t, err)
	if !bytes.Equal(before, after) || helm.Active(b.lane).Active {
		t.Fatal("all return removed the manual drain or retained helm")
	}
	out.Reset()
	if code := runIntentIn(mustIntentCommand(t, "helm return"), []string{"--all"}, &out, &out, b.seat, b.owners); code != 0 || !strings.Contains(out.String(), "landing start") {
		t.Fatalf("repeated all return hid remaining drain: %d %s", code, &out)
	}
}

func TestHelmDrainReturnRemovalFailure(t *testing.T) {
	t.Parallel()
	b := newHelmFleetBed(t)
	if code, _, raw := b.helm(t, b.seat, "take", "--repo", b.lane, "--reason", "by hand"); code != 0 {
		t.Fatalf("take: %d %s", code, raw)
	}
	drain := requireHelmDrain(t, b.lane, helmDrainSource(helm.Active(b.lane).Record))
	lockPath := filepath.Join(plain.Dir(b.lane), "lane.lock")
	helmMust(t, os.Remove(lockPath), os.Mkdir(lockPath, 0700))
	if code, _, raw := b.helm(t, b.seat, "return", "--repo", b.lane); code != 0 || helm.Active(b.lane).Active || !strings.Contains(raw, "lane.lock") || !strings.Contains(raw, "landing start") {
		t.Fatalf("return must remove override and report drain failure: %d %s", code, raw)
	}
	if after := requireHelmDrain(t, b.lane, drain.Source); !reflect.DeepEqual(after, drain) {
		t.Fatal("failed clear changed drain")
	}
	helmMust(t, os.Remove(lockPath))
	b.owners.landing.person = func(string) (string, error) { return "Wido", nil }
	b.owners.landing.ready = func(string) error { return fmt.Errorf("keeper unavailable") }
	var out bytes.Buffer
	code := runIntentIn(mustIntentCommand(t, "landing start"), []string{"--json"}, &out, &out, b.seat, b.owners)
	if code == 0 || !strings.Contains(out.String(), "admission open") {
		t.Fatalf("return's repair command did not reopen admission: %d %s", code, &out)
	}
	if remaining, err := plain.ReadDrain(b.lane); err != nil || remaining != nil {
		t.Fatalf("repair left drain: %+v %v", remaining, err)
	}
}

func TestHelmDrainAuthorityAndSignatureFailure(t *testing.T) {
	t.Parallel()
	b := newHelmFleetBed(t)
	seat, err := helm.Locate(b.lane)
	helmMust(t, err, os.MkdirAll(filepath.Join(seat.Signature, "blocker"), 0700))
	if code, _, raw := b.helm(t, b.seat, "take", "--repo", b.lane, "--reason", "repair"); code != 1 {
		t.Fatalf("signature write: %d %s", code, raw)
	}
	if drain, err := plain.ReadDrain(b.lane); err != nil || drain != nil {
		t.Fatalf("drain written before signature: %+v %v", drain, err)
	}
	helmMust(t, os.Remove(filepath.Join(seat.Signature, "blocker")), os.Remove(seat.Signature))
	if code, _, raw := b.helm(t, b.seat, "take", "--repo", b.lane, "--reason", "repair"); code != 0 {
		t.Fatalf("person retry: %d %s", code, raw)
	}
	drain := requireHelmDrain(t, b.lane, helmDrainSource(helm.Active(b.lane).Record))
	signature := helm.Active(b.lane).Record
	b.owners.helm.pid = func() int64 { return 80 }
	for _, act := range []string{"take", "return"} {
		args := []string{"--repo", b.lane}
		if act == "take" {
			args = append(args, "--reason", "agent")
		}
		if code, _, raw := b.helm(t, b.seat, act, args...); code != 3 {
			t.Fatalf("agent %s: %d %s", act, code, raw)
		}
		if got := requireHelmDrain(t, b.lane, drain.Source); !reflect.DeepEqual(got, drain) || !reflect.DeepEqual(signature, helm.Active(b.lane).Record) {
			t.Fatal("agent changed signature or drain")
		}
	}
	// Unavailable ledger advice must not turn an ordinary seat take into a
	// lane failure when no lane registration is known.
	b.owners.helm.pid = func() int64 { return 20 }
	b.owners.policies.Registry = func(string) (config.PolicyRegistry, error) {
		return config.PolicyRegistry{}, fmt.Errorf("ledger unavailable")
	}
	if code, _, raw := b.helm(t, b.seat, "take", "--repo", b.second, "--reason", "recover seat"); code != 0 {
		t.Fatalf("ledger advice refused an ordinary seat take: %d %s", code, raw)
	}
}
