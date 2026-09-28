// Package shellquote owns quoting a string as one shell word. It has three
// spellings because three surfaces render differently, and each spelling is
// part of what a surface prints or installs:
//
//   - Quote always single-quotes: commands the engine writes into files and
//     configuration (merge drivers, arm exports, printed next steps).
//   - Word leaves a safe word bare and single-quotes the rest: hook commands
//     a runtime reads back.
//   - Token matches bash's printf %q: paths a refusal lists for a person.
package shellquote

import "strings"

// Quote renders value in single quotes, closing and escaping each embedded
// single quote.
func Quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// Word renders value bare when it is a non-empty run of
// [A-Za-z0-9_@%+=:,./-], and as Quote otherwise.
func Word(value string) string {
	if value == "" {
		return Quote(value)
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if !isAlnum(c) && strings.IndexByte("_@%+=:,./-", c) < 0 {
			return Quote(value)
		}
	}
	return value
}

// Token renders value the way bash's printf %q does: safe words unchanged,
// other bytes backslash-escaped, and control bytes in $'...' form.
func Token(value string) string {
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
		if isTokenSafe(c) || c >= 0x80 {
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

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func isTokenSafe(c byte) bool {
	return isAlnum(c) || strings.IndexByte("_./-+,:=@%^", c) >= 0
}

func octal3(c byte) string {
	return string([]byte{'0' + c>>6&7, '0' + c>>3&7, '0' + c&7})
}
