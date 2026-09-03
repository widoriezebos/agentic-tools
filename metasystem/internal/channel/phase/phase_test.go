package phase_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
)

func writeConfig(t *testing.T, root, config, local string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	if local != "" {
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf.local"), []byte(local), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func fakeRoot(t *testing.T, face string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "fake")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "base-url"), []byte("http://127.0.0.1:1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, root, "channel.destination.fleet.adapter=fake\nchannel.destination.fleet.fake.dir="+dir+"\nchannel.destination.fleet.fake.face="+face+"\nchannel.human.slack.user-id=slack-human\nchannel.human.telegram.user-id=7001\n", "channel.human.totp-secret=JBSWY3DPEHPK3PXP\n")
	return root
}

func TestLoadResolvesAdaptersThroughOneTable(t *testing.T) {
	for _, tc := range []struct{ face, user string }{{"slack", "slack-human"}, {"telegram", "7001"}} {
		root := fakeRoot(t, tc.face)
		loaded, err := phase.Load(root, true)
		if err != nil || loaded.Provider == nil || loaded.Adapter != "fake" || loaded.Face != tc.face || loaded.HumanUserID != tc.user {
			t.Fatalf("face %s: %+v %v", tc.face, loaded, err)
		}
	}
	t.Run("telegram missing token is typed", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "channel.destination.fleet.adapter=telegram\nchannel.destination.fleet.telegram.chat-id=1000\n", "")
		_, err := phase.Load(root, false)
		if err == nil || !channel.IsKind(err, channel.Unconfigured) {
			t.Fatal(err)
		}
	})
	t.Run("committed token is ignored", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "channel.destination.fleet.adapter=telegram\nchannel.destination.fleet.telegram.chat-id=1000\nchannel.destination.fleet.telegram.bot-token=committed\n", "")
		_, err := phase.Load(root, false)
		if err == nil || !strings.Contains(err.Error(), "committed secret setting") || strings.Contains(err.Error(), "=committed") {
			t.Fatal(err)
		}
	})
	t.Run("unknown adapter is named", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "channel.destination.fleet.adapter=carrier-pigeon\n", "")
		_, err := phase.Load(root, false)
		if err == nil || err.Error() != `unknown channel adapter "carrier-pigeon"` {
			t.Fatal(err)
		}
	})
	t.Run("absent adapter is silent", func(t *testing.T) {
		loaded, err := phase.Load(t.TempDir(), true)
		if err != nil || loaded.Provider != nil {
			t.Fatal(loaded, err)
		}
	})
}

func TestOutboundVerbsDoNotRequireHumanAuthentication(t *testing.T) {
	loaded, err := phase.Load(fakeRoot(t, "telegram"), false)
	if err != nil || loaded.Provider == nil || loaded.HumanUserID != "" || loaded.TOTPSecret != "" {
		t.Fatal(loaded, err)
	}
}

func TestAbsentAdapterNeverCallsNilProvider(t *testing.T) {
	loaded, err := phase.Load(t.TempDir(), true)
	if err != nil || loaded.Provider != nil {
		t.Fatal(loaded, err)
	}
}

func TestCloseWithoutAdapterClosesLocallyWithoutCallingProvider(t *testing.T) {
	root := t.TempDir()
	q, err := channel.Ask(channel.AskRequest{Context: context.Background(), RepoRoot: root, Goal: "g", Kind: "other", Machine: "m", Facts: []string{"fact"}})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := phase.Load(root, false)
	if err != nil || loaded.Provider != nil {
		t.Fatal(loaded, err)
	}
	if err := channel.Close(root, q.ID, "withdrawn", loaded.Provider, loaded.Destination); err != nil {
		t.Fatal(err)
	}
	got, err := channel.ReadQuestion(root, q.ID)
	if err != nil || got.State != "closed" {
		t.Fatal(got, err)
	}
}

func TestLoadRejectsUnknownFakeFace(t *testing.T) {
	_, err := phase.Load(fakeRoot(t, "whatsapp"), false)
	if err == nil || err.Error() != `unknown fake face "whatsapp"` {
		t.Fatal(err)
	}
}

func TestLoadIsTheOnlyLoader(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "cmd", "metasystem", "channel_verbs.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "channel.destination.") || strings.Contains(string(b), "loadChannel") {
		t.Fatal("command package still owns channel loading")
	}
}
