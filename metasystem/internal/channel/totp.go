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
