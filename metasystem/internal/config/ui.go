package config

import "fmt"

const (
	UIListenKey     = "ui.listen"
	DefaultUIListen = "127.0.0.1:7878"
)

func UIListen(confPath, flag string, flagSet bool) (string, error) {
	value, _, err := Get(GetParams{
		Key:        UIListenKey,
		Flag:       flag,
		FlagSet:    flagSet,
		Default:    DefaultUIListen,
		DefaultSet: true,
		ConfPath:   confPath,
	})
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", UIListenKey, err)
	}
	return value, nil
}
