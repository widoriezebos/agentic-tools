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

// UISetting is one `ui.` key as the interface describes itself: what the key
// decides, what it is when nobody sets it, and what it is on this seat.
//
// The register below is the one place that says which keys the interface has
// and what each is for. The resolvers above read them; this says what they
// mean, so a human asking the Project Partner what can be configured here is
// answered out of the same file that answers the engine.
type UISetting struct {
	Key string
	// Purpose is what this key decides, in one sentence.
	Purpose string
	// Default is the value the engine uses when no source holds the key.
	Default string
	// Value is what this seat resolves the key to, which ReadUISettings fills
	// and UISettings leaves empty.
	Value string
	// Family marks a key whose last segment is a name the seat chooses — the
	// per-runtime Partner command — so a reader knows it is a pattern rather
	// than a key to set verbatim.
	Family bool
	// Secret marks a value that must never leave this seat. No `ui.` key is
	// one today; the field exists so that the day one is, its value is left
	// out rather than remembered about.
	Secret bool
}

// UISettings is every `ui.` key, with what it decides and its default. It
// reads nothing.
func UISettings() []UISetting {
	return []UISetting{
		{Key: UIListenKey, Default: DefaultUIListen,
			Purpose: "The loopback address the interface server listens on."},
		{Key: UISubjectKey, Default: "",
			Purpose: "The name the interface gives this workspace. Unset, the interface names the template itself and an adopted workspace after the checkout it serves."},
		{Key: UISessionHoursKey, Default: strconv.Itoa(DefaultUISessionHours),
			Purpose: "How many hours a signed-in browser session lasts before it expires."},
		{Key: UIHumanKey, Default: "",
			Purpose: "The handle this seat signs in as. Unset, the sign-in sheet asks once."},
		{Key: UIPartnerRuntimeKey, Default: "",
			Purpose: "Which agent answers as the Project Partner: claude, codex or devin. Unset is a seat with no Partner, and naming one also requires a signed-in human for every act."},
		{Key: UIPartnerModelKey, Default: "",
			Purpose: "The model the Partner's session runs. Unset is the runtime's own default."},
		{Key: UIPartnerCommandPrefix + "<runtime>", Default: "", Family: true,
			Purpose: "How to start one runtime's ACP server, as a command line. Unset is that runtime's published entry point on PATH."},
	}
}

// ReadUISettings is the register with each key resolved on this seat.
//
// A family is left at its default, because its name is a pattern and there is
// no single value to read. A secret is left unread, so that no reader of this
// register can carry one off the seat. A key this seat cannot resolve carries
// the reader's own refusal where its value would be.
func ReadUISettings(confPath string) []UISetting {
	return readUISettings(confPath, os.LookupEnv)
}

func readUISettings(confPath string, lookupEnv func(string) (string, bool)) []UISetting {
	settings := UISettings()
	for index := range settings {
		if settings[index].Family || settings[index].Secret {
			continue
		}
		value, _, err := Get(GetParams{
			Key:        settings[index].Key,
			Default:    settings[index].Default,
			DefaultSet: true,
			ConfPath:   confPath,
			LookupEnv:  lookupEnv,
		})
		if err != nil {
			settings[index].Value = "this seat could not resolve this key: " + err.Error()
			continue
		}
		settings[index].Value = value
	}
	return settings
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
