package usecase

import (
	"testing"

	"jaga-warung-go/internal/domain"
)

func TestTranslateStatus(t *testing.T) {
	cases := []struct {
		transaction string
		fraud       string
		want        string
	}{
		{"settlement", "accept", domain.PaymentStatusPaid},
		{"capture", "accept", domain.PaymentStatusPaid},
		{"capture", "challenge", domain.PaymentStatusPending},
		{"expire", "", domain.PaymentStatusExpired},
		{"pending", "", domain.PaymentStatusPending},
		{"deny", "", domain.PaymentStatusCanceled},
		{"cancel", "", domain.PaymentStatusCanceled},
		{"unknown", "", domain.PaymentStatusPending},
	}
	for _, tc := range cases {
		got := translateStatus(tc.transaction, tc.fraud)
		if got != tc.want {
			t.Errorf("translateStatus(%q, %q) = %q, want %q", tc.transaction, tc.fraud, got, tc.want)
		}
	}
}
