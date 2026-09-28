package landpath

import (
	"path/filepath"
	"strings"
)

// WrapperToken is the live commit wrapper token the pre-commit guard
// verifies (internal/validate.WrapperToken): the wrapper's pid and kernel
// start second, a fresh 32-hex-character nonce, and the creation time. Its
// JSON form is the one `lease commit-token` wrote for the shell wrapper, so
// a guard at any base verifies a token this path mints.
type WrapperToken struct {
	WrapperPid          int64  `json:"wrapperPid"`
	WrapperPidStartedAt int64  `json:"wrapperPidStartedAt"`
	Nonce               string `json:"nonce"`
	CreatedAt           string `json:"createdAt"`
}

// TokenPath is the one per-checkout wrapper token path.
func TokenPath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "mains", "worktree-commit-token.json")
}

// shellQuote renders one path the way bash's printf %q does for the paths a
// refusal lists: safe words unchanged, other bytes backslash-escaped, and
// control bytes in $'...' form.
func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	control := false
	for i := 0; i < len(value); i++ {
		if value[i] < 0x20 || value[i] == 0x7f {
			control = true
			break
		}
	}
	if control {
		var out strings.Builder
		out.WriteString("$'")
		for i := 0; i < len(value); i++ {
			c := value[i]
			switch {
			case c == '\n':
				out.WriteString(`\n`)
			case c == '\t':
				out.WriteString(`\t`)
			case c == '\r':
				out.WriteString(`\r`)
			case c < 0x20 || c == 0x7f:
				out.WriteString(`\` + octal3(c))
			case c == '\'' || c == '\\':
				out.WriteByte('\\')
				out.WriteByte(c)
			default:
				out.WriteByte(c)
			}
		}
		out.WriteString("'")
		return out.String()
	}
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		if isShellSafe(c) || c >= 0x80 {
			out.WriteByte(c)
			continue
		}
		if c == '~' && i != 0 {
			out.WriteByte(c)
			continue
		}
		out.WriteByte('\\')
		out.WriteByte(c)
	}
	return out.String()
}

func isShellSafe(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	return strings.IndexByte("_./-+,:=@%^", c) >= 0
}

func octal3(c byte) string {
	return string([]byte{'0' + c>>6&7, '0' + c>>3&7, '0' + c&7})
}
