package config

import (
	"fmt"
	"os"
)

const (
	UIListenKey     = "ui.listen"
	DefaultUIListen = "127.0.0.1:7878"
	UISubjectKey    = "ui.subject"
)

// UIListen resolves the interface listen address and returns it unvalidated;
// the loopback rule lives in lifecycle.ValidateListen alone.
func UIListen(confPath, flag string, flagSet bool) (string, error) {
	return uiListen(confPath, flag, flagSet, os.LookupEnv)
}

// uiListen takes the environment lookup so a test can resolve without the
// ambient environment, which it may not clear.
func uiListen(confPath, flag string, flagSet bool, lookupEnv func(string) (string, bool)) (string, error) {
	value, _, err := Get(GetParams{
		Key:        UIListenKey,
		Flag:       flag,
		FlagSet:    flagSet,
		Default:    DefaultUIListen,
		DefaultSet: true,
		ConfPath:   confPath,
		LookupEnv:  lookupEnv,
	})
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", UIListenKey, err)
	}
	return value, nil
}

// UISubject resolves the name the interface gives this workspace. The default
// is empty, deliberately: what an unconfigured subject means is the interface's
// rule, which names the template itself and an adopted workspace after the
// checkout it serves, and that rule has one owner.
func UISubject(confPath string) (string, error) {
	return uiSubject(confPath, os.LookupEnv)
}

// uiSubject takes the environment lookup for the same reason uiListen does.
func uiSubject(confPath string, lookupEnv func(string) (string, bool)) (string, error) {
	value, _, err := Get(GetParams{
		Key:        UISubjectKey,
		Default:    "",
		DefaultSet: true,
		ConfPath:   confPath,
		LookupEnv:  lookupEnv,
	})
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", UISubjectKey, err)
	}
	return value, nil
}
