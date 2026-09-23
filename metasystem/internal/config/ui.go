package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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
	// UIPartnerRuntimeKey names which agent answers as the Project Partner:
	// claude, codex or devin. Unset — the default — is a seat with no
	// Partner, which is deliberate: configuring one also closes the boot
	// proof's path to the act routes, and no seat should have that happen to
	// it because a default changed under it.
	UIPartnerRuntimeKey = "ui.partner.runtime"
	// UIPartnerModelKey is the model the Partner's session runs. Unset is the
	// runtime's own default for that runtime; a model the runtime cannot
	// select refuses the turn with the runtime's own words.
	UIPartnerModelKey = "ui.partner.model"
	// UIPartnerCommandPrefix is where a seat says how to start one runtime's
	// ACP server, as a command line: ui.partner.command.claude,
	// ui.partner.command.codex, ui.partner.command.devin. Unset is the
	// runtime's published entry point on PATH.
	UIPartnerCommandPrefix = "ui.partner.command."
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

// UIPartner resolves the Partner's three settings: which runtime answers,
// which model it runs, and the command that starts that runtime's ACP server.
// An empty runtime is a seat with no Partner, and the other two are then not
// read at all — a seat that named no Partner has no command to resolve.
func UIPartner(confPath string) (runtime, model, command string, err error) {
	return uiPartner(confPath, os.LookupEnv)
}

func uiPartner(confPath string, lookupEnv func(string) (string, bool)) (string, string, string, error) {
	runtime, err := uiPartnerValue(confPath, UIPartnerRuntimeKey, lookupEnv)
	if err != nil {
		return "", "", "", err
	}
	runtime = strings.TrimSpace(runtime)
	if runtime == "" {
		return "", "", "", nil
	}
	model, err := uiPartnerValue(confPath, UIPartnerModelKey, lookupEnv)
	if err != nil {
		return "", "", "", err
	}
	command, err := uiPartnerValue(confPath, UIPartnerCommandPrefix+runtime, lookupEnv)
	if err != nil {
		return "", "", "", err
	}
	return runtime, strings.TrimSpace(model), strings.TrimSpace(command), nil
}

// uiPartnerValue reads one ui.partner. key the way every other ui. key is
// read, defaulting to empty.
func uiPartnerValue(confPath, key string, lookupEnv func(string) (string, bool)) (string, error) {
	value, _, err := Get(GetParams{
		Key:        key,
		Default:    "",
		DefaultSet: true,
		ConfPath:   confPath,
		LookupEnv:  lookupEnv,
	})
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", key, err)
	}
	return value, nil
}
