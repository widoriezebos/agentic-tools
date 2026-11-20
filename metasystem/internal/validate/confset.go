package validate

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// ConfSetting is one key=value assignment for SetConfKeys.
type ConfSetting struct {
	Key   string
	Value string
	Unset bool
}

// SetConfKeys rewrites a metasystem.conf-format file so each setting's
// key holds exactly its value: the first line carrying the key is
// replaced in place, later duplicates of it are dropped, and a key the
// file does not carry is appended at the end. Comments, blank lines,
// and unrelated keys keep their positions. The rewrite is atomic, like
// TailorConf's.
func SetConfKeys(confPath string, settings []ConfSetting, sync atomicfile.DirSync) (bool, error) {
	data, err := os.ReadFile(confPath)
	if err != nil {
		return false, err
	}

	pending := map[string]string{}
	removed := map[string]bool{}
	order := []string{}
	for _, setting := range settings {
		if _, known := pending[setting.Key]; !known {
			order = append(order, setting.Key)
		}
		pending[setting.Key] = setting.Value
		removed[setting.Key] = setting.Unset
	}

	var out []string
	written := map[string]bool{}
	for _, raw := range splitLines(string(data)) {
		stripped := strings.TrimSpace(raw)
		if stripped == "" || !strings.Contains(raw, "=") {
			out = append(out, raw)
			continue
		}
		key := strings.TrimSpace(strings.SplitN(raw, "=", 2)[0])
		value, wanted := pending[key]
		if !wanted {
			out = append(out, raw)
			continue
		}
		if removed[key] {
			continue
		}
		if !written[key] {
			written[key] = true
			out = append(out, key+"="+value)
		}
	}
	for _, key := range order {
		if !written[key] && !removed[key] {
			out = append(out, key+"="+pending[key])
		}
	}

	return sync.WriteFile(confPath, []byte(strings.Join(out, "\n")+"\n"), 0600, filepath.Dir(confPath))
}

// SetAttributed publishes the local value before its attribution under the caller's settings lock.
func SetAttributed(checkout, conf string, setting ConfSetting, by string, at time.Time, sync atomicfile.DirSync) (current config.PolicyValue, same, durable bool, err error) {
	local := conf + ".local"
	data, err := os.ReadFile(local)
	if os.IsNotExist(err) {
		err = os.WriteFile(local, nil, 0600)
	}
	if err != nil {
		return
	}
	value, found, _ := config.ConfLookup(local, setting.Key)
	same = found && !setting.Unset && value == setting.Value || !found && setting.Unset
	if same {
		durable, _ = sync.Republish(local, data)
		// An existing identical value remains visible if its re-publication fails.
	} else {
		settings := []ConfSetting{setting}
		if config.PolicyScope(setting.Key) != "" || found {
			settings = append(settings, ConfSetting{Key: config.PolicyRevisionKey(setting.Key), Value: rand.Text(), Unset: setting.Unset})
		}
		durable, err = SetConfKeys(local, settings, sync)
		if err != nil {
			return
		}
	}
	if setting.Unset {
		return current, same, durable, nil
	}
	current, recorded := config.RecordPolicy(checkout, conf, setting.Key, setting.Value, by, at, same, sync)
	return current, same, durable && recorded, nil
}
