package domain

import "testing"

func TestConfidenceForEvent(t *testing.T) {
	cases := []struct {
		name      string
		kind      EventKind
		source    string
		estimated bool
		want      Confidence
	}{
		{"purchase always exact", EventPurchase, "purchase_receipt", false, ConfidenceExact},
		{"purchase ignores estimated flag", EventPurchase, "shopping_order", true, ConfidenceExact},
		{"home_prepared purchase is exact", EventPurchase, "home_prepared", false, ConfidenceExact},
		{"counted consume is exact", EventConsume, "manual_count", false, ConfidenceExact},
		{"estimated consume is estimated", EventConsume, "manual_count", true, ConfidenceEstimated},
		{"mark_empty is always exact even if flagged estimated", EventConsume, SourceMarkEmpty, true, ConfidenceExact},
		{"counted discard is exact", EventDiscard, "manual_count", false, ConfidenceExact},
		{"estimated discard is estimated", EventDiscard, "manual_count", true, ConfidenceEstimated},
		{"counted adjust is exact", EventAdjust, "manual_count", false, ConfidenceExact},
		{"estimated adjust is estimated", EventAdjust, "manual_count", true, ConfidenceEstimated},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ConfidenceForEvent(c.kind, c.source, c.estimated)
			if got != c.want {
				t.Errorf("ConfidenceForEvent(%s, %s, %v) = %s, want %s", c.kind, c.source, c.estimated, got, c.want)
			}
		})
	}
}

func TestNormalizeGTIN(t *testing.T) {
	// Real GTIN-13 (EAN-13) with a valid check digit.
	got, err := NormalizeGTIN("7300400176354")
	if err != nil {
		t.Fatalf("NormalizeGTIN: %v", err)
	}
	want := "07300400176354" // 13 digits, left-padded with one zero to 14
	if got != want {
		t.Errorf("NormalizeGTIN(valid13) = %q, want %q", got, want)
	}
	if len(got) != 14 {
		t.Errorf("expected 14-digit canonical form, got %d digits: %q", len(got), got)
	}

	// Same digits with a non-digit separator stripped before validation.
	got2, err := NormalizeGTIN("7-300400176354")
	if err != nil {
		t.Fatalf("NormalizeGTIN with dash: %v", err)
	}
	if got2 != got {
		t.Errorf("NormalizeGTIN with dash = %q, want %q (same as without dash)", got2, got)
	}

	// GTIN-8 (EAN-8) — a valid 8-digit barcode.
	got8, err := NormalizeGTIN("96385074")
	if err != nil {
		t.Fatalf("NormalizeGTIN(valid8): %v", err)
	}
	want8 := "00000096385074" // 8 digits, left-padded with six zeros to 14
	if got8 != want8 {
		t.Errorf("NormalizeGTIN(valid8) = %q, want %q", got8, want8)
	}
	if len(got8) != 14 {
		t.Errorf("expected 14-digit canonical form for GTIN-8, got %d digits: %q", len(got8), got8)
	}

	// GTIN-12 (UPC-A) — a valid 12-digit barcode.
	got12, err := NormalizeGTIN("012345678905")
	if err != nil {
		t.Fatalf("NormalizeGTIN(valid12): %v", err)
	}
	want12 := "00012345678905" // 12 digits, left-padded with two zeros to 14
	if got12 != want12 {
		t.Errorf("NormalizeGTIN(valid12) = %q, want %q", got12, want12)
	}
	if len(got12) != 14 {
		t.Errorf("expected 14-digit canonical form for GTIN-12, got %d digits: %q", len(got12), got12)
	}

	// Wrong check digit (last digit altered from the valid 7300400176354).
	if _, err := NormalizeGTIN("7300400176353"); err == nil {
		t.Error("expected error for invalid check digit, got nil")
	}

	// Wrong length.
	if _, err := NormalizeGTIN("12345"); err == nil {
		t.Error("expected error for invalid length, got nil")
	}
}

func TestWouldCreateLocationCycle(t *testing.T) {
	// Self-parent is always a cycle, regardless of ancestor list.
	if !WouldCreateLocationCycle("basement", "basement", nil) {
		t.Error("expected self-parent to be a cycle")
	}

	// basement -> freezer would create a cycle if freezer's ancestors already include basement
	// (i.e. basement is already an ancestor of freezer, so basement can't also become freezer's
	// descendant).
	if !WouldCreateLocationCycle("basement", "freezer", []string{"basement", "house"}) {
		t.Error("expected a cycle when candidate parent's ancestors include the child")
	}

	// A genuinely new, unrelated nesting is fine.
	if WouldCreateLocationCycle("chest-freezer", "basement", []string{"house"}) {
		t.Error("did not expect a cycle for an unrelated parent")
	}

	// No ancestors at all (candidate parent is currently a root) is fine.
	if WouldCreateLocationCycle("drawer", "fridge", nil) {
		t.Error("did not expect a cycle when candidate parent has no ancestors")
	}
}
