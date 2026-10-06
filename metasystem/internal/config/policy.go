package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// PolicyRegistry is a current observation of the host's declared owners.
// An absent coordinator is an empty layer; unreadable declarations are errors.
type PolicyRegistry struct{ Lane, Coordinator string }
type PolicyReaders struct {
	Registry func(string) (PolicyRegistry, error)
	Helm     func(string) helm.State
	ConfPath func(string) (string, error)
}

// PolicyReadError retains the failed layer so a caller can name its repair.
type PolicyReadError struct {
	Source, Checkout string
	Err              error
}

func (e *PolicyReadError) Error() string { return e.Err.Error() }
func (e *PolicyReadError) Unwrap() error { return e.Err }

type PolicyContext struct {
	Checkout, CallingCheckout string
	Readers                   PolicyReaders
}

type PolicyValue struct {
	Value    string    `json:"value"`
	Source   string    `json:"source"`
	Checkout string    `json:"checkout"`
	SetBy    string    `json:"set-by"`
	At       time.Time `json:"at,omitempty"`
}

type PolicyResolution struct {
	Name string `json:"name"`
	PolicyValue
	Previous   *PolicyValue `json:"previous,omitempty"`
	HelmHolder string       `json:"helm-holder,omitempty"`
}

type PolicyRecord struct {
	Name        string      `json:"name"`
	Checkout    string      `json:"checkout"`
	Value       string      `json:"value"`
	SetBy       string      `json:"set-by"`
	At          time.Time   `json:"at"`
	Previous    PolicyValue `json:"previous"`
	Fingerprint string      `json:"fingerprint"`
}

func PolicyScope(key string) string {
	switch key {
	case "landing.batch", "landing.proof", "landing.on-red", "landing.trunk-red":
		return "lane"
	case "seat.driver", "review.stop", "goal.raise":
		return "seat"
	case "question.route":
		return "coordinator"
	case "settings.apply":
		return "committed"
	}
	return ""
}

func policyValueProblem(key, value string) error {
	grammar := "auto or person"
	valid := value == "auto" || value == "person"
	switch key {
	case "settings.apply":
		grammar, valid = "boundary or now", value == "boundary" || value == "now"
	case "landing.batch", "review.stop":
		grammar = "auto, person, or a decimal integer at least 1"
		if key == "review.stop" {
			grammar = "auto, person, or a decimal integer at least 0"
		}
		// Decimal caps have no machine-integer ceiling.
		if digitsOnlyValue.MatchString(value) && (key == "review.stop" || strings.TrimLeft(value, "0") != "") {
			valid = true
		}
	}
	if !valid {
		return fmt.Errorf("%s must be %s, not %q", key, grammar, value)
	}
	return nil
}

// ResolvePolicy owns policy precedence and attribution. Configuration bytes
// are authoritative; metadata can only attribute a matching observation.
func ResolvePolicy(p GetParams) (PolicyResolution, error) {
	result := PolicyResolution{Name: p.Key}
	if PolicyScope(p.Key) == "" {
		return result, fmt.Errorf("%s is not a decision policy", p.Key)
	}
	if p.Mode != "" && !modePattern.MatchString(p.Mode) {
		return result, fmt.Errorf("invalid mode: %s", p.Mode)
	}
	context := PolicyContext{}
	if p.Policy != nil {
		context = *p.Policy
	}
	result.Checkout = context.Checkout
	if result.Checkout == "" && p.ConfPath != "" {
		if seat, err := helm.Locate(filepath.Dir(p.ConfPath)); err == nil {
			result.Checkout = seat.Checkout
		} else {
			result.Checkout = filepath.Dir(p.ConfPath)
		}
	}
	if context.CallingCheckout == "" {
		context.CallingCheckout = result.Checkout
	}
	readLayer := func(checkout, conf string, calling bool) (PolicyValue, bool, error) {
		layer := PolicyValue{Checkout: checkout, SetBy: "set outside settings; author unknown"}
		var found bool
		var err error
		if p.Key != "settings.apply" && calling {
			lookup := p.LookupEnv
			if lookup == nil {
				lookup = os.LookupEnv
			}
			if p.FlagSet {
				layer.Value, layer.Source, found = p.Flag, "flag", true
			} else if v, ok := lookup(EnvName(p.Key)); ok {
				layer.Value, layer.Source, found = v, "env", true
			}
		}
		if conf != "" && !found {
			paths := []string{conf + ".local", conf}
			if p.Key == "settings.apply" {
				paths = []string{conf}
			}
			for _, path := range paths {
				if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
					continue
				} else if statErr != nil {
					return layer, false, statErr
				}
				layer.Value, found, err = ConfLookup(path, p.Key)
				if err != nil {
					return layer, false, err
				}
				if found {
					layer.Source = "conf"
					if path != conf {
						layer.Source = "conf-local"
					}
					break
				}
			}
		}
		if found {
			if err := policyValueProblem(p.Key, layer.Value); err != nil {
				return layer, false, &PolicyReadError{Source: layer.Source, Checkout: checkout, Err: err}
			}
			path, pathErr := policyRecordPath(checkout, p.Key)
			if pathErr == nil {
				var record PolicyRecord
				data, readErr := os.ReadFile(path)
				if readErr == nil && json.Unmarshal(data, &record) == nil && record.Name == p.Key && record.Checkout == checkout && record.Value == layer.Value && record.Fingerprint == policyFingerprint(conf, p.Key, layer) && record.SetBy != "" && !record.At.IsZero() {
					layer.SetBy, layer.At = record.SetBy, record.At
				}
			}
		}
		return layer, found, nil
	}
	registry := PolicyRegistry{}
	if p.Key != "settings.apply" && context.Readers.Registry != nil {
		var err error
		registry, err = context.Readers.Registry(result.Checkout)
		if err != nil {
			return result, err
		}
	}
	underlying, found, err := readLayer(result.Checkout, p.ConfPath, result.Checkout == context.CallingCheckout)
	if err != nil {
		return result, err
	}
	if !found && p.Key != "settings.apply" && context.Readers.Registry != nil {
		if registry.Coordinator != "" && registry.Coordinator != result.Checkout {
			conf := filepath.Join(registry.Coordinator, "metasystem.conf")
			if context.Readers.ConfPath != nil {
				conf, err = context.Readers.ConfPath(registry.Coordinator)
				if err != nil {
					return result, err
				}
			}
			underlying, found, err = readLayer(registry.Coordinator, conf, false)
			if err != nil {
				return result, err
			}
			if found {
				underlying.Source = "coordinator/" + underlying.Source
			}
		}
	}
	if !found {
		value := "auto"
		if p.Key == "settings.apply" {
			value = "boundary"
		}
		underlying = PolicyValue{Value: value, Source: "built-in", Checkout: result.Checkout, SetBy: "built-in"}
	}
	result.PolicyValue = underlying
	// Scope remains the checkout whose policy is requested, even when a
	// coordinator supplied its default.
	result.Checkout = context.Checkout
	if result.Checkout == "" {
		result.Checkout = underlying.Checkout
	}
	if p.Key != "settings.apply" && result.Checkout != "" {
		readHelm := context.Readers.Helm
		if readHelm == nil {
			readHelm = helm.Active
		}
		state := readHelm(result.Checkout)
		if state.Malformed != "" {
			return result, &PolicyReadError{Source: "helm", Checkout: result.Checkout, Err: fmt.Errorf("%s", state.Malformed)}
		}
		if state.Diagnostic != "" && context.Checkout != "" {
			return result, fmt.Errorf("%s", state.Diagnostic)
		}
		if state.Active {
			result.Previous = &underlying
			result.Value, result.Source, result.SetBy, result.At, result.HelmHolder = "person", "helm", state.By, state.Since, state.By
		}
	}
	return result, nil
}

func policyRecordPath(checkout, key string) (string, error) {
	seat, err := helm.Locate(checkout)
	if err != nil {
		return "", err
	}
	return filepath.Join(seat.Dir, "policy", key+".json"), nil
}

func policyFingerprint(conf, key string, value PolicyValue) string {
	path := conf
	if value.Source == "conf-local" {
		path += ".local"
	}
	if value.Source != "conf" && value.Source != "conf-local" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		name, _, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(name) == key {
			return fmt.Sprintf("%x", sha256.Sum256([]byte(value.Source+"\x00"+line)))
		}
	}
	return ""
}

// WithPolicyLock serializes the configuration write and its attribution;
// callers recheck the registered destination inside the same lock.
func WithPolicyLock(checkout string, write func() error) error {
	seat, err := helm.Locate(checkout)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(seat.Dir, 0700); err != nil {
		return err
	}
	held, err := lock.File(filepath.Join(seat.Dir, "settings.lock"), 0600, lock.TryExclusive)
	if err != nil {
		return err
	}
	defer held.Release()
	return write()
}

func RecordPolicy(checkout, conf, key, value, by string, at time.Time) error {
	path, err := policyRecordPath(checkout, key)
	if err != nil {
		return err
	}
	if strings.TrimSpace(by) == "" {
		by = "author unknown"
	}
	current := PolicyValue{Value: value, Source: "conf-local", Checkout: checkout, SetBy: by, At: at.UTC()}
	record := PolicyRecord{Name: key, Checkout: checkout, Value: value, SetBy: by, At: at.UTC(), Previous: current, Fingerprint: policyFingerprint(conf, key, current)}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteFile(path, append(data, '\n'), 0600, "")
	return err
}
