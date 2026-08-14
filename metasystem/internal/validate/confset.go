package validate

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"os"
	"strings"
)

// ConfSetting is one key=value assignment for SetConfKeys.
type ConfSetting struct {
	Key   string
	Value string
}

// SetConfKeys rewrites a metasystem.conf-format file so each setting's
// key holds exactly its value: the first line carrying the key is
// replaced in place, later duplicates of it are dropped, and a key the
// file does not carry is appended at the end. Comments, blank lines,
// and unrelated keys keep their positions. The rewrite is atomic, like
// TailorConf's.
func SetConfKeys(confPath string, settings []ConfSetting) error {
	data, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}

	pending := map[string]string{}
	order := []string{}
	for _, setting := range settings {
		if _, known := pending[setting.Key]; !known {
			order = append(order, setting.Key)
		}
		pending[setting.Key] = setting.Value
	}

	var out []string
	written := map[string]bool{}
	for _, raw := range splitLines(string(data)) {
		stripped := strings.TrimSpace(raw)
		if stripped == "" || strings.HasPrefix(stripped, "#") || !strings.Contains(raw, "=") {
			out = append(out, raw)
			continue
		}
		key := strings.TrimSpace(strings.SplitN(raw, "=", 2)[0])
		value, wanted := pending[key]
		if !wanted {
			out = append(out, raw)
			continue
		}
		if !written[key] {
			written[key] = true
			out = append(out, key+"="+value)
		}
	}
	for _, key := range order {
		if !written[key] {
			out = append(out, key+"="+pending[key])
		}
	}

	_, err = atomicfile.WriteText(confPath, strings.Join(out, "\n")+"\n", "")
	return err
}
