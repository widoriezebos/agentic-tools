package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// O5: every case resolves through the injected environment lookup, so no case
// depends on the ambient environment and none has to set one.
func TestUIListenResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		committed string
		flag      string
		flagSet   bool
		lookup    func(string) (string, bool)
		want      string
	}{
		{name: "default", want: DefaultUIListen},
		{name: "flag over committed value", committed: "127.0.0.1:9000", flag: "localhost:not-validated-here", flagSet: true, want: "localhost:not-validated-here"},
		{name: "empty flag still wins", committed: "127.0.0.1:9000", flagSet: true, want: ""},
		{
			name:      "environment over committed value",
			committed: "127.0.0.1:9000",
			lookup:    mapEnv(map[string]string{EnvName(UIListenKey): "127.0.0.1:9100"}),
			want:      "127.0.0.1:9100",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			confPath := filepath.Join(t.TempDir(), "metasystem.conf")
			body := ""
			if tc.committed != "" {
				body = UIListenKey + "=" + tc.committed + "\n"
			}
			err := os.WriteFile(confPath, []byte(body), 0o644)
			testutil.Require(t, "write configuration", err, nil)
			lookup := tc.lookup
			if lookup == nil {
				lookup = noEnv
			}

			got, err := uiListen(confPath, tc.flag, tc.flagSet, lookup)
			testutil.Require(t, "resolve UI listen address", err, nil)
			testutil.Expect(t, "resolved UI listen address", got, tc.want)
		})
	}
}

// The private store's three bounds, read like every other `ui.` key: the shipped
// defaults where nobody sets them, this seat's numbers where it does, zero
// accepted as the one way to turn a bound off, and a refusal rather than a silent
// fallback for anything that is not a whole number in range (g1-s54 D4).
//
// O5: every case resolves through the injected environment lookup, so no case
// depends on the ambient environment and none has to set one.
func TestUIStoreBoundsResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		committed string
		lookup    func(string) (string, bool)
		want      UIStore
		refused   string
	}{
		{
			name: "the shipped defaults",
			want: UIStore{
				WireMB:           DefaultUIStoreWireMB,
				ConversationMB:   DefaultUIStoreConversationMB,
				ConversationDays: DefaultUIStoreConversationDays,
			},
		},
		{
			name: "a seat's own numbers",
			committed: UIStoreWireMBKey + "=32\n" +
				UIStoreConversationMBKey + "=16\n" +
				UIStoreConversationDaysKey + "=365\n",
			want: UIStore{WireMB: 32, ConversationMB: 16, ConversationDays: 365},
		},
		{
			name: "zero disables a bound",
			committed: UIStoreWireMBKey + "=0\n" +
				UIStoreConversationMBKey + "=0\n" +
				UIStoreConversationDaysKey + "=0\n",
			want: UIStore{},
		},
		{
			name:      "the environment over the committed value",
			committed: UIStoreWireMBKey + "=32\n",
			lookup:    mapEnv(map[string]string{EnvName(UIStoreWireMBKey): "64"}),
			want: UIStore{
				WireMB:           64,
				ConversationMB:   DefaultUIStoreConversationMB,
				ConversationDays: DefaultUIStoreConversationDays,
			},
		},
		{
			name:      "a value that is not a number is refused",
			committed: UIStoreConversationMBKey + "=a couple\n",
			refused:   UIStoreConversationMBKey + " must be a whole number from 0 to 100000, where 0 disables the bound",
		},
		{
			name:      "a negative bound is refused",
			committed: UIStoreWireMBKey + "=-1\n",
			refused:   UIStoreWireMBKey + " must be a whole number from 0 to 100000, where 0 disables the bound",
		},
		{
			name:      "a bound past the range is refused with the range in it",
			committed: UIStoreConversationDaysKey + "=100000\n",
			refused:   UIStoreConversationDaysKey + " must be a whole number from 0 to 36500, where 0 disables the bound",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			confPath := filepath.Join(t.TempDir(), "metasystem.conf")
			err := os.WriteFile(confPath, []byte(tc.committed), 0o644)
			testutil.Require(t, "write configuration", err, nil)
			lookup := tc.lookup
			if lookup == nil {
				lookup = noEnv
			}

			got, err := uiStoreBounds(confPath, lookup)

			if tc.refused != "" {
				testutil.Require(t, "the bound is refused", err != nil, true)
				testutil.Expect(t, "in these words", err.Error(), tc.refused)
				testutil.Expect(t, "and nothing is resolved", got, UIStore{})
				return
			}
			testutil.Require(t, "resolve the store's bounds", err, nil)
			testutil.Expect(t, "the three numbers", got, tc.want)
		})
	}
}

// The register is the one place that says which keys the interface has, so a
// bound a seat can set is a bound the Partner can be asked about.
func TestTheStoreBoundsAreInTheRegister(t *testing.T) {
	t.Parallel()
	held := map[string]string{}
	for _, setting := range UISettings() {
		held[setting.Key] = setting.Default
	}
	testutil.Expect(t, "the journal's key and its default", held[UIStoreWireMBKey], "8")
	testutil.Expect(t, "the conversation's size", held[UIStoreConversationMBKey], "2")
	testutil.Expect(t, "and its days", held[UIStoreConversationDaysKey], "90")
}
