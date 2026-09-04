package channel

import (
	"testing"
	"time"
)

func TestStripCode(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		clean   string
		code    string
		present bool
	}{
		{name: "last field with punctuation", text: "approve 123456.", clean: "approve", code: "123456", present: true},
		{name: "digits before last field", text: "order 123456 now", clean: "order 123456 now"},
		{name: "code only", text: "123456", clean: "", code: "123456", present: true},
		{name: "five digits", text: "approve 12345", clean: "approve 12345"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clean, code, present := StripCode(test.text)
			if clean != test.clean || code != test.code || present != test.present {
				t.Fatalf("StripCode(%q) = (%q, %q, %t), want (%q, %q, %t)", test.text, clean, code, present, test.clean, test.code, test.present)
			}
		})
	}
}

func TestMaskCodesPreservesEveryOtherByte(t *testing.T) {
	const secret = "JBSWY3DPEHPK3PXP"
	at := time.Unix(1234567890, 0)
	code, err := TOTPCode(secret, at)
	if err != nil {
		t.Fatal(err)
	}
	fact := "123456"
	if fact == code {
		fact = "654321"
	}
	clean := "repeat\t" + code + " then " + code + ", fact " + fact + "\n"
	want := "repeat\t[code] then [code], fact " + fact + "\n"
	if got := MaskCodes(clean, secret, at); got != want {
		t.Fatalf("masked text = %q, want %q", got, want)
	}
}
