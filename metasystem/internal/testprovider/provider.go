// Package testprovider supplies registered, isolated provider state to fixtures.
package testprovider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

func Home(root string) string { return filepath.Join(root, ".metasystem") }
func Register(t *testing.T, root string) string {
	t.Helper()
	home := Home(root)
	if _, _, err := lane.Register(home, lane.Layout{Checkout: lane.CheckoutRoot(root), Install: lane.InstallRoot(root)}, "fixture", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	return home
}
func Record(root, class, detail, source string, at time.Time) (outage.Mark, error) {
	home := Home(root)
	if _, _, err := lane.Register(home, lane.Layout{Checkout: lane.CheckoutRoot(root), Install: lane.InstallRoot(root)}, "fixture", at); err != nil {
		return outage.Mark{}, err
	}
	return outage.Observe(home, "claude", "fixture-model", class, detail, source, at)
}
func Read(root string, runtimes ...string) (outage.Mark, bool) {
	s, err := outage.ReadProviders(Home(root))
	if err != nil {
		return outage.Mark{}, false
	}
	runtime := "claude"
	if len(runtimes) > 0 {
		runtime = runtimes[0]
	}
	m := s.Current[outage.Provider(runtime)].Mark
	return m, m.ConsecutiveFailures > 0
}
func StandingAt(root string, at time.Time) (outage.Mark, bool) {
	s, err := outage.ReadProviders(Home(root))
	if err != nil {
		return outage.Mark{}, false
	}
	return s.Standing("claude", at)
}
func Clear(root string) error {
	m, _ := Read(root)
	at, _ := time.Parse(time.RFC3339Nano, m.LastAt)
	_, err := outage.Observe(Home(root), "claude", "fixture-model", "", "", "fixture-success", at.Add(time.Nanosecond))
	return err
}
func Path(root string) string {
	owner, _, _ := lane.Read(Home(root))
	return filepath.Join(root, "artifacts", "agents", fmt.Sprintf("providers-%d.json", owner.CustodyEpoch))
}
func Write(root string, m outage.Mark) error {
	s, err := outage.ReadProviders(Home(root))
	if err != nil {
		return err
	}
	c := s.Current["anthropic"]
	c.Mark = m
	s.Current["anthropic"] = c
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(Path(root), data, 0o600)
}
