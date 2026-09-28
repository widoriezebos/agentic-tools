package refusal

import "testing"

func TestDelegateReceiptRefusalRegistration(t *testing.T) {
	want := Row{
		Code:     "DELEGATE_RECEIPT_REFUSED",
		Owner:    "internal/validate",
		Site:     "conformance.go#conformanceRun.boundaryViolations",
		Shape:    Agent,
		Override: "remove the delegate change to memory/receipts.log, then work review j2:<that round> --check-only --stage review",
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
