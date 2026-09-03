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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func get(root, key, def string) (string, error) {
	v, code, err := config.Get(config.GetParams{Key: key, ConfPath: filepath.Join(root, "metasystem.conf"), Default: def, DefaultSet: def != ""})
	if code != 0 {
		return "", err
	}
	return v, nil
}
func secret(root, key string) (string, error) {
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
func load(root string) (channel.Provider, channel.DestinationConfig, string, string, string, error) {
	adapter, err := get(root, "channel.destination.fleet.adapter", "")
	if err != nil {
		return nil, channel.DestinationConfig{}, "", "", "", nil
	}
	var p channel.Provider
	var d channel.DestinationConfig
	if adapter == "fake" {
		dir, e := get(root, "channel.destination.fleet.fake.dir", "")
		if e != nil {
			return nil, d, "", "", "", e
		}
		p, d, err = fake.Provider(dir)
		if err != nil {
			return nil, d, "", "", "", err
		}
	} else if adapter == "slack" {
		cid, e := get(root, "channel.destination.fleet.slack.channel-id", "")
		if e != nil {
			return nil, d, "", "", "", e
		}
		token, e := secret(root, "channel.destination.fleet.slack.bot-token")
		if e != nil {
			return nil, d, "", "", "", e
		}
		base, _ := get(root, "channel.destination.fleet.slack.api-base", "https://slack.com/api")
		d = channel.DestinationConfig{Name: "fleet", Provider: "slack", ChannelID: cid, Token: token, APIBase: base, Secrets: []string{token}}
		p = slack.New(nil)
	} else {
		return nil, d, "", "", "", fmt.Errorf("unknown channel adapter %q", adapter)
	}
	user, err := get(root, "channel.human.slack.user-id", "")
	if err != nil {
		return nil, d, "", "", "", err
	}
	totp, err := secret(root, "channel.human.totp-secret")
	return p, d, adapter, user, totp, err
}

// Run performs the bounded channel duty after every other tick duty.
func Run(ctx context.Context, root string) (int, error) {
	p, d, adapter, user, totp, err := load(root)
	if p == nil && err == nil {
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
	res, err := channel.Poll(ctx, channel.PollConfig{RepoRoot: root, Destination: "fleet", ProviderName: adapter, HumanUserID: user, TOTPSecret: totp, Machine: machine, Lineage: lineage, Provider: p, DestinationConfig: d, Now: time.Now(), MaxDispositions: 5})
	if err != nil {
		return res.Undelivered + 1, err
	}
	text, err := channel.ComposeReport(channel.ReportConfig{RepoRoot: root, Machine: machine, Now: time.Now(), Undelivered: res.Undelivered})
	if err != nil {
		return res.Undelivered + 1, err
	}
	state := channel.LoadStatusState(root)
	minutes := 240
	if raw, e := get(root, "channel.status.interval-minutes", "240"); e == nil {
		if n, e := strconv.Atoi(raw); e == nil && n > 0 {
			minutes = n
		}
	}
	if channel.ShouldPost(state, time.Now(), time.Duration(minutes)*time.Minute, text, false) {
		ref, e := p.Post(ctx, d, text, nil)
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
