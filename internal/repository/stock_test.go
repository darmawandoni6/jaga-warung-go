package repository

import (
	"testing"

	"jaga-warung-go/internal/domain"
)

// stockSign is the whole of "stock is derived": quantity is stored positive and
// the direction comes from the type. If this drifts, products.stock drifts.
func TestStockSign(t *testing.T) {
	cases := []struct {
		movementType string
		want         int64
	}{
		{domain.MovementRestock, 1},
		{domain.MovementAdjustmentGain, 1},
		{domain.MovementSale, -1},
		{domain.MovementAdjustmentLoss, -1},
		// An unknown type must not silently add stock.
		{"bogus", 0},
	}

	for _, tc := range cases {
		t.Run(tc.movementType, func(t *testing.T) {
			if got := stockSign(tc.movementType); got != tc.want {
				t.Fatalf("stockSign(%q) = %d, want %d", tc.movementType, got, tc.want)
			}
		})
	}
}

// The four movement types in domain must stay in sync with the CHECK constraint
// documented in docs/schema.md, otherwise inserts fail at the DB level.
func TestMovementTypesMatchSchema(t *testing.T) {
	allowed := map[string]bool{
		"restock":         true,
		"sale":            true,
		"adjustment_loss": true,
		"adjustment_gain": true,
	}

	for _, mt := range []string{
		domain.MovementRestock,
		domain.MovementSale,
		domain.MovementAdjustmentLoss,
		domain.MovementAdjustmentGain,
	} {
		if !allowed[mt] {
			t.Fatalf("movement type %q is not in the schema CHECK constraint", mt)
		}
	}
}

func TestValidMovementType(t *testing.T) {
	for _, mt := range []string{
		domain.MovementRestock,
		domain.MovementSale,
		domain.MovementAdjustmentLoss,
		domain.MovementAdjustmentGain,
	} {
		if !domain.ValidMovementType(mt) {
			t.Fatalf("%q should be valid", mt)
		}
	}

	for _, s := range []string{"", "Restock", "SALE", "magic", "adjustment"} {
		if domain.ValidMovementType(s) {
			t.Fatalf("%q should be rejected", s)
		}
	}
}
