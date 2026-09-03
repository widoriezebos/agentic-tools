package phase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/fake"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/slack"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/telegram"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

const (
	TelegramBotTokenKey = "channel.destination.fleet.telegram.bot-token"
	TelegramAPIBaseKey  = "channel.destination.fleet.telegram.api-base"
)

func Get(root, key, def string) (string, error) {
	v, code, err := config.Get(config.GetParams{Key: key, ConfPath: filepath.Join(root, "metasystem.conf"), Default: def, DefaultSet: def != ""})
	if code != 0 {
		return "", err
	}
	return v, nil
}

func Secret(root, key string) (string, error) {
	if v, ok := os.LookupEnv(config.EnvName(key)); ok {
		return v, nil
	}
	path := filepath.Join(root, "metasystem.conf")
	if v, ok, err := config.ConfLookup(path+".local", key); err == nil && ok {
		return v, nil
	}
	if v, ok, _ := config.ConfLookup(path, key); ok && v != "" {
		return "", fmt.Errorf("committed secret setting %s is ignored", key)
	}
	return "", fmt.Errorf("no value configured for %s", key)
}

type Loaded struct {
	Provider    channel.Provider
	Destination channel.DestinationConfig
	Adapter     string
	Face        string
	HumanUserID string
	TOTPSecret  string
}

type adapterLoad func(root string) (p channel.Provider, d channel.DestinationConfig, face string, err error)

var adapters = map[string]adapterLoad{
	"slack":    loadSlack,
	"telegram": loadTelegram,
	"fake":     loadFake,
}

func loadSlack(root string) (channel.Provider, channel.DestinationConfig, string, error) {
	cid, err := Get(root, "channel.destination.fleet.slack.channel-id", "")
	if err != nil {
		return nil, channel.DestinationConfig{}, "slack", err
	}
	token, err := Secret(root, "channel.destination.fleet.slack.bot-token")
	if err != nil {
		return nil, channel.DestinationConfig{}, "slack", channel.ErrUnconfigured(err.Error())
	}
	base, err := Get(root, "channel.destination.fleet.slack.api-base", "https://slack.com/api")
	if err != nil {
		return nil, channel.DestinationConfig{}, "slack", err
	}
	dest := channel.DestinationConfig{Name: "fleet", Provider: "slack", ChannelID: cid, Token: token, APIBase: base, Secrets: []string{token}}
	return slack.New(nil), dest, "slack", nil
}

func loadTelegram(root string) (channel.Provider, channel.DestinationConfig, string, error) {
	chatID, err := Get(root, "channel.destination.fleet.telegram.chat-id", "")
	if err != nil {
		return nil, channel.DestinationConfig{}, "telegram", err
	}
	token, err := Secret(root, TelegramBotTokenKey)
	if err != nil {
		return nil, channel.DestinationConfig{}, "telegram", channel.ErrUnconfigured(err.Error())
	}
	base, err := Get(root, TelegramAPIBaseKey, "https://api.telegram.org")
	if err != nil {
		return nil, channel.DestinationConfig{}, "telegram", err
	}
	dest := channel.DestinationConfig{Name: "fleet", Provider: "telegram", ChannelID: chatID, Token: token, APIBase: base, Secrets: []string{token}}
	return telegram.New(nil), dest, "telegram", nil
}

func loadFake(root string) (channel.Provider, channel.DestinationConfig, string, error) {
	dir, err := Get(root, "channel.destination.fleet.fake.dir", "")
	if err != nil {
		return nil, channel.DestinationConfig{}, "", err
	}
	face, err := Get(root, "channel.destination.fleet.fake.face", "slack")
	if err != nil {
		return nil, channel.DestinationConfig{}, "", err
	}
	switch face {
	case "slack":
		p, d, err := fake.Provider(dir)
		return p, d, face, err
	case "telegram":
		p, d, err := fake.TelegramProvider(dir)
		return p, d, face, err
	default:
		return nil, channel.DestinationConfig{}, "", fmt.Errorf("unknown fake face %q", face)
	}
}

func Load(root string, withHuman bool) (Loaded, error) {
	adapter, err := Get(root, "channel.destination.fleet.adapter", "")
	if err != nil || adapter == "" {
		return Loaded{}, nil
	}
	resolver := adapters[adapter]
	if resolver == nil {
		return Loaded{}, fmt.Errorf("unknown channel adapter %q", adapter)
	}
	p, d, face, err := resolver(root)
	loaded := Loaded{Provider: p, Destination: d, Adapter: adapter, Face: face}
	if err != nil || !withHuman {
		return loaded, err
	}
	loaded.HumanUserID, err = Get(root, "channel.human."+face+".user-id", "")
	if err != nil {
		return loaded, err
	}
	loaded.TOTPSecret, err = Secret(root, "channel.human.totp-secret")
	return loaded, err
}

// Run performs the bounded channel duty after every other tick duty.
func Run(ctx context.Context, root string) (int, error) {
	loaded, err := Load(root, true)
	if loaded.Provider == nil && err == nil {
		return 0, nil
	}
	if err != nil {
		return 1, err
	}
	machine, err := goal.ResolveMachine(root)
	if err != nil {
		return 1, err
	}
	lineage := os.Getenv("METASYSTEM_OWNER_LINEAGE")
	if lineage == "" {
		lineage = "steward"
	}
	res, err := channel.Poll(ctx, channel.PollConfig{RepoRoot: root, Destination: "fleet", ProviderName: loaded.Adapter, HumanUserID: loaded.HumanUserID, TOTPSecret: loaded.TOTPSecret, Machine: machine, Lineage: lineage, Provider: loaded.Provider, DestinationConfig: loaded.Destination, Now: time.Now(), MaxDispositions: 5})
	if err != nil {
		return res.Undelivered + 1, err
	}
	text, err := channel.ComposeReport(channel.ReportConfig{RepoRoot: root, Machine: machine, Now: time.Now(), Undelivered: res.Undelivered})
	if err != nil {
		return res.Undelivered + 1, err
	}
	state := channel.LoadStatusState(root)
	minutes := 240
	if raw, e := Get(root, "channel.status.interval-minutes", "240"); e == nil {
		if n, e := strconv.Atoi(raw); e == nil && n > 0 {
			minutes = n
		}
	}
	if channel.ShouldPost(state, time.Now(), time.Duration(minutes)*time.Minute, text, false) {
		ref, e := loaded.Provider.Post(ctx, loaded.Destination, text, nil)
		if e != nil {
			return res.Undelivered + 1, e
		}
		state = channel.StatusState{LastPost: time.Now().UTC(), ContentDigest: channel.Digest(text), Ref: ref}
		e = channel.SaveStatusState(root, state)
		if e != nil {
			return res.Undelivered + 1, e
		}
	}
	return res.Undelivered, nil
}
