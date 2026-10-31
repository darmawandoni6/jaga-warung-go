package domain

import "testing"

// DeriveDebtStatus is the single source of the active/partial/paid transition,
// so it gets table tests: a wrong branch here silently mislabels every debt.
func TestDeriveDebtStatus(t *testing.T) {
	cases := []struct {
		name      string
		totalDebt int64
		paid      int64
		want      string
	}{
		{"nothing paid", 100000, 0, DebtStatusActive},
		{"partially paid", 100000, 40000, DebtStatusPartial},
		{"fully paid", 100000, 100000, DebtStatusPaid},
		{"overpaid", 100000, 120000, DebtStatusPaid},
		// A zero-total debt with no payment is not "paid": there is nothing to
		// settle, so it stays active rather than reading as a settled debt.
		{"zero total, nothing paid", 0, 0, DebtStatusActive},
		{"zero total with payment", 0, 5000, DebtStatusPaid},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DeriveDebtStatus(tc.totalDebt, tc.paid); got != tc.want {
				t.Fatalf("DeriveDebtStatus(%d, %d) = %q, want %q",
					tc.totalDebt, tc.paid, got, tc.want)
			}
		})
	}
}

// bad_debt must never be produced by the derivation: it is a manual write-off.
func TestDeriveDebtStatus_NeverReturnsBadDebt(t *testing.T) {
	for _, total := range []int64{0, 1000, 100000} {
		for _, paid := range []int64{0, 1, 500, 1000, 200000} {
			if got := DeriveDebtStatus(total, paid); got == DebtStatusBadDebt {
				t.Fatalf("DeriveDebtStatus(%d, %d) returned bad_debt, which must be manual",
					total, paid)
			}
		}
	}
}

func TestValidDebtStatus(t *testing.T) {
	valid := []string{DebtStatusActive, DebtStatusPartial, DebtStatusPaid, DebtStatusBadDebt}
	for _, s := range valid {
		if !ValidDebtStatus(s) {
			t.Fatalf("%q should be valid", s)
		}
	}

	invalid := []string{"", "Active", "PAID", "cancelled", "bad debt"}
	for _, s := range invalid {
		if ValidDebtStatus(s) {
			t.Fatalf("%q should be rejected", s)
		}
	}
}

// The constants must match the chk_debt_status CHECK constraint in
// docs/schema.md, or inserts fail at the DB level.
func TestDebtStatusConstantsMatchSchema(t *testing.T) {
	allowed := map[string]bool{
		"active":   true,
		"partial":  true,
		"paid":     true,
		"bad_debt": true,
	}
	for _, s := range []string{DebtStatusActive, DebtStatusPartial, DebtStatusPaid, DebtStatusBadDebt} {
		if !allowed[s] {
			t.Fatalf("status %q is not in the schema CHECK constraint", s)
		}
	}
}
