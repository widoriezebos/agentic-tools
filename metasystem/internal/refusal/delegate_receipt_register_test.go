package refusal

import "testing"

func TestDelegateReceiptRefusalRegistration(t *testing.T) {
	want := Row{
		Code:     "DELEGATE_RECEIPT_REFUSED",
		Owner:    "internal/validate",
		Site:     "conformance.go:715",
		Shape:    Agent,
		Override: "remove the delegate change to memory/receipts.log, then validate conformance --stage review --job <that round>",
		Commands: 2,
	}
	var got Row
	for _, row := range Rows {
		if row.Code == want.Code {
			got = row
		}
	}
	if got != want {
		t.Fatalf("row = %+v, want %+v", got, want)
	}
}
