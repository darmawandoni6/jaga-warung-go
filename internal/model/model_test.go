package model

import (
	"encoding/json"
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

// Table names must equal schema.ts exactly (D8).
func TestTableNames(t *testing.T) {
	want := map[string]string{
		"Account":         "accounts",
		"RefreshToken":    "refresh_tokens",
		"Product":         "products",
		"Category":        "categories",
		"Transaction":     "transactions",
		"TransactionItem": "transaction_items",
		"Debt":            "debts",
		"DebtPayment":     "debt_payments",
		"CashFlow":        "cash_flows",
		"StockMovement":   "stock_movements",
	}

	models := AllModels()
	if len(models) != len(want) {
		t.Fatalf("AllModels() = %d models, want %d", len(models), len(want))
	}

	cache := &sync.Map{}
	for _, m := range models {
		s, err := schema.Parse(m, cache, schema.NamingStrategy{})
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if got := want[s.Name]; got != s.Table {
			t.Errorf("%s table = %q, want %q", s.Name, s.Table, got)
		}
	}
}

// deleted_at must reach the client so a delete is a visible write (B2).
func TestJSONContract(t *testing.T) {
	b, err := json.Marshal(Product{Barcode: "1"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["deleted_at"]; !ok {
		t.Error("deleted_at must be serialized")
	}
}

// Money is BIGINT rupiah, never fractional (D3).
func TestMoneyIsInteger(t *testing.T) {
	cache := &sync.Map{}
	for _, m := range AllModels() {
		s, err := schema.Parse(m, cache, schema.NamingStrategy{})
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		for _, f := range s.Fields {
			if f.FieldType.Kind() == 32 || f.FieldType.Kind() == 64 { // float32, float64
				t.Errorf("%s.%s is %s, money must be integer rupiah (D3)", s.Name, f.Name, f.FieldType)
			}
		}
	}
}

// No sync columns survive: sync is deferred, not half-built.
func TestNoSyncColumns(t *testing.T) {
	cache := &sync.Map{}
	for _, m := range AllModels() {
		s, err := schema.Parse(m, cache, schema.NamingStrategy{})
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		for _, f := range s.Fields {
			if f.DBName == "owner_id" || f.DBName == "seq" {
				t.Errorf("%s has %s; sync is deferred", s.Name, f.DBName)
			}
		}
	}
}

// FK actions come from the associations now, so they must be declared there.
// Dropping a constraint:OnDelete silently downgrades to NO ACTION.
func TestFKOnDelete(t *testing.T) {
	want := []struct{ schema, field, action string }{
		{"Transaction", "Items", "CASCADE"},
		{"TransactionItem", "Product", "RESTRICT"},
		{"Product", "Movements", "RESTRICT"},
		{"Debt", "Payments", "CASCADE"},
	}

	cache := &sync.Map{}
	for _, w := range want {
		var m any
		for _, candidate := range AllModels() {
			if s, err := schema.Parse(candidate, cache, schema.NamingStrategy{}); err == nil && s.Name == w.schema {
				m = candidate
				break
			}
		}
		if m == nil {
			t.Fatalf("model %s not found", w.schema)
		}
		s, err := schema.Parse(m, cache, schema.NamingStrategy{})
		if err != nil {
			t.Fatalf("parse %s: %v", w.schema, err)
		}
		rel, ok := s.Relationships.Relations[w.field]
		if !ok {
			t.Fatalf("%s.%s relationship missing", w.schema, w.field)
		}
		if rel.ParseConstraint().OnDelete != w.action {
			t.Errorf("%s.%s ON DELETE = %q, want %q", w.schema, w.field, rel.ParseConstraint().OnDelete, w.action)
		}
	}
}
