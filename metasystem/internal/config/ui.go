package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	UIListenKey     = "ui.listen"
	DefaultUIListen = "127.0.0.1:7878"
	UISubjectKey    = "ui.subject"
	// UISessionHoursKey is how long a browser session the human signed into
	// lasts. Twelve hours is a working day and the evening after it; a seat
	// that wants a shorter leash sets its own.
	UISessionHoursKey     = "ui.session.hours"
	DefaultUISessionHours = 12
	// UIHumanKey is the handle this seat signs in as when no enrolled
	// terminal started the server. Unset, the sign-in sheet asks once.
	UIHumanKey = "ui.human"
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

// UISessionHours resolves how long a signed-in browser session lasts. It is
// read like every other ui. key; a value that is not a positive whole number of
// hours is a refusal rather than a silent fallback, because a seat that meant
// to shorten its sessions must not be given the default instead.
func UISessionHours(confPath string) (time.Duration, error) {
	return uiSessionHours(confPath, os.LookupEnv)
}

func uiSessionHours(confPath string, lookupEnv func(string) (string, bool)) (time.Duration, error) {
	value, _, err := Get(GetParams{
		Key:        UISessionHoursKey,
		Default:    strconv.Itoa(DefaultUISessionHours),
		DefaultSet: true,
		ConfPath:   confPath,
		LookupEnv:  lookupEnv,
	})
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", UISessionHoursKey, err)
	}
	hours, convErr := strconv.ParseInt(value, 10, 32)
	if convErr != nil || hours < 1 || hours > 24*365 {
		return 0, fmt.Errorf("%s must be a whole number of hours, from 1 to %d", UISessionHoursKey, 24*365)
	}
	return time.Duration(hours) * time.Hour, nil
}

// UIHuman resolves the handle this seat signs in as. The default is empty,
// deliberately: what an unconfigured handle means belongs to the interface,
// which asks the human once and remembers the answer.
func UIHuman(confPath string) (string, error) {
	return uiHuman(confPath, os.LookupEnv)
}

func uiHuman(confPath string, lookupEnv func(string) (string, bool)) (string, error) {
	value, _, err := Get(GetParams{
		Key:        UIHumanKey,
		Default:    "",
		DefaultSet: true,
		ConfPath:   confPath,
		LookupEnv:  lookupEnv,
	})
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", UIHumanKey, err)
	}
	return value, nil
}
