package channel

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const TOTPStep = int64(30)

func TOTPCode(secret string, at time.Time) (string, error) {
	return codeForStep(secret, at.Unix()/TOTPStep)
}

func codeForStep(secret string, step int64) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimRight(strings.TrimSpace(secret), "=")))
	if err != nil {
		return "", fmt.Errorf("invalid TOTP secret")
	}
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 15
	n := (uint32(sum[offset])&0x7f)<<24 | uint32(sum[offset+1])<<16 | uint32(sum[offset+2])<<8 | uint32(sum[offset+3])
	return fmt.Sprintf("%06d", n%1000000), nil
}

func VerifyTOTP(secret, code string, at time.Time) (int64, bool) {
	if len(code) != 6 {
		return 0, false
	}
	if _, err := strconv.Atoi(code); err != nil {
		return 0, false
	}
	current := at.Unix() / TOTPStep
	for _, step := range []int64{current - 1, current, current + 1} {
		expected, err := codeForStep(secret, step)
		if err == nil && hmac.Equal([]byte(expected), []byte(code)) {
			return step, true
		}
	}
	return 0, false
}

func SplitTOTP(text string) (answer, code string, ok bool) {
	fields := strings.Fields(text)
	if len(fields) < 2 {
		return "", "", false
	}
	code = fields[len(fields)-1]
	if len(code) != 6 {
		return "", "", false
	}
	if _, err := strconv.Atoi(code); err != nil {
		return "", "", false
	}
	answer = strings.Join(fields[:len(fields)-1], " ")
	return answer, code, answer != ""
}

func StripCode(text string) (clean, code string, present bool) {
	fieldStart, fieldEnd := lastField(text)
	if fieldStart == fieldEnd {
		return text, "", false
	}
	trimmedEnd := fieldEnd
	for trimmedEnd > fieldStart && strings.ContainsRune(".,;:!?", rune(text[trimmedEnd-1])) {
		trimmedEnd--
	}
	code = text[fieldStart:trimmedEnd]
	if !sixASCIIDigits(code) {
		return text, "", false
	}
	cleanEnd := fieldStart
	for cleanEnd > 0 {
		r, size := utf8.DecodeLastRuneInString(text[:cleanEnd])
		if !unicode.IsSpace(r) {
			break
		}
		cleanEnd -= size
	}
	return text[:cleanEnd] + text[fieldEnd:], code, true
}

func MaskCodes(clean, secret string, at time.Time) string {
	var masked strings.Builder
	for start := 0; start < len(clean); {
		r, size := utf8.DecodeRuneInString(clean[start:])
		if unicode.IsSpace(r) {
			masked.WriteString(clean[start : start+size])
			start += size
			continue
		}
		end := start + size
		for end < len(clean) {
			r, size = utf8.DecodeRuneInString(clean[end:])
			if unicode.IsSpace(r) {
				break
			}
			end += size
		}
		trimmedEnd := end
		for trimmedEnd > start && strings.ContainsRune(".,;:!?", rune(clean[trimmedEnd-1])) {
			trimmedEnd--
		}
		field := clean[start:trimmedEnd]
		if sixASCIIDigits(field) {
			if _, ok := VerifyTOTP(secret, field, at); ok {
				masked.WriteString("[code]")
				masked.WriteString(clean[trimmedEnd:end])
				start = end
				continue
			}
		}
		masked.WriteString(clean[start:end])
		start = end
	}
	return masked.String()
}

func lastField(text string) (int, int) {
	end := len(text)
	for end > 0 {
		r, size := utf8.DecodeLastRuneInString(text[:end])
		if !unicode.IsSpace(r) {
			break
		}
		end -= size
	}
	start := end
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(text[:start])
		if unicode.IsSpace(r) {
			break
		}
		start -= size
	}
	return start, end
}

func sixASCIIDigits(value string) bool {
	if len(value) != 6 {
		return false
	}
	for i := range value {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}
